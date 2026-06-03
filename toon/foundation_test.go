package toon

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"testing"
)

func TestSpecVersion(t *testing.T) {
	if SpecVersion != "3.3" {
		t.Fatalf("SpecVersion = %q, want 3.3", SpecVersion)
	}
}

func TestNodePreservesObjectFieldOrderAndWasQuoted(t *testing.T) {
	n := &Node{Kind: ObjectKind, Object: []Field{
		{Key: "z", Value: &Node{Kind: NullKind}},
		{Key: "a.b", WasQuoted: true, Value: &Node{Kind: StringKind, String: "literal"}},
		{Key: "a.b", WasQuoted: false, Value: &Node{Kind: StringKind, String: "path"}},
	}}

	if n.Object[0].Key != "z" || n.Object[1].Key != "a.b" || n.Object[2].Key != "a.b" {
		t.Fatalf("object field order was not preserved: %#v", n.Object)
	}
	if !n.Object[1].WasQuoted || n.Object[2].WasQuoted {
		t.Fatalf("WasQuoted not distinct from key text: %#v", n.Object)
	}
}

func TestOptionDefaultsAndResolution(t *testing.T) {
	enc := ResolveEncodeOptions(func(o *EncodeOptions) {
		o.Delimiter = Pipe
		o.Limits.MaxDepth = 9
	})
	if enc.IndentSize != 2 || enc.Delimiter != Pipe || enc.KeyFolding != KeyFoldingOff || enc.FlattenDepth != math.MaxInt || enc.NumberMode != NumberLossless || enc.Limits.MaxDepth != 9 {
		t.Fatalf("bad encode defaults/resolution: %#v", enc)
	}

	dec := ResolveDecodeOptions(func(o *DecodeOptions) {
		o.Strict = false
		o.ExpandPaths = ExpandPathsSafe
	})
	if dec.IndentSize != 2 || dec.Strict || dec.ExpandPaths != ExpandPathsSafe {
		t.Fatalf("bad decode defaults/resolution: %#v", dec)
	}
}

func TestEnumValues(t *testing.T) {
	if Comma != ',' || Tab != '\t' || Pipe != '|' {
		t.Fatalf("bad delimiter values")
	}
	if KeyFoldingOff != 0 || KeyFoldingSafe != 1 || ExpandPathsOff != 0 || ExpandPathsSafe != 1 {
		t.Fatalf("bad folding/path enum values")
	}
	if NumberLossless != 0 || NumberFloat64 != 1 || NumberStringForUnsafe != 2 {
		t.Fatalf("bad number mode enum values")
	}
}

func TestStructuredErrors(t *testing.T) {
	cause := errors.New("cause")
	err := &Error{Code: ErrInvalidEscape, Line: 2, Column: 4, Message: "bad escape", Context: "x", Cause: cause}
	wrapped := fmt.Errorf("wrap: %w", err)

	if CodeOf(wrapped) != ErrInvalidEscape {
		t.Fatalf("CodeOf = %q", CodeOf(wrapped))
	}
	if !errors.Is(err, cause) {
		t.Fatalf("Error does not unwrap cause")
	}
	var asErr *Error
	if !errors.As(wrapped, &asErr) || asErr.Context != "x" {
		t.Fatalf("errors.As failed: %#v", asErr)
	}
	if got := NewError(ErrMissingColon, "missing colon").Error(); got != "missing colon" {
		t.Fatalf("Error() = %q", got)
	}
}

func TestResourceLimits(t *testing.T) {
	c := NewLimitCounter(ResourceLimits{MaxBytes: 3, MaxStringBytes: 2, MaxNodes: 1, MaxDepth: 2, MaxArrayLength: 4})
	if err := c.CheckInputBytes(3); err != nil {
		t.Fatalf("unexpected input limit error: %v", err)
	}
	if err := c.CheckStringBytes(2); err != nil {
		t.Fatalf("unexpected string limit error: %v", err)
	}
	if err := c.AddNode(); err != nil {
		t.Fatalf("unexpected node limit error: %v", err)
	}
	if err := c.CheckDepth(2); err != nil {
		t.Fatalf("unexpected depth limit error: %v", err)
	}
	if err := c.CheckArrayLength(4); err != nil {
		t.Fatalf("unexpected array limit error: %v", err)
	}

	for name, err := range map[string]error{
		"bytes":  c.CheckInputBytes(4),
		"string": c.CheckStringBytes(3),
		"nodes":  c.AddNode(),
		"depth":  c.CheckDepth(3),
		"array":  c.CheckArrayLength(5),
	} {
		if CodeOf(err) != ErrResourceLimit {
			t.Fatalf("%s CodeOf = %q, err %v", name, CodeOf(err), err)
		}
	}
}

func TestKeyValidation(t *testing.T) {
	validKeys := []string{"a", "A_1", "a.b", "_x.y2"}
	invalidKeys := []string{"", "1a", ".a", "a-b", "a b"}
	for _, s := range validKeys {
		if !IsValidUnquotedKey(s) {
			t.Fatalf("IsValidUnquotedKey(%q) = false", s)
		}
	}
	for _, s := range invalidKeys {
		if IsValidUnquotedKey(s) {
			t.Fatalf("IsValidUnquotedKey(%q) = true", s)
		}
	}

	if !IsValidSafeSegment("a_1") || IsValidSafeSegment("a.b") || IsValidSafeSegment("1a") {
		t.Fatalf("safe segment validation failed")
	}
}

func TestNeedsQuotesMatrix(t *testing.T) {
	cases := []struct {
		value string
		want  bool
	}{
		{"", true},
		{" leading", true},
		{"trailing ", true},
		{"true", true},
		{"false", true},
		{"null", true},
		{"12", true},
		{"01", true},
		{"1e+2", true},
		{"a:b", true},
		{"a\"b", true},
		{`a\b`, true},
		{"[", true},
		{"]", true},
		{"{", true},
		{"}", true},
		{"a\nb", true},
		{"a,b", true},
		{"a|b", false},
		{"-", true},
		{"-abc", true},
		{"abc", false},
	}
	for _, tc := range cases {
		if got := NeedsQuotes(tc.value, Comma); got != tc.want {
			t.Fatalf("NeedsQuotes(%q) = %v, want %v", tc.value, got, tc.want)
		}
	}
	if !NeedsQuotes("a|b", Pipe) {
		t.Fatalf("pipe delimiter did not require quotes")
	}
}

func TestEscapeString(t *testing.T) {
	in := "\\\"\n\r\t" + string(rune(0x01)) + "é"
	want := `\\\"\n\r\t\u0001é`
	if got := EscapeString(in); got != want {
		t.Fatalf("EscapeString = %q, want %q", got, want)
	}
}

func TestUnescapeQuotedToken(t *testing.T) {
	valid := map[string]string{
		`"\\"`:     `\`,
		`"\""`:     `"`,
		`"\n"`:     "\n",
		`"\r"`:     "\r",
		`"\t"`:     "\t",
		`"\u00E9"`: "é",
	}
	for token, want := range valid {
		got, err := UnescapeQuotedToken(token)
		if err != nil || got != want {
			t.Fatalf("UnescapeQuotedToken(%q) = %q, %v; want %q", token, got, err, want)
		}
	}

	invalid := []string{`"\x"`, `"\`, `"\u12"`, `"\u12X4"`, `"\uD800"`, `"abc`, `"a" b`}
	for _, token := range invalid {
		if _, err := UnescapeQuotedToken(token); err == nil {
			t.Fatalf("UnescapeQuotedToken(%q) succeeded", token)
		}
	}
	if _, err := UnescapeQuotedToken(`"abc`); CodeOf(err) != ErrUnterminatedString {
		t.Fatalf("unterminated code = %q", CodeOf(err))
	}
}

func TestSplitDelimited(t *testing.T) {
	got, err := SplitDelimited(` a, ,"b,c",d\,e,a|b `, Comma)
	if err != nil {
		t.Fatalf("SplitDelimited error: %v", err)
	}
	want := []string{"a", "", `"b,c"`, `d\`, "e", "a|b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SplitDelimited = %#v, want %#v", got, want)
	}
	got, err = SplitDelimited(`a,b|c`, Pipe)
	if err != nil || !reflect.DeepEqual(got, []string{"a,b", "c"}) {
		t.Fatalf("pipe SplitDelimited = %#v, %v", got, err)
	}
	if _, err := SplitDelimited(`"abc`, Comma); CodeOf(err) != ErrUnterminatedString {
		t.Fatalf("unterminated split code = %q", CodeOf(err))
	}
}

func TestParsePrimitiveToken(t *testing.T) {
	cases := []struct {
		token string
		kind  Kind
		str   string
		bool  bool
		num   string
	}{
		{"", StringKind, "", false, ""},
		{`"true"`, StringKind, "true", false, ""},
		{"true", BoolKind, "", true, ""},
		{"false", BoolKind, "", false, ""},
		{"null", NullKind, "", false, ""},
		{"123", NumberKind, "", false, "123"},
		{"-0", NumberKind, "", false, "0"},
		{"01", StringKind, "01", false, ""},
		{"[]", StringKind, "[]", false, ""},
		{"abc", StringKind, "abc", false, ""},
	}
	for _, tc := range cases {
		n, err := ParsePrimitiveToken(tc.token)
		if err != nil {
			t.Fatalf("ParsePrimitiveToken(%q) error: %v", tc.token, err)
		}
		if n.Kind != tc.kind || n.String != tc.str || n.Bool != tc.bool || n.Number.Raw != tc.num {
			t.Fatalf("ParsePrimitiveToken(%q) = %#v", tc.token, n)
		}
	}
}

func TestNumberValidationAndCanonicalization(t *testing.T) {
	valid := []string{"0", "-0", "1", "-1", "1.2", "1e2", "1E-2", "10e+20"}
	invalid := []string{"", "-", ".1", "1.", "1e", "01", "-01", "00.1"}
	for _, s := range valid {
		if !IsValidNumberToken(s) {
			t.Fatalf("IsValidNumberToken(%q) = false", s)
		}
	}
	for _, s := range invalid {
		if IsValidNumberToken(s) {
			t.Fatalf("IsValidNumberToken(%q) = true", s)
		}
	}
	if got := CanonicalizeNumberToken("-0"); got != "0" {
		t.Fatalf("CanonicalizeNumberToken(-0) = %q", got)
	}
	if got := CanonicalizeNumberToken("-0.0e+3"); got != "0" {
		t.Fatalf("CanonicalizeNumberToken(-0.0e+3) = %q", got)
	}
	if got := CanonicalizeNumberToken("-0.1"); got != "-0.1" {
		t.Fatalf("CanonicalizeNumberToken(-0.1) = %q", got)
	}
}
