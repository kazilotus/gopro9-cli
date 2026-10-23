# ADR 0003: sysexits-style exit codes and a stable JSON envelope

**Status:** accepted · 2026-10-23

## Decision

Exit codes follow `<sysexits.h>`: 0 ok, 64 usage, 65 bad data, 69 camera unavailable or
operation unsupported, 70 internal, 74 local I/O, 75 camera busy / refused, 78 config,
130 interrupted. Every SDK sentinel maps to exactly one code in `internal/cli/connect.go`.

`--output json|yaml` always writes `{"ok":true,"data":...}` or
`{"ok":false,"error":{"code","exit","message"}}`; stdout carries data only, stderr
carries progress and diagnostics. Destructive commands require `--yes`; there is no
interactive prompt, so scripts behave identically to humans.

## Consequences

Shell users and scripts can branch on exit codes without parsing messages; golden tests
pin every renderer.
