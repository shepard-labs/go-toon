# Agent Instructions For go-toon

This repository implements Token-Oriented Object Notation (TOON) v3.3 in Go. Treat TOON as a precise data format, not as YAML, JSON, or markdown with different punctuation.

## Project Map

- `toon/` is the core ordered document library: node model, encoder, decoder, validation, errors, limits, strings, and numbers.
- `toon/reflect/` maps Go values and structs to and from ordered `*toon.Node` trees.
- `formats/` normalizes JSON, YAML, CSV, and XML into ordered `*toon.Node` values and writes ordered JSON.
- `cmd/toon/` is the CLI: `toon encode`, `toon decode`, and `toon validate`.
- `docs/ai/` contains condensed TOON references for AI agents.
- `skills/toon-format/` contains a portable skill for agents that need to write, debug, or convert TOON.
- `mcp/go-toon-mcp/` exposes this repo's validator and converters to MCP-compatible agents.

## Core Invariants

- Preserve object field order. Do not use Go maps in paths that must preserve order.
- `*toon.Node` is the primary representation for ordered documents.
- Encoder output must be deterministic: LF line endings, no trailing newline, and no trailing spaces.
- Strict decode is enabled by default.
- JSON duplicate keys are rejected by default.
- XML DTDs and external entities remain blocked.
- YAML aliases must reject cycles.
- Resource limits must remain configurable and safe for untrusted input.
- Patch releases must not change canonical output except to fix spec-compliance bugs.

## Stable API Surface

Treat these as stable public API unless the user is intentionally making a versioned API change:

- `toon.Node`, `toon.Field`, `toon.Number`, `toon.Kind`
- `toon.Encode`, `toon.EncodeToWriter`
- `toon.Decode`, `toon.DecodeReader`, `toon.Validate`
- `toon.EncodeOptions`, `toon.DecodeOptions`, `toon.ResourceLimits`
- `toon.Error`, `toon.ErrorCode`, `toon.CodeOf`
- `toon/reflect`: `Marshal`, `Unmarshal`, `UnmarshalReader`, `UnmarshalNode`, `NodeFromValue`, `NodeToValue`, `ValueOptions`, `Options`, and `UnmarshalOptions`

## TOON Generation Rules For Agents

TOON is newer than common formats, so do not rely on memory alone for non-trivial examples.

- Read `docs/ai/toon-cheatsheet.md` before generating or explaining TOON.
- Use examples from `docs/ai/toon-examples.md` when possible.
- Validate non-trivial TOON before presenting it as final.
- If the MCP server is available, use `toon_validate` for hand-written TOON.
- For JSON input, prefer `json_to_toon` or the `formats.FromJSON` path over manual conversion.
- For debugging, convert TOON to ordered JSON with `toon_to_json` to inspect structure.
- Do not introduce syntax not covered by TOON v3.3.

## Editing Guidance

- Keep changes focused and small.
- Add or update tests in the package whose behavior changes.
- Do not weaken security defaults without an explicit issue or user request.
- Do not add new dependencies without a clear reason.
- Prefer library support before CLI-only behavior.
- For public API changes, update README examples and relevant docs in the same change.

## Test Expectations

Run for normal code changes:

```sh
go test ./...
go vet ./...
```

Run for decoder, parser, or safety-sensitive changes:

```sh
go test -race ./...
go test ./toon -run=Fuzz -fuzz=FuzzDecode -fuzztime=10s
```

Run for CLI changes:

```sh
go test ./cmd/toon
go build ./cmd/toon
```

Run for MCP changes:

```sh
go test ./mcp/go-toon-mcp
go build ./mcp/go-toon-mcp
```

## Test Placement

- Encoder changes: `toon/encode_test.go` and conformance tests.
- Decoder changes: `toon/decode_test.go` and conformance tests.
- Format normalization changes: `formats/formats_test.go`.
- CLI changes: `cmd/toon/main_test.go`.
- MCP changes: `mcp/go-toon-mcp/*_test.go`.
- Number canonicalization: `toon/foundation_test.go` and `toon/encode_test.go`.
- Resource limits: positive and rejection tests in the affected package.
