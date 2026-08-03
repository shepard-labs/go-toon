---
name: toon-format
description: Use this whenever the user asks to write, edit, debug, validate, convert, explain, or generate Token-Oriented Object Notation (TOON). This skill is important because TOON is newer than JSON/YAML and agents may otherwise invent invalid syntax. Use it for TOON examples, compact LLM payloads, JSON-to-TOON conversion, TOON-to-JSON conversion, parser errors, go-toon library usage, and any task that mentions TOON, Token-Oriented Object Notation, tabular arrays, key folding, path expansion, or go-toon.
---

# TOON Format Skill

Use this skill to work with Token-Oriented Object Notation (TOON) safely and accurately.

TOON is its own data format. Do not treat it as YAML, JSON, markdown, or CSV with different punctuation. When the output matters, validate instead of trusting memory.

## Default Workflow

1. Classify the task as generation, conversion, debugging, explanation, or Go API usage.
2. For generation or editing, read `references/syntax.md` if the syntax is non-trivial.
3. Use `references/examples.md` for patterns instead of inventing syntax.
4. Validate non-trivial TOON before finalizing.
5. If validation fails, revise using the exact line, column, and error code.
6. Return concise output. Include explanation only when the user asked for it or it prevents misuse.

## Validation Priority

Use the strongest available validation path:

1. MCP `toon_validate` if available.
2. Local CLI `toon validate` if working in a repo with the `toon` command.
3. Go API `toon.Validate` if writing tests or code.
4. If no validator is available, explicitly say the TOON is unvalidated.

For JSON conversion, prefer MCP `json_to_toon` or the go-toon `formats.FromJSON` path. For YAML, CSV, and XML conversion, prefer MCP `yaml_to_toon`, `csv_to_toon`, and `xml_to_toon` when available. These tools use go-toon's own format normalizers and preserve security defaults such as XML DTD rejection and YAML alias cycle rejection. For inspection, use MCP `toon_to_json` or `toon decode`.

Use MCP `toon_examples` when unsure of syntax shape, `toon_error_explain` when validation fails, and `toon_spec_lookup` for concise TOON v3.3 reference notes.

## Core Rules To Remember

- Objects are ordered `key: value` fields.
- Nested objects use indentation.
- Primitive arrays use headers like `tags[3]: red,green,blue`.
- Arrays of objects can use list form or tabular form.
- Tabular arrays look like `items[2]{sku,qty}:` followed by rows.
- Array counts must match the actual item count.
- Strings can be unquoted only when safe in context.
- Use `null`, `true`, and `false` exactly for those primitive values.
- Dotted keys are literal unless path expansion is explicitly enabled.
- Do not add YAML comments, anchors, aliases, or block scalars unless the TOON spec supports them.

## When Debugging

1. Validate first.
2. Identify whether the problem is indentation, array count, tabular width, quoting, duplicate keys, malformed header, or unsupported syntax.
3. Show the smallest corrected snippet.
4. If the user needs structural confidence, convert the fixed TOON to JSON.

See `references/common-errors.md` for frequent mistakes and fixes.

## When Writing Go Code With go-toon

Use `references/go-toon-api.md` for package entrypoints.

Key guidance:

- Use `*toon.Node` when preserving order matters.
- Use `toon.Encode` and `toon.Decode` for core TOON bytes.
- Use `formats.FromJSON`, `formats.FromYAML`, `formats.FromCSV`, and `formats.FromXML` for input normalization.
- Use `formats.ToJSON` for ordered JSON output.
- Use `toon/reflect` for Go struct convenience APIs.
- Branch on `toon.CodeOf(err)` or `errors.As`, not error string text.

## Output Discipline

When the user asks for TOON only, return TOON only. Do not wrap it in prose. If validation was requested or materially useful, a short note after the block is acceptable.

When the user asks for a conversion, preserve field order from the source when possible.
