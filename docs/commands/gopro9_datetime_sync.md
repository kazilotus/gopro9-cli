## gopro9 datetime sync

Set the camera clock to this host's current local time

```
gopro9 datetime sync [flags]
```

### Options

```
      --force   attempt the operation even if the capability matrix marks it unsupported
  -h, --help    help for sync
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

