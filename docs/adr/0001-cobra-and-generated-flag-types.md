# ADR 0001: cobra for the command tree, typing recovered through generated pflag.Values

**Status:** accepted · 2026-10-23

## Context

The CLI needs shell completion, generated reference docs, nested command groups and a
large ecosystem of examples. It also must be "end-to-end typed": a setting option on the
command line should be validated before anything reaches the camera.

## Decision

Use `spf13/cobra`. Every enumerated value a user can type (setting options, preset
groups, webcam resolution and FOV, output format) is a Go type implementing
`pflag.Value`, generated from the Open GoPro spec where possible. Commands never
touch raw strings for those values.

## Consequences

- Completion and `docs markdown|man` come for free.
- Typing lives in `pkg/gopro/*` types, so the SDK is as strict as the CLI.
- Rejected: `alecthomas/kong` (struct-typed parsing, weaker docs/completion story).
