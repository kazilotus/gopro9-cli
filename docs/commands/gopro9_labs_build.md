## gopro9 labs build

Compose a Labs command from typed flags and render it

```
gopro9 labs build [flags]
```

### Examples

```
  gopro9 labs build --mode video --res 4K --fps 60 --lens linear --ev -0.5 --start-in 10s --duration 30s
  gopro9 labs build --mode photo --start-at 06:30 --repeat 10m --message "Sunrise"
  gopro9 labs build --ext HIST=1 --ext BURN=1 --meta OWNR="Jane Doe"
```

### Options

```
      --color string         gopro, natural, flat
      --duration duration    stop capture after this long
      --ev float             exposure compensation (-2.0 .. 2.0)
      --ext stringArray      Labs extension FOURCC=value (repeatable), e.g. HIST=1
      --fps int              frames per second
      --gps string           on or off
  -h, --help                 help for build
      --hypersmooth string   off, on, high, boost, auto
      --interval float       time-lapse interval in seconds (0.5, 2, 60 ...)
      --invert               invert the terminal rendering for light backgrounds
      --iso string           ISO range MIN:MAX, e.g. 100:1600
      --lens string          wide, narrow, medium, linear, superview, max-superview, horizon, max
      --message string       text to show on the camera screen
      --meta stringArray     permanent metadata FOURCC=value (repeatable), e.g. OWNR=Name
      --mode string          video, video-loop, photo, burst, night, timelapse, timelapse-photo, timewarp, nightlapse, nightlapse-photo, liveburst
      --no-qr                print only the command string, no QR code
      --png string           write the QR code as PNG to this file
      --raw stringArray      raw token appended verbatim (repeatable)
      --repeat duration      repeat the whole script every interval
      --res string           resolution token: 4K, 2.7K, 1440, 1080, 4K4:3, 5K ...
      --size int             PNG size in pixels (default 512)
      --start-at string      start capture at this clock time (HH:MM)
      --start-in duration    start capture after this delay
      --svg string           write the QR code as SVG to this file
      --wb string            white balance in kelvin (2300..6500) or 'auto'/'native' via --raw wA/wN
```

### Options inherited from parent commands

```
      --host string                    camera IP or hostname (default: auto-discover on the USB link)
      --output table|json|yaml|plain   output format (default table)
      --port int                       camera HTTP port (default 8080)
  -q, --quiet                          suppress progress and informational output
      --timeout duration               per-request timeout (default 5s)
  -v, --verbose                        log HTTP requests to stderr
  -y, --yes                            assume yes for confirmations
```

### SEE ALSO

* [gopro9 labs](gopro9_labs.md)	 - GoPro Labs command language: build, validate, explain and render QR codes

