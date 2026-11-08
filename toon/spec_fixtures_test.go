package toon_test

// Conformance harness for the language-agnostic TOON spec fixtures vendored in
// testdata/spec (see scripts/sync-spec-fixtures.sh).
//
// Cases are keyed by "<category>/<file>#<index>" (e.g. "decode/objects.json#3"),
// never by their prose names. knownFailures lists cases that the current
// implementation does not yet satisfy; every entry must be removed once the
// case passes (TestSpecKnownFailuresStillFail enforces this).
//
// To regenerate the knownFailures entries for the current failures:
//
//	SPEC_PRINT_FAILURES=1 go test ./toon -run TestSpecFixtures -v

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/shepard-labs/go-toon/toon"
)

// specTarget is the spec version the fixtures are run against. Cases whose
// minSpecVersion is newer are skipped.
const specTarget = "4.4"

const specFixturesDir = "testdata/spec/fixtures"

type specFile struct {
	Version     string     `json:"version"`
	Category    string     `json:"category"`
	Description string     `json:"description"`
	Tests       []specCase `json:"tests"`
}

type specCase struct {
	Name           string          `json:"name"`
	Input          json.RawMessage `json:"input"`
	Expected       json.RawMessage `json:"expected"`
	ShouldError    bool            `json:"shouldError"`
	Options        specOptions     `json:"options"`
	SpecSection    string          `json:"specSection"`
	Note           string          `json:"note"`
	MinSpecVersion string          `json:"minSpecVersion"`
}

type specOptions struct {
	Delimiter  *string `json:"delimiter"`
	IndentSize *int    `json:"indentSize"`
	Strict     *bool   `json:"strict"`
}

func (o specOptions) String() string {
	var parts []string
	if o.Delimiter != nil {
		parts = append(parts, fmt.Sprintf("delimiter=%q", *o.Delimiter))
	}
	if o.IndentSize != nil {
		parts = append(parts, fmt.Sprintf("indentSize=%d", *o.IndentSize))
	}
	if o.Strict != nil {
		parts = append(parts, fmt.Sprintf("strict=%t", *o.Strict))
	}
	if len(parts) == 0 {
		return "{}"
	}
	return "{" + strings.Join(parts, " ") + "}"
}

// specRun is one fixture case with its stable key.
type specRun struct {
	Key      string // "<category>/<file>#<index>"
	Category string
	File     string
	Index    int
	Case     specCase
}

func loadSpecRuns(t testing.TB) []specRun {
	t.Helper()
	var runs []specRun
	for _, category := range []string{"encode", "decode"} {
		paths, err := filepath.Glob(filepath.Join(specFixturesDir, category, "*.json"))
		if err != nil {
			t.Fatal(err)
		}
		if len(paths) == 0 {
			t.Fatalf("no %s fixtures under %s; run scripts/sync-spec-fixtures.sh", category, specFixturesDir)
		}
		sort.Strings(paths)
		for _, path := range paths {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var f specFile
			if err := json.Unmarshal(data, &f); err != nil {
				t.Fatalf("%s: %v", path, err)
			}
			if f.Category != category {
				t.Fatalf("%s: category %q does not match directory %q", path, f.Category, category)
			}
			name := filepath.Base(path)
			for i, c := range f.Tests {
				runs = append(runs, specRun{
					Key:      fmt.Sprintf("%s/%s#%d", category, name, i),
					Category: category,
					File:     name,
					Index:    i,
					Case:     c,
				})
			}
		}
	}
	return runs
}

// versionAfter reports whether spec version a ("major.minor") is newer than b.
func versionAfter(a, b string) bool {
	parse := func(v string) (int, int) {
		maj, min, _ := strings.Cut(v, ".")
		x, _ := strconv.Atoi(maj)
		y, _ := strconv.Atoi(min)
		return x, y
	}
	am, an := parse(a)
	bm, bn := parse(b)
	return am > bm || (am == bm && an > bn)
}

// specResult is the outcome of running one case. Message is empty on success.
type specResult struct {
	Message string
}

func (r specResult) failed() bool { return r.Message != "" }

// runSpecCase executes a case and describes any failure. Panics in the
// implementation under test are reported as failures.
func runSpecCase(run specRun) (res specResult) {
	c := run.Case
	defer func() {
		if p := recover(); p != nil {
			res = specResult{Message: fmt.Sprintf("panic: %v", p)}
		}
	}()
	describe := func(problem, input, expected, got string) specResult {
		return specResult{Message: fmt.Sprintf("%s\n  case:     %s (%s, spec §%s)\n  options:  %s\n  input:    %s\n  expected: %s\n  got:      %s",
			problem, run.Key, c.Name, strings.TrimPrefix(c.SpecSection, "§"), c.Options, input, expected, got)}
	}

	switch run.Category {
	case "encode":
		root, err := parseOrderedJSON(c.Input)
		if err != nil {
			return specResult{Message: fmt.Sprintf("%s: bad fixture input: %v", run.Key, err)}
		}
		out, err := toon.Encode(root, encodeOptions(c.Options)...)
		if c.ShouldError {
			if err == nil {
				return describe("expected an encode error but encoding succeeded", string(c.Input), "<error>", strconv.Quote(string(out)))
			}
			return specResult{}
		}
		var want string
		if err := json.Unmarshal(c.Expected, &want); err != nil {
			return specResult{Message: fmt.Sprintf("%s: bad fixture expected (want string): %v", run.Key, err)}
		}
		if err != nil {
			return describe("unexpected encode error: "+err.Error(), string(c.Input), strconv.Quote(want), "<error: "+err.Error()+">")
		}
		if string(out) != want {
			return describe("encoded output differs", string(c.Input), strconv.Quote(want), strconv.Quote(string(out)))
		}
	case "decode":
		var input string
		if err := json.Unmarshal(c.Input, &input); err != nil {
			return specResult{Message: fmt.Sprintf("%s: bad fixture input (want string): %v", run.Key, err)}
		}
		got, err := toon.Decode([]byte(input), decodeOptions(c.Options)...)
		if c.ShouldError {
			if err == nil {
				return describe("expected a decode error but decoding succeeded", strconv.Quote(input), "<error>", renderNode(got))
			}
			return specResult{}
		}
		want, perr := parseOrderedJSON(c.Expected)
		if perr != nil {
			return specResult{Message: fmt.Sprintf("%s: bad fixture expected: %v", run.Key, perr)}
		}
		if err != nil {
			return describe("unexpected decode error: "+err.Error(), strconv.Quote(input), renderNode(want), "<error: "+err.Error()+">")
		}
		if !nodesEqual(want, got) {
			return describe("decoded value differs", strconv.Quote(input), renderNode(want), renderNode(got))
		}
	default:
		return specResult{Message: "unknown category " + run.Category}
	}
	return specResult{}
}

func encodeOptions(o specOptions) []toon.EncodeOption {
	var opts []toon.EncodeOption
	if o.Delimiter != nil {
		d := toon.Comma
		if *o.Delimiter != "" {
			d = toon.Delimiter((*o.Delimiter)[0])
		}
		opts = append(opts, func(eo *toon.EncodeOptions) { eo.Delimiter = d })
	}
	if o.IndentSize != nil {
		n := *o.IndentSize
		opts = append(opts, func(eo *toon.EncodeOptions) { eo.IndentSize = n })
	}
	return opts
}

func decodeOptions(o specOptions) []toon.DecodeOption {
	var opts []toon.DecodeOption
	if o.IndentSize != nil {
		n := *o.IndentSize
		opts = append(opts, func(do *toon.DecodeOptions) { do.IndentSize = n })
	}
	if o.Strict != nil {
		s := *o.Strict
		opts = append(opts, func(do *toon.DecodeOptions) { do.Strict = s })
	}
	return opts
}

// --- ordered JSON -> *toon.Node -------------------------------------------

func parseOrderedJSON(raw json.RawMessage) (*toon.Node, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	n, err := parseJSONValue(dec)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, fmt.Errorf("trailing data after JSON value")
	}
	return n, nil
}

func parseJSONValue(dec *json.Decoder) (*toon.Node, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch v := tok.(type) {
	case nil:
		return &toon.Node{Kind: toon.NullKind}, nil
	case bool:
		return &toon.Node{Kind: toon.BoolKind, Bool: v}, nil
	case string:
		return &toon.Node{Kind: toon.StringKind, String: v}, nil
	case json.Number:
		return &toon.Node{Kind: toon.NumberKind, Number: toon.Number{Raw: v.String()}}, nil
	case json.Delim:
		switch v {
		case '[':
			n := &toon.Node{Kind: toon.ArrayKind, Array: []*toon.Node{}}
			for dec.More() {
				item, err := parseJSONValue(dec)
				if err != nil {
					return nil, err
				}
				n.Array = append(n.Array, item)
			}
			if _, err := dec.Token(); err != nil {
				return nil, err
			}
			return n, nil
		case '{':
			n := &toon.Node{Kind: toon.ObjectKind, Object: []toon.Field{}}
			for dec.More() {
				keyTok, err := dec.Token()
				if err != nil {
					return nil, err
				}
				key, ok := keyTok.(string)
				if !ok {
					return nil, fmt.Errorf("non-string object key %v", keyTok)
				}
				val, err := parseJSONValue(dec)
				if err != nil {
					return nil, err
				}
				n.Object = append(n.Object, toon.Field{Key: key, Value: val})
			}
			if _, err := dec.Token(); err != nil {
				return nil, err
			}
			return n, nil
		}
	}
	return nil, fmt.Errorf("unexpected JSON token %v", tok)
}

// --- comparison and rendering ---------------------------------------------

// nodesEqual compares ordered trees. Numbers compare by value; WasQuoted is
// ignored.
func nodesEqual(a, b *toon.Node) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.Kind != b.Kind {
		return false
	}
	switch a.Kind {
	case toon.NullKind:
		return true
	case toon.BoolKind:
		return a.Bool == b.Bool
	case toon.StringKind:
		return a.String == b.String
	case toon.NumberKind:
		return normalizeNumber(a.Number.Raw) == normalizeNumber(b.Number.Raw)
	case toon.ArrayKind:
		if len(a.Array) != len(b.Array) {
			return false
		}
		for i := range a.Array {
			if !nodesEqual(a.Array[i], b.Array[i]) {
				return false
			}
		}
		return true
	case toon.ObjectKind:
		if len(a.Object) != len(b.Object) {
			return false
		}
		for i := range a.Object {
			if a.Object[i].Key != b.Object[i].Key || !nodesEqual(a.Object[i].Value, b.Object[i].Value) {
				return false
			}
		}
		return true
	}
	return false
}

// normalizeNumber reduces a JSON-style decimal to "<sign><digits>e<exp>" with
// no leading/trailing zero digits, so equal values compare equal regardless of
// notation. Zero (including -0) normalizes to "0". Unparseable input is
// returned unchanged.
func normalizeNumber(raw string) string {
	s := raw
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(strings.TrimPrefix(s, "-"), "+")
	exp := 0
	if i := strings.IndexAny(s, "eE"); i >= 0 {
		e, err := strconv.Atoi(strings.TrimPrefix(s[i+1:], "+"))
		if err != nil {
			return raw
		}
		exp = e
		s = s[:i]
	}
	if i := strings.IndexByte(s, '.'); i >= 0 {
		exp -= len(s) - i - 1
		s = s[:i] + s[i+1:]
	}
	if s == "" || strings.Trim(s, "0123456789") != "" {
		return raw
	}
	s = strings.TrimLeft(s, "0")
	if s == "" {
		return "0"
	}
	trimmed := strings.TrimRight(s, "0")
	exp += len(s) - len(trimmed)
	out := trimmed + "e" + strconv.Itoa(exp)
	if neg {
		out = "-" + out
	}
	return out
}

// renderNode prints a node as ordered, compact JSON-like text for diagnostics.
func renderNode(n *toon.Node) string {
	if n == nil {
		return "<nil>"
	}
	var sb strings.Builder
	writeNode(&sb, n)
	return sb.String()
}

func writeNode(sb *strings.Builder, n *toon.Node) {
	if n == nil {
		sb.WriteString("<nil>")
		return
	}
	switch n.Kind {
	case toon.NullKind:
		sb.WriteString("null")
	case toon.BoolKind:
		sb.WriteString(strconv.FormatBool(n.Bool))
	case toon.StringKind:
		b, _ := json.Marshal(n.String)
		sb.Write(b)
	case toon.NumberKind:
		sb.WriteString(n.Number.Raw)
	case toon.ArrayKind:
		sb.WriteByte('[')
		for i, item := range n.Array {
			if i > 0 {
				sb.WriteByte(',')
			}
			writeNode(sb, item)
		}
		sb.WriteByte(']')
	case toon.ObjectKind:
		sb.WriteByte('{')
		for i, f := range n.Object {
			if i > 0 {
				sb.WriteByte(',')
			}
			b, _ := json.Marshal(f.Key)
			sb.Write(b)
			sb.WriteByte(':')
			writeNode(sb, f.Value)
		}
		sb.WriteByte('}')
	default:
		fmt.Fprintf(sb, "<kind %d>", n.Kind)
	}
}

// --- tests ----------------------------------------------------------------

// TestSpecFixtures runs every fixture case. Cases listed in knownFailures are
// skipped (with their reason) when they fail as expected.
func TestSpecFixtures(t *testing.T) {
	runs := loadSpecRuns(t)
	print := os.Getenv("SPEC_PRINT_FAILURES") != ""
	var generated []string
	for _, run := range runs {
		run := run
		if versionAfter(run.Case.MinSpecVersion, specTarget) {
			continue
		}
		t.Run(fmt.Sprintf("%s/%s/%d", run.Category, run.File, run.Index), func(t *testing.T) {
			res := runSpecCase(run)
			reason, known := knownFailures[run.Key]
			switch {
			case res.failed() && known:
				if print {
					generated = append(generated, formatKnownFailure(run, res))
				}
				t.Skipf("known failure: %s", reason)
			case res.failed():
				if print {
					generated = append(generated, formatKnownFailure(run, res))
				}
				t.Error(res.Message)
			case known:
				t.Errorf("%s is listed in knownFailures (%q) but now passes; delete the entry", run.Key, reason)
			}
		})
	}
	if print {
		sort.Strings(generated)
		fmt.Printf("// ---- generated knownFailures entries (%d) ----\n%s\n", len(generated), strings.Join(generated, "\n"))
	}
}

func formatKnownFailure(run specRun, res specResult) string {
	kind := strings.SplitN(res.Message, "\n", 2)[0]
	switch {
	case strings.HasPrefix(kind, "panic"):
		kind = "panic"
	case strings.HasPrefix(kind, "expected"):
		kind = "accepts invalid input"
	case strings.HasPrefix(kind, "unexpected"):
		kind = "rejects valid input"
	case strings.HasPrefix(kind, "encoded"), strings.HasPrefix(kind, "decoded"):
		kind = "output differs"
	}
	return fmt.Sprintf("\t%q: %q,", run.Key, kind+": "+run.Case.Name)
}

// TestSpecKnownFailuresStillFail guards the knownFailures map itself: every
// entry must name an existing, version-eligible case that still fails, so
// later phases are forced to delete entries as they fix behavior.
func TestSpecKnownFailuresStillFail(t *testing.T) {
	runs := loadSpecRuns(t)
	byKey := make(map[string]specRun, len(runs))
	for _, run := range runs {
		byKey[run.Key] = run
	}
	keys := make([]string, 0, len(knownFailures))
	for k := range knownFailures {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		run, ok := byKey[key]
		switch {
		case !ok:
			t.Errorf("knownFailures entry %q does not match any fixture case; delete it", key)
		case versionAfter(run.Case.MinSpecVersion, specTarget):
			t.Errorf("knownFailures entry %q is skipped (minSpecVersion %s); delete it", key, run.Case.MinSpecVersion)
		case !runSpecCase(run).failed():
			t.Errorf("knownFailures entry %q (%s) now passes; delete it", key, knownFailures[key])
		}
	}
}

// TestSpecFixturesLoaded sanity-checks the vendored fixture set so a bad sync
// cannot silently shrink coverage.
func TestSpecFixturesLoaded(t *testing.T) {
	counts := map[string]int{}
	for _, run := range loadSpecRuns(t) {
		counts[run.Category]++
	}
	if counts["encode"] == 0 || counts["decode"] == 0 {
		t.Fatalf("expected both encode and decode cases, got %v", counts)
	}
	t.Logf("spec fixture cases: %v", counts)
}

// knownFailures maps "<category>/<file>#<index>" to a short reason. Entries are
// removed as the implementation is brought up to the targeted spec version; the
// map must be empty when the upgrade is complete.
var knownFailures = map[string]string{}
