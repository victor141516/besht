## Compact and correct static object console output

### Probe

Small Besht script:

```ts
let user = { id: 1, name: "Ada", active: true }
console.log(user)
console.error(user)
user.name = "Grace"
console.log(user)
```

Current compiled output for known-field objects is a single-quoted `printf` format that contains shell
variable references:

```sh
printf '{ id: "$_obj_user_id", name: "$_obj_user_name", active: $(if [ "$_obj_user_active" = 1 ]; then printf true; else printf false; fi) }\n'
```

At runtime that prints the references literally:

```text
{ id: "$_obj_user_id", name: "$_obj_user_name", active: $(if [ "$_obj_user_active" = 1 ]; then printf true; else printf false; fi) }
```

A direct shell script would use a fixed format string plus shell arguments:

```sh
printf '{\n  id: %s,\n  name: %s,\n  active: %s,\n}\n' \
  "$_obj_user_id" \
  "$_obj_user_name" \
  "$(if [ "$_obj_user_active" = 1 ]; then printf true; else printf false; fi)"
```

### Desired direction

For objects with compiler-known fields, emit one `printf` with a static multi-line format and field
arguments. This matches the documented object display shape and lets the shell expand current object
slots, including values changed after object creation.

Keep the existing validated runtime loop for dynamic-key objects.

### Guardrails

- Preserve field order from object metadata.
- Preserve boolean display as `true`/`false`.
- Reflect later property assignment values.
- Keep stderr redirection for `console.error(object)`.
- Keep dynamic-key object printing on the existing key-validation loop.
- Do not change object enumeration APIs in this branch.

### Verification

- Codegen tests for static object `console.log` / `console.error` using a `printf` argument list.
- Runtime integration test proving actual values print and later assignments are reflected.
- Existing dynamic object metadata pollution test must continue to use the validated loop.
- Update `AGENTS.md`, `README.md`, and `skills/besht-scripting/SKILL.md`.
- `gofmt` changed Go files.
- `git diff --check`
- `make build`
- `make test`
- `go test ./...`
- `bun node-eq/compare $(rg --files -g '*.bsh' node-eq/tests | sort)`
