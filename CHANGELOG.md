# Changelog

All notable changes to `go-toon` are listed here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [1.1.0]

Targets TOON spec 4.4 (`toon-spec: 4.4`, `toon.SpecVersion = "4.4"`).

### Added

- Keyed tabular arrays: `users[2:]{age,city}:` with entry rows `alice: 30,Oslo`, and the root form `[2:]{age,city}:`. The encoder emits them for objects of two or more entries whose values are non-empty objects with the same keys.
- Nested field groups in tabular headers, for example `users[2]{id,name,geo{lat,lng}}:`. Rows stay flat and cells follow depth-first order.
- Comment lines: a line whose first non-space character is `#` is dropped by the decoder.
- `toon.ErrInvalidUTF8` (`invalid_utf8`): the decoder rejects ill-formed UTF-8 in both modes and strips a single leading byte-order mark.
- The vendored TOON v4.4 spec fixtures in `toon/testdata/spec/`, the `toon/spec_fixtures_test.go` runner, and `scripts/sync-spec-fixtures.sh` (pinned to `v4.4.0`).
- `formats.TOONToJSONOptions.NonStrict` turns strict decoding off; strict stays the default even when `Decode` sets other fields, and `Decode.Strict` is ignored.
- Round-trip tests in `toon/reflect` for maps and structs that encode as keyed tabular arrays and nested field groups.

### Changed

- Encoder output follows spec 4.4:
  - The first field of an object list item is always on the hyphen line, an empty object is a bare `-`, and nested arrays in list items use `- [M]:` (an empty inner array is `- [0]:`).
  - Control characters escape as `\u00xx` with lowercase hex; strings starting with `#` and number-like strings with a leading plus are quoted.
- Encoding a string value or object key that contains ill-formed UTF-8 (invalid bytes or unpaired surrogates) returns an `invalid_utf8` error.
- Decoder behavior follows spec 4.4:
  - Only SP and HTAB count as whitespace; NBSP is content.
  - Lines split on LF, with one trailing CR dropped per line.
  - A line whose first unquoted `[` precedes its first unquoted colon must be a valid header.
  - Row-width mismatches, trailing content after a complete root array, and scalar lines outside the root position are errors in both modes.
  - Non-strict mode applies only the five recoveries of spec §14.4.
  - Only tokens matching the number grammar decode as numbers (`+1` and `NaN` are strings).
- Numbers outside the `float64` range decode with their token text kept in `Number.Raw`. `NumberLossless` writes the canonical decimal token, `NumberFloat64` returns an error for such values, and `NumberStringForUnsafe` writes them as quoted strings.
- `formats.FromJSON` rejects lone surrogate escapes instead of replacing them with U+FFFD.
- `formats.TOONToJSON` with zero-value options decodes in strict mode.
- Dotted keys are always literal on encode and decode.
- `toon_spec_lookup`, `toon_examples`, and `toon_error_explain` in `go-toon-mcp` cover keyed tabular arrays, nested field groups, and comments.

### Deprecated

- `EncodeOptions.IncludeLengthMarkers`, `EncodeOptions.KeyFolding`, `EncodeOptions.FlattenDepth`, `DecodeOptions.ExpandPaths`, and the `KeyFoldingMode` and `PathExpansionMode` types. The fields stay on the structs and are ignored.
- `toon.ErrPathExpansionConflict` is no longer produced.

### Removed

- Key folding and path expansion behavior.
- The `#N` length marker.
- CLI flags `--key-folding`, `--flatten-depth`, `--length-markers`, and `--expand-paths`.
- The matching arguments of the `go-toon-mcp` tools.
