# Static List Index Compactness Plan

## Context

The current compiler keeps the general newline-safe list indexing path even when both the list contents and index are static:

```ts
let items = ["a", "b"]
console.log(items[0])
```

Current output includes:

```sh
items='a
b'
printf '%s\n' "$(printf '%s\n' "$items" | sed -n "$(( 0 + 1 ))p")"
```

A shell author would normally write:

```sh
items='a
b'
printf '%s\n' 'a'
```

## Change

Fold in-range static scalar list indexing:

- `["a", "b"][0]` emits `'a'`
- `let items = ["a", "b"]; items[1]` emits `'b'`
- Static `Array.of(...)`, `Array.from({ length: N })`, and static string `.split(...)` values should benefit through the existing static-list machinery when available

Keep the change conservative:

- Only fold static integer indexes
- Only fold known in-range values
- Only fold scalar values without embedded newlines
- Leave dynamic indexes, unknown lists, nested values, and out-of-range behavior on the existing runtime path

## Verification

- Add codegen tests proving static list literal and static list variable indexes skip `sed`.
- Add integration coverage proving runtime output remains correct.
- Update `AGENTS.md`, `README.md`, and `skills/besht-scripting/SKILL.md`.
- Run `go test ./internal/codegen`, `go test ./...`, `make build`, `make test`, `bun node-eq/compare $(rg --files -g '*.bsh' node-eq/tests | sort)`, and `git diff --check`.
