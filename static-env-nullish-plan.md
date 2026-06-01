## Compact process.env nullish coalescing

### Probe

Small Besht script:

```ts
console.log(process.env.PATH ?? "missing")
let home = process.env.HOME ?? "/tmp"
console.log(home)
```

Current compiled output materializes `process.env.NAME` through the nullish sentinel and then runs a
general `??` command substitution:

```sh
printf '%s\n' "$(_bst_l=$(if [ -n "${PATH+x}" ]; then printf '%s' "$PATH"; else printf '%s' "$_BESHT_NULLISH_SENTINEL"; fi); if [ "$_bst_l" = "$_BESHT_NULLISH_SENTINEL" ]; then _bst_r='missing'; printf '%s' "$_bst_r"; else printf '%s' "$_bst_l"; fi)"
home=$(_bst_l=$(if [ -n "${HOME+x}" ]; then printf '%s' "$HOME"; else printf '%s' "$_BESHT_NULLISH_SENTINEL"; fi); if [ "$_bst_l" = "$_BESHT_NULLISH_SENTINEL" ]; then _bst_r='/tmp'; printf '%s' "$_bst_r"; else printf '%s' "$_bst_l"; fi)
```

A direct POSIX shell script would naturally use unset-only parameter expansion:

```sh
printf '%s\n' "${PATH-missing}"
home=${HOME-'/tmp'}
printf '%s\n' "$home"
```

`??` must preserve empty strings, so `${VAR-default}` is correct and `${VAR:-default}` is not.

### Desired direction

Add a compact lowering for `process.env.NAME ?? fallback` when the fallback can be emitted safely as a
single shell word. This should avoid the nullish runtime helper and command substitution for common
literal/default cases.

### Guardrails

- Preserve unset-only semantics: unset uses fallback; set-but-empty remains empty.
- Keep the existing sentinel path for dynamic fallbacks, fallbacks with command substitutions, or any
  value that cannot be safely embedded in POSIX parameter expansion.
- Keep direct `process.env.NAME` behavior unchanged when no `??` fallback is present.
- Keep dynamic/nullish behavior for non-env `??` unchanged.

### Verification

- Codegen tests for `process.env.NAME ?? "literal"` in assignments and console arguments.
- Runtime integration test proving unset uses fallback and empty stays empty.
- Negative/fallback-shape test for dynamic fallback to ensure it keeps the sentinel path.
- Update `AGENTS.md`, `README.md`, and `skills/besht-scripting/SKILL.md`.
- `gofmt` changed Go files.
- `git diff --check`
- `make build`
- `make test`
- `go test ./...`
- `bun node-eq/compare $(rg --files -g '*.bsh' node-eq/tests | sort)`
