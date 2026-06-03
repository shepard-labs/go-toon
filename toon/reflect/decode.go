package reflect

import (
	"encoding"
	"reflect"
	"strconv"

	"github.com/shepard-labs/go-toon/toon"
)

// nodeWalker populates Go values from an ordered *toon.Node.
type nodeWalker struct {
	limits *toon.LimitCounter
}

func (w *nodeWalker) toValue(n *toon.Node, v reflect.Value, depth int) error {
	if err := w.limits.AddNode(); err != nil {
		return err
	}
	if err := w.limits.CheckDepth(depth); err != nil {
		return err
	}
	if n == nil {
		if v.CanSet() {
			v.Set(reflect.Zero(v.Type()))
		}
		return nil
	}
	if v.Kind() == reflect.Interface && v.NumMethod() == 0 {
		return w.assignAny(n, v, depth)
	}
	if v.CanAddr() {
		addr := v.Addr()
		if n.Kind == toon.StringKind {
			if u, ok := addr.Interface().(encoding.TextUnmarshaler); ok {
				return u.UnmarshalText([]byte(n.String))
			}
		}
	}
	switch n.Kind {
	case toon.NullKind:
		if v.CanSet() {
			v.Set(reflect.Zero(v.Type()))
		}
		return nil
	case toon.BoolKind:
		return w.assignBool(n, v)
	case toon.NumberKind:
		return w.assignNumber(n, v)
	case toon.StringKind:
		return w.assignString(n, v, depth)
	case toon.ArrayKind:
		return w.assignArray(n, v, depth)
	case toon.ObjectKind:
		return w.assignObject(n, v, depth)
	default:
		return toon.NewError(toon.ErrUnmarshalType, "unsupported node kind")
	}
}

func (w *nodeWalker) assignAny(n *toon.Node, v reflect.Value, depth int) error {
	if err := w.limits.CheckDepth(depth); err != nil {
		return err
	}
	if !v.CanSet() {
		return toon.NewError(toon.ErrNonPointerTarget, "cannot set interface value")
	}
	var out any
	switch n.Kind {
	case toon.NullKind:
		v.Set(reflect.Zero(v.Type()))
		return nil
	case toon.BoolKind:
		out = n.Bool
	case toon.NumberKind:
		f, err := strconv.ParseFloat(n.Number.Raw, 64)
		if err != nil {
			return toon.Errorf(toon.ErrUnmarshalType, "cannot parse number %q: %v", n.Number.Raw, err)
		}
		out = f
	case toon.StringKind:
		out = n.String
	case toon.ArrayKind:
		slice := make([]any, len(n.Array))
		for i, item := range n.Array {
			elem := reflect.New(reflect.TypeOf((*any)(nil)).Elem()).Elem()
			if err := w.toValue(item, elem, depth+1); err != nil {
				return err
			}
			slice[i] = elem.Interface()
		}
		out = slice
	case toon.ObjectKind:
		m := make(map[string]any, len(n.Object))
		for _, field := range n.Object {
			elem := reflect.New(reflect.TypeOf((*any)(nil)).Elem()).Elem()
			if err := w.toValue(field.Value, elem, depth+1); err != nil {
				return err
			}
			m[field.Key] = elem.Interface()
		}
		out = m
	default:
		return toon.NewError(toon.ErrUnmarshalType, "unsupported node kind for interface")
	}
	v.Set(reflect.ValueOf(out))
	return nil
}

func (w *nodeWalker) assignBool(n *toon.Node, v reflect.Value) error {
	if v.Kind() != reflect.Bool {
		return w.typeMismatch("bool", v)
	}
	v.SetBool(n.Bool)
	return nil
}

func (w *nodeWalker) assignNumber(n *toon.Node, v reflect.Value) error {
	raw := n.Number.Raw
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i, err := strconv.ParseInt(raw, 10, v.Type().Bits())
		if err != nil {
			return toon.Errorf(toon.ErrUnmarshalType, "cannot assign number %q to %s: %v", raw, v.Type(), err)
		}
		v.SetInt(i)
		return nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		u, err := strconv.ParseUint(raw, 10, v.Type().Bits())
		if err != nil {
			return toon.Errorf(toon.ErrUnmarshalType, "cannot assign number %q to %s: %v", raw, v.Type(), err)
		}
		v.SetUint(u)
		return nil
	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(raw, v.Type().Bits())
		if err != nil {
			return toon.Errorf(toon.ErrUnmarshalType, "cannot assign number %q to %s: %v", raw, v.Type(), err)
		}
		v.SetFloat(f)
		return nil
	default:
		return w.typeMismatch("number", v)
	}
}

func (w *nodeWalker) assignString(n *toon.Node, v reflect.Value, depth int) error {
	switch v.Kind() {
	case reflect.String:
		if err := w.limits.CheckStringBytes(len(n.String)); err != nil {
			return err
		}
		v.SetString(n.String)
		return nil
	case reflect.Slice:
		if v.Type().Elem().Kind() == reflect.Uint8 {
			data, err := base64StdEncoding.DecodeString(n.String)
			if err != nil {
				return toon.Errorf(toon.ErrUnmarshalType, "invalid base64 string: %v", err)
			}
			if v.IsNil() || v.Cap() < len(data) {
				v.Set(reflect.MakeSlice(v.Type(), len(data), len(data)))
			} else {
				v.Set(v.Slice(0, len(data)))
			}
			reflect.Copy(v, reflect.ValueOf(data))
			return nil
		}
	}
	return w.typeMismatch("string", v)
}

func (w *nodeWalker) assignArray(n *toon.Node, v reflect.Value, depth int) error {
	switch v.Kind() {
	case reflect.Slice:
		return w.assignSlice(n, v, depth+1)
	case reflect.Array:
		return w.assignFixedArray(n, v, depth+1)
	default:
		return w.typeMismatch("array", v)
	}
}

func (w *nodeWalker) assignSlice(n *toon.Node, v reflect.Value, depth int) error {
	if err := w.limits.CheckDepth(depth); err != nil {
		return err
	}
	if err := w.limits.CheckArrayLength(len(n.Array)); err != nil {
		return err
	}
	if v.IsNil() || v.Cap() < len(n.Array) {
		v.Set(reflect.MakeSlice(v.Type(), len(n.Array), len(n.Array)))
	} else {
		v.Set(v.Slice(0, len(n.Array)))
	}
	for i, item := range n.Array {
		if err := w.toValue(item, v.Index(i), depth+1); err != nil {
			return err
		}
	}
	return nil
}

func (w *nodeWalker) assignFixedArray(n *toon.Node, v reflect.Value, depth int) error {
	if err := w.limits.CheckDepth(depth); err != nil {
		return err
	}
	if err := w.limits.CheckArrayLength(len(n.Array)); err != nil {
		return err
	}
	if len(n.Array) > v.Len() {
		return toon.Errorf(toon.ErrUnmarshalType, "array of length %d does not fit in [%d]%s", len(n.Array), v.Len(), v.Type().Elem())
	}
	zero := reflect.Zero(v.Type().Elem())
	for i := 0; i < v.Len(); i++ {
		if i < len(n.Array) {
			if err := w.toValue(n.Array[i], v.Index(i), depth+1); err != nil {
				return err
			}
		} else {
			v.Index(i).Set(zero)
		}
	}
	return nil
}

func (w *nodeWalker) assignObject(n *toon.Node, v reflect.Value, depth int) error {
	switch v.Kind() {
	case reflect.Struct:
		return w.assignStruct(n, v, depth+1)
	case reflect.Map:
		return w.assignMap(n, v, depth+1)
	default:
		return w.typeMismatch("object", v)
	}
}

func (w *nodeWalker) assignStruct(n *toon.Node, v reflect.Value, depth int) error {
	if err := w.limits.CheckDepth(depth); err != nil {
		return err
	}
	if err := w.limits.CheckArrayLength(len(n.Object)); err != nil {
		return err
	}
	fields := structFields(v.Type(), "toon")
	byKey := make(map[string]fieldInfo, len(fields))
	for _, fi := range fields {
		byKey[fi.Key] = fi
	}
	for _, field := range n.Object {
		fi, ok := byKey[field.Key]
		if !ok {
			continue
		}
		fv := v.FieldByName(fi.GoName)
		if !fv.IsValid() || !fv.CanSet() {
			continue
		}
		if err := w.toValue(field.Value, fv, depth+1); err != nil {
			return err
		}
	}
	return nil
}

func (w *nodeWalker) assignMap(n *toon.Node, v reflect.Value, depth int) error {
	if err := w.limits.CheckDepth(depth); err != nil {
		return err
	}
	if v.Type().Key().Kind() != reflect.String {
		return toon.NewError(toon.ErrUnmarshalType, "map keys must be strings for unmarshal")
	}
	if err := w.limits.CheckArrayLength(len(n.Object)); err != nil {
		return err
	}
	if v.IsNil() {
		v.Set(reflect.MakeMapWithSize(v.Type(), len(n.Object)))
	}
	elemType := v.Type().Elem()
	keyType := v.Type().Key()
	for _, field := range n.Object {
		if err := w.limits.CheckStringBytes(len(field.Key)); err != nil {
			return err
		}
		elemPtr := reflect.New(elemType)
		if err := w.toValue(field.Value, elemPtr.Elem(), depth+1); err != nil {
			return err
		}
		keyVal := reflect.New(keyType).Elem()
		keyVal.SetString(field.Key)
		v.SetMapIndex(keyVal, elemPtr.Elem())
	}
	return nil
}

func (w *nodeWalker) typeMismatch(want string, v reflect.Value) error {
	return toon.Errorf(toon.ErrUnmarshalType, "cannot assign %s to %s", want, v.Type())
}
