// Package webcam drives the HERO9's USB webcam mode. The camera streams
// MPEG-TS over UDP to the host (port 8554 by default); use package stream
// to receive it.
package webcam

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/kazilotus/gopro9-cli/pkg/gopro/transport"
)

// Client issues webcam operations.
type Client struct{ t *transport.Client }

// New wraps a transport client.
func New(t *transport.Client) *Client { return &Client{t: t} }

// DefaultPort is where the camera sends the stream unless told otherwise.
const DefaultPort = 8554

// ModeSwitchTimeout bounds webcam start/stop/exit, which reconfigure the
// camera's USB pipeline and take far longer than ordinary commands. Aborting
// the request early (as a 5 s timeout does) can leave the camera's status
// subsystem wedged until a power cycle.
const ModeSwitchTimeout = 30 * time.Second

// Status is the webcam state machine position.
type Status int

// Webcam statuses.
const (
	StatusOff              Status = 0
	StatusIdle             Status = 1
	StatusHighPowerPreview Status = 2
	StatusLowPowerPreview  Status = 3
)

func (s Status) String() string {
	switch s {
	case StatusOff:
		return "off"
	case StatusIdle:
		return "idle"
	case StatusHighPowerPreview:
		return "streaming"
	case StatusLowPowerPreview:
		return "preview"
	default:
		return strconv.Itoa(int(s))
	}
}

// MarshalText implements encoding.TextMarshaler.
func (s Status) MarshalText() ([]byte, error) { return []byte(s.String()), nil }

// Error is the camera-side webcam error code.
type Error int

// Webcam error codes.
const (
	ErrNone          Error = 0
	ErrSetPreset     Error = 1
	ErrSetWindowSize Error = 2
	ErrExecStream    Error = 3
	ErrShutter       Error = 4
	ErrComTimeout    Error = 5
	ErrInvalidParam  Error = 6
	ErrUnavailable   Error = 7
	ErrExit          Error = 8
)

func (e Error) String() string {
	switch e {
	case ErrNone:
		return "none"
	case ErrSetPreset:
		return "set preset"
	case ErrSetWindowSize:
		return "set window size"
	case ErrExecStream:
		return "exec stream"
	case ErrShutter:
		return "shutter"
	case ErrComTimeout:
		return "com timeout"
	case ErrInvalidParam:
		return "invalid param"
	case ErrUnavailable:
		return "unavailable"
	case ErrExit:
		return "exit"
	default:
		return strconv.Itoa(int(e))
	}
}

// MarshalText implements encoding.TextMarshaler.
func (e Error) MarshalText() ([]byte, error) { return []byte(e.String()), nil }

// State is the webcam status response.
type State struct {
	Status Status `json:"status" yaml:"status"`
	Error  Error  `json:"error" yaml:"error"`
}

// Version is the webcam feature version response.
type Version struct {
	Version         int  `json:"version" yaml:"version"`
	USB31Compatible bool `json:"usb_3_1_compatible" yaml:"usb_3_1_compatible"`
	MaxLensSupport  bool `json:"max_lens_support" yaml:"max_lens_support"`
}

// Resolution is a webcam output size.
type Resolution int

// Supported resolutions (wire values from the Open GoPro spec).
const (
	Res480  Resolution = 4
	Res720  Resolution = 7
	Res1080 Resolution = 12
)

func (r Resolution) String() string {
	switch r {
	case Res480:
		return "480"
	case Res720:
		return "720"
	case Res1080:
		return "1080"
	default:
		return strconv.Itoa(int(r))
	}
}

// Set implements pflag.Value.
func (r *Resolution) Set(s string) error {
	v, err := ParseResolution(s)
	if err != nil {
		return err
	}
	*r = v
	return nil
}

// Type implements pflag.Value.
func (Resolution) Type() string { return "480|720|1080" }

// ParseResolution accepts "480", "720", "1080" (with or without a trailing "p").
func ParseResolution(s string) (Resolution, error) {
	switch strings.TrimSuffix(strings.ToLower(strings.TrimSpace(s)), "p") {
	case "480":
		return Res480, nil
	case "720":
		return Res720, nil
	case "1080":
		return Res1080, nil
	}
	return 0, fmt.Errorf("webcam: unknown resolution %q (want 480, 720 or 1080)", s)
}

// FOV is the webcam digital lens.
type FOV int

// Supported fields of view (wire values match the video lens setting).
const (
	FOVWide      FOV = 0
	FOVNarrow    FOV = 2
	FOVSuperview FOV = 3
	FOVLinear    FOV = 4
)

func (f FOV) String() string {
	switch f {
	case FOVWide:
		return "wide"
	case FOVNarrow:
		return "narrow"
	case FOVSuperview:
		return "superview"
	case FOVLinear:
		return "linear"
	default:
		return strconv.Itoa(int(f))
	}
}

// Set implements pflag.Value.
func (f *FOV) Set(s string) error {
	v, err := ParseFOV(s)
	if err != nil {
		return err
	}
	*f = v
	return nil
}

// Type implements pflag.Value.
func (FOV) Type() string { return "wide|narrow|superview|linear" }

// ParseFOV accepts the lens names.
func ParseFOV(s string) (FOV, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "wide", "w":
		return FOVWide, nil
	case "narrow", "n":
		return FOVNarrow, nil
	case "superview", "s":
		return FOVSuperview, nil
	case "linear", "l":
		return FOVLinear, nil
	}
	return 0, fmt.Errorf("webcam: unknown field of view %q (want wide, narrow, superview or linear)", s)
}

// StartOptions configure Start. Zero values mean "camera default".
type StartOptions struct {
	Resolution Resolution
	FOV        FOV
	HasFOV     bool // FOVWide is 0, so the caller must say whether FOV was set
	Port       int  // UDP port on the host; 0 keeps the camera default (8554)
}

// Start begins streaming at full power.
func (c *Client) Start(ctx context.Context, o StartOptions) error {
	var params transport.Params
	if o.Resolution != 0 {
		params = append(params, transport.P("res", strconv.Itoa(int(o.Resolution))))
	}
	if o.HasFOV {
		params = append(params, transport.P("fov", strconv.Itoa(int(o.FOV))))
	}
	if o.Port != 0 {
		params = append(params, transport.P("port", strconv.Itoa(o.Port)))
	}
	_, err := c.t.Get(ctx, transport.Request{Path: "/gopro/webcam/start", Params: params, Timeout: ModeSwitchTimeout})
	return err
}

// Preview begins the low-power preview stream.
func (c *Client) Preview(ctx context.Context) error {
	_, err := c.t.Get(ctx, transport.Request{Path: "/gopro/webcam/preview", Timeout: ModeSwitchTimeout})
	return err
}

// Stop ends streaming but stays in webcam mode (idle).
func (c *Client) Stop(ctx context.Context) error {
	_, err := c.t.Get(ctx, transport.Request{Path: "/gopro/webcam/stop", Timeout: ModeSwitchTimeout})
	return err
}

// Exit leaves webcam mode entirely.
func (c *Client) Exit(ctx context.Context) error {
	_, err := c.t.Get(ctx, transport.Request{Path: "/gopro/webcam/exit", Timeout: ModeSwitchTimeout})
	return err
}

// Status reads the webcam state.
func (c *Client) Status(ctx context.Context) (State, error) {
	return transport.GetJSON[State](ctx, c.t, transport.Request{Path: "/gopro/webcam/status", Idempotent: true})
}

// Version reads the webcam feature version.
func (c *Client) Version(ctx context.Context) (Version, error) {
	return transport.GetJSON[Version](ctx, c.t, transport.Request{Path: "/gopro/webcam/version", Idempotent: true})
}
