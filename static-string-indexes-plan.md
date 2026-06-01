## Static string indexes

### Probe

Small Besht script:

```ts
console.log("abc"[1])
let s = "abc"
console.log(s[2])
```

Current compiled output:

```sh
printf '%s\n' "$(printf '%s' 'abc' | cut -c$(( 1 + 1 )))"
s='abc'
printf '%s\n' "$(printf '%s' "$s" | cut -c$(( 2 + 1 )))"
```

A direct shell script would naturally use constants for these known values:

```sh
printf '%s\n' 'b'
s='abc'
printf '%s\n' 'c'
```

### Desired direction

Fold static ASCII string indexes with known non-negative integer indexes:

- string literals: `"abc"[1]` -> `'b'`
- variables bound to static ASCII strings: `s[2]` -> `'c'`
- out-of-range static indexes should compile to `''`, matching the existing `cut` path's current empty output

Keep the existing `cut` path for dynamic strings, dynamic indexes, non-ASCII strings, and other unknown cases.

### Implementation notes

- Reuse the existing `stringConstMap` for variables bound to string literals.
- Add a small static string index helper near the current list-index helper.
- Run the static string index helper before the current string `cut` branch in `genIndexExpr`.
- Update `AGENTS.md`, `README.md`, and `skills/besht-scripting/SKILL.md` to mention static string indexes.

### Verification

- Codegen tests for direct string literal indexes, variables bound to string literals, and out-of-range indexes.
- Runtime integration test for the same values.
- `gofmt` changed Go files.
- `git diff --check`
- `make build`
- `make test`
- `go test ./...`
- `bun node-eq/compare $(rg --files -g '*.bsh' node-eq/tests | sort)`
