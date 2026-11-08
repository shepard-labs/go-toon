package reflect

import (
	"bytes"
	"reflect"
	"testing"
)

type rtServer struct {
	Host string `toon:"host"`
	Port int    `toon:"port"`
}

type rtGeo struct {
	Lat int `toon:"lat"`
	Lon int `toon:"lon"`
}

type rtPlace struct {
	ID  int   `toon:"id"`
	Geo rtGeo `toon:"geo"`
}

type rtTask struct {
	ID    int      `toon:"id"`
	Tags  []string `toon:"tags"`
	Owner rtGeo    `toon:"owner"`
}

type rtDoc struct {
	Servers map[string]rtServer `toon:"servers"`
	Places  []rtPlace           `toon:"places"`
	Tasks   []rtTask            `toon:"tasks"`
	NoTags  []string            `toon:"no_tags"`
	NoMap   map[string]int      `toon:"no_map"`
	Meta    struct{}            `toon:"meta"`
}

func rtMarshal(t *testing.T, v any) string {
	t.Helper()
	var buf bytes.Buffer
	if err := Marshal(&buf, v); err != nil {
		t.Fatalf("Marshal error: %v", err)
	}
	return buf.String()
}

func TestRoundTripKeyedTabularMap(t *testing.T) {
	in := map[string]rtServer{"alpha": {"a", 1}, "beta": {"b", 2}}
	out := rtMarshal(t, in)
	if want := "[2:]{host,port}:\n  alpha: a,1\n  beta: b,2"; out != want {
		t.Fatalf("Marshal = %q, want %q", out, want)
	}
	var got map[string]rtServer
	if err := Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if !reflect.DeepEqual(got, in) {
		t.Fatalf("round trip = %#v, want %#v", got, in)
	}
}

func TestRoundTripNestedFieldGroups(t *testing.T) {
	in := []rtPlace{{1, rtGeo{5, 6}}, {2, rtGeo{7, 8}}}
	out := rtMarshal(t, in)
	if want := "[2]{id,geo{lat,lon}}:\n  1,5,6\n  2,7,8"; out != want {
		t.Fatalf("Marshal = %q, want %q", out, want)
	}
	var got []rtPlace
	if err := Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if !reflect.DeepEqual(got, in) {
		t.Fatalf("round trip = %#v, want %#v", got, in)
	}
}

func TestRoundTripListItemFirstFieldLayouts(t *testing.T) {
	// Non-uniform rows fall back to list form with the first field on the hyphen line.
	in := []rtTask{
		{ID: 1, Tags: []string{"a", "b"}, Owner: rtGeo{1, 2}},
		{ID: 2, Tags: []string{}, Owner: rtGeo{3, 4}},
	}
	out := rtMarshal(t, in)
	var got []rtTask
	if err := Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("Unmarshal(%q) error: %v", out, err)
	}
	if !reflect.DeepEqual(got, in) {
		t.Fatalf("round trip of %q = %#v, want %#v", out, got, in)
	}

	// First field is itself an array or an object.
	type firstArray struct {
		Tags []string `toon:"tags"`
		N    int      `toon:"n"`
	}
	type firstObject struct {
		Geo rtGeo `toon:"geo"`
		N   int   `toon:"n"`
	}
	arrays := []firstArray{{[]string{"x", "y"}, 1}, {[]string{"z"}, 2}}
	var gotArrays []firstArray
	out = rtMarshal(t, arrays)
	if err := Unmarshal([]byte(out), &gotArrays); err != nil || !reflect.DeepEqual(gotArrays, arrays) {
		t.Fatalf("array-first round trip of %q = %#v, %v", out, gotArrays, err)
	}
	objects := []firstObject{{rtGeo{1, 2}, 1}, {rtGeo{3, 4}, 2}}
	var gotObjects []firstObject
	out = rtMarshal(t, objects)
	if err := Unmarshal([]byte(out), &gotObjects); err != nil || !reflect.DeepEqual(gotObjects, objects) {
		t.Fatalf("object-first round trip of %q = %#v, %v", out, gotObjects, err)
	}
}

func TestRoundTripEmptyContainers(t *testing.T) {
	in := rtDoc{
		Servers: map[string]rtServer{"alpha": {"a", 1}, "beta": {"b", 2}},
		Places:  []rtPlace{{1, rtGeo{5, 6}}, {2, rtGeo{7, 8}}},
		Tasks:   []rtTask{{ID: 1, Tags: []string{"a"}, Owner: rtGeo{1, 2}}},
		NoTags:  []string{},
		NoMap:   map[string]int{},
	}
	out := rtMarshal(t, in)
	var got rtDoc
	if err := Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("Unmarshal(%q) error: %v", out, err)
	}
	if !reflect.DeepEqual(got, in) {
		t.Fatalf("round trip of %q = %#v, want %#v", out, got, in)
	}

	if out := rtMarshal(t, []int{}); out != "[]" {
		t.Fatalf("empty slice = %q", out)
	}
	if out := rtMarshal(t, map[string]int{}); out != "" {
		t.Fatalf("empty map = %q", out)
	}
	var empty []int
	if err := Unmarshal([]byte("[]"), &empty); err != nil || len(empty) != 0 {
		t.Fatalf("Unmarshal empty array = %#v, %v", empty, err)
	}
}
