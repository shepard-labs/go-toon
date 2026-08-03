package main

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/shepard-labs/go-toon/formats"
	"github.com/shepard-labs/go-toon/toon"
)

func TestToolsListIncludesExpandedTools(t *testing.T) {
	got := tools()
	names := make([]string, 0, len(got))
	for _, tool := range got {
		names = append(names, tool.Name)
	}
	for _, want := range []string{
		"toon_validate",
		"toon_to_json",
		"json_to_toon",
		"yaml_to_toon",
		"csv_to_toon",
		"xml_to_toon",
		"toon_examples",
		"toon_error_explain",
		"toon_spec_lookup",
	} {
		if !slices.Contains(names, want) {
			t.Fatalf("tools list missing %q; got %v", want, names)
		}
	}
}

func TestToonValidateTool(t *testing.T) {
	res, err := handleToonValidate(mustJSON(t, map[string]any{"input": "id: 1"}))
	if err != nil {
		t.Fatalf("handleToonValidate returned error: %v", err)
	}
	var payload map[string]any
	decodeToolPayload(t, res, &payload)
	if payload["valid"] != true {
		t.Fatalf("expected valid=true, got %#v", payload)
	}
}

func TestToonValidateToolError(t *testing.T) {
	res, err := handleToonValidate(mustJSON(t, map[string]any{"input": "tags[2]: red,green,blue"}))
	if err != nil {
		t.Fatalf("handleToonValidate returned error: %v", err)
	}
	var payload struct {
		Valid bool `json:"valid"`
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	decodeToolPayload(t, res, &payload)
	if payload.Valid {
		t.Fatalf("expected invalid payload")
	}
	if payload.Error.Code != "array_count_mismatch" {
		t.Fatalf("expected array_count_mismatch, got %q", payload.Error.Code)
	}
}

func TestJSONToTOONTool(t *testing.T) {
	input := `{"user":{"id":1,"name":"Ada"},"tags":["go","toon"]}`
	res, err := handleJSONToTOON(mustJSON(t, map[string]any{"input": input}))
	if err != nil {
		t.Fatalf("handleJSONToTOON returned error: %v", err)
	}
	payload := decodeTOONPayload(t, res)
	if !payload.Valid {
		t.Fatalf("expected valid payload")
	}
	n, err := formats.FromJSON(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	wantBytes, err := toon.Encode(n)
	if err != nil {
		t.Fatal(err)
	}
	want := string(wantBytes)
	if payload.TOON != want {
		t.Fatalf("TOON mismatch\nwant: %q\n got: %q", want, payload.TOON)
	}
}

func TestToonToJSONTool(t *testing.T) {
	res, err := handleToonToJSON(mustJSON(t, map[string]any{"input": "id: 1\nname: Ada", "indent": ""}))
	if err != nil {
		t.Fatalf("handleToonToJSON returned error: %v", err)
	}
	var payload struct {
		Valid bool   `json:"valid"`
		JSON  string `json:"json"`
	}
	decodeToolPayload(t, res, &payload)
	if !payload.Valid {
		t.Fatalf("expected valid payload")
	}
	n, err := toon.Decode([]byte("id: 1\nname: Ada"))
	if err != nil {
		t.Fatal(err)
	}
	var expected bytes.Buffer
	if err := formats.ToJSON(&expected, n, func(o *formats.JSONOutputOptions) { o.Indent = "  " }); err != nil {
		t.Fatal(err)
	}
	if payload.JSON != expected.String() {
		t.Fatalf("unexpected JSON: %q", payload.JSON)
	}
}

func TestYAMLToTOONTool(t *testing.T) {
	input := "user:\n  id: 1\n  name: Ada\ntags:\n  - go\n  - toon\n"
	res, err := handleYAMLToTOON(mustJSON(t, map[string]any{"input": input}))
	if err != nil {
		t.Fatalf("handleYAMLToTOON returned error: %v", err)
	}
	payload := decodeTOONPayload(t, res)
	if !payload.Valid {
		t.Fatalf("expected valid payload")
	}
	n, err := formats.FromYAML(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	wantBytes, err := toon.Encode(n)
	if err != nil {
		t.Fatal(err)
	}
	want := string(wantBytes)
	if payload.TOON != want {
		t.Fatalf("TOON mismatch\nwant: %q\n got: %q", want, payload.TOON)
	}
}

func TestCSVToTOONTool(t *testing.T) {
	input := "sku,qty\nA1,2\nB2,1\n"
	res, err := handleCSVToTOON(mustJSON(t, map[string]any{"input": input, "root_key": "items"}))
	if err != nil {
		t.Fatalf("handleCSVToTOON returned error: %v", err)
	}
	payload := decodeTOONPayload(t, res)
	if !payload.Valid {
		t.Fatalf("expected valid payload")
	}
	n, err := formats.FromCSV(strings.NewReader(input), func(o *formats.CSVOptions) {
		o.HeaderMode = formats.CSVHeaderPresent
		o.Delimiter = ','
		o.InferTypes = true
	})
	if err != nil {
		t.Fatal(err)
	}
	n = &toon.Node{Kind: toon.ObjectKind, Object: []toon.Field{{Key: "items", Value: n}}}
	wantBytes, err := toon.Encode(n)
	if err != nil {
		t.Fatal(err)
	}
	want := string(wantBytes)
	if payload.TOON != want {
		t.Fatalf("TOON mismatch\nwant: %q\n got: %q", want, payload.TOON)
	}
}

func TestXMLToTOONTool(t *testing.T) {
	input := `<item sku="A1"><qty>2</qty></item>`
	res, err := handleXMLToTOON(mustJSON(t, map[string]any{"input": input}))
	if err != nil {
		t.Fatalf("handleXMLToTOON returned error: %v", err)
	}
	payload := decodeTOONPayload(t, res)
	if !payload.Valid {
		t.Fatalf("expected valid payload")
	}
	n, err := formats.FromXML(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	wantBytes, err := toon.Encode(n)
	if err != nil {
		t.Fatal(err)
	}
	want := string(wantBytes)
	if payload.TOON != want {
		t.Fatalf("TOON mismatch\nwant: %q\n got: %q", want, payload.TOON)
	}
}

func TestConversionToolOptionParity(t *testing.T) {
	res, err := handleJSONToTOON(mustJSON(t, map[string]any{
		"input":          `{"a":{"b":{"c":1}},"tags":["x","y"]}`,
		"indent":         4,
		"delimiter":      "pipe",
		"length_markers": true,
		"key_folding":    "safe",
		"flatten_depth":  3,
	}))
	if err != nil {
		t.Fatalf("handleJSONToTOON returned error: %v", err)
	}
	payload := decodeTOONPayload(t, res)
	n, err := formats.FromJSON(strings.NewReader(`{"a":{"b":{"c":1}},"tags":["x","y"]}`))
	if err != nil {
		t.Fatal(err)
	}
	wantBytes, err := toon.Encode(n, func(o *toon.EncodeOptions) {
		o.IndentSize = 4
		o.Delimiter = toon.Pipe
		o.IncludeLengthMarkers = true
		o.KeyFolding = toon.KeyFoldingSafe
		o.FlattenDepth = 3
	})
	if err != nil {
		t.Fatal(err)
	}
	if payload.TOON != string(wantBytes) {
		t.Fatalf("option parity mismatch\nwant: %q\n got: %q", string(wantBytes), payload.TOON)
	}
}

func TestInvalidEnumOptionsAreRejected(t *testing.T) {
	if _, err := handleYAMLToTOON(mustJSON(t, map[string]any{"input": "id: 1", "documents": "merge"})); err == nil {
		t.Fatalf("expected invalid YAML documents option to fail")
	}
	if _, err := handleYAMLToTOON(mustJSON(t, map[string]any{"input": "id: 1", "scalars": "json"})); err == nil {
		t.Fatalf("expected invalid YAML scalars option to fail")
	}
	if _, err := handleCSVToTOON(mustJSON(t, map[string]any{"input": "a\n1", "header": "auto"})); err == nil {
		t.Fatalf("expected invalid CSV header option to fail")
	}
}

func TestEmbeddedExamplesValidate(t *testing.T) {
	for topic, example := range toonExampleTopics {
		toonText, ok := example["toon"].(string)
		if !ok || toonText == "" {
			t.Fatalf("example %q missing toon text", topic)
		}
		if err := toon.Validate([]byte(toonText)); err != nil {
			t.Fatalf("example %q is invalid: %v", topic, err)
		}
	}
}

func TestSpecExamplesValidate(t *testing.T) {
	for topic, entry := range toonSpecTopics {
		toonText, ok := entry["example"].(string)
		if !ok || toonText == "" {
			continue
		}
		if err := toon.Validate([]byte(toonText)); err != nil {
			t.Fatalf("spec example %q is invalid: %v", topic, err)
		}
	}
}

func TestToonExamplesTool(t *testing.T) {
	res, err := handleToonExamples(mustJSON(t, map[string]any{"topic": "tabular-array"}))
	if err != nil {
		t.Fatalf("handleToonExamples returned error: %v", err)
	}
	var payload struct {
		Topic string `json:"topic"`
		TOON  string `json:"toon"`
	}
	decodeToolPayload(t, res, &payload)
	if payload.Topic != "tabular_array" {
		t.Fatalf("expected tabular_array topic, got %q", payload.Topic)
	}
	if payload.TOON == "" {
		t.Fatalf("expected TOON example")
	}
}

func TestToonErrorExplainTool(t *testing.T) {
	res, err := handleToonErrorExplain(mustJSON(t, map[string]any{"code": "array-count-mismatch"}))
	if err != nil {
		t.Fatalf("handleToonErrorExplain returned error: %v", err)
	}
	var payload struct {
		Code         string `json:"code"`
		FixedExample string `json:"fixed_example"`
	}
	decodeToolPayload(t, res, &payload)
	if payload.Code != "array_count_mismatch" {
		t.Fatalf("expected array_count_mismatch, got %q", payload.Code)
	}
	if payload.FixedExample == "" {
		t.Fatalf("expected fixed example")
	}
}

func TestToonSpecLookupTool(t *testing.T) {
	res, err := handleToonSpecLookup(mustJSON(t, map[string]any{"topic": "path-expansion"}))
	if err != nil {
		t.Fatalf("handleToonSpecLookup returned error: %v", err)
	}
	var payload struct {
		Topic   string `json:"topic"`
		Summary string `json:"summary"`
	}
	decodeToolPayload(t, res, &payload)
	if payload.Topic != "path_expansion" {
		t.Fatalf("expected path_expansion, got %q", payload.Topic)
	}
	if payload.Summary == "" {
		t.Fatalf("expected summary")
	}
}

func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func decodeToolPayload(t *testing.T, res toolResult, v any) {
	t.Helper()
	if len(res.Content) != 1 {
		t.Fatalf("expected one content block, got %d", len(res.Content))
	}
	if err := json.Unmarshal([]byte(res.Content[0].Text), v); err != nil {
		t.Fatalf("failed to decode tool payload: %v\npayload: %s", err, res.Content[0].Text)
	}
}

func decodeTOONPayload(t *testing.T, res toolResult) struct {
	Valid bool   `json:"valid"`
	TOON  string `json:"toon"`
} {
	t.Helper()
	var payload struct {
		Valid bool   `json:"valid"`
		TOON  string `json:"toon"`
	}
	decodeToolPayload(t, res, &payload)
	return payload
}
