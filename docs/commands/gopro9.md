## gopro9

Control a GoPro HERO9 Black over USB-C

### Synopsis

gopro9 drives a HERO9 Black through the Open GoPro HTTP API exposed over the
USB-C cable (GoPro Connect mode). It covers camera control, settings, presets,
media transfer, telemetry, webcam and preview streams, and GoPro Labs tooling.

Output is human-readable by default; pass --output json for scripting.
Exit codes follow sysexits(3): 0 ok, 64 usage, 65 bad data, 69 camera
unavailable or unsupported, 70 internal, 74 local I/O, 75 camera busy.

### Options

```
  -h, --help                           help for gopro9
      --host string                    camera IP or hostname (default: auto-discover on the USB link)
      --output table|json|yaml|plain   output format (default table)
      --port int                       camera HTTP port (default 8080)
  -q, --quiet                          suppress progress and informational output
      --timeout duration               per-request timeout (default 5s)
  -v, --verbose                        log HTTP requests to stderr
  -y, --yes                            assume yes for confirmations
```

### SEE ALSO

* [gopro9 capabilities](gopro9_capabilities.md)	 - Show what the HERO9 supports over USB (verified / spec / unsupported)
* [gopro9 capture](gopro9_capture.md)	 - Switch mode and capture in one step
* [gopro9 completion](gopro9_completion.md)	 - Generate shell completion script
* [gopro9 config](gopro9_config.md)	 - Show or write the configuration file (flags > env > file > defaults)
* [gopro9 datetime](gopro9_datetime.md)	 - Read or set the camera clock
* [gopro9 discover](gopro9_discover.md)	 - Find cameras on the USB link
* [gopro9 docs](gopro9_docs.md)	 - Generate command reference documentation
* [gopro9 doctor](gopro9_doctor.md)	 - Diagnose the USB link and check the camera against the capability matrix
* [gopro9 info](gopro9_info.md)	 - Show camera identity: model, firmware, serial, Open GoPro API version
* [gopro9 keepalive](gopro9_keepalive.md)	 - Send heartbeats so the camera does not sleep (Ctrl-C to stop)
* [gopro9 labs](gopro9_labs.md)	 - GoPro Labs command language: build, validate, explain and render QR codes
* [gopro9 media](gopro9_media.md)	 - List, download, inspect and delete files on the SD card
* [gopro9 power](gopro9_power.md)	 - Sleep the camera or trigger the locate beep
* [gopro9 preset](gopro9_preset.md)	 - List and activate presets and preset groups
* [gopro9 setting](gopro9_setting.md)	 - Read and change camera settings with typed validation
* [gopro9 shutter](gopro9_shutter.md)	 - Press the shutter in the current mode
* [gopro9 state](gopro9_state.md)	 - Dump the full typed camera state (every status and setting)
* [gopro9 status](gopro9_status.md)	 - One-screen camera summary: battery, SD card, mode, recording state
* [gopro9 stream](gopro9_stream.md)	 - Receive the preview or webcam stream: play it, record it, or just start/stop it
* [gopro9 turbo](gopro9_turbo.md)	 - Toggle turbo transfer (prioritises USB bandwidth for downloads)
* [gopro9 version](gopro9_version.md)	 - Print gopro9 build information
* [gopro9 webcam](gopro9_webcam.md)	 - Use the camera as a USB webcam (MPEG-TS over UDP)
* [gopro9 zoom](gopro9_zoom.md)	 - Set digital zoom as a percentage

