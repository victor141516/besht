# Static String Variable Methods Plan

## Context

Small Besht sample:

```ts
let s = "hello"
let upper = s.toUpperCase()
console.log(upper)
console.log(s.includes("ell"))
console.log(s.indexOf("l"))
```

Current compact output with `--opt-no-add-binaries-check --opt-no-source-map`:

```sh
_bst_includes()    { case "$1" in *"$2"*) return 0;; *) return 1;; esac; }

s='hello'
upper=$(printf '%s' "$s" | tr '[:lower:]' '[:upper:]')
printf '%s\n' "$upper"
printf '%s\n' "$(if [ $(_bst_includes "$s" 'ell' && printf 1 || printf 0) = 1 ]; then printf true; else printf false; fi)"
printf '%s\n' "$(awk -v _s="$s" -v _needle='l' 'BEGIN{_n=length(_needle); if(_n==0){printf "0"; exit} p=index(_s,_needle)-1; print (p<0)?-1:p}')"
```

A hand-written shell script would use constants:

```sh
s='hello'
upper='HELLO'
printf '%s\n' "$upper"
printf '%s\n' true
printf '%s\n' 2
```

The compiler already folds static string method calls on literal receivers and tracks variables bound to string literals for `.length`. It misses using that same tracked value for string search/transform methods.

## Implementation

- Add a codegen helper that resolves ASCII string expressions from literals and variables bound to static string literals.
- Reuse it in static string search and transform folding.
- Let static boolean console/condition paths recognize folded static string searches.
- Preserve dynamic fallback for control-flow reassigned variables, non-ASCII strings, and non-static arguments.

## Verification

- Add codegen coverage for static string variable transforms and searches.
- Add fallback coverage for control-flow reassigned string variables.
- Add runtime integration coverage for folded outputs.
- Update `AGENTS.md`, `README.md`, and `skills/besht-scripting/SKILL.md`.
- Run focused codegen tests and the full gate before committing.
