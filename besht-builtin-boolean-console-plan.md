## Compact Besht builtin boolean console output

### Probe

Small Besht script:

```ts
let path = "todo.md"
console.log(Besht.fs.isFile(path))
```

Current compiled output treats the `Besht.fs.isFile()` method call as a scalar value and prints the raw boolean storage value:

```sh
path='todo.md'
printf '%s\n' "$(if [ -f $path ]; then printf 1; else printf 0; fi)"
```

A direct shell script would normally print readable boolean text without nesting a command substitution inside `printf`:

```sh
path='todo.md'
if [ -f "$path" ]; then
  printf '%s\n' true
else
  printf '%s\n' false
fi
```

### Desired direction

Recognize `Besht.fs.*` and `Besht.strings.*` predicate method calls as boolean expressions everywhere the generator formats booleans, especially `console.log()` and `console.error()`. This lets console output render `true`/`false` instead of `1`/`0`, while preserving condition-position lowering to compact shell tests.

### Guardrails

- Keep the `Besht` namespace lowering as parser-level method calls mapped to builtin predicates.
- Preserve existing condition-position output (`if (Besht.fs.isFile(path))`) as direct shell tests.
- Preserve boolean storage as `1`/`0` for variable assignment.
- Quote dynamic file/string predicate arguments safely.
- Update docs and tests in the same branch.

### Verification

- Codegen tests for `console.log(Besht.fs.isFile(path))` and `console.error(Besht.strings.isEmpty(value))` rendering `true`/`false`.
- Condition tests still use direct `[ ... ]` checks.
- Runtime integration test for readable boolean output.
- Update `AGENTS.md`, `README.md`, and `skills/besht-scripting/SKILL.md`.
- `gofmt` changed Go files.
- `git diff --check`
- `make build`
- `make test`
- `go test ./...`
- `bun node-eq/compare $(rg --files -g '*.bsh' node-eq/tests | sort)`
