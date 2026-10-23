## gopro9 completion

Generate shell completion script

### Synopsis

Generate a completion script for your shell. Example: source <(gopro9 completion zsh)

```
gopro9 completion bash|zsh|fish|powershell [flags]
```

### Options

```
  -h, --help   help for completion
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

