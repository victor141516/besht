## Compact static forEach loops

### Probe

Small Besht script:

```ts
let names = ["a", "b"]
names.forEach(x => console.log(x))
```

Current compiled output stores the list in a temp and uses a heredoc-backed `while` loop:

```sh
names='a
b'
_foreach_2_6="$names"
if [ -n "$_foreach_2_6" ]; then
    while IFS= read -r _cb_2_15_x; do
        printf '%s\n' "$_cb_2_15_x"
    done <<__BESHT_FOREACH_2_6
$_foreach_2_6
__BESHT_FOREACH_2_6
fi
```

A direct shell script for static scalar elements would naturally use:

```sh
names='a
b'
for _cb_2_15_x in 'a' 'b'; do
    printf '%s\n' "$_cb_2_15_x"
done
```

### Desired direction

When a statement-position `.forEach()` receiver is a static scalar list with no newline elements, compile the callback loop to a compact POSIX `for ... in ...; do` loop.

### Guardrails

- Keep dynamic, nested, and newline-sensitive receivers on the existing heredoc loop.
- Preserve current-shell side effects for assignments and `Set.add()`.
- Preserve optional index parameter behavior.
- Preserve callback validation errors for `return`, `break`, `continue`, and pure value expressions.
- Update docs and tests in the same branch.

### Verification

- Codegen tests for static scalar `.forEach()` without and with index.
- Fallback tests for dynamic/newline-sensitive receivers.
- Runtime integration test showing callback side effects still persist.
- Update `AGENTS.md`, `README.md`, and `skills/besht-scripting/SKILL.md`.
- `gofmt` changed Go files.
- `git diff --check`
- `make build`
- `make test`
- `go test ./...`
- `bun node-eq/compare $(rg --files -g '*.bsh' node-eq/tests | sort)`
