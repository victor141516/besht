# node-eq Tests

The comparison fixtures are grouped by purpose:

- `advent/` contains larger Advent-style language exercises.
- `commands/` covers `$()` command behavior, pipes, and script args.
- `imports/` keeps Besht and shell import fixtures beside their dependencies.
- `language/` contains focused language and API parity coverage.
- `regressions/` tracks resolved or still-known parity gaps.

Run the recursive suite with:

```sh
bun node-eq/compare $(rg --files -g '*.bsh' node-eq/tests | sort)
```

Keep import fixtures and their dependencies in the same directory unless the import paths are updated with the move.
