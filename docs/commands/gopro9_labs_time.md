## gopro9 labs time

Render the Labs time-sync QR code (sets the camera clock when scanned)

```
gopro9 labs time [flags]
```

### Options

```
      --at string    encode this time instead of now (RFC3339 or YYYY-MM-DDTHH:MM:SS)
  -h, --help         help for time
      --invert       invert the terminal rendering for light backgrounds
      --no-qr        print only the command string, no QR code
      --png string   write the QR code as PNG to this file
      --precision    include milliseconds (default true)
      --size int     PNG size in pixels (default 512)
      --svg string   write the QR code as SVG to this file
      --utc          encode UTC instead of local wall time
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

