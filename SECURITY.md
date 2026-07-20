# Security policy

Report vulnerabilities through GitHub's private
[security advisory form](https://github.com/pawnkit/pawndebug/security/advisories/new).
Please do not open a public issue before a fix is available.

AMX files and DAP input are untrusted. Messages are limited to 8 MiB, and
headers are limited to 8 KiB. Expression evaluation reads a fixed set of AMX
registers and never invokes a shell. Native libraries are not loaded.

Useful reports include malformed input that causes excessive resource use,
path handling that reads unintended files, or expression evaluation that
reaches the host environment.
