package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEncodeJSONFileToStdout(t *testing.T) {
	dir := t.TempDir()
	in := writeTemp(t, dir, "in.json", `{"a":1,"b":"x"}`)
	code, out, errOut := runCLI([]string{"encode", in}, "")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, errOut)
	}
	if out != "a: 1\nb: x" {
		t.Fatalf("stdout = %q", out)
	}
}

func TestEncodeInputOutputFlags(t *testing.T) {
	dir := t.TempDir()
	in := writeTemp(t, dir, "in.json", `{"a":1}`)
	out := filepath.Join(dir, "out.toon")
	code, stdout, stderr := runCLI([]string{"encode", "--input", in, "--output", out}, "")
	if code != 0 || stdout != "" {
		t.Fatalf("code=%d stdout=%q stderr=%s", code, stdout, stderr)
	}
	if got := readFile(t, out); got != "a: 1" {
		t.Fatalf("output file = %q", got)
	}
}

func TestEncodeJSONFromStdinRequiresFormat(t *testing.T) {
	code, out, stderr := runCLI([]string{"encode", "--format", "json"}, `{"a":1}`)
	if code != 0 || out != "a: 1" {
		t.Fatalf("code=%d out=%q stderr=%s", code, out, stderr)
	}
	code, out, stderr = runCLI([]string{"encode"}, `{"a":1}`)
	if code == 0 || out != "" || !strings.Contains(stderr, "stdin encode requires --format") {
		t.Fatalf("expected stdin format error, code=%d out=%q stderr=%q", code, out, stderr)
	}
}

func TestEncodeAutoDetection(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]string{
		"in.json": `{"a":1}`,
		"in.yaml": "a: 1\n",
		"in.yml":  "a: 1\n",
		"in.xml":  "<a>1</a>",
		"in.csv":  "a\n1\n",
		"in.tsv":  "a\tb\n1\t2\n",
	}
	wants := map[string]string{
		"in.json": "a: 1",
		"in.yaml": "a: 1",
		"in.yml":  "a: 1",
		"in.xml":  "a: 1",
		"in.csv":  "[1]{a}:\n  1",
		"in.tsv":  "[1]{a,b}:\n  1,2",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			path := writeTemp(t, dir, name, body)
			code, out, stderr := runCLI([]string{"encode", path}, "")
			if code != 0 || out != wants[name] {
				t.Fatalf("code=%d out=%q stderr=%s want=%q", code, out, stderr, wants[name])
			}
		})
	}
}

func TestEncodeFlags(t *testing.T) {
	code, out, stderr := runCLI([]string{"encode", "--format", "json", "--delimiter", "pipe"}, `{"tags":["a,b","c|d"]}`)
	if code != 0 || out != "tags[2|]: a,b|\"c|d\"" {
		t.Fatalf("delimiter code=%d out=%q stderr=%s", code, out, stderr)
	}
	code, out, stderr = runCLI([]string{"encode", "--format", "json", "--key-folding", "safe", "--flatten-depth", "2"}, `{"a":{"b":{"c":1}}}`)
	if code != 0 || out != "a.b:\n  c: 1" {
		t.Fatalf("folding code=%d out=%q stderr=%s", code, out, stderr)
	}
	code, out, stderr = runCLI([]string{"encode", "--format", "json", "--length-markers"}, `{"tags":["a","b"]}`)
	if code != 0 || out != "tags[#2]: a,b" {
		t.Fatalf("length markers code=%d out=%q stderr=%s", code, out, stderr)
	}
}

func TestEncodeCSVFlags(t *testing.T) {
	code, out, stderr := runCLI([]string{"encode", "--format", "csv", "--csv-header", "absent", "--csv-delimiter", "|", "--csv-infer-types=false", "--csv-root-key", "rows"}, "1|true\n")
	if code != 0 || out != "rows[1]{field1,field2}:\n  \"1\",\"true\"" {
		t.Fatalf("CSV flags code=%d out=%q stderr=%s", code, out, stderr)
	}
}

func TestEncodeYAMLAndXMLFlags(t *testing.T) {
	code, out, stderr := runCLI([]string{"encode", "--format", "yaml", "--yaml-docs", "array", "--yaml-scalars", "string"}, "a: 1\n---\nb: true\n")
	if code != 0 || out != "[2]:\n  - a: \"1\"\n  - b: \"true\"" {
		t.Fatalf("YAML flags code=%d out=%q stderr=%s", code, out, stderr)
	}
	code, out, stderr = runCLI([]string{"encode", "--format", "xml", "--xml-attr-prefix", "attr_", "--xml-text-key", "text", "--xml-infer-types=false", "--xml-mixed-content", "preserve", "--xml-namespaces", "local"}, `<a id="1">x</a>`)
	if code != 0 || out != "a:\n  attr_id: \"1\"\n  text: x" {
		t.Fatalf("XML flags code=%d out=%q stderr=%s", code, out, stderr)
	}
}

func TestDecodeFlags(t *testing.T) {
	dir := t.TempDir()
	in := writeTemp(t, dir, "in.toon", "a.b: 1")
	code, out, stderr := runCLI([]string{"decode", in, "--expand-paths", "safe", "--indent", "4"}, "")
	if code != 0 {
		t.Fatalf("decode code=%d stderr=%s", code, stderr)
	}
	if out != "{\n    \"a\": {\n        \"b\": 1\n    }\n}" {
		t.Fatalf("decode out = %q", out)
	}
	code, _, stderr = runCLI([]string{"decode", "--strict=false", "--format", "json"}, "a: 1\na: 2")
	if code != 0 || !strings.Contains(stderr, "") {
		t.Fatalf("decode non-strict code=%d stderr=%q", code, stderr)
	}
}

func TestValidate(t *testing.T) {
	dir := t.TempDir()
	valid := writeTemp(t, dir, "valid.toon", "a: 1")
	code, out, stderr := runCLI([]string{"validate", valid}, "")
	if code != 0 || out != "" || stderr != "" {
		t.Fatalf("valid code=%d out=%q stderr=%q", code, out, stderr)
	}
	invalid := writeTemp(t, dir, "invalid.toon", "a[2]: 1")
	code, out, stderr = runCLI([]string{"validate", invalid}, "")
	if code == 0 || out != "" || !strings.Contains(stderr, "array count mismatch") || !strings.Contains(stderr, "1: a[2]: 1") {
		t.Fatalf("invalid code=%d out=%q stderr=%q", code, out, stderr)
	}
}

func TestIOFailuresAndStats(t *testing.T) {
	code, out, stderr := runCLI([]string{"encode", "missing.json"}, "")
	if code == 0 || out != "" || !strings.Contains(stderr, "missing.json") {
		t.Fatalf("input failure code=%d out=%q stderr=%q", code, out, stderr)
	}
	dir := t.TempDir()
	in := writeTemp(t, dir, "in.json", `{"a":1}`)
	code, out, stderr = runCLI([]string{"encode", in, "--output", dir}, "")
	if code == 0 || out != "" || !strings.Contains(stderr, "is a directory") {
		t.Fatalf("output failure code=%d out=%q stderr=%q", code, out, stderr)
	}
	code, out, stderr = runCLI([]string{"encode", in, "--stats"}, "")
	if code != 0 || out != "a: 1" || !strings.Contains(stderr, "stats:") {
		t.Fatalf("stats code=%d out=%q stderr=%q", code, out, stderr)
	}
}

func runCLI(args []string, input string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	code := run(args, strings.NewReader(input), &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func writeTemp(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
