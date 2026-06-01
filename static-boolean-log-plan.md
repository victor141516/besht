# Static Boolean Log Compactness Plan

## Context

The current compiler emits a command substitution and shell `if` just to print a statically known boolean:

```ts
console.log(Boolean(""))
console.log(Boolean("x"))
```

Current output:

```sh
printf '%s\n' "$(if [ 0 = 1 ]; then printf true; else printf false; fi)"
printf '%s\n' "$(if [ 1 = 1 ]; then printf true; else printf false; fi)"
```

A shell author would normally write:

```sh
printf '%s\n' false
printf '%s\n' true
```

## Change

Teach `console.log()` and `console.error()` value formatting to recognize statically known boolean expressions and emit `true` or `false` directly.

Keep the change conservative:

- Fold boolean literals and static `Boolean(value)` calls.
- Fold simple static boolean expressions built from `!`, `&&`, `||`, and erased `as` wrappers.
- Leave variables and dynamic comparisons on the existing boolean formatting path.

## Verification

- Add codegen tests proving static boolean logs avoid `if [ ... ]`.
- Add runtime integration coverage for stdout and stderr formatting.
- Update `AGENTS.md`, `README.md`, and `skills/besht-scripting/SKILL.md`.
- Run `go test ./internal/codegen`, `go test ./...`, `make build`, `make test`, `bun node-eq/compare $(rg --files -g '*.bsh' node-eq/tests | sort)`, and `git diff --check`.
