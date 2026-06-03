package formats

import (
	"bytes"
	"strings"
	"testing"

	"github.com/shepard-labs/go-toon/toon"
)

func BenchmarkLargeJSONInputNormalization(b *testing.B) {
	var src strings.Builder
	src.WriteString(`{"rows":[`)
	for i := range 1000 {
		if i > 0 {
			src.WriteByte(',')
		}
		src.WriteString(`{"id":` + benchItoa(i) + `,"name":"user_` + benchItoa(i) + `","active":true}`)
	}
	src.WriteString(`]}`)
	data := []byte(src.String())
	b.ReportAllocs()
	for b.Loop() {
		if _, err := FromJSON(bytes.NewReader(data)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLargeCSVInputNormalization(b *testing.B) {
	var src strings.Builder
	src.WriteString("id,name,active\n")
	for i := range 1000 {
		src.WriteString(benchItoa(i) + ",user_" + benchItoa(i) + ",true\n")
	}
	data := []byte(src.String())
	b.ReportAllocs()
	for b.Loop() {
		if _, err := FromCSV(bytes.NewReader(data), func(o *CSVOptions) {
			o.HeaderMode = CSVHeaderPresent
			o.InferTypes = true
		}); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkJSONOutputOrderedNode(b *testing.B) {
	n := &toon.Node{Kind: toon.ObjectKind, Object: []toon.Field{{Key: "rows", Value: &toon.Node{Kind: toon.ArrayKind}}}}
	for i := range 1000 {
		n.Object[0].Value.Array = append(n.Object[0].Value.Array, &toon.Node{Kind: toon.ObjectKind, Object: []toon.Field{
			{Key: "id", Value: &toon.Node{Kind: toon.NumberKind, Number: toon.Number{Raw: benchItoa(i)}}},
			{Key: "name", Value: &toon.Node{Kind: toon.StringKind, String: "user_" + benchItoa(i)}},
		}})
	}
	b.ReportAllocs()
	for b.Loop() {
		var out bytes.Buffer
		if err := ToJSON(&out, n); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkWideJSONObjectNormalization(b *testing.B) {
	var src strings.Builder
	src.WriteByte('{')
	for i := range 1000 {
		if i > 0 {
			src.WriteByte(',')
		}
		src.WriteString(`"field_` + benchItoa(i) + `":` + benchItoa(i))
	}
	src.WriteByte('}')
	data := []byte(src.String())
	b.ReportAllocs()
	for b.Loop() {
		if _, err := FromJSON(bytes.NewReader(data)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkWideCSVHeaderNormalization(b *testing.B) {
	var src strings.Builder
	for i := range 1000 {
		if i > 0 {
			src.WriteByte(',')
		}
		src.WriteString("field_" + benchItoa(i))
	}
	src.WriteByte('\n')
	for i := range 1000 {
		if i > 0 {
			src.WriteByte(',')
		}
		src.WriteString(benchItoa(i))
	}
	src.WriteByte('\n')
	data := []byte(src.String())
	b.ReportAllocs()
	for b.Loop() {
		if _, err := FromCSV(bytes.NewReader(data), func(o *CSVOptions) {
			o.HeaderMode = CSVHeaderPresent
			o.InferTypes = true
		}); err != nil {
			b.Fatal(err)
		}
	}
}

func benchItoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
