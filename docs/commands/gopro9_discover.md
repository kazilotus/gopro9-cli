## gopro9 discover

Find cameras on the USB link

### Synopsis

Probe every network interface for a HERO9 (the camera always takes host .51
of its USB subnet) and enrich matches with mDNS metadata (firmware, serial).
With --output plain, prints just the first camera's IP for use in scripts.

```
gopro9 discover [flags]
```

### Options

```
  -h, --help                 help for discover
      --mdns-wait duration   how long to wait for mDNS answers (default 1.5s)
      --no-mdns              skip mDNS enrichment (faster)
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

