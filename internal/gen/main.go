// Command gen produces typed Go code and docs from the vendored Open GoPro
// OpenAPI spec and the HERO9 override table.
//
// Usage: go run ./internal/gen -root .
package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
)

func main() {
	root := flag.String("root", ".", "repository root")
	check := flag.Bool("check", false, "do not write; exit 1 if any output would change")
	flag.Parse()
	if err := run(*root, *check); err != nil {
		fmt.Fprintln(os.Stderr, "gen:", err)
		os.Exit(1)
	}
}

func run(root string, check bool) error {
	spec, err := loadSpec(filepath.Join(root, "spec", "opengopro", "openapi.json"))
	if err != nil {
		return err
	}
	ov, err := loadOverrides(filepath.Join(root, "spec", "hero9_overrides.yaml"))
	if err != nil {
		return err
	}
	m, err := buildModel(spec, ov)
	if err != nil {
		return err
	}
	outputs := map[string]string{
		filepath.Join(root, "pkg", "gopro", "settings", "zz_settings.go"): emitSettings(m),
		filepath.Join(root, "pkg", "gopro", "status", "zz_status.go"):     emitStatus(m),
		filepath.Join(root, "pkg", "gopro", "capability", "zz_matrix.go"): emitCapability(m),
		filepath.Join(root, "docs", "CAPABILITIES.md"):                    emitCapabilityDoc(m),
	}
	stale := 0
	for path, content := range outputs {
		data := []byte(content)
		if filepath.Ext(path) == ".go" {
			formatted, ferr := format.Source(data)
			if ferr != nil {
				_ = os.WriteFile(path+".broken", data, 0o600)
				return fmt.Errorf("format %s: %w (raw output saved to %s.broken)", path, ferr, path)
			}
			data = formatted
		}
		if check {
			old, rerr := os.ReadFile(path)
			if rerr != nil || !bytes.Equal(old, data) {
				fmt.Fprintln(os.Stderr, "stale:", path)
				stale++
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			return err
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			return err
		}
		fmt.Println("wrote", path)
	}
	if stale > 0 {
		return fmt.Errorf("%d generated file(s) out of date; run `go generate ./...`", stale)
	}
	return nil
}
