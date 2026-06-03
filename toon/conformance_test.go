package toon

import "testing"

func TestConformanceEncoderGoldenMinimum(t *testing.T) {
	cases := []struct {
		name string
		node *Node
		want string
		opts []EncodeOption
	}{
		{"empty object", obj(), "", nil},
		{"empty array", arr(), "[]", nil},
		{"root primitive", str("x"), "x", nil},
		{"nested objects", obj(field("a", obj(field("b", num("1"))))), "a:\n  b: 1", nil},
		{"primitive arrays", obj(field("a", arr(str("x"), str("y")))), "a[2]: x,y", nil},
		{"arrays of arrays", obj(field("a", arr(arr(num("1"), num("2"))))), "a[1]:\n  - [2]: 1,2", nil},
		{"tabular", obj(field("rows", arr(obj(field("id", num("1")), field("name", str("Ada")))))), "rows[1]{id,name}:\n  1,Ada", nil},
		{"mixed arrays", obj(field("a", arr(num("1"), obj(field("b", num("2")))))), "a[2]:\n  - 1\n  - b: 2", nil},
		{"object list items", obj(field("a", arr(obj(field("b", num("1")))))), "a[1]{b}:\n  1", nil},
		{"key quoting", obj(field("sp ace", str("x"))), "\"sp ace\": x", nil},
		{"string escaping", str("a\n\"b"), "\"a\\n\\\"b\"", nil},
		{"comma delimiter", obj(field("a", arr(str("x"), str("y")))), "a[2]: x,y", nil},
		{"tab delimiter", obj(field("a", arr(str("x"), str("y")))), "a[2\t]: x\ty", []EncodeOption{func(o *EncodeOptions) { o.Delimiter = Tab }}},
		{"pipe delimiter", obj(field("a", arr(str("x"), str("y")))), "a[2|]: x|y", []EncodeOption{func(o *EncodeOptions) { o.Delimiter = Pipe }}},
		{"safe key folding", obj(field("a", obj(field("b", num("1"))))), "a.b: 1", []EncodeOption{func(o *EncodeOptions) { o.KeyFolding = KeyFoldingSafe }}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { assertEncode(t, tc.node, tc.want, tc.opts...) })
	}
}

func TestConformanceDecoderStrictNegativeMinimum(t *testing.T) {
	cases := []struct {
		name string
		in   string
		code ErrorCode
		opts []DecodeOption
	}{
		{"invalid indentation", "a:\n b: 1", ErrInvalidIndent, nil},
		{"tab indentation", "a:\n\tb: 1", ErrTabIndent, nil},
		{"missing colon", "a\nb: 1", ErrMissingColon, nil},
		{"invalid escape", "a: \"\\x\"", ErrInvalidEscape, nil},
		{"unterminated string", "a: \"x", ErrUnterminatedString, nil},
		{"malformed header length", "a[01]: 1", ErrMalformedHeader, nil},
		{"content between bracket and colon", "a[1] : 1", ErrMalformedHeader, nil},
		{"header delimiter mismatch", "a[1|]{x,y}:\n  1,2", ErrHeaderDelimiterMismatch, nil},
		{"inline count mismatch", "a[2]: 1", ErrArrayCountMismatch, nil},
		{"list count mismatch", "a[2]:\n  - 1", ErrArrayCountMismatch, nil},
		{"tabular row count mismatch", "a[2]{x}:\n  1", ErrArrayCountMismatch, nil},
		{"tabular width mismatch", "a[1]{x,y}:\n  1", ErrTabularWidthMismatch, nil},
		{"duplicate sibling key", "a: 1\na: 2", ErrDuplicateKey, nil},
		{"path expansion conflict", "a: 1\na.b: 2", ErrPathExpansionConflict, []DecodeOption{func(o *DecodeOptions) { o.ExpandPaths = ExpandPathsSafe }}},
		{"blank line inside array", "a[2]:\n  - 1\n  \n  - 2", ErrInvalidInputFormat, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Decode([]byte(tc.in), tc.opts...)
			if CodeOf(err) != tc.code {
				t.Fatalf("CodeOf = %q, err %v, want %q", CodeOf(err), err, tc.code)
			}
		})
	}
}

func TestConformanceDefaultSecurityPosture(t *testing.T) {
	if !DefaultDecodeOptions().Strict {
		t.Fatalf("strict decode must be default")
	}
	if DefaultDecodeOptions().ExpandPaths != ExpandPathsOff {
		t.Fatalf("path expansion must be off by default")
	}
	if DefaultEncodeOptions().KeyFolding != KeyFoldingOff {
		t.Fatalf("key folding must be off by default")
	}
	if DefaultEncodeOptions().NumberMode != NumberLossless {
		t.Fatalf("lossless number mode must be default")
	}
}
