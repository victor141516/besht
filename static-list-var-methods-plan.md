# Static List Variable Methods Plan

## Context

Small Besht sample:

```ts
let list = ["a", "b", "c"]
let text = list.join("|")
console.log(text)
console.log(list.includes("b"))
console.log(list.indexOf("c"))
```

Current compact output with `--opt-no-add-binaries-check --opt-no-source-map`:

```sh
list='a
b
c'
text=$(printf '%s
' "$list" | awk -v s='|' 'NR>1{printf s}{if($0=="__BESHT_NL__")printf "\n"; else printf "%s",$0}')
printf '%s\n' "$text"
printf '%s\n' "$(if [ $(printf '%s\n' "$list" | grep -qxF 'b' && printf 1 || printf 0) = 1 ]; then printf true; else printf false; fi)"
printf '%s\n' "$(printf '%s\n' "$list" | awk -v _needle='c' 'BEGIN{found=-1}{if(found<0 && $0 == _needle) found=NR-1}END{printf "%s", found}')"
```

A hand-written shell script would use constants:

```sh
list='a
b
c'
text='a|b|c'
printf '%s\n' "$text"
printf '%s\n' true
printf '%s\n' 2
```

The compiler already folds inline static list literal, `Array.of(...)`, `Array.from({ length: N })`, and string `.split(...)` methods. It misses variables bound to those static list expressions.

## Implementation

- Resolve identifiers whose current binding is a static scalar list expression without newlines.
- Reuse that resolution in static list `.join()`, `.toString()`, `.includes()`, `.indexOf()`, and `.lastIndexOf()`.
- Preserve dynamic fallback for lists assigned through control flow, mutated lists, spread/nested/newline-sensitive lists, and non-static needles/separators.

## Verification

- Add codegen coverage for static list variables across literal, `Array.from`, and `.split` receivers.
- Add fallback coverage for reassigned/mutated list variables.
- Add runtime integration coverage for folded outputs.
- Update `AGENTS.md`, `README.md`, and `skills/besht-scripting/SKILL.md`.
- Run focused codegen tests and the full gate before committing.
