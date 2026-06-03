package formats

import "github.com/shepard-labs/go-toon/toon"

type JSONOptions struct {
	AllowDuplicateKeys bool
	Limits             toon.ResourceLimits
}

type JSONOption func(*JSONOptions)

type JSONOutputOptions struct {
	Indent string
}

type JSONOutputOption func(*JSONOutputOptions)

type YAMLDocumentMode int

const (
	YAMLDocumentsError YAMLDocumentMode = iota
	YAMLDocumentsArray
)

type YAMLScalarMode int

const (
	YAMLScalarsCore YAMLScalarMode = iota
	YAMLScalarsString
)

type YAMLOptions struct {
	Documents YAMLDocumentMode
	Scalars   YAMLScalarMode
	Limits    toon.ResourceLimits
}

type YAMLOption func(*YAMLOptions)

type CSVHeaderMode int

const (
	CSVHeaderExplicit CSVHeaderMode = iota
	CSVHeaderPresent
	CSVHeaderAbsent
	CSVHeaderAuto
)

type CSVOptions struct {
	HeaderMode      CSVHeaderMode
	Delimiter       rune
	InferTypes      bool
	AllowRaggedRows bool
	Limits          toon.ResourceLimits
}

type CSVOption func(*CSVOptions)

type XMLMixedContentMode int

const (
	XMLMixedContentCompact XMLMixedContentMode = iota
	XMLMixedContentPreserve
)

type XMLNamespaceMode int

const (
	XMLNamespacesLocal XMLNamespaceMode = iota
	XMLNamespacesQualified
	XMLNamespacesURI
)

type XMLOptions struct {
	AttributePrefix    string
	TextKey            string
	InferTypes         bool
	TrimWhitespaceText bool
	MixedContent       XMLMixedContentMode
	Namespaces         XMLNamespaceMode
	Limits             toon.ResourceLimits
}

type XMLOption func(*XMLOptions)

type JSONToTOONOptions struct {
	JSON   JSONOptions
	Encode toon.EncodeOptions
}

type YAMLToTOONOptions struct {
	YAML   YAMLOptions
	Encode toon.EncodeOptions
}

type XMLToTOONOptions struct {
	XML    XMLOptions
	Encode toon.EncodeOptions
}

type CSVToTOONOptions struct {
	CSV    CSVOptions
	Encode toon.EncodeOptions
}

type TOONToJSONOptions struct {
	Decode toon.DecodeOptions
	JSON   JSONOutputOptions
}

func resolveJSONOptions(opts ...JSONOption) JSONOptions {
	var o JSONOptions
	for _, opt := range opts {
		if opt != nil {
			opt(&o)
		}
	}
	return o
}

func resolveJSONOutputOptions(opts ...JSONOutputOption) JSONOutputOptions {
	var o JSONOutputOptions
	for _, opt := range opts {
		if opt != nil {
			opt(&o)
		}
	}
	return o
}

func resolveYAMLOptions(opts ...YAMLOption) YAMLOptions {
	var o YAMLOptions
	for _, opt := range opts {
		if opt != nil {
			opt(&o)
		}
	}
	return o
}

func resolveCSVOptions(opts ...CSVOption) CSVOptions {
	o := CSVOptions{Delimiter: ','}
	for _, opt := range opts {
		if opt != nil {
			opt(&o)
		}
	}
	if o.Delimiter == 0 {
		o.Delimiter = ','
	}
	return o
}

func resolveXMLOptions(opts ...XMLOption) XMLOptions {
	o := XMLOptions{AttributePrefix: "@", TextKey: "#text", InferTypes: true, TrimWhitespaceText: true}
	for _, opt := range opts {
		if opt != nil {
			opt(&o)
		}
	}
	if o.AttributePrefix == "" {
		o.AttributePrefix = "@"
	}
	if o.TextKey == "" {
		o.TextKey = "#text"
	}
	return o
}
