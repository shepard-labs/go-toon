package toon

import "testing"

func BenchmarkEncodeLargeOrderedObject(b *testing.B) {
	n := &Node{Kind: ObjectKind}
	for i := range 1000 {
		n.Object = append(n.Object, Field{Key: "field_" + benchItoa(i), Value: &Node{Kind: NumberKind, Number: Number{Raw: benchItoa(i)}}})
	}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := Encode(n); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEncodeLargeTabularArray(b *testing.B) {
	rows := &Node{Kind: ArrayKind}
	for i := range 1000 {
		rows.Array = append(rows.Array, &Node{Kind: ObjectKind, Object: []Field{
			{Key: "id", Value: &Node{Kind: NumberKind, Number: Number{Raw: benchItoa(i)}}},
			{Key: "name", Value: &Node{Kind: StringKind, String: "user_" + benchItoa(i)}},
			{Key: "active", Value: &Node{Kind: BoolKind, Bool: i%2 == 0}},
		}})
	}
	n := &Node{Kind: ObjectKind, Object: []Field{{Key: "rows", Value: rows}}}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := Encode(n); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecodeRepresentativeTOON(b *testing.B) {
	n := &Node{Kind: ObjectKind, Object: []Field{{Key: "rows", Value: &Node{Kind: ArrayKind}}}}
	for i := range 1000 {
		n.Object[0].Value.Array = append(n.Object[0].Value.Array, &Node{Kind: ObjectKind, Object: []Field{
			{Key: "id", Value: &Node{Kind: NumberKind, Number: Number{Raw: benchItoa(i)}}},
			{Key: "name", Value: &Node{Kind: StringKind, String: "user_" + benchItoa(i)}},
		}})
	}
	data, err := Encode(n)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := Decode(data); err != nil {
			b.Fatal(err)
		}
	}
}

func benchItoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
