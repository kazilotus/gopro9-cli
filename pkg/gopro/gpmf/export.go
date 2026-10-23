package gpmf

import (
	"encoding/csv"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"strconv"
	"time"
)

type gpxTrkpt struct {
	Lat  float64 `xml:"lat,attr"`
	Lon  float64 `xml:"lon,attr"`
	Ele  float64 `xml:"ele"`
	Time string  `xml:"time,omitempty"`
	Ext  *gpxExt `xml:"extensions,omitempty"`
}

type gpxExt struct {
	Speed   float64 `xml:"speed"`
	Speed3D float64 `xml:"speed3d"`
	Fix     uint32  `xml:"fix"`
	DOP     float64 `xml:"dop"`
}

type gpxDoc struct {
	XMLName xml.Name `xml:"gpx"`
	Version string   `xml:"version,attr"`
	Creator string   `xml:"creator,attr"`
	Xmlns   string   `xml:"xmlns,attr"`
	Trk     struct {
		Name   string `xml:"name"`
		Trkseg struct {
			Trkpt []gpxTrkpt `xml:"trkpt"`
		} `xml:"trkseg"`
	} `xml:"trk"`
}

// WriteGPX writes fixes as a GPX 1.1 track. Speeds (m/s), fix type and
// dilution of precision go into per-point <extensions>.
func WriteGPX(w io.Writer, fixes []GPSFix, name string) error {
	doc := gpxDoc{Version: "1.1", Creator: "gopro9-cli", Xmlns: "http://www.topografix.com/GPX/1/1"}
	doc.Trk.Name = name
	doc.Trk.Trkseg.Trkpt = make([]gpxTrkpt, 0, len(fixes))
	for _, f := range fixes {
		pt := gpxTrkpt{Lat: f.Lat, Lon: f.Lon, Ele: f.Alt, Ext: &gpxExt{Speed: f.Speed2D, Speed3D: f.Speed3D, Fix: f.Fix, DOP: float64(f.Precision) / 100}}
		if !f.Time.IsZero() {
			pt.Time = f.Time.UTC().Format(time.RFC3339Nano)
		}
		doc.Trk.Trkseg.Trkpt = append(doc.Trk.Trkseg.Trkpt, pt)
	}
	if _, err := io.WriteString(w, xml.Header); err != nil {
		return err
	}
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return fmt.Errorf("gpmf: encode gpx: %w", err)
	}
	if err := enc.Close(); err != nil {
		return err
	}
	_, err := io.WriteString(w, "\n")
	return err
}

// WriteCSV writes one row per sample: payload index, sample index within the
// stream, then one column per value named after Units (or colN). NaN values
// are written as empty cells.
func WriteCSV(w io.Writer, s *Stream) error {
	cw := csv.NewWriter(w)
	header := []string{"payload", "index"}
	for c := range s.Columns {
		switch {
		case len(s.Units) == 1 && s.Columns > 1 && s.Units[0] != "":
			header = append(header, fmt.Sprintf("%s_%d", s.Units[0], c))
		case c < len(s.Units) && s.Units[c] != "":
			header = append(header, s.Units[c])
		default:
			header = append(header, "col"+strconv.Itoa(c))
		}
	}
	if len(s.Text) > 0 {
		header = append(header, "text")
	}
	if err := cw.Write(header); err != nil {
		return err
	}
	for i, row := range s.Samples {
		rec := make([]string, 0, len(header))
		rec = append(rec, strconv.Itoa(s.PayloadOf(i)), strconv.Itoa(i))
		for c := range s.Columns {
			if c >= len(row) || math.IsNaN(row[c]) {
				rec = append(rec, "")
				continue
			}
			rec = append(rec, strconv.FormatFloat(row[c], 'f', -1, 64))
		}
		if len(s.Text) > 0 {
			txt := ""
			if i < len(s.Text) {
				txt = s.Text[i]
			}
			rec = append(rec, txt)
		}
		if err := cw.Write(rec); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}
