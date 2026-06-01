# Static Named Object Entries Plan

## Context

Small Besht sample:

```ts
let user = { id: 1, name: "Ada", active: true }
console.log(Object.entries(user).length)
for (entry of Object.entries(user)) {
    console.log(entry[0] + "=" + entry[1])
}
```

Current compiled output with `--opt-no-add-binaries-check --opt-no-source-map` builds entries through runtime object metadata, validates each key, uses `eval` to read each `_obj_user_*` slot, counts entries with `wc`, and iterates through a heredoc:

```sh
printf '%s\n' "$(printf '%s\n' "$(for _bst_obj_key in $_objkeys_user; do ... printf '%s\037%s\n' "$_bst_obj_key" "$_bst_obj_value"; done)" | wc -l | tr -d ' ')"
_forlist_3_1=$(for _bst_obj_key in $_objkeys_user; do ...; done)
while IFS= read -r entry; do
    printf '%s\n' "$(printf '%s\n' "$entry" | sed -n "$(( 0 + 1 ))p")=$(printf '%s\n' "$entry" | sed -n "$(( 1 + 1 ))p")"
done <<__BESHT_FOR_3_1
$_forlist_3_1
__BESHT_FOR_3_1
```

The runtime output is also not the natural `key=value` output for this static case; it prints packed entry rows with the unit separator still embedded.

A hand-written shell script would use fixed entries:

```sh
printf '%s\n' 3
printf '%s\n' 'id=1'
printf '%s\n' 'name=Ada'
printf '%s\n' 'active=true'
```

Inline object literal `Object.entries()` already folds when all entry values are static scalar values. Named objects keep the dynamic metadata path because keys and values can change later. This pass will fold only object literal bindings whose entries are static scalar values and whose object has not been invalidated by later property assignment.

## Implementation

- Track static packed `[key, value]` entry rows for object literal bindings.
- Invalidate tracked entries on any dot or computed property assignment to the object.
- Fold `Object.entries(name)` to a static newline-backed packed-row list while the entry list remains tracked.
- Reuse existing static list length binding for `.length`.
- Make compact `for-of` over static packed entries expose a temporary row as newline-delimited fields so `entry[0]` and `entry[1]` compile naturally inside the loop.
- Preserve runtime metadata paths for mutated objects, non-static values, nested/list/object/command/fetch values, object parameters, aliases with unknown slots, and dynamic cases.

## Verification

- Add codegen coverage for static named object entries, length, and compact `for-of`.
- Add fallback coverage after property mutation.
- Add runtime integration coverage for folded named-object entry output.
- Update `AGENTS.md`, `README.md`, and `skills/besht-scripting/SKILL.md`.
- Run focused codegen tests and the full gate before committing.
