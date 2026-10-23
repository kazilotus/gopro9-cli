# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project adheres to
[Semantic Versioning](https://semver.org).

## [Unreleased]

### Changed

- Module targets Go 1.26 (toolchain go1.26.10) to pick up the crypto/x509 fix flagged by govulncheck.

### Added

- M4 live: `webcam start|preview|stop|exit|status|version`, `stream start|stop|play|record`
  (UDP MPEG-TS receiver with stats, player hand-off to ffplay/mpv/vlc).
- M3 media: `media list|tree|download|thumbnail|screennail|gpmf|telemetry|hilight|delete`;
  resumable, size-verified, parallel downloads with turbo transfer; LRV/THM companions.
- M2 control: `shutter start|stop|toggle`, `capture video|photo|timelapse`, `preset list|load|group`,
  `setting list|get|set|describe|dump|apply` with typed validation and HERO9 support warnings,
  `zoom`, `power sleep|locate`, `turbo on|off`.
- M1 connect: USB discovery (subnet probe + native mDNS), `discover`, `doctor` (link checks,
  Host-guard test, capability-drift probes, fixture recording), `info`, `status`, `state`
  (typed, `--raw`, `--watch`), `keepalive`, `datetime get|set|sync`, `capabilities`.
- SDK: `pkg/gopro` facade, `transport`, `ogp`, `gpcontrol`, `presets`, `discovery`; generated
  `settings`, `status` and `capability` packages from the vendored Open GoPro spec.
- Fake HERO9 (`internal/fakecam`) driving CLI golden tests; hardware e2e suite behind `-tags e2e`.
- M0 skeleton: `version`, `completion`, `docs` commands; output formats table/json/yaml/plain;
  sysexits-style exit codes; CI (lint, tests on 3 OSes, cross-build, govulncheck, GoReleaser snapshot).
