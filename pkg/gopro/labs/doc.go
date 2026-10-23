// Package labs implements the GoPro Labs QR command language as it applies to
// the HERO9 Black: a tolerant tokenizer that turns a command string into typed
// commands, a validator that applies the grammar's rules and the HERO9
// extension support table, a human-readable explainer, a fluent Builder that
// produces canonical command strings, QR renderers (PNG, SVG, terminal), and a
// helper for the Labs precision time-sync command.
//
// # Grammar summary
//
// A Labs command string is a sequence of commands with no delimiters. Every
// command starts with a lowercase letter or a special character; uppercase
// letters and digits continue the current command. Spaces are only allowed
// inside double-quoted strings.
//
//   - Modes: m followed by uppercase letters, e.g. mV (video), mP (photo),
//     mTW (TimeWarp), mNL (night-lapse video), mVL (looping video).
//   - Settings: one lowercase letter plus a value, e.g. r4 (4K), r1080,
//     p60 (60 fps), fW (wide lens), cF (flat colour), x-.5 (EV -0.5),
//     w55 (5500 K), i16M2 (ISO 1600 max / 200 min), e2 (HyperSmooth high),
//     hS1 (HindSight 15 s), g1 (GPS on), sL (sharpness low), aL (audio low).
//   - Options: o plus an uppercase option letter and value; options stack,
//     so oV1B4D2 equals oV1oB4oD2. oT sets the clock, oM stores temporary
//     metadata.
//   - Actions: ! with an optional time prefix (seconds, hh:mm, hh;mm for
//     clock-aligned repeats, or sun-relative s/r/n) and an uppercase action:
//     !S start, !E end, !N sleep, !R repeat, !Lname load a saved script,
//     !SAVEname=script, !W join Wi-Fi, !GS/!GM/!GL live stream, !O shut down.
//   - Conditionals: <, > or == followed by a camera-state letter (b battery,
//     f SD free, t:W weekday, ...) or a clock time and a value; the condition
//     applies to the next command only, + bundles further commands, ~ starts
//     the else branch.
//   - Variables: =A5 assigns, =A+1 / =A*B compute, =Tt:W reads camera state.
//   - Metadata: *FOUR=value is permanent (HERO9 uses the older !MFOUR=value
//     form), $FOUR=value lasts until power off. FourCCs that name Labs
//     extensions (BITR, HIST, BOOT, ...) configure those features.
//   - Macros: *NAME="..." defines, ^NAME runs. "text" shows a message, with
//     $V interpolating variable V.
//
// # HERO9 caveat
//
// Commands reach a HERO9 Black only as QR codes shown to the lens (the camera
// reads them from a phone or laptop screen, even while cabled and recording).
// This package produces and renders the strings; the CLI displays them. The
// extension support table reflects the published Labs feature matrix for the
// HERO9; entries marked Uncertain are listed inconsistently across GoPro's
// pages and should be tried on the camera.
package labs
