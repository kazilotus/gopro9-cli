package exitcode

import "testing"

func TestString(t *testing.T) {
	t.Parallel()
	cases := map[Code]string{
		OK: "ok", Usage: "usage", DataErr: "data", Unavailable: "unavailable",
		Software: "software", IOErr: "io", TempFail: "tempfail", Config: "config",
		Interrupted: "interrupted", Code(1): "unknown",
	}
	for c, want := range cases {
		if got := c.String(); got != want {
			t.Errorf("Code(%d).String() = %q, want %q", int(c), got, want)
		}
	}
}
