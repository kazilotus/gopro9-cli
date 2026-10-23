package gpmf

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// ErrNoTrack is returned when an MP4 holds no GoPro metadata (gpmd) track.
var ErrNoTrack = errors.New("gpmf: no GPMF metadata track in container")

// TrackTiming describes the metadata track's sample timing.
type TrackTiming struct {
	// Timescale is the number of units per second in Durations.
	Timescale uint32 `json:"timescale" yaml:"timescale"`
	// Durations is the duration of each sample (one DEVC payload) in timescale units.
	Durations []uint32 `json:"durations" yaml:"durations"`
}

// Seconds returns the duration of sample i in seconds, defaulting to 1 s when unknown.
func (tt *TrackTiming) Seconds(i int) float64 {
	if tt == nil || tt.Timescale == 0 || i < 0 || i >= len(tt.Durations) {
		return 1
	}
	return float64(tt.Durations[i]) / float64(tt.Timescale)
}

// Start returns the start offset of sample i in seconds from the track start.
func (tt *TrackTiming) Start(i int) float64 {
	if tt == nil || tt.Timescale == 0 {
		return float64(i)
	}
	var sum uint64
	for j := 0; j < i && j < len(tt.Durations); j++ {
		sum += uint64(tt.Durations[j])
	}
	return float64(sum) / float64(tt.Timescale)
}

// IsMP4 reports whether data starts with an ISO-BMFF box (an `ftyp` at offset 4).
func IsMP4(data []byte) bool {
	return len(data) >= 12 && string(data[4:8]) == "ftyp"
}

type box struct {
	typ        string
	start, end int // payload bounds within the buffer
}

// boxes iterates the boxes in data[start:end].
func boxes(data []byte, start, end int) ([]box, error) {
	var out []box
	off := start
	for off+8 <= end {
		size := int(binary.BigEndian.Uint32(data[off : off+4]))
		typ := string(data[off+4 : off+8])
		hdr := 8
		switch size {
		case 0:
			size = end - off
		case 1:
			if off+16 > end {
				return out, fmt.Errorf("%w: truncated largesize box %s", ErrInvalid, typ)
			}
			size64 := binary.BigEndian.Uint64(data[off+8 : off+16])
			if size64 > uint64(end-off) {
				return out, fmt.Errorf("%w: box %s size %d exceeds buffer", ErrTruncated, typ, size64)
			}
			size = int(size64)
			hdr = 16
		}
		if size < hdr || off+size > end {
			return out, fmt.Errorf("%w: box %s size %d exceeds buffer", ErrTruncated, typ, size)
		}
		out = append(out, box{typ: typ, start: off + hdr, end: off + size})
		off += size
	}
	return out, nil
}

func child(data []byte, b box, path ...string) (box, bool) {
	cur := b
	for _, want := range path {
		kids, _ := boxes(data, cur.start, cur.end)
		found := false
		for _, k := range kids {
			if k.typ == want {
				cur, found = k, true
				break
			}
		}
		if !found {
			return box{}, false
		}
	}
	return cur, true
}

// HeaderFromMP4 returns the raw GPMF payload of the moov/udta/GPMF box, or nil.
func HeaderFromMP4(data []byte) []byte {
	top, _ := boxes(data, 0, len(data))
	for _, b := range top {
		if b.typ != "moov" {
			continue
		}
		if g, ok := child(data, b, "udta", "GPMF"); ok {
			return data[g.start:g.end]
		}
	}
	return nil
}

// ExtractFromMP4 returns the samples of the GoPro metadata track in order,
// each one a DEVC payload, together with the track's sample timing. The track
// is the first whose hdlr handler type is "meta" or whose sample description
// FourCC is "gpmd".
func ExtractFromMP4(data []byte) ([][]byte, *TrackTiming, error) {
	top, err := boxes(data, 0, len(data))
	if err != nil {
		return nil, nil, err
	}
	for _, b := range top {
		if b.typ != "moov" {
			continue
		}
		kids, _ := boxes(data, b.start, b.end)
		for _, trak := range kids {
			if trak.typ != "trak" || !isMetaTrack(data, trak) {
				continue
			}
			return readTrack(data, trak)
		}
	}
	return nil, nil, ErrNoTrack
}

func isMetaTrack(data []byte, trak box) bool {
	if h, ok := child(data, trak, "mdia", "hdlr"); ok && h.end-h.start >= 12 && string(data[h.start+8:h.start+12]) == "meta" {
		return true
	}
	if sd, ok := child(data, trak, "mdia", "minf", "stbl", "stsd"); ok && sd.end-sd.start >= 16 && string(data[sd.start+12:sd.start+16]) == "gpmd" {
		return true
	}
	return false
}

func readTrack(data []byte, trak box) ([][]byte, *TrackTiming, error) {
	timing := &TrackTiming{}
	if mdhd, ok := child(data, trak, "mdia", "mdhd"); ok && mdhd.end-mdhd.start >= 16 {
		if data[mdhd.start] == 1 { // version 1: 64-bit times
			if mdhd.end-mdhd.start >= 24 {
				timing.Timescale = binary.BigEndian.Uint32(data[mdhd.start+20 : mdhd.start+24])
			}
		} else {
			timing.Timescale = binary.BigEndian.Uint32(data[mdhd.start+12 : mdhd.start+16])
		}
	}
	stbl, ok := child(data, trak, "mdia", "minf", "stbl")
	if !ok {
		return nil, nil, fmt.Errorf("%w: track has no sample table", ErrInvalid)
	}
	sizes, err := sampleSizes(data, stbl)
	if err != nil {
		return nil, nil, err
	}
	offsets, err := chunkOffsets(data, stbl)
	if err != nil {
		return nil, nil, err
	}
	timing.Durations = sampleDurations(data, stbl, len(sizes))

	// stsc: map samples to chunks.
	type stscEntry struct{ firstChunk, perChunk uint32 }
	var stsc []stscEntry
	if sc, ok := child(data, stbl, "stsc"); ok && sc.end-sc.start >= 8 {
		n := int(binary.BigEndian.Uint32(data[sc.start+4 : sc.start+8]))
		for i := 0; i < n && sc.start+8+(i+1)*12 <= sc.end; i++ {
			e := sc.start + 8 + i*12
			stsc = append(stsc, stscEntry{binary.BigEndian.Uint32(data[e : e+4]), binary.BigEndian.Uint32(data[e+4 : e+8])})
		}
	}
	if len(stsc) == 0 {
		stsc = []stscEntry{{1, 1}}
	}

	samples := make([][]byte, 0, len(sizes))
	sample := 0
	for ci, off := range offsets {
		per := uint32(1)
		for _, e := range stsc {
			if uint32(ci+1) >= e.firstChunk {
				per = e.perChunk
			}
		}
		pos := off
		for j := uint32(0); j < per && sample < len(sizes); j++ {
			sz := uint64(sizes[sample])
			if pos+sz > uint64(len(data)) {
				return samples, timing, fmt.Errorf("%w: sample %d extends past end of file", ErrTruncated, sample)
			}
			samples = append(samples, data[pos:pos+sz])
			pos += sz
			sample++
		}
	}
	return samples, timing, nil
}

func sampleSizes(data []byte, stbl box) ([]uint32, error) {
	sz, ok := child(data, stbl, "stsz")
	if !ok || sz.end-sz.start < 12 {
		return nil, fmt.Errorf("%w: missing stsz", ErrInvalid)
	}
	constSize := binary.BigEndian.Uint32(data[sz.start+4 : sz.start+8])
	count := int(binary.BigEndian.Uint32(data[sz.start+8 : sz.start+12]))
	out := make([]uint32, 0, count)
	for i := range count {
		if constSize != 0 {
			out = append(out, constSize)
			continue
		}
		e := sz.start + 12 + i*4
		if e+4 > sz.end {
			return out, fmt.Errorf("%w: stsz lists %d samples but holds %d", ErrTruncated, count, i)
		}
		out = append(out, binary.BigEndian.Uint32(data[e:e+4]))
	}
	return out, nil
}

func chunkOffsets(data []byte, stbl box) ([]uint64, error) {
	if co, ok := child(data, stbl, "stco"); ok && co.end-co.start >= 8 {
		n := int(binary.BigEndian.Uint32(data[co.start+4 : co.start+8]))
		out := make([]uint64, 0, n)
		for i := range n {
			e := co.start + 8 + i*4
			if e+4 > co.end {
				return out, fmt.Errorf("%w: stco short", ErrTruncated)
			}
			out = append(out, uint64(binary.BigEndian.Uint32(data[e:e+4])))
		}
		return out, nil
	}
	if co, ok := child(data, stbl, "co64"); ok && co.end-co.start >= 8 {
		n := int(binary.BigEndian.Uint32(data[co.start+4 : co.start+8]))
		out := make([]uint64, 0, n)
		for i := range n {
			e := co.start + 8 + i*8
			if e+8 > co.end {
				return out, fmt.Errorf("%w: co64 short", ErrTruncated)
			}
			out = append(out, binary.BigEndian.Uint64(data[e:e+8]))
		}
		return out, nil
	}
	return nil, fmt.Errorf("%w: missing stco/co64", ErrInvalid)
}

func sampleDurations(data []byte, stbl box, n int) []uint32 {
	out := make([]uint32, 0, n)
	st, ok := child(data, stbl, "stts")
	if !ok || st.end-st.start < 8 {
		return nil
	}
	entries := int(binary.BigEndian.Uint32(data[st.start+4 : st.start+8]))
	for i := 0; i < entries && st.start+8+(i+1)*8 <= st.end; i++ {
		e := st.start + 8 + i*8
		count := binary.BigEndian.Uint32(data[e : e+4])
		delta := binary.BigEndian.Uint32(data[e+4 : e+8])
		for j := uint32(0); j < count && len(out) < n; j++ {
			out = append(out, delta)
		}
	}
	return out
}
