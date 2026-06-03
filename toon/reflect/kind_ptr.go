package reflect

import "reflect"

// ptrKind is the reflect.Kind constant for pointer types. Aliasing it
// locally avoids the reflect package-name collision at every call site.
const ptrKind = reflect.Ptr
