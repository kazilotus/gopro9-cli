## gopro9 media hilight remove

Remove a HiLight tag from an existing video at --at

```
gopro9 media hilight remove <DIR/NAME> [flags]
```

### Options

```
      --at duration   offset of the tag to remove
  -h, --help          help for remove
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

