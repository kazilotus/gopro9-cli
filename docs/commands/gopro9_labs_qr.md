## gopro9 labs qr

Render a Labs command string as a QR code

```
gopro9 labs qr <command> [flags]
```

### Examples

```
  gopro9 labs qr 'mVr4p60!S!10E'            # 4K60 video, record 10 s
  gopro9 labs qr '!MBOOT="!Lbt"' --png boot.png
```

### Options

```
  -h, --help         help for qr
      --invert       invert the terminal rendering for light backgrounds
      --no-qr        print only the command string, no QR code
      --png string   write the QR code as PNG to this file
      --size int     PNG size in pixels (default 512)
      --svg string   write the QR code as SVG to this file
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

