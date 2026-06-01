## Static folded comparisons

### Probe

Small Besht scripts:

```ts
console.log(Math.min(4, 2) === 2)
console.log((2 + 3) === 5)
console.log("hello".charAt(99) === "")
console.log("hi".toUpperCase() === "HI")
console.log("  hi  ".trim() === "hi")
```

Current compiled output folds the left and right expressions individually, but still emits a
nested runtime comparison and boolean formatter:

```sh
printf '%s\n' "$(if [ $(if { _bst_left=2; _bst_right=2; ... }; then printf 1; else printf 0; fi) = 1 ]; then printf true; else printf false; fi)"
```

A direct shell script would naturally print constants:

```sh
printf '%s\n' true
printf '%s\n' true
printf '%s\n' true
printf '%s\n' true
printf '%s\n' true
```

### Desired direction

Extend static comparison folding so equality comparisons can use already-foldable scalar
expressions on either side, not only direct literals:

- static arithmetic expressions
- static `Math.*` and `Number.parseInt`/`parseFloat` results
- static string method results such as `charAt()`, `trim()`, and `toUpperCase()`
- static boolean-producing expressions that already fold to `true`/`false`

Keep the existing runtime comparison path for dynamic expressions, variables that may be assigned
inside control flow, non-static method calls, and relational comparisons that cannot be reduced to
numbers safely.

### Implementation notes

- Add a generator-aware static comparison helper that can call existing static folding helpers.
- Prefer the generator-aware helper from codegen paths that already have a `Generator`.
- Leave the existing free `staticComparisonResult` available for parser-independent or low-level
  literal checks if that keeps the patch small.
- Update `AGENTS.md`, `README.md`, and `skills/besht-scripting/SKILL.md`.

### Verification

- Codegen tests for folded comparisons over static arithmetic, string methods, and math/number APIs.
- Integration test proving runtime output matches JS for folded true/false cases.
- `gofmt` changed Go files.
- `git diff --check`
- `make build`
- `make test`
- `go test ./...`
- `bun node-eq/compare $(rg --files -g '*.bsh' node-eq/tests | sort)`
