# Static List Variable Length Plan

## Context

Small Besht sample:

```ts
let files = ["a", "b", "c"]
let count = files.length
console.log(count)
```

Current compact output with `--opt-no-add-binaries-check --opt-no-source-map`:

```sh
files='a
b
c'
count=$(printf '%s\n' "$files" | wc -l | tr -d ' ')
printf '%s\n' "$count"
```

A hand-written shell script would use the known length:

```sh
files='a
b
c'
count=3
printf '%s\n' "$count"
```

The compiler already folds `.length` for inline static list literals, string `.split(...)`, `Array.of(...)`, and `Array.from({ length: N })` bindings. It misses variables bound directly to static list literals.

## Implementation

- Populate `listLenMap` for variables bound to static scalar list literals.
- Preserve the runtime `wc -l` fallback for lists reassigned through control flow, mutated lists, dynamic lists, nested lists, and newline-sensitive values.
- Keep this narrow so existing static factory and split length behavior remains unchanged.

## Verification

- Add codegen coverage for static list-literal variable `.length`.
- Add fallback coverage for control-flow reassigned list variables.
- Add runtime integration coverage for the folded output.
- Update `AGENTS.md`, `README.md`, and `skills/besht-scripting/SKILL.md`.
- Run focused codegen tests and the full gate before committing.
