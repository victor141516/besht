# Static Named Object Values Plan

## Context

Small Besht sample:

```ts
let user = { id: 1, name: "Ada", active: true }
console.log(Object.values(user).length)
console.log(Object.values(user).join(","))
for (value in Object.values(user)) {
    console.log(value)
}
```

Current compiled output with `--opt-no-add-binaries-check --opt-no-source-map` uses runtime metadata and dynamic value lookup even though the object has static scalar values and has not been mutated:

```sh
_objkeys_user='id name active'
printf '%s\n' "$(printf '%s\n' "$(for _bst_obj_key in $_objkeys_user; do ... eval "_bst_obj_value=\"\${_obj_user_${_bst_obj_key}}\"" ...; done)" | wc -l | tr -d ' ')"
printf '%s\n' "$(printf '%s\n' "$(for _bst_obj_key in $_objkeys_user; do ...; done)" | awk -v s=',' ...)"
_forlist_4_1=$(for _bst_obj_key in $_objkeys_user; do ...; done)
while IFS= read -r value; do
    printf '%s\n' "$value"
done <<__BESHT_FOR_4_1
$_forlist_4_1
__BESHT_FOR_4_1
```

A hand-written shell script would use static scalar values:

```sh
_objkeys_user='id name active'
printf '%s\n' 3
printf '%s\n' '1,Ada,true'
for value in '1' 'Ada' 'true'; do
    printf '%s\n' "$value"
done
```

Inline object literal `Object.values()` calls already fold when all values are scalar and static. Named objects currently keep the dynamic metadata path because property values and key sets can change later. This pass will fold only object literal bindings whose values are static scalar values and whose object has not been invalidated by later property assignment.

## Implementation

- Track static scalar value lists for object literal bindings alongside the existing static key tracking.
- Invalidate that tracked value list on any dot or computed property assignment to the object.
- Fold `Object.values(name)` to a static newline-backed list while the value list remains tracked.
- Reuse existing list folding paths for `.length`, `.join(...)`, and compact list loops.
- Preserve runtime metadata paths for mutated objects, non-static values, nested/list/object/command/fetch values, object parameters, aliases with unknown slots, and dynamic cases.

## Verification

- Add codegen coverage for static named object values, length, join, and compact list loops.
- Add fallback coverage after property mutation.
- Add runtime integration coverage for folded named-object value output.
- Update `AGENTS.md`, `README.md`, and `skills/besht-scripting/SKILL.md`.
- Run focused codegen tests and the full gate before committing.
