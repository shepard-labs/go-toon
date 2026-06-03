package formats

import (
	"io"

	"github.com/shepard-labs/go-toon/toon"
)

func JSONToTOON(r io.Reader, w io.Writer, opts JSONToTOONOptions) error {
	n, err := FromJSON(r, func(o *JSONOptions) { *o = opts.JSON })
	if err != nil {
		return err
	}
	return toon.EncodeToWriter(w, n, func(o *toon.EncodeOptions) { mergeEncodeOptions(o, opts.Encode) })
}

func YAMLToTOON(r io.Reader, w io.Writer, opts YAMLToTOONOptions) error {
	n, err := FromYAML(r, func(o *YAMLOptions) { *o = opts.YAML })
	if err != nil {
		return err
	}
	return toon.EncodeToWriter(w, n, func(o *toon.EncodeOptions) { mergeEncodeOptions(o, opts.Encode) })
}

func XMLToTOON(r io.Reader, w io.Writer, opts XMLToTOONOptions) error {
	xmlOpts := []XMLOption{}
	if opts.XML != (XMLOptions{}) {
		xmlOpts = append(xmlOpts, func(o *XMLOptions) { *o = opts.XML })
	}
	n, err := FromXML(r, xmlOpts...)
	if err != nil {
		return err
	}
	return toon.EncodeToWriter(w, n, func(o *toon.EncodeOptions) { mergeEncodeOptions(o, opts.Encode) })
}

func CSVToTOON(r io.Reader, w io.Writer, opts CSVToTOONOptions) error {
	n, err := FromCSV(r, func(o *CSVOptions) { *o = opts.CSV })
	if err != nil {
		return err
	}
	return toon.EncodeToWriter(w, n, func(o *toon.EncodeOptions) { mergeEncodeOptions(o, opts.Encode) })
}

func TOONToJSON(r io.Reader, w io.Writer, opts TOONToJSONOptions) error {
	n, err := toon.DecodeReader(r, func(o *toon.DecodeOptions) { mergeDecodeOptions(o, opts.Decode) })
	if err != nil {
		return err
	}
	return ToJSON(w, n, func(o *JSONOutputOptions) { *o = opts.JSON })
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
	if src.Limits != (toon.ResourceLimits{}) {
		dst.Limits = src.Limits
	}
}

func mergeDecodeOptions(dst *toon.DecodeOptions, src toon.DecodeOptions) {
	if src.IndentSize != 0 {
		dst.IndentSize = src.IndentSize
	}
	if !src.Strict {
		dst.Strict = false
	}
	if src.ExpandPaths != 0 {
		dst.ExpandPaths = src.ExpandPaths
	}
	if src.Limits != (toon.ResourceLimits{}) {
		dst.Limits = src.Limits
	}
}
