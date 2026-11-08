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
		"input":     `{"a":{"b":{"c":1}},"tags":["x","y"]}`,
		"indent":    4,
		"delimiter": "pipe",
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
	})
	if err != nil {
		t.Fatal(err)
	}
	if payload.TOON != string(wantBytes) {
		t.Fatalf("option parity mismatch\nwant: %q\n got: %q", string(wantBytes), payload.TOON)
	}
}

func TestRemovedOptionsAreNotAdvertised(t *testing.T) {
	removed := []string{"length_markers", "key_folding", "flatten_depth", "expand_paths"}
	for _, tl := range tools() {
		props, _ := tl.InputSchema["properties"].(map[string]any)
		for _, name := range removed {
			if _, ok := props[name]; ok {
				t.Fatalf("tool %s still advertises removed option %q", tl.Name, name)
			}
		}
	}
	for topic := range toonSpecTopics {
		if topic == "key_folding" || topic == "path_expansion" {
			t.Fatalf("spec topic %q should be removed", topic)
		}
	}
}

func TestConversionOutputUsesV4Layouts(t *testing.T) {
	res, err := handleJSONToTOON(mustJSON(t, map[string]any{
		"input": `{"servers":{"a":{"h":"x","p":1},"b":{"h":"y","p":2}},"rows":[{"id":1,"geo":{"lat":5,"lon":6}},{"id":2,"geo":{"lat":7,"lon":8}}]}`,
	}))
	if err != nil {
		t.Fatalf("handleJSONToTOON returned error: %v", err)
	}
	payload := decodeTOONPayload(t, res)
	want := "servers[2:]{h,p}:\n  a: x,1\n  b: y,2\nrows[2]{id,geo{lat,lon}}:\n  1,5,6\n  2,7,8"
	if payload.TOON != want {
		t.Fatalf("TOON mismatch\nwant: %q\n got: %q", want, payload.TOON)
	}
	if err := toon.Validate([]byte(payload.TOON)); err != nil {
		t.Fatalf("converted output does not validate: %v", err)
	}
}

func TestToonValidateAcceptsCommentsAndRejectsLengthMarker(t *testing.T) {
	res, err := handleToonValidate(mustJSON(t, map[string]any{"input": "# note\na: 1"}))
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Valid bool `json:"valid"`
	}
	decodeToolPayload(t, res, &payload)
	if !payload.Valid {
		t.Fatalf("comment line should validate")
	}
	res, err = handleToonValidate(mustJSON(t, map[string]any{"input": "tags[#2]: a,b"}))
	if err != nil {
		t.Fatal(err)
	}
	decodeToolPayload(t, res, &payload)
	if payload.Valid {
		t.Fatalf("length marker syntax should be rejected")
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

func TestErrorExplanationExamples(t *testing.T) {
	for code, entry := range toonErrorExplanations {
		if invalid, ok := entry["invalid_example"].(string); ok {
			err := toon.Validate([]byte(invalid))
			if err == nil {
				t.Fatalf("%s: invalid_example %q validates", code, invalid)
			}
			if got := string(toon.CodeOf(err)); got != code {
				t.Fatalf("%s: invalid_example %q fails with %s", code, invalid, got)
			}
		}
		if fixed, ok := entry["fixed_example"].(string); ok {
			if err := toon.Validate([]byte(fixed)); err != nil {
				t.Fatalf("%s: fixed_example %q is invalid: %v", code, fixed, err)
			}
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

func TestExampleAndSpecTopicsCoverV4Syntax(t *testing.T) {
	for _, topic := range []string{"keyed_tabular", "nested_field_groups", "comments"} {
		if _, ok := toonExampleTopics[topic]; !ok {
			t.Fatalf("missing example topic %q", topic)
		}
		if _, ok := toonSpecTopics[topic]; !ok {
			t.Fatalf("missing spec topic %q", topic)
		}
	}
	for _, topic := range []string{"folded_keys"} {
		if _, ok := toonExampleTopics[topic]; ok {
			t.Fatalf("example topic %q should be removed", topic)
		}
	}
	if _, ok := toonErrorExplanations["invalid_utf8"]; !ok {
		t.Fatalf("missing invalid_utf8 explanation")
	}
	if _, ok := toonErrorExplanations["path_expansion_conflict"]; ok {
		t.Fatalf("path_expansion_conflict explanation should be removed")
	}
	overview, _ := toonSpecTopics["overview"]["summary"].(string)
	if !strings.Contains(overview, "v4.4") {
		t.Fatalf("overview should mention v4.4: %q", overview)
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
	res, err := handleToonSpecLookup(mustJSON(t, map[string]any{"topic": "keyed-tabular"}))
	if err != nil {
		t.Fatalf("handleToonSpecLookup returned error: %v", err)
	}
	var payload struct {
		Topic   string `json:"topic"`
		Summary string `json:"summary"`
	}
	decodeToolPayload(t, res, &payload)
	if payload.Topic != "keyed_tabular" {
		t.Fatalf("expected keyed_tabular, got %q", payload.Topic)
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
