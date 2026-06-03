package examples

import (
	"fmt"

	"github.com/shepard-labs/go-toon/toon"
)

func Example_encodeOptions() {
	rows := &toon.Node{Kind: toon.ArrayKind, Array: []*toon.Node{
		{Kind: toon.ObjectKind, Object: []toon.Field{
			{Key: "id", Value: &toon.Node{Kind: toon.NumberKind, Number: toon.Number{Raw: "1"}}},
			{Key: "name", Value: &toon.Node{Kind: toon.StringKind, String: "Ada"}},
		}},
		{Kind: toon.ObjectKind, Object: []toon.Field{
			{Key: "id", Value: &toon.Node{Kind: toon.NumberKind, Number: toon.Number{Raw: "2"}}},
			{Key: "name", Value: &toon.Node{Kind: toon.StringKind, String: "Grace"}},
		}},
	}}

	encoded, err := toon.Encode(rows, func(o *toon.EncodeOptions) {
		o.Delimiter = toon.Pipe
		o.IndentSize = 4
	})
	if err != nil {
		panic(err)
	}

	fmt.Print(string(encoded))
}

func Example_decodeOptions() {
	input := []byte("user.name: Ada\nuser.language: Go\n")

	decoded, err := toon.Decode(input, func(o *toon.DecodeOptions) {
		o.ExpandPaths = toon.ExpandPathsSafe
	})
	if err != nil {
		panic(err)
	}

	encoded, err := toon.Encode(decoded)
	if err != nil {
		panic(err)
	}

	fmt.Print(string(encoded))
}

func Example_resourceLimits() {
	input := []byte("name: go-toon\n")

	err := toon.Validate(input, func(o *toon.DecodeOptions) {
		o.Limits = toon.ResourceLimits{MaxBytes: 8}
	})

	fmt.Println(err != nil)
}
