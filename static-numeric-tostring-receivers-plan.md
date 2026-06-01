# Static Numeric ToString Receivers Plan

## Context

Small Besht sample:

```ts
let rounded = Math.round(2.7).toString()
let parsed = Number.parseInt("42").toString()
let fixed = Math.max(4, 2).toString()
console.log(rounded)
console.log(parsed)
console.log(fixed)
```

Current compact output with `--opt-no-add-binaries-check --opt-no-source-map`:

```sh
rounded=$(printf '%s' 3)
parsed=$(printf '%s' 42)
fixed=$(printf '%s' 4)
printf '%s\n' "$rounded"
printf '%s\n' "$parsed"
printf '%s\n' "$fixed"
```

A hand-written shell script would use constants:

```sh
rounded='3'
parsed='42'
fixed='4'
printf '%s\n' "$rounded"
printf '%s\n' "$parsed"
printf '%s\n' "$fixed"
```

The compiler already folds direct static numeric literal `.toString()` and static numeric API calls by themselves. It misses the same constant when a static numeric API call is the receiver of `.toString()`.

## Implementation

- Add a helper that recognizes numeric expressions that already have static codegen, including static arithmetic, `Number.parseInt(...)`, and literal-argument `Math.*` calls.
- Use that helper for zero-argument `.toString()` receiver folding.
- Preserve dynamic fallback for non-static numeric receivers and unsupported parse/math calls.

## Verification

- Add codegen coverage for static `Math.*` and `Number.parseInt()` receivers of `.toString()`.
- Add fallback coverage for dynamic parse/math receivers.
- Add runtime integration coverage for the folded outputs.
- Update `AGENTS.md`, `README.md`, and `skills/besht-scripting/SKILL.md`.
- Run focused codegen tests and the full gate before committing.
