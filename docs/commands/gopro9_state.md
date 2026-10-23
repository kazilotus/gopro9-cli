## gopro9 state

Dump the full typed camera state (every status and setting)

### Synopsis

Print every status and setting the camera reports, decoded into typed
values where the Open GoPro spec documents them. --watch repeats the query
and prints only the keys whose values changed.

```
gopro9 state [flags]
```

### Options

```
  -h, --help             help for state
      --raw              print the camera's raw JSON instead of the typed view
      --watch duration   re-query at this interval and print changed keys (0 = once)
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

* [gopro9](gopro9.md)	 - Control a GoPro HERO9 Black over USB-C

