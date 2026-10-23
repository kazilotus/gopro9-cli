# ADR 0005: Labs command language and GPMF decoding ship in the USB tool

**Status:** accepted · 2026-10-23

## Context

GoPro Labs commands enter a HERO9 only through QR codes; the HTTP API has no Labs
endpoint on this model. Telemetry is served by the camera as an MP4 holding a `gpmd`
track, not as raw GPMF.

## Decision

- `pkg/gopro/labs`: tolerant tokenizer, validator with a HERO9 extension-support table,
  explainer, fluent builder and QR renderers. The camera reads QR codes from a screen
  while cabled and recording, so the CLI can author a script and confirm its effect with
  `state --watch` in one session.
- `pkg/gopro/gpmf`: stdlib-only KLV parser plus a minimal ISO-BMFF walker that extracts
  the `gpmd` samples and per-sample durations; GPX and CSV exporters.
- Firmware upload is out of scope by owner decision; the endpoint is documented in the
  capability matrix but has no command.
