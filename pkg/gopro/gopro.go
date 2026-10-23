// Package gopro is the importable SDK for controlling a GoPro HERO9 Black over
// its USB (or Wi-Fi) HTTP API. The Camera facade groups the per-area clients;
// each is usable on its own with a shared transport.
//
// Typed settings, statuses and the capability matrix are generated from the
// Open GoPro OpenAPI spec; see the settings, status and capability packages.
package gopro

import (
	"log/slog"
	"time"

	"github.com/kazilotus/gopro9-cli/pkg/gopro/gpcontrol"
	"github.com/kazilotus/gopro9-cli/pkg/gopro/media"
	"github.com/kazilotus/gopro9-cli/pkg/gopro/ogp"
	"github.com/kazilotus/gopro9-cli/pkg/gopro/transport"
)

// Camera is the facade over one connected HERO9.
type Camera struct {
	Endpoint  transport.Endpoint
	Transport *transport.Client
	OGP       *ogp.Client
	Legacy    *gpcontrol.Client
	Media     *media.Client
}

// Option configures New.
type Option func(*config)

type config struct {
	timeout time.Duration
	retries int
	logger  *slog.Logger
	ua      string
}

// WithTimeout sets the per-request timeout.
func WithTimeout(d time.Duration) Option { return func(c *config) { c.timeout = d } }

// WithRetries sets the retry count for idempotent requests.
func WithRetries(n int) Option { return func(c *config) { c.retries = n } }

// WithLogger enables request tracing.
func WithLogger(l *slog.Logger) Option { return func(c *config) { c.logger = l } }

// WithUserAgent sets the HTTP User-Agent.
func WithUserAgent(ua string) Option { return func(c *config) { c.ua = ua } }

// New builds a Camera for the endpoint. It performs no I/O.
func New(ep transport.Endpoint, opts ...Option) *Camera {
	cfg := config{timeout: 5 * time.Second, retries: 2, logger: slog.New(slog.DiscardHandler), ua: "gopro9-cli"}
	for _, o := range opts {
		o(&cfg)
	}
	t := transport.New(ep,
		transport.WithTimeout(cfg.timeout),
		transport.WithRetries(cfg.retries, 200*time.Millisecond),
		transport.WithLogger(cfg.logger),
		transport.WithUserAgent(cfg.ua),
	)
	return &Camera{Endpoint: ep, Transport: t, OGP: ogp.New(t), Legacy: gpcontrol.New(t), Media: media.New(t)}
}
