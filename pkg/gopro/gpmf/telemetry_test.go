package gpmf

import (
	"bytes"
	"encoding/csv"
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTelemetry(t *testing.T) {
	t.Parallel()
	tel, err := ParseTelemetry(track(3))
	require.NoError(t, err)
	assert.Equal(t, uint32(1), tel.Device.ID)
	assert.Equal(t, "Hero9 Black", tel.Device.Name)
	assert.Equal(t, 3, tel.Payloads)
	assert.Equal(t, []string{"ACCL", "GPS5", "GPSU", "GPSF", "GPSP", "SHUT"}, tel.Order)

	accl, ok := tel.Stream("ACCL")
	require.True(t, ok)
	assert.Equal(t, "Accelerometer", accl.Name)
	assert.Equal(t, []string{"m/s\xb2"}, accl.Units)
	assert.Equal(t, []float64{418}, accl.Scale)
	assert.Equal(t, "s", accl.Type)
	assert.Equal(t, 3, accl.Columns)
	require.Len(t, accl.Samples, 6)
	assert.InDelta(t, 1.0, accl.Samples[0][0], 1e-9)
	assert.InDelta(t, 2.0, accl.Samples[0][1], 1e-9)
	assert.InDelta(t, -1.0, accl.Samples[0][2], 1e-9)
	assert.InDelta(t, 3.0, accl.Samples[1][2], 1e-9)
	assert.Equal(t, []int{2, 4, 6}, accl.Payloads)
	assert.Equal(t, []float64{30.5, 30.5, 30.5}, accl.Temperature)
	assert.Equal(t, 0, accl.PayloadOf(1))
	assert.Equal(t, 1, accl.PayloadOf(2))
	assert.Equal(t, 2, accl.PayloadOf(99))
	start, end := accl.PayloadRange(1)
	assert.Equal(t, 2, start)
	assert.Equal(t, 4, end)
	start, end = accl.PayloadRange(7)
	assert.Zero(t, start)
	assert.Zero(t, end)

	gps, ok := tel.Stream("GPS5")
	require.True(t, ok)
	assert.Equal(t, 5, gps.Columns)
	assert.Equal(t, []string{"deg", "deg", "m", "m/s", "m/s"}, gps.Units)
	assert.InDelta(t, 32.707, gps.Samples[0][0], 1e-6)
	assert.InDelta(t, -117.1576, gps.Samples[0][1], 1e-6)
	assert.InDelta(t, 12.345, gps.Samples[0][2], 1e-9)
	assert.InDelta(t, 5.0, gps.Samples[0][3], 1e-9)
	assert.InDelta(t, 5.2, gps.Samples[0][4], 1e-9)

	gpsu, ok := tel.Stream("GPSU")
	require.True(t, ok)
	assert.Equal(t, []string{"260123120000.500", "260123120000.500", "260123120000.500"}, gpsu.Text)
	assert.Equal(t, "U", gpsu.Type)

	shut, ok := tel.Stream("SHUT")
	require.True(t, ok)
	assert.InDelta(t, 0.0025, shut.Samples[2][0], 1e-9)
	assert.Empty(t, shut.Scale)

	_, ok = tel.Stream("NOPE")
	assert.False(t, ok)
}

func TestParseTelemetryTolerant(t *testing.T) {
	t.Parallel()
	data := track(2)
	tel, err := ParseTelemetry(data[:len(data)-30])
	require.ErrorIs(t, err, ErrTruncated)
	assert.Equal(t, 1, tel.Payloads, "the truncated DEVC is dropped, the complete one kept")
	accl, ok := tel.Stream("ACCL")
	require.True(t, ok)
	assert.Len(t, accl.Samples, 2)

	// A STRM without a data item and a character data stream.
	tel, err = ParseTelemetry(nested("DEVC",
		nested("STRM", text("STNM", "empty")),
		nested("STRM", text("STNM", "names"), klv("NAME", 'c', 4, 2, []byte("abcdefgh"))),
	))
	require.NoError(t, err)
	assert.Equal(t, []string{"NAME"}, tel.Order)
	assert.Equal(t, []string{"abcd", "efgh"}, tel.Streams["NAME"].Text)
	assert.Len(t, tel.Streams["NAME"].Samples, 2)
}

func TestGPS(t *testing.T) {
	t.Parallel()
	tel, err := ParseTelemetry(track(3))
	require.NoError(t, err)
	fixes := tel.GPS()
	require.Len(t, fixes, 4, "payload 1 has fix 0 and is dropped")
	want := time.Date(2026, 1, 23, 12, 0, 0, 500_000_000, time.UTC)
	assert.True(t, fixes[0].Time.Equal(want), fixes[0].Time)
	assert.True(t, fixes[1].Time.Equal(want.Add(500*time.Millisecond)))
	assert.Equal(t, uint32(3), fixes[0].Fix)
	assert.Equal(t, uint16(500), fixes[0].Precision)
	assert.InDelta(t, 32.7070002, fixes[2].Lat, 1e-7, "third payload samples")
	assert.InDelta(t, 5.2, fixes[0].Speed3D, 1e-9)

	// Without GPSF, nothing is dropped; without GPSU, times are zero.
	bare := nested("DEVC", strm(cat(text("STNM", "gps"), klv("SCAL", 'l', 4, 5, cat(be32(1), be32(1), be32(1), be32(1), be32(1)))),
		klv("GPS5", 'l', 20, 1, cat(be32(1), be32(2), be32(3), be32(4), be32(5)))))
	tel, err = ParseTelemetry(bare)
	require.NoError(t, err)
	fixes = tel.GPS()
	require.Len(t, fixes, 1)
	assert.True(t, fixes[0].Time.IsZero())
	assert.InDelta(t, 2, fixes[0].Lon, 0)
	assert.Zero(t, fixes[0].Fix)

	// Short rows are skipped; no GPS5 at all yields nil.
	short := nested("DEVC", strm(text("STNM", "gps"), klv("GPS5", 'l', 8, 1, cat(be32(1), be32(2)))))
	tel, err = ParseTelemetry(short)
	require.NoError(t, err)
	assert.Empty(t, tel.GPS())
	assert.Nil(t, (&Telemetry{Streams: map[string]*Stream{}}).GPS())
	assert.Zero(t, perPayload(nil, 0))
	assert.Zero(t, perPayload(&Stream{Payloads: []int{0}}, 0))
}

func TestWriteGPX(t *testing.T) {
	t.Parallel()
	tel, err := ParseTelemetry(track(2))
	require.NoError(t, err)
	var buf bytes.Buffer
	require.NoError(t, WriteGPX(&buf, tel.GPS(), "GH010001"))
	out := buf.String()
	assert.True(t, strings.HasPrefix(out, xml.Header))
	assert.Contains(t, out, `<gpx version="1.1" creator="gopro9-cli" xmlns="http://www.topografix.com/GPX/1/1">`)
	assert.Contains(t, out, "<trkpt lat=")
	assert.Contains(t, out, "<time>2026-01-23T12:00:00.5Z</time>")
	assert.Contains(t, out, "<speed>5</speed>")
	assert.Contains(t, out, "<dop>5</dop>")
	var doc struct {
		Trk struct {
			Name   string `xml:"name"`
			Trkseg struct {
				Trkpt []struct {
					Lat float64 `xml:"lat,attr"`
					Ele float64 `xml:"ele"`
				} `xml:"trkpt"`
			} `xml:"trkseg"`
		} `xml:"trk"`
	}
	require.NoError(t, xml.Unmarshal(buf.Bytes(), &doc))
	assert.Equal(t, "GH010001", doc.Trk.Name)
	assert.Len(t, doc.Trk.Trkseg.Trkpt, 2)
	assert.InDelta(t, 32.707, doc.Trk.Trkseg.Trkpt[0].Lat, 1e-6)

	// Zero time is omitted.
	buf.Reset()
	require.NoError(t, WriteGPX(&buf, []GPSFix{{Lat: 1, Lon: 2}}, "x"))
	assert.NotContains(t, buf.String(), "<time>")
	require.Error(t, WriteGPX(failWriter{}, nil, "x"))
}

func TestWriteCSV(t *testing.T) {
	t.Parallel()
	tel, err := ParseTelemetry(track(2))
	require.NoError(t, err)
	var buf bytes.Buffer
	gps, _ := tel.Stream("GPS5")
	require.NoError(t, WriteCSV(&buf, gps))
	rows, err := csv.NewReader(&buf).ReadAll()
	require.NoError(t, err)
	require.Len(t, rows, 5)
	assert.Equal(t, []string{"payload", "index", "deg", "deg", "m", "m/s", "m/s"}, rows[0])
	assert.Equal(t, "0", rows[1][0])
	assert.Equal(t, "1", rows[3][0], "third sample belongs to payload 1")
	assert.Equal(t, "32.707", rows[1][2])

	// Single unit is suffixed per column; NaN and text handled.
	buf.Reset()
	accl, _ := tel.Stream("ACCL")
	require.NoError(t, WriteCSV(&buf, accl))
	rows, err = csv.NewReader(&buf).ReadAll()
	require.NoError(t, err)
	assert.Equal(t, []string{"payload", "index", "m/s\xb2_0", "m/s\xb2_1", "m/s\xb2_2"}, rows[0])

	buf.Reset()
	gpsu, _ := tel.Stream("GPSU")
	require.NoError(t, WriteCSV(&buf, gpsu))
	rows, err = csv.NewReader(&buf).ReadAll()
	require.NoError(t, err)
	assert.Equal(t, []string{"payload", "index", "col0", "text"}, rows[0])
	assert.Equal(t, []string{"0", "0", "1769169600.5", "260123120000.500"}, rows[1])

	buf.Reset()
	require.NoError(t, WriteCSV(&buf, &Stream{Columns: 2, Samples: [][]float64{{1}}, Text: []string{"a", "b"}}))
	rows, err = csv.NewReader(&buf).ReadAll()
	require.NoError(t, err)
	assert.Equal(t, []string{"0", "0", "1", "", "a"}, rows[1])
	require.Error(t, WriteCSV(failWriter{}, gps))
}

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, assert.AnError }

func realFile(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "gpmf", name))
	require.NoError(t, err, "real-camera fixture must be present")
	return data
}

func TestRealHeaderBlob(t *testing.T) {
	t.Parallel()
	data := realFile(t, "gpmf_GH050365.bin")
	assert.False(t, IsMP4(data))
	items, err := Parse(data)
	require.NoError(t, err)
	require.NotEmpty(t, items)
	assert.Equal(t, "DEVC", items[0].Key)
	id, name := DeviceInfo(items)
	assert.Equal(t, uint32(1), id)
	assert.Equal(t, "Global Settings", name)
	fw, ok := items[0].Find("FMWR")
	require.True(t, ok)
	assert.Equal(t, "HD9.01.01.70.00", fw.String())
}

func TestRealTelemetryMP4(t *testing.T) {
	t.Parallel()
	data := realFile(t, "telemetry_GH050365.bin")
	require.True(t, IsMP4(data))

	samples, timing, err := ExtractFromMP4(data)
	require.NoError(t, err)
	assert.Len(t, samples, 178)
	assert.Equal(t, uint32(1000), timing.Timescale)
	assert.Len(t, timing.Durations, 178)
	assert.InDelta(t, 0.996, timing.Seconds(0), 1e-9)
	assert.InDelta(t, 0.996*5, timing.Start(5), 1e-9)

	header := HeaderFromMP4(data)
	require.NotEmpty(t, header)
	assert.Equal(t, realFile(t, "gpmf_GH050365.bin"), header, "udta/GPMF equals the /gopro/media/gpmf blob")

	tel, err := ParseTelemetry(data)
	require.NoError(t, err)
	assert.Equal(t, 178, tel.Payloads)
	assert.Equal(t, "HERO9 Black", tel.Device.Name)
	assert.NotEmpty(t, tel.Header)
	for _, key := range []string{"GPS5", "ACCL", "GYRO"} {
		s, ok := tel.Stream(key)
		require.True(t, ok, key)
		assert.NotEmpty(t, s.Samples, key)
		assert.Len(t, s.Payloads, 178, key)
	}
	accl, _ := tel.Stream("ACCL")
	assert.Equal(t, 3, accl.Columns)
	assert.Greater(t, len(accl.Samples), 178*100, "~200 Hz accelerometer")

	fixes := tel.GPS()
	require.NotEmpty(t, fixes)
	for i, f := range fixes {
		assert.GreaterOrEqual(t, f.Lat, -90.0)
		assert.LessOrEqual(t, f.Lat, 90.0)
		assert.GreaterOrEqual(t, f.Lon, -180.0)
		assert.LessOrEqual(t, f.Lon, 180.0)
		assert.False(t, f.Time.IsZero())
		if i > 0 {
			assert.False(t, f.Time.Before(fixes[i-1].Time), "times must be non-decreasing at %d", i)
		}
	}
	t.Logf("device %q payloads=%d fixes=%d first=%s last=%s", tel.Device.Name, tel.Payloads, len(fixes), fixes[0].Time, fixes[len(fixes)-1].Time)

	var buf bytes.Buffer
	require.NoError(t, WriteGPX(&buf, fixes, "GH050365"))
	var doc struct {
		Trk struct {
			Trkseg struct {
				Trkpt []struct{} `xml:"trkpt"`
			} `xml:"trkseg"`
		} `xml:"trk"`
	}
	require.NoError(t, xml.Unmarshal(buf.Bytes(), &doc))
	assert.Len(t, doc.Trk.Trkseg.Trkpt, len(fixes))

	buf.Reset()
	gyro, _ := tel.Stream("GYRO")
	require.NoError(t, WriteCSV(&buf, gyro))
	rows, err := csv.NewReader(&buf).ReadAll()
	require.NoError(t, err)
	assert.Len(t, rows, len(gyro.Samples)+1)
}
