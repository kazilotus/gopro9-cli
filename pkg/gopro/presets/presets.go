// Package presets models the HERO9's preset groups and presets.
package presets

import (
	"fmt"
	"strconv"
	"strings"
)

// Group is a preset group (the camera's top-level mode).
type Group string

// Preset groups on the HERO9 Black.
const (
	GroupVideo     Group = "PRESET_GROUP_ID_VIDEO"
	GroupPhoto     Group = "PRESET_GROUP_ID_PHOTO"
	GroupTimelapse Group = "PRESET_GROUP_ID_TIMELAPSE"
)

// groupIDs are the numeric identifiers used by presets/set_group?id=.
var groupIDs = map[Group]uint32{GroupVideo: 1000, GroupPhoto: 1001, GroupTimelapse: 1002}

var groupNames = map[Group]string{GroupVideo: "video", GroupPhoto: "photo", GroupTimelapse: "timelapse"}

// ID returns the wire identifier for set_group.
func (g Group) ID() (uint32, bool) {
	id, ok := groupIDs[g]
	return id, ok
}

// String returns the short CLI name ("video").
func (g Group) String() string {
	if n, ok := groupNames[g]; ok {
		return n
	}
	return string(g)
}

// MarshalText implements encoding.TextMarshaler.
func (g Group) MarshalText() ([]byte, error) { return []byte(g.String()), nil }

// UnmarshalText implements encoding.TextUnmarshaler.
func (g *Group) UnmarshalText(b []byte) error {
	v, err := ParseGroup(string(b))
	if err != nil {
		return err
	}
	*g = v
	return nil
}

// Set implements pflag.Value.
func (g *Group) Set(s string) error { return g.UnmarshalText([]byte(s)) }

// Type implements pflag.Value.
func (Group) Type() string { return "video|photo|timelapse" }

// ParseGroup accepts "video", "photo", "timelapse", the wire enum name, or the numeric id.
func ParseGroup(s string) (Group, error) {
	key := strings.ToLower(strings.TrimSpace(s))
	for g, n := range groupNames {
		if key == n || strings.EqualFold(s, string(g)) {
			return g, nil
		}
	}
	if n, err := strconv.ParseUint(key, 10, 32); err == nil {
		for g, id := range groupIDs {
			if uint64(id) == n {
				return g, nil
			}
		}
	}
	return "", fmt.Errorf("presets: unknown group %q (want video, photo or timelapse)", s)
}

// Groups lists all groups in display order.
func Groups() []Group { return []Group{GroupVideo, GroupPhoto, GroupTimelapse} }

// Status is the camera's full preset state (presets/get).
type Status struct {
	Groups []GroupStatus `json:"presetGroupArray" yaml:"groups"`
}

// GroupStatus is one preset group with its presets.
type GroupStatus struct {
	ID           Group    `json:"id" yaml:"id"`
	Presets      []Preset `json:"presetArray" yaml:"presets"`
	CanAddPreset bool     `json:"canAddPreset" yaml:"can_add_preset"`
}

// Preset is one camera preset.
type Preset struct {
	ID          uint32          `json:"id" yaml:"id"`
	Mode        string          `json:"mode" yaml:"mode"`
	TitleID     string          `json:"titleId" yaml:"title_id"`
	TitleNumber int             `json:"titleNumber,omitempty" yaml:"title_number,omitempty"`
	UserDefined bool            `json:"userDefined" yaml:"user_defined"`
	Icon        string          `json:"icon" yaml:"icon"`
	Settings    []PresetSetting `json:"settingArray" yaml:"settings"`
	IsModified  bool            `json:"isModified" yaml:"is_modified"`
}

// PresetSetting is a setting value captured in a preset.
type PresetSetting struct {
	ID        uint32 `json:"id" yaml:"id"`
	Value     uint32 `json:"value" yaml:"value"`
	IsCaption bool   `json:"isCaption" yaml:"is_caption"`
}

// Title returns a readable name derived from the title id ("PRESET_TITLE_MAX_VIDEO" → "Max Video").
func (p Preset) Title() string {
	t := strings.TrimPrefix(p.TitleID, "PRESET_TITLE_")
	words := strings.Split(strings.ToLower(t), "_")
	for i, w := range words {
		if w != "" {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

// Find locates a preset by numeric id, title id, or readable title (case-insensitive).
func (s Status) Find(key string) (Preset, Group, bool) {
	key = strings.TrimSpace(key)
	for _, g := range s.Groups {
		for _, p := range g.Presets {
			if strconv.FormatUint(uint64(p.ID), 10) == key ||
				strings.EqualFold(p.TitleID, key) ||
				strings.EqualFold(p.Title(), key) ||
				strings.EqualFold(strings.ReplaceAll(p.Title(), " ", "-"), key) {
				return p, g.ID, true
			}
		}
	}
	return Preset{}, "", false
}
