package toon

import (
	"strconv"
	"strings"
)

func IsNumericLike(s string) bool {
	if s == "" {
		return false
	}
	i := 0
	if s[i] == '-' {
		i++
		if i == len(s) {
			return false
		}
	}
	digits := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
		digits++
	}
	if digits == 0 {
		return false
	}
	if i < len(s) && s[i] == '.' {
		i++
		fracDigits := 0
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
			fracDigits++
		}
		if fracDigits == 0 {
			return false
		}
	}
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		i++
		if i < len(s) && (s[i] == '+' || s[i] == '-') {
			i++
		}
		expDigits := 0
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
			expDigits++
		}
		if expDigits == 0 {
			return false
		}
	}
	return i == len(s)
}

func IsValidNumberToken(s string) bool {
	if !IsNumericLike(s) {
		return false
	}
	i := 0
	if s[i] == '-' {
		i++
	}
	intStart := i
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	intPart := s[intStart:i]
	return len(intPart) == 1 || intPart[0] != '0'
}

func CanonicalizeNumberToken(s string) string {
	canonical, _ := canonicalizeLosslessNumber(s)
	if canonical != "" {
		return canonical
	}
	return s
}

func DecodeNumberToken(s string) string {
	if !IsValidNumberToken(s) {
		return s
	}
	_, digits, _ := parseDecimalParts(s)
	if trimLeadingZeros(digits) == "" {
		return "0"
	}
	return s
}

func canonicalizeNumber(s string, mode NumberMode) (string, bool, error) {
	switch mode {
	case NumberFloat64:
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return "", false, Errorf(ErrInvalidInputFormat, "invalid number %q", s)
		}
		return canonicalizeFloat64(f), false, nil
	case NumberStringForUnsafe:
		canonical, unsafe, err := canonicalizeLosslessNumberWithSafety(s)
		return canonical, unsafe, err
	default:
		canonical, err := canonicalizeLosslessNumber(s)
		return canonical, false, err
	}
}

func canonicalizeFloat64(f float64) string {
	s := strconv.FormatFloat(f, 'g', -1, 64)
	canonical, err := canonicalizeLosslessNumber(s)
	if err != nil {
		return strings.ToLower(s)
	}
	return canonical
}

func canonicalizeLosslessNumber(s string) (string, error) {
	canonical, _, err := canonicalizeLosslessNumberWithSafety(s)
	return canonical, err
}

func canonicalizeLosslessNumberWithSafety(s string) (string, bool, error) {
	if !IsValidNumberToken(s) {
		return "", false, Errorf(ErrInvalidInputFormat, "invalid number %q", s)
	}
	sign, digits, power := parseDecimalParts(s)
	digits = trimLeadingZeros(digits)
	if digits == "" {
		return "0", false, nil
	}
	digits, power = trimTrailingZeroDigits(digits, power)
	adjusted := len(digits) + power - 1
	unsafe := adjusted < -6 || adjusted >= 21
	if !unsafe {
		return sign + plainDecimal(digits, power), false, nil
	}
	return sign + exponentDecimal(digits, power), true, nil
}

func parseDecimalParts(s string) (sign string, digits string, power int) {
	if strings.HasPrefix(s, "-") {
		sign = "-"
		s = s[1:]
	}
	if e := strings.IndexAny(s, "eE"); e >= 0 {
		exp, _ := strconv.Atoi(s[e+1:])
		power += exp
		s = s[:e]
	}
	if dot := strings.IndexByte(s, '.'); dot >= 0 {
		power -= len(s) - dot - 1
		s = s[:dot] + s[dot+1:]
	}
	return sign, s, power
}

func trimLeadingZeros(s string) string {
	i := 0
	for i < len(s) && s[i] == '0' {
		i++
	}
	return s[i:]
}

func trimTrailingZeroDigits(digits string, power int) (string, int) {
	for strings.HasSuffix(digits, "0") {
		digits = digits[:len(digits)-1]
		power++
	}
	return digits, power
}

func plainDecimal(digits string, power int) string {
	if power >= 0 {
		return digits + strings.Repeat("0", power)
	}
	point := len(digits) + power
	if point > 0 {
		frac := strings.TrimRight(digits[point:], "0")
		if frac == "" {
			return digits[:point]
		}
		return digits[:point] + "." + frac
	}
	return "0." + strings.Repeat("0", -point) + digits
}

func exponentDecimal(digits string, power int) string {
	exp := len(digits) + power - 1
	mantissa := digits[:1]
	if len(digits) > 1 {
		mantissa += "." + strings.TrimRight(digits[1:], "0")
	}
	if strings.HasSuffix(mantissa, ".") {
		mantissa = mantissa[:len(mantissa)-1]
	}
	if exp >= 0 {
		return mantissa + "e+" + strconv.Itoa(exp)
	}
	return mantissa + "e" + strconv.Itoa(exp)
}
