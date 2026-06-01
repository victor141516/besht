# Static Number Bindings Plan

## Context

Static numeric arithmetic folds when all operands are literals, but a named literal loses that static information.

Input:

```ts
let start = 2
let step = 3
let total = start + step * 4
console.log(total)
if (total > 10) console.log("big")
```

Current compact output with `--opt-no-add-binaries-check --opt-no-source-map`:

```sh
start=2
step=3
total=$(( $start + ($step * 4) ))
printf '%s\n' "$total"
if awk -v _a=$total -v _b=10 'BEGIN{OFMT="%.17g";exit !(_a > _b)}'; then
    printf '%s\n' 'big'
fi
```

A direct shell version would usually keep this constant:

```sh
start=2
step=3
total=14
printf '%s\n' "$total"
printf '%s\n' 'big'
```

## Implementation

- Add a numeric constant map beside the existing static string/list maps.
- Record variables bound to static numeric literals and static numeric arithmetic.
- Use numeric bindings in arithmetic folding, unary numeric folding, numeric relational comparisons, and static numeric `Math.*`/number methods where the generator already has static-number paths.
- Clear the binding when assignment becomes dynamic.
- Do not fold identifiers assigned inside control flow, matching existing static string/list guards.

## Verification

- Add codegen coverage showing named numeric arithmetic and comparisons fold.
- Add fallback coverage for variables assigned inside control flow.
- Add runtime integration coverage for the sample behavior.
- Update `AGENTS.md`, `README.md`, and `skills/besht-scripting/SKILL.md`.
- Run the focused codegen suite and the full gate before commit.
