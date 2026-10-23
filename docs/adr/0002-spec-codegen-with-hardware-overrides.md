# ADR 0002: generate settings, statuses and the capability matrix from the Open GoPro spec, corrected by hardware overrides

**Status:** accepted · 2026-10-23

## Context

GoPro publishes an OpenAPI document with `x-supported-cameras` on every operation and
`x-option-metadata` on every setting option. It is the only machine-readable source of
setting and status semantics, but for the HERO9 it is wrong in both directions: it omits
operations that work (shutter, webcam, get_date_time) and lists ones that return 404
(media/info, last_captured, wired_usb). The camera also reports setting values the spec
does not list for the HERO9.

## Decision

Vendor `spec/opengopro/openapi.json` (pinned by SHA-256 in `SPEC_VERSION`) and generate
`pkg/gopro/{settings,status,capability}/zz_*.go` plus `docs/CAPABILITIES.md` with
`internal/gen`. A repo-owned `spec/hero9_overrides.yaml` wins over the spec; every
override carries evidence in its note. `gopro9 doctor` probes the live camera and
reports drift against the matrix.

Setting enums include every option the spec documents for any camera, flagged with
HERO9 support. The CLI warns on non-HERO9 options instead of refusing, because the
camera itself holds values the spec omits; the camera's 403 body (`available_options`)
is decoded to tell the user what is valid right now.

## Consequences

- Regeneration is a reviewed diff; CI fails when generated files are stale.
- Upstream spec updates are a one-file change plus override review.
