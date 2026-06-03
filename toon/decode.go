package toon

import (
	"io"
	"strconv"
	"strings"
)

type ParsedLine struct {
	Raw        string
	Content    string
	Depth      int
	LineNumber int
	Blank      bool
}

type ArrayHeader struct {
	Key       *string
	Length    int
	Delimiter Delimiter
	Fields    []string
}

func Decode(data []byte, opts ...DecodeOption) (*Node, error) {
	options := ResolveDecodeOptions(opts...)
	if options.IndentSize <= 0 {
		return nil, NewError(ErrInvalidIndent, "indent size must be positive")
	}
	dec := &decoder{opts: options, limits: NewLimitCounter(options.Limits)}
	if err := dec.limits.CheckInputBytes(int64(len(data))); err != nil {
		return nil, err
	}
	lines, err := dec.scan(data)
	if err != nil {
		return nil, err
	}
	var n *Node
	if len(lines) == 0 {
		n = &Node{Kind: ObjectKind}
		if err := dec.accountNode(n, 0); err != nil {
			return nil, err
		}
	} else {
		n, _, err = dec.parseRoot(lines)
		if err != nil {
			return nil, err
		}
	}
	if options.ExpandPaths == ExpandPathsSafe {
		n, err = dec.expandPaths(n)
		if err != nil {
			return nil, err
		}
	}
	return n, nil
}

func DecodeReader(r io.Reader, opts ...DecodeOption) (*Node, error) {
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
}

func (d *decoder) scan(data []byte) ([]ParsedLine, error) {
	s := string(data)
	lines := make([]ParsedLine, 0, countLines(s))
	lineNumber := 1
	for start := 0; start < len(s); lineNumber++ {
		end := start
		for end < len(s) && s[end] != '\n' && s[end] != '\r' {
			end++
		}
		raw := s[start:end]
		if end == len(s) && raw == "" {
			break
		}
		spaces := 0
		for spaces < len(raw) {
			if raw[spaces] == '\t' {
				return nil, &Error{Code: ErrTabIndent, Line: lineNumber, Column: spaces + 1, Message: "tab indentation is not allowed", Context: raw}
			}
			if raw[spaces] != ' ' {
				break
			}
			spaces++
		}
		if strings.TrimSpace(raw) == "" {
			depth := spaces / d.opts.IndentSize
			lines = append(lines, ParsedLine{Raw: raw, Depth: depth, LineNumber: lineNumber, Blank: true})
			start = nextLineStart(s, end)
			continue
		}
		if d.opts.Strict && spaces%d.opts.IndentSize != 0 {
			return nil, &Error{Code: ErrInvalidIndent, Line: lineNumber, Column: 1, Message: "indentation is not a multiple of indent size", Context: raw}
		}
		depth := spaces / d.opts.IndentSize
		lines = append(lines, ParsedLine{Raw: raw, Content: raw[spaces:], Depth: depth, LineNumber: lineNumber})
		start = nextLineStart(s, end)
	}
	lines = dropIgnorableBlankLines(lines)
	return lines, nil
}

func countLines(s string) int {
	if len(s) == 0 {
		return 0
	}
	lines := 1
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\n':
			lines++
		case '\r':
			lines++
			if i+1 < len(s) && s[i+1] == '\n' {
				i++
			}
		}
	}
	if s[len(s)-1] == '\n' || s[len(s)-1] == '\r' {
		lines--
	}
	return lines
}

func nextLineStart(s string, end int) int {
	if end >= len(s) {
		return len(s)
	}
	if s[end] == '\r' && end+1 < len(s) && s[end+1] == '\n' {
		return end + 2
	}
	return end + 1
}

func (d *decoder) parseRoot(lines []ParsedLine) (*Node, int, error) {
	if len(lines) == 1 && lines[0].Depth == 0 && lines[0].Content == "[]" {
		n := &Node{Kind: ArrayKind}
		return n, 1, d.accountNode(n, 0)
	}
	if lines[0].Depth != 0 {
		return nil, 0, lineError(lines[0], ErrInvalidIndent, "root content must start at depth 0")
	}
	if h, ok, err := d.parseHeaderLine(lines[0], true); err != nil {
		return nil, 0, err
	} else if ok && h.Key == nil {
		return d.parseArray(lines, 0, h, 0)
	}
	if len(lines) == 1 && !looksLikeObjectLine(lines[0].Content) {
		return d.parsePrimitiveLine(lines[0])
	}
	if d.opts.Strict {
		for _, line := range lines {
			if line.Depth == 0 && !looksLikeObjectLine(line.Content) {
				return nil, 0, lineError(line, ErrMissingColon, "invalid top-level content")
			}
		}
	}
	return d.parseObject(lines, 0, 0)
}

func (d *decoder) parseObject(lines []ParsedLine, start, depth int) (*Node, int, error) {
	n := &Node{Kind: ObjectKind}
	if err := d.accountNode(n, depth); err != nil {
		return nil, start, err
	}
	seen := duplicateTracker{}
	i := start
	for i < len(lines) {
		line := lines[i]
		if line.Blank {
			break
		}
		if line.Depth < depth {
			break
		}
		if line.Depth > depth {
			return nil, i, lineError(line, ErrInvalidIndent, "unexpected indentation")
		}
		h, ok, err := d.parseHeaderLine(line, true)
		if err != nil {
			return nil, i, err
		}
		if ok && h.Key != nil {
			value, next, err := d.parseArray(lines, i, h, depth+1)
			if err != nil {
				return nil, i, err
			}
			field := Field{Key: *h.Key, Value: value}
			if err := d.addField(n, field, &seen, line); err != nil {
				return nil, i, err
			}
			i = next
			continue
		}
		key, quoted, valueToken, hasValue, err := parseKeyValue(line.Content)
		if err != nil {
			return nil, i, lineError(line, CodeOf(err), err.Error())
		}
		field := Field{Key: key, WasQuoted: quoted}
		if hasValue {
			if strings.TrimSpace(valueToken) == "[]" {
				field.Value = &Node{Kind: ArrayKind}
				if err := d.accountNode(field.Value, depth+1); err != nil {
					return nil, i, err
				}
			} else {
				field.Value, err = d.parsePrimitiveTokenAt(valueToken, line, depth+1)
				if err != nil {
					return nil, i, err
				}
			}
			i++
		} else if i+1 < len(lines) && lines[i+1].Depth > depth {
			field.Value, i, err = d.parseObject(lines, i+1, depth+1)
			if err != nil {
				return nil, i, err
			}
		} else {
			field.Value = &Node{Kind: ObjectKind}
			if err := d.accountNode(field.Value, depth+1); err != nil {
				return nil, i, err
			}
			i++
		}
		if err := d.addField(n, field, &seen, line); err != nil {
			return nil, i, err
		}
	}
	return n, i, nil
}

func (d *decoder) parseArray(lines []ParsedLine, start int, h ArrayHeader, itemDepth int) (*Node, int, error) {
	n := &Node{Kind: ArrayKind}
	if err := d.accountNode(n, itemDepth-1); err != nil {
		return nil, start, err
	}
	if err := d.limits.CheckArrayLength(h.Length); err != nil {
		return nil, start, err
	}
	line := lines[start]
	inline := strings.TrimSpace(afterColon(line.Content))
	if inline != "" {
		if h.Fields != nil {
			return nil, start, lineError(line, ErrMalformedHeader, "tabular header cannot have inline values")
		}
		tokens, err := SplitDelimited(inline, h.Delimiter)
		if err != nil {
			return nil, start, lineError(line, CodeOf(err), err.Error())
		}
		if d.opts.Strict && len(tokens) != h.Length {
			return nil, start, lineError(line, ErrArrayCountMismatch, "inline array count mismatch")
		}
		for _, token := range tokens {
			item, err := d.parsePrimitiveTokenAt(token, line, itemDepth)
			if err != nil {
				return nil, start, err
			}
			n.Array = append(n.Array, item)
		}
		return n, start + 1, nil
	}
	if h.Length == 0 {
		return n, start + 1, nil
	}
	if h.Fields != nil {
		return d.parseTabularArray(n, lines, start, h, itemDepth)
	}
	return d.parseListArray(n, lines, start, h, itemDepth)
}

func (d *decoder) parseTabularArray(n *Node, lines []ParsedLine, start int, h ArrayHeader, itemDepth int) (*Node, int, error) {
	i := start + 1
	rows := 0
	for i < len(lines) && lines[i].Depth == itemDepth {
		line := lines[i]
		if line.Blank {
			return nil, i, lineError(line, ErrInvalidInputFormat, "blank line inside array")
		}
		if isKeyValueByDisambiguation(line.Content, h.Delimiter) {
			break
		}
		tokens, err := SplitDelimited(line.Content, h.Delimiter)
		if err != nil {
			return nil, i, lineError(line, CodeOf(err), err.Error())
		}
		if d.opts.Strict && len(tokens) != len(h.Fields) {
			return nil, i, lineError(line, ErrTabularWidthMismatch, "tabular row width mismatch")
		}
		obj := &Node{Kind: ObjectKind}
		if err := d.accountNode(obj, itemDepth); err != nil {
			return nil, i, err
		}
		width := minInt(len(tokens), len(h.Fields))
		for col := range width {
			value, err := d.parsePrimitiveTokenAt(tokens[col], line, itemDepth+1)
			if err != nil {
				return nil, i, err
			}
			obj.Object = append(obj.Object, Field{Key: h.Fields[col], Value: value})
		}
		n.Array = append(n.Array, obj)
		rows++
		i++
	}
	if d.opts.Strict && rows != h.Length {
		return nil, i, lineError(lines[start], ErrArrayCountMismatch, "tabular row count mismatch")
	}
	return n, i, nil
}

func (d *decoder) parseListArray(n *Node, lines []ParsedLine, start int, h ArrayHeader, itemDepth int) (*Node, int, error) {
	i := start + 1
	count := 0
	for i < len(lines) && lines[i].Depth == itemDepth {
		line := lines[i]
		if line.Blank {
			return nil, i, lineError(line, ErrInvalidInputFormat, "blank line inside array")
		}
		if !strings.HasPrefix(line.Content, "-") || (len(line.Content) > 1 && line.Content[1] != ' ') {
			break
		}
		rest := strings.TrimSpace(line.Content[1:])
		var item *Node
		var err error
		if rest == "" {
			if i+1 < len(lines) && lines[i+1].Depth > itemDepth {
				item, i, err = d.parseObject(lines, i+1, itemDepth+1)
			} else {
				item = &Node{Kind: ObjectKind}
				if err = d.accountNode(item, itemDepth); err == nil {
					i++
				}
			}
		} else if h2, ok, err2 := d.parseHeaderContent(rest, false); err2 != nil {
			return nil, i, lineError(line, CodeOf(err2), err2.Error())
		} else if ok {
			fake := ParsedLine{Raw: rest, Content: rest, Depth: itemDepth, LineNumber: line.LineNumber}
			newLines := append([]ParsedLine{fake}, lines[i+1:]...)
			var next int
			item, next, err = d.parseArray(newLines, 0, h2, itemDepth+1)
			if err == nil {
				i += next
			}
		} else if looksLikeObjectLine(rest) {
			key, quoted, valueToken, hasValue, err2 := parseKeyValue(rest)
			if err2 != nil {
				return nil, i, lineError(line, CodeOf(err2), err2.Error())
			}
			item = &Node{Kind: ObjectKind}
			if err = d.accountNode(item, itemDepth); err == nil {
				field := Field{Key: key, WasQuoted: quoted}
				if hasValue {
					field.Value, err = d.parsePrimitiveTokenAt(valueToken, line, itemDepth+1)
				} else {
					field.Value = &Node{Kind: ObjectKind}
					err = d.accountNode(field.Value, itemDepth+1)
				}
				if err == nil {
					item.Object = append(item.Object, field)
					if i+1 < len(lines) && lines[i+1].Depth > itemDepth {
						child, next, err2 := d.parseObject(lines, i+1, itemDepth+1)
						if err2 != nil {
							return nil, i, err2
						}
						item.Object = append(item.Object, child.Object...)
						i = next
					} else {
						i++
					}
				}
			}
		} else {
			item, err = d.parsePrimitiveTokenAt(rest, line, itemDepth)
			if err == nil {
				i++
			}
		}
		if err != nil {
			return nil, i, err
		}
		n.Array = append(n.Array, item)
		count++
	}
	if d.opts.Strict && count != h.Length {
		return nil, i, lineError(lines[start], ErrArrayCountMismatch, "list array count mismatch")
	}
	return n, i, nil
}

func (d *decoder) parsePrimitiveLine(line ParsedLine) (*Node, int, error) {
	n, err := d.parsePrimitiveTokenAt(line.Content, line, 0)
	return n, 1, err
}

func (d *decoder) parsePrimitiveTokenAt(token string, line ParsedLine, depth int) (*Node, error) {
	n, err := ParsePrimitiveToken(token)
	if err != nil {
		return nil, lineError(line, CodeOf(err), err.Error())
	}
	if err := d.accountNode(n, depth); err != nil {
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

func (d *decoder) addField(obj *Node, field Field, seen *duplicateTracker, line ParsedLine) error {
	if d.opts.Strict {
		if seen.has(field.Key) {
			return lineError(line, ErrDuplicateKey, "duplicate key")
		}
	}
	if !d.opts.Strict {
		for i := range obj.Object {
			if obj.Object[i].Key == field.Key {
				obj.Object[i] = field
				return nil
			}
		}
	}
	seen.add(field.Key)
	obj.Object = append(obj.Object, field)
	return nil
}

type duplicateTracker struct {
	keys  [8]string
	count int
	set   map[string]struct{}
}

func (t *duplicateTracker) has(key string) bool {
	if t.set != nil {
		_, ok := t.set[key]
		return ok
	}
	for i := 0; i < t.count; i++ {
		if t.keys[i] == key {
			return true
		}
	}
	return false
}

func (t *duplicateTracker) add(key string) {
	if t.set != nil {
		t.set[key] = struct{}{}
		return
	}
	if t.count < len(t.keys) {
		t.keys[t.count] = key
		t.count++
		return
	}
	t.set = make(map[string]struct{}, len(t.keys)+1)
	for i := 0; i < t.count; i++ {
		t.set[t.keys[i]] = struct{}{}
	}
	t.set[key] = struct{}{}
}

func (d *decoder) parseHeaderLine(line ParsedLine, allowKey bool) (ArrayHeader, bool, error) {
	h, ok, err := d.parseHeaderContent(line.Content, allowKey)
	if err != nil {
		return h, ok, lineError(line, CodeOf(err), err.Error())
	}
	return h, ok, nil
}

func (d *decoder) parseHeaderContent(content string, allowKey bool) (ArrayHeader, bool, error) {
	bracket := findHeaderBracket(content)
	if bracket < 0 {
		return ArrayHeader{}, false, nil
	}
	keyPart := content[:bracket]
	if strings.Contains(keyPart, ":") {
		return ArrayHeader{}, false, nil
	}
	if keyPart != "" && !allowKey {
		return ArrayHeader{}, false, nil
	}
	close := strings.IndexByte(content[bracket:], ']')
	if close < 0 {
		return ArrayHeader{}, false, NewError(ErrMalformedHeader, "malformed array header")
	}
	close += bracket
	inside := content[bracket+1 : close]
	delim := Comma
	if strings.HasSuffix(inside, string(byte(Tab))) {
		delim = Tab
		inside = inside[:len(inside)-1]
	} else if strings.HasSuffix(inside, "|") {
		delim = Pipe
		inside = inside[:len(inside)-1]
	}
	if inside == "" || (len(inside) > 1 && inside[0] == '0') {
		return ArrayHeader{}, false, NewError(ErrMalformedHeader, "malformed header length")
	}
	length, err := strconv.Atoi(inside)
	if err != nil || length < 0 {
		return ArrayHeader{}, false, NewError(ErrMalformedHeader, "malformed header length")
	}
	h := ArrayHeader{Length: length, Delimiter: delim}
	if keyPart != "" {
		key, quoted, err := parseKeyToken(keyPart)
		if err != nil {
			return ArrayHeader{}, false, err
		}
		_ = quoted
		h.Key = &key
	}
	rest := content[close+1:]
	if strings.HasPrefix(rest, "{") {
		fieldEnd := findClosingBrace(rest)
		if fieldEnd < 0 {
			return ArrayHeader{}, false, NewError(ErrMalformedHeader, "malformed field header")
		}
		fieldText := rest[1:fieldEnd]
		if d.opts.Strict && containsUnquotedOtherDelimiter(fieldText, delim) {
			return ArrayHeader{}, false, NewError(ErrHeaderDelimiterMismatch, "header delimiter mismatch")
		}
		fields, err := SplitDelimited(fieldText, delim)
		if err != nil {
			return ArrayHeader{}, false, err
		}
		for _, token := range fields {
			key, _, err := parseKeyToken(token)
			if err != nil {
				return ArrayHeader{}, false, err
			}
			h.Fields = append(h.Fields, key)
		}
		rest = rest[fieldEnd+1:]
	}
	if !strings.HasPrefix(rest, ":") {
		return ArrayHeader{}, false, NewError(ErrMalformedHeader, "array header missing colon")
	}
	if d.opts.Strict && strings.TrimSpace(rest[:0]) != "" {
		return ArrayHeader{}, false, NewError(ErrMalformedHeader, "content between header and colon")
	}
	return h, true, nil
}

func (d *decoder) expandPaths(n *Node) (*Node, error) {
	if n == nil || n.Kind != ObjectKind {
		return n, nil
	}
	return d.expandObject(n, 0)
}

func (d *decoder) expandObject(n *Node, depth int) (*Node, error) {
	result := &Node{Kind: ObjectKind}
	for _, field := range n.Object {
		value := field.Value
		if value != nil && value.Kind == ObjectKind {
			var err error
			value, err = d.expandObject(value, depth+1)
			if err != nil {
				return nil, err
			}
		}
		if field.WasQuoted || !strings.Contains(field.Key, ".") {
			if err := mergeField(result, Field{Key: field.Key, WasQuoted: field.WasQuoted, Value: value}, d.opts.Strict); err != nil {
				return nil, err
			}
			continue
		}
		parts := strings.Split(field.Key, ".")
		for _, part := range parts {
			if !IsValidSafeSegment(part) {
				if err := mergeField(result, Field{Key: field.Key, Value: value}, d.opts.Strict); err != nil {
					return nil, err
				}
				goto nextField
			}
		}
		if err := insertPath(result, parts, value, d.opts.Strict); err != nil {
			return nil, err
		}
	nextField:
	}
	return result, nil
}

func parseKeyValue(content string) (key string, quoted bool, value string, hasValue bool, err error) {
	colon := findUnquoted(content, ':')
	if colon < 0 {
		return "", false, "", false, NewError(ErrMissingColon, "missing colon")
	}
	key, quoted, err = parseKeyToken(content[:colon])
	if err != nil {
		return "", false, "", false, err
	}
	value = strings.TrimSpace(content[colon+1:])
	return key, quoted, value, value != "", nil
}

func parseKeyToken(token string) (string, bool, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return "", false, NewError(ErrMissingColon, "empty key")
	}
	if strings.HasPrefix(token, "\"") {
		value, err := UnescapeQuotedToken(token)
		return value, true, err
	}
	if !IsValidUnquotedKey(token) {
		return "", false, NewError(ErrMissingColon, "invalid unquoted key")
	}
	return token, false, nil
}

func looksLikeObjectLine(content string) bool {
	if _, _, _, _, err := parseKeyValue(content); err == nil {
		return true
	}
	if _, ok, err := (&decoder{opts: DefaultDecodeOptions()}).parseHeaderContent(content, true); err == nil && ok {
		return true
	}
	return false
}

func lineError(line ParsedLine, code ErrorCode, message string) *Error {
	if code == "" {
		code = ErrInvalidInputFormat
	}
	return &Error{Code: code, Line: line.LineNumber, Message: message, Context: line.Raw}
}

func afterColon(content string) string {
	colon := findUnquoted(content, ':')
	if colon < 0 {
		return ""
	}
	return content[colon+1:]
}

func findHeaderBracket(content string) int {
	inQuote := false
	escaped := false
	for i := 0; i < len(content); i++ {
		ch := content[i]
		if inQuote {
			if escaped {
				escaped = false
			} else if ch == '\\' {
				escaped = true
			} else if ch == '"' {
				inQuote = false
			}
			continue
		}
		if ch == '"' {
			inQuote = true
		} else if ch == '[' {
			return i
		}
	}
	return -1
}

func findClosingBrace(s string) int { return findUnquoted(s, '}') }

func findUnquoted(s string, target byte) int {
	inQuote := false
	escaped := false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if inQuote {
			if escaped {
				escaped = false
			} else if ch == '\\' {
				escaped = true
			} else if ch == '"' {
				inQuote = false
			}
			continue
		}
		if ch == '"' {
			inQuote = true
		} else if ch == target {
			return i
		}
	}
	return -1
}

func containsUnquotedOtherDelimiter(s string, active Delimiter) bool {
	for _, delim := range []Delimiter{Comma, Tab, Pipe} {
		if delim != active && findUnquoted(s, byte(delim)) >= 0 {
			return true
		}
	}
	return false
}

func isKeyValueByDisambiguation(s string, delimiter Delimiter) bool {
	colon := findUnquoted(s, ':')
	if colon < 0 {
		return false
	}
	delim := findUnquoted(s, byte(delimiter))
	return delim < 0 || colon < delim
}

func mergeField(obj *Node, field Field, strict bool) error {
	for i := range obj.Object {
		if obj.Object[i].Key == field.Key {
			if strict {
				return NewError(ErrPathExpansionConflict, "path expansion conflict")
			}
			obj.Object[i] = field
			return nil
		}
	}
	obj.Object = append(obj.Object, field)
	return nil
}

func insertPath(obj *Node, parts []string, value *Node, strict bool) error {
	current := obj
	for i, part := range parts {
		last := i == len(parts)-1
		idx := fieldIndex(current.Object, part)
		if last {
			if idx >= 0 {
				if current.Object[idx].Value != nil && current.Object[idx].Value.Kind == ObjectKind && value != nil && value.Kind == ObjectKind {
					for _, f := range value.Object {
						if err := mergeField(current.Object[idx].Value, f, strict); err != nil {
							return err
						}
					}
					return nil
				}
				if strict {
					return NewError(ErrPathExpansionConflict, "path expansion conflict")
				}
				current.Object[idx].Value = value
				return nil
			}
			current.Object = append(current.Object, Field{Key: part, Value: value})
			return nil
		}
		if idx < 0 {
			child := &Node{Kind: ObjectKind}
			current.Object = append(current.Object, Field{Key: part, Value: child})
			current = child
			continue
		}
		if current.Object[idx].Value == nil || current.Object[idx].Value.Kind != ObjectKind {
			if strict {
				return NewError(ErrPathExpansionConflict, "path expansion conflict")
			}
			current.Object[idx].Value = &Node{Kind: ObjectKind}
		}
		current = current.Object[idx].Value
	}
	return nil
}

func fieldIndex(fields []Field, key string) int {
	for i, field := range fields {
		if field.Key == key {
			return i
		}
	}
	return -1
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func dropIgnorableBlankLines(lines []ParsedLine) []ParsedLine {
	result := make([]ParsedLine, 0, len(lines))
	for i, line := range lines {
		if !line.Blank {
			result = append(result, line)
			continue
		}
		prevArray := false
		for j := i - 1; j >= 0; j-- {
			if lines[j].Blank {
				continue
			}
			if lines[j].Depth >= line.Depth {
				continue
			}
			if h, ok, _ := (&decoder{opts: DefaultDecodeOptions()}).parseHeaderContent(lines[j].Content, true); ok && h.Fields == nil {
				prevArray = true
			}
			break
		}
		if prevArray {
			result = append(result, line)
		}
	}
	return result
}
