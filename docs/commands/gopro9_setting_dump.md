## gopro9 setting dump

Write all documented settings as YAML (name: value), suitable for `setting apply`

```
gopro9 setting dump [flags]
```

### Options

```
  -h, --help   help for dump
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

