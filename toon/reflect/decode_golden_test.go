package reflect

import (
	"reflect"
	"testing"

	"github.com/shepard-labs/go-toon/toon"
)

type decodeUser struct {
	ID    int      `toon:"id"`
	Name  string   `toon:"name"`
	Tags  []string `toon:"tags"`
	Score float64  `toon:"score"`
	Raw   []byte   `toon:"raw"`
}

type decodeBoxed struct {
	V any `toon:"v"`
}

func TestDecodeGoldenStruct(t *testing.T) {
	in := []byte("id: 7\nname: Ada\ntags[2]: admin,ops\nscore: 9.5\nraw: SGk=\n")
	var got decodeUser
	if err := Unmarshal(in, &got); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	want := decodeUser{ID: 7, Name: "Ada", Tags: []string{"admin", "ops"}, Score: 9.5, Raw: []byte("Hi")}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("decoded = %+v, want %+v", got, want)
	}
}

func TestDecodeGoldenEmptyArray(t *testing.T) {
	in := []byte("tags: []")
	var got decodeUser
	if err := Unmarshal(in, &got); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if len(got.Tags) != 0 {
		t.Fatalf("tags = %v", got.Tags)
	}
}

func TestDecodeGoldenMissingFieldsAreZero(t *testing.T) {
	in := []byte("id: 5")
	var got decodeUser
	if err := Unmarshal(in, &got); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	want := decodeUser{ID: 5}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("decoded = %+v, want %+v", got, want)
	}
}

func TestDecodeGoldenUnknownFieldsIgnored(t *testing.T) {
	in := []byte("id: 1\nextra: x\nname: Z")
	var got decodeUser
	if err := Unmarshal(in, &got); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if got.ID != 1 || got.Name != "Z" {
		t.Fatalf("decoded = %+v", got)
	}
}

func TestDecodeGoldenIntoMap(t *testing.T) {
	in := []byte("a: 1\nb: 2\nc: 3")
	var got map[string]int
	if err := Unmarshal(in, &got); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	want := map[string]int{"a": 1, "b": 2, "c": 3}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("decoded = %v, want %v", got, want)
	}
}

func TestDecodeGoldenIntoSlice(t *testing.T) {
	in := []byte("[3]: 1,2,3")
	var got []int
	if err := Unmarshal(in, &got); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if !reflect.DeepEqual(got, []int{1, 2, 3}) {
		t.Fatalf("decoded = %v", got)
	}
}

func TestDecodeGoldenIntoArray(t *testing.T) {
	in := []byte("[3]: 1,2,3")
	var got [3]int
	if err := Unmarshal(in, &got); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if got != [3]int{1, 2, 3} {
		t.Fatalf("decoded = %v", got)
	}
}

func TestDecodeGoldenFixedArrayOverflow(t *testing.T) {
	in := []byte("[4]: 1,2,3,4")
	var got [3]int
	if err := Unmarshal(in, &got); toon.CodeOf(err) != toon.ErrUnmarshalType {
		t.Fatalf("overflow: want ErrUnmarshalType, got %v", err)
	}
}

func TestDecodeGoldenFixedArrayUnderflowZeros(t *testing.T) {
	in := []byte("[2]: 7,8")
	var got [3]int
	if err := Unmarshal(in, &got); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if got != [3]int{7, 8, 0} {
		t.Fatalf("decoded = %v", got)
	}
}

func TestDecodeGoldenNumberOverflow(t *testing.T) {
	type small struct {
		ID int32 `toon:"id"`
	}
	in := []byte("id: 9999999999")
	var got small
	if err := Unmarshal(in, &got); toon.CodeOf(err) != toon.ErrUnmarshalType {
		t.Fatalf("overflow: want ErrUnmarshalType, got %v", err)
	}
}

func TestDecodeGoldenNestedStruct(t *testing.T) {
	type Inner struct {
		X int `toon:"x"`
		Y int `toon:"y"`
	}
	type Outer struct {
		Inner Inner  `toon:"inner"`
		Name  string `toon:"name"`
	}
	in := []byte("inner:\n  x: 1\n  y: 2\nname: z")
	var got Outer
	if err := Unmarshal(in, &got); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if got.Inner.X != 1 || got.Inner.Y != 2 || got.Name != "z" {
		t.Fatalf("decoded = %+v", got)
	}
}

func TestDecodeGoldenTypeMismatch(t *testing.T) {
	in := []byte("id: notanumber")
	var got decodeUser
	if err := Unmarshal(in, &got); toon.CodeOf(err) != toon.ErrUnmarshalType {
		t.Fatalf("mismatch: want ErrUnmarshalType, got %v", err)
	}
}

func TestDecodeGoldenAnyFromVariousKinds(t *testing.T) {
	cases := []struct {
		in   string
		want any
	}{
		{"v: 42", float64(42)},
		{"v: 3.14", 3.14},
		{"v: hi", "hi"},
		{"v: true", true},
		{"v: null", nil},
		{"v[2]: 1,2", []any{float64(1), float64(2)}},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			var got decodeBoxed
			if err := Unmarshal([]byte(tc.in), &got); err != nil {
				t.Fatalf("Unmarshal error: %v", err)
			}
			if !reflect.DeepEqual(got.V, tc.want) {
				t.Fatalf("decoded = %#v, want %#v", got.V, tc.want)
			}
		})
	}
}

func TestDecodeGoldenByteSliceFromBase64(t *testing.T) {
	in := []byte(`raw: "SGVsbG8="`)
	var got decodeUser
	if err := Unmarshal(in, &got); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if string(got.Raw) != "Hello" {
		t.Fatalf("byte slice = %q", got.Raw)
	}
}

func TestDecodeGoldenTabularArray(t *testing.T) {
	in := []byte("rows[2]{id,name}:\n  1,Ada\n  2,Bob")
	var got struct {
		Rows []struct {
			ID   int    `toon:"id"`
			Name string `toon:"name"`
		} `toon:"rows"`
	}
	if err := Unmarshal(in, &got); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if len(got.Rows) != 2 {
		t.Fatalf("rows len = %d", len(got.Rows))
	}
	if got.Rows[0].ID != 1 || got.Rows[0].Name != "Ada" {
		t.Fatalf("row 0 = %+v", got.Rows[0])
	}
	if got.Rows[1].ID != 2 || got.Rows[1].Name != "Bob" {
		t.Fatalf("row 1 = %+v", got.Rows[1])
	}
}

func TestDecodeGoldenDecodeErrorPropagates(t *testing.T) {
	in := []byte("a:\n b: 1")
	var got decodeUser
	if err := Unmarshal(in, &got); toon.CodeOf(err) != toon.ErrInvalidIndent {
		t.Fatalf("decode error propagation: want ErrInvalidIndent, got %v", err)
	}
}

func TestDecodeGoldenStrictDuplicates(t *testing.T) {
	in := []byte("id: 1\nid: 2")
	var got decodeUser
	if err := Unmarshal(in, &got); toon.CodeOf(err) != toon.ErrDuplicateKey {
		t.Fatalf("duplicate: want ErrDuplicateKey, got %v", err)
	}
}

func TestDecodeGoldenNonStrictDuplicates(t *testing.T) {
	in := []byte("id: 1\nid: 2")
	var got decodeUser
	if err := Unmarshal(in, &got, func(o *UnmarshalOptions) {
		o.Decode.Strict = false
	}); err != nil {
		t.Fatalf("non-strict dup: %v", err)
	}
	if got.ID != 2 {
		t.Fatalf("non-strict dup id = %d", got.ID)
	}
}

func TestDecodeGoldenUnmarshalNode(t *testing.T) {
	n := &toon.Node{Kind: toon.ObjectKind, Object: []toon.Field{
		{Key: "id", Value: &toon.Node{Kind: toon.NumberKind, Number: toon.Number{Raw: "42"}}},
		{Key: "name", Value: &toon.Node{Kind: toon.StringKind, String: "X"}},
	}}
	var got decodeUser
	if err := UnmarshalNode(n, &got); err != nil {
		t.Fatalf("UnmarshalNode error: %v", err)
	}
	if got.ID != 42 || got.Name != "X" {
		t.Fatalf("decoded = %+v", got)
	}
}

func TestDecodeGoldenResourceLimits(t *testing.T) {
	in := []byte("id: 1\nname: x")
	var dst decodeUser
	err := Unmarshal(in, &dst, func(o *UnmarshalOptions) {
		o.Value.Limits = toon.ResourceLimits{MaxNodes: 1}
	})
	if toon.CodeOf(err) != toon.ErrResourceLimit {
		t.Fatalf("node limit: got %v", err)
	}
}
