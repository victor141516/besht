# Static Destructure Plan

## Context

Small Besht sample:

```ts
const [name, age] = ["Ada", 37]
console.log(name)
console.log(age)
```

Current compact output with `--opt-no-add-binaries-check --opt-no-source-map`:

```sh
_destructure_1_1='Ada
37'
name=$(printf '%s\n' "$_destructure_1_1" | sed -n '1p')
age=$(printf '%s\n' "$_destructure_1_1" | sed -n '2p')
printf '%s\n' "$name"
printf '%s\n' "$age"
```

A hand-written shell script would assign the values directly:

```sh
name='Ada'
age='37'
printf '%s\n' "$name"
printf '%s\n' "$age"
```

The same gap appears when destructuring a variable bound to a static scalar list:

```ts
let pair = ["Ada", "Lovelace"]
const [first, last] = pair
```

The compiler already tracks static list words for compact `for` loops and static list indexes. Destructuring can reuse that information for scalar, newline-free list values.

## Implementation

- Detect static scalar destructuring sources before the existing temp-and-`sed` path.
- Support inline static scalar list expressions and identifiers bound to static scalar lists.
- Emit direct assignments for each destructured name, using `''` for missing static positions to match current out-of-range extraction behavior.
- Preserve the existing temp-and-`sed` path for dynamic, nested, spread, newline-sensitive, or control-flow reassigned sources.

## Verification

- Add codegen coverage for inline static list destructuring and named static list destructuring.
- Add fallback coverage for control-flow reassigned list variables.
- Add runtime integration coverage for folded destructuring.
- Update `AGENTS.md`, `README.md`, and `skills/besht-scripting/SKILL.md`.
- Run focused codegen tests and the full gate before committing.
