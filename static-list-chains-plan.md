## Static scalar list method chains

### Probe

Small Besht script:

```ts
console.log(["a", "b"].concat(["c"]).join(","))
console.log(["a", "b", "c"].slice(1).join(","))
console.log(["a", "b"].reverse().join(","))
console.log(["a", "b"].push("c").join(","))
let xs = ["a", "b"]
console.log(xs.concat(["c"]).join(","))
console.log(xs.slice(1).join(","))
```

Current compiled output folds the base list, but each method chain still emits nested shell:

```sh
printf '%s\n' "$(printf '%s
' "$(printf '%s\n%s' 'a
b' 'c')" | awk -v s=',' 'NR>1{printf s}{if($0=="__BESHT_NL__")printf "\n"; else printf "%s",$0}')"
```

A direct shell script would naturally print constants:

```sh
printf '%s\n' 'a,b,c'
printf '%s\n' 'b,c'
printf '%s\n' 'b,a'
printf '%s\n' 'a,b,c'
xs='a
b'
printf '%s\n' 'a,b,c'
printf '%s\n' 'b'
```

### Desired direction

Extend static scalar list value recognition through simple list-returning methods:

- `.concat(otherStaticList)`
- `.slice(start, end?)`
- `.reverse()`
- `.push(value)`
- `.unshift(value)`
- `.pop()`
- `.shift()`

Then existing `.join()`, `.toString()`, `.length`, indexes, loops, and variable bindings can reuse the
same static value path.

Keep runtime paths for dynamic receivers, dynamic arguments, values containing newlines, nested lists,
and variables that may have been assigned inside control flow.

### Implementation notes

- Extend the static scalar-list helper rather than special-casing `.join()`.
- Reuse `staticListMap` for variables bound to static scalar lists.
- Preserve existing behavior for mutating-looking list methods: these methods return new lists in Besht
  expression position; they do not mutate the receiver.
- Keep statement-position list mutations (`xs.unshift("a")`) in sync with static list metadata when
  their receiver and argument are still static.
- Update `AGENTS.md`, `README.md`, and `skills/besht-scripting/SKILL.md`.

### Verification

- Codegen tests for inline static chains and variable-backed static chains.
- Dynamic fallback test after control-flow assignment.
- Runtime integration test for the folded outputs.
- `gofmt` changed Go files.
- `git diff --check`
- `make build`
- `make test`
- `go test ./...`
- `bun node-eq/compare $(rg --files -g '*.bsh' node-eq/tests | sort)`
