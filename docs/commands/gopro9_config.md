## gopro9 config

Show or write the configuration file (flags > env > file > defaults)

### Options

```
  -h, --help   help for config
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
* [gopro9 config path](gopro9_config_path.md)	 - Print the configuration file path
* [gopro9 config set](gopro9_config_set.md)	 - Write the current global flags (--host, --port, --timeout, --output, --quiet) to the config file
* [gopro9 config show](gopro9_config_show.md)	 - Print the configuration file location and contents

