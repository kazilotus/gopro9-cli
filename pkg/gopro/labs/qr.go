package labs

import (
	"fmt"
	"strings"

	"github.com/skip2/go-qrcode"
)

// QRPNG renders the command as a PNG of size×size pixels (quiet zone included).
func QRPNG(s string, size int) ([]byte, error) {
	if size <= 0 {
		size = 512
	}
	q, err := qrcode.New(s, qrcode.Medium)
	if err != nil {
		return nil, fmt.Errorf("labs: qr: %w", err)
	}
	return q.PNG(size)
}

// QRSVG renders the command as a scalable SVG with a 4-module quiet zone.
func QRSVG(s string) (string, error) {
	bm, err := bitmap(s)
	if err != nil {
		return "", err
	}
	n := len(bm)
	var sb strings.Builder
	fmt.Fprintf(&sb, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" shape-rendering="crispEdges">`, n, n)
	fmt.Fprintf(&sb, `<rect width="%d" height="%d" fill="#fff"/>`, n, n)
	sb.WriteString(`<path fill="#000" d="`)
	for y, row := range bm {
		for x, dark := range row {
			if dark {
				fmt.Fprintf(&sb, "M%d %dh1v1h-1z", x, y)
			}
		}
	}
	sb.WriteString(`"/></svg>`)
	return sb.String(), nil
}

// QRANSI renders the command for a terminal using half-block characters, two
// modules per line. invert swaps dark and light for dark terminal themes.
func QRANSI(s string, invert bool) (string, error) {
	bm, err := bitmap(s)
	if err != nil {
		return "", err
	}
	n := len(bm)
	at := func(x, y int) bool {
		if y >= n {
			return false
		}
		return bm[y][x] != invert
	}
	var sb strings.Builder
	for y := 0; y < n; y += 2 {
		for x := range n {
			top, bottom := at(x, y), at(x, y+1)
			switch {
			case top && bottom:
				sb.WriteRune('█')
			case top:
				sb.WriteRune('▀')
			case bottom:
				sb.WriteRune('▄')
			default:
				sb.WriteByte(' ')
			}
		}
		sb.WriteByte('\n')
	}
	return sb.String(), nil
}

// bitmap returns the module matrix including the quiet zone.
func bitmap(s string) ([][]bool, error) {
	q, err := qrcode.New(s, qrcode.Medium)
	if err != nil {
		return nil, fmt.Errorf("labs: qr: %w", err)
	}
	return q.Bitmap(), nil
}
