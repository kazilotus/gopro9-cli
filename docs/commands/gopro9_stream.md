## gopro9 stream

Receive the preview or webcam stream: play it, record it, or just start/stop it

### Synopsis

The camera pushes MPEG-TS over UDP to this host. "start"/"stop" only toggle the
low-resolution preview stream on the camera. "play" and "record" additionally
open the UDP port here, start the stream, and stop it again when done.
Use --webcam to drive the full-resolution webcam stream instead of the preview.

### Options

```
  -h, --help           help for stream
      --udp-port int   UDP port on this host that receives the stream (default 8554)
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
* [gopro9 stream play](gopro9_stream_play.md)	 - Start the stream and open it in ffplay, mpv or vlc
* [gopro9 stream record](gopro9_stream_record.md)	 - Start the stream and save the raw MPEG-TS to a file
* [gopro9 stream start](gopro9_stream_start.md)	 - Ask the camera to start the preview stream
* [gopro9 stream stop](gopro9_stream_stop.md)	 - Ask the camera to stop the preview stream

