package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/shepard-labs/go-toon/formats"
	"github.com/shepard-labs/go-toon/toon"
)

func main() {
	if code := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr); code != 0 {
		os.Exit(code)
	}
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "error: command required: encode, decode, or validate")
		return 2
	}
	var err error
	switch args[0] {
	case "encode":
		err = runEncode(args[1:], stdin, stdout, stderr)
	case "decode":
		err = runDecode(args[1:], stdin, stdout)
	case "validate":
		err = runValidate(args[1:], stdin)
	default:
		fmt.Fprintf(stderr, "error: unknown command %q\n", args[0])
		return 2
	}
	if err != nil {
		renderError(stderr, err)
		return 1
	}
	return 0
}

func runEncode(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("encode", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	inputFlag := fs.String("input", "", "input file")
	fs.StringVar(inputFlag, "i", "", "input file")
	output := fs.String("output", "", "output file")
	fs.StringVar(output, "o", "", "output file")
	format := fs.String("format", "auto", "input format")
	indent := fs.Int("indent", 2, "TOON indent")
	delimiter := fs.String("delimiter", "comma", "TOON delimiter")
	lengthMarkers := fs.Bool("length-markers", false, "emit TOON length markers")
	keyFolding := fs.String("key-folding", "off", "key folding")
	flattenDepth := fs.Int("flatten-depth", 0, "flatten depth")
	stats := fs.Bool("stats", false, "print stats")
	csvHeader := fs.String("csv-header", "auto", "CSV header mode")
	csvDelimiter := fs.String("csv-delimiter", "", "CSV delimiter")
	csvInfer := fs.Bool("csv-infer-types", true, "CSV infer types")
	csvRootKey := fs.String("csv-root-key", "", "CSV root key")
	yamlDocs := fs.String("yaml-docs", "error", "YAML document mode")
	yamlScalars := fs.String("yaml-scalars", "core", "YAML scalar mode")
	xmlAttrPrefix := fs.String("xml-attr-prefix", "@", "XML attribute prefix")
	xmlTextKey := fs.String("xml-text-key", "#text", "XML text key")
	xmlInfer := fs.Bool("xml-infer-types", true, "XML infer types")
	xmlMixed := fs.String("xml-mixed-content", "compact", "XML mixed content")
	xmlNamespaces := fs.String("xml-namespaces", "local", "XML namespaces")
	if err := fs.Parse(normalizeFlagArgs(args, map[string]bool{
		"--input": true, "-i": true, "--output": true, "-o": true, "--format": true, "--indent": true,
		"--delimiter": true, "--length-markers": true, "--key-folding": true, "--flatten-depth": true, "--csv-header": true,
		"--csv-delimiter": true, "--csv-root-key": true, "--yaml-docs": true, "--yaml-scalars": true,
		"--xml-attr-prefix": true, "--xml-text-key": true, "--xml-mixed-content": true, "--xml-namespaces": true,
		"--csv-infer-types": true, "--xml-infer-types": true, "--stats": false,
	})); err != nil {
		return err
	}
	input, err := resolveInput(*inputFlag, fs.Args())
	if err != nil {
		return err
	}
	data, err := readInput(input, stdin)
	if err != nil {
		return err
	}
	fmtName, err := resolveFormat(*format, input, true)
	if err != nil {
		return err
	}
	start := time.Now()
	var n *toon.Node
	switch fmtName {
	case "json":
		n, err = formats.FromJSON(bytes.NewReader(data), func(o *formats.JSONOptions) { o.Limits = cliLimits() })
	case "yaml":
		n, err = formats.FromYAML(bytes.NewReader(data), func(o *formats.YAMLOptions) {
			o.Documents = parseYAMLDocs(*yamlDocs)
			o.Scalars = parseYAMLScalars(*yamlScalars)
			o.Limits = cliLimits()
		})
	case "xml":
		n, err = formats.FromXML(bytes.NewReader(data), func(o *formats.XMLOptions) {
			o.AttributePrefix = *xmlAttrPrefix
			o.TextKey = *xmlTextKey
			o.InferTypes = *xmlInfer
			o.MixedContent = parseXMLMixed(*xmlMixed)
			o.Namespaces = parseXMLNamespaces(*xmlNamespaces)
			o.TrimWhitespaceText = true
			o.Limits = cliLimits()
		})
	case "csv":
		mode, sourceDelimiter := parseCSVSettings(*csvHeader, *csvDelimiter, input)
		n, err = formats.FromCSV(bytes.NewReader(data), func(o *formats.CSVOptions) {
			o.HeaderMode = mode
			o.Delimiter = sourceDelimiter
			o.InferTypes = *csvInfer
			o.Limits = cliLimits()
		})
		if err == nil && *csvRootKey != "" {
			n = &toon.Node{Kind: toon.ObjectKind, Object: []toon.Field{{Key: *csvRootKey, Value: n}}}
		}
	default:
		return toon.NewError(toon.ErrUnsupportedFeature, "encode format must be json, yaml, xml, csv, or auto")
	}
	if err != nil {
		return err
	}
	encOpts := encodeOptions(*indent, *delimiter, *lengthMarkers, *keyFolding, *flattenDepth)
	out, err := toon.Encode(n, func(o *toon.EncodeOptions) { *o = encOpts })
	if err != nil {
		return err
	}
	if err := writeOutput(*output, stdout, out); err != nil {
		return err
	}
	if *stats {
		fmt.Fprintf(stderr, "stats: input_bytes=%d output_bytes=%d elapsed=%s\n", len(data), len(out), time.Since(start).Round(time.Millisecond))
	}
	return nil
}

func runDecode(args []string, stdin io.Reader, stdout io.Writer) error {
	fs := flag.NewFlagSet("decode", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	output := fs.String("output", "", "output file")
	fs.StringVar(output, "o", "", "output file")
	strict := fs.Bool("strict", true, "strict decode")
	expandPaths := fs.String("expand-paths", "off", "path expansion")
	indent := fs.Int("indent", 2, "JSON indent")
	format := fs.String("format", "json", "output format")
	if err := fs.Parse(normalizeFlagArgs(args, map[string]bool{
		"--output": true, "-o": true, "--expand-paths": true, "--indent": true, "--format": true, "--strict": true,
	})); err != nil {
		return err
	}
	if *format != "json" {
		return toon.NewError(toon.ErrUnsupportedFeature, "decode output format must be json")
	}
	input, err := resolveInput("", fs.Args())
	if err != nil {
		return err
	}
	data, err := readInput(input, stdin)
	if err != nil {
		return err
	}
	n, err := toon.Decode(data, func(o *toon.DecodeOptions) {
		o.Strict = *strict
		o.ExpandPaths = parseExpandPaths(*expandPaths)
		o.IndentSize = 2
		o.Limits = cliLimits()
	})
	if err != nil {
		return err
	}
	var b bytes.Buffer
	if err := formats.ToJSON(&b, n, func(o *formats.JSONOutputOptions) { o.Indent = strings.Repeat(" ", *indent) }); err != nil {
		return err
	}
	return writeOutput(*output, stdout, b.Bytes())
}

func runValidate(args []string, stdin io.Reader) error {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	strict := fs.Bool("strict", true, "strict decode")
	indent := fs.Int("indent", 2, "TOON indent")
	if err := fs.Parse(normalizeFlagArgs(args, map[string]bool{"--indent": true, "--strict": true})); err != nil {
		return err
	}
	input, err := resolveInput("", fs.Args())
	if err != nil {
		return err
	}
	data, err := readInput(input, stdin)
	if err != nil {
		return err
	}
	return toon.Validate(data, func(o *toon.DecodeOptions) {
		o.Strict = *strict
		o.IndentSize = *indent
		o.Limits = cliLimits()
	})
}

func encodeOptions(indent int, delimiter string, lengthMarkers bool, folding string, flattenDepth int) toon.EncodeOptions {
	o := toon.DefaultEncodeOptions()
	o.IndentSize = indent
	o.Delimiter = parseDelimiter(delimiter)
	o.IncludeLengthMarkers = lengthMarkers
	o.KeyFolding = parseKeyFolding(folding)
	if flattenDepth > 0 {
		o.FlattenDepth = flattenDepth
	}
	o.Limits = cliLimits()
	return o
}

func resolveInput(flagInput string, positional []string) (string, error) {
	if flagInput != "" && len(positional) > 0 {
		return "", errors.New("input specified twice")
	}
	if flagInput != "" {
		return flagInput, nil
	}
	if len(positional) > 1 {
		return "", errors.New("too many input arguments")
	}
	if len(positional) == 1 {
		return positional[0], nil
	}
	return "", nil
}

func normalizeFlagArgs(args []string, valueFlags map[string]bool) []string {
	flags := make([]string, 0, len(args))
	positionals := make([]string, 0, 1)
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") && arg != "-" {
			flags = append(flags, arg)
			name := arg
			if eq := strings.IndexByte(name, '='); eq >= 0 {
				name = name[:eq]
			}
			if valueFlags[name] && !strings.Contains(arg, "=") && i+1 < len(args) {
				i++
				flags = append(flags, args[i])
			}
			continue
		}
		positionals = append(positionals, arg)
	}
	return append(flags, positionals...)
}

func readInput(path string, stdin io.Reader) ([]byte, error) {
	if path == "" || path == "-" {
		return io.ReadAll(stdin)
	}
	return os.ReadFile(path)
}

func writeOutput(path string, stdout io.Writer, data []byte) error {
	if path == "" || path == "-" {
		_, err := stdout.Write(data)
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func resolveFormat(format, input string, stdinAmbiguous bool) (string, error) {
	if format != "auto" {
		return format, nil
	}
	if input == "" || input == "-" {
		if stdinAmbiguous {
			return "", toon.NewError(toon.ErrInvalidInputFormat, "stdin encode requires --format")
		}
		return "toon", nil
	}
	switch strings.ToLower(filepath.Ext(input)) {
	case ".json":
		return "json", nil
	case ".yaml", ".yml":
		return "yaml", nil
	case ".xml":
		return "xml", nil
	case ".csv", ".tsv":
		return "csv", nil
	case ".toon":
		return "toon", nil
	}
	return "", toon.NewError(toon.ErrInvalidInputFormat, "could not auto-detect input format")
}

func parseDelimiter(s string) toon.Delimiter {
	switch s {
	case "tab":
		return toon.Tab
	case "pipe":
		return toon.Pipe
	default:
		return toon.Comma
	}
}

func parseKeyFolding(s string) toon.KeyFoldingMode {
	if s == "safe" {
		return toon.KeyFoldingSafe
	}
	return toon.KeyFoldingOff
}

func parseExpandPaths(s string) toon.PathExpansionMode {
	if s == "safe" {
		return toon.ExpandPathsSafe
	}
	return toon.ExpandPathsOff
}

func parseCSVSettings(header, delimiter, input string) (formats.CSVHeaderMode, rune) {
	delim := ','
	if strings.ToLower(filepath.Ext(input)) == ".tsv" {
		delim = '\t'
	}
	if delimiter != "" {
		delim = []rune(delimiter)[0]
	}
	switch header {
	case "present", "auto":
		return formats.CSVHeaderPresent, delim
	case "absent":
		return formats.CSVHeaderAbsent, delim
	default:
		return formats.CSVHeaderPresent, delim
	}
}

func parseYAMLDocs(s string) formats.YAMLDocumentMode {
	if s == "array" {
		return formats.YAMLDocumentsArray
	}
	return formats.YAMLDocumentsError
}

func parseYAMLScalars(s string) formats.YAMLScalarMode {
	if s == "string" {
		return formats.YAMLScalarsString
	}
	return formats.YAMLScalarsCore
}

func parseXMLMixed(s string) formats.XMLMixedContentMode {
	if s == "preserve" {
		return formats.XMLMixedContentPreserve
	}
	return formats.XMLMixedContentCompact
}

func parseXMLNamespaces(s string) formats.XMLNamespaceMode {
	switch s {
	case "qualified":
		return formats.XMLNamespacesQualified
	case "uri":
		return formats.XMLNamespacesURI
	default:
		return formats.XMLNamespacesLocal
	}
}

func cliLimits() toon.ResourceLimits {
	return toon.ResourceLimits{MaxDepth: 512, MaxStringBytes: 64 * 1024 * 1024}
}

func renderError(w io.Writer, err error) {
	var te *toon.Error
	if errors.As(err, &te) {
		fmt.Fprintf(w, "error: %s\n", te.Error())
		if te.Context != "" {
			if te.Line > 0 {
				fmt.Fprintf(w, "  %d: %s\n", te.Line, te.Context)
			} else {
				fmt.Fprintf(w, "  %s\n", te.Context)
			}
		}
		return
	}
	fmt.Fprintf(w, "error: %v\n", err)
}
