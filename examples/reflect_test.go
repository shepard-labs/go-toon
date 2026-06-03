package examples

import (
	"bytes"
	"fmt"

	"github.com/shepard-labs/go-toon/toon"
	refl "github.com/shepard-labs/go-toon/toon/reflect"
)

type user struct {
	ID    int      `toon:"id"`
	Name  string   `toon:"name"`
	Email string   `toon:"email"`
	Tags  []string `toon:"tags"`
	Raw   []byte   `toon:"raw"`
}

func Example_marshalUnmarshal() {
	in := user{
		ID:    42,
		Name:  "Ada",
		Email: "ada@example.com",
		Tags:  []string{"math", "computing"},
		Raw:   []byte("binary"),
	}

	var buf bytes.Buffer
	if err := refl.Marshal(&buf, in); err != nil {
		panic(err)
	}

	var out user
	if err := refl.Unmarshal(buf.Bytes(), &out); err != nil {
		panic(err)
	}

	fmt.Print(buf.String())
	fmt.Println()
	fmt.Printf("decoded: id=%d name=%q\n", out.ID, out.Name)
}

type taggedUser struct {
	ID       int    `toon:"id"`
	Name     string `toon:"name"`
	Password string `toon:"-"`
	Handle   string `toon:"handle"`
}

func Example_structTags() {
	in := taggedUser{ID: 1, Name: "Ada", Password: "secret", Handle: "@ada"}

	var buf bytes.Buffer
	if err := refl.Marshal(&buf, in); err != nil {
		panic(err)
	}

	fmt.Print(buf.String())
}

func Example_byteSlice() {
	in := struct {
		Name   string `toon:"name"`
		Avatar []byte `toon:"avatar"`
	}{
		Name:   "Ada",
		Avatar: []byte{0x89, 0x50, 0x4E, 0x47},
	}

	var buf bytes.Buffer
	if err := refl.Marshal(&buf, in); err != nil {
		panic(err)
	}

	var out struct {
		Name   string `toon:"name"`
		Avatar []byte `toon:"avatar"`
	}
	if err := refl.Unmarshal(buf.Bytes(), &out); err != nil {
		panic(err)
	}

	fmt.Print(buf.String())
	fmt.Println()
	fmt.Printf("decoded avatar length: %d\n", len(out.Avatar))
}

type order struct {
	ID     int               `toon:"id"`
	Items  []string          `toon:"items"`
	Counts map[string]int    `toon:"counts"`
	Notes  map[string]string `toon:"notes"`
}

func Example_maps() {
	in := order{
		ID:     1,
		Items:  []string{"a", "b", "c"},
		Counts: map[string]int{"a": 1, "b": 2, "c": 3},
		Notes:  map[string]string{"a": "first", "b": "second"},
	}

	var buf bytes.Buffer
	if err := refl.Marshal(&buf, in); err != nil {
		panic(err)
	}

	fmt.Print(buf.String())
}

func Example_namedTypes() {
	type userID int64
	type score float64
	type active bool
	type raw []byte

	in := struct {
		ID     userID `toon:"id"`
		Score  score  `toon:"score"`
		Active active `toon:"active"`
		Raw    raw    `toon:"raw"`
	}{
		ID:     42,
		Score:  9.5,
		Active: true,
		Raw:    raw("hello"),
	}

	var buf bytes.Buffer
	if err := refl.Marshal(&buf, in); err != nil {
		panic(err)
	}

	fmt.Print(buf.String())
}

type linkedNode struct {
	Value int         `toon:"value"`
	Next  *linkedNode `toon:"next"`
	Tail  *linkedNode `toon:"tail"`
}

func Example_cycleDetection() {
	tail := &linkedNode{Value: 3}
	mid := &linkedNode{Value: 2, Next: tail}
	head := &linkedNode{Value: 1, Next: mid, Tail: tail}

	var buf bytes.Buffer
	err := refl.Marshal(&buf, head)
	fmt.Println("cycle error:", err != nil)
	fmt.Println("code:", toon.CodeOf(err))
}

func Example_errorHandling() {
	var buf bytes.Buffer
	err := refl.Marshal(&buf, func() {})
	fmt.Println("func error:", err != nil)
	fmt.Println("code:", toon.CodeOf(err))

	decoded, derr := toon.Decode([]byte("id: not-a-number"))
	if derr != nil {
		fmt.Println("decode error:", toon.CodeOf(derr))
	}
	var dst struct {
		ID int64 `toon:"id"`
	}
	uerr := refl.UnmarshalNode(decoded, &dst)
	fmt.Println("type-mismatch code:", toon.CodeOf(uerr))
	fmt.Println("type-mismatch via CodeOf:", toon.CodeOf(uerr) == toon.ErrUnmarshalType)
}

func Example_reflectResourceLimits() {
	type big struct {
		A int `toon:"a"`
		B int `toon:"b"`
		C int `toon:"c"`
	}
	in := big{A: 1, B: 2, C: 3}

	var buf bytes.Buffer
	err := refl.Marshal(&buf, in, func(o *refl.Options) {
		o.Value.Limits = toon.ResourceLimits{MaxNodes: 2}
	})
	fmt.Println("limit error:", err != nil)
	fmt.Println("code:", toon.CodeOf(err))
}

type base struct {
	ID int `toon:"id"`
}

type employee struct {
	base
	Name string `toon:"name"`
}

func Example_embeddedStruct() {
	in := employee{base: base{ID: 1}, Name: "Ada"}

	var buf bytes.Buffer
	if err := refl.Marshal(&buf, in); err != nil {
		panic(err)
	}

	fmt.Print(buf.String())
}

type nullableUser struct {
	ID    int    `toon:"id"`
	Name  string `toon:"name"`
	Email string `toon:"email"`
}

func Example_unmarshalMissingFields() {
	input := []byte("id: 7\nname: Ada\n")

	var out nullableUser
	if err := refl.Unmarshal(input, &out); err != nil {
		panic(err)
	}

	fmt.Printf("id=%d name=%q email=%q\n", out.ID, out.Name, out.Email)
}

func Example_nodeRoundTrip() {
	in := user{ID: 1, Name: "Ada", Tags: []string{"x"}}

	node, err := refl.NodeFromValue(in)
	if err != nil {
		panic(err)
	}

	var out user
	if err := refl.NodeToValue(node, &out); err != nil {
		panic(err)
	}

	fmt.Printf("kind=%d fields=%d name=%q\n", node.Kind, len(node.Object), out.Name)
}
