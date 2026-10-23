## gopro9 datetime set

Set the camera clock to an explicit local time

```
gopro9 datetime set <RFC3339|YYYY-MM-DDTHH:MM:SS> [flags]
```

### Options

```
      --force   attempt the operation even if the capability matrix marks it unsupported
  -h, --help    help for set
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

* [gopro9 datetime](gopro9_datetime.md)	 - Read or set the camera clock

