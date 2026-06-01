## Compact static range loops

### Probe

Small Besht script:

```ts
for (i of Besht.iter.range(1, 3)) {
    console.log(i)
}
```

Current compiled output uses a counter `while` loop:

```sh
i=1
while [ "$i" -le 3 ]; do
    printf '%s\n' "$i"
    i=$(( i + 1 ))
done
```

A direct shell script for static bounds would naturally be:

```sh
for i in 1 2 3; do
    printf '%s\n' "$i"
done
```

### Desired direction

When `Besht.iter.range(start, end)` has static integer bounds and the range is small enough to inline
reasonably, compile it to a compact static `for` loop.

### Guardrails

- Preserve inclusive range semantics.
- Keep dynamic bounds on the existing counter `while` loop.
- Keep very large static ranges on the existing loop to avoid huge generated shell.
- Preserve existing loop variable scoping/mangling.
- Do not change list iteration behavior in this branch.

### Verification

- Codegen tests for literal static ranges and static arithmetic bounds.
- Fallback tests for dynamic range bounds and large static ranges.
- Runtime integration test for static range output.
- Update `AGENTS.md`, `README.md`, and `skills/besht-scripting/SKILL.md`.
- `gofmt` changed Go files.
- `git diff --check`
- `make build`
- `make test`
- `go test ./...`
- `bun node-eq/compare $(rg --files -g '*.bsh' node-eq/tests | sort)`
