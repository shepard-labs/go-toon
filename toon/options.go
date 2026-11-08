package toon

import "math"

type Delimiter byte

const (
	Comma Delimiter = ','
	Tab   Delimiter = '\t'
	Pipe  Delimiter = '|'
)

// KeyFoldingMode is retained for source compatibility only.
//
// Deprecated: TOON v4 has no key folding; the encoder ignores it.
type KeyFoldingMode int

const (
	KeyFoldingOff KeyFoldingMode = iota
	KeyFoldingSafe
)

// PathExpansionMode is retained for source compatibility only.
//
// Deprecated: TOON v4 has no path expansion; the decoder ignores it.
type PathExpansionMode int

const (
	ExpandPathsOff PathExpansionMode = iota
	ExpandPathsSafe
)

type NumberMode int

const (
	NumberLossless NumberMode = iota
	NumberFloat64
	NumberStringForUnsafe
)

type EncodeOptions struct {
	IndentSize int
	Delimiter  Delimiter
	// Deprecated: the #N length marker was removed from TOON; ignored.
	IncludeLengthMarkers bool
	// Deprecated: key folding was removed from TOON; ignored.
	KeyFolding KeyFoldingMode
	// Deprecated: key folding was removed from TOON; ignored.
	FlattenDepth int
	NumberMode   NumberMode
	Limits       ResourceLimits
}

type DecodeOptions struct {
	IndentSize int
	Strict     bool
	// Deprecated: path expansion was removed from TOON; ignored.
	ExpandPaths PathExpansionMode
	Limits      ResourceLimits
}

type EncodeOption func(*EncodeOptions)
type DecodeOption func(*DecodeOptions)

func DefaultEncodeOptions() EncodeOptions {
	return EncodeOptions{
		IndentSize:   2,
		Delimiter:    Comma,
		KeyFolding:   KeyFoldingOff,
		FlattenDepth: math.MaxInt,
		NumberMode:   NumberLossless,
	}
}

func DefaultDecodeOptions() DecodeOptions {
	return DecodeOptions{
		IndentSize:  2,
		Strict:      true,
		ExpandPaths: ExpandPathsOff,
	}
}

func ResolveEncodeOptions(opts ...EncodeOption) EncodeOptions {
	resolved := DefaultEncodeOptions()
	for _, opt := range opts {
		if opt != nil {
			opt(&resolved)
		}
	}
	return resolved
}

func ResolveDecodeOptions(opts ...DecodeOption) DecodeOptions {
	resolved := DefaultDecodeOptions()
	for _, opt := range opts {
		if opt != nil {
			opt(&resolved)
		}
	}
	return resolved
}
