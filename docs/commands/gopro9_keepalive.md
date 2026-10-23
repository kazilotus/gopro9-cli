## gopro9 keepalive

Send heartbeats so the camera does not sleep (Ctrl-C to stop)

```
gopro9 keepalive [flags]
```

### Options

```
  -h, --help                help for keepalive
      --interval duration   time between heartbeats (default 3s)
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

