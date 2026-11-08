package toon

import (
	"strings"
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

// IsValidSafeSegment reports whether s is a dot-free identifier segment.
//
// Deprecated: it served key folding and path expansion, which TOON v4 removed.
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

// NeedsQuotes reports whether an encoder must quote s as a string value under
// SPEC §7.2, given the document delimiter. The root-primitive U+FEFF rule is
// position-dependent and handled by the encoder.
func NeedsQuotes(s string, delimiter Delimiter) bool {
	if s == "" || isSpaceOrTab(s[0]) || isSpaceOrTab(s[len(s)-1]) {
		return true
	}
	if s == "true" || s == "false" || s == "null" {
		return true
	}
	if s[0] == '-' || s[0] == '#' || isQuotingNumericLike(s) {
		return true
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < 0x20 || c == ':' || c == '"' || c == '\\' || c == '[' || c == ']' || c == '{' || c == '}' || c == byte(delimiter) {
			return true
		}
	}
	return false
}

// EscapeString escapes s for use between double quotes per SPEC §7.1: the five
// short escapes, lowercase \u00xx for other C0 controls, everything else literal.
func EscapeString(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
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
			if c < 0x20 {
				b.WriteString(`\u00`)
				b.WriteByte("0123456789abcdef"[c>>4])
				b.WriteByte("0123456789abcdef"[c&0xf])
			} else {
				b.WriteByte(c)
			}
		}
	}
	return b.String()
}

func isSpaceOrTab(b byte) bool {
	return b == ' ' || b == '\t'
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
