// Package reflect provides a reflection-based convenience layer above the
// toon.Node ordered document model. It lets callers encode Go values into
// TOON and decode *toon.Node values back into Go values without manually
// constructing the ordered node graph.
//
// The core toon package owns the stable, ordered *toon.Node model. This
// subpackage is a thin projection that walks Go values via the standard
// library reflect package and produces *toon.Node graphs (encode) or
// populates Go destinations from a *toon.Node (decode).
//
// Behavior contract:
//   - Accepted Go kinds: nil, bool, string, []byte, all int/uint/float types
//     and their named derivatives, []T/[N]T, map[string]V, struct.
//   - Map keys are sorted lexicographically so encode output is deterministic
//     regardless of Go's randomized map iteration order.
//   - Struct fields are emitted in declaration order. The optional "toon"
//     tag may rename a field ("toon:\"name\"") or skip it ("toon:\"-\"").
//     Unexported fields are skipped.
//   - The walker enforces the same ResourceLimits and NumberMode policies
//     as the rest of the toon package.
//   - The decode side requires a non-nil pointer destination, populates
//     structs and maps, and supports encoding.TextUnmarshaler as an
//     escape hatch for custom types.
package reflect

import (
	"io"
	"reflect"
	"unsafe"

	"github.com/shepard-labs/go-toon/toon"
)

// ValueOptions configures the value-walker behavior shared by NodeFromValue
// and NodeToValue.
type ValueOptions struct {
	// TagName is the struct tag consulted for field rename and skip directives.
	// Default: "toon". Set to "-" or empty string to disable tag inspection.
	TagName string
	// Limits are the resource limits applied while walking the value graph.
	// A zero ResourceLimits disables limit enforcement.
	Limits toon.ResourceLimits
	// NumberMode controls how non-finite float values are handled during
	// encode. It does not affect decode (decode parses numbers losslessly).
	NumberMode toon.NumberMode
}

// Options configures the Marshal convenience function. It bundles value-walker
// options with encoder options so the call site does not mix two variadic
// option lists.
type Options struct {
	Value  ValueOptions
	Encode toon.EncodeOptions
}

// UnmarshalOptions configures the Unmarshal convenience function. It bundles
// value-walker options with decoder options.
type UnmarshalOptions struct {
	Value  ValueOptions
	Decode toon.DecodeOptions
}

// ValueOption mutates a ValueOptions value.
type ValueOption func(*ValueOptions)

// Option mutates a Options value.
type Option func(*Options)

// UnmarshalOption mutates a UnmarshalOptions value.
type UnmarshalOption func(*UnmarshalOptions)

// DefaultValueOptions returns the default value-walker options.
func DefaultValueOptions() ValueOptions {
	return ValueOptions{
		TagName:    "toon",
		NumberMode: toon.NumberLossless,
	}
}

// DefaultOptions returns the default options for Marshal.
func DefaultOptions() Options {
	return Options{
		Value:  DefaultValueOptions(),
		Encode: toon.DefaultEncodeOptions(),
	}
}

// DefaultUnmarshalOptions returns the default options for Unmarshal.
func DefaultUnmarshalOptions() UnmarshalOptions {
	return UnmarshalOptions{
		Value:  DefaultValueOptions(),
		Decode: toon.DefaultDecodeOptions(),
	}
}

// NodeFromValue walks v and produces an ordered *toon.Node tree. v may be
// any value reflect.ValueOf accepts, including nil. The returned *toon.Node
// is suitable for direct use with toon.Encode.
func NodeFromValue(v any, opts ...ValueOption) (*toon.Node, error) {
	o := resolveValueOptions(opts...)
	limits := toon.NewLimitCounter(o.Limits)
	w := &valueWalker{opts: o, limits: limits, seen: map[unsafe.Pointer]struct{}{}}
	return w.fromValue(reflectValueOf(v), 0)
}

// Marshal encodes v as TOON and writes the result to w. It is a convenience
// wrapper that calls NodeFromValue followed by toon.EncodeToWriter.
func Marshal(w io.Writer, v any, opts ...Option) error {
	o := resolveOptions(opts...)
	n, err := NodeFromValue(v, func(vo *ValueOptions) { *vo = o.Value })
	if err != nil {
		return err
	}
	return toon.EncodeToWriter(w, n, func(eo *toon.EncodeOptions) { mergeEncodeOptions(eo, o.Encode) })
}

// NodeToValue walks the ordered *toon.Node n and populates v. v must be a
// non-nil pointer to a destination value (struct, slice, map, primitive).
func NodeToValue(n *toon.Node, v any, opts ...ValueOption) error {
	if v == nil {
		return toon.NewError(toon.ErrNonPointerTarget, "destination must not be nil")
	}
	rv := reflectValueOf(v)
	if rv.Kind() != ptrKind {
		return toon.NewError(toon.ErrNonPointerTarget, "destination must be a pointer")
	}
	if rv.IsNil() {
		return toon.NewError(toon.ErrNonPointerTarget, "destination must not be a nil pointer")
	}
	return nodeToValueOf(n, rv.Elem(), opts...)
}

// nodeToValueOf is the internal entry point that takes a reflect.Value
// directly. It is used by Unmarshal/UnmarshalNode to avoid double pointer
// dereferencing.
func nodeToValueOf(n *toon.Node, v reflect.Value, opts ...ValueOption) error {
	o := resolveValueOptions(opts...)
	limits := toon.NewLimitCounter(o.Limits)
	w := &nodeWalker{limits: limits}
	return w.toValue(n, v, 0)
}

// Unmarshal decodes TOON data and populates the destination pointed to by v.
// v must be a non-nil pointer. Strict decoding is enabled by default; the
// same DecodeOptions as toon.Decode are accepted.
func Unmarshal(data []byte, v any, opts ...UnmarshalOption) error {
	if v == nil {
		return toon.NewError(toon.ErrNonPointerTarget, "destination must not be nil")
	}
	rv := reflectValueOf(v)
	if rv.Kind() != ptrKind {
		return toon.NewError(toon.ErrNonPointerTarget, "destination must be a pointer")
	}
	if rv.IsNil() {
		return toon.NewError(toon.ErrNonPointerTarget, "destination must not be a nil pointer")
	}
	o := resolveUnmarshalOptions(opts...)
	n, err := toon.Decode(data, func(d *toon.DecodeOptions) {
		*d = o.Decode
		if (o.Value.Limits != toon.ResourceLimits{}) {
			d.Limits = mergeResourceLimits(d.Limits, o.Value.Limits)
		}
	})
	if err != nil {
		return err
	}
	return nodeToValueOf(n, rv.Elem(), func(vo *ValueOptions) { *vo = o.Value })
}

// UnmarshalReader reads TOON from r and populates v.
func UnmarshalReader(r io.Reader, v any, opts ...UnmarshalOption) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	return Unmarshal(data, v, opts...)
}

// UnmarshalNode populates v from an already-decoded *toon.Node.
func UnmarshalNode(n *toon.Node, v any, opts ...UnmarshalOption) error {
	if v == nil {
		return toon.NewError(toon.ErrNonPointerTarget, "destination must not be nil")
	}
	rv := reflectValueOf(v)
	if rv.Kind() != ptrKind {
		return toon.NewError(toon.ErrNonPointerTarget, "destination must be a pointer")
	}
	if rv.IsNil() {
		return toon.NewError(toon.ErrNonPointerTarget, "destination must not be a nil pointer")
	}
	o := resolveUnmarshalOptions(opts...)
	return nodeToValueOf(n, rv.Elem(), func(vo *ValueOptions) { *vo = o.Value })
}

func resolveValueOptions(opts ...ValueOption) ValueOptions {
	o := DefaultValueOptions()
	for _, opt := range opts {
		if opt != nil {
			opt(&o)
		}
	}
	return o
}

func resolveOptions(opts ...Option) Options {
	o := DefaultOptions()
	for _, opt := range opts {
		if opt != nil {
			opt(&o)
		}
	}
	return o
}

func resolveUnmarshalOptions(opts ...UnmarshalOption) UnmarshalOptions {
	o := DefaultUnmarshalOptions()
	for _, opt := range opts {
		if opt != nil {
			opt(&o)
		}
	}
	return o
}

func mergeEncodeOptions(dst *toon.EncodeOptions, src toon.EncodeOptions) {
	if src.IndentSize != 0 {
		dst.IndentSize = src.IndentSize
	}
	if src.Delimiter != 0 {
		dst.Delimiter = src.Delimiter
	}
	if src.KeyFolding != 0 {
		dst.KeyFolding = src.KeyFolding
	}
	if src.FlattenDepth != 0 {
		dst.FlattenDepth = src.FlattenDepth
	}
	if src.NumberMode != 0 {
		dst.NumberMode = src.NumberMode
	}
	if (src.Limits != toon.ResourceLimits{}) {
		dst.Limits = src.Limits
	}
}

func mergeResourceLimits(dst, src toon.ResourceLimits) toon.ResourceLimits {
	if src.MaxDepth > 0 {
		dst.MaxDepth = src.MaxDepth
	}
	if src.MaxNodes > 0 {
		dst.MaxNodes = src.MaxNodes
	}
	if src.MaxBytes > 0 {
		dst.MaxBytes = src.MaxBytes
	}
	if src.MaxStringBytes > 0 {
		dst.MaxStringBytes = src.MaxStringBytes
	}
	if src.MaxArrayLength > 0 {
		dst.MaxArrayLength = src.MaxArrayLength
	}
	return dst
}
