# TOON Syntax Reference

This is a compact guide for agents. Prefer validation over relying on this document alone.

## Objects

```toon
id: 1
name: Ada
active: true
```

Fields are ordered. Keep their order unless the user asks to reorder them.

Nested objects use indentation:

```toon
user:
  id: 1
  name: Ada
```

## Primitives

```toon
empty: null
ok: true
count: 42
price: 9.99
name: Ada
```

TOON uses lowercase `null`, `true`, and `false`.

## Strings

Unquoted strings are allowed only when safe. Quote strings that contain delimiters, escapes, ambiguous primitive text, or whitespace-sensitive content.

```toon
name: Ada
message: "hello, world"
literal_true: "true"
literal_null: "null"
```

When unsure, quote and validate.

## Primitive Arrays

```toon
tags[3]: red,green,blue
```

The count inside brackets must match the number of values.

## List Arrays

```toon
users[2]:
  - id: 1
    name: Ada
  - id: 2
    name: Linus
```

Use list arrays for complex or irregular objects.

## Tabular Arrays

```toon
items[2]{sku,qty,price}:
  A1,2,9.99
  B2,1,14.5
```

Use tabular arrays when every row has the same primitive fields in the same order. The row width must match the header field count.

## Nested Field Groups

When a column holds uniform nested objects, the header lists them as a group. Rows stay flat; cells follow the header depth-first.

```toon
users[2]{id,name,geo{lat,lng}}:
  1,Ada,1.5,2.5
  2,Linus,3,4
```

Groups nest: `rows[2]{id,a{b{c,d}}}:`. A group needs a name, and field lists cannot be empty.

## Keyed Tabular Arrays

An object of two or more entries whose values are non-empty objects with the same keys uses `[N:]`. Each row starts with the entry key, then a colon, then the cells.

```toon
users[2:]{age,city}:
  alice: 30,Oslo
  bob: 25,Rome
```

At the document root the header has no key:

```toon
[2:]{age,city}:
  alice: 30,Oslo
  bob: 25,Rome
```

## List Items

The first field of an object item sits on the hyphen line; later fields align under it. An empty object is a bare `-`. A nested array item is `- [M]:`.

```toon
items[4]:
  - id: 1
    tags[2]: a,b
  - id: 2
  - [2]: 1,2
  - [0]:
```

## Delimiters

The header declares the delimiter when it is not a comma: `items[2|]{sku|qty}:` for pipe, and a tab character inside the brackets and field list for tab. Rows use the same delimiter.

```toon
items[2|]{sku|qty}:
  A1|2
  B2|1
```

## Empty Arrays

```toon
items: []
```

## Dotted Keys

```toon
a.b.c: 1
```

Dotted keys are always literal: this is one key named `a.b.c`. There is no folding or path expansion.

## Comments

A line whose first non-space character is `#` is a comment. Decoders drop comment lines. Encoders never write them, and quote strings that start with `#`.

```toon
# users
name: Ada
# trailing note
age: 36
```

Only spaces may precede the `#`. A tab-indented `#` line is not a comment, and strict mode rejects it as tab indentation.

## Escapes

Quoted strings accept `\\`, `\"`, `\n`, `\r`, `\t`, and `\uXXXX`. The encoder writes other control characters as `\u00xx` with lowercase hex. `\/`, `\b`, `\f`, and surrogate escapes are rejected.

## Numbers

Numbers are plain decimals or exponents (`1e-7`, `1e+21`); `+1`, `NaN`, and `007` decode as strings. The decoder keeps the token text losslessly, including out-of-range values like `1e500`.

## Text Encoding

Documents must be valid UTF-8. A leading byte-order mark is stripped. Only space and tab are whitespace.

## Invalid Patterns To Avoid

```yaml
items:
  - one
  - two
```

YAML list syntax is not a TOON array.

```toon
tags[2]: red,green,blue
```

The array count is wrong.

```toon
items[2]{sku,qty}:
  A1,2,9.99
```

The row has too many columns.
