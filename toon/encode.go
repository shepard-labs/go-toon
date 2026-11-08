package toon

import (
	"bytes"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"
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
		if cols, ok := keyedColumns(n.Object); ok {
			return e.writeKeyed(nil, n.Object, cols, 0)
		}
		return e.writeObjectFields(n.Object, 0)
	case ArrayKind:
		if len(n.Array) == 0 {
			e.buf.WriteString("[]")
			return nil
		}
		return e.writeArray(nil, n.Array, 0, true)
	default:
		var value string
		var err error
		if n.Kind == StringKind && strings.HasPrefix(n.String, "\uFEFF") {
			value = quote(n.String)
		} else {
			value, err = e.primitive(n, e.opts.Delimiter)
		}
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
	return e.writeFieldBody(field, depth)
}

// writeFieldBody writes a field from its key onward; the caller has already
// positioned the cursor (indentation or list-item hyphen). Any scope the field
// opens has its content at depth+1.
func (e *encoder) writeFieldBody(field Field, depth int) error {
	value := field.Value
	if value == nil {
		value = &Node{Kind: NullKind}
	}
	switch value.Kind {
	case ObjectKind:
		if cols, ok := keyedColumns(value.Object); ok {
			return e.writeKeyed(&field.Key, value.Object, cols, depth)
		}
		e.buf.WriteString(encodeKey(field.Key))
		e.buf.WriteByte(':')
		if len(value.Object) == 0 {
			return nil
		}
		e.buf.WriteByte('\n')
		return e.writeObjectFields(value.Object, depth+1)
	case ArrayKind:
		if len(value.Array) == 0 {
			e.buf.WriteString(encodeKey(field.Key))
			e.buf.WriteString(": []")
			return nil
		}
		return e.writeArray(&field.Key, value.Array, depth, true)
	default:
		primitive, err := e.primitive(value, e.opts.Delimiter)
		if err != nil {
			return err
		}
		e.buf.WriteString(encodeKey(field.Key))
		e.buf.WriteString(": ")
		e.buf.WriteString(primitive)
		return nil
	}
}

// writeKeyed writes an object in keyed tabular form: a keyed header followed
// by one entry row per field at depth+1.
func (e *encoder) writeKeyed(key *string, fields []Field, cols []column, depth int) error {
	e.writeHeader(key, len(fields), true, cols)
	e.buf.WriteByte(':')
	for _, field := range fields {
		e.buf.WriteByte('\n')
		e.indent(depth + 1)
		e.buf.WriteString(encodeKey(field.Key))
		e.buf.WriteString(": ")
		if err := e.writeCells(field.Value.Object, cols, false); err != nil {
			return err
		}
	}
	return nil
}

// writeArray writes a non-empty array whose header sits at depth, with the
// content one level deeper. Tabular form is only available where a keyless
// fields-bearing header is valid: with a key, or at the document root.
func (e *encoder) writeArray(key *string, items []*Node, depth int, tabular bool) error {
	if allPrimitive(items) {
		e.writeHeader(key, len(items), false, nil)
		e.buf.WriteString(": ")
		return e.writePrimitiveList(items, e.opts.Delimiter)
	}
	if tabular && allNonEmptyObjects(items) {
		if cols, ok := classifyColumns(items); ok {
			e.writeHeader(key, len(items), false, cols)
			e.buf.WriteByte(':')
			for _, item := range items {
				e.buf.WriteByte('\n')
				e.indent(depth + 1)
				if err := e.writeCells(item.Object, cols, false); err != nil {
					return err
				}
			}
			return nil
		}
	}
	e.writeHeader(key, len(items), false, nil)
	e.buf.WriteByte(':')
	for _, item := range items {
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
		if len(n.Object) == 0 {
			return nil
		}
		// The first field stands on the hyphen line but counts as depth+1,
		// so scope content it opens lands at depth+2 (SPEC §10).
		e.buf.WriteByte(' ')
		if err := e.writeFieldBody(n.Object[0], depth+1); err != nil {
			return err
		}
		for _, rest := range n.Object[1:] {
			e.buf.WriteByte('\n')
			if err := e.writeField(rest, depth+1); err != nil {
				return err
			}
		}
		return nil
	case ArrayKind:
		e.buf.WriteByte(' ')
		if len(n.Array) == 0 {
			e.writeHeader(nil, 0, false, nil)
			e.buf.WriteByte(':')
			return nil
		}
		return e.writeArray(nil, n.Array, depth, false)
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

func (e *encoder) writeHeader(key *string, length int, keyed bool, cols []column) {
	if key != nil {
		e.buf.WriteString(encodeKey(*key))
	}
	e.buf.WriteByte('[')
	e.buf.WriteString(strconv.Itoa(length))
	if keyed {
		e.buf.WriteByte(':')
	}
	if e.opts.Delimiter != Comma {
		e.buf.WriteByte(byte(e.opts.Delimiter))
	}
	e.buf.WriteByte(']')
	if len(cols) > 0 {
		e.buf.WriteByte('{')
		e.writeFieldList(cols)
		e.buf.WriteByte('}')
	}
}

func (e *encoder) writeFieldList(cols []column) {
	for i, col := range cols {
		if i > 0 {
			e.buf.WriteByte(byte(e.opts.Delimiter))
		}
		e.buf.WriteString(encodeKey(col.key))
		if col.sub != nil {
			e.buf.WriteByte('{')
			e.writeFieldList(col.sub)
			e.buf.WriteByte('}')
		}
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

// writeCells writes the leaf values of an object row in depth-first pre-order
// of cols. started reports whether a cell was already written on this line.
func (e *encoder) writeCells(fields []Field, cols []column, started bool) error {
	for i, col := range cols {
		value := fieldValue(fields, i, col.key)
		if col.sub != nil {
			if err := e.writeCells(value.Object, col.sub, started || i > 0); err != nil {
				return err
			}
			continue
		}
		if started || i > 0 {
			e.buf.WriteByte(byte(e.opts.Delimiter))
		}
		cell, err := e.primitive(value, e.opts.Delimiter)
		if err != nil {
			return err
		}
		e.buf.WriteString(cell)
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
		canonical, quoted, err := canonicalizeNumber(n.Number.Raw, e.opts.NumberMode)
		if err != nil {
			return "", err
		}
		if quoted {
			return quote(canonical), nil
		}
		return canonical, nil
	case StringKind:
		if NeedsQuotes(n.String, delimiter) {
			return quote(n.String), nil
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
		if !utf8.ValidString(n.String) {
			return invalidUTF8Error("string value")
		}
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
			if !utf8.ValidString(field.Key) {
				return invalidUTF8Error("object key")
			}
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

func encodeKey(key string) string {
	if IsValidUnquotedKey(key) {
		return key
	}
	return quote(key)
}

func quote(s string) string {
	return `"` + EscapeString(s) + `"`
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

// invalidUTF8Error reports a string or key that is not a sequence of Unicode
// scalar values (ill-formed UTF-8, including encoded surrogates), which TOON
// cannot represent (SPEC §3).
func invalidUTF8Error(what string) error {
	return Errorf(ErrInvalidUTF8, "%s is not valid UTF-8 (ill-formed bytes or unpaired surrogate)", what)
}

// column is one field of a tabular header. A column with sub set is a nested
// field group: its values are objects sharing the sub-columns' key set.
type column struct {
	key string
	sub []column
}

// classifyColumns derives the shared column tree of objs (SPEC §9.3). Every
// element must already be a non-empty object. It reports false unless all
// objects have the same key set and every column is uniform-primitive or
// nested-uniform. Field order is that of the first object, at every level.
func classifyColumns(objs []*Node) ([]column, bool) {
	keys := objs[0].Object
	if hasDuplicateKeys(keys) {
		return nil, false
	}
	for _, o := range objs[1:] {
		if !sameKeySet(keys, o.Object) {
			return nil, false
		}
	}
	cols := make([]column, len(keys))
	for i, f := range keys {
		cols[i].key = f.Key
		primitives, objects := true, true
		for _, o := range objs {
			v := fieldValue(o.Object, i, f.Key)
			primitives = primitives && isPrimitive(v)
			objects = objects && isNonEmptyObject(v)
		}
		switch {
		case primitives:
		case objects:
			nested := make([]*Node, len(objs))
			for j, o := range objs {
				nested[j] = fieldValue(o.Object, i, f.Key)
			}
			sub, ok := classifyColumns(nested)
			if !ok {
				return nil, false
			}
			cols[i].sub = sub
		default:
			return nil, false
		}
	}
	return cols, true
}

// keyedColumns reports whether an object qualifies for keyed tabular form
// (SPEC §9.5) and returns its columns.
func keyedColumns(fields []Field) ([]column, bool) {
	if len(fields) < 2 || hasDuplicateKeys(fields) {
		return nil, false
	}
	values := make([]*Node, len(fields))
	for i, f := range fields {
		if !isNonEmptyObject(f.Value) {
			return nil, false
		}
		values[i] = f.Value
	}
	return classifyColumns(values)
}

// fieldValue returns the value stored under key, trying position i first since
// objects in a table usually share the header's field order.
func fieldValue(fields []Field, i int, key string) *Node {
	if i < len(fields) && fields[i].Key == key {
		return fields[i].Value
	}
	for _, f := range fields {
		if f.Key == key {
			return f.Value
		}
	}
	return nil
}

func hasDuplicateKeys(fields []Field) bool {
	seen := make(map[string]struct{}, len(fields))
	for _, f := range fields {
		if _, dup := seen[f.Key]; dup {
			return true
		}
		seen[f.Key] = struct{}{}
	}
	return false
}

// sameKeySet reports whether a and b hold the same keys in any order. a must
// not contain duplicate keys.
func sameKeySet(a, b []Field) bool {
	if len(a) != len(b) {
		return false
	}
	inOrder := true
	for i := range a {
		if a[i].Key != b[i].Key {
			inOrder = false
			break
		}
	}
	if inOrder {
		return true
	}
	remaining := make(map[string]struct{}, len(a))
	for _, f := range a {
		remaining[f.Key] = struct{}{}
	}
	for _, f := range b {
		if _, ok := remaining[f.Key]; !ok {
			return false
		}
		delete(remaining, f.Key)
	}
	return true
}

func isNonEmptyObject(n *Node) bool {
	return n != nil && n.Kind == ObjectKind && len(n.Object) > 0
}

func allNonEmptyObjects(nodes []*Node) bool {
	for _, n := range nodes {
		if !isNonEmptyObject(n) {
			return false
		}
	}
	return len(nodes) > 0
}
