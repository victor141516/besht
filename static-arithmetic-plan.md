# Static Arithmetic Compactness Plan

## Context

The current compiler preserves shell arithmetic even when the whole numeric expression is static:

```ts
console.log(2 + 3)
console.log(10 - 4)
console.log(6 * 7)
```

Current output:

```sh
printf '%s\n' "$(( 2 + 3 ))"
printf '%s\n' "$(( 10 - 4 ))"
printf '%s\n' "$(( 6 * 7 ))"
```

A shell author would normally write:

```sh
printf '%s\n' 5
printf '%s\n' 6
printf '%s\n' 42
```

## Change

Fold static numeric arithmetic expressions in value position:

- integer and float literals
- unary `-`
- binary `+`, `-`, `*`, `/`, and `%` when both sides are static numbers

Keep behavior aligned with the compiler's current numeric model:

- Use integer formatting for integer results.
- Use floating formatting for non-integer results.
- Do not fold division or modulo by zero.
- Leave dynamic expressions, update expressions, and command/function results on the existing shell/awk paths.

## Verification

- Add codegen tests for static arithmetic folding and dynamic fallback.
- Add runtime integration coverage.
- Update `AGENTS.md`, `README.md`, and `skills/besht-scripting/SKILL.md`.
- Run `go test ./internal/codegen`, `go test ./...`, `make build`, `make test`, `bun node-eq/compare $(rg --files -g '*.bsh' node-eq/tests | sort)`, and `git diff --check`.
