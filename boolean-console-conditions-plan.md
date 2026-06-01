## Compact dynamic boolean console output

### Probe

Small Besht script:

```ts
let n = 3
console.log(n > 0)
console.log(n == 3)
```

Current compiled output first computes each boolean as `1`/`0`, then wraps it in another `if` to print `true`/`false`:

```sh
n=3
printf '%s\n' "$(if [ $(if [ "$n" -gt 0 ]; then printf 1; else printf 0; fi) = 1 ]; then printf true; else printf false; fi)"
printf '%s\n' "$(if [ $(if { _bst_left="$n"; _bst_right=3; ...; [ "$_bst_left" = "$_bst_right" ]; }; then printf 1; else printf 0; fi) = 1 ]; then printf true; else printf false; fi)"
```

A direct shell script would reuse the condition and print the final boolean text:

```sh
n=3
if [ "$n" -gt 0 ]; then printf '%s\n' true; else printf '%s\n' false; fi
if { _bst_left="$n"; _bst_right=3; [ "$_bst_left" = "$_bst_right" ]; }; then printf '%s\n' true; else printf '%s\n' false; fi
```

### Desired direction

When `console.log()` or `console.error()` receives a single non-static boolean expression, emit a direct shell `if <condition>; then printf true; else printf false; fi` instead of generating a boolean capture and formatting that capture. For multi-argument console calls, use the same condition directly inside one command substitution for the boolean argument.

### Guardrails

- Preserve static boolean console folding to direct `printf true`/`printf false`.
- Preserve boolean variable storage as `1`/`0`.
- Preserve object/list console formatting paths.
- Keep dynamic condition generation shared with `if (...)`.
- Update docs and tests in the same branch.

### Verification

- Codegen tests for single-argument comparison console output.
- Codegen tests for multi-argument boolean console output.
- Runtime integration test for dynamic boolean console output.
- Update `AGENTS.md`, `README.md`, and `skills/besht-scripting/SKILL.md`.
- `gofmt` changed Go files.
- `git diff --check`
- `make build`
- `make test`
- `go test ./...`
- `bun node-eq/compare $(rg --files -g '*.bsh' node-eq/tests | sort)`
