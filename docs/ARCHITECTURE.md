# gopro9-cli — Architecture

A production-grade Go CLI that exposes **everything a GoPro HERO9 Black can do over a USB-C
cable**: Open GoPro 2.0 HTTP API, legacy gpControl, media file server, webcam and preview
streams, firmware upload, and GoPro Labs command/QR tooling.

Target camera: HERO9 Black, firmware HD9.01.01.72.xx (stock) and HD9.01.01.72.70 (Labs).
Target hosts: macOS, Linux, Windows (amd64 + arm64).

---

## 1. Goals and non-goals

### Goals

| # | Goal | How it is enforced |
|---|------|--------------------|
| G1 | Complete USB surface. Every HERO9 operation reachable over the USB Ethernet link has a typed command. | `capabilities.yaml` is the single source of truth; a test fails if any `Verified`/`Spec` operation has no CLI command and no SDK method. |
| G2 | End-to-end typed. No `map[string]any`, no stringly-typed setting IDs or option values crossing a package boundary. | Generated enums for every setting/status; `golangci-lint` forbids `interface{}`/`any` in exported signatures via `forbidigo`. |
| G3 | Importable SDK. The CLI is a thin shell over `pkg/gopro`, which third parties can `go get`. | `internal/` holds only CLI concerns; `pkg/` has zero dependencies on cobra or output formatting. |
| G4 | Pyramid-tested. Fast unit base, contract/integration middle, thin hardware E2E top. | Coverage gates per layer; E2E behind build tag; fake camera validated against the OpenAPI spec. |
| G5 | Scriptable. Stable JSON output, meaningful exit codes, no interactive prompts unless a TTY and not `--yes`. | Golden-file tests on every renderer; exit codes are an enum with a test that every error maps to one. |
| G6 | Publish-ready OSS. Releases, signatures, SBOM, docs, governance files. | GoReleaser + cosign + syft in CI; release blocked unless `CHANGELOG` has an entry. |

### Non-goals (v1)

- BLE and Wi-Fi transports. The HTTP layer is transport-agnostic (`--host` works over Wi-Fi at
  10.5.5.9), but discovery, pairing and BLE wakeup are out of scope.
- MTP mass-storage mode. Different USB configuration, no HTTP; macOS has no native MTP stack.
- COHN, live streaming (RTMP), camera naming, custom-preset editing. Not on HERO9 firmware.
- GUI, TUI, or a daemon. One process, one invocation, exits.

---

## 2. Ground truth: what the camera actually exposes

Measured on the real camera (serial redacted), see `testdata/fixtures/hero9_HD9.01.01.72.70/`.

**USB link.** CDC-ECM composite device (VID `0x2672`, PID `0x0052`). Host gets a DHCP lease on
`172.2x.1xx.0/24`; camera is always `.51`, host `.52+`. mDNS advertises
`_gopro-web._tcp` → `<serial>.local:80` with TXT `model`, `fw_version`, `platform_version`.
IPv6 link-local exists but HTTP is IPv4-only.

**Ports.** 80, 8000, 8001, 8080 serve the identical HTTP handler set. 8081 is a raw,
single-client TCP socket (not HTTP) — treated as unknown and unused. No TLS. UDP 8554 is the
destination for preview/webcam MPEG-TS once started.

**HTTP quirks that the transport layer must handle.**

- Host header is validated: unknown or missing `Host` → `403 Connection forbidden`. The camera
  IP, `10.5.5.9` and `localhost` are accepted.
- HTTP method is ignored (GET/POST/PUT/DELETE/OPTIONS/HEAD all return 200 on GET endpoints).
  We always send GET except firmware upload (multipart POST).
- `HEAD` on media files is 403, but `GET` with `Range` returns 206 → resumable downloads use
  a 0-0 range probe rather than HEAD.
- Open GoPro misses return `404 {}`; unknown paths return `404 text/plain`.
- `Connection: close`, no keep-alive. Camera sleeps after ~10 min idle unless
  `/gopro/camera/keep_alive` is sent.

**Spec vs. reality.** The OpenAPI spec's `x-supported-cameras` says shutter, webcam and
`get_date_time` are *not* HERO9, yet they answer on this firmware. Conversely
`media/info`, `last_captured`, `wired_usb` are listed as HERO9 but return 404. Hence
§4.3: a repo-owned capability table overrides the spec.

---

## 3. Repository layout

```
gopro9-cli/
├── cmd/gopro9/main.go               # 10 lines: build info → internal/cli.Main(ctx, os.Args, stdio)
├── internal/
│   ├── cli/                         # cobra tree; one file per command group; no business logic
│   │   ├── root.go                  # global flags, config, logger, exit-code mapping
│   │   ├── camera.go  setting.go  preset.go  media.go  webcam.go  stream.go
│   │   ├── firmware.go  labs.go  raw.go  doctor.go  capabilities.go  completion.go
│   │   └── render/                  # table | json | yaml | plain renderers (golden-tested)
│   ├── config/                      # XDG config file + env + flags precedence (koanf)
│   ├── gen/                         # `go run ./internal/gen` → generated Go from spec + overrides
│   │   ├── main.go  model.go  templates/*.tmpl
│   └── fakecam/                     # httptest HERO9 emulator driven by fixtures (test only)
├── pkg/gopro/                       # PUBLIC SDK — no cobra, no fmt.Print, context everywhere
│   ├── gopro.go                     # Camera facade: New(opts) → *Camera with sub-clients
│   ├── transport/                   # http.Client wrapper: Host header, retries, timeouts, tracing hooks
│   ├── discovery/                   # USB iface + DHCP heuristic + mDNS; returns typed Endpoint
│   ├── capability/                  # generated matrix: Op → Support{Spec,Verified,Unsupported}
│   ├── ogp/                         # Open GoPro 2.0 operations (control, query, presets, hilight, zoom…)
│   ├── gpcontrol/                   # legacy /gp/gpControl (info, status, locate, sleep, delete, analytics)
│   ├── settings/                    # GENERATED: SettingID, per-setting enum types, registry
│   ├── status/                      # GENERATED: StatusID, typed State accessors
│   ├── presets/                     # typed preset groups/presets (EnumPresetGroup, EnumPresetTitle…)
│   ├── media/                       # list, download (range/resume/parallel), thumbnails, delete, DCIM walk
│   ├── gpmf/                        # GPMF/telemetry parser → typed streams (GPS5, ACCL, GYRO…) + GPX/CSV
│   ├── webcam/                      # start/preview/stop/exit/status/version with typed Res/FOV
│   ├── stream/                      # UDP 8554 receiver, TS passthrough to file/stdout, ffplay handoff
│   ├── ota/                         # firmware package validation + multipart upload (opt-in, dangerous)
│   └── labs/                        # Labs command language: tokenizer, AST, validator, builder, QR render
├── spec/
│   ├── opengopro/openapi.json       # vendored upstream spec (pinned commit in SPEC_VERSION)
│   ├── hero9_overrides.yaml         # empirical corrections to x-supported-cameras
│   └── gpcontrol_legacy.yaml        # hand-written legacy endpoint catalogue (not in upstream spec)
├── testdata/
│   ├── fixtures/hero9_HD9.01.01.72.70/   # recorded real responses (scrubbed)
│   ├── golden/                      # CLI output snapshots
│   └── gpmf/                        # sample telemetry blobs
├── test/e2e/                        # //go:build e2e — real camera over USB
├── docs/
│   ├── ARCHITECTURE.md  (this)  COMMANDS.md (generated)  CAPABILITIES.md (generated)
│   └── adr/                         # architecture decision records
├── .github/                         # workflows, templates, CODEOWNERS, dependabot, SECURITY
├── .goreleaser.yaml  .golangci.yaml  Makefile  Taskfile.yml (optional)  go.mod
├── README.md  LICENSE  CONTRIBUTING.md  CODE_OF_CONDUCT.md  SECURITY.md  CHANGELOG.md
```

Dependency rule (enforced by `depguard`): `cmd → internal/cli → pkg/gopro → stdlib + tiny deps`.
`pkg/gopro` may not import `internal/`.

---

## 4. Core design

### 4.1 Camera facade and transport

```go
cam, err := gopro.New(ctx,
    gopro.WithEndpoint(ep),            // from discovery or --host
    gopro.WithTimeout(5*time.Second),
    gopro.WithRetry(3, backoff.Exponential),
    gopro.WithLogger(slog.Default()),
)
state, err := cam.Query.State(ctx)                       // typed status.State
err = cam.Settings.Set(ctx, settings.VideoResolution4K)  // compile-time typed
err = cam.Control.Shutter(ctx, ogp.ShutterStart)
```

`transport.Client` is the only place that knows about HTTP:

- Sets `Host` to the camera IP (fixes the 403 rebinding guard when using `.local` names).
- Normalises errors into `*transport.Error{Op, Status, Body, URL}` with sentinel wrapping:
  `ErrNotFound` (404), `ErrForbidden` (403), `ErrBusy` (camera encoding/transferring),
  `ErrUnreachable` (dial), `ErrTimeout`.
- Idempotent GETs retry on dial/timeout only; never retry shutter/delete/upload.
- Optional `Keepalive(ctx)` goroutine for long sessions (`stream`, `webcam`, `media download --all`).
- Per-request `slog` + `httptrace` hooks for `--verbose`.

### 4.2 Code generation from the OpenAPI spec

`go run ./internal/gen` reads `spec/opengopro/openapi.json` + `spec/hero9_overrides.yaml` and
emits (committed, CI checks `git diff --exit-code` after regen):

| Output | Content |
|--------|---------|
| `pkg/gopro/settings/zz_settings.go` | `type SettingID uint16` with constants; one Go type per setting (`type VideoResolution uint8`) with constants only for options whose `x-option-metadata` includes `HERO9 Black`; `String()`, `Parse…()`, `MarshalText/UnmarshalText`, `pflag.Value`; a `Registry` mapping ID ↔ name ↔ type for the generic `setting set <name> <value>` path. |
| `pkg/gopro/status/zz_status.go` | `type StatusID uint16`; `State` struct with typed fields (`BatteryPercent uint8`, `Encoding bool`, `SDRemainingKB uint64`, enums for multi-value statuses) decoded from the `{"status": {"1": 1,…}}` wire shape; unknown IDs preserved in `Extra map[StatusID]int64` for forward-compat (the only permitted map, unexported wire type). |
| `pkg/gopro/capability/zz_matrix.go` | `Op` enum for all 110 spec operations + legacy catalogue; `Support(op) Level` where Level ∈ {Verified, Spec, Unsupported}. |
| `docs/COMMANDS.md`, `docs/CAPABILITIES.md` | Generated reference docs. |

Setting ranges without enums (`88` LCD brightness 10–100) become `type LCDBrightness uint8`
with a `Validate()` bound check.

### 4.3 Capability matrix

`spec/hero9_overrides.yaml`:

```yaml
spec_version: "2.0"            # upstream openapi info.version
camera: "HERO9 Black"
firmware_tested: ["HD9.01.01.72.00", "HD9.01.01.72.70"]
overrides:
  OGP_SHUTTER:                 { support: verified, note: "spec omits HERO9; answers 200" }
  GPCAMERA_WEBCAM_START_OGP:   { support: verified }
  OGP_GET_DATE_AND_TIME_DST:   { support: verified }
  OGP_MEDIA_INFO:              { support: unsupported, note: "404 on 1.72" }
  OGP_GET_LAST_MEDIA:          { support: unsupported, note: "400 {}" }
  OGP_SET_WIRED_USB_CONTROL:   { support: unsupported, note: "HERO10+; USB control via camera menu" }
```

Every CLI command declares its `capability.Op`. Before executing, the CLI checks the level:
`Unsupported` → exit 69 (`EX_UNAVAILABLE`) with a message, unless `--force`.
`capabilities` command prints the whole matrix; `doctor` probes the live camera and diffs it
against the matrix (this is how the overrides file was produced and how firmware drift is caught).

### 4.4 Command tree

Verbs are `noun verb`. Everything supports `--output table|json|yaml|plain`, `--host`,
`--timeout`, `--verbose`, `--quiet`, `--yes`. Destructive commands require `--yes` or a TTY
confirmation.

```
gopro9
├── discover                       find camera (iface, IPs, mDNS TXT) → Endpoint
├── doctor                         link + DHCP + Host-header + port + mDNS + capability-drift report
├── capabilities [--op X]          support matrix (verified / spec / unsupported)
├── info                           model, firmware, serial, board, AP SSID (gpControl/info)
├── version                        Open GoPro API version + CLI build info
├── status                         one-screen summary (battery, SD, mode, recording, temp flags)
├── state [--raw] [--watch 1s]     full typed status + settings; --watch streams diffs
├── keepalive [--interval 3s]      heartbeat until Ctrl-C
├── datetime  get | set <RFC3339> | sync
├── shutter   start | stop | toggle
├── capture   video --duration 10s | photo | timelapse --duration
├── preset    list | load <id|title> | group <video|photo|timelapse>
├── setting   list [--group] | get <name> | set <name> <value> | describe <name> | dump | apply <file>
├── zoom      <0-100>
├── power     sleep | locate on|off
├── turbo     on | off
├── media
│   ├── list [--dir 100GOPRO] [--grouped] [--since] [--type mp4|jpg|lrv|thm]
│   ├── tree                       raw DCIM directory walk (HTML index parser)
│   ├── download <path…> [--out DIR] [--all] [--since] [--lrv] [--thm] [--parallel 2] [--resume] [--verify]
│   ├── thumbnail <path> [--out]   | screennail <path> [--out]
│   ├── gpmf <path> [--out]        raw GPMF header blob
│   ├── telemetry <path> [--format raw|json|gpx|csv] [--stream GPS5|ACCL|GYRO|…]
│   ├── hilight add <path> --at 12.5s | remove <path> --at | moment
│   └── delete file <path> | group <path> | all      (requires --yes)
├── webcam    start [--res 480|720|1080] [--fov wide|narrow|superview|linear] | preview | stop | exit | status | version
├── stream    start [--port 8554] | stop | play [--player ffplay|vlc|mpv] | record --out file.ts [--duration]
├── firmware  info | validate <zip|dir> | upload <zip|dir> --i-understand-this-can-brick-the-camera
├── labs
│   ├── qr "<command>" [--png out.png] [--svg] [--ansi] [--size]     render a Labs QR code
│   ├── time [--utc] [--precision]                                   precision time-sync QR
│   ├── build [--mode video] [--res 4K] [--fps 60] [--ev -0.5] … [--start-in 10s] [--repeat 600s]  → command string
│   ├── validate "<command>"                                          tokenizer + grammar check, HERO9 token support
│   └── explain  "<command>"                                          human-readable expansion
├── raw get <path> [--port 8080] [--query k=v]…                        escape hatch, prints body
├── completion bash|zsh|fish|powershell
└── docs      markdown | man                                          generate reference docs
```

Why `labs` belongs in a USB tool: Labs commands can only enter the HERO9 via QR codes, but the
camera reads QR codes from a laptop screen while cabled and recording, and the CLI can
confirm the effect via `state --watch`. `labs build` gives users a typed way to produce
command strings instead of hand-writing `mVr4p60x-.5!S`.

### 4.5 Typed Labs command language

`pkg/gopro/labs` implements the documented grammar as a small compiler:

- **Lexer** splits on lowercase/special leading characters (`m`, `r`, `p`, `f`, `o`, `!`, `$`, `*`, `=`, `<`, `>`, `"`…).
- **AST** node types: `ModeCmd`, `SettingCmd`, `OptionCmd`, `Action{Kind, When}`,
  `Conditional`, `Assignment`, `Metadata{Permanent bool}`, `Macro`.
- **Validator** checks token legality, `!` never followed by lowercase, photo modes have no `!E`,
  quotes balanced, and HERO9 support for each extension token (`BITR` → unsupported on HERO9).
- **Builder** is the inverse: typed Go options → canonical string.
- **QR** rendering via `skip2/go-qrcode` (PNG/SVG) and a UTF-8 half-block terminal renderer.

### 4.6 Media download engine

- Range probe (`Range: bytes=0-0`) for size, since HEAD is 403.
- Resume from existing partial file length; `--verify` compares size (camera has no checksum).
- Bounded parallelism (default 2; the camera's single-core HTTP server degrades above that).
- Chapter awareness: `GH01xxxx`…`GH0nxxxx` grouped by `glrv`/`ls` flags from media list.
- Turbo transfer toggled automatically around bulk downloads and restored on exit (deferred,
  signal-safe).
- Progress to stderr via `schollz/progressbar` only when stderr is a TTY.

### 4.7 Output, errors, exit codes

- stdout = data, stderr = diagnostics. `--output json` always emits a single JSON document
  with a stable schema (`{"ok":true,"data":…}` / `{"ok":false,"error":{"code","message"}}`).
- Exit codes follow `sysexits.h`: 0 ok, 64 usage, 65 data (bad response), 69 unavailable
  (capability/camera off), 70 software, 74 io, 75 temp failure (camera busy), 78 config.
- All errors wrap sentinels; `errors.Is` is the only way the CLI inspects them.

### 4.8 Configuration

Precedence: flags > env (`GOPRO9_HOST`, `GOPRO9_OUTPUT`, `GOPRO9_TIMEOUT`) > config file
(`$XDG_CONFIG_HOME/gopro9/config.yaml`) > discovery > defaults. `gopro9 config show|path|init`.

---

## 5. Testing strategy (the pyramid)

```
            ┌──────────────┐
            │  E2E (real   │   test/e2e, //go:build e2e, GOPRO9_E2E=1
            │  camera)     │   read-only by default; GOPRO9_E2E_MUTATING=1 for shutter/settings
            ├──────────────┤   snapshot+restore settings; never deletes media; never uploads firmware
            │ Integration/ │   internal/fakecam httptest server fed by fixtures;
            │ Contract     │   CLI run in-process → golden files; fakecam validated against openapi.json
            ├──────────────┤
            │    Unit      │   enums, URL builders, parsers (media list, DCIM HTML, GPMF, Labs grammar),
            │              │   capability matrix, renderers, range math, exit-code mapping
            └──────────────┘   + property tests (pgregory.net/rapid) + go fuzz corpora
```

| Layer | Target | Tooling | Gate |
|-------|--------|---------|------|
| Unit | `pkg/**`, `internal/cli/render`, `internal/gen` | `testing`, `testify/require`, `rapid`, `go test -fuzz` corpora checked in | ≥ 90% statements in `pkg/gopro/{settings,status,labs,gpmf,media}`; `-race` always |
| Contract | `internal/fakecam` ↔ `spec/openapi.json` | `pb33f/libopenapi-validator` validates every fakecam request/response pair; every `Verified`/`Spec` op must have a fakecam handler | Fails CI if any op lacks a handler or a CLI command |
| Integration | full command tree | `cli.Main()` executed in-process with injected transport pointing at fakecam; golden files under `testdata/golden/` (`-update` flag) | ≥ 80% in `internal/cli` |
| E2E | real HERO9 over USB | `go test -tags e2e ./test/e2e`; self-hosted runner label `hero9-usb`; manual `workflow_dispatch` | Not required for merge; required before tagging a release |
| Static | everything | `golangci-lint` (errcheck, exhaustive, exhaustruct on wire structs, gocritic, gosec, revive, staticcheck, forbidigo, depguard, gofumpt, goimports), `go vet`, `govulncheck`, `nilaway` (advisory) | Blocks merge |
| Mutation (optional, nightly) | `pkg/gopro/labs`, `pkg/gopro/gpmf` | `gremlins` | Report only |

Fixtures are recorded with `gopro9 doctor --record DIR` from a real camera and scrubbed
(serial, SSID, MAC) by the recorder before writing. `internal/fakecam` replays them and models
state transitions (recording flag, keep-alive expiry, busy → `ErrBusy`) so integration tests can
cover the unhappy paths that are unsafe to drive on hardware.

---

## 6. Engineering standards

- **Go 1.25**, `GOTOOLCHAIN` pinned in `go.mod`; `GOFLAGS=-mod=mod` never; `go.sum` committed; `vendor/` not committed.
- Dependencies kept minimal and vetted: `cobra`, `pflag`, `koanf`, `go-qrcode`, `zeroconf` (mDNS), `libopenapi-validator` (test only), `testify`, `rapid`. Each new dep needs an ADR note.
- `context.Context` first parameter everywhere in `pkg/`. No globals, no `init()` side effects, no `log.Fatal` outside `main`.
- Structured logging via `log/slog`; `--verbose` enables HTTP traces to stderr.
- Signal handling: `SIGINT`/`SIGTERM` cancel the root context; deferred cleanup (turbo off, stream stop, webcam exit) runs with a 3 s grace context.
- Windows: discovery falls back to scanning interfaces for `172.2x.x.x/24` and probing `.51`; no `ioreg`/`networksetup` shell-outs anywhere.
- Conventional Commits; `CHANGELOG.md` in Keep-a-Changelog format, generated by `git-cliff`; SemVer; `v0.x` until E2E suite is green on both stock and Labs firmware.
- Public API stability: `pkg/gopro` follows SemVer from `v1.0.0`; `apidiff` runs in CI for PRs touching `pkg/`.

---

## 7. Release and repository hygiene

**CI (`.github/workflows/ci.yml`)**: lint → generate-check → unit+integration (matrix: ubuntu/macos/windows × amd64; arm64 via macos-14) → `govulncheck` → build all targets → upload coverage (Codecov) → `goreleaser release --snapshot` smoke.

**Release (`release.yml`, on tag `v*`)**: GoReleaser builds darwin/linux/windows × amd64/arm64, generates SBOM (syft), signs checksums and binaries with cosign keyless (Sigstore), publishes GitHub Release with git-cliff notes, pushes Homebrew formula to `kazilotus/homebrew-tap`, Scoop manifest, `.deb`/`.rpm`/`.apk` via nfpm, and a minimal distroless Docker image (for Wi-Fi `--host` use).

**Scheduled**: weekly `govulncheck` + Dependabot (gomod, actions); CodeQL on push/PR; OpenSSF Scorecard.

**Governance files**: `LICENSE` (MIT recommended; Apache-2.0 if you want an explicit patent grant), `CONTRIBUTING.md` (dev setup, `make`, fixture recording, how to add an op), `CODE_OF_CONDUCT.md` (Contributor Covenant 2.1), `SECURITY.md` (private disclosure, supported versions), issue templates (bug with `gopro9 doctor --output json` attached, feature, capability-drift), PR template with checklist (tests, docs regenerated, changelog), `CODEOWNERS`, `.editorconfig`, `.gitattributes` (LF, fixtures as binary).

**Docs**: README with 60-second quickstart, animated terminal demo (vhs), command reference generated by `gopro9 docs markdown`, `docs/CAPABILITIES.md` generated, ADRs in `docs/adr/` (0001 cobra, 0002 spec codegen + overrides, 0003 exit codes, 0004 fake camera as contract boundary, 0005 labs grammar scope), `pkg.go.dev` badge, GoReportCard.

---

## 8. Delivery milestones

| Milestone | Scope | Exit criteria |
|-----------|-------|---------------|
| M0 Skeleton | repo hygiene, CI, lint, goreleaser snapshot, `version`, `completion`, `docs` | green CI on 3 OSes, `go install` works |
| M1 Connect | `transport`, `discovery`, `doctor`, `info`, `version`, `state`, `status`, `keepalive`, codegen for settings/status/capabilities, fakecam + first golden tests | `doctor` passes on real camera; capability matrix matches §2 |
| M2 Control | `shutter`, `capture`, `preset`, `setting`, `zoom`, `power`, `turbo`, `datetime` | E2E mutating suite green with snapshot/restore |
| M3 Media | `media list/tree/download/thumbnail/screennail/gpmf/telemetry/hilight/delete`, GPMF parser (GPS5, ACCL, GYRO, SHUT, ISOE, WBAL) → GPX/CSV | resume + verify tested against fakecam fault injection; telemetry matches `gpmf-parser` reference on fixtures |
| M4 Live | `webcam`, `stream` (UDP receiver, record, player handoff) | 1080p webcam start → TS packets received → stop/exit cleanup under SIGINT |
| M5 Labs + OTA | `labs qr/time/build/validate/explain`, `firmware info/validate/upload` | Labs grammar fuzz corpus; upload gated and tested only against fakecam |
| v1.0.0 | docs, Homebrew tap, signed release, `apidiff` baseline | E2E green on stock 72.00 and Labs 72.70 |

---

## 9. Open decisions (ADR candidates)

1. **cobra vs kong.** cobra for ecosystem (completion, docs gen, familiarity); typed-ness is
   recovered by generated `pflag.Value` enums. kong would give struct-typed parsing for free
   but weaker docs/completion. Recommendation: cobra.
2. **Spec pinning.** Vendor `openapi.json` and record upstream commit; regen is a reviewed PR.
   Alternative: fetch at build time (rejected: non-reproducible builds).
3. **GPMF parser scope.** Own minimal parser for the handful of HERO9 streams vs. a full
   port. Start minimal with a reference-comparison test; grow by fixture.
4. **Firmware upload.** Ship behind a long flag and a capability `Dangerous` tag, or omit
   from the binary and keep as a documented `raw` recipe. Recommendation: ship, gated,
   because "everything over USB" includes it and the fake camera lets us test it safely.
