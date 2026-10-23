package labs

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Mode is a capture mode selectable through the m command.
type Mode string

// Modes available on the HERO9 Black.
const (
	ModeVideo          Mode = "V"
	ModeVideoLooping   Mode = "VL"
	ModePhoto          Mode = "P"
	ModePhotoBurst     Mode = "PB"
	ModePhotoNight     Mode = "PN"
	ModeTimelapseVideo Mode = "T"
	ModeTimelapsePhoto Mode = "TP"
	ModeTimeWarp       Mode = "TW"
	ModeNightLapse     Mode = "NL"
	ModeNightLapsePhot Mode = "NP"
	ModeLiveBurst      Mode = "L"
)

// Lens is a digital lens selectable through the f command.
type Lens string

// Lenses.
const (
	LensNarrow       Lens = "N"
	LensMedium       Lens = "M"
	LensWide         Lens = "W"
	LensLinear       Lens = "L"
	LensSuperView    Lens = "S"
	LensMaxSuperView Lens = "V"
	LensHorizonLevel Lens = "H"
	LensMax          Lens = "X"
)

// Color is a colour profile selectable through the c command.
type Color string

// Colour profiles.
const (
	ColorGoPro   Color = "G"
	ColorNatural Color = "N"
	ColorFlat    Color = "F"
)

// HyperSmooth levels for the e command.
type HyperSmooth string

// HyperSmooth levels.
const (
	HyperSmoothOff   HyperSmooth = "0"
	HyperSmoothOn    HyperSmooth = "1"
	HyperSmoothHigh  HyperSmooth = "2"
	HyperSmoothBoost HyperSmooth = "3"
	HyperSmoothAuto  HyperSmooth = "4"
)

// Builder composes a canonical Labs command string. Zero value is ready to use.
// Commands are emitted in a stable order: mode, settings, extensions and
// metadata, message, then actions (start, end, repeat) followed by raw tokens.
type Builder struct {
	mode        Mode
	settings    []string
	extensions  []string
	message     string
	startAt     string
	startIn     time.Duration
	hasStart    bool
	duration    time.Duration
	repeatEvery time.Duration
	repeat      bool
	raw         []string
}

// Mode selects the capture mode.
func (b *Builder) Mode(m Mode) *Builder { b.mode = m; return b }

// Resolution sets the resolution token value: "4", "27", "1080", "4T", "5K" are passed through.
func (b *Builder) Resolution(r string) *Builder { return b.setting("r", strings.TrimSpace(r)) }

// FPS sets the frame rate.
func (b *Builder) FPS(fps int) *Builder { return b.setting("p", strconv.Itoa(fps)) }

// Interval sets a time-lapse interval in seconds (p.5, p2, p60).
func (b *Builder) Interval(seconds float64) *Builder { return b.setting("p", trimFloat(seconds)) }

// Lens sets the digital lens.
func (b *Builder) Lens(l Lens) *Builder { return b.setting("f", string(l)) }

// EV sets exposure compensation (-2 .. 2).
func (b *Builder) EV(ev float64) *Builder { return b.setting("x", trimFloat(ev)) }

// WhiteBalance sets Kelvin/100 (23..65), or 0 for auto.
func (b *Builder) WhiteBalance(kelvin int) *Builder {
	if kelvin <= 0 {
		return b.setting("w", "A")
	}
	return b.setting("w", strconv.Itoa(kelvin/100))
}

// ISO sets the ISO range; min 0 keeps the camera's minimum. Values are in
// hundreds (16 = ISO 1600, 2 = ISO 200), as the camera expects.
func (b *Builder) ISO(minHundreds, maxHundreds int) *Builder {
	v := strconv.Itoa(maxHundreds)
	if minHundreds > 0 {
		v += "M" + strconv.Itoa(minHundreds)
	}
	return b.setting("i", v)
}

// Color sets the colour profile.
func (b *Builder) Color(c Color) *Builder { return b.setting("c", string(c)) }

// HyperSmooth sets stabilisation.
func (b *Builder) HyperSmooth(h HyperSmooth) *Builder { return b.setting("e", string(h)) }

// GPS toggles GPS.
func (b *Builder) GPS(on bool) *Builder { return b.setting("g", boolDigit(on)) }

// Setting adds an arbitrary setting token (key letter plus value).
func (b *Builder) Setting(key, value string) *Builder { return b.setting(key, value) }

func (b *Builder) setting(key, value string) *Builder {
	b.settings = append(b.settings, key+value)
	return b
}

// StartIn starts capture after a delay (!<seconds>S).
func (b *Builder) StartIn(d time.Duration) *Builder {
	b.startIn, b.startAt, b.hasStart = d, "", true
	return b
}

// StartAt starts capture at a clock time "hh:mm" (!hh:mmS).
func (b *Builder) StartAt(hhmm string) *Builder {
	b.startAt, b.startIn, b.hasStart = hhmm, 0, true
	return b
}

// Start starts capture immediately (!S).
func (b *Builder) Start() *Builder { b.hasStart, b.startAt, b.startIn = true, "", 0; return b }

// Duration ends capture after d (!<seconds>E). Implies Start when none was set.
func (b *Builder) Duration(d time.Duration) *Builder {
	b.duration = d
	if !b.hasStart {
		b.hasStart = true
	}
	return b
}

// Repeat repeats the whole script; with d > 0 a pause of d is inserted first (!<seconds>R).
func (b *Builder) Repeat(d time.Duration) *Builder { b.repeat, b.repeatEvery = true, d; return b }

// Message shows text on the camera screen.
func (b *Builder) Message(text string) *Builder { b.message = text; return b }

// Metadata stores a FourCC value; permanent uses the HERO9's !M form.
func (b *Builder) Metadata(fourcc, value string, permanent bool) *Builder {
	b.extensions = append(b.extensions, metadataToken(fourcc, value, permanent))
	return b
}

// Extension configures a Labs extension FourCC until power off ($FOUR=value).
func (b *Builder) Extension(fourcc, value string) *Builder {
	b.extensions = append(b.extensions, metadataToken(fourcc, value, false))
	return b
}

// Raw appends a token verbatim at the end.
func (b *Builder) Raw(token string) *Builder { b.raw = append(b.raw, token); return b }

func metadataToken(fourcc, value string, permanent bool) string {
	fourcc = strings.ToUpper(fourcc)
	if _, err := strconv.ParseFloat(strings.ReplaceAll(value, ",", ""), 64); err != nil && !isNumberList(value) {
		value = `"` + strings.ReplaceAll(value, `"`, `'`) + `"`
	}
	if permanent {
		return "!M" + fourcc + "=" + value
	}
	return "$" + fourcc + "=" + value
}

func isNumberList(v string) bool {
	if v == "" {
		return false
	}
	for _, part := range strings.Split(v, ",") {
		if _, err := strconv.ParseFloat(part, 64); err != nil {
			return false
		}
	}
	return true
}

// String renders the canonical command string.
func (b *Builder) String() string {
	var sb strings.Builder
	if b.mode != "" {
		sb.WriteString("m" + string(b.mode))
	}
	for _, s := range b.settings {
		sb.WriteString(s)
	}
	for _, e := range b.extensions {
		sb.WriteString(e)
	}
	if b.message != "" {
		sb.WriteString(`"` + strings.ReplaceAll(b.message, `"`, `'`) + `"`)
	}
	if b.hasStart {
		switch {
		case b.startAt != "":
			sb.WriteString("!" + b.startAt + "S")
		case b.startIn > 0:
			sb.WriteString("!" + seconds(b.startIn) + "S")
		default:
			sb.WriteString("!S")
		}
	}
	if b.duration > 0 {
		sb.WriteString("!" + seconds(b.duration) + "E")
	}
	if b.repeat {
		if b.repeatEvery > 0 {
			sb.WriteString("!" + seconds(b.repeatEvery) + "R")
		} else {
			sb.WriteString("!R")
		}
	}
	for _, r := range b.raw {
		sb.WriteString(r)
	}
	return sb.String()
}

// Script parses the built string, returning the first validation error if any.
func (b *Builder) Script() (Script, error) {
	sc, err := Parse(b.String())
	if err != nil {
		return sc, err
	}
	if errs := Errors(Validate(sc)); len(errs) > 0 {
		return sc, fmt.Errorf("%w: %s", ErrSyntax, errs[0].Message)
	}
	return sc, nil
}

func seconds(d time.Duration) string {
	return strconv.FormatInt(int64(d.Round(time.Second)/time.Second), 10)
}

func trimFloat(f float64) string {
	s := strconv.FormatFloat(f, 'f', -1, 64)
	s = strings.TrimPrefix(s, "0")
	s = strings.TrimPrefix(s, "-0")
	if f < 0 && !strings.HasPrefix(s, "-") {
		s = "-" + s
	}
	return s
}

func boolDigit(b bool) string {
	if b {
		return "1"
	}
	return "0"
}
