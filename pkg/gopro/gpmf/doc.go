// Package gpmf parses GoPro Metadata Format (GPMF) streams as returned by the
// camera's /gopro/media/gpmf (file header) and /gopro/media/telemetry (full
// sensor track) endpoints, and exports them as GPX or CSV.
//
// # Format
//
// GPMF is a sequence of KLV items: a 4-byte FourCC key, one type character,
// one byte structure size, a big-endian uint16 repeat count, then size*repeat
// payload bytes zero-padded to a 4-byte boundary. Multi-byte numbers are
// big-endian. A type of 0 marks a nested container whose payload is itself a
// KLV sequence. A telemetry track is a series of DEVC containers, one per
// roughly one second of video, each holding the device id (DVID), device name
// (DVNM) and one STRM container per sensor. A STRM carries descriptive items
// (STNM name, SIUN/UNIT units, SCAL divisors, TYPE for complex samples, TMPC
// temperature, TSMP total samples) followed by one data item whose repeat
// count is the number of samples in that payload, for example GPS5 (lat, lon,
// altitude, 2D speed, 3D speed), ACCL, GYRO, SHUT, GPSU (UTC time), GPSF
// (fix) and GPSP (precision).
//
// Parse returns the generic item tree; ParseTelemetry merges the per-payload
// streams into Stream values with scaled float64 samples and remembers the
// payload boundaries so timing can be reconstructed. Telemetry.GPS derives
// time-stamped fixes and WriteGPX/WriteCSV export them.
//
// # Limitations
//
// Half-precision floats (type 'H') are skipped structurally but decode as NaN.
// Non-numeric columns of complex (TYPE-described) samples — characters,
// FourCCs, GUIDs and UTC strings — decode as NaN in Samples; UTC date strings
// used as whole samples (type 'U', e.g. GPSU) decode to Unix seconds and are
// also kept verbatim in Stream.Text. The parser is tolerant: on truncated or
// malformed input it returns everything parsed so far together with a wrapped
// ErrTruncated or ErrInvalid, and it never panics.
package gpmf
