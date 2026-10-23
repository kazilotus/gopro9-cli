## gopro9 capture timelapse

Run the time-lapse preset for a fixed duration

```
gopro9 capture timelapse [flags]
```

### Options

```
      --duration duration   how long to run the time-lapse (default 1m0s)
  -h, --help                help for timelapse
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

* [gopro9 capture](gopro9_capture.md)	 - Switch mode and capture in one step

