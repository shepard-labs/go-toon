package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/shepard-labs/go-toon/formats"
	"github.com/shepard-labs/go-toon/toon"
)

const protocolVersion = "2024-11-05"

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

type toolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

type textContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type toolResult struct {
	Content []textContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

type toonValidateArgs struct {
	Input       string `json:"input"`
	Strict      *bool  `json:"strict,omitempty"`
	ExpandPaths string `json:"expand_paths,omitempty"`
}

type toonToJSONArgs struct {
	Input       string `json:"input"`
	Strict      *bool  `json:"strict,omitempty"`
	ExpandPaths string `json:"expand_paths,omitempty"`
	Indent      string `json:"indent,omitempty"`
}

type jsonToTOONArgs struct {
	Input              string `json:"input"`
	Indent             int    `json:"indent,omitempty"`
	Delimiter          string `json:"delimiter,omitempty"`
	LengthMarkers      bool   `json:"length_markers,omitempty"`
	KeyFolding         string `json:"key_folding,omitempty"`
	FlattenDepth       int    `json:"flatten_depth,omitempty"`
	AllowDuplicateKeys bool   `json:"allow_duplicate_keys,omitempty"`
}

type yamlToTOONArgs struct {
	Input         string `json:"input"`
	Documents     string `json:"documents,omitempty"`
	Scalars       string `json:"scalars,omitempty"`
	Indent        int    `json:"indent,omitempty"`
	Delimiter     string `json:"delimiter,omitempty"`
	LengthMarkers bool   `json:"length_markers,omitempty"`
	KeyFolding    string `json:"key_folding,omitempty"`
	FlattenDepth  int    `json:"flatten_depth,omitempty"`
}

type csvToTOONArgs struct {
	Input           string `json:"input"`
	Header          string `json:"header,omitempty"`
	CSVDelimiter    string `json:"csv_delimiter,omitempty"`
	InferTypes      *bool  `json:"infer_types,omitempty"`
	AllowRaggedRows bool   `json:"allow_ragged_rows,omitempty"`
	RootKey         string `json:"root_key,omitempty"`
	Indent          int    `json:"indent,omitempty"`
	Delimiter       string `json:"delimiter,omitempty"`
	LengthMarkers   bool   `json:"length_markers,omitempty"`
	KeyFolding      string `json:"key_folding,omitempty"`
	FlattenDepth    int    `json:"flatten_depth,omitempty"`
}

type xmlToTOONArgs struct {
	Input              string `json:"input"`
	AttributePrefix    string `json:"attribute_prefix,omitempty"`
	TextKey            string `json:"text_key,omitempty"`
	InferTypes         *bool  `json:"infer_types,omitempty"`
	TrimWhitespaceText *bool  `json:"trim_whitespace_text,omitempty"`
	MixedContent       string `json:"mixed_content,omitempty"`
	Namespaces         string `json:"namespaces,omitempty"`
	Indent             int    `json:"indent,omitempty"`
	Delimiter          string `json:"delimiter,omitempty"`
	LengthMarkers      bool   `json:"length_markers,omitempty"`
	KeyFolding         string `json:"key_folding,omitempty"`
	FlattenDepth       int    `json:"flatten_depth,omitempty"`
}

type toonExamplesArgs struct {
	Topic string `json:"topic,omitempty"`
}

type toonErrorExplainArgs struct {
	Code string `json:"code"`
}

type toonSpecLookupArgs struct {
	Topic string `json:"topic"`
}

var toonExampleTopics = map[string]map[string]any{
	"object": {
		"topic":       "object",
		"description": "Ordered object fields use key: value lines.",
		"toon":        "id: 1\nname: Ada\nactive: true",
	},
	"nested_object": {
		"topic":       "nested_object",
		"description": "Nested objects use indentation. The default go-toon indent is two spaces.",
		"toon":        "user:\n  id: 1\n  name: Ada\n  active: true",
	},
	"primitive_array": {
		"topic":       "primitive_array",
		"description": "Primitive arrays use a length header and inline delimiter-separated values.",
		"toon":        "tags[3]: go,toon,ordered",
	},
	"array_of_objects": {
		"topic":       "array_of_objects",
		"description": "Use list items for complex or irregular object arrays.",
		"toon":        "users[2]:\n  - id: 1\n    name: Ada\n  - id: 2\n    name: Linus",
	},
	"tabular_array": {
		"topic":       "tabular_array",
		"description": "Use tabular arrays when each object has the same primitive fields in the same order.",
		"toon":        "items[2]{sku,qty,price}:\n  A1,2,9.99\n  B2,1,14.5",
	},
	"quoted_strings": {
		"topic":       "quoted_strings",
		"description": "Quote strings that contain delimiters, ambiguous primitive text, or punctuation that is unsafe unquoted.",
		"toon":        "title: \"Hello, world\"\nliteral_bool: \"true\"\nliteral_null: \"null\"\npath: \"a:b\"",
	},
	"empty_array": {
		"topic":       "empty_array",
		"description": "Empty arrays are explicit.",
		"toon":        "items: []",
	},
	"folded_keys": {
		"topic":       "folded_keys",
		"description": "Dotted keys are literal unless safe path expansion is enabled by the decoder.",
		"toon":        "a.b.c: 1",
	},
}

var toonErrorExplanations = map[string]map[string]any{
	"invalid_indent": {
		"code":          "invalid_indent",
		"meaning":       "Indentation does not match the configured TOON indentation rules.",
		"likely_causes": []string{"Mixed indentation widths", "A nested field is not indented under its parent"},
		"fix":           "Use consistent spaces. go-toon defaults to two spaces.",
	},
	"tab_indent": {
		"code":          "tab_indent",
		"meaning":       "A tab was used for indentation where spaces are required.",
		"likely_causes": []string{"Editor inserted tabs", "Copied YAML or Makefile-style indentation"},
		"fix":           "Replace indentation tabs with spaces.",
	},
	"invalid_escape": {
		"code":          "invalid_escape",
		"meaning":       "A quoted string contains an unsupported escape sequence.",
		"likely_causes": []string{"Backslash before a character that TOON does not escape", "Malformed unicode escape"},
		"fix":           "Use valid TOON string escapes or remove the unnecessary backslash.",
	},
	"unterminated_string": {
		"code":          "unterminated_string",
		"meaning":       "A quoted string started but did not close.",
		"likely_causes": []string{"Missing closing quote", "Unescaped quote inside the string"},
		"fix":           "Close the string or escape the embedded quote.",
	},
	"array_count_mismatch": {
		"code":            "array_count_mismatch",
		"meaning":         "The array header count does not match the number of items.",
		"invalid_example": "tags[2]: red,green,blue",
		"fixed_example":   "tags[3]: red,green,blue",
	},
	"tabular_width_mismatch": {
		"code":            "tabular_width_mismatch",
		"meaning":         "A tabular row has a different number of columns than the header declares.",
		"invalid_example": "items[1]{sku,qty}:\n  A1,2,9.99",
		"fixed_example":   "items[1]{sku,qty,price}:\n  A1,2,9.99",
	},
	"duplicate_key": {
		"code":          "duplicate_key",
		"meaning":       "An object or input format contains duplicate keys where duplicates are rejected.",
		"likely_causes": []string{"Repeated object field", "Duplicate JSON key", "Duplicate CSV header"},
		"fix":           "Rename, remove, or intentionally merge the duplicate field. Do not silently discard one unless requested.",
	},
	"malformed_header": {
		"code":            "malformed_header",
		"meaning":         "An array or tabular array header is syntactically invalid.",
		"invalid_example": "items[]{sku,qty}:",
		"fixed_example":   "items[0]{sku,qty}:",
	},
	"header_delimiter_mismatch": {
		"code":          "header_delimiter_mismatch",
		"meaning":       "The delimiter used in an array header does not match row or value delimiters.",
		"likely_causes": []string{"Mixed comma, pipe, or tab delimiters", "Header declares one delimiter but rows use another"},
		"fix":           "Use one delimiter consistently for the array header and values.",
	},
	"missing_colon": {
		"code":            "missing_colon",
		"meaning":         "An object field or array header is missing its colon separator.",
		"invalid_example": "name Ada",
		"fixed_example":   "name: Ada",
	},
	"path_expansion_conflict": {
		"code":          "path_expansion_conflict",
		"meaning":       "Safe path expansion would collide with an existing literal or expanded key.",
		"likely_causes": []string{"Both a.b and a are present", "Dotted keys expand into an existing object path"},
		"fix":           "Rename a key, keep dotted keys literal, or disable path expansion.",
	},
	"resource_limit": {
		"code":          "resource_limit",
		"meaning":       "Input exceeded configured safety limits.",
		"likely_causes": []string{"Too much input", "Too deep nesting", "Too many nodes", "String or array too large"},
		"fix":           "Reduce input size or intentionally raise the relevant ResourceLimits.",
	},
	"invalid_input_format": {
		"code":          "invalid_input_format",
		"meaning":       "A source format such as JSON, YAML, CSV, or XML is invalid or unsafe.",
		"likely_causes": []string{"Malformed source input", "XML DTD", "YAML alias cycle", "Ragged CSV row"},
		"fix":           "Fix the source input or use explicit safe options where appropriate.",
	},
	"unsupported_feature": {
		"code":          "unsupported_feature",
		"meaning":       "The requested syntax or option is not supported by go-toon.",
		"likely_causes": []string{"Using syntax from another format", "Requesting a CLI or converter mode not implemented"},
		"fix":           "Use supported TOON v3.3 syntax and go-toon options.",
	},
	"unsupported_kind": {
		"code":    "unsupported_kind",
		"meaning": "A Go value or node kind cannot be represented by the requested operation.",
		"fix":     "Convert to a supported TOON node kind or use a supported reflect mapping.",
	},
	"cyclic_value": {
		"code":    "cyclic_value",
		"meaning": "A Go value graph contains a cycle during reflection-based conversion.",
		"fix":     "Break the cycle or provide an acyclic DTO before converting to TOON.",
	},
	"unmarshal_type": {
		"code":    "unmarshal_type",
		"meaning": "A TOON node cannot be assigned to the requested Go destination type.",
		"fix":     "Adjust the destination type or decode into `*toon.Node` first.",
	},
	"non_pointer_target": {
		"code":    "non_pointer_target",
		"meaning": "Unmarshal was called with a nil or non-pointer destination.",
		"fix":     "Pass a non-nil pointer, such as `&dst`.",
	},
}

var toonSpecTopics = map[string]map[string]any{
	"overview": {
		"topic":   "overview",
		"summary": "TOON is an ordered object notation. go-toon implements TOON v3.3 with ordered nodes, deterministic encoding, strict decode defaults, and safe format normalization.",
		"rules":   []string{"Preserve field order", "Validate non-trivial hand-written TOON", "Do not assume YAML, JSON, CSV, or markdown syntax applies"},
	},
	"objects": {
		"topic":   "objects",
		"summary": "Objects are ordered key-value fields written as `key: value`. Nested objects use indentation.",
		"example": "user:\n  id: 1\n  name: Ada",
	},
	"arrays": {
		"topic":   "arrays",
		"summary": "Arrays use length headers. Primitive arrays may be inline; complex arrays use list items.",
		"example": "tags[3]: go,toon,ordered",
	},
	"tabular_arrays": {
		"topic":   "tabular_arrays",
		"summary": "A tabular array is an array of objects with identical primitive fields. The header declares fields and each row supplies matching values.",
		"example": "items[2]{sku,qty}:\n  A1,2\n  B2,1",
	},
	"strings": {
		"topic":   "strings",
		"summary": "Strings can be unquoted only when safe. Quote values containing delimiters, unsafe punctuation, escapes, leading/trailing whitespace, or ambiguous primitive text.",
		"example": "title: \"Hello, world\"\nliteral_null: \"null\"",
	},
	"numbers": {
		"topic":   "numbers",
		"summary": "go-toon preserves decoded number tokens losslessly by default through `toon.Number.Raw`. Canonical encoding may normalize number spelling.",
		"example": "count: 42\nprice: 9.99",
	},
	"key_folding": {
		"topic":   "key_folding",
		"summary": "Safe key folding is an encoder option that can flatten nested single-field objects into dotted keys when it will not collide with literal keys.",
		"example": "a.b.c: 1",
	},
	"path_expansion": {
		"topic":   "path_expansion",
		"summary": "Safe path expansion is a decoder option. Dotted keys are literal unless expansion is explicitly enabled.",
		"example": "a.b: 1",
	},
	"validation": {
		"topic":   "validation",
		"summary": "Use `toon.Validate`, `toon validate`, or MCP `toon_validate` before returning non-trivial TOON. Structured errors include stable codes and sometimes line/column details.",
	},
	"security": {
		"topic":   "security",
		"summary": "go-toon keeps strict decode enabled by default, rejects duplicate JSON keys by default, blocks XML DTDs and external entities, rejects YAML alias cycles, and supports resource limits.",
	},
}

func main() {
	if err := serve(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func serve(r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 64*1024*1024)
	enc := json.NewEncoder(w)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var req request
		if err := json.Unmarshal(line, &req); err != nil {
			if err := enc.Encode(response{JSONRPC: "2.0", Error: &rpcError{Code: -32700, Message: "parse error"}}); err != nil {
				return err
			}
			continue
		}
		if len(req.ID) == 0 {
			continue
		}
		if err := enc.Encode(handle(req)); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func handle(req request) response {
	res := response{JSONRPC: "2.0", ID: req.ID}
	switch req.Method {
	case "initialize":
		res.Result = map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities": map[string]any{
				"tools": map[string]any{},
			},
			"serverInfo": map[string]any{
				"name":    "go-toon-mcp",
				"version": "0.1.0",
			},
		}
	case "tools/list":
		res.Result = map[string]any{"tools": tools()}
	case "tools/call":
		result, err := callTool(req.Params)
		if err != nil {
			res.Result = toolError(err)
		} else {
			res.Result = result
		}
	default:
		res.Error = &rpcError{Code: -32601, Message: "method not found"}
	}
	return res
}

func tools() []tool {
	return []tool{
		{
			Name:        "toon_validate",
			Description: "Validate TOON input with go-toon's decoder and return structured validity and parser error details.",
			InputSchema: objectSchema(map[string]any{
				"input":        map[string]any{"type": "string", "description": "TOON document to validate."},
				"strict":       map[string]any{"type": "boolean", "description": "Whether strict decode is enabled. Defaults to true."},
				"expand_paths": map[string]any{"type": "string", "enum": []string{"off", "safe"}, "description": "Path expansion mode. Defaults to off."},
			}, []string{"input"}),
		},
		{
			Name:        "toon_to_json",
			Description: "Decode TOON and return ordered JSON for inspection or downstream use.",
			InputSchema: objectSchema(map[string]any{
				"input":        map[string]any{"type": "string", "description": "TOON document to decode."},
				"strict":       map[string]any{"type": "boolean", "description": "Whether strict decode is enabled. Defaults to true."},
				"expand_paths": map[string]any{"type": "string", "enum": []string{"off", "safe"}, "description": "Path expansion mode. Defaults to off."},
				"indent":       map[string]any{"type": "string", "description": "JSON indentation string. Defaults to two spaces."},
			}, []string{"input"}),
		},
		{
			Name:        "json_to_toon",
			Description: "Convert JSON to canonical TOON through go-toon's ordered JSON parser and encoder.",
			InputSchema: objectSchema(map[string]any{
				"input":                map[string]any{"type": "string", "description": "JSON document to convert."},
				"indent":               map[string]any{"type": "integer", "description": "TOON indent size. Defaults to 2."},
				"delimiter":            map[string]any{"type": "string", "enum": []string{"comma", "tab", "pipe"}, "description": "Array delimiter. Defaults to comma."},
				"length_markers":       map[string]any{"type": "boolean", "description": "Emit # length markers in array headers."},
				"key_folding":          map[string]any{"type": "string", "enum": []string{"off", "safe"}, "description": "Key folding mode. Defaults to off."},
				"flatten_depth":        map[string]any{"type": "integer", "description": "Maximum safe key folding depth."},
				"allow_duplicate_keys": map[string]any{"type": "boolean", "description": "Allow duplicate JSON object keys by keeping the last value. Defaults to false."},
			}, []string{"input"}),
		},
		{
			Name:        "yaml_to_toon",
			Description: "Convert YAML to canonical TOON through go-toon's YAML normalizer and encoder.",
			InputSchema: objectSchema(map[string]any{
				"input":          map[string]any{"type": "string", "description": "YAML document to convert."},
				"documents":      map[string]any{"type": "string", "enum": []string{"error", "array"}, "description": "Multiple-document handling. Defaults to error."},
				"scalars":        map[string]any{"type": "string", "enum": []string{"core", "string"}, "description": "YAML scalar mode. Defaults to core."},
				"indent":         map[string]any{"type": "integer", "description": "TOON indent size. Defaults to 2."},
				"delimiter":      map[string]any{"type": "string", "enum": []string{"comma", "tab", "pipe"}, "description": "Array delimiter. Defaults to comma."},
				"length_markers": map[string]any{"type": "boolean", "description": "Emit # length markers in array headers."},
				"key_folding":    map[string]any{"type": "string", "enum": []string{"off", "safe"}, "description": "Key folding mode. Defaults to off."},
				"flatten_depth":  map[string]any{"type": "integer", "description": "Maximum safe key folding depth."},
			}, []string{"input"}),
		},
		{
			Name:        "csv_to_toon",
			Description: "Convert CSV to canonical TOON through go-toon's CSV normalizer and encoder.",
			InputSchema: objectSchema(map[string]any{
				"input":             map[string]any{"type": "string", "description": "CSV document to convert."},
				"header":            map[string]any{"type": "string", "enum": []string{"present", "absent"}, "description": "Whether the first row is a header. Defaults to present."},
				"csv_delimiter":     map[string]any{"type": "string", "description": "Single-character CSV delimiter. Defaults to comma."},
				"infer_types":       map[string]any{"type": "boolean", "description": "Infer numbers, booleans, and nulls from cells. Defaults to true."},
				"allow_ragged_rows": map[string]any{"type": "boolean", "description": "Allow rows with fewer or more fields than the header."},
				"root_key":          map[string]any{"type": "string", "description": "Optional object key to wrap the resulting array."},
				"indent":            map[string]any{"type": "integer", "description": "TOON indent size. Defaults to 2."},
				"delimiter":         map[string]any{"type": "string", "enum": []string{"comma", "tab", "pipe"}, "description": "TOON array delimiter. Defaults to comma."},
				"length_markers":    map[string]any{"type": "boolean", "description": "Emit # length markers in array headers."},
				"key_folding":       map[string]any{"type": "string", "enum": []string{"off", "safe"}, "description": "Key folding mode. Defaults to off."},
				"flatten_depth":     map[string]any{"type": "integer", "description": "Maximum safe key folding depth."},
			}, []string{"input"}),
		},
		{
			Name:        "xml_to_toon",
			Description: "Convert XML to canonical TOON through go-toon's XML normalizer and encoder with DTD rejection preserved.",
			InputSchema: objectSchema(map[string]any{
				"input":                map[string]any{"type": "string", "description": "XML document to convert."},
				"attribute_prefix":     map[string]any{"type": "string", "description": "Prefix for XML attributes. Defaults to @."},
				"text_key":             map[string]any{"type": "string", "description": "Object key for XML text content. Defaults to #text."},
				"infer_types":          map[string]any{"type": "boolean", "description": "Infer numbers, booleans, and nulls from text and attributes. Defaults to true."},
				"trim_whitespace_text": map[string]any{"type": "boolean", "description": "Trim and skip whitespace-only text. Defaults to true."},
				"mixed_content":        map[string]any{"type": "string", "enum": []string{"compact", "preserve"}, "description": "Mixed content handling. Defaults to compact."},
				"namespaces":           map[string]any{"type": "string", "enum": []string{"local", "qualified", "uri"}, "description": "Namespace rendering. Defaults to local."},
				"indent":               map[string]any{"type": "integer", "description": "TOON indent size. Defaults to 2."},
				"delimiter":            map[string]any{"type": "string", "enum": []string{"comma", "tab", "pipe"}, "description": "TOON array delimiter. Defaults to comma."},
				"length_markers":       map[string]any{"type": "boolean", "description": "Emit # length markers in array headers."},
				"key_folding":          map[string]any{"type": "string", "enum": []string{"off", "safe"}, "description": "Key folding mode. Defaults to off."},
				"flatten_depth":        map[string]any{"type": "integer", "description": "Maximum safe key folding depth."},
			}, []string{"input"}),
		},
		{
			Name:        "toon_examples",
			Description: "Return validated TOON examples for common syntax topics.",
			InputSchema: objectSchema(map[string]any{
				"topic": map[string]any{"type": "string", "description": "Example topic. Empty returns the available topic list."},
			}, nil),
		},
		{
			Name:        "toon_error_explain",
			Description: "Explain a go-toon error code with likely causes and a small invalid/fixed example when useful.",
			InputSchema: objectSchema(map[string]any{
				"code": map[string]any{"type": "string", "description": "go-toon error code, such as array_count_mismatch or duplicate_key."},
			}, []string{"code"}),
		},
		{
			Name:        "toon_spec_lookup",
			Description: "Return a concise TOON v3.3 reference note for a syntax or behavior topic.",
			InputSchema: objectSchema(map[string]any{
				"topic": map[string]any{"type": "string", "description": "Topic such as objects, arrays, tabular_arrays, strings, numbers, key_folding, path_expansion, validation, or security."},
			}, []string{"topic"}),
		},
	}
}

func objectSchema(properties map[string]any, required []string) map[string]any {
	schema := map[string]any{
		"type":                 "object",
		"properties":           properties,
		"additionalProperties": false,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func callTool(raw json.RawMessage) (toolResult, error) {
	var params toolCallParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return toolResult{}, err
	}
	switch params.Name {
	case "toon_validate":
		return handleToonValidate(params.Arguments)
	case "toon_to_json":
		return handleToonToJSON(params.Arguments)
	case "json_to_toon":
		return handleJSONToTOON(params.Arguments)
	case "yaml_to_toon":
		return handleYAMLToTOON(params.Arguments)
	case "csv_to_toon":
		return handleCSVToTOON(params.Arguments)
	case "xml_to_toon":
		return handleXMLToTOON(params.Arguments)
	case "toon_examples":
		return handleToonExamples(params.Arguments)
	case "toon_error_explain":
		return handleToonErrorExplain(params.Arguments)
	case "toon_spec_lookup":
		return handleToonSpecLookup(params.Arguments)
	default:
		return toolResult{}, fmt.Errorf("unknown tool %q", params.Name)
	}
}

func handleToonValidate(raw json.RawMessage) (toolResult, error) {
	var args toonValidateArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return toolResult{}, err
	}
	decodeOpts, err := decodeOptions(args.Strict, args.ExpandPaths)
	if err != nil {
		return toolResult{}, err
	}
	err = toon.Validate([]byte(args.Input), func(o *toon.DecodeOptions) { *o = decodeOpts })
	if err != nil {
		return jsonToolResult(map[string]any{
			"valid": false,
			"error": errorDetails(err),
		})
	}
	return jsonToolResult(map[string]any{"valid": true})
}

func handleToonToJSON(raw json.RawMessage) (toolResult, error) {
	var args toonToJSONArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return toolResult{}, err
	}
	decodeOpts, err := decodeOptions(args.Strict, args.ExpandPaths)
	if err != nil {
		return toolResult{}, err
	}
	n, err := toon.Decode([]byte(args.Input), func(o *toon.DecodeOptions) { *o = decodeOpts })
	if err != nil {
		return jsonToolResult(map[string]any{
			"valid": false,
			"error": errorDetails(err),
		})
	}
	indent := args.Indent
	if indent == "" {
		indent = "  "
	}
	var out bytes.Buffer
	if err := formats.ToJSON(&out, n, func(o *formats.JSONOutputOptions) { o.Indent = indent }); err != nil {
		return toolResult{}, err
	}
	return jsonToolResult(map[string]any{
		"valid": true,
		"json":  out.String(),
	})
}

func handleJSONToTOON(raw json.RawMessage) (toolResult, error) {
	var args jsonToTOONArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return toolResult{}, err
	}
	n, err := formats.FromJSON(strings.NewReader(args.Input), func(o *formats.JSONOptions) {
		o.AllowDuplicateKeys = args.AllowDuplicateKeys
	})
	if err != nil {
		return jsonToolResult(map[string]any{
			"valid": false,
			"error": errorDetails(err),
		})
	}
	encodeOpts, err := encodeOptions(encodeArgs{
		Indent:        args.Indent,
		Delimiter:     args.Delimiter,
		LengthMarkers: args.LengthMarkers,
		KeyFolding:    args.KeyFolding,
		FlattenDepth:  args.FlattenDepth,
	})
	if err != nil {
		return toolResult{}, err
	}
	out, err := toon.Encode(n, func(o *toon.EncodeOptions) { *o = encodeOpts })
	if err != nil {
		return jsonToolResult(map[string]any{
			"valid": false,
			"error": errorDetails(err),
		})
	}
	return jsonToolResult(map[string]any{
		"valid": true,
		"toon":  string(out),
	})
}

func handleYAMLToTOON(raw json.RawMessage) (toolResult, error) {
	var args yamlToTOONArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return toolResult{}, err
	}
	documents, err := yamlDocumentMode(args.Documents)
	if err != nil {
		return toolResult{}, err
	}
	scalars, err := yamlScalarMode(args.Scalars)
	if err != nil {
		return toolResult{}, err
	}
	n, err := formats.FromYAML(strings.NewReader(args.Input), func(o *formats.YAMLOptions) {
		o.Documents = documents
		o.Scalars = scalars
	})
	if err != nil {
		return invalidToolResult(err)
	}
	return encodeNodeToolResult(n, encodeArgs{
		Indent:        args.Indent,
		Delimiter:     args.Delimiter,
		LengthMarkers: args.LengthMarkers,
		KeyFolding:    args.KeyFolding,
		FlattenDepth:  args.FlattenDepth,
	})
}

func handleCSVToTOON(raw json.RawMessage) (toolResult, error) {
	var args csvToTOONArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return toolResult{}, err
	}
	csvDelimiter, err := csvDelimiter(args.CSVDelimiter)
	if err != nil {
		return toolResult{}, err
	}
	inferTypes := true
	if args.InferTypes != nil {
		inferTypes = *args.InferTypes
	}
	headerMode, err := csvHeaderMode(args.Header)
	if err != nil {
		return toolResult{}, err
	}
	n, err := formats.FromCSV(strings.NewReader(args.Input), func(o *formats.CSVOptions) {
		o.HeaderMode = headerMode
		o.Delimiter = csvDelimiter
		o.InferTypes = inferTypes
		o.AllowRaggedRows = args.AllowRaggedRows
	})
	if err != nil {
		return invalidToolResult(err)
	}
	if args.RootKey != "" {
		n = &toon.Node{Kind: toon.ObjectKind, Object: []toon.Field{{Key: args.RootKey, Value: n}}}
	}
	return encodeNodeToolResult(n, encodeArgs{
		Indent:        args.Indent,
		Delimiter:     args.Delimiter,
		LengthMarkers: args.LengthMarkers,
		KeyFolding:    args.KeyFolding,
		FlattenDepth:  args.FlattenDepth,
	})
}

func handleXMLToTOON(raw json.RawMessage) (toolResult, error) {
	var args xmlToTOONArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return toolResult{}, err
	}
	inferTypes := true
	if args.InferTypes != nil {
		inferTypes = *args.InferTypes
	}
	trimWhitespace := true
	if args.TrimWhitespaceText != nil {
		trimWhitespace = *args.TrimWhitespaceText
	}
	mixedContent, err := xmlMixedContentMode(args.MixedContent)
	if err != nil {
		return toolResult{}, err
	}
	namespaces, err := xmlNamespaceMode(args.Namespaces)
	if err != nil {
		return toolResult{}, err
	}
	attrPrefix := args.AttributePrefix
	if attrPrefix == "" {
		attrPrefix = "@"
	}
	textKey := args.TextKey
	if textKey == "" {
		textKey = "#text"
	}
	n, err := formats.FromXML(strings.NewReader(args.Input), func(o *formats.XMLOptions) {
		o.AttributePrefix = attrPrefix
		o.TextKey = textKey
		o.InferTypes = inferTypes
		o.TrimWhitespaceText = trimWhitespace
		o.MixedContent = mixedContent
		o.Namespaces = namespaces
	})
	if err != nil {
		return invalidToolResult(err)
	}
	return encodeNodeToolResult(n, encodeArgs{
		Indent:        args.Indent,
		Delimiter:     args.Delimiter,
		LengthMarkers: args.LengthMarkers,
		KeyFolding:    args.KeyFolding,
		FlattenDepth:  args.FlattenDepth,
	})
}

func handleToonExamples(raw json.RawMessage) (toolResult, error) {
	var args toonExamplesArgs
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &args); err != nil {
			return toolResult{}, err
		}
	}
	if args.Topic == "" {
		return jsonToolResult(map[string]any{
			"topics": sortedKeys(toonExampleTopics),
		})
	}
	example, ok := toonExampleTopics[normalizeTopic(args.Topic)]
	if !ok {
		return toolResult{}, fmt.Errorf("unknown example topic %q", args.Topic)
	}
	return jsonToolResult(example)
}

func handleToonErrorExplain(raw json.RawMessage) (toolResult, error) {
	var args toonErrorExplainArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return toolResult{}, err
	}
	explanation, ok := toonErrorExplanations[normalizeTopic(args.Code)]
	if !ok {
		return toolResult{}, fmt.Errorf("unknown TOON error code %q", args.Code)
	}
	return jsonToolResult(explanation)
}

func handleToonSpecLookup(raw json.RawMessage) (toolResult, error) {
	var args toonSpecLookupArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return toolResult{}, err
	}
	entry, ok := toonSpecTopics[normalizeTopic(args.Topic)]
	if !ok {
		return toolResult{}, fmt.Errorf("unknown TOON spec topic %q", args.Topic)
	}
	return jsonToolResult(entry)
}

type encodeArgs struct {
	Indent        int
	Delimiter     string
	LengthMarkers bool
	KeyFolding    string
	FlattenDepth  int
}

func encodeNodeToolResult(n *toon.Node, args encodeArgs) (toolResult, error) {
	encodeOpts, err := encodeOptions(args)
	if err != nil {
		return toolResult{}, err
	}
	out, err := toon.Encode(n, func(o *toon.EncodeOptions) { *o = encodeOpts })
	if err != nil {
		return invalidToolResult(err)
	}
	return jsonToolResult(map[string]any{
		"valid": true,
		"toon":  string(out),
	})
}

func invalidToolResult(err error) (toolResult, error) {
	return jsonToolResult(map[string]any{
		"valid": false,
		"error": errorDetails(err),
	})
}

func decodeOptions(strict *bool, expandPaths string) (toon.DecodeOptions, error) {
	o := toon.DefaultDecodeOptions()
	if strict != nil {
		o.Strict = *strict
	}
	switch expandPaths {
	case "", "off":
		o.ExpandPaths = toon.ExpandPathsOff
	case "safe":
		o.ExpandPaths = toon.ExpandPathsSafe
	default:
		return o, fmt.Errorf("unsupported expand_paths %q", expandPaths)
	}
	return o, nil
}

func encodeOptions(args encodeArgs) (toon.EncodeOptions, error) {
	o := toon.DefaultEncodeOptions()
	if args.Indent != 0 {
		o.IndentSize = args.Indent
	}
	switch args.Delimiter {
	case "", "comma":
		o.Delimiter = toon.Comma
	case "tab":
		o.Delimiter = toon.Tab
	case "pipe":
		o.Delimiter = toon.Pipe
	default:
		return o, fmt.Errorf("unsupported delimiter %q", args.Delimiter)
	}
	o.IncludeLengthMarkers = args.LengthMarkers
	switch args.KeyFolding {
	case "", "off":
		o.KeyFolding = toon.KeyFoldingOff
	case "safe":
		o.KeyFolding = toon.KeyFoldingSafe
	default:
		return o, fmt.Errorf("unsupported key_folding %q", args.KeyFolding)
	}
	if args.FlattenDepth != 0 {
		o.FlattenDepth = args.FlattenDepth
	}
	return o, nil
}

func yamlDocumentMode(mode string) (formats.YAMLDocumentMode, error) {
	switch mode {
	case "", "error":
		return formats.YAMLDocumentsError, nil
	case "array":
		return formats.YAMLDocumentsArray, nil
	default:
		return formats.YAMLDocumentsError, fmt.Errorf("unsupported documents %q", mode)
	}
}

func yamlScalarMode(mode string) (formats.YAMLScalarMode, error) {
	switch mode {
	case "", "core":
		return formats.YAMLScalarsCore, nil
	case "string":
		return formats.YAMLScalarsString, nil
	default:
		return formats.YAMLScalarsCore, fmt.Errorf("unsupported scalars %q", mode)
	}
}

func csvHeaderMode(mode string) (formats.CSVHeaderMode, error) {
	switch mode {
	case "", "present":
		return formats.CSVHeaderPresent, nil
	case "absent":
		return formats.CSVHeaderAbsent, nil
	default:
		return formats.CSVHeaderPresent, fmt.Errorf("unsupported header %q", mode)
	}
}

func csvDelimiter(value string) (rune, error) {
	if value == "" {
		return ',', nil
	}
	runes := []rune(value)
	if len(runes) != 1 {
		return 0, fmt.Errorf("csv_delimiter must be exactly one character")
	}
	return runes[0], nil
}

func xmlMixedContentMode(mode string) (formats.XMLMixedContentMode, error) {
	switch mode {
	case "", "compact":
		return formats.XMLMixedContentCompact, nil
	case "preserve":
		return formats.XMLMixedContentPreserve, nil
	default:
		return formats.XMLMixedContentCompact, fmt.Errorf("unsupported mixed_content %q", mode)
	}
}

func xmlNamespaceMode(mode string) (formats.XMLNamespaceMode, error) {
	switch mode {
	case "", "local":
		return formats.XMLNamespacesLocal, nil
	case "qualified":
		return formats.XMLNamespacesQualified, nil
	case "uri":
		return formats.XMLNamespacesURI, nil
	default:
		return formats.XMLNamespacesLocal, fmt.Errorf("unsupported namespaces %q", mode)
	}
}

func normalizeTopic(value string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), "-", "_"))
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

func errorDetails(err error) map[string]any {
	details := map[string]any{
		"message": err.Error(),
	}
	var toonErr *toon.Error
	if errors.As(err, &toonErr) {
		details["code"] = string(toonErr.Code)
		if toonErr.Line > 0 {
			details["line"] = toonErr.Line
		}
		if toonErr.Column > 0 {
			details["column"] = toonErr.Column
		}
		if toonErr.Context != "" {
			details["context"] = toonErr.Context
		}
	}
	return details
}

func jsonToolResult(v any) (toolResult, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return toolResult{}, err
	}
	return toolResult{Content: []textContent{{Type: "text", Text: string(data)}}}, nil
}

func toolError(err error) toolResult {
	data, marshalErr := json.MarshalIndent(map[string]any{
		"valid": false,
		"error": map[string]any{"message": err.Error()},
	}, "", "  ")
	if marshalErr != nil {
		data = []byte(`{"valid":false,"error":{"message":"tool error"}}`)
	}
	return toolResult{Content: []textContent{{Type: "text", Text: string(data)}}, IsError: true}
}
