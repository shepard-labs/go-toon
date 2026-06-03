package reflect

import "reflect"

// reflectValueOf is a thin alias to stdlib reflect.ValueOf. It exists so
// the rest of the package can refer to it without colliding with this
// package's own name.
func reflectValueOf(v any) reflect.Value {
	return reflect.ValueOf(v)
}
