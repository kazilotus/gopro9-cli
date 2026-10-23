package gpmf

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mp4box(typ string, payload ...[]byte) []byte {
	body := make([]byte, 0, 64)
	for _, p := range payload {
		body = append(body, p...)
	}
	b := make([]byte, 8, 8+len(body))
	binary.BigEndian.PutUint32(b, uint32(8+len(body)))
	copy(b[4:], typ)
	return append(b, body...)
}

func u32s(vs ...uint32) []byte {
	b := make([]byte, 0, 4*len(vs))
	for _, v := range vs {
		b = binary.BigEndian.AppendUint32(b, v)
	}
	return b
}

// buildMP4 assembles a minimal container whose gpmd track samples are the given payloads.
func buildMP4(samples [][]byte, delta uint32, co64 bool, handler, entry string) []byte {
	const timescale uint32 = 1000
	ftyp := mp4box("ftyp", []byte("avc1"), u32s(1), []byte("avc1"))
	mdhd := mp4box("mdhd", u32s(0, 0, 0, timescale, 0, 0))
	hdlr := mp4box("hdlr", u32s(0, 0), []byte(handler), u32s(0, 0, 0), []byte{0})
	stsd := mp4box("stsd", u32s(0, 1), mp4box(entry, make([]byte, 8)))
	sizes := make([]uint32, 0, 3+len(samples))
	sizes = append(sizes, 0, 0, uint32(len(samples)))
	for _, s := range samples {
		sizes = append(sizes, uint32(len(s)))
	}
	stsz := mp4box("stsz", u32s(sizes...))
	stsc := mp4box("stsc", u32s(0, 1, 1, 1, 1))
	stts := mp4box("stts", u32s(0, 1, uint32(len(samples)), delta))
	// Chunk offsets are patched after layout.
	var stco []byte
	if co64 {
		body := u32s(0, uint32(len(samples)))
		for range samples {
			body = binary.BigEndian.AppendUint64(body, 0)
		}
		stco = mp4box("co64", body)
	} else {
		body := u32s(0, uint32(len(samples)))
		for range samples {
			body = binary.BigEndian.AppendUint32(body, 0)
		}
		stco = mp4box("stco", body)
	}
	stbl := mp4box("stbl", stsd, stsz, stsc, stco, stts)
	minf := mp4box("minf", stbl)
	mdia := mp4box("mdia", mdhd, hdlr, minf)
	trak := mp4box("trak", mdia)
	udta := mp4box("udta", mp4box("GPMF", nested("DEVC", klv("DVID", 'L', 4, 1, beU32(1)), text("DVNM", "Global Settings"))))
	moov := mp4box("moov", trak, udta)
	mdatBody := make([]byte, 0, 1024)
	for _, s := range samples {
		mdatBody = append(mdatBody, s...)
	}
	mdat := mp4box("mdat", mdatBody)
	file := cat(ftyp, moov, mdat)
	// Patch offsets.
	first := uint64(len(ftyp) + len(moov) + 8)
	idx := indexOf(file, []byte("stco"))
	entrySize := 4
	if co64 {
		idx = indexOf(file, []byte("co64"))
		entrySize = 8
	}
	pos := idx + 4 + 8
	off := first
	for _, s := range samples {
		if co64 {
			binary.BigEndian.PutUint64(file[pos:], off)
		} else {
			binary.BigEndian.PutUint32(file[pos:], uint32(off))
		}
		pos += entrySize
		off += uint64(len(s))
	}
	return file
}

func indexOf(b, sub []byte) int {
	for i := 0; i+len(sub) <= len(b); i++ {
		if bytes.Equal(b[i:i+len(sub)], sub) {
			return i
		}
	}
	return -1
}

func TestExtractFromMP4Synthetic(t *testing.T) {
	t.Parallel()
	samples := [][]byte{payloadN(0, 3), payloadN(1, 3), payloadN(2, 3)}
	file := buildMP4(samples, 996, false, "meta", "gpmd")
	require.True(t, IsMP4(file))
	got, timing, err := ExtractFromMP4(file)
	require.NoError(t, err)
	require.Len(t, got, 3)
	assert.Equal(t, samples[1], got[1])
	assert.Equal(t, uint32(1000), timing.Timescale)
	assert.Equal(t, []uint32{996, 996, 996}, timing.Durations)
	assert.InDelta(t, 1.992, timing.Start(2), 1e-9)
	assert.InDelta(t, 1.0, timing.Seconds(99), 0, "out of range defaults to 1s")
	assert.InDelta(t, 3.0, (*TrackTiming)(nil).Start(3), 0)

	hdr := HeaderFromMP4(file)
	require.NotEmpty(t, hdr)
	_, name := DeviceInfo(mustParse(t, hdr))
	assert.Equal(t, "Global Settings", name)

	tel, err := ParseTelemetry(file)
	require.NoError(t, err)
	assert.Equal(t, 3, tel.Payloads)
	assert.NotNil(t, tel.Timing)
	assert.InDelta(t, 0.996, tel.PayloadDuration(0), 1e-9)
	fixes := tel.GPS()
	require.Len(t, fixes, 6)
	assert.InDelta(t, 0.498, fixes[1].Time.Sub(fixes[0].Time).Seconds(), 1e-9, "spacing follows track duration")

	// co64 and the stsd fallback when hdlr is not "meta".
	file = buildMP4(samples, 1000, true, "vide", "gpmd")
	got, _, err = ExtractFromMP4(file)
	require.NoError(t, err)
	assert.Len(t, got, 3)

	// No metadata track at all.
	file = buildMP4(samples, 1000, false, "vide", "avc1")
	_, _, err = ExtractFromMP4(file)
	require.ErrorIs(t, err, ErrNoTrack)
	tel, err = ParseTelemetry(file)
	require.ErrorIs(t, err, ErrNoTrack)
	assert.Zero(t, tel.Payloads)
	assert.Nil(t, HeaderFromMP4(mp4box("ftyp")))
}

func mustParse(t *testing.T, data []byte) []Item {
	t.Helper()
	items, err := Parse(data)
	require.NoError(t, err)
	return items
}

func TestMP4Errors(t *testing.T) {
	t.Parallel()
	samples := [][]byte{payloadN(0, 3)}
	file := buildMP4(samples, 1000, false, "meta", "gpmd")

	// Truncated file: sample extends past the end.
	_, _, err := ExtractFromMP4(file[:len(file)-10])
	require.Error(t, err)

	// Box size larger than buffer.
	bad := append([]byte{}, file...)
	binary.BigEndian.PutUint32(bad[0:], 0xffffff)
	_, err = boxes(bad, 0, len(bad))
	require.ErrorIs(t, err, ErrTruncated)

	// largesize box.
	large := make([]byte, 16)
	binary.BigEndian.PutUint32(large, 1)
	copy(large[4:], "free")
	binary.BigEndian.PutUint64(large[8:], 16)
	bs, err := boxes(large, 0, len(large))
	require.NoError(t, err)
	assert.Equal(t, "free", bs[0].typ)
	_, err = boxes(large[:12], 0, 12)
	require.ErrorIs(t, err, ErrInvalid)
	binary.BigEndian.PutUint64(large[8:], 1<<40)
	_, err = boxes(large, 0, len(large))
	require.ErrorIs(t, err, ErrTruncated)

	// size 0 = to end of buffer.
	zero := mp4box("free", []byte{1, 2, 3, 4})
	binary.BigEndian.PutUint32(zero, 0)
	bs, err = boxes(zero, 0, len(zero))
	require.NoError(t, err)
	assert.Equal(t, len(zero), bs[0].end)

	// Track without stbl / stsz / stco.
	trakNoStbl := mp4box("moov", mp4box("trak", mp4box("mdia", mp4box("hdlr", u32s(0, 0), []byte("meta")))))
	_, _, err = ExtractFromMP4(cat(mp4box("ftyp", []byte("avc1")), trakNoStbl))
	require.ErrorIs(t, err, ErrInvalid)
	noStsz := mp4box("moov", mp4box("trak", mp4box("mdia", mp4box("hdlr", u32s(0, 0), []byte("meta")), mp4box("minf", mp4box("stbl")))))
	_, _, err = ExtractFromMP4(cat(mp4box("ftyp", []byte("avc1")), noStsz))
	require.ErrorIs(t, err, ErrInvalid)
	noStco := mp4box("moov", mp4box("trak", mp4box("mdia", mp4box("hdlr", u32s(0, 0), []byte("meta")), mp4box("minf", mp4box("stbl", mp4box("stsz", u32s(0, 4, 1)))))))
	_, _, err = ExtractFromMP4(cat(mp4box("ftyp", []byte("avc1")), noStco))
	require.ErrorIs(t, err, ErrInvalid)

	// Constant sample size and short tables.
	stbl := mp4box("stbl", mp4box("stsz", u32s(0, 4, 2)), mp4box("stco", u32s(0, 5, 1)), mp4box("stts", u32s(0, 1, 2, 5)))
	_, err = sampleSizes([]byte{}, box{})
	require.Error(t, err)
	sizes, err := sampleSizes(stbl, box{start: 8, end: len(stbl)})
	require.NoError(t, err)
	assert.Equal(t, []uint32{4, 4}, sizes)
	_, err = chunkOffsets(stbl, box{start: 8, end: len(stbl)})
	require.ErrorIs(t, err, ErrTruncated)
	assert.Equal(t, []uint32{5}, sampleDurations(stbl, box{start: 8, end: len(stbl)}, 1))
	assert.Nil(t, sampleDurations(mp4box("stbl"), box{start: 8, end: 8}, 1))
	shortSz := mp4box("stbl", mp4box("stsz", u32s(0, 0, 3, 1)))
	_, err = sampleSizes(shortSz, box{start: 8, end: len(shortSz)})
	require.ErrorIs(t, err, ErrTruncated)
	shortCo64 := mp4box("stbl", mp4box("co64", u32s(0, 2, 0, 1)))
	_, err = chunkOffsets(shortCo64, box{start: 8, end: len(shortCo64)})
	require.ErrorIs(t, err, ErrTruncated)

	// mdhd version 1 timescale.
	v1 := mp4box("mdhd", []byte{1, 0, 0, 0}, make([]byte, 16), u32s(48000), make([]byte, 8))
	trak := mp4box("trak", mp4box("mdia", v1, mp4box("hdlr", u32s(0, 0), []byte("meta")), mp4box("minf", mp4box("stbl", mp4box("stsz", u32s(0, 0, 0)), mp4box("stco", u32s(0, 0))))))
	_, timing, err := readTrack(trak, box{start: 8, end: len(trak)})
	require.NoError(t, err)
	assert.Equal(t, uint32(48000), timing.Timescale)
}
