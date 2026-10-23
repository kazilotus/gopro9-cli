package status

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValuesFromFixture(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "fixtures", "hero9_HD9.01.01.72.70", "gopro_camera_state.resp"))
	require.NoError(t, err)
	var state struct {
		Status Values `json:"status"`
	}
	require.NoError(t, json.Unmarshal(raw, &state))
	s := state.Status
	assert.True(t, s.BatteryPresent)
	assert.Equal(t, InternalBatteryBarsCharging, s.InternalBatteryBars)
	assert.Equal(t, "Charging", s.InternalBatteryBars.String())
	assert.False(t, s.Encoding)
	assert.False(t, s.Busy)
	assert.Equal(t, int64(98), s.InternalBatteryPercentage)
	assert.Equal(t, "HERO9 Black", s.AccessPointSSID)
	assert.Positive(t, s.SDCardRemaining)
	assert.NotEmpty(t, s.Unknown, "firmware reports statuses the spec does not document")

	out, err := json.Marshal(s)
	require.NoError(t, err)
	assert.Contains(t, string(out), `"internal-battery-bars":"Charging"`)
}

func TestStatusID(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "Battery Present", StatusBatteryPresent.String())
	assert.Equal(t, "status-9999", StatusID(9999).String())
	assert.True(t, StatusBatteryPresent.SupportedOnHERO9())
	assert.False(t, StatusID(9999).SupportedOnHERO9())
	b, err := StatusInternalBatteryBars.MarshalText()
	require.NoError(t, err)
	assert.Equal(t, "Internal Battery Bars", string(b))
}

func TestEnumString(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "Zero", InternalBatteryBarsZero.String())
	assert.Equal(t, "77", InternalBatteryBars(77).String())
	b, err := InternalBatteryBarsOne.MarshalText()
	require.NoError(t, err)
	assert.Equal(t, "One", string(b))
}

func TestRawValue(t *testing.T) {
	t.Parallel()
	assert.Equal(t, int64(5), rawValue{raw: []byte(`5`)}.Int())
	assert.Equal(t, int64(5), rawValue{raw: []byte(`5.9`)}.Int())
	assert.Equal(t, int64(0), rawValue{raw: []byte(`"x"`)}.Int())
	assert.Equal(t, "x", rawValue{raw: []byte(`"x"`)}.Str())
	assert.Equal(t, "12", rawValue{raw: []byte(`12`)}.Str())
}

func TestUnmarshalErrors(t *testing.T) {
	t.Parallel()
	var v Values
	require.Error(t, v.UnmarshalJSON([]byte(`[]`)))
	require.Error(t, v.UnmarshalJSON([]byte(`{"abc":1}`)))
	require.NoError(t, v.UnmarshalJSON([]byte(`{"1":1,"70":42,"30":"cam","9999":"zzz"}`)))
	assert.True(t, v.BatteryPresent)
	assert.Equal(t, int64(42), v.InternalBatteryPercentage)
	assert.Equal(t, "cam", v.AccessPointSSID)
	assert.Equal(t, "zzz", v.Unknown[StatusID(9999)])
}
