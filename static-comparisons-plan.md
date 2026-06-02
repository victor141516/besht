# Static Comparison Compactness Plan

## Context

The current compiler emits the full dynamic equality block even when both comparison sides are scalar literals:

```ts
let same = "a" === "a"
if ("a" === "a") console.log("same")
```

Current output includes:

```sh
same=$(if { _bst_left='a'; _bst_right='a'; ... [ "$_bst_left" = "$_bst_right" ]; }; then printf 1; else printf 0; fi)
if { _bst_left='a'; _bst_right='a'; ... [ "$_bst_left" = "$_bst_right" ]; }; then
    printf '%s\n' 'same'
fi
```

A shell author would normally write:

```sh
same=1
if true; then
    printf '%s\n' 'same'
fi
```

## Change

Fold static scalar comparisons:

- `==`, `!=`, `===`, and `!==` for static scalar literal values
- numeric `<`, `<=`, `>`, and `>=` for static numeric literals

Keep the change conservative:

- Only fold strings, no-expression template strings, numbers, booleans, `null`, `undefined`, and erased `as` wrappers.
- Preserve current Besht semantics where strict equality is the same as loose equality.
- Leave dynamic expressions, function calls, command outputs, lists, objects, and sets on the existing comparison path.

## Verification

- Add codegen tests for folded equality and numeric comparisons plus dynamic fallback.
- Add runtime integration coverage.
- Update `AGENTS.md`, `README.md`, and `skills/besht-scripting/SKILL.md`.
- Run `go test ./internal/codegen`, `go test ./...`, `make build`, `make test`, `bun node-eq/compare $(rg --files -g '*.bsh' node-eq/tests | sort)`, and `git diff --check`.
