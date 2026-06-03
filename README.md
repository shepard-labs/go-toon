# go-toon

[![CI](https://github.com/shepard-labs/go-toon/actions/workflows/ci.yml/badge.svg)](https://github.com/shepard-labs/go-toon/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/shepard-labs/go-toon/toon.svg)](https://pkg.go.dev/github.com/shepard-labs/go-toon/toon)
[![Go Version](https://img.shields.io/github/go-mod/go-version/shepard-labs/go-toon)](https://github.com/shepard-labs/go-toon)
[![TOON Spec](https://img.shields.io/badge/TOON-v3.3-blueviolet)](https://github.com/toon-format/spec/blob/main/SPEC.md)

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

## Library API

Core package:

```go
import "github.com/shepard-labs/go-toon/toon"
```

- `toon.Node`, `toon.Field`, `toon.Number`, and `toon.Kind` represent ordered documents.
- `toon.Encode` and `toon.EncodeToWriter` encode ordered nodes to TOON.
- `toon.Decode`, `toon.DecodeReader`, and `toon.Validate` parse and validate TOON.
- `toon.EncodeOptions` and `toon.DecodeOptions` are the concrete option sources of truth.
- `toon.Error` carries stable `toon.ErrorCode` values. Use `toon.CodeOf(err)` or `errors.As` to branch without string matching.

Formats package:

```go
import "github.com/shepard-labs/go-toon/formats"
```

- `formats.FromJSON`, `FromYAML`, `FromCSV`, and `FromXML` normalize inputs into ordered `*toon.Node` values.
- `formats.ToJSON` writes ordered JSON from a node without routing through Go maps.
- `formats.JSONToTOON`, `YAMLToTOON`, `CSVToTOON`, `XMLToTOON`, and `TOONToJSON` are high-level conversion helpers.

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

JSON normalization uses a purpose-built ordered scanner to preserve object order, detect duplicate keys, and retain lossless number tokens without routing through Go maps.
