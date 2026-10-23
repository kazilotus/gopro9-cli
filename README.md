# gopro9

[![CI](https://github.com/kazilotus/gopro9-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/kazilotus/gopro9-cli/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/kazilotus/gopro9-cli/pkg/gopro.svg)](https://pkg.go.dev/github.com/kazilotus/gopro9-cli/pkg/gopro)
[![Go Report Card](https://goreportcard.com/badge/github.com/kazilotus/gopro9-cli)](https://goreportcard.com/report/github.com/kazilotus/gopro9-cli)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Control a **GoPro HERO9 Black over a USB-C cable** from the terminal: camera control,
typed settings, presets, media download and telemetry, webcam and preview streams, and
GoPro Labs QR tooling. Ships as a single static binary and as an importable Go SDK
(`pkg/gopro`).

> Status: pre-1.0. The command surface tracks the HERO9's Open GoPro 2.0 HTTP API as it
> actually behaves on firmware 1.72 (stock and Labs), verified against real hardware.

## Install

```sh
brew install kazilotus/tap/gopro9        # macOS / Linux (coming with v1.0)
go install github.com/kazilotus/gopro9-cli/cmd/gopro9@latest
```

Or download a signed release archive from the [Releases](https://github.com/kazilotus/gopro9-cli/releases) page.

## Quick start

1. On the camera: **Preferences → Connections → USB Connection → GoPro Connect**.
2. Plug in the cable. The camera appears as a USB Ethernet adapter.

```sh
gopro9 discover            # find the camera on the USB link
gopro9 status              # battery, SD card, mode, recording state
gopro9 setting set video-resolution 4k
gopro9 shutter start && sleep 5 && gopro9 shutter stop
gopro9 media list --output json
gopro9 media download --all --out ~/GoPro
gopro9 media telemetry 100GOPRO/GH010516.MP4 --format gpx > ride.gpx
gopro9 webcam start --res 1080 && gopro9 stream play
gopro9 labs qr 'mVr4p60!S!10E' --ansi      # render a Labs command as a QR code
```

Every command accepts `--output table|json|yaml|plain`. Exit codes follow `sysexits(3)`.

## Documentation

- [Command reference](docs/commands/gopro9.md) (generated)
- [Capability matrix](docs/CAPABILITIES.md): what the HERO9 actually supports, verified vs. spec
- [Architecture](docs/ARCHITECTURE.md)
- [Contributing](CONTRIBUTING.md) · [Security](SECURITY.md) · [Changelog](CHANGELOG.md)

## Scope

In: everything the HERO9 exposes over the USB HTTP link, including the legacy `gpControl`
endpoints and the DCIM file server. Out: BLE, Wi-Fi pairing, MTP mode, COHN, firmware
upload. See the architecture doc for the reasoning.

## License

MIT © kazilotus
