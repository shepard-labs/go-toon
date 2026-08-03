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

## Empty Arrays

```toon
items: []
```

## Folded Keys

```toon
a.b.c: 1
```

Dotted keys are literal unless a decoder explicitly enables safe path expansion.

## Invalid Patterns To Avoid

```yaml
# YAML comments are not TOON guidance to copy blindly.
items:
  - one
  - two
```

```toon
tags[2]: red,green,blue
```

The array count is wrong.

```toon
items[2]{sku,qty}:
  A1,2,9.99
```

The row has too many columns.
