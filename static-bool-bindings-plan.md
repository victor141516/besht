# Static Boolean Bindings Plan

## Context

Current compacting work has folded many static expressions to constants, but a boolean value that is first stored in a variable still takes the generic boolean-rendering path when printed or tested.

Input:

```ts
let ready = true
let same = "a" === "a"
console.log(ready)
console.log(same)
if (same) console.log("same")
```

Current compact output with `--opt-no-add-binaries-check --opt-no-source-map`:

```sh
ready=1
same=1
printf '%s\n' "$(if [ $ready = 1 ]; then printf true; else printf false; fi)"
printf '%s\n' "$(if [ $same = 1 ]; then printf true; else printf false; fi)"
if [ "$same" = 1 ]; then
    printf '%s\n' 'same'
fi
```

A shell author would usually keep the simple assignment but avoid the repeated conversion when the value is statically known:

```sh
ready=1
same=1
printf '%s\n' true
printf '%s\n' true
printf '%s\n' 'same'
```

## Implementation

- Add a `boolConstMap` beside the existing static string/list maps.
- Record variables assigned static boolean expressions, including static comparisons.
- Clear the binding when assignment becomes dynamic.
- Do not trust static boolean bindings for identifiers assigned inside control flow, following the existing string/list guard.
- Use the static binding in boolean console args, boolean template interpolation, `Boolean(...)`, and `if`/ternary conditions.
- Keep dynamic booleans and control-flow-assigned variables on the existing shell-test path.

## Verification

- Add unit coverage for direct `console.log` output and guarded dynamic fallback.
- Add runtime integration coverage for static boolean bindings.
- Update `AGENTS.md`, `README.md`, and `skills/besht-scripting/SKILL.md`.
- Run `go test ./internal/codegen`, then the full gate before commit.
