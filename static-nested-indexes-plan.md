## Static nested indexes

### Probe

Small Besht script:

```ts
console.log(Object.entries({ name: "Ada", active: true })[0][0])
console.log(Object.entries({ name: "Ada", active: true })[0][1])
let entries = Object.entries({ name: "Ada", active: true })
console.log(entries[1][0])
console.log(entries[1][1])
```

Current compiled output keeps nested runtime list indexing even though all rows are static:

```sh
printf '%s\n' "$(printf '%s\n' "$(printf '%s\n' 'nameAda
activetrue' | sed -n "$(( 0 + 1 ))p" | tr '\037' '\n')" | sed -n "$(( 0 + 1 ))p")"
```

A direct shell script would naturally print constants:

```sh
printf '%s\n' 'name'
printf '%s\n' 'Ada'
entries='nameAda
activetrue'
printf '%s\n' 'active'
printf '%s\n' 'true'
```

### Desired direction

Fold static nested list indexes when both indexes are known non-negative integers:

- `Object.entries(staticObject)[row][col]`
- variables bound to static object entries, while they have not been assigned in control flow
- static nested list literals such as `[["a", "b"], ["c", "d"]][1][0]`

Keep the current `sed`/`tr` fallback for dynamic rows, dynamic columns, out-of-range indexes,
newline-sensitive values, and variables that may have been reassigned inside control flow.

### Implementation notes

- Reuse the unit-separator row representation already used by static object entries and nested lists.
- Add a static nested-index helper before the current list-index fallback in `genIndexExpr`.
- Update `AGENTS.md`, `README.md`, and `skills/besht-scripting/SKILL.md`.

### Verification

- Codegen tests for inline static object entries, named static entries, nested list literals, and dynamic fallback.
- Runtime integration test for the folded outputs.
- `gofmt` changed Go files.
- `git diff --check`
- `make build`
- `make test`
- `go test ./...`
- `bun node-eq/compare $(rg --files -g '*.bsh' node-eq/tests | sort)`
