package main

import (
	"bytes"
	"encoding/gob"
	"html/template"
	"log"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/boltdb/bolt"
	"github.com/gorilla/mux"
	"github.com/robwc/godragon"
)

var db *bolt.DB

var siteT *template.Template

var champPage string = `{{ define "body" }}
<h1>{{ .Name }} --- {{ .Title }}</h1>
<p>{{ .Blurb }}</p>

<h2>Type: {{$tlen := len .Tags}}{{ range $i, $e := .Tags}}{{$e}}{{ $v := add $i 1}}{{if ne $v $tlen}}, {{end}}{{end}}</h2>

<table>
<tr><td>HP</td><td>{{ .Stats.HP }}(+{{.Stats.HPPerLevel}})</td><tr>
<tr><td>HP Regen</td><td>{{.Stats.HPRegen}}(+{{.Stats.HPRegenPerLevel}})</td></tr> 
<tr><td>Mana</td><td>{{ .Stats.MP }}(+{{.Stats.MPPerLevel}})</td></tr>
<tr><td>Mana Regen</td><td>{{ .Stats.MPRegen}}(+{{.Stats.MPRegenPerLevel}})</td></tr>
<tr><td>Armor</td><td>{{ .Stats.Armor}}(+{{.Stats.ArmorPerLevel}})</td></tr>
<tr><td>MR</td><td>{{.Stats.SpellBlock}}(+{{.Stats.SpellBlockPerLevel}})</td></tr>
<tr><td>AD</td><td>{{ .Stats.AttackDamage}}(+{{.Stats.AttackDamagePerLevel}})</td></tr>
<tr><td>AS</td><td>{{ ascalc .Stats.AttackSpeedOffset}}(+{{.Stats.AttackSpeedPerLevel}})</td></tr>
<tr><td>Crit</td><td>{{ .Stats.Crit}}(+{{.Stats.CritPerLevel}})</td></tr>
<tr><td>Range</td><td>{{ .Stats.AttackRange}}</td></tr>
<tr><td>MS</td><td>{{ .Stats.MoveSpeed }}</td><tr>          
</table>
{{ end }}`

var page string = `<html>
<head>{{ template "head" .}}</head>
<body>{{ template "body" .}}</body>
</html>`

var head string = `
{{ define "head" }}
<title>LoL Facts</title>
{{ end }}`

func updateDatabase(db *bolt.DB, version string) error {

	champs, err := godragon.StaticChampions(version)
	if err != nil {
		return err
	}
	var buff bytes.Buffer

	db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("champs"))
		return err
	})

	for i := range champs {
		db.Update(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte("champs"))

			enc := gob.NewEncoder(&buff)
			enc.Encode(champs[i])
			err := b.Put([]byte(champs[i].Name), buff.Bytes())
			buff.Reset()
			return err
		})
	}
	return nil
}

func init() {
	siteT = template.New("site")
	siteT, _ = siteT.Parse(page)
	siteT, _ = siteT.Parse(head)

}

func main() {
	// load up database
	var err error
	db, err = bolt.Open("champs.db", 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	updateDatabase(db, "6.5.1")

	r := mux.NewRouter()
	r.HandleFunc("/", homeHandler)
	r.HandleFunc("/champ/{name}", lolChamp)
	http.Handle("/", r)
	http.ListenAndServe(":8888", nil)
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

func homeHandler(resp http.ResponseWriter, req *http.Request) {

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

		fmap := template.FuncMap{"add": add, "mult": mult, "ascalc": ascalc}

		champT, err := siteT.Funcs(fmap).Parse(champPage)
		if err != nil {
			panic(err)
		}

		champT.Execute(resp, &champ)
		return nil
	})

}
