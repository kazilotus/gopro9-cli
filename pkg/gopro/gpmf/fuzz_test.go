package gpmf

import (
	"bytes"
	"testing"
)

func FuzzParse(f *testing.F) {
	f.Add(track(2))
	f.Add(klv("ACCL", 's', 6, 2, make([]byte, 12)))
	f.Add(nested("DEVC", klv("DVID", 'L', 4, 1, []byte{0, 0, 0, 1}), text("DVNM", "Fake HERO9  ")))
	f.Add([]byte("DEVC\x00\x01\xff\xff"))
	f.Add([]byte{})
	f.Add(buildMP4([][]byte{payloadN(0, 3)}, 996, false, "meta", "gpmd"))
	f.Fuzz(func(t *testing.T, data []byte) {
		items, _ := Parse(data) // must never panic
		for _, it := range items {
			_ = it.Numbers("")
			_ = it.Numbers("cL")
			_ = it.Strings()
			_ = it.String()
		}
		_, _, _ = ExtractFromMP4(data)
		_ = HeaderFromMP4(data)
		tel, _ := ParseTelemetry(data)
		fixes := tel.GPS()
		var buf bytes.Buffer
		_ = WriteGPX(&buf, fixes, "fuzz")
		for _, s := range tel.Streams {
			_ = WriteCSV(&buf, s)
		}
	})
}
