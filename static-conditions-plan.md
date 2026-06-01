# Static Condition Compactness Plan

## Context

The current compiler emits shell condition machinery even when a condition is plainly static:

```ts
let label = true ? "yes" : "no"
console.log(label)
if (false) {
    console.log("bad")
} else {
    console.log("fallback")
}
```

Current output includes:

```sh
label=$(if true; then printf '%s' 'yes'; else printf '%s' 'no'; fi)
if false; then
    printf '%s\n' 'bad'
else
    printf '%s\n' 'fallback'
fi
```

A shell author would normally write:

```sh
label='yes'
printf '%s\n' "$label"
printf '%s\n' 'fallback'
```

## Change

Fold static boolean conditions in code generation:

- `true ? a : b` emits `a`
- `false ? a : b` emits `b`
- `if (true) { ... } else { ... }` emits only the `then` body
- `if (false) { ... } else { ... }` emits only the `else` body when there is no `else if`

Keep the first implementation intentionally conservative. Static means a literal boolean expression made from `true`, `false`, `!`, `&&`, `||`, and `as` wrappers. Dynamic expressions must continue to use normal shell tests.

## Verification

- Add codegen tests proving static ternaries and simple static if/else do not emit unnecessary shell condition wrappers.
- Add integration coverage proving runtime output remains correct.
- Update `AGENTS.md`, `README.md`, and `skills/besht-scripting/SKILL.md` because this changes generated output behavior.
- Run `go test ./internal/codegen`, `go test ./...`, `make build`, `make test`, `bun node-eq/compare $(rg --files -g '*.bsh' node-eq/tests | sort)`, and `git diff --check`.
