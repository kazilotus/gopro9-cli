## gopro9 media download

Download media files, resumable, with bounded parallelism

### Synopsis

Download the given files (as "100GOPRO/GH010397.MP4") or, with --all, every
file matching the filters. Files are written under --out preserving the DCIM
directory. Partial files are resumed by default and sizes are verified
against the camera. Turbo transfer is enabled for the duration and restored.

```
gopro9 media download [DIR/NAME ...] [flags]
```

### Options

```
      --all            download every file matching the filters
      --dir string     only this DCIM directory (e.g. 100GOPRO)
      --group string   only chapters of this 4-digit capture group
  -h, --help           help for download
      --lrv            also fetch the low-resolution .LRV proxy of each video
      --no-resume      restart partial files instead of resuming
      --no-turbo       do not toggle turbo transfer
      --no-verify      skip the final size check
      --out string     destination directory (default ".")
      --overwrite      re-download files that already exist complete
      --parallel int   concurrent transfers (the camera degrades above 2) (default 2)
      --since string   only files modified after a duration ago (24h) or a date (2026-01-31)
      --thm            also fetch the .THM thumbnail of each file
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

