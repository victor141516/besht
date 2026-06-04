# besht

A TypeScript-flavored language that compiles to POSIX sh.

Write shell scripts that are readable and portable. Besht accepts TypeScript-style annotations for editor help and representation hints, but it does not type-check them.

---

## Installation

```sh
brew install victor141516/tap/besht
```

---

## Quick start

```sh
# Compile to stdout (single bundled file)
besht compile script.bsh

# Generate editor declarations in ./stdlib.d.bsh
besht init

# Compile to a single bundled file
besht compile script.bsh -o script.sh

# Compile each .bsh file to its own .sh file (recommended for multi-file projects)
besht compile script.bsh --split -o build/

# Validate imports, command usage, and unsupported fetch APIs
besht compile --check script.bsh

# View source and compiled shell side by side in the terminal
besht visualize script.bsh

# Opt in to jq-backed JSON support
besht compile script.bsh --opt-use-jq
# Run directly
besht compile script.bsh | sh
```

## Output modes

### Compile (`compile`)

All imported Besht modules are inlined into a single `.sh` file. Explicit `.sh` imports are sourced from the generated script. Good for one-file scripts and piping to `sh`.

One-file bundled output omits module separator comments for a more natural small-script shape. Bundled output with multiple Besht modules keeps `# --- module: name ---` separators between modules.

### Split (`--split`)

Each `.bsh` file compiles to its own `.sh` file in the output directory, preserving the source directory structure. Besht imports become POSIX source (`. file.sh`) calls at runtime. Explicit `.sh` imports are copied into the output directory and sourced with include guards.

```sh
besht compile main.bsh --split -o build/
# Produces:
#   build/main.sh         (entry: has #!/bin/sh, sets _BESHT_ROOT, sources libs)
#   build/lib/log.sh      (library: has include guard, sources its own deps)
#   build/lib/checks.sh   (library)
```

The entry point sets `_BESHT_ROOT` once at startup. All sourced files use it to locate siblings:

```sh
# build/main.sh
_BESHT_ROOT="$(cd "$(dirname "$0")" && pwd)"
. "$_BESHT_ROOT"/'lib/log.sh'
. "$_BESHT_ROOT"/'lib/checks.sh'
```

Library files include a guard against double-sourcing (safe for diamond dependencies):

```sh
# build/lib/log.sh
[ -n "$_BESHT_LOADED_lib__log" ] && return 0
_BESHT_LOADED_lib__log=1
```

### Visualization (`visualize`)

`besht visualize <file.bsh>` opens an in-terminal side-by-side view sized to the current terminal width. The left pane shows Besht source with line numbers, including blank and unmapped source lines, under the input file name; the right pane shows the compiled POSIX shell with line numbers under the same file name changed to `.sh`. Panes use a bat-style line-number gutter and rule. When one source line expands into multiple shell lines, the source pane leaves blank rows beside the generated block so the next source line aligns after that block. Long lines wrap inside their pane with a `↳` continuation marker instead of hiding text behind horizontal scrolling, and the pager disables horizontal chopping/scrolling. The view is for inspection only: it does not write an output file, and displayed shell omits `# besht:file:line:col` source comments. When `bat` is installed and output is a terminal, the view uses TypeScript and shell syntax highlighting; otherwise it falls back to plain text.

---

## Language

Files use the `.bsh` extension.

### Current behavior

- Type annotations and `as` assertions are accepted for TypeScript-compatible syntax, editor support, and occasional compiler representation hints. They never validate values or produce type mismatch errors.
- Template literals support `${expr}`, not just `${name}`.
- TypeScript `for...of` loops are supported as aliases for Besht array loops: `for (item of items)`, `for (const item of items)`, and `for (let item of items)` iterate values like `for (item in items)`.
- Ternary expressions `cond ? a : b` and nullish coalescing `value ?? fallback` are supported.
- Compound assignments `+=`, `-=`, and `*=` are supported.
- Postfix `++`/`--` are supported in statement position; prefix `++name`/`--name` are supported in expression position.
- Logical operators `&&`, `||`, `!`, and nullish coalescing `??` are supported.
- Static value-position `||` and `&&` expressions compile to the selected side when the left operand's truthiness is known.
- Static boolean `console.log()`/`console.error()` arguments such as `Boolean("")`, `true`, `!false`, static comparisons, and variables bound to static boolean expressions render directly as `true`/`false`; `Besht.fs.*` and `Besht.strings.*` predicates also print readable `true`/`false` in console calls. Dynamic boolean console arguments reuse the same shell condition once and print `true`/`false` from it.
- Strict equality `===` and `!==` compile the same as `==` and `!=`.
- Static scalar equality comparisons, including equality comparisons against variables bound to static string literals, and static numeric relational comparisons compile to constants, including comparisons over already-folded arithmetic, string methods/transforms, `Math.*`, and parseable `Number.parseInt()`/`Number.parseFloat()` calls. Dynamic relational comparisons over compiler-known integer expressions use POSIX `[ ]`; floats and unknown values keep the `awk` path. Dynamic equality keeps the multiline-safe shell path.
- String equality preserves spaces and newlines, including multiline template literals.
- Static boolean `if` conditions and ternary expressions, including variables bound to static boolean expressions, fold to the selected branch or value; dynamic and control-flow-assigned conditions keep normal shell tests.
- Static ASCII string expressions built from literals, variables bound to static ASCII strings, concatenation, template interpolation, and chained static ASCII transforms fold `[index]`, `.includes()`, `.startsWith()`, `.endsWith()`, `.indexOf()`, `.lastIndexOf()`, and `.charAt()` calls with static arguments to constants; dynamic and non-ASCII string indexes/searches keep the helper/`awk` path.
- Static ASCII string literal `.split()` calls and variables bound to static ASCII strings calling `.split()` with static separators compile to quoted newline-backed arrays and compact `for` loops when elements contain no newlines; dynamic and non-ASCII splits keep the POSIX tool path.
- Static string literal `Number.parseInt()` calls with parseable prefixes and static radix compile to numeric constants; dynamic calls use an AWK-backed parser, including non-decimal radix values.
- Static numeric arithmetic over literal numbers and variables bound to static numeric expressions compiles to constants; dynamic and control-flow-assigned arithmetic keeps shell arithmetic or POSIX `awk`.
- Static numeric literal, static numeric expression, or static numeric variable `.toString()`/`.toFixed()` calls, static numeric API receivers of `.toString()`, and literal-argument `Math.*` calls compile to constants; dynamic numeric calls keep the POSIX `awk` path.
- Static primitive `.toString()` calls in direct bindings, string concatenation, and template interpolation compile to constants; dynamic receivers keep the normal runtime formatting path.
- Static ASCII string expressions built from literals, variables bound to static ASCII strings, concatenation, template interpolation, and chained static ASCII transforms fold transforms such as `.trim()`, `.toUpperCase()`, `.slice()`, `.substring()`, `.repeat()`, `.replace()`/`.replaceAll()`, `.concat()`, and `.padStart()`/`.padEnd()` with static arguments to constants; dynamic and non-ASCII transforms keep the POSIX tool path. Dynamic string `slice()`, `at()`, and indexing use AWK substring extraction.
- Simple prefix-strip ternaries such as `s.startsWith("#") ? s.slice(1) : s` compile to compact POSIX parameter expansion.
- `switch/case/default` compiles to shell `case/esac`.
- `if`/`else if`/`else`, `for`, and `while` bodies can be either braced blocks or a single bracketless statement; multiple statements still require braces.
- Static scalar array literals and array-returning method chains over static scalar arrays (`concat`, `slice`, `reverse`, `push`, `unshift`, `pop`, `shift`) compile to quoted newline-backed shell strings when values do not contain newlines; dynamic, spread, nested, and newline-sensitive arrays keep the `printf` builder.
- Static scalar `Array.of(...)` calls and Besht's narrow static `Array.from({ length: N })` calls compile to quoted newline-backed shell strings and compact loops when values contain no newlines; dynamic factories keep the existing builder path.
- Static string literals, variables bound to static string literals, static scalar array expressions, and variables bound to static scalar arrays compile `.length` properties to numeric constants; dynamic lengths keep the POSIX `wc` path.
- `for (... in [...])` and loops over static scalar array expressions or variables bound to them compile to compact shell `for` loops when values do not contain newlines; dynamic arrays keep the newline-safe read loop.
- Static scalar array indexes with known in-range integer indexes and static nested-array indexes with known row/column indexes compile to constants; dynamic, unknown, and out-of-range indexes keep the POSIX `sed`/packed-row path.
- Static scalar array literals and variables bound to static scalar arrays fold `.join()` and `.toString()` calls to one quoted string when elements contain no newlines and the separator is static; dynamic joins keep the newline-safe `awk` path.
- Static scalar array literals and variables bound to static scalar arrays fold `.includes()`, `.indexOf()`, and `.lastIndexOf()` calls with static scalar needles to constants; dynamic searches keep the POSIX `grep`/`awk` path.
- Inline static scalar object literal `Object.keys()`, `Object.values()`, `Object.entries()`, and `Object.hasOwn()` calls compile to constants; unmutated named object `Object.keys()`, static-scalar `Object.values()`/`Object.entries()`, static-key `Object.hasOwn()`, literal-only `Object.assign({}, ...)`, and literal-only object spread results also fold from compiler-managed metadata.
- Direct reads of scalar properties from static object literal bindings compile to constants when the object is not assigned, computed-assigned, aliased, or passed to a function.
- Object literals compile to per-property shell variables; object spread (`{ ...source, key: value }`) and `Object.assign(target, ...sources)` shallow-copy scalar-safe compiler-managed objects with the same left-to-right key order; `Object.keys(obj)` returns known compiler-managed object keys as `string[]`, `Object.values(obj)` returns values as `string[]`, `Object.entries(obj)` returns `[key, value]` rows as `string[][]`, and `Object.hasOwn(obj, key)` checks known key membership. `JSON.parse()` and `JSON.stringify(value)` are available when compiling with `--opt-use-jq`.
- Static scalar array destructuring over literals and variables bound to them emits direct assignments; dynamic destructuring keeps the temp-and-`sed` path.
- Static boolean object properties used directly in conditions can fold to the selected branch; dynamic boolean object properties compile to direct `= 1` shell tests. Non-boolean property conditions keep generic JavaScript-style truthiness.
- Classes support constructors, instance properties/methods, `new`, `this`, static properties/methods, and getters/setters.
- TypeScript-only class modifiers such as `private`, `public`, `protected`, and `readonly` are accepted and ignored.
- `Record<K, V>` annotations are accepted for object-map style code; they are annotations only and add no runtime type checks.
- Tuple destructuring declarations such as `const [x, y] = pair` are supported for array/tuple values.
- `null` and `undefined` are accepted for TypeScript-compatible syntax; `??` falls back only for nullish values and preserves `""`, `0`, and `false`. Static nullish coalescing folds to the known side when the left operand is provably nullish or non-nullish. Optional chaining supports `obj?.prop`, `obj?.[key]`, `obj?.method()`, and nested chains.
- `$()` calls support array spreading with `...args`; spreading an entire command vector must use sole-argument form `$(...cmd)`.
- Literal `$()` command words are emitted bare when they are conservative shell-safe words, and quoted when they need protection.
- Single-command stdout/stderr redirects compile directly as `cmd > file`; pipeline redirects keep grouping braces so the redirect applies to the whole pipeline.
- `.d.bsh` files are declaration-only and never emit shell output. A `stdlib.d.bsh` file beside the entry `.bsh` file is auto-loaded for compile, split compile, and `--check`; run `besht init` to generate one in the current directory. Imported module directories are not scanned for their own `stdlib.d.bsh` files.
- Extensionless imports resolve to `.bsh` by default. Pass `--opt-resolve-ts-imports` to fall back to `.ts` only when the `.bsh` file is absent. Explicit named `.sh` imports require `assert { type: "shell" }` and source existing shell functions. Shell imports must resolve inside the compiler root unless `--opt-allow-external-shell-imports` is passed.
- `type` aliases and `interface` declarations are parsed and silently ignored (no runtime output).
- Simple `type Name = ExistingType` aliases can be used in annotations, including `string[]` and `Set<string>`.
- Type assertions such as `[] as string[]` are parsed and erased at compile time.
- `new Set<T>()` supports `.add(value)` and `.has(value)` with no runtime type checking; straight-line static adds and membership checks compile to constants.
- Nested arrays such as `string[][]` preserve row structure for `.map()`, nested indexing, and row `.length`.
- Generated shell includes `# besht:file:line:col` source comments at non-class statement boundaries and before explicit class constructor/accessor/method shell functions.
- When the runtime preamble is empty, generated shell uses a single blank separator between the header and the first statement.
- Semicolons are optional (only required inside `for` headers).
- `Array.from({ length })` differs from JavaScript: it creates the numeric array `0` through `length - 1` and does not support general iterables or mapper callbacks. `Array.of(...)` creates an array from the given values. `Array.isArray(value)` is a static predicate for compiler-known arrays and adds no runtime shape metadata.
- `Object.keys(obj)`, `Object.values(obj)`, `Object.entries(obj)`, `Object.hasOwn(obj, key)`, `Object.assign(target, ...sources)`, and object spread differ from broad JavaScript object reflection/copying: they use compiler-managed object key metadata, require Besht-compatible keys, reject unsupported object surfaces, and do not emit runtime helpers.
- JSON support is opt-in through `--opt-use-jq` and invokes `jq` in generated code. `JSON.parse()` returns a compact `JSONValue`; property/index access on that value returns another `JSONValue`; `: string`, `: number`, `: boolean`, or `as ...` extract scalars with runtime validation. `JSON.stringify()` supports `JSONValue`, strings, numbers, booleans, null/undefined, scalar arrays, and scalar-valued compiler-managed objects.
- `fetch(url).text()` is a synchronous, curl-backed, text-only GET slice. It emits `curl -sS -- <url>` and intentionally does not support `await`, options, POST, headers, body, `.json()`, `.status`, `.ok`, or `.headers` yet.
- Arrow functions can be stored in variables, passed to functions, called as function values, and passed to array callback APIs; direct array callbacks still support the compact inline forms and optional zero-based index parameter.
- Generated shell elides string runtime helpers unless one-argument string `.includes()`, `.startsWith()`, or `.endsWith()` actually needs them.

### Variables

Declare with `let`. Types are optional and are never validated by the compiler.

```ts
let name: string = "Alice";
let count: number = 42;
let price: number = 3.14; // float literal supported
let verbose: boolean = true;
let files: string[] = ["a.txt", "b.txt", "c.txt"];
```

Use `const` for values that should never be reassigned:

```ts
const MAX_RETRIES: number = 3;
const APP_NAME: string = "myapp";
```

Reassign with a plain assignment (no `let`):

```ts
count = count + 1;
count += 2;
count++;            // postfix increment
count--;            // postfix decrement
let next = ++count;  // prefix increment in expressions
```

Preferred annotation forms include `string`, `number`, `boolean`, `object`, `T[]`, `Array<T>`, `Set<T>`, `status`, union types (`string | null`), and tuple types (`[string, number]`). Besht does not type-check annotations; `null` and `undefined` are runtime nullish sentinels for `??` and comparisons.

### Strings

Both `"..."` and `'...'` are plain literals — no interpolation. Use backtick template literals for interpolation and embedded expressions:

```ts
let name: string = "Alice"           // plain literal
let also: string = 'Alice'           // same — plain literal
let tmpl: string = `Hello ${name}!`  // template literal — interpolates ${name}
let sum = `sum=${a + b}`             // expressions inside ${...}
let pattern: string = '^foo-[0-9]+'  // single-quoted literal text
let path = "C:\\temp\\new\\file.txt" // escape backslashes in double-quoted strings
let escape: string = "newline:\n tab:\t backslash:\\ quote:\" dollar:\$"  // escape sequences
let unicode: string = "A \u0041 ñ \u00F1"  // unicode escapes
```

Backticks are template literals and should be used only when interpolation or multiline template text is needed. In template literal text, `$` stays literal unless it starts a Besht `${expr}` interpolation. Shell parameter forms such as `$*`, `$?`, and `$$` are emitted as literal text.

Use `+` to concatenate:

```ts
let greeting: string = `Hello, ` + name + `!`;
let label: string = "check:" + name;
let bigger = a > b ? a : b;
```

### Environment variables

Use `process.env.NAME` for JavaScript-style environment access. Missing variables are nullish, so `??` falls back only when the variable is unset and preserves an explicitly empty value. Static safe defaults compile to POSIX unset-only expansion such as `${PORT-8080}`.

```ts
let home: string = process.env.HOME
let port: string = process.env.PORT ?? "8080"
let debug: string = process.env.DEBUG ?? "false"
```

### Script arguments

Use `Besht.args` to read command-line arguments passed to the compiled shell script. Missing positional values and options return a nullish value, so use `??` for defaults. Empty strings are preserved.

```ts
let all: string[] = Besht.args.argv()
let input = Besht.args.positional(1) ?? "-"
let branch = Besht.args.option("branch", "b") ?? "main"
let dryRun = Besht.args.flag("dry-run", "d")
```

- `Besht.args.argv()` returns positional arguments as `string[]` after option parsing.
- `Besht.args.positional(n)` returns the 1-based positional argument or a nullish value when absent.
- `Besht.args.option(longName, shortName?)` supports `--name=value`, `--name value`, and `-n value`; absent options are nullish.
- `Besht.args.flag(longName, shortName?)` returns `true` when `--name` or `-n` is present.
- `--` stops option and flag parsing; later values are treated as positional arguments.

Top-level scripts that only read `Besht.args.positional()` compile to a compact inline scan of `"$@"`. Scripts that use `argv()`, `option()`, `flag()`, or read args inside functions keep the shared parser runtime so option parsing and script-argument capture remain consistent.

When translating shell `while`/`case` or `getopts` option parsers, prefer the argument helpers instead of rewriting the parser loop. Use `Besht.args.option("root", "r")` for `--root`/`-r`, `Besht.args.flag("verbose", "v")` for booleans, and `Besht.args.positional(n)` or `Besht.args.argv()` for remaining positional values. Only keep a manual parser when the original behavior is genuinely custom.

### Fetch

The first `fetch()` slice is synchronous, curl-backed, and text-only. It performs a GET with `curl -sS -- <url>` and returns stdout through `.text()`.

```ts
let body: string = fetch("file:///tmp/data.txt").text()

let response = fetch(url) // runs curl once here
let first: string = response.text()
let second: string = response.text() // reuses the stored body
```

This slice deliberately rejects or defers `await`, `fetch(url, options)`, POST/method options, headers, request bodies, redirects policy, streaming, abort/clone, response `.json()`, `.status`, `.ok`, and `.headers`. `--check` rejects those unsupported response properties and methods.

### Nullish coalescing

`a ?? b` returns `a` unless it is `null` or `undefined`. Unlike shell `${VAR:-default}`, it preserves valid falsey values. Static `??` expressions compile to the selected side when the left side is known.

```ts
let missing = undefined
let empty = ""
let zero = 0
let nope = false

console.log(missing ?? "fallback") // fallback
console.log(empty ?? "fallback")   // empty string
console.log(zero ?? 99)               // 0
console.log(nope ?? true)             // false
```

### Optional chaining

Optional chaining short-circuits when the receiver is `null` or `undefined`, returns a nullish value, and composes with `??`. Empty strings, `0`, and `false` are preserved.

```ts
let name = user?.name ?? "anonymous"
let item = items?.[i] ?? "fallback"
let cell = matrix?.[row]?.[col] ?? "missing"
let trimmed = maybeText?.trim() ?? ""
```

Optional chaining only guards nullish receivers. It does not add runtime type checking or validate object shapes. General optional function calls (`fn?.()`), optional method-value calls (`obj.method?.()`), and optional-chain assignment targets (`obj?.prop = value`) are not supported.

### Functions

```ts
function greet(name: string, times: number): string {
    let result: string = ""
    for (let i: number = 0; i < times; i++) {
        result = `${result}Hello, ${name}!\n`
    }
    return result
}

let msg: string = greet("Alice", 3)
$("printf", "%s", msg).run()
```

Functions with no return type are void:

```ts
function log(msg: string) {
    $("printf", "[LOG] %s\n", msg).stderr("&1").run()
}
```

### Print

`console.log()` writes a string to stdout with a trailing newline. `console.error()` writes the same format to stderr. Boolean arguments print as `true`/`false`; dynamic boolean expressions compile to one shell condition instead of a nested boolean formatter. Arrays print in Bun-style `[ a, b ]` format; static scalar array output compiles to one quoted `printf` line, while dynamic and newline-sensitive arrays keep the generic formatter. Objects print each property on its own line and reflect current property values. Inline object literals compile to a direct multi-line `printf`.

```ts
console.log("Usage: myscript [options]");
console.log("  --help    Show help");
console.log("Result: " + value);
console.error("Something went wrong");
console.log(["a", "b"]);             // [ a, b ]
console.log({ apple: 3, banana: 2 });  // multi-line object output
```

### Control flow

**If / else if / else** — condition must be in parentheses. Bodies may be braced blocks or one bracketless statement:

```ts
if (count > 10) {
    $("echo", "many").run()
} else if (count > 0) {
    $("echo", "some").run()
} else {
    $("echo", "none").run()
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
  $("echo", i.toString()).run();
}

for (let i: number = 0; i < 10; i++) total += i
```

**For — range:**

```ts
for (i in Besht.iter.range(1, 10)) {
  $("echo", i.toString()).run();
}

for (i in Besht.iter.range(1, 10)) total += i
```

Small static range bounds compile to compact POSIX `for` loops. Dynamic or very large ranges keep a counter `while` loop.

**For — array:**

```ts
let fruits: string[] = ["apple", "banana", "cherry"];
for (fruit in fruits) {
  $("echo", fruit).run();
}

for (let fruit in fruits) {
  $("echo", fruit).run();
}

for (const fruit of fruits) {
  $("echo", fruit).run();
}

for (fruit in fruits) $("echo", fruit).run()
```

`for...of` is accepted for TypeScript compatibility and behaves like Besht's value-iteration `for...in` array loop.

**For — command output:**

```ts
for (file in $("find", "/var/log", "-name", "*.log").run().readStdoutLines()) {
  $("echo", `found: ${file}`).run();
}
```

**Break and continue:**

```ts
for (f in files) {
  if (Besht.strings.isEmpty(f)) {
    continue;
  }
  if (f == "STOP") {
    break;
  }
  $("echo", f).run();
}
```

**Switch / case / default** — compiles to shell `case/esac`:

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

**Logical operators:**

```ts
let active = true && !false    // logical AND, NOT
let either = active || false   // logical OR
let fallback = maybe ?? "default" // nullish fallback only
let same = x === y             // strict equality (same as ==)
let diff = x !== y             // strict inequality (same as !=)
let sameTree = draw() === `*
#`                              // multiline strings compare safely

// || and && in value position return actual values (JS semantics).
// Static known-left cases compile directly to the selected value.
let count = acc[word] || 0     // returns acc[word] if truthy, else 0

// ?? falls back only for null/undefined, preserving empty string, 0, and false
let port = Besht.args.option("port", "p") ?? "8080"
let label = maybeName ?? "anonymous"
```

### String methods

Strings have TypeScript-compatible methods:

```ts
let s: string = "  Hello, World!  ";

s.trim(); // "Hello, World!"
s.trimStart(); // "Hello, World!  "
s.trimEnd(); // "  Hello, World!"
s.toUpperCase(); // "  HELLO, WORLD!  "
s.toLowerCase(); // "  hello, world!  "
s.replace("World", "besht"); // "  Hello, besht!  "
s.replaceAll("l", "L"); // "  HeLLo, WorLd!  "
s.split(","); // string[] ["  Hello", " World!  "]
s.split(""); // string[] of characters
s.includes("World"); // boolean
s.includes("World", 4); // boolean, search from position
s.startsWith("  Hello"); // boolean
s.startsWith("World", 9); // boolean, compare at position
s.endsWith("!  "); // boolean
s.endsWith("Hello", 7); // boolean, compare against first length chars
s.indexOf("World"); // int (0-based, -1 if not found)
s.indexOf("l", 5); // int, search from position
s.lastIndexOf("l"); // int (last zero-based match, -1 if not found)
s.lastIndexOf("l", 8); // int, search backward from position
s.slice(2, 7); // "Hello"
s.substring(2, 7); // "Hello"
s.at(2); // "H"
s.charAt(2); // "H"
s[1]; // " "
s.padStart(20, "-"); // "-----  Hello, World!  "
s.padEnd(20, "."); // "  Hello, World!  ..."
s.concat(" More text"); // "  Hello, World!   More text"
s.length; // int (character count)
```

One-argument string `includes()`, `startsWith()`, and `endsWith()` use tiny POSIX helper functions that are emitted only when the generated shell calls them. Two-argument string search methods use inline `awk` instead. Static ASCII string expressions built from literals, variables bound to static ASCII strings, concatenation, template interpolation, and chained transforms fold indexes, searches, and `charAt()` calls with static arguments to constants.

Static ASCII string expressions built from literals, variables bound to static ASCII strings, concatenation, template interpolation, and chained transforms fold transforms with static arguments to constants, including `.replace()`/`.replaceAll()` and `.concat()` calls. Dynamic and non-ASCII transforms use POSIX tools such as `sed`, `tr`, and `awk`; dynamic string `slice()`, `at()`, and indexing use AWK substring extraction.

Simple prefix-strip ternaries such as `s.startsWith("#") ? s.slice(1) : s` compile to compact POSIX parameter expansion.

Static ASCII string literal `.split()` calls and variables bound to static ASCII strings calling `.split()` with static separators compile to constants when the resulting array elements contain no newlines. Dynamic and non-ASCII splits use POSIX tools such as `tr` and `awk`.

Primitive values have basic formatting helpers:

```ts
let s = "x";
s.toString(); // "x"

let b = true;
b.toString(); // "true"

let code: status = /* catch variable */;
code.toString(); // numeric status

let n = 42;
n.toString(); // "42"

let pi = 3.14159;
pi.toFixed(2); // "3.14"
```

### Math methods

```ts
Math.min(a, b); // smaller of two numbers (works with floats)
Math.max(a, b); // larger of two numbers (works with floats)
Math.round(3.7); // 4, rounds half-up
Math.floor(3.9); // 3
Math.ceil(3.1); // 4
Math.trunc(3.9); // 3
Math.abs(-5); // 5
Math.sign(-5); // -1
Math.pow(2, 8); // 256
Math.sqrt(16); // 4
```

Literal-argument `Math` calls compile to constants. Dynamic `Math` methods compile to `awk` arithmetic and support decimal numbers. POSIX `$((...))` is integer-only, so besht uses `awk` wherever a dynamic float operand is present.

When a variable is reassigned, besht updates its float-tracking metadata from the new right-hand side: float-producing expressions keep later arithmetic on `awk`, while integer/non-float reassignment clears the float marker so later integer arithmetic can use shell integer lowering again.

### Number builtins

```ts
Number.parseInt("42");        // parse string to integer
Number.parseInt("42", 10);    // optional radix argument
Number.parseInt("2a", 10);    // 2; static parseable prefixes compile to constants
Number.parseInt(hexByte, 16);  // dynamic radix parsing is supported
Number.parseFloat("3.14");    // parse string to float
Number.isFinite(n);           // true (no NaN/Infinity in shell)
Number.isInteger(n);          // check if value is integer
Number.isSafeInteger(n);      // check if value is a safe integer
Number.isNaN(n);              // always false for current besht values
Number.MAX_SAFE_INTEGER;      // 9007199254740991
Number.MIN_SAFE_INTEGER;      // -9007199254740991
Number.EPSILON;               // 2.220446049250313e-16
```

`Number.isNaN()` is always false for currently representable besht values because besht has no NaN runtime sentinel.

### Objects

Object literals compile to per-property shell variables:

```ts
let user = { id: 1, name: "Victor", active: true };
let userName: string = user.name;  // property access
user.name = "Compiler Tester";     // property assignment

// Computed property access with dynamic keys
let key: string = "name";
let val: string = user[key];       // read property by key
user[key] = "Updated";            // assign property by key
let keys: string[] = Object.keys(user); // ["id", "name", "active"]
let values: string[] = Object.values(user); // ["1", "Compiler Tester", "true"]
let entries: string[][] = Object.entries(user); // [["id", "1"], ["name", "Compiler Tester"], ...]
let hasName: boolean = Object.hasOwn(user, "name") // true
let hasBadKey: boolean = Object.hasOwn(user, "bad-key") // false
let copy = Object.assign({}, user, { active: false }) // fresh scalar-safe object copy
let merged = { ...user, active: false } // fresh scalar-safe object spread copy

function showKeys(obj: object): string[] {
  return Object.keys(obj)
}
```

Static and computed object keys must contain only letters, numbers, and `_`.
`Object.keys(obj)`, `Object.values(obj)`, and `Object.entries(obj)` return known keys, scalar values, or `[key, value]` rows in insertion order and include aliases, object parameters, later `obj.prop = value`, and `obj[key] = value` additions. `Object.assign(target, ...sources)` mutates the first argument and returns that same object; an inline object-literal target such as `{}` creates a fresh object. Object spread always creates a fresh object and copies spread sources and explicit properties left to right. Existing keys keep their first position, new keys append when first introduced, and later spreads or properties overwrite values. This differs from JavaScript's runtime reflection over arbitrary enumerable own properties: Besht uses compiler-managed object metadata, static and computed keys must contain only letters, numbers, and `_`, and `process.env` is not enumerable. Inline static scalar object literals compile these calls to constants; unmutated named object `Object.keys()`, static-scalar `Object.values()`/`Object.entries()`, static-key `Object.hasOwn()`, literal-only `Object.assign({}, ...)`, and literal-only object spread calls also fold to constants. Statically known boolean values are rendered as `true`/`false` in values and entries output. `Object.values()`, `Object.entries()`, `Object.assign()`, and object spread reject statically known array/object/set/command/fetch values because the current `string[]`, packed `string[][]`, and scalar object slot representations cannot preserve deeper nested values. `Object.hasOwn(obj, key)` checks exact key membership against the same metadata and returns `false` for dynamic keys that are not valid Besht object keys. Class instances, `this`, and `process.env` are rejected as `Object.assign()` targets or object spread sources; `const` targets are rejected for `Object.assign()`. These helpers and object spread do not emit a runtime helper library.

### Classes

Classes compile to POSIX sh functions plus compiler-managed object property slots. Supported features are constructors, instance properties, instance methods, `new`, `this`, static properties/methods, and getters/setters. TypeScript-only modifiers (`private`, `public`, `protected`, `readonly`) are accepted and ignored. Inheritance, decorators, and abstract classes are not supported.

```ts
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
  private static Deltas: Record<string, [number, number]> = { U: [-1, 0] }
  readonly matrix: string[][]
}
```

Getter bodies take no parameters, must declare a non-void return type, and cannot assign to `this` properties. Setter bodies take exactly one parameter and do not declare a non-void return type. A getter/setter cannot share a name with a class field; source methods named `get_<name>` or `set_<name>` are reserved for accessor lowering.

### Array methods

Arrays have TypeScript-compatible methods:

```ts
let l: string[] = ["alpha", "beta", "gamma"];
let matrix: string[][] = ["ab", "cd"].map(row => row.split("") as string[]);
let indexes: number[] = Array.from({ length: 3 }); // [0, 1, 2]
let chosen: string[] = Array.of("alpha", "omega"); // ["alpha", "omega"]
let isArray: boolean = Array.isArray(chosen); // true for compiler-known arrays

l.push("delta"); // new array with "delta" appended
l.unshift("zero"); // new array with "zero" prepended
l.pop(); // new array without last element
l.shift(); // new array without first element
l.concat(other); // two arrays joined
l.slice(1, 3); // ["beta", "gamma"]
l.join(", "); // "alpha, beta, gamma"
["a", "b", "c"].join(","); // static scalar literal compiles to 'a,b,c'
["a", "b"].concat(["c"]).join(","); // static scalar chains compile to 'a,b,c'
l.toString(); // "alpha,beta,gamma" for scalar arrays; same as l.join(",")
l.includes("beta"); // boolean, uses `grep -qxF` membership and does not emit the string `_bst_includes` helper
l.indexOf("gamma"); // int (0-based, -1 if not found)
l.lastIndexOf("beta"); // int (last zero-based match, -1 if not found)
l.reverse(); // ["gamma", "beta", "alpha"]
l.map(x => x + "!"); // new array with callback expression applied to each item
l.map((x, i) => i.toString() + ":" + x); // second callback arg is zero-based index
l.forEach((x, i) => console.log(i.toString() + ":" + x)); // statement-only side effects
l.filter(x => x.startsWith("a")); // new array with truthy callback results
let anyA = l.some(x => x.startsWith("a")); // true if any callback result is truthy; false for an empty array
let allNamed = l.every(x => x.length > 0); // true if all callback results are truthy; true for an empty array
let hit = l.find((x, i) => i == 1) ?? "missing"; // first matching element, or nullish when no match
let at = l.findIndex(x => x == "beta"); // 1, or -1 if no match
let copied = [...l, "omega"]; // array spread in array literals
l.length; // number
matrix[0][1]; // nested indexing
matrix[0].length; // row length
const [row, col] = [1, 2]; // tuple/array destructuring, direct assignments for static scalar arrays
let maybe = matrix?.[row]?.[col] ?? "missing"
```

`Array.from({ length })` is a Besht-specific numeric range factory. Unlike JavaScript, it does not create an array of `undefined` values and does not accept arbitrary iterables or mapper callbacks.

Array `.toString()` is currently a scalar-array API slice. JavaScript nested-array flattening for `string[][]` and packed row arrays is not implemented.

### Sets

`Set<T>` is a lightweight newline-backed collection for membership tracking. Type parameters are annotations only; `.add(value)` mutates the set and `.has(value)` checks membership without runtime type checks. Straight-line static scalar adds and static membership checks compile to direct assignments and constants; dynamic values, callback/control-flow adds, and newline-containing values keep the runtime `awk`/`grep` path.

```ts
let visited = new Set<string>()
visited.add("0,0")
if (visited.has("0,0")) {
  console.log("seen")
}
```


### Arrow callbacks

Arrow functions can be stored in variables, passed to functions, called as function values, and passed to array callback APIs. Direct arrows support expression-bodied and block-bodied forms for array `.map()`, `.filter()`, `.reduce()`, and statement-position `.forEach()`. Scalar-array predicate callbacks for `.some()`, `.every()`, `.find()`, and `.findIndex()` may be direct arrow expressions or stored callbacks.

When translating shell pipelines that process already-known text or numbers, prefer native Besht data operations over spawning `sed`/`awk`/`grep`/`tr`. Use `map`, `filter`, `reduce`, `forEach((item, index) => ...)`, `join`, and string methods such as `trim()`, `startsWith()`, and `toUpperCase()` for in-memory transformations. Keep command pipelines for external data sources and tool-specific work.

For literal delimiter-separated records or shell snippets that use `awk -F:`, `cut`, `paste`, or repeated membership probes over a static table, model the data directly. Use object literals for key/value records, object spread or `Object.assign()` for scalar-safe copies/merges, `Object.keys()`, `Object.entries()`, and `Object.hasOwn()` for enumeration and probes, `Set<T>` for membership groups, and `JSON.stringify()` for JSON output instead of preserving the text-processing pipeline.

```ts
let names = ["alice", "bob", "anna"]
let shouted = names.map(name => name.toUpperCase())
let aNames = shouted.filter(name => name.startsWith("A"))
let hasAnna = names.some(name => name == "anna")
let allShort = names.every((name, i) => name.length < 10 && i >= 0)
let firstB = names.find(name => name.startsWith("b")) ?? "none"
console.log(aNames.join(","))

let initials = ""
names.forEach((name, i) => {
    console.log(i.toString() + ":" + name)
    initials = initials + name.charAt(0)
})

let typed = names.filter((name: string) => name.includes("a"))
let labeled = names.map((name, i) => {
    if (i == 0) return "first:" + name
    return i.toString() + ":" + name
})

let addBang = (name: string): string => name + "!"
function applyName(name: string, cb: (name: string) => string): string {
    return cb(name)
}
let called = applyName("bob", addBang)
let markedNames = names.map(addBang)

function makeCounter(start: number): () => string {
    let n = start
    return (): string => {
        n = n + 1
        return n.toString()
    }
}
let next = makeCounter(0)
console.log(next()) // 1
console.log(next()) // 2

function makePrefixFilter(prefix: string): (name: string) => boolean {
    return (name: string): boolean => name.startsWith(prefix)
}
let aOnly = names.filter(makePrefixFilter("a"))

// reduce with expression body
let total = nums.reduce((acc, n) => acc + n, 0)
let lines = nums.reduce((acc, n) => [...acc, "#".repeat(n)], [] as string[]).join("\n")

let rawWords = ["  alpha", "Beta", "apricot", "banana"]
let labels = rawWords
    .map(word => word.trim())
    .filter(word => word.startsWith("a"))
    .map(word => "item:" + word)
    .join(", ")

// reduce with block body and object accumulator
let counts = words.reduce((acc, word) => {
    acc[word] = (acc[word] || 0) + 1
    return acc
}, {})
let countWord = (acc: object, word: string): object => {
    acc[word] = (acc[word] || 0) + 1
    return acc
}
let storedCounts = words.reduce(countWord, {})
console.log(counts)
```

`.map()` supports expression or block bodies and one or two direct arrow parameters: `(item)` or `(item, index)`. `return` inside a block-bodied `.map()` callback emits that mapped value for the current item and continues the callback loop. Block-bodied `.map()` callbacks currently support `return`, `if`/`else`, and assignment statements; arbitrary expression statements are rejected. `.filter()`, `.some()`, `.every()`, `.find()`, and `.findIndex()` use JavaScript-style truthiness and may receive `(item, index)`. `.some()` short-circuits on the first truthy callback result and returns `false` for an empty array. `.every()` short-circuits on the first falsey callback result and returns `true` for an empty array. `.find()` returns the first matching scalar element, or a nullish value when no element matches so `??` fallbacks work. `.reduce()` takes a 2-parameter callback plus an initial value; direct arrows and stored callbacks support scalar/array accumulators, and stored Besht callbacks can mutate compiler-managed object accumulators. `.forEach()` is statement-only, takes a direct arrow or stored callback with `(item)` or `(item, index)`, compiles static scalar direct-arrow receivers to compact `for` loops, runs stored callback calls in the current shell so outer assignments and `Set.add()` side effects persist, and rejects direct-arrow callback `return`, `break`, `continue`, and pure value expressions. Arrow values lower to generated shell functions or closure ids; returned arrows get independent captured environments, callback factories can be passed directly to array methods, and direct function-value calls in value position use Besht's return-slot path so captured mutations persist. Array callback expressions used in value or condition position run in the current shell, so assignment, `Set.add()`, and returned-closure mutations persist after `.map()`, `.filter()`, `.some()`, `.every()`, `.find()`, and `.findIndex()` complete.

### Array indexing

Access array elements by zero-based index. Nested arrays preserve row boundaries, so `matrix[row][col]`, `matrix.length`, and `matrix[0].length` work for `T[][]` values:

```ts
let args: string[] = $("printf", "%s\n", "a", "b").run().readStdoutLines()
let first: string = args[0]
let second: string = args[1]
let item: string = args[i]   // variable index
args[1] = "BETA"             // index assignment
let empty: string[] = []     // empty array

let matrix: string[][] = rows.map(row => row.split("") as string[])
let cell: string = matrix[row][col]
let width: number = matrix[0].length
```

Static scalar array indexes with known in-range integer indexes and static nested-array indexes with known row/column indexes compile to constants. Dynamic, unknown, and out-of-range indexes compile to `sed`/packed-row extraction (POSIX sh compatible). Index assignment uses `awk` to replace the Nth line.

### Error handling

`try / catch` traps command failures. The catch variable holds the exit code.

```ts
try {
    $("rsync", "-az", "./dist/", "server:/opt/app/").run()
    $("ssh", "server", "systemctl restart myapp").run()
} catch (code: status) {
    $("echo", `Deploy failed with exit code ${code}`).stderr("&1").run()
    process.exit(1)
}
```

Use `?` to propagate failure out of the enclosing function:

```ts
function read_config(path: string): string {
    let content: string = $("cat", path)?
    return content
}
```

### Pipes

Use `.pipe()` to build pipelines:

```ts
let user: string = $("whoami").pipe($("tr", "[:lower:]", "[:upper:]")).run().readStdout()
console.log($("pwd").run().readStdout()) // inline reads can compile directly to "$(pwd)"
```

When translating shell scripts, keep shell structure in Besht's command model instead of embedding shell fragments in strings:

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

| Shell idiom | Besht pattern |
| ----------- | ------------- |
| `cmd arg "$value"` | `$("cmd", "arg", value).run()` |
| `cmd1 \| cmd2` | `$("cmd1").pipe($("cmd2")).run()` |
| `out=$(cmd)` | `let out = $("cmd").run().readStdout()` |
| `cmd > file`, `cmd >> file` | `.stdout(file)`, `.stdout(file, "append")` |
| `cmd >/dev/null`, `cmd 2>/dev/null`, `cmd 2>&1` | `.stdout("null")`, `.stderr("null")`, `.stderr("&1")` |
| `(cd dir && cmd)` | `$("cmd").workdir(dir).run()` |
| `VAR=value cmd` | `$("cmd").env("VAR", value).run()` |
| `cmd && next` | run a named command, check `.exitCode()`, then use `if` |
| `${1-default}` | `Besht.args.positional(1) ?? "default"` |
| `${1:-default}` | read the positional arg, then use `Besht.strings.isEmpty()` to apply the empty-string default |
| `while`/`case` parser for `--root`, `-r`, `--verbose` | `Besht.args.option("root", "r")`, `Besht.args.flag("verbose", "v")`, `Besht.args.positional(n)` |
| `printf "$TEAM" \| awk -F: ...` over a literal table | object literals, `Set<T>`, `Object.entries()`, `Object.hasOwn()`, array callbacks, `JSON.stringify()` |

Avoid `$("sh", "-c", "...")` or inline strings containing `cd`, pipes, redirects, or `VAR=value cmd` unless the script intentionally invokes a shell interpreter. Pass arguments as separate values and use ordinary single-quoted or double-quoted strings for grep/sed/awk patterns that must stay literal.

Use `.env(name, value)` to prefix one command invocation with an environment variable. The name must be a string literal POSIX shell identifier.

```ts
$("make", "build").env("CI", "1").run() // CI=1 make build
```

Use `.workdir(path)` to run a command or pipeline from a specific directory without changing the parent script's current directory.

```ts
let root: string = $("pwd").workdir("/").run().readStdout() // /
$("make", "test").workdir("/repo/app").run()
```

Use `.exitCode()` for explicit success-gated command chains:

```ts
let probe = $("find", ".", "-maxdepth", "1", "-type", "f").stdout("null").stderr("null")
probe.run()
if (probe.exitCode() == 0) {
  console.log("ok")
}
```

For shell defaults like `${1:-.}`, remember that the `:-` form treats both missing and empty arguments as absent:

```ts
let root = Besht.args.positional(1) ?? "."
if (Besht.strings.isEmpty(root)) {
  root = "."
}
```

### Besht helpers

Besht-specific helpers live under the global `Besht` object. They compile to inline POSIX tests or small generated shell; they are not runtime namespace objects.

These predicates are booleans. Conditions compile to direct tests, assignments store `1`/`0`, and `console.log()`/`console.error()` render them as `true`/`false`.

File helpers:

| API                         | Condition emitted |
| --------------------------- | ----------------- |
| `Besht.fs.isFile(p)`        | `[ -f "$p" ]`     |
| `Besht.fs.isDir(p)`         | `[ -d "$p" ]`     |
| `Besht.fs.isReadable(p)`    | `[ -r "$p" ]`     |
| `Besht.fs.isWritable(p)`    | `[ -w "$p" ]`     |
| `Besht.fs.isExecutable(p)`  | `[ -x "$p" ]`     |

String predicates:

| API                              | Condition emitted |
| -------------------------------- | ----------------- |
| `Besht.strings.isEmpty(s)`       | `[ -z "$s" ]`     |
| `Besht.strings.isNonEmpty(s)`    | `[ -n "$s" ]`     |

Iteration helpers:

| API                            | Description                              |
| ------------------------------ | ---------------------------------------- |
| `Besht.iter.range(start, end)` | Inclusive ascending integer range; small static ranges compile to compact `for` loops |

Array operations should use native array syntax and methods:

| Native API             | Description                   |
| ---------------------- | ----------------------------- |
| `items.length`          | Number of elements             |
| `items[0]`              | First element                  |
| `items.slice(1)`        | All elements except the first  |
| `items.push(value)`     | New array with value appended  |
| `[...items, value]`     | New array with value appended  |
| `items.includes(value)` | True if value is in the array  |
| `items.concat(other)`   | Concatenate two arrays         |

Array helpers:

| Function                   | Description                                 |
| -------------------------- | ------------------------------------------- |
| `Array.from({ length: n })` | Create the numeric array `0` through `n - 1` |
| `Array.of(a, b, ...)`        | Create an array from the given values         |
| `Array.isArray(value)`       | Static predicate for compiler-known arrays    |

`Array.from({ length })` is intentionally different from JavaScript: Besht creates a numeric array from `0` through `length - 1` and does not support general iterables or mapper callbacks.
`Array.isArray()` is evaluated from Besht's inferred representations. It returns true for expressions the compiler knows are arrays and false otherwise; it does not add runtime shape metadata or dynamic JavaScript-style inspection.

Object helpers:

| Function                 | Description                        |
| ------------------------ | ---------------------------------- |
| `Object.keys(obj)`       | Return object keys as a `string[]` |
| `Object.values(obj)`     | Return object values as a `string[]` |
| `Object.entries(obj)`    | Return object `[key, value]` rows as a `string[][]` |
| `Object.hasOwn(obj, key)` | Return whether a compiler-managed object has an exact key |
| `Object.assign(target, ...sources)` | Shallow-copy scalar-safe compiler-managed object fields, mutating and returning `target` |

These object helpers and object spread are metadata-backed Besht APIs, not general JavaScript reflection over arbitrary runtime objects. Keys must be valid Besht object keys, nested object/array/set/command/fetch values are rejected by the scalar-safe copy APIs, and `process.env` is not enumerable.

JSON helper:

| Function | Description |
| -------- | ----------- |
| `JSON.parse(value)` | Validate and compact JSON text as a `JSONValue` when compiled with `--opt-use-jq` |
| `JSON.stringify(value)` | Encode `JSONValue`, strings, numbers, booleans, null/undefined, scalar arrays, and scalar-valued compiler-managed objects as JSON when compiled with `--opt-use-jq` |

JSON support requires the `--opt-use-jq` compiler flag and invokes `jq` in generated code. Without the flag, compiling a program that calls `JSON.parse()` or `JSON.stringify()` is an error. `JSON.parse(text)` validates immediately with `jq -c .`; invalid JSON prints `[besht] JSON.parse() failed` and exits nonzero. Property and index access on a `JSONValue` returns another `JSONValue`; missing final properties, out-of-range array indexes, and JSON `null` become Besht nullish values for `??`. Accessing through a missing/null intermediate fails unless optional chaining is used. Add `: string`, `: number`, `: boolean`, or `as ...` when you want a JSON scalar extracted into a normal Besht value; wrong non-null JSON types fail at runtime. Generated shell shares JSON path and scalar-extraction helper functions instead of inlining the same jq programs at every read.

```ts
let data = JSON.parse("{\"user\":{\"name\":\"Ada\"},\"scores\":[7]}")
let name: string = data.user.name
let score = data.scores[0] as number
let title: string = data.user.title ?? "Engineer"

console.log(JSON.stringify({ id: 7, name: "Ada", active: true }))
console.log(JSON.stringify(["Ada", "Grace"]))
console.log(JSON.stringify(data.user))
console.log(JSON.stringify(Number.parseInt("2a", 10)))
```

### Type conversion

Use JS-style conversion APIs for new code:

| API                      | Description                                      |
| ------------------------ | ------------------------------------------------ |
| `value.toString()`       | Convert `string`, `number`, `boolean`, or `status` to `string` |
| `items.toString()`       | Convert a scalar array to a comma-joined string, like `items.join(",")` |
| `Number.parseInt(value)` | Parse `string` to `number`                       |
| `Number.parseInt(value, 10)` | Parse with an optional radix argument        |
| `Boolean(value)`          | Convert a value to a primitive boolean using JavaScript-like truthiness |

```ts
let n: number = 42
let msg: string = "Count is " + n.toString()

let raw: string = $("wc", "-l", "file").run().readStdout()
let lines: number = Number.parseInt(raw)
let flag: boolean = Boolean(raw)
let boolText: string = flag.toString() // "true"
```

`Boolean(value)` returns a primitive Besht boolean, not a Boolean object wrapper. It treats `false`, `0`, `0.0`, `""`, `null`, and `undefined` as false; non-empty strings such as `"0"` and `"false"`, non-zero numbers, arrays, and objects are true.

Other:

| Function             | Description                      |
| -------------------- | -------------------------------- |
| `console.log(s)`     | Print string + newline to stdout |
| `console.error(s)`   | Print string + newline to stderr |
| `process.env.NAME`  | Read environment variable; unset values are nullish |
| `process.exit(code)` | Exit with optional code          |

```ts
process.exit()  // exit 0
process.exit(7) // exit 7
process.exit(code) // exit with number or status variable

```

### Modules and imports

Split code across files with `export` and `import`.

```ts
// lib/log.bsh
export function info(msg: string) {
    $("printf", "[INFO] %s\n", msg).stderr("&1").run()
}

export function error(msg: string) {
    $("printf", "[ERROR] %s\n", msg).stderr("&1").run()
}

export const echoCmd = ["echo", "from module"]
export default ["echo", "default command"]
```

```ts
// main.bsh
import defaultCmd from "./lib/log";
import { info, error, echoCmd } from "./lib/log";

info("Starting up");
$(...echoCmd).run();
$(...defaultCmd).run();
// $(...echoCmd, "extra") is rejected; append extras to the array first.
```

Imports are resolved at compile time. Named imports can reference exported functions, classes, and top-level `export const`/`export let` values. `export default <expr>` is imported with `import name from "./module"`. All Besht modules are concatenated into a single `.sh` file in bundled mode. Extensionless imports use `.bsh` only unless `--opt-resolve-ts-imports` is passed; with that opt-in, `.bsh` still wins and `.ts` is used only if `.bsh` is absent.

Declaration files with the `.d.bsh` suffix provide editor-compatible declarations and compiler ABI hints without emitting shell. Declared functions are called by their declared names; besht does not generate wrappers for them. You can import declaration files explicitly, or place `stdlib.d.bsh` next to the entry file to make its declarations available automatically to bundled compile, split compile, and `--check`. Run `besht init` from a project directory to write the standard declarations to `./stdlib.d.bsh`; it will not overwrite different existing content unless you pass `besht init --force`. Only the entry directory is searched for this automatic stdlib file.

Existing POSIX shell files can be imported explicitly with named imports and an assertion:

```ts
// legacy.sh defines: legacy_log() { printf '%s\n' "$1"; }
import { legacy_log } from "./legacy.sh" assert { type: "shell" };

legacy_log("from besht");
```

`--check` validates imports using the same module resolver as compilation and rejects unsupported fetch response APIs such as `.status` and `.json()`. Shell imports require a `.sh` path and `assert { type: "shell" }`. They are named-only; default shell imports are rejected. Besht does not parse the shell file or infer exports, so imported shell functions are unchecked varargs and return `string` when used in value position. By default, shell imports must stay inside the compiler root. Pass `--opt-allow-external-shell-imports` to permit explicit `.sh` imports outside that root. Bundled output sources the resolved `.sh` file with a guard. Split output copies in-root `.sh` dependencies into the output tree and sources them through `_BESHT_ROOT`; external opt-in shell imports are sourced from their original absolute path. Shell import guards use unique relative shell paths so names like `a-b.sh` and `a_b.sh` cannot collide.

### Comments

```ts
// single-line

/* multi-line
   comment */
```

---

## Example: deploy script

```ts
import { info, error } from "./lib/log"

function deploy(env: string, version: string) {
    info("Deploying v${version} to ${env}")

    try {
        $("rsync", "-az", "./dist/", env + ":/opt/app/").run()
        $("ssh", env, "systemctl restart myapp").run()
        $("ssh", env, "curl -sf http://localhost:8080/health").run()
    } catch (code: status) {
        error("Deploy failed (exit ${code}), rolling back")
        $("ssh", env, "systemctl restart myapp-previous").run()
        process.exit(1)
    }

    info("Deploy successful")
}

let env: string = Besht.args.positional(1) ?? ""
let version: string = Besht.args.positional(2) ?? ""

deploy(env, version)
```

Compile and run:

```sh
besht compile deploy.bsh -o deploy.sh
chmod +x deploy.sh
./deploy.sh production v1.2.3
```

---

## Example: find large files

```ts
function format_size(bytes: number): string {
    if (bytes > 1073741824) {
        return (bytes / 1073741824).toString() + "GB"
    } else if (bytes > 1048576) {
        return (bytes / 1048576).toString() + "MB"
    }
    return (bytes / 1024).toString() + "KB"
}

let target: string = Besht.args.positional(1) ?? "."
let threshold: number = 1048576

for (file in $("find", target, "-type", "f").run().readStdoutLines()) {
    let size: number = $("wc", "-c", file).run().readStdout()
    if (size > threshold) {
        let human: string = format_size(size)
        $("printf", "%s\t%s\n", human, file).run()
    }
}
```

---

## CLI reference

```
besht compile <file.bsh>            Compile and print to stdout (single bundled file)
besht init                          Write ./stdlib.d.bsh declarations
besht init --force                  Overwrite ./stdlib.d.bsh declarations
besht compile <file.bsh> -o <out.sh>  Compile to a single bundled file
besht compile <file.bsh> --split -o <dir/>  Compile each file separately into <dir/>
besht compile --check <file.bsh>   Validate imports, command usage, and unsupported fetch APIs (no output)
besht visualize <file.bsh>         Open an in-terminal side-by-side source/shell view
besht compile <file.bsh> --opt-no-add-binaries-check  Omit runtime utility self-check when present
besht compile <file.bsh> --opt-no-source-map           Omit source comments from compiled output
besht compile <file.bsh> --opt-resolve-ts-imports      Resolve extensionless imports to .ts only when .bsh is absent
besht compile <file.bsh> --opt-allow-external-shell-imports  Allow explicit .sh imports outside the compiler root
besht compile <file.bsh> --opt-use-jq                  Enable jq-backed JSON codegen
besht --version                     Show version
besht --help                        Show usage
```

The legacy `besht <file.bsh> [flags]` and `besht --check <file.bsh>` forms remain accepted as aliases for `besht compile`.

Besht emits the runtime `printf`/`grep`/`sed` self-check only when generated output uses the compiler's `grep`/`sed` paths or the args runtime. Simple scripts that only need direct `printf` output skip it.

---

## Running tests

```sh
# Run all tests
make test

# Run node-eq parity fixtures
bun node-eq/compare $(rg --files -g '*.bsh' node-eq/tests | sort)

# Run only JSON parity fixtures
bun node-eq/compare $(rg --files -g '*.bsh' node-eq/tests/language/json | sort)

# Run with coverage report (terminal)
make cover

# Run with coverage report (HTML)
make cover-html
open coverage.html
```

## Maintaining the Besht scripting skill

`todo.md` documents the ongoing loop for improving `skills/besht-scripting/SKILL.md` with no-hints validation agents and node-eq guardrail fixtures. That process is intentionally never-ending: new compiler features, new idioms, and new agent failure modes should keep feeding small skill-improvement iterations.
