# todo.md — Future ideas for besht

Items here are not scheduled. They were identified during development and saved for later consideration.

---

## Dedicated `.d.bsh` declaration files

Currently `declare` statements must be written inline inside `.bsh` files. A natural next step is supporting dedicated declaration files (`.d.bsh`) that contain only `declare` statements — similar to TypeScript's `.d.ts` files.

These would be auto-discovered (e.g., a `stdlib.d.bsh` next to the entry point), or explicitly imported. The compiler would parse them for editor tooling but emit no code.

Implementation notes:

- The `.d.bsh` extension is already handled by `resolveImportPath` — files with that suffix are loaded but not added to `c.modules`
- A future step is auto-loading a `stdlib.d.bsh` from the same directory as the entry file
- Another step is a `besht init` command that generates a `stdlib.d.bsh` with declarations for all built-in functions

---

## Compile-time type checking (optional, strict mode)

Type annotations are currently ignored entirely. A future `--strict` flag could enable compile-time type checking for users who want it.

Design: the type checker already exists in `internal/checker/checker.go` (gutted to a no-op). It could be re-enabled under `--strict` by restoring the full check logic, while keeping the default behavior annotation-only.

---

## Source maps

Source comments like `# besht:file:line:col` are now emitted at statement boundaries. A future enhancement could add richer source mapping for debugging.

---

## Mixed-type arithmetic: `awk` when a variable _might_ be float

**Status: partial — detects float-producing expressions (Math.* calls, float literals) at compile time.**

Variables produced by `Math.*` calls now trigger awk arithmetic. The gap: intermediate results that are stored in variables and then used in further arithmetic. Example:

```ts
let r = Math.round(2.7)   // r = 3 (integer result, but tracked as float-producing)
let half = r / 2            // → awk (works correctly, produces 1.5)
```

This now works because the type tracker marks `r` as float-producing. The remaining gap: if a float-producing variable is reassigned to an integer value, the tracker doesn't update. For practical besht programs this is acceptable.

---

## Float precision difference between awk and JavaScript

**Status: known cosmetic difference, not a compiler bug.**

`Math.sqrt(2) * Math.sqrt(2)` produces `1.99999` in awk (limited precision) vs `2.0000000000000004` in JavaScript. This is a runtime precision difference, not a semantic error. The comparison tests accept this divergence.

---

## TypeScript class follow-ups

**Status: phases 1 and 2 implemented on `feat/class-support`.** Basic classes now support constructors, instance properties, instance methods, `new`, `this`, static properties, and static methods. Remaining class-related work:

### Inheritance (`extends`)

```ts
class Animal {
  name: string
  constructor(name: string) { this.name = name }
  speak(): string { return this.name + " makes a sound" }
}

class Dog extends Animal {
  breed: string
  constructor(name: string, breed: string) {
    super(name)
    this.breed = breed
  }
  speak(): string { return this.name + " barks" }
}
```

Shell has no inheritance. Possible strategies include flattening parent fields into child constructors or introducing a vtable-like dispatch layer for overridden methods. This is the hardest class phase.

### Getters and setters

```ts
class Circle {
  radius: number
  get area(): number { return Math.PI * this.radius * this.radius }
  set area(a: number) { this.radius = Math.sqrt(a / Math.PI) }
}
```

Getters/setters would compile to methods (`Circle__get_area`, `Circle__set_area`), with property access/assignment rewritten to calls.

### Abstract classes and interfaces

```ts
abstract class Shape {
  abstract area(): number
  describe(): string { return "Area: " + to_str(this.area()) }
}
```

Abstract declarations would be ignored at runtime, with concrete methods compiling like normal class methods.

### Access modifiers

```ts
class BankAccount {
  private balance: number = 0
  public deposit(amount: number) { this.balance += amount }
}
```

`public`, `private`, and `protected` should be annotation-only and ignored by default. They may matter later under `--strict`.

### Decorators

```ts
@log
class MyService {
  @memoize
  expensive(): string { ... }
}
```

Out of scope for besht. Decorators have no shell equivalent and would require a compile-time metaprogramming system.

---

## `fetch()` HTTP client builtin

Support Node.js-style `fetch()` for making HTTP requests from besht scripts.

```ts
let response = await fetch("https://api.example.com/data")
let body: string = response.text()

let res = await fetch("https://api.example.com/submit", {
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify({ name: "Alice" })
})
console.log(to_str(res.status))
let json = res.json()
```

**Compilation strategy:** `fetch()` would compile to `curl` commands under the hood. Since besht has no async runtime, `await fetch()` would be synchronous (curl already is in shell). The `Response` object would compile to a set of shell variables or an object literal holding `status`, `body`, etc.

Implementation considerations:
- `fetch(url)` → `curl -sf "$url"` for simple GETs
- `fetch(url, { method: "POST", body: "..." })` → `curl -sf -X POST -d "$body" "$url"`
- `response.text()` → captured stdout from curl
- `response.status` → captured exit code or `-w "%{http_code}"`
- `response.json()` → parse with a simple awk/sed extractor (full JSON parsing in POSIX sh is hard)
- Headers support via `-H` flags
- `await` keyword is syntactic sugar — besht is synchronous, so `await fetch()` is just `fetch()`
- Need to handle: URL quoting, special characters in body, redirect following (`-L`), timeout (`--max-time`)
- Consider also supporting `response.headers` and `response.ok`

---

## JavaScript built-in API coverage

Expand besht's JS-compatible standard API surface for basic values while preserving POSIX sh output and the current runtime representations.

Recommended phases:

- **Number / Math:** consider additional high-value methods only when they map cleanly to POSIX sh without broad runtime metadata.
- **String:** consider regex-dependent APIs like `match()` or `search()` after lower-risk string methods.
- **Array / list:** add list-compatible APIs such as `toString()`, `Array.isArray()`, and related helpers.
- **Boolean:** decide whether `Boolean(value)` and `Boolean.prototype.toString()` are useful enough given booleans are stored as `1`/`0` and rendered as `true`/`false` in string contexts.
- **Object:** add reliable object shape metadata first, then implement known-shape APIs like `Object.keys()`, `Object.values()`, `Object.entries()`, and `Object.hasOwn()`.
- **Object copying:** evaluate `Object.assign()` and `Object.fromEntries()` after object alias/field metadata is reliable.
- **JSON:** consider limited `JSON.stringify()` for known object/list shapes; defer full `JSON.parse()` unless besht gains a real parser or explicitly depends on an external tool like `jq`.

Implementation notes:

- Static namespaces such as `Object`, `Array`, `Boolean`, and `JSON` need parser/codegen handling similar to the existing `Number.*` special case.
- Module qualification must continue to exempt standard namespaces so they are not rewritten as imported class/function names.
- Callback-heavy APIs (`some`, `every`, `find`, `forEach`) should build on the reusable arrow callback lowering already used by `map`, `filter`, and `reduce`.
- Every added API needs checker, codegen, unit tests, node-eq comparison coverage where practical, and updates to README.md, AGENTS.md, and skills/besht-scripting/SKILL.md.

---

## Arrow functions and callbacks

**Status: partial — expression-bodied callbacks for `list.map()` and `list.filter()` are implemented, and `list.reduce()` supports expression-bodied and block-bodied two-parameter callbacks.**

Continue expanding JavaScript/TypeScript callback syntax so APIs such as `forEach()`, `some()`, `every()`, and `find()` can be implemented cleanly.

Design questions:

- Whether arrow functions are expression-only callbacks or full closure-like values.
- How callback parameters are name-mangled in POSIX sh without `local`.
- Whether callbacks can capture outer variables, and if so how mutations should behave.
- How callback-returning APIs interact with newline-delimited list storage.
- Whether callback support should be limited to compiler-known list methods before becoming a general function-value feature.
