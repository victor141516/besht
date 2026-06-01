## Compact integer comparisons

### Probe

Small Besht script:

```ts
let n = 2
if (n < 3) console.log("small")
while (n > 0) n--
```

Current compiled condition output:

```sh
if awk -v _a=$n -v _b=3 'BEGIN{OFMT="%.17g";exit !(_a < _b)}'; then
    printf '%s\n' 'small'
fi
while awk -v _a=$n -v _b=0 'BEGIN{OFMT="%.17g";exit !(_a > _b)}'; do
    n=$(( $n - 1 ))
done
```

A direct shell script for integer-shaped values would naturally use:

```sh
if [ "$n" -lt 3 ]; then
    printf '%s\n' 'small'
fi
while [ "$n" -gt 0 ]; do
    n=$(( n - 1 ))
done
```

### Desired direction

When both sides of a relational comparison are compiler-known integer expressions, compile to POSIX integer tests (`-lt`, `-gt`, `-le`, `-ge`). Keep the existing `awk` path for floats, unknown numeric values, function results, command output, and any expression that is not safely known to be integer-shaped.

### Guardrails

- Do not use type annotations as proof of integer-ness; annotations are ignored.
- Track integer-ness from actual bindings and assignments.
- Preserve float behavior and hostile/non-numeric runtime safety by keeping `awk` unless both operands are known integer expressions.
- Keep static comparisons folded to constants before reaching this dynamic path.
- Update docs and tests in the same branch.

### Verification

- Codegen tests for `if` and `while` integer comparisons using `[ ]`.
- Codegen fallback tests for known float-producing expressions.
- Runtime integration test for an integer loop.
- Update `AGENTS.md`, `README.md`, and `skills/besht-scripting/SKILL.md`.
- `gofmt` changed Go files.
- `git diff --check`
- `make build`
- `make test`
- `go test ./...`
- `bun node-eq/compare $(rg --files -g '*.bsh' node-eq/tests | sort)`
