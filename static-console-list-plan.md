## Compact static list console output

### Probe

Small Besht script:

```ts
console.log(["a", "b"])
console.log([])
console.error([1, true, "x"])
let xs = ["a", "b"]
console.log(xs)
```

Current compiled output routes static scalar lists through the generic list printer:

```sh
{ printf '[ '; if [ -n 'a
b' ]; then printf '%s\n' 'a
b' | awk 'BEGIN{first=1}{if(!first)printf ", "; printf "%s",$0; first=0}'; fi; printf ' ]\n'; }
```

A direct shell script would naturally print the already-known display strings:

```sh
printf '%s\n' '[ a, b ]'
printf '%s\n' '[  ]'
printf '%s\n' '[ 1, 1, x ]' >&2
xs='a
b'
printf '%s\n' '[ a, b ]'
```

### Desired direction

When `console.log()` or `console.error()` receives a statically known scalar list with no newline
elements, emit one quoted display string instead of the generic `awk` list formatter.

### Guardrails

- Preserve current list display semantics exactly, including booleans represented as `1`/`0` inside
  list output.
- Keep dynamic, nested, and newline-sensitive lists on the existing generic formatter.
- Reuse the generator-aware static scalar list recognizer so variables bound to static lists and static
  list method chains can benefit when safe.
- Do not change object logging in this branch.

### Verification

- Codegen tests for static list literals, empty lists, stderr, variables, and static list method chains.
- Fallback tests for dynamic/control-flow assigned lists and newline-containing lists.
- Runtime integration test for the displayed output.
- Update `AGENTS.md`, `README.md`, and `skills/besht-scripting/SKILL.md`.
- `gofmt` changed Go files.
- `git diff --check`
- `make build`
- `make test`
- `go test ./...`
- `bun node-eq/compare $(rg --files -g '*.bsh' node-eq/tests | sort)`
