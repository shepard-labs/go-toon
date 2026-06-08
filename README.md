# go-toon

[![CI](https://github.com/shepard-labs/go-toon/actions/workflows/ci.yml/badge.svg)](https://github.com/shepard-labs/go-toon/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/shepard-labs/go-toon/graph/badge.svg)](https://codecov.io/gh/shepard-labs/go-toon)
[![Go Reference](https://pkg.go.dev/badge/github.com/shepard-labs/go-toon/toon.svg)](https://pkg.go.dev/github.com/shepard-labs/go-toon/toon)
[![Go Version](https://img.shields.io/github/go-mod/go-version/shepard-labs/go-toon)](https://github.com/shepard-labs/go-toon)
[![TOON Spec](https://img.shields.io/badge/TOON-v3.3-blueviolet)](https://github.com/toon-format/spec/blob/main/SPEC.md)
[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/shepard-labs/go-toon)

`go-toon` is a Go implementation of Token-Oriented Object Notation (TOON) v3.3. It provides an ordered-node library, format normalization helpers, ordered JSON output, and a `toon` CLI.

The TOON specification this implementation follows lives at [toon-format/spec/SPEC.md](https://github.com/toon-format/spec/blob/main/SPEC.md).

The library exposes `toon.SpecVersion = "3.3"`.

## Features

- 🚀 **TOON v3.3 compliant** — encoder, decoder, and validator follow the [TOON spec](https://github.com/toon-format/spec/blob/main/SPEC.md).
- 📦 **Ordered document model** — `toon.Node` preserves field order end-to-end; Go `map` is never the primary representation.
- 🔄 **Format normalization** — convert JSON, YAML, CSV, and XML into ordered `*toon.Node` values, plus ordered JSON output that bypasses Go maps.
- 🛡️ **Security-first defaults** — strict decode on, JSON duplicate keys rejected, XML DTDs and external entities blocked, YAML alias cycle detection, configurable resource limits.
- 🔢 **Lossless numbers** — raw decimal token preservation by default, with explicit opt-ins for `float64` and safe-string fallbacks for unsafe values.
- 🧰 **Production CLI** — `toon encode`, `decode`, and `validate` with a flag surface mirroring library options.
- ⚡ **Battle-tested** — race tests, fuzz targets, golden conformance tests, and cross-platform builds for `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`, and `windows/amd64`.

## Comparison

| Feature                          |                                          go-toon |                  toon-format/toon-go |
|----------------------------------|-------------------------------------------------:|-------------------------------------:|
| Primary API style                | 🧱 Ordered `*toon.Node` core plus `toon/reflect` | 🪶 Top-level `Marshal` / `Unmarshal` |
| Decode target                    |                          🧱 Ordered `*toon.Node` | 🗺️ `any`, `map[string]any`, `[]any` |
| Preserves decoded object order   |                                            ✅ Yes |                                 ❌ No |
| Lossless number tokens on decode |                                            ✅ Yes |  ⚠️ No, decodes numbers as `float64` |
| Struct marshal/unmarshal         |                        ✅ Yes, via `toon/reflect` |                     ✅ Yes, top-level |
| `omitempty` struct tags          |                                            ✅ Yes |                                ✅ Yes |
| Custom `time.Time` formatting    |                                            ✅ Yes |                                ✅ Yes |
| Optional length markers          |                                            ✅ Yes |                                ✅ Yes |
| JSON/YAML/CSV/XML to TOON        |                                            ✅ Yes |                                 ❌ No |
| Ordered TOON to JSON             |                                            ✅ Yes |                                 ❌ No |
| CLI                              |                                            ✅ Yes |                                 ❌ No |
| Validation-only API              |                                            ✅ Yes |                   ❌ No dedicated API |
| Typed error codes                |                                            ✅ Yes |           ❌ No public error-code API |
| Resource limits                  |                                            ✅ Yes |          ❌ No public resource limits |
| Key folding / path expansion     |                                            ✅ Yes |                                 ❌ No |
| Reusable encoder/decoder types   |                           ❌ No public core types |                                ✅ Yes |
| String convenience APIs          |        ❌ No core `EncodeString` / `DecodeString` |                                ✅ Yes |
| External dependencies            |                            📦 `gopkg.in/yaml.v3` |                         ✅ None found |

## Library API

### Core package

```go
import "github.com/shepard-labs/go-toon/toon"
```

#### Types

- `toon.Node` — the ordered document model. A node can represent any TOON value.
- `toon.Field` — a key-value pair within an `ObjectKind` node.
- `toon.Number` — stores the raw string representation of a number for lossless round-trips.
- `toon.Kind` — the type of node (`NullKind`, `BoolKind`, `NumberKind`, `StringKind`, `ArrayKind`, `ObjectKind`).

#### Encoding

```go
n := &toon.Node{
    Kind: toon.ObjectKind,
    Object: []toon.Field{
        {Key: "id", Value: &toon.Node{Kind: toon.NumberKind, Number: toon.Number{Raw: "1"}}},
        {Key: "user", Value: &toon.Node{
            Kind: toon.ObjectKind,
            Object: []toon.Field{
                {Key: "name", Value: &toon.Node{Kind: toon.StringKind, String: "Ada"}},
                {Key: "active", Value: &toon.Node{Kind: toon.BoolKind, Bool: true}},
            },
        }},
    },
}

// Encode to bytes
data, err := toon.Encode(n)

// Encode to a writer
var buf bytes.Buffer
err = toon.EncodeToWriter(&buf, n)

// With custom indentation
data, err = toon.Encode(n, func(o *toon.EncodeOptions) {
    o.IndentSize = 4
})

// With optional length markers in array headers
data, err = toon.Encode(n, func(o *toon.EncodeOptions) {
    o.IncludeLengthMarkers = true
})
// items[#2]{sku,qty,price}:
```

#### Decoding

```go
// Decode from bytes
n, err := toon.Decode([]byte("id: 1\nuser:\n  name: Ada\n  active: true"))

// Decode from a reader
n, err = toon.DecodeReader(strings.NewReader("id: 1"))

// Validate without decoding
err = toon.Validate([]byte("id: 1"))

// With path expansion (dot-separated keys become nested objects)
n, err = toon.Decode([]byte("a.b: 1"), func(o *toon.DecodeOptions) {
    o.ExpandPaths = toon.ExpandPathsSafe
})

// With strict mode disabled
n, err = toon.Decode([]byte("id: 1"), func(o *toon.DecodeOptions) {
    o.Strict = false
})

// With resource limits
n, err = toon.Decode([]byte("id: 1"), func(o *toon.DecodeOptions) {
    o.Limits = toon.ResourceLimits{
        MaxDepth:       10,
        MaxNodes:       1000,
        MaxBytes:       1024,
        MaxStringBytes: 1024,
        MaxArrayLength: 100,
    }
})
```

#### Error handling

```go
err := toon.Validate([]byte("invalid"))
if err != nil {
    // Extract the error code for branching without string matching
    code := toon.CodeOf(err)
    if code == toon.ErrDuplicateKey {
        // handle duplicate key
    }

    // Or use errors.As
    var toonErr *toon.Error
    if errors.As(err, &toonErr) {
        fmt.Println(toonErr.Line, toonErr.Column, toonErr.Code)
    }
}
```

#### Tabular arrays

```go
// Encoding tabular arrays
n := &toon.Node{
    Kind: toon.ObjectKind,
    Object: []toon.Field{
        {Key: "items", Value: &toon.Node{
            Kind: toon.ArrayKind,
            Array: []*toon.Node{
                {Kind: toon.ObjectKind, Object: []toon.Field{
                    {Key: "sku", Value: &toon.Node{Kind: toon.StringKind, String: "A1"}},
                    {Key: "qty", Value: &toon.Node{Kind: toon.NumberKind, Number: toon.Number{Raw: "2"}}},
                    {Key: "price", Value: &toon.Node{Kind: toon.NumberKind, Number: toon.Number{Raw: "9.9900"}}},
                }},
                {Kind: toon.ObjectKind, Object: []toon.Field{
                    {Key: "sku", Value: &toon.Node{Kind: toon.StringKind, String: "B2"}},
                    {Key: "qty", Value: &toon.Node{Kind: toon.NumberKind, Number: toon.Number{Raw: "1"}}},
                    {Key: "price", Value: &toon.Node{Kind: toon.NumberKind, Number: toon.Number{Raw: "14.5"}}},
                }},
            },
        }},
    },
}
data, _ := toon.Encode(n)
// Output: items[2]{sku,qty,price}:\n  A1,2,9.99\n  B2,1,14.5
```

#### Key folding

```go
n := &toon.Node{
    Kind: toon.ObjectKind,
    Object: []toon.Field{
        {Key: "a", Value: &toon.Node{
            Kind: toon.ObjectKind,
            Object: []toon.Field{
                {Key: "b", Value: &toon.Node{
                    Kind: toon.ObjectKind,
                    Object: []toon.Field{
                        {Key: "c", Value: &toon.Node{Kind: toon.NumberKind, Number: toon.Number{Raw: "1"}}},
                    },
                }},
            },
        }},
    },
}
data, _ := toon.Encode(n, func(o *toon.EncodeOptions) {
    o.KeyFolding = toon.KeyFoldingSafe
})
// Output: a.b.c: 1
```

#### Number modes

```go
// Lossless (default) — preserves decimal tokens
n, _ := toon.Decode([]byte("pi: 3.141592653589793238462643383279"))
// n.Object[0].Value.Number.Raw == "3.141592653589793238462643383279"

// Float64 — may lose precision
n, _ = toon.Decode([]byte("pi: 3.141592653589793238462643383279"), func(o *toon.DecodeOptions) {
    o.Limits = toon.ResourceLimits{} // number mode is set via EncodeOptions
})

// String for unsafe values on encode
n := &toon.Node{Kind: toon.NumberKind, Number: toon.Number{Raw: "1e500"}}
data, _ := toon.Encode(n, func(o *toon.EncodeOptions) {
    o.NumberMode = toon.NumberStringForUnsafe
})
// Output: 1e500 (quoted as a string)
```

#### Utility functions

```go
// Check if a string is a valid unquoted TOON key
toon.IsValidUnquotedKey("myField")  // true
toon.IsValidUnquotedKey("123abc")   // false

// Escape a string for TOON output
toon.EscapeString("hello\nworld")   // "hello\\nworld"

// Check if a string needs quoting
toon.NeedsQuotes("hello", toon.Comma)  // false
toon.NeedsQuotes("hello,world", toon.Comma)  // true

// Parse a primitive token
n, _ := toon.ParsePrimitiveToken("42")  // NumberKind with Raw="42"
n, _ = toon.ParsePrimitiveToken("true") // BoolKind with Bool=true
n, _ = toon.ParsePrimitiveToken("null") // NullKind

// Canonicalize a number token
toon.CanonicalizeNumberToken("007")   // "7"
toon.CanonicalizeNumberToken("1.200") // "1.2"
```

#### Error codes

```go
const (
    ErrInvalidIndent           ErrorCode = "invalid_indent"
    ErrTabIndent               ErrorCode = "tab_indent"
    ErrInvalidEscape           ErrorCode = "invalid_escape"
    ErrUnterminatedString      ErrorCode = "unterminated_string"
    ErrArrayCountMismatch      ErrorCode = "array_count_mismatch"
    ErrTabularWidthMismatch    ErrorCode = "tabular_width_mismatch"
    ErrDuplicateKey            ErrorCode = "duplicate_key"
    ErrMalformedHeader         ErrorCode = "malformed_header"
    ErrHeaderDelimiterMismatch ErrorCode = "header_delimiter_mismatch"
    ErrMissingColon            ErrorCode = "missing_colon"
    ErrPathExpansionConflict   ErrorCode = "path_expansion_conflict"
    ErrResourceLimit           ErrorCode = "resource_limit"
    ErrInvalidInputFormat      ErrorCode = "invalid_input_format"
    ErrUnsupportedFeature      ErrorCode = "unsupported_feature"
    ErrUnsupportedKind         ErrorCode = "unsupported_kind"
    ErrCyclicValue             ErrorCode = "cyclic_value"
    ErrUnmarshalType           ErrorCode = "unmarshal_type"
    ErrNonPointerTarget        ErrorCode = "non_pointer_target"
)
```

### Reflection package

```go
import "github.com/shepard-labs/go-toon/toon/reflect"
```

#### Marshaling Go values to TOON

```go
type User struct {
    ID    int      `toon:"id"`
    Name  string   `toon:"name"`
    Tags  []string `toon:"tags"`
    Score float64  `toon:"score"`
}

// Marshal to TOON bytes via a writer
var buf bytes.Buffer
err := reflect.Marshal(&buf, User{ID: 42, Name: "Ada", Tags: []string{"a", "b"}, Score: 9.5})
// buf: "id: 42\nname: Ada\ntags:\n- a\n- b\nscore: 9.5"

// Marshal to a *toon.Node
node, err := reflect.NodeFromValue(User{ID: 42, Name: "Ada"})
// node.Kind == toon.ObjectKind

// With custom number mode for non-finite floats
node, err = reflect.NodeFromValue(math.NaN(), func(o *reflect.ValueOptions) {
    o.NumberMode = toon.NumberStringForUnsafe
})
// node.Kind == toon.StringKind, node.String == "NaN"

// With resource limits
_, err = reflect.NodeFromValue([]int{1, 2, 3, 4, 5}, func(o *reflect.ValueOptions) {
    o.Limits = toon.ResourceLimits{MaxArrayLength: 2}
})
// err.Code == toon.ErrResourceLimit

// Struct with skipped and unexported fields
type Record struct {
    Name     string `toon:"name"`
    Optional string `toon:"optional,omitempty"` // skipped when empty
    Skip     string `toon:"-"`                  // skipped
    hidden   string                              // unexported — skipped
}
node, _ = reflect.NodeFromValue(Record{Name: "x", Skip: "y", hidden: "z"})
// node.Object = [{Key: "name", Value: String("x")}]
```

#### Custom time formatting

```go
type Event struct {
    At time.Time `toon:"at"`
}

var buf bytes.Buffer
err := reflect.Marshal(&buf, Event{At: time.Date(2025, 6, 4, 12, 30, 0, 0, time.UTC)}, func(o *reflect.Options) {
    o.Value.TimeFormatter = func(t time.Time) string {
        return t.UTC().Format("2006-01-02")
    }
})
// buf: "at: 2025-06-04"
```

#### Unmarshaling TOON to Go values

```go
// Unmarshal TOON bytes to a Go value
var u User
err := reflect.Unmarshal([]byte("id: 42\nname: Ada\ntags:\n- a\n- b\nscore: 9.5"), &u)
// u.ID == 42, u.Name == "Ada", u.Tags == []string{"a", "b"}, u.Score == 9.5

// Unmarshal from a reader
var u2 User
err = reflect.UnmarshalReader(strings.NewReader("id: 1\nname: Bob"), &u2)

// Unmarshal from an already-decoded node
n, _ := toon.Decode([]byte("name: Bob\nage: 30"))
type Person struct {
    Name string `toon:"name"`
    Age  int    `toon:"age"`
}
var p Person
err = reflect.UnmarshalNode(n, &p)
// p.Name == "Bob", p.Age == 30

// With strict mode disabled
err = reflect.Unmarshal([]byte("id: 1"), &u, func(o *reflect.UnmarshalOptions) {
    o.Decode.Strict = false
})

// []byte is encoded as base64
var b []byte
n := &toon.Node{Kind: toon.StringKind, String: "SGVsbG8="}
err = reflect.NodeToValue(n, &b)
// b == []byte("Hello")
```

#### TextMarshaler / TextUnmarshaler

```go
type Timestamp struct {
    Time time.Time
}

func (t Timestamp) MarshalText() ([]byte, error) {
    return []byte(t.Time.Format(time.RFC3339)), nil
}

func (t *Timestamp) UnmarshalText(b []byte) error {
    var err error
    t.Time, err = time.Parse(time.RFC3339, string(b))
    return err
}

// MarshalText is honored on encode
node, _ := reflect.NodeFromValue(Timestamp{Time: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)})
// node.Kind == toon.StringKind, node.String == "2025-01-01T00:00:00Z"

// UnmarshalText is honored on decode
var ts Timestamp
n := &toon.Node{Kind: toon.StringKind, String: "2025-06-04T12:00:00Z"}
reflect.NodeToValue(n, &ts)
// ts.Time == time.Date(2025, 6, 4, 12, 0, 0, 0, time.UTC)
```

#### Cycle detection

```go
type ListNode struct {
    Value int
    Next  *ListNode
}

a := &ListNode{Value: 1}
b := &ListNode{Value: 2, Next: a}
a.Next = b // creates a cycle

_, err := reflect.NodeFromValue(a)
// err.Code == toon.ErrCyclicValue
```

### Formats package

```go
import "github.com/shepard-labs/go-toon/formats"
```

#### Parsing JSON

```go
// Parse JSON into a TOON node
n, err := formats.FromJSON(strings.NewReader(`{"name":"Ada","age":36,"active":true}`))
// n.Kind == toon.ObjectKind

// Allow duplicate keys (last value wins)
n, err = formats.FromJSON(strings.NewReader(`{"a":1,"a":2}`), func(o *formats.JSONOptions) {
    o.AllowDuplicateKeys = true
})

// With resource limits
n, err = formats.FromJSON(strings.NewReader(`{"a":1}`), func(o *formats.JSONOptions) {
    o.Limits = toon.ResourceLimits{MaxBytes: 1024}
})
```

#### Parsing YAML

```go
// Parse YAML into a TOON node
n, err := formats.FromYAML(strings.NewReader(`name: Ada
age: 36
tags:
- a
- b`))

// Multiple documents → array
n, err = formats.FromYAML(strings.NewReader(`a: 1
---
b: 2`), func(o *formats.YAMLOptions) {
    o.Documents = formats.YAMLDocumentsArray
})
// n.Kind == toon.ArrayKind, n.Array has 2 elements

// Force all scalars to strings
n, err = formats.FromYAML(strings.NewReader(`a: 1`), func(o *formats.YAMLOptions) {
    o.Scalars = formats.YAMLScalarsString
})
// n.Object[0].Value.Kind == toon.StringKind (not NumberKind)
```

#### Parsing CSV

```go
// With headers and type inference
n, err := formats.FromCSV(strings.NewReader(`id,name,zip
1,Ada,05
2,Bob,10`), func(o *formats.CSVOptions) {
    o.HeaderMode = formats.CSVHeaderPresent
    o.InferTypes = true
})
// n.Kind == toon.ArrayKind, n.Array[0] == {id: 1, name: "Ada", zip: "05"}

// Without headers, custom delimiter
n, err = formats.FromCSV(strings.NewReader(`1|true
2|false`), func(o *formats.CSVOptions) {
    o.HeaderMode = formats.CSVHeaderAbsent
    o.Delimiter = '|'
    o.InferTypes = false
})
// n.Array[0] == {field1: "1", field2: "true"}
```

#### Parsing XML

```go
// Parse XML into a TOON node
n, err := formats.FromXML(strings.NewReader(`<user id="1">Ada</user>`))
// n.Kind == toon.ObjectKind, n.Object = [{Key: "user", Value: { @id: 1, #text: "Ada" }}]

// Preserve mixed content
n, err = formats.FromXML(strings.NewReader(`<p>Hello <b>world</b>!</p>`), func(o *formats.XMLOptions) {
    o.MixedContent = formats.XMLMixedContentPreserve
})
// n.Object = [{Key: "p", Value: [{#text: "Hello "}, {b: "world"}, {#text: "!"}]}]

// Namespace-qualified names
n, err = formats.FromXML(strings.NewReader(`<ns:root xmlns:ns="urn:x"><ns:a>1</ns:a></ns:root>`), func(o *formats.XMLOptions) {
    o.Namespaces = formats.XMLNamespacesQualified
})
// n.Object = [{Key: "ns:root", Value: {ns:a: 1}}]
```

#### Serializing TOON to JSON

```go
// Write a TOON node as ordered JSON
n, _ := formats.FromJSON(strings.NewReader(`{"z":1,"a":{"b":2}}`))
var out bytes.Buffer
formats.ToJSON(&out, n)
// out: {"z":1,"a":{"b":2}}

// With indentation
formats.ToJSON(&out, n, func(o *formats.JSONOutputOptions) {
    o.Indent = "  "
})
// out:
// {
//   "z": 1,
//   "a": {
//     "b": 2
//   }
// }
```

#### High-level format conversions

```go
// JSON → TOON
var toonOut bytes.Buffer
formats.JSONToTOON(strings.NewReader(`{"a":1,"b":[2,3]}`), &toonOut, formats.JSONToTOONOptions{})
// toonOut: "a: 1\nb:\n- 2\n- 3"

// TOON → JSON
var jsonOut bytes.Buffer
formats.TOONToJSON(strings.NewReader("a: 1"), &jsonOut,
    formats.TOONToJSONOptions{JSON: formats.JSONOutputOptions{Indent: "  "}})
// jsonOut: "{\n  \"a\": 1\n}"

// YAML → TOON
var out bytes.Buffer
formats.YAMLToTOON(strings.NewReader("a: 1\nb: 2"), &out, formats.YAMLToTOONOptions{})
// out: "a: 1\nb: 2"

// XML → TOON
formats.XMLToTOON(strings.NewReader("<a>1</a>"), &out, formats.XMLToTOONOptions{})
// out: "a: 1"

// CSV → TOON
formats.CSVToTOON(strings.NewReader("a\n1\n"), &out,
    formats.CSVToTOONOptions{CSV: formats.CSVOptions{HeaderMode: formats.CSVHeaderPresent, InferTypes: true}})
// out: "[1]{a}:\n  1"
```

## CLI

Build:

```sh
go build ./cmd/toon
```

Encode external formats to TOON:

```sh
toon encode input.json
toon encode --input input.yaml --output output.toon
toon encode --format json < input.json
toon encode data.csv --csv-header present --csv-infer-types=true
toon encode data.tsv
```

Decode TOON to ordered JSON:

```sh
toon decode input.toon
toon decode input.toon --expand-paths safe --indent 4
```

Validate TOON:

```sh
toon validate input.toon
```

Important encode flags:

- `--format json|yaml|xml|csv|auto`
- `--delimiter comma|tab|pipe`
- `--length-markers`
- `--key-folding off|safe`
- `--flatten-depth N`
- `--stats`
- `--csv-header auto|present|absent`
- `--csv-delimiter CHAR`
- `--csv-infer-types true|false`
- `--csv-root-key NAME`
- `--yaml-docs error|array`
- `--yaml-scalars core|string`
- `--xml-attr-prefix @`
- `--xml-text-key '#text'`
- `--xml-infer-types true|false`
- `--xml-mixed-content compact|preserve`
- `--xml-namespaces local|qualified|uri`

## Defaults And Security

- TOON decode strict mode is enabled by default.
- Safe path expansion is off by default.
- Safe key folding is off by default.
- Number handling defaults to lossless raw tokens.
- JSON duplicate keys are rejected by default.
- CSV library callers must choose header-present or header-absent mode explicitly; the CLI treats `auto` as header-present for normal CSV workflows.
- XML DTDs are rejected and external entity processing is not enabled.
- YAML aliases are resolved through the YAML AST with cycle rejection.
- Resource limits are configurable for input bytes, string bytes, node count, depth, and array length.
- CLI safe defaults set `MaxDepth` to `512` and `MaxStringBytes` to `64 MiB`.
- `toon/reflect` decode requires a non-nil pointer destination (`toon.ErrNonPointerTarget` otherwise).
- `toon/reflect` cycles on encode are rejected with `toon.ErrCyclicValue`; type mismatches on decode surface as `toon.ErrUnmarshalType`.
- `toon/reflect` `[]byte` (and named `[]byte` types) marshal as base64 strings and unmarshal from base64 strings.
- `toon/reflect` `TextMarshaler` is honored on encode; `TextUnmarshaler` is honored on decode as an escape hatch.
- `toon/reflect` decode propagates `toon.Decode` errors (e.g., `ErrInvalidIndent`, `ErrDuplicateKey`) with their original code via `toon.CodeOf`.

## Number Modes

- `toon.NumberLossless` preserves decimal tokens where possible and canonicalizes without unsafe `float64` conversion.
- `toon.NumberFloat64` may round through `float64`; use it only when precision loss is acceptable.
- `toon.NumberStringForUnsafe` emits quoted strings for numbers outside the safe canonicalization domain.

Patch releases must not change canonical output except to fix spec-compliance bugs.

## Format Normalization

JSON:

- Uses `json.Decoder.Token()` with `UseNumber()`.
- Preserves object order.
- Rejects duplicate keys unless explicitly allowed, in which case deterministic last-write-wins applies.

YAML:

- Uses `gopkg.in/yaml.v3` AST to preserve mapping order.
- Multiple documents error by default or become an array when requested.
- Core scalars become typed primitives; timestamps remain strings.
- Merge keys are resolved deterministically.
- Unknown tags and non-string keys are stringified.

CSV:

- Header-present rows become arrays of objects keyed by header names.
- Header-absent rows use generated `field1`, `field2`, ... keys.
- Empty cells remain empty strings.
- Type inference is configurable.
- Numeric-looking cells with forbidden leading zeros remain strings.

XML:

- Elements become object fields keyed by element name.
- Attributes use `@` by default.
- Text with attributes or children uses `#text` by default.
- Repeated child elements become arrays.
- Comments and processing instructions are ignored.
- Mixed content defaults to compact mode; preserve mode keeps exact text/child order as an array.
- Namespace handling supports local, qualified, and URI-expanded modes.

## Compatibility And Versioning

This module follows semantic versioning.

- Patch releases fix bugs and spec-compliance issues without intentional API breaks.
- Minor releases may add options and non-breaking APIs.
- Major releases may track breaking TOON spec changes or public API changes.

The ordered `toon.Node` model is the stable core API. Go maps are not used as the primary representation for ordered documents.

## Release Verification

Recommended release checks:

```sh
go test ./...
go test -race ./...
go test ./toon -run=Fuzz -fuzz=FuzzDecode -fuzztime=10s
go test ./toon/reflect -run=Fuzz -fuzz=FuzzMarshalValue -fuzztime=10s
go test ./toon/reflect -run=Fuzz -fuzz=FuzzUnmarshalValue -fuzztime=10s
go test ./... -bench=. -run='^$'
```

Supported build-check targets for `cmd/toon`:

- `linux/amd64`
- `linux/arm64`
- `darwin/amd64`
- `darwin/arm64`
- `windows/amd64`

## Benchmarks

Latest local benchmark snapshot, run with Go 1.26 on Darwin ARM64, Apple M4 Max:

```sh
go test ./toon ./formats -bench=. -run='^$' -benchmem -count=5
```

| Benchmark | Time/op | Bytes/op | Allocs/op |
|---|---:|---:|---:|
| `BenchmarkEncodeLargeOrderedObject` | 48-49 us | 33 KB | 110 |
| `BenchmarkEncodeLargeTabularArray` | 91-94 us | 66 KB | 113 |
| `BenchmarkDecodeRepresentativeTOON` | 192-204 us | 565 KB | 6031 |
| `BenchmarkLargeJSONInputNormalization` | 197-201 us | 772 KB | 7033 |
| `BenchmarkLargeCSVInputNormalization` | 294-297 us | 982 KB | 12026 |
| `BenchmarkJSONOutputOrderedNode` | 88-92 us | 188 KB | 122 |
| `BenchmarkWideJSONObjectNormalization` | 89-91 us | 329 KB | 1050 |
| `BenchmarkWideCSVHeaderNormalization` | 147 us | 557 KB | 2085 |
| `BenchmarkMarshalSmallStruct` (`toon/reflect`) | ~3.0 us | 4.5 KB | 62 |
| `BenchmarkUnmarshalSmallStruct` (`toon/reflect`) | ~3.3 us | 5.3 KB | 50 |
| `BenchmarkMarshalLargeMap` (`toon/reflect`, 1k keys) | ~333 us | 305 KB | 6122 |
| `BenchmarkUnmarshalLargeArray` (`toon/reflect`, 256 ints) | ~21 us | 42 KB | 280 |
| `BenchmarkNodeFromValue` (`toon/reflect`) | ~2.0 us | 3.5 KB | 48 |
| `BenchmarkNodeToValue` (`toon/reflect`) | ~1.5 us | 0.8 KB | 9 |

JSON normalization uses a purpose-built ordered scanner to preserve object order, detect duplicate keys, and retain lossless number tokens without routing through Go maps.
