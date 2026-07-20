# Architecture

`cmd/pawndebug` connects stdin and stdout to the DAP server. The server owns
protocol sequencing and translates requests into the small interface in
`debug/backend.go`.

`backend/goamx` implements that interface. It loads one AMX runtime, installs a
debug hook, and preserves VM state while execution is stopped. The DAP package
does not depend on goamx, which keeps protocol tests independent from the VM.

Messages use standard DAP `Content-Length` framing. The server processes one
request at a time; it does not currently support asynchronous pause requests.
