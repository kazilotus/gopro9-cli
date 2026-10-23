package labs

import (
	"fmt"
	"strings"
)

// Severity levels for validation issues.
const (
	SeverityError   = "error"
	SeverityWarning = "warning"
	SeverityInfo    = "info"
)

// Issue is one validation finding.
type Issue struct {
	Pos      int    `json:"pos" yaml:"pos"`
	Severity string `json:"severity" yaml:"severity"`
	Message  string `json:"message" yaml:"message"`
}

// String implements fmt.Stringer.
func (i Issue) String() string { return fmt.Sprintf("%s at %d: %s", i.Severity, i.Pos, i.Message) }

// photoModes capture a single still and therefore never use !E.
var photoModes = map[string]bool{"mP": true, "mPB": true, "mPN": true, "mNP": true, "mTP": true, "mL": true}

// videoOnlySettings make no sense in photo modes.
var videoOnlySettings = map[string]bool{"r": true, "p": true, "b": true, "e": true, "hS": true}

// knownActions lists the action keys the grammar documents.
var knownActions = map[string]bool{
	"S": true, "SQ": true, "E": true, "N": true, "NQ": true, "R": true, "RQ": true, "L": true, "SAVE": true,
	"W": true, "GS": true, "GM": true, "GL": true, "GSC": true, "GMC": true, "GLC": true, "U": true, "C": true,
	"F": true, "B": true, "X": true, "TR": true, "TN": true, "Z": true, "O": true, "OR": true,
	"RESET": true, "FORMAT": true, "FRESET": true, "PRESET": true, "WRESET": true,
}

// Validate applies the grammar rules and the HERO9 extension table.
//
// Errors: a lowercase letter after '!', !E in a photo mode, a shutdown inside a
// repeating script, an unknown action. Warnings: unsupported or unknown
// extensions, video settings in photo mode, WAKE values other than 1 on HERO9.
// Uncertain extensions produce info-level issues.
func Validate(sc Script) []Issue {
	var issues []Issue
	add := func(pos int, sev, format string, args ...any) {
		issues = append(issues, Issue{Pos: pos, Severity: sev, Message: fmt.Sprintf(format, args...)})
	}
	repeats := false
	for _, c := range sc.Commands {
		if c.Kind == Action && (c.Key == "R" || c.Key == "RQ") {
			repeats = true
		}
	}
	mode := ""
	for _, c := range sc.Commands {
		switch c.Kind {
		case ModeCmd:
			mode = c.Key
			if c.Arg == "" {
				add(c.Pos, SeverityError, "mode command %q has no mode letter", c.Raw)
			}
		case SettingCmd:
			if photoModes[mode] && videoOnlySettings[c.Key] {
				add(c.Pos, SeverityWarning, "%s is a video setting but the active mode is %s", c.Raw, mode)
			}
			if c.Arg == "" {
				add(c.Pos, SeverityError, "setting %q has no value", c.Raw)
			}
		case Action:
			validateAction(c, mode, repeats, add)
		case ExtensionCmd:
			validateExtension(c, add)
		case Metadata:
			if isFourCC(c.Key) && !strings.HasPrefix(strings.ToUpper(c.Key), "OWN") {
				add(c.Pos, SeverityWarning, "%s is not a known Labs extension; it will be stored as plain metadata", c.Key)
			}
		case Unknown:
			add(c.Pos, SeverityError, "unrecognised command %q", c.Raw)
		case OptionCmd, Conditional, Bundle, Else, Assignment, Macro, MacroCall, Message:
			// always well-formed once tokenised
		}
	}
	return issues
}

func validateAction(c Command, mode string, repeats bool, add func(int, string, string, ...any)) {
	switch {
	case c.Key == "":
		add(c.Pos, SeverityError, "%q: '!' needs an action letter", c.Raw)
	case isLower(c.Key[0]):
		add(c.Pos, SeverityError, "%q: never put a lowercase letter after '!' (did you mean !%s?)", c.Raw, strings.ToUpper(c.Key))
	case c.Key == "E" && photoModes[mode]:
		add(c.Pos, SeverityError, "%q: photo modes do not use !E; pause with !<seconds>N after !S instead", c.Raw)
	case (c.Key == "O" || c.Key == "OR") && repeats:
		add(c.Pos, SeverityError, "%q: never shut down inside a repeating script; use !<seconds>N to pause", c.Raw)
	case !knownActions[c.Key]:
		add(c.Pos, SeverityError, "unknown action %q", c.Raw)
	}
}

func validateExtension(c Command, add func(int, string, string, ...any)) {
	ext, ok := LookupExtension(c.Key)
	if !ok {
		add(c.Pos, SeverityWarning, "%s is not a known Labs extension", c.Key)
		return
	}
	switch ext.Support {
	case Unsupported:
		add(c.Pos, SeverityWarning, "%s (%s) is not supported on the HERO9 Black", ext.FourCC, ext.Description)
	case Uncertain:
		add(c.Pos, SeverityInfo, "%s (%s) has inconsistent HERO9 support in GoPro's documentation; try it on the camera", ext.FourCC, ext.Description)
	case Supported:
		if ext.FourCC == "WAKE" && c.Arg != "1" && c.Arg != "0" && c.Arg != "" {
			add(c.Pos, SeverityWarning, "WAKE=%s: the HERO9 only supports WAKE=1", c.Arg)
		}
	}
}

// Errors returns only the error-severity issues.
func Errors(issues []Issue) []Issue {
	var out []Issue
	for _, i := range issues {
		if i.Severity == SeverityError {
			out = append(out, i)
		}
	}
	return out
}
