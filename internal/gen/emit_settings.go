package main

import (
	"fmt"
	"strings"
)

func emitSettings(m *model) string {
	var b strings.Builder
	w := func(format string, a ...any) { fmt.Fprintf(&b, format, a...) }
	w("%s", header)
	w("package settings\n\n")
	w("import (\n\t\"fmt\"\n\t\"strconv\"\n\t\"strings\"\n)\n\n")

	// IDs
	w("// Setting IDs supported by the HERO9 Black.\nconst (\n")
	for _, s := range m.Settings {
		w("\t// Setting%s is setting %d (%s).\n\tSetting%s SettingID = %d\n", s.TypeName, s.ID, s.Title, s.TypeName, s.ID)
	}
	w(")\n\n")

	// per-setting types
	for _, s := range m.Settings {
		if s.IsEnum {
			emitEnumSetting(&b, s)
		} else {
			emitNumericSetting(&b, s)
		}
	}

	// registry
	w("var descriptors = []Descriptor{\n")
	for _, s := range m.Settings {
		w("\t{\n\t\tID: Setting%s, Name: %q, Title: %q, Numeric: %v,\n", s.TypeName, s.Name, s.Title, !s.IsEnum)
		if s.IsEnum {
			w("\t\tOptions: []OptionInfo{\n")
			for _, o := range s.Options {
				w("\t\t\t{Value: %d, Name: %q, Label: %q, HERO9: %v},\n", o.Value, o.Name, o.Label, o.HERO9)
			}
			w("\t\t},\n")
			w("\t\tparse: func(s string) (Option, error) { return Parse%s(s) },\n", s.TypeName)
			w("\t\tfromValue: func(v uint32) (Option, bool) { o := %s(v); return o, o.Valid() },\n", s.TypeName)
		} else {
			w("\t\tMin: %d, Max: %d,\n", s.Min, s.Max)
			w("\t\tparse: func(s string) (Option, error) { return Parse%s(s) },\n", s.TypeName)
			w("\t\tfromValue: func(v uint32) (Option, bool) { o := %s(v); return o, o.Valid() },\n", s.TypeName)
		}
		w("\t},\n")
	}
	w("}\n\n")

	// Values struct
	w("// Values holds every HERO9 setting decoded from the camera state.\n")
	w("// Settings the spec does not document for this model are kept in Unknown.\n")
	w("type Values struct {\n")
	for _, s := range m.Settings {
		w("\t%s %s `json:\"%s\" yaml:\"%s\"`\n", s.TypeName, s.TypeName, s.Name, s.Name)
	}
	w("\tUnknown map[SettingID]int64 `json:\"unknown,omitempty\" yaml:\"unknown,omitempty\"`\n}\n\n")

	w("// apply stores one raw setting value into the typed field, or Unknown.\n")
	w("func (v *Values) apply(id SettingID, raw int64) {\n\tswitch id {\n")
	for _, s := range m.Settings {
		w("\tcase Setting%s:\n\t\tv.%s = %s(raw)\n", s.TypeName, s.TypeName, s.TypeName)
	}
	w("\tdefault:\n\t\tif v.Unknown == nil {\n\t\t\tv.Unknown = map[SettingID]int64{}\n\t\t}\n\t\tv.Unknown[id] = raw\n\t}\n}\n\n")

	w("// Get returns the typed option currently held for id, if the setting is known.\n")
	w("func (v Values) Get(id SettingID) (Option, bool) {\n\tswitch id {\n")
	for _, s := range m.Settings {
		w("\tcase Setting%s:\n\t\treturn v.%s, true\n", s.TypeName, s.TypeName)
	}
	w("\tdefault:\n\t\treturn nil, false\n\t}\n}\n")
	return b.String()
}

func emitEnumSetting(b *strings.Builder, s settingModel) {
	w := func(format string, a ...any) { fmt.Fprintf(b, format, a...) }
	t := s.TypeName
	w("// %s is setting %d: %s.\n", t, s.ID, s.Title)
	if s.Desc != "" {
		w("//\n// %s\n", wrapComment(s.Desc))
	}
	w("type %s uint16\n\n", t)
	w("// %s options documented by the Open GoPro spec. Options the spec does not\n// list for the HERO9 Black are marked; the camera may still report or reject them.\nconst (\n", t)
	for _, o := range s.Options {
		note := ""
		if !o.HERO9 {
			note = " (not listed for HERO9)"
		}
		w("\t// %s is %q%s.\n\t%s %s = %d\n", o.Const, o.Label, note, o.Const, t, o.Value)
	}
	w(")\n\n")
	w("var %sLabels = map[%s]string{\n", lower(t), t)
	for _, o := range s.Options {
		w("\t%s: %q,\n", o.Const, o.Label)
	}
	w("}\n\n")
	w("var %sHERO9 = map[%s]bool{\n", lower(t), t)
	for _, o := range s.Options {
		if o.HERO9 {
			w("\t%s: true,\n", o.Const)
		}
	}
	w("}\n\n")
	w("// ID implements Option.\nfunc (%s) ID() SettingID { return Setting%s }\n\n", t, t)
	w("// Value implements Option.\nfunc (o %s) Value() uint32 { return uint32(o) }\n\n", t)
	w("// Valid reports whether o is an option documented by the Open GoPro spec.\nfunc (o %s) Valid() bool { _, ok := %sLabels[o]; return ok }\n\n", t, lower(t))
	w("// SupportedOnHERO9 reports whether the spec lists o for the HERO9 Black.\nfunc (o %s) SupportedOnHERO9() bool { return %sHERO9[o] }\n\n", t, lower(t))
	w("// String returns the human label, or the numeric value for unknown options.\n")
	w("func (o %s) String() string {\n\tif l, ok := %sLabels[o]; ok {\n\t\treturn l\n\t}\n\treturn strconv.FormatUint(uint64(o), 10)\n}\n\n", t, lower(t))
	w("// Parse%s accepts a label (case/punctuation-insensitive) first, then a numeric option value.\n", t)
	w("func Parse%s(s string) (%s, error) {\n", t, t)
	w("\tkey := normalize(s)\n\tfor o, l := range %sLabels {\n\t\tif normalize(l) == key {\n\t\t\treturn o, nil\n\t\t}\n\t}\n", lower(t))
	w("\tif n, err := strconv.ParseUint(strings.TrimSpace(s), 10, 32); err == nil {\n\t\to := %s(n)\n\t\tif o.Valid() {\n\t\t\treturn o, nil\n\t\t}\n\t\treturn 0, fmt.Errorf(\"settings: %%d is not a valid %s option\", n)\n\t}\n", t, s.Title)
	w("\treturn 0, fmt.Errorf(\"settings: unknown %s option %%q\", s)\n}\n\n", s.Title)
	emitFlagMethods(b, t, s.Title)
}

func emitNumericSetting(b *strings.Builder, s settingModel) {
	w := func(format string, a ...any) { fmt.Fprintf(b, format, a...) }
	t := s.TypeName
	w("// %s is setting %d: %s.\n", t, s.ID, s.Title)
	if s.Desc != "" {
		w("//\n// %s\n", wrapComment(s.Desc))
	}
	w("type %s uint32\n\n", t)
	if s.Max > 0 {
		w("// Bounds for %s.\nconst (\n\t%sMin %s = %d\n\t%sMax %s = %d\n)\n\n", t, t, t, s.Min, t, t, s.Max)
	}
	w("// ID implements Option.\nfunc (%s) ID() SettingID { return Setting%s }\n\n", t, t)
	w("// Value implements Option.\nfunc (o %s) Value() uint32 { return uint32(o) }\n\n", t)
	if s.Max > 0 {
		w("// Valid reports whether o is within the documented range.\nfunc (o %s) Valid() bool { return o >= %sMin && o <= %sMax }\n\n", t, t, t)
	} else {
		w("// Valid always reports true: the spec documents no bounds for this setting.\nfunc (%s) Valid() bool { return true }\n\n", t)
	}
	w("// SupportedOnHERO9 always reports true for numeric settings documented for the HERO9.\nfunc (%s) SupportedOnHERO9() bool { return true }\n\n", t)
	w("// String returns the decimal value.\nfunc (o %s) String() string { return strconv.FormatUint(uint64(o), 10) }\n\n", t)
	w("// Parse%s parses a decimal value and checks the documented range.\n", t)
	w("func Parse%s(s string) (%s, error) {\n\tn, err := strconv.ParseUint(s, 10, 32)\n\tif err != nil {\n\t\treturn 0, fmt.Errorf(\"settings: %s must be an integer: %%w\", err)\n\t}\n\to := %s(n)\n\tif !o.Valid() {\n\t\treturn 0, fmt.Errorf(\"settings: %s %%d out of range\", n)\n\t}\n\treturn o, nil\n}\n\n", t, t, s.Title, t, s.Title)
	emitFlagMethods(b, t, s.Title)
}

func emitFlagMethods(b *strings.Builder, t, title string) {
	w := func(format string, a ...any) { fmt.Fprintf(b, format, a...) }
	w("// Set implements pflag.Value.\nfunc (o *%s) Set(s string) error {\n\tv, err := Parse%s(s)\n\tif err != nil {\n\t\treturn err\n\t}\n\t*o = v\n\treturn nil\n}\n\n", t, t)
	w("// Type implements pflag.Value.\nfunc (%s) Type() string { return %q }\n\n", t, kebab(title))
	w("// MarshalText implements encoding.TextMarshaler.\nfunc (o %s) MarshalText() ([]byte, error) { return []byte(o.String()), nil }\n\n", t)
	w("// UnmarshalText implements encoding.TextUnmarshaler.\nfunc (o *%s) UnmarshalText(text []byte) error { return o.Set(string(text)) }\n\n", t)
}

func lower(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}

func wrapComment(s string) string {
	words := strings.Fields(s)
	var lines []string
	var cur strings.Builder
	for _, wd := range words {
		if cur.Len()+len(wd)+1 > 90 {
			lines = append(lines, cur.String())
			cur.Reset()
		}
		if cur.Len() > 0 {
			cur.WriteByte(' ')
		}
		cur.WriteString(wd)
	}
	if cur.Len() > 0 {
		lines = append(lines, cur.String())
	}
	return strings.Join(lines, "\n// ")
}

const header = "// Code generated by internal/gen from spec/opengopro/openapi.json; DO NOT EDIT.\n\n"
