## Static logical value expressions

### Probe

Small Besht script:

```ts
console.log("left" || "fallback")
console.log("" || "fallback")
console.log("left" && "right")
console.log("" && "right")
console.log(0 || 42)
console.log(7 && 42)
console.log(false || true)
console.log(false && true)
```

Current compiled output treats value-position logical expressions as booleans and wraps static
string/number results in a nested command substitution:

```sh
printf '%s\n' "$(if [ $( _l='left'; if [ -n "$_l" ] && [ "$_l" != "0" ]; then printf '%s' "$_l"; else printf '%s' 'fallback'; fi ) = 1 ]; then printf true; else printf false; fi)"
```

That is much harder to read than a direct shell script, and it is also wrong for JS-style value
semantics. The runtime output for the probe is currently:

```text
false
false
false
false
false
false
true
false
```

The direct/static result should be:

```text
left
fallback
right

42
42
true
false
```

### Desired direction

For value-position `||` and `&&`, preserve JavaScript value semantics:

- `a || b` returns `a` when `a` is truthy, otherwise `b`
- `a && b` returns `b` when `a` is truthy, otherwise `a`

When the left side has statically known truthiness, compile directly to the selected side:

```sh
printf '%s\n' 'left'
printf '%s\n' 'fallback'
printf '%s\n' 'right'
printf '%s\n' ''
printf '%s\n' 42
printf '%s\n' 42
printf '%s\n' true
printf '%s\n' false
```

Dynamic value-position logic can keep the existing subshell shape for now, but it should return
the selected value and should not be wrapped as a boolean unless both operands are actually boolean
expressions. Condition-position logicals should continue to compile as shell boolean tests.

### Implementation notes

- Add a static logical-value fold in `genBinaryRHS` before the dynamic `||`/`&&` path.
- Keep `genBinaryCondition` behavior for `if`, `while`, and ternary conditions.
- Adjust `isBooleanExpr` so `&&` and `||` are boolean only when both operands are boolean expressions.
- Adjust logical type inference so static logical expressions infer the selected side's type, and
  boolean operand pairs still infer boolean.
- Add codegen and integration tests that prove compact output and runtime JS-value semantics.
- Update `AGENTS.md`, `README.md`, and `skills/besht-scripting/SKILL.md`.

### Verification

- `gofmt` changed Go files.
- `git diff --check`
- `make build`
- `make test`
- `go test ./...`
- `bun node-eq/compare $(rg --files -g '*.bsh' node-eq/tests | sort)`
