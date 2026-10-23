## gopro9 webcam

Use the camera as a USB webcam (MPEG-TS over UDP)

### Synopsis

Webcam mode streams MPEG-TS to this host on UDP port 8554. Start it, then open
the stream with "gopro9 stream play" or point ffplay/OBS at udp://@:8554.
"stop" pauses streaming but stays in webcam mode; "exit" returns the camera
to normal operation.

### Options

```
  -h, --help   help for webcam
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
* [gopro9 webcam exit](gopro9_webcam_exit.md)	 - Leave webcam mode
* [gopro9 webcam preview](gopro9_webcam_preview.md)	 - Start the low-power preview stream
* [gopro9 webcam start](gopro9_webcam_start.md)	 - Start streaming at full power
* [gopro9 webcam status](gopro9_webcam_status.md)	 - Show webcam state and last error
* [gopro9 webcam stop](gopro9_webcam_stop.md)	 - Stop streaming (stay in webcam mode)
* [gopro9 webcam version](gopro9_webcam_version.md)	 - Show the webcam feature version

