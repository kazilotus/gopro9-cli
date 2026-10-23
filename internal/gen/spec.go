package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const hero9 = "HERO9 Black"

// --- OpenAPI subset -------------------------------------------------------

type openAPI struct {
	Info struct {
		Version string `json:"version"`
	} `json:"info"`
	Paths      map[string]map[string]operation `json:"paths"`
	Components struct {
		Schemas map[string]schema `json:"schemas"`
	} `json:"components"`
}

type operation struct {
	OperationID      string   `json:"operationId"`
	Summary          string   `json:"summary"`
	Description      string   `json:"description"`
	Tags             []string `json:"tags"`
	SupportedCameras []string `json:"x-supported-cameras"`
}

type schema struct {
	Title            string              `json:"title"`
	Description      string              `json:"description"`
	Type             string              `json:"type"`
	Enum             []json.RawMessage   `json:"enum"`
	EnumDescriptions map[string]string   `json:"x-enumDescriptions"`
	OptionMetadata   map[string][]string `json:"x-option-metadata"`
	SupportedCameras []string            `json:"x-supported-cameras"`
	Properties       map[string]schema   `json:"properties"`
}

// --- overrides ------------------------------------------------------------

type overrides struct {
	SpecVersion    string                   `yaml:"spec_version"`
	Camera         string                   `yaml:"camera"`
	FirmwareTested []string                 `yaml:"firmware_tested"`
	Overrides      map[string]overrideEntry `yaml:"overrides"`
	Legacy         map[string]legacyEntry   `yaml:"legacy"`
	Transport      map[string]yaml.Node     `yaml:"transport_facts"`
}

type overrideEntry struct {
	Support string `yaml:"support"`
	Note    string `yaml:"note"`
}

type legacyEntry struct {
	Path    string `yaml:"path"`
	Support string `yaml:"support"`
	Note    string `yaml:"note"`
}

// --- model ------------------------------------------------------------------

type optionModel struct {
	Value uint32
	Label string // "2.7K 4:3"
	Const string // "VideoResolution2_7K4x3"
	Name  string // "2.7k-4-3"
}

type settingModel struct {
	ID       uint16
	Title    string
	TypeName string
	Name     string // kebab
	Desc     string
	IsEnum   bool
	Options  []optionModel
	Min, Max uint32 // numeric settings only (0,0 = unbounded)
}

type enumValue struct {
	Value int64
	Label string
	Const string
}

type statusModel struct {
	ID        uint16
	Name      string // "Internal Battery Bars"
	FieldName string // "InternalBatteryBars"
	Kind      string // bool | int | string | enum
	Desc      string
	Enum      []enumValue
	HERO9     bool
}

type opModel struct {
	ID      string // operationId
	Const   string
	Method  string
	Path    string
	Summary string
	Tags    []string
	InSpec  bool // HERO9 listed in x-supported-cameras
	Level   string
	Note    string
	Legacy  bool
}

type model struct {
	SpecVersion string
	Firmware    []string
	Settings    []settingModel
	Statuses    []statusModel
	Ops         []opModel
}

func loadSpec(path string) (*openAPI, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s openAPI
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &s, nil
}

func loadOverrides(path string) (*overrides, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var o overrides
	if err := yaml.Unmarshal(b, &o); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &o, nil
}

func buildModel(spec *openAPI, ov *overrides) (*model, error) {
	m := &model{SpecVersion: spec.Info.Version, Firmware: ov.FirmwareTested}
	if err := buildSettings(spec, m); err != nil {
		return nil, err
	}
	if err := buildStatuses(spec, m); err != nil {
		return nil, err
	}
	buildOps(spec, ov, m)
	return m, nil
}

func buildSettings(spec *openAPI, m *model) error {
	for name, sch := range spec.Components.Schemas {
		if !strings.HasPrefix(name, "Setting_") || !supportsHERO9(sch.SupportedCameras) {
			continue
		}
		id, err := strconv.ParseUint(strings.TrimPrefix(name, "Setting_"), 10, 16)
		if err != nil {
			return fmt.Errorf("setting schema %s: %w", name, err)
		}
		sm := settingModel{
			ID:       uint16(id),
			Title:    sch.Title,
			TypeName: pascal(sch.Title),
			Name:     kebab(sch.Title),
			Desc:     firstParagraph(sch.Description),
			IsEnum:   len(sch.Enum) > 0,
		}
		if sm.IsEnum {
			seen := map[string]bool{}
			for _, e := range sch.Enum {
				key := string(e)
				v, err := strconv.ParseUint(key, 10, 32)
				if err != nil {
					continue // non-numeric enum members are not setting options
				}
				if !supportsHERO9(sch.OptionMetadata[key]) {
					continue
				}
				label := sch.EnumDescriptions[key]
				if label == "" {
					label = "Option " + key
				}
				c := sm.TypeName + optionIdent(label)
				if seen[c] {
					c += "V" + key
				}
				seen[c] = true
				sm.Options = append(sm.Options, optionModel{Value: uint32(v), Label: label, Const: c, Name: kebab(label)})
			}
			sort.Slice(sm.Options, func(i, j int) bool { return sm.Options[i].Value < sm.Options[j].Value })
		} else {
			sm.Min, sm.Max = parseRange(sch.Description)
		}
		m.Settings = append(m.Settings, sm)
	}
	sort.Slice(m.Settings, func(i, j int) bool { return m.Settings[i].ID < m.Settings[j].ID })
	return nil
}

func buildStatuses(spec *openAPI, m *model) error {
	state, ok := spec.Components.Schemas["State"]
	if !ok {
		return errors.New("spec has no State schema")
	}
	for idStr, sch := range state.Properties["status"].Properties {
		id, err := strconv.ParseUint(idStr, 10, 16)
		if err != nil {
			return fmt.Errorf("status id %q: %w", idStr, err)
		}
		name := boldTitle(sch.Description)
		if name == "" {
			name = "Status " + idStr
		}
		sm := statusModel{
			ID: uint16(id), Name: name, FieldName: pascal(name),
			Desc: firstParagraph(stripBold(sch.Description)), HERO9: supportsHERO9(sch.SupportedCameras),
		}
		table := parseValueTable(sch.Description)
		switch {
		case sch.Type == "string":
			sm.Kind = "string"
		case len(table) > 0:
			sm.Kind = "enum"
			seen := map[string]bool{}
			for _, row := range table {
				c := sm.FieldName + optionIdent(row.Label)
				if seen[c] {
					c += "V" + strconv.FormatInt(row.Value, 10)
				}
				seen[c] = true
				sm.Enum = append(sm.Enum, enumValue{Value: row.Value, Label: row.Label, Const: c})
			}
		case len(sch.Enum) == 2 && string(sch.Enum[0]) == "0" && string(sch.Enum[1]) == "1":
			sm.Kind = "bool"
		default:
			sm.Kind = "int"
		}
		m.Statuses = append(m.Statuses, sm)
	}
	sort.Slice(m.Statuses, func(i, j int) bool { return m.Statuses[i].ID < m.Statuses[j].ID })
	return nil
}

func buildOps(spec *openAPI, ov *overrides, m *model) {
	for path, methods := range spec.Paths {
		for method, op := range methods {
			if op.OperationID == "" || strings.Contains(op.OperationID, "::") {
				continue // per-setting pseudo-operations are covered by the settings registry
			}
			om := opModel{
				ID: op.OperationID, Const: opConst(op.OperationID), Method: strings.ToUpper(method),
				Path: path, Summary: strings.TrimSpace(op.Summary), Tags: op.Tags,
				InSpec: supportsHERO9(op.SupportedCameras),
			}
			om.Level = "unsupported"
			if om.InSpec {
				om.Level = "spec"
			}
			if o, ok := ov.Overrides[op.OperationID]; ok {
				om.Level, om.Note = o.Support, o.Note
			}
			m.Ops = append(m.Ops, om)
		}
	}
	for id, l := range ov.Legacy {
		p, _, _ := strings.Cut(l.Path, "?")
		m.Ops = append(m.Ops, opModel{
			ID: id, Const: opConst(id), Method: "GET", Path: p, Summary: "Legacy gpControl endpoint",
			Tags: []string{"Legacy"}, Level: l.Support, Note: l.Note, Legacy: true,
		})
	}
	sort.Slice(m.Ops, func(i, j int) bool { return m.Ops[i].ID < m.Ops[j].ID })
}

func opConst(id string) string {
	// OGP_GET_STATE -> OpOGPGetState ; GP_INFO -> OpGPInfo
	parts := strings.Split(id, "_")
	var b strings.Builder
	b.WriteString("Op")
	for i, p := range parts {
		if p == "" {
			continue
		}
		if i == 0 && (p == "OGP" || p == "GPCAMERA" || p == "GP") {
			b.WriteString(p)
			continue
		}
		b.WriteString(strings.ToUpper(p[:1]) + strings.ToLower(p[1:]))
	}
	return b.String()
}
