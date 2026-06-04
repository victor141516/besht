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
- `value.toString()` and `Number.parseInt(value)` for common conversion.
- Native array-style APIs: indexing, `.length`, `.slice()`, `.push()`, `.concat()`, `.includes()`, `.map()`, `.filter()`, `.some()`, `.every()`, `.find()`, `.findIndex()`, `.reduce()`, and statement-position `.forEach()`.
- `Boolean(value)` for primitive boolean coercion.
- `Object.keys(obj)`, scalar-only `Object.values(obj)`, scalar-only `Object.entries(obj)`, and `Object.hasOwn(obj, key)` over compiler-managed object key metadata.
- Besht-only helpers under `Besht.fs.*`, `Besht.strings.*`, `Besht.args.*`, and `Besht.iter.*`.

Removed public APIs:

- `env()`
- `exit()`
- `to_str()`
- `to_int()`
- current non-JS-compatible `String(value)` alias
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
- Expand object APIs only after preserving the current no-runtime-metadata boundary. Near candidates are `Object.assign()` and `Object.fromEntries()`; nested `Object.values()` / `Object.entries()` support requires a broader object/array representation design.
- `JSON.parse()` and `JSON.stringify()` are implemented as opt-in jq-backed slices (`--opt-use-jq`). `JSON.parse()` returns compact `JSONValue` data with jq-backed path access and scalar extraction; `JSON.stringify()` handles `JSONValue`, strings, numbers, booleans, null/undefined, scalar arrays, and scalar-valued compiler-managed objects.
Implementation notes:

- Parser/semantics/codegen recognize `Besht.*`, `process.*`, `Array.*`, `Object.*`, `Boolean`, `Number`, `Math`, and other standard namespaces enough for the implemented slices. Future namespaces such as `JSON` must continue to be exempt from module qualification.
- Static namespaces should keep using handling similar to the existing `Number.*` and `Object.*` paths instead of ad-hoc emitted runtime libraries.
- Callback-heavy APIs should build on the reusable arrow callback lowering and function-value callback paths already used by `map`, `filter`, `some`, `every`, `find`, `findIndex`, `reduce`, and statement-position `forEach`.
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

## Object spread syntax

Add object spread once `Object.assign()` is implemented. Object spread should be based on the same object-copy and merge machinery as `Object.assign()` rather than introducing a separate object representation path.

Candidate syntax:

```ts
let merged = { ...defaults, active: true }
let copy = { ...source }
```

Expected design:

- Lower object spread through the same validated key-copy behavior as `Object.assign()`.
- Preserve left-to-right overwrite semantics and key order: existing keys keep their first position, new keys append when first introduced, and later spreads/properties overwrite values.
- Keep the first slice scalar-safe, matching the initial `Object.assign()` boundary.
- Treat computed or dynamic keys through the same compiler-managed `_objkeys_*` metadata and runtime key validation used by `Object.assign()`.
- Reuse future jq-backed nested-value support if `Object.assign()` grows a richer `--opt-use-jq` mode.

---

## Rest parameter declarations

Add TypeScript-style rest parameter support for declarations and user functions after the first `Object.assign()` slice. This should allow standard-library signatures such as:

```ts
function assign(target: object, ...sources: object[]): object
```

The first `Object.assign()` implementation should avoid this parser/signature expansion and expose only two practical declaration overloads. Future rest parameter work should decide how `...args` maps to shell function arguments, user-authored functions, declaration-only functions, callbacks, and imported/exported function signatures.

---

## TypeScript/Besht behavior divergence table

Add a user-facing table that lists cases where code is syntactically valid TypeScript and syntactically valid Besht, but the behavior differs. The table should use very short code examples and show both outcomes clearly: normal TypeScript/JavaScript behavior and Besht behavior, including whether each side type-checks, compiles, fails at runtime, or fails at Besht compile time.

The table should live in README.md and the most useful distilled guidance should also appear in `skills/besht-scripting/SKILL.md`.

Candidate rows:

| Case | Example | Normal TypeScript / JavaScript | Besht |
| ---- | ------- | ------------------------------ | ----- |
| Type annotations | `let n: number = "x"` | TypeScript reports a type error | Compiles; annotations are ignored |
| `Array.from({ length })` | `Array.from({ length: 3 })` | Creates three `undefined` values | Creates `[0, 1, 2]` |
| Unsupported `Array.from()` forms | `Array.from("abc")` | Creates `["a", "b", "c"]` | Fails at Besht compile time |
| Object reflection boundary | `Object.keys(process.env)` | Returns enumerable environment keys in Node-like runtimes | Fails at Besht compile time; `process.env` is not enumerable |
| Scalar-only object values | `Object.values({ xs: ["a"] })` | Returns the nested array value | Fails at Besht compile time until nested values are supported |
| Static predicates | `Array.isArray(value)` | Runtime shape check | Compiler-known representation check; unknown dynamic values are not runtime-inspected |

When writing the final table, prefer examples that are already covered by compiler tests or node-eq fixtures. Add missing tests if a documented divergence is compiler-enforced and not already guarded.

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

- **Number / Math:** consider additional high-value methods only when they map cleanly to POSIX sh without broad runtime metadata.
- **String:** the old non-JS-compatible global `String(value)` alias has been removed. Besht should eventually have a JS-compatible global `String` object, but only after designing what native-like `String(...)`, static `String.*` APIs, and string wrapper behavior mean under Besht's no-runtime-metadata constraint. Consider regex-dependent APIs like `match()` or `search()` after lower-risk string methods.
- **Array:** Consider related helpers when they map cleanly to current array representations without runtime shape metadata.
- **Boolean:** `Boolean(value)` is implemented as primitive boolean coercion, and boolean `.toString()` already renders `true`/`false`. Future Boolean object wrappers remain out of scope.
- **Object:** `Object.keys()`, narrow scalar-value `Object.values()`, scalar-value `Object.entries()`, and `Object.hasOwn(obj, key)` are implemented over compiler-managed object key metadata. Future richer known-shape APIs should keep the same no-runtime-metadata boundary unless a broader object model is designed.
- **Object copying:** evaluate `Object.assign()` and `Object.fromEntries()` after object alias/field metadata is reliable.
- **JSON:** `JSON.parse()` and `JSON.stringify()` are implemented behind `--opt-use-jq` for the first jq-backed slice: compact `JSONValue` parsing, JSON property/index access, scalar extraction via annotations/assertions, `JSONValue` stringification, scalar values, scalar arrays, and scalar-valued compiler-managed objects. Future work is richer JSON/Besht value interop, such as nested Besht object/array serialization beyond the current scalar-safe boundary.

Implementation notes:

- Static namespaces such as `Boolean` and `JSON` use parser/codegen handling similar to the existing `Number.*` special case. `Array.*`, `Object.keys()`, `Object.values()`, `Object.entries()`, `Object.hasOwn()`, and opt-in JSON slices are implemented.
- Module qualification must continue to exempt standard namespaces so they are not rewritten as imported class/function names.
- Callback-heavy APIs should build on the reusable arrow callback lowering and function-value callback paths already used by `map`, `filter`, `some`, `every`, `find`, `findIndex`, `reduce`, and statement-position `forEach`.
- Every added API needs semantics, codegen, unit tests, node-eq comparison coverage where practical, and updates to README.md, AGENTS.md, and skills/besht-scripting/SKILL.md.
