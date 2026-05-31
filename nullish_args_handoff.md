# Handoff: Nullish Coalescing and CLI Args Helpers

## Scope

Simplify command-line parameter handling while staying POSIX/Bourne-shell compatible. Do not implement defaults inside Besht args helpers; defaults should use TypeScript-style nullish coalescing.

## Agreed API Direction

Use `Besht.args` instead of globals:

```ts
let all = Besht.args.argv()
let input = Besht.args.positional(1) ?? "input.txt"
let branch = Besht.args.option("branch", "b") ?? "main"
let dryRun = Besht.args.flag("dry-run", "d")
```

Rules discussed:

- `Besht.args.argv()` returns all positional command-line args as `string[]`.
- `Besht.args.positional(n)` returns the 1-based positional arg or `undefined`.
- `Besht.args.option(longName, shortName?)` supports `--name value`, `--name=value`, and optional `-n value`.
- `Besht.args.flag(longName, shortName?)` supports `--name` and optional `-n`, returning boolean.
- Long names are mandatory for `option` and `flag`.
- Short-only helpers are intentionally not included.
- Defaults should be written with `??`, e.g. `Besht.args.option("branch", "b") ?? "main"`.

## Required `??` Semantics

Add TypeScript-compatible nullish coalescing before args helpers. `a ?? b` must return `b` only when `a` is `null` or `undefined`; it must preserve other falsy values:

- `"" ?? "fallback"` returns `""`.
- `0 ?? 99` returns `0`.
- `false ?? true` returns `false`.
- `undefined ?? "fallback"` returns `"fallback"`.
- `null ?? "fallback"` returns `"fallback"`.

The operator should support chaining and grouping:

```ts
let value = missing ?? nullValue ?? "fallback"
let grouped = (missing ?? "x") + "!"
```

Generated shell should not use `${var:-fallback}` for `??`, because POSIX `${var:-...}` also falls back for empty strings. Use an internal sentinel or helper strategy that distinguishes nullish from empty string/false/zero.

## Tests Added

Added future-facing node-eq coverage in:

```text
node-eq/tests/nullish_coalescing.bsh
```

The fixture covers:

- `undefined` and `null` fallbacks.
- Preservation of `""`, `0`, and `false`.
- Chained `??` expressions.
- Grouping with `+` and ternary expressions.
- Numeric fallback expressions.
- List indexing with present, empty, and missing entries.
- Object properties with present, empty, zero, false, and undefined values.
- Function return values typed as `string | undefined`.
- Literal and nested fallback expressions.

These tests are expected to fail until parser/checker/codegen support for `??` exists.

## Implementation Notes

Likely compiler work:

1. Lexer: add a `??` token without interfering with `?` propagation syntax.
2. AST: add a nullish-coalescing expression node or represent it as a binary expression with a distinct operator.
3. Parser: add precedence for `??`. Keep it lower than `+`/comparison and compatible with grouping. Decide whether to reject mixed `??` with `&&`/`||` without parentheses, as JavaScript does, or support a simpler deterministic precedence.
4. Semantic validation/codegen hints: infer the result representation from the left non-nullish value and right fallback. Besht does not type-check annotations, so this is for codegen dispatch only.
5. Codegen: emit shell that tests nullish only, not generic falsiness. Avoid `${x:-fallback}` because it treats empty string as missing.
6. Module rewrite: ensure any new expression node descends into both operands.
7. Docs: update README, AGENTS.md, and `skills/besht-scripting/SKILL.md` when implementing language support.

## Args Helper Codegen Shape

Keep helper shell POSIX-compatible: scan `"$@"` with `while [ "$#" -gt 0 ]`, `case`, `shift`, and `printf`. Do not use arrays or Bash-only syntax.

Possible helper behavior:

```sh
_bst_args_option branch b "$@"
_bst_args_flag dry-run d "$@"
_bst_args_positional 1 "$@"
_bst_args_argv "$@"
```

The helpers should return empty output plus a compiler-visible nullish sentinel or status signal when the argument is missing, so `??` can distinguish missing from an explicitly empty value.
