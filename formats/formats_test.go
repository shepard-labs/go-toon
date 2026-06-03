package formats

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/shepard-labs/go-toon/toon"
)

func TestJSONInputOutput(t *testing.T) {
	n, err := FromJSON(strings.NewReader(`{"z":1,"a":{"b":2},"arr":[true,null,"x"],"big":12345678901234567890}`))
	if err != nil {
		t.Fatalf("FromJSON error: %v", err)
	}
	want := obj(
		field("z", num("1")),
		field("a", obj(field("b", num("2")))),
		field("arr", arr(boolNode(true), nullNode(), str("x"))),
		field("big", num("12345678901234567890")),
	)
	assertNode(t, n, want)

	if _, err := FromJSON(strings.NewReader(`{"a":1,"a":2}`)); toon.CodeOf(err) != toon.ErrDuplicateKey {
		t.Fatalf("duplicate CodeOf = %q, err %v", toon.CodeOf(err), err)
	}
	n, err = FromJSON(strings.NewReader(`{"a":1,"a":2}`), func(o *JSONOptions) { o.AllowDuplicateKeys = true })
	if err != nil {
		t.Fatalf("duplicate allowed error: %v", err)
	}
	assertNode(t, n, obj(field("a", num("2"))))
	if _, err := FromJSON(strings.NewReader(`{"a":`)); toon.CodeOf(err) != toon.ErrInvalidInputFormat {
		t.Fatalf("invalid JSON CodeOf = %q", toon.CodeOf(err))
	}
	if _, err := FromJSON(strings.NewReader(`{"a":"abcd"}`), func(o *JSONOptions) { o.Limits.MaxStringBytes = 3 }); toon.CodeOf(err) != toon.ErrResourceLimit {
		t.Fatalf("JSON limit CodeOf = %q", toon.CodeOf(err))
	}

	var out bytes.Buffer
	if err := ToJSON(&out, want, func(o *JSONOutputOptions) { o.Indent = "  " }); err != nil {
		t.Fatalf("ToJSON error: %v", err)
	}
	if got := out.String(); got != "{\n  \"z\": 1,\n  \"a\": {\n    \"b\": 2\n  },\n  \"arr\": [\n    true,\n    null,\n    \"x\"\n  ],\n  \"big\": 12345678901234567890\n}" {
		t.Fatalf("ToJSON = %s", got)
	}
}

func TestJSONScannerEdges(t *testing.T) {
	n, err := FromJSON(strings.NewReader(`{"s":"a\n\u003c\uD83D\uDE00","n":-12.30e+4}`))
	if err != nil {
		t.Fatalf("FromJSON escaped string error: %v", err)
	}
	assertNode(t, n, obj(field("s", str("a\n<😀")), field("n", num("-12.30e+4"))))

	for _, input := range []string{
		`{"n":01}`,
		`{"n":1.}`,
		`{"n":1e}`,
		`{"a":1} {"b":2}`,
		`{"s":"
"}`,
	} {
		if _, err := FromJSON(strings.NewReader(input)); toon.CodeOf(err) != toon.ErrInvalidInputFormat {
			t.Fatalf("FromJSON(%q) CodeOf = %q, err %v", input, toon.CodeOf(err), err)
		}
	}
}

func TestYAMLInput(t *testing.T) {
	n, err := FromYAML(strings.NewReader("z: 1\na: true\nts: 2024-01-02\n? [x, y]\n: v\n"))
	if err != nil {
		t.Fatalf("FromYAML error: %v", err)
	}
	assertNode(t, n, obj(field("z", num("1")), field("a", boolNode(true)), field("ts", str("2024-01-02")), field("2", str("v"))))

	if _, err := FromYAML(strings.NewReader("a: 1\n---\nb: 2\n")); toon.CodeOf(err) != toon.ErrInvalidInputFormat {
		t.Fatalf("multi-doc CodeOf = %q", toon.CodeOf(err))
	}
	n, err = FromYAML(strings.NewReader("a: 1\n---\nb: 2\n"), func(o *YAMLOptions) { o.Documents = YAMLDocumentsArray })
	if err != nil {
		t.Fatalf("multi-doc array error: %v", err)
	}
	assertNode(t, n, arr(obj(field("a", num("1"))), obj(field("b", num("2")))))

	n, err = FromYAML(strings.NewReader("a: 1\n"), func(o *YAMLOptions) { o.Scalars = YAMLScalarsString })
	if err != nil {
		t.Fatalf("string scalar YAML error: %v", err)
	}
	assertNode(t, n, obj(field("a", str("1"))))

	n, err = FromYAML(strings.NewReader("base: &base\n  a: 1\nobj:\n  <<: *base\n  b: 2\n"))
	if err != nil {
		t.Fatalf("merge YAML error: %v", err)
	}
	assertNode(t, n, obj(field("base", obj(field("a", num("1")))), field("obj", obj(field("a", num("1")), field("b", num("2"))))))
	if _, err := FromYAML(strings.NewReader("a: &a [*a]\n")); toon.CodeOf(err) != toon.ErrInvalidInputFormat {
		t.Fatalf("alias cycle CodeOf = %q, err %v", toon.CodeOf(err), err)
	}
}

func TestCSVInput(t *testing.T) {
	if _, err := FromCSV(strings.NewReader("a,b\n1,2\n")); toon.CodeOf(err) != toon.ErrUnsupportedFeature {
		t.Fatalf("default CSV CodeOf = %q", toon.CodeOf(err))
	}
	n, err := FromCSV(strings.NewReader("id,name,zip,empty\n1,Ada,05,\n2,Bob,10,x\n"), func(o *CSVOptions) {
		o.HeaderMode = CSVHeaderPresent
		o.InferTypes = true
	})
	if err != nil {
		t.Fatalf("FromCSV header error: %v", err)
	}
	assertNode(t, n, arr(
		obj(field("id", num("1")), field("name", str("Ada")), field("zip", str("05")), field("empty", str(""))),
		obj(field("id", num("2")), field("name", str("Bob")), field("zip", num("10")), field("empty", str("x"))),
	))
	n, err = FromCSV(strings.NewReader("1|true\n2|false\n"), func(o *CSVOptions) {
		o.HeaderMode = CSVHeaderAbsent
		o.Delimiter = '|'
		o.InferTypes = false
	})
	if err != nil {
		t.Fatalf("FromCSV absent error: %v", err)
	}
	assertNode(t, n, arr(obj(field("field1", str("1")), field("field2", str("true"))), obj(field("field1", str("2")), field("field2", str("false")))))
	if _, err := FromCSV(strings.NewReader("a,a\n1,2\n"), func(o *CSVOptions) { o.HeaderMode = CSVHeaderPresent }); toon.CodeOf(err) != toon.ErrDuplicateKey {
		t.Fatalf("duplicate header CodeOf = %q", toon.CodeOf(err))
	}
	if _, err := FromCSV(strings.NewReader("a,b\n1\n"), func(o *CSVOptions) { o.HeaderMode = CSVHeaderPresent }); toon.CodeOf(err) != toon.ErrInvalidInputFormat {
		t.Fatalf("ragged CodeOf = %q", toon.CodeOf(err))
	}
}

func TestXMLInput(t *testing.T) {
	assertFromXML(t, `<name>Ada</name>`, obj(field("name", str("Ada"))))
	assertFromXML(t, `<user id="1">Ada</user>`, obj(field("user", obj(field("@id", num("1")), field("#text", str("Ada"))))))
	assertFromXML(t, `<root><item>1</item><item>05</item></root>`, obj(field("root", obj(field("item", arr(num("1"), str("05")))))))
	assertFromXML(t, `<root>  <a>1</a><!--x--><?pi x?></root>`, obj(field("root", obj(field("a", num("1"))))))
	assertFromXML(t, `<ns:root xmlns:ns="urn:x"><ns:a>1</ns:a></ns:root>`, obj(field("root", obj(field("a", num("1"))))))
	n, err := FromXML(strings.NewReader(`<p>Hello <b>world</b>!</p>`), func(o *XMLOptions) { o.MixedContent = XMLMixedContentPreserve })
	if err != nil {
		t.Fatalf("XML preserve error: %v", err)
	}
	assertNode(t, n, obj(field("p", arr(obj(field("#text", str("Hello"))), obj(field("b", str("world"))), obj(field("#text", str("!")))))))
	n, err = FromXML(strings.NewReader(`<a>1</a>`), func(o *XMLOptions) { o.InferTypes = false })
	if err != nil {
		t.Fatalf("XML no infer error: %v", err)
	}
	assertNode(t, n, obj(field("a", str("1"))))
	if _, err := FromXML(strings.NewReader(`<!DOCTYPE x><x/>`)); toon.CodeOf(err) != toon.ErrInvalidInputFormat {
		t.Fatalf("DTD CodeOf = %q", toon.CodeOf(err))
	}
}

func TestHighLevelConversions(t *testing.T) {
	var toonOut bytes.Buffer
	if err := JSONToTOON(strings.NewReader(`{"a":1}`), &toonOut, JSONToTOONOptions{}); err != nil {
		t.Fatalf("JSONToTOON error: %v", err)
	}
	if toonOut.String() != "a: 1" {
		t.Fatalf("JSONToTOON = %q", toonOut.String())
	}
	var jsonOut bytes.Buffer
	if err := TOONToJSON(strings.NewReader("a: 1"), &jsonOut, TOONToJSONOptions{JSON: JSONOutputOptions{Indent: "  "}}); err != nil {
		t.Fatalf("TOONToJSON error: %v", err)
	}
	if jsonOut.String() != "{\n  \"a\": 1\n}" {
		t.Fatalf("TOONToJSON = %q", jsonOut.String())
	}

	var out bytes.Buffer
	if err := YAMLToTOON(strings.NewReader("a: 1"), &out, YAMLToTOONOptions{}); err != nil || out.String() != "a: 1" {
		t.Fatalf("YAMLToTOON = %q, %v", out.String(), err)
	}
	out.Reset()
	if err := CSVToTOON(strings.NewReader("a\n1\n"), &out, CSVToTOONOptions{CSV: CSVOptions{HeaderMode: CSVHeaderPresent, InferTypes: true}}); err != nil || out.String() != "[1]{a}:\n  1" {
		t.Fatalf("CSVToTOON = %q, %v", out.String(), err)
	}
	out.Reset()
	if err := XMLToTOON(strings.NewReader("<a>1</a>"), &out, XMLToTOONOptions{}); err != nil || out.String() != "a: 1" {
		t.Fatalf("XMLToTOON = %q, %v", out.String(), err)
	}
}

func assertFromXML(t *testing.T, input string, want *toon.Node) {
	t.Helper()
	n, err := FromXML(strings.NewReader(input))
	if err != nil {
		t.Fatalf("FromXML(%q) error: %v", input, err)
	}
	assertNode(t, n, want)
}

func assertNode(t *testing.T, got, want *toon.Node) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func obj(fields ...toon.Field) *toon.Node { return &toon.Node{Kind: toon.ObjectKind, Object: fields} }
func arr(items ...*toon.Node) *toon.Node  { return &toon.Node{Kind: toon.ArrayKind, Array: items} }
func field(key string, value *toon.Node) toon.Field {
	return toon.Field{Key: key, Value: value}
}
func str(s string) *toon.Node    { return &toon.Node{Kind: toon.StringKind, String: s} }
func num(s string) *toon.Node    { return &toon.Node{Kind: toon.NumberKind, Number: toon.Number{Raw: s}} }
func boolNode(v bool) *toon.Node { return &toon.Node{Kind: toon.BoolKind, Bool: v} }
func nullNode() *toon.Node       { return &toon.Node{Kind: toon.NullKind} }
