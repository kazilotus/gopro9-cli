# ADR 0004: an in-process fake HERO9 is the integration-test contract boundary

**Status:** accepted · 2026-10-23

## Context

Hardware is slow, stateful and unsafe to drive in CI (deletion, firmware). Yet the CLI's
value is in handling the camera's quirks: positional query parameters, forbidden HEAD,
`%2F` rejection, mode-dependent 403s, slow webcam mode switches, UDP push streams.

## Decision

`internal/fakecam` replays recorded fixtures (`testdata/fixtures/<firmware>/`, scrubbed
by `gopro9 doctor --record`) and models state: recording, settings with HERO9 validation
and the real 403 body, presets, zoom, turbo, webcam status, media deletion with
companions, keep-alives, the Host guard, and synthetic MPEG-TS over UDP. Every quirk
discovered on hardware is encoded in the fake the same day so regressions are caught
without a camera.

Hardware tests live behind `-tags e2e`, are read-only by default, and are required
before tagging a release but not for merging.
