# gopro9

[![CI](https://github.com/kazilotus/gopro9-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/kazilotus/gopro9-cli/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/kazilotus/gopro9-cli/pkg/gopro.svg)](https://pkg.go.dev/github.com/kazilotus/gopro9-cli/pkg/gopro)
[![Go Report Card](https://goreportcard.com/badge/github.com/kazilotus/gopro9-cli)](https://goreportcard.com/report/github.com/kazilotus/gopro9-cli)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Control a **GoPro HERO9 Black over a USB-C cable** from the terminal, and from Go.

- Camera control: shutter, timed captures, presets, every documented setting with typed
  validation, zoom, turbo transfer, sleep, locate, clock.
- Media: list with chapter groups, resumable size-verified parallel downloads,
  thumbnails, raw GPMF, telemetry decoded to **GPX / CSV / JSON**, HiLights, deletion.
- Live: webcam mode and the preview stream received over UDP, recorded to `.ts` or
  handed to ffplay / mpv / vlc.
- GoPro Labs: compose, validate, explain and render Labs command QR codes.
- Diagnostics: USB discovery (subnet probe + mDNS), `doctor` with capability-drift
  probes, a generated capability matrix of what this firmware really supports.

The SDK behind it, [`pkg/gopro`](https://pkg.go.dev/github.com/kazilotus/gopro9-cli/pkg/gopro),
is importable on its own: settings, statuses and the capability matrix are generated
from the Open GoPro OpenAPI spec and corrected against real hardware.

> Verified on HERO9 Black firmware HD9.01.01.72.00 (stock) and 72.70 (Labs).
> Firmware upload is deliberately not implemented.

## Install

```sh
go install github.com/kazilotus/gopro9-cli/cmd/gopro9@latest
```

Or download a signed archive, `.deb`, `.rpm` or `.apk` from
[Releases](https://github.com/kazilotus/gopro9-cli/releases). Checksums are signed with
cosign (keyless); the verification command is in each release's notes.

## Quick start

1. On the camera: **Preferences → Connections → USB Connection → GoPro Connect**.
2. Plug in the cable. The camera appears as a USB Ethernet adapter and takes `.51`
   of a private subnet.

```sh
gopro9 doctor                 # link, API, Host-guard, ports, mDNS, capability probes
gopro9 status                 # battery, SD card, mode, recording state
gopro9 setting set video-resolution 4K
gopro9 capture video --duration 10s
gopro9 media list --grouped
gopro9 media download --all --since 24h --out ~/GoPro
gopro9 media telemetry 100GOPRO/GH010516.MP4 --format gpx --out ride.gpx
gopro9 stream play            # preview stream in ffplay
gopro9 webcam start --res 1080 && gopro9 stream play --webcam
gopro9 labs build --mode video --res 4K --fps 60 --start-in 10s --duration 30s
```

Every command accepts `--output table|json|yaml|plain`; JSON output is a stable
`{"ok":…,"data":…}` envelope. Exit codes follow `sysexits(3)`: 0 ok, 64 usage,
65 bad data, 69 camera unavailable or unsupported, 70 internal, 74 local I/O,
75 camera busy or refused, 78 config. Destructive commands need `--yes`.

Configuration precedence is flags → `GOPRO9_*` environment → `gopro9 config` file →
USB discovery → defaults.

## Command tree

```
gopro9
├── discover · doctor · capabilities · info · status · state [--watch] · keepalive
├── datetime  get | set | sync
├── shutter   start | stop | toggle          capture  video | photo | timelapse
├── preset    list | load | group            setting  list | get | set | describe | dump | apply
├── zoom · turbo on|off · power sleep|locate on|off
├── media     list | tree | download | thumbnail | screennail | gpmf | telemetry
│             hilight add|remove|moment | delete file|group|all
├── webcam    start | preview | stop | exit | status | version
├── stream    start | stop | play | record
├── labs      qr | time | build | validate | explain | extensions
└── config show|path|set · completion · docs · version
```

Full reference: [docs/commands/gopro9.md](docs/commands/gopro9.md).

## Documentation

- [Capability matrix](docs/CAPABILITIES.md): every Open GoPro operation with its
  verified / spec / unsupported level on the HERO9, plus settings and statuses.
- [Architecture](docs/ARCHITECTURE.md) and [decision records](docs/adr/).
- [Contributing](CONTRIBUTING.md) · [Security](SECURITY.md) · [Changelog](CHANGELOG.md)

## Things learned from the hardware

These are encoded in the SDK and the fake camera used by the tests:

- The camera parses `/gopro/camera/setting?setting=N&option=M` positionally;
  `option` first is a 400. `%2F` in media paths is a 400 too.
- `HEAD` on media files is 403; sizes come from a one-byte `Range` request.
- A foreign `Host` header is rejected (DNS-rebinding guard). The same API is served on
  ports 80, 8000, 8001 and 8080. There is no authentication anywhere.
- Webcam start/stop/exit are slow mode switches. Aborting the request after 5 s left the
  camera's status endpoint returning 500 until a power cycle, so they get 30 s.
- The telemetry endpoint returns an MP4 with a `gpmd` track, not raw GPMF.

## Scope

In: everything the HERO9 exposes over the USB HTTP link, including legacy `gpControl`
endpoints and the DCIM file server. Out: BLE, Wi-Fi pairing, MTP mode, COHN, firmware
upload.

## License

MIT © kazilotus
