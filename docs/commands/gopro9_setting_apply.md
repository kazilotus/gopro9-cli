## gopro9 setting apply

Apply settings from a YAML map of name: value (as produced by `setting dump`)

### Synopsis

Applies every entry in order of setting id. Settings the camera reports as
not applicable in its current mode (for example time-lapse rates while in
video mode) are skipped and listed on stderr; pass --strict to fail instead.

```
gopro9 setting apply <file.yaml|-> [flags]
```

### Options

```
  -h, --help     help for apply
      --strict   fail when the camera rejects any setting instead of skipping inapplicable ones
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

* [gopro9 setting](gopro9_setting.md)	 - Read and change camera settings with typed validation

