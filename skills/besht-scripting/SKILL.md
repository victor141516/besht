---
name: besht-scripting
description: >
  Write, edit, and debug besht scripts (.bsh files). Use when the user wants
  to create a shell script using besht, asks how to do something in besht,
  needs to compile a .bsh file to POSIX sh, or asks about besht syntax —
  variables, constants, raw strings r"...", String.raw, $() command expressions,
  .pipe(), .stdout(), .stderr(), .readStdout(), .readStdoutLines(), .readStderr(),
  functions, if/while/for/switch, break/continue, try/catch, imports,
  list/string/number methods, Array.from({ length }), Array.of(), Array.isArray(), Set<T>, nested lists, object literals, classes, getters/setters, logical operators, nullish coalescing ??, args.argv()/positional()/option()/flag(), string
  concatenation, process.env.NAME, process.exit(), env(), console.log(), value.toString(), Number.parseInt(), to_str(), String(), to_int(), Besht.conditions.* wrappers, or
  fetch(url).text(), built-in functions like file_exists, len, and range.
---

Write and edit besht scripts. Besht is a TypeScript-flavored language that compiles to POSIX sh. Files use the `.bsh` extension.

## Compile

```sh
besht script.bsh                    # print compiled sh to stdout
besht init                          # write ./stdlib.d.bsh declarations
besht init --force                  # overwrite ./stdlib.d.bsh declarations
besht script.bsh -o out.sh          # write to file
besht --check script.bsh            # validate imports, commands, and unsupported fetch APIs
besht --check --strict script.bsh   # type-check with validation
besht script.bsh | sh               # compile and run
besht script.bsh --split -o build/  # compile each file to its own .sh
besht script.bsh --opt-no-add-binaries-check  # omit runtime self-check
besht script.bsh --opt-no-source-map            # omit source comments from compiled output
besht script.bsh --opt-resolve-ts-imports       # allow extensionless imports to fall back to .ts
besht script.bsh --opt-allow-external-shell-imports  # allow explicit .sh imports outside compiler root
```

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

Use raw strings (`r"..."`) for regex patterns, AWK programs, sed expressions, Windows paths, or any text containing `$`, `^`, `[`, or `\` that must stay literal. `String.raw\`...\`` is identical to `r"..."`.

Escape sequences in double-quoted strings: `\n` (newline), `\t` (tab), `\r` (carriage return), `\\` (backslash), `\"` (double quote), `\'` (single quote), `\uXXXX` (unicode). Single-quoted strings do NOT process escapes.

## Environment Variables

```ts
let home: string = process.env.HOME
let port: string = process.env.PORT ?? "8080"
let debug: string = process.env.DEBUG ?? "false"

// Older helper remains supported for now:
let legacyHome: string = env("HOME")
let legacyPort: string = env("PORT", "8080")
```

`process.env.NAME` is nullish only when the variable is unset. Empty strings are preserved, so use `??` for defaults.

## Script Arguments

```ts
let all: string[] = args.argv()
let input = args.positional(1) ?? "-"
let branch = args.option("branch", "b") ?? "main"
let dryRun = args.flag("dry-run", "d")
```

Use `??` for argument defaults. `args.positional()` and `args.option()` are nullish when absent and preserve empty strings when present. `args.flag()` returns a boolean. `--` stops option parsing, so later `-`-prefixed values are positional.

## Fetch

`fetch()` is currently a narrow synchronous text-only GET API backed by `curl -sS -- <url>`.

```ts
let body: string = fetch(url).text()

let response = fetch(url) // runs curl once
let first: string = response.text()
let second: string = response.text() // reuses the stored body
```

This slice supports only `fetch(url).text()` and assigned response `.text()`. It does not support `await`, options objects, POST/method overrides, headers, request bodies, `.json()`, `.status`, `.ok`, `.headers`, streaming, abort, or clone yet. `besht --check` rejects unsupported response properties and methods even without `--strict`.

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

## Process Exit

```ts
process.exit()
process.exit(7)
process.exit(code) // number or status

// Older helper remains supported for now:
exit(0)
```

## Condition Helpers

Use `Besht.conditions.*` for file and string predicates. They compile to inline POSIX tests and add no runtime helpers. Older global names such as `file_exists()` and `is_empty()` remain supported for now.

```ts
if (Besht.conditions.fileExists(path)) {
  console.log("file exists")
}

let empty: boolean = Besht.conditions.isEmpty(value)
let ready = Besht.conditions.isReadable(path) && Besht.conditions.isExecutable(path)

Besht.conditions.fileExists(path)   // file_exists(path), [ -f path ]
Besht.conditions.isDir(path)        // is_dir(path), [ -d path ]
Besht.conditions.isReadable(path)   // is_readable(path), [ -r path ]
Besht.conditions.isWritable(path)   // is_writable(path), [ -w path ]
Besht.conditions.isExecutable(path) // is_executable(path), [ -x path ]
Besht.conditions.isEmpty(value)     // is_empty(value), [ -z value ]
Besht.conditions.isSet(value)       // is_set(value), [ -n value ]
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

`type` and `interface` declarations are accepted for TypeScript-compatible syntax and editor support:

```ts
type Status = "active" | "inactive"
type Result<T> = { ok: true; value: T } | { ok: false; error: string }
interface Config { host: string; port: number }
export type Callback = (data: string) => void
export interface Repository { findById(id: number): Result<string> }
```

Simple aliases such as `type Factory = string[]` can be used in later annotations. Complex unions and interfaces exist so users can write TypeScript-compatible syntax and get editor support.

`.d.bsh` files are declaration-only and never emit shell output. Declared functions are called by their declared names; besht does not generate wrappers for them. Import declaration files explicitly for shared declarations, or run `besht init` to create `./stdlib.d.bsh` with standard declarations beside the entry script. That file auto-loads for normal compile, `--split`, and `--check`. `besht init` leaves matching files unchanged and requires `--force` before overwriting different content. Only the entry script directory is scanned; imported module directories are not scanned for their own `stdlib.d.bsh` files.

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

Arrow callbacks support both expression-bodied and block-bodied forms for list `.map()`, `.filter()`, and `.reduce()`. Scalar-list `.some()`, `.every()`, `.find()`, and `.findIndex()` use direct expression-bodied arrow predicates.

```ts
let names = ["alice", "bob", "anna"]
let upper = names.map(name => name.toUpperCase())
let aNames = upper.filter(name => name.startsWith("A"))
let hasAnna = names.some(name => name == "anna")
let allShort = names.every((name, i) => name.length < 10 && i >= 0)
let firstB = names.find(name => name.startsWith("b")) ?? "none"
let copied = [...aNames, "AMY"]
let indexes = Array.from({ length: 3 })
let chosen = Array.of("alice", "anna")
let chosenIsList = Array.isArray(chosen)
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

`Array.from({ length })` creates `[0, 1, ... length - 1]`. `Array.of(...)` creates a list from the given values. `Array.isArray(value)` is a static predicate: it returns true for values Besht can infer as lists and false otherwise, without runtime shape metadata. `.map()` supports expression or block bodies and one or two parameters: `(item)` or `(item, index)`. `return` inside a block-bodied `.map()` callback emits that mapped value for the current item and continues the loop. Block-bodied `.map()` currently supports `return`, `if`/`else`, and assignment statements; arbitrary expression statements are rejected. `.filter()`, `.some()`, `.every()`, `.find()`, and `.findIndex()` use JavaScript-style truthiness and may receive `(item, index)`. `.some(callback)` returns true for the first truthy callback result and false for an empty list. `.every(callback)` returns false for the first falsey callback result and true for an empty list. `.find(callback)` returns the first matching scalar element, or nullish when no item matches so `??` fallbacks work. `.findIndex(callback)` returns the zero-based index of the first truthy callback result or `-1`. `.reduce()` takes a 2-parameter arrow (accumulator, current) with either expression or block body, plus an initial value. Type assertions such as `[] as string[]` are erased and are useful for empty list accumulators. List literal spread such as `[...items, extra]` is supported generically. Defer `forEach` and general arrow function values.

## String and List Search

Strings support `includes()`, `startsWith()`, and `endsWith()`, including optional search-position or length arguments. Lists also support `items.includes(x)` for exact item membership.

## Sets and Nested Lists

```ts
const visited = new Set<string>()
visited.add("0,0")
if (visited.has("0,0")) console.log("seen")

let matrix: string[][] = ["ab", "cd"].map(row => row.split("") as string[])
console.log(matrix[0][1])
console.log(matrix[0].length)
const [row, col] = [0, 1]
console.log(matrix?.[row]?.[col] ?? "missing")
```

`Set<T>` has `new Set<T>()`, `.has(value)`, and mutating `.add(value)`. Type parameters are annotations only. Nested list rows are preserved through `.map()` when the callback returns a list, and `split("")` splits a string into characters. Tuple/list destructuring declarations evaluate the source once and assign each name from list indexes. Optional chaining supports `obj?.prop`, `obj?.[key]`, `obj?.method()`, and nested chains; it short-circuits only on `null`/`undefined` and composes with `??`. General `fn?.()`, `obj.method?.()`, and optional assignment targets are not supported.

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

// Objects work as function parameters
function printUser(u) {
    console.log(u.name)
}
printUser(user)
```

## Classes

Classes support constructors, instance properties/methods, `new`, `this`, static properties/methods, and getters/setters. TypeScript-only modifiers (`private`, `public`, `protected`, `readonly`) are accepted and ignored. Inheritance, decorators, and abstract classes are not supported. Generated shell keeps `# besht:file:line:col` comments before explicit class constructor, accessor, and method functions unless `--opt-no-source-map` is used.

```ts
class User {
    name: string
    constructor(name: string) {
        this.name = name
    }
    greet(): string {
        return "Hello, " + this.name
    }
    get label(): string {
        return this.name
    }
    set label(value: string) {
        this.name = value
    }
}
let u = new User("Alice")
console.log(u.greet())
console.log(u.label)
u.name = "Bob"
u.label = "Carol"

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

class Game {
    private static Deltas: Record<string, [number, number]> = { U: [-1, 0] }
    readonly matrix: string[][]
}
```

Getters take no parameters, must return a value, and cannot assign to `this.prop`. Setters take exactly one parameter and do not return a value. Methods that mutate `this` must be void methods. Constructors and setters can set fields, but value-returning methods cannot assign to `this.prop`.

## Number Builtins

```ts
let n = Number.parseInt("42")
let n10 = Number.parseInt("42", 10)
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

## Type Conversion

Use JS-style conversion APIs for new code. `value.toString()` works on `string`, `number`, `boolean`, and `status`; booleans render as `true` or `false`. `Number.parseInt(value)` accepts one argument or an optional radix argument.

```ts
let countText = count.toString()
let flagText = flag.toString()
let lines = Number.parseInt(raw)
let lines10 = Number.parseInt(raw, 10)
```

Older helpers remain supported for now: `String(value)`, `to_str(value)`, and `to_int(value)`.

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

## Optional Chaining

```ts
let name = user?.name ?? "anonymous"
let item = items?.[i] ?? "fallback"
let trimmed = maybeText?.trim() ?? ""
```

Optional chaining returns a nullish value when the receiver is `null` or `undefined`; `??` can then provide a fallback without treating `""`, `0`, or `false` as missing.

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

Available file and string tests:

```ts
if (file_exists(path)) { ... }
if (is_dir(path)) { ... }
if (is_readable(path)) { ... }
if (is_writable(path)) { ... }
if (is_executable(path)) { ... }
if (is_empty(s)) { ... }     // [ -z "$s" ]
if (is_set(s)) { ... }       // [ -n "$s" ]
```

## Native List Operations

Prefer TypeScript-style list syntax and methods in new code:

```ts
let n: number = files.length
let first: string = files[0]
let rest: string[] = files.slice(1)
let withNew = files.push("new.txt")
let alsoWithNew = [...files, "new.txt"]
let text: string = files.toString() // scalar lists: same as files.join(",")
let hasConfig: boolean = files.includes("config.txt")
let allFiles = files.concat(otherFiles)
```

Compatibility aliases remain supported for now: `len(files)`, `head(files)`, `tail(files)`, `append(files, value)`, `contains(files, value)`, and `concat(files, otherFiles)`. Scalar `list.toString()` is supported as comma-join output; nested-list JavaScript flattening is not part of this slice.

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

Named imports can reference exported functions, classes, and exported top-level values. Default values use `export default <expr>` and `import name from "./module"`. By default extensionless imports resolve only to `.bsh`; pass `--opt-resolve-ts-imports` to use `.ts` when `.bsh` is absent. With `--split -o build/`, each `.bsh` or opt-in `.ts` module is compiled separately.

Existing POSIX shell files can be imported only with named imports and an assertion:

```ts
import { legacy_log } from "./legacy.sh" assert { type: "shell" };
legacy_log("from besht");
```

Shell imports require a literal `.sh` path and `assert { type: "shell" }`. Default shell imports are rejected. Imported shell functions are unchecked varargs and return `string` in value position. `--check` validates imports and unsupported fetch response APIs. By default shell imports must stay inside the compiler root; pass `--opt-allow-external-shell-imports` to permit explicit `.sh` imports outside that root.

## Comments

```ts
// single line
/* multi
   line */
```

## Float Literals and Math

Float (decimal) literals are supported. `Math.*` methods support decimals.

If you reassign a variable, besht updates its float-tracking metadata from the new right-hand side: float-producing expressions keep later arithmetic on `awk`, while integer or non-float reassignment clears the float marker so later integer arithmetic can use shell integer lowering again.

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
let ok = true;
ok.toString(); // "true"

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
files.toString(); // "z,a,b,a" for scalar lists
Array.of("a", "b"); // ["a", "b"]
Array.isArray(files); // true for compiler-known lists
```

Note: booleans print as `true`/`false` in string contexts and can be used directly in conditions.

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
let s: string = n.toString(); // number -> string
let raw: string = $("wc", "-l", "file.txt");
let lines: number = Number.parseInt(raw); // string -> number

// Older helpers remain supported for now:
let legacyS: string = to_str(n);
let legacyLines: number = to_int(raw);
```

## Type Rules

- `$()` returns `command`; call `.run().readStdout()` or `.run().readStdoutLines()` to read output
- `boolean` values work directly in `if`/`while` conditions and render as `true`/`false` in string contexts
- `list<T>` values can be indexed, joined, and iterated with `for`
- `status` type holds exit codes; only usable in `catch` clauses
- String, number, boolean, and status values can be converted with `.toString()`; strings can be parsed with `Number.parseInt()`. Older `String()`, `to_str()`, and `to_int()` helpers remain supported for now
- `if`/`else if`/`else`, `for`, and `while` bodies can be braced blocks or one bracketless statement; multiple statements still need braces
- Semicolons are optional — only required inside `for (init; cond; update)` headers
- `===`/`!==` are aliases for `==`/`!=`
- Objects and classes support the operations described above; unsupported TypeScript features are listed in their sections
- `String.raw\`...\`` is identical to `r"..."` — backslashes are literal
- `list.join(sep)` supports multi-character separators
- Scalar `list.toString()` is supported as `list.join(",")`; nested-list JS flattening is not implemented
