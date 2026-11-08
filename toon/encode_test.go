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
	assertEncode(t, n, "groups[1]:\n  - users[2]{id,name}:\n      1,Ada\n      2,Bob\n    status: active")
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

func TestEncodeDeprecatedOptionsAreIgnored(t *testing.T) {
	ignore := func(o *EncodeOptions) {
		o.IncludeLengthMarkers = true
		o.KeyFolding = KeyFoldingSafe
		o.FlattenDepth = 2
	}
	assertEncode(t, obj(field("tags", arr(str("a"), str("b")))), "tags[2]: a,b", ignore)
	assertEncode(t, obj(field("a", obj(field("b", obj(field("c", num("1"))))))), "a:\n  b:\n    c: 1", ignore)
	assertEncode(t, obj(field("items", arr(
		obj(field("sku", str("A1")), field("qty", num("2"))),
		obj(field("sku", str("B2")), field("qty", num("1"))),
	))), "items[2]{sku,qty}:\n  A1,2\n  B2,1", ignore)
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

func TestEncodeNumberCanonicalBoundaries(t *testing.T) {
	for raw, want := range map[string]string{
		"0":                       "0",
		"-0.0":                    "0",
		"0e5":                     "0",
		"1E+3":                    "1000",
		"1.5000":                  "1.5",
		"1.0":                     "1",
		"05":                      "", // invalid JSON number
		"0.000001":                "0.000001",
		"1e-6":                    "0.000001",
		"1.5e-6":                  "0.0000015",
		"0.0000001":               "1e-7",
		"-1.5e-7":                 "-1.5e-7",
		"999999999999999999999":   "999999999999999999999",
		"1e20":                    "100000000000000000000",
		"1e21":                    "1e+21",
		"1.5E21":                  "1.5e+21",
		"-1000000000000000000000": "-1e+21",
	} {
		got, err := Encode(num(raw))
		if want == "" {
			if CodeOf(err) != ErrInvalidInputFormat {
				t.Fatalf("Encode(%q) error = %v, want invalid_input_format", raw, err)
			}
			continue
		}
		if err != nil || string(got) != want {
			t.Fatalf("Encode(%q) = %q, %v; want %q", raw, got, err, want)
		}
	}
}

func TestEncodeKeyedTabular(t *testing.T) {
	entry := func(host string, port string) *Node {
		return obj(field("host", str(host)), field("port", num(port)))
	}
	servers := obj(field("alpha", entry("a", "1")), field("beta", entry("b", "2")))

	assertEncode(t, obj(field("servers", servers)), "servers[2:]{host,port}:\n  alpha: a,1\n  beta: b,2")
	assertEncode(t, servers, "[2:]{host,port}:\n  alpha: a,1\n  beta: b,2")
	assertEncode(t, obj(field("servers", servers)), "servers[2:|]{host|port}:\n  alpha: a|1\n  beta: b|2",
		func(o *EncodeOptions) { o.Delimiter = Pipe })
	assertEncode(t, obj(field("servers", servers)), "servers[2:\t]{host\tport}:\n  alpha: a\t1\n  beta: b\t2",
		func(o *EncodeOptions) { o.Delimiter = Tab })

	// A keyed object nested under a non-keyed object stays in object-field position.
	assertEncode(t, obj(field("env", obj(field("servers", servers), field("name", str("x"))))),
		"env:\n  servers[2:]{host,port}:\n    alpha: a,1\n    beta: b,2\n  name: x")

	// Entry values may list their keys in a different order than the first entry.
	reordered := obj(field("a", obj(field("x", num("1")), field("y", num("2")))), field("b", obj(field("y", num("4")), field("x", num("3")))))
	assertEncode(t, reordered, "[2:]{x,y}:\n  a: 1,2\n  b: 3,4")

	ineligible := map[string]*Node{
		"single entry":     obj(field("a", entry("a", "1"))),
		"primitive entry":  obj(field("a", entry("a", "1")), field("b", num("2"))),
		"different keys":   obj(field("a", obj(field("x", num("1")))), field("b", obj(field("y", num("1"))))),
		"extra key":        obj(field("a", obj(field("x", num("1")))), field("b", obj(field("x", num("1")), field("y", num("1"))))),
		"empty entry":      obj(field("a", obj()), field("b", obj())),
		"array column":     obj(field("a", obj(field("t", arr(str("x"))))), field("b", obj(field("t", arr(str("y")))))),
		"null vs object":   obj(field("a", obj(field("t", obj(field("x", num("1")))))), field("b", obj(field("t", &Node{Kind: NullKind})))),
		"empty obj column": obj(field("a", obj(field("t", obj()))), field("b", obj(field("t", obj())))),
	}
	for name, n := range ineligible {
		got, err := Encode(n)
		if err != nil || strings.Contains(string(got), ":]{") || strings.Contains(string(got), "[2:") {
			t.Fatalf("%s: keyed form used: %q, %v", name, got, err)
		}
	}
}

func TestEncodeKeyedTabularQuotesEntryKeysAndCells(t *testing.T) {
	n := obj(
		field("my key", obj(field("v", str("a,b")))),
		field("a:b", obj(field("v", str("#c")))),
	)
	assertEncode(t, n, "[2:]{v}:\n  \"my key\": \"a,b\"\n  \"a:b\": \"#c\"")
}

func TestEncodeNestedFieldGroups(t *testing.T) {
	row := func(id string, lat, lon string) *Node {
		return obj(field("id", num(id)), field("geo", obj(field("lat", num(lat)), field("lon", num(lon)))))
	}
	assertEncode(t, obj(field("rows", arr(row("1", "5", "6"), row("2", "7", "8")))), "rows[2]{id,geo{lat,lon}}:\n  1,5,6\n  2,7,8")

	// Sub-object key order may vary; the first element decides the header.
	swapped := obj(field("geo", obj(field("lon", num("8")), field("lat", num("7")))), field("id", num("2")))
	assertEncode(t, arr(row("1", "5", "6"), swapped), "[2]{id,geo{lat,lon}}:\n  1,5,6\n  2,7,8")

	assertEncode(t, obj(field("rows", arr(row("1", "5", "6"), row("2", "7", "8")))), "rows[2|]{id|geo{lat|lon}}:\n  1|5|6\n  2|7|8",
		func(o *EncodeOptions) { o.Delimiter = Pipe })

	// Quoted subfield names; deeply nested group followed by a sibling.
	deep := obj(field("t", obj(field("p q", obj(field("x", num("1")))))), field("z", num("2")))
	assertEncode(t, arr(deep), "[1]{t{\"p q\"{x}},z}:\n  1,2")

	// A nested column that is not uniform in all rows disqualifies tabular form.
	mixed := arr(
		obj(field("id", num("1")), field("geo", obj(field("lat", num("5"))))),
		obj(field("id", num("2")), field("geo", obj(field("lon", num("6"))))),
	)
	assertEncode(t, mixed, "[2]:\n  - id: 1\n    geo:\n      lat: 5\n  - id: 2\n    geo:\n      lon: 6")
}

func TestEncodeListFormRules(t *testing.T) {
	// Arrays containing an empty object use list form; the empty object is a bare hyphen.
	assertEncode(t, obj(field("items", arr(obj(), obj(field("a", num("1")))))), "items[2]:\n  -\n  - a: 1")

	// The first field always stands on the hyphen line; its scope content is at depth+2.
	assertEncode(t, obj(field("items", arr(obj(field("a", obj(field("x", num("1")))), field("b", num("2"))), num("5")))),
		"items[2]:\n  - a:\n      x: 1\n    b: 2\n  - 5")
	assertEncode(t, obj(field("items", arr(obj(field("a", obj()), field("b", num("1")))))), "items[1]:\n  - a:\n    b: 1")
	assertEncode(t, obj(field("items", arr(obj(field("cfg", obj(
		field("a", obj(field("x", num("1")))), field("b", obj(field("x", num("2")))),
	)), field("s", str("ok"))), num("5")))), "items[2]:\n  - cfg[2:]{x}:\n      a: 1\n      b: 2\n    s: ok\n  - 5")

	// Nested arrays: header on the hyphen line, items one level below the hyphen.
	assertEncode(t, arr(arr(arr(num("1")))), "[1]:\n  - [1]:\n    - [1]: 1")
	assertEncode(t, obj(field("a", arr(arr(obj(field("x", num("1"))), obj(field("x", num("2"))))))),
		"a[1]:\n  - [2]:\n    - x: 1\n    - x: 2")
	assertEncode(t, obj(field("a", arr(arr(obj(field("x", arr(arr(num("1"))))))))),
		"a[1]:\n  - [1]:\n    - x[1]:\n        - [1]: 1")

	// Empty arrays: key: [] / [] at the root / - [0] as a list item (delimiter declared).
	assertEncode(t, obj(field("a", arr())), "a: []")
	assertEncode(t, arr(), "[]")
	assertEncode(t, obj(field("p", arr(arr(), arr()))), "p[2]:\n  - [0]:\n  - [0]:")
	assertEncode(t, obj(field("p", arr(arr(), arr()))), "p[2|]:\n  - [0|]:\n  - [0|]:", func(o *EncodeOptions) { o.Delimiter = Pipe })
	assertEncode(t, obj(field("items", arr(obj(field("data", arr())), num("1")))), "items[2]:\n  - data: []\n  - 1")

	// A null column value mixed with objects disqualifies the table.
	assertEncode(t, arr(obj(field("a", num("1"))), obj(field("a", &Node{Kind: NullKind}))), "[2]{a}:\n  1\n  null")
	assertEncode(t, arr(obj(field("a", obj(field("b", num("1"))))), obj(field("a", &Node{Kind: NullKind}))),
		"[2]:\n  - a:\n      b: 1\n  - a: null")
}

func TestEncodeQuotingInEveryPosition(t *testing.T) {
	assertEncode(t, str("#x"), "\"#x\"")
	assertEncode(t, str("\uFEFFx"), "\"\uFEFFx\"")
	assertEncode(t, obj(field("k", str("\uFEFFx"))), "k: \uFEFFx")
	assertEncode(t, obj(field("k", str("+1"))), "k: \"+1\"")
	assertEncode(t, obj(field("k", arr(str("#a"), str("b")))), "k[2]: \"#a\",b")
	assertEncode(t, arr(obj(field("k", str("#a")))), "[1]{k}:\n  \"#a\"")
	assertEncode(t, obj(field("k", str("x\u007fy\u0085z"))), "k: x\u007fy\u0085z")
	assertEncode(t, obj(field("k", str("a\u001fb"))), "k: \"a\\u001fb\"")
	assertEncode(t, obj(field("a\u001fb", num("1"))), "\"a\\u001fb\": 1")
	assertEncode(t, obj(field("", num("1")), field("#c", num("2")), field("-d", num("3"))), "\"\": 1\n\"#c\": 2\n\"-d\": 3")
	assertEncode(t, obj(field("a.b", num("1")), field("_x", num("2")), field("1a", num("3"))), "a.b: 1\n_x: 2\n\"1a\": 3")
}

func TestEncodeRejectsIllFormedUTF8(t *testing.T) {
	bad := string([]byte{'a', 0xff, 'b'})
	surrogate := "\xed\xa0\x80" // UTF-8 encoding of U+D800
	for name, n := range map[string]*Node{
		"root string":  str(bad),
		"field value":  obj(field("k", str(bad))),
		"surrogate":    obj(field("k", str(surrogate))),
		"object key":   obj(field(bad, num("1"))),
		"surrogate k":  obj(field(surrogate, num("1"))),
		"array item":   arr(num("1"), str(bad)),
		"tabular cell": arr(obj(field("a", str(bad)))),
		"nested key":   obj(field("a", obj(field(bad, num("1"))))),
	} {
		if out, err := Encode(n); CodeOf(err) != ErrInvalidUTF8 {
			t.Fatalf("%s: CodeOf = %q, err %v, out %q", name, CodeOf(err), err, out)
		}
	}
	assertEncode(t, str("caf\u00e9 \U0001F600"), "caf\u00e9 \U0001F600")
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

func TestEncodeOutOfRangeExponent(t *testing.T) {
	for _, raw := range []string{"12e9223372036854775807", "12e+9223372036854775808", "1.5e-99999999999999999999", "-3E18446744073709551616"} {
		n := obj(field("a", num(raw)))
		assertEncode(t, n, "a: "+raw)
		assertEncode(t, n, `a: "`+raw+`"`, func(o *EncodeOptions) { o.NumberMode = NumberStringForUnsafe })
		if out, err := Encode(n, func(o *EncodeOptions) { o.NumberMode = NumberFloat64 }); CodeOf(err) != ErrInvalidInputFormat {
			t.Fatalf("NumberFloat64 %q: out %q, CodeOf = %q, err %v", raw, out, CodeOf(err), err)
		}
		if got := CanonicalizeNumberToken(raw); got != raw {
			t.Fatalf("CanonicalizeNumberToken(%q) = %q", raw, got)
		}
	}
	// Zero stays zero whatever its exponent; in-range exponents still canonicalize.
	assertEncode(t, obj(field("a", num("0e9223372036854775807"))), "a: 0")
	assertEncode(t, obj(field("a", num("12e5"))), "a: 1200000")
}

func TestEncodeKeyedTabularRejectsDuplicateKeys(t *testing.T) {
	n := obj(field("t", obj(
		field("a", obj(field("x", num("1")))),
		field("a", obj(field("x", num("2")))),
	)))
	assertEncode(t, n, "t:\n  a:\n    x: 1\n  a:\n    x: 2")
}
