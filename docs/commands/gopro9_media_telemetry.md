## gopro9 media telemetry

Extract a video's telemetry (GPS, accelerometer, gyro) without downloading the video

### Synopsis

Fetches the camera's telemetry track for a video and decodes the GPMF streams.

Formats:
  summary  one row per stream (default)
  gpx      GPS track as GPX 1.1 (from GPS5 + GPSU)
  csv      one stream as CSV (--stream, default GPS5)
  json     every stream with scaled samples
  raw      the camera's bytes untouched (an MP4 holding the gpmd track)

--from FILE decodes a previously saved raw telemetry file instead of asking
the camera, so no connection is needed.

```
gopro9 media telemetry <DIR/NAME> [flags]
```

### Examples

```
  gopro9 media telemetry 100GOPRO/GH010516.MP4 --format gpx --out ride.gpx
  gopro9 media telemetry 100GOPRO/GH010516.MP4 --format csv --stream ACCL --out accl.csv
  gopro9 media telemetry --from GH010516.telemetry.mp4 --format summary
```

### Options

```
      --format string   summary | gpx | csv | json | raw (default "summary")
      --from string     decode this saved telemetry file instead of fetching from the camera
  -h, --help            help for telemetry
      --out string      output file (default: stdout for text formats, <name>.telemetry.mp4 for raw)
      --stream string   stream FourCC for --format csv (GPS5, ACCL, GYRO, SHUT, ...) (default "GPS5")
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

* [gopro9 media](gopro9_media.md)	 - List, download, inspect and delete files on the SD card

