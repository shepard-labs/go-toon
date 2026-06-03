package toon

import (
	"fmt"
	"strings"
	"unicode/utf16"
)

func IsValidUnquotedKey(s string) bool {
	if s == "" || !isAlphaUnderscore(s[0]) {
		return false
	}
	for i := 1; i < len(s); i++ {
		if !isAlphaDigitUnderscoreDot(s[i]) {
			return false
		}
	}
	return true
}

func IsValidSafeSegment(s string) bool {
	if s == "" || !isAlphaUnderscore(s[0]) {
		return false
	}
	for i := 1; i < len(s); i++ {
		if !isAlphaDigitUnderscore(s[i]) {
			return false
		}
	}
	return true
}

func NeedsQuotes(s string, delimiter Delimiter) bool {
	if s == "" || strings.TrimSpace(s) != s {
		return true
	}
	lower := strings.ToLower(s)
	if lower == "true" || lower == "false" || lower == "null" {
		return true
	}
	if s == "-" || strings.HasPrefix(s, "-") || IsNumericLike(s) {
		return true
	}
	for _, r := range s {
		if r < 0x20 || r == ':' || r == '"' || r == '\\' || r == '[' || r == ']' || r == '{' || r == '}' || r == rune(delimiter) {
			return true
		}
	}
	return false
}

func EscapeString(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 {
				b.WriteString(`\u`)
				b.WriteString(strings.ToUpper(hex4(r)))
			} else {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}

func UnescapeQuotedToken(token string) (string, error) {
	if token == "" || token[0] != '"' {
		return "", NewError(ErrUnterminatedString, "quoted token must start with quote")
	}
	var b strings.Builder
	for i := 1; i < len(token); i++ {
		ch := token[i]
		switch ch {
		case '"':
			if strings.TrimSpace(token[i+1:]) != "" {
				return "", NewError(ErrInvalidEscape, "characters after closing quote")
			}
			return b.String(), nil
		case '\\':
			if i+1 >= len(token) {
				return "", NewError(ErrInvalidEscape, "backslash at end of string")
			}
			i++
			switch token[i] {
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
				if i+4 >= len(token) {
					return "", NewError(ErrInvalidEscape, "truncated unicode escape")
				}
				r, ok := parseHex4(token[i+1 : i+5])
				if !ok {
					return "", NewError(ErrInvalidEscape, "non-hex unicode escape")
				}
				if utf16.IsSurrogate(r) {
					return "", NewError(ErrInvalidEscape, "lone surrogate escape")
				}
				b.WriteRune(r)
				i += 4
			default:
				return "", NewError(ErrInvalidEscape, "unknown escape")
			}
		default:
			b.WriteByte(ch)
		}
	}
	return "", NewError(ErrUnterminatedString, "unterminated quoted string")
}

func SplitDelimited(s string, delimiter Delimiter) ([]string, error) {
	var tokens []string
	start := 0
	inQuote := false
	escaped := false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if inQuote {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inQuote = false
			}
			continue
		}
		if ch == '"' {
			inQuote = true
			continue
		}
		if ch == byte(delimiter) {
			tokens = append(tokens, strings.TrimSpace(s[start:i]))
			start = i + 1
		}
	}
	if inQuote {
		return nil, NewError(ErrUnterminatedString, "unterminated quoted string")
	}
	if escaped {
		return nil, NewError(ErrInvalidEscape, "backslash at end of string")
	}
	tokens = append(tokens, strings.TrimSpace(s[start:]))
	return tokens, nil
}

func ParsePrimitiveToken(token string) (*Node, error) {
	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		return &Node{Kind: StringKind, String: ""}, nil
	}
	if strings.HasPrefix(trimmed, "\"") {
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
	if IsValidNumberToken(trimmed) {
		return &Node{Kind: NumberKind, Number: Number{Raw: DecodeNumberToken(trimmed)}}, nil
	}
	return &Node{Kind: StringKind, String: trimmed}, nil
}

func isAlphaUnderscore(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || b == '_'
}

func isAlphaDigitUnderscore(b byte) bool {
	return isAlphaUnderscore(b) || (b >= '0' && b <= '9')
}

func isAlphaDigitUnderscoreDot(b byte) bool {
	return isAlphaDigitUnderscore(b) || b == '.'
}

func hex4(r rune) string {
	return fmt.Sprintf("%04x", r)
}

func parseHex4(s string) (rune, bool) {
	if len(s) != 4 {
		return 0, false
	}
	n := rune(0)
	for _, ch := range s {
		n <<= 4
		switch {
		case ch >= '0' && ch <= '9':
			n += ch - '0'
		case ch >= 'a' && ch <= 'f':
			n += ch - 'a' + 10
		case ch >= 'A' && ch <= 'F':
			n += ch - 'A' + 10
		default:
			return 0, false
		}
	}
	return n, true
}
