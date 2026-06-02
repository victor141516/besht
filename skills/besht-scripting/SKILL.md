---
name: besht-scripting
description: >
  Write, edit, and debug besht scripts (.bsh files). Use when the user wants
  to create a shell script using besht, asks how to do something in besht,
  needs to compile a .bsh file to POSIX sh, or asks about besht syntax —
  variables, constants, raw strings r"...", String.raw, $() command expressions,
  .pipe(), .stdout(), .stderr(), .readStdout(), .readStdoutLines(), .readStderr(),
  functions, if/while/for/switch, break/continue, try/catch, imports,
  list/string/number methods, Array.from({ length }), Array.of(), Array.isArray(), Object.keys(), Object.hasOwn(), JSON.stringify(), Set<T>, nested lists, object literals, classes, getters/setters, logical operators, nullish coalescing ??, Besht.args.argv()/positional()/option()/flag(), string
  concatenation, process.env.NAME, process.exit(), console.log(), value.toString(), Boolean(value), Number.parseInt(), Besht.fs.*, Besht.strings.*, Besht.iter.range(), or
  fetch(url).text().
---

Write and edit besht scripts. Besht is a TypeScript-flavored language that compiles to POSIX sh. Files use the `.bsh` extension.

## Compile

```sh
besht compile script.bsh            # print compiled sh to stdout
besht init                          # write ./stdlib.d.bsh declarations
besht init --force                  # overwrite ./stdlib.d.bsh declarations
besht compile script.bsh -o out.sh  # write to file
besht compile --check script.bsh    # validate imports, commands, and unsupported fetch APIs
besht visualize script.bsh          # inspect source and compiled shell side by side in the terminal
besht compile script.bsh | sh       # compile and run
besht compile script.bsh --split -o build/  # compile each file to its own .sh
besht compile script.bsh --opt-no-add-binaries-check  # omit runtime self-check when present
besht compile script.bsh --opt-no-source-map           # omit source comments from compiled output
besht compile script.bsh --opt-resolve-ts-imports      # allow extensionless imports to fall back to .ts
besht compile script.bsh --opt-allow-external-shell-imports  # allow explicit .sh imports outside compiler root
besht compile script.bsh --opt-use-jq                  # enable jq-backed JSON.stringify() codegen
```

Bundled one-file output omits module separator comments. Bundled output with multiple Besht modules keeps `# --- module: name ---` separators. Besht emits runtime self-checks only when generated output needs the corresponding utilities; simple direct-output scripts skip them. When options leave the runtime preamble empty, generated entry scripts keep a single blank separator between the header and the first shell statement.

`besht visualize <file.bsh>` does not write a compiled file. It opens a terminal-width viewer with Besht source on the left, including blank and unmapped source lines, and compiled shell on the right using bat-style line-number gutters; each pane is headed by the input file name and its `.sh` output name. The displayed shell omits source-map comments. Long lines wrap inside each pane with a `↳` continuation marker, and the pager disables horizontal scrolling so hidden horizontal content stays visible. When `bat` is installed and output is a terminal, the view uses TypeScript and shell syntax highlighting; otherwise it falls back to plain text.

## Variable Declarations

Types are optional. Use `let` for mutable variables and `const` for immutable ones. Besht accepts annotations for TypeScript-compatible syntax, editor support, and representation hints, but never validates them as types.

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

In template literal text, `$` stays literal unless it starts a Besht `${expr}` interpolation. Shell parameter forms such as `$*`, `$?`, and `$$` are emitted as literal text.

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

```

`process.env.NAME` is nullish only when the variable is unset. Empty strings are preserved, so use `??` for defaults. Static safe defaults compile to POSIX unset-only expansion such as `${PORT-8080}`.

## Script Arguments

```ts
let all: string[] = Besht.args.argv()
let input = Besht.args.positional(1) ?? "-"
let branch = Besht.args.option("branch", "b") ?? "main"
let dryRun = Besht.args.flag("dry-run", "d")
```

Use `??` for argument defaults. `Besht.args.positional()` and `Besht.args.option()` are nullish when absent and preserve empty strings when present. `Besht.args.flag()` returns a boolean. `--` stops option parsing, so later `-`-prefixed values are positional. Top-level scripts that only read positional arguments compile to a compact inline scan; `argv()`, `option()`, `flag()`, and args reads inside functions use the shared parser runtime.

Do not transliterate ordinary shell `while`/`case` or `getopts` parsers when they only read named options, flags, and positionals. Map them to `Besht.args.option("name", "n")`, `Besht.args.flag("verbose", "v")`, `Besht.args.positional(n)`, and `Besht.args.argv()`. Write a manual argument parser only when the original script has custom behavior that these helpers cannot express.

## Fetch

`fetch()` is currently a narrow synchronous text-only GET API backed by `curl -sS -- <url>`.

```ts
let body: string = fetch(url).text()

let response = fetch(url) // runs curl once
let first: string = response.text()
let second: string = response.text() // reuses the stored body
```

This slice supports only `fetch(url).text()` and assigned response `.text()`. It does not support `await`, options objects, POST/method overrides, headers, request bodies, `.json()`, `.status`, `.ok`, `.headers`, streaming, abort, or clone yet. `besht compile --check` rejects unsupported response properties and methods.

## Print

```ts
console.log("Usage: myscript [options]");
console.log("Result: " + value);
console.error("Something went wrong");
console.log(["a", "b"]); // [ a, b ]; static scalar list output compiles to one printf

// Objects are printed in multi-line format using current property values
// Inline object literals use a direct multi-line printf
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

```

## Besht Helpers

Use `Besht.fs.*` for file predicates, `Besht.strings.*` for string predicates, and `Besht.iter.range()` for inclusive integer ranges. These compile to inline POSIX tests or compact generated shell and add no runtime namespace object. Predicate conditions compile to direct tests, assignments store `1`/`0`, and console output renders `true`/`false`.

```ts
if (Besht.fs.isFile(path)) {
  console.log("file exists")
}

let empty: boolean = Besht.strings.isEmpty(value)
let ready = Besht.fs.isReadable(path) && Besht.fs.isExecutable(path)

Besht.fs.isFile(path)        // [ -f path ]
Besht.fs.isDir(path)         // [ -d path ]
Besht.fs.isReadable(path)    // [ -r path ]
Besht.fs.isWritable(path)    // [ -w path ]
Besht.fs.isExecutable(path)  // [ -x path ]
Besht.strings.isEmpty(value) // [ -z value ]
Besht.strings.isNonEmpty(value) // [ -n value ]
Besht.iter.range(1, 5)       // 1, 2, 3, 4, 5
```

Small static `Besht.iter.range()` loops compile to compact POSIX `for` loops; dynamic or very large ranges keep a counter loop.

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

**Types:** `string`, `number`, `boolean`, `list<T>`, `T[]`, `Array<T>`, `Set<T>`, `status`, union types (`string | null`), tuple types (`[string, number]`), and `Record<K, V>` are accepted as annotations only. Besht does not type-check. Nested lists use `T[][]` or `list<list<T>>`. `null` and `undefined` are accepted for TypeScript-compatible syntax and work with `??`.

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

`||` and `&&` in value position return actual values (JS semantics): `a || b` returns `a` if truthy, else `b`. `a && b` returns `b` if `a` is truthy, else `a`. Static known-left `||`/`&&` expressions compile directly to the selected value. In condition position (`if`/`while`), they return 1/0 as booleans. `a ?? b` returns `b` only when `a` is `null`/`undefined`; it preserves empty string, `0`, and `false`. Static `??` expressions compile to the selected side when the left side is known.

Static scalar equality comparisons, including equality comparisons against variables bound to static string literals, and static numeric relational comparisons compile to constants, including comparisons over already-folded arithmetic, string methods/transforms, `Math.*`, and parseable `Number.parseInt()`/`Number.parseFloat()` calls. Dynamic relational comparisons over compiler-known integer expressions use POSIX `[ ]`; floats and unknown values keep the `awk` path. Dynamic equality keeps the multiline-safe shell path.

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

Arrow callbacks support both expression-bodied and block-bodied forms for list `.map()`, `.filter()`, `.reduce()`, and statement-position `.forEach()`. Scalar-list `.some()`, `.every()`, `.find()`, and `.findIndex()` use direct expression-bodied arrow predicates.

When translating shell pipelines that transform already-known text or numbers, prefer native Besht lists and methods over external `sed`/`awk`/`grep`/`tr` commands. For example, a shell pipeline that trims lines, filters by prefix, uppercases, joins labels, sums numbers, or prints `NR - 1` indexes is usually clearer as `list.map(...)`, `filter(...)`, `word.trim()`, `startsWith(...)`, `toUpperCase()`, `join(...)`, `reduce(...)`, and `forEach((item, index) => ...)`. Reserve command pipelines for real external data sources or operations that need an external tool.

For literal delimiter-separated records or shell snippets that use `awk -F:`, `cut`, `paste`, or repeated membership probes over a static table, model the data directly. Use object literals for key/value records, `Object.keys()`, `Object.entries()`, and `Object.hasOwn()` for enumeration and probes, `Set<T>` for membership groups, and `JSON.stringify()` for JSON output instead of preserving the text-processing pipeline.

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
let initials = ""
names.forEach((name, i) => {
    console.log(i.toString() + ":" + name)
    initials = initials + name.charAt(0)
})
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

let rawWords = ["  alpha", "Beta", "apricot", "banana"]
let cleaned = rawWords.map(word => word.trim())
let labels = cleaned
    .filter(word => word.startsWith("a"))
    .map(word => "item:" + word)
    .join(", ")
```

`Array.from({ length })` differs from JavaScript: it creates `[0, 1, ... length - 1]` and does not support general iterables or mapper callbacks. `Array.of(...)` creates a list from the given values. `Array.isArray(value)` is a static predicate: it returns true for values Besht can infer as lists and false otherwise, without runtime shape metadata. `.map()` supports expression or block bodies and one or two parameters: `(item)` or `(item, index)`. `return` inside a block-bodied `.map()` callback emits that mapped value for the current item and continues the loop. Block-bodied `.map()` currently supports `return`, `if`/`else`, and assignment statements; arbitrary expression statements are rejected. `.filter()`, `.some()`, `.every()`, `.find()`, and `.findIndex()` use JavaScript-style truthiness and may receive `(item, index)`. `.some(callback)` returns true for the first truthy callback result and false for an empty list. `.every(callback)` returns false for the first falsey callback result and true for an empty list. `.find(callback)` returns the first matching scalar element, or nullish when no item matches so `??` fallbacks work. `.findIndex(callback)` returns the zero-based index of the first truthy callback result or `-1`. `.reduce()` takes a 2-parameter arrow (accumulator, current) with either expression or block body, plus an initial value. `.forEach()` is statement-only, takes a direct arrow callback with `(item)` or `(item, index)`, compiles static scalar receivers to compact `for` loops, preserves outer assignment and `Set.add()` side effects, and rejects callback `return`, `break`, `continue`, and pure value expressions. Type assertions such as `[] as string[]` are erased and are useful for empty list accumulators. List literal spread such as `[...items, extra]` is supported generically. Defer general arrow function values.

Inline static scalar object literal `Object.keys()`, `Object.values()`, `Object.entries()`, and `Object.hasOwn()` calls compile to constants. Unmutated named object `Object.keys()`, static-scalar `Object.values()`/`Object.entries()`, and static-key `Object.hasOwn()` calls also fold from compiler-managed static object metadata; mutated or dynamic objects keep metadata-backed output so assignments stay visible.

## String and List Search

Strings support `includes()`, `startsWith()`, and `endsWith()`, including optional search-position or length arguments. Static ASCII string expressions built from literals, variables bound to static ASCII strings, concatenation, template interpolation, and chained transforms fold `[index]`, searches, and `charAt()` calls with static arguments to constants. Lists also support `items.includes(x)` for exact item membership.

Static ASCII string expressions built from literals, variables bound to static ASCII strings, concatenation, template interpolation, and chained transforms fold transforms such as `.trim()`, `.toUpperCase()`, `.slice()`, `.substring()`, `.repeat()`, `.replace()`/`.replaceAll()`, `.concat()`, and `.padStart()`/`.padEnd()` with static arguments to constants. Dynamic and non-ASCII transforms use POSIX tools; dynamic string `slice()`, `at()`, and indexing support normal substring extraction.

Simple prefix-strip ternaries such as `s.startsWith("#") ? s.slice(1) : s` compile compactly while keeping the same Besht behavior.

Static ASCII string literal `.split()` calls with static separators compile to constants when the resulting list elements contain no newlines; variables bound to static ASCII strings can use the same split folding. Dynamic and non-ASCII splits use POSIX tools.

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

`Set<T>` has `new Set<T>()`, `.has(value)`, and mutating `.add(value)`. Type parameters are annotations only. Straight-line static scalar adds and static membership checks compile compactly; dynamic values and callback/control-flow adds keep runtime membership checks. Nested list rows are preserved through `.map()` when the callback returns a list, and static nested indexes with known row and column fold to constants. `split("")` splits a string into characters. Tuple/list destructuring declarations evaluate the source once and assign each name from list indexes; static scalar list destructuring emits direct assignments. Optional chaining supports `obj?.prop`, `obj?.[key]`, `obj?.method()`, and nested chains; it short-circuits only on `null`/`undefined` and composes with `??`. General `fn?.()`, `obj.method?.()`, and optional assignment targets are not supported.

## Objects

```ts
let user = { id: 1, name: "Victor", active: true }
let userName: string = user.name
user.name = "Compiler Tester"
let keys: string[] = Object.keys(user)
let values: string[] = Object.values(user)
let entries: string[][] = Object.entries(user)
let hasName: boolean = Object.hasOwn(user, "name")
let json: string = JSON.stringify(user) // compile with --opt-use-jq

// Computed property access with dynamic keys
let key: string = "name"
let val: string = user[key]
user[key] = "Updated"

// Static and dynamic keys must contain only letters, numbers, and _

// Objects work as function parameters
function printUser(u: object) {
    console.log(u.name)
}
printUser(user)

function showKeys(obj: object): string[] {
    return Object.keys(obj)
}
```

`Object.keys(obj)`, `Object.values(obj)`, and `Object.entries(obj)` return keys, scalar values, or `[key, value]` rows in insertion order, including aliases, object parameters, and later dot or computed-key assignments. This differs from JavaScript runtime reflection: Besht uses compiler-managed object metadata, object keys must contain only letters, numbers, and `_`, and `process.env` is not enumerable. Unmutated named object key lists, static-scalar value and entry lists, static-key `Object.hasOwn()` calls, and safe direct reads of scalar properties from static object literal bindings compile to constants. Statically known boolean values are rendered as `true`/`false` in `Object.values()` and `Object.entries()` output. Static boolean object properties used directly in conditions can fold to the selected branch; dynamic boolean object properties compile to direct shell tests. Non-boolean property conditions keep JavaScript-style truthiness. `Object.values()` and `Object.entries()` reject statically known list/object/set/command/fetch values because the current list representations cannot preserve deeper nested object values. `Object.hasOwn(obj, key)` checks exact key membership against the same compiler-managed metadata and returns `false` for invalid dynamic key strings. These helpers do not add a runtime helper library.

`JSON.stringify(value)` differs from JavaScript: it encodes only strings, numbers, booleans, scalar lists, and scalar-valued compiler-managed objects, requires `--opt-use-jq`, and invokes `jq` in generated code. The runtime self-check verifies `jq` exists only when JSON code is emitted.

```ts
console.log(JSON.stringify({ id: 7, name: "Ada", active: true }))
console.log(JSON.stringify(["Ada", "Grace"]))
console.log(JSON.stringify(Number.parseInt("2a", 10)))
```

`JSON.parse()` is not supported.

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

Use JS-style conversion APIs for new code. `value.toString()` works on `string`, `number`, `boolean`, and `status`; booleans render as `true` or `false`. `Number.parseInt(value)` accepts one argument or an optional radix argument, including non-decimal radix values such as 16.

```ts
let countText = count.toString()
let flagText = flag.toString()
let lines = Number.parseInt(raw)
let lines10 = Number.parseInt(raw, 10)
let red = Number.parseInt(hexByte, 16)
```

Static numeric arithmetic over literal numbers and variables bound to static numeric expressions compiles to constants. Dynamic arithmetic and variables assigned inside control flow keep shell arithmetic or POSIX `awk`.

Static string literal `Number.parseInt()` calls with parseable prefixes and static radix compile to numeric constants, such as `Number.parseInt("2a", 10)` to `2`. Static numeric literal, static numeric expression, static numeric variable, and static numeric API receiver `.toString()` and `.toFixed()` calls also compile to constants. Static primitive `.toString()` calls in direct bindings, string concatenation, and template interpolation compile to constants; dynamic receivers keep the normal runtime formatting path.

## Command Execution

All external commands use `$()` expressions. Arguments are separate strings. The compiler emits conservative shell-safe literal command words bare when possible and quotes anything that needs protection.

Before translating a shell pipeline into `$()` calls, ask whether the pipeline is actually processing external command output or only a literal/static variable from the script. Literal data such as `TEAM='ada:admin:yes...'` piped through `awk`, `cut`, `paste`, `grep`, or `sed` should become Besht data structures and methods, not a command pipeline.

```ts
// Shell shape: printf "$TEAM" | awk -F: '$3 == "yes" { print $1 "=" $2 }'
let roles = { ada: "admin", grace: "member", linus: "member", ken: "guest" }
let active = new Set<string>()
active.add("ada")
active.add("grace")
Object.entries(roles).forEach(entry => {
    if (active.has(entry[0])) {
        console.log(entry[0] + "=" + entry[1])
    }
})
```

When translating shell-style scripts, translate structure into command methods instead of embedding shell syntax in strings:

| Shell idiom | Besht pattern |
| ----------- | ------------- |
| `cmd arg "$value"` | `$("cmd", "arg", value).run()` |
| `cmd1 \| cmd2 \| cmd3` | `$("cmd1").pipe($("cmd2")).pipe($("cmd3")).run()` |
| `out=$(cmd)` | `let out = $("cmd").run().readStdout()` |
| `cmd \| while read ...` for line output | `for (line in $("cmd").run().readStdoutLines()) { ... }` |
| `cmd > file`, `cmd >> file` | `$("cmd").stdout(file).run()`, `$("cmd").stdout(file, "append").run()` |
| `cmd >/dev/null`, `cmd 2>/dev/null`, `cmd 2>&1` | `.stdout("null")`, `.stderr("null")`, `.stderr("&1")` |
| `(cd dir && cmd)` | `$("cmd").workdir(dir).run()` |
| `VAR=value cmd` | `$("cmd").env("VAR", value).run()` |
| `cmd && next` | run a named command, inspect `.exitCode()`, then use `if` |
| `${1-default}` | `Besht.args.positional(1) ?? "default"` |
| `${1:-default}` | read the positional arg, then use `Besht.strings.isEmpty()` to apply the empty-string default |
| `while`/`case` parser for `--root`, `-r`, `--verbose` | `Besht.args.option("root", "r")`, `Besht.args.flag("verbose", "v")`, `Besht.args.positional(n)` |
| `printf "$TEAM" \| awk -F: ...` over a literal table | object literals, `Set<T>`, `Object.entries()`, `Object.hasOwn()`, list callbacks, `JSON.stringify()` |

Avoid `$("sh", "-c", "...")`, `$("bash", "-c", "...")`, embedded `cd`, `VAR=value cmd`, `cmd1 | cmd2`, or redirect text inside command strings unless the script's real purpose is to invoke a shell interpreter. Besht should own quoting, argument boundaries, pipes, redirects, per-command environment, and per-command working directory. Use raw strings (`r"..."`) for grep/sed/awk patterns and globs that must stay literal.

```ts
// Capture stdout explicitly
let user: string = $("whoami").run().readStdout()
let branch: string = $("git", "rev-parse", "--abbrev-ref", "HEAD").run().readStdout()
console.log($("pwd").run().readStdout()) // inline reads can compile directly to "$(pwd)"

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
// Single-command redirects compile directly; pipeline redirects are grouped.

// Redirect stderr
$("make", "build").stderr("null").run()     // 2>/dev/null
$("make", "build").stderr("&1").run()       // 2>&1
let errors: string = $("make").run().readStderr()  // stderr only

// Per-command environment variable
$("make", "build").env("CI", "1").run()     // CI=1 make build

// Per-command working directory; parent script cwd is unchanged
let root: string = $("pwd").workdir("/").run().readStdout()
$("make", "test").workdir("/repo/app").run()

// Shell-style cmd && next, expressed through exit code inspection
let probe = $("find", ".", "-maxdepth", "1", "-type", "f").stdout("null").stderr("null")
probe.run()
if (probe.exitCode() == 0) {
  console.log("ok")
}

// Shell ${1:-.}: default on missing OR empty positional arg
let root = Besht.args.positional(1) ?? "."
if (Besht.strings.isEmpty(root)) {
  root = "."
}

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
  for (i in Besht.iter.range(1, times)) {
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

Static boolean conditions such as `if (Boolean("x"))`, `Array.isArray(value) ? a : b`, static string/list searches, static `Object.hasOwn()`, static comparisons, and variables bound to static boolean expressions compile to only the selected branch or value. Dynamic conditions and variables assigned inside control flow keep normal POSIX shell tests.

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
  $("echo", i.toString()).run();
}

for (let i: number = 0; i < 10; i++) total += i
```

**For — integer range:**

```ts
for (i in Besht.iter.range(1, 10)) {
  $("echo", i.toString()).run();
}

for (i in Besht.iter.range(1, 10)) total += i
```

Small static range bounds emit compact `for i in ...; do` shell. Use variables freely when the bounds are dynamic.

**For — list:**

```ts
for (f in files) {
  $("echo", f).run();
}

for (let f in files) {
  $("echo", f).run();
}

for (f in files) $("echo", f).run()
```

TypeScript `for...of` loops are not supported.

Static scalar list expressions, static scalar `Array.of(...)` calls, static `Array.from({ length: N })` calls, and variables bound to them compile to compact shell `for` loops when elements do not contain newlines. Dynamic lists use Besht's newline-safe read loop.

**For — command output (line by line):**

```ts
for (line in $("find", "/var/log", "-name", "*.log").run().readStdoutLines()) {
  $("echo", line).run();
}
```

**Break and continue:**

````ts
for (f in files) {
    if (Besht.strings.isEmpty(f)) { continue }
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

Static scalar list indexes with known in-range integer indexes and static nested-list indexes with known row/column indexes compile to constants. Dynamic, unknown, and out-of-range indexes keep the POSIX `sed`/packed-row extraction path.

## Error Handling

**try/catch** — catches any failing command in the block:

```ts
try {
  $("rsync", "-az", "./dist/", "server:/opt/app/").run();
  $("ssh", "server", "systemctl restart myapp").run();
} catch (code: status) {
  $("echo", "Failed with exit code " + code.toString())
    .stderr("&1")
    .run();
  process.exit(1);
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
| `console.log(s)`     | Write string + newline to stdout   |
| `console.error(s)`   | Write string + newline to stderr   |
| `process.env.NAME`   | Read environment variable          |

## Besht File and String Tests

```ts
if (Besht.fs.isFile(path)) { ... }
if (Besht.fs.isDir(path)) { ... }
if (Besht.fs.isReadable(path)) { ... }
if (Besht.fs.isWritable(path)) { ... }
if (Besht.fs.isExecutable(path)) { ... }
if (Besht.strings.isEmpty(s)) { ... }       // [ -z "$s" ]
if (Besht.strings.isNonEmpty(s)) { ... }    // [ -n "$s" ]
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
let literalText: string = ["a", "b", "c"].join(",") // compiles to 'a,b,c'
let compactText: string = ["a", "b"].concat(["c"]).join(",") // compiles to 'a,b,c'
let hasConfig: boolean = files.includes("config.txt")
let allFiles = files.concat(otherFiles)
```

Scalar `list.toString()` is supported as comma-join output; nested-list JavaScript flattening is not part of this slice. Static scalar list literals and variables bound to static scalar lists fold `.join()` and `.toString()` calls to one quoted string when elements contain no newlines and the separator is static.

Static scalar list literals and list-returning method chains over static scalar lists (`concat`, `slice`, `reverse`, `push`, `unshift`, `pop`, `shift`) compile to quoted newline-backed shell strings when values do not contain newlines; dynamic, spread, nested, and newline-sensitive lists keep the generated `printf` builder.

Static scalar `Array.of(...)` calls and Besht's narrow static `Array.from({ length: N })` calls compile to quoted newline-backed shell strings when values contain no newlines; dynamic factories keep the generated builder. Besht `Array.from({ length })` creates a numeric range and does not support JavaScript's general iterable or mapper forms.

Static string literals, variables bound to static string literals, static scalar list expressions, and variables bound to static scalar lists compile `.length` properties to numeric constants; dynamic lengths use POSIX `wc`.

Static scalar list literals and variables bound to static scalar lists fold `.includes()`, `.indexOf()`, and `.lastIndexOf()` calls to constants when the needle is static. Dynamic list searches keep the POSIX `grep`/`awk` path.

Static scalar list indexes with known in-range integer indexes and static nested-list indexes with known row/column indexes compile to constants. Dynamic, unknown, and out-of-range indexes keep the POSIX `sed`/packed-row path.

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

Named imports can reference exported functions, classes, and exported top-level values. Default values use `export default <expr>` and `import name from "./module"`. By default extensionless imports resolve only to `.bsh`; pass `--opt-resolve-ts-imports` to use `.ts` when `.bsh` is absent. With `besht compile --split -o build/`, each `.bsh` or opt-in `.ts` module is compiled separately.

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

Float (decimal) literals are supported. `Math.*` methods support decimals. Literal-argument `Math.*` calls, including arguments from variables bound to static numeric expressions, compile to constants; dynamic calls use POSIX `awk`.

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
s[1]; // "e"
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

Note: booleans print as `true`/`false` in string contexts and can be used directly in conditions. Static boolean `console.log()` and `console.error()` arguments such as `Boolean("")`, `true`, `!false`, static comparisons, and variables bound to static boolean expressions render directly without a shell `if`; `Besht.fs.*` and `Besht.strings.*` predicates also render as readable `true`/`false` in console output, and dynamic boolean console arguments reuse the condition once and print `true`/`false` from it. Static boolean `if` and ternary conditions such as `Boolean(value)`, `Array.isArray(value)`, static string/list searches, static `Object.hasOwn()`, and static comparisons fold to the selected branch or value.

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
process.exit(0);
process.exit(code); // code: number or status
```

## Common Patterns

**Read a required argument:**

```ts
let target_env: string = Besht.args.positional(1) ?? "";
if (Besht.strings.isEmpty(target_env)) {
  console.log("Usage: script.bsh <env>");
  process.exit(1);
}
```

**Check and create a directory:**

```ts
if (!Besht.fs.isDir(path)) {
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
  process.exit(1);
}
```

**Type conversion:**

```ts
let n: number = 42;
let s: string = n.toString(); // number -> string
let raw: string = $("wc", "-l", "file.txt").run().readStdout();
let lines: number = Number.parseInt(raw); // string -> number

// Older helpers remain supported for now:
```

## Type Rules

- `$()` returns `command`; call `.run().readStdout()` or `.run().readStdoutLines()` to read output
- `boolean` values work directly in `if`/`while` conditions and render as `true`/`false` in string contexts
- `list<T>` values can be indexed, joined, and iterated with `for`
- `status` type holds exit codes; only usable in `catch` clauses
- String, number, boolean, and status values can be converted with `.toString()`; strings can be parsed with `Number.parseInt()`
- `if`/`else if`/`else`, `for`, and `while` bodies can be braced blocks or one bracketless statement; multiple statements still need braces
- Semicolons are optional — only required inside `for (init; cond; update)` headers
- `===`/`!==` are aliases for `==`/`!=`
- Objects and classes support the operations described above; unsupported TypeScript features are listed in their sections
- `String.raw\`...\`` is identical to `r"..."` — backslashes are literal
- `list.join(sep)` supports multi-character separators
- Scalar `list.toString()` is supported as `list.join(",")`; nested-list JS flattening is not implemented
