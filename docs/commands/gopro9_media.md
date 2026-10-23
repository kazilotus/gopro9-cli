## gopro9 media

List, download, inspect and delete files on the SD card

### Options

```
  -h, --help   help for media
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
* [gopro9 media delete](gopro9_media_delete.md)	 - Delete files from the SD card (requires --yes)
* [gopro9 media download](gopro9_media_download.md)	 - Download media files, resumable, with bounded parallelism
* [gopro9 media gpmf](gopro9_media_gpmf.md)	 - Save a video's raw GPMF metadata header
* [gopro9 media hilight](gopro9_media_hilight.md)	 - Add or remove HiLight tags
* [gopro9 media list](gopro9_media_list.md)	 - List media files (videos, photos) with sizes and chapter groups
* [gopro9 media screennail](gopro9_media_screennail.md)	 - Save a file's larger JPEG preview
* [gopro9 media telemetry](gopro9_media_telemetry.md)	 - Extract a video's telemetry (GPS, accelerometer, gyro) without downloading the video
* [gopro9 media thumbnail](gopro9_media_thumbnail.md)	 - Save a file's JPEG thumbnail
* [gopro9 media tree](gopro9_media_tree.md)	 - Walk the raw DCIM file server (shows LRV/THM proxies and camera databases)

