# todo.md — Future ideas for besht

Items here are not scheduled. They were identified during development and saved for later consideration.

---

## Source maps

Future enhancement: add richer source mapping for debugging beyond the current inline `# besht:file:line:col` source comments.

---

## Standard-library namespace and TypeScript-compatible builtins

Move Besht-specific built-in functions toward a fake globally available standard-library object named `Besht`, grouped by functionality, while preferring TypeScript/JavaScript-standard APIs where they already exist.

Requirements to preserve for later design:

- Group Besht-only helpers under `Besht.<group>.*` with camelCase names. For example, condition helpers such as `file_exists(p)` should become something like `Besht.conditions.fileExists(p)`.
- Prefer native list syntax and methods over new global list functions. Old global helpers remain supported for compatibility for now.
- Keep generated POSIX sh as small and optimal as possible: these standard-looking APIs should compile to the same minimal inline tests, index reads, and list operations the compiler can already produce, not to a bulky runtime library.
- Replace `to_str(value)` with TypeScript-style `value.toString()` and replace `to_int(value)` with `Number.parseInt(value)`.
- Remaining: broader JS stdlib migration, other Besht namespace groups, eventual removal of old `env()` / `exit()` / condition-helper global names, eventual removal of old global list helpers, and the larger move away from `list<T>` terminology. The old names remain supported for now.

Potential grouping sketch:

- Standard TypeScript/JavaScript namespaces and methods for conversions and process access: `.toString()`, `Number.parseInt()`, `process.env`, and `process.exit()`.

Implementation notes:

- Parser/checker/codegen will need to recognize `Besht.*` and `process.*` as standard namespaces so module qualification does not rewrite them.
- Future migration work should keep README.md, AGENTS.md, `skills/besht-scripting/SKILL.md`, and node-eq fixtures in sync.

---

## Float precision difference between awk and JavaScript

**Status: known cosmetic difference, not a compiler bug.**

`Math.sqrt(2) * Math.sqrt(2)` produces `1.99999` in awk (limited precision) vs `2.0000000000000004` in JavaScript. This is a runtime precision difference, not a semantic error. The comparison tests accept this divergence.


---

## TypeScript class follow-ups

**Status: phases 1 and 2 implemented on `feat/class-support`.** Basic classes now support constructors, instance properties, instance methods, `new`, `this`, static properties, static methods, and getters/setters. Remaining class-related work:

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

### Abstract classes and interfaces

```ts
abstract class Shape {
  abstract area(): number
  describe(): string { return "Area: " + to_str(this.area()) }
}
```

Abstract declarations would be ignored at runtime, with concrete methods compiling like normal class methods.

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
- **Array / list:** Consider related helpers when they map cleanly to current list representations without runtime shape metadata.
- **Boolean:** decide whether `Boolean(value)` and `Boolean.prototype.toString()` are useful enough given booleans are stored as `1`/`0` and rendered as `true`/`false` in string contexts.
- **Object:** add reliable object shape metadata first, then implement known-shape APIs like `Object.keys()`, `Object.values()`, `Object.entries()`, and `Object.hasOwn()`.
- **Object copying:** evaluate `Object.assign()` and `Object.fromEntries()` after object alias/field metadata is reliable.
- **JSON:** consider limited `JSON.stringify()` for known object/list shapes; defer full `JSON.parse()` unless besht gains a real parser or explicitly depends on an external tool like `jq`.

Implementation notes:

- Static namespaces such as `Object`, `Array`, `Boolean`, and `JSON` need parser/codegen handling similar to the existing `Number.*` special case.
- Module qualification must continue to exempt standard namespaces so they are not rewritten as imported class/function names.
- Callback-heavy APIs should build on the reusable arrow callback lowering already used by `map`, `filter`, `some`, `every`, `find`, `findIndex`, and `reduce`; `forEach` remains future work.
- Every added API needs checker, codegen, unit tests, node-eq comparison coverage where practical, and updates to README.md, AGENTS.md, and skills/besht-scripting/SKILL.md.

---

## Arrow functions and callbacks

**Status: partial — expression-bodied callbacks for `list.map()`, `list.filter()`, `list.some()`, `list.every()`, `list.find()`, and `list.findIndex()` are implemented, and `list.reduce()` supports expression-bodied and block-bodied two-parameter callbacks.**

Continue expanding JavaScript/TypeScript callback syntax so APIs such as `forEach()` and general callback values can be implemented cleanly.

Design questions:

- Whether arrow functions are expression-only callbacks or full closure-like values.
- How callback parameters are name-mangled in POSIX sh without `local`.
- Whether callbacks can capture outer variables, and if so how mutations should behave.
- How callback-returning APIs interact with newline-delimited list storage.
- Whether callback support should be limited to compiler-known list methods before becoming a general function-value feature.

---

## JS-style standard library API surface

Reshape Besht's built-in helper APIs into a more TypeScript/JavaScript-like standard library surface while keeping generated POSIX sh as small and optimal as possible.

Proposed direction:

- Expose grouped standard-library helpers through global objects instead of top-level snake_case builtins. For Besht-specific helpers, use a global `Besht` object with functionality-based groups and camelCase names, for example `Besht.conditions.fileExists(path)` instead of `file_exists(path)`.
- Prefer existing array/list syntax and methods over standalone list helper functions. The compiler should keep lowering these forms to small shell output. Old global list helpers remain supported for compatibility for now.
- Replace `to_str(value)` with TypeScript-style `value.toString()` and replace `to_int(value)` with `Number.parseInt(value)`.
- Replace `env()` and `exit()` with TypeScript/Node-style `process.env` access and `process.exit(code)`.

Open design questions:

- Whether old builtin names remain as migration aliases, warnings, or are removed in one breaking change. -> For now they remain compatibility aliases; eventual removal is future work.
- Whether `Besht.conditions` is the canonical namespace name, and how to group other non-JS-standard helpers. -> other non-JS-standard helpers should also be grouped. Analyse the list of helpers and decide the best names for other groups.
- Whether `process.env.NAME` should support default values, and if so what syntax replaces `env("NAME", "default")`. -> Default values will use the more typescript-like `process.env.NAME ?? "default"`. The compiler should be smart and use the proper shell syntax when the default value is known at compile time.
- Whether this is purely API syntax sugar over existing runtime representations, or if docs should also move from `list<T>` terminology toward `Array<T>`/`T[]` as the preferred user-facing type. -> Native list helper replacements are available and documented; the larger `list<T>` terminology removal remains future work.
