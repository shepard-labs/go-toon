package toon

import (
	"strings"
	"unicode/utf16"
)

// trimSpaces trims U+0020 and nothing else: tabs, NBSP and every other Unicode
// space are token content (SPEC §1.2, §12).
func trimSpaces(s string) string { return strings.Trim(s, " ") }

// skipQuoted returns the index just past the quoted span that opens at s[i]
// (which must be '"'). The span closes at the next '"' that is not escaped by a
// backslash; without one it runs to the end of s (SPEC §5.2).
func skipQuoted(s string, i int) int {
	for j := i + 1; j < len(s); j++ {
		switch s[j] {
		case '\\':
			j++
		case '"':
			return j + 1
		}
	}
	return len(s)
}

// findUnquoted returns the index of the first occurrence of target outside any
// quoted span, or -1.
func findUnquoted(s string, target byte) int {
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"':
			i = skipQuoted(s, i) - 1
		case target:
			return i
		}
	}
	return -1
}

// scanBracketColon returns the index of the first unquoted colon (-1 if none)
// and, when that colon exists, the index of the first unquoted '[' before it
// (-1 if none). The pair drives the line classification of SPEC §5.2.
func scanBracketColon(s string) (bracket, colon int) {
	bracket = -1
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"':
			i = skipQuoted(s, i) - 1
		case '[':
			if bracket < 0 {
				bracket = i
			}
		case ':':
			return bracket, i
		}
	}
	return -1, -1
}

// UnescapeQuotedToken decodes a complete quoted token per SPEC §7.1. The token
// must start with '"' and its closing '"' must be its last byte.
func UnescapeQuotedToken(token string) (string, error) {
	if token == "" || token[0] != '"' {
		return "", NewError(ErrUnterminatedString, "quoted token must start with quote")
	}
	end := -1
	escapes := false
	for i := 1; i < len(token); i++ {
		c := token[i]
		if c == '\\' {
			escapes = true
			i++
			continue
		}
		if c == '"' {
			end = i
			break
		}
	}
	if end < 0 {
		return "", NewError(ErrUnterminatedString, "unterminated quoted string")
	}
	if end != len(token)-1 {
		return "", NewError(ErrInvalidEscape, "characters after closing quote")
	}
	body := token[1:end]
	if !escapes {
		return body, nil
	}
	var b strings.Builder
	b.Grow(len(body))
	for i := 0; i < len(body); i++ {
		c := body[i]
		if c != '\\' {
			b.WriteByte(c)
			continue
		}
		i++
		switch body[i] {
		case '\\':
			b.WriteByte('\\')
		case '"':
			b.WriteByte('"')
		case 'n':
			b.WriteByte('\n')
		case 'r':
			b.WriteByte('\r')
		case 't':
			b.WriteByte('\t')
		case 'u':
			if i+4 >= len(body) {
				return "", NewError(ErrInvalidEscape, "truncated unicode escape")
			}
			r, ok := parseHex4(body[i+1 : i+5])
			if !ok {
				return "", NewError(ErrInvalidEscape, "non-hex unicode escape")
			}
			if utf16.IsSurrogate(r) {
				return "", NewError(ErrInvalidEscape, "surrogate escape")
			}
			b.WriteRune(r)
			i += 4
		default:
			return "", NewError(ErrInvalidEscape, "unknown escape")
		}
	}
	return b.String(), nil
}

// SplitDelimited splits s at every unquoted occurrence of delimiter, trimming
// spaces around each token and preserving empty tokens (SPEC §11.2). A '"'
// opens a quoted span wherever it occurs; an unterminated span runs to the end
// of s, and whether a token that starts with a quote is complete is decided
// when the token is decoded.
func SplitDelimited(s string, delimiter Delimiter) ([]string, error) {
	tokens := make([]string, 0, strings.Count(s, string(rune(delimiter)))+1)
	start := 0
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch == '"' {
			i = skipQuoted(s, i) - 1
			continue
		}
		if ch == byte(delimiter) {
			tokens = append(tokens, trimSpaces(s[start:i]))
			start = i + 1
		}
	}
	tokens = append(tokens, trimSpaces(s[start:]))
	return tokens, nil
}

// ParsePrimitiveToken decodes one value token per SPEC §4: a token starting with
// '"' must be a complete quoted string; otherwise true/false/null, a JSON-grammar
// number (kept lossless in Number.Raw), or a string.
func ParsePrimitiveToken(token string) (*Node, error) {
	trimmed := trimSpaces(token)
	if trimmed == "" {
		return &Node{Kind: StringKind, String: ""}, nil
	}
	if trimmed[0] == '"' {
		s, err := UnescapeQuotedToken(trimmed)
		if err != nil {
			return nil, err
		}
		return &Node{Kind: StringKind, String: s}, nil
	}
	switch trimmed {
	case "null":
		return &Node{Kind: NullKind}, nil
	case "true":
		return &Node{Kind: BoolKind, Bool: true}, nil
	case "false":
		return &Node{Kind: BoolKind}, nil
	}
	if isJSONNumberToken(trimmed) {
		return &Node{Kind: NumberKind, Number: Number{Raw: DecodeNumberToken(trimmed)}}, nil
	}
	return &Node{Kind: StringKind, String: trimmed}, nil
}

// isJSONNumberToken reports whether s matches the RFC 8259 number grammar,
// /^-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?$/ (SPEC §4).
func isJSONNumberToken(s string) bool {
	i := 0
	if i < len(s) && s[i] == '-' {
		i++
	}
	switch {
	case i >= len(s):
		return false
	case s[i] == '0':
		i++
	case s[i] >= '1' && s[i] <= '9':
		i = skipDigits(s, i)
	default:
		return false
	}
	if i < len(s) && s[i] == '.' {
		j := skipDigits(s, i+1)
		if j == i+1 {
			return false
		}
		i = j
	}
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		i++
		if i < len(s) && (s[i] == '+' || s[i] == '-') {
			i++
		}
		j := skipDigits(s, i)
		if j == i {
			return false
		}
		i = j
	}
	return i == len(s)
}

func skipDigits(s string, i int) int {
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	return i
}
