package reflect

import (
	"bytes"
	"fmt"
	"math/rand"
	"testing"

	"github.com/shepard-labs/go-toon/toon"
)

type benchUser struct {
	ID    int64     `toon:"id"`
	Name  string    `toon:"name"`
	Email string    `toon:"email"`
	Tags  []string  `toon:"tags"`
	Score float64   `toon:"score"`
	Raw   []byte    `toon:"raw"`
	Meta  benchMeta `toon:"meta"`
}

type benchMeta struct {
	Active  bool   `toon:"active"`
	City    string `toon:"city"`
	Country string `toon:"country"`
}

func makeBenchUser() benchUser {
	tags := make([]string, 8)
	for i := range tags {
		tags[i] = fmt.Sprintf("tag-%d", i)
	}
	return benchUser{
		ID:    42,
		Name:  "Ada Lovelace",
		Email: "ada@example.com",
		Tags:  tags,
		Score: 98.6,
		Raw:   []byte("binary payload bytes here"),
		Meta:  benchMeta{Active: true, City: "London", Country: "UK"},
	}
}

func makeBenchMap() map[string]int {
	m := make(map[string]int, 1000)
	for i := 0; i < 1000; i++ {
		m[fmt.Sprintf("key-%04d", i)] = i
	}
	return m
}

func makeBenchArray() [256]int {
	var a [256]int
	for i := range a {
		a[i] = i
	}
	return a
}

func BenchmarkMarshalSmallStruct(b *testing.B) {
	v := makeBenchUser()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		if err := Marshal(&buf, v); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkUnmarshalSmallStruct(b *testing.B) {
	v := makeBenchUser()
	var buf bytes.Buffer
	if err := Marshal(&buf, v); err != nil {
		b.Fatal(err)
	}
	data := buf.Bytes()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var dst benchUser
		if err := Unmarshal(data, &dst); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMarshalLargeMap(b *testing.B) {
	v := makeBenchMap()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		if err := Marshal(&buf, v); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkUnmarshalLargeArray(b *testing.B) {
	v := makeBenchArray()
	var buf bytes.Buffer
	if err := Marshal(&buf, v); err != nil {
		b.Fatal(err)
	}
	data := buf.Bytes()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var dst [256]int
		if err := Unmarshal(data, &dst); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkNodeFromValue(b *testing.B) {
	v := makeBenchUser()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := NodeFromValue(v)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEncodeOnly(b *testing.B) {
	inputs := make([][]byte, 1000)
	for i := range inputs {
		inputs[i] = []byte(fmt.Sprintf("payload-%d-%d", rand.Int63(), i))
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		if err := Marshal(&buf, inputs); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkNodeToValue(b *testing.B) {
	v := makeBenchUser()
	n, err := NodeFromValue(v)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var dst benchUser
		if err := NodeToValue(n, &dst); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCoreEncodeBaseline(b *testing.B) {
	n, err := toon.Decode([]byte("id: 42\nname: Ada\nemail: ada@example.com\ntags[8]: tag-0,tag-1,tag-2,tag-3,tag-4,tag-5,tag-6,tag-7\nscore: 98.6\nraw: YmluYXJ5IHBheWxvYWQgYnl0ZXMgaGVyZQ==\nmeta:\n  active: true\n  city: London\n  country: UK\n"))
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := toon.Encode(n); err != nil {
			b.Fatal(err)
		}
	}
}
