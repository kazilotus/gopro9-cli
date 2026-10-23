package gpmf

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSimpleTypes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		item []byte
		want []float64
	}{
		{"int8", klv("TSTb", 'b', 1, 2, []byte{0xff, 0x7f}), []float64{-1, 127}},
		{"uint8", klv("TSTB", 'B', 1, 2, []byte{0xff, 0x7f}), []float64{255, 127}},
		{"int16", klv("TSTs", 's', 2, 2, cat(be16(-2), be16(300))), []float64{-2, 300}},
		{"uint16", klv("TSTS", 'S', 2, 1, beU16(65535)), []float64{65535}},
		{"int32", klv("TSTl", 'l', 4, 1, be32(-70000)), []float64{-70000}},
		{"uint32", klv("TSTL", 'L', 4, 1, beU32(4000000000)), []float64{4000000000}},
		{"float32", klv("TSTf", 'f', 4, 1, bef32(1.5)), []float64{1.5}},
		{"float64", klv("TSTd", 'd', 8, 1, bef64(-2.25)), []float64{-2.25}},
		{"int64", klv("TSTj", 'j', 8, 1, be64(-5)), []float64{-5}},
		{"uint64", klv("TSTJ", 'J', 8, 1, beU64(7)), []float64{7}},
		{"q15.16", klv("TSTq", 'q', 4, 1, be32(1<<16+1<<15)), []float64{1.5}},
		{"q31.32", klv("TSTQ", 'Q', 8, 1, be64(1<<32+1<<31)), []float64{1.5}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			items, err := Parse(tc.item)
			require.NoError(t, err)
			require.Len(t, items, 1)
			rows := items[0].Numbers("")
			require.Len(t, rows, len(tc.want))
			for i, w := range tc.want {
				assert.InDelta(t, w, rows[i][0], 1e-9)
			}
		})
	}
}

func TestParseNonNumericTypes(t *testing.T) {
	t.Parallel()
	items, err := Parse(cat(
		klv("FOUR", 'F', 4, 1, []byte("ABCD")),
		klv("GUID", 'G', 16, 1, make([]byte, 16)),
		klv("HALF", 'H', 2, 2, []byte{0x3c, 0x00, 0x40, 0x00}),
		klv("DATE", 'U', 16, 1, []byte("260123120000.500")),
		text("NAME", "hello\x00\x00"),
	))
	require.NoError(t, err)
	require.Len(t, items, 5)
	assert.True(t, math.IsNaN(items[0].Numbers("")[0][0]), "FourCC is not numeric")
	assert.True(t, math.IsNaN(items[1].Numbers("")[0][0]), "GUID is not numeric")
	half := items[2].Numbers("")
	require.Len(t, half, 2)
	assert.True(t, math.IsNaN(half[0][0]), "float16 unsupported")
	ts := items[3].Numbers("")[0][0]
	assert.InDelta(t, 1769169600.5, ts, 1e-3)
	assert.Equal(t, "260123120000.500", items[3].String())
	assert.Equal(t, "hello", items[4].String())
	assert.Nil(t, items[4].Numbers(""), "character payloads are not numeric")
	assert.Empty(t, items[0].String(), "String only decodes c and U")

	_, err = ParseUTC("garbage")
	require.Error(t, err)
	assert.True(t, math.IsNaN(decode('U', []byte("bad date string!"))))
}

func TestParseStringsAndMultiColumn(t *testing.T) {
	t.Parallel()
	items, err := Parse(cat(
		klv("SIUN", 'c', 3, 2, []byte("degm/s")),
		klv("ACCL", 's', 6, 2, cat(be16(1), be16(2), be16(3), be16(4), be16(5), be16(6))),
	))
	require.NoError(t, err)
	assert.Equal(t, []string{"deg", "m/s"}, items[0].Strings())
	assert.Nil(t, items[1].Strings())
	rows := items[1].Numbers("")
	assert.Equal(t, [][]float64{{1, 2, 3}, {4, 5, 6}}, rows)
}

func TestParsePaddingAndTrailingZeros(t *testing.T) {
	t.Parallel()
	data := cat(klv("ODD1", 'B', 1, 3, []byte{1, 2, 3}), klv("NEXT", 'L', 4, 1, beU32(9)), make([]byte, 7))
	items, err := Parse(data)
	require.NoError(t, err)
	require.Len(t, items, 2)
	assert.Equal(t, []byte{1, 2, 3}, items[0].Raw)
	assert.Equal(t, "NEXT", items[1].Key)
	items, err = Parse(nil)
	require.NoError(t, err)
	assert.Empty(t, items)
}

func TestParseNested(t *testing.T) {
	t.Parallel()
	data := nested("DEVC", klv("DVID", 'L', 4, 1, beU32(42)), text("DVNM", "Cam"), nested("STRM", text("STNM", "x"), klv("SHUT", 'f', 4, 1, bef32(1))))
	items, err := Parse(data)
	require.NoError(t, err)
	require.Len(t, items, 1)
	dev := items[0]
	assert.Equal(t, byte(0), dev.Type)
	require.Len(t, dev.Children, 3)
	id, name := DeviceInfo(items)
	assert.Equal(t, uint32(42), id)
	assert.Equal(t, "Cam", name)
	s, ok := dev.Find("STRM")
	require.True(t, ok)
	_, ok = s.Find("SHUT")
	assert.True(t, ok)
	_, ok = s.Find("NOPE")
	assert.False(t, ok)
	id, name = DeviceInfo(nil)
	assert.Zero(t, id)
	assert.Empty(t, name)
	// FourCC-typed DVID (older firmware) decodes from its bytes.
	id, _ = DeviceInfo([]Item{{Key: "DEVC", Children: []Item{{Key: "DVID", Type: 'F', Size: 4, Repeat: 1, Raw: []byte{0, 0, 1, 2}}}}})
	assert.Equal(t, uint32(0x0102), id)
}

func TestParseFakecamBlobShape(t *testing.T) {
	t.Parallel()
	// DEVC header with size=1 repeat=len, DVNM with trailing spaces, zero padding to 2048.
	inner := cat(klv("DVID", 'L', 4, 1, beU32(1)), text("DVNM", "Fake HERO9  "))
	blob := klv("DEVC", 0, 1, len(inner), inner)
	for len(blob) < 2048 {
		blob = append(blob, 0)
	}
	items, err := Parse(blob)
	require.NoError(t, err)
	id, name := DeviceInfo(items)
	assert.Equal(t, uint32(1), id)
	assert.Equal(t, "Fake HERO9", name)
	tel, err := ParseTelemetry(blob)
	require.NoError(t, err)
	assert.Equal(t, 1, tel.Payloads)
	assert.Empty(t, tel.Streams)
}

func TestParseComplexType(t *testing.T) {
	t.Parallel()
	// TYPE "cL": one char + one uint32 per sample, size 5.
	data := nested("STRM",
		text("STNM", "complex"),
		text("TYPE", "cL"),
		klv("CPLX", '?', 5, 2, cat([]byte{'a'}, beU32(10), []byte{'b'}, beU32(20))),
	)
	items, err := Parse(data)
	require.NoError(t, err)
	cp, ok := items[0].Find("CPLX")
	require.True(t, ok)
	rows := cp.Numbers("cL")
	require.Len(t, rows, 2)
	assert.True(t, math.IsNaN(rows[0][0]))
	assert.InDelta(t, 10, rows[0][1], 0)
	assert.InDelta(t, 20, rows[1][1], 0)
	assert.Nil(t, cp.Numbers("LL"), "type string size mismatch")
	assert.Nil(t, cp.Numbers("cZ"), "unknown type char")
	assert.Nil(t, cp.Numbers(""), "complex needs a TYPE")

	tel, err := ParseTelemetry(nested("DEVC", data))
	require.NoError(t, err)
	s, ok := tel.Stream("CPLX")
	require.True(t, ok)
	assert.Equal(t, "cL", s.Type)
	assert.Equal(t, 2, s.Columns)
	assert.InDelta(t, 20, s.Samples[1][1], 0)
}

func TestParseErrors(t *testing.T) {
	t.Parallel()
	full := klv("ACCL", 's', 6, 2, make([]byte, 12))

	items, err := Parse(full[:5])
	require.ErrorIs(t, err, ErrTruncated, "short header")
	assert.Empty(t, items)

	items, err = Parse(full[:12])
	require.ErrorIs(t, err, ErrTruncated, "short payload")
	assert.Empty(t, items)

	_, err = Parse(klv("ACCL", 'Z', 6, 1, make([]byte, 6)))
	require.ErrorIs(t, err, ErrInvalid, "unknown type")

	_, err = Parse(klv("\x01BAD", 'L', 4, 1, make([]byte, 4)))
	require.ErrorIs(t, err, ErrInvalid, "non-printable key")

	_, err = Parse(klv("SIZE", 'L', 3, 1, make([]byte, 3)))
	require.ErrorIs(t, err, ErrInvalid, "size not a multiple of element size")

	// Items before the fault are preserved, including inside containers.
	good := klv("GOOD", 'L', 4, 1, beU32(1))
	items, err = Parse(cat(good, full[:12]))
	require.ErrorIs(t, err, ErrTruncated)
	require.Len(t, items, 1)
	assert.Equal(t, "GOOD", items[0].Key)

	items, err = Parse(nested("DEVC", good, klv("BAD!", 'Z', 1, 1, []byte{1})))
	require.ErrorIs(t, err, ErrInvalid)
	require.Len(t, items, 1)
	assert.Len(t, items[0].Children, 1)

	// Nesting bomb.
	deep := klv("LEAF", 'B', 1, 1, []byte{1})
	for range maxDepth + 1 {
		deep = nested("NEST", deep)
	}
	_, err = Parse(deep)
	require.ErrorIs(t, err, ErrInvalid)

	// Zero-size numeric item decodes to nothing rather than dividing by zero.
	assert.Nil(t, Item{Key: "ZERO", Type: 'L', Size: 0, Repeat: 1}.Numbers(""))
	assert.Nil(t, Item{Key: "ZERO", Type: 'c', Size: 0, Repeat: 1}.Strings())
}
