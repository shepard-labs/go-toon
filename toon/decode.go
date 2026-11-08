package toon

import (
	"bytes"
	"io"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"
)

// ParsedLine is one line of the comment-stripped, blank-free line sequence that
// the decoder classifies (SPEC §12 line preparation).
type ParsedLine struct {
	Raw        string
	Content    string
	Depth      int
	LineNumber int
	// Deprecated: never set; blank lines are not part of the line sequence.
	Blank bool
	// BlankBefore is the line number of the first blank line directly before
	// this line (comment lines are transparent), or 0.
	BlankBefore int
}

// ArrayHeader is a decoded array or keyed-tabular header (SPEC §6).
type ArrayHeader struct {
	Key       *string
	Length    int
	Delimiter Delimiter
	// Keyed marks the keyed-tabular form key[N:]{...}: (SPEC §9.5).
	Keyed bool
	// Fields lists the leaf field names in depth-first header order.
	Fields []string

	keyQuoted bool
	ops       []fieldOp
	dup       bool
	rest      string
}

type fieldOpKind uint8

const (
	opLeaf fieldOpKind = iota
	opOpen
	opClose
)

// fieldOp is one step of the depth-first walk over a header's field list: a
// leaf field takes the next cell, opOpen/opClose bracket a nested field group.
type fieldOp struct {
	name   string
	quoted bool
	kind   fieldOpKind
}

func Decode(data []byte, opts ...DecodeOption) (*Node, error) {
	options := ResolveDecodeOptions(opts...)
	if options.IndentSize <= 0 {
		return nil, NewError(ErrInvalidIndent, "indent size must be positive")
	}
	dec := &decoder{opts: options, limits: NewLimitCounter(options.Limits), guard: math.MaxInt}
	if err := dec.limits.CheckInputBytes(int64(len(data))); err != nil {
		return nil, err
	}
	lines, err := dec.scan(data)
	if err != nil {
		return nil, err
	}
	dec.lines = lines
	return dec.parseDocument()
}

func DecodeReader(r io.Reader, opts ...DecodeOption) (*Node, error) {
	// Read one byte past MaxBytes so Decode reports an oversized input without
	// buffering the rest of the stream.
	if limit := ResolveDecodeOptions(opts...).Limits.MaxBytes; limit > 0 && limit < math.MaxInt64 {
		r = io.LimitReader(r, limit+1)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return Decode(data, opts...)
}

func Validate(data []byte, opts ...DecodeOption) error {
	_, err := Decode(data, opts...)
	return err
}

type decoder struct {
	opts   DecodeOptions
	limits *LimitCounter
	lines  []ParsedLine
	pos    int
	// guard is the smallest content depth among the array scopes that have
	// started reading rows, entries or items. A blank line followed by a line at
	// or below that depth lies inside a header span (SPEC §12).
	guard int
	// headerOps totals the field-list entries of every header parsed so far.
	headerOps int
}

// scan performs the line preparation of SPEC §12: UTF-8 validation, BOM, CR,
// trailing spaces, comment lines, blank lines, indentation.
func (d *decoder) scan(data []byte) ([]ParsedLine, error) {
	if !utf8.Valid(data) {
		off := firstInvalidUTF8(data)
		return nil, &Error{Code: ErrInvalidUTF8, Line: 1 + bytes.Count(data[:off], []byte{'\n'}), Message: "input is not valid UTF-8"}
	}
	s := strings.TrimPrefix(string(data), byteOrderMark)
	var lines []ParsedLine
	pendingBlank := 0
	lineNumber := 0
	for start := 0; start < len(s); {
		lineNumber++
		end := strings.IndexByte(s[start:], '\n')
		if end < 0 {
			end = len(s)
		} else {
			end += start
		}
		raw := s[start:end]
		start = end + 1
		raw = strings.TrimRight(strings.TrimSuffix(raw, "\r"), " ")
		if raw == "" {
			if pendingBlank == 0 {
				pendingBlank = lineNumber
			}
			continue
		}
		if isCommentLine(raw) {
			continue
		}
		spaces, tabs := 0, 0
		run := 0
		for run < len(raw) && (raw[run] == ' ' || raw[run] == '\t') {
			if raw[run] == ' ' {
				spaces++
			} else {
				tabs++
			}
			run++
		}
		content := raw[run:]
		depth := 0
		if d.opts.Strict {
			if tabs > 0 {
				return nil, &Error{Code: ErrTabIndent, Line: lineNumber, Column: run, Message: "tab indentation is not allowed", Context: raw}
			}
			if spaces%d.opts.IndentSize != 0 {
				return nil, &Error{Code: ErrInvalidIndent, Line: lineNumber, Column: 1, Message: "indentation is not a multiple of indent size", Context: raw}
			}
			depth = spaces / d.opts.IndentSize
		} else {
			if content == "" {
				if pendingBlank == 0 {
					pendingBlank = lineNumber
				}
				continue
			}
			depth = tabs + spaces/d.opts.IndentSize
		}
		lines = append(lines, ParsedLine{Raw: raw, Content: content, Depth: depth, LineNumber: lineNumber, BlankBefore: pendingBlank})
		pendingBlank = 0
	}
	return lines, nil
}

// byteOrderMark is U+FEFF in UTF-8.
const byteOrderMark = "\xEF\xBB\xBF"

func isCommentLine(raw string) bool {
	i := 0
	for i < len(raw) && raw[i] == ' ' {
		i++
	}
	return i < len(raw) && raw[i] == '#'
}

func firstInvalidUTF8(data []byte) int {
	for i := 0; i < len(data); {
		r, size := utf8.DecodeRune(data[i:])
		if r == utf8.RuneError && size <= 1 {
			return i
		}
		i += size
	}
	return len(data)
}

// peek returns the next unconsumed line, or nil at end of input. In strict mode
// it rejects a line that follows a blank line inside a header span.
func (d *decoder) peek() (*ParsedLine, error) {
	if d.pos >= len(d.lines) {
		return nil, nil
	}
	ln := &d.lines[d.pos]
	if d.opts.Strict && ln.BlankBefore > 0 && ln.Depth >= d.guard {
		return nil, &Error{Code: ErrInvalidInputFormat, Line: ln.BlankBefore, Message: "blank line inside array"}
	}
	return ln, nil
}

// nextInScope returns the next line of the scope opened by a line at depth f
// whose content stands at depth m, or nil once the scope has ended.
func (d *decoder) nextInScope(f, m int) (*ParsedLine, error) {
	ln, err := d.peek()
	if err != nil || ln == nil || ln.Depth <= f {
		return nil, err
	}
	if ln.Depth < m {
		return nil, lineError(ln, ErrInvalidIndent, "line is shallower than the scope's content depth")
	}
	if ln.Depth > m {
		return nil, lineError(ln, ErrInvalidIndent, "unexpected indentation")
	}
	return ln, nil
}

// scopeDepth finds the content depth of the scope opened by a line at depth f.
// ok is false when no following line is deeper than f. A first line deeper than
// f+1 is a depth jump: an error in strict mode, adopted as the content depth
// otherwise (SPEC §8, §14.4 #5).
func (d *decoder) scopeDepth(f int) (m int, ok bool, err error) {
	ln, err := d.peek()
	if err != nil || ln == nil || ln.Depth <= f {
		return 0, false, err
	}
	if ln.Depth != f+1 && d.opts.Strict {
		return 0, false, lineError(ln, ErrInvalidIndent, "indentation depth jump")
	}
	return ln.Depth, true, nil
}

func (d *decoder) enterScope(m int) {
	if m < d.guard {
		d.guard = m
	}
}

func (d *decoder) parseDocument() (*Node, error) {
	if len(d.lines) == 0 {
		n := &Node{Kind: ObjectKind}
		return n, d.accountNode(n, 0)
	}
	first := &d.lines[0]
	if first.Depth != 0 {
		return nil, lineError(first, ErrInvalidIndent, "root content must start at depth 0")
	}
	content := first.Content
	bracket, colon := scanBracketColon(content)
	switch {
	case colon >= 0 && bracket == 0:
		h, err := d.parseHeader(content)
		if err != nil {
			return nil, lineError(first, CodeOf(err), err.Error())
		}
		d.pos = 1
		n, err := d.parseArrayBody(h, first, 0, 0)
		if err != nil {
			return nil, err
		}
		return n, d.checkNoTrailing()
	case colon < 0 && content == "[]":
		d.pos = 1
		n := &Node{Kind: ArrayKind}
		if err := d.accountNode(n, 0); err != nil {
			return nil, err
		}
		return n, d.checkNoTrailing()
	case colon < 0 && len(d.lines) == 1:
		return d.primitiveNode(content, first, 0)
	}
	obj := &Node{Kind: ObjectKind}
	if err := d.accountNode(obj, 0); err != nil {
		return nil, err
	}
	var seen fieldSet
	if err := d.parseObjectBody(obj, &seen, 0, -1, 0); err != nil {
		return nil, err
	}
	return obj, nil
}

// checkNoTrailing enforces that nothing follows a completed root array, empty
// root array or keyed tabular root (SPEC §5).
func (d *decoder) checkNoTrailing() error {
	if d.pos < len(d.lines) {
		return lineError(&d.lines[d.pos], ErrInvalidInputFormat, "unexpected content after the root value")
	}
	return nil
}

// parseObjectBody reads fields at content depth m for a scope opened at depth f
// (f is -1 for the root object) into obj.
func (d *decoder) parseObjectBody(obj *Node, seen *fieldSet, m, f, lvl int) error {
	for {
		ln, err := d.nextInScope(f, m)
		if err != nil || ln == nil {
			return err
		}
		d.pos++
		if err := d.parseField(ln, ln.Content, m, obj, seen, lvl); err != nil {
			return err
		}
	}
}

// parseField parses the key-value or keyed-header field carried by content,
// which stands at depth fdepth, and adds it to obj. The line has been consumed.
func (d *decoder) parseField(ln *ParsedLine, content string, fdepth int, obj *Node, seen *fieldSet, lvl int) error {
	bracket, colon := scanBracketColon(content)
	if colon < 0 {
		return lineError(ln, ErrMissingColon, "expected key: value")
	}
	if bracket >= 0 {
		h, err := d.parseHeader(content)
		if err != nil {
			return lineError(ln, CodeOf(err), err.Error())
		}
		if h.Key == nil {
			return lineError(ln, ErrMalformedHeader, "array header without a key is not allowed here")
		}
		value, err := d.parseArrayBody(h, ln, fdepth, lvl+1)
		if err != nil {
			return err
		}
		return d.addField(obj, Field{Key: *h.Key, WasQuoted: h.keyQuoted, Value: value}, seen, ln)
	}
	key, quoted, err := parseKeyToken(content[:colon])
	if err != nil {
		return lineError(ln, CodeOf(err), err.Error())
	}
	if err := d.limits.CheckStringBytes(len(key)); err != nil {
		return err
	}
	field := Field{Key: key, WasQuoted: quoted}
	switch rest := trimSpaces(content[colon+1:]); rest {
	case "":
		child := &Node{Kind: ObjectKind}
		if err := d.accountNode(child, lvl+1); err != nil {
			return err
		}
		m, ok, err := d.scopeDepth(fdepth)
		if err != nil {
			return err
		}
		if ok {
			var childSeen fieldSet
			if err := d.parseObjectBody(child, &childSeen, m, fdepth, lvl+1); err != nil {
				return err
			}
		}
		field.Value = child
	case "[]":
		field.Value = &Node{Kind: ArrayKind}
		if err := d.accountNode(field.Value, lvl+1); err != nil {
			return err
		}
	default:
		field.Value, err = d.primitiveNode(rest, ln, lvl+1)
		if err != nil {
			return err
		}
	}
	return d.addField(obj, field, seen, ln)
}

// parseArrayBody decodes the array (or keyed tabular object) a header opens.
// The header line has been consumed; f is the depth its scope hangs from and
// lvl the nesting level of the resulting node.
func (d *decoder) parseArrayBody(h *ArrayHeader, ln *ParsedLine, f, lvl int) (*Node, error) {
	if d.opts.Strict {
		if err := d.limits.CheckArrayLength(h.Length); err != nil {
			return nil, err
		}
	}
	if h.Keyed {
		return d.parseKeyedBody(h, ln, f, lvl)
	}
	if len(h.ops) > 0 {
		return d.parseTabularBody(h, ln, f, lvl)
	}
	arr := &Node{Kind: ArrayKind}
	if err := d.accountNode(arr, lvl); err != nil {
		return nil, err
	}
	if inline := trimSpaces(h.rest); inline != "" {
		tokens, _ := SplitDelimited(inline, h.Delimiter)
		if d.opts.Strict && len(tokens) != h.Length {
			return nil, lineError(ln, ErrArrayCountMismatch, "inline array count mismatch")
		}
		for _, token := range tokens {
			if err := d.limits.CheckArrayLength(len(arr.Array) + 1); err != nil {
				return nil, err
			}
			item, err := d.primitiveNode(token, ln, lvl+1)
			if err != nil {
				return nil, err
			}
			arr.Array = append(arr.Array, item)
		}
		return arr, nil
	}
	return arr, d.parseListBody(arr, h, ln, f, lvl)
}

func (d *decoder) parseListBody(arr *Node, h *ArrayHeader, ln *ParsedLine, f, lvl int) error {
	m, ok, err := d.scopeDepth(f)
	if err != nil {
		return err
	}
	count := 0
	if ok {
		saved := d.guard
		defer func() { d.guard = saved }()
		for {
			item, err := d.nextInScope(f, m)
			if err != nil {
				return err
			}
			if item == nil || !isListItem(item.Content) {
				break
			}
			d.pos++
			d.enterScope(m)
			if err := d.limits.CheckArrayLength(count + 1); err != nil {
				return err
			}
			node, err := d.parseListItem(item, m, lvl+1)
			if err != nil {
				return err
			}
			arr.Array = append(arr.Array, node)
			count++
		}
	}
	if d.opts.Strict && count != h.Length {
		return lineError(ln, ErrArrayCountMismatch, "list array count mismatch")
	}
	return nil
}

// isListItem reports whether content is the bare marker "-" or "-" followed by
// spaces (SPEC §5.2 class 2).
func isListItem(content string) bool {
	return content == "-" || strings.HasPrefix(content, "- ")
}

// parseListItem decodes the list item on ln (already consumed), whose hyphen
// stands at depth dep (SPEC §9.2, §9.4, §10).
func (d *decoder) parseListItem(ln *ParsedLine, dep, lvl int) (*Node, error) {
	rest := ""
	if len(ln.Content) > 1 {
		rest = strings.TrimLeft(ln.Content[1:], " ")
	}
	if rest == "" {
		n := &Node{Kind: ObjectKind}
		return n, d.accountNode(n, lvl)
	}
	bracket, colon := scanBracketColon(rest)
	switch {
	case colon >= 0 && bracket == 0:
		h, err := d.parseHeader(rest)
		if err != nil {
			return nil, lineError(ln, CodeOf(err), err.Error())
		}
		if h.Keyed || len(h.ops) > 0 {
			return nil, lineError(ln, ErrMalformedHeader, "a header with a field list is only valid as the document's root header")
		}
		return d.parseArrayBody(h, ln, dep, lvl)
	case colon >= 0:
		obj := &Node{Kind: ObjectKind}
		if err := d.accountNode(obj, lvl); err != nil {
			return nil, err
		}
		var seen fieldSet
		if err := d.parseField(ln, rest, dep+1, obj, &seen, lvl); err != nil {
			return nil, err
		}
		return obj, d.parseObjectBody(obj, &seen, dep+1, dep, lvl)
	case rest == "[]":
		n := &Node{Kind: ArrayKind}
		return n, d.accountNode(n, lvl)
	}
	return d.primitiveNode(rest, ln, lvl)
}

func (d *decoder) parseTabularBody(h *ArrayHeader, ln *ParsedLine, f, lvl int) (*Node, error) {
	arr := &Node{Kind: ArrayKind}
	if err := d.accountNode(arr, lvl); err != nil {
		return nil, err
	}
	m, ok, err := d.scopeDepth(f)
	if err != nil {
		return nil, err
	}
	rows := 0
	if ok {
		saved := d.guard
		defer func() { d.guard = saved }()
		for {
			row, err := d.nextInScope(f, m)
			if err != nil {
				return nil, err
			}
			if row == nil || !isTabularRow(row.Content, h.Delimiter) {
				break
			}
			d.pos++
			d.enterScope(m)
			cells, _ := SplitDelimited(row.Content, h.Delimiter)
			if len(cells) != len(h.Fields) {
				return nil, lineError(row, ErrTabularWidthMismatch, "tabular row width mismatch")
			}
			if err := d.limits.CheckArrayLength(rows + 1); err != nil {
				return nil, err
			}
			obj, err := d.buildRecord(h, cells, row, lvl+1)
			if err != nil {
				return nil, err
			}
			arr.Array = append(arr.Array, obj)
			rows++
		}
	}
	if d.opts.Strict && rows != h.Length {
		return nil, lineError(ln, ErrArrayCountMismatch, "tabular row count mismatch")
	}
	return arr, nil
}

// isTabularRow applies the row disambiguation of SPEC §9.3.
func isTabularRow(content string, delimiter Delimiter) bool {
	colon := findUnquoted(content, ':')
	if colon < 0 {
		return true
	}
	delim := findUnquoted(content, byte(delimiter))
	return delim >= 0 && delim < colon
}

func (d *decoder) parseKeyedBody(h *ArrayHeader, ln *ParsedLine, f, lvl int) (*Node, error) {
	obj := &Node{Kind: ObjectKind}
	if err := d.accountNode(obj, lvl); err != nil {
		return nil, err
	}
	m, ok, err := d.scopeDepth(f)
	if err != nil {
		return nil, err
	}
	entries := 0
	var seen fieldSet
	if ok {
		saved := d.guard
		defer func() { d.guard = saved }()
		for {
			row, err := d.nextInScope(f, m)
			if err != nil || row == nil {
				if err != nil {
					return nil, err
				}
				break
			}
			colon := findUnquoted(row.Content, ':')
			if colon < 0 {
				return nil, lineError(row, ErrMissingColon, "entry row without a colon")
			}
			d.pos++
			d.enterScope(m)
			key, quoted, err := parseKeyToken(row.Content[:colon])
			if err != nil {
				return nil, lineError(row, CodeOf(err), err.Error())
			}
			if err := d.limits.CheckStringBytes(len(key)); err != nil {
				return nil, err
			}
			var cells []string
			if rest := trimSpaces(row.Content[colon+1:]); rest != "" {
				cells, _ = SplitDelimited(rest, h.Delimiter)
			}
			if len(cells) != len(h.Fields) {
				return nil, lineError(row, ErrTabularWidthMismatch, "entry row width mismatch")
			}
			if err := d.limits.CheckArrayLength(entries + 1); err != nil {
				return nil, err
			}
			value, err := d.buildRecord(h, cells, row, lvl+1)
			if err != nil {
				return nil, err
			}
			if err := d.addField(obj, Field{Key: key, WasQuoted: quoted, Value: value}, &seen, row); err != nil {
				return nil, err
			}
			entries++
		}
	}
	if d.opts.Strict && entries != h.Length {
		return nil, lineError(ln, ErrArrayCountMismatch, "keyed tabular entry count mismatch")
	}
	return obj, nil
}

// buildRecord materializes one row or entry value by walking the header's field
// list in order: a leaf field takes the next cell, a nested group an object.
func (d *decoder) buildRecord(h *ArrayHeader, cells []string, ln *ParsedLine, lvl int) (*Node, error) {
	root := &Node{Kind: ObjectKind}
	if err := d.accountNode(root, lvl); err != nil {
		return nil, err
	}
	var stackBuf [8]*Node
	stack := stackBuf[:0]
	top := root
	next := 0
	for _, op := range h.ops {
		switch op.kind {
		case opLeaf:
			value, err := d.primitiveNode(cells[next], ln, lvl+len(stack)+1)
			if err != nil {
				return nil, err
			}
			next++
			setField(top, Field{Key: op.name, WasQuoted: op.quoted, Value: value}, h.dup)
		case opOpen:
			child := &Node{Kind: ObjectKind}
			if err := d.accountNode(child, lvl+len(stack)+1); err != nil {
				return nil, err
			}
			setField(top, Field{Key: op.name, WasQuoted: op.quoted, Value: child}, h.dup)
			stack = append(stack, top)
			top = child
		case opClose:
			top = stack[len(stack)-1]
			stack = stack[:len(stack)-1]
		}
	}
	return root, nil
}

// setField appends field to obj; with dup set, a field of the same name is
// replaced in place instead (last write wins, first position: SPEC §14.4 #2).
func setField(obj *Node, field Field, dup bool) {
	if dup {
		for i := range obj.Object {
			if obj.Object[i].Key == field.Key {
				obj.Object[i] = field
				return
			}
		}
	}
	obj.Object = append(obj.Object, field)
}

func (d *decoder) primitiveNode(token string, ln *ParsedLine, lvl int) (*Node, error) {
	n, err := ParsePrimitiveToken(token)
	if err != nil {
		return nil, lineError(ln, CodeOf(err), err.Error())
	}
	if err := d.accountNode(n, lvl); err != nil {
		return nil, err
	}
	if n.Kind == StringKind {
		if err := d.limits.CheckStringBytes(len(n.String)); err != nil {
			return nil, err
		}
	}
	return n, nil
}

func (d *decoder) accountNode(n *Node, depth int) error {
	if err := d.limits.AddNode(); err != nil {
		return err
	}
	return d.limits.CheckDepth(depth)
}

// addField appends field to obj, applying the duplicate-key rule of SPEC §14.3
// (error) or, in non-strict mode, §14.4 #2 (last write wins, first position).
func (d *decoder) addField(obj *Node, field Field, seen *fieldSet, ln *ParsedLine) error {
	if i, ok := seen.lookup(field.Key); ok {
		if d.opts.Strict {
			return lineError(ln, ErrDuplicateKey, "duplicate key")
		}
		obj.Object[i] = field
		return nil
	}
	seen.add(field.Key, len(obj.Object))
	obj.Object = append(obj.Object, field)
	return nil
}

// fieldSet maps keys to their field index; the first few keys live in an array
// so small objects allocate nothing.
type fieldSet struct {
	keys  [8]string
	count int
	index map[string]int
}

func (t *fieldSet) lookup(key string) (int, bool) {
	if t.index != nil {
		i, ok := t.index[key]
		return i, ok
	}
	for i := 0; i < t.count; i++ {
		if t.keys[i] == key {
			return i, true
		}
	}
	return 0, false
}

// add records key at field position pos. Positions are assigned in insertion
// order, so in array mode the slot number is the position.
func (t *fieldSet) add(key string, pos int) {
	if t.index != nil {
		t.index[key] = pos
		return
	}
	if t.count < len(t.keys) {
		t.keys[t.count] = key
		t.count++
		return
	}
	t.index = make(map[string]int, len(t.keys)*2)
	for i := 0; i < t.count; i++ {
		t.index[t.keys[i]] = i
	}
	t.index[key] = pos
}

func lineError(line *ParsedLine, code ErrorCode, message string) *Error {
	if code == "" {
		code = ErrInvalidInputFormat
	}
	return &Error{Code: code, Line: line.LineNumber, Message: message, Context: line.Raw}
}

// parseKeyToken decodes a key token per SPEC §7.4: surrounding spaces are
// trimmed; a token starting with '"' must be a complete quoted token; anything
// else is a literal key. An empty token yields the empty key; callers that
// cannot accept it check for it.
func parseKeyToken(token string) (key string, quoted bool, err error) {
	token = trimSpaces(token)
	if token != "" && token[0] == '"' {
		key, err = UnescapeQuotedToken(token)
		return key, true, err
	}
	return token, false, nil
}

// parseHeader parses a header line and charges its field-list entries against
// the node limit.
func (d *decoder) parseHeader(content string) (*ArrayHeader, error) {
	h, err := parseHeader(content, d.opts.Strict, d.limits)
	if err != nil {
		return nil, err
	}
	d.headerOps += len(h.ops)
	if limit := d.limits.Limits.MaxNodes; limit > 0 && d.headerOps > limit {
		return nil, Errorf(ErrResourceLimit, "node limit exceeded: %d header fields > %d", d.headerOps, limit)
	}
	return h, nil
}

// parseHeader parses an array or keyed-tabular header line (SPEC §6). content
// must contain an unquoted '[' before its first unquoted colon. limits may be
// nil.
func parseHeader(content string, strict bool, limits *LimitCounter) (*ArrayHeader, error) {
	b := findUnquoted(content, '[')
	if b < 0 {
		return nil, NewError(ErrMalformedHeader, "malformed array header")
	}
	h := &ArrayHeader{Delimiter: Comma}
	if keyPart := content[:b]; keyPart != "" {
		if c := keyPart[len(keyPart)-1]; c == ' ' || c == '\t' {
			return nil, NewError(ErrMalformedHeader, "whitespace between key and bracket segment")
		}
		key, quoted, err := parseKeyToken(keyPart)
		if err != nil {
			return nil, err
		}
		if err := limits.CheckStringBytes(len(key)); err != nil {
			return nil, err
		}
		h.Key, h.keyQuoted = &key, quoted
	}
	i := skipDigits(content, b+1)
	digits := content[b+1 : i]
	if digits == "" || (len(digits) > 1 && digits[0] == '0') {
		return nil, NewError(ErrMalformedHeader, "malformed header length")
	}
	length, err := strconv.Atoi(digits)
	if err != nil {
		length = math.MaxInt
	}
	h.Length = length
	if i < len(content) && content[i] == ':' {
		h.Keyed = true
		i++
	}
	if i < len(content) {
		switch content[i] {
		case '\t':
			h.Delimiter = Tab
			i++
		case '|':
			h.Delimiter = Pipe
			i++
		}
	}
	if i >= len(content) || content[i] != ']' {
		return nil, NewError(ErrMalformedHeader, "malformed bracket segment")
	}
	i++
	if i < len(content) && content[i] == '{' {
		var end int
		h.ops, end, h.dup, err = parseFieldList(content, i, h.Delimiter, strict, limits)
		if err != nil {
			return nil, err
		}
		for _, op := range h.ops {
			if op.kind == opLeaf {
				h.Fields = append(h.Fields, op.name)
			}
		}
		i = end
	}
	if i >= len(content) || content[i] != ':' {
		return nil, NewError(ErrMalformedHeader, "array header missing colon")
	}
	h.rest = content[i+1:]
	if h.Keyed && len(h.ops) == 0 {
		return nil, NewError(ErrMalformedHeader, "keyed header requires a field list")
	}
	if len(h.ops) > 0 && trimSpaces(h.rest) != "" {
		return nil, NewError(ErrMalformedHeader, "unexpected content after a header with a field list")
	}
	return h, nil
}

// parseFieldList parses the field list that opens at s[i] == '{', including
// nested field groups, into the depth-first op sequence. end is the index just
// past the closing brace. dup reports a field name repeated within one list
// (an error in strict mode). Group nesting and field count are checked against
// limits, which may be nil, while the list is read.
func parseFieldList(s string, i int, delim Delimiter, strict bool, limits *LimitCounter) (ops []fieldOp, end int, dup bool, err error) {
	var stack []*fieldSet
	var names fieldSet
	cur := &names
	stack = append(stack, cur)
	i++
	afterGroup := false
	register := func(name string) error {
		if _, seen := cur.lookup(name); seen {
			if strict {
				return NewError(ErrDuplicateKey, "duplicate field name in field list")
			}
			dup = true
			return nil
		}
		cur.add(name, 0)
		return nil
	}
	for {
		start := i
		for i < len(s) {
			c := s[i]
			if c == '"' {
				i = skipQuoted(s, i)
				continue
			}
			if c == byte(delim) || c == '{' || c == '}' {
				break
			}
			if c == ',' || c == '\t' || c == '|' {
				return nil, 0, false, NewError(ErrHeaderDelimiterMismatch, "header delimiter mismatch")
			}
			i++
		}
		if i >= len(s) {
			return nil, 0, false, NewError(ErrMalformedHeader, "unmatched brace in field list")
		}
		token, c := s[start:i], s[i]
		switch {
		case afterGroup:
			if c == '{' || trimSpaces(token) != "" {
				return nil, 0, false, NewError(ErrMalformedHeader, "unexpected content after a nested field group")
			}
			afterGroup = false
		case c == '{':
			if token != "" && (token[len(token)-1] == ' ' || token[len(token)-1] == '\t') {
				return nil, 0, false, NewError(ErrMalformedHeader, "whitespace between a field name and its field group")
			}
			name, quoted, err := fieldName(token, limits)
			if err != nil {
				return nil, 0, false, err
			}
			if err := register(name); err != nil {
				return nil, 0, false, err
			}
			ops = append(ops, fieldOp{name: name, quoted: quoted, kind: opOpen})
			if err := checkFieldList(limits, len(stack), len(ops)); err != nil {
				return nil, 0, false, err
			}
			cur = &fieldSet{}
			stack = append(stack, cur)
			i++
			continue
		default:
			name, quoted, err := fieldName(token, limits)
			if err != nil {
				return nil, 0, false, err
			}
			if err := register(name); err != nil {
				return nil, 0, false, err
			}
			ops = append(ops, fieldOp{name: name, quoted: quoted, kind: opLeaf})
			if err := checkFieldList(limits, len(stack)-1, len(ops)); err != nil {
				return nil, 0, false, err
			}
		}
		i++
		if c == byte(delim) {
			continue
		}
		stack = stack[:len(stack)-1]
		if len(stack) == 0 {
			return ops, i, dup, nil
		}
		cur = stack[len(stack)-1]
		ops = append(ops, fieldOp{kind: opClose})
		afterGroup = true
	}
}

// checkFieldList applies the depth limit to the field-group nesting and the
// node limit to the number of field-list entries read so far.
func checkFieldList(limits *LimitCounter, groupDepth, ops int) error {
	if err := limits.CheckDepth(groupDepth); err != nil {
		return err
	}
	if limits != nil && limits.Limits.MaxNodes > 0 && ops > limits.Limits.MaxNodes {
		return Errorf(ErrResourceLimit, "node limit exceeded: %d header fields > %d", ops, limits.Limits.MaxNodes)
	}
	return nil
}

// fieldName decodes one field name token; an empty name is a header error.
func fieldName(token string, limits *LimitCounter) (string, bool, error) {
	token = trimSpaces(token)
	if token == "" {
		return "", false, NewError(ErrMalformedHeader, "empty field name in field list")
	}
	name, quoted, err := parseKeyToken(token)
	if err != nil {
		return "", false, err
	}
	return name, quoted, limits.CheckStringBytes(len(name))
}
