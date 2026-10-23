// Package exitcode defines process exit codes following sysexits(3).
package exitcode

// Code is a process exit status.
type Code int

// Exit codes. Values follow <sysexits.h> so shell users get familiar semantics.
const (
	OK          Code = 0
	Usage       Code = 64 // command line usage error
	DataErr     Code = 65 // camera returned data we could not interpret
	Unavailable Code = 69 // camera unreachable or operation unsupported on this firmware
	Software    Code = 70 // internal error (bug)
	IOErr       Code = 74 // local file I/O error
	TempFail    Code = 75 // camera busy; retry later
	Config      Code = 78 // configuration error
	Interrupted Code = 130
)

// String returns the symbolic name of the code.
func (c Code) String() string {
	switch c {
	case OK:
		return "ok"
	case Usage:
		return "usage"
	case DataErr:
		return "data"
	case Unavailable:
		return "unavailable"
	case Software:
		return "software"
	case IOErr:
		return "io"
	case TempFail:
		return "tempfail"
	case Config:
		return "config"
	case Interrupted:
		return "interrupted"
	default:
		return "unknown"
	}
}
