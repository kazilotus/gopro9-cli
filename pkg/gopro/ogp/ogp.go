// Package ogp implements the Open GoPro 2.0 HTTP operations the HERO9 Black
// answers over USB: control, query, presets, settings, hilights and zoom.
// Media, webcam and preview-stream operations live in their own packages.
package ogp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/kazilotus/gopro9-cli/pkg/gopro/presets"
	"github.com/kazilotus/gopro9-cli/pkg/gopro/settings"
	"github.com/kazilotus/gopro9-cli/pkg/gopro/status"
	"github.com/kazilotus/gopro9-cli/pkg/gopro/transport"
)

// Client issues Open GoPro operations.
type Client struct{ t *transport.Client }

// New wraps a transport client.
func New(t *transport.Client) *Client { return &Client{t: t} }

// Version is the Open GoPro API version string reported by the camera.
type Version struct {
	Version string `json:"version" yaml:"version"`
}

// Version queries /gopro/version.
func (c *Client) Version(ctx context.Context) (Version, error) {
	return transport.GetJSON[Version](ctx, c.t, transport.Request{Path: "/gopro/version", Idempotent: true})
}

// State is the full camera state: every status and every setting.
type State struct {
	Status   status.Values   `json:"status" yaml:"status"`
	Settings settings.Values `json:"settings" yaml:"settings"`
}

// State queries /gopro/camera/state.
func (c *Client) State(ctx context.Context) (State, error) {
	return transport.GetJSON[State](ctx, c.t, transport.Request{Path: "/gopro/camera/state", Idempotent: true})
}

// KeepAlive sends the heartbeat that stops the camera from sleeping.
func (c *Client) KeepAlive(ctx context.Context) error {
	_, err := c.t.Get(ctx, transport.Request{Path: "/gopro/camera/keep_alive", Idempotent: true})
	return err
}

// SetAnalyticsClientInfo registers the client with the camera's analytics (harmless; part of the connect handshake).
func (c *Client) SetAnalyticsClientInfo(ctx context.Context) error {
	_, err := c.t.Get(ctx, transport.Request{Path: "/gopro/camera/analytics/set_client_info", Idempotent: true})
	return err
}

// ShutterAction is start or stop.
type ShutterAction string

// Shutter actions.
const (
	ShutterStart ShutterAction = "start"
	ShutterStop  ShutterAction = "stop"
)

// Shutter starts or stops capture in the current mode.
func (c *Client) Shutter(ctx context.Context, a ShutterAction) error {
	if a != ShutterStart && a != ShutterStop {
		return fmt.Errorf("ogp: invalid shutter action %q", a)
	}
	_, err := c.t.Get(ctx, transport.Request{Path: "/gopro/camera/shutter/" + string(a)})
	return err
}

// SettingRejectedError is returned when the camera answers a setting change
// with 403. The camera lists the options it would accept right now; an empty
// list means the setting does not apply in the current mode or preset.
type SettingRejectedError struct {
	Setting          settings.SettingID
	Option           uint32
	AvailableOptions []uint32
	Err              error
}

func (e *SettingRejectedError) Error() string {
	if len(e.AvailableOptions) == 0 {
		return fmt.Sprintf("camera rejected %s=%d: setting not applicable in the current mode", e.Setting, e.Option)
	}
	return fmt.Sprintf("camera rejected %s=%d: valid now: %v", e.Setting, e.Option, e.AvailableOptions)
}

// Unwrap exposes the transport error (and thus transport.ErrForbidden).
func (e *SettingRejectedError) Unwrap() error { return e.Err }

// Applicable reports whether the camera would accept any option for the setting right now.
func (e *SettingRejectedError) Applicable() bool { return len(e.AvailableOptions) > 0 }

// SetSetting applies one typed setting option.
func (c *Client) SetSetting(ctx context.Context, opt settings.Option) error {
	// Order matters: the camera returns 400 when option precedes setting.
	params := transport.Params{
		transport.P("setting", strconv.FormatUint(uint64(opt.ID()), 10)),
		transport.P("option", strconv.FormatUint(uint64(opt.Value()), 10)),
	}
	_, err := c.t.Get(ctx, transport.Request{Path: "/gopro/camera/setting", Params: params})
	var te *transport.Error
	if errors.As(err, &te) && te.Status == 403 {
		rej := &SettingRejectedError{Setting: opt.ID(), Option: opt.Value(), Err: err}
		var body struct {
			Available []uint32 `json:"available_options"`
		}
		if json.Unmarshal(te.Body, &body) == nil {
			rej.AvailableOptions = body.Available
		}
		return rej
	}
	return err
}

// GetSetting reads one setting's current typed value via the state query.
func (c *Client) GetSetting(ctx context.Context, id settings.SettingID) (settings.Option, error) {
	st, err := c.State(ctx)
	if err != nil {
		return nil, err
	}
	if opt, ok := st.Settings.Get(id); ok {
		return opt, nil
	}
	if raw, ok := st.Settings.Unknown[id]; ok {
		return rawOption{id: id, value: raw}, nil
	}
	return nil, fmt.Errorf("ogp: camera did not report setting %s", id)
}

// rawOption is an undocumented setting value surfaced from the camera state.
type rawOption struct {
	id    settings.SettingID
	value int64
}

func (r rawOption) ID() settings.SettingID { return r.id }
func (r rawOption) Value() uint32          { return uint32(r.value) }
func (r rawOption) String() string         { return strconv.FormatInt(r.value, 10) }
func (rawOption) Valid() bool              { return false }
func (rawOption) SupportedOnHERO9() bool   { return false }

// Presets queries the preset status.
func (c *Client) Presets(ctx context.Context) (presets.Status, error) {
	return transport.GetJSON[presets.Status](ctx, c.t, transport.Request{Path: "/gopro/camera/presets/get", Idempotent: true})
}

// LoadPreset activates a preset by numeric id.
func (c *Client) LoadPreset(ctx context.Context, id uint32) error {
	_, err := c.t.Get(ctx, transport.Request{Path: "/gopro/camera/presets/load", Params: transport.Params{transport.P("id", strconv.FormatUint(uint64(id), 10))}})
	return err
}

// SetPresetGroup switches to a preset group (video, photo, timelapse).
func (c *Client) SetPresetGroup(ctx context.Context, g presets.Group) error {
	id, ok := g.ID()
	if !ok {
		return fmt.Errorf("ogp: unknown preset group %q", g)
	}
	_, err := c.t.Get(ctx, transport.Request{Path: "/gopro/camera/presets/set_group", Params: transport.Params{transport.P("id", strconv.FormatUint(uint64(id), 10))}})
	return err
}

// DigitalZoom sets zoom as a percentage 0–100.
func (c *Client) DigitalZoom(ctx context.Context, percent uint8) error {
	if percent > 100 {
		return fmt.Errorf("ogp: zoom %d%% out of range 0-100", percent)
	}
	_, err := c.t.Get(ctx, transport.Request{Path: "/gopro/camera/digital_zoom", Params: transport.Params{transport.P("percent", strconv.Itoa(int(percent)))}})
	return err
}

// TurboTransfer toggles turbo mode, which prioritises USB bandwidth for media transfer.
func (c *Client) TurboTransfer(ctx context.Context, on bool) error {
	_, err := c.t.Get(ctx, transport.Request{Path: "/gopro/media/turbo_transfer", Params: transport.Params{transport.P("p", boolParam(on))}})
	return err
}

// DateTime is the camera clock as reported by get_date_time.
type DateTime struct {
	Date string `json:"date" yaml:"date"` // YYYY_MM_DD
	Time string `json:"time" yaml:"time"` // HH_MM_SS
}

// Parse converts the wire format into a time.Time in the given location.
func (d DateTime) Parse(loc *time.Location) (time.Time, error) {
	return time.ParseInLocation("2006_01_02 15_04_05", d.Date+" "+d.Time, loc)
}

// GetDateTime reads the camera clock.
func (c *Client) GetDateTime(ctx context.Context) (DateTime, error) {
	return transport.GetJSON[DateTime](ctx, c.t, transport.Request{Path: "/gopro/camera/get_date_time", Idempotent: true})
}

// SetDateTime writes the camera clock (local wall time; the HERO9 has no timezone field).
func (c *Client) SetDateTime(ctx context.Context, t time.Time) error {
	params := transport.Params{transport.P("date", t.Format("2006_01_02")), transport.P("time", t.Format("15_04_05"))}
	_, err := c.t.Get(ctx, transport.Request{Path: "/gopro/camera/set_date_time", Params: params})
	return err
}

// HilightFile adds a HiLight tag to an existing media file at offset.
func (c *Client) HilightFile(ctx context.Context, path string, offset time.Duration) error {
	return c.hilight(ctx, "/gopro/media/hilight/file", path, offset)
}

// HilightRemove removes a HiLight tag from an existing media file at offset.
func (c *Client) HilightRemove(ctx context.Context, path string, offset time.Duration) error {
	return c.hilight(ctx, "/gopro/media/hilight/remove", path, offset)
}

func (c *Client) hilight(ctx context.Context, endpoint, path string, offset time.Duration) error {
	if path == "" {
		return errors.New("ogp: media path required")
	}
	params := transport.Params{transport.P("path", path)}
	if offset > 0 {
		params = append(params, transport.P("ms", strconv.FormatInt(offset.Milliseconds(), 10)))
	}
	_, err := c.t.Get(ctx, transport.Request{Path: endpoint, Params: params})
	return err
}

// HilightMoment tags the current moment while recording.
func (c *Client) HilightMoment(ctx context.Context) error {
	_, err := c.t.Get(ctx, transport.Request{Path: "/gopro/media/hilight/moment"})
	return err
}

func boolParam(b bool) string {
	if b {
		return "1"
	}
	return "0"
}
