// Package gpcontrol implements the legacy /gp/gpControl endpoints that the
// HERO9 still serves and that have no Open GoPro 2.0 equivalent.
package gpcontrol

import (
	"context"

	"github.com/kazilotus/gopro9-cli/pkg/gopro/ogp"
	"github.com/kazilotus/gopro9-cli/pkg/gopro/transport"
)

// Client issues legacy gpControl operations.
type Client struct{ t *transport.Client }

// New wraps a transport client.
func New(t *transport.Client) *Client { return &Client{t: t} }

// Info is the camera identity block from /gp/gpControl/info.
type Info struct {
	ModelNumber             int      `json:"model_number" yaml:"model_number"`
	ModelName               string   `json:"model_name" yaml:"model_name"`
	FirmwareVersion         string   `json:"firmware_version" yaml:"firmware_version"`
	SerialNumber            string   `json:"serial_number" yaml:"serial_number"`
	BoardType               string   `json:"board_type" yaml:"board_type"`
	APMAC                   string   `json:"ap_mac" yaml:"ap_mac"`
	APSSID                  string   `json:"ap_ssid" yaml:"ap_ssid"`
	APHasDefaultCredentials string   `json:"ap_has_default_credentials" yaml:"ap_has_default_credentials"`
	Capabilities            string   `json:"capabilities" yaml:"capabilities"`
	LensCount               string   `json:"lens_count" yaml:"lens_count"`
	UpdateRequired          string   `json:"update_required" yaml:"update_required"`
	RequiredNetworks        []string `json:"required_networks" yaml:"required_networks"`
	HTTPKeepalive           int      `json:"http_keepalive" yaml:"http_keepalive"`
}

// IsLabs reports whether the firmware is a GoPro Labs build (patch version .70+).
func (i Info) IsLabs() bool {
	// HD9.01.01.72.70 → last component >= 70
	v := i.FirmwareVersion
	if len(v) < 2 {
		return false
	}
	last := v[len(v)-2:]
	return last >= "70"
}

type infoEnvelope struct {
	Info Info `json:"info"`
}

// Info queries camera identity.
func (c *Client) Info(ctx context.Context) (Info, error) {
	env, err := transport.GetJSON[infoEnvelope](ctx, c.t, transport.Request{Path: "/gp/gpControl/info", Idempotent: true})
	return env.Info, err
}

// Status queries the legacy status block, which has the same shape as the Open GoPro state.
func (c *Client) Status(ctx context.Context) (ogp.State, error) {
	return transport.GetJSON[ogp.State](ctx, c.t, transport.Request{Path: "/gp/gpControl/status", Idempotent: true})
}

// Sleep puts the camera to sleep.
func (c *Client) Sleep(ctx context.Context) error {
	_, err := c.t.Get(ctx, transport.Request{Path: "/gp/gpControl/command/system/sleep"})
	return err
}

// Locate turns the locate beep on or off.
func (c *Client) Locate(ctx context.Context, on bool) error {
	p := "0"
	if on {
		p = "1"
	}
	_, err := c.t.Get(ctx, transport.Request{Path: "/gp/gpControl/command/system/locate", Params: transport.Params{transport.P("p", p)}})
	return err
}

// AnalyticsBlob downloads the camera's binary analytics log.
func (c *Client) AnalyticsBlob(ctx context.Context) ([]byte, error) {
	resp, err := c.t.Get(ctx, transport.Request{Path: "/gp/gpControl/analytics/get", Idempotent: true})
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}
