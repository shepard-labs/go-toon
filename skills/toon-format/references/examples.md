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

## Nested Field Groups

```toon
users[2]{id,name,geo{lat,lng}}:
  1,Ada,1.5,2.5
  2,Linus,3,4
```

## Keyed Tabular Array

```toon
users[2:]{age,city}:
  alice: 30,Oslo
  bob: 25,Rome
```

## Keyed Tabular Root

```toon
[2:]{age,city}:
  alice: 30,Oslo
  bob: 25,Rome
```

## List Items With Nested Arrays

```toon
items[4]:
  - id: 1
    tags[2]: a,b
  - id: 2
  - [2]: 1,2
  - [0]:
```

## Pipe Delimiter

```toon
items[2|]{sku|qty}:
  A1|2
  B2|1
```

## Comments

```toon
# users
name: Ada
# trailing note
age: 36
```

## Dotted Key

```toon
a.b: 1
```

This is one literal key, `a.b`.

## Quoted Strings

```toon
title: "Hello, world"
literal_bool: "true"
literal_null: "null"
path: "a:b"
note: "#hash"
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
