## gopro9 media gpmf

Save a video's raw GPMF metadata header

```
gopro9 media gpmf <DIR/NAME> [flags]
```

### Options

```
  -h, --help         help for gpmf
      --out string   output file (default <name>.gpmf; - for stdout)
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

* [gopro9 media](gopro9_media.md)	 - List, download, inspect and delete files on the SD card

