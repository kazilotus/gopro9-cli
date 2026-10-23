// Package media lists, downloads, inspects and deletes files on the camera's
// SD card through the Open GoPro media endpoints and the DCIM file server.
package media

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/kazilotus/gopro9-cli/pkg/gopro/transport"
)

// Client issues media operations.
type Client struct{ t *transport.Client }

// New wraps a transport client.
func New(t *transport.Client) *Client { return &Client{t: t} }

// Kind classifies a file by extension.
type Kind string

// File kinds.
const (
	KindVideo     Kind = "video" // .MP4
	KindPhoto     Kind = "photo" // .JPG
	KindRaw       Kind = "raw"   // .GPR
	KindLowRes    Kind = "lrv"   // .LRV proxy
	KindThumbnail Kind = "thm"   // .THM
	KindOther     Kind = "other"
)

// File is one entry on the SD card as reported by the media list.
type File struct {
	Directory string    `json:"directory" yaml:"directory"` // "100GOPRO"
	Name      string    `json:"name" yaml:"name"`           // "GH010397.MP4"
	Size      int64     `json:"size" yaml:"size"`
	Created   time.Time `json:"created" yaml:"created"`
	Modified  time.Time `json:"modified" yaml:"modified"`
	HasLRV    bool      `json:"has_lrv" yaml:"has_lrv"`
	LRVSize   int64     `json:"lrv_size,omitempty" yaml:"lrv_size,omitempty"`
	Raw       bool      `json:"raw" yaml:"raw"`
	Kind      Kind      `json:"kind" yaml:"kind"`
	// Group is the 4-digit capture number shared by chapters of one recording.
	Group string `json:"group" yaml:"group"`
	// Chapter is the 2-digit chapter index within the group (01 for the first).
	Chapter string `json:"chapter" yaml:"chapter"`
}

// Path returns "DIR/NAME", the form every media endpoint expects.
func (f File) Path() string { return f.Directory + "/" + f.Name }

// LRVName returns the low-resolution proxy name for a video ("GL010397.LRV").
func (f File) LRVName() string {
	if f.Kind != KindVideo || len(f.Name) < 8 {
		return ""
	}
	return "GL" + f.Name[2:8] + ".LRV"
}

// THMName returns the thumbnail name ("GH010397.THM") for GoPro-named media.
func (f File) THMName() string {
	if f.Group == "" {
		return ""
	}
	return strings.TrimSuffix(f.Name, path.Ext(f.Name)) + ".THM"
}

// List is the parsed media list.
type List struct {
	ID    string `json:"id" yaml:"id"`
	Files []File `json:"files" yaml:"files"`
}

// Filter narrows a List.
type Filter struct {
	Directory string
	Kinds     []Kind
	Since     time.Time
	Group     string
}

// Apply returns the files matching f, preserving order.
func (l List) Apply(f Filter) []File {
	var out []File
	for i := range l.Files {
		file := &l.Files[i]
		if f.Directory != "" && !strings.EqualFold(file.Directory, f.Directory) {
			continue
		}
		if len(f.Kinds) > 0 && !containsKind(f.Kinds, file.Kind) {
			continue
		}
		if !f.Since.IsZero() && file.Modified.Before(f.Since) {
			continue
		}
		if f.Group != "" && file.Group != f.Group {
			continue
		}
		out = append(out, *file)
	}
	return out
}

// Find returns the file at "DIR/NAME".
func (l List) Find(p string) (File, bool) {
	dir, name, ok := strings.Cut(p, "/")
	if !ok {
		return File{}, false
	}
	for i := range l.Files {
		if strings.EqualFold(l.Files[i].Directory, dir) && strings.EqualFold(l.Files[i].Name, name) {
			return l.Files[i], true
		}
	}
	return File{}, false
}

// Groups returns files grouped by capture number, each group ordered by chapter.
func (l List) Groups() [][]File {
	byGroup := map[string][]File{}
	var order []string
	for i := range l.Files {
		f := &l.Files[i]
		key := f.Directory + "/" + f.Group
		if _, ok := byGroup[key]; !ok {
			order = append(order, key)
		}
		byGroup[key] = append(byGroup[key], *f)
	}
	out := make([][]File, 0, len(order))
	for _, k := range order {
		g := byGroup[k]
		sort.Slice(g, func(i, j int) bool { return g[i].Chapter < g[j].Chapter })
		out = append(out, g)
	}
	return out
}

func containsKind(ks []Kind, k Kind) bool {
	for _, x := range ks {
		if x == k {
			return true
		}
	}
	return false
}

// wire shapes -------------------------------------------------------------

type wireList struct {
	ID    string    `json:"id"`
	Media []wireDir `json:"media"`
}

type wireDir struct {
	D  string     `json:"d"`
	Fs []wireFile `json:"fs"`
}

type wireFile struct {
	N    string `json:"n"`
	Cre  string `json:"cre"`
	Mod  string `json:"mod"`
	S    string `json:"s"`
	GLRV string `json:"glrv"`
	LS   string `json:"ls"`
	Raw  string `json:"raw"`
}

// fileNameRe captures prefix, chapter and group: GH 01 0397 .MP4
var fileNameRe = regexp.MustCompile(`^(?i)([A-Z]{2})(\d{2})(\d{4})\.([A-Z0-9]+)$`)

// ParseList decodes the camera's media list JSON.
func ParseList(data []byte) (List, error) {
	var w wireList
	if err := json.Unmarshal(data, &w); err != nil {
		return List{}, fmt.Errorf("media: decode list: %w", err)
	}
	out := List{ID: w.ID}
	for _, d := range w.Media {
		for _, f := range d.Fs {
			out.Files = append(out.Files, toFile(d.D, f))
		}
	}
	return out, nil
}

func toFile(dir string, w wireFile) File {
	f := File{Directory: dir, Name: w.N, HasLRV: w.GLRV != "", Raw: w.Raw == "1"}
	f.Size, _ = strconv.ParseInt(w.S, 10, 64)
	f.LRVSize, _ = strconv.ParseInt(w.GLRV, 10, 64)
	if cre, err := strconv.ParseInt(w.Cre, 10, 64); err == nil && cre > 0 {
		f.Created = time.Unix(cre, 0).UTC()
	}
	if mod, err := strconv.ParseInt(w.Mod, 10, 64); err == nil && mod > 0 {
		f.Modified = time.Unix(mod, 0).UTC()
	}
	f.Kind = kindOf(w.N)
	if m := fileNameRe.FindStringSubmatch(w.N); m != nil {
		f.Chapter, f.Group = m[2], m[3]
	}
	return f
}

func kindOf(name string) Kind {
	switch strings.ToUpper(path.Ext(name)) {
	case ".MP4":
		return KindVideo
	case ".JPG", ".JPEG":
		return KindPhoto
	case ".GPR":
		return KindRaw
	case ".LRV":
		return KindLowRes
	case ".THM":
		return KindThumbnail
	default:
		return KindOther
	}
}

// List fetches and parses the media list.
func (c *Client) List(ctx context.Context) (List, error) {
	resp, err := c.t.Get(ctx, transport.Request{Path: "/gopro/media/list", Idempotent: true})
	if err != nil {
		return List{}, err
	}
	return ParseList(resp.Body)
}

// Thumbnail fetches the JPEG thumbnail for a media path.
func (c *Client) Thumbnail(ctx context.Context, p string) ([]byte, error) {
	return c.blob(ctx, "/gopro/media/thumbnail", p)
}

// Screennail fetches the larger JPEG preview for a media path.
func (c *Client) Screennail(ctx context.Context, p string) ([]byte, error) {
	return c.blob(ctx, "/gopro/media/screennail", p)
}

// GPMF fetches the GPMF metadata header of a video.
func (c *Client) GPMF(ctx context.Context, p string) ([]byte, error) {
	return c.blob(ctx, "/gopro/media/gpmf", p)
}

// Telemetry fetches the full GPMF telemetry track of a video.
func (c *Client) Telemetry(ctx context.Context, p string) ([]byte, error) {
	return c.blob(ctx, "/gopro/media/telemetry", p)
}

func (c *Client) blob(ctx context.Context, endpoint, p string) ([]byte, error) {
	if err := validatePath(p); err != nil {
		return nil, err
	}
	resp, err := c.t.Get(ctx, transport.Request{Path: endpoint, Params: transport.Params{transport.P("path", p)}, Idempotent: true})
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// DeleteFile removes one file ("DIR/NAME").
func (c *Client) DeleteFile(ctx context.Context, p string) error {
	if err := validatePath(p); err != nil {
		return err
	}
	_, err := c.t.Get(ctx, transport.Request{Path: "/gopro/media/delete/file", Params: transport.Params{transport.P("path", p)}})
	return err
}

// DeleteGroup removes every chapter of the recording that contains p (legacy endpoint).
func (c *Client) DeleteGroup(ctx context.Context, p string) error {
	if err := validatePath(p); err != nil {
		return err
	}
	_, err := c.t.Get(ctx, transport.Request{Path: "/gp/gpControl/command/storage/delete/group", Params: transport.Params{transport.P("p", "/"+p)}})
	return err
}

// DeleteAll erases every file on the SD card (legacy endpoint).
func (c *Client) DeleteAll(ctx context.Context) error {
	_, err := c.t.Get(ctx, transport.Request{Path: "/gp/gpControl/command/storage/delete/all"})
	return err
}

// ErrBadPath is returned for media paths that are not "DIR/NAME".
var ErrBadPath = errors.New(`media: path must look like "100GOPRO/GH010397.MP4"`)

func validatePath(p string) error {
	dir, name, ok := strings.Cut(p, "/")
	if !ok || dir == "" || name == "" || strings.ContainsAny(p, "\\?#") || strings.Contains(name, "/") {
		return fmt.Errorf("%w: %q", ErrBadPath, p)
	}
	return nil
}
