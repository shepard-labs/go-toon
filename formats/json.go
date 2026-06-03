package formats

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/shepard-labs/go-toon/toon"
)

func FromJSON(r io.Reader, opts ...JSONOption) (*toon.Node, error) {
	o := resolveJSONOptions(opts...)
	lim := newLimits(o.Limits)
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if err := lim.c.CheckInputBytes(int64(len(data))); err != nil {
		return nil, err
	}
	p := jsonParser{data: string(data), opts: o, limits: lim}
	n, err := p.parseValue(0)
	if err != nil {
		return nil, err
	}
	p.skipSpace()
	if p.pos != len(p.data) {
		return nil, p.errf("unexpected JSON token %q", p.data[p.pos])
	}
	return n, nil
}

type jsonParser struct {
	data   string
	pos    int
	opts   JSONOptions
	limits limits
}

func (p *jsonParser) parseValue(depth int) (*toon.Node, error) {
	p.skipSpace()
	if p.pos >= len(p.data) {
		return nil, p.errf("unexpected end of JSON")
	}
	switch p.data[p.pos] {
	case '{':
		return p.parseObject(depth)
	case '[':
		return p.parseArray(depth)
	case '"':
		s, err := p.parseString()
		if err != nil {
			return nil, err
		}
		n := &toon.Node{Kind: toon.StringKind, String: s}
		return n, p.limits.node(n, depth)
	case 't':
		if err := p.consumeLiteral("true"); err != nil {
			return nil, err
		}
		n := &toon.Node{Kind: toon.BoolKind, Bool: true}
		return n, p.limits.node(n, depth)
	case 'f':
		if err := p.consumeLiteral("false"); err != nil {
			return nil, err
		}
		n := &toon.Node{Kind: toon.BoolKind}
		return n, p.limits.node(n, depth)
	case 'n':
		if err := p.consumeLiteral("null"); err != nil {
			return nil, err
		}
		n := &toon.Node{Kind: toon.NullKind}
		return n, p.limits.node(n, depth)
	default:
		if p.data[p.pos] == '-' || isJSONDigit(p.data[p.pos]) {
			return p.parseNumber(depth)
		}
	}
	return nil, p.errf("invalid JSON value")
}

func (p *jsonParser) parseObject(depth int) (*toon.Node, error) {
	p.pos++
	n := &toon.Node{Kind: toon.ObjectKind}
	if err := p.limits.node(n, depth); err != nil {
		return nil, err
	}
	seen := duplicateTracker{}
	p.skipSpace()
	if p.consumeByte('}') {
		return n, nil
	}
	for {
		p.skipSpace()
		if p.pos >= len(p.data) || p.data[p.pos] != '"' {
			return nil, p.errf("JSON object key is not string")
		}
		key, err := p.parseString()
		if err != nil {
			return nil, err
		}
		if !p.opts.AllowDuplicateKeys && seen.has(key) {
			return nil, duplicateErr("duplicate JSON key")
		}
		seen.add(key)
		p.skipSpace()
		if !p.consumeByte(':') {
			return nil, p.errf("JSON object missing colon")
		}
		value, err := p.parseValue(depth + 1)
		if err != nil {
			return nil, err
		}
		field := toon.Field{Key: key, Value: value}
		if p.opts.AllowDuplicateKeys {
			n.Object = upsert(n.Object, field)
		} else {
			n.Object = append(n.Object, field)
		}
		p.skipSpace()
		if p.consumeByte('}') {
			return n, nil
		}
		if !p.consumeByte(',') {
			return nil, p.errf("JSON object missing comma")
		}
	}
}

func (p *jsonParser) parseArray(depth int) (*toon.Node, error) {
	p.pos++
	n := &toon.Node{Kind: toon.ArrayKind}
	if err := p.limits.node(n, depth); err != nil {
		return nil, err
	}
	p.skipSpace()
	if p.consumeByte(']') {
		return n, nil
	}
	for {
		item, err := p.parseValue(depth + 1)
		if err != nil {
			return nil, err
		}
		n.Array = append(n.Array, item)
		if err := p.limits.c.CheckArrayLength(len(n.Array)); err != nil {
			return nil, err
		}
		p.skipSpace()
		if p.consumeByte(']') {
			return n, nil
		}
		if !p.consumeByte(',') {
			return nil, p.errf("JSON array missing comma")
		}
	}
}

func (p *jsonParser) parseString() (string, error) {
	p.pos++
	start := p.pos
	for p.pos < len(p.data) {
		c := p.data[p.pos]
		if c == '"' {
			s := p.data[start:p.pos]
			p.pos++
			if !utf8.ValidString(s) {
				return "", p.errf("invalid UTF-8 in JSON string")
			}
			return s, nil
		}
		if c == '\\' {
			return p.parseEscapedString(start)
		}
		if c < 0x20 {
			return "", p.errf("control character in JSON string")
		}
		p.pos++
	}
	return "", p.errf("unterminated JSON string")
}

func (p *jsonParser) parseEscapedString(start int) (string, error) {
	var b strings.Builder
	for {
		b.WriteString(p.data[start:p.pos])
		p.pos++
		if p.pos >= len(p.data) {
			return "", p.errf("unterminated JSON escape")
		}
		switch c := p.data[p.pos]; c {
		case '"', '\\', '/':
			b.WriteByte(c)
			p.pos++
		case 'b':
			b.WriteByte('\b')
			p.pos++
		case 'f':
			b.WriteByte('\f')
			p.pos++
		case 'n':
			b.WriteByte('\n')
			p.pos++
		case 'r':
			b.WriteByte('\r')
			p.pos++
		case 't':
			b.WriteByte('\t')
			p.pos++
		case 'u':
			r, err := p.parseUnicodeEscape()
			if err != nil {
				return "", err
			}
			b.WriteRune(r)
		default:
			return "", p.errf("invalid JSON escape")
		}
		start = p.pos
		for p.pos < len(p.data) {
			c := p.data[p.pos]
			if c == '"' {
				b.WriteString(p.data[start:p.pos])
				p.pos++
				return b.String(), nil
			}
			if c == '\\' {
				break
			}
			if c < 0x20 {
				return "", p.errf("control character in JSON string")
			}
			p.pos++
		}
		if p.pos >= len(p.data) {
			return "", p.errf("unterminated JSON string")
		}
	}
}

func (p *jsonParser) parseUnicodeEscape() (rune, error) {
	p.pos++
	r, err := p.readHexRune()
	if err != nil {
		return 0, err
	}
	if utf16IsHighSurrogate(r) {
		if p.pos+2 <= len(p.data) && p.data[p.pos:p.pos+2] == `\u` {
			p.pos += 2
			lo, err := p.readHexRune()
			if err != nil {
				return 0, err
			}
			if utf16IsLowSurrogate(lo) {
				return utf16DecodeRune(r, lo), nil
			}
			return utf8.RuneError, nil
		}
		return utf8.RuneError, nil
	}
	if utf16IsLowSurrogate(r) {
		return utf8.RuneError, nil
	}
	return r, nil
}

func (p *jsonParser) readHexRune() (rune, error) {
	if p.pos+4 > len(p.data) {
		return 0, p.errf("short JSON unicode escape")
	}
	var v rune
	for i := 0; i < 4; i++ {
		h, ok := fromHex(p.data[p.pos+i])
		if !ok {
			return 0, p.errf("invalid JSON unicode escape")
		}
		v = v<<4 | rune(h)
	}
	p.pos += 4
	return v, nil
}

func (p *jsonParser) parseNumber(depth int) (*toon.Node, error) {
	start := p.pos
	if p.consumeByte('-') && p.pos >= len(p.data) {
		return nil, p.errf("invalid JSON number")
	}
	if p.consumeByte('0') {
		if p.pos < len(p.data) && isJSONDigit(p.data[p.pos]) {
			return nil, p.errf("invalid JSON number")
		}
	} else if p.pos < len(p.data) && isJSONDigit1To9(p.data[p.pos]) {
		for p.pos < len(p.data) && isJSONDigit(p.data[p.pos]) {
			p.pos++
		}
	} else {
		return nil, p.errf("invalid JSON number")
	}
	if p.pos < len(p.data) && p.data[p.pos] == '.' {
		p.pos++
		if p.pos >= len(p.data) || !isJSONDigit(p.data[p.pos]) {
			return nil, p.errf("invalid JSON number")
		}
		for p.pos < len(p.data) && isJSONDigit(p.data[p.pos]) {
			p.pos++
		}
	}
	if p.pos < len(p.data) && (p.data[p.pos] == 'e' || p.data[p.pos] == 'E') {
		p.pos++
		if p.pos < len(p.data) && (p.data[p.pos] == '+' || p.data[p.pos] == '-') {
			p.pos++
		}
		if p.pos >= len(p.data) || !isJSONDigit(p.data[p.pos]) {
			return nil, p.errf("invalid JSON number")
		}
		for p.pos < len(p.data) && isJSONDigit(p.data[p.pos]) {
			p.pos++
		}
	}
	raw := p.data[start:p.pos]
	if !isJSONNumberTerminator(p.peekByte()) {
		return nil, p.errf("invalid JSON number")
	}
	n := &toon.Node{Kind: toon.NumberKind, Number: toon.Number{Raw: raw}}
	return n, p.limits.node(n, depth)
}

func (p *jsonParser) consumeLiteral(lit string) error {
	if len(p.data)-p.pos < len(lit) || p.data[p.pos:p.pos+len(lit)] != lit {
		return p.errf("invalid JSON literal")
	}
	p.pos += len(lit)
	if !isJSONNumberTerminator(p.peekByte()) {
		return p.errf("invalid JSON literal")
	}
	return nil
}

func (p *jsonParser) skipSpace() {
	for p.pos < len(p.data) {
		switch p.data[p.pos] {
		case ' ', '\n', '\r', '\t':
			p.pos++
		default:
			return
		}
	}
}

func (p *jsonParser) consumeByte(c byte) bool {
	if p.pos < len(p.data) && p.data[p.pos] == c {
		p.pos++
		return true
	}
	return false
}

func (p *jsonParser) peekByte() byte {
	if p.pos >= len(p.data) {
		return 0
	}
	return p.data[p.pos]
}

func (p *jsonParser) errf(format string, args ...any) error {
	return formatErr("invalid JSON: "+fmt.Sprintf(format, args...), nil)
}

func isJSONDigit(c byte) bool { return c >= '0' && c <= '9' }

func isJSONDigit1To9(c byte) bool { return c >= '1' && c <= '9' }

func isJSONNumberTerminator(c byte) bool {
	switch c {
	case 0, ' ', '\n', '\r', '\t', ',', ']', '}':
		return true
	}
	return false
}

func fromHex(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}

func utf16IsHighSurrogate(r rune) bool { return r >= 0xd800 && r <= 0xdbff }

func utf16IsLowSurrogate(r rune) bool { return r >= 0xdc00 && r <= 0xdfff }

func utf16DecodeRune(high, low rune) rune { return (high-0xd800)<<10 + (low - 0xdc00) + 0x10000 }

func ToJSON(w io.Writer, n *toon.Node, opts ...JSONOutputOption) error {
	o := resolveJSONOutputOptions(opts...)
	var b strings.Builder
	if err := writeJSONNode(&b, n, o, 0); err != nil {
		return err
	}
	_, err := io.WriteString(w, b.String())
	return err
}

func writeJSONNode(b *strings.Builder, n *toon.Node, o JSONOutputOptions, depth int) error {
	if n == nil {
		b.WriteString("null")
		return nil
	}
	switch n.Kind {
	case toon.NullKind:
		b.WriteString("null")
	case toon.BoolKind:
		if n.Bool {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case toon.NumberKind:
		if !toon.IsValidNumberToken(n.Number.Raw) {
			return toon.Errorf(toon.ErrInvalidInputFormat, "invalid JSON number %q", n.Number.Raw)
		}
		b.WriteString(toon.CanonicalizeNumberToken(n.Number.Raw))
	case toon.StringKind:
		writeJSONString(b, n.String)
	case toon.ArrayKind:
		b.WriteByte('[')
		for i, item := range n.Array {
			if i > 0 {
				b.WriteByte(',')
			}
			newlineJSON(b, o, depth+1)
			if err := writeJSONNode(b, item, o, depth+1); err != nil {
				return err
			}
		}
		if len(n.Array) > 0 {
			newlineJSON(b, o, depth)
		}
		b.WriteByte(']')
	case toon.ObjectKind:
		b.WriteByte('{')
		for i, f := range n.Object {
			if i > 0 {
				b.WriteByte(',')
			}
			newlineJSON(b, o, depth+1)
			writeJSONString(b, f.Key)
			b.WriteByte(':')
			if o.Indent != "" {
				b.WriteByte(' ')
			}
			if err := writeJSONNode(b, f.Value, o, depth+1); err != nil {
				return err
			}
		}
		if len(n.Object) > 0 {
			newlineJSON(b, o, depth)
		}
		b.WriteByte('}')
	}
	return nil
}

func writeJSONString(b *strings.Builder, s string) {
	b.WriteByte('"')
	start := 0
	for i := 0; i < len(s); {
		if c := s[i]; c < utf8.RuneSelf {
			esc := ""
			switch c {
			case '\\', '"':
				esc = `\` + string(c)
			case '\b':
				esc = `\b`
			case '\f':
				esc = `\f`
			case '\n':
				esc = `\n`
			case '\r':
				esc = `\r`
			case '\t':
				esc = `\t`
			case '<':
				esc = `\u003c`
			case '>':
				esc = `\u003e`
			case '&':
				esc = `\u0026`
			default:
				if c < 0x20 {
					esc = `\u00` + hex[c>>4:c>>4+1] + hex[c&0xf:c&0xf+1]
				}
			}
			if esc != "" {
				b.WriteString(s[start:i])
				b.WriteString(esc)
				i++
				start = i
				continue
			}
			i++
			continue
		}

		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			b.WriteString(s[start:i])
			b.WriteString(`\ufffd`)
			i++
			start = i
			continue
		}
		if r == '\u2028' || r == '\u2029' {
			b.WriteString(s[start:i])
			if r == '\u2028' {
				b.WriteString(`\u2028`)
			} else {
				b.WriteString(`\u2029`)
			}
			i += size
			start = i
			continue
		}
		i += size
	}
	b.WriteString(s[start:])
	b.WriteByte('"')
}

const hex = "0123456789abcdef"

func newlineJSON(b *strings.Builder, o JSONOutputOptions, depth int) {
	if o.Indent == "" {
		return
	}
	b.WriteByte('\n')
	b.WriteString(strings.Repeat(o.Indent, depth))
}

func intFieldName(i int) string { return "field" + strconv.Itoa(i) }
