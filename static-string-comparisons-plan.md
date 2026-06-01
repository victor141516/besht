# Static String Comparisons Plan

## Context

Small Besht sample:

```ts
let mode = "prod"
let msg = mode == "prod" ? "live" : "dev"
if (mode == "prod") console.log(msg)
console.log(mode !== "test")
```

Current compact output with `--opt-no-add-binaries-check --opt-no-source-map`:

```sh
mode='prod'
msg=$(if { _bst_left="$mode"; _bst_right='prod'; [ -n "${_BESHT_NULLISH_SENTINEL+x}" ] && [ "$_bst_left" = "$_BESHT_NULLISH_SENTINEL" ] && _bst_left=; [ -n "${_BESHT_NULLISH_SENTINEL+x}" ] && [ "$_bst_right" = "$_BESHT_NULLISH_SENTINEL" ] && _bst_right=; [ "$_bst_left" = "$_bst_right" ]; }; then printf '%s' 'live'; else printf '%s' 'dev'; fi)
if { _bst_left="$mode"; _bst_right='prod'; [ -n "${_BESHT_NULLISH_SENTINEL+x}" ] && [ "$_bst_left" = "$_BESHT_NULLISH_SENTINEL" ] && _bst_left=; [ -n "${_BESHT_NULLISH_SENTINEL+x}" ] && [ "$_bst_right" = "$_BESHT_NULLISH_SENTINEL" ] && _bst_right=; [ "$_bst_left" = "$_bst_right" ]; }; then
    printf '%s\n' "$msg"
fi
printf '%s\n' "$(if [ $(if { _bst_left="$mode"; _bst_right='test'; ... } = 1 ]; then printf true; else printf false; fi)"
```

A hand-written shell script would use constants:

```sh
mode='prod'
msg='live'
printf '%s\n' "$msg"
printf '%s\n' true
```

The compiler already tracks variables bound to string literals and folds literal-only comparisons. It misses using tracked string bindings for equality comparisons.

## Implementation

- Add a generator-aware static comparison path for `==`, `===`, `!=`, and `!==`.
- Resolve string identifiers from `stringConstMap` when they are not control-flow assigned.
- Use the generator-aware comparison path in boolean conditions, ternaries, and expression RHS comparisons.
- Preserve dynamic fallback for control-flow assigned variables, parameters, and values not known as static strings.

## Verification

- Add codegen coverage for static string variable comparisons in assignments, `if`, ternaries, and boolean console output.
- Keep dynamic fallback coverage by using function parameters and control-flow reassigned variables.
- Add runtime integration coverage for folded output.
- Update `AGENTS.md`, `README.md`, and `skills/besht-scripting/SKILL.md`.
- Run focused codegen tests and the full gate before committing.
