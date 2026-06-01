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
besht script.bsh

# Generate editor declarations in ./stdlib.d.bsh
besht init

# Compile to a single bundled file
besht script.bsh -o script.sh

# Compile each .bsh file to its own .sh file (recommended for multi-file projects)
besht script.bsh --split -o build/

# Validate imports, command usage, and unsupported fetch APIs
besht --check script.bsh

# Run directly
besht script.bsh | sh
```

## Output modes

### Bundled (default)

All imported Besht modules are inlined into a single `.sh` file. Explicit `.sh` imports are sourced from the generated script. Good for one-file scripts and piping to `sh`.

One-file bundled output omits module separator comments for a more natural small-script shape. Bundled output with multiple Besht modules keeps `# --- module: name ---` separators between modules.

### Split (`--split`)

Each `.bsh` file compiles to its own `.sh` file in the output directory, preserving the source directory structure. Besht imports become POSIX source (`. file.sh`) calls at runtime. Explicit `.sh` imports are copied into the output directory and sourced with include guards.

```sh
besht main.bsh --split -o build/
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

---

## Language

Files use the `.bsh` extension.

### Current behavior

- Type annotations and `as` assertions are accepted for TypeScript-compatible syntax, editor support, and occasional compiler representation hints. They never validate values or produce type mismatch errors.
- Template literals support `${expr}`, not just `${name}`.
- `for (item of items)` and `for (let item of items)` are accepted as aliases for `for (item in items)`.
- Ternary expressions `cond ? a : b` and nullish coalescing `value ?? fallback` are supported.
- Compound assignments `+=`, `-=`, and `*=` are supported.
- Postfix `++`/`--` are supported in statement position; prefix `++name`/`--name` are supported in expression position.
- Logical operators `&&`, `||`, `!`, and nullish coalescing `??` are supported.
- Static boolean `console.log()`/`console.error()` arguments such as `Boolean("")`, `true`, and `!false` render directly as `true`/`false`; static boolean `if` and ternary conditions such as `Boolean(value)`, `Array.isArray(value)`, static string/list searches, static `Object.hasOwn()`, and static comparisons fold to the selected branch or value. Dynamic booleans keep the general formatting path.
- Strict equality `===` and `!==` compile the same as `==` and `!=`.
- Static scalar equality comparisons and static numeric relational comparisons, including arithmetic literal operands, compile to constants. Dynamic relational comparisons over compiler-known integer expressions use POSIX `[ ]`; floats and unknown values keep the `awk` path. Dynamic equality keeps the multiline-safe shell path.
- String equality preserves spaces and newlines, including multiline template literals.
- Static boolean `if` conditions and ternary expressions fold to the selected branch or value; dynamic conditions keep normal shell tests.
- Static ASCII string literals, variables bound to static ASCII string literals, and chained static ASCII transforms fold `.includes()`, `.startsWith()`, `.endsWith()`, `.indexOf()`, `.lastIndexOf()`, and `.charAt()` calls with static arguments to constants; dynamic and non-ASCII string searches keep the helper/`awk` path.
- Static ASCII string literal `.split()` calls with static separators compile to quoted newline-backed list strings and compact `for` loops when elements contain no newlines; dynamic and non-ASCII splits keep the POSIX tool path.
- Static string literal `Number.parseInt()` calls with parseable prefixes and static radix compile to numeric constants; dynamic calls keep the shell arithmetic path.
- Static numeric arithmetic over literal numbers compiles to constants; dynamic arithmetic keeps shell arithmetic or POSIX `awk`.
- Static numeric literal `.toString()`/`.toFixed()` calls and literal-argument `Math.*` calls compile to constants; dynamic numeric calls keep the POSIX `awk` path.
- Static primitive `.toString()` fragments inside string concatenation and template interpolation compile to constants; dynamic receivers keep the normal runtime formatting path.
- Static ASCII string literals, variables bound to static ASCII string literals, and chained static ASCII transforms fold transforms such as `.trim()`, `.toUpperCase()`, `.slice()`, `.substring()`, `.repeat()`, and `.padStart()`/`.padEnd()` with static arguments to constants; dynamic and non-ASCII transforms keep the POSIX tool path.
- `switch/case/default` compiles to shell `case/esac`.
- `if`/`else if`/`else`, `for`, and `while` bodies can be either braced blocks or a single bracketless statement; multiple statements still require braces.
- Static scalar list literals and list-returning method chains over static scalar lists (`concat`, `slice`, `reverse`, `push`, `unshift`, `pop`, `shift`) compile to quoted newline-backed shell strings when values do not contain newlines; dynamic, spread, nested, and newline-sensitive lists keep the `printf` builder.
- Static scalar `Array.of(...)` calls and static `Array.from({ length: N })` calls compile to quoted newline-backed shell strings and compact loops when values contain no newlines; dynamic factories keep the existing builder path.
- Static string literals, variables bound to static string literals, and static scalar list expression `.length` properties compile to numeric constants; dynamic lengths keep the POSIX `wc` path.
- `for (... of [...])` and loops over static scalar list expressions or variables bound to them compile to compact shell `for` loops when values do not contain newlines; dynamic lists keep the newline-safe read loop.
- Static scalar list indexes with known in-range integer indexes compile to constants; dynamic, unknown, and out-of-range indexes keep the POSIX `sed` path.
- Static scalar list expression `.join()` and `.toString()` calls compile to one quoted string when elements contain no newlines and the separator is static; dynamic joins keep the newline-safe `awk` path.
- Static scalar list expression `.includes()`, `.indexOf()`, and `.lastIndexOf()` calls with static scalar needles compile to constants; dynamic searches keep the POSIX `grep`/`awk` path.
- Inline static scalar object literal `Object.keys()`, `Object.values()`, `Object.entries()`, and `Object.hasOwn()` calls compile to constants; unmutated named object `Object.keys()` and static-key `Object.hasOwn()` calls also fold from compiler-managed key metadata.
- Object literals compile to per-property shell variables; `Object.keys(obj)` returns known object keys as `string[]`, `Object.values(obj)` returns values as `string[]`, `Object.entries(obj)` returns `[key, value]` rows as `string[][]`, and `Object.hasOwn(obj, key)` checks known key membership.
- Boolean object properties used directly in conditions compile to direct `= 1` shell tests; non-boolean property conditions keep generic JavaScript-style truthiness.
- Classes support constructors, instance properties/methods, `new`, `this`, static properties/methods, and getters/setters.
- TypeScript-only class modifiers such as `private`, `public`, `protected`, and `readonly` are accepted and ignored.
- `Record<K, V>` annotations are accepted for object-map style code; they are annotations only and add no runtime type checks.
- Tuple destructuring declarations such as `const [x, y] = pair` are supported for list/tuple values.
- `null` and `undefined` are accepted for TypeScript-compatible syntax; `??` falls back only for nullish values and preserves `""`, `0`, and `false`. Static nullish coalescing folds to the known side when the left operand is provably nullish or non-nullish. Optional chaining supports `obj?.prop`, `obj?.[key]`, `obj?.method()`, and nested chains.
- `$()` calls support list spreading with `...args`; spreading an entire command vector must use sole-argument form `$(...cmd)`.
- Literal `$()` command words are emitted bare when they are conservative shell-safe words, and quoted when they need protection.
- Single-command stdout/stderr redirects compile directly as `cmd > file`; pipeline redirects keep grouping braces so the redirect applies to the whole pipeline.
- `.d.bsh` files are declaration-only and never emit shell output. A `stdlib.d.bsh` file beside the entry `.bsh` file is auto-loaded for compile, split compile, and `--check`; run `besht init` to generate one in the current directory. Imported module directories are not scanned for their own `stdlib.d.bsh` files.
- Extensionless imports resolve to `.bsh` by default. Pass `--opt-resolve-ts-imports` to fall back to `.ts` only when the `.bsh` file is absent. Explicit named `.sh` imports require `assert { type: "shell" }` and source existing shell functions. Shell imports must resolve inside the compiler root unless `--opt-allow-external-shell-imports` is passed.
- `type` aliases and `interface` declarations are parsed and silently ignored (no runtime output).
- Simple `type Name = ExistingType` aliases can be used in annotations, including `string[]` and `Set<string>`.
- Type assertions such as `[] as string[]` are parsed and erased at compile time.
- `new Set<T>()` supports `.add(value)` and `.has(value)` with no runtime type checking; straight-line static adds and membership checks compile to constants.
- Nested lists such as `string[][]` preserve row structure for `.map()`, nested indexing, and row `.length`.
- Generated shell includes `# besht:file:line:col` source comments at non-class statement boundaries and before explicit class constructor/accessor/method shell functions.
- Semicolons are optional (only required inside `for` headers).
- `Array.from({ length })` creates a numeric list from `0` to `length - 1`; `Array.of(...)` creates a list from the given values; `Array.isArray(value)` is a static predicate for compiler-known list values and adds no runtime shape metadata.
- `Object.keys(obj)`, `Object.values(obj)`, `Object.entries(obj)`, and `Object.hasOwn(obj, key)` use compiler-managed object key metadata and do not emit runtime helpers.
- `fetch(url).text()` is a synchronous, curl-backed, text-only GET slice. It emits `curl -sS -- <url>` and intentionally does not support `await`, options, POST, headers, body, `.json()`, `.status`, `.ok`, or `.headers` yet.
- Arrow callbacks support expression and block bodies for list `.map()`, `.reduce()`, and statement-position `.forEach()`; `.map()`, `.filter()`, `.some()`, `.every()`, `.find()`, `.findIndex()`, and `.forEach()` callbacks may also receive a zero-based index parameter.
- Generated shell elides string runtime helpers unless one-argument string `.includes()`, `.startsWith()`, or `.endsWith()` actually needs them.

### Variables

Declare with `let`. Types are optional and are never validated by the compiler.

```ts
let name: string = "Alice";
let count: number = 42;
let price: number = 3.14; // float literal supported
let verbose: boolean = true;
let files: list<string> = ["a.txt", "b.txt", "c.txt"];
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

Available annotation forms include `string`, `number`, `boolean`, `object`, `list<T>`, `T[]`, `Array<T>`, `Set<T>`, `status`, union types (`string | null`), and tuple types (`[string, number]`). Besht does not type-check annotations; `null` and `undefined` are runtime nullish sentinels for `??` and comparisons.

### Strings

Both `"..."` and `'...'` are plain literals — no interpolation. Use backtick template literals for interpolation and embedded expressions:

```ts
let name: string = "Alice"           // plain literal
let also: string = 'Alice'           // same — plain literal
let tmpl: string = `Hello ${name}!`  // template literal — interpolates ${name}
let sum = `sum=${a + b}`             // expressions inside ${...}
let pattern: string = r"^foo-[0-9]+" // raw string — always single-quoted in sh
let rawpath = String.raw`C:\temp\new\file.txt` // tagged raw template — backslashes literal
let escape: string = "newline:\n tab:\t backslash:\\ quote:\" dollar:\$"  // escape sequences
let unicode: string = "A \u0041 ñ \u00F1"  // unicode escapes
```

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

`console.log()` writes a string to stdout with a trailing newline. `console.error()` writes the same format to stderr. Lists print in Bun-style `[ a, b ]` format; static scalar list output compiles to one quoted `printf` line, while dynamic and newline-sensitive lists keep the generic formatter. Objects print each property on its own line and reflect current property values. Inline object literals compile to a direct multi-line `printf`.

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

**For — list:**

```ts
let fruits: list<string> = ["apple", "banana", "cherry"];
for (fruit in fruits) {
  $("echo", fruit).run();
}

for (let fruit of fruits) {
  $("echo", fruit).run();
}

for (fruit of fruits) $("echo", fruit).run()
```

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

// || and && in value position return actual values (JS semantics)
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
s.split(","); // list<string> ["  Hello", " World!  "]
s.split(""); // list<string> of characters
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
s.padStart(20, "-"); // "-----  Hello, World!  "
s.padEnd(20, "."); // "  Hello, World!  ..."
s.concat(" More text"); // "  Hello, World!   More text"
s.length; // int (character count)
```

One-argument string `includes()`, `startsWith()`, and `endsWith()` use tiny POSIX helper functions that are emitted only when the generated shell calls them. Two-argument string search methods use inline `awk` instead. Static ASCII string literal searches and searches on variables bound to static ASCII string literals compile to constants when arguments are static.

Static ASCII string literal transforms, transforms on variables bound to static ASCII string literals, and chained static ASCII transforms compile to constants when arguments are static. Dynamic and non-ASCII transforms use POSIX tools such as `sed`, `tr`, `cut`, and `awk`.

Static ASCII string literal `.split()` calls with static separators compile to constants when the resulting list elements contain no newlines. Dynamic and non-ASCII splits use POSIX tools such as `tr` and `awk`.

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

function showKeys(obj: object): string[] {
  return Object.keys(obj)
}
```

Static and computed object keys must contain only letters, numbers, and `_`.
`Object.keys(obj)`, `Object.values(obj)`, and `Object.entries(obj)` return known keys, scalar values, or `[key, value]` rows in insertion order and include aliases, object parameters, later `obj.prop = value`, and `obj[key] = value` additions. Inline static scalar object literals compile these calls to constants; unmutated named object `Object.keys()` and static-key `Object.hasOwn()` calls also fold to constants. Statically known boolean values are rendered as `true`/`false` in values and entries output. `Object.values()` and `Object.entries()` reject statically known list/object/set/command/fetch values because the current `string[]` and packed `string[][]` representations cannot preserve deeper nested values. `Object.hasOwn(obj, key)` checks exact key membership against the same metadata and returns `false` for dynamic keys that are not valid Besht object keys. These helpers do not emit a runtime helper library. `process.env` is not enumerable; access individual variables with `process.env.NAME`.

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

### List methods

Lists have TypeScript-compatible Array methods:

```ts
let l: list<string> = ["alpha", "beta", "gamma"];
let matrix: string[][] = ["ab", "cd"].map(row => row.split("") as string[]);
let indexes: number[] = Array.from({ length: 3 }); // [0, 1, 2]
let chosen: string[] = Array.of("alpha", "omega"); // ["alpha", "omega"]
let isList: boolean = Array.isArray(chosen); // true for compiler-known lists

l.push("delta"); // new list with "delta" appended
l.unshift("zero"); // new list with "zero" prepended
l.pop(); // new list without last element
l.shift(); // new list without first element
l.concat(other); // two lists joined
l.slice(1, 3); // ["beta", "gamma"]
l.join(", "); // "alpha, beta, gamma"
["a", "b", "c"].join(","); // static scalar literal compiles to 'a,b,c'
["a", "b"].concat(["c"]).join(","); // static scalar chains compile to 'a,b,c'
l.toString(); // "alpha,beta,gamma" for scalar lists; same as l.join(",")
l.includes("beta"); // boolean, uses `grep -qxF` membership and does not emit the string `_bst_includes` helper
l.indexOf("gamma"); // int (0-based, -1 if not found)
l.lastIndexOf("beta"); // int (last zero-based match, -1 if not found)
l.reverse(); // ["gamma", "beta", "alpha"]
l.map(x => x + "!"); // new list with callback expression applied to each item
l.map((x, i) => i.toString() + ":" + x); // second callback arg is zero-based index
l.forEach((x, i) => console.log(i.toString() + ":" + x)); // statement-only side effects
l.filter(x => x.startsWith("a")); // new list with truthy callback results
let anyA = l.some(x => x.startsWith("a")); // true if any callback result is truthy; false for an empty list
let allNamed = l.every(x => x.length > 0); // true if all callback results are truthy; true for an empty list
let hit = l.find((x, i) => i == 1) ?? "missing"; // first matching element, or nullish when no match
let at = l.findIndex(x => x == "beta"); // 1, or -1 if no match
let copied = [...l, "omega"]; // list spread in list literals
l.length; // number
matrix[0][1]; // nested indexing
matrix[0].length; // row length
const [row, col] = [1, 2]; // tuple/list destructuring
let maybe = matrix?.[row]?.[col] ?? "missing"
```

`list.toString()` is currently a scalar-list API slice. JavaScript nested-list flattening for `string[][]` and packed row lists is not implemented.

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

Arrow callbacks support both expression-bodied and block-bodied forms for list `.map()`, `.filter()`, `.reduce()`, and statement-position `.forEach()`. Scalar-list predicate callbacks for `.some()`, `.every()`, `.find()`, and `.findIndex()` are direct arrow expressions with one item parameter or `(item, index)`.

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

// reduce with expression body
let total = nums.reduce((acc, n) => acc + n, 0)
let lines = nums.reduce((acc, n) => [...acc, "#".repeat(n)], [] as string[]).join("\n")

// reduce with block body and object accumulator
let counts = words.reduce((acc, word) => {
    acc[word] = (acc[word] || 0) + 1
    return acc
}, {})
console.log(counts)
```

`.map()` supports expression or block bodies and one or two parameters: `(item)` or `(item, index)`. `return` inside a block-bodied `.map()` callback emits that mapped value for the current item and continues the callback loop. Block-bodied `.map()` callbacks currently support `return`, `if`/`else`, and assignment statements; arbitrary expression statements are rejected. `.filter()`, `.some()`, `.every()`, `.find()`, and `.findIndex()` use JavaScript-style truthiness and may receive `(item, index)`. `.some()` short-circuits on the first truthy callback result and returns `false` for an empty list. `.every()` short-circuits on the first falsey callback result and returns `true` for an empty list. `.find()` returns the first matching scalar element, or a nullish value when no element matches so `??` fallbacks work. `.reduce()` takes a 2-parameter arrow (accumulator, current) with either expression or block body, plus an initial value. `.forEach()` is statement-only, takes a direct arrow callback with `(item)` or `(item, index)`, compiles static scalar receivers to compact `for` loops, runs in the current shell so outer assignments and `Set.add()` side effects persist, and rejects callback `return`, `break`, `continue`, and pure value expressions. Arrows are not general function values and cannot be stored in variables; general arrow function values are still future work.

### List indexing

Access list elements by zero-based index. Nested lists preserve row boundaries, so `matrix[row][col]`, `matrix.length`, and `matrix[0].length` work for `T[][]` values:

```ts
let args: list<string> = $("printf", "%s\n", "a", "b").run().readStdoutLines()
let first: string = args[0]
let second: string = args[1]
let item: string = args[i]   // variable index
args[1] = "BETA"             // index assignment
let empty: string[] = []     // empty list

let matrix: string[][] = rows.map(row => row.split("") as string[])
let cell: string = matrix[row][col]
let width: number = matrix[0].length
```

Static scalar list indexes with known in-range integer indexes compile to constants. Dynamic, unknown, and out-of-range indexes compile to a `sed -n` line extraction (POSIX sh compatible). Index assignment uses `awk` to replace the Nth line.

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

Use `.env(name, value)` to prefix one command invocation with an environment variable. The name must be a string literal POSIX shell identifier.

```ts
$("make", "build").env("CI", "1").run() // CI=1 make build
```

Use `.workdir(path)` to run a command or pipeline from a specific directory without changing the parent script's current directory.

```ts
let root: string = $("pwd").workdir("/").run().readStdout() // /
$("make", "test").workdir("/repo/app").run()
```

### Besht helpers

Besht-specific helpers live under the global `Besht` object. They compile to inline POSIX tests or small generated shell; they are not runtime namespace objects.

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

List operations should use native list syntax and methods:

| Native API             | Description                   |
| ---------------------- | ----------------------------- |
| `list.length`          | Number of elements            |
| `list[0]`              | First element                 |
| `list.slice(1)`        | All elements except the first |
| `list.push(value)`     | New list with value appended  |
| `[...list, value]`     | New list with value appended  |
| `list.includes(value)` | True if value is in list      |
| `list.concat(other)`   | Concatenate two lists         |

Array helpers:

| Function                   | Description                                 |
| -------------------------- | ------------------------------------------- |
| `Array.from({ length: n })` | Create the numeric list `0` through `n - 1` |
| `Array.of(a, b, ...)`        | Create a list from the given values          |
| `Array.isArray(value)`       | Static predicate for compiler-known list values |

`Array.isArray()` is evaluated from Besht's inferred representations. It returns true for expressions the compiler knows are lists and false otherwise; it does not add runtime shape metadata or dynamic JavaScript-style inspection.

Object helpers:

| Function                 | Description                        |
| ------------------------ | ---------------------------------- |
| `Object.keys(obj)`       | Return object keys as a `string[]` |
| `Object.values(obj)`     | Return object values as a `string[]` |
| `Object.entries(obj)`    | Return object `[key, value]` rows as a `string[][]` |
| `Object.hasOwn(obj, key)` | Return whether a compiler-managed object has an exact key |

### Type conversion

Use JS-style conversion APIs for new code:

| API                      | Description                                      |
| ------------------------ | ------------------------------------------------ |
| `value.toString()`       | Convert `string`, `number`, `boolean`, or `status` to `string` |
| `list.toString()`        | Convert a scalar list to a comma-joined string, like `list.join(",")` |
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
// $(...echoCmd, "extra") is rejected; append extras to the list first.
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
besht deploy.bsh -o deploy.sh
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
besht <file.bsh>                    Compile and print to stdout (single bundled file)
besht init                          Write ./stdlib.d.bsh declarations
besht init --force                  Overwrite ./stdlib.d.bsh declarations
besht <file.bsh> -o <out.sh>        Compile to a single bundled file
besht <file.bsh> --split -o <dir/>  Compile each file separately into <dir/>
besht --check <file.bsh>            Validate imports, command usage, and unsupported fetch APIs (no output)
besht <file.bsh> --opt-no-add-binaries-check  Omit runtime utility self-check when present
besht <file.bsh> --opt-no-source-map            Omit source comments from compiled output
besht <file.bsh> --opt-resolve-ts-imports       Resolve extensionless imports to .ts only when .bsh is absent
besht <file.bsh> --opt-allow-external-shell-imports  Allow explicit .sh imports outside the compiler root
besht --version                     Show version
besht --help                        Show usage
```

Besht emits the runtime `printf`/`grep`/`sed` self-check only when generated output uses the compiler's `grep`/`sed` paths or the args runtime. Simple scripts that only need direct `printf` output skip it.

---

## Running tests

```sh
# Run all tests
make test

# Run node-eq parity fixtures
bun node-eq/compare $(rg --files -g '*.bsh' node-eq/tests | sort)

# Run with coverage report (terminal)
make cover

# Run with coverage report (HTML)
make cover-html
open coverage.html
```
