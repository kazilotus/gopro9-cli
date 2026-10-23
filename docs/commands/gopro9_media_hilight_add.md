## gopro9 media hilight add

Add a HiLight tag to an existing video at --at

```
gopro9 media hilight add <DIR/NAME> [flags]
```

### Options

```
      --at duration   offset into the video (e.g. 12.5s)
  -h, --help          help for add
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

* [gopro9 media hilight](gopro9_media_hilight.md)	 - Add or remove HiLight tags

