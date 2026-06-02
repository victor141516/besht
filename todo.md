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
- `node-eq/tests/language/callbacks/skill_native_data_idioms.*`: translating static text/number pipelines into native lists, callbacks, string transforms, joins, reductions, and indexed `forEach`.
- `node-eq/tests/commands/skill_args_env_predicates.*`: translating shell argument parsers, environment defaults, file predicates, and string predicates into `Besht.args`, `process.env`, `Besht.fs`, and `Besht.strings`.
- `node-eq/tests/language/objects/skill_object_data_idioms.*`: translating static delimiter-separated records into objects, callbacks, dynamic object property reads, `Object.hasOwn()`, and `JSON.stringify()`.

### Feature families to keep probing

- Commands: pipelines, redirects, stderr/stdout capture, `.exitCode()`, `.clone()`, command spreading, raw patterns, and avoiding shell injection.
- Script interface: args parsing, `--`, empty arguments, `process.env.NAME ?? fallback`, `Besht.fs.*`, `Besht.strings.*`, and `process.exit()`.
- In-memory data: list methods, callbacks, reduction, `forEach`, static transforms, `Set<T>`, object literals, dynamic keys, object helpers, optional chaining, and JSON output.
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
- Native list/array-style APIs: indexing, `.length`, `.slice()`, `.push()`, `.concat()`, `.includes()`, `.map()`, `.filter()`, `.some()`, `.every()`, `.find()`, `.findIndex()`, `.reduce()`, and statement-position `.forEach()`.
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
- Keep generated POSIX sh as small and optimal as possible: standard-looking APIs should compile to minimal inline tests, index reads, and list operations rather than broad runtime metadata.
- Object helpers remain backed by compiler-managed object key metadata, not broad runtime shape metadata.
- Dynamic object metadata keys must be validated before generated shell uses `eval`; invalid dynamic keys fail closed instead of becoming shell injection vectors.
- `Object.values()` and `Object.entries()` should keep rejecting statically known list/object/set/command/fetch values until the representation can preserve nested values safely.

Remaining future work:

- Broader JS stdlib migration for APIs that map cleanly to POSIX sh without broad runtime metadata.
- The larger move away from `list<T>` terminology toward `Array<T>` / `T[]` as the preferred user-facing type in docs, examples, declarations, and diagnostics.
- Expand object APIs only after preserving the current no-runtime-metadata boundary. Near candidates are `Object.assign()` and `Object.fromEntries()`; nested `Object.values()` / `Object.entries()` support requires a broader object/list representation design.
- `JSON.stringify()` is implemented as an opt-in jq-backed slice (`--opt-use-jq`) for strings, numbers, booleans, scalar lists, and scalar-valued compiler-managed objects. `JSON.parse()` remains deferred unless Besht gains a parser or a broader jq-backed JSON design.
- General callback values and closures remain future work. Current callback lowering is method-specific and compiler-known, including statement-position `forEach()`.

Implementation notes:

- Parser/checker/codegen recognize `Besht.*`, `process.*`, `Array.*`, `Object.*`, `Boolean`, `Number`, `Math`, and other standard namespaces enough for the implemented slices. Future namespaces such as `JSON` must continue to be exempt from module qualification.
- Static namespaces should keep using handling similar to the existing `Number.*` and `Object.*` paths instead of ad-hoc emitted runtime libraries.
- Callback-heavy APIs should build on the reusable arrow callback lowering already used by `map`, `filter`, `some`, `every`, `find`, `findIndex`, `reduce`, and statement-position `forEach`.
- Any API that introduces dynamic object keys or slots must validate names before generated shell uses `eval`; tests should cover polluted `_objkeys_*` metadata and unsafe computed keys.
- Future migration work should keep README.md, AGENTS.md, `skills/besht-scripting/SKILL.md`, stdlib declarations, checker/codegen tests, and node-eq fixtures in sync.

---

## Optional jq-backed codegen and JSON support

Add one opt-in compiler flag, `--opt-use-jq`, that permits generated output to depend on `jq`. Without this flag, `JSON.parse()` and `JSON.stringify()` should produce a compile-time semantic error explaining that they require `--opt-use-jq`. With the flag enabled, codegen may also choose `jq` for other operations when it materially simplifies generated shell, but the existing POSIX/static-folding paths should remain the default when they are smaller or clearer.

Runtime dependency checks should follow the existing external-binary check model:

- Passing `--opt-use-jq` alone must not force a check.
- If generated shell actually calls `jq`, emit a runtime check that `jq` is installed and working.
- `--opt-no-add-binaries-check` suppresses the `jq` check too.

Introduce an internal JSON representation hint such as `TypeJSON`, exposed to users as a `JSONValue` annotation. This remains a compiler representation hint, not compile-time type checking.

Suggested standard declarations:

```ts
type JSONValue = string

declare namespace JSON {
    function parse(value: string): JSONValue
    function stringify(value): string
}
```

`JSON.parse(text)` should:

- evaluate `text` safely;
- validate and canonicalize immediately with `jq -c .`;
- store compact valid JSON as a `JSONValue`;
- on invalid JSON, print a Besht-specific runtime error such as `[besht] JSON.parse() failed` and exit nonzero.

JSON property/index access should be as close to native JavaScript behavior as practical:

```ts
let data = JSON.parse(body)
let user = data.user          // JSONValue
let first = data.items[0]     // JSONValue
```

- Property/index access on a `JSONValue` returns another `JSONValue`.
- Missing final properties behave like JavaScript `undefined`, so `data.user.name ?? "Anonymous"` can use the fallback when `user` exists but `name` is absent.
- JSON `null` behaves as Besht nullish for `??`.
- Accessing through a missing or null intermediate value should fail like JavaScript unless optional chaining is used. For example, `data.missing.name` should fail, while `data.missing?.name ?? "fallback"` should use the fallback.

JSON scalar extraction should be driven by annotations or `as` assertions only when the source expression is JSON-backed:

```ts
let name: string = data.user.name
let count = data.count as number
let ok: boolean = data.enabled
```

Supported first-slice extraction targets:

- `string`
- `number`
- `boolean`
- `JSONValue`

Extraction rules:

- Missing final property and JSON `null` produce Besht nullish so `??` can handle them.
- Non-null JSON values with the wrong asserted type fail loudly at runtime with a Besht-specific error.
- Normal non-JSON `expr as Type` and `let x: Type = expr` remain compile-time-erased annotations. Do not introduce compile-time type mismatch errors.

`JSON.stringify(value)` should always require `--opt-use-jq`, even for static values the compiler could theoretically fold without jq. First-slice accepted inputs:

- `JSONValue`, canonicalized or passed through compactly with `jq -c .`;
- strings, numbers, booleans, null/undefined;
- scalar lists;
- scalar object literals or named objects that the compiler can serialize safely.

Reject commands, fetch responses, sets, and unsupported nested Besht values until a broader representation is designed. Use `jq` where it helps with dynamic escaping or assembly, but do not require every compiler-built JSON literal to be piped back through `jq` when direct emission is safe.

Implementation plan:

1. Add `UseJQ bool` to `codegen.Options`, wire `--opt-use-jq` through `cmd/besht/main.go`, usage text, CLI tests, README.md, AGENTS.md, and `skills/besht-scripting/SKILL.md`.
2. Add `TypeJSON` / `JSONValue` representation support in AST type parsing, checker inference, module export/import type inference, and codegen `varTypeMap`.
3. Teach parser/checker/codegen that `JSON.parse` and `JSON.stringify` are compiler-known builtins, and exempt `JSON` from module/class qualification like `Object`, `Array`, `Number`, and `Math`.
4. Add semantic validation that rejects JSON builtins without `UseJQ`, while preserving the no compile-time type-checking policy.
5. Track jq usage in codegen and extend runtime check generation so a jq check appears only when emitted shell calls jq and `NoCheck` is false.
6. Implement `JSON.parse()` codegen with safe input handling, `jq -c .`, and Besht-specific parse failure output.
7. Implement JSON path lowering for property and index access on `JSONValue`, including optional chaining and JS-like missing/intermediate behavior.
8. Implement JSON scalar extraction/assertion for `string`, `number`, `boolean`, and `JSONValue` via both `as Type` and let/const annotations.
9. Implement first-slice `JSON.stringify()` for safe scalar/list/object inputs and `JSONValue`.
10. Add checker, codegen, integration, split-mode/runtime-check, and node-eq coverage where practical.

---

## Checker package cleanup after removing type checking

**Status: type checking has been removed; package naming cleanup remains future work.** `internal/checker` now owns semantic validation and representation/surface checks: function/declaration signature collection, name and const validation, builtin/method arity, fetch surface validation, object surface validation, statement-only `forEach()` validation, and similar checks for code the compiler cannot emit safely.

Future refactor: rename or split `internal/checker` into a semantic validation package such as `internal/semantics`, so the package name no longer implies user-facing type checking.

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
console.log(res.status.toString())
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
- **String:** the old non-JS-compatible global `String(value)` alias has been removed. Besht should eventually have a JS-compatible global `String` object, but only after designing what native-like `String(...)`, static `String.*` APIs, and string wrapper behavior mean under Besht's no-runtime-metadata constraint. Consider regex-dependent APIs like `match()` or `search()` after lower-risk string methods.
- **Array / list:** Consider related helpers when they map cleanly to current list representations without runtime shape metadata.
- **Boolean:** `Boolean(value)` is implemented as primitive boolean coercion, and boolean `.toString()` already renders `true`/`false`. Future Boolean object wrappers remain out of scope.
- **Object:** `Object.keys()`, narrow scalar-value `Object.values()`, scalar-value `Object.entries()`, and `Object.hasOwn(obj, key)` are implemented over compiler-managed object key metadata. Future richer known-shape APIs should keep the same no-runtime-metadata boundary unless a broader object model is designed.
- **Object copying:** evaluate `Object.assign()` and `Object.fromEntries()` after object alias/field metadata is reliable.
- **JSON:** `JSON.stringify()` is implemented behind `--opt-use-jq` for scalar values, scalar lists, and scalar-valued compiler-managed objects. Future work is `JSON.parse()` or richer JSON extraction, which should remain deferred until there is a broader parser/jq design.

Implementation notes:

- Static namespaces such as `Boolean` and `JSON` use parser/codegen handling similar to the existing `Number.*` special case. `Array.*`, `Object.keys()`, `Object.values()`, `Object.entries()`, `Object.hasOwn()`, and opt-in `JSON.stringify()` slices are implemented.
- Module qualification must continue to exempt standard namespaces so they are not rewritten as imported class/function names.
- Callback-heavy APIs should build on the reusable arrow callback lowering already used by `map`, `filter`, `some`, `every`, `find`, `findIndex`, `reduce`, and statement-position `forEach`.
- Every added API needs checker, codegen, unit tests, node-eq comparison coverage where practical, and updates to README.md, AGENTS.md, and skills/besht-scripting/SKILL.md.

---

## Arrow functions and callbacks

**Status: partial — expression-bodied callbacks for `list.map()`, `list.filter()`, `list.some()`, `list.every()`, `list.find()`, and `list.findIndex()` are implemented, `list.reduce()` supports expression-bodied and block-bodied two-parameter callbacks, and `list.forEach()` supports statement-position side-effect callbacks.**

Continue expanding JavaScript/TypeScript callback syntax so general callback values can be implemented cleanly.

Design questions:

- Whether arrow functions are expression-only callbacks or full closure-like values.
- How callback parameters are name-mangled in POSIX sh without `local`.
- Whether callbacks can capture outer variables, and if so how mutations should behave.
- How callback-returning APIs interact with newline-delimited list storage.
- Whether callback support should be limited to compiler-known list methods before becoming a general function-value feature.
