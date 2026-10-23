package main

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// supportsHERO9 reports whether an x-supported-cameras list names the HERO9 Black.
func supportsHERO9(cameras []string) bool {
	for _, c := range cameras {
		if c == hero9 {
			return true
		}
	}
	return false
}

var replacements = []struct{ from, to string }{
	{"+", " Plus "}, {"%", " Pct "}, {"°", " Deg "}, {"&", " And "}, {"/", " "}, {"@", " At "},
}

// pascal turns a human label into an exported Go identifier fragment.
// "2.7K 4:3" -> "2_7K4x3", "Anti-Flicker" -> "AntiFlicker", "-2.0" -> "Minus2_0".
func pascal(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "-") {
		s = "Minus " + s[1:]
	}
	for _, r := range replacements {
		s = strings.ReplaceAll(s, r.from, r.to)
	}
	s = strings.ReplaceAll(s, ":", "x")
	// decimal point between digits -> underscore
	s = regexp.MustCompile(`(\d)\.(\d)`).ReplaceAllString(s, "${1}_${2}")
	var b strings.Builder
	upperNext := true
	for _, r := range s {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			if upperNext {
				r = unicode.ToUpper(r)
			}
			b.WriteRune(r)
			upperNext = false
		case r == '_':
			b.WriteRune(r)
			upperNext = true
		default:
			upperNext = true
		}
	}
	out := b.String()
	if out == "" {
		return "Unnamed"
	}
	if unicode.IsDigit(rune(out[0])) {
		out = "N" + out // identifiers cannot start with a digit (used for status field names)
	}
	return out
}

// optionIdent names an option constant fragment. Unlike pascal it may start
// with a digit because it is always suffixed to a type name. It drops a
// redundant ".0" ("30.0" -> "30") and keeps only the part of a label before
// a colon or parenthesis, capped at six words, so long explanatory labels
// such as "Camera Idle: No one is attempting to change camera settings"
// become "CameraIdle".
func optionIdent(label string) string {
	label = strings.TrimSpace(label)
	// Cut explanatory tails ("Camera Idle: No one ...", "Auto (default)") but
	// keep aspect ratios such as "4:3", where the colon is between digits.
	for _, sep := range []string{": ", " (", " - "} {
		if i := strings.Index(label, sep); i > 0 {
			label = strings.TrimSpace(label[:i])
		}
	}
	if words := strings.Fields(label); len(words) > 6 {
		label = strings.Join(words[:6], " ")
	}
	label = strings.TrimSuffix(label, ".0")
	out := pascal(label)
	if label != "" && unicode.IsDigit(rune(label[0])) {
		out = strings.TrimPrefix(out, "N")
	}
	return out
}

// kebab turns a human label into a CLI-friendly lowercase token.
// "Video Resolution" -> "video-resolution", "2.7K 4:3" -> "2.7k-4-3".
func kebab(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	dash := false
	for _, r := range s {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.':
			b.WriteRune(r)
			dash = false
		default:
			if !dash && b.Len() > 0 {
				b.WriteRune('-')
				dash = true
			}
		}
	}
	return strings.TrimRight(b.String(), "-")
}

var boldRe = regexp.MustCompile(`^\*\*(.+?)\*\*`)

func boldTitle(desc string) string {
	m := boldRe.FindStringSubmatch(strings.TrimSpace(desc))
	if m == nil {
		return ""
	}
	return strings.TrimSpace(m[1])
}

func stripBold(desc string) string {
	return strings.TrimSpace(boldRe.ReplaceAllString(strings.TrimSpace(desc), ""))
}

// firstParagraph returns the first non-table paragraph of a markdown description.
func firstParagraph(desc string) string {
	for _, para := range strings.Split(strings.ReplaceAll(desc, "\r\n", "\n"), "\n\n") {
		p := strings.TrimSpace(para)
		if p == "" || strings.HasPrefix(p, "|") {
			continue
		}
		return strings.Join(strings.Fields(p), " ")
	}
	return ""
}

type tableRow struct {
	Value int64
	Label string
}

var rowRe = regexp.MustCompile(`^\|\s*(-?\d+)\s*\|\s*([^|]+?)\s*\|`)

// parseValueTable extracts "| Value | Meaning |" rows from a markdown description.
func parseValueTable(desc string) []tableRow {
	var rows []tableRow
	inTable := false
	for _, line := range strings.Split(desc, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "| Value") {
			inTable = true
			continue
		}
		if !inTable {
			continue
		}
		if !strings.HasPrefix(line, "|") {
			break
		}
		m := rowRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		v, err := strconv.ParseInt(m[1], 10, 64)
		if err != nil {
			continue
		}
		rows = append(rows, tableRow{Value: v, Label: m[2]})
	}
	return rows
}

var rangeRe = regexp.MustCompile(`(\d+)\s*-\s*(\d+)`)

func parseRange(desc string) (uint32, uint32) {
	m := rangeRe.FindStringSubmatch(desc)
	if m == nil {
		return 0, 0
	}
	lo, _ := strconv.ParseUint(m[1], 10, 32)
	hi, _ := strconv.ParseUint(m[2], 10, 32)
	if hi <= lo {
		return 0, 0
	}
	return uint32(lo), uint32(hi)
}
