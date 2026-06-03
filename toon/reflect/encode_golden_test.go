package reflect

import (
	"bytes"
	"strings"
	"testing"

	"github.com/shepard-labs/go-toon/toon"
)

type goldenUser struct {
	ID    int      `toon:"id"`
	Name  string   `toon:"name"`
	Tags  []string `toon:"tags"`
	Score float64  `toon:"score"`
	Raw   []byte   `toon:"raw"`
	Meta  struct {
		Active bool   `toon:"active"`
		City   string `toon:"city"`
	} `toon:"meta"`
}

type goldenRow struct {
	SKU string `toon:"sku"`
	Qty int    `toon:"qty"`
}

func TestEncodeGoldenRootValues(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want string
	}{
		{"nil", nil, "null"},
		{"true", true, "true"},
		{"false", false, "false"},
		{"empty string", "", `""`},
		{"plain string", "hello", "hello"},
		{"int", 42, "42"},
		{"negative", -7, "-7"},
		{"float", 3.14, "3.14"},
		{"empty slice", []int{}, "[]"},
		{"empty map", map[string]int{}, ""},
		{"empty struct", struct{}{}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertMarshal(t, tc.in, tc.want)
		})
	}
}

func TestEncodeGoldenStruct(t *testing.T) {
	v := goldenUser{
		ID:    7,
		Name:  "Ada",
		Tags:  []string{"admin", "ops"},
		Score: 9.5,
		Raw:   []byte("Hi"),
	}
	v.Meta.Active = true
	v.Meta.City = "London"
	want := "id: 7\n" +
		"name: Ada\n" +
		"tags[2]: admin,ops\n" +
		"score: 9.5\n" +
		`raw: SGk=` + "\n" +
		"meta:\n" +
		"  active: true\n" +
		"  city: London"
	assertMarshal(t, v, want)
}

func TestEncodeGoldenMapOrder(t *testing.T) {
	in := map[string]int{"c": 3, "a": 1, "b": 2}
	want := "a: 1\nb: 2\nc: 3"
	assertMarshal(t, in, want)
}

func TestEncodeGoldenNestedArray(t *testing.T) {
	in := map[string]any{
		"items": []any{
			map[string]any{"sku": "A1", "qty": 2},
			map[string]any{"sku": "B2", "qty": 1},
		},
	}
	want := "items[2]{qty,sku}:\n  2,A1\n  1,B2"
	assertMarshal(t, in, want)
}

func TestEncodeGoldenEmpty(t *testing.T) {
	type Empty struct{}
	assertMarshal(t, Empty{}, "")
}

func TestEncodeGoldenByteSliceEdgeCases(t *testing.T) {
	assertMarshal(t, []byte{}, `""`)
	assertMarshal(t, []byte{0}, "AA==")
	assertMarshal(t, []byte{0xff, 0xfe}, "//4=")
}

func TestEncodeGoldenConformanceWithNodeFromValue(t *testing.T) {
	v := goldenRow{SKU: "X1", Qty: 5}
	fromValue, err := marshalBytes(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	n, err := NodeFromValue(v)
	if err != nil {
		t.Fatalf("NodeFromValue: %v", err)
	}
	fromNode, err := toon.Encode(n)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if !bytes.Equal(fromValue, fromNode) {
		t.Fatalf("conformance broken: %q vs %q", fromValue, fromNode)
	}
}

func TestEncodeGoldenConformanceWithRoundTrip(t *testing.T) {
	in := goldenUser{ID: 1, Name: "A", Tags: []string{"x"}}
	first, err := marshalBytes(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	n, err := toon.Decode(first)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	second, err := toon.Encode(n)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("round trip broken:\nfirst=%q\nsecond=%q", first, second)
	}
}

func TestEncodeGoldenPointerToStruct(t *testing.T) {
	v := &goldenUser{ID: 9, Name: "Z"}
	assertMarshal(t, v, "id: 9\nname: Z\ntags: []\nscore: 0\nraw: \"\"\nmeta:\n  active: false\n  city: \"\"")
}

func TestEncodeGoldenPointerNil(t *testing.T) {
	var p *goldenUser
	assertMarshal(t, p, "null")
}

func TestEncodeGoldenAnyValue(t *testing.T) {
	type Box struct {
		V any `toon:"v"`
	}
	assertMarshal(t, Box{V: 42}, "v: 42")
	assertMarshal(t, Box{V: "hi"}, "v: hi")
	assertMarshal(t, Box{V: nil}, "v: null")
	assertMarshal(t, Box{V: []int{1, 2}}, "v[2]: 1,2")
}

func TestEncodeGoldenEmbeddedStruct(t *testing.T) {
	type Inner struct {
		X int `toon:"x"`
		Y int `toon:"y"`
	}
	type Outer struct {
		Inner
		Name string `toon:"name"`
	}
	v := Outer{Name: "z"}
	v.X = 1
	v.Y = 2
	assertMarshal(t, v, "x: 1\ny: 2\nname: z")
}

func TestEncodeGoldenNumberModes(t *testing.T) {
	t.Run("lossless", func(t *testing.T) {
		assertMarshal(t, 0.1, "0.1")
		assertMarshal(t, 1e-7, "1e-7")
	})
	t.Run("float64", func(t *testing.T) {
		opts := func(o *Options) {
			o.Value.NumberMode = toon.NumberFloat64
			o.Encode.NumberMode = toon.NumberFloat64
		}
		assertMarshal(t, 0.1, "0.1", opts)
	})
}

func TestEncodeGoldenTabularDetection(t *testing.T) {
	in := map[string]any{
		"rows": []any{
			map[string]any{"id": 1, "name": "Ada"},
			map[string]any{"id": 2, "name": "Bob"},
		},
	}
	assertMarshal(t, in, "rows[2]{id,name}:\n  1,Ada\n  2,Bob")
}

func TestEncodeGoldenArrayOfArrays(t *testing.T) {
	in := map[string]any{
		"pairs": []any{
			[]any{1, 2},
			[]any{3, 4},
		},
	}
	assertMarshal(t, in, "pairs[2]:\n  - [2]: 1,2\n  - [2]: 3,4")
}

func TestEncodeGoldenStableOutput(t *testing.T) {
	in := map[string]int{"z": 1, "y": 2, "x": 3, "w": 4}
	first, err := marshalBytes(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for i := 0; i < 5; i++ {
		next, err := marshalBytes(in)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if !bytes.Equal(first, next) {
			t.Fatalf("trial %d: %q != %q", i, first, next)
		}
	}
}

func TestEncodeGoldenSortedKeysWithNested(t *testing.T) {
	in := map[string]any{
		"z": map[string]int{"b": 1, "a": 2},
		"a": 1,
	}
	want := "a: 1\nz:\n  a: 2\n  b: 1"
	assertMarshal(t, in, want)
}

// helpers

func assertMarshal(t *testing.T, in any, want string, opts ...Option) {
	t.Helper()
	got, err := marshalString(in, opts...)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if got != want {
		t.Fatalf("marshal:\ngot:  %q\nwant: %q", got, want)
	}
	if strings.Contains(got, "\r") {
		t.Fatalf("output contains CR: %q", got)
	}
	if strings.HasSuffix(got, "\n") {
		t.Fatalf("output has trailing newline: %q", got)
	}
}

func marshalString(v any, opts ...Option) (string, error) {
	b, err := marshalBytes(v, opts...)
	return string(b), err
}

func marshalBytes(v any, opts ...Option) ([]byte, error) {
	var buf bytes.Buffer
	if err := Marshal(&buf, v, opts...); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
