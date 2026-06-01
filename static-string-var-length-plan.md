# Static String Variable Length Compactness Plan

## Context

The current compiler folds direct literal lengths, but not variables bound to static string literals:

```ts
let text = "abc"
console.log(text.length)
```

Current output:

```sh
text='abc'
printf '%s\n' "$(printf '%s' "$text" | wc -c | tr -d ' ')"
```

A shell author would normally write:

```sh
text='abc'
printf '%s\n' 3
```

## Change

Fold `.length` on variables that are known to hold static string literals.

Keep the change conservative:

- Fold direct identifiers only when `stringConstMap` has a known value.
- Do not fold variables assigned inside control flow, because later loop iterations or branch-dependent assignments can make the initial value stale.
- Keep dynamic strings and unknown variables on the existing POSIX `wc -c` path.

## Verification

- Add codegen tests for static string variable `.length` and control-flow assignment fallback.
- Add runtime integration coverage.
- Update `AGENTS.md`, `README.md`, and `skills/besht-scripting/SKILL.md`.
- Run `go test ./internal/codegen`, `go test ./...`, `make build`, `make test`, `bun node-eq/compare $(rg --files -g '*.bsh' node-eq/tests | sort)`, and `git diff --check`.
