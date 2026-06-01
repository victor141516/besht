## Compact simple runtime checks

### Probe

Small Besht script:

```ts
console.log("hello")
```

Current compiled output starts with the runtime utility self-check:

```sh
_r=$(printf 'hello:world' | grep -F 'hello' | sed 's/hello/goodbye/' 2>/dev/null) || {
  printf '[besht] FATAL: required utilities (printf/grep/sed) are not working correctly\n' >&2
  exit 1
}
```

The meaningful script body is just:

```sh
printf '%s\n' 'hello'
```

A direct shell script for this case would not preflight `grep` or `sed`.

### Desired direction

Keep the runtime check for generated shell that uses the compiler's `grep`/`sed`-based paths or the args runtime, but skip it for scripts whose generated body only needs shell builtins and direct `printf` output.

### Guardrails

- Preserve `--opt-no-add-binaries-check` behavior.
- Keep the check before compiled code when it is emitted.
- Keep split-mode entry scripts consistent with bundled output.
- Do not remove runtime helpers themselves.
- Treat this as output elision only; script behavior must not change.

### Verification

- Integration test: simple `console.log("hello")` omits the check.
- Integration test: a generated `sed` path still emits the check before user code.
- Integration test: `--opt-no-add-binaries-check` still omits the check.
- Update `AGENTS.md`, `README.md`, and `skills/besht-scripting/SKILL.md`.
- `gofmt` changed Go files.
- `git diff --check`
- `make build`
- `make test`
- `go test ./...`
- `bun node-eq/compare $(rg --files -g '*.bsh' node-eq/tests | sort)`
