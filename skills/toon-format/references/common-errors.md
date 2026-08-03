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

Fix: Replace indentation tabs with spaces. This repo defaults to two spaces.

## `missing_colon`

Cause: An object field or array header is missing `:`.

Invalid:

```toon
name Ada
```

Fixed:

```toon
name: Ada
```

## `malformed_header`

Cause: An array or tabular header is malformed.

Invalid:

```toon
items[]{sku,qty}:
```

Fixed:

```toon
items[0]{sku,qty}:
```

## `path_expansion_conflict`

Cause: Safe path expansion would create a collision between literal and expanded keys.

Fix: Keep dotted keys literal, rename one key, or disable path expansion.
