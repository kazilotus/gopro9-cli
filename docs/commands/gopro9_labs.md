## gopro9 labs

GoPro Labs command language: build, validate, explain and render QR codes

### Synopsis

GoPro Labs firmware adds a scripting language entered by showing the camera a
QR code. These commands need no USB connection: they compose and validate
command strings and render them as QR codes (terminal, PNG or SVG) for the
camera to scan from your screen. The camera can read QR codes while cabled
and recording, so "gopro9 state --watch" can confirm the effect.

### Options

```
  -h, --help   help for labs
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

* [gopro9](gopro9.md)	 - Control a GoPro HERO9 Black over USB-C
* [gopro9 labs build](gopro9_labs_build.md)	 - Compose a Labs command from typed flags and render it
* [gopro9 labs explain](gopro9_labs_explain.md)	 - Translate a Labs command into plain English, one line per token
* [gopro9 labs extensions](gopro9_labs_extensions.md)	 - List Labs extension FourCCs and their HERO9 support
* [gopro9 labs qr](gopro9_labs_qr.md)	 - Render a Labs command string as a QR code
* [gopro9 labs time](gopro9_labs_time.md)	 - Render the Labs time-sync QR code (sets the camera clock when scanned)
* [gopro9 labs validate](gopro9_labs_validate.md)	 - Check a Labs command for syntax errors and HERO9 support

