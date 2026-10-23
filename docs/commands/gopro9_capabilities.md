## gopro9 capabilities

Show what the HERO9 supports over USB (verified / spec / unsupported)

### Synopsis

The capability matrix is generated from the Open GoPro spec and corrected by
real-hardware probing. "verified" operations were exercised on a HERO9 over USB;
"spec" ones are listed for HERO9 upstream but not yet exercised; "unsupported"
ones are absent for HERO9 or known to fail on this firmware.

```
gopro9 capabilities [flags]
```

### Options

```
  -h, --help           help for capabilities
      --level string   filter by level: verified, spec or unsupported
      --op string      show a single operation by ID
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

