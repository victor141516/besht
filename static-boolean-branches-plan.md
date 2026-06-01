## Static boolean branches and ternaries

### Probe

Small Besht script:

```ts
if (Boolean("")) {
    console.log("bad")
} else {
    console.log("fallback")
}
if ("hello".startsWith("he")) console.log("starts")
if (1 + 1 == 2) console.log("math")
let label = Boolean("x") ? "yes" : "no"
console.log(label)
let found = "hello".includes("ell") ? "yes" : "no"
console.log(found)
```

Current compiled output still keeps shell control flow for values the compiler already knows:

```sh
if [ 0 = 1 ]; then
    printf '%s\n' 'bad'
else
    printf '%s\n' 'fallback'
fi
if true; then
    printf '%s\n' 'starts'
fi
if { _bst_left=2; _bst_right=2; ... [ "$_bst_left" = "$_bst_right" ]; }; then
    printf '%s\n' 'math'
fi
label=$(if [ 1 = 1 ]; then printf '%s' 'yes'; else printf '%s' 'no'; fi)
found=$(if true; then printf '%s' 'yes'; else printf '%s' 'no'; fi)
```

A direct shell script would naturally be:

```sh
printf '%s\n' 'fallback'
printf '%s\n' 'starts'
printf '%s\n' 'math'
label='yes'
printf '%s\n' "$label"
found='yes'
printf '%s\n' "$found"
```

### Desired direction

Use the generator-aware static boolean evaluator for `if` statements and ternary expressions, not only
for console boolean arguments and method conditions. It already understands richer static values such
as `Boolean(value)`, `Array.isArray(value)`, static `Object.hasOwn()`, static string/list boolean
methods, and nullish-aware booleans.

Also extend static comparison folding so arithmetic operands such as `1 + 1 == 2` can be recognized
as constants before shell condition generation.

### Guardrails

- Keep dynamic conditions on the existing shell condition path.
- Do not evaluate conditions with variable reads that may be assigned inside control flow.
- Preserve `else if` order; only remove branches when all preceding decisions are statically known.
- Keep runtime behavior for non-static string/list searches, file tests, command results, and function calls.

### Verification

- Codegen tests for static `if` branches with `Boolean`, static string methods, static comparisons, and
  static `else if` chains.
- Codegen tests for static ternaries over the same richer boolean expressions.
- Runtime integration test for the probe output.
- `gofmt` changed Go files.
- `git diff --check`
- `make build`
- `make test`
- `go test ./...`
- `bun node-eq/compare $(rg --files -g '*.bsh' node-eq/tests | sort)`
