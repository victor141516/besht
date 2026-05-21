---
name: besht-scripting
description: >
  Write, edit, and debug besht scripts (.bsh files). Use when the user wants
  to create a shell script using besht, asks how to do something in besht,
  needs to compile a .bsh file to POSIX sh, or asks about besht syntax —
  variables, constants, raw strings r"...", String.raw, $() command expressions,
  .pipe(), .stdout(), .stderr(), .readStdout(), .readStdoutLines(), .readStderr(),
  functions, if/while/for/switch, break/continue, try/catch, imports,
  list/string/number methods, Array.from({ length }), Array.of(), Set<T>, nested lists, object literals, classes, logical operators, nullish coalescing ??, args.argv()/positional()/option()/flag(), string
  concatenation, env(), console.log(), to_str(), String(), to_int(), or
  built-in functions like file_exists, len, and range.
---

Write and edit besht scripts. Besht is a TypeScript-flavored language that compiles to POSIX sh. Files use the `.bsh` extension.

## Compile

```sh
besht script.bsh                    # print compiled sh to stdout
besht script.bsh -o out.sh          # write to file
besht --check script.bsh            # type-check and validate imports only
besht --check --strict script.bsh   # type-check with validation
besht script.bsh | sh               # compile and run
besht script.bsh --split -o build/  # compile each file to its own .sh
besht script.bsh --opt-no-add-binaries-check  # omit runtime self-check
besht script.bsh --opt-no-source-map            # omit sourcemap from compiled output
besht script.bsh --opt-resolve-ts-imports       # allow extensionless imports to fall back to .ts
besht script.bsh --opt-allow-external-shell-imports  # allow explicit .sh imports outside compiler root
```

Generated shell keeps runtime boilerplate minimal: string helpers for one-argument `includes()`, `startsWith()`, and `endsWith()` are emitted only when used; list `.includes()` uses `grep -qxF` membership and does not emit the string `_bst_includes` helper.

## Variable Declarations

Types are optional. Use `let` for mutable variables, `const` for immutable ones. Pass `--strict` to validate annotations.

```ts
const VERSION: string = "1.0.0";
const THRESHOLD: number = 90;
```

Reassign with a plain assignment (no keyword):

```ts
count = count + 1;
count += 2;
count++;
count--;
let next = ++count;
```

## String Literals

Both `"..."` and `'...'` are plain literals — no interpolation. Use backtick template literals for interpolation:

```ts
let name: string = "Alice"           // plain — no interpolation
let also: string = 'Bob'             // same
let tmpl: string = `Hello ${name}!`  // template literal — interpolates ${name}
let sum = `sum=${a + b}`             // expressions inside ${...}
let pattern: string = r"^foo-[0-9]+" // raw — always single-quoted in sh output
let rawpath = String.raw`C:\temp\new\file.txt` // tagged raw template — same as r"..."
let escape: string = "newline:\n tab:\t backslash:\\ quote:\" dollar:\$"  // escape sequences
let unicode: string = "A \u0041 ñ \u00F1"  // unicode escapes
```

Concatenation with `+`:

```ts
let label: string = "check:" + name;
let msg: string = `Hello, ${name}!`; // preferred when interpolation is needed
let bigger = a > b ? a : b;
```

Raw strings (`r"..."`) are always emitted single-quoted in sh. Use them for regex patterns, AWK programs, sed expressions — anything with `$`, `^`, `[`, `\` that must be literal. `String.raw\`...\`` is identical to `r"..."`.

Escape sequences in double-quoted strings: `\n` (newline), `\t` (tab), `\r` (carriage return), `\\` (backslash), `\"` (double quote), `\'` (single quote), `\uXXXX` (unicode). Single-quoted strings do NOT process escapes.

## Environment Variables

```ts
let home: string = env("HOME");
let port: string = env("PORT", "8080"); // with default
```

## Script Arguments

```ts
let all: string[] = args.argv()
let input = args.positional(1) ?? "-"
let branch = args.option("branch", "b") ?? "main"
let dryRun = args.flag("dry-run", "d")
```

Use `??` for argument defaults. `args.positional()` and `args.option()` are nullish when absent and preserve empty strings when present. `args.flag()` returns a boolean. `--` stops option parsing, so later `-`-prefixed values are positional.

## Print

```ts
console.log("Usage: myscript [options]");
console.log("Result: " + value);
console.error("Something went wrong");
console.log(["a", "b"]); // [ a, b ]

// Objects are printed in multi-line format
console.log({ apple: 3, banana: 2 });
// Output:
// {
//   apple: 3,
//   banana: 2,
// }
```

## All Variables Declared with `let`

```ts
let name: string = "Alice";
let count: number = 42;
let verbose: boolean = true;
let files: list<string> = ["a.txt", "b.txt"];
```

Reassign without `let`:

```ts
count = count + 1;
```

**Types:** `string`, `number`, `boolean`, `list<T>`, `T[]`, `Array<T>`, `Set<T>`, `status`, union types (`string | null`), tuple types (`[string, number]`), and `Record<K, V>` — annotations only, ignored by compiler unless `--strict`. Nested lists use `T[][]` or `list<list<T>>`. `null` and `undefined` are accepted for TypeScript-compatible syntax and work with `??`.

## Type Aliases and Interfaces

`type` and `interface` declarations are parsed and silently ignored — no shell code is emitted:

```ts
type Status = "active" | "inactive"
type Result<T> = { ok: true; value: T } | { ok: false; error: string }
interface Config { host: string; port: number }
export type Callback = (data: string) => void
export interface Repository { findById(id: number): Result<string> }
```

Simple aliases such as `type Factory = string[]` can be used in later annotations. Complex unions and interfaces exist so users can write TypeScript-compatible syntax and get editor support.

## Logical Operators

```ts
let active = true && !false     // AND, NOT
let either = active || false    // OR
let fallback = maybe ?? "default" // nullish fallback only
let same = x === y              // strict equality (same as ==)
let diff = x !== y              // strict inequality (same as !=)
let sameBlock = output === `a
b`                                // multiline-safe string equality
```

`||` and `&&` in value position return actual values (JS semantics): `a || b` returns `a` if truthy, else `b`. `a && b` returns `b` if `a` is truthy, else `a`. In condition position (`if`/`while`), they return 1/0 as booleans. `a ?? b` returns `b` only when `a` is `null`/`undefined`; it preserves empty string, `0`, and `false`.

## Switch / Case

```ts
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
```

## Arrow Callbacks

Arrow callbacks support both expression-bodied and block-bodied forms for list `.map()`, `.filter()`, and `.reduce()`.

```ts
let names = ["alice", "bob", "anna"]
let upper = names.map(name => name.toUpperCase())
let aNames = upper.filter(name => name.startsWith("A"))
let copied = [...aNames, "AMY"]
let indexes = Array.from({ length: 3 })
let chosen = Array.of("alice", "anna")
let labeled = names.map((name, i) => {
    if (i == 0) return "first:" + name
    return i.toString() + ":" + name
})

// reduce with block body and object accumulator
let counts = words.reduce((acc, word) => {
    acc[word] = (acc[word] || 0) + 1
    return acc
}, {})

// reduce with expression body
let total = nums.reduce((acc, n) => acc + n, 0)
let lines = nums.reduce((acc, n) => [...acc, "#".repeat(n)], [] as string[]).join("\n")
```

`Array.from({ length })` creates `[0, 1, ... length - 1]`. `Array.of(...)` creates a list from the given values. `.map()` supports expression or block bodies and one or two parameters: `(item)` or `(item, index)`. `return` inside a block-bodied `.map()` callback emits that mapped value for the current item and continues the loop. Block-bodied `.map()` currently supports `return`, `if`/`else`, and assignment statements; arbitrary expression statements are rejected. `.filter()` uses JavaScript-style truthiness and may receive `(item, index)`. `.findIndex(callback)` returns the zero-based index of the first truthy callback result or `-1`. `.reduce()` takes a 2-parameter arrow (accumulator, current) with either expression or block body, plus an initial value. Type assertions such as `[] as string[]` are erased and are useful for empty list accumulators. List literal spread such as `[...items, extra]` is supported generically. Defer `forEach`, `some`, `every`, `find`, and general arrow function values.

## String and List Search

One-argument string `s.includes(x)`, `s.startsWith(x)`, and `s.endsWith(x)` compile through tiny `_bst_*` helper functions that are included only when generated shell calls them. Two-argument string search methods use inline `awk`. List `items.includes(x)` is different from string `.includes()`: it checks newline-delimited membership with `grep -qxF` and does not require `_bst_includes`.

## Sets and Nested Lists

```ts
const visited = new Set<string>()
visited.add("0,0")
if (visited.has("0,0")) console.log("seen")

let matrix: string[][] = ["ab", "cd"].map(row => row.split("") as string[])
console.log(matrix[0][1])
console.log(matrix[0].length)
const [row, col] = [0, 1]
console.log(matrix[row]?.[col])
```

`Set<T>` has `new Set<T>()`, `.has(value)`, and mutating `.add(value)`. Type parameters are annotations only. Nested list rows are preserved through `.map()` when the callback returns a list, and `split("")` splits a string into characters. Tuple/list destructuring declarations evaluate the source once and assign each name from list indexes. Optional element access currently lowers to normal indexing; out-of-range indexes produce the same empty value used for `undefined`.

## Objects

```ts
let user = { id: 1, name: "Victor", active: true }
let userName: string = user.name
user.name = "Compiler Tester"

// Computed property access with dynamic keys
let key: string = "name"
let val: string = user[key]
user[key] = "Updated"

// Dynamic keys must contain only letters, numbers, and _

// Objects work as function parameters — property access uses global _obj_* variables
function printUser(u) {
    console.log(u.name) // reads _obj_user_name directly
}
printUser(user)
```

## Classes

Classes support constructors, instance properties/methods, `new`, `this`, and static properties/methods. They compile to POSIX sh functions and `_obj_<slot>_<prop>` / `_class_<Class>_<prop>` variables. TypeScript-only modifiers (`private`, `public`, `protected`, `readonly`) are accepted and ignored. Inheritance, getters/setters, decorators, and abstract classes are not supported.

```ts
class User {
    name: string
    constructor(name: string) {
        this.name = name
    }
    greet(): string {
        return "Hello, " + this.name
    }
}
let u = new User("Alice")
console.log(u.greet())
u.name = "Bob"

class MathUtils {
    static PI: number = 3.14159
    static round(n: number): number {
        return Math.round(n)
    }
}
console.log(MathUtils.PI)

class Game {
    private static Deltas: Record<string, [number, number]> = { U: [-1, 0] }
    readonly matrix: string[][]
}
```

Methods that mutate `this` must be void methods. Constructors can set fields, but value-returning methods cannot assign to `this.prop` because shell command substitution would lose those writes.

## Number Builtins

```ts
let n = Number.parseInt("42", 10)
let f = Number.parseFloat("3.14")
let fin = Number.isFinite(f)
let isInt = Number.isInteger(n)
let safe = Number.isSafeInteger(n)
let nan = Number.isNaN(n) // always false for current besht values
let maxSafe = Number.MAX_SAFE_INTEGER
let minSafe = Number.MIN_SAFE_INTEGER
let eps = Number.EPSILON
```

`Number.isNaN()` is always false for currently representable besht values because besht has no NaN runtime sentinel.

## String() Builtin

`String(value)` is an alias for `to_str()`:

```ts
let label = String(count)
```

## Command Execution

All external commands use `$()` expressions. Arguments are separate strings — the compiler single-quotes everything safely.

```ts
// Capture stdout explicitly
let user: string = $("whoami").run().readStdout()
let branch: string = $("git", "rev-parse", "--abbrev-ref", "HEAD").run().readStdout()

// Capture stdout → list of lines
let logsCmd = $("find", "/var/log", "-name", "*.log")
logsCmd.run()
let logs: list<string> = logsCmd.readStdoutLines()

// Side-effect only — .run() required for statements with no capture
$("chmod", "+x", "script.sh").run()
$("git", "add", ".").run()

// Spread a list into command arguments
let args: list<string> = ["-n", "hello"]
$("echo", ...args).run()

// Pipeline chaining
let result: string = $("cat", "/etc/passwd")
    .pipe($("grep", "root"))
    .pipe($("cut", "-d:", "-f1"))
    .run()
    .readStdout()

// Redirect stdout
$("make", "build").stdout("/tmp/build.log").run()
$("echo", "line").stdout("/tmp/out.txt", "append").run()

// Redirect stderr
$("make", "build").stderr("null").run()     // 2>/dev/null
$("make", "build").stderr("&1").run()       // 2>&1
let errors: string = $("make").run().readStderr()  // stderr only

// Per-command environment variable
$("make", "build").env("CI", "1").run()     // CI=1 make build

// Per-command working directory; parent script cwd is unchanged
let root: string = $("pwd").workdir("/").run().readStdout()
$("make", "test").workdir("/repo/app").run()

// Use raw strings for patterns containing special characters
$("grep", "-v", r"^sha256").run()
$("grep", "-E", r"HOP-[0-9]{4,5}").run()
$("sed", r"s/foo/bar/g").run()
```

## Functions

```ts
function greet(name: string, times: number): string {
  let result: string = "";
  for (i in range(1, times)) {
    result = "${result}Hello, ${name}!\n";
  }
  return result;
}

let msg: string = greet("Alice", 3);
console.log(msg);
```

Void functions omit the return type:

```ts
function log(msg: string) {
  $("printf", "[LOG] %s\\n", msg).stderr("&1").run();
}
```

## Control Flow

**If / else if / else** — condition must be in parentheses. Bodies can be braced blocks or one bracketless statement:

```ts
if (count > 10) {
  $("echo", "many").run();
} else if (count > 0) {
  $("echo", "some").run();
} else {
  $("echo", "none").run();
}

if (count < 0) return "negative"
else console.log("non-negative")
```

**While:**

```ts
while (count > 0) {
  count = count - 1;
}

while (count > 0) count--
```

**C-style for:**

```ts
for (let i: number = 0; i < 10; i++) {
  $("echo", to_str(i)).run();
}

for (let i: number = 0; i < 10; i++) total += i
```

**For — integer range:**

```ts
for (i in range(1, 10)) {
  $("echo", to_str(i)).run();
}

for (i in range(1, 10)) total += i
```

**For — list:**

```ts
for (f in files) {
  $("echo", f).run();
}

for (let f of files) {
  $("echo", f).run();
}

for (f of files) $("echo", f).run()
```

**For — command output (line by line):**

```ts
for (line in $("find", "/var/log", "-name", "*.log").run().readStdoutLines()) {
  $("echo", line).run();
}
```

**Break and continue:**

````ts
for (f in files) {
    if (is_empty(f)) { continue }
    if (f == "STOP") { break }
    $("echo", f).run()
}

## List Indexing

Zero-based index access:

```ts
let first: string = args[0]
let second: string = args[1]
let item: string = args[i]    // variable index ok
args[1] = "BETA"              // index assignment
let empty: string[] = []      // empty list
let cell: string = matrix[row][col]
let width: number = matrix[0].length
````

## Error Handling

**try/catch** — catches any failing command in the block:

```ts
try {
  $("rsync", "-az", "./dist/", "server:/opt/app/").run();
  $("ssh", "server", "systemctl restart myapp").run();
} catch (code: status) {
  $("echo", "Failed with exit code " + to_str(code))
    .stderr("&1")
    .run();
  exit(1);
}
```

**`?` propagation** — fail fast inside a function:

```ts
function read_config(path: string): string {
    let content: string = $("cat", path)?
    return content
}
```

## Environment and Output

| Builtin              | Description                        |
| -------------------- | ---------------------------------- |
| `env(NAME)`          | Read environment variable          |
| `env(NAME, default)` | Read env var with fallback default |
| `console.log(s)`     | Write string + newline to stdout   |
| `console.error(s)`   | Write string + newline to stderr   |

## Built-in File Tests

These compile to inline POSIX `[ ]` tests:

```ts
if (file_exists(path)) { ... }
if (is_dir(path)) { ... }
if (is_readable(path)) { ... }
if (is_writable(path)) { ... }
if (is_executable(path)) { ... }
if (is_empty(s)) { ... }     // [ -z "$s" ]
if (is_set(s)) { ... }       // [ -n "$s" ]
```

## Built-in List Operations

```ts
let n: number = len(files)               // element count
let first: string = head(files)       // first element
let rest: list<string> = tail(files)  // all but first
files = append(files, "new.txt")      // add element
if (contains(files, "x.txt")) { ... } // membership test
```

## Imports and Modules

```ts
// lib/log.bsh
export function info(msg: string) {
  $("printf", "[INFO] %s\\n", msg).stderr("&1").run();
}

export function error(msg: string) {
  $("printf", "[ERROR] %s\\n", msg).stderr("&1").run();
}

export const cmd = ["echo", "named"]
export default ["echo", "default"]
```

```ts
// main.bsh
import defaultCmd from "./lib/log";
import { info, error, cmd } from "./lib/log";

info("Starting");
$(...cmd).run();
$(...defaultCmd).run();
// Mixed command-name spread is rejected: use $(...cmd) as the whole command vector.
```

Named imports can reference exported functions, classes, and exported top-level values. Imported value annotations/inferred types are available to non-strict codegen, so imported lists dispatch list methods like `.join()` and `.length`. Default values use `export default <expr>` and `import name from "./module"`. By default extensionless imports resolve only to `.bsh`; pass `--opt-resolve-ts-imports` to use `.ts` when `.bsh` is absent. With `--split -o build/`, each `.bsh` or opt-in `.ts` module becomes its own `.sh` that sources its dependencies at runtime.

Existing POSIX shell files can be imported only with named imports and an assertion:

```ts
import { legacy_log } from "./legacy.sh" assert { type: "shell" };
legacy_log("from besht");
```

Shell imports require a literal `.sh` path and `assert { type: "shell" }`. Default shell imports are rejected. Besht does not parse shell exports; imported shell functions are unchecked varargs and return `string` in value position. `--check` validates imports with the same module resolver as compilation. By default shell imports must stay inside the compiler root; pass `--opt-allow-external-shell-imports` to permit explicit `.sh` imports outside that root. Bundled output sources the resolved `.sh` file with a guard. Split output copies in-root `.sh` dependencies into the output tree and sources them via safely quoted `_BESHT_ROOT` paths; external opt-in shell imports are sourced from their original absolute path. Shell import guards use unique relative shell paths, so similarly named files such as `a-b.sh` and `a_b.sh` do not collide.

## Comments

```ts
// single line
/* multi
   line */
```

## Float Literals and Math

Float (decimal) literals are supported natively. All `Math.*` methods use `awk` internally and support decimals.

```ts
let price: number = 3.14;
let neg: number = -1.5;

Math.min(a, b); // minimum
Math.max(a, b); // maximum
Math.round(3.7); // 4
Math.floor(3.9); // 3
Math.ceil(3.1); // 4
Math.trunc(3.9); // 3
Math.abs(-5); // 5
Math.sign(-5); // -1
Math.pow(2, 8); // 256
Math.sqrt(16); // 4

let n = 42;
n.toString(); // "42"

let pi = 3.14159;
pi.toFixed(2); // "3.14"

let s = "hello";
s.substring(1, 4); // "ell"
s.charAt(1); // "e"
s.indexOf("l", 3); // 3, optional start position
s.lastIndexOf("l"); // 3
s.lastIndexOf("l", 2); // 2, optional backward start position
s.includes("e", 1); // true
s.startsWith("ll", 2); // true
s.endsWith("hel", 3); // true, optional length

let files = ["a", "b", "a"];
files.unshift("z"); // ["z", "a", "b", "a"]
files.lastIndexOf("a"); // 2
Array.of("a", "b"); // ["a", "b"]
```

Note: booleans still evaluate as `1`/`0` for shell conditions, but string contexts now render `true`/`false`.

## Operators

| Category            | Operators                   |
| ------------------- | --------------------------- |
| Arithmetic          | `+` `-` `*` `/` `%`         |
| Comparison (number) | `>` `<` `>=` `<=` `==` `!=` |
| Comparison (string) | `==` `!=`                   |
| Logical             | `&&` `\|\|` `!`             |
| Pipe                | `\|`                        |
| Propagate           | `?`                         |

## Exit

```ts
exit(0);
exit(code); // code: number or status
```

## Common Patterns

**Read a required argument:**

```ts
let target_env: string = env("1", "");
if (is_empty(target_env)) {
  console.log("Usage: script.bsh <env>");
  exit(1);
}
```

**Check and create a directory:**

```ts
if (!is_dir(path)) {
  $("mkdir", "-p", path).run();
}
```

**Iterate command output:**

```ts
for (file in $("find", ".", "-name", r"*.log", "-mtime", "+7").run().readStdoutLines()) {
    $("rm", file).run()
}
```

**Deploy with rollback:**

```ts
try {
  $("rsync", "-az", "./dist/", host + ":/opt/app/").run();
  $("ssh", host, "systemctl restart myapp").run();
} catch (code: status) {
  $("ssh", host, "systemctl restart myapp-previous").run();
  exit(1);
}
```

**Type conversion:**

```ts
let n: number = 42;
let s: string = to_str(n); // int → string
let raw: string = $("wc", "-l", "file.txt");
let lines: number = to_int(raw); // string → number
```

## Type Rules

- `$()` returns `command`; call `.run().readStdout()` or `.run().readStdoutLines()` to read output
- `boolean` values are `1`/`0` at runtime; use in `if`/`while` conditions directly
- `list<T>` compiles to newline-delimited strings; iterated with `for`
- `status` type holds exit codes; only usable in `catch` clauses
- String and number are coercible to each other (shell convention)
- Boolean renders as `true`/`false` in string contexts, but still uses `1`/`0` for shell conditions
- `if`/`else if`/`else`, `for`, and `while` bodies can be braced blocks or one bracketless statement; multiple statements still need braces
- Semicolons are optional — only required inside `for (init; cond; update)` headers
- `===`/`!==` are aliases for `==`/`!=` (no type distinction in shell)
- Object literals compile to per-property shell variables (`_obj_<name>_<prop>`)
- Classes use compiler-managed instance slot IDs (`let u = new User()` stores `u='u'`) and class/static shell symbols
- Object property access inside functions resolves to the original `_obj_*` variables (shell globals)
- `String.raw\`...\`` is identical to `r"..."` — backslashes are literal
- `list.join(sep)` uses awk (not `paste -sd`) to support multi-character separators
- AWK arithmetic uses `OFMT="%.17g"` for JavaScript-compatible float precision
