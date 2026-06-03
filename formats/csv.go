package formats

import (
	"encoding/csv"
	"io"

	"github.com/shepard-labs/go-toon/toon"
)

func FromCSV(r io.Reader, opts ...CSVOption) (*toon.Node, error) {
	o := resolveCSVOptions(opts...)
	if o.HeaderMode == CSVHeaderExplicit || o.HeaderMode == CSVHeaderAuto {
		return nil, toon.NewError(toon.ErrUnsupportedFeature, "CSV header mode must be explicit: present or absent")
	}
	reader := csv.NewReader(r)
	reader.Comma = o.Delimiter
	reader.FieldsPerRecord = -1
	lim := newLimits(o.Limits)
	root := &toon.Node{Kind: toon.ArrayKind}
	if err := lim.node(root, 0); err != nil {
		return nil, err
	}
	first, err := reader.Read()
	if err == io.EOF {
		return root, nil
	}
	if err != nil {
		return nil, formatErr("invalid CSV", err)
	}
	var headers []string
	if o.HeaderMode == CSVHeaderPresent {
		headers = first
		if hasDuplicate(headers) {
			return nil, duplicateErr("duplicate CSV header")
		}
	} else {
		for i := range first {
			headers = append(headers, intFieldName(i+1))
		}
		if err := appendCSVRecord(root, first, headers, o, lim); err != nil {
			return nil, err
		}
	}
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, formatErr("invalid CSV", err)
		}
		if err := appendCSVRecord(root, record, headers, o, lim); err != nil {
			return nil, err
		}
	}
	return root, nil
}

func appendCSVRecord(root *toon.Node, record, headers []string, o CSVOptions, lim limits) error {
	if !o.AllowRaggedRows && len(record) != len(headers) {
		return formatErr("ragged CSV row", nil)
	}
	obj := &toon.Node{Kind: toon.ObjectKind}
	if err := lim.node(obj, 1); err != nil {
		return err
	}
	width := min(len(record), len(headers))
	for i := 0; i < width; i++ {
		value := stringNode(record[i])
		if o.InferTypes && record[i] != "" {
			value = inferCell(record[i])
		}
		if err := lim.node(value, 2); err != nil {
			return err
		}
		obj.Object = append(obj.Object, toon.Field{Key: headers[i], Value: value})
	}
	root.Array = append(root.Array, obj)
	return lim.c.CheckArrayLength(len(root.Array))
}
