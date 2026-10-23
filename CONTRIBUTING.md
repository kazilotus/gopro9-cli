# Contributing

Thanks for helping make `gopro9` better.

## Development setup

```sh
git clone https://github.com/kazilotus/gopro9-cli && cd gopro9-cli
make build          # bin/gopro9
make test           # unit + integration (fake camera), race detector
make lint           # golangci-lint v2
make generate-check # generated code matches spec/
```

Go 1.25 and `golangci-lint` v2.8+ are required. `make help` lists all targets.

## Project layout

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md). The short version:

- `pkg/gopro` is the importable SDK. No cobra, no printing, `context.Context` everywhere.
- `internal/cli` is the cobra command tree. It renders typed results and nothing else.
- `spec/` holds the vendored Open GoPro OpenAPI spec and the HERO9 override table that
  drives code generation (`internal/gen`).

## Adding or changing an operation

1. If the upstream spec is wrong for HERO9, add an entry to `spec/hero9_overrides.yaml`
   with evidence in the `note`.
2. Run `make generate` and commit the regenerated `zz_*.go` and docs.
3. Add the SDK method in the relevant `pkg/gopro/...` package with a unit test.
4. Add a handler to `internal/fakecam` (replaying a fixture under `testdata/fixtures/`).
5. Add the CLI command with a golden test in `internal/cli`.
6. If you have hardware, add a case in `test/e2e` (read-only unless unavoidable).

## Recording fixtures

```sh
gopro9 doctor --record testdata/fixtures/hero9_<firmware>/
```

The recorder scrubs serial numbers, SSIDs and MAC addresses before writing.

## Commit messages

[Conventional Commits](https://www.conventionalcommits.org): `feat:`, `fix:`, `docs:`,
`test:`, `ci:`, `chore:`, `refactor:`. Breaking SDK changes use `!` and a `BREAKING CHANGE:`
footer. The changelog is generated from these.

## Pull requests

Keep PRs focused. CI must be green (lint, generate-check, tests on three OSes, cross-build,
govulncheck, GoReleaser snapshot).
