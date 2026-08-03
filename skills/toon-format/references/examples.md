# TOON Examples

## Flat Object

```toon
id: 1
name: Ada
active: true
```

## Nested Object

```toon
user:
  id: 1
  name: Ada
  active: true
profile:
  role: engineer
  timezone: UTC
```

## Primitive Array

```toon
tags[3]: go,toon,ordered
```

## List Array

```toon
users[2]:
  - id: 1
    name: Ada
  - id: 2
    name: Linus
```

## Tabular Array

```toon
items[2]{sku,qty,price}:
  A1,2,9.99
  B2,1,14.5
```

## Quoted Strings

```toon
title: "Hello, world"
literal_bool: "true"
literal_null: "null"
path: "a:b"
```

## JSON To TOON

JSON:

```json
{"user":{"id":1,"name":"Ada"},"tags":["go","toon"]}
```

TOON:

```toon
user:
  id: 1
  name: Ada
tags[2]: go,toon
```
