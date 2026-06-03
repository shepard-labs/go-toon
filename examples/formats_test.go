package examples

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/shepard-labs/go-toon/formats"
	"github.com/shepard-labs/go-toon/toon"
)

func Example_jsonToTOON() {
	jsonInput := strings.NewReader(`{"name":"Ada","languages":["Go","TOON"],"active":true}`)

	var out bytes.Buffer
	if err := formats.JSONToTOON(jsonInput, &out, formats.JSONToTOONOptions{}); err != nil {
		panic(err)
	}

	fmt.Print(out.String())
}

func Example_toonToJSON() {
	toonInput := strings.NewReader("name: Ada\nlanguages: Go,TOON\nactive: true\n")

	var out bytes.Buffer
	if err := formats.TOONToJSON(toonInput, &out, formats.TOONToJSONOptions{
		JSON: formats.JSONOutputOptions{Indent: "  "},
	}); err != nil {
		panic(err)
	}

	fmt.Println(out.String())
}

func Example_fromYAML() {
	yamlInput := strings.NewReader("name: Ada\nactive: true\nscore: 42\n")

	node, err := formats.FromYAML(yamlInput)
	if err != nil {
		panic(err)
	}

	encoded, err := toon.Encode(node)
	if err != nil {
		panic(err)
	}

	fmt.Print(string(encoded))
}

func Example_fromCSV() {
	csvInput := strings.NewReader("id,name\n1,Ada\n2,Grace\n")

	node, err := formats.FromCSV(csvInput, func(o *formats.CSVOptions) {
		o.HeaderMode = formats.CSVHeaderPresent
		o.InferTypes = true
	})
	if err != nil {
		panic(err)
	}

	encoded, err := toon.Encode(node)
	if err != nil {
		panic(err)
	}

	fmt.Print(string(encoded))
}

func Example_fromXML() {
	xmlInput := strings.NewReader(`<user id="1"><name>Ada</name><active>true</active></user>`)

	node, err := formats.FromXML(xmlInput, func(o *formats.XMLOptions) {
		o.AttributePrefix = "@"
		o.TextKey = "#text"
		o.InferTypes = true
	})
	if err != nil {
		panic(err)
	}

	encoded, err := toon.Encode(node)
	if err != nil {
		panic(err)
	}

	fmt.Print(string(encoded))
}
