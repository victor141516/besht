## Compact static Set membership

### Probe

Small Besht script:

```ts
let seen = new Set<string>()
seen.add("a")
seen.add("b")
console.log(seen.has("a"))
```

Current compiled output runs de-dup pipelines for each static add and a `grep` membership test:

```sh
seen=""
seen=$( { if [ -n "$seen" ]; then printf '%s
' "$seen"; fi; printf '%s
' 'a'; } | awk '!seen[$0]++')
seen=$( { if [ -n "$seen" ]; then printf '%s
' "$seen"; fi; printf '%s
' 'b'; } | awk '!seen[$0]++')
printf '%s\n' "$(if [ $([ -n "$seen" ] && printf '%s\n' "$seen" | grep -qxF -- 'a' && printf 1 || printf 0) = 1 ]; then printf true; else printf false; fi)"
```

A direct shell script for known values would be much simpler:

```sh
seen='a
b'
printf '%s\n' true
```

### Desired direction

Track simple static Set contents when a Set is created empty and then receives literal/static scalar `.add(value)` statements. Use those known contents to compile static `.has(value)` to `1` or `0`, which also lets `console.log()` print direct `true`/`false`.

### Guardrails

- Keep dynamic `.add()` values on the existing de-dup pipeline path.
- Invalidate static Set contents if the Set is reassigned or receives a dynamic add.
- Preserve runtime Set representation as newline-delimited unique values.
- Preserve current behavior for non-Set lists and dynamic membership checks.
- Update docs and tests in the same branch.

### Verification

- Codegen tests for static `Set.add()` statements and folded static `Set.has()`.
- Fallback tests for dynamic `Set.add()` invalidation.
- Runtime integration test for static adds plus membership output.
- Update `AGENTS.md`, `README.md`, and `skills/besht-scripting/SKILL.md`.
- `gofmt` changed Go files.
- `git diff --check`
- `make build`
- `make test`
- `go test ./...`
- `bun node-eq/compare $(rg --files -g '*.bsh' node-eq/tests | sort)`
