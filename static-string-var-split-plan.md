# Static String Variable Split Plan

## Context

Small Besht sample:

```ts
let csv: string = "a,b,c"
let sep: string = ","
let parts: list<string> = csv.split(sep)
console.log(parts.length)
for (part in csv.split(sep)) {
    console.log(part)
}
```

Current compiled output for the assignment still uses a POSIX tool even though both the receiver and separator are statically known:

```sh
csv='a,b,c'
sep=','
parts=$(printf '%s' "$csv" | tr '$sep' '\n')
```

The list-loop expression over `csv.split(sep)` also currently falls through to the generic method path and errors with `unknown command method "split"`.

A hand-written shell script would use a constant newline-backed value and a compact word loop:

```sh
csv='a,b,c'
sep=','
parts='a
b
c'
printf '%s\n' 3
for part in 'a' 'b' 'c'; do
    printf '%s\n' "$part"
done
```

The compiler already folds inline static ASCII string literal `.split()` calls. The previous iteration added a `Generator` helper that resolves static ASCII strings from literal expressions and variables bound to static string literals. This pass should reuse that helper for `.split()`.

## Implementation

- Convert static split detection from a literal-only helper to a generator-aware helper.
- Resolve both the `.split()` receiver and separator with `staticASCIIStringExprValue`.
- Reuse the new helper in static list length, list binding, assignment emission, compact list loops, and direct expression generation.
- Preserve fallback behavior for control-flow reassigned variables, non-ASCII receivers/separators, newline-sensitive split results, and dynamic values.

## Verification

- Add codegen coverage for `csv.split(sep)` assignment, `.length`, and compact list loops.
- Add fallback coverage for control-flow reassigned receivers.
- Add runtime integration coverage for folded variable split output.
- Update `AGENTS.md`, `README.md`, and `skills/besht-scripting/SKILL.md`.
- Run focused codegen tests and the full gate before committing.
