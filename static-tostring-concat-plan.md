# Static ToString Concatenation Plan

## Context

Small Besht sample:

```ts
let a = "count=" + (2 + 3).toString()
let b = "flag=" + true.toString()
console.log(a)
console.log(b)
```

Current compact output with `--opt-no-add-binaries-check --opt-no-source-map`:

```sh
a="count=$(printf '%s' 5)"
b="flag=$(if [ 1 = 1 ]; then printf true; else printf false; fi)"
printf '%s\n' "$a"
printf '%s\n' "$b"
```

A hand-written shell script would use constants:

```sh
a='count=5'
b='flag=true'
printf '%s\n' "$a"
printf '%s\n' "$b"
```

The compiler already folds direct static number method assignments such as `(42).toString()`, and static boolean console arguments. It misses the same constant value when `.toString()` appears as a subexpression in string concatenation or template interpolation.

## Implementation

- Add a static string-fragment helper for expressions that can be rendered as compile-time string text.
- Recognize static primitive `.toString()` calls for numbers, booleans, strings, and static arithmetic/comparison expressions.
- Reuse the helper in string concatenation and template interpolation so static fragments do not emit command substitutions.
- Preserve dynamic fallback for non-static receiver expressions and runtime booleans.

## Verification

- Add codegen coverage for static primitive `.toString()` inside concatenation and template literals.
- Add fallback coverage for dynamic primitive `.toString()` calls.
- Add runtime integration coverage for the folded outputs.
- Update `AGENTS.md`, `README.md`, and `skills/besht-scripting/SKILL.md`.
- Run focused codegen tests and the full gate before committing.
