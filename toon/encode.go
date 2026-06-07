package toon

import (
	"bytes"
	"io"
	"slices"
	"strconv"
)

func Encode(n *Node, opts ...EncodeOption) ([]byte, error) {
	options := ResolveEncodeOptions(opts...)
	if options.IndentSize <= 0 {
		return nil, NewError(ErrInvalidIndent, "indent size must be positive")
	}
	enc := &encoder{opts: options, limits: NewLimitCounter(options.Limits)}
	if n == nil {
		n = &Node{Kind: NullKind}
	}
	if err := enc.checkNode(n, 0); err != nil {
		return nil, err
	}
	if err := enc.writeRoot(n); err != nil {
		return nil, err
	}
	return enc.buf.Bytes(), nil
}

func EncodeToWriter(w io.Writer, n *Node, opts ...EncodeOption) error {
	data, err := Encode(n, opts...)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

type encoder struct {
	opts   EncodeOptions
	limits *LimitCounter
	buf    bytes.Buffer
}

func (e *encoder) writeRoot(n *Node) error {
	switch n.Kind {
	case ObjectKind:
		fields := e.foldFields(n.Object, 0)
		return e.writeObjectFields(fields, 0)
	case ArrayKind:
		return e.writeArray(nil, n, 0, false)
	default:
		value, err := e.primitive(n, e.opts.Delimiter)
		if err != nil {
			return err
		}
		e.buf.WriteString(value)
		return nil
	}
}

func (e *encoder) writeObjectFields(fields []Field, depth int) error {
	for i, field := range fields {
		if i > 0 {
			e.buf.WriteByte('\n')
		}
		if err := e.writeField(field, depth); err != nil {
			return err
		}
	}
	return nil
}

func (e *encoder) writeField(field Field, depth int) error {
	e.indent(depth)
	value := field.Value
	if value == nil {
		value = &Node{Kind: NullKind}
	}
	switch value.Kind {
	case ObjectKind:
		e.buf.WriteString(encodeKey(field.Key))
		e.buf.WriteByte(':')
		fields := e.foldFields(value.Object, depth+1)
		if len(fields) == 0 {
			return nil
		}
		e.buf.WriteByte('\n')
		return e.writeObjectFields(fields, depth+1)
	case ArrayKind:
		return e.writeArray(&field.Key, value, depth, false)
	default:
		e.buf.WriteString(encodeKey(field.Key))
		primitive, err := e.primitive(value, e.opts.Delimiter)
		if err != nil {
			return err
		}
		e.buf.WriteString(": ")
		e.buf.WriteString(primitive)
		return nil
	}
}

func (e *encoder) writeArray(key *string, n *Node, depth int, listItem bool) error {
	if len(n.Array) == 0 {
		if key == nil {
			e.buf.WriteString("[]")
		} else {
			e.buf.WriteString(encodeKey(*key))
			e.buf.WriteString(": []")
		}
		return nil
	}
	if allPrimitive(n.Array) {
		e.writeHeader(key, len(n.Array), e.opts.Delimiter, nil)
		e.buf.WriteString(": ")
		return e.writePrimitiveList(n.Array, e.opts.Delimiter)
	}
	if !listItem {
		if fields, ok := tabularFields(n.Array); ok {
			e.writeHeader(key, len(n.Array), e.opts.Delimiter, fields)
			e.buf.WriteByte(':')
			for _, item := range n.Array {
				e.buf.WriteByte('\n')
				e.indent(depth + 1)
				if err := e.writeTabularRow(item, fields); err != nil {
					return err
				}
			}
			return nil
		}
	}
	e.writeHeader(key, len(n.Array), e.opts.Delimiter, nil)
	e.buf.WriteByte(':')
	for _, item := range n.Array {
		e.buf.WriteByte('\n')
		if err := e.writeListItem(item, depth+1); err != nil {
			return err
		}
	}
	return nil
}

func (e *encoder) writeListItem(n *Node, depth int) error {
	if n == nil {
		n = &Node{Kind: NullKind}
	}
	e.indent(depth)
	e.buf.WriteByte('-')
	switch n.Kind {
	case ObjectKind:
		fields := e.foldFields(n.Object, depth+1)
		if len(fields) == 0 {
			return nil
		}
		first := fields[0]
		value := first.Value
		if value == nil {
			value = &Node{Kind: NullKind}
		}
		if value.Kind == ArrayKind && len(value.Array) > 0 {
			if tabFields, ok := tabularFields(value.Array); ok {
				e.buf.WriteByte(' ')
				if err := e.writeArray(&first.Key, value, depth, false); err != nil {
					return err
				}
				for _, rest := range fields[1:] {
					e.buf.WriteByte('\n')
					if err := e.writeField(rest, depth+1); err != nil {
						return err
					}
				}
				_ = tabFields
				return nil
			}
		}
		if isPrimitive(value) {
			primitive, err := e.primitive(value, e.opts.Delimiter)
			if err != nil {
				return err
			}
			e.buf.WriteByte(' ')
			e.buf.WriteString(encodeKey(first.Key))
			e.buf.WriteString(": ")
			e.buf.WriteString(primitive)
		} else {
			e.buf.WriteByte('\n')
			if err := e.writeField(first, depth+1); err != nil {
				return err
			}
		}
		for _, rest := range fields[1:] {
			e.buf.WriteByte('\n')
			if err := e.writeField(rest, depth+1); err != nil {
				return err
			}
		}
		return nil
	case ArrayKind:
		if allPrimitive(n.Array) {
			e.buf.WriteByte(' ')
			return e.writeArray(nil, n, depth, true)
		}
		e.buf.WriteByte('\n')
		return e.writeArray(nil, n, depth+1, true)
	default:
		primitive, err := e.primitive(n, e.opts.Delimiter)
		if err != nil {
			return err
		}
		e.buf.WriteByte(' ')
		e.buf.WriteString(primitive)
		return nil
	}
}

func (e *encoder) writeHeader(key *string, length int, delimiter Delimiter, fields []string) {
	if key != nil {
		e.buf.WriteString(encodeKey(*key))
	}
	e.buf.WriteByte('[')
	if e.opts.IncludeLengthMarkers {
		e.buf.WriteByte('#')
	}
	e.buf.WriteString(strconv.Itoa(length))
	if delimiter != Comma {
		e.buf.WriteByte(byte(delimiter))
	}
	e.buf.WriteByte(']')
	if len(fields) > 0 {
		e.buf.WriteByte('{')
		for i, field := range fields {
			if i > 0 {
				e.buf.WriteByte(byte(delimiter))
			}
			e.buf.WriteString(encodeKey(field))
		}
		e.buf.WriteByte('}')
	}
}

func (e *encoder) writePrimitiveList(nodes []*Node, delimiter Delimiter) error {
	for i, n := range nodes {
		if i > 0 {
			e.buf.WriteByte(byte(delimiter))
		}
		value, err := e.primitive(n, delimiter)
		if err != nil {
			return err
		}
		e.buf.WriteString(value)
	}
	return nil
}

func (e *encoder) writeTabularRow(n *Node, fields []string) error {
	for i := range fields {
		if i > 0 {
			e.buf.WriteByte(byte(e.opts.Delimiter))
		}
		value, err := e.primitive(n.Object[i].Value, e.opts.Delimiter)
		if err != nil {
			return err
		}
		e.buf.WriteString(value)
	}
	return nil
}

func (e *encoder) primitive(n *Node, delimiter Delimiter) (string, error) {
	if n == nil {
		return "null", nil
	}
	switch n.Kind {
	case NullKind:
		return "null", nil
	case BoolKind:
		if n.Bool {
			return "true", nil
		}
		return "false", nil
	case NumberKind:
		canonical, quote, err := canonicalizeNumber(n.Number.Raw, e.opts.NumberMode)
		if err != nil {
			return "", err
		}
		if quote {
			return `"` + EscapeString(canonical) + `"`, nil
		}
		return canonical, nil
	case StringKind:
		if NeedsQuotes(n.String, delimiter) {
			return `"` + EscapeString(n.String) + `"`, nil
		}
		return n.String, nil
	default:
		return "", NewError(ErrUnsupportedFeature, "node kind cannot be encoded as primitive")
	}
}

func (e *encoder) indent(depth int) {
	for i := 0; i < depth*e.opts.IndentSize; i++ {
		e.buf.WriteByte(' ')
	}
}

func (e *encoder) checkNode(n *Node, depth int) error {
	if n == nil {
		return e.limits.AddNode()
	}
	if err := e.limits.AddNode(); err != nil {
		return err
	}
	if err := e.limits.CheckDepth(depth); err != nil {
		return err
	}
	switch n.Kind {
	case StringKind:
		return e.limits.CheckStringBytes(len(n.String))
	case ArrayKind:
		if err := e.limits.CheckArrayLength(len(n.Array)); err != nil {
			return err
		}
		for _, item := range n.Array {
			if err := e.checkNode(item, depth+1); err != nil {
				return err
			}
		}
	case ObjectKind:
		for _, field := range n.Object {
			if err := e.limits.CheckStringBytes(len(field.Key)); err != nil {
				return err
			}
			if err := e.checkNode(field.Value, depth+1); err != nil {
				return err
			}
		}
	}
	return nil
}

func (e *encoder) foldFields(fields []Field, depth int) []Field {
	if e.opts.KeyFolding != KeyFoldingSafe || depth > e.opts.FlattenDepth {
		return fields
	}
	literalKeys := make([]string, 0, len(fields))
	for _, field := range fields {
		literalKeys = append(literalKeys, field.Key)
	}
	result := make([]Field, 0, len(fields))
	for _, field := range fields {
		folded := e.foldField(field, 1)
		if folded.Key != field.Key && containsKey(literalKeys, folded.Key) {
			folded = field
		}
		result = append(result, folded)
	}
	return result
}

func (e *encoder) foldField(field Field, segments int) Field {
	if segments == 1 && !IsValidSafeSegment(field.Key) {
		return field
	}
	if segments >= e.opts.FlattenDepth || field.Value == nil || field.Value.Kind != ObjectKind || len(field.Value.Object) != 1 {
		return field
	}
	next := field.Value.Object[0]
	if !IsValidSafeSegment(next.Key) {
		return field
	}
	folded := Field{Key: field.Key + "." + next.Key, Value: next.Value}
	if next.Value != nil && next.Value.Kind == ObjectKind && len(next.Value.Object) == 1 && segments+1 < e.opts.FlattenDepth {
		return e.foldField(folded, segments+1)
	}
	return folded
}

func encodeKey(key string) string {
	if IsValidUnquotedKey(key) {
		return key
	}
	return `"` + EscapeString(key) + `"`
}

func allPrimitive(nodes []*Node) bool {
	if len(nodes) == 0 {
		return false
	}
	for _, n := range nodes {
		if !isPrimitive(n) {
			return false
		}
	}
	return true
}

func isPrimitive(n *Node) bool {
	return n == nil || n.Kind == NullKind || n.Kind == BoolKind || n.Kind == NumberKind || n.Kind == StringKind
}

func tabularFields(nodes []*Node) ([]string, bool) {
	if len(nodes) == 0 || nodes[0] == nil || nodes[0].Kind != ObjectKind || len(nodes[0].Object) == 0 {
		return nil, false
	}
	fields := make([]string, len(nodes[0].Object))
	for i, field := range nodes[0].Object {
		if !isPrimitive(field.Value) {
			return nil, false
		}
		fields[i] = field.Key
	}
	for _, node := range nodes[1:] {
		if node == nil || node.Kind != ObjectKind || len(node.Object) != len(fields) {
			return nil, false
		}
		for i, field := range node.Object {
			if field.Key != fields[i] || !isPrimitive(field.Value) {
				return nil, false
			}
		}
	}
	return fields, true
}

func containsKey(keys []string, key string) bool {
	return slices.Contains(keys, key)
}
