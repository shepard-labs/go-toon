package reflect

import (
	"encoding"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/shepard-labs/go-toon/toon"
)

func TestNodeFromValueScalars(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want *toon.Node
	}{
		{"nil interface", nil, &toon.Node{Kind: toon.NullKind}},
		{"true", true, &toon.Node{Kind: toon.BoolKind, Bool: true}},
		{"false", false, &toon.Node{Kind: toon.BoolKind, Bool: false}},
		{"empty string", "", &toon.Node{Kind: toon.StringKind, String: ""}},
		{"plain string", "hello", &toon.Node{Kind: toon.StringKind, String: "hello"}},
		{"int", 42, &toon.Node{Kind: toon.NumberKind, Number: toon.Number{Raw: "42"}}},
		{"negative int", -7, &toon.Node{Kind: toon.NumberKind, Number: toon.Number{Raw: "-7"}}},
		{"int64", int64(9223372036854775807), &toon.Node{Kind: toon.NumberKind, Number: toon.Number{Raw: "9223372036854775807"}}},
		{"uint", uint(7), &toon.Node{Kind: toon.NumberKind, Number: toon.Number{Raw: "7"}}},
		{"float64", 3.14, &toon.Node{Kind: toon.NumberKind, Number: toon.Number{Raw: "3.14"}}},
		{"named int", namedInt(5), &toon.Node{Kind: toon.NumberKind, Number: toon.Number{Raw: "5"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NodeFromValue(tc.in)
			if err != nil {
				t.Fatalf("NodeFromValue error: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("NodeFromValue = %#v, want %#v", got, tc.want)
			}
		})
	}
}

type namedInt int64

type scalar struct {
	Name string
	Age  int
}

func TestNodeFromValueSliceAndArray(t *testing.T) {
	arr := &toon.Node{Kind: toon.ArrayKind, Array: []*toon.Node{
		{Kind: toon.NumberKind, Number: toon.Number{Raw: "1"}},
		{Kind: toon.NumberKind, Number: toon.Number{Raw: "2"}},
		{Kind: toon.NumberKind, Number: toon.Number{Raw: "3"}},
	}}
	got, err := NodeFromValue([]int{1, 2, 3})
	if err != nil {
		t.Fatalf("NodeFromValue error: %v", err)
	}
	if !reflect.DeepEqual(got, arr) {
		t.Fatalf("slice = %#v", got)
	}

	got, err = NodeFromValue([3]string{"a", "b", "c"})
	if err != nil {
		t.Fatalf("NodeFromValue error: %v", err)
	}
	want := &toon.Node{Kind: toon.ArrayKind, Array: []*toon.Node{
		{Kind: toon.StringKind, String: "a"},
		{Kind: toon.StringKind, String: "b"},
		{Kind: toon.StringKind, String: "c"},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("array = %#v", got)
	}
}

func TestNodeFromValueByteSlice(t *testing.T) {
	got, err := NodeFromValue([]byte("Hello"))
	if err != nil {
		t.Fatalf("NodeFromValue error: %v", err)
	}
	if got.Kind != toon.StringKind || got.String != "SGVsbG8=" {
		t.Fatalf("byte slice = %#v", got)
	}
}

type typedBytes []byte

func TestNodeFromValueTypedByteSlice(t *testing.T) {
	got, err := NodeFromValue(typedBytes("Hi"))
	if err != nil {
		t.Fatalf("NodeFromValue error: %v", err)
	}
	if got.Kind != toon.StringKind || got.String != "SGk=" {
		t.Fatalf("typed byte slice = %#v", got)
	}
}

func TestNodeFromValueMapSortsKeys(t *testing.T) {
	got, err := NodeFromValue(map[string]int{"b": 2, "a": 1, "c": 3})
	if err != nil {
		t.Fatalf("NodeFromValue error: %v", err)
	}
	if got.Kind != toon.ObjectKind || len(got.Object) != 3 {
		t.Fatalf("map shape = %#v", got)
	}
	if got.Object[0].Key != "a" || got.Object[1].Key != "b" || got.Object[2].Key != "c" {
		t.Fatalf("map order = %v %v %v", got.Object[0].Key, got.Object[1].Key, got.Object[2].Key)
	}
}

func TestNodeFromValueStructOrderAndTags(t *testing.T) {
	type S struct {
		Z      string `toon:"z"`
		A      int    `toon:"alpha"`
		M      string `toon:"-"` // skipped
		hidden string
		B      bool `toon:"beta"`
	}
	v := S{Z: "z", A: 1, M: "skip", hidden: "secret", B: true}
	got, err := NodeFromValue(v)
	if err != nil {
		t.Fatalf("NodeFromValue error: %v", err)
	}
	if got.Kind != toon.ObjectKind {
		t.Fatalf("kind = %v", got.Kind)
	}
	keys := []string{}
	for _, f := range got.Object {
		keys = append(keys, f.Key)
	}
	if strings.Join(keys, ",") != "z,alpha,beta" {
		t.Fatalf("struct keys = %v", keys)
	}
	for _, f := range got.Object {
		if f.Key == "beta" {
			if f.Value == nil || f.Value.Kind != toon.BoolKind || !f.Value.Bool {
				t.Fatalf("beta value = %#v", f.Value)
			}
		}
	}
}

type embed struct {
	Inner struct {
		X int `toon:"x"`
		Y int `toon:"y"`
	}
	Name string `toon:"name"`
}

func TestNodeFromValueEmbedStruct(t *testing.T) {
	v := embed{}
	v.Inner.X = 1
	v.Inner.Y = 2
	v.Name = "x"
	got, err := NodeFromValue(v)
	if err != nil {
		t.Fatalf("NodeFromValue error: %v", err)
	}
	if got.Kind != toon.ObjectKind {
		t.Fatalf("kind = %v", got.Kind)
	}
	if len(got.Object) != 2 {
		t.Fatalf("object length = %d", len(got.Object))
	}
	if got.Object[0].Key != "Inner" || got.Object[1].Key != "name" {
		t.Fatalf("keys = %v %v", got.Object[0].Key, got.Object[1].Key)
	}
	inner := got.Object[0].Value
	if inner.Kind != toon.ObjectKind || len(inner.Object) != 2 {
		t.Fatalf("inner = %#v", inner)
	}
	if inner.Object[0].Key != "x" || inner.Object[1].Key != "y" {
		t.Fatalf("inner keys = %v %v", inner.Object[0].Key, inner.Object[1].Key)
	}
}

type anonInner struct {
	X int `toon:"x"`
	Y int `toon:"y"`
}

type anonEmbed struct {
	anonInner
	Name string `toon:"name"`
}

func TestNodeFromValueAnonymousEmbed(t *testing.T) {
	v := anonEmbed{Name: "x"}
	v.X = 1
	v.Y = 2
	got, err := NodeFromValue(v)
	if err != nil {
		t.Fatalf("NodeFromValue error: %v", err)
	}
	keys := []string{}
	for _, f := range got.Object {
		keys = append(keys, f.Key)
	}
	if strings.Join(keys, ",") != "x,y,name" {
		t.Fatalf("anon embed keys = %v", keys)
	}
}

type listNode struct {
	Value int
	Next  *listNode
}

func TestNodeFromValueCycleDetection(t *testing.T) {
	a := &listNode{Value: 1}
	b := &listNode{Value: 2, Next: a}
	a.Next = b
	if _, err := NodeFromValue(a); toon.CodeOf(err) != toon.ErrCyclicValue {
		t.Fatalf("expected ErrCyclicValue, got %v", err)
	}
}

func TestNodeFromValueUnsupportedKinds(t *testing.T) {
	cases := map[string]any{
		"func":    func() {},
		"chan":    make(chan int),
		"complex": complex(1, 2),
		"int map": map[int]string{1: "a"},
	}
	for name, v := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := NodeFromValue(v)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
		})
	}
}

func TestNodeFromValueAllUnexported(t *testing.T) {
	got, err := NodeFromValue(struct{ x int }{x: 1})
	if err != nil {
		t.Fatalf("NodeFromValue error: %v", err)
	}
	if got.Kind != toon.ObjectKind || len(got.Object) != 0 {
		t.Fatalf("all-unexported should produce empty object, got %#v", got)
	}
}

type textMarshalable struct {
	S string
}

func (t textMarshalable) MarshalText() ([]byte, error) {
	return []byte("text:" + t.S), nil
}

func TestNodeFromValueTextMarshaler(t *testing.T) {
	got, err := NodeFromValue(textMarshalable{S: "hi"})
	if err != nil {
		t.Fatalf("NodeFromValue error: %v", err)
	}
	if got.Kind != toon.StringKind || got.String != "text:hi" {
		t.Fatalf("text marshaler = %#v", got)
	}
}

func TestNodeFromValueNonFiniteFloatLossless(t *testing.T) {
	if _, err := NodeFromValue(mathNaN()); toon.CodeOf(err) != toon.ErrUnsupportedKind {
		t.Fatalf("NaN lossless: want ErrUnsupportedKind, got %v", err)
	}
}

func TestNodeFromValueNonFiniteFloatUnsafe(t *testing.T) {
	got, err := NodeFromValue(mathNaN(), func(o *ValueOptions) { o.NumberMode = toon.NumberStringForUnsafe })
	if err != nil {
		t.Fatalf("NodeFromValue error: %v", err)
	}
	if got.Kind != toon.StringKind {
		t.Fatalf("NaN unsafe: kind = %v", got.Kind)
	}
}

func TestNodeFromValueResourceLimits(t *testing.T) {
	_, err := NodeFromValue([]int{1, 2, 3, 4, 5}, func(o *ValueOptions) { o.Limits = toon.ResourceLimits{MaxArrayLength: 2} })
	if toon.CodeOf(err) != toon.ErrResourceLimit {
		t.Fatalf("MaxArrayLength: got %v", err)
	}

	deep := makeNested(20)
	_, err = NodeFromValue(deep, func(o *ValueOptions) { o.Limits = toon.ResourceLimits{MaxDepth: 5} })
	if toon.CodeOf(err) != toon.ErrResourceLimit {
		t.Fatalf("MaxDepth: got %v", err)
	}

	big := strings.Repeat("x", 100)
	_, err = NodeFromValue(big, func(o *ValueOptions) { o.Limits = toon.ResourceLimits{MaxStringBytes: 10} })
	if toon.CodeOf(err) != toon.ErrResourceLimit {
		t.Fatalf("MaxStringBytes: got %v", err)
	}
}

func TestMarshalWritesTOON(t *testing.T) {
	var buf strings.Builder
	if err := Marshal(&buf, scalar{Name: "Ada", Age: 36}); err != nil {
		t.Fatalf("Marshal error: %v", err)
	}
	want := "Name: Ada\nAge: 36"
	if buf.String() != want {
		t.Fatalf("Marshal = %q, want %q", buf.String(), want)
	}
}

func TestNodeToValueScalars(t *testing.T) {
	var b bool
	if err := NodeToValue(&toon.Node{Kind: toon.BoolKind, Bool: true}, &b); err != nil {
		t.Fatalf("NodeToValue error: %v", err)
	}
	if !b {
		t.Fatalf("bool decode")
	}

	var s string
	if err := NodeToValue(&toon.Node{Kind: toon.StringKind, String: "hi"}, &s); err != nil {
		t.Fatalf("NodeToValue error: %v", err)
	}
	if s != "hi" {
		t.Fatalf("string = %q", s)
	}

	var i int
	if err := NodeToValue(&toon.Node{Kind: toon.NumberKind, Number: toon.Number{Raw: "42"}}, &i); err != nil {
		t.Fatalf("NodeToValue error: %v", err)
	}
	if i != 42 {
		t.Fatalf("int = %d", i)
	}

	var f float64
	if err := NodeToValue(&toon.Node{Kind: toon.NumberKind, Number: toon.Number{Raw: "3.14"}}, &f); err != nil {
		t.Fatalf("NodeToValue error: %v", err)
	}
	if f != 3.14 {
		t.Fatalf("float = %v", f)
	}
}

func TestNodeToValueNumberOverflow(t *testing.T) {
	var i int8
	err := NodeToValue(&toon.Node{Kind: toon.NumberKind, Number: toon.Number{Raw: "500"}}, &i)
	if toon.CodeOf(err) != toon.ErrUnmarshalType {
		t.Fatalf("overflow: want ErrUnmarshalType, got %v", err)
	}
}

func TestNodeToValueIntoStruct(t *testing.T) {
	type Tagged struct {
		Name string `toon:"name"`
		Age  int    `toon:"age"`
	}
	n := &toon.Node{Kind: toon.ObjectKind, Object: []toon.Field{
		{Key: "name", Value: &toon.Node{Kind: toon.StringKind, String: "Bob"}},
		{Key: "age", Value: &toon.Node{Kind: toon.NumberKind, Number: toon.Number{Raw: "30"}}},
	}}
	var s Tagged
	if err := NodeToValue(n, &s); err != nil {
		t.Fatalf("NodeToValue error: %v", err)
	}
	if s.Name != "Bob" || s.Age != 30 {
		t.Fatalf("struct = %+v", s)
	}
}

func TestNodeToValueIntoMap(t *testing.T) {
	n := &toon.Node{Kind: toon.ObjectKind, Object: []toon.Field{
		{Key: "a", Value: &toon.Node{Kind: toon.NumberKind, Number: toon.Number{Raw: "1"}}},
		{Key: "b", Value: &toon.Node{Kind: toon.NumberKind, Number: toon.Number{Raw: "2"}}},
	}}
	var m map[string]int
	if err := NodeToValue(n, &m); err != nil {
		t.Fatalf("NodeToValue error: %v", err)
	}
	if m["a"] != 1 || m["b"] != 2 || len(m) != 2 {
		t.Fatalf("map = %v", m)
	}
}

func TestNodeToValueIntoSlice(t *testing.T) {
	n := &toon.Node{Kind: toon.ArrayKind, Array: []*toon.Node{
		{Kind: toon.NumberKind, Number: toon.Number{Raw: "1"}},
		{Kind: toon.NumberKind, Number: toon.Number{Raw: "2"}},
		{Kind: toon.NumberKind, Number: toon.Number{Raw: "3"}},
	}}
	var s []int
	if err := NodeToValue(n, &s); err != nil {
		t.Fatalf("NodeToValue error: %v", err)
	}
	if !reflect.DeepEqual(s, []int{1, 2, 3}) {
		t.Fatalf("slice = %v", s)
	}
}

func TestNodeToValueIntoByteSlice(t *testing.T) {
	n := &toon.Node{Kind: toon.StringKind, String: "SGVsbG8="}
	var b []byte
	if err := NodeToValue(n, &b); err != nil {
		t.Fatalf("NodeToValue error: %v", err)
	}
	if string(b) != "Hello" {
		t.Fatalf("byte slice = %q", b)
	}
}

func TestNodeToValueTypeMismatch(t *testing.T) {
	var s string
	err := NodeToValue(&toon.Node{Kind: toon.BoolKind, Bool: true}, &s)
	if toon.CodeOf(err) != toon.ErrUnmarshalType {
		t.Fatalf("mismatch: want ErrUnmarshalType, got %v", err)
	}
}

func TestNodeToValueNonPointer(t *testing.T) {
	var s string
	if err := NodeToValue(&toon.Node{Kind: toon.StringKind, String: "x"}, s); toon.CodeOf(err) != toon.ErrNonPointerTarget {
		t.Fatalf("non-pointer: want ErrNonPointerTarget, got %v", err)
	}
}

func TestNodeToValueNilPointer(t *testing.T) {
	var sp *string
	if err := NodeToValue(&toon.Node{Kind: toon.StringKind, String: "x"}, sp); toon.CodeOf(err) != toon.ErrNonPointerTarget {
		t.Fatalf("nil pointer: want ErrNonPointerTarget, got %v", err)
	}
}

func TestNodeToValueNilInterface(t *testing.T) {
	if err := NodeToValue(&toon.Node{Kind: toon.StringKind, String: "x"}, nil); toon.CodeOf(err) != toon.ErrNonPointerTarget {
		t.Fatalf("nil iface: want ErrNonPointerTarget, got %v", err)
	}
}

type textUnmarshalable struct {
	S   string
	err error
}

func (t *textUnmarshalable) UnmarshalText(b []byte) error {
	if t.err != nil {
		return t.err
	}
	t.S = "got:" + string(b)
	return nil
}

func TestNodeToValueTextUnmarshaler(t *testing.T) {
	dst := &textUnmarshalable{}
	if err := NodeToValue(&toon.Node{Kind: toon.StringKind, String: "abc"}, dst); err != nil {
		t.Fatalf("NodeToValue error: %v", err)
	}
	if dst.S != "got:abc" {
		t.Fatalf("text unmarshaler = %q", dst.S)
	}
}

func TestNodeToValueTextUnmarshalerError(t *testing.T) {
	custom := &toon.Error{Code: toon.ErrInvalidInputFormat, Message: "boom"}
	dst := &textUnmarshalable{err: custom}
	err := NodeToValue(&toon.Node{Kind: toon.StringKind, String: "abc"}, dst)
	if !errors.Is(err, custom) {
		t.Fatalf("want errors.Is(custom), got %v", err)
	}
}

type noopMarshaler string

func (n noopMarshaler) MarshalText() ([]byte, error) { return []byte(n), nil }

func TestMarshalValueImplementsTextMarshaler(t *testing.T) {
	got, err := NodeFromValue(noopMarshaler("ok"))
	if err != nil {
		t.Fatalf("NodeFromValue error: %v", err)
	}
	if got.Kind != toon.StringKind || got.String != "ok" {
		t.Fatalf("got = %#v", got)
	}
}

func TestUnmarshalFull(t *testing.T) {
	type Tagged struct {
		Name string `toon:"name"`
		Age  int    `toon:"age"`
	}
	in := []byte("name: Ada\nage: 36\n")
	var s Tagged
	if err := Unmarshal(in, &s); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if s.Name != "Ada" || s.Age != 36 {
		t.Fatalf("decoded = %+v", s)
	}
}

func TestUnmarshalNodeBypassesDecode(t *testing.T) {
	type Tagged struct {
		Name string `toon:"name"`
	}
	n := &toon.Node{Kind: toon.ObjectKind, Object: []toon.Field{
		{Key: "name", Value: &toon.Node{Kind: toon.StringKind, String: "X"}},
	}}
	var s Tagged
	if err := UnmarshalNode(n, &s); err != nil {
		t.Fatalf("UnmarshalNode error: %v", err)
	}
	if s.Name != "X" {
		t.Fatalf("got = %+v", s)
	}
}

func TestUnmarshalNonPointer(t *testing.T) {
	var s scalar
	if err := Unmarshal([]byte("name: x"), s); toon.CodeOf(err) != toon.ErrNonPointerTarget {
		t.Fatalf("non-pointer: want ErrNonPointerTarget, got %v", err)
	}
}

func TestUnmarshalNilPointer(t *testing.T) {
	var sp *scalar
	if err := Unmarshal([]byte("name: x"), sp); toon.CodeOf(err) != toon.ErrNonPointerTarget {
		t.Fatalf("nil pointer: want ErrNonPointerTarget, got %v", err)
	}
}

func TestUnmarshalDecodeErrorPropagates(t *testing.T) {
	var s scalar
	err := Unmarshal([]byte("a:\n b: 1"), &s)
	if toon.CodeOf(err) != toon.ErrInvalidIndent {
		t.Fatalf("decode error propagation: want ErrInvalidIndent, got %v", err)
	}
}

func TestRoundTripEquality(t *testing.T) {
	type Deep struct {
		Numbers []int             `toon:"numbers"`
		Labels  map[string]string `toon:"labels"`
		Active  bool              `toon:"active"`
		Name    string            `toon:"name"`
	}
	original := Deep{
		Numbers: []int{1, 2, 3},
		Labels:  map[string]string{"x": "y", "a": "b"},
		Active:  true,
		Name:    "ada",
	}
	var got Deep
	if err := roundTrip(original, &got); err != nil {
		t.Fatalf("round trip error: %v", err)
	}
	if !reflect.DeepEqual(original, got) {
		t.Fatalf("round trip mismatch\noriginal=%+v\ngot=%+v", original, got)
	}
}

func roundTrip(in, out any) error {
	var buf strings.Builder
	if err := Marshal(&buf, in); err != nil {
		return err
	}
	return Unmarshal([]byte(buf.String()), out)
}

func TestEncodeArrayLengthLimit(t *testing.T) {
	_, err := NodeFromValue([]int{1, 2, 3}, func(o *ValueOptions) { o.Limits = toon.ResourceLimits{MaxArrayLength: 1} })
	if toon.CodeOf(err) != toon.ErrResourceLimit {
		t.Fatalf("array limit: got %v", err)
	}
}

func TestEncodeDepthLimit(t *testing.T) {
	_, err := NodeFromValue(makeNested(10), func(o *ValueOptions) { o.Limits = toon.ResourceLimits{MaxDepth: 3} })
	if toon.CodeOf(err) != toon.ErrResourceLimit {
		t.Fatalf("depth limit: got %v", err)
	}
}

func TestEncodeNodeCountLimit(t *testing.T) {
	_, err := NodeFromValue(map[string]int{"a": 1, "b": 2, "c": 3}, func(o *ValueOptions) { o.Limits = toon.ResourceLimits{MaxNodes: 1} })
	if toon.CodeOf(err) != toon.ErrResourceLimit {
		t.Fatalf("node limit: got %v", err)
	}
}

func TestMapKeySortStable(t *testing.T) {
	in := map[string]int{"c": 1, "a": 2, "b": 3}
	for trial := 0; trial < 5; trial++ {
		got, err := NodeFromValue(in)
		if err != nil {
			t.Fatalf("trial %d: %v", trial, err)
		}
		keys := []string{got.Object[0].Key, got.Object[1].Key, got.Object[2].Key}
		if strings.Join(keys, ",") != "a,b,c" {
			t.Fatalf("trial %d: keys = %v", trial, keys)
		}
	}
}

func TestMarshalNil(t *testing.T) {
	var buf strings.Builder
	if err := Marshal(&buf, nil); err != nil {
		t.Fatalf("Marshal nil error: %v", err)
	}
	if buf.String() != "null" {
		t.Fatalf("nil = %q", buf.String())
	}
}

func TestMarshalEmptyStruct(t *testing.T) {
	type Empty struct{}
	var buf strings.Builder
	if err := Marshal(&buf, Empty{}); err != nil {
		t.Fatalf("Marshal empty struct error: %v", err)
	}
	if buf.String() != "" {
		t.Fatalf("empty struct = %q", buf.String())
	}
}

func TestEncodeConformanceWithNode(t *testing.T) {
	type S struct {
		ID    int      `toon:"id"`
		Name  string   `toon:"name"`
		Tags  []string `toon:"tags"`
		Score float64  `toon:"score"`
	}
	v := S{ID: 7, Name: "Ada", Tags: []string{"a", "b"}, Score: 9.5}

	n, err := NodeFromValue(v)
	if err != nil {
		t.Fatalf("NodeFromValue: %v", err)
	}
	fromNode, err := toon.Encode(n)
	if err != nil {
		t.Fatalf("Encode from node: %v", err)
	}
	fromValue, err := toon.Encode(n)
	if err != nil {
		t.Fatalf("Encode from value: %v", err)
	}
	if string(fromNode) != string(fromValue) {
		t.Fatalf("conformance broken: %q vs %q", fromNode, fromValue)
	}
}

// helpers

func makeNested(depth int) any {
	if depth == 0 {
		return 1
	}
	return []any{makeNested(depth - 1)}
}

func mathNaN() float64 {
	var z float64
	return z / z
}

var _ encoding.TextMarshaler = textMarshalable{}
var _ encoding.TextUnmarshaler = (*textUnmarshalable)(nil)
