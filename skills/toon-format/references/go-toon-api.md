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

Use these conversions instead of Go maps when order matters.

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
