package toon

import (
	"reflect"
	"strings"
	"testing"
)

func TestDecodePositiveGoldenRootValues(t *testing.T) {
	assertDecode(t, "", obj())
	assertDecode(t, "[]", arr())
	assertDecode(t, "true", boolNode(true))
	assertDecode(t, "-0", num("0"))
	assertDecode(t, "hello", str("hello"))
}

func TestDecodePositiveGoldenObjects(t *testing.T) {
	assertDecode(t, "id: 1\nuser:\n  name: Ada\n  active: true\nempty:", obj(
		field("id", num("1")),
		field("user", obj(field("name", str("Ada")), field("active", boolNode(true)))),
		field("empty", obj()),
	))
	assertDecode(t, "items: []", obj(field("items", arr())))
	assertDecode(t, "items[0]:", obj(field("items", arr())))
	assertDecode(t, "[0]:", arr())
}

func TestDecodePositiveGoldenArrays(t *testing.T) {
	assertDecode(t, "tags[3]: admin,ops,dev", obj(field("tags", arr(str("admin"), str("ops"), str("dev")))))
	assertDecode(t, "tags[2|]: a,b|\"c|d\"", obj(field("tags", arr(str("a,b"), str("c|d")))))
	assertDecode(t, "tags[2\t]: a,b\tc|d", obj(field("tags", arr(str("a,b"), str("c|d")))))
	assertDecode(t, "items[2]{sku,qty}:\n  A1,2\n  B2,1", obj(field("items", arr(
		obj(field("sku", str("A1")), field("qty", num("2"))),
		obj(field("sku", str("B2")), field("qty", num("1"))),
	))))
	assertDecode(t, "items[4]:\n  - 1\n  - id: 2\n    name: Bob\n  - [2]: a,b\n  -", obj(field("items", arr(
		num("1"),
		obj(field("id", num("2")), field("name", str("Bob"))),
		arr(str("a"), str("b")),
		obj(),
	))))
}

func TestDecodePositiveGoldenQuotedAndDotted(t *testing.T) {
	assertDecode(t, "\"sp ace\": \"hello: \\\"toon\\\"\\\\\\n\"", obj(Field{Key: "sp ace", WasQuoted: true, Value: str("hello: \"toon\"\\\n")}))
	assertDecode(t, "a.b: 1", obj(field("a.b", num("1"))))
	got, err := Decode([]byte("a.b: 1"), func(o *DecodeOptions) { o.ExpandPaths = ExpandPathsSafe })
	if err != nil {
		t.Fatalf("Decode expansion error: %v", err)
	}
	want := obj(field("a", obj(field("b", num("1")))))
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expanded = %#v, want %#v", got, want)
	}
}

func TestDecodeLosslessNumericDecoding(t *testing.T) {
	assertDecode(t, "n: 1.2300e+3", obj(field("n", num("1.2300e+3"))))
	assertDecode(t, "n: 01", obj(field("n", str("01"))))
}

func TestDecodeStrictNegativeErrors(t *testing.T) {
	cases := []struct {
		name string
		in   string
		code ErrorCode
	}{
		{"invalid indent", "a:\n b: 1", ErrInvalidIndent},
		{"tab indent", "a:\n\tb: 1", ErrTabIndent},
		{"missing colon", "a\nb: 1", ErrMissingColon},
		{"invalid escape", "a: \"\\x\"", ErrInvalidEscape},
		{"unterminated string", "a: \"x", ErrUnterminatedString},
		{"bad header length", "a[01]: 1", ErrMalformedHeader},
		{"content before colon", "a[1] : 1", ErrMalformedHeader},
		{"header delimiter mismatch", "a[1|]{x,y}:\n  1,2", ErrHeaderDelimiterMismatch},
		{"inline count", "a[2]: 1", ErrArrayCountMismatch},
		{"list count", "a[2]:\n  - 1", ErrArrayCountMismatch},
		{"tabular count", "a[2]{x}:\n  1", ErrArrayCountMismatch},
		{"tabular width", "a[1]{x,y}:\n  1", ErrTabularWidthMismatch},
		{"duplicate", "a: 1\na: 2", ErrDuplicateKey},
		{"path conflict", "a: 1\na.b: 2", ErrPathExpansionConflict},
		{"blank line inside array", "a[2]:\n  - 1\n  \n  - 2", ErrInvalidInputFormat},
		{"multiple root primitives", "a\nb", ErrMissingColon},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts := []DecodeOption{}
			if tc.code == ErrPathExpansionConflict {
				opts = append(opts, func(o *DecodeOptions) { o.ExpandPaths = ExpandPathsSafe })
			}
			_, err := Decode([]byte(tc.in), opts...)
			if CodeOf(err) != tc.code {
				t.Fatalf("CodeOf = %q, err %v; want %q", CodeOf(err), err, tc.code)
			}
		})
	}
}

func TestDecodeNonStrictBehavior(t *testing.T) {
	assertDecode(t, "a: 1\na: 2", obj(field("a", num("2"))), func(o *DecodeOptions) { o.Strict = false })
	assertDecode(t, "a[2]: 1", obj(field("a", arr(num("1")))), func(o *DecodeOptions) { o.Strict = false })
	got, err := Decode([]byte("a: 1\na.b: 2"), func(o *DecodeOptions) {
		o.Strict = false
		o.ExpandPaths = ExpandPathsSafe
	})
	if err != nil {
		t.Fatalf("non-strict expansion error: %v", err)
	}
	want := obj(field("a", obj(field("b", num("2")))))
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("non-strict expansion = %#v, want %#v", got, want)
	}
}

func TestValidateAndReader(t *testing.T) {
	if err := Validate([]byte("a: 1")); err != nil {
		t.Fatalf("Validate positive error: %v", err)
	}
	if err := Validate([]byte("a: \"x")); CodeOf(err) != ErrUnterminatedString {
		t.Fatalf("Validate negative CodeOf = %q", CodeOf(err))
	}
	n, err := DecodeReader(strings.NewReader("a: 1"))
	if err != nil || !reflect.DeepEqual(n, obj(field("a", num("1")))) {
		t.Fatalf("DecodeReader = %#v, %v", n, err)
	}
}

func TestDecodeResourceLimits(t *testing.T) {
	for name, tc := range map[string]struct {
		in     string
		limits ResourceLimits
	}{
		"bytes":  {"a: 1", ResourceLimits{MaxBytes: 3}},
		"nodes":  {"a: 1", ResourceLimits{MaxNodes: 1}},
		"depth":  {"a:\n  b: 1", ResourceLimits{MaxDepth: 1}},
		"string": {"a: abcd", ResourceLimits{MaxStringBytes: 3}},
		"array":  {"a[2]: 1,2", ResourceLimits{MaxArrayLength: 1}},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := Decode([]byte(tc.in), func(o *DecodeOptions) { o.Limits = tc.limits })
			if CodeOf(err) != ErrResourceLimit {
				t.Fatalf("CodeOf = %q, err %v", CodeOf(err), err)
			}
		})
	}
}

func TestDecodeRoundTripEncodeFixtures(t *testing.T) {
	fixtures := []*Node{
		obj(field("id", num("1")), field("name", str("Ada"))),
		obj(field("tags", arr(str("admin"), str("ops")))),
		obj(field("items", arr(obj(field("sku", str("A1")), field("qty", num("2")))))),
	}
	for _, fixture := range fixtures {
		encoded, err := Encode(fixture)
		if err != nil {
			t.Fatalf("Encode fixture error: %v", err)
		}
		decoded, err := Decode(encoded)
		if err != nil {
			t.Fatalf("Decode fixture %q error: %v", encoded, err)
		}
		reencoded, err := Encode(decoded)
		if err != nil {
			t.Fatalf("Re-encode fixture error: %v", err)
		}
		if string(reencoded) != string(encoded) {
			t.Fatalf("round trip = %q, want %q", reencoded, encoded)
		}
	}
}

func fuzzDecodeLimits() DecodeOption {
	return func(o *DecodeOptions) {
		o.Limits = ResourceLimits{
			MaxBytes:       256 * 1024,
			MaxDepth:       64,
			MaxNodes:       100_000,
			MaxArrayLength: 10_000,
			MaxStringBytes: 64 * 1024,
		}
	}
}

func FuzzDecode(f *testing.F) {
	f.Add("a: 1")
	f.Add("items[2]:\n  - 1\n  - 2")
	f.Fuzz(func(t *testing.T, s string) {
		if len(s) > 256*1024 {
			t.Skip()
		}
		_, _ = Decode([]byte(s), fuzzDecodeLimits())
	})
}

func FuzzUnescapeQuotedToken(f *testing.F) {
	f.Add(`"hello"`)
	f.Fuzz(func(t *testing.T, s string) { _, _ = UnescapeQuotedToken(s) })
}

func FuzzSplitDelimited(f *testing.F) {
	f.Add("a,b")
	f.Fuzz(func(t *testing.T, s string) { _, _ = SplitDelimited(s, Comma) })
}

func FuzzHeaderParser(f *testing.F) {
	f.Add("a[1]: 1")
	f.Fuzz(func(t *testing.T, s string) {
		d := &decoder{opts: DefaultDecodeOptions()}
		_, _, _ = d.parseHeaderContent(s, true)
	})
}

func FuzzNumberCanonicalizer(f *testing.F) {
	f.Add("1.2300e+3")
	f.Fuzz(func(t *testing.T, s string) { _ = CanonicalizeNumberToken(s) })
}

func assertDecode(t *testing.T, in string, want *Node, opts ...DecodeOption) {
	t.Helper()
	got, err := Decode([]byte(in), opts...)
	if err != nil {
		t.Fatalf("Decode(%q) error: %v", in, err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Decode(%q) = %#v, want %#v", in, got, want)
	}
	if err := Validate([]byte(in), opts...); err != nil {
		t.Fatalf("Validate(%q) error: %v", in, err)
	}
}
