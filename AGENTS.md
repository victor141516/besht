# AGENTS.md

Guidance for AI coding agents working on the **besht** compiler codebase.

## Branch Workflow

Every new feature or fix follows this branch workflow:

1. **Create a branch** from `master` before making any changes. Name it descriptively (e.g. `feat/object-params`, `fix/join-separator`).
2. **Apply changes** on that branch.
3. **Test before committing** — run `make build` and `make test` (or relevant node-eq compare tests). Only if those succeed, create a commit on the branch.
4. **When the feature/fix is finished**, ask the user for confirmation before merging the branch to `master`. Only merge after explicit approval.
5. **If the user requests a different task** while the current branch is still in progress (feature not yet finished):
   - Tell the user that the current changes are **not finished** and will not be applied when switching to the new task.
   - Ask whether they want to merge the current branch as-is despite being incomplete, or discard/keep it for later.
   - Once resolved, checkout `master` and create a new branch from there for the new task.

This workflow is mandatory. Never commit directly on `master`. Never merge without user confirmation. Never switch tasks without informing the user about unfinished work on the current branch.

## Future Ideas Workflow

When a user mentions something that would be a good future improvement but isn't being implemented now, ask whether they want to add it to `todo.md`. When asking, include the **exact text** that would be written, so the user can review and adjust it before it's saved. Example:

> That's a good idea for later. Would you like me to add this to `todo.md`?
>
> **Proposed entry:**
>
> ```
> ## Compound assignment operators (+=, -=, *=)
> TypeScript supports i += 1. These would desugar to i = i + 1 in besht.
> ```
>
> Should I add it?

## To-Do Task Completion Workflow

Tasks in the to-do list must be completed fully before they are removed. Once a task is completely finished, delete it from the to-do list.

If a user asks to work on a task and it is only partially completed because some parts are unclear, clearly explain:

- what was completed
- what is still pending
- what questions or doubts need to be clarified

If the task description is vague, tell the user exactly what information is missing and ask for the answers needed to finish the implementation. For example: if you give me these answers, I can complete the implementation.

The only valid reason to stop working on a task is that the requirement is not clear. If the requirements are clear, continue working until the task is fully implemented.

When the task is fully done:

- remove it from the to-do list
- tell the user that the item was removed from the to-do list
- summarize the changes made
- mention that the documentation was updated
- mention the tests that were added or run

## Ongoing Skill Improvement Loop

`todo.md` contains the canonical, never-ending process for improving `skills/besht-scripting/SKILL.md` through no-hints validation agents. When continuing that work, read the todo entry first, then use this AGENTS.md file for compiler internals, pitfalls, branch workflow, and test expectations. Keep the skill file itself limited to user-facing Besht syntax and practical examples; put validation process notes in `todo.md` and AGENTS.md.

---

## Type Checking Policy

**Besht has no type checking — not at compile time, not at runtime. This is by design.**

- Type annotations (`let x: string = "hi"`, `function f(a: number): string`) are **completely ignored** by the compiler
- They exist only so users can write TypeScript-compatible syntax and get editor support (autocomplete, type hints) via `declare` statements and `.d.bsh` files
- The compiler never errors on type mismatches
- The `internal/semantics/validator.go` package performs semantic validation and signature collection only; it does not validate annotation/type mismatches

`--check` performs semantic validation only: imports, names, command lifecycle, unsupported surfaces, and emitter-required syntax. It must not validate annotation/type mismatches.

---

## MANDATORY: Keep This File Updated

**You must update AGENTS.md, SKILL.md, and README.md as part of every task — without waiting for the user to ask.**

`skills/besht-scripting/SKILL.md` is for people who want to use Besht. Keep it limited to user-facing language syntax, compiler commands, flags, and practical scripting examples. Do not put compiler implementation details, internal architecture, test layout, node-eq instructions, agent workflow rules, or contributor-only pitfalls in the skill file. Put those details in this `AGENTS.md` instead.

This is not optional and does not require a prompt. Any time you add, change, rename, or remove something in the compiler or language, update the affected sections in all three files before marking the task complete. The rule applies to:

- User-facing changes: new syntax, new builtins, new methods, renamed/removed keywords, new CLI flags, changed compilation behavior, new output formats
- Internal changes that affect how agents should work on this codebase: architecture shifts, new pitfalls, changed Go APIs, new test patterns

**Do not wait until the end of a session.** Update the docs in the same commit as the code change. A stale AGENTS.md, SKILL.md, or README.md is a bug.

### What to update and where

| Changed                                        | Update                                                                                    |
| ---------------------------------------------- | ----------------------------------------------------------------------------------------- |
| Language syntax (new keyword, builtin, method) | AGENTS.md Syntax Reference + SKILL.md + README.md                                         |
| CLI flag added/renamed                         | AGENTS.md Commands + CLI Flags table + SKILL.md Compile section + README.md CLI reference |
| Compilation behavior changed                   | AGENTS.md Key Design Decisions + Pitfalls + README.md                                     |
| Architecture changed                           | AGENTS.md Architecture section                                                            |
| Test layout or test workflow changed           | AGENTS.md Commands + Test Layout                                                          |
| New pitfall discovered                         | AGENTS.md Common Pitfalls                                                                 |

---

## Project Overview

**besht** is a TypeScript-flavored language that compiles to POSIX sh. It provides TypeScript-compatible annotations for editor support and compiler representation hints, structured control flow, module imports, and a safe command-execution model over raw shell scripting. The compiler is written in Go.

Source files use `.bsh` extension. The compiler produces a single `.sh` file (POSIX sh, no bashisms).

**Design principle:** The user never writes shell syntax. All external command invocation goes through `$()` command expressions. The compiler controls all quoting and argument passing.

**TypeScript-to-shell translation directive:** Prefer translating TypeScript constructs into reusable lower-level compiler transformations, then implement library APIs as compositions of those transformations. Do not add one-off special emitters for individual APIs when a generic construct can model them. For example, implement callback transformation generically, then express `Array.prototype.reduce` through the callback transformation plus reduce semantics, rather than hard-coding a bespoke reduce-only implementation.

## Commands

```bash
# Build the compiler
make build                        # → dist/besht

# Run all tests
make test

# Run node-eq parity fixtures
bun node-eq/compare $(rg --files -g '*.bsh' node-eq/tests | sort)

# Coverage report (terminal)
make cover

# Coverage report (HTML)
make cover-html && open coverage.html

# Compile a .bsh file
go run ./cmd/besht/ init
go run ./cmd/besht/ init --force
go run ./cmd/besht/ compile <file.bsh>
go run ./cmd/besht/ compile <file.bsh> -o out.sh
go run ./cmd/besht/ compile --check <file.bsh>

# View source and compiled shell side by side in the terminal
go run ./cmd/besht/ visualize <file.bsh>

# Split mode — one .sh per .bsh, imports become source calls
go run ./cmd/besht/ compile <file.bsh> --split -o <outdir/>

# Omit the conditional runtime POSIX self-check from compiled output
go run ./cmd/besht/ compile <file.bsh> --opt-no-add-binaries-check

# Opt in to extensionless .ts import fallback when .bsh is absent
go run ./cmd/besht/ compile <file.bsh> --opt-resolve-ts-imports

# Allow explicit .sh imports outside the compiler root
go run ./cmd/besht/ compile <file.bsh> --opt-allow-external-shell-imports

# Opt in to jq-backed JSON codegen
go run ./cmd/besht/ compile <file.bsh> --opt-use-jq
```

### CLI Flags

The preferred CLI shape is mode-first: `besht compile ...` or `besht visualize ...`. Legacy `besht <file.bsh> [flags]` and `besht --check <file.bsh>` are still accepted as compile aliases for compatibility, but do not add new docs or tooling that rely on the legacy spelling.

| Flag                          | Description                                                           |
| ----------------------------- | --------------------------------------------------------------------- |
| `-o <path>`                   | Output file or directory (required with `--split`)                    |
| `init`                        | Write `./stdlib.d.bsh` declarations in the current directory          |
| `init --force`                | Overwrite a different existing `./stdlib.d.bsh`                       |
| `compile`                     | Compile `.bsh` to POSIX sh, printing to stdout unless `-o` is used    |
| `visualize`                   | Open a terminal side-by-side source/compiled-shell view, no output file |
| `--split`                     | Compile each `.bsh` to its own `.sh`; imports become `. source` calls |
| `--check`                     | Validate imports, command usage, and unsupported fetch APIs, no output |
| `--opt-no-add-binaries-check` | Omit the conditional runtime POSIX utility self-check block           |
| `--opt-no-source-map`          | Omit `# besht:file:line:col` source comments from compiled output        |
| `--opt-resolve-ts-imports`     | Let extensionless imports fall back to `.ts` only when `.bsh` is absent |
| `--opt-allow-external-shell-imports` | Allow explicit `.sh` imports outside the compiler root          |
| `--opt-use-jq`               | Enable jq-backed JSON codegen                                   |
| `--version`                   | Print version                                                         |

### `--opt-*` flags

All flags that change how code is transformed or what is emitted share the `--opt-` prefix. Currently:

| Flag                          | Effect                                                                          |
| ----------------------------- | ------------------------------------------------------------------------------- |
| `--opt-no-add-binaries-check` | Do not emit the `_r=$(printf ...)` runtime check when it would otherwise be needed |
| `--opt-no-source-map`          | Do not emit `# besht:file:line:col` source comments in compiled output         |
| `--opt-resolve-ts-imports`     | Resolve extensionless imports to `.bsh` first, then `.ts` if `.bsh` is absent |
| `--opt-allow-external-shell-imports` | Permit explicit `.sh` imports outside the compiler root; `.bsh` imports remain root-confined |
| `--opt-use-jq` | Permit generated JSON code to invoke `jq`; required for `JSON.parse()`, JSON path/extraction code, and `JSON.stringify()` |

Pass via `codegen.Options{NoCheck: true, NoSourceMap: true, ResolveTsImports: true, AllowExternalShellImports: true, UseJQ: true}` in Go code.

Generated shell emits inline `# besht:file:line:col` source comments at non-class statement boundaries and before explicit class constructor/accessor/method shell functions. Class declarations skip the generic statement-boundary comment so synthetic property accessors and implicit default constructors do not receive source comments.

`besht visualize` depends on those source comments internally. It must force `codegen.Options.NoSourceMap = false` for the internal compile even if the flag is passed, then remove all `# besht:file:line:col` lines from the displayed shell pane. The source pane must also fill gaps between mapped source lines from the original file, including blank lines and unmapped code lines such as closing braces. The view is terminal-only: it pages the rendered text through an in-terminal pager when available, falls back to stdout when not attached to a terminal, and must never write a compiled output file. It should query the live terminal width through `/dev/tty`/`stty size` before falling back to `COLUMNS` or the default width, so large terminals get wide panes. Panes should keep a bat-style line-number gutter and rule (`line   │ code`) rather than the older plain `|` separators. The header row should show only the input file name above the source pane and the same base name with `.sh` above the compiled pane. Long lines must wrap inside their own pane with a `↳` continuation marker in the gutter instead of being truncated or hidden behind horizontal scrolling; when using `less`, do not pass `-S`, explicitly disable chop-long-lines, and install a temporary `--lesskey-src` file that maps horizontal scroll keys to `noaction` when supported so only vertical scrolling remains. When stdout is a color-capable terminal and `bat` is installed, visualization uses `bat --language=TypeScript` for source lines and `bat --language=sh` for compiled shell lines; `ShellScript` is not a valid bat syntax name. Keep renderer padding/wrapping ANSI-aware and expand tabs before measuring so color escape sequences and tab stops do not shift the side-by-side columns.

When no runtime helpers, args snapshot, or POSIX self-check are emitted, generated entry scripts keep exactly one blank separator between the header and the first body line. Do not reintroduce a double-empty preamble gap.

## Architecture

```
cmd/besht/
└── main.go              # CLI entry point: flags, dispatch to codegen.CompileFile

internal/
├── ast/
│   └── ast.go           # All AST node types (Pos, Type, Statement, Expression, ClassDecl)
│                        # No logic — pure data structures
├── lexer/
│   ├── token.go         # TokenType enum + keywords map + Token struct
│   └── lexer.go         # Hand-written lexer; TokDollar for $() expressions
├── parser/
│   └── parser.go        # Recursive descent parser → AST
│                        # parseCmdExpr() handles $() command expressions
├── semantics/
│   └── validator.go     # Semantic validator + scope resolver; walks AST, annotates representation hints
│                        # FnSig, Scope, Validator structs; RegisterFn for cross-module sigs
│                        # checkCommandMethodArity() routes .pipe()/.run()/.readStdoutLines() etc.
├── viewer/
│   └── viewer.go        # Terminal side-by-side source/shell renderer for `besht visualize`
│                        # Compiles with source comments internally, strips them from display
└── codegen/
    ├── codegen.go       # Generator: AST → POSIX sh string
    │                    # genCmdPipeline/genCmdChain build shell pipelines
    │                    # cmdArgWordForExpr() emits safe command words/quotes safely
    └── modules.go       # Multi-file compilation: Compiler, load(), emit(), emitSplit()
                         # Import resolution, module name qualification, dep ordering
                         # rewriteFnCalls() AST pass qualifies imported fn names
                         # CompileFileSplit() → per-file .sh with source + include guards
```

**Compilation pipeline:**

```
.bsh source
  → Entry stdlib.d.bsh auto-load       ← CompileFile, CompileFileSplit, and CheckFile
  │                                      load only filepath.Dir(entry)/stdlib.d.bsh when present
  → Lexer  (lexer.Tokenize)
  → Parser (parser.Parse)
  → Semantics validator (Validator.Validate) ← registers cross-module fn sigs first
  → Command Analysis pass                ← assigns identities to $() calls, tracks
  │                                         run/text/lines/exitCode usage per object,
  │                                         emits warnings for unrun commands,
  │                                         errors for double-run without clone()
  → collectObjectTypes() pre-pass        ← populates objPropTypeMap for object property
  │                                         type resolution inside function bodies
  → rewriteFnCalls() AST pass            ← qualifies imported/local Besht fn call names
  → Shell import collection              ← validates explicit .sh imports and source/copy plan
  → Codegen (codegen.CompileFile)        ← bundled mode: single .sh output
          or (codegen.CompileFileSplit)   ← split mode: one .sh per .bsh
          or viewer.Build                 ← visualize mode: CompileFile with source
                                             comments enabled internally, then side-by-side
                                             terminal rendering with comments hidden
```

Bundled output omits `# --- module: name ---` separator comments when only one Besht module emits code. Keep separators for multi-module bundled output so generated files remain navigable.

**Split mode output structure:**

- Entry point `.sh`: `#!/bin/sh`, sets `_BESHT_ROOT`, then `. "$_BESHT_ROOT"/'lib/x.sh'` per Besht or copied shell import
- Library `.sh`: include guard (`[ -n "$_BESHT_LOADED_..." ] && return 0`), then source lines, then function definitions
- Explicit imported `.sh` files are copied as raw shell dependencies and sourced with `_BESHT_SHELL_LOADED_...` guards
- `_BESHT_ROOT` is set once by the entry point; all sourced files reuse it for their own sub-sources

## Key Design Decisions

### No Raw Shell — `$()` Command Expressions

The user never writes shell code directly. All external commands go through:

```ts
$("git", "log", "--oneline", "-5");
$("curl", "-sf", url).pipe($("jq", ".name"));
$("make", "build").stderr("null");
```

The compiler emits shell-safe literal command words bare when possible (`git status --short`) and single-quotes anything that needs protection (`'*.go'`, `'hello world'`, shell reserved command names, embedded quotes). Single-quote escaping handles embedded `'` characters automatically (`'it'"'"'s alive'`). Variable references are passed as `"$var"`. This prevents shell injection and gives the compiler full control over quoting strategy.

The `shell { }` syntax has been **removed**. There is no escape hatch to raw shell.

### Command Object Model (lazy pipeline)

`$()` always returns a **Command object** — a lazy pipeline description that produces **no shell code** at the point of declaration. Execution only happens when `.run()` is called on the object.

**Lifecycle:**

```ts
// 1. Declaration — no shell code emitted yet
let cmd = $("cat", "/etc/passwd").pipe($("grep", "root")).stderr("null");

// 2. Building — still no shell code; methods return the same Command
let cmd2 = $("make", "build").stdout("/tmp/out.log").stderr("&1");

// 3. Execution — .run() emits the actual shell pipeline
cmd.run(); // returns the same Command object (self) for chaining

// 4. Inspection — only valid after .run(); compiler tracks usage
let out = cmd.readStdout(); // captured stdout (same shell var on every call)
let rows = cmd.readStdoutLines(); // same as readStdout(), split by newline
let code = cmd.exitCode(); // captured exit code

// Chain style also works:
let result = $("whoami").run().readStdout();
```

**What the compiler emits depends on what is used:**

| What the user does                               | What the compiler emits                                |
| ------------------------------------------------ | ------------------------------------------------------ |
| Command declared but `.run()` never called       | Nothing — plus a compile-time **warning** on that line |
| `.run()` called, `.readStdout()`/`.readStdoutLines()` never used | Bare shell command, no variable                        |
| `.run()` called, `.readStdout()` used                  | `_varname=$(cmd …)` captures stdout                    |
| `.run()` called, `.exitCode()` used              | Command runs, `_varname_exit=$?` captures exit code    |
| Both `.readStdout()` and `.exitCode()` used            | Both captured                                          |

The captured variable name is derived from the **besht variable name** for readability. `let result = $("whoami")` where `result.run()` is later called generates `result=$(whoami)`. The same variable is reused for every `.readStdout()` call — no duplication.

Immediate anonymous reads such as `console.log($("pwd").run().readStdout())` compile directly to command substitution (`"$(pwd)"`) instead of creating a temporary capture variable. Named command objects and inline `.exitCode()` reads still emit capture/exit variables so later reads remain correct.

**Single-run enforcement:**

Each Command object may only have `.run()` called once. Calling `.run()` a second time on the same object is a **compile error**. To run the same pipeline again, use `.clone()` which returns a fresh, un-run copy of the pipeline.

```ts
let cmd = $("whoami");
cmd.run();
cmd.run(); // ← compile error: "cmd already run on line N; use cmd.clone()"
cmd.clone().run(); // ← correct
```

The compiler assigns each `$()` call a unique identity during the semantic analysis pass, tracks which variables hold that identity (alias analysis), and reports an error if `.run()` is seen twice on the same identity.

**No auto-run:** `$()` as a bare statement (with no `.run()`) emits nothing and produces a warning. There is no implicit execution. The user must always call `.run()` explicitly.

**Output philosophy:** Generated shell should be readable and minimal. Side-effect-only commands (`cmd.run()` with no `.readStdout()`/`.readStdoutLines()`/`.exitCode()`) compile to a bare shell command with no variable. Capture variables are only emitted when the corresponding method is actually used. Helper code, metadata, array/object metadata, runtime checks, and other shell boilerplate should only be emitted when the compiled program uses functionality that requires it. For example, if an array operation needs metadata, do not emit that metadata for programs that never use that operation. Static scalar array literals, array-returning method chains over static scalar arrays (`concat`, `slice`, `reverse`, `toReversed`, `sort`, `toSorted`, `toSpliced`, `fill`, `push`, `unshift`, `pop`, `shift`), static scalar `Array.of(...)` calls, static `Array.from("text")` calls, and static `Array.from({ length: N })` calls compile to quoted newline-backed shell strings when elements contain no newlines; dynamic, nested, spread, and newline-sensitive array expressions keep the `printf` builder. Static string literals, variables bound to static string literals, static scalar array expressions, and variables bound to static scalar arrays compile `.length` properties to numeric constants; dynamic lengths keep the `wc` path. Static scalar array expressions and variables bound to them compile to compact shell `for item in ...; do` loops when elements contain no newlines; dynamic and newline-sensitive array expressions keep the heredoc-backed read loop so assignments and `break` persist in the current shell. Small static integer `Besht.iter.range()` loops compile to compact `for i in 1 2 3; do` shell; dynamic and large static ranges keep the counter `while` loop. Static scalar array indexes and `.at()` calls with known in-range integer indexes, including negative `.at()` indexes, and static nested-array indexes with known row/column indexes compile to constants; dynamic, unknown, and out-of-range indexes keep the POSIX `sed`/`awk`/packed-row path. Static scalar array destructuring over literals and variables bound to them emits direct assignments; dynamic and newline-sensitive destructuring keeps the temp-and-`sed` path. Static scalar array expressions and variables bound to static scalar arrays fold `.join()` and `.toString()` calls to one quoted string when elements contain no newlines and the separator is static; dynamic and newline-sensitive joins keep the `awk` path. Static scalar array expressions and variables bound to static scalar arrays fold `.includes()`, `.indexOf()`, `.lastIndexOf()`, and `.at()` calls with static scalar needles or indexes to constants; dynamic searches and `.at()` calls keep the `grep`/`awk` path. Static Set bindings populated by straight-line static `.add()` calls compile `.has()` to constants; dynamic, control-flow, callback, and newline-sensitive Set values keep the `awk`/`grep` runtime path. Static scalar array `console.log()` and `console.error()` output compiles to one quoted display string when elements contain no newlines; dynamic and newline-sensitive array printing keeps the generic `awk` formatter. Static ASCII string expressions built from literals, variables bound to static ASCII strings, concatenation, template interpolation, and chained static ASCII transforms fold `.includes()`, `.startsWith()`, `.endsWith()`, `.indexOf()`, `.lastIndexOf()`, `.localeCompare()`, `.charAt()`, and transforms (`trim`, `trimStart`, `trimLeft`, `trimEnd`, `trimRight`, `toUpperCase`, `toLowerCase`, `slice`, `substring`, `repeat`, `replace`, `replaceAll`, `concat`, `padStart`, `padEnd`) with static arguments to constants; dynamic and non-ASCII string searches/transforms keep the helper/`awk` or POSIX `sed`/`tr`/`awk` paths. Dynamic string `slice()`, `at()`, and string indexing use `awk substr` rather than `cut`. The ternary shape `x.startsWith(prefix) ? x.slice(prefix.length) : x` compiles to POSIX `${x#prefix}` parameter expansion when `x` is a simple identifier and the prefix is shell-pattern-safe. Static ASCII string literal `.split()` calls, static ASCII `Array.from("text")` calls, and variables bound to static ASCII strings calling `.split()` with static separators compile to quoted newline-backed arrays and compact `for item in ...; do` loops when resulting elements contain no newlines; dynamic and non-ASCII splits and string `Array.from(value)` calls keep the POSIX `tr`/`awk` path. Inline static scalar object literal `Object.keys()`, `Object.values()`, `Object.entries()`, and `Object.hasOwn()` calls compile to constants; unmutated named object `Object.keys()`, static-scalar `Object.values()`/`Object.entries()`, static-key `Object.hasOwn()`, literal-only `Object.assign({}, ...)`, and literal-only object spread calls also fold to constants, while mutated or dynamic objects keep compiler-managed metadata so assignments, computed keys, and spread copies stay visible. Direct reads of scalar properties from static object literal bindings compile to constants when the object root is not assigned, computed-assigned, aliased, or passed to a function; mutation and alias escapes keep property reads dynamic. Static boolean `if` conditions and ternary expressions, including `Boolean(value)`, `Array.isArray(value)`, static string/array/Set searches, static `Object.hasOwn()`, static boolean object properties, and static comparisons with arithmetic literal operands, compile to the selected branch or value; dynamic conditions keep shell tests. Dynamic boolean object properties used directly in conditions compile to direct `[ "$_obj_name_prop" = 1 ]` tests; optional property chains, `process.env`, and non-boolean properties keep their nullish or generic truthiness paths. Static safe `process.env.NAME ?? fallback` defaults compile to POSIX unset-only `${NAME-fallback}` expansion; dynamic or unsafe defaults keep the nullish sentinel path. Static string literal `Number.parseInt()` and global `parseInt()` calls with parseable prefixes and static radix compile to numeric constants; dynamic parseInt uses inline AWK when a general parser is needed; `Number.parseInt(value.slice(start, start + 2), 16)` and `parseInt(value.slice(start, start + 2), 16)` use a tiny `_bst_hex_byte` helper selected from static slice and radix analysis, and broader `Number.parseInt(value.slice(a, b), radix)` / `parseInt(value.slice(a, b), radix)` calls pass slice bounds into the AWK parser instead of emitting a separate substring command. Compiler-known dynamic integer relational comparisons compile to POSIX integer tests; float and unknown relational comparisons keep the POSIX `awk` path. Static numeric arithmetic over literal numbers and variables bound to static numeric expressions, static numeric expression/variable `.toString()`/`.toFixed()` calls, static numeric API receivers of `.toString()`, `Math.*` constants, and literal-argument `Math.*` calls compile to constants; dynamic numeric expressions keep shell arithmetic or the POSIX `awk` path. Single-command redirects append directly to the command; pipeline redirects keep `{ ...; }` grouping so the redirect applies to the whole pipeline. String runtime helpers (`_bst_starts_with`, `_bst_ends_with`, `_bst_includes`) are emitted only when generated shell calls the corresponding one-argument string method; two-argument string search methods use inline `awk`, and array `.includes()` uses `grep -qxF` without emitting `_bst_includes`. Top-level scripts that only read `Besht.args.positional()` use an inline `"$@"` scan instead of the full args runtime; `argv()`, `option()`, `flag()`, args reads inside functions, and split output keep the shared parser runtime. This unused-feature elision is best effort and must never break correctness. All boilerplate is opt-out via `--opt-*` flags.

String `localeCompare(other)` is intentionally compact: static ASCII calls fold to `-1`, `0`, or `1`, and dynamic calls lower to `LC_ALL=C awk` bytewise comparison. Do not expand it into full ICU locale collation without a deliberate locale/runtime design.

Static scalar `Object.fromEntries(...)` calls over literal `[key, value]` rows or static `Object.entries(obj)` output may fold to compiler-managed object slots. Dynamic `Object.fromEntries(entries)` must build a fresh object, validate every dynamic key before generated shell uses `eval`, preserve first-seen key order, let later rows overwrite values, and avoid adding a runtime helper.

Array `sort()` / `toSorted()` are the narrow default lexical slice only: no comparator callback support. Static scalar receivers and static scalar method chains should fold with the same constant-array machinery as `reverse()` and `toReversed()`. Dynamic scalar receivers lower to `LC_ALL=C sort`. Keep Besht's existing array-returning convention: value-position `let sorted = items.sort()` returns a sorted value, while statement-position `items.sort()` rebinds the named receiver to that result. `toReversed()`, `toSorted()`, and `toSpliced()` are copy-style APIs and must not be added to statement-position receiver rebinding.

Array `toSpliced(start, deleteCount?, ...items)` is a copied splice for scalar arrays. Static scalar receivers and scalar inserted values should fold to constants. Dynamic receivers lower to one `awk` pass that normalizes JavaScript-style start/delete bounds, emits prefix, inserted values, and suffix, and preserves known-length empty arrays.

Array `fill(value, start?, end?)` is a scalar-array slice with JavaScript-style bound normalization. Static scalar receivers and scalar fill values should fold with the same constant-array machinery as `slice()`. Dynamic scalar receivers lower to a single `awk` pass and must preserve known-length empty arrays. It follows the existing array-returning convention: value position returns a filled value, statement position rebinds the named receiver.

Array `flatMap(callback)` is a one-level scalar-array callback slice built as `map()` plus unpacking of callback-returned scalar arrays. Reuse `genListMapIntoVar` / `genListMap` so map callback restrictions, return-slot callbacks, and current-shell side effects stay aligned. Do not use this as a general `flat()` implementation; nested arrays and empty inner arrays still need a broader representation design.

Array `.at()` must return the nullish sentinel for out-of-range indexes so `items.at(99) ?? fallback` behaves like JavaScript `undefined ?? fallback`; do not collapse missing elements to an empty string.

Static primitive `.toString()` calls in direct bindings, string concatenation, and template interpolation compile to constants; dynamic receivers keep runtime formatting.

Static `String(value)` calls fold primitives, null/undefined, scalar arrays, object literals, and Set literals to JS-compatible display strings. Dynamic booleans render `true`/`false`, dynamic scalar arrays reuse comma-join lowering, object-producing expressions such as `Object.assign(...)` must still emit their side effects before the returned label becomes `"[object Object]"`, and direct `String(JSONValue)` remains unsupported in favor of scalar extraction or `JSON.stringify()`.

Static `??` expressions compile to the selected side when the left operand is provably nullish or non-nullish. Preserve `""`, `0`, and `false` as non-nullish; control-flow assigned variables and optional/dynamic nullish sources must keep the sentinel path.

Static numeric API receivers of `.toString()`, such as `Math.round(2.7).toString()`, `Number.parseInt("42").toString()`, and `parseInt("42").toString()`, compile to quoted constants; dynamic receivers keep runtime formatting.

`Math.E`, `Math.LN2`, `Math.LN10`, `Math.LOG2E`, `Math.LOG10E`, `Math.PI`, `Math.SQRT1_2`, and `Math.SQRT2` parse as static numeric literals. Keep Math methods such as `Math.min(...)` on the existing method-call path.

Static ASCII string literal indexes and indexes into variables bound to static ASCII strings compile to constants when the index is a known non-negative integer; dynamic indexes, non-ASCII strings, and control-flow assigned string variables keep the AWK substring path.

Static value-position `||` and `&&` expressions compile to the selected side when the left operand's truthiness is known. Preserve JavaScript value semantics: static string/number results stay strings/numbers, while selected boolean results still render as `true`/`false` for console output.

Static boolean `console.log()` and `console.error()` arguments such as `Boolean("")`, `true`, simple static `!`/`&&`/`||` expressions, static comparisons, and variables bound to static boolean expressions render directly as `true`/`false`; static boolean `if` and ternary conditions use the same generator-aware constant path. `Besht.fs.*` and `Besht.strings.*` predicates are boolean expressions too: in single-argument console calls, emit a direct shell `if` that prints `true`/`false`; in value position, keep boolean storage as `1`/`0`. Dynamic boolean console arguments should reuse `genCondition()` directly: single-argument console calls emit `if <condition>; then printf '%s\n' true; else printf '%s\n' false; fi`, while multi-argument console calls use one `$(if <condition>; then printf true; else printf false; fi)` argument. Do not compute a `1`/`0` boolean capture and then wrap it in a second formatter when the condition is available.

Variables bound to static boolean expressions may fold in boolean output, template interpolation, `Boolean(...)`, `if`, and ternary conditions. Do not fold variables assigned inside control flow because later loop iterations or branch-dependent assignments can make the initial value stale.

Variables bound to static string literals may fold `.length` to a numeric constant. Do not fold variables assigned inside control flow because later loop iterations or branch-dependent assignments can make the initial value stale.


Static scalar equality comparisons, including equality comparisons against variables bound to static string literals, and static numeric relational comparisons compile to constants, including comparisons whose operands are themselves static folded arithmetic, string methods/transforms, `Math.*`, parseable `Number.parseInt()`/`Number.parseFloat()` calls, or global `parseInt()`/`parseFloat()` aliases. Dynamic relational comparisons over compiler-known integer expressions compile to POSIX `[ "$n" -lt 3 ]` style tests; floats, function results, command output, annotations-only numbers, and unknown values keep the `awk` path. Dynamic equality must keep the `_bst_left`/`_bst_right` binding block because direct `[ $(fn) = value ]` breaks POSIX argument parsing for spaces and newlines.

Variables bound to static numeric expressions may fold in arithmetic, numeric comparisons, unary numeric expressions, numeric `.toString()`/`.toFixed()`, and static `Math.*` calls. Do not fold variables assigned inside control flow because later loop iterations or branch-dependent assignments can make the initial value stale.

### Variable Name Mangling

### Variable Name Mangling

Shell has no scoping. Besht mangles all locals inside functions:

- Function params: `_<qualFnName>_<paramName>` (e.g. `_main__greet_name`)
- Local `let` vars inside functions: `_<fnName>_<varName>` (e.g. `_greet_result`)
- Top-level vars: no mangling (plain name)
- Loop vars and catch vars: same mangling rules as locals

`codegen.Generator.paramMap` tracks name→mangled-name within each function scope. `rewriteVarRefs()` applies the mapping inside string literals.

### Module Naming

Module names derive from the file path relative to the entry file's directory:

- Entry file `examples/main.bsh` → root = `examples/`
- Imported `examples/lib/log.bsh` → module name `lib/log`
- Qualified function prefix: `lib__log` (slashes → `__`, hyphens → `_`)
- Function `info` in `lib/log` → shell function `lib__log__info`

The `rewriteFnCalls()` AST pass in `modules.go` walks the entire program AST before codegen and rewrites all `FnCallExpr.Name` fields for imported and locally-defined functions to their qualified names.

### Return Values

- Functions returning `string`/`number`/`Array<T>`/`T[]`: emit `printf '%s'` to stdout; callers capture with `$(fn args)`
- Void functions: no capture; callers emit bare `fn args`
- Functions returning `status`: exit code via `$?`

### Types at Runtime

| Besht type | Shell representation                                              |
| ---------- | ----------------------------------------------------------------- |
| `string`   | shell string                                                      |
| `number`   | shell string containing digits; arithmetic via `$((...))`         |
| `boolean`  | `1` (true) / `0` (false); tested with `[ "$x" = 1 ]`              |
| `T[]` / `Array<T>` | newline-delimited string; nested array rows use unit-separator-packed lines |
| `Set<T>`   | newline-delimited unique string values for membership checks               |
| `status`   | exit code captured as `$?`                                        |
| `command`  | lazy pipeline description; no shell code until `.run()` is called |

Prefer native array APIs for new user-facing examples and compiler work: `items.length`, `items[0]`, `items.slice(1)`, `items.push(value)` or `[...items, value]`, `items.includes(value)`, `items.concat(other)`, `items.sort()`, `items.toSorted()`, and `items.toSpliced(...)`. The old global list helpers `len`, `head`, `tail`, `append`, `contains`, and `concat` remain supported for compatibility, but do not add new global list helpers.

### POSIX Compliance Invariants

- No `local` keyword (not POSIX)
- No bash arrays (`arr=()`)
- No `[[` double brackets
- No `$'...'` quoting
- No `{1..10}` brace expansion
- Arithmetic: `$(( expr ))` only
- Test: `[ ]` only (single bracket)
- Literal command args are either conservative shell-safe bare words or single-quoted

## Test Layout

```
internal/lexer/lexer_test.go       # Token types, keywords, errors, position tracking
internal/parser/parser_test.go     # Every AST node type; error recovery
internal/semantics/validator_test.go # Semantic validation, scope, builtins/surfaces
internal/codegen/codegen_test.go   # Unit: AST → sh output patterns (uses Generate())
internal/codegen/integration_test.go # E2E: temp files → CompileFile() → sh output
internal/viewer/viewer_test.go     # Side-by-side visualization renderer behavior
```

`node-eq/tests/` is organized by fixture purpose: `advent/`, `commands/`, `imports/`, `language/`, and `regressions/`. Focused API fixtures live under their language subdirectories, including `node-eq/tests/language/json/` for `JSON.parse()`/path/extraction/`JSON.stringify()` parity coverage. Run it recursively with `bun node-eq/compare $(rg --files -g '*.bsh' node-eq/tests | sort)`. Fixtures that need non-default compiler flags may include a top-of-file `// besht-compile-flags: ...` directive; the compare runner applies those flags only to that fixture. Keep imported fixture dependencies beside their importing `.bsh` files unless the import paths are updated in the same change.

Tests use `go test ./...`. Coverage target: `make cover`. Current coverage: ~75%.

## Syntax Reference (current)

```ts
// Variable declaration — type annotations optional everywhere
let name: string = "Alice"          // plain literal — " and ' produce no interpolation
let also: string = 'Alice'          // same — both quote styles are plain literals
let tmpl: string = `Hello ${name}!` // template literal — ${var} interpolation
let pattern: string = '^foo-[0-9]+$'  // single-quoted literal text
let path: string = "C:\\temp\\new\\file.txt" // escape backslashes in double-quoted strings
let escape: string = "newline:\n tab:\t backslash:\\ quote:\" dollar:\$"  // escape sequences
let unicode: string = "A \u0041 ñ \u00F1"  // unicode escapes
let count: number = 42
let price: number = 3.14          // float literal — compiled to awk arithmetic
let flag: boolean = true
let files: string[] = ["a.txt", "b.txt"]
let rows: string[][] = [["a", "b"], ["c", "d"]]
let seen = new Set<string>()

// Union and tuple types (annotations only — ignored by compiler)
let maybeValue: string | null = "present"
let nothing = null
let tuple: [string, number] = ["age", 30]
const [tupleName, tupleAge] = tuple
let absent = undefined

// Without type annotations — identical behavior
let city = "Paris"
let items = ["x", "y"]

// Array types. Prefer T[] or Array<T>.
let a: string[] = ["a", "b"]
let b: Array<string> = ["c", "d"]
let c: string[] = ["e", "f"]
let matrix: string[][] = rows.map(row => row.join("").split("") as string[])
let indexes: number[] = Array.from({ length: 3 }) // [0, 1, 2]; Besht numeric range, not JS undefined slots
let selected: string[] = Array.of("a", "b") // ["a", "b"]
let objectKeys: string[] = Object.keys(user) // compiler-managed object key array, not general JS reflection
let objectValues: string[] = Object.values(user) // object value array
let objectEntries: string[][] = Object.entries(user) // packed [key, value] rows
let objectHasName: boolean = Object.hasOwn(user, "name")
let objectCopy: object = Object.assign({}, user, { active: false })
let objectFromEntries: object = Object.fromEntries(objectEntries)
let objectSpread: object = { ...user, active: false }
let parsed: JSONValue = JSON.parse("{\"user\":{\"name\":\"Ada\"}}") // requires --opt-use-jq and jq
let parsedName: string = parsed.user.name
let jsonUser: string = JSON.stringify(user) // scalar Besht/object values only; requires --opt-use-jq and jq
let jsonArray: string = JSON.stringify(["a", "b"])

// Constants (compile-time immutability)
const MAX: number = 100
const APP: string = "myapp"

// Reassignment
count = count + 1
count += 2

// Postfix increment/decrement (statement position only)
count++
count--

// Prefix increment/decrement (expression position)
let next = ++count
let prev = --count

// Logical operators
let active: boolean = true && !false
let either: boolean = active || false
let negated: boolean = !active
let fallback: string = maybeValue ?? "default" // nullish only; preserves "", 0, false

// Strict equality (same as == / != — no type distinction in shell)
let same: boolean = x === y
let diff: boolean = x !== y
let sameBlock: boolean = output === `a
b` // multiline-safe string equality

// Ternary
let bigger: number = a > b ? a : b
let staticLabel: string = true ? "yes" : "no" // compiles to the selected value

// Environment variables
let home: string = process.env.HOME
let port: string = process.env.PORT ?? "8080"
let nodeHome: string = process.env.HOME
let nodePort: string = process.env.PORT ?? "8080"

// Script arguments
let argv: string[] = Besht.args.argv()
let input: string = Besht.args.positional(1) ?? "-"
let branchArg: string = Besht.args.option("branch", "b") ?? "main"
let dryRun: boolean = Besht.args.flag("dry-run", "d")

// Fetch text-only GETs. Synchronous and curl-backed (`curl -sS -- <url>`).
let body: string = fetch(url).text()
let response = fetch(url) // runs curl once at assignment time
let bodyAgain: string = response.text() // reuses stored response text
// Deferred: await, options/POST/headers/body, json(), status, ok, headers.

// Type conversion
let s: string = count.toString()
let b: string = flag.toString() // true or false
let label: string = String(["a", "b"]) // "a,b"
let n: number = Number.parseInt(s)
let n10: number = Number.parseInt(s, 10)
let aliasN: number = parseInt(s, 10)
let truthy: boolean = Boolean(s)

// Boolean(value) returns a primitive boolean using JS-like truthiness for current Besht values.
// It does not create Boolean object wrappers or add runtime type metadata.
let label: string = "check:" + name
let msg: string = `Hello, ${name}!`  // template literal for interpolation
let sum: string = `sum=${a + b}`
let sum: string = `sum=${a + b}`     // full expressions inside ${...}

// Number builtins
let pi: number = Number.parseInt("42")
let pi10: number = Number.parseInt("42", 10)
let pf: number = Number.parseFloat("3.14")
let gpi: number = parseInt("42", 10)
let gpf: number = parseFloat("3.14")
let fin: boolean = Number.isFinite(pf)
let isInt: boolean = Number.isInteger(pi)
let safe: boolean = Number.isSafeInteger(pi)
let nan: boolean = Number.isNaN(pi) // always false for current besht values
let maxSafe: number = Number.MAX_SAFE_INTEGER
let minSafe: number = Number.MIN_SAFE_INTEGER
let eps: number = Number.EPSILON

// Functions
function greet(name: string): string {
    return "Hello, ${name}!"
}

// Exported function
export function info(msg: string) {
    $("printf", "[INFO] %s\\n", msg).stderr("&1").run()
}

// If/else — parens required around condition; bodies can be braced or a single statement
if (count > 0) {
    $("echo", "pos").run()
} else if (count == 0) {
    $("echo", "zero").run()
} else {
    $("echo", "neg").run()
}
if (count < 0) return "negative"
else console.log("non-negative")

// Switch/case — compiles to shell case/esac
switch (mode) {
    case "dev":
        message = "development"
        break
    case "prod":
        message = "production"
        break
    default:
        message = "unknown"
        break
}

// While — parens required
while (count > 0) {
    count = count - 1
}
while (count > 0) count--

// C-style for loop (TypeScript syntax)
for (let i: number = 0; i < 10; i++) {
    $("echo", i.toString()).run()
}
for (let i: number = 0; i < 10; i++) total += i

// Bare init (no let) — i implicitly declared in loop scope
for (i = 0; i < 10; i++) {
    $("echo", i.toString()).run()
}

// For range — iterate integers
for (i in Besht.iter.range(1, 10)) {
    $("echo", i.toString()).run()
}
for (i in Besht.iter.range(1, 10)) total += i

// For array
for (f in files) {
    $("echo", f).run()
}
for (f in files) $("echo", f).run()

// Declaration form
for (let f in files) {
    $("echo", f).run()
}

// TypeScript for...of is accepted as the same value-iteration array loop.
for (const f of files) {
    $("echo", f).run()
}

// Break and continue
for (f in files) {
    if (Besht.strings.isEmpty(f)) { continue }
    if (f == "stop") { break }
    $("echo", f).run()
}

// Array indexing (0-based)
let first: string = files[0] // static scalar array indexes fold to constants when known
let item: string = files[i]
let cell: string = matrix[row][col] // static nested array indexes fold to constants when known
let letter: string = "abc"[1] // static ASCII string indexes fold to constants when known
let maybeName: string = user?.name ?? "anonymous"
let maybeItem: string = items?.[i] ?? "fallback"
let maybeCell: string = matrix?.[row]?.[col] ?? "missing"
let maybeTrimmed: string = maybeText?.trim() ?? ""
let width: number = matrix[0].length

// Array index assignment
items[1] = "BETA"

// Empty array
let empty: string[] = []

// Object literals — compile to per-property shell variables
let user = {
    id: 1,
    name: "Victor",
    active: true
}

// Property access and assignment
let userName: string = user.name
user.name = "Compiler Tester"

// Computed property access and assignment (dynamic keys)
let key: string = "name"
let val: string = user[key]         // reads _obj_user_${key}
user[key] = "Updated"              // writes _obj_user_${key}
let keys: string[] = Object.keys(user) // ["id", "name", "active"]
let values: string[] = Object.values(user)
let entries: string[][] = Object.entries(user)
let hasName: boolean = Object.hasOwn(user, "name")
let copy = Object.assign({}, user)
let rebuilt = Object.fromEntries(entries)
let spreadCopy = { ...user, active: false }

function showKeys(obj: object): string[] {
    return Object.keys(obj)
}

// Classes — constructors, instance properties/methods, getters/setters, new, this
class User {
    name: string
    age: number

    constructor(name: string, age: number) {
        this.name = name
        this.age = age
    }

    greet(): string {
        return "Hello, " + this.name
    }

    get label(): string {
        return this.name + " (" + this.age.toString() + ")"
    }

    set label(value: string) {
        this.name = value
    }
}

let u = new User("Alice", 30)
console.log(u.greet())
console.log(u.name)
console.log(u.label)
u.name = "Bob"
u.label = "Carol"

// Static properties and methods
class MathUtils {
    static PI: number = 3.14159
    static get label(): string {
        return "math"
    }
    static round(n: number): number {
        return Math.round(n)
    }
}
console.log(MathUtils.PI)
console.log(MathUtils.label)
console.log(MathUtils.round(2.7))

class Game {
    readonly matrix: string[][]
    private static Deltas: Record<string, [number, number]> = { U: [-1, 0] }
}

// console.log() and console.error() builtins
console.log("Hello, " + name)
console.error("Something went wrong")

// $() creates a lazy Command object — no shell code until .run() is called
// Declaring a command without ever calling .run() → warning + no code emitted

// Side-effect commands: .run() emits bare shell, no capture variable
$("chmod", "+x", "script.sh").run()
$("git", "add", ".").run()

// Capture stdout: .run() followed by .readStdout() → compiler emits capture variable
let userCmd = $("whoami")
userCmd.run()
let user: string = userCmd.readStdout()      // reads from captured var

// Chain style (same result):
let branch = $("git", "rev-parse", "--abbrev-ref", "HEAD").run().readStdout()

// Spread command arguments
let args: string[] = ["-n", "hello"]
$("echo", ...args).run()

// Capture as lines: .run() + .readStdoutLines()
let logCmd = $("git", "log", "--oneline", "-20")
logCmd.run()
let log_lines: string[] = logCmd.readStdoutLines()

// Pipeline — built lazily before run()
let result: string = $("cat", "/etc/passwd")
    .pipe($("grep", "root"))
    .pipe($("cut", "-d:", "-f1"))
    .run()
    .readStdout()

// Redirect — still lazy until .run()
$("make", "build").stdout("/tmp/build.log").run()
$("echo", "more").stdout("/tmp/out.txt", "append").run()
$("make", "build").stderr("null").run()     // 2>/dev/null
$("make", "build").stderr("&1").run()       // 2>&1
let errors = $("make").run().readStderr()  // stderr only
$("make", "build").env("CI", "1").run()     // CI=1 make build
let root = $("pwd").workdir("/").run().readStdout() // run one command from /
$("make", "test").workdir("/repo/app").run() // parent script cwd is unchanged

// Spread an array into command arguments
let args: string[] = ["-n", "hello"]
$("echo", ...args).run()

// exitCode — captured only when used
let makeCmd = $("make", "build")
makeCmd.run()
let code = makeCmd.exitCode()   // 0 or non-zero

// clone() — to run the same pipeline more than once
let cmd = $("whoami")
cmd.run()
cmd.run()           // ← COMPILE ERROR: already run; use clone()
cmd.clone().run()   // ← correct

// Process exit
process.exit()
process.exit(7)
process.exit(code) // number or status

// Try/catch
try {
    $("rsync", "-az", src, dest).run()
} catch (code: status) {
    console.log("failed: " + code.toString())
    process.exit(1)
}

// Error propagation
function read_file(path: string): string {
    let content = $("cat", path).run().readStdout()?
    return content
}

// Besht helper namespaces
if (Besht.fs.isFile(path)) console.log("file")
if (Besht.fs.isDir(path)) console.log("dir")
if (Besht.fs.isReadable(path)) console.log("readable")
if (Besht.fs.isWritable(path)) console.log("writable")
if (Besht.fs.isExecutable(path)) console.log("executable")
let empty: boolean = Besht.strings.isEmpty(value)
let set: boolean = Besht.strings.isNonEmpty(value)

// Math object methods
let circle: number = 2 * Math.PI
let mn: number = Math.min(a, b)     // dynamic args → awk comparison
let mx: number = Math.max(a, b)
let r: number = Math.round(3.7)     // static literal → 4
let fl: number = Math.floor(3.9)    // static literal → 3
let cl: number = Math.ceil(3.1)     // static literal → 4
let tr: number = Math.trunc(3.9)    // static literal → 3
let ab: number = Math.abs(-5)       // static literal → 5
let sg: number = Math.sign(-5)      // static literal → -1
let pw: number = Math.pow(2, 8)     // static literals → 256
let sq: number = Math.sqrt(16)      // static literal → 4
let ns: string = count.toString()
let bs: string = flag.toString()    // true or false
let fixed: string = price.toFixed(2)

Reassignment updates float metadata from the new right-hand side: float-producing values keep later arithmetic on `awk`, while integer/non-float reassignment clears the float marker so later integer arithmetic can lower back to `$((...))` when applicable.

// Negative numbers
let neg: number = -42
let negf: number = -1.5

// String methods
let trimmed: string = name.trim()
let leftTrimmed: string = name.trimLeft()
let rightTrimmed: string = name.trimRight()
let upper: string = name.toUpperCase()
let lower: string = name.toLowerCase()
let parts: string[] = name.split(",")
let chars: string[] = name.split("")
let r: string = name.replace("old", "new")
let ra: string = name.replaceAll("a", "b")
let slen: number = name.length
let sw: boolean = name.startsWith("Hello")
let swFrom: boolean = name.startsWith("World", 7)
let ew: boolean = name.endsWith("!")
let ewLen: boolean = name.endsWith("Hello", 5)
let has: boolean = name.includes("World") // emits _bst_includes helper only when this one-arg string helper is used
let hasFrom: boolean = name.includes("World", 4)
let last: number = name.lastIndexOf("l")
let lastFrom: number = name.lastIndexOf("l", 8)
let firstFrom: number = name.indexOf("l", 3)
let sub: string = name.slice(0, 5)
let sub2: string = name.substring(0, 5)
let ch: string = name.charAt(1)
let cmp: number = name.localeCompare("zzz")

// Array methods. Prefer these native APIs over the older global helper aliases.
let flen: number = files.length
let first: string = files[0]
let rest: string[] = files.slice(1)
let pushed: string[] = files.push("new.txt")
let alsoPushed: string[] = [...files, "new.txt"]
let prepended: string[] = files.unshift("first.txt")
let popped: string[] = files.pop()
let joined: string = files.join(", ")
let arrayText: string = files.toString() // scalar arrays only; same as files.join(",")
let merged: string[] = files.concat(other)
let rev: string[] = files.reverse()
let revCopy: string[] = files.toReversed()
let sorted: string[] = files.sort()
let sortedCopy: string[] = files.toSorted()
let splicedCopy: string[] = files.toSpliced(1, 1, "new")
let filled: string[] = files.fill("x", 1, -1)
let compactText: string = ["a", "b"].concat(["c"]).join(",") // static chains compile to 'a,b,c'
let copied: string[] = [...files, "extra"]
let indexes: number[] = Array.from({ length: 3 })
let selected: string[] = Array.of("a", "b")
let mapped: string[] = files.map(f => f + ".bak")
let chars: string[] = files.flatMap(f => f.split(""))
let labeled: string[] = files.map((f, i) => i.toString() + ":" + f)
let classified: string[] = files.map((f, i) => {
    if (i == 0) return "first:" + f
    return "next:" + f
})
files.forEach((f, i) => console.log(i.toString() + ":" + f))
let filtered: string[] = files.filter(f => f.endsWith(".txt"))
let hasTxt: boolean = files.some((f, i) => f.endsWith(".txt") && i >= 0)
let allNamed: boolean = files.every(f => f.length > 0)
let firstTxt: string = files.find(f => f.endsWith(".txt")) ?? "missing"
let foundIndex: number = files.findIndex(f => f == "b.txt")
let lastTxt: string = files.findLast(f => f.endsWith(".txt")) ?? "missing"
let lastTxtIndex: number = files.findLastIndex(f => f.endsWith(".txt"))
let reduced: number = nums.reduce((acc, n) => acc + n, 0)
let reversed: string = nums.reduceRight((acc, n) => acc + n.toString(), "")
let lines: string = nums.reduce((acc, n) => [...acc, "#".repeat(n)], [] as string[]).join("\n")
let counts = words.reduce((acc, word) => {
    acc[word] = (acc[word] || 0) + 1
    return acc
}, {})
let sliced: string[] = files.slice(1, 3)
let found: boolean = files.includes("a.txt") // array membership via grep -qxF; does not emit _bst_includes
let idx: number = files.indexOf("b.txt")
let lastIdx: number = files.lastIndexOf("b.txt")
// Old global list helper names remain supported for now: len, head, tail, append, contains, concat.

// Set<T> collections — newline-backed, no runtime type checks
let visited = new Set<string>()
visited.add("0,0")
let wasSeen: boolean = visited.has("0,0")

// Imports
import def from "./lib/log"
import { info, error, cmd } from "./lib/log"
import { legacy_log } from "./legacy.sh" assert { type: "shell" }
$(...cmd).run()
$(...def).run()
legacy_log("from shell")
// Mixed command-name spread is rejected: $(...cmd, "extra")

// Declaration file
declare function external_name(name: string): string
// Run `besht init` to generate entry-directory stdlib.d.bsh declarations.
// That file is auto-loaded for compile, split compile, and --check.

// Type aliases and interfaces — parsed and silently ignored
type Status = "active" | "inactive"
type Result<T> = { ok: true; value: T } | { ok: false; error: string }
interface Config { host: string; port: number }
export type Callback = (data: string) => void
export interface Repository { findById(id: number): Result<string> }

// Iterate command output
for (line in $("find", "/var/log", "-name", "*.log").run().readStdoutLines()) {
    $("echo", line).run()
}
```

## Adding a New Builtin

Builtins are compile-time functions with no runtime overhead. To add one:

1. Add name to `IsBuiltin()` in `ast/ast.go`
2. Add case in `checkBuiltinCall()` in `semantics/validator.go`
3. Add case in `genBuiltinCapture()` (value position) in `codegen/codegen.go`
4. If usable as a statement, add case in `genBuiltinStmt()` in `codegen/codegen.go`
5. If usable as a condition, add case in `genBuiltinCondition()` in `codegen/codegen.go`

## Adding a New Command Method

Command methods chain on `command` type values. With the lazy Command model:

1. Add case in `checkCommandMethodArity()` in `semantics/validator.go` — specify arg count and return type
2. If the method is a **terminal** (causes execution or capture): handle in the Command Analysis pass so it's accounted for in the emit decision
3. Add case in `genCmdChain()` in `codegen/codegen.go` — emit the shell equivalent
4. If the method records a capture (like `.readStdout()` or `.exitCode()`), codegen usually reads from the pre-assigned capture variable. Immediate anonymous `.run().readStdout()` / `.run().readStderr()` expressions may compile directly to command substitution; named command objects and `.exitCode()` must keep `genRunCall()` capture/exit-variable emission so reuse stays correct.

## Known Semantic Differences from TypeScript

README.md contains the user-facing TypeScript/Besht divergence table. Keep it synchronized with `skills/besht-scripting/SKILL.md` whenever a syntactically TypeScript-like form intentionally behaves differently in Besht, and add or reference tests for compile-enforced rows.

**Booleans are stored as `1`/`0` in shell variables.** In string contexts besht now renders them as `true`/`false`, but condition generation still relies on `1`/`0`.

**`includes()`, `startsWith()`, `endsWith()` still drive conditions via `1`/`0`, but string contexts now render them as `true`/`false`.**

**Float literals use awk for arithmetic.** `3.7 + 1.2` → `$(awk -v _a=3.7 -v _b=1.2 'BEGIN{print _a + _b}')`. `$((...))` is integer-only in POSIX sh.

**`Number.isNaN()` is always false for currently representable besht values.** Besht has no NaN runtime sentinel, so the API exists for JS-compatible syntax but can't observe NaN.

**`Boolean(value)` is a primitive coercion builtin only.** It returns Besht boolean `1`/`0` and relies on existing boolean string rendering for `true`/`false`. Keep the slice narrow: falsey values are `false`, `0`, `0.0`, `""`, `null`, and `undefined`; non-empty strings including `"0"`/`"false"`, non-zero numbers, arrays, objects, and sets are truthy. Do not add Boolean object wrappers, `new Boolean`, `Boolean.parse`, or runtime type metadata for this API.

**`String(value)` is primitive stringification only.** It does not add `new String(...)`, string wrapper objects, `String.raw`, or other static `String.*` APIs. Keep it aligned with JavaScript display values for current Besht representations: null/undefined, booleans, numbers, strings, scalar arrays, compiler-managed objects, and Sets. Do not fold object-producing calls if doing so would skip mutations or object metadata updates.

**`Array.isArray(value)` is a compiler-known representation predicate, not runtime shape inspection.** A generic or `unknown` function parameter passed an array still returns false unless the parameter is annotated or inferred as a Besht array representation.

---

## Common Pitfalls for Agents

**Never emit `local` in generated shell output.** Not POSIX. Use `_fn_varname` mangling instead.

**Skill validation prompts must not leak Besht-specific answers.** When using subagents to evaluate `skills/besht-scripting/SKILL.md`, ask them only to read the skill and translate or write the requested `.bsh` artifact. Do not mention expected APIs such as `$()`, `.pipe()`, `.workdir()`, `.env()`, or `.exitCode()` in the prompt; those cues must come from the skill itself so the validation measures the skill rather than the evaluator.

**`rewriteFnCalls()` runs before codegen.** When adding new expression types that contain function calls, add a case to `rewriteStmt()`/`rewriteExpr()` in `modules.go` so the pass descends into them.

**`rewriteVarRefs()` must be called on `TemplateLit` values only.** Only template literals (`` `...${var}...` ``) contain `${...}` references that need var-name rewriting. Plain `StringLit` values are emitted as single-quoted sh and contain no shell expansions. Do NOT call `rewriteVarRefs()` on plain string values.

**`paramMap` is per-function scope.** Saved/restored on function entry/exit in both `genFnDecl` and `genModuleFnDecl`. Loop vars and catch vars must be registered into `paramMap` and cleaned up.

**`genCmdChain()` is recursive.** It walks `.pipe().pipe().stdout()` chains by recursing on the receiver. The base case is `*ast.CmdExpr`. Terminal methods must handle any redirect string accumulated from inner calls.

**`\$` in regular strings produces a literal dollar sign.** `"price \$5"` → `"price \$5"` in sh (POSIX: `\$` in double-quotes is literal). Do NOT rely on `$` alone being literal — use `\$` explicitly.

**Compile-time warning for `-`-prefixed string literals in `$()` args.** If a string literal starting with `-` containing special characters (`$`, `^`, `[`, `]`, etc.) is passed as a `$()` argument, the compiler warns and suggests adding `-e`/`--` before it. Suppressed when the preceding argument is `-e` or `--`.

**Runtime POSIX self-check.** Compiled scripts emit a `_r=$(printf 'hello:world' | grep -F 'hello' | sed ...)` pipeline at the top of the preamble only when generated shell uses Besht's `grep`/`sed`-based paths or the args runtime. Simple scripts that only need shell builtins and direct `printf` output skip it. In split output, the entry script emits the check if any generated module needs it. If the result is wrong the script exits immediately. Omit with `--opt-no-add-binaries-check` at compile time.

**`string.repeat(n)` uses awk.** The old `printf '%0.s'` approach emitted nothing. The correct implementation loops in awk: `awk -v _s=STR -v _n=N 'BEGIN{r=""; for(i=0;i<_n;i++) r=r _s; printf "%s",r}'`.

**`Math.round()` matches JS semantics: `floor(x+0.5)`.** This rounds half toward +∞ (so `round(-2.5) = -2`). The naïve `int(x + 0.5)` is wrong for negatives. The correct awk formula is `_y=_x+0.5; print int(_y)-((_y<int(_y))?1:0)`.

**`inferReceiverType()` is recursive.** It handles `BinaryExpr`, `MethodCallExpr`, and literal nodes — not just `IdentExpr`. This is required so chained `+` concatenations like `("a" + x) + "!" + n` correctly infer string type at every level.

**`strInner()` always uses `${var}` notation.** When extracting the inner content of a double-quoted string like `"$a"` for concatenation, `strInner` returns `${a}` not `$a`. This prevents `"$a" + "bc"` from producing `"$abc"` (wrong variable name). Never revert this to `$var`.

**`genArgs()` applies `ensureArgSafe()` to all generated args.** Any `$(...)` expression is automatically wrapped in `"$(...)"` when passed as an argument to a command, `console.log`, or a function call. This prevents word-splitting of multi-word command output.

**Command literal words prefer natural bare output only when safe.** `genCmdArgs()` uses `cmdArgWordForExpr()` so ordinary string literals such as `"git"` and `"--short"` can emit as `git --short`, while globs, spaces, embedded quotes, variables, command substitutions, shell reserved command names, and command-position assignments remain quoted or protected. Do not route general string/assignment/array emission through this command-word path.

**`escapeForDoubleQuote()` handles shell-active characters in string literals.** When emitting a double-quoted sh string, backtick `` ` `` and every literal dollar form must be escaped, including `$(`, `$WORD`, `$1`, `${text}`, `$*`, `$?`, and `$$`. Besht template interpolation is inserted separately from parsed `${expr}` nodes by `genTemplateLiteral()`/`strInner()`; do not treat literal template text as shell syntax. `\$` (already escaped by the lexer for literal dollars) must also be left as-is. Any new string emission path must call `escapeForDoubleQuote()` before wrapping in `"..."`.

**`cmdArgQuote()` must not double-quote already-quoted values.** Plain string literals emit single-quoted strings from `genExprRHS` (e.g. `'s/foo/bar/'`). If `cmdArgQuote` then calls `shellQuote()` on that value again, it produces malformed shell like `'''s/foo/bar/'''`. The fix: `cmdArgQuote` short-circuits on values that already start and end with `'`. Values starting with `$` are passed raw as variable references. Complex expressions that happen to start with `$` skip quoting — be aware.

**Command `.env(name, value)` prefixes one command invocation only.** It emits `NAME=value command ...` inside the generated pipeline and does not mutate the parent shell environment.

**Command `.workdir(path)` changes cwd for one command or pipeline only.** It wraps the generated command in a subshell-style `cd path && ...` expression so subsequent statements keep the parent shell cwd. Keep redirects inside the workdir wrapper when generating command text.

**node-eq command parity should cover idiomatic command APIs.** `node-eq/runtime.ts` mirrors command `.env()` and `.workdir()`. `node-eq/run-bsh` should not translate syntax that Besht itself rejects; fixtures use ordinary quoted strings for glob, grep, and sed patterns. `node-eq/tests/commands/skill_pipeline_idioms.bsh` is paired with a source shell script to keep future skill iterations focused on pipes, redirects, workdir, env, quoted patterns, and exit-code gating.

**Skill validation should cover native data idioms, not only commands.** `node-eq/tests/language/callbacks/skill_native_data_idioms.bsh` is paired with a shell source that uses `sed`/`grep`/`tr`/`awk` over literal data. The Besht fixture intentionally uses native arrays, callbacks, string transforms, joins, numeric reduction, and indexed `forEach`; keep this as a guardrail for teaching agents to translate in-memory shell pipelines into Besht data operations instead of process pipelines.

**Skill validation should cover script interface idioms.** `node-eq/tests/commands/skill_args_env_predicates.bsh` is paired with a shell source that reads options, flags, positionals, environment defaults, file predicates, and string predicates. The Besht fixture intentionally uses `Besht.args.option()`/`.flag()`/`.positional()`, `process.env.NAME ?? fallback`, `Besht.fs.*`, and `Besht.strings.*`; keep it as a guardrail against agents transliterating ordinary shell option parser loops or `[ -f ]`/`[ -z ]` checks.

**Skill validation should cover static record idioms.** `node-eq/tests/language/objects/skill_object_data_idioms.bsh` is paired with a shell source that uses `awk -F:`, `cut`, `paste`, and membership probes over a literal colon-delimited table. The Besht fixture intentionally uses object literals, array callbacks, dynamic object property reads, `Object.hasOwn()`, and `JSON.stringify()`; keep it as a guardrail against agents preserving text-processing pipelines for static in-memory records.

**JSON object values must preserve expression types.** `JSON.stringify({ count: items.length })` must pass `items.length` to jq as JSON number data, not as a string. Keep `inferReceiverType()` aware that `.length` on strings and arrays is numeric so object-literal JSON codegen chooses `--argjson`.

**JSONValue is compact JSON text, not an object/array mirror.** `JSON.parse()` returns internal `TypeJSON`/user-facing `JSONValue`; property and index access on it must stay jq-backed and return another `JSONValue`. Missing final JSON fields, out-of-range array indexes, and JSON `null` must become the Besht nullish sentinel for `??`; accessing through a missing/null intermediate should fail unless optional chaining short-circuits it. String/number/boolean extraction happens only when the JSON-backed expression is annotated or asserted with that target type, and non-JSON annotations remain erased. Keep generated JSON reads routed through shared runtime helpers: `_bst_json_canonical()` owns common jq validation/compaction, `_bst_json_get_prop()`/`_bst_json_get_index()` own path reads, `_bst_json_scalar()` owns common scalar handling, and the one-per-type cell helpers (`_bst_json_cell_string()`, `_bst_json_cell_number()`, `_bst_json_cell_boolean()`) own type-specific jq predicates. Do not reintroduce inline jq scalar/path programs at every JSON read site.

**`command` objects do not auto-coerce.** Unlike the old model, `command` no longer coerces to `string` on assignment. You must explicitly call `.run()` then `.readStdout()` to get a string. The Command Analysis pass enforces this.

**`run()` returns `self`, not `void`.** This enables chaining: `$("whoami").run().readStdout()`. Do not declare `.run()` as returning `void` anywhere in the compiler — it returns the same `Command` identity.

**The Command Analysis pass runs before codegen.** It assigns a unique identity (integer) to every `CmdExpr` node in the AST, traces which variable names hold each identity through assignments, and for each identity records: which line `run()` was called (if any), and whether `.readStdout()`, `.readStdoutLines()`, `.readStderr()`, or `.exitCode()` are used. Codegen then uses this map to decide what shell code to emit for each command. If a command has no `run()` call, emit nothing and warn. If it has `run()` plus `.readStdout()` usage, emit `varname=$(...)`. If it has `run()` but no capture methods, emit a bare shell command.

**Module generator vs plain generator.** `moduleGenerator` embeds `Generator`. When adding new statement types, add a case to `genModuleTopStmt()` if they can contain function calls that need qualification.

**Semantics cross-module signatures and values.** `Compiler.load()` builds a `globalSigs` map of exported signatures and a `globalVars` map for exported top-level values. When adding new exported constructs, register them there.

**Object literals compile to per-property shell variables.** `let user = { id: 1, name: "Victor" }` emits `_obj_user_id=1` and `_obj_user_name="Victor"`. Property access `user.name` reads `_obj_user_name`. Property assignment `user.name = "X"` writes `_obj_user_name="X"`. Since shell variables are global, these `_obj_*` variables are accessible inside functions. When an object is passed as a function parameter, `genProperty` uses `objPropTypeMap` (populated by `collectObjectTypes` pre-pass) to resolve the original `_obj_` prefix directly — no copying needed. Inside a function, `student.name` emits `$_obj_student_name` (using the original top-level variable name, not the mangled parameter name).

**`Object.keys(obj)`, `Object.values(obj)`, `Object.entries(obj)`, `Object.hasOwn(obj, key)`, `Object.assign(target, ...sources)`, and object spread use `_objkeys_*` metadata.** Object literal bindings, aliases, reduce object accumulators, class instance slots, static object maps, dot property assignments, computed property assignments, scalar-safe `Object.assign()` copies, and object spread copies must keep `_objkeys_<object>` in sync. `Object.keys()` lowers to a newline-delimited list from that metadata, `Object.values()` evals each keyed scalar `_obj_*` slot in the same order, and `Object.entries()` emits existing nested-array unit-separator packed `[key, value]` rows for scalar values. `Object.assign()` must use the reusable object-copy lowering: validate target/source object slots before `eval`, loop over source `_objkeys_*`, validate each key, copy the current scalar slot value, append missing target keys, preserve first-seen key order, and let later sources overwrite values. Object spread lowers to a fresh object identity plus the same copy lowering, with explicit properties represented as literal source chunks so `{ ...a, x: 1, ...b }` preserves left-to-right order and overwrite behavior. Inline object-literal targets such as `{}` create a fresh object identity; named `Object.assign()` targets are mutated and returned; assignment to an existing variable rebinds that variable to the returned object identity. Unmutated named object `Object.keys()`, static-scalar `Object.values()`/`Object.entries()`, static-key `Object.hasOwn()`, literal-only `Object.assign({}, ...)`, and literal-only object spread calls may fold from `staticObjectMap`, `staticObjectEntryMap`, and `staticObjectValueMap`; any property assignment or dynamic copy into that static object must invalidate those maps so later reads use runtime metadata. Statically known boolean values format as `true`/`false` when enumerated through `Object.values()` or `Object.entries()`, while the stored shell slots remain `1`/`0`. `Object.values()`, `Object.entries()`, `Object.assign()`, and object spread must reject statically known array/object/set/command/fetch values because the current `string[]`, packed `string[][]`, and scalar object slot representations cannot preserve deeper nested values without corrupting rows. These helpers and object spread must not emit a runtime helper. `Object.hasOwn()` checks exact line membership against that metadata, returns `1`/`0`, and returns false for dynamic keys that fail `[A-Za-z0-9_]+` instead of mutating metadata or exiting. Static and computed object keys must match `[A-Za-z0-9_]`; computed assignments, assign copies, and spread copies append only validated keys. The `object` annotation parses to `TypeObject` so function parameters and object-returning functions can carry object slot names for Object helpers and object-spread returns. `process.env` is intentionally not enumerable; reject `Object.keys(process.env)`, `Object.values(process.env)`, `Object.entries(process.env)`, `Object.hasOwn(process.env, key)`, `Object.assign(process.env, ...)`, and `{ ...process.env }` in `--check` and compile paths. `Object.assign()` also rejects class instances, `this`, `const` targets, and unsupported scalar surfaces for now; object spread rejects class instances, `this`, `process.env`, and unsupported scalar surfaces as spread sources.

`Object.fromEntries(entries)` shares the same `_objkeys_*` object metadata boundary but creates a fresh object instead of mutating a target. Static literal rows and static `Object.entries(obj)` output may fold to object slots. Dynamic rows must be scalar `[key, value]` pairs, validate each key before `eval`, append only first-seen keys to `_objkeys_*`, and let later rows overwrite the property slot value.

**Classes use compiler-managed instance slots.** `let u = new User("Alice")` stores `u='u'`, calls `User__constructor "$u" ...`, and writes instance fields as `_obj_u_name`. `this.prop` inside constructors and methods uses tightly controlled `eval` to construct `_obj_${slot}_${prop}` names; user values must flow through temporary shell variables, never interpolated directly into the eval string. Static properties use `_class_<Class>_<prop>` and static/instance methods compile to `<Class>__<method>` shell functions. Explicit getters/setters compile to `<Class>__get_<name>` and `<Class>__set_<name>`; property reads/writes lower to those calls when present. Accessors cannot share a property name with a field, and source methods named `get_<name>`/`set_<name>` conflict with the corresponding accessor. TypeScript-only modifiers (`private`, `public`, `protected`, `readonly`) are accepted and ignored.

**Static object maps use object backing.** `static Deltas: Record<string, [number, number]> = { U: [-1, 0] }` emits `_obj__class_Class_Deltas_U` storage, and `Class.Deltas[key]` lowers to computed object access. `Record<K, V>` is annotation-only; it guides compiler inference but adds no runtime type checking.

**Class methods and getters that mutate `this` must be void-style calls.** Value-returning methods and getters are invoked through command substitution (`$(Class__method "$slot")`), which runs in a subshell. Any `this.prop = value` mutation inside that subshell is lost, so semantics/codegen reject non-void methods and getters that assign to `this` properties. Constructors and setters are exempt because they are called directly.

**Optional string search positions are normalized in awk.** `indexOf`/`includes`/`startsWith` position, `lastIndexOf` position, and `endsWith` length must use `awkArg()` plus awk-side `int()` truncation and clamping. Do not strip quotes into shell arithmetic for these arguments.

**Escape sequences in double-quoted strings.** `\n`, `\t`, `\r`, `\\`, `\"`, `\'`, `\uXXXX` are processed by the lexer into their actual characters. Single-quoted strings do NOT process escapes — they are always literal.

**Postfix `++` and `--` are statement-only; prefix updates are expressions.** `count++` and `count--` compile to assignment statements. Prefix `++count` and `--count` are supported in expression position and return the updated numeric value. They must resolve through `paramMap` like other variable reads/writes.

**Arrow functions are callback-capable function values.** Direct array `.map(x => expr)`, `.map((x, i) => { return expr })`, `.flatMap(x => exprOrArray)`, `.filter((x, i) => truthyExpr)`, `.some((x, i) => truthyExpr)`, `.every((x, i) => truthyExpr)`, `.find((x, i) => truthyExpr)`, `.findIndex((x, i) => truthyExpr)`, `.findLast((x, i) => truthyExpr)`, `.findLastIndex((x, i) => truthyExpr)`, `.reduce((acc, cur) => { ... }, init)`, `.reduceRight((acc, cur) => { ... }, init)`, and statement-position `.forEach((x, i) => { ... })` callbacks are supported, and arrows can also be assigned to variables, passed to functions, called as `cb(value)`, returned from functions, and passed to array callback methods as stored callbacks. Parenthesized arrows may carry TypeScript-shaped return hints: `(x: string): string => x`. `ast.TypeFunction` is a representation hint only; do not add type mismatch checks. Non-capturing arrow value declarations lower to generated shell functions named `_bst_arrow_<binding>_<line>_<col>`, store that shell function name in the binding, and name params as `<function_name>_param_<param_name>`; never emit `local`. Capturing arrows lower to closure ids plus `_bst_closure_*` environment variables and are invoked through `_bst_call`.

Direct-arrow array callbacks keep the existing inline lowering and restrictions. `.map()` and `.flatMap()` callbacks may take `(item)` or `(item, index)` and block-bodied `return` emits one mapped value then continues the callback loop; supported block statements are `return`, `if`/`else`, and assignments. Do not splice generic expression statements into map/flatMap callback shell source — reject unsupported statements instead of treating expression values as commands. `.filter()`, `.some()`, `.every()`, `.find()`, `.findIndex()`, `.findLast()`, and `.findLastIndex()` use JavaScript-style truthiness and may take the same optional zero-based index parameter. `.some()` returns `1` on the first truthy callback result and `0` for an empty array; `.every()` returns `0` on the first falsey callback result and `1` for an empty array; `.find()` and `.findLast()` return the matching scalar element or `_BESHT_NULLISH_SENTINEL` so `??` fallbacks work. `.findLast()` and `.findLastIndex()` must evaluate callbacks from the end of the array and short-circuit on the first truthy result from that reverse traversal.

Stored callback values are invoked through `_bst_call` when the value may be a closure id. Array `.map()`, `.flatMap()`, `.filter()`, `.some()`, `.every()`, `.find()`, `.findIndex()`, `.findLast()`, `.findLastIndex()`, `.reduce()`, and `.reduceRight()` pass `(item, index)` or `(accumulator, current)` as shell arguments; one-parameter callbacks simply ignore the extra index. Statement-position `.forEach(storedCallback)` invokes the function directly in the current shell so outer assignments and `Set.add()` side effects persist. Direct value-position calls to generated Besht functions/function values use the `_BESHT_RETURN_SLOT` path instead of stdout command substitution, so `let x = cb(value)`, function-call arguments, template interpolations, conditions, and similar generated expressions preserve captured mutation. Keep declared external shell functions on stdout capture because they do not know the return-slot protocol. Stored `.reduce(callback, {})` and `.reduceRight(callback, {})` over compiler-managed object accumulators are supported for Besht callback values by passing the accumulator slot in the current shell.

Returned arrows that capture function-local variables use a generated closure body plus per-instance closure ids, so `makeCounter()`-style factories get independent mutable state. `genReturn` must sync captured temp variables back to `_bst_closure_${env}_<name>` before returning. Direct callback expressions inside array-producing methods have a delayed-expression prelude so calls such as `items.map(x => next() + x)` run `next()` inside the generated loop rather than once while building the shell source. Array-producing and predicate callback expressions used in value or condition position are pre-evaluated into temp vars with current-shell heredoc loops, so assignment, `Set.add()`, and returned-closure side effects persist after `.map()`, `.flatMap()`, `.filter()`, `.some()`, `.every()`, `.find()`, `.findIndex()`, `.findLast()`, and `.findLastIndex()` complete.

Module rewriting must stay lexical-scope aware: `rewriteFnCalls` qualifies real module function calls but must not qualify `cb(...)` when `cb` is a function parameter, `let` binding, destructured binding, loop variable, catch variable, or arrow parameter. Function identifiers used as values are resolved through `Generator.fnValueMap`, so local function values store `module__fn` in bundled output while imported value bindings remain handled by the import variable maps.

**`Array.from({ length })` is narrow by design.** It only supports an object literal with a numeric `length` field (including shorthand `{ length }`) and emits a zero-based numeric array. Do not broaden it to arbitrary iterables or mapper callbacks without adding parser/semantics/codegen coverage and docs.

**`of` is not a keyword.** Keep `of` tokenized as `TokIdent` so `Array.of()` and ordinary member/identifier parsing continue to work. `parseFor()` special-cases an identifier literal `of` only in the loop-header slot and lowers `for (x of xs)`, `for (let x of xs)`, and `for (const x of xs)` to the same `ast.ForStmt` shape as Besht's value-iteration `for...in`.

**`Array.isArray(value)` is static.** It returns true only when codegen can infer `value` as `TypeList`; otherwise it emits false. Do not add runtime array metadata or dynamic shape inspection for this API without a broader object/array representation design.

**Type assertions are erased.** `expr as Type` exists for TypeScript-compatible syntax and compiler type inference only. It emits the inner expression unchanged. This is especially useful for empty array accumulators such as `[] as string[]`.

**Tuple/array destructuring declarations are lowered to index reads unless the source is static.** `const [dr, dc] = pair` evaluates dynamic `pair` once into a temp and assigns each name from one-based shell line extraction. Static scalar array literals and variables bound to them emit direct assignments, with out-of-range static positions assigned `''`. Element type inference comes from the array/tuple annotation when available.

**Array literal spread is generic.** `[...items, extra]` expands the existing newline-delimited array representation and appends normal elements. It is used by reduce array accumulators and should remain an array literal transformation, not a reduce-specific special case.

**Array predicate callbacks short-circuit in current-shell heredoc loops.** `items.some(callback)`, `items.every(callback)`, `items.find(callback)`, `items.findIndex(callback)`, `items.findLast(callback)`, and `items.findLastIndex(callback)` use direct arrow callbacks with one item parameter or `(item, index)`. Keep callback params in `paramMap`, increment the optional index once per scalar element, and avoid pipeline loops when state must persist inside the generated predicate loop. `find()` and `findLast()` must require the nullish runtime helper and initialize their result to `_BESHT_NULLISH_SENTINEL`; no-match must compose with `??`. `findLast()` and `findLastIndex()` must preserve JavaScript reverse callback order and short-circuiting, so do not implement them as a forward scan that merely keeps the last match. Nested-array element decoding is not part of these scalar predicate methods yet.

**Array expression loops use value iteration and run in the current shell.** `for (const move in moves.split("") as string[])` and `for (const move of moves.split("") as string[])` use a heredoc-backed `while read`, not a pipeline, so `break` and assignments inside the loop persist. Keep `for...of` behavior in sync with the existing `for...in` array loop lowering.

**Optional chaining uses flags on existing postfix AST nodes.** `IndexExpr`, `PropertyExpr`, and `MethodCallExpr` have `Optional bool`; keep all AST walkers descending into their receivers, indexes, and arguments. Codegen emits POSIX shell that stores the receiver once, compares it with `_BESHT_NULLISH_SENTINEL`, and returns that same sentinel on short-circuit so `??` can distinguish nullish from `""`, `0`, and `false`. Optional chaining guards nullish receivers only; do not add runtime shape/type checks. General `fn?.()`, `obj.method?.()`, and optional assignment targets remain unsupported.

**String runtime helpers are lazy.** `_bst_starts_with`, `_bst_ends_with`, and `_bst_includes` definitions belong in the preamble only when the generated body actually calls those helpers. Generate the body first (or otherwise track helper use before assembling the preamble) for single-file, bundled, and split output. In bundled module output, emit the union of helpers needed by all modules near the top-level preamble. In split output, emit helpers per generated file only when that module body needs them. Preserve the conditional entry-file POSIX self-check behavior. Array `.includes()` is separate: it uses `grep -qxF` and must not mark `_bst_includes` as needed.

**Dynamic `items.join(sep)` and scalar `items.toString()` use awk, not `paste -sd`.** Multi-character separators for dynamic joins require awk since `paste -sd` only uses the first character of the delimiter. Static scalar array literals and variables bound to them fold to quoted constants when elements contain no newlines and the separator is static. Scalar array `toString()` reuses the same join lowering with `,` as the separator and does not implement JavaScript nested-array flattening for `string[][]` or packed rows. The dynamic generated join shape is: `awk -v s=', ' 'NR>1{printf s}{printf "%s",$0}'`.

**AWK `OFMT="%.17g"` is set in all arithmetic BEGIN blocks.** This matches JavaScript double-precision output for division results (e.g., `72.66666666666667` instead of `72.6667`).

**`collectObjectTypes` pre-pass runs before codegen.** It walks the AST to populate `objPropTypeMap` (mapping `"varName.propName"` → type) so that `genProperty` and `inferReceiverType` can resolve object property types even inside function bodies. Without this pre-pass, functions defined before object literals would have empty `objPropTypeMap` entries.

**`fnParamTypes` tracks function parameter types during codegen.** Set in `genFnDecl`/`genModuleFnDecl` from `param.Type` annotations. Used by `inferReceiverType` to determine that a function parameter like `scores: number[]` is an array, enabling `scores.length` and `scores[i]` to work correctly.

**`||` and `&&` in value position return actual values (JS semantics), not booleans.** `a || b` returns `a` if truthy, else `b`. `a && b` returns `b` if `a` is truthy, else `a`. This is different from condition position (used in `if`/`while`) which returns 1/0. Static value-position logicals with known left truthiness compile directly to the selected side; dynamic value-position logicals use a subshell with `_l=temp` capture to test the left side, then return the appropriate value.

**`??` uses an internal nullish sentinel for dynamic nullish values.** Do not lower it to `${var:-fallback}` because empty string, `0`, and `false` must be preserved. Only `null`, `undefined`, missing `args` values, missing `process.env.NAME` variables, and missing indexes in nullish-left position should trigger the fallback. Static nullish coalescing may bypass the sentinel branch only when the left side is provably nullish or provably non-nullish. `process.env.NAME ?? fallback` may use POSIX `${NAME-fallback}` only for safe static fallback words, because that expansion is unset-only and preserves empty strings. Variables assigned in control flow and optional/dynamic nullish sources must keep the runtime sentinel comparison.

**`process.env.NAME ?? fallback` must use unset-only detection.** For safe static fallback words, prefer compact `${NAME-fallback}`. Otherwise lower `process.env.NAME` with `${NAME+x}` and `_BESHT_NULLISH_SENTINEL`; never use `${NAME:-fallback}` for this API because an explicitly empty environment variable must be preserved.

**`Besht` is a standard namespace exemption.** Module rewriting must not qualify `Besht` as a class/function-like identifier. `Besht.fs.*`, `Besht.strings.*`, `Besht.args.*`, and `Besht.iter.*` are parser-level method calls on `Besht` groups. In condition position, file/string predicates must emit minimal quoted tests (`[ -f "$p" ]`, `[ -d "$p" ]`, `[ -r "$p" ]`, `[ -w "$p" ]`, `[ -x "$p" ]`, `[ -z "$s" ]`, `[ -n "$s" ]`) and must not introduce runtime helpers. Console formatting should treat these predicates as booleans and print `true`/`false`, not raw `1`/`0`.

**`Besht.args.*` helpers parse script arguments in generated POSIX sh.** `Besht.args.argv()` returns positional args only, `Besht.args.positional(n)` returns a 1-based positional value or nullish, `Besht.args.option(long, short?)` supports `--long=value`, `--long value`, and optional `-s value`, and `Besht.args.flag(long, short?)` returns boolean `1`/`0`. Keep defaults outside helpers via `??`.

**`--` stops args option parsing.** After `--`, values that look like options or flags are positional. For example, `script --branch=dev -- -d file` has `Besht.args.flag("dry-run", "d") == false` and positional args `-d`, `file`.

**`fetch()` is currently a narrow synchronous text-only builtin.** It returns internal `TypeFetchResponse`, supports only `fetch(url).text()` and `let response = fetch(url); response.text()`, and lowers to `curl -sS -- <url>`. Assigned responses run curl once and store stdout in `_obj_<response>_body`; aliases and reassignments copy or refresh that body slot. `.body` is internal and not a user-facing property. `codegen.CheckFile` enables a narrow fetch-surface validation pass so `--check` rejects unsupported response properties/methods like `.status` and `.json()` without annotation type checking. Do not add `await`, options, POST, headers, bodies, `.json()`, `.status`, `.ok`, or `.headers` without a separate design.

**Equality conditions bind operands before comparing.** `genBinaryCondition()` must assign both sides to temporary variables and compare `[ "$_bst_left" = "$_bst_right" ]` / `!=`. Direct `[ $(fn) = "multi\nline" ]` breaks POSIX `[ ]` argument parsing when strings contain spaces or newlines.

**`reduce()` and `reduceRight()` emit current-shell loops, not pipes.** Using `printf | while read` would create a subshell, causing `_obj_*` variable mutations inside the loop to be lost. Forward `reduce()` keeps the heredoc `while read; do ...; done << EOF` pattern; `reduceRight()` first stages receiver items into indexed shell variables, then walks those indexes backward in the current shell. The accumulator variable name is passed as an override to `withCallbackParams()` so `acc` resolves to the result variable name inside the callback.

**Computed property access uses `eval` with `\${_obj_varName_${key}}` for reading.** The outer shell expands `${key}` to the key value before eval runs. The `\${_obj_varName_` is an escaped `${` that eval interprets as a variable reference. For writing, `eval "_obj_varName_${key}=$value"` uses the outer shell's `${key}` expansion to construct the full variable name.

**Computed object keys are validated before eval.** Dynamic keys must match `[A-Za-z0-9_]+`. Generated code stores the key and value in temporary variables, rejects unsafe keys, and assigns through escaped temp-var references.

**`console.log` for objects prints multi-line format matching bun.** Inline object literals emit one static `printf` format with one argument per field. Dynamic-key objects iterate `_objkeys_*` at runtime; static-key objects use known fields and emit one `printf` format with object slot values as arguments. The output format is `{ key: value, }` with one property per line. Boolean properties (where `objPropTypeMap` indicates `TypeBoolean`, or inline field expressions are boolean) are displayed as `true`/`false`; numeric and string values are displayed as-is. Static-key object printing must still read the current `_obj_*` slots so later property assignments are reflected.

**`return acc` inside a reduce callback with object accumulator is a no-op.** Object mutations are applied via property assignments that persist across iterations. The `genReturn` function consults the scoped `reduceReturns` stack, skips `return acc`, and rejects returning a different value from an object-accumulator callback.

## Adding a New Language Feature

1. **Token**: add `TokXxx` to `token.go`, add keyword string to `keywords` map if keyword-based
2. **AST node**: add struct to `ast.go` implementing `Statement` or `Expression`
3. **Parser**: add parse function, wire into `parseStatement()` or `parsePrimary()`
4. **Semantics**: add case in `checkSemanticStmt()` or `checkSemanticExpr()`; validate supported surfaces and update scope
5. **Codegen**: add case in `genStmt()` or `genExprRHS()`; handle `paramMap`/`rewriteVarRefs` if needed
6. **Module rewrite pass**: add cases to `rewriteStmt()`/`rewriteExpr()` in `modules.go` if the new node contains function calls
7. **Tests**: add lexer test, parser test, semantics validator test (valid + error), codegen unit test, integration test
8. **Examples**: update `examples/healthcheck/` if the feature is user-visible
9. **README**: update syntax reference
10. **SKILL.md**: update `skills/besht-scripting/SKILL.md`
11. **AGENTS.md**: update this file (Syntax Reference, Types, Architecture, Pitfalls)

**Prefer native array APIs over global list helpers.** New language work, examples, and docs should use `items.length`, `items[0]`, `items.slice(1)`, `items.push(value)` or `[...items, value]`, `items.includes(value)`, `items.concat(other)`, `items.sort()`, `items.toSorted()`, and `items.toSpliced(...)`. The old global helpers `len`, `head`, `tail`, `append`, `contains`, and `concat` remain supported for compatibility, but do not add new global list helpers or present them as the primary API.

**`Set<T>` is a compiler-backed membership collection.** `new Set<T>()` initializes an empty newline-delimited set, `.has(value)` emits a POSIX `grep -qxF` membership test, and `.add(value)` mutates the set with an `awk` uniqueness filter. Straight-line static scalar `.add()` calls track known values and fold static `.has()` calls to `1`/`0`; invalidate that static Set map on reassignment, dynamic adds, control-flow adds, callback adds, or newline-containing values. Type parameters are annotations only; do not add runtime type checks.

**Nested arrays are encoded as packed rows.** When `.map()` returns an array, each row is packed with ASCII unit separator (`\037`) so the outer array remains newline-delimited. `matrix[row]` decodes one row, `matrix[row][col]` indexes into it, and `.length` on a decoded row counts row items. Keep this generic; do not special-case fixtures.

**Blockless `if`/`else` bodies are parsed as one-statement blocks.** `if (cond) return x` and `else expr` should produce the same AST shape as braced single-statement blocks.

**Import value exports, shell imports, declaration files, and .ts fallback.** `export const name = expr` and `export default expr` are module-level value exports. Module codegen qualifies exported values as `<module>__<name>` and default exports as `<module>__default`; imported values are mapped through `moduleGenerator.importVarMap` into `paramMap` so normal identifier and command-spread generation uses the qualified shell variable. Imported value representation hints must also be seeded into `varTypeMap` for both the source import name and qualified shell name so codegen dispatches imported array/boolean methods and formatting correctly. Keep extensionless import resolution `.bsh`-first. Only `codegen.Options.ResolveTsImports` / `--opt-resolve-ts-imports` may fall back to `.ts`, and `.d.bsh` imports remain declaration-only. `CompileFile`, `CompileFileSplit`, and `CheckFile` auto-load `stdlib.d.bsh` from the entry file directory when present; do not scan imported module directories for their own stdlib files, and do not emit/source declaration files in bundled or split output. Declared function calls must keep the declared external name unqualified; do not rewrite them to `<module>__<name>` because no Besht wrapper is emitted. Explicit `.sh` imports require named imports plus `assert { type: "shell" }`; default shell imports are rejected, shell files are not parsed for exports, and semantics validator registration treats imported shell functions as unchecked varargs returning `string`. `--check` uses the module compiler path so import validation matches compilation. By default shell imports must stay inside the compiler root; `codegen.Options.AllowExternalShellImports` / `--opt-allow-external-shell-imports` permits explicit `.sh` imports outside that root without relaxing `.bsh` module imports. Bundled output sources the resolved shell file with a guard, while split output copies in-root raw shell dependencies into the output tree and sources them via safely quoted `_BESHT_ROOT` paths. External opt-in shell imports in split output are sourced from their original absolute path instead of copied outside the output root. Shell import guard variables are generated from unique relative shell paths so names like `a-b.sh` and `a_b.sh` cannot collide.
