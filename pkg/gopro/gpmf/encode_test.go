package gpmf

import (
	"encoding/binary"
	"math"
)

// klv encodes one item. Payload is padded to 4 bytes.
func klv(key string, typ byte, size int, repeat int, payload []byte) []byte {
	b := make([]byte, 0, headerLen+padded(len(payload)))
	b = append(b, []byte(key)...)
	b = append(b, typ, byte(size), byte(repeat>>8), byte(repeat))
	b = append(b, payload...)
	for len(b)%4 != 0 {
		b = append(b, 0)
	}
	return b
}

func nested(key string, children ...[]byte) []byte {
	payload := make([]byte, 0, 256)
	for _, c := range children {
		payload = append(payload, c...)
	}
	// Mirror the camera: size 1, repeat = payload length.
	return klv(key, 0, 1, len(payload), payload)
}

func text(key, s string) []byte { return klv(key, 'c', 1, len(s), []byte(s)) }

func be16(v int16) []byte    { b := make([]byte, 2); binary.BigEndian.PutUint16(b, uint16(v)); return b }
func beU16(v uint16) []byte  { b := make([]byte, 2); binary.BigEndian.PutUint16(b, v); return b }
func be32(v int32) []byte    { b := make([]byte, 4); binary.BigEndian.PutUint32(b, uint32(v)); return b }
func beU32(v uint32) []byte  { b := make([]byte, 4); binary.BigEndian.PutUint32(b, v); return b }
func be64(v int64) []byte    { b := make([]byte, 8); binary.BigEndian.PutUint64(b, uint64(v)); return b }
func beU64(v uint64) []byte  { b := make([]byte, 8); binary.BigEndian.PutUint64(b, v); return b }
func bef32(v float32) []byte { return beU32(math.Float32bits(v)) }
func bef64(v float64) []byte { return beU64(math.Float64bits(v)) }

func cat(parts ...[]byte) []byte {
	out := make([]byte, 0, 64)
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

// strm builds a STRM container with metadata and a data item.
func strm(meta []byte, data []byte) []byte { return nested("STRM", meta, data) }

// payloadN builds one DEVC payload with ACCL (2 samples), GPS5 (2 samples), SHUT (1 sample).
func payloadN(n int, gpsFix uint32) []byte {
	accl := strm(
		cat(text("STNM", "Accelerometer"), klv("SIUN", 'c', 4, 1, []byte("m/s\xb2")), klv("SCAL", 's', 2, 1, be16(418)), klv("TMPC", 'f', 4, 1, bef32(30.5))),
		klv("ACCL", 's', 6, 2, cat(be16(418), be16(836), be16(-418), be16(0), be16(209), be16(1254))),
	)
	gps := strm(
		cat(
			text("STNM", "GPS (Lat., Long., Alt., 2D speed, 3D speed)"),
			klv("SIUN", 'c', 3, 5, []byte("degdegm\x00\x00m/sm/s")),
			klv("SCAL", 'l', 4, 5, cat(be32(10000000), be32(10000000), be32(1000), be32(1000), be32(100))),
			klv("GPSU", 'U', 16, 1, []byte("260123120000.500")[:16]),
			klv("GPSF", 'L', 4, 1, beU32(gpsFix)),
			klv("GPSP", 'S', 2, 1, beU16(500)),
		),
		klv("GPS5", 'l', 20, 2, cat(
			be32(327070000+int32(n)), be32(-1171576000), be32(12345), be32(5000), be32(520),
			be32(327080000+int32(n)), be32(-1171577000), be32(12355), be32(6000), be32(640),
		)),
	)
	shut := strm(
		cat(text("STNM", "Exposure time (shutter speed)"), klv("SIUN", 'c', 1, 1, []byte("s"))),
		klv("SHUT", 'f', 4, 1, bef32(0.0025)),
	)
	return nested("DEVC", klv("DVID", 'L', 4, 1, beU32(1)), text("DVNM", "Hero9 Black"), accl, gps, shut)
}

func track(payloads int) []byte {
	out := make([]byte, 0, 512*payloads)
	for i := range payloads {
		fix := uint32(3)
		if i == 1 {
			fix = 0 // second payload has no fix
		}
		out = append(out, payloadN(i, fix)...)
	}
	return out
}
