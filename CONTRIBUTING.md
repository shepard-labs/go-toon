# Contributing to go-toon

Thanks for your interest in contributing to `go-toon`, a Go implementation of Token-Oriented Object Notation (TOON) v3.3.

This guide describes the workflow, conventions, and verification gates contributors are expected to follow.

## Code Of Conduct

Be respectful and constructive. Focus on the technical merits of changes. Reports of harassment or abuse should be opened as a confidential issue tagged `conduct`.

## Project Scope

`go-toon` provides:

- An ordered-node library (`toon` package) implementing TOON v3.3.
- Format normalization helpers (`formats` package) for JSON, YAML, CSV, and XML.
- A `toon` CLI (`cmd/toon`) that wraps the library.

Out of scope:

- Reflection-heavy arbitrary `Marshal(any)` paths.
- CLI-only behavior with no library equivalent.
- Advertised streaming decode.

If a proposal does not fit the scope above, open an issue before writing code.

## Source References

Contributors should follow the normative TOON behavior described in:

- TOON format specification: `toon-format/spec`, `SPEC.md`, version `3.3`, dated `2026-05-21`.
- Reference implementation: `toon-format/toon`, TypeScript package `packages/toon`.
- Reference CLI behavior: `toon-format/toon`, TypeScript package `packages/cli`.

Where the reference implementation and the spec differ, this repository follows the spec. Cite section numbers in code comments only when they remove ambiguity.

## Getting Started

Prerequisites:

- Go matching `go.mod` (currently `go 1.26`).
- A POSIX-like shell for the verification commands below. Windows contributors may use Git Bash or WSL.

Clone and build:

```sh
git clone https://github.com/shepard-labs/go-toon
cd go-toon
go build ./...
go test ./...
```

Build the CLI:

```sh
go build ./cmd/toon
./toon --help
```

## Repository Layout

- `toon/` — core library: ordered node model, encoder, decoder, errors, limits, number canonicalization.
- `formats/` — JSON, YAML, CSV, XML normalization and ordered JSON output.
- `cmd/toon/` — CLI entry point.
- `.github/workflows/` — CI definitions.

## Workflow

1. Open an issue describing the change for anything beyond a trivial fix. Reference the spec section or phase spec when applicable.
2. Fork or create a branch off `main`.
3. Make focused commits. One logical change per commit is preferred.
4. Add or update tests for every behavior change.
5. Run the verification gates listed below.
6. Open a pull request that links the issue and explains spec or behavior implications.

Do not bundle unrelated changes. If a refactor is required to land a fix, split it into its own commit.

## Coding Conventions

Go style:

- Format with `gofmt`. Most editors do this on save.
- Run `go vet ./...` before sending a PR.
- Keep exported identifiers documented when their behavior is non-obvious.
- Prefer explicit error wrapping with `fmt.Errorf("...: %w", err)`.
- Do not introduce new dependencies without justification in the PR description.

Ordered model:

- Treat `*toon.Node` as the primary representation for ordered documents.
- Do not introduce Go `map` types in code paths that must preserve order.
- New options belong on `toon.EncodeOptions`, `toon.DecodeOptions`, or the matching `formats` option structs.

Errors:

- Return `toon.Error` with a stable `toon.ErrorCode` for library-visible errors.
- Branch on `toon.CodeOf(err)` or `errors.As`, not on error message text.
- Do not change existing error codes in patch releases.

Determinism:

- Encoder output must be deterministic: LF line endings, no trailing newline, no trailing spaces, ordered fields preserved.
- Patch releases must not change canonical output except to fix spec-compliance bugs. PRs that change golden files must justify each change against the spec.

Security defaults:

- Strict TOON decode is on by default.
- Safe key folding and safe path expansion are off by default.
- JSON duplicate keys are rejected by default.
- XML DTDs are rejected and external entities are not enabled.
- YAML aliases resolve through the AST with cycle rejection.
- Resource limits are configurable and the CLI uses safe defaults (`MaxDepth=512`, `MaxStringBytes=64 MiB`).

Do not weaken these defaults in a PR without an issue that documents the rationale.

## Tests

Test layout mirrors the package layout. New behavior needs new tests in the same package.

Required coverage by area:

- Encoder changes: add or update goldens in `toon/encode_test.go` and `toon/conformance_test.go`.
- Decoder changes: positive and strict-negative cases in `toon/decode_test.go` and `toon/conformance_test.go`.
- Format normalization changes: `formats/formats_test.go` with round trips where applicable.
- CLI changes: `cmd/toon/main_test.go` integration tests.
- Number canonicalization: `toon/foundation_test.go`, `toon/encode_test.go`.
- Resource limits: `toon/foundation_test.go`, `toon/encode_test.go`, `toon/decode_test.go`, `formats/formats_test.go`.
- Decoder behavior at boundaries: extend or seed `FuzzDecode` corpus when adding parser paths.

Tests should not depend on the host locale, timezone, or wall-clock time. Use deterministic fixtures.

## Verification Gates

Run these locally before requesting review:

```sh
go test ./...
go test -race ./...
go test ./toon -run=Fuzz -fuzz=FuzzDecode -fuzztime=10s
go test ./... -bench=. -run='^$' -benchtime=1x
```

For changes that touch `cmd/toon`, run the cross-platform build matrix:

```sh
GOOS=linux   GOARCH=amd64 go build ./cmd/toon
GOOS=linux   GOARCH=arm64 go build ./cmd/toon
GOOS=darwin  GOARCH=amd64 go build ./cmd/toon
GOOS=darwin  GOARCH=arm64 go build ./cmd/toon
GOOS=windows GOARCH=amd64 go build ./cmd/toon
```

CI replays the same gates. A PR is not ready for review until CI is green.

## Benchmarks

Performance-sensitive changes should include a before-and-after snapshot:

```sh
go test ./toon ./formats -bench=. -run='^$' -benchmem -count=5
```

Paste the relevant rows in the PR description. The README benchmark table is updated only by maintainers as part of a release.

## Documentation

Update documentation in the same PR as the behavior change:

- Public API changes: `README.md` Library API or CLI section.
- New options or flags: `README.md`.
- Default or security changes: `README.md` Defaults And Security.

Do not create new top-level documentation files for changes that fit into existing files.

## Commit Messages

Write commit messages in the imperative mood. The first line is a short summary. The body explains the why, the spec section if relevant, and any user-visible behavior change.

Example:

```
toon: reject NaN and Inf in NumberLossless mode

NumberLossless previously accepted non-finite float64 values when
constructed through formats.FromJSON. The TOON spec section 11 forbids
non-finite numbers in canonical output, so reject them at construction
time and return ErrNumberOutOfDomain.
```

Reference issues with `Fixes #N` or `Refs #N` on a trailing line.

## Pull Request Checklist

Before requesting review:

- [ ] Issue opened or linked for non-trivial changes.
- [ ] Tests added or updated for every behavior change.
- [ ] `go test ./...` passes.
- [ ] `go test -race ./...` passes.
- [ ] `go vet ./...` is clean.
- [ ] Fuzz smoke target passes for decoder changes.
- [ ] Cross-platform build matrix passes for `cmd/toon` changes.
- [ ] Documentation updated alongside the change.
- [ ] No unrelated changes in the diff.
- [ ] Canonical output changes are justified against the TOON spec.

## Versioning

`go-toon` follows semantic versioning:

- Patch releases fix bugs and spec-compliance issues without intentional API breaks.
- Minor releases may add options and non-breaking APIs.
- Major releases may track breaking TOON spec changes or public API changes.

Treat `toon.Node`, `toon.Field`, `toon.Number`, `toon.Kind`, the encode and decode entry points, `EncodeOptions`, `DecodeOptions`, `Error`, and `ErrorCode` as the stable API surface. Changes to their shape require a minor or major version bump. `toon/reflect` is stable from v1.0.0 — its `Marshal`, `Unmarshal`, `UnmarshalReader`, `UnmarshalNode`, `NodeFromValue`, `NodeToValue`, `ValueOptions`, `Options`, and `UnmarshalOptions` are part of the stable surface from this release onward.

## Security Issues

Do not file public issues for security-sensitive problems such as parser-driven panics on untrusted input, memory exhaustion, or sandbox escapes. Open a private security advisory on GitHub instead, or contact the maintainers listed in `go.mod` and the repository profile.

Include a minimal reproducer, the affected version, the resource limits in effect, and the expected behavior.

## License

By contributing, you agree that your contributions are licensed under the same license as the repository. See `LICENSE` if present; otherwise ask the maintainers before submitting non-trivial code.
