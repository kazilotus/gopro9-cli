package gpmf

import (
	"math"
	"strings"
	"time"
)

// Stream is one sensor stream merged across every DEVC payload.
type Stream struct {
	Key   string    `json:"key" yaml:"key"`     // data FourCC, e.g. "GPS5"
	Name  string    `json:"name" yaml:"name"`   // STNM
	Units []string  `json:"units" yaml:"units"` // SIUN or UNIT, one per column when present
	Scale []float64 `json:"scale" yaml:"scale"` // SCAL divisors (one, or one per column)
	Type  string    `json:"type" yaml:"type"`   // TYPE string for complex samples, else the type character
	// Columns is the number of values per sample.
	Columns int `json:"columns" yaml:"columns"`
	// Samples holds scaled values. Non-numeric columns are NaN (see package doc).
	Samples [][]float64 `json:"samples" yaml:"samples"`
	// Text holds the verbatim string form of character/UTC samples (e.g. GPSU), one per sample.
	Text []string `json:"text,omitempty" yaml:"text,omitempty"`
	// Payloads is the cumulative sample count at the end of each DEVC payload that contained this stream.
	Payloads []int `json:"payloads" yaml:"payloads"`
	// Temperature is TMPC per payload, when the stream reports it.
	Temperature []float64 `json:"temperature,omitempty" yaml:"temperature,omitempty"`
}

// PayloadOf returns the index of the payload that sample i belongs to.
func (s *Stream) PayloadOf(i int) int {
	for p, end := range s.Payloads {
		if i < end {
			return p
		}
	}
	if len(s.Payloads) == 0 {
		return 0
	}
	return len(s.Payloads) - 1
}

// PayloadRange returns the [start, end) sample indexes of payload p.
func (s *Stream) PayloadRange(p int) (int, int) {
	if p < 0 || p >= len(s.Payloads) {
		return 0, 0
	}
	start := 0
	if p > 0 {
		start = s.Payloads[p-1]
	}
	return start, s.Payloads[p]
}

// Device identifies the recording device.
type Device struct {
	ID   uint32 `json:"id" yaml:"id"`
	Name string `json:"name" yaml:"name"`
}

// Telemetry is the merged content of a GPMF track.
type Telemetry struct {
	Device  Device             `json:"device" yaml:"device"`
	Streams map[string]*Stream `json:"streams" yaml:"streams"`
	// Order lists stream keys in first-seen order.
	Order []string `json:"order" yaml:"order"`
	// Payloads is the number of DEVC containers.
	Payloads int `json:"payloads" yaml:"payloads"`
	// Timing is the per-payload duration from the MP4 track; nil for raw GPMF,
	// where every payload is nominally one second.
	Timing *TrackTiming `json:"timing,omitempty" yaml:"timing,omitempty"`
	// Header is the raw GPMF file header (moov/udta/GPMF) when parsed from an MP4.
	Header []byte `json:"-" yaml:"-"`
}

// PayloadDuration returns the duration of payload p in seconds.
func (t *Telemetry) PayloadDuration(p int) float64 { return t.Timing.Seconds(p) }

// PayloadStart returns the start of payload p in seconds from the track start.
func (t *Telemetry) PayloadStart(p int) float64 { return t.Timing.Start(p) }

// Stream returns the stream with the given data key.
func (t *Telemetry) Stream(key string) (*Stream, bool) {
	s, ok := t.Streams[key]
	return s, ok
}

// metadataKeys are STRM children that describe the data item rather than being it.
var metadataKeys = map[string]bool{
	"STNM": true, "SIUN": true, "UNIT": true, "SCAL": true, "TYPE": true, "TSMP": true,
	"TMPC": true, "ORIN": true, "ORIO": true, "MTRX": true, "RMRK": true, "TIMO": true,
	"STMP": true, "TICK": true, "TOCK": true, "DVNM": true, "DVID": true, "STPS": true,
}

// ParseTelemetry parses a track and merges streams across payloads.
//
// data may be raw GPMF (a sequence of DEVC containers, as served by
// /gopro/media/gpmf) or an MP4 whose gpmd track samples are the payloads (as
// served by /gopro/media/telemetry). A parse error on a trailing payload still
// returns the telemetry assembled so far, together with the error.
func ParseTelemetry(data []byte) (*Telemetry, error) {
	t := &Telemetry{Streams: map[string]*Stream{}}
	if IsMP4(data) {
		samples, timing, err := ExtractFromMP4(data)
		if err != nil && len(samples) == 0 {
			return t, err
		}
		t.Timing = timing
		t.Header = HeaderFromMP4(data)
		var firstErr error
		for _, sample := range samples {
			items, perr := Parse(sample)
			if perr != nil && firstErr == nil {
				firstErr = perr
			}
			t.addPayloads(items)
		}
		if err == nil {
			err = firstErr
		}
		return t, err
	}
	items, perr := Parse(data)
	t.addPayloads(items)
	return t, perr
}

func (t *Telemetry) addPayloads(items []Item) {
	if t.Device.Name == "" && t.Device.ID == 0 {
		t.Device.ID, t.Device.Name = DeviceInfo(items)
	}
	for _, dev := range items {
		if dev.Key != "DEVC" {
			continue
		}
		t.Payloads++
		for _, strm := range dev.Children {
			if strm.Key != "STRM" {
				continue
			}
			t.addStream(strm)
		}
	}
}

// addStream merges one STRM container. The data item is the last child that
// is not descriptive metadata; any other non-metadata children (for example
// GPSU, GPSF and GPSP inside the GPS5 stream) become auxiliary single-sample
// streams of their own so callers can correlate them per payload.
func (t *Telemetry) addStream(strm Item) {
	var (
		name    string
		units   []string
		scale   []float64
		typeDef string
		tmpc    = math.NaN()
		extras  []*Item
	)
	for i := range strm.Children {
		c := &strm.Children[i]
		switch c.Key {
		case "STNM":
			name = c.String()
		case "SIUN", "UNIT":
			if len(units) == 0 {
				units = c.Strings()
			}
		case "SCAL":
			scale = flatten(c.Numbers(""))
		case "TYPE":
			typeDef = c.String()
		case "TMPC":
			if n := flatten(c.Numbers("")); len(n) > 0 {
				tmpc = n[0]
			}
		default:
			if !metadataKeys[c.Key] && c.Type != 0 {
				extras = append(extras, c)
			}
		}
	}
	if len(extras) == 0 {
		return
	}
	data := extras[len(extras)-1]
	t.appendSamples(data, name, units, scale, typeDef, tmpc)
	for _, aux := range extras[:len(extras)-1] {
		t.appendSamples(aux, "", nil, nil, "", math.NaN())
	}
}

func (t *Telemetry) appendSamples(data *Item, name string, units []string, scale []float64, typeDef string, tmpc float64) {
	s, ok := t.Streams[data.Key]
	if !ok {
		s = &Stream{Key: data.Key, Name: name, Units: units, Scale: scale, Type: typeDef}
		if s.Type == "" {
			s.Type = string(data.Type)
		}
		t.Streams[data.Key] = s
		t.Order = append(t.Order, data.Key)
	}
	var rows [][]float64
	if data.Type == 'c' {
		for _, txt := range data.Strings() {
			s.Text = append(s.Text, txt)
			rows = append(rows, []float64{math.NaN()})
		}
	} else {
		rows = data.Numbers(typeDef)
		if data.Type == 'U' {
			for r := 0; r < int(data.Repeat) && (r+1)*int(data.Size) <= len(data.Raw); r++ {
				s.Text = append(s.Text, strings.TrimRight(string(data.Raw[r*int(data.Size):(r+1)*int(data.Size)]), "\x00 "))
			}
		}
	}
	for _, row := range rows {
		for c := range row {
			if len(scale) > 0 {
				if d := scale[c%len(scale)]; d != 0 {
					row[c] /= d
				}
			}
		}
		if s.Columns < len(row) {
			s.Columns = len(row)
		}
	}
	s.Samples = append(s.Samples, rows...)
	s.Payloads = append(s.Payloads, len(s.Samples))
	if !math.IsNaN(tmpc) {
		s.Temperature = append(s.Temperature, tmpc)
	}
}

func flatten(rows [][]float64) []float64 {
	out := make([]float64, 0, len(rows))
	for _, r := range rows {
		out = append(out, r...)
	}
	return out
}

// GPSFix is one time-stamped GPS sample.
type GPSFix struct {
	Time      time.Time `json:"time" yaml:"time"`
	Lat       float64   `json:"lat" yaml:"lat"`
	Lon       float64   `json:"lon" yaml:"lon"`
	Alt       float64   `json:"alt" yaml:"alt"`
	Speed2D   float64   `json:"speed_2d" yaml:"speed_2d"`
	Speed3D   float64   `json:"speed_3d" yaml:"speed_3d"`
	Fix       uint32    `json:"fix" yaml:"fix"`
	Precision uint16    `json:"precision" yaml:"precision"`
}

// GPS derives fixes from GPS5. Each payload's samples are spread evenly over
// the payload's duration (from the MP4 track timing, else one second). The
// absolute time of a payload comes from its GPSU sample when present;
// otherwise from the track offset relative to the first GPSU seen, or it is
// left zero. GPSF and GPSP are applied per payload. Samples with Fix == 0 are
// dropped only when a GPSF stream exists.
func (t *Telemetry) GPS() []GPSFix {
	gps5, ok := t.Streams["GPS5"]
	if !ok {
		return nil
	}
	gpsu, gpsf, gpsp := t.Streams["GPSU"], t.Streams["GPSF"], t.Streams["GPSP"]
	// Payload indexes of gps5 are relative to the payloads that contained GPS5;
	// with a complete track they line up with the DEVC index, which the timing
	// refers to. Streams that skip payloads are rare, so this is a sound default.
	bases := t.payloadBases(gpsu, len(gps5.Payloads))
	fixes := make([]GPSFix, 0, len(gps5.Samples))
	for p := range gps5.Payloads {
		start, end := gps5.PayloadRange(p)
		n := end - start
		if n == 0 {
			continue
		}
		base := bases[p]
		fix, prec := perPayload(gpsf, p), perPayload(gpsp, p)
		if gpsf != nil && fix == 0 {
			continue
		}
		// Spread samples up to the next payload's timestamp when it is known,
		// which keeps the series monotonic; otherwise use the payload duration.
		span := time.Duration(t.PayloadDuration(p) * float64(time.Second))
		if p+1 < len(bases) && !base.IsZero() && bases[p+1].After(base) {
			span = bases[p+1].Sub(base)
		}
		step := span / time.Duration(n)
		for i := start; i < end; i++ {
			row := gps5.Samples[i]
			if len(row) < 5 {
				continue
			}
			f := GPSFix{Lat: row[0], Lon: row[1], Alt: row[2], Speed2D: row[3], Speed3D: row[4], Fix: uint32(fix), Precision: uint16(prec)}
			if !base.IsZero() {
				f.Time = base.Add(time.Duration(i-start) * step)
			}
			fixes = append(fixes, f)
		}
	}
	return fixes
}

// payloadBases returns the absolute start time of each payload: the GPSU
// sample when present, otherwise extrapolated from the track timing relative
// to the first GPSU seen. Entries stay zero when no GPSU exists at all.
func (t *Telemetry) payloadBases(gpsu *Stream, n int) []time.Time {
	bases := make([]time.Time, n)
	var epoch time.Time
	epochOffset := 0.0
	for p := range n {
		if gpsu != nil && p < len(gpsu.Text) {
			if ts, err := ParseUTC(gpsu.Text[p]); err == nil {
				bases[p] = ts
				if epoch.IsZero() {
					epoch, epochOffset = ts, t.PayloadStart(p)
				}
			}
		}
	}
	for p := range n {
		if bases[p].IsZero() && !epoch.IsZero() {
			bases[p] = epoch.Add(time.Duration((t.PayloadStart(p) - epochOffset) * float64(time.Second)))
		}
	}
	return bases
}

// perPayload returns the first value of the sample for payload p, or 0.
func perPayload(s *Stream, p int) float64 {
	if s == nil {
		return 0
	}
	start, end := s.PayloadRange(p)
	if end <= start || len(s.Samples[start]) == 0 {
		return 0
	}
	v := s.Samples[start][0]
	if math.IsNaN(v) {
		return 0
	}
	return v
}
