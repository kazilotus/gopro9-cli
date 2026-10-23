## gopro9 webcam start

Start streaming at full power

```
gopro9 webcam start [flags]
```

### Options

```
      --fov wide|narrow|superview|linear   digital lens (applied by stop → set lens → start, the only sequence the HERO9 honours) (default wide)
  -h, --help                               help for start
      --res 480|720|1080                   output resolution (default: camera's choice)
      --udp-port int                       UDP port on this host the camera should stream to (default 8554)
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

* [gopro9 webcam](gopro9_webcam.md)	 - Use the camera as a USB webcam (MPEG-TS over UDP)

