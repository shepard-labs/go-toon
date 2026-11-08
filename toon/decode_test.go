package toon

import (
	"errors"
	"reflect"
	"strconv"
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
	// SPEC §8: dotted keys are literal; the deprecated ExpandPaths option is a no-op.
	assertDecode(t, "a.b: 1", obj(field("a.b", num("1"))))
	assertDecode(t, "a.b: 1", obj(field("a.b", num("1"))), func(o *DecodeOptions) { o.ExpandPaths = ExpandPathsSafe })
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
		{"blank line inside array", "a[2]:\n  - 1\n  \n  - 2", ErrInvalidInputFormat},
		{"multiple root primitives", "a\nb", ErrMissingColon},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Decode([]byte(tc.in))
			if CodeOf(err) != tc.code {
				t.Fatalf("CodeOf = %q, err %v; want %q", CodeOf(err), err, tc.code)
			}
		})
	}
}

func TestDecodeNonStrictBehavior(t *testing.T) {
	assertDecode(t, "a: 1\na: 2", obj(field("a", num("2"))), func(o *DecodeOptions) { o.Strict = false })
	assertDecode(t, "a[2]: 1", obj(field("a", arr(num("1")))), func(o *DecodeOptions) { o.Strict = false })
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
		_, _ = parseHeader(s, true, nil)
		_, _ = parseHeader(s, false, nil)
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

func nonStrict(o *DecodeOptions) { o.Strict = false }

// SPEC §4: ill-formed UTF-8 errors in both modes and is never replaced by U+FFFD.
func TestDecodeRejectsIllFormedUTF8(t *testing.T) {
	inputs := map[string]string{
		"invalid byte":    "a: \xff",
		"truncated":       "a: \xe2\x82",
		"surrogate bytes": "a: \xed\xa0\x80",
		"overlong":        "a: \xc0\xaf",
		"in key":          "\xff: 1",
		"in comment":      "# \xff\na: 1",
		"later line":      "a: 1\nb: 2\nc: \xf0\x9f",
	}
	for name, in := range inputs {
		for _, opts := range [][]DecodeOption{nil, {nonStrict}} {
			n, err := Decode([]byte(in), opts...)
			if CodeOf(err) != ErrInvalidUTF8 || n != nil {
				t.Fatalf("%s: Decode(%q) = %v, %v; want invalid_utf8", name, in, n, err)
			}
		}
	}
	var perr *Error
	_, err := Decode([]byte("a: 1\nb: \xff"))
	if !errorsAs(err, &perr) || perr.Line != 2 {
		t.Fatalf("invalid UTF-8 error line = %#v", err)
	}
	assertDecode(t, "a: \u00e9\U0001F600", obj(field("a", str("\u00e9\U0001F600"))))
}

// SPEC §12 line preparation: BOM, CR, NBSP.
func TestDecodeLinePreparation(t *testing.T) {
	assertDecode(t, "\ufeffa: 1", obj(field("a", num("1"))))
	assertDecode(t, "\ufeff", obj())
	assertDecode(t, "\ufeffhello", str("hello"))
	assertDecode(t, "\ufeff\ufeffa: 1", obj(field("\ufeffa", num("1"))))
	assertDecode(t, "a: 1\r\nb: 2\r\n", obj(field("a", num("1")), field("b", num("2"))))
	assertDecode(t, "a: x\ry", obj(field("a", str("x\ry"))))
	assertDecode(t, "a: x\r\r\nb: 1", obj(field("a", str("x\r")), field("b", num("1"))))
	assertDecode(t, "a: 1 \r\n", obj(field("a", num("1"))))
	assertDecode(t, "a: \u00a0v\u00a0", obj(field("a", str("\u00a0v\u00a0"))))
	assertDecode(t, "a\u00a0: 1", obj(field("a\u00a0", num("1"))))
	// An NBSP run is content, never indentation: the second line is a depth-0 key.
	assertDecode(t, "a:\n\u00a0\u00a0b: 1", obj(field("a", obj()), field("\u00a0\u00a0b", num("1"))))
	assertDecode(t, "\u00a0", str("\u00a0"))
}

// SPEC §15: no key is special.
func TestDecodeProtoKeysAreOrdinary(t *testing.T) {
	assertDecode(t, "__proto__: x\nconstructor:\n  prototype: 1", obj(
		field("__proto__", str("x")),
		field("constructor", obj(field("prototype", num("1")))),
	))
	assertDecode(t, "__proto__[2]: 1,2\nt[1]{__proto__}:\n  a\nm[1:]{v}:\n  __proto__: 1", obj(
		field("__proto__", arr(num("1"), num("2"))),
		field("t", arr(obj(field("__proto__", str("a"))))),
		field("m", obj(field("__proto__", obj(field("v", num("1")))))),
	))
}

// SPEC §15: a declared length must not drive allocation.
func TestDecodeHugeDeclaredLengthDoesNotAllocate(t *testing.T) {
	for name, in := range map[string]string{
		"inline": "a[4611686018427387904]: 1",
		"list":   "a[4611686018427387904]:\n  - 1",
		"table":  "a[4611686018427387904]{x}:\n  1",
		"keyed":  "a[4611686018427387904:]{x}:\n  k: 1",
		"root":   "[18446744073709551616]: 1",
	} {
		allocs := testing.AllocsPerRun(5, func() { _, _ = Decode([]byte(in), nonStrict) })
		if allocs > 60 {
			t.Fatalf("%s: %v allocations for a tiny document", name, allocs)
		}
		if _, err := Decode([]byte(in)); CodeOf(err) != ErrArrayCountMismatch {
			t.Fatalf("%s: strict CodeOf = %q, err %v", name, CodeOf(err), err)
		}
		if _, err := Decode([]byte(in), func(o *DecodeOptions) { o.Limits.MaxArrayLength = 100 }); CodeOf(err) != ErrResourceLimit {
			t.Fatalf("%s: limited CodeOf = %q, err %v", name, CodeOf(err), err)
		}
	}
	assertDecode(t, "a[999999999999]:\n  - x", obj(field("a", arr(str("x")))), nonStrict)
}

// SPEC §14.4: non-strict decoding applies exactly five recoveries.
func TestDecodeNonStrictRecoveries(t *testing.T) {
	// 1. Declared counts never truncate or terminate a scope.
	assertDecode(t, "a[1]: 1,2,3", obj(field("a", arr(num("1"), num("2"), num("3")))), nonStrict)
	assertDecode(t, "t[1]{a}:\n  1\n  2", obj(field("t", arr(obj(field("a", num("1"))), obj(field("a", num("2")))))), nonStrict)
	assertDecode(t, "l[0]:\n  - x\n  - y", obj(field("l", arr(str("x"), str("y")))), nonStrict)
	// 2. Duplicate names and keys: last write wins at the first position.
	assertDecode(t, "t[1]{a,b,a}:\n  1,2,3", obj(field("t", arr(obj(field("a", num("3")), field("b", num("2")))))), nonStrict)
	assertDecode(t, "a: 1\nb: 2\na: 3", obj(field("a", num("3")), field("b", num("2"))), nonStrict)
	// 3. Indentation: depth = tabs + spaces/indentSize.
	assertDecode(t, "a:\n\tb: 1\n   c: 2", obj(field("a", obj(field("b", num("1")), field("c", num("2"))))), nonStrict)
	// 4. Blank lines inside a header span are ignored.
	assertDecode(t, "l[2]:\n  - a\n\n  - b", obj(field("l", arr(str("a"), str("b")))), nonStrict)
	// 5. Depth jump adopts the first line's depth.
	assertDecode(t, "a:\n    b: 1\n    c: 2", obj(field("a", obj(field("b", num("1")), field("c", num("2"))))), nonStrict)

	// Everything else still errors in non-strict mode.
	for _, in := range []string{
		"a: \"x\" y",                  // characters after a closing quote
		"a: \"\\x\"",                  // invalid escape
		"t[1]{a}:\n  1,2",             // row width
		"m[1:]{a}:\n  k:",             // entry row width
		"a: 1\n  b: 2",                // over-indented
		"  a: 1",                      // indented root
		"[2]: 1\njunk: 3",             // trailing content after a root array
		"[]\njunk: 3",                 // trailing content after a root empty array
		"foo [2]: x",                  // whitespace before a bracket segment
		"k[01]: 1",                    // malformed length
		"a[1]{x,}:\n  1",              // empty field entry
		"a[1]{x|y}:\n  1,2",           // other delimiter in field list
		"a:\n  [2]: 1,2",              // keyless header in field position
		"hello\nworld",                // second scalar line
		"l[1]:\n  - a: 1\n  x: 1",     // non-item line at item depth
		"l[1]:\n  - a: 1\n      b: 2", // over-indented under a list-item field
		"a: 1\na.b: 2\n  c: 3",        // over-indent after dotted keys
	} {
		if n, err := Decode([]byte(in), nonStrict); err == nil {
			t.Fatalf("non-strict Decode(%q) succeeded: %#v", in, n)
		}
	}
}

func TestDecodeRootForms(t *testing.T) {
	assertDecode(t, "# only a comment\n\n  # another", obj())
	assertDecode(t, "[2]: 1,2", arr(num("1"), num("2")))
	assertDecode(t, "[2:]{v}:\n  a: 1\n  b: 2", obj(
		field("a", obj(field("v", num("1")))),
		field("b", obj(field("v", num("2")))),
	))
	assertDecode(t, "items[2]", str("items[2]"))
	for _, in := range []string{"[2:]{v}:\n  a: 1\n  b: 2\njunk: 3", "[1]: x\n[1]: y", "[]\n[]"} {
		for _, opts := range [][]DecodeOption{nil, {nonStrict}} {
			if _, err := Decode([]byte(in), opts...); err == nil {
				t.Fatalf("Decode(%q) accepted trailing root content", in)
			}
		}
	}
}

func TestDecodeNestedFieldGroupsAndKeyedTables(t *testing.T) {
	assertDecode(t, "orders[2]{id,customer{name,country},total}:\n  1,Ada,DK,99\n  2,Bob,UK,149", obj(field("orders", arr(
		obj(field("id", num("1")), field("customer", obj(field("name", str("Ada")), field("country", str("DK")))), field("total", num("99"))),
		obj(field("id", num("2")), field("customer", obj(field("name", str("Bob")), field("country", str("UK")))), field("total", num("149"))),
	))))
	assertDecode(t, "users[2:]{age,city}:\n  alice: 30,Berlin\n  bob: 25,Oslo", obj(field("users", obj(
		field("alice", obj(field("age", num("30")), field("city", str("Berlin")))),
		field("bob", obj(field("age", num("25")), field("city", str("Oslo")))),
	))))
	// SPEC §10: keyed tabular object as the first field of a list-item object.
	assertDecode(t, "l[1]:\n  - m[1:]{v}:\n      k: 1\n    s: ok", obj(field("l", arr(obj(
		field("m", obj(field("k", obj(field("v", num("1")))))),
		field("s", str("ok")),
	)))))
}

// Nested field groups may nest without a cap; parsing must not recurse per level.
func TestDecodeDeepFieldGroups(t *testing.T) {
	const depth = 20000
	in := "t[1]{" + strings.Repeat("a{", depth) + "b" + strings.Repeat("}", depth) + "}:\n  1"
	n, err := Decode([]byte(in))
	if err != nil || n.Kind != ObjectKind {
		t.Fatalf("deep field groups: %v", err)
	}
	if _, err := Decode([]byte("t[1]{a{b{c}}}:\n  1"), func(o *DecodeOptions) { o.Limits.MaxDepth = 4 }); CodeOf(err) != ErrResourceLimit {
		t.Fatalf("MaxDepth over nested groups: %v", err)
	}
	if _, err := Decode([]byte("t[1]{a{b{c}}}:\n  1"), func(o *DecodeOptions) { o.Limits.MaxDepth = 5 }); err != nil {
		t.Fatalf("MaxDepth exactly fitting nested groups: %v", err)
	}
}

// SPEC §4: the number grammar is JSON's, never a host parser's.
func TestDecodeNumberGrammar(t *testing.T) {
	for _, s := range []string{"05", "-05", ".5", "1.", "+5", "Infinity", "NaN", "0x10", "1_000", "1e", "--1"} {
		assertDecode(t, "v: "+s, obj(field("v", str(s))))
	}
	for s, want := range map[string]string{"-0": "0", "0e1": "0", "1E+03": "1E+03", "1.5000": "1.5000", "-1.5e-7": "-1.5e-7"} {
		assertDecode(t, "v: "+s, obj(field("v", num(want))))
	}
	assertDecode(t, "v: 123456789012345678901234567890", obj(field("v", num("123456789012345678901234567890"))))
}

func errorsAs(err error, target **Error) bool { return errors.As(err, target) }

type countingReader struct{ n int64 }

func (r *countingReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 'a'
	}
	r.n += int64(len(p))
	return len(p), nil
}

func TestDecodeReaderHonorsMaxBytes(t *testing.T) {
	r := &countingReader{}
	_, err := DecodeReader(r, func(o *DecodeOptions) { o.Limits.MaxBytes = 1000 })
	if CodeOf(err) != ErrResourceLimit {
		t.Fatalf("CodeOf = %q, err %v", CodeOf(err), err)
	}
	if r.n > 1<<16 {
		t.Fatalf("read %d bytes past a 1000-byte limit", r.n)
	}
	if _, err := DecodeReader(strings.NewReader("a: 1"), func(o *DecodeOptions) { o.Limits.MaxBytes = 4 }); err != nil {
		t.Fatalf("input at the limit: %v", err)
	}
	if _, err := DecodeReader(strings.NewReader("a: 12"), func(o *DecodeOptions) { o.Limits.MaxBytes = 4 }); CodeOf(err) != ErrResourceLimit {
		t.Fatalf("input over the limit: CodeOf = %q, err %v", CodeOf(err), err)
	}
}

func TestDecodeNonStrictDeclaredLengthOverLimit(t *testing.T) {
	limit := func(o *DecodeOptions) { o.Limits.MaxArrayLength = 5 }
	nonStrict := func(o *DecodeOptions) { limit(o); o.Strict = false }
	for _, in := range []string{"a[100]: 1,2", "a[100]:\n  - 1", "a[100]{x}:\n  1"} {
		if _, err := Decode([]byte(in), nonStrict); err != nil {
			t.Fatalf("non-strict Decode(%q): %v", in, err)
		}
		if _, err := Decode([]byte(in), limit); CodeOf(err) != ErrResourceLimit {
			t.Fatalf("strict Decode(%q) CodeOf = %q, err %v", in, CodeOf(err), err)
		}
	}
	if _, err := Decode([]byte("a[100]: 1,2,3,4,5,6"), nonStrict); CodeOf(err) != ErrResourceLimit {
		t.Fatalf("actual element count over the limit: CodeOf = %q, err %v", CodeOf(err), err)
	}
}

func TestDecodeFieldListLimits(t *testing.T) {
	depth := func(o *DecodeOptions) { o.Limits.MaxDepth = 3 }
	if _, err := Decode([]byte("t[0]{a{b{c}}}:"), depth); err != nil {
		t.Fatalf("nesting within MaxDepth: %v", err)
	}
	for _, in := range []string{
		"t[0]{a{b{c{d{e}}}}}:",
		"t[0]{" + strings.Repeat("a{", 1_000_000) + "a" + strings.Repeat("}", 1_000_000) + "}:",
	} {
		if _, err := Decode([]byte(in), depth); CodeOf(err) != ErrResourceLimit {
			t.Fatalf("CodeOf = %q, err %v", CodeOf(err), err)
		}
	}

	nodes := func(o *DecodeOptions) { o.Limits.MaxNodes = 100 }
	var wide strings.Builder
	wide.WriteString("t[0]{f0")
	for i := 1; i < 5000; i++ {
		wide.WriteString(",f" + strconv.Itoa(i))
	}
	wide.WriteString("}:")
	if _, err := Decode([]byte(wide.String()), nodes); CodeOf(err) != ErrResourceLimit {
		t.Fatalf("wide header: CodeOf = %q, err %v", CodeOf(err), err)
	}
	// Field-list entries of successive headers add up against the node limit.
	var many strings.Builder
	for i := 0; i < 30; i++ {
		many.WriteString("k" + strconv.Itoa(i) + "[0]{a,b,c,d,e}:\n")
	}
	if _, err := Decode([]byte("k[0]{a,b,c,d,e}:"), nodes); err != nil {
		t.Fatalf("single small header: %v", err)
	}
	if _, err := Decode([]byte(strings.TrimSuffix(many.String(), "\n")), nodes); CodeOf(err) != ErrResourceLimit {
		t.Fatalf("cumulative header fields: CodeOf = %q, err %v", CodeOf(err), err)
	}
}

func TestDecodeMaxStringBytesAppliesToKeys(t *testing.T) {
	limit := func(o *DecodeOptions) { o.Limits.MaxStringBytes = 4 }
	for _, in := range []string{
		"abcdefgh: 1",
		"abcdefgh:\n  a: 1",
		"abcdefgh[1]: 1",
		"t[1]{abcdefgh}:\n  1",
		"t[1]{a{abcdefgh}}:\n  1",
		"t[1:]{a}:\n  abcdefgh: 1",
		`"abcdefgh": 1`,
	} {
		if _, err := Decode([]byte(in), limit); CodeOf(err) != ErrResourceLimit {
			t.Fatalf("Decode(%q) CodeOf = %q, err %v", in, CodeOf(err), err)
		}
	}
	if _, err := Decode([]byte("abcd: 1\nt[1]{ab}:\n  1\nk[1:]{a}:\n  efgh: 1"), limit); err != nil {
		t.Fatalf("keys within the limit: %v", err)
	}
}

func TestDecodeKeepsOutOfRangeExponentVerbatim(t *testing.T) {
	for _, raw := range []string{"12e9223372036854775807", "12E+9223372036854775808", "1.5e-99999999999999999999", "-3e18446744073709551616"} {
		n, err := Decode([]byte("a: " + raw))
		if err != nil {
			t.Fatalf("Decode(%q): %v", raw, err)
		}
		if got := n.Object[0].Value; got.Kind != NumberKind || got.Number.Raw != raw {
			t.Fatalf("Decode(%q) = %#v", raw, got)
		}
	}
}
