# Static String Bindings Plan

## Context

Small Besht sample:

```ts
let raw = "  alpha  "
let word = "alpha"
console.log(raw.trim())
console.log(word.includes("ph"))
if (word.startsWith("al")) console.log(word.toUpperCase())
```

Current compact output with `--opt-no-add-binaries-check --opt-no-source-map`:

```sh
_bst_starts_with() { case "$1" in "$2"*) return 0;; *) return 1;; esac; }
_bst_includes()    { case "$1" in *"$2"*) return 0;; *) return 1;; esac; }

raw='  alpha  '
word='alpha'
printf '%s\n' "$(printf '%s' "$raw" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
printf '%s\n' "$(if [ $(_bst_includes "$word" 'ph' && printf 1 || printf 0) = 1 ]; then printf true; else printf false; fi)"
if _bst_starts_with "$word" 'al'; then
    printf '%s\n' "$(printf '%s' "$word" | tr '[:lower:]' '[:upper:]')"
fi
```

A hand-written shell script would normally avoid the helpers and text tools because all values are known:

```sh
raw='  alpha  '
word='alpha'
printf '%s\n' 'alpha'
printf '%s\n' true
printf '%s\n' 'ALPHA'
```

The compiler already folds many static ASCII string literal method calls. It does not yet reuse that same static knowledge when the receiver is a variable bound to a static string literal.

## Implementation

- Extend static string text lookup so safe identifiers bound to static string literals can participate in existing string-method constant folding.
- Keep the existing guard for variables assigned inside control flow.
- Fold string search methods, transform methods, and zero-expression template literals through the existing static method machinery.
- Preserve dynamic fallback for non-constant receivers, non-ASCII paths that currently require runtime tools, and control-flow-assigned variables.

## Verification

- Add codegen coverage for static string variable method folds and absence of runtime helpers/tools.
- Add fallback coverage for control-flow-assigned string variables.
- Add runtime integration coverage for folded string variable methods.
- Update `AGENTS.md`, `README.md`, and `skills/besht-scripting/SKILL.md`.
- Run focused codegen tests and the full gate before committing.
