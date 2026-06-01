# Static String Replace And Concat Plan

## Context

Small Besht sample:

```ts
let s = "hello world"
let a = s.replace("world", "besht")
let b = s.replaceAll("l", "L")
let c = "hello".concat(" world")
console.log(a)
console.log(b)
console.log(c)
```

Current compact output with `--opt-no-add-binaries-check --opt-no-source-map`:

```sh
s='hello world'
a=$(printf '%s' "$s" | sed "s/world/besht/")
b=$(printf '%s' "$s" | sed "s/l/L/g")
c="hello world"
printf '%s\n' "$a"
printf '%s\n' "$b"
printf '%s\n' "$c"
```

A hand-written shell script would use constants:

```sh
s='hello world'
a='hello besht'
b='heLLo worLd'
c='hello world'
printf '%s\n' "$a"
printf '%s\n' "$b"
printf '%s\n' "$c"
```

The compiler already folds many static ASCII string methods and, after the previous merged iteration, can resolve variables bound to static ASCII string literals. `replace`, `replaceAll`, and `concat` still stay on the dynamic path.

## Implementation

- Extend static string transform folding to include `replace`, `replaceAll`, and `concat`.
- Resolve receiver and arguments through the existing static ASCII string expression helper.
- Preserve dynamic fallback for control-flow reassigned variables, non-ASCII strings, and dynamic arguments.
- Keep the static fold JS-like for string search replacement, including literal metacharacters, instead of invoking sed regex semantics.

## Verification

- Add codegen coverage for inline static literals and variables bound to static string literals.
- Add special-character coverage proving static replacement is literal.
- Add fallback coverage for control-flow reassigned receivers.
- Add runtime integration coverage for folded output.
- Update `AGENTS.md`, `README.md`, and `skills/besht-scripting/SKILL.md`.
- Run focused codegen tests and the full gate before committing.
