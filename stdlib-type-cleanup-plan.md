# Standard Library Namespace Cleanup and Type-Checking Removal Plan

This plan captures the decisions from the `todo.md` grilling session before implementation starts.

## Goals

- Remove Besht's user-facing type checker entirely.
- Keep TypeScript-compatible annotations, declarations, and `as` assertions for editor support and compiler representation hints only.
- Clean up the standard-library surface by removing legacy globals and grouping Besht-specific helpers under `Besht.*`.
- Preserve generated POSIX sh minimality. New namespaces must compile to inline shell and must not create runtime namespace objects or broad runtime metadata.

## Public API Decisions

### Remove Legacy Globals Completely

Remove the old global APIs from parser/checker/codegen/declarations/docs/tests:

- `env()` -> use `process.env.NAME ?? fallback`
- `exit()` -> use `process.exit(code)`
- `to_str()` -> use `value.toString()`
- `to_int()` -> use `Number.parseInt(value)`
- `String(value)` -> remove current alias; future real JS-compatible `String` global object remains future work in `todo.md`
- `len(list)` -> use `list.length`
- `head(list)` -> use `list[0]`
- `tail(list)` -> use `list.slice(1)`
- `append(list, value)` -> use `list.push(value)` or `[...list, value]`
- `contains(list, value)` -> use `list.includes(value)`
- `concat(a, b)` -> use `a.concat(b)`
- `range(start, end)` -> use `Besht.iter.range(start, end)`
- `file_exists`, `is_dir`, `is_readable`, `is_writable`, `is_executable`, `is_empty`, `is_set`
- `args.*`
- `Besht.conditions.*`

No migration shims or migration notes should remain in `README.md`, `AGENTS.md`, or `skills/besht-scripting/SKILL.md`.

### New Besht Namespaces

Besht-specific helpers live under the global `Besht` object with camelCase group and function names:

- `Besht.fs.isFile(path)` lowers to `[ -f path ]`
- `Besht.fs.isDir(path)` lowers to `[ -d path ]`
- `Besht.fs.isReadable(path)` lowers to `[ -r path ]`
- `Besht.fs.isWritable(path)` lowers to `[ -w path ]`
- `Besht.fs.isExecutable(path)` lowers to `[ -x path ]`
- `Besht.strings.isEmpty(value)` lowers to `[ -z value ]`
- `Besht.strings.isNonEmpty(value)` lowers to `[ -n value ]`
- `Besht.args.argv()`
- `Besht.args.positional(index)`
- `Besht.args.option(longName, shortName?)`
- `Besht.args.flag(longName, shortName?)`
- `Besht.iter.range(start, end)`

`Besht.iter.range()` keeps current inclusive-end, ascending-only behavior and remains usable both in `for (... in ...)` and value position as `number[]`.

### JS/Node-Like APIs That Stay

Keep these global JS/Node-shaped APIs:

- `fetch(url)` as the current synchronous text-only slice only
- `Boolean(value)` as primitive boolean coercion
- `process.env.NAME`
- `process.exit(code)`
- `console.log()` / `console.error()`
- `Number.*`, `Math.*`, `Array.*`, and `Object.*` supported subsets

Unsupported JS namespace methods should remain outside the recognized builtin surface for now.

## Type-Checking Removal Decisions

- Remove `--strict` from CLI usage, option parsing, docs, tests, and `codegen.Options`.
- If users pass `--strict`, it should be an unknown flag.
- Keep `--check`, but define it as parse/import/name/surface/command validation with no type validation.
- Remove annotation mismatch checks, return type mismatch checks, declared function arity checks, and strict-only tests.
- Keep semantic validation that protects compiler correctness or unsupported language surfaces:
  - import resolution and shell import validation
  - unknown Besht function/name validation
  - `const` reassignment errors
  - command lifecycle analysis, including unrun warnings and double-run errors
  - unsupported `fetch` response properties/methods
  - unsupported `Object.*` shapes such as non-compiler-managed objects, nested values in `Object.values()` / `Object.entries()`, and `process.env` enumeration
  - statement-only `forEach()` restrictions
  - builtin and method arity validation
  - syntactic constraints required for emission, such as getter/setter parameter counts

### Annotation Semantics After Removal

Docs should say:

> Besht does not type-check. Type annotations and `as` assertions are accepted for TypeScript-compatible syntax, editor support, and occasional compiler representation hints. They never validate values or produce type mismatch errors.

Source annotations must not decide source function return behavior. Instead:

- Source functions are value-returning if their body contains any `return <expr>` in a simple syntactic scan of nested blocks/branches.
- Source functions with no valued return are side-effecting/void.
- Source class methods and getters follow the same body-based return inference.
- Constructors and setters are always side-effecting/void.
- Function parameter annotations, local variable annotations, and `as Type` may still guide shell representation inference for empty lists, object/list methods, object parameters, callbacks, and similar cases.
- `declare function` return annotations remain trusted ABI hints because declarations have no body.
- `declare function` parameter lists are editor documentation only and must not enforce arity or types.

## Implementation Steps

1. Update AST/parser builtin recognition for new `Besht.fs`, `Besht.strings`, `Besht.args`, and `Besht.iter` surfaces.
2. Remove legacy builtin names and `Besht.conditions` handling.
3. Remove `--strict` from CLI and `codegen.Options`; adjust `CheckFile`, module loading, and checker calls.
4. Rework checker usage so no annotation/type mismatch validation runs, while semantic validators remain.
5. Add source function and class method return inference for codegen.
6. Preserve annotation and `as` representation hints where codegen needs them.
7. Update `internal/stdlib/declarations.go` to the canonical current surface only.
8. Update README, AGENTS, and `skills/besht-scripting/SKILL.md` with current APIs only and no references to removed APIs.
9. Rewrite relevant `todo.md` sections so the namespace/type-checking decisions are settled, not open questions.
10. Update Go tests and node-eq fixtures to use the new APIs and remove strict/type-check-only cases.
11. Run verification: `make build`, `make test`, and `bun node-eq/compare $(rg --files -g '*.bsh' node-eq/tests | sort)`.

## Acceptance Criteria

- No old API references remain in README, AGENTS, or the Besht scripting skill.
- Generated `stdlib.d.bsh` exposes only the canonical current surface.
- `--strict` is gone.
- `--check` still catches unsupported compiler surfaces and command lifecycle errors without type checking.
- Old globals are unavailable.
- New `Besht.*` APIs lower to the same small POSIX shell patterns as the old helpers.
- Source return behavior is inferred from bodies, not annotations.
- All required build/test/node-eq checks pass before commit.
