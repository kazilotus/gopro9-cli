package media

import (
	"context"
	"regexp"
	"sort"
	"strings"

	"github.com/kazilotus/gopro9-cli/pkg/gopro/transport"
)

// Entry is one row of the DCIM file server's HTML directory listing.
type Entry struct {
	Name  string `json:"name" yaml:"name"`
	IsDir bool   `json:"is_dir" yaml:"is_dir"`
	Path  string `json:"path" yaml:"path"` // relative to /videos/DCIM/
}

var hrefRe = regexp.MustCompile(`href="(/videos/DCIM/[^"?]*)"`)

// ParseDCIM extracts entries from the camera's HTML directory index.
// Sort links ("?nd", "?dd") and the parent link are ignored.
func ParseDCIM(html []byte, dir string) []Entry {
	prefix := "/videos/DCIM/"
	if dir != "" {
		prefix += strings.Trim(dir, "/") + "/"
	}
	seen := map[string]bool{}
	var out []Entry
	for _, m := range hrefRe.FindAllSubmatch(html, -1) {
		href := string(m[1])
		if !strings.HasPrefix(href, prefix) || href == prefix {
			continue
		}
		rel := strings.TrimPrefix(href, prefix)
		if rel == ".." || strings.HasPrefix(rel, "../") || seen[rel] {
			continue
		}
		seen[rel] = true
		isDir := strings.HasSuffix(rel, "/")
		out = append(out, Entry{Name: strings.TrimSuffix(rel, "/"), IsDir: isDir, Path: strings.TrimSuffix(strings.TrimPrefix(href, "/videos/DCIM/"), "/")})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].IsDir != out[j].IsDir {
			return out[i].IsDir
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// Tree lists a DCIM directory ("" for the root) through the file server.
// Unlike List, this shows every file including .THM, .LRV and camera databases.
func (c *Client) Tree(ctx context.Context, dir string) ([]Entry, error) {
	p := "/videos/DCIM/"
	if dir != "" {
		p += strings.Trim(dir, "/") + "/"
	}
	resp, err := c.t.Get(ctx, transport.Request{Path: p, Idempotent: true})
	if err != nil {
		return nil, err
	}
	return ParseDCIM(resp.Body, dir), nil
}
