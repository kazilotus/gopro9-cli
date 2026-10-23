## gopro9 stream play

Start the stream and open it in ffplay, mpv or vlc

```
gopro9 stream play [flags]
```

### Options

```
  -h, --help               help for play
      --player string      player binary: ffplay, mpv or vlc (default: first found)
      --res 480|720|1080   webcam resolution when --webcam is set
      --webcam             use the full-resolution webcam stream instead of the preview
```

### Options inherited from parent commands

```
      --host string                    camera IP or hostname (default: auto-discover on the USB link)
      --output table|json|yaml|plain   output format (default table)
      --port int                       camera HTTP port (default 8080)
  -q, --quiet                          suppress progress and informational output
      --timeout duration               per-request timeout (default 5s)
      --udp-port int                   UDP port on this host that receives the stream (default 8554)
  -v, --verbose                        log HTTP requests to stderr
  -y, --yes                            assume yes for confirmations
```

### SEE ALSO

* [gopro9 stream](gopro9_stream.md)	 - Receive the preview or webcam stream: play it, record it, or just start/stop it

