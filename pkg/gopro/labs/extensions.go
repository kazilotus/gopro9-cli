package labs

import (
	"sort"
	"strings"
)

// Support says whether a Labs extension works on the HERO9 Black.
type Support uint8

// Support levels.
const (
	// Unsupported: the Labs feature matrix lists the extension for other cameras only.
	Unsupported Support = iota
	// Supported: the Labs feature matrix lists the extension for the HERO9.
	Supported
	// Uncertain: GoPro's pages disagree; try it on the camera.
	Uncertain
)

// String implements fmt.Stringer.
func (s Support) String() string {
	switch s {
	case Supported:
		return "supported"
	case Uncertain:
		return "uncertain"
	default:
		return "unsupported"
	}
}

// ValueKind is the type of value an extension FourCC takes.
type ValueKind uint8

// Value kinds.
const (
	ValueNone   ValueKind = iota // flag-like: =1 enables
	ValueInt                     // integer
	ValueFloat                   // decimal
	ValueString                  // quoted string
	ValueList                    // comma-separated numbers
)

// String implements fmt.Stringer.
func (k ValueKind) String() string {
	switch k {
	case ValueInt:
		return "int"
	case ValueFloat:
		return "float"
	case ValueString:
		return "string"
	case ValueList:
		return "list"
	default:
		return "none"
	}
}

// Extension describes one Labs extension FourCC.
type Extension struct {
	FourCC      string    `json:"fourcc" yaml:"fourcc"`
	Description string    `json:"description" yaml:"description"`
	Support     Support   `json:"support" yaml:"support"`
	Value       ValueKind `json:"value" yaml:"value"`
}

var extensionTable = []Extension{
	{"BOOT", "run a script at boot", Supported, ValueString},
	{"HIST", "histogram overlay (1-11, 0 off)", Supported, ValueInt},
	{"HSTP", "histogram position and size", Supported, ValueList},
	{"BURN", "burn-in overlay of time and date", Supported, ValueInt},
	{"BRNT", "burn-in timing", Supported, ValueInt},
	{"BRNX", "burn-in position", Supported, ValueList},
	{"CBAR", "colour bar overlay", Supported, ValueInt},
	{"LBAR", "luma sweep overlay", Supported, ValueInt},
	{"LOGO", "logo burn-in", Supported, ValueInt},
	{"LLTZ", "location and timezone for solar time-lapse (lat,long,tz)", Supported, ValueList},
	{"GPSL", "continuous GPS logging to MISC", Supported, ValueInt},
	{"SYNC", "time synchronisation", Supported, ValueInt},
	{"TCAL", "timecode calibration offset in ms", Supported, ValueInt},
	{"QRDR", "read QR codes while recording", Supported, ValueInt},
	{"HNDL", "camera handle 1-31 for multi-camera QR addressing", Supported, ValueInt},
	{"DSPC", "Labs message contrast", Supported, ValueInt},
	{"DSPL", "Labs message display duration", Supported, ValueInt},
	{"DBGL", "debug logging to MISC/qrlog.txt", Supported, ValueInt},
	{"HSTO", "HindSight timeout in minutes", Supported, ValueInt},
	{"WAKE", "wake on power (HERO9: WAKE=1 only)", Supported, ValueInt},
	{"OWNR", "owner name", Supported, ValueString},
	{"BASE", "altered file base name", Supported, ValueString},
	{"ARCH", "archive mode", Supported, ValueInt},
	{"GAIN", "digital audio gain in dB", Supported, ValueInt},
	{"MUTE", "mute audio channels (mask)", Supported, ValueInt},
	{"SOLO", "solo one audio channel", Supported, ValueInt},
	{"HDMI", "Media Mod HDMI output mode", Supported, ValueInt},
	{"RTMP", "live stream RTMP URL", Supported, ValueString},
	{"JOIN", "Wi-Fi network to join (ssid:password)", Supported, ValueString},
	{"SPED", "SD card speed test", Supported, ValueInt},
	{"ZONE", "timezone", Supported, ValueInt},
	{"PRXY", "store LRV files as NLE proxies", Uncertain, ValueInt},
	{"DAUD", "disable audio in recordings", Uncertain, ValueInt},
	{"PRES", "create a named preset", Unsupported, ValueString},
	{"TUSB", "trust USB power source", Unsupported, ValueInt},
	{"64BT", "larger chapter size", Unsupported, ValueInt},
	{"FSOS", "repair all truncated files on scan", Unsupported, ValueInt},
	{"DLRV", "disable LRV creation", Unsupported, ValueInt},
	{"IWFR", "increase SD write frequency", Unsupported, ValueInt},
	{"BITR", "HEVC bitrate in Mb/s", Unsupported, ValueInt},
	{"BITH", "H.264 bitrate in Mb/s", Unsupported, ValueInt},
	{"NR01", "noise reduction 1-100", Unsupported, ValueInt},
	{"IFRM", "all I-frame encoding", Unsupported, ValueInt},
	{"24HZ", "24.0 fps instead of 23.976", Unsupported, ValueInt},
	{"ALLI", "all integer frame rates", Unsupported, ValueInt},
	{"EXPL", "exposure lock on capture", Unsupported, ValueInt},
	{"EXPX", "maximum shutter time", Unsupported, ValueFloat},
	{"EXPN", "minimum shutter time", Unsupported, ValueFloat},
	{"EXPQ", "fixed shutter time", Unsupported, ValueFloat},
	{"DAMP", "auto-exposure damping", Unsupported, ValueFloat},
	{"BFLK", "bypass deflicker", Unsupported, ValueInt},
	{"LOWL", "low-light shutter bias", Unsupported, ValueInt},
	{"GCMP", "disable gyro-compensated exposure", Unsupported, ValueInt},
	{"LOGB", "custom log curve (base,offset)", Unsupported, ValueList},
	{"GAMA", "gamma for HDRI work", Unsupported, ValueFloat},
	{"WBLK", "white balance lock on capture", Unsupported, ValueInt},
	{"WBLC", "white balance lock now", Unsupported, ValueInt},
	{"WBDV", "automatic red gain for diving", Unsupported, ValueInt},
	{"WIDE", "sensor-native colour gamut", Unsupported, ValueInt},
	{"MTRX", "custom RGB to YUV matrix", Unsupported, ValueList},
	{"TONE", "tone-mapping control 0-3", Unsupported, ValueInt},
	{"EVBS", "global EV bias", Unsupported, ValueFloat},
	{"DIVE", "underwater HyperSmooth refraction", Unsupported, ValueFloat},
	{"WARM", "red signal adjustment", Unsupported, ValueInt},
	{"COOL", "blue signal adjustment", Unsupported, ValueInt},
	{"DIOP", "diopter correction", Unsupported, ValueFloat},
	{"BERS", "bypass ERS compensation", Unsupported, ValueInt},
	{"STOP", "stop-motion frame count", Unsupported, ValueInt},
	{"ONIO", "onion-skin opacity", Unsupported, ValueInt},
	{"SHEL", "command shell over USB serial", Unsupported, ValueInt},
	{"OPNW", "non-secure HTTP for Open GoPro", Unsupported, ValueInt},
	{"LEVL", "spirit level overlay size", Unsupported, ValueInt},
	{"GUID", "shooting guides", Unsupported, ValueList},
	{"GUIC", "shooting guide luma", Unsupported, ValueList},
	{"EXPS", "display ISO and shutter", Unsupported, ValueInt},
	{"FRNT", "Labs messages on the front LCD", Unsupported, ValueInt},
	{"FONT", "dot-matrix font", Unsupported, ValueInt},
	{"FEAT", "redisplay active feature message", Unsupported, ValueInt},
	{"GRAB", "capture Labs overlay frames", Unsupported, ValueInt},
	{"PMXY", "processing message position", Unsupported, ValueList},
	{"AUDS", "display audio levels", Unsupported, ValueInt},
	{"WG84", "WGS84 GPS altitude", Unsupported, ValueInt},
	{"DIST", "distance-based time-lapse interval in m", Unsupported, ValueFloat},
	{"GOAL", "speed-timing overlay", Unsupported, ValueInt},
	{"HS30", "HindSight shortcut 30 s", Unsupported, ValueInt},
	{"BYPS", "bypass pop-ups", Unsupported, ValueInt},
	{"MMSK", "auto-generate motion mask", Unsupported, ValueInt},
	{"RLTC", "read LTC timecode from audio", Unsupported, ValueInt},
	{"OLTC", "LTC frame offset", Unsupported, ValueInt},
	{"FAST", "fewer Labs notifications on boot", Unsupported, ValueInt},
	{"STDL", "start delay in ms", Unsupported, ValueInt},
}

var extensionIndex = func() map[string]Extension {
	m := make(map[string]Extension, len(extensionTable))
	for _, e := range extensionTable {
		m[e.FourCC] = e
	}
	return m
}()

// Extensions returns every known Labs extension sorted by FourCC.
func Extensions() []Extension {
	out := make([]Extension, len(extensionTable))
	copy(out, extensionTable)
	sort.Slice(out, func(i, j int) bool { return out[i].FourCC < out[j].FourCC })
	return out
}

// LookupExtension finds an extension by FourCC (case-insensitive). Owner
// tags OWNA..OWNZ and OWNa..OWNz resolve to a synthetic supported entry.
func LookupExtension(fourcc string) (Extension, bool) {
	key := strings.ToUpper(fourcc)
	if e, ok := extensionIndex[key]; ok {
		return e, true
	}
	if len(fourcc) == 4 && strings.HasPrefix(key, "OWN") {
		return Extension{FourCC: fourcc, Description: "owner information tag", Support: Supported, Value: ValueString}, true
	}
	return Extension{}, false
}
