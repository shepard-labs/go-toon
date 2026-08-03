# go-toon MCP Server

`go-toon-mcp` exposes this repository's TOON validator and converters to MCP-compatible agents. It is intentionally small and uses the local Go library directly instead of reimplementing TOON parsing rules.

## Tools

- `toon_validate`: validates TOON and returns structured parser error details.
- `toon_to_json`: decodes TOON and returns ordered JSON for inspection.
- `json_to_toon`: converts JSON to canonical TOON through `formats.FromJSON` and `toon.Encode`.
- `yaml_to_toon`: converts YAML to canonical TOON through `formats.FromYAML` and `toon.Encode`.
- `csv_to_toon`: converts CSV to canonical TOON through `formats.FromCSV` and `toon.Encode`.
- `xml_to_toon`: converts XML to canonical TOON through `formats.FromXML` and `toon.Encode` while preserving DTD rejection.
- `toon_examples`: returns valid TOON examples for common syntax topics.
- `toon_error_explain`: explains stable go-toon error codes with likely causes and fixes.
- `toon_spec_lookup`: returns concise TOON v3.3 reference notes for common syntax and behavior topics.

## Build

```sh
go build ./mcp/go-toon-mcp
```

## Run

The server speaks JSON-RPC over stdio, as expected by MCP clients:

```sh
go run ./mcp/go-toon-mcp
```

## Example Client Configuration

Use the absolute path for this repository on your machine.

```json
{
  "mcpServers": {
    "go-toon": {
      "command": "go",
      "args": ["run", "/absolute/path/to/go-toon/mcp/go-toon-mcp"]
    }
  }
}
```

For lower startup overhead, build the binary and point the client at it:

```json
{
  "mcpServers": {
    "go-toon": {
      "command": "/absolute/path/to/go-toon/go-toon-mcp"
    }
  }
}
```

## Agent Guidance

Agents should use `toon_validate` before returning non-trivial hand-written TOON. For JSON, YAML, CSV, or XML input, use the matching `*_to_toon` tool rather than manually converting large documents. Use `toon_to_json` to inspect structure when debugging parser errors or path expansion behavior. Use `toon_examples`, `toon_error_explain`, and `toon_spec_lookup` when the model is unsure about syntax or needs to explain a repair.
