## gopro9 power sleep

Put the camera to sleep (wake it with the mode button or power cycle)

```
gopro9 power sleep [flags]
```

### Options

```
      --force   attempt the operation even if the capability matrix marks it unsupported
  -h, --help    help for sleep
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

* [gopro9 power](gopro9_power.md)	 - Sleep the camera or trigger the locate beep

