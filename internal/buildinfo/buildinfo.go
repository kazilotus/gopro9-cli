// Package buildinfo exposes version metadata injected at build time.
package buildinfo

import "runtime/debug"

// Set by -ldflags at release time; defaults describe a development build.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// Info is the resolved build information.
type Info struct {
	Version   string `json:"version" yaml:"version"`
	Commit    string `json:"commit" yaml:"commit"`
	Date      string `json:"date" yaml:"date"`
	GoVersion string `json:"go_version" yaml:"go_version"`
	Modified  bool   `json:"modified" yaml:"modified"`
}

// Get returns build information, falling back to debug.ReadBuildInfo for
// `go install` builds that carry no ldflags.
func Get() Info {
	info := Info{Version: Version, Commit: Commit, Date: Date}
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return info
	}
	info.GoVersion = bi.GoVersion
	if info.Version == "dev" && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		info.Version = bi.Main.Version
	}
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			if info.Commit == "none" {
				info.Commit = s.Value
			}
		case "vcs.time":
			if info.Date == "unknown" {
				info.Date = s.Value
			}
		case "vcs.modified":
			info.Modified = s.Value == "true"
		}
	}
	return info
}
