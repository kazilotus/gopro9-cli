## gopro9 media list

List media files (videos, photos) with sizes and chapter groups

```
gopro9 media list [flags]
```

### Options

```
      --dir string     only this DCIM directory (e.g. 100GOPRO)
      --group string   only chapters of this 4-digit capture group
      --grouped        collapse chapters into one row per recording
  -h, --help           help for list
      --since string   only files modified after a duration ago (24h) or a date (2026-01-31)
      --type string    comma-separated kinds: video, photo, raw
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

