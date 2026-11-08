# Common TOON Errors

Use structured parser errors when available. These notes help explain and fix the most common cases.

## `array_count_mismatch`

Cause: The array header count does not match the number of items.

Invalid:

```toon
tags[2]: red,green,blue
```

Fixed:

```toon
tags[3]: red,green,blue
```

## `tabular_width_mismatch`

Cause: A tabular row has a different number of columns than the header.

Invalid:

```toon
items[1]{sku,qty}:
  A1,2,9.99
```

Fixed:

```toon
items[1]{sku,qty,price}:
  A1,2,9.99
```

## `duplicate_key`

Cause: An object contains duplicate keys in strict mode or JSON input contains duplicate object keys.

Fix: Rename, remove, or intentionally merge the duplicate field. Do not silently discard one unless the user requested that behavior.

## `tab_indent`

Cause: The document uses tab indentation where spaces are required.

Fix: Replace indentation tabs with spaces. This repo defaults to two spaces. A `#` line indented with a tab is not a comment.

## `missing_colon`

Cause: An object field or array header is missing `:`.

Invalid:

```toon
id: 1
name Ada
```

Fixed:

```toon
id: 1
name: Ada
```

A document that is a single line with no colon, such as `name Ada`, is a valid root string, not an error.

## `malformed_header`

Cause: An array or tabular header is malformed. Examples: an empty length `[]`, a leading-zero length `[03]`, an empty field list `{}`, an empty or nameless field entry (`{a,}`, `{a,{b}}`), or unmatched braces.

Invalid:

```toon
items[]{sku,qty}:
```

Fixed:

```toon
items[0]{sku,qty}:
```

## `invalid_indent`

Cause: A line is indented by an amount that is not a multiple of the indent size, or is indented deeper than its parent allows.

Invalid:

```toon
a: 1
   b: 2
```

Fixed:

```toon
a:
  b: 2
```

## `invalid_utf8`

Cause: The input contains bytes that are not valid UTF-8. The decoder rejects them in strict and non-strict mode. Encoding a string value or object key with ill-formed UTF-8 (invalid bytes or unpaired surrogates) returns the same error.

Fix: Re-encode the source as UTF-8 and remove binary data from text values.

## Dotted keys

There is no path expansion error. `a.b: 1` is one key named `a.b`, and `path_expansion_conflict` is no longer produced.
