## gopro9 setting

Read and change camera settings with typed validation

### Synopsis

Settings are addressed by name (video-resolution, frames-per-second, ...) or
numeric id. Values accept the human label ("4K", "2.7K 4:3", "Linear") or the
numeric option. Only options the HERO9 supports are accepted; see
"gopro9 setting describe <name>".

### Options

```
  -h, --help   help for setting
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
* [gopro9 setting apply](gopro9_setting_apply.md)	 - Apply settings from a YAML map of name: value (as produced by `setting dump`)
* [gopro9 setting describe](gopro9_setting_describe.md)	 - Show a setting's id, title and the options the HERO9 accepts
* [gopro9 setting dump](gopro9_setting_dump.md)	 - Write all documented settings as YAML (name: value), suitable for `setting apply`
* [gopro9 setting get](gopro9_setting_get.md)	 - Show one setting's current value
* [gopro9 setting list](gopro9_setting_list.md)	 - Show current values of all documented settings
* [gopro9 setting set](gopro9_setting_set.md)	 - Change one setting (value may be a label or a number)

