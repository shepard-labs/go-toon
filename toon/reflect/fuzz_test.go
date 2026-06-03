package reflect

import (
	"bytes"
	"reflect"
	"testing"
	"unicode/utf8"

	"github.com/shepard-labs/go-toon/toon"
)

func FuzzMarshalValue(f *testing.F) {
	f.Add(int64(0), "x", []byte("[]byte string"))
	f.Add(int64(42), "name", []byte(""))
	f.Add(int64(-1), "k", []byte("abc"))
	f.Add(int64(1<<62), "k", []byte("abc"))

	f.Fuzz(func(t *testing.T, id int64, key string, raw []byte) {
		type S struct {
			ID  int64  `toon:"id"`
			Key string `toon:"key"`
			Raw []byte `toon:"raw"`
		}
		v := S{ID: id, Key: key, Raw: raw}
		var buf bytes.Buffer
		if err := Marshal(&buf, v); err != nil {
			return
		}
		node, err := toon.Decode(buf.Bytes())
		if err != nil {
			t.Fatalf("decode of fuzzed marshal: %v\nout=%q", err, buf.String())
		}
		var got S
		if err := UnmarshalNode(node, &got); err != nil {
			t.Fatalf("UnmarshalNode of fuzzed marshal: %v\nout=%q", err, buf.String())
		}
		if !utf8.ValidString(key) {
			return
		}
		if !reflect.DeepEqual(got, v) {
			t.Fatalf("round-trip mismatch: got %+v want %+v\nout=%q", got, v, buf.String())
		}
	})
}

func FuzzUnmarshalValue(f *testing.F) {
	f.Add("id: 1\nname: x\nraw: SGk=")
	f.Add("id: 1\nname: x\nraw: \"\"\n")
	f.Add("id: 0\nname: \"\"\nraw: \"\"\n")
	f.Add("id: -1\nname: a\nraw: YQ==\n")
	f.Add("id: 1\nname: x\n")
	f.Add("a:\n b: 1\n c: 2\n")

	f.Fuzz(func(t *testing.T, in string) {
		type S struct {
			ID   int64  `toon:"id"`
			Name string `toon:"name"`
			Raw  []byte `toon:"raw"`
		}
		var dst S
		_ = Unmarshal([]byte(in), &dst)
	})
}
