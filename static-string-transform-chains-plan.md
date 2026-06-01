## Compact static string transform chains

### Probe

Small Besht script:

```ts
let name = "  Ada  "
console.log(name.trim())
let upper = name.trim().toUpperCase()
console.log(upper)
```

Current compiled output folds the first transform but not the chained transform:

```sh
name='  Ada  '
printf '%s\n' 'Ada'
upper=$(printf '%s' 'Ada' | tr '[:lower:]' '[:upper:]')
printf '%s\n' "$upper"
```

A direct shell script would just use the final literal:

```sh
name='  Ada  '
printf '%s\n' 'Ada'
upper='ADA'
printf '%s\n' "$upper"
```

### Desired direction

Fold chained static ASCII string transforms when every receiver and argument is static, so calls like `name.trim().toUpperCase()` and `" x ".trim().padEnd(3, ".")` compile to literals.

### Guardrails

- Keep non-ASCII transform chains on the existing POSIX tool path.
- Keep dynamic receivers or arguments on the existing runtime path.
- Preserve current argument validation errors for transform methods.
- Do not alter standalone string search lowering; chained static transform receivers may feed the existing static search folding path.
- Update docs and tests in the same branch.

### Verification

- Codegen tests for chained static transforms over literals and variables bound to static strings.
- Fallback tests for dynamic transform arguments and non-ASCII receivers.
- Runtime integration test for a folded chain.
- Update `AGENTS.md`, `README.md`, and `skills/besht-scripting/SKILL.md`.
- `gofmt` changed Go files.
- `git diff --check`
- `make build`
- `make test`
- `go test ./...`
- `bun node-eq/compare $(rg --files -g '*.bsh' node-eq/tests | sort)`
