# Static Nullish Plan

## Context

Small Besht sample:

```ts
let name = "Ada"
console.log(name ?? "fallback")
let missing = null
console.log(missing ?? "fallback")
```

Current compiled output with `--opt-no-add-binaries-check --opt-no-source-map` emits the nullish sentinel helper and a subshell for both expressions:

```sh
_BESHT_NULLISH_SENTINEL=__BESHT_NULLISH_$$
name='Ada'
printf '%s\n' "$(_bst_l="$name"; if [ "$_bst_l" = "$_BESHT_NULLISH_SENTINEL" ]; then _bst_r='fallback'; printf '%s' "$_bst_r"; else printf '%s' "$_bst_l"; fi)"
missing="$_BESHT_NULLISH_SENTINEL"
printf '%s\n' "$(_bst_l="$missing"; if [ "$_bst_l" = "$_BESHT_NULLISH_SENTINEL" ]; then _bst_r='fallback'; printf '%s' "$_bst_r"; else printf '%s' "$_bst_l"; fi)"
```

A hand-written shell script would avoid the helper and branch entirely when the nullish outcome is statically known:

```sh
name='Ada'
printf '%s\n' "$name"
missing=
printf '%s\n' 'fallback'
```

`??` must still preserve empty string, `0`, and `false`, so this pass must not become shell `${var:-fallback}`. Fold only when the compiler can prove the left side is nullish (`null` or `undefined`, or a variable bound to them without control-flow reassignment) or prove it is non-nullish (static scalar literals and variables bound to them without control-flow reassignment).

## Implementation

- Track variables bound to static nullish literals separately from static string literals.
- Remove stale nullish tracking when variables are rebound to non-nullish or unknown values.
- Add a static nullish classifier for `??` left operands.
- In `a ?? b`, emit the left expression directly when statically non-nullish and emit the right expression directly when statically nullish.
- Preserve the current sentinel/subshell path for dynamic values, optional chaining, `process.env`, args helpers, callback results, list `find()`, and control-flow assigned variables.

## Verification

- Add codegen coverage for literal and variable static nullish/non-nullish cases.
- Add coverage proving empty string, `0`, and `false` preserve the left side.
- Add fallback coverage for control-flow assigned variables.
- Add runtime integration coverage for the folded static outputs.
- Update `AGENTS.md`, `README.md`, and `skills/besht-scripting/SKILL.md`.
- Run focused codegen tests and the full gate before committing.
