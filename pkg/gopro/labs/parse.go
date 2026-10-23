package labs

import (
	"errors"
	"fmt"
	"strings"
)

// Kind classifies a parsed command.
type Kind int

// Command kinds.
const (
	Unknown      Kind = iota // unrecognised leading character
	ModeCmd                  // mV, mP, mTW ...
	SettingCmd               // r4, p60, fW, x-.5 ...
	OptionCmd                // oV1, oB4, oT..., oF...
	Action                   // !S, !10E, !12:00N, !Lname, !SAVEname=...
	Conditional              // <08:45, >T1, ==v12
	Bundle                   // + (bundle the next command under the current condition)
	Else                     // ~ (else branch)
	Assignment               // =A5, =E/G, =Tt:W
	Metadata                 // *OWNR="x", $FOUR=1, !MFOUR=1, oMFOUR=1 (non-extension FourCCs)
	Macro                    // *NAME="script"
	MacroCall                // ^NAME
	Message                  // "text with $VAR"
	ExtensionCmd             // $BITR=150, *HIST=5, !MHSTO=30 (Labs extension FourCCs)
)

var kindNames = [...]string{"unknown", "mode", "setting", "option", "action", "conditional", "bundle", "else", "assignment", "metadata", "macro", "macro-call", "message", "extension"}

// String implements fmt.Stringer.
func (k Kind) String() string {
	if int(k) < len(kindNames) && k >= 0 {
		return kindNames[k]
	}
	return "unknown"
}

// MarshalText implements encoding.TextMarshaler.
func (k Kind) MarshalText() ([]byte, error) { return []byte(k.String()), nil }

// Command is one parsed unit of a Labs script.
type Command struct {
	Kind Kind   `json:"kind" yaml:"kind"`
	Raw  string `json:"raw" yaml:"raw"`
	// Key is the command's identifying letter(s): "mV", "r", "oV", "S" (for !S),
	// ">" (conditional operator), "A" (assigned variable), FourCC for metadata.
	Key string `json:"key,omitempty" yaml:"key,omitempty"`
	// Arg is the value or operand: "4" for r4, "1" for oV1, "bt" for !Lbt.
	Arg string `json:"arg,omitempty" yaml:"arg,omitempty"`
	// When is an action's time prefix: "10" (seconds), "12:00", "00;05", "s-30".
	When string `json:"when,omitempty" yaml:"when,omitempty"`
	// Permanent marks metadata stored across power cycles (* or !M forms).
	Permanent bool `json:"permanent,omitempty" yaml:"permanent,omitempty"`
	Pos       int  `json:"pos" yaml:"pos"`
}

// Script is a parsed command string.
type Script struct {
	Source   string    `json:"source" yaml:"source"`
	Commands []Command `json:"commands" yaml:"commands"`
}

// Sentinel errors.
var (
	ErrSyntax      = errors.New("labs: syntax error")
	ErrUnsupported = errors.New("labs: not supported on HERO9 Black")
)

// Parse tokenises a Labs command string. It is tolerant: unrecognised
// characters become Unknown commands rather than errors, and it never panics.
// Unbalanced quotes and empty input return an ErrSyntax-wrapped error.
func Parse(s string) (Script, error) {
	sc := Script{Source: s}
	if strings.TrimSpace(s) == "" {
		return sc, fmt.Errorf("%w: empty command", ErrSyntax)
	}
	p := &parser{src: s}
	for p.i < len(p.src) {
		start := p.i
		c := p.src[p.i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			p.i++
			continue
		case c == '"':
			if err := p.message(); err != nil {
				return sc, err
			}
		case c == '!':
			if err := p.action(); err != nil {
				return sc, err
			}
		case c == '$':
			if err := p.dollar(); err != nil {
				return sc, err
			}
		case c == '*':
			if err := p.star(); err != nil {
				return sc, err
			}
		case c == '^':
			p.macroCall()
		case c == '=':
			p.assignOrEquals()
		case c == '<' || c == '>':
			p.i++
			p.conditional(string(c), start)
		case c == '+':
			p.i++
			p.emit(Command{Kind: Bundle, Raw: "+", Key: "+", Pos: start})
		case c == '~':
			p.i++
			p.emit(Command{Kind: Else, Raw: "~", Key: "~", Pos: start})
		case c == 'o':
			if err := p.option(); err != nil {
				return sc, err
			}
		case c == 'm':
			p.mode()
		case isLower(c):
			p.setting()
		default:
			p.i++
			p.emit(Command{Kind: Unknown, Raw: string(c), Key: string(c), Pos: start})
		}
		if p.i == start { // defensive: guarantee progress
			p.i++
		}
	}
	sc.Commands = p.cmds
	return sc, nil
}

type parser struct {
	src  string
	i    int
	cmds []Command
}

func (p *parser) emit(c Command) { p.cmds = append(p.cmds, c) }

func (p *parser) peek() byte {
	if p.i < len(p.src) {
		return p.src[p.i]
	}
	return 0
}

// take consumes characters while pred holds and returns them.
func (p *parser) take(pred func(byte) bool) string {
	start := p.i
	for p.i < len(p.src) && pred(p.src[p.i]) {
		p.i++
	}
	return p.src[start:p.i]
}

func isLower(c byte) bool   { return c >= 'a' && c <= 'z' }
func isUpper(c byte) bool   { return c >= 'A' && c <= 'Z' }
func isDigit(c byte) bool   { return c >= '0' && c <= '9' }
func isAlnum(c byte) bool   { return isLower(c) || isUpper(c) || isDigit(c) }
func isNumeric(c byte) bool { return isDigit(c) || c == '.' || c == '-' }

// isValueChar continues a setting/option value: uppercase letters, digits, sign, dot.
func isValueChar(c byte) bool { return isUpper(c) || isNumeric(c) }

// quoted consumes a double-quoted string starting at the current '"' and
// returns its contents (quotes stripped). Single quotes may appear inside.
func (p *parser) quoted() (string, error) {
	start := p.i
	p.i++ // opening quote
	end := strings.IndexByte(p.src[p.i:], '"')
	if end < 0 {
		return "", fmt.Errorf("%w: unbalanced quote at position %d", ErrSyntax, start)
	}
	s := p.src[p.i : p.i+end]
	p.i += end + 1
	return s, nil
}

func (p *parser) message() error {
	start := p.i
	text, err := p.quoted()
	if err != nil {
		return err
	}
	p.emit(Command{Kind: Message, Raw: p.src[start:p.i], Key: `"`, Arg: text, Pos: start})
	return nil
}

// value consumes a metadata/extension value: a quoted string, or a run of
// numbers and commas.
func (p *parser) value() (string, error) {
	if p.peek() == '"' {
		return p.quoted()
	}
	return p.take(func(c byte) bool { return isNumeric(c) || c == ',' }), nil
}

func isFourCC(s string) bool {
	if len(s) != 4 {
		return false
	}
	for i := range len(s) {
		if !isAlnum(s[i]) {
			return false
		}
	}
	return true
}

// metadataKind classifies a FourCC assignment as Extension or Metadata.
func metadataKind(fourcc string) Kind {
	if _, ok := LookupExtension(fourcc); ok {
		return ExtensionCmd
	}
	return Metadata
}

func (p *parser) action() error {
	start := p.i
	p.i++ // '!'
	// Time prefix: digits/colon/semicolon/dot, or sun-relative s/r/n with an optional signed offset.
	when := ""
	if c := p.peek(); c == 's' || c == 'r' || c == 'n' {
		if p.i+1 < len(p.src) && (isNumeric(p.src[p.i+1]) || isUpper(p.src[p.i+1])) {
			p.i++
			when = string(c) + p.take(isNumeric)
		}
	}
	if when == "" {
		when = p.take(func(c byte) bool { return isDigit(c) || c == ':' || c == ';' || c == '.' })
	}
	switch c := p.peek(); {
	case c == 0:
		p.emit(Command{Kind: Action, Raw: p.src[start:p.i], Key: "", When: when, Pos: start})
		return nil
	case isLower(c):
		// Grammar violation ("never put a lowercase letter after !"); keep for Validate.
		p.i++
		p.emit(Command{Kind: Action, Raw: p.src[start:p.i], Key: string(c), When: when, Pos: start})
		return nil
	case c == 'M' && p.i+5 <= len(p.src) && isFourCC(p.src[p.i+1:p.i+5]) && p.i+5 < len(p.src) && p.src[p.i+5] == '=':
		fourcc := p.src[p.i+1 : p.i+5]
		p.i += 6
		v, err := p.value()
		if err != nil {
			return err
		}
		p.emit(Command{Kind: metadataKind(fourcc), Raw: p.src[start:p.i], Key: fourcc, Arg: v, Permanent: true, Pos: start})
		return nil
	case strings.HasPrefix(p.src[p.i:], "SAVE"):
		p.i += 4
		name := p.take(isAlnum)
		if p.peek() != '=' {
			return fmt.Errorf("%w: !SAVE%s needs '=script' at position %d", ErrSyntax, name, start)
		}
		p.i++
		script := p.src[p.i:]
		p.i = len(p.src)
		p.emit(Command{Kind: Action, Raw: p.src[start:p.i], Key: "SAVE", Arg: name + "=" + script, When: when, Pos: start})
		return nil
	case c == 'L' && !strings.HasPrefix(p.src[p.i:], "LC"):
		p.i++
		name := p.take(isAlnum)
		p.emit(Command{Kind: Action, Raw: p.src[start:p.i], Key: "L", Arg: name, When: when, Pos: start})
		return nil
	}
	key := p.take(isUpper)
	arg := p.take(isNumeric)
	p.emit(Command{Kind: Action, Raw: p.src[start:p.i], Key: key, Arg: arg, When: when, Pos: start})
	return nil
}

func (p *parser) dollar() error {
	start := p.i
	p.i++ // '$'
	name := p.take(isAlnum)
	if p.peek() == '=' && isFourCC(name) {
		p.i++
		v, err := p.value()
		if err != nil {
			return err
		}
		p.emit(Command{Kind: metadataKind(name), Raw: p.src[start:p.i], Key: name, Arg: v, Pos: start})
		return nil
	}
	p.emit(Command{Kind: Unknown, Raw: p.src[start:p.i], Key: "$", Arg: name, Pos: start})
	return nil
}

func (p *parser) star() error {
	start := p.i
	p.i++ // '*'
	name := p.take(isAlnum)
	if p.peek() != '=' || name == "" {
		if p.peek() == '=' {
			p.i++
			p.take(isNumeric)
		}
		p.emit(Command{Kind: Unknown, Raw: p.src[start:p.i], Key: "*", Arg: name, Pos: start})
		return nil
	}
	p.i++
	quoted := p.peek() == '"'
	v, err := p.value()
	if err != nil {
		return err
	}
	raw := p.src[start:p.i]
	if isFourCC(name) {
		if _, ok := LookupExtension(name); ok {
			p.emit(Command{Kind: ExtensionCmd, Raw: raw, Key: name, Arg: v, Permanent: true, Pos: start})
			return nil
		}
		if !quoted || !looksLikeScript(v) {
			p.emit(Command{Kind: Metadata, Raw: raw, Key: name, Arg: v, Permanent: true, Pos: start})
			return nil
		}
	}
	if quoted {
		p.emit(Command{Kind: Macro, Raw: raw, Key: name, Arg: v, Permanent: true, Pos: start})
		return nil
	}
	p.emit(Command{Kind: Metadata, Raw: raw, Key: name, Arg: v, Permanent: true, Pos: start})
	return nil
}

// looksLikeScript guesses whether a quoted value is a macro body rather than
// free text: it starts with a command character and contains no spaces.
func looksLikeScript(v string) bool {
	if v == "" || strings.ContainsAny(v, " \t") {
		return false
	}
	c := v[0]
	return isLower(c) || strings.IndexByte(`!$*=<>^+~"`, c) >= 0
}

func (p *parser) macroCall() {
	start := p.i
	p.i++ // '^'
	name := p.take(isAlnum)
	p.emit(Command{Kind: MacroCall, Raw: p.src[start:p.i], Key: "^", Arg: name, Pos: start})
}

func (p *parser) assignOrEquals() {
	start := p.i
	p.i++ // '='
	if p.peek() == '=' {
		p.i++
		p.conditional("==", start)
		return
	}
	if !isAlnum(p.peek()) || isDigit(p.peek()) {
		p.take(isNumeric)
		p.emit(Command{Kind: Unknown, Raw: p.src[start:p.i], Key: "=", Pos: start})
		return
	}
	variable := string(p.src[p.i])
	p.i++
	expr := p.expression()
	p.emit(Command{Kind: Assignment, Raw: p.src[start:p.i], Key: variable, Arg: expr, Pos: start})
}

// expression consumes an assignment right-hand side: an optional operator
// followed by a number, a variable/state letter with an optional :X qualifier,
// or a $FOURCC read.
func (p *parser) expression() string {
	start := p.i
	if c := p.peek(); strings.IndexByte("+-*/^%#&|", c) >= 0 && c != 0 {
		p.i++
	}
	p.operand()
	return p.src[start:p.i]
}

// operand consumes a number, a letter (with optional :X qualifier), a time
// literal, or a $FOURCC reference.
func (p *parser) operand() {
	switch c := p.peek(); {
	case c == '$':
		p.i++
		p.take(isAlnum)
	case isNumeric(c):
		p.take(func(c byte) bool { return isNumeric(c) || c == ':' })
	case isAlnum(c):
		p.i++
		if p.peek() == ':' && p.i+1 < len(p.src) && isUpper(p.src[p.i+1]) {
			p.i += 2
		}
	}
}

func (p *parser) conditional(op string, start int) {
	// operand then optional numeric threshold
	p.operand()
	p.take(isNumeric)
	p.emit(Command{Kind: Conditional, Raw: p.src[start:p.i], Key: op, Arg: p.src[start+len(op) : p.i], Pos: start})
}

// letterValued option letters take a single uppercase letter as their value.
const letterValued = "FS"

func (p *parser) option() error {
	start := p.i
	p.i++ // 'o'
	if !isUpper(p.peek()) {
		p.emit(Command{Kind: Unknown, Raw: p.src[start:p.i], Key: "o", Pos: start})
		return nil
	}
	for isUpper(p.peek()) {
		optStart := p.i
		letter := string(p.src[p.i])
		p.i++
		switch letter {
		case "M": // temporary metadata oMFOUR=value
			name := p.take(isAlnum)
			if p.peek() == '=' {
				p.i++
				v, err := p.value()
				if err != nil {
					return err
				}
				p.emit(Command{Kind: metadataKind(name), Raw: "o" + p.src[optStart:p.i], Key: name, Arg: v, Pos: optStart})
				return nil
			}
			p.emit(Command{Kind: OptionCmd, Raw: "o" + p.src[optStart:p.i], Key: "oM", Arg: name, Pos: optStart})
		case "T": // clock: oTyymmddhhmmss.sss | oTD1 | oTZ-7
			switch p.peek() {
			case 'D', 'Z':
				sub := string(p.src[p.i])
				p.i++
				v := p.take(isNumeric)
				p.emit(Command{Kind: OptionCmd, Raw: "o" + p.src[optStart:p.i], Key: "oT" + sub, Arg: v, Pos: optStart})
			default:
				v := p.take(isNumeric)
				p.emit(Command{Kind: OptionCmd, Raw: "o" + p.src[optStart:p.i], Key: "oT", Arg: v, Pos: optStart})
			}
		default:
			var v string
			if strings.Contains(letterValued, letter) && isUpper(p.peek()) {
				v = string(p.src[p.i])
				p.i++
				v += p.take(isNumeric)
			} else {
				v = p.take(isNumeric)
			}
			p.emit(Command{Kind: OptionCmd, Raw: "o" + p.src[optStart:p.i], Key: "o" + letter, Arg: v, Pos: optStart})
		}
	}
	return nil
}

func (p *parser) mode() {
	start := p.i
	p.i++ // 'm'
	m := p.take(func(c byte) bool { return isUpper(c) || c == '3' })
	p.emit(Command{Kind: ModeCmd, Raw: p.src[start:p.i], Key: "m" + m, Arg: m, Pos: start})
}

func (p *parser) setting() {
	start := p.i
	key := string(p.src[p.i])
	p.i++
	// Two-letter keys: hS (HindSight), eL (exposure lock), tE/tD/tW... take the
	// uppercase letter as part of the key when followed by digits.
	if key == "h" && p.peek() == 'S' {
		p.i++
		key = "hS"
	} else if key == "e" && p.peek() == 'L' {
		p.i++
		key = "eL"
	}
	v := p.take(isValueChar)
	p.emit(Command{Kind: SettingCmd, Raw: p.src[start:p.i], Key: key, Arg: v, Pos: start})
}
