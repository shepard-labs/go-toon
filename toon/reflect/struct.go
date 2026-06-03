package reflect

import (
	"reflect"
	"strings"
)

// fieldInfo describes a single struct field exposed by the value walker.
// GoName is the original Go field name used for reflect lookups; Key is the
// name that appears in the *toon.Node output (after tag renaming).
type fieldInfo struct {
	GoName string
	Key    string
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
		name, skip := parseFieldTag(sf.Tag, tagName)
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
		*out = append(*out, fieldInfo{GoName: sf.Name, Key: key})
	}
}

func parseFieldTag(tag reflect.StructTag, tagName string) (name string, skip bool) {
	if tagName == "" {
		return "", false
	}
	raw, ok := tag.Lookup(tagName)
	if !ok {
		return "", false
	}
	if i := strings.IndexByte(raw, ','); i >= 0 {
		raw = raw[:i]
	}
	if raw == "-" {
		return "", true
	}
	return raw, false
}
