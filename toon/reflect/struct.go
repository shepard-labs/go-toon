package reflect

import (
	"reflect"
	"strings"
)

// fieldInfo describes a single struct field exposed by the value walker.
// GoName is the original Go field name used for reflect lookups; Key is the
// name that appears in the *toon.Node output (after tag renaming).
type fieldInfo struct {
	GoName    string
	Key       string
	OmitEmpty bool
}

// structFields returns the visible, sorted-by-declaration-order fields of t.
// Unexported fields and fields with tag value "-" are skipped. Embedded
// struct fields are flattened.
func structFields(t reflect.Type, tagName string) []fieldInfo {
	seen := make(map[string]bool)
	out := make([]fieldInfo, 0, t.NumField())
	collectFields(t, &out, seen, tagName)
	return out
}

func collectFields(t reflect.Type, out *[]fieldInfo, seen map[string]bool, tagName string) {
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		if sf.Anonymous {
			ft := sf.Type
			for ft.Kind() == reflect.Ptr {
				ft = ft.Elem()
			}
			if ft.Kind() == reflect.Struct {
				collectFields(ft, out, seen, tagName)
				continue
			}
		}
		if !sf.IsExported() {
			continue
		}
		name, skip, omitEmpty := parseFieldTag(sf.Tag, tagName)
		if skip {
			continue
		}
		key := name
		if key == "" {
			key = sf.Name
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		*out = append(*out, fieldInfo{GoName: sf.Name, Key: key, OmitEmpty: omitEmpty})
	}
}

func parseFieldTag(tag reflect.StructTag, tagName string) (name string, skip bool, omitEmpty bool) {
	if tagName == "" {
		return "", false, false
	}
	raw, ok := tag.Lookup(tagName)
	if !ok {
		return "", false, false
	}
	parts := strings.Split(raw, ",")
	if parts[0] == "-" {
		return "", true, false
	}
	for _, opt := range parts[1:] {
		if opt == "omitempty" {
			omitEmpty = true
		}
	}
	return parts[0], false, omitEmpty
}

func isEmptyValue(v reflect.Value) bool {
	if !v.IsValid() {
		return true
	}
	for v.Kind() == reflect.Interface || v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return true
		}
		v = v.Elem()
	}
	return v.IsZero()
}
