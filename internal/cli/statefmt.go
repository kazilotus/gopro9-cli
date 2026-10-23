package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/kazilotus/gopro9-cli/pkg/gopro/ogp"
	"github.com/kazilotus/gopro9-cli/pkg/gopro/settings"
	"github.com/kazilotus/gopro9-cli/pkg/gopro/transport"
)

func allSettingDescriptors() []settings.Descriptor { return settings.All() }

// statusRows flattens the typed status struct into "status.<key>" rows using
// its JSON encoding, so enum Stringers and bools render consistently.
func statusRows(st ogp.State) [][]string {
	b, err := json.Marshal(st.Status)
	if err != nil {
		return nil
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		return nil
	}
	rows := make([][]string, 0, len(m))
	for k, v := range m {
		if k == "unknown" {
			var unk map[string]json.RawMessage
			_ = json.Unmarshal(v, &unk)
			for uk, uv := range unk {
				rows = append(rows, []string{"status." + uk, strings.Trim(string(uv), `"`)})
			}
			continue
		}
		rows = append(rows, []string{"status." + k, strings.Trim(string(v), `"`)})
	}
	return rows
}

func sortRows(rows [][]string) {
	sort.Slice(rows, func(i, j int) bool { return rows[i][0] < rows[j][0] })
}

func rawStateRequest() transport.Request {
	return transport.Request{Path: "/gopro/camera/state", Idempotent: true}
}

var _ = fmt.Sprint
