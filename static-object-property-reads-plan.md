# Static Object Property Reads Plan

## Context

Small Besht sample:

```ts
let user = { name: "Ada", city: "Paris" }
console.log(user.name)
console.log(user.city)
```

Current compact output with `--opt-no-add-binaries-check --opt-no-source-map`:

```sh
_obj_user_name='Ada'
_obj_user_city='Paris'
user='user'
_objkeys_user='name city'
printf '%s\n' "$_obj_user_name"
printf '%s\n' "$_obj_user_city"
```

A hand-written shell script would print the known values directly:

```sh
printf '%s\n' 'Ada'
printf '%s\n' 'Paris'
```

This branch takes the next safe step: keep existing object storage for now, but fold direct reads of statically known scalar object properties to constants. A later branch can decide when object storage itself is dead and can be omitted.

## Implementation

- Track statically known scalar property values for object literal bindings.
- Return shell literals for direct `obj.prop` reads when the property is still statically known.
- Store boolean properties as their shell boolean representation (`1`/`0`) so boolean formatting and conditions remain correct.
- Invalidate a property's static value on dot assignment, computed assignment, or unsupported scalar/object values.
- Preserve dynamic fallback for object aliases, object parameters, class/static object maps, computed property reads, and properties assigned inside control flow.

## Verification

- Add codegen coverage for direct static object property reads.
- Add codegen fallback coverage for mutated properties.
- Add runtime integration coverage for string and boolean property reads.
- Update `AGENTS.md`, `README.md`, and `skills/besht-scripting/SKILL.md`.
- Run focused codegen tests and the full gate before committing.
