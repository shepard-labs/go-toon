package toon

import (
	"bytes"
	"strings"
	"testing"
)

func TestEncodeGoldenRootValues(t *testing.T) {
	assertEncode(t, obj(), "")
	assertEncode(t, arr(), "[]")
	assertEncode(t, str("hello"), "hello")
	assertEncode(t, boolNode(true), "true")
	assertEncode(t, num("-0"), "0")
}

func TestEncodeGoldenNestedObject(t *testing.T) {
	n := obj(
		field("id", num("1")),
		field("user", obj(
			field("name", str("Ada")),
			field("active", boolNode(true)),
		)),
	)
	assertEncode(t, n, "id: 1\nuser:\n  name: Ada\n  active: true")
}

func TestEncodeGoldenPrimitiveArray(t *testing.T) {
	n := obj(field("tags", arr(str("admin"), str("ops"), str("dev"))))
	assertEncode(t, n, "tags[3]: admin,ops,dev")
}

func TestEncodeGoldenArrayOfPrimitiveArrays(t *testing.T) {
	n := obj(field("pairs", arr(
		arr(num("1"), num("2")),
		arr(num("3"), num("4")),
	)))
	assertEncode(t, n, "pairs[2]:\n  - [2]: 1,2\n  - [2]: 3,4")
}

func TestEncodeGoldenTabularArray(t *testing.T) {
	n := obj(field("items", arr(
		obj(field("sku", str("A1")), field("qty", num("2")), field("price", num("9.9900"))),
		obj(field("sku", str("B2")), field("qty", num("1")), field("price", num("14.5"))),
	)))
	assertEncode(t, n, "items[2]{sku,qty,price}:\n  A1,2,9.99\n  B2,1,14.5")
}

func TestEncodeGoldenMixedArraysAndObjectListItems(t *testing.T) {
	n := obj(field("items", arr(
		num("1"),
		obj(field("id", num("2")), field("name", str("Bob"))),
		arr(str("a"), str("b")),
		obj(),
	)))
	assertEncode(t, n, "items[4]:\n  - 1\n  - id: 2\n    name: Bob\n  - [2]: a,b\n  -")
}

func TestEncodeGoldenTabularFirstListItem(t *testing.T) {
	n := obj(field("groups", arr(
		obj(
			field("users", arr(
				obj(field("id", num("1")), field("name", str("Ada"))),
				obj(field("id", num("2")), field("name", str("Bob"))),
			)),
			field("status", str("active")),
		),
	)))
	assertEncode(t, n, "groups[1]:\n  - users[2]{id,name}:\n    1,Ada\n    2,Bob\n    status: active")
}

func TestEncodeGoldenKeyQuotingAndStringEscaping(t *testing.T) {
	n := obj(
		field("sp ace", str("hello: \"toon\"\\\n")),
		field("valid_key", str("plain")),
	)
	assertEncode(t, n, "\"sp ace\": \"hello: \\\"toon\\\"\\\\\\n\"\nvalid_key: plain")
}

func TestEncodeGoldenDelimitersAndHeaders(t *testing.T) {
	n := obj(field("tags", arr(str("a,b"), str("c|d"))))
	assertEncode(t, n, "tags[2]: \"a,b\",c|d")
	assertEncode(t, n, "tags[2|]: a,b|\"c|d\"", func(o *EncodeOptions) { o.Delimiter = Pipe })
	assertEncode(t, n, "tags[2\t]: a,b\tc|d", func(o *EncodeOptions) { o.Delimiter = Tab })
}

func TestEncodeGoldenSafeKeyFolding(t *testing.T) {
	n := obj(field("a", obj(field("b", obj(field("c", num("1")))))))
	assertEncode(t, n, "a.b.c: 1", func(o *EncodeOptions) { o.KeyFolding = KeyFoldingSafe })
	assertEncode(t, n, "a.b:\n  c: 1", func(o *EncodeOptions) {
		o.KeyFolding = KeyFoldingSafe
		o.FlattenDepth = 2
	})
}

func TestEncodeGoldenFoldingCollisionPrevention(t *testing.T) {
	n := obj(
		field("a", obj(field("b", num("1")))),
		field("a.b", num("2")),
	)
	assertEncode(t, n, "a:\n  b: 1\na.b: 2", func(o *EncodeOptions) { o.KeyFolding = KeyFoldingSafe })
}

func TestEncodeGoldenNumberModes(t *testing.T) {
	n := obj(
		field("zero", num("-0")),
		field("plain", num("1.2300e+3")),
		field("small", num("0.0000001")),
		field("large", num("1000000000000000000000")),
	)
	assertEncode(t, n, "zero: 0\nplain: 1230\nsmall: 1e-7\nlarge: 1e+21")
	assertEncode(t, obj(field("n", num("9007199254740993"))), "n: 9007199254740992", func(o *EncodeOptions) { o.NumberMode = NumberFloat64 })
	assertEncode(t, obj(field("n", num("1000000000000000000000"))), "n: \"1e+21\"", func(o *EncodeOptions) { o.NumberMode = NumberStringForUnsafe })
	if _, err := Encode(num("01")); CodeOf(err) != ErrInvalidInputFormat {
		t.Fatalf("invalid number CodeOf = %q, err %v", CodeOf(err), err)
	}
}

func TestEncodeToWriter(t *testing.T) {
	var b bytes.Buffer
	if err := EncodeToWriter(&b, obj(field("x", str("y")))); err != nil {
		t.Fatalf("EncodeToWriter error: %v", err)
	}
	if b.String() != "x: y" {
		t.Fatalf("EncodeToWriter = %q", b.String())
	}
}

func TestEncodeResourceLimits(t *testing.T) {
	for name, tc := range map[string]struct {
		n      *Node
		limits ResourceLimits
	}{
		"nodes":  {obj(field("a", str("b"))), ResourceLimits{MaxNodes: 1}},
		"depth":  {obj(field("a", obj(field("b", str("c"))))), ResourceLimits{MaxDepth: 1}},
		"string": {str("abcd"), ResourceLimits{MaxStringBytes: 3}},
		"array":  {arr(str("a"), str("b")), ResourceLimits{MaxArrayLength: 1}},
	} {
		_, err := Encode(tc.n, func(o *EncodeOptions) { o.Limits = tc.limits })
		if CodeOf(err) != ErrResourceLimit {
			t.Fatalf("%s CodeOf = %q, err %v", name, CodeOf(err), err)
		}
	}
}

func assertEncode(t *testing.T, n *Node, want string, opts ...EncodeOption) {
	t.Helper()
	got, err := Encode(n, opts...)
	if err != nil {
		t.Fatalf("Encode error: %v", err)
	}
	if string(got) != want {
		t.Fatalf("Encode = %q, want %q", string(got), want)
	}
	if strings.Contains(string(got), "\r") {
		t.Fatalf("Encode contains CR: %q", got)
	}
	if strings.HasSuffix(string(got), "\n") {
		t.Fatalf("Encode has trailing newline: %q", got)
	}
	for line := range strings.SplitSeq(string(got), "\n") {
		if strings.HasSuffix(line, " ") || strings.HasSuffix(line, "\t") {
			t.Fatalf("Encode has trailing whitespace in %q", line)
		}
	}
	got2, err := Encode(n, opts...)
	if err != nil {
		t.Fatalf("second Encode error: %v", err)
	}
	if !bytes.Equal(got, got2) {
		t.Fatalf("Encode unstable: %q then %q", got, got2)
	}
}

func obj(fields ...Field) *Node { return &Node{Kind: ObjectKind, Object: fields} }

func arr(items ...*Node) *Node { return &Node{Kind: ArrayKind, Array: items} }

func field(key string, value *Node) Field { return Field{Key: key, Value: value} }

func str(s string) *Node { return &Node{Kind: StringKind, String: s} }

func num(s string) *Node { return &Node{Kind: NumberKind, Number: Number{Raw: s}} }

func boolNode(v bool) *Node { return &Node{Kind: BoolKind, Bool: v} }
