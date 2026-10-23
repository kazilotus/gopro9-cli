package labs

import (
	"fmt"
	"strings"
)

var modeNames = map[string]string{
	"V": "Video", "VL": "Video Looping", "VN": "Video Night", "P": "Photo", "PB": "Photo Burst", "PN": "Photo Night",
	"T": "Time-lapse video", "TP": "Time-lapse photo", "TW": "TimeWarp", "NL": "Night-lapse video", "NP": "Night-lapse photo",
	"L": "Live Burst", "ST": "Star Trails", "LP": "Light Painting", "LT": "Vehicle Lights", "E": "Easy UX", "PR": "Pro UX",
	"H": "MAX HERO", "HF": "MAX HERO front", "HR": "MAX HERO rear", "3": "MAX 360", "3F": "MAX 360 front", "3R": "MAX 360 rear",
	"F": "MAX front monitoring", "R": "MAX rear monitoring",
}

var settingNames = map[string]string{
	"r": "resolution", "p": "frame rate / interval", "f": "lens", "b": "bitrate", "c": "colour", "d": "bit depth or power-up default",
	"e": "HyperSmooth", "eL": "exposure lock", "hS": "HindSight", "w": "white balance", "i": "ISO", "s": "sharpness / shutter",
	"x": "EV compensation", "g": "GPS", "q": "Quick Capture", "v": "voice control", "a": "audio", "t": "touch / tuning",
}

var lensNames = map[string]string{"N": "narrow", "M": "medium", "W": "wide", "L": "linear", "S": "SuperView", "V": "Max SuperView", "H": "linear + horizon levelling", "X": "max"}

var optionNames = map[string]string{
	"oV": "beep volume", "oB": "LCD brightness", "oS": "screen timeout", "oF": "front LCD", "oC": "auto power-off (min)",
	"oD": "tally LEDs", "oW": "wireless", "oR": "orientation", "oE": "external mic", "oL": "language", "oT": "clock",
	"oTD": "daylight saving", "oTZ": "timezone offset", "oI": "capture info overlay", "oM": "temporary metadata",
	"oN": "noise reduction",
}

var actionNames = map[string]string{
	"S": "start capture", "SQ": "start capture (approximate time)", "E": "end capture", "N": "sleep / pause", "NQ": "sleep (approximate)",
	"R": "repeat script", "RQ": "repeat script (approximate)", "L": "load saved script", "SAVE": "save script", "W": "join Wi-Fi network",
	"GS": "start live stream 480p", "GM": "start live stream 720p", "GL": "start live stream 1080p",
	"GSC": "start live stream 480p (save copy)", "GMC": "start live stream 720p (save copy)", "GLC": "start live stream 1080p (save copy)",
	"U": "upload", "C": "clear oldest media", "F": "frame grab", "B": "beep / blink", "X": "exit script", "TR": "TimeWarp real-time",
	"TN": "TimeWarp normal", "Z": "ignore buttons", "O": "shut down", "OR": "shut down and restart", "RESET": "erase permanent metadata",
	"FORMAT": "format SD card", "FRESET": "factory reset", "PRESET": "reset presets", "WRESET": "reset wireless",
}

// Explain renders one human-readable line per command.
func Explain(sc Script) []string {
	out := make([]string, 0, len(sc.Commands))
	for _, c := range sc.Commands {
		out = append(out, fmt.Sprintf("%s → %s", c.Raw, describe(c)))
	}
	return out
}

func describe(c Command) string {
	switch c.Kind {
	case ModeCmd:
		if n, ok := modeNames[c.Arg]; ok {
			return n + " mode"
		}
		return "unknown mode " + c.Arg
	case SettingCmd:
		return describeSetting(c)
	case OptionCmd:
		name := optionNames[c.Key]
		if name == "" {
			name = "option " + c.Key
		}
		return fmt.Sprintf("%s = %s", name, c.Arg)
	case Action:
		return describeAction(c)
	case Conditional:
		return "if " + describeCondition(c)
	case Bundle:
		return "also under the same condition"
	case Else:
		return "else"
	case Assignment:
		return fmt.Sprintf("set variable %s = %s", c.Key, c.Arg)
	case Metadata:
		scope := "temporary"
		if c.Permanent {
			scope = "permanent"
		}
		return fmt.Sprintf("%s metadata %s = %q", scope, c.Key, c.Arg)
	case Macro:
		return "define macro " + c.Key
	case MacroCall:
		return "run macro " + c.Arg
	case Message:
		return fmt.Sprintf("show message %q", c.Arg)
	case ExtensionCmd:
		return describeExtension(c)
	default:
		return "unrecognised"
	}
}

func describeSetting(c Command) string {
	name := settingNames[c.Key]
	if name == "" {
		name = "setting " + c.Key
	}
	switch c.Key {
	case "r":
		return "resolution " + resolutionLabel(c.Arg)
	case "p":
		if strings.HasPrefix(c.Arg, ".") {
			return "interval " + c.Arg + " s"
		}
		return "frame rate " + c.Arg + " fps"
	case "f":
		if l, ok := lensNames[c.Arg]; ok {
			return "lens " + l
		}
	case "x":
		return "EV " + c.Arg
	case "w":
		switch c.Arg {
		case "A":
			return "white balance auto"
		case "N":
			return "white balance native"
		default:
			return "white balance " + c.Arg + "00 K"
		}
	case "g", "q", "v":
		state := "on"
		if c.Arg == "0" {
			state = "off"
		}
		return name + " " + state
	}
	return name + " " + c.Arg
}

func resolutionLabel(v string) string {
	switch {
	case v == "4":
		return "4K"
	case v == "27":
		return "2.7K"
	case v == "5":
		return "5K"
	case v == "53":
		return "5.3K"
	case v == "8":
		return "8K"
	case strings.HasPrefix(v, "4") && len(v) == 2:
		return "4K " + aspect(v[1])
	case strings.HasPrefix(v, "27") && len(v) == 3:
		return "2.7K " + aspect(v[2])
	}
	return v
}

func aspect(c byte) string {
	switch c {
	case 'T':
		return "4:3"
	case 'X':
		return "8:7"
	case 'V':
		return "vertical"
	case 'S':
		return "square"
	}
	return string(c)
}

func describeAction(c Command) string {
	name := actionNames[c.Key]
	if name == "" {
		name = "action " + c.Key
	}
	if c.Key == "L" {
		name = "load saved script " + c.Arg
	}
	if c.Key == "SAVE" {
		n, body, _ := strings.Cut(c.Arg, "=")
		name = fmt.Sprintf("save script %s = %s", n, body)
	}
	if c.Arg != "" && c.Key != "L" && c.Key != "SAVE" {
		name += " " + c.Arg
	}
	switch {
	case c.When == "":
		return name
	case strings.Contains(c.When, ":"):
		return fmt.Sprintf("%s at %s", name, c.When)
	case strings.Contains(c.When, ";"):
		return fmt.Sprintf("%s aligned to every %s", name, strings.Replace(c.When, ";", ":", 1))
	case c.When[0] == 's' || c.When[0] == 'r' || c.When[0] == 'n':
		return fmt.Sprintf("%s relative to %s (%s)", name, map[byte]string{'s': "sunset", 'r': "sunrise", 'n': "noon"}[c.When[0]], c.When[1:])
	default:
		return fmt.Sprintf("%s after %s s", name, c.When)
	}
}

var stateNames = map[string]string{
	"a": "accelerometer G", "b": "battery %", "b:T": "battery temperature", "c": "distance from start (m)", "d": "GPS DOP",
	"e": "random %", "f": "SD free GB", "f:U": "SD used GB", "g": "gyro °/s", "h": "height m", "i": "ISO", "k": "speed km/h",
	"l": "loop count", "m": "motion %", "n": "network", "n:L": "live streaming", "p": "sound pressure dB", "p:T": "processor temperature",
	"r": "recording", "r:C": "remote connected", "r:A": "app connected", "s": "shutter 1/x", "t:Y": "year", "t:M": "month",
	"t:D": "day", "t:H": "hour", "t:N": "minute", "t:S": "second", "t:W": "weekday", "t:B": "seconds since boot",
	"t:Q": "seconds since QR scan", "t:R": "seconds recording", "u": "USB power", "v": "mode number", "y": "mode presses", "z": "shutter presses",
}

func describeCondition(c Command) string {
	op := map[string]string{"<": "less than", ">": "at least", "==": "equal to"}[c.Key]
	arg := c.Arg
	if arg != "" && isDigit(arg[0]) && strings.Contains(arg, ":") {
		return fmt.Sprintf("time is %s %s", op, arg)
	}
	// operand letter (+ optional :X), then threshold
	// operand: a letter, optionally ":X"; the threshold starts at the first digit
	// that does not directly follow a colon.
	i := 0
	for i < len(arg) && (isAlnum(arg[i]) || arg[i] == ':') {
		if i > 0 && isDigit(arg[i]) && arg[i-1] != ':' {
			break
		}
		i++
	}
	operand, threshold := arg[:i], arg[i:]
	if name, ok := stateNames[operand]; ok {
		return fmt.Sprintf("%s is %s %s", name, op, threshold)
	}
	return fmt.Sprintf("variable %s is %s %s", operand, op, threshold)
}

func describeExtension(c Command) string {
	ext, _ := LookupExtension(c.Key)
	scope := ""
	if c.Permanent {
		scope = ", permanent"
	}
	s := fmt.Sprintf("Labs extension %s (%s) = %s%s", ext.FourCC, ext.Description, c.Arg, scope)
	switch ext.Support {
	case Unsupported:
		s += " [not supported on HERO9]"
	case Uncertain:
		s += " [HERO9 support uncertain]"
	case Supported:
	}
	return s
}
