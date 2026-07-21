# DAP capabilities

| Request | Support | Notes |
|---|---|---|
| `initialize` | Yes | Reports the capabilities listed here. |
| `launch` | Yes | Loads one standalone AMX file. |
| `setBreakpoints` | Yes | Requires compiler line metadata. |
| `configurationDone` | Yes | No additional action is needed. |
| `threads` | Yes | Reports one Pawn runtime thread. |
| `continue` | Yes | Runs until a breakpoint or program exit. |
| `next` | Partial | Advances one AMX instruction. |
| `stepIn` | Partial | Advances one AMX instruction. |
| `stepOut` | Partial | Advances one AMX instruction. |
| `stackTrace` | Partial | Reports the current runtime frame. |
| `scopes` | Yes | Reports one Pawn scope. |
| `variables` | Partial | Registers, scalars, and bounded one-dimensional arrays. |
| `evaluate` | Partial | Reads PRI, ALT, HEA, STK, or FRM. |
| `disconnect` | Yes | Closes the loaded runtime. |
| `terminate` | Yes | Closes the loaded runtime. |
| `pause` | No | Execution currently runs synchronously. |
| `disassemble` | No | Reported as unsupported during initialization. |
| `readMemory` | No | Reported as unsupported during initialization. |

Source paths are matched after cleaning and conversion to absolute paths.
Native libraries are never loaded into the adapter process.
