package toon

const SpecVersion = "3.3"

type Kind int

const (
	NullKind Kind = iota
	BoolKind
	NumberKind
	StringKind
	ArrayKind
	ObjectKind
)

type Node struct {
	Kind   Kind
	Bool   bool
	String string
	Number Number
	Array  []*Node
	Object []Field
}

type Field struct {
	Key       string
	WasQuoted bool
	Value     *Node
}

type Number struct {
	Raw string
}
