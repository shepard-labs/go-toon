# go-toon API Reference

Import the core package:

```go
import "github.com/shepard-labs/go-toon/toon"
```

## Core Types

- `toon.Node`: ordered representation of a TOON value.
- `toon.Field`: ordered object field.
- `toon.Number`: raw decimal token for lossless numbers.
- `toon.Kind`: node kind enum.

## Core Entry Points

```go
data, err := toon.Encode(n)
n, err := toon.Decode(data)
err := toon.Validate(data)
```

Writer and reader forms:

```go
err := toon.EncodeToWriter(w, n)
n, err := toon.DecodeReader(r)
```

`toon.SpecVersion` is `"4.4"`. Decode is strict by default; pass `func(o *toon.DecodeOptions) { o.Strict = false }` to apply only the spec's non-strict recoveries.

## Options

- `toon.EncodeOptions`: `IndentSize`, `Delimiter` (`toon.Comma`, `toon.Tab`, `toon.Pipe`), `NumberMode`, `Limits`.
- `toon.DecodeOptions`: `IndentSize`, `Strict`, `Limits`.
- `IncludeLengthMarkers`, `KeyFolding`, `FlattenDepth`, and `ExpandPaths` are deprecated and ignored. Dotted keys are always literal.

## Numbers

Decoded numbers keep their token text in `Number.Raw`, including values outside the `float64` range such as `1e500`. `EncodeOptions.NumberMode` controls the encoder:

- `toon.NumberLossless` (default): writes the canonical decimal token (`1e500` → `1e+500`).
- `toon.NumberFloat64`: rounds through `float64`; values outside its range return an error.
- `toon.NumberStringForUnsafe`: writes values outside the `float64` range as quoted strings.

## UTF-8

Decode rejects ill-formed UTF-8 with `toon.ErrInvalidUTF8`. Encoding a string value or object key with ill-formed UTF-8 returns the same `toon.ErrInvalidUTF8` error.

## Format Conversion

```go
import "github.com/shepard-labs/go-toon/formats"
```

```go
n, err := formats.FromJSON(r)
n, err := formats.FromYAML(r)
n, err := formats.FromCSV(r)
n, err := formats.FromXML(r)
err := formats.ToJSON(w, n)
```

Use these conversions instead of Go maps when order matters. `formats.FromJSON` rejects lone surrogate escapes. `formats.TOONToJSON` with zero-value options decodes in strict mode.

## Struct Convenience

```go
import toonreflect "github.com/shepard-labs/go-toon/toon/reflect"
```

```go
n, err := toonreflect.NodeFromValue(v)
err := toonreflect.NodeToValue(n, &dst)
err := toonreflect.Marshal(w, v)
err := toonreflect.Unmarshal(data, &dst)
```

## Errors

Branch on error codes, not error strings:

```go
if code := toon.CodeOf(err); code == toon.ErrDuplicateKey {
    // handle duplicate key
}
```

Or use `errors.As` with `*toon.Error` to access line, column, code, message, context, and cause.
