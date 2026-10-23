package main

import (
	"fmt"
	"strings"
)

func emitStatus(m *model) string {
	var b strings.Builder
	w := func(format string, a ...any) { fmt.Fprintf(&b, format, a...) }
	w("%s", header)
	w("package status\n\n")
	w("import \"strconv\"\n\n")

	w("// Status IDs documented by the Open GoPro spec.\nconst (\n")
	for _, s := range m.Statuses {
		w("\t// Status%s is status %d (%s).\n\tStatus%s StatusID = %d\n", s.FieldName, s.ID, s.Name, s.FieldName, s.ID)
	}
	w(")\n\n")

	w("var statusNames = map[StatusID]string{\n")
	for _, s := range m.Statuses {
		w("\tStatus%s: %q,\n", s.FieldName, s.Name)
	}
	w("}\n\n")

	w("var hero9Statuses = map[StatusID]bool{\n")
	for _, s := range m.Statuses {
		if s.HERO9 {
			w("\tStatus%s: true,\n", s.FieldName)
		}
	}
	w("}\n\n")

	// enum types
	for _, s := range m.Statuses {
		if s.Kind != "enum" {
			continue
		}
		t := s.FieldName
		w("// %s is the value domain of status %d (%s).\n", t, s.ID, s.Name)
		if s.Desc != "" {
			w("//\n// %s\n", wrapComment(s.Desc))
		}
		w("type %s int64\n\n", t)
		w("// %s values.\nconst (\n", t)
		for _, e := range s.Enum {
			w("\t// %s is %q.\n\t%s %s = %d\n", e.Const, e.Label, e.Const, t, e.Value)
		}
		w(")\n\n")
		w("var %sLabels = map[%s]string{\n", lower(t), t)
		for _, e := range s.Enum {
			w("\t%s: %q,\n", e.Const, e.Label)
		}
		w("}\n\n")
		w("// String returns the documented meaning, or the raw number when undocumented.\n")
		w("func (v %s) String() string {\n\tif l, ok := %sLabels[v]; ok {\n\t\treturn l\n\t}\n\treturn strconv.FormatInt(int64(v), 10)\n}\n\n", t, lower(t))
		w("// MarshalText implements encoding.TextMarshaler.\nfunc (v %s) MarshalText() ([]byte, error) { return []byte(v.String()), nil }\n\n", t)
	}

	// Values struct
	w("// Values is the typed decoding of the camera's status block.\n")
	w("// Fields are named after the spec's status titles. IDs the spec does not\n")
	w("// document are preserved in Unknown.\n")
	w("type Values struct {\n")
	for _, s := range m.Statuses {
		goType := map[string]string{"bool": "bool", "int": "int64", "string": "string", "enum": s.FieldName}[s.Kind]
		w("\t// %s: %s\n", s.FieldName, strings.TrimSpace(firstSentence(s.Desc, s.Name)))
		w("\t%s %s `json:\"%s\" yaml:\"%s\"`\n", s.FieldName, goType, kebab(s.Name), kebab(s.Name))
	}
	w("\tUnknown map[StatusID]string `json:\"unknown,omitempty\" yaml:\"unknown,omitempty\"`\n}\n\n")

	w("// apply stores one raw status value into the typed field, or Unknown.\n")
	w("func (v *Values) apply(id StatusID, raw rawValue) {\n\tswitch id {\n")
	for _, s := range m.Statuses {
		switch s.Kind {
		case "bool":
			w("\tcase Status%s:\n\t\tv.%s = raw.Int() != 0\n", s.FieldName, s.FieldName)
		case "int":
			w("\tcase Status%s:\n\t\tv.%s = raw.Int()\n", s.FieldName, s.FieldName)
		case "string":
			w("\tcase Status%s:\n\t\tv.%s = raw.Str()\n", s.FieldName, s.FieldName)
		case "enum":
			w("\tcase Status%s:\n\t\tv.%s = %s(raw.Int())\n", s.FieldName, s.FieldName, s.FieldName)
		}
	}
	w("\tdefault:\n\t\tif v.Unknown == nil {\n\t\t\tv.Unknown = map[StatusID]string{}\n\t\t}\n\t\tv.Unknown[id] = raw.Str()\n\t}\n}\n")
	return b.String()
}

func firstSentence(desc, fallback string) string {
	if desc == "" {
		return fallback
	}
	if i := strings.Index(desc, ". "); i > 0 {
		return desc[:i+1]
	}
	return desc
}
