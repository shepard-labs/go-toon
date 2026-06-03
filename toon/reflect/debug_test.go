package reflect

import (
	"bytes"
	"reflect"
	"testing"
)

type docExample struct {
	ID    int      `toon:"id"`
	Name  string   `toon:"name"`
	Tags  []string `toon:"tags"`
	Score float64  `toon:"score"`
}

func TestReadmeExampleRoundTrip(t *testing.T) {
	in := docExample{ID: 42, Name: "Ada", Tags: []string{"a", "b"}, Score: 9.5}
	var buf bytes.Buffer
	if err := Marshal(&buf, in); err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out docExample
	if err := Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reflect.DeepEqual(out, in) {
		t.Fatalf("round-trip: got %+v want %+v", out, in)
	}
}
