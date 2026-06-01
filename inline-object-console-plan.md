## Inline object console output

### Probe

Small Besht script:

```ts
console.log({ apple: 3, banana: 2 })
console.error({ name: "Ada", active: true })
```

Current compiler behavior errors before emitting shell:

```text
cannot log object: unable to resolve variable name
```

The README and skill examples document inline object logging, and a direct shell script would naturally
emit a fixed `printf` format:

```sh
printf '{\n  apple: %s,\n  banana: %s,\n}\n' 3 2
printf '{\n  name: %s,\n  active: %s,\n}\n' 'Ada' true >&2
```

### Desired direction

Add a direct console output path for inline object literals. Emit one multi-line `printf` format with one
argument per field, preserving object literal field order and boolean display as `true`/`false`.

### Guardrails

- Keep named/dynamic object printing on the existing paths.
- Validate inline object keys with the same static key validator.
- Use normal expression generation for field values so strings, numbers, command substitutions, and
  simple expressions keep the usual quoting behavior.
- Use boolean display formatting for boolean field values.
- Preserve stderr redirection for `console.error({ ... })`.
- Do not change `Object.keys()`/`Object.values()`/`Object.entries()` behavior in this branch.

### Verification

- Codegen tests for inline object `console.log` and `console.error`.
- Runtime integration test for stdout and stderr output.
- Existing object tests and dynamic-key metadata validation keep passing.
- Update `AGENTS.md`, `README.md`, and `skills/besht-scripting/SKILL.md`.
- `gofmt` changed Go files.
- `git diff --check`
- `make build`
- `make test`
- `go test ./...`
- `bun node-eq/compare $(rg --files -g '*.bsh' node-eq/tests | sort)`
