# Static Boolean ToString Bindings Plan

## Context

Small Besht sample:

```ts
let a = true.toString()
let b = false.toString()
let c = ("x" === "x").toString()
let d = Boolean("").toString()
console.log(a)
console.log(b)
console.log(c)
console.log(d)
```

Current compact output with `--opt-no-add-binaries-check --opt-no-source-map`:

```sh
a=$(if [ 1 = 1 ]; then printf true; else printf false; fi)
b=$(if [ 0 = 1 ]; then printf true; else printf false; fi)
c=$(if [ 1 = 1 ]; then printf true; else printf false; fi)
d=$(if [ 0 = 1 ]; then printf true; else printf false; fi)
printf '%s\n' "$a"
printf '%s\n' "$b"
printf '%s\n' "$c"
printf '%s\n' "$d"
```

A hand-written shell script would use constants:

```sh
a='true'
b='false'
c='true'
d='false'
printf '%s\n' "$a"
printf '%s\n' "$b"
printf '%s\n' "$c"
printf '%s\n' "$d"
```

The compiler already folds static numeric `.toString()` assignments and static primitive `.toString()` fragments inside string concatenation/template interpolation. Direct static boolean `.toString()` assignments still take the dynamic boolean formatting path.

## Implementation

- Reuse the existing static string-fragment helper when generating method-call expression values.
- Fold static primitive `.toString()` calls to quoted constants for direct assignments and other plain expression contexts.
- Preserve the current dynamic fallback for variable receivers, status values, and non-static expressions.

## Verification

- Add codegen coverage for direct static boolean/comparison/builtin `.toString()` bindings.
- Add runtime integration coverage for the folded outputs.
- Update `AGENTS.md`, `README.md`, and `skills/besht-scripting/SKILL.md`.
- Run focused codegen tests and the full gate before committing.
