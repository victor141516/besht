# Static Boolean Bindings Plan

## Context

Small Besht sample:

```ts
let ok = Boolean("x")
let no = Boolean("")
if (ok) console.log(ok)
if (no) console.log("bad")
else console.log(no)
```

Current compact output with `--opt-no-add-binaries-check --opt-no-source-map`:

```sh
ok=1
no=0
if [ "$ok" = 1 ]; then
    printf '%s\n' "$(if [ $ok = 1 ]; then printf true; else printf false; fi)"
fi
if [ "$no" = 1 ]; then
    printf '%s\n' 'bad'
else
    printf '%s\n' "$(if [ $no = 1 ]; then printf true; else printf false; fi)"
fi
```

A hand-written shell script would avoid both dynamic condition branches and boolean formatting substitutions:

```sh
ok=1
no=0
printf '%s\n' true
printf '%s\n' false
```

The compiler already folds inline static boolean expressions in `if`/ternary contexts and static boolean console arguments. It does not yet reuse that static knowledge after a variable binding.

## Implementation

- Track top-level and local variables bound to statically known boolean expressions in the generator.
- Reuse that knowledge when folding conditions, ternaries, and string formatting for boolean values.
- Invalidate a variable's static boolean value on reassignment when the new expression is not statically known.
- Preserve dynamic behavior for variables assigned inside control flow or from runtime expressions.

## Verification

- Add codegen tests for folded static boolean bindings in console output, `if`, and ternary expressions.
- Add a reassignment fallback test showing dynamic assignments still use runtime formatting/conditions.
- Add runtime integration coverage for static boolean binding output.
- Update `AGENTS.md`, `README.md`, and `skills/besht-scripting/SKILL.md`.
- Run focused codegen tests and the full gate before committing.
