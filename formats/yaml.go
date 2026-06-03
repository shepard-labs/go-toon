package formats

import (
	"bytes"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/shepard-labs/go-toon/toon"
	"gopkg.in/yaml.v3"
)

func FromYAML(r io.Reader, opts ...YAMLOption) (*toon.Node, error) {
	o := resolveYAMLOptions(opts...)
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	lim := newLimits(o.Limits)
	if err := lim.c.CheckInputBytes(int64(len(data))); err != nil {
		return nil, err
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	docs := []*toon.Node{}
	for {
		var doc yaml.Node
		if err := dec.Decode(&doc); err == io.EOF {
			break
		} else if err != nil {
			return nil, formatErr("invalid YAML", err)
		}
		if len(doc.Content) == 0 {
			continue
		}
		n, err := yamlNodeToToon(doc.Content[0], o, lim, 0, nil)
		if err != nil {
			return nil, err
		}
		docs = append(docs, n)
	}
	if len(docs) == 0 {
		n := &toon.Node{Kind: toon.NullKind}
		return n, lim.node(n, 0)
	}
	if len(docs) > 1 {
		if o.Documents != YAMLDocumentsArray {
			return nil, formatErr("multiple YAML documents", nil)
		}
		n := &toon.Node{Kind: toon.ArrayKind, Array: docs}
		return n, lim.node(n, 0)
	}
	return docs[0], nil
}

func yamlNodeToToon(n *yaml.Node, o YAMLOptions, lim limits, depth int, stack []*yaml.Node) (*toon.Node, error) {
	if slices.Contains(stack, n) {
		return nil, formatErr("YAML alias cycle", nil)
	}
	stack = append(stack, n)
	switch n.Kind {
	case yaml.AliasNode:
		return yamlNodeToToon(n.Alias, o, lim, depth, stack)
	case yaml.SequenceNode:
		out := &toon.Node{Kind: toon.ArrayKind}
		if err := lim.node(out, depth); err != nil {
			return nil, err
		}
		for _, child := range n.Content {
			item, err := yamlNodeToToon(child, o, lim, depth+1, stack)
			if err != nil {
				return nil, err
			}
			out.Array = append(out.Array, item)
			if err := lim.c.CheckArrayLength(len(out.Array)); err != nil {
				return nil, err
			}
		}
		return out, nil
	case yaml.MappingNode:
		out := &toon.Node{Kind: toon.ObjectKind}
		if err := lim.node(out, depth); err != nil {
			return nil, err
		}
		for i := 0; i < len(n.Content); i += 2 {
			keyNode, valNode := n.Content[i], n.Content[i+1]
			if keyNode.Value == "<<" {
				merge, err := yamlNodeToToon(valNode, o, lim, depth+1, stack)
				if err != nil {
					return nil, err
				}
				if merge.Kind == toon.ObjectKind {
					for _, f := range merge.Object {
						out.Object = upsert(out.Object, f)
					}
				}
				continue
			}
			key := yamlKeyString(keyNode)
			value, err := yamlNodeToToon(valNode, o, lim, depth+1, stack)
			if err != nil {
				return nil, err
			}
			out.Object = upsert(out.Object, toon.Field{Key: key, Value: value})
		}
		return out, nil
	case yaml.ScalarNode:
		out := yamlScalarToToon(n, o)
		return out, lim.node(out, depth)
	}
	out := stringNode(n.Value)
	return out, lim.node(out, depth)
}

func yamlScalarToToon(n *yaml.Node, o YAMLOptions) *toon.Node {
	if o.Scalars == YAMLScalarsString || n.Tag == "!!timestamp" || (strings.HasPrefix(n.Tag, "!") && !strings.HasPrefix(n.Tag, "!!")) {
		return stringNode(n.Value)
	}
	switch n.Tag {
	case "!!null":
		return &toon.Node{Kind: toon.NullKind}
	case "!!bool":
		return &toon.Node{Kind: toon.BoolKind, Bool: n.Value == "true" || n.Value == "True" || n.Value == "TRUE"}
	case "!!int", "!!float":
		clean := strings.ReplaceAll(n.Value, "_", "")
		if toon.IsValidNumberToken(clean) {
			return &toon.Node{Kind: toon.NumberKind, Number: toon.Number{Raw: clean}}
		}
	}
	return stringNode(n.Value)
}

func yamlKeyString(n *yaml.Node) string {
	if n.Kind == yaml.ScalarNode {
		return n.Value
	}
	if n.Kind == yaml.SequenceNode {
		return strconv.Itoa(len(n.Content))
	}
	return n.Value
}
