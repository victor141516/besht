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

---

## Type Checking Policy

**Besht has no type checking — not at compile time, not at runtime. This is by design.**

- Type annotations (`let x: string = "hi"`, `function f(a: number): string`) are **completely ignored** by the compiler
- They exist only so users can write TypeScript-compatible syntax and get editor support (autocomplete, type hints) via `declare` statements and `.d.bsh` files
- The compiler never errors on type mismatches
- The `internal/checker/checker.go` file exists but only collects function signatures — it performs no validation

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
go run ./cmd/besht/ <file.bsh>
go run ./cmd/besht/ <file.bsh> -o out.sh
go run ./cmd/besht/ --check <file.bsh>

# Split mode — one .sh per .bsh, imports become source calls
go run ./cmd/besht/ <file.bsh> --split -o <outdir/>

# Omit the runtime POSIX self-check from compiled output
go run ./cmd/besht/ <file.bsh> --opt-no-add-binaries-check

# Opt in to extensionless .ts import fallback when .bsh is absent
go run ./cmd/besht/ <file.bsh> --opt-resolve-ts-imports

# Allow explicit .sh imports outside the compiler root
go run ./cmd/besht/ <file.bsh> --opt-allow-external-shell-imports
```

### CLI Flags

| Flag                          | Description                                                           |
| ----------------------------- | --------------------------------------------------------------------- |
| `-o <path>`                   | Output file or directory (required with `--split`)                    |
| `init`                        | Write `./stdlib.d.bsh` declarations in the current directory          |
| `init --force`                | Overwrite a different existing `./stdlib.d.bsh`                       |
| `--split`                     | Compile each `.bsh` to its own `.sh`; imports become `. source` calls |
| `--check`                     | Validate imports, command usage, and unsupported fetch APIs, no output |
| `--opt-no-add-binaries-check` | Omit the runtime POSIX utility self-check block                       |
| `--opt-no-source-map`          | Omit `# besht:file:line:col` source comments from compiled output        |
| `--opt-resolve-ts-imports`     | Let extensionless imports fall back to `.ts` only when `.bsh` is absent |
| `--opt-allow-external-shell-imports` | Allow explicit `.sh` imports outside the compiler root          |
| `--version`                   | Print version                                                         |

### `--opt-*` flags

All flags that change how code is transformed or what is emitted share the `--opt-` prefix. Currently:

| Flag                          | Effect                                                                          |
| ----------------------------- | ------------------------------------------------------------------------------- |
| `--opt-no-add-binaries-check` | Do not emit the `_r=$(printf ...)` runtime check at the top of compiled scripts |
| `--opt-no-source-map`          | Do not emit `# besht:file:line:col` source comments in compiled output         |
| `--opt-resolve-ts-imports`     | Resolve extensionless imports to `.bsh` first, then `.ts` if `.bsh` is absent |
| `--opt-allow-external-shell-imports` | Permit explicit `.sh` imports outside the compiler root; `.bsh` imports remain root-confined |

Pass via `codegen.Options{NoCheck: true, NoSourceMap: true, ResolveTsImports: true, AllowExternalShellImports: true}` in Go code.

Generated shell emits inline `# besht:file:line:col` source comments at non-class statement boundaries and before explicit class constructor/accessor/method shell functions. Class declarations skip the generic statement-boundary comment so synthetic property accessors and implicit default constructors do not receive source comments.

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
├── checker/
│   └── checker.go       # Type checker + scope resolver; walks AST, annotates types
│                        # FnSig, Scope, Checker structs; RegisterFn for cross-module sigs
│                        # checkCommandMethod() routes .pipe()/.run()/.readStdoutLines() etc.
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
  → Checker (checker.Check)              ← registers cross-module fn sigs first
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

The compiler emits shell-safe literal command words bare when possible (`git status --short`) and single-quotes anything that needs protection (`'*.go'`, `'hello world'`, raw strings, shell reserved command names, embedded quotes). Single-quote escaping handles embedded `'` characters automatically (`'it'"'"'s alive'`). Variable references are passed as `"$var"`. This prevents shell injection and gives the compiler full control over quoting strategy.

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

**Output philosophy:** Generated shell should be readable and minimal. Side-effect-only commands (`cmd.run()` with no `.readStdout()`/`.readStdoutLines()`/`.exitCode()`) compile to a bare shell command with no variable. Capture variables are only emitted when the corresponding method is actually used. Helper code, metadata, array/object metadata, runtime checks, and other shell boilerplate should only be emitted when the compiled program uses functionality that requires it. For example, if an array operation needs metadata, do not emit that metadata for programs that never use that operation. Static scalar list literals, static scalar `Array.of(...)` calls, and static `Array.from({ length: N })` calls compile to quoted newline-backed shell strings when elements contain no newlines; dynamic, nested, spread, and newline-sensitive list expressions keep the `printf` builder. Static string literal and static scalar list literal `.length` properties compile to numeric constants; dynamic lengths keep the `wc` path. Static scalar list literals and variables bound to them compile to compact shell `for item in ...; do` loops when elements contain no newlines; dynamic and newline-sensitive list expressions keep the heredoc-backed read loop so assignments and `break` persist in the current shell. Static scalar list indexes with known in-range integer indexes compile to constants; dynamic, unknown, and out-of-range indexes keep the POSIX `sed` path. Static scalar list literal `.join()` and `.toString()` calls compile to one quoted string when elements contain no newlines and the separator is static; dynamic and newline-sensitive joins keep the `awk` path. Static scalar list literal `.includes()`, `.indexOf()`, and `.lastIndexOf()` with static scalar needles compile to constants; dynamic searches keep the `grep`/`awk` path. Static ASCII string literal `.includes()`, `.startsWith()`, `.endsWith()`, `.indexOf()`, `.lastIndexOf()`, and `.charAt()` calls with static arguments compile to constants; dynamic and non-ASCII string searches keep the helper/`awk` path. Static ASCII string literal transforms (`trim`, `trimStart`, `trimEnd`, `toUpperCase`, `toLowerCase`, `slice`, `substring`, `repeat`, `padStart`, `padEnd`) with static arguments compile to constants; dynamic and non-ASCII transforms keep the POSIX `sed`/`tr`/`cut`/`awk` path. Static ASCII string literal `.split()` calls with static separators compile to quoted newline-backed list strings and compact `for item in ...; do` loops when resulting elements contain no newlines; dynamic and non-ASCII splits keep the POSIX `tr`/`awk` path. Inline static scalar object literal `Object.keys()`, `Object.values()`, `Object.entries()`, and `Object.hasOwn()` calls compile to constants; named objects keep compiler-managed object metadata so assignments and computed keys stay visible. Static boolean `if` conditions and ternary expressions compile to the selected branch or value; dynamic conditions keep shell tests. Static string literal `Number.parseInt()` calls with parseable prefixes and static radix compile to numeric constants; dynamic parseInt keeps the shell arithmetic path. Static numeric arithmetic over literal numbers, static numeric literal `.toString()`/`.toFixed()` calls, and literal-argument `Math.*` calls compile to constants; dynamic numeric expressions keep shell arithmetic or the POSIX `awk` path. Single-command redirects append directly to the command; pipeline redirects keep `{ ...; }` grouping so the redirect applies to the whole pipeline. String runtime helpers (`_bst_starts_with`, `_bst_ends_with`, `_bst_includes`) are emitted only when generated shell calls the corresponding one-argument string method; two-argument string search methods use inline `awk`, and list `.includes()` uses `grep -qxF` without emitting `_bst_includes`. Top-level scripts that only read `Besht.args.positional()` use an inline `"$@"` scan instead of the full args runtime; `argv()`, `option()`, `flag()`, args reads inside functions, and split output keep the shared parser runtime. This unused-feature elision is best effort and must never break correctness. All boilerplate is opt-out via `--opt-*` flags.


Static boolean `console.log()` and `console.error()` arguments such as `Boolean("")`, `true`, and simple static `!`/`&&`/`||` expressions render directly as `true`/`false`; dynamic boolean expressions keep the general formatting path.

Variables bound to static string literals may fold `.length` to a numeric constant. Do not fold variables assigned inside control flow because later loop iterations or branch-dependent assignments can make the initial value stale.


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

- Functions returning `string`/`number`/`list<T>`: emit `printf '%s'` to stdout; callers capture with `$(fn args)`
- Void functions: no capture; callers emit bare `fn args`
- Functions returning `status`: exit code via `$?`

### Types at Runtime

| Besht type | Shell representation                                              |
| ---------- | ----------------------------------------------------------------- |
| `string`   | shell string                                                      |
| `number`   | shell string containing digits; arithmetic via `$((...))`         |
| `boolean`  | `1` (true) / `0` (false); tested with `[ "$x" = 1 ]`              |
| `list<T>`  | newline-delimited string; nested list rows use unit-separator-packed lines |
| `Set<T>`   | newline-delimited unique string values for membership checks               |
| `status`   | exit code captured as `$?`                                        |
| `command`  | lazy pipeline description; no shell code until `.run()` is called |

Prefer native list APIs for new user-facing examples and compiler work: `list.length`, `list[0]`, `list.slice(1)`, `list.push(value)` or `[...list, value]`, `list.includes(value)`, and `list.concat(other)`. The old global list helpers `len`, `head`, `tail`, `append`, `contains`, and `concat` remain supported for compatibility, but do not add new global list helpers.

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
internal/checker/checker_test.go   # Semantic validation, scope, builtins/surfaces
internal/codegen/codegen_test.go   # Unit: AST → sh output patterns (uses Generate())
internal/codegen/integration_test.go # E2E: temp files → CompileFile() → sh output
```

`node-eq/tests/` is organized by fixture purpose: `advent/`, `commands/`, `imports/`, `language/`, and `regressions/`. Run it recursively with `bun node-eq/compare $(rg --files -g '*.bsh' node-eq/tests | sort)`. Keep imported fixture dependencies beside their importing `.bsh` files unless the import paths are updated in the same change.

Tests use `go test ./...`. Coverage target: `make cover`. Current coverage: ~75%.

## Syntax Reference (current)

```ts
// Variable declaration — type annotations optional everywhere
let name: string = "Alice"          // plain literal — " and ' produce no interpolation
let also: string = 'Alice'          // same — both quote styles are plain literals
let tmpl: string = `Hello ${name}!` // template literal — ${var} interpolation
let pattern: string = r"^foo-[0-9]+$"  // raw string — always single-quoted in sh output
let rawpath: string = String.raw`C:\temp\new\file.txt` // tagged raw template — same as r"..."
let escape: string = "newline:\n tab:\t backslash:\\ quote:\" dollar:\$"  // escape sequences
let unicode: string = "A \u0041 ñ \u00F1"  // unicode escapes
let count: number = 42
let price: number = 3.14          // float literal — compiled to awk arithmetic
let flag: boolean = true
let files: list<string> = ["a.txt", "b.txt"]
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

// Array types — all three forms compile identically to list<T>
let a: string[] = ["a", "b"]
let b: Array<string> = ["c", "d"]
let c: list<string> = ["e", "f"]
let matrix: string[][] = rows.map(row => row.join("").split("") as string[])
let indexes: number[] = Array.from({ length: 3 }) // [0, 1, 2]
let selected: string[] = Array.of("a", "b") // ["a", "b"]
let objectKeys: string[] = Object.keys(user) // object key list
let objectValues: string[] = Object.values(user) // object value list
let objectEntries: string[][] = Object.entries(user) // packed [key, value] rows
let objectHasName: boolean = Object.hasOwn(user, "name")

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
let n: number = Number.parseInt(s)
let n10: number = Number.parseInt(s, 10)
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

// For list
for (f in files) {
    $("echo", f).run()
}
for (f in files) $("echo", f).run()

// For-of alias
for (f of files) {
    $("echo", f).run()
}

// For-of alias with declaration
for (let f of files) {
    $("echo", f).run()
}

// Break and continue
for (f in files) {
    if (Besht.strings.isEmpty(f)) { continue }
    if (f == "stop") { break }
    $("echo", f).run()
}

// List indexing (0-based)
let first: string = files[0] // static scalar list indexes fold to constants when known
let item: string = files[i]
let cell: string = matrix[row][col]
let maybeName: string = user?.name ?? "anonymous"
let maybeItem: string = items?.[i] ?? "fallback"
let maybeCell: string = matrix?.[row]?.[col] ?? "missing"
let maybeTrimmed: string = maybeText?.trim() ?? ""
let width: number = matrix[0].length

// List index assignment
items[1] = "BETA"

// Empty list
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
let args: list<string> = ["-n", "hello"]
$("echo", ...args).run()

// Capture as lines: .run() + .readStdoutLines()
let logCmd = $("git", "log", "--oneline", "-20")
logCmd.run()
let log_lines: list<string> = logCmd.readStdoutLines()

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

// Spread a list into command arguments
let args: list<string> = ["-n", "hello"]
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
let upper: string = name.toUpperCase()
let lower: string = name.toLowerCase()
let parts: list<string> = name.split(",")
let chars: list<string> = name.split("")
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

// List methods. Prefer these native APIs over the older global helper aliases.
let flen: number = files.length
let first: string = files[0]
let rest: list<string> = files.slice(1)
let pushed: list<string> = files.push("new.txt")
let alsoPushed: list<string> = [...files, "new.txt"]
let prepended: list<string> = files.unshift("first.txt")
let popped: list<string> = files.pop()
let joined: string = files.join(", ")
let listText: string = files.toString() // scalar lists only; same as files.join(",")
let merged: list<string> = files.concat(other)
let rev: list<string> = files.reverse()
let copied: list<string> = [...files, "extra"]
let indexes: number[] = Array.from({ length: 3 })
let selected: string[] = Array.of("a", "b")
let mapped: list<string> = files.map(f => f + ".bak")
let labeled: list<string> = files.map((f, i) => i.toString() + ":" + f)
let classified: list<string> = files.map((f, i) => {
    if (i == 0) return "first:" + f
    return "next:" + f
})
files.forEach((f, i) => console.log(i.toString() + ":" + f))
let filtered: list<string> = files.filter(f => f.endsWith(".txt"))
let hasTxt: boolean = files.some((f, i) => f.endsWith(".txt") && i >= 0)
let allNamed: boolean = files.every(f => f.length > 0)
let firstTxt: string = files.find(f => f.endsWith(".txt")) ?? "missing"
let foundIndex: number = files.findIndex(f => f == "b.txt")
let reduced: number = nums.reduce((acc, n) => acc + n, 0)
let lines: string = nums.reduce((acc, n) => [...acc, "#".repeat(n)], [] as string[]).join("\n")
let counts = words.reduce((acc, word) => {
    acc[word] = (acc[word] || 0) + 1
    return acc
}, {})
let sliced: list<string> = files.slice(1, 3)
let found: boolean = files.includes("a.txt") // list membership via grep -qxF; does not emit _bst_includes
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
2. Add case in `checkBuiltinCall()` in `checker/checker.go`
3. Add case in `genBuiltinCapture()` (value position) in `codegen/codegen.go`
4. If usable as a statement, add case in `genBuiltinStmt()` in `codegen/codegen.go`
5. If usable as a condition, add case in `genBuiltinCondition()` in `codegen/codegen.go`

## Adding a New Command Method

Command methods chain on `command` type values. With the lazy Command model:

1. Add case in `checkCommandMethod()` in `checker/checker.go` — specify arg count and return type
2. If the method is a **terminal** (causes execution or capture): handle in the Command Analysis pass so it's accounted for in the emit decision
3. Add case in `genCmdChain()` in `codegen/codegen.go` — emit the shell equivalent
4. If the method records a capture (like `.readStdout()` or `.exitCode()`), codegen usually reads from the pre-assigned capture variable. Immediate anonymous `.run().readStdout()` / `.run().readStderr()` expressions may compile directly to command substitution; named command objects and `.exitCode()` must keep `genRunCall()` capture/exit-variable emission so reuse stays correct.

## Known Semantic Differences from TypeScript

**Booleans are stored as `1`/`0` in shell variables.** In string contexts besht now renders them as `true`/`false`, but condition generation still relies on `1`/`0`.

**`includes()`, `startsWith()`, `endsWith()` still drive conditions via `1`/`0`, but string contexts now render them as `true`/`false`.**

**Float literals use awk for arithmetic.** `3.7 + 1.2` → `$(awk -v _a=3.7 -v _b=1.2 'BEGIN{print _a + _b}')`. `$((...))` is integer-only in POSIX sh.

**`Number.isNaN()` is always false for currently representable besht values.** Besht has no NaN runtime sentinel, so the API exists for JS-compatible syntax but can't observe NaN.

**`Boolean(value)` is a primitive coercion builtin only.** It returns Besht boolean `1`/`0` and relies on existing boolean string rendering for `true`/`false`. Keep the slice narrow: falsey values are `false`, `0`, `0.0`, `""`, `null`, and `undefined`; non-empty strings including `"0"`/`"false"`, non-zero numbers, arrays, objects, and sets are truthy. Do not add Boolean object wrappers, `new Boolean`, `Boolean.parse`, or runtime type metadata for this API.

---

## Common Pitfalls for Agents

**Never emit `local` in generated shell output.** Not POSIX. Use `_fn_varname` mangling instead.

**`rewriteFnCalls()` runs before codegen.** When adding new expression types that contain function calls, add a case to `rewriteStmt()`/`rewriteExpr()` in `modules.go` so the pass descends into them.

**`rewriteVarRefs()` must be called on `TemplateLit` values only.** Only template literals (`` `...${var}...` ``) contain `${...}` references that need var-name rewriting. Plain `StringLit` and `RawStringLit` are emitted as single-quoted sh and contain no shell expansions. Do NOT call `rewriteVarRefs()` on plain string values.

**`paramMap` is per-function scope.** Saved/restored on function entry/exit in both `genFnDecl` and `genModuleFnDecl`. Loop vars and catch vars must be registered into `paramMap` and cleaned up.

**`genCmdChain()` is recursive.** It walks `.pipe().pipe().stdout()` chains by recursing on the receiver. The base case is `*ast.CmdExpr`. Terminal methods must handle any redirect string accumulated from inner calls.

**`r"..."` raw strings always compile to single-quoted sh.** Use for regex patterns, AWK programs, sed expressions, grep patterns — anything containing `$`, `^`, `[`, `\` that should be literal.

**`\$` in regular strings produces a literal dollar sign.** `"price \$5"` → `"price \$5"` in sh (POSIX: `\$` in double-quotes is literal). Do NOT rely on `$` alone being literal — use `\$` explicitly.

**Compile-time warning for `-`-prefixed string literals in `$()` args.** If a string literal starting with `-` containing special characters (`$`, `^`, `[`, `]`, etc.) is passed as a `$()` argument, the compiler warns and suggests `r"..."` or adding `-e`/`--` before it. Suppressed when the preceding argument is `-e` or `--`.

**Runtime POSIX self-check.** Every compiled script emits a `_r=$(printf 'hello:world' | grep -F 'hello' | sed ...)` pipeline at the top of the preamble that verifies `grep`, `sed`, and `printf` work correctly end-to-end. If the result is wrong the script exits immediately. Omit with `--opt-no-add-binaries-check` at compile time.

**`string.repeat(n)` uses awk.** The old `printf '%0.s'` approach emitted nothing. The correct implementation loops in awk: `awk -v _s=STR -v _n=N 'BEGIN{r=""; for(i=0;i<_n;i++) r=r _s; printf "%s",r}'`.

**`Math.round()` matches JS semantics: `floor(x+0.5)`.** This rounds half toward +∞ (so `round(-2.5) = -2`). The naïve `int(x + 0.5)` is wrong for negatives. The correct awk formula is `_y=_x+0.5; print int(_y)-((_y<int(_y))?1:0)`.

**`inferReceiverType()` is recursive.** It handles `BinaryExpr`, `MethodCallExpr`, and literal nodes — not just `IdentExpr`. This is required so chained `+` concatenations like `("a" + x) + "!" + n` correctly infer string type at every level.

**`strInner()` always uses `${var}` notation.** When extracting the inner content of a double-quoted string like `"$a"` for concatenation, `strInner` returns `${a}` not `$a`. This prevents `"$a" + "bc"` from producing `"$abc"` (wrong variable name). Never revert this to `$var`.

**`genArgs()` applies `ensureArgSafe()` to all generated args.** Any `$(...)` expression is automatically wrapped in `"$(...)"` when passed as an argument to a command, `console.log`, or a function call. This prevents word-splitting of multi-word command output.

**Command literal words prefer natural bare output only when safe.** `genCmdArgs()` uses `cmdArgWordForExpr()` so ordinary string literals such as `"git"` and `"--short"` can emit as `git --short`, while raw strings, globs, spaces, embedded quotes, variables, command substitutions, shell reserved command names, and command-position assignments remain quoted or protected. Do not route general string/assignment/list emission through this command-word path.

**`escapeForDoubleQuote()` handles shell-active characters in string literals.** When emitting a double-quoted sh string, backtick `` ` ``, `$(`, and bare `$WORD`/`$1` (not inside `${...}`) must be escaped as `` \` ``, `\$(`, and `\$WORD`. The `${besht_var}` interpolation form is intentional and must be left intact. `\$` (already escaped by the lexer for literal dollars) must also be left as-is. Any new string emission path must call `escapeForDoubleQuote()` before wrapping in `"..."`.

**`cmdArgQuote()` must not double-quote already-quoted values.** `RawStringLit` emits a single-quoted string from `genExprRHS` (e.g. `'s/foo/bar/'`). If `cmdArgQuote` then calls `shellQuote()` on that value again, it produces malformed shell like `'''s/foo/bar/'''`. The fix: `cmdArgQuote` short-circuits on values that already start and end with `'`. Values starting with `$` are passed raw as variable references. Complex expressions that happen to start with `$` skip quoting — be aware.

**Command `.env(name, value)` prefixes one command invocation only.** It emits `NAME=value command ...` inside the generated pipeline and does not mutate the parent shell environment.

**Command `.workdir(path)` changes cwd for one command or pipeline only.** It wraps the generated command in a subshell-style `cd path && ...` expression so subsequent statements keep the parent shell cwd. Keep redirects inside the workdir wrapper when generating command text.

**`command` objects do not auto-coerce.** Unlike the old model, `command` no longer coerces to `string` on assignment. You must explicitly call `.run()` then `.readStdout()` to get a string. The Command Analysis pass enforces this.

**`run()` returns `self`, not `void`.** This enables chaining: `$("whoami").run().readStdout()`. Do not declare `.run()` as returning `void` anywhere in the compiler — it returns the same `Command` identity.

**The Command Analysis pass runs before codegen.** It assigns a unique identity (integer) to every `CmdExpr` node in the AST, traces which variable names hold each identity through assignments, and for each identity records: which line `run()` was called (if any), and whether `.readStdout()`, `.readStdoutLines()`, `.readStderr()`, or `.exitCode()` are used. Codegen then uses this map to decide what shell code to emit for each command. If a command has no `run()` call, emit nothing and warn. If it has `run()` plus `.readStdout()` usage, emit `varname=$(...)`. If it has `run()` but no capture methods, emit a bare shell command.

**Module generator vs plain generator.** `moduleGenerator` embeds `Generator`. When adding new statement types, add a case to `genModuleTopStmt()` if they can contain function calls that need qualification.

**Checker cross-module signatures and values.** `Compiler.load()` builds a `globalSigs` map of exported signatures and a `globalVars` map for exported top-level values. When adding new exported constructs, register them there.

**Object literals compile to per-property shell variables.** `let user = { id: 1, name: "Victor" }` emits `_obj_user_id=1` and `_obj_user_name="Victor"`. Property access `user.name` reads `_obj_user_name`. Property assignment `user.name = "X"` writes `_obj_user_name="X"`. Since shell variables are global, these `_obj_*` variables are accessible inside functions. When an object is passed as a function parameter, `genProperty` uses `objPropTypeMap` (populated by `collectObjectTypes` pre-pass) to resolve the original `_obj_` prefix directly — no copying needed. Inside a function, `student.name` emits `$_obj_student_name` (using the original top-level variable name, not the mangled parameter name).

**`Object.keys(obj)`, `Object.values(obj)`, `Object.entries(obj)`, and `Object.hasOwn(obj, key)` use `_objkeys_*` metadata.** Object literal bindings, aliases, reduce object accumulators, class instance slots, static object maps, dot property assignments, and computed property assignments must keep `_objkeys_<object>` in sync. `Object.keys()` lowers to a newline-delimited list from that metadata, `Object.values()` evals each keyed scalar `_obj_*` slot in the same order, and `Object.entries()` emits existing nested-list unit-separator packed `[key, value]` rows for scalar values. Statically known boolean values format as `true`/`false` when enumerated through `Object.values()` or `Object.entries()`, while the stored shell slots remain `1`/`0`. `Object.values()` and `Object.entries()` must reject statically known list/object/set/command/fetch values because the current `string[]` and packed `string[][]` representations cannot preserve deeper nested values without corrupting rows. These helpers must not emit a runtime helper. `Object.hasOwn()` checks exact line membership against that metadata, returns `1`/`0`, and returns false for dynamic keys that fail `[A-Za-z0-9_]+` instead of mutating metadata or exiting. Static and computed object keys must match `[A-Za-z0-9_]`; computed assignments append only validated keys. The `object` annotation parses to `TypeObject` so function parameters can carry object slot names for Object helpers. `process.env` is intentionally not enumerable; reject `Object.keys(process.env)`, `Object.values(process.env)`, `Object.entries(process.env)`, and `Object.hasOwn(process.env, key)` in `--check` and compile paths.

**Classes use compiler-managed instance slots.** `let u = new User("Alice")` stores `u='u'`, calls `User__constructor "$u" ...`, and writes instance fields as `_obj_u_name`. `this.prop` inside constructors and methods uses tightly controlled `eval` to construct `_obj_${slot}_${prop}` names; user values must flow through temporary shell variables, never interpolated directly into the eval string. Static properties use `_class_<Class>_<prop>` and static/instance methods compile to `<Class>__<method>` shell functions. Explicit getters/setters compile to `<Class>__get_<name>` and `<Class>__set_<name>`; property reads/writes lower to those calls when present. Accessors cannot share a property name with a field, and source methods named `get_<name>`/`set_<name>` conflict with the corresponding accessor. TypeScript-only modifiers (`private`, `public`, `protected`, `readonly`) are accepted and ignored.

**Static object maps use object backing.** `static Deltas: Record<string, [number, number]> = { U: [-1, 0] }` emits `_obj__class_Class_Deltas_U` storage, and `Class.Deltas[key]` lowers to computed object access. `Record<K, V>` is annotation-only; it guides compiler inference but adds no runtime type checking.

**Class methods and getters that mutate `this` must be void-style calls.** Value-returning methods and getters are invoked through command substitution (`$(Class__method "$slot")`), which runs in a subshell. Any `this.prop = value` mutation inside that subshell is lost, so checker/codegen reject non-void methods and getters that assign to `this` properties. Constructors and setters are exempt because they are called directly.

**`String.raw\`...\`` compiles identically to `r"..."`.** Backslashes are literal — no escape sequence processing. Use for Windows paths, regex, or any string containing `\`.

**Optional string search positions are normalized in awk.** `indexOf`/`includes`/`startsWith` position, `lastIndexOf` position, and `endsWith` length must use `awkArg()` plus awk-side `int()` truncation and clamping. Do not strip quotes into shell arithmetic for these arguments.

**Escape sequences in double-quoted strings.** `\n`, `\t`, `\r`, `\\`, `\"`, `\'`, `\uXXXX` are processed by the lexer into their actual characters. Single-quoted strings do NOT process escapes — they are always literal.

**Postfix `++` and `--` are statement-only; prefix updates are expressions.** `count++` and `count--` compile to assignment statements. Prefix `++count` and `--count` are supported in expression position and return the updated numeric value. They must resolve through `paramMap` like other variable reads/writes.

**Arrow callbacks support compiler-known list methods only.** Direct list `.map(x => expr)`, `.map((x, i) => { return expr })`, `.filter((x, i) => truthyExpr)`, `.some((x, i) => truthyExpr)`, `.every((x, i) => truthyExpr)`, `.find((x, i) => truthyExpr)`, `.findIndex((x, i) => truthyExpr)`, `.reduce((acc, cur) => { ... }, init)`, and statement-position `.forEach((x, i) => { ... })` callbacks are supported. `.map()` callbacks may take `(item)` or `(item, index)` and block-bodied `return` emits one mapped value then continues the callback loop; supported block statements are `return`, `if`/`else`, and assignments. Do not splice generic expression statements into map callback shell source — reject unsupported statements instead of treating expression values as commands. `.filter()`, `.some()`, `.every()`, and `.findIndex()` use JavaScript-style truthiness and may take the same optional zero-based index parameter. `.some()` returns `1` on the first truthy callback result and `0` for an empty list; `.every()` returns `0` on the first falsey callback result and `1` for an empty list; `.find()` returns the first matching scalar element or `_BESHT_NULLISH_SENTINEL` so `??` fallbacks work. `.reduce()` takes a 2-parameter arrow (accumulator, current) with either expression or block body, plus an initial value. `.forEach()` has no value result, takes `(item)` or `(item, index)`, rejects `return`/`break`/`continue` plus pure value expressions, and emits a current-shell heredoc loop so outer assignments and `Set.add()` side effects persist. Arrows are not general function values, cannot be stored in variables, and should be treated as side-effect-free except in `reduce` and `forEach` block bodies. Callback params are temporarily added to `paramMap` via `withCallbackParams()` and restored after body generation; never emit `local`.

**`Array.from({ length })` is narrow by design.** It only supports an object literal with a numeric `length` field (including shorthand `{ length }`) and emits a zero-based numeric list. Do not broaden it to arbitrary iterables or mapper callbacks without adding parser/checker/codegen coverage and docs.

**`Array.isArray(value)` is static.** It returns true only when codegen can infer `value` as `TypeList`; otherwise it emits false. Do not add runtime array metadata or dynamic shape inspection for this API without a broader object/list representation design.

**Type assertions are erased.** `expr as Type` exists for TypeScript-compatible syntax and compiler type inference only. It emits the inner expression unchanged. This is especially useful for empty list accumulators such as `[] as string[]`.

**Tuple/list destructuring declarations are lowered to index reads.** `const [dr, dc] = pair` evaluates `pair` once into a temp and assigns each name from one-based shell line extraction. Element type inference comes from the list/tuple annotation when available.

**List literal spread is generic.** `[...items, extra]` expands the existing newline-delimited list and appends normal elements. It is used by reduce list accumulators and should remain a list literal transformation, not a reduce-specific special case.

**List predicate callbacks short-circuit in current-shell heredoc loops.** `list.some(callback)`, `list.every(callback)`, `list.find(callback)`, and `list.findIndex(callback)` use direct arrow callbacks with one item parameter or `(item, index)`. Keep callback params in `paramMap`, increment the optional index once per scalar element, and avoid pipeline loops when state must persist inside the generated predicate loop. `find()` must require the nullish runtime helper and initialize its result to `_BESHT_NULLISH_SENTINEL`; no-match must compose with `??`. Nested-list element decoding is not part of these scalar predicate methods yet.

**For-of list expression loops run in the current shell.** `for (const move of moves.split("") as string[])` uses a heredoc-backed `while read`, not a pipeline, so `break` and assignments inside the loop persist.

**Optional chaining uses flags on existing postfix AST nodes.** `IndexExpr`, `PropertyExpr`, and `MethodCallExpr` have `Optional bool`; keep all AST walkers descending into their receivers, indexes, and arguments. Codegen emits POSIX shell that stores the receiver once, compares it with `_BESHT_NULLISH_SENTINEL`, and returns that same sentinel on short-circuit so `??` can distinguish nullish from `""`, `0`, and `false`. Optional chaining guards nullish receivers only; do not add runtime shape/type checks. General `fn?.()`, `obj.method?.()`, and optional assignment targets remain unsupported.

**String runtime helpers are lazy.** `_bst_starts_with`, `_bst_ends_with`, and `_bst_includes` definitions belong in the preamble only when the generated body actually calls those helpers. Generate the body first (or otherwise track helper use before assembling the preamble) for single-file, bundled, and split output. In bundled module output, emit the union of helpers needed by all modules near the top-level preamble. In split output, emit helpers per generated file only when that module body needs them. Preserve the entry-file POSIX self-check block behavior. List `.includes()` is separate: it uses `grep -qxF` and must not mark `_bst_includes` as needed.

**`list.join(sep)` and scalar `list.toString()` use awk, not `paste -sd`.** Multi-character separators for `join` require awk since `paste -sd` only uses the first character of the delimiter. Scalar list `toString()` reuses the same join lowering with `,` as the separator and does not implement JavaScript nested-array flattening for `string[][]` or packed rows. The generated join shape is: `awk -v s=', ' 'NR>1{printf s}{printf "%s",$0}'`.

**AWK `OFMT="%.17g"` is set in all arithmetic BEGIN blocks.** This matches JavaScript double-precision output for division results (e.g., `72.66666666666667` instead of `72.6667`).

**`collectObjectTypes` pre-pass runs before codegen.** It walks the AST to populate `objPropTypeMap` (mapping `"varName.propName"` → type) so that `genProperty` and `inferReceiverType` can resolve object property types even inside function bodies. Without this pre-pass, functions defined before object literals would have empty `objPropTypeMap` entries.

**`fnParamTypes` tracks function parameter types during codegen.** Set in `genFnDecl`/`genModuleFnDecl` from `param.Type` annotations. Used by `inferReceiverType` to determine that a function parameter like `scores: number[]` is a list, enabling `scores.length` and `scores[i]` to work correctly.

**`||` and `&&` in value position return actual values (JS semantics), not booleans.** `a || b` returns `a` if truthy, else `b`. `a && b` returns `b` if `a` is truthy, else `a`. This is different from condition position (used in `if`/`while`) which returns 1/0. The implementation uses a subshell with `_l=temp` capture to test the left side, then returns the appropriate value.

**`??` uses an internal nullish sentinel, not shell default expansion.** Do not lower it to `${var:-fallback}` because empty string, `0`, and `false` must be preserved. Only `null`, `undefined`, missing `args` values, missing `process.env.NAME` variables, and missing indexes in nullish-left position should trigger the fallback.

**`process.env.NAME ?? fallback` must use unset-only detection.** Lower `process.env.NAME` with `${NAME+x}` and `_BESHT_NULLISH_SENTINEL`; never use `${NAME:-fallback}` for this API because an explicitly empty environment variable must be preserved.

**`Besht` is a standard namespace exemption.** Module rewriting must not qualify `Besht` as a class/function-like identifier. `Besht.fs.*`, `Besht.strings.*`, `Besht.args.*`, and `Besht.iter.*` are parser-level method calls on `Besht` groups. In condition position, file/string predicates must emit minimal tests (`[ -f ... ]`, `[ -d ... ]`, `[ -r ... ]`, `[ -w ... ]`, `[ -x ... ]`, `[ -z ... ]`, `[ -n ... ]`) and must not introduce runtime helpers.

**`Besht.args.*` helpers parse script arguments in generated POSIX sh.** `Besht.args.argv()` returns positional args only, `Besht.args.positional(n)` returns a 1-based positional value or nullish, `Besht.args.option(long, short?)` supports `--long=value`, `--long value`, and optional `-s value`, and `Besht.args.flag(long, short?)` returns boolean `1`/`0`. Keep defaults outside helpers via `??`.

**`--` stops args option parsing.** After `--`, values that look like options or flags are positional. For example, `script --branch=dev -- -d file` has `Besht.args.flag("dry-run", "d") == false` and positional args `-d`, `file`.

**`fetch()` is currently a narrow synchronous text-only builtin.** It returns internal `TypeFetchResponse`, supports only `fetch(url).text()` and `let response = fetch(url); response.text()`, and lowers to `curl -sS -- <url>`. Assigned responses run curl once and store stdout in `_obj_<response>_body`; aliases and reassignments copy or refresh that body slot. `.body` is internal and not a user-facing property. `codegen.CheckFile` enables a narrow fetch-surface validation pass so `--check` rejects unsupported response properties/methods like `.status` and `.json()` without annotation type checking. Do not add `await`, options, POST, headers, bodies, `.json()`, `.status`, `.ok`, or `.headers` without a separate design.

**Equality conditions bind operands before comparing.** `genBinaryCondition()` must assign both sides to temporary variables and compare `[ "$_bst_left" = "$_bst_right" ]` / `!=`. Direct `[ $(fn) = "multi\nline" ]` breaks POSIX `[ ]` argument parsing when strings contain spaces or newlines.

**`reduce()` emits a heredoc-based while loop, not a pipe.** Using `printf | while read` would create a subshell, causing `_obj_*` variable mutations inside the loop to be lost. The heredoc `while read; do ...; done << EOF` pattern runs in the current shell context. The accumulator variable name is passed as an override to `withCallbackParams()` so `acc` resolves to the result variable name inside the callback.

**Computed property access uses `eval` with `\${_obj_varName_${key}}` for reading.** The outer shell expands `${key}` to the key value before eval runs. The `\${_obj_varName_` is an escaped `${` that eval interprets as a variable reference. For writing, `eval "_obj_varName_${key}=$value"` uses the outer shell's `${key}` expansion to construct the full variable name.

**Computed object keys are validated before eval.** Dynamic keys must match `[A-Za-z0-9_]+`. Generated code stores the key and value in temporary variables, rejects unsafe keys, and assigns through escaped temp-var references.

**`console.log` for objects prints multi-line format matching bun.** Dynamic-key objects iterate `_objkeys_*` at runtime; static-key objects use known fields. The output format is `{ key: value, }` with one property per line. Boolean properties (where `objPropTypeMap` indicates `TypeBoolean`) are displayed as `true`/`false`; numeric and string values are displayed as-is.

**`return acc` inside a reduce callback with object accumulator is a no-op.** Object mutations are applied via property assignments that persist across iterations. The `genReturn` function consults the scoped `reduceReturns` stack, skips `return acc`, and rejects returning a different value from an object-accumulator callback.

## Adding a New Language Feature

1. **Token**: add `TokXxx` to `token.go`, add keyword string to `keywords` map if keyword-based
2. **AST node**: add struct to `ast.go` implementing `Statement` or `Expression`
3. **Parser**: add parse function, wire into `parseStatement()` or `parsePrimary()`
4. **Checker**: add case in `checkStmt()` or `checkExpr()`; validate types, update scope
5. **Codegen**: add case in `genStmt()` or `genExprRHS()`; handle `paramMap`/`rewriteVarRefs` if needed
6. **Module rewrite pass**: add cases to `rewriteStmt()`/`rewriteExpr()` in `modules.go` if the new node contains function calls
7. **Tests**: add lexer test, parser test, checker test (valid + error), codegen unit test, integration test
8. **Examples**: update `examples/healthcheck/` if the feature is user-visible
9. **README**: update syntax reference
10. **SKILL.md**: update `skills/besht-scripting/SKILL.md`
11. **AGENTS.md**: update this file (Syntax Reference, Types, Architecture, Pitfalls)

**Prefer native list APIs over global list helpers.** New language work, examples, and docs should use `list.length`, `list[0]`, `list.slice(1)`, `list.push(value)` or `[...list, value]`, `list.includes(value)`, and `list.concat(other)`. The old global helpers `len`, `head`, `tail`, `append`, `contains`, and `concat` remain supported for compatibility, but do not add new global list helpers or present them as the primary API.

**`Set<T>` is a compiler-backed membership collection.** `new Set<T>()` initializes an empty newline-delimited set, `.has(value)` emits a POSIX `grep -qxF` membership test, and `.add(value)` mutates the set with an `awk` uniqueness filter. Type parameters are annotations only; do not add runtime type checks.

**Nested lists are encoded as packed rows.** When `.map()` returns a list, each row is packed with ASCII unit separator (`\037`) so the outer list remains newline-delimited. `matrix[row]` decodes one row, `matrix[row][col]` indexes into it, and `.length` on a decoded row counts row items. Keep this generic; do not special-case fixtures.

**Blockless `if`/`else` bodies are parsed as one-statement blocks.** `if (cond) return x` and `else expr` should produce the same AST shape as braced single-statement blocks.

**Import value exports, shell imports, declaration files, and .ts fallback.** `export const name = expr` and `export default expr` are module-level value exports. Module codegen qualifies exported values as `<module>__<name>` and default exports as `<module>__default`; imported values are mapped through `moduleGenerator.importVarMap` into `paramMap` so normal identifier and command-spread generation uses the qualified shell variable. Imported value representation hints must also be seeded into `varTypeMap` for both the source import name and qualified shell name so codegen dispatches imported list/boolean methods and formatting correctly. Keep extensionless import resolution `.bsh`-first. Only `codegen.Options.ResolveTsImports` / `--opt-resolve-ts-imports` may fall back to `.ts`, and `.d.bsh` imports remain declaration-only. `CompileFile`, `CompileFileSplit`, and `CheckFile` auto-load `stdlib.d.bsh` from the entry file directory when present; do not scan imported module directories for their own stdlib files, and do not emit/source declaration files in bundled or split output. Declared function calls must keep the declared external name unqualified; do not rewrite them to `<module>__<name>` because no Besht wrapper is emitted. Explicit `.sh` imports require named imports plus `assert { type: "shell" }`; default shell imports are rejected, shell files are not parsed for exports, and checker registration treats imported shell functions as unchecked varargs returning `string`. `--check` uses the module compiler path so import validation matches compilation. By default shell imports must stay inside the compiler root; `codegen.Options.AllowExternalShellImports` / `--opt-allow-external-shell-imports` permits explicit `.sh` imports outside that root without relaxing `.bsh` module imports. Bundled output sources the resolved shell file with a guard, while split output copies in-root raw shell dependencies into the output tree and sources them via safely quoted `_BESHT_ROOT` paths. External opt-in shell imports in split output are sourced from their original absolute path instead of copied outside the output root. Shell import guard variables are generated from unique relative shell paths so names like `a-b.sh` and `a_b.sh` cannot collide.
