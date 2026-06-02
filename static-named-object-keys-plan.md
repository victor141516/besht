# Static Named Object Keys Plan

## Context

Small Besht sample:

```ts
let user = { id: 1, name: "Ada", active: true }
console.log(Object.hasOwn(user, "name"))
console.log(Object.hasOwn(user, "missing"))
console.log(Object.keys(user).length)
console.log(Object.keys(user).join(","))
for (key in Object.keys(user)) {
    console.log(key)
}
```

Current compiled output with `--opt-no-add-binaries-check --opt-no-source-map` uses runtime metadata pipelines even though the object has not been mutated:

```sh
_objkeys_user='id name active'
printf '%s\n' "$(if [ $(_bst_obj_key='name'; ... grep -qxF ...) = 1 ]; then printf true; else printf false; fi)"
printf '%s\n' "$(printf '%s\n' "$(if [ -n "$_objkeys_user" ]; then printf '%s\n' $_objkeys_user; fi)" | wc -l | tr -d ' ')"
_forlist_5_1=$(if [ -n "$_objkeys_user" ]; then printf '%s\n' $_objkeys_user; fi)
while IFS= read -r key; do
    printf '%s\n' "$key"
done <<__BESHT_FOR_5_1
$_forlist_5_1
__BESHT_FOR_5_1
```

A hand-written shell script would use constants and a direct word loop:

```sh
_objkeys_user='id name active'
printf '%s\n' true
printf '%s\n' false
printf '%s\n' 3
printf '%s\n' 'id,name,active'
for key in 'id' 'name' 'active'; do
    printf '%s\n' "$key"
done
```

Inline object literal APIs already fold to constants. Named objects keep runtime metadata because later assignments can mutate the key set. This pass will fold only object literal bindings whose key set has not been invalidated by later property assignment.

## Implementation

- Track static key sets for object literal bindings.
- Invalidate that tracked key set on any dot or computed property assignment to the object.
- Fold `Object.keys(name)` to a static newline-backed list while the key set remains tracked.
- Fold `Object.keys(name).length`, `.join(...)`, compact list loops, and `Object.hasOwn(name, staticKey)` from the same tracked key set.
- Preserve dynamic metadata paths for mutated objects, object parameters, aliases with unknown slots, and dynamic keys.

## Verification

- Add codegen coverage for static named object keys, length, join, compact list loops, and `hasOwn`.
- Add fallback coverage after a property mutation.
- Add runtime integration coverage for folded named-object key output.
- Update `AGENTS.md`, `README.md`, and `skills/besht-scripting/SKILL.md`.
- Run focused codegen tests and the full gate before committing.
