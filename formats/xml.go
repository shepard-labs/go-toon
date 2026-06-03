package formats

import (
	"bytes"
	"encoding/xml"
	"io"
	"strings"

	"github.com/shepard-labs/go-toon/toon"
)

func FromXML(r io.Reader, opts ...XMLOption) (*toon.Node, error) {
	o := resolveXMLOptions(opts...)
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	lim := newLimits(o.Limits)
	if err := lim.c.CheckInputBytes(int64(len(data))); err != nil {
		return nil, err
	}
	dec := xml.NewDecoder(bytes.NewReader(data))
	stack := []*xmlElement{}
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, formatErr("invalid XML", err)
		}
		switch t := tok.(type) {
		case xml.Directive:
			if strings.HasPrefix(strings.TrimSpace(string(t)), "DOCTYPE") {
				return nil, toon.NewError(toon.ErrInvalidInputFormat, "XML DTD is not supported")
			}
		case xml.StartElement:
			el := &xmlElement{name: xmlName(t.Name, o), attrs: t.Attr}
			stack = append(stack, el)
		case xml.CharData:
			if len(stack) == 0 {
				continue
			}
			text := string(t)
			if o.TrimWhitespaceText {
				if strings.TrimSpace(text) == "" {
					continue
				}
				text = strings.TrimSpace(text)
			}
			stack[len(stack)-1].parts = append(stack[len(stack)-1].parts, xmlPart{text: text})
		case xml.EndElement:
			if len(stack) == 0 {
				return nil, formatErr("unexpected XML end element", nil)
			}
			el := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			value, err := xmlElementNode(el, o, lim, len(stack)+1)
			if err != nil {
				return nil, err
			}
			field := toon.Field{Key: el.name, Value: value}
			if len(stack) == 0 {
				root := &toon.Node{Kind: toon.ObjectKind, Object: []toon.Field{field}}
				if err := lim.node(root, 0); err != nil {
					return nil, err
				}
				return root, nil
			}
			stack[len(stack)-1].parts = append(stack[len(stack)-1].parts, xmlPart{field: field})
		}
	}
	return nil, formatErr("missing XML root", nil)
}

type xmlElement struct {
	name  string
	attrs []xml.Attr
	parts []xmlPart
}

type xmlPart struct {
	text  string
	field toon.Field
}

func xmlElementNode(el *xmlElement, o XMLOptions, lim limits, depth int) (*toon.Node, error) {
	attrs := []toon.Field{}
	for _, attr := range el.attrs {
		if attr.Name.Space == "xmlns" || attr.Name.Local == "xmlns" {
			continue
		}
		attrs = append(attrs, toon.Field{Key: o.AttributePrefix + xmlName(attr.Name, o), Value: xmlScalar(attr.Value, o)})
	}
	childFields := []toon.Field{}
	texts := []string{}
	for _, part := range el.parts {
		if part.field.Key != "" {
			childFields = appendXMLField(childFields, part.field)
		} else {
			texts = append(texts, part.text)
		}
	}
	if len(attrs) == 0 && len(childFields) == 0 && len(texts) == 1 {
		n := xmlScalar(texts[0], o)
		return n, lim.node(n, depth)
	}
	if o.MixedContent == XMLMixedContentPreserve && len(el.parts) > 1 && len(childFields) > 0 && len(texts) > 0 {
		arr := &toon.Node{Kind: toon.ArrayKind}
		for _, part := range el.parts {
			if part.field.Key != "" {
				arr.Array = append(arr.Array, &toon.Node{Kind: toon.ObjectKind, Object: []toon.Field{part.field}})
			} else {
				arr.Array = append(arr.Array, &toon.Node{Kind: toon.ObjectKind, Object: []toon.Field{{Key: o.TextKey, Value: xmlScalar(part.text, o)}}})
			}
		}
		if err := lim.node(arr, depth); err != nil {
			return nil, err
		}
		for _, item := range arr.Array {
			if err := lim.node(item, depth+1); err != nil {
				return nil, err
			}
		}
		return arr, nil
	}
	obj := &toon.Node{Kind: toon.ObjectKind, Object: attrs}
	if len(texts) > 0 {
		obj.Object = append(obj.Object, toon.Field{Key: o.TextKey, Value: xmlScalar(strings.Join(texts, " "), o)})
	}
	obj.Object = append(obj.Object, childFields...)
	return obj, lim.node(obj, depth)
}

func appendXMLField(fields []toon.Field, field toon.Field) []toon.Field {
	for i := range fields {
		if fields[i].Key == field.Key {
			if fields[i].Value.Kind != toon.ArrayKind {
				fields[i].Value = &toon.Node{Kind: toon.ArrayKind, Array: []*toon.Node{fields[i].Value}}
			}
			fields[i].Value.Array = append(fields[i].Value.Array, field.Value)
			return fields
		}
	}
	return append(fields, field)
}

func xmlScalar(s string, o XMLOptions) *toon.Node {
	if o.InferTypes {
		return inferCell(s)
	}
	return stringNode(s)
}

func xmlName(name xml.Name, o XMLOptions) string {
	switch o.Namespaces {
	case XMLNamespacesURI:
		if name.Space != "" {
			return "{" + name.Space + "}" + name.Local
		}
	case XMLNamespacesQualified:
		if name.Space != "" {
			return name.Space + ":" + name.Local
		}
	}
	return name.Local
}
