# Static Built String Methods Plan

## Context

Small Besht sample:

```ts
let c = ("x" + "y").toUpperCase()
console.log(c)
console.log(("abc" + "def").includes("cd"))
```

Current compact output with `--opt-no-add-binaries-check --opt-no-source-map`:

```sh
_bst_includes()    { case "$1" in *"$2"*) return 0;; *) return 1;; esac; }
c=$(printf '%s' "xy" | tr '[:lower:]' '[:upper:]')
printf '%s\n' "$c"
printf '%s\n' "$(if [ $(_bst_includes "abcdef" 'cd' && printf 1 || printf 0) = 1 ]; then printf true; else printf false; fi)"
```

A hand-written shell script would use constants:

```sh
c='XY'
printf '%s\n' "$c"
printf '%s\n' true
```

The compiler already folds static string methods and transforms on literal receivers. It misses receivers that are still compile-time strings after concatenation or static template interpolation.

## Implementation

- Extend the static string-fragment helper to compose static string concatenation and static template interpolation.
- Reuse that helper for static ASCII string method/transform receiver and argument detection.
- Keep non-ASCII, dynamic, and unsupported receivers on the existing POSIX helper/tool paths.

## Verification

- Add codegen coverage for static string transforms and search methods on built static strings.
- Add fallback coverage for dynamic built strings where needed.
- Add runtime integration coverage for the folded outputs.
- Update `AGENTS.md`, `README.md`, and `skills/besht-scripting/SKILL.md`.
- Run focused codegen tests and the full gate before committing.
