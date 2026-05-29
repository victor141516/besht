# besht

A TypeScript-flavored language that compiles to POSIX sh.

Write shell scripts that are readable and portable. By default, besht accepts TypeScript-style annotations for editor help and compiles without type validation; pass `--strict` to enable compile-time type checking.

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

# Generate editor/strict-check declarations in ./stdlib.d.bsh
besht init

# Compile to a single bundled file
besht script.bsh -o script.sh

# Compile each .bsh file to its own .sh file (recommended for multi-file projects)
besht script.bsh --split -o build/

# Type-check and validate imports only
besht --check script.bsh

# Type-check and validate annotations
besht --check --strict script.bsh

# Run directly
besht script.bsh | sh
```

## Output modes

### Bundled (default)

All imported Besht modules are inlined into a single `.sh` file. Explicit `.sh` imports are sourced from the generated script. Good for one-file scripts and piping to `sh`.

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

- Type annotations are optional and ignored unless `--strict` is enabled.
- Template literals support `${expr}`, not just `${name}`.
- `for (item of items)` and `for (let item of items)` are accepted as aliases for `for (item in items)`.
- Ternary expressions `cond ? a : b` and nullish coalescing `value ?? fallback` are supported.
- Compound assignments `+=`, `-=`, and `*=` are supported.
- Postfix `++`/`--` are supported in statement position; prefix `++name`/`--name` are supported in expression position.
- Logical operators `&&`, `||`, `!`, and nullish coalescing `??` are supported.
- Strict equality `===` and `!==` compile the same as `==` and `!=`.
- String equality preserves spaces and newlines, including multiline template literals.
- `switch/case/default` compiles to shell `case/esac`.
- `if`/`else if`/`else`, `for`, and `while` bodies can be either braced blocks or a single bracketless statement; multiple statements still require braces.
- Object literals compile to per-property shell variables.
- Classes support constructors, instance properties/methods, `new`, `this`, static properties/methods, and getters/setters.
- TypeScript-only class modifiers such as `private`, `public`, `protected`, and `readonly` are accepted and ignored.
- `Record<K, V>` annotations are accepted for object-map style code; they are annotations only and add no runtime type checks.
- Tuple destructuring declarations such as `const [x, y] = pair` are supported for list/tuple values.
- `null` and `undefined` are accepted for TypeScript-compatible syntax; `??` falls back only for nullish values and preserves `""`, `0`, and `false`. Optional chaining supports `obj?.prop`, `obj?.[key]`, `obj?.method()`, and nested chains.
- `$()` calls support list spreading with `...args`; spreading an entire command vector must use sole-argument form `$(...cmd)`.
- `.d.bsh` files are declaration-only and never emit shell output. A `stdlib.d.bsh` file beside the entry `.bsh` file is auto-loaded for compile, split compile, and `--check`; run `besht init` to generate one in the current directory. Imported module directories are not scanned for their own `stdlib.d.bsh` files.
- Extensionless imports resolve to `.bsh` by default. Pass `--opt-resolve-ts-imports` to fall back to `.ts` only when the `.bsh` file is absent. Explicit named `.sh` imports require `assert { type: "shell" }` and source existing shell functions. Shell imports must resolve inside the compiler root unless `--opt-allow-external-shell-imports` is passed.
- `type` aliases and `interface` declarations are parsed and silently ignored (no runtime output).
- Simple `type Name = ExistingType` aliases can be used in annotations, including `string[]` and `Set<string>`.
- Type assertions such as `[] as string[]` are parsed and erased at compile time.
- `new Set<T>()` supports `.add(value)` and `.has(value)` with no runtime type checking.
- Nested lists such as `string[][]` preserve row structure for `.map()`, nested indexing, and row `.length`.
- Generated shell includes `# besht:file:line:col` source comments at non-class statement boundaries and before explicit class constructor/accessor/method shell functions.
- Semicolons are optional (only required inside `for` headers).
- `Array.from({ length })` creates a numeric list from `0` to `length - 1`; `Array.isArray(value)` is a static predicate for compiler-known list values and adds no runtime shape metadata.
- Arrow callbacks support expression and block bodies for list `.map()` and `.reduce()`; `.map()`, `.filter()`, `.some()`, `.every()`, `.find()`, and `.findIndex()` callbacks may also receive a zero-based index parameter.
- Generated shell elides string runtime helpers unless one-argument string `.includes()`, `.startsWith()`, or `.endsWith()` actually needs them.

### Variables

Declare with `let`. Types are optional and only validated with `--strict`.

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

Available types: `string`, `number`, `boolean`, `list<T>`, `T[]`, `Array<T>`, `Set<T>`, `status`, union types (`string | null`), tuple types (`[string, number]`). Type annotations are ignored by the compiler unless `--strict` is enabled. `null` and `undefined` are runtime nullish sentinels for `??` and comparisons.

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

Use `process.env.NAME` for JavaScript-style environment access. Missing variables are nullish, so `??` falls back only when the variable is unset and preserves an explicitly empty value.

```ts
let home: string = process.env.HOME
let port: string = process.env.PORT ?? "8080"
let debug: string = process.env.DEBUG ?? "false"
```

The older `env()` helper remains supported for now:

```ts
let home: string = env("HOME")
let port: string = env("PORT", "8080")
```

### Script arguments

Use `args` to read command-line arguments passed to the compiled shell script. Missing positional values and options return a nullish value, so use `??` for defaults. Empty strings are preserved.

```ts
let all: string[] = args.argv()
let input = args.positional(1) ?? "-"
let branch = args.option("branch", "b") ?? "main"
let dryRun = args.flag("dry-run", "d")
```

- `args.argv()` returns positional arguments as `string[]` after option parsing.
- `args.positional(n)` returns the 1-based positional argument or a nullish value when absent.
- `args.option(longName, shortName?)` supports `--name=value`, `--name value`, and `-n value`; absent options are nullish.
- `args.flag(longName, shortName?)` returns `true` when `--name` or `-n` is present.
- `--` stops option and flag parsing; later values are treated as positional arguments.

### Nullish coalescing

`a ?? b` returns `a` unless it is `null` or `undefined`. Unlike shell `${VAR:-default}`, it preserves valid falsey values.

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

`console.log()` writes a string to stdout with a trailing newline. `console.error()` writes the same format to stderr. Lists print in Bun-style `[ a, b ]` format; objects print each property on its own line.

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
  $("echo", to_str(i)).run();
}

for (let i: number = 0; i < 10; i++) total += i
```

**For — range:**

```ts
for (i in range(1, 10)) {
  $("echo", to_str(i)).run();
}

for (i in range(1, 10)) total += i
```

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
  if (is_empty(f)) {
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
let port = args.option("port", "p") ?? "8080"
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

One-argument string `includes()`, `startsWith()`, and `endsWith()` use tiny POSIX helper functions that are emitted only when the generated shell calls them. Two-argument string search methods use inline `awk` instead.

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

All `Math` methods are compiled to `awk` arithmetic and support decimal numbers. POSIX `$((...))` is integer-only, so besht uses `awk` wherever a float operand is present.

When a variable is reassigned, besht updates its float-tracking metadata from the new right-hand side: float-producing expressions keep later arithmetic on `awk`, while integer/non-float reassignment clears the float marker so later integer arithmetic can use shell integer lowering again.

### Number builtins

```ts
Number.parseInt("42");        // parse string to integer
Number.parseInt("42", 10);    // optional radix argument
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
```

Computed object keys must contain only letters, numbers, and `_`.

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
l.toString(); // "alpha,beta,gamma" for scalar lists; same as l.join(",")
l.includes("beta"); // boolean, uses `grep -qxF` membership and does not emit the string `_bst_includes` helper
l.indexOf("gamma"); // int (0-based, -1 if not found)
l.lastIndexOf("beta"); // int (last zero-based match, -1 if not found)
l.reverse(); // ["gamma", "beta", "alpha"]
l.map(x => x + "!"); // new list with callback expression applied to each item
l.map((x, i) => i.toString() + ":" + x); // second callback arg is zero-based index
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

`Set<T>` is a lightweight newline-backed collection for membership tracking. Type parameters are annotations only; `.add(value)` mutates the set and `.has(value)` checks membership without runtime type checks.

```ts
let visited = new Set<string>()
visited.add("0,0")
if (visited.has("0,0")) {
  console.log("seen")
}
```


### Arrow callbacks

Arrow callbacks support both expression-bodied and block-bodied forms for list `.map()`, `.filter()`, and `.reduce()`. Scalar-list predicate callbacks for `.some()`, `.every()`, `.find()`, and `.findIndex()` are direct arrow expressions with one item parameter or `(item, index)`.

```ts
let names = ["alice", "bob", "anna"]
let shouted = names.map(name => name.toUpperCase())
let aNames = shouted.filter(name => name.startsWith("A"))
let hasAnna = names.some(name => name == "anna")
let allShort = names.every((name, i) => name.length < 10 && i >= 0)
let firstB = names.find(name => name.startsWith("b")) ?? "none"
console.log(aNames.join(","))

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

`.map()` supports expression or block bodies and one or two parameters: `(item)` or `(item, index)`. `return` inside a block-bodied `.map()` callback emits that mapped value for the current item and continues the callback loop. Block-bodied `.map()` callbacks currently support `return`, `if`/`else`, and assignment statements; arbitrary expression statements are rejected. `.filter()`, `.some()`, `.every()`, `.find()`, and `.findIndex()` use JavaScript-style truthiness and may receive `(item, index)`. `.some()` short-circuits on the first truthy callback result and returns `false` for an empty list. `.every()` short-circuits on the first falsey callback result and returns `true` for an empty list. `.find()` returns the first matching scalar element, or a nullish value when no element matches so `??` fallbacks work. `.reduce()` takes a 2-parameter arrow (accumulator, current) with either expression or block body, plus an initial value. Arrows are not general function values and cannot be stored in variables. `forEach` and general arrow function values are still future work.

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

Compiles to a `sed -n` line extraction (POSIX sh compatible). Index assignment uses `awk` to replace the Nth line.

### Error handling

`try / catch` traps command failures. The catch variable holds the exit code.

```ts
try {
    $("rsync", "-az", "./dist/", "server:/opt/app/").run()
    $("ssh", "server", "systemctl restart myapp").run()
} catch (code: status) {
    $("echo", `Deploy failed with exit code ${code}`).stderr("&1").run()
    exit(1)
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

### Built-in functions

These compile to inline shell tests — they are not real function calls.

| Function           | Condition emitted |
| ------------------ | ----------------- |
| `file_exists(p)`   | `[ -f "$p" ]`     |
| `is_dir(p)`        | `[ -d "$p" ]`     |
| `is_readable(p)`   | `[ -r "$p" ]`     |
| `is_writable(p)`   | `[ -w "$p" ]`     |
| `is_executable(p)` | `[ -x "$p" ]`     |
| `is_empty(s)`      | `[ -z "$s" ]`     |
| `is_set(s)`        | `[ -n "$s" ]`     |

The same tests are also available through the `Besht.conditions.*` namespace. These wrappers compile to the same inline shell tests as the older global names, work in strict mode, and do not add runtime helper functions. The older names remain supported for now.

| Wrapper                              | Older name         | Condition emitted |
| ------------------------------------ | ------------------ | ----------------- |
| `Besht.conditions.fileExists(p)`     | `file_exists(p)`   | `[ -f "$p" ]`     |
| `Besht.conditions.isDir(p)`          | `is_dir(p)`        | `[ -d "$p" ]`     |
| `Besht.conditions.isReadable(p)`     | `is_readable(p)`   | `[ -r "$p" ]`     |
| `Besht.conditions.isWritable(p)`     | `is_writable(p)`   | `[ -w "$p" ]`     |
| `Besht.conditions.isExecutable(p)`   | `is_executable(p)` | `[ -x "$p" ]`     |
| `Besht.conditions.isEmpty(s)`        | `is_empty(s)`      | `[ -z "$s" ]`     |
| `Besht.conditions.isSet(s)`          | `is_set(s)`        | `[ -n "$s" ]`     |

List operations should use native list syntax in new code:

| Native API                    | Older helper alias       | Description                   |
| ----------------------------- | ------------------------ | ----------------------------- |
| `list.length`                 | `len(list)`              | Number of elements            |
| `list[0]`                     | `head(list)`             | First element                 |
| `list.slice(1)`               | `tail(list)`             | All elements except the first |
| `list.push(value)`            | `append(list, value)`    | New list with value appended  |
| `[...list, value]`            | `append(list, value)`    | New list with value appended  |
| `list.includes(value)`        | `contains(list, value)`  | True if value is in list      |
| `list.concat(other)`          | `concat(list, other)`    | Concatenate two lists         |

The old helper names remain supported for compatibility. Prefer the native forms above for new code.

Array helpers:

| Function                   | Description                                 |
| -------------------------- | ------------------------------------------- |
| `Array.from({ length: n })` | Create the numeric list `0` through `n - 1` |
| `Array.of(a, b, ...)`        | Create a list from the given values          |
| `Array.isArray(value)`       | Static predicate for compiler-known list values |

`Array.isArray()` is evaluated from Besht's inferred types. It returns true for expressions the compiler knows are lists and false otherwise; it does not add runtime shape metadata or dynamic JavaScript-style inspection.

### Type conversion

Use JS-style conversion APIs for new code:

| API                      | Description                                      |
| ------------------------ | ------------------------------------------------ |
| `value.toString()`       | Convert `string`, `number`, `boolean`, or `status` to `string` |
| `list.toString()`        | Convert a scalar list to a comma-joined string, like `list.join(",")` |
| `Number.parseInt(value)` | Parse `string` to `number`                       |
| `Number.parseInt(value, 10)` | Parse with an optional radix argument        |

```ts
let n: number = 42
let msg: string = "Count is " + n.toString()

let raw: string = $("wc", "-l", "file").run().readStdout()
let lines: number = Number.parseInt(raw)
let flag: boolean = true
let boolText: string = flag.toString() // "true"
```

Older helpers remain supported for now: `to_str(value)`, `String(value)`, and `to_int(str)`.

Other:

| Function             | Description                      |
| -------------------- | -------------------------------- |
| `env(NAME)`          | Read environment variable        |
| `env(NAME, default)` | Read env var with fallback       |
| `console.log(s)`     | Print string + newline to stdout |
| `console.error(s)`   | Print string + newline to stderr |
| `exit(code)`         | Exit with code                   |
| `process.env.NAME`  | Read environment variable; unset values are nullish |
| `process.exit(code)` | Exit with optional code          |

```ts
process.exit()  // exit 0
process.exit(7) // exit 7
process.exit(code) // exit with number or status variable

// Older names remain supported for now:
exit(0)
exit(code)
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

Declaration files with the `.d.bsh` suffix provide declarations for strict checking and editor-compatible syntax without emitting shell. Declared functions are called by their declared names; besht does not generate wrappers for them. You can import declaration files explicitly, or place `stdlib.d.bsh` next to the entry file to make its declarations available automatically to bundled compile, split compile, and `--check`. Run `besht init` from a project directory to write the standard declarations to `./stdlib.d.bsh`; it will not overwrite different existing content unless you pass `besht init --force`. Only the entry directory is searched for this automatic stdlib file.

Existing POSIX shell files can be imported explicitly with named imports and an assertion:

```ts
// legacy.sh defines: legacy_log() { printf '%s\n' "$1"; }
import { legacy_log } from "./legacy.sh" assert { type: "shell" };

legacy_log("from besht");
```

`--check` validates imports using the same module resolver as compilation. Shell imports require a `.sh` path and `assert { type: "shell" }`. They are named-only; default shell imports are rejected. Besht does not parse the shell file or infer exports, so imported shell functions are unchecked varargs and return `string` when used in value position. By default, shell imports must stay inside the compiler root. Pass `--opt-allow-external-shell-imports` to permit explicit `.sh` imports outside that root. Bundled output sources the resolved `.sh` file with a guard. Split output copies in-root `.sh` dependencies into the output tree and sources them through `_BESHT_ROOT`; external opt-in shell imports are sourced from their original absolute path. Shell import guards use unique relative shell paths so names like `a-b.sh` and `a_b.sh` cannot collide.

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
        exit(1)
    }

    info("Deploy successful")
}

let env: string = env("1")
let version: string = env("2")

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
        return to_str(bytes / 1073741824) + "GB"
    } else if (bytes > 1048576) {
        return to_str(bytes / 1048576) + "MB"
    }
    return to_str(bytes / 1024) + "KB"
}

let target: string = env("1", ".")
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
besht --check <file.bsh>            Type-check and validate imports only (no output)
besht --check --strict <file.bsh>   Type-check with validation
besht <file.bsh> --opt-no-add-binaries-check  Omit runtime utility self-check
besht <file.bsh> --opt-no-source-map            Omit source comments from compiled output
besht <file.bsh> --opt-resolve-ts-imports       Resolve extensionless imports to .ts only when .bsh is absent
besht <file.bsh> --opt-allow-external-shell-imports  Allow explicit .sh imports outside the compiler root
besht --version                     Show version
besht --help                        Show usage
```

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
