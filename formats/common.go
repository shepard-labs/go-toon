package formats

import "github.com/shepard-labs/go-toon/toon"

type limits struct {
	c *toon.LimitCounter
}

func newLimits(l toon.ResourceLimits) limits { return limits{c: toon.NewLimitCounter(l)} }

func (l limits) node(n *toon.Node, depth int) error {
	if err := l.c.AddNode(); err != nil {
		return err
	}
	if err := l.c.CheckDepth(depth); err != nil {
		return err
	}
	switch n.Kind {
	case toon.StringKind:
		return l.c.CheckStringBytes(len(n.String))
	case toon.ArrayKind:
		return l.c.CheckArrayLength(len(n.Array))
	case toon.ObjectKind:
		for _, f := range n.Object {
			if err := l.c.CheckStringBytes(len(f.Key)); err != nil {
				return err
			}
		}
	}
	return nil
}

func inferCell(s string) *toon.Node {
	if s == "" {
		return &toon.Node{Kind: toon.StringKind, String: ""}
	}
	n, err := toon.ParsePrimitiveToken(s)
	if err != nil {
		return &toon.Node{Kind: toon.StringKind, String: s}
	}
	return n
}

func stringNode(s string) *toon.Node { return &toon.Node{Kind: toon.StringKind, String: s} }

func formatErr(message string, cause error) error {
	return &toon.Error{Code: toon.ErrInvalidInputFormat, Message: message, Cause: cause}
}

func duplicateErr(message string) error { return toon.NewError(toon.ErrDuplicateKey, message) }

func hasDuplicate(values []string) bool {
	seen := duplicateTracker{}
	for _, v := range values {
		if seen.has(v) {
			return true
		}
		seen.add(v)
	}
	return false
}

type duplicateTracker struct {
	keys  [8]string
	count int
	set   map[string]struct{}
}

func (t *duplicateTracker) has(key string) bool {
	if t.set != nil {
		_, ok := t.set[key]
		return ok
	}
	for i := 0; i < t.count; i++ {
		if t.keys[i] == key {
			return true
		}
	}
	return false
}

func (t *duplicateTracker) add(key string) {
	if t.set != nil {
		t.set[key] = struct{}{}
		return
	}
	if t.count < len(t.keys) {
		t.keys[t.count] = key
		t.count++
		return
	}
	t.set = make(map[string]struct{}, len(t.keys)+1)
	for i := 0; i < t.count; i++ {
		t.set[t.keys[i]] = struct{}{}
	}
	t.set[key] = struct{}{}
}

func upsert(fields []toon.Field, f toon.Field) []toon.Field {
	for i := range fields {
		if fields[i].Key == f.Key {
			fields[i] = f
			return fields
		}
	}
	return append(fields, f)
}
