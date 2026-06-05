# todo.md — Future ideas for besht

Items here are not scheduled. They were identified during development and saved for later consideration.

---

## Never-ending Besht scripting skill improvement loop

**Status: ongoing forever.** `skills/besht-scripting/SKILL.md` is never "done". It should keep improving as the compiler grows, as agents find new awkward translations, and as more idiomatic Besht patterns become possible. A streak of clean no-hints validation runs is useful evidence for one period of work, but it is not a permanent completion condition.

The purpose of this loop is to make the Besht scripting skill strong enough that a fresh agent can write clean, valid, idiomatic Besht by reading the skill file alone. The evaluator must not teach the validation agent the answer through the prompt.

### Loop to repeat

1. Start from the current compiler, README.md, AGENTS.md, node-eq fixtures, and `skills/besht-scripting/SKILL.md`; treat those files as authoritative over memory.
2. Pick one feature family or idiom at a time. Use the compiler and tests to understand the real supported language surface before designing the validation task.
3. Create a small non-mutating shell-style source script for that feature family. Keep it realistic enough to tempt shell-shaped translation, but safe to run repeatedly.
4. Spawn a fresh agent thread. Tell it only to read `skills/besht-scripting/SKILL.md` and translate the script into Besht. Do not mention the expected APIs, shortcuts, pitfalls, or desired implementation shape.
5. Review the generated Besht before compiling it. Look for raw shell hacks, injected shell strings, avoidable `$("sh", "-c", ...)`, manual `cd`, manual env prefixes, copied shell parser loops, command pipelines over static data, missing `.run()`, double-run command objects, or less natural patterns when Besht has native support.
6. If the generated Besht is not clean, update `skills/besht-scripting/SKILL.md` with concise user-facing guidance and examples. Keep compiler internals, node-eq details, and agent workflow details out of the skill file; put those in AGENTS.md and this todo entry instead.
7. Add or update a paired node-eq fixture when the lesson is worth preserving. Pair the original shell-style script with an idiomatic `.bsh` script so future compare runs guard the behavior and style.
8. Compile and compare the generated/fixture Besht against the original shell behavior. Use `./node-eq/compare ...` plus a direct shell-vs-compiled diff when the original shell source is saved beside the fixture.
9. Run the relevant gates before committing. For broader or compiler-touching changes, run `make build`, `make test`, and full `./node-eq/compare $(rg --files -g '*.bsh' node-eq/tests | sort)`.
10. Commit the iteration on a feature branch. Do not merge to master without explicit user approval.
11. Repeat with a different feature family. Even after several excellent outputs in a row, keep this item open because future compiler changes, new APIs, and new agent failure modes will create more opportunities.

### Validation prompt template

Keep prompts minimal and no-hints:

````text
Do not edit any files. Read `skills/besht-scripting/SKILL.md`, then translate the shell script below into a Besht `.bsh` script. Return the Besht code and a short note about assumptions.

```sh
# shell script here
```
````

Do not add hints like "use `$()`", "use `.pipe()`", "use `.workdir()`", "use `Besht.args`", "avoid awk", or "use Object.entries". Those facts must come from the skill file, otherwise the run validates the prompt rather than the skill.

### Existing guardrail fixtures

- `node-eq/tests/commands/skill_pipeline_idioms.*`: command expressions, pipes, redirects, `.env()`, `.workdir()`, raw patterns, and exit-code-gated command chains.
- `node-eq/tests/language/callbacks/skill_native_data_idioms.*`: translating static text/number pipelines into native arrays, callbacks, string transforms, joins, reductions, and indexed `forEach`.
- `node-eq/tests/commands/skill_args_env_predicates.*`: translating shell argument parsers, environment defaults, file predicates, and string predicates into `Besht.args`, `process.env`, `Besht.fs`, and `Besht.strings`.
- `node-eq/tests/language/objects/skill_object_data_idioms.*`: translating static delimiter-separated records into objects, callbacks, dynamic object property reads, `Object.hasOwn()`, and `JSON.stringify()`.

### Feature families to keep probing

- Commands: pipelines, redirects, stderr/stdout capture, `.exitCode()`, `.clone()`, command spreading, raw patterns, and avoiding shell injection.
- Script interface: args parsing, `--`, empty arguments, `process.env.NAME ?? fallback`, `Besht.fs.*`, `Besht.strings.*`, and `process.exit()`.
- In-memory data: array methods, callbacks, reduction, `forEach`, static transforms, `Set<T>`, object literals, dynamic keys, object helpers, optional chaining, and JSON output.
- Control flow: `if`/`else`, `switch`, loops, `break`/`continue`, `try`/`catch`, command failure handling, and status values.
- Modules and declarations: imports, `.d.bsh`, shell imports with assertions, split output, exported values, default exports, and TypeScript import fallback flags.
- Classes: constructors, instance/static properties, methods, getters/setters, `this`, and unsupported inheritance/modifier boundaries.
- New compiler work: every new language feature, flag, optimization, or pitfall should get at least one no-hints skill validation slice if it affects how users write Besht.

### What "good" means

Good generated Besht should compile, match the source script behavior, and look like natural Besht rather than transliterated shell. It should prefer compiler-supported language constructs over shell-shaped workarounds, but it should still use external commands when the task genuinely depends on external tools or external data.

This process intentionally has no final checkbox. The practical stopping point for a work session is a documented, tested, committed improvement plus a note about what feature family should be probed next.

---

## Standard-library namespace and JS-style API surface

**Status: breaking cleanup implemented.** The user-facing standard-library surface is now the canonical TypeScript/JavaScript-shaped API plus Besht-specific helpers under `Besht.*`.

Current canonical surface:

- `process.env.NAME ?? fallback` for environment variables.
- `process.exit(code)` for process exit.
- `value.toString()`, `value.valueOf()`, `String(value)`, `string.localeCompare(other)`, `Number.parseInt(value)`, global `parseInt(value)`, and global `parseFloat(value)` for common conversion and scalar string ordering.
- Native array-style APIs: indexing, `.at()`, `.length`, `.slice()`, `.push()`, `.concat()`, `.fill()`, default lexical `.sort()`, `.toReversed()`, `.toSorted()`, `.toSpliced()`, `.includes()`, `.map()`, `.flatMap()`, `.filter()`, `.some()`, `.every()`, `.find()`, `.findIndex()`, `.findLast()`, `.findLastIndex()`, `.reduce()`, `.reduceRight()`, and statement-position `.forEach()`.
- `Boolean(value)` for primitive boolean coercion.
- `Object.keys(obj)`, scalar-only `Object.values(obj)`, scalar-only `Object.entries(obj)`, `Object.hasOwn(obj, key)`, scalar-safe `Object.assign(target, ...sources)`, scalar-safe `Object.fromEntries(entries)`, and scalar-safe object spread over compiler-managed object key metadata.
- Besht-only helpers under `Besht.fs.*`, `Besht.strings.*`, `Besht.args.*`, and `Besht.iter.*`.

Removed public APIs:

- `env()`
- `exit()`
- `to_str()`
- `to_int()`
- old non-JS-compatible `String(value)` alias behavior
- global list helpers `len`, `head`, `tail`, `append`, `contains`, `concat`
- bare `range()`
- global condition helpers `file_exists`, `is_dir`, `is_readable`, `is_writable`, `is_executable`, `is_empty`, `is_set`
- `args.*`
- `Besht.conditions.*`

Requirements to preserve for future standard-library work:

- Prefer TypeScript/JavaScript-standard APIs where they already exist.
- Group Besht-only helpers under `Besht.<group>.*` with camelCase names.
- Keep generated POSIX sh as small and optimal as possible: standard-looking APIs should compile to minimal inline tests, index reads, and array operations rather than broad runtime metadata.
- Object helpers remain backed by compiler-managed object key metadata, not broad runtime shape metadata.
- Dynamic object metadata keys must be validated before generated shell uses `eval`; invalid dynamic keys fail closed instead of becoming shell injection vectors.
- `Object.values()` and `Object.entries()` should keep rejecting statically known array/object/set/command/fetch values until the representation can preserve nested values safely.

Remaining future work:

- Broader JS stdlib migration for APIs that map cleanly to POSIX sh without broad runtime metadata.
- Expand object APIs only after preserving the current no-runtime-metadata boundary. The next near candidate is nested `Object.values()` / `Object.entries()` support, which requires a broader object/array representation design.
- `JSON.parse()` and `JSON.stringify()` are implemented as opt-in jq-backed slices (`--opt-use-jq`). `JSON.parse()` returns compact `JSONValue` data with jq-backed path access and scalar extraction; `JSON.stringify()` handles `JSONValue`, strings, numbers, booleans, null/undefined, scalar arrays, and scalar-valued compiler-managed objects.
Implementation notes:

- Parser/semantics/codegen recognize `Besht.*`, `process.*`, `Array.*`, `Object.*`, `Boolean`, `Number`, `Math`, and other standard namespaces enough for the implemented slices. Future namespaces such as `JSON` must continue to be exempt from module qualification.
- Static namespaces should keep using handling similar to the existing `Number.*` and `Object.*` paths instead of ad-hoc emitted runtime libraries.
- Callback-heavy APIs should build on the reusable arrow callback lowering and function-value callback paths already used by `map`, `flatMap`, `filter`, `some`, `every`, `find`, `findIndex`, `findLast`, `findLastIndex`, `reduce`, `reduceRight`, and statement-position `forEach`.
- Any API that introduces dynamic object keys or slots must validate names before generated shell uses `eval`; tests should cover polluted `_objkeys_*` metadata and unsafe computed keys.
- Future migration work should keep README.md, AGENTS.md, `skills/besht-scripting/SKILL.md`, stdlib declarations, semantics/codegen tests, and node-eq fixtures in sync.

---

## jq-backed alternate lowering mode

`--opt-use-jq` can mean more than enabling `JSON.parse()` and `JSON.stringify()`: when jq-backed codegen produces simpler or safer shell than the standard POSIX-only lowering, the compiler may choose a jq-backed implementation for supported APIs.

This should be considered for object handling, array handling, JSON interop, and future JS-style standard-library APIs. The same Besht/TypeScript-style source should be able to compile either to normal POSIX-oriented output or to jq-assisted output when the user passes `--opt-use-jq`.

Rules:

- Use jq only when `--opt-use-jq` is enabled.
- If a feature requires jq and the flag is absent, fail at compile time with a clear error mentioning `--opt-use-jq`.
- If the non-jq implementation is simpler or already optimal, keep using it even when jq is enabled.
- Prefer jq-backed lowering when it materially simplifies generated shell, improves correctness for nested data, or avoids fragile POSIX string/object encoding.
- Keep docs, stdlib declarations, semantics/codegen tests, and node-eq fixtures clear about which APIs require jq and which merely have an optional jq-backed lowering.

---

## Rest parameter declarations

Add TypeScript-style rest parameter support for declarations and user functions after the first `Object.assign()` slice. This should allow standard-library signatures such as:

```ts
function assign(target: object, ...sources: object[]): object
```

`Object.assign()` currently supports multiple sources in compiler-recognized calls, but the generated standard-library declarations expose only two practical overloads. Future rest parameter work should decide how `...args` maps to shell function arguments, user-authored functions, declaration-only functions, callbacks, and imported/exported function signatures.

---

## Float precision difference between awk and JavaScript

**Status: known cosmetic difference, not a compiler bug.**

`Math.sqrt(2) * Math.sqrt(2)` produces `1.99999` in awk (limited precision) vs `2.0000000000000004` in JavaScript. This is a runtime precision difference, not a semantic error. The comparison tests accept this divergence.

---

## `fetch()` HTTP client builtin

**Status: first synchronous text-only slice implemented and intentionally kept as-is until Besht has promises.** Supported today:

```ts
let body: string = fetch("https://example.com/data.txt").text();

let response = fetch(url); // runs curl once
let again: string = response.text(); // reuses stored body
```

This lowers to `curl -sS -- <url>` and returns stdout text. Keep this API frozen at the current text-only surface until a promise/async design exists in Besht. Do not incrementally add Node-style options, richer response fields, `.json()`, or `await fetch()` before promises are designed.

When promises are implemented, revisit a richer Node.js-style `fetch()` design:

```ts
let response = await fetch("https://api.example.com/data");
let body: string = response.text();

let res = await fetch("https://api.example.com/submit", {
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify({ name: "Alice" }),
});
console.log(res.status.toString());
let json = res.json();
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

- **Number / Math:** `Math.E`, `Math.LN2`, `Math.LN10`, `Math.LOG2E`, `Math.LOG10E`, `Math.PI`, `Math.SQRT1_2`, and `Math.SQRT2` are implemented as numeric constants. `parseInt()` and `parseFloat()` are implemented as global aliases for the existing `Number.parseInt()` / `Number.parseFloat()` lowering. Consider additional high-value methods only when they map cleanly to POSIX sh without broad runtime metadata.
- **String:** JS-compatible primitive `String(value)` is implemented for current Besht scalar representations: primitives, null/undefined, scalar arrays, compiler-managed objects, and Sets. Primitive `valueOf()` is implemented for strings, numbers, booleans, and status values as identity. `String.prototype.localeCompare()` is implemented as a compact bytewise C-locale comparison returning `-1`, `0`, or `1`. `trimLeft()` / `trimRight()` are implemented as aliases for `trimStart()` / `trimEnd()`, and legacy `substr(start, length?)` is implemented with JavaScript-style negative start and optional length handling. Besht deliberately does not add `new String(...)`, string wrapper objects, `String.raw`, other static `String.*` APIs, direct `String(JSONValue)` conversion, or full ICU locale collation. Consider regex-dependent APIs like `match()` or `search()` only after a regex representation is designed.
- **Array:** `Array.prototype.at()` is implemented for positive and negative indexes, `Array.from(string)` is implemented for character arrays, `Array.prototype.fill()` is implemented for scalar arrays with JavaScript-style start/end bounds, default lexical `Array.prototype.sort()` is implemented without callback support, `Array.prototype.toReversed()` is implemented as a non-mutating reversed copy, `Array.prototype.toSorted()` is implemented as a non-mutating default lexical sorted copy, `Array.prototype.toSpliced()` is implemented as a non-mutating copied splice with JavaScript-style start/delete bounds, scalar-array `findLast()` / `findLastIndex()` callbacks are implemented with reverse traversal and JavaScript-style no-match results, `Array.prototype.flatMap()` is implemented as one-level flattening for callback-returned scalar arrays, and `Array.prototype.reduceRight()` is implemented with the same initial-value-only accumulator contract as `reduce()`. General `flat()` remains deferred until the nested-array representation can preserve empty inner arrays and deeper nesting cleanly.
- **Boolean:** `Boolean(value)` is implemented as primitive boolean coercion, and boolean `.toString()` already renders `true`/`false`. Future Boolean object wrappers remain out of scope.
- **Object:** `Object.keys()`, narrow scalar-value `Object.values()`, scalar-value `Object.entries()`, `Object.hasOwn(obj, key)`, scalar primitive `Object.is(a, b)`, scalar-safe `Object.assign(target, ...sources)`, scalar-safe `Object.fromEntries(entries)`, and scalar-safe object spread are implemented over compiler-managed object key metadata. Future richer known-shape APIs should keep the same no-runtime-metadata boundary unless a broader object model is designed.
- **Object copying:** `Object.assign()`, `Object.fromEntries()`, and object spread are implemented for scalar-safe compiler-managed objects. Future work should evaluate richer nested-value support.
- **JSON:** `JSON.parse()` and `JSON.stringify()` are implemented behind `--opt-use-jq` for the first jq-backed slice: compact `JSONValue` parsing, JSON property/index access, scalar extraction via annotations/assertions, `JSONValue` stringification, scalar values, scalar arrays, and scalar-valued compiler-managed objects. Future work is richer JSON/Besht value interop, such as nested Besht object/array serialization beyond the current scalar-safe boundary.

Implementation notes:

- Static namespaces such as `Boolean`, `String`, and `JSON` use parser/codegen handling similar to the existing `Number.*` special case. `Array.*`, `Object.keys()`, `Object.values()`, `Object.entries()`, `Object.hasOwn()`, `Object.is()`, `Object.assign()`, `Object.fromEntries()`, object spread, and opt-in JSON slices are implemented.
- Module qualification must continue to exempt standard namespaces so they are not rewritten as imported class/function names.
- Callback-heavy APIs should build on the reusable arrow callback lowering and function-value callback paths already used by `map`, `flatMap`, `filter`, `some`, `every`, `find`, `findIndex`, `findLast`, `findLastIndex`, `reduce`, `reduceRight`, and statement-position `forEach`.
- Every added API needs semantics, codegen, unit tests, node-eq comparison coverage where practical, and updates to README.md, AGENTS.md, and skills/besht-scripting/SKILL.md.

Priority order from the June 2026 JS API coverage pass:

1. `Array.prototype.at()` for positive and negative indexes. Implemented.
2. `Array.from(string)` for character arrays. Implemented.
3. `Object.fromEntries()` over scalar-safe `[key, value]` entries. Implemented.
4. Default lexical `Array.prototype.sort()` without callback support. Implemented.
5. Revisit a JavaScript-compatible `String(value)` design. Implemented for primitive stringification without wrappers or static `String.*` APIs.
6. Common `Math` constants (`Math.PI`, `Math.E`, logarithm constants, and square-root constants). Implemented.
7. `Array.prototype.findLast()` and `Array.prototype.findLastIndex()` for reverse scalar-array predicate searches. Implemented.
8. `Array.prototype.fill(value, start?, end?)` for bounded scalar-array replacement. Implemented.
9. `Array.prototype.flatMap(callback)` for one-level flattening of callback-returned scalar arrays. Implemented.
10. Global `parseInt()` and `parseFloat()` aliases for common JavaScript numeric parsing. Implemented.
11. `Array.prototype.reduceRight(callback, initialValue)` for right-to-left scalar, array, and object accumulation. Implemented.
12. `Array.prototype.toReversed()` for non-mutating reversed array copies. Implemented.
13. `Array.prototype.toSorted()` for non-mutating default lexical sorted array copies. Implemented.
14. `Array.prototype.toSpliced(start, deleteCount?, ...items)` for non-mutating copied splice operations. Implemented.
15. `String.prototype.localeCompare(other)` for compact bytewise string ordering. Implemented.
16. `String.prototype.trimLeft()` / `trimRight()` aliases for `trimStart()` / `trimEnd()`. Implemented.
17. Primitive `valueOf()` for strings, numbers, booleans, and status values. Implemented.
18. Legacy `String.prototype.substr(start, length?)` for start/length substring extraction. Implemented.
19. `Object.is(a, b)` for scalar primitive SameValue-style comparisons over Besht's representable primitive values. Implemented.
