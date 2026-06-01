# Boolean Property Conditions Plan

## Context

Object property values already compile to direct shell variables, but boolean object properties used as conditions still go through generic truthiness logic.

Input:

```ts
let user = { name: "Ada", city: "Paris", active: true }
console.log(user.name)
console.log(user.city)
if (user.active) console.log("active")
```

Current compact output with `--opt-no-add-binaries-check --opt-no-source-map`:

```sh
_obj_user_name='Ada'
_obj_user_city='Paris'
_obj_user_active=1
user='user'
_objkeys_user='name city active'
printf '%s\n' "$_obj_user_name"
printf '%s\n' "$_obj_user_city"
if (_bst_cond="$_obj_user_active"; [ -n "$_bst_cond" ] && [ "$_bst_cond" != 0 ]); then
    printf '%s\n' 'active'
fi
```

A hand-written shell script would normally test the boolean slot directly:

```sh
if [ "$_obj_user_active" = 1 ]; then
    printf '%s\n' 'active'
fi
```

## Implementation

- Teach `genCondition` to detect `PropertyExpr` values whose inferred type is boolean.
- Emit `[ <property-value> = 1 ]` for boolean properties, matching existing boolean console formatting.
- Keep optional property chains and `process.env` on their current nullish-aware path.
- Keep non-boolean property conditions on generic truthiness.

## Verification

- Add codegen coverage for object boolean property conditions.
- Add fallback coverage for string property truthiness.
- Add runtime integration coverage for boolean property conditions.
- Update `AGENTS.md`, `README.md`, and `skills/besht-scripting/SKILL.md`.
- Run the focused codegen suite and the full gate before commit.
