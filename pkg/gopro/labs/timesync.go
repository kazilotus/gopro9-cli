package labs

import (
	"fmt"
	"time"
)

// TimeSyncCommand returns the Labs command that sets the camera clock.
//
// Format: oTyymmddhhmmss, optionally with ".sss" milliseconds when precision
// is requested (the Labs "precision time" QR). The camera stores local wall
// time with no timezone, so t is rendered in its own location: pass a local
// time for a camera that should show local time, or t.UTC() for UTC. The
// milliseconds let the camera compensate for the moment the QR is scanned.
func TimeSyncCommand(t time.Time, precision bool) string {
	s := "oT" + t.Format("060102150405")
	if precision {
		s += fmt.Sprintf(".%03d", t.Nanosecond()/int(time.Millisecond))
	}
	return s
}
