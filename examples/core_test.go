package examples

import (
	"fmt"

	"github.com/shepard-labs/go-toon/toon"
)

func Example_encodeDecodeNode() {
	data := &toon.Node{
		Kind: toon.ObjectKind,
		Object: []toon.Field{
			{Key: "name", Value: &toon.Node{Kind: toon.StringKind, String: "go-toon"}},
			{Key: "stable", Value: &toon.Node{Kind: toon.BoolKind, Bool: true}},
			{Key: "version", Value: &toon.Node{Kind: toon.NumberKind, Number: toon.Number{Raw: "3.3"}}},
			{Key: "tags", Value: &toon.Node{Kind: toon.ArrayKind, Array: []*toon.Node{
				{Kind: toon.StringKind, String: "encoding"},
				{Kind: toon.StringKind, String: "structured-data"},
			}}},
		},
	}

	encoded, err := toon.Encode(data)
	if err != nil {
		panic(err)
	}

	decoded, err := toon.Decode(encoded)
	if err != nil {
		panic(err)
	}

	fmt.Print(string(encoded))
	fmt.Printf("decoded root kind: %d\n", decoded.Kind)
}

func Example_validate() {
	input := []byte("name: go-toon\ntags: encoding,structured-data\n")

	if err := toon.Validate(input); err != nil {
		panic(err)
	}

	fmt.Println("valid TOON")
}
