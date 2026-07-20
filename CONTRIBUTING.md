# Contributing

PawnKit is maintained by volunteers, so reviews may take a little time.

Debugger fixes are welcome, particularly small DAP transcripts or AMX fixtures
that reproduce the problem.

Run the project checks before opening a pull request:

```sh
task check
```

Keep protocol messages bounded and do not evaluate expressions through a shell.
Update `docs/capabilities.md` when support for a DAP request changes.
