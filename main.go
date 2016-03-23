package main

import (
	"bytes"
	"encoding/base64"
	"encoding/gob"
	"image/jpeg"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/boltdb/bolt"
	"github.com/gorilla/mux"
	"github.com/robwc/godragon"
)

var db *bolt.DB

var siteT *template.Template
var fmap template.FuncMap

var homePage string = `{{ define "body" }}
<div class="row">

<div class="col-md-6">
<div class="panel panel-default">
<div class="panel-heading">Champs</div>
<div class="panel-body">
<table>
{{ range $k, $v := .Champs }}
<tr><td><a href="/champ/{{html $v.Name}}">{{html $v.Name}}</a></td></tr>
{{ end }}
</table>
</div>
</div>
</div>

<div class="col-md-6">
<div class="panel panel-default">
<div class="panel-heading">Items</div>
<div class="panel-body">
<table>
{{ range $k, $v := .Items }}
<tr><td><a href="/item/{{html $v.Name}}">{{html $v.Name}}</a></td></tr>
{{ end }}
</table>
</div>
</div>
</div>

</div>
{{ end }}`

var itemPage string = `{{ define "body" }}
<h1>{{.Name}}</h1>
<img src="data:image/jpg;base64,{{.Image.Encoded}}">
{{ end }}`

var champPage string = `{{ define "body" }}
<h1>{{ .Name }} --- {{ .Title }}</h1>
<img src="data:image/jpg;base64,{{.Image.Encoded}}">
<p>{{ .Blurb }}</p>

<h2>Type: {{$tlen := len .Tags}}{{ range $i, $e := .Tags}}{{$e}}{{ $v := add $i 1}}{{if ne $v $tlen}}, {{end}}{{end}}</h2>

<table class="table">
<tr><td>HP</td><td>{{ .Stats.HP }}(+{{.Stats.HPPerLevel}})</td><tr>
<tr><td>HP Regen</td><td>{{.Stats.HPRegen}}(+{{.Stats.HPRegenPerLevel}})</td></tr> 
<tr><td>Mana</td><td>{{ .Stats.MP }}(+{{.Stats.MPPerLevel}})</td></tr>
<tr><td>Mana Regen</td><td>{{ .Stats.MPRegen}}(+{{.Stats.MPRegenPerLevel}})</td></tr>
<tr><td>Armor</td><td>{{ .Stats.Armor}}(+{{.Stats.ArmorPerLevel}})</td></tr>
<tr><td>MR</td><td>{{.Stats.SpellBlock}}(+{{.Stats.SpellBlockPerLevel}})</td></tr>
<tr><td>AD</td><td>{{ .Stats.AttackDamage}}(+{{.Stats.AttackDamagePerLevel}})</td></tr>
<tr><td>AS</td><td>{{ ascalc .Stats.AttackSpeedOffset}}(+{{asplcalc .Stats.AttackSpeedPerLevel}})</td></tr>
<tr><td>Crit</td><td>{{ .Stats.Crit}}(+{{.Stats.CritPerLevel}})</td></tr>
<tr><td>Range</td><td>{{ .Stats.AttackRange}}</td></tr>
<tr><td>MS</td><td>{{ .Stats.MoveSpeed }}</td><tr>          
</table>

<h3>Spells</h3>
<div class="panel panel-default">
<div class="panel-heading">
  <div class="panel-title">{{.Passive.Name}}</div>
</div>
<div class="panel-body">
<img src="data:image/jpg;base64,{{.Passive.Image.Encoded}}">
<p>Passive</p>
<p>
{{.Passive.Description}}
</p>
</div>
</div>
{{ range $i, $v := .Spells }}
<div class="panel panel-default">
<div class="panel-heading">
  <div class="panel-title">{{$v.Name}}</div>
</div>
<div class="panel-body">
<img src="data:image/jpg;base64,{{$v.Image.Encoded}}">
<p>{{ $tlen := len .Cooldown }}{{ range $i, $e := .Cooldown}}{{$v := add $i 1}}{{ $e }}{{ if ne $v $tlen}}/{{end }}{{ end }}</p>
<p>
{{$v.Description}}
</p>
</div>
</div>
{{end}}
{{ end }}`

var page string = `
<!DOCTYPE html>
<html lang="en">
<head>{{ template "head" .}}</head>
<body><div class="container">{{ template "body" .}}</div></body>
</html>`

var head string = `
{{ define "head" }}
<title>LoL Facts</title>
<meta charset="utf-8"> 
<meta name="viewport" content="width=device-width, initial-scale=1">
<link href="https://maxcdn.bootstrapcdn.com/bootstrap/3.3.6/css/bootstrap.min.css" rel="stylesheet" integrity="sha384-1q8mTJOASx8j1Au+a5WDVnPi2lkFfwwEAa8hDDdjZlpLegxhjVME1fgjWPGmkzs7" crossorigin="anonymous">
<style>
td {width:50%;}
</style>
{{ end }}`

func updateDatabase(db *bolt.DB, version string) error {

	champs, err := godragon.NewStaticChampions(os.Getenv("RIOTKEY"))
	if err != nil {
		return err
	}

	items, err := godragon.StaticItems(version)
	if err != nil {
		return err
	}

	db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte("champs"))
		sb, err := tx.CreateBucketIfNotExists([]byte("splash"))

		for i := range champs {

			var buff bytes.Buffer

			champ := champs[i]

			champ.Image.EncodeImage("6.5.1")
			champ.Passive.Image.EncodeImage("6.5.1")

			for s := range champ.Spells {
				champ.Spells[s].Image.EncodeImage("6.5.1")
			}

			champName := strings.Replace(champ.Name, " ", "", -1)
			champName = strings.Replace(champName, ".", "", -1)
			champName = strings.Replace(champName, "'", "", -1)
			champName = strings.Replace(champName, "'", "", -1)
			champName = strings.Replace(champName, "'", "", -1)

			simg, err := godragon.FetchChampSplashImage(champName, 0)
			if err != nil {
				log.Println(err)
			} else {

				var ibuff bytes.Buffer
				err = jpeg.Encode(&ibuff, simg, nil)
				if err != nil {
					log.Println(err)
				}

				encsimg := base64.StdEncoding.EncodeToString(ibuff.Bytes())

				err = sb.Put([]byte(champName), []byte(encsimg))
				if err != nil {
					log.Println(err)
				}
			}

			enc := gob.NewEncoder(&buff)
			enc.Encode(champ)
			err = b.Put([]byte(champ.Name), buff.Bytes())
			if err != nil {
				log.Println(err)
			}
		}

		ib, err := tx.CreateBucketIfNotExists([]byte("items"))

		for i := range items {
			var buff bytes.Buffer

			item := items[i]

			item.Image.EncodeImage(version)

			enc := gob.NewEncoder(&buff)
			enc.Encode(item)
			err := ib.Put([]byte(item.Name), buff.Bytes())
			if err != nil {
				log.Println(err)
			}
		}
		return err

	})
	return nil
}

func init() {
	siteT = template.New("site")
	siteT, _ = siteT.Parse(page)
	siteT, _ = siteT.Parse(head)

	fmap = template.FuncMap{"add": add,
		"mult":     mult,
		"ascalc":   ascalc,
		"asplcalc": asplcalc,
		"spesc":    spesc,
		"unsp":     unsp}

}

func main() {
	// load up database
	var err error
	db, err = bolt.Open("champs.db", 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	/*err = updateDatabase(db, "6.2.1")
	if err != nil {
		log.Fatal(err)
	}
	*/

	r := mux.NewRouter()
	r.HandleFunc("/", homeHandler)
	r.HandleFunc("/champ/{name}", lolChamp)
	r.HandleFunc("/item/{name}", lolItem)
	http.Handle("/", r)
	log.Println("listening")
	http.ListenAndServe("127.0.0.1:8888", nil)
}

func add(a, b int) int {
	return a + b
}

func mult(a int, b float32) float32 {
	return float32(a) * b
}

func ascalc(aso float32) string {
	s := math.Pow(10, float64(3))
	v := float64(0.625 / (math.Floor((1-float64(aso))*s) / s))
	nv := strconv.FormatFloat(v, 'f', -1, 32)
	return nv[:5]
}

func asplcalc(as float32) string {
	s := math.Pow(10, float64(3))
	v := float64(0.625 / (math.Floor((1-float64(as))*s) / s))
	nv := strconv.FormatFloat(v, 'f', -1, 32)
	return nv
}

func spesc(txt string) string {
	return strings.Replace(txt, " ", "_", -1)
}

func unsp(txt string) string {
	return strings.Replace(txt, "_", " ", -1)
}

func homeHandler(resp http.ResponseWriter, req *http.Request) {

	type Home struct {
		Items  []*godragon.Item
		Champs []*godragon.Champion
	}

	db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("champs"))

		c := b.Cursor()

		h := &Home{}

		for k, v := c.First(); k != nil; k, v = c.Next() {
			var champ godragon.Champion
			var buff bytes.Buffer

			buff.Write(v)
			dec := gob.NewDecoder(&buff)
			if err := dec.Decode(&champ); err != nil {
				log.Println(err)
				resp.Write([]byte("Not Found"))
				return err
			}

			h.Champs = append(h.Champs, &champ)
		}

		b = tx.Bucket([]byte("items"))

		c = b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			var item godragon.Item
			var buff bytes.Buffer

			buff.Write(v)
			dec := gob.NewDecoder(&buff)
			if err := dec.Decode(&item); err != nil {
				log.Println(err)
				resp.Write([]byte("Not Found"))
				return err
			}

			h.Items = append(h.Items, &item)
		}

		homeT, err := siteT.Funcs(fmap).Parse(homePage)
		if err != nil {
			panic(err)
		}

		homeT.Execute(resp, &h)
		return nil

	})
}

func lolItem(resp http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	name := unsp(vars["name"])

	db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("items"))
		i := b.Get([]byte(name))

		var buff bytes.Buffer
		var item godragon.Item
		buff.Write(i)
		dec := gob.NewDecoder(&buff)
		if err := dec.Decode(&item); err != nil {
			log.Println(err)
			resp.Write([]byte("Not Found"))
			return err
		}

		itemT, err := siteT.Funcs(fmap).Parse(itemPage)
		if err != nil {
			panic(err)
		}

		itemT.Execute(resp, &item)
		return nil

	})
}

func lolChamp(resp http.ResponseWriter, req *http.Request) {

	vars := mux.Vars(req)
	name := vars["name"]

	db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("champs"))
		c := b.Get([]byte(name))

		var buff bytes.Buffer
		var champ godragon.Champion
		buff.Write(c)
		dec := gob.NewDecoder(&buff)
		if err := dec.Decode(&champ); err != nil {
			log.Println(err)
			resp.Write([]byte("Not Found"))
			return err
		}
		/*
			bb := tx.Bucket([]byte("splash"))
			bimg := bb.Get([]byte(name))

			var ibuff bytes.Buffer
			var img string
			buff.Write(bimg)
			dec = gob.NewDecoder(&ibuff)
			if err := dec.Decode(&img); err != nil {
				log.Println(err)
				return err
			}

			var bbuff bytes.Buffer
			type Bg struct {
				Background string
			}
			bg := &Bg{}
			siteT.ExecuteTemplate(&bbuff, "site", &bg)
		*/
		champT, err := siteT.Funcs(fmap).Parse(champPage)
		if err != nil {
			panic(err)
		}

		champT.Execute(resp, &champ)
		return nil
	})

}
