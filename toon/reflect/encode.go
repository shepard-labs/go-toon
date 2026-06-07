package reflect

import (
	"encoding"
	"math"
	"reflect"
	"sort"
	"strconv"
	"time"
	"unsafe"

	"github.com/shepard-labs/go-toon/toon"
)

// valueWalker walks Go values and produces ordered *toon.Node trees.
type valueWalker struct {
	opts   ValueOptions
	limits *toon.LimitCounter
	seen   map[unsafe.Pointer]struct{}
}

// fromValue is the recursive entry point. It accounts for one node and one
// depth step before dispatching on the kind.
func (w *valueWalker) fromValue(v reflect.Value, depth int) (*toon.Node, error) {
	if err := w.limits.AddNode(); err != nil {
		return nil, err
	}
	if err := w.limits.CheckDepth(depth); err != nil {
		return nil, err
	}
	if !v.IsValid() {
		return &toon.Node{Kind: toon.NullKind}, nil
	}
	if v.CanInterface() {
		iface := v.Interface()
		if iface == nil {
			return &toon.Node{Kind: toon.NullKind}, nil
		}
		if w.opts.TimeFormatter != nil {
			if t, ok := iface.(time.Time); ok {
				return w.fromString(w.opts.TimeFormatter(t), depth+1)
			}
		}
		if tm, ok := iface.(encoding.TextMarshaler); ok {
			s, err := tm.MarshalText()
			if err != nil {
				return nil, toon.Errorf(toon.ErrInvalidInputFormat, "TextMarshaler.MarshalText failed: %v", err)
			}
			return w.fromString(string(s), depth+1)
		}
	}
	for {
		switch v.Kind() {
		case reflect.Ptr, reflect.Interface:
			if v.IsNil() {
				return &toon.Node{Kind: toon.NullKind}, nil
			}
			if v.Kind() == reflect.Ptr {
				ptr := unsafe.Pointer(v.Pointer())
				if _, ok := w.seen[ptr]; ok {
					return nil, toon.NewError(toon.ErrCyclicValue, "cyclic value detected")
				}
				w.seen[ptr] = struct{}{}
				defer delete(w.seen, ptr)
			}
			v = v.Elem()
			depth++
			if err := w.limits.CheckDepth(depth); err != nil {
				return nil, err
			}
			if !v.IsValid() {
				return &toon.Node{Kind: toon.NullKind}, nil
			}
		default:
			return w.fromConcrete(v, depth)
		}
	}
}

// fromConcrete dispatches on the reflect.Kind of a non-pointer value.
func (w *valueWalker) fromConcrete(v reflect.Value, depth int) (*toon.Node, error) {
	switch v.Kind() {
	case reflect.Bool:
		return &toon.Node{Kind: toon.BoolKind, Bool: v.Bool()}, nil
	case reflect.String:
		return w.fromString(v.String(), depth+1)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return w.fromInt(v.Int(), depth+1)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return w.fromUint(v.Uint(), depth+1)
	case reflect.Float32, reflect.Float64:
		return w.fromFloat(v.Float(), depth+1)
	case reflect.Slice, reflect.Array:
		if v.Kind() == reflect.Slice && v.Type().Elem().Kind() == reflect.Uint8 {
			return w.fromString(base64StdEncoding.EncodeToString(v.Bytes()), depth+1)
		}
		return w.fromSlice(v, depth+1)
	case reflect.Map:
		return w.fromMap(v, depth+1)
	case reflect.Struct:
		return w.fromStruct(v, depth+1)
	case reflect.Complex64, reflect.Complex128:
		return nil, toon.Errorf(toon.ErrUnsupportedKind, "complex numbers are not supported")
	case reflect.Chan, reflect.Func, reflect.UnsafePointer:
		return nil, toon.Errorf(toon.ErrUnsupportedKind, "%s is not supported", v.Kind())
	default:
		return nil, toon.Errorf(toon.ErrUnsupportedKind, "unsupported Go kind: %s", v.Kind())
	}
}

func (w *valueWalker) fromString(s string, depth int) (*toon.Node, error) {
	if err := w.limits.CheckStringBytes(len(s)); err != nil {
		return nil, err
	}
	if err := w.limits.CheckDepth(depth); err != nil {
		return nil, err
	}
	return &toon.Node{Kind: toon.StringKind, String: s}, nil
}

func (w *valueWalker) fromInt(n int64, depth int) (*toon.Node, error) {
	if err := w.limits.CheckDepth(depth); err != nil {
		return nil, err
	}
	var buf [24]byte
	raw := strconv.AppendInt(buf[:0], n, 10)
	return &toon.Node{Kind: toon.NumberKind, Number: toon.Number{Raw: string(raw)}}, nil
}

func (w *valueWalker) fromUint(n uint64, depth int) (*toon.Node, error) {
	if err := w.limits.CheckDepth(depth); err != nil {
		return nil, err
	}
	var buf [24]byte
	raw := strconv.AppendUint(buf[:0], n, 10)
	return &toon.Node{Kind: toon.NumberKind, Number: toon.Number{Raw: string(raw)}}, nil
}

func (w *valueWalker) fromFloat(f float64, depth int) (*toon.Node, error) {
	if err := w.limits.CheckDepth(depth); err != nil {
		return nil, err
	}
	if math.IsNaN(f) || math.IsInf(f, 0) {
		switch w.opts.NumberMode {
		case toon.NumberStringForUnsafe:
			raw := strconv.FormatFloat(f, 'g', -1, 64)
			return w.fromString(raw, depth+1)
		default:
			return nil, toon.Errorf(toon.ErrUnsupportedKind, "non-finite float is not allowed under NumberMode=%d", w.opts.NumberMode)
		}
	}
	var buf [32]byte
	raw := strconv.AppendFloat(buf[:0], f, 'g', -1, 64)
	return &toon.Node{Kind: toon.NumberKind, Number: toon.Number{Raw: string(raw)}}, nil
}

func (w *valueWalker) fromSlice(v reflect.Value, depth int) (*toon.Node, error) {
	if err := w.limits.CheckDepth(depth); err != nil {
		return nil, err
	}
	if err := w.limits.CheckArrayLength(v.Len()); err != nil {
		return nil, err
	}
	arr := &toon.Node{Kind: toon.ArrayKind}
	for i := 0; i < v.Len(); i++ {
		child, err := w.fromValue(v.Index(i), depth+1)
		if err != nil {
			return nil, err
		}
		arr.Array = append(arr.Array, child)
	}
	return arr, nil
}

func (w *valueWalker) fromMap(v reflect.Value, depth int) (*toon.Node, error) {
	if err := w.limits.CheckDepth(depth); err != nil {
		return nil, err
	}
	if v.Type().Key().Kind() != reflect.String {
		return nil, toon.Errorf(toon.ErrUnsupportedKind, "map keys must be strings; got %s", v.Type().Key())
	}
	keys := v.MapKeys()
	if err := w.limits.CheckArrayLength(len(keys)); err != nil {
		return nil, err
	}
	indices := make([]int, len(keys))
	for i := range indices {
		indices[i] = i
	}
	sort.SliceStable(indices, func(a, b int) bool {
		return keys[indices[a]].String() < keys[indices[b]].String()
	})
	obj := &toon.Node{Kind: toon.ObjectKind}
	for _, idx := range indices {
		kv := keys[idx]
		sv := kv.String()
		child, err := w.fromValue(v.MapIndex(kv), depth+1)
		if err != nil {
			return nil, err
		}
		obj.Object = append(obj.Object, toon.Field{Key: sv, Value: child})
	}
	return obj, nil
}

func (w *valueWalker) fromStruct(v reflect.Value, depth int) (*toon.Node, error) {
	if err := w.limits.CheckDepth(depth); err != nil {
		return nil, err
	}
	tagName := w.opts.TagName
	if tagName == "" {
		tagName = "toon"
	}
	fields := structFields(v.Type(), tagName)
	if err := w.limits.CheckArrayLength(len(fields)); err != nil {
		return nil, err
	}
	obj := &toon.Node{Kind: toon.ObjectKind}
	for _, fi := range fields {
		fv := v.FieldByName(fi.GoName)
		if !fv.IsValid() {
			continue
		}
		if fi.OmitEmpty && isEmptyValue(fv) {
			continue
		}
		child, err := w.fromValue(fv, depth+1)
		if err != nil {
			return nil, err
		}
		obj.Object = append(obj.Object, toon.Field{Key: fi.Key, Value: child})
	}
	return obj, nil
}
