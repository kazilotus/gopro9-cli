## gopro9 doctor

Diagnose the USB link and check the camera against the capability matrix

### Synopsis

Runs connectivity checks (discovery, API version, Host-header guard, alternate
ports, mDNS) and then exercises every read-only operation to detect capability
drift between this firmware and the matrix shipped with gopro9.

Exit code is 69 when any check fails. --record DIR saves the probe responses
as scrubbed fixtures for the fake camera used in tests.

```
gopro9 doctor [flags]
```

### Options

```
  -h, --help            help for doctor
      --no-probe        skip the capability drift probes
      --record string   directory to write scrubbed response fixtures into
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

