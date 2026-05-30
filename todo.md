# todo.md — Future ideas for besht

Items here are not scheduled. They were identified during development and saved for later consideration.

---

## Standard-library namespace and JS-style API surface

Reshape Besht's built-in helper APIs into a more TypeScript/JavaScript-like standard-library surface while keeping generated POSIX sh small and optimal.

Requirements to preserve for later design:

- Group Besht-only helpers under `Besht.<group>.*` with camelCase names. For example, condition helpers such as `file_exists(p)` should become something like `Besht.conditions.fileExists(p)`.
- Prefer TypeScript/JavaScript-standard APIs where they already exist, such as `.toString()`, `Number.parseInt()`, `process.env`, and `process.exit()`.
- Prefer native array/list syntax and methods over standalone list helper functions. Old global helpers remain supported for compatibility for now.
- Keep generated POSIX sh as small and optimal as possible: these standard-looking APIs should compile to the same minimal inline tests, index reads, and list operations the compiler can already produce, not to a bulky runtime library.
- Replace `to_str(value)` with TypeScript-style `value.toString()` and replace `to_int(value)` with `Number.parseInt(value)`.
- Replace `env()` and `exit()` with TypeScript/Node-style `process.env` access and `process.exit(code)`.

Remaining future work:

- Broader JS stdlib migration.
- Other Besht namespace groups beyond `Besht.conditions`.
- Eventual removal, warning, or deprecation strategy for old `env()` / `exit()` / condition-helper global names.
- Eventual removal, warning, or deprecation strategy for old global list helpers.
- The larger move away from `list<T>` terminology toward `Array<T>` / `T[]` as the preferred user-facing type.

Open design questions:

- Whether old builtin names remain as migration aliases, warnings, or are removed in one breaking change. For now they remain compatibility aliases; eventual removal is future work.
- Whether `Besht.conditions` is the canonical namespace name, and how to group other non-JS-standard helpers. Other non-JS-standard helpers should also be grouped; analyze the list of helpers and decide the best names for other groups.
- Whether `process.env.NAME` should support default values, and if so what syntax replaces `env("NAME", "default")`. Current direction: use the TypeScript-like `process.env.NAME ?? "default"`, with compiler lowering that preserves explicitly empty environment values.
- Whether this is purely API syntax sugar over existing runtime representations, or whether docs should also move away from `list<T>` terminology.

Implementation notes:

- Parser/checker/codegen need to recognize `Besht.*`, `process.*`, and future standard namespaces such as `Object`, `Array`, `Boolean`, and `JSON` so module qualification does not rewrite them.
- Static namespaces should use handling similar to the existing `Number.*` special case.
- Callback-heavy APIs should build on the reusable arrow callback lowering already used by `map`, `filter`, `some`, `every`, `find`, `findIndex`, and `reduce`; `forEach` remains future work.
- Future migration work should keep README.md, AGENTS.md, `skills/besht-scripting/SKILL.md`, and node-eq fixtures in sync.

---

## Float precision difference between awk and JavaScript

**Status: known cosmetic difference, not a compiler bug.**

`Math.sqrt(2) * Math.sqrt(2)` produces `1.99999` in awk (limited precision) vs `2.0000000000000004` in JavaScript. This is a runtime precision difference, not a semantic error. The comparison tests accept this divergence.

---

## `fetch()` HTTP client builtin

**Status: first synchronous text-only slice implemented and intentionally kept as-is until Besht has promises.** Supported today:

```ts
let body: string = fetch("https://example.com/data.txt").text()

let response = fetch(url) // runs curl once
let again: string = response.text() // reuses stored body
```

This lowers to `curl -sS -- <url>` and returns stdout text. Keep this API frozen at the current text-only surface until a promise/async design exists in Besht. Do not incrementally add Node-style options, richer response fields, `.json()`, or `await fetch()` before promises are designed.

When promises are implemented, revisit a richer Node.js-style `fetch()` design:

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

Post-promises implementation considerations:

- Future `fetch(url, { method: "POST", body: "..." })` design should preserve `--` URL delimiting and safe argument quoting.
- `response.status` could use captured exit code or `-w "%{http_code}"`.
- `response.json()` would need either a limited extractor or a deliberate dependency such as `jq`; full JSON parsing in POSIX sh is hard.
- Headers support via `-H` flags.
- Need to handle URL quoting, special characters in body, redirect following (`-L`), timeout (`--max-time`), `response.headers`, and `response.ok`.

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
