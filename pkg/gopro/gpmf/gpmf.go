package gpmf

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

// Sentinel errors. Parse wraps them with position detail; use errors.Is.
var (
	ErrTruncated = errors.New("gpmf: truncated input")
	ErrInvalid   = errors.New("gpmf: invalid item")
)

// Item is one KLV element. Nested containers (Type == 0) carry Children and
// an empty Raw; every other item carries its unpadded payload in Raw.
type Item struct {
	Key      string
	Type     byte
	Size     uint8
	Repeat   uint16
	Raw      []byte
	Children []Item
}

const headerLen = 8

// elemSize returns the byte width of one element of a simple type, or 0 for
// unknown types. Complex ('?') and nested (0) types have no fixed element size.
func elemSize(t byte) int {
	switch t {
	case 'b', 'B', 'c':
		return 1
	case 's', 'S', 'H':
		return 2
	case 'l', 'L', 'f', 'F', 'q':
		return 4
	case 'd', 'j', 'J', 'Q':
		return 8
	case 'G', 'U':
		return 16
	default:
		return 0
	}
}

func knownType(t byte) bool { return t == 0 || t == '?' || elemSize(t) > 0 }

func padded(n int) int { return (n + 3) &^ 3 }

// Parse decodes a KLV sequence into a tree. Trailing zero bytes are treated
// as padding. On error the items decoded before the fault are returned along
// with a wrapped ErrTruncated or ErrInvalid.
func Parse(data []byte) ([]Item, error) {
	return parse(data, 0)
}

const maxDepth = 16

func parse(data []byte, depth int) ([]Item, error) {
	var items []Item
	off := 0
	for off < len(data) {
		if allZero(data[off:]) {
			return items, nil // padding
		}
		if len(data)-off < headerLen {
			return items, fmt.Errorf("%w: %d header bytes at offset %d", ErrTruncated, len(data)-off, off)
		}
		key := string(data[off : off+4])
		typ := data[off+4]
		size := data[off+5]
		repeat := binary.BigEndian.Uint16(data[off+6 : off+8])
		if !validKey(key) {
			return items, fmt.Errorf("%w: key %q at offset %d", ErrInvalid, key, off)
		}
		if !knownType(typ) {
			return items, fmt.Errorf("%w: key %s has unknown type %q at offset %d", ErrInvalid, key, typ, off)
		}
		payloadLen := int(size) * int(repeat)
		start := off + headerLen
		if start+payloadLen > len(data) {
			return items, fmt.Errorf("%w: key %s needs %d payload bytes, %d available", ErrTruncated, key, payloadLen, len(data)-start)
		}
		it := Item{Key: key, Type: typ, Size: size, Repeat: repeat}
		payload := data[start : start+payloadLen]
		if typ == 0 {
			if depth >= maxDepth {
				return items, fmt.Errorf("%w: nesting deeper than %d at key %s", ErrInvalid, maxDepth, key)
			}
			children, err := parse(payload, depth+1)
			it.Children = children
			items = append(items, it)
			if err != nil {
				return items, err
			}
		} else {
			if es := elemSize(typ); es > 0 && size != 0 && int(size)%es != 0 {
				return items, fmt.Errorf("%w: key %s size %d is not a multiple of element size %d", ErrInvalid, key, size, es)
			}
			it.Raw = payload
			items = append(items, it)
		}
		off = start + padded(payloadLen)
	}
	return items, nil
}

func validKey(k string) bool {
	for i := range len(k) {
		if k[i] < 0x20 || k[i] > 0x7e {
			return false
		}
	}
	return true
}

func allZero(b []byte) bool {
	for _, c := range b {
		if c != 0 {
			return false
		}
	}
	return true
}

// Find returns the first direct child with the given key.
func (it Item) Find(key string) (Item, bool) {
	for _, c := range it.Children {
		if c.Key == key {
			return c, true
		}
	}
	return Item{}, false
}

// String decodes a character ('c') or UTC ('U') payload as text with trailing
// NUL bytes and spaces removed. Other types return "".
func (it Item) String() string {
	if it.Type != 'c' && it.Type != 'U' {
		return ""
	}
	return strings.TrimRight(string(it.Raw), "\x00 ")
}

// Strings splits a character payload into Repeat strings of Size bytes each,
// which is how SIUN/UNIT lists units. Other types return nil.
func (it Item) Strings() []string {
	if it.Type != 'c' || it.Size == 0 {
		return nil
	}
	out := make([]string, 0, it.Repeat)
	for i := 0; i+int(it.Size) <= len(it.Raw); i += int(it.Size) {
		out = append(out, strings.TrimRight(string(it.Raw[i:i+int(it.Size)]), "\x00 "))
	}
	return out
}

// Numbers decodes the payload as Repeat samples of numeric columns.
//
// For simple types the column count is Size/elemSize. For complex items
// (Type '?') typeDef is the TYPE string describing one sample; non-numeric
// columns decode as NaN. UTC strings (type 'U') decode as Unix seconds.
// Nested, character and malformed items return nil.
func (it Item) Numbers(typeDef string) [][]float64 {
	switch {
	case it.Type == 0 || it.Type == 'c' || it.Size == 0:
		return nil
	case it.Type == '?':
		return it.complexNumbers(typeDef)
	}
	es := elemSize(it.Type)
	cols := int(it.Size) / es
	if cols == 0 {
		return nil
	}
	out := make([][]float64, 0, it.Repeat)
	for r := 0; r < int(it.Repeat) && (r+1)*int(it.Size) <= len(it.Raw); r++ {
		row := make([]float64, cols)
		for c := range cols {
			off := r*int(it.Size) + c*es
			row[c] = decode(it.Type, it.Raw[off:off+es])
		}
		out = append(out, row)
	}
	return out
}

func (it Item) complexNumbers(typeDef string) [][]float64 {
	total := 0
	for i := range len(typeDef) {
		es := elemSize(typeDef[i])
		if es == 0 {
			return nil
		}
		total += es
	}
	if total == 0 || total != int(it.Size) {
		return nil
	}
	out := make([][]float64, 0, it.Repeat)
	for r := 0; r < int(it.Repeat) && (r+1)*total <= len(it.Raw); r++ {
		row := make([]float64, len(typeDef))
		off := r * total
		for c := range len(typeDef) {
			es := elemSize(typeDef[c])
			row[c] = decode(typeDef[c], it.Raw[off:off+es])
			off += es
		}
		out = append(out, row)
	}
	return out
}

// decode converts one big-endian element to float64. Non-numeric types yield NaN.
func decode(t byte, b []byte) float64 {
	switch t {
	case 'b':
		return float64(int8(b[0]))
	case 'B':
		return float64(b[0])
	case 's':
		return float64(int16(binary.BigEndian.Uint16(b)))
	case 'S':
		return float64(binary.BigEndian.Uint16(b))
	case 'l':
		return float64(int32(binary.BigEndian.Uint32(b)))
	case 'L':
		return float64(binary.BigEndian.Uint32(b))
	case 'f':
		return float64(math.Float32frombits(binary.BigEndian.Uint32(b)))
	case 'd':
		return math.Float64frombits(binary.BigEndian.Uint64(b))
	case 'j':
		return float64(int64(binary.BigEndian.Uint64(b)))
	case 'J':
		return float64(binary.BigEndian.Uint64(b))
	case 'q':
		return float64(int32(binary.BigEndian.Uint32(b))) / 65536
	case 'Q':
		return float64(int64(binary.BigEndian.Uint64(b))) / 4294967296
	case 'U':
		t, err := ParseUTC(string(b))
		if err != nil {
			return math.NaN()
		}
		return float64(t.UnixNano()) / 1e9
	default: // 'H', 'F', 'G', 'c' and unknown
		return math.NaN()
	}
}

// ParseUTC parses the GPMF UTC date format "yymmddhhmmss.sss".
func ParseUTC(s string) (time.Time, error) {
	s = strings.TrimRight(s, "\x00 ")
	t, err := time.Parse("060102150405.000", s)
	if err != nil {
		return time.Time{}, fmt.Errorf("gpmf: bad UTC string %q: %w", s, err)
	}
	return t.UTC(), nil
}

// DeviceInfo returns the DVID and DVNM of the first DEVC container found.
func DeviceInfo(items []Item) (uint32, string) {
	for _, it := range items {
		if it.Key != "DEVC" {
			continue
		}
		var id uint32
		var name string
		if d, ok := it.Find("DVID"); ok {
			if n := d.Numbers(""); len(n) > 0 && len(n[0]) > 0 && !math.IsNaN(n[0][0]) {
				id = uint32(n[0][0])
			} else if d.Type == 'c' || d.Type == 'F' {
				id = binary.BigEndian.Uint32(padTo4(d.Raw))
			}
		}
		if n, ok := it.Find("DVNM"); ok {
			name = n.String()
		}
		return id, name
	}
	return 0, ""
}

func padTo4(b []byte) []byte {
	out := make([]byte, 4)
	copy(out, b)
	return out
}
