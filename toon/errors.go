package toon

import (
	"errors"
	"fmt"
)

type ErrorCode string

const (
	ErrInvalidIndent           ErrorCode = "invalid_indent"
	ErrTabIndent               ErrorCode = "tab_indent"
	ErrInvalidEscape           ErrorCode = "invalid_escape"
	ErrUnterminatedString      ErrorCode = "unterminated_string"
	ErrArrayCountMismatch      ErrorCode = "array_count_mismatch"
	ErrTabularWidthMismatch    ErrorCode = "tabular_width_mismatch"
	ErrDuplicateKey            ErrorCode = "duplicate_key"
	ErrMalformedHeader         ErrorCode = "malformed_header"
	ErrHeaderDelimiterMismatch ErrorCode = "header_delimiter_mismatch"
	ErrMissingColon            ErrorCode = "missing_colon"
	ErrPathExpansionConflict   ErrorCode = "path_expansion_conflict"
	ErrResourceLimit           ErrorCode = "resource_limit"
	ErrInvalidInputFormat      ErrorCode = "invalid_input_format"
	ErrUnsupportedFeature      ErrorCode = "unsupported_feature"
)

type Error struct {
	Code    ErrorCode
	Line    int
	Column  int
	Message string
	Context string
	Cause   error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	msg := e.Message
	if msg == "" {
		msg = string(e.Code)
	}
	if e.Line > 0 && e.Column > 0 {
		return fmt.Sprintf("%s at line %d, column %d", msg, e.Line, e.Column)
	}
	if e.Line > 0 {
		return fmt.Sprintf("%s at line %d", msg, e.Line)
	}
	return msg
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func NewError(code ErrorCode, message string) *Error {
	return &Error{Code: code, Message: message}
}

func Errorf(code ErrorCode, format string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

func CodeOf(err error) ErrorCode {
	var toonErr *Error
	if errors.As(err, &toonErr) {
		return toonErr.Code
	}
	return ""
}
