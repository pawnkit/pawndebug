# pawndebug

`pawndebug` is a Debug Adapter Protocol server for Pawn AMX programs. Editors
start it over stdio, then send the path to a compiled AMX file.

The adapter uses [goamx](https://github.com/pawnkit/goamx), so it does not need
a game server. Execution stops at the first instruction and continues from the
same VM state.

## Install

Download a release archive or install from source:

```sh
go install github.com/pawnkit/pawndebug/cmd/pawndebug@latest
pawndebug --version
```

Release archives are available for Linux, macOS, and Windows on amd64 and
arm64.

## Editor setup

An editor extension can launch `pawndebug` as a stdio DAP server. A launch
request needs one argument:

```json
{
  "program": "/path/to/gamemode.amx"
}
```

Compile the AMX with debug metadata to use source breakpoints and Pawn symbol
names.

## Current support

The adapter can launch an AMX program, stop on source lines, continue, and step
one instruction at a time. It exposes one runtime frame, scalar Pawn symbols,
and the PRI, ALT, HEA, STK, and FRM registers.

Step in, step over, and step out currently behave the same way. The adapter
does not expose raw memory, disassembly, nested call frames, or native plugins.
See the [capability matrix](docs/capabilities.md) for request-level details.

## Development

```sh
task check
```

Small debugger fixes and DAP transcripts are welcome. See
[CONTRIBUTING.md](CONTRIBUTING.md).
