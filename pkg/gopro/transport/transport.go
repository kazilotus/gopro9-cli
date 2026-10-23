// Package transport is the only package that speaks HTTP to the camera.
//
// It encodes the HERO9's server quirks so higher layers never see them:
// the mandatory Host header (DNS-rebinding guard), method-agnostic routing,
// `Connection: close` on every response, HEAD being forbidden on media files,
// and the 404/{} shape of unsupported Open GoPro operations.
package transport

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Sentinel errors. Callers use errors.Is; the concrete *Error carries detail.
var (
	ErrNotFound    = errors.New("transport: not found")
	ErrForbidden   = errors.New("transport: forbidden")
	ErrBadRequest  = errors.New("transport: bad request")
	ErrServer      = errors.New("transport: camera server error")
	ErrUnreachable = errors.New("transport: camera unreachable")
	ErrTimeout     = errors.New("transport: request timed out")
	ErrBadResponse = errors.New("transport: unexpected response body")
)

// Error is the detailed failure returned by every Client method.
type Error struct {
	Op     string // logical operation, e.g. "GET /gopro/version"
	URL    string
	Status int    // HTTP status, 0 when the request never completed
	Body   []byte // first bytes of the response body, for diagnostics
	Err    error  // sentinel or underlying error
}

func (e *Error) Error() string {
	if e.Status != 0 {
		return fmt.Sprintf("%s: HTTP %d: %s", e.Op, e.Status, bytes.TrimSpace(truncate(e.Body, 120)))
	}
	return fmt.Sprintf("%s: %v", e.Op, e.Err)
}

// Unwrap exposes the sentinel for errors.Is.
func (e *Error) Unwrap() error { return e.Err }

// Endpoint identifies a camera HTTP server.
type Endpoint struct {
	Host string // IP literal or hostname
	Port int
}

// String returns host:port.
func (e Endpoint) String() string { return net.JoinHostPort(e.Host, strconv.Itoa(e.Port)) }

// BaseURL returns the http:// origin.
func (e Endpoint) BaseURL() string { return "http://" + e.String() }

// Client talks to one camera.
type Client struct {
	ep      Endpoint
	http    *http.Client
	timeout time.Duration
	retries int
	backoff time.Duration
	log     *slog.Logger
	userAgt string
}

// Option configures a Client.
type Option func(*Client)

// WithHTTPClient replaces the underlying *http.Client (tests, custom dialers).
func WithHTTPClient(h *http.Client) Option { return func(c *Client) { c.http = h } }

// WithTimeout sets the per-request deadline (default 5s).
func WithTimeout(d time.Duration) Option { return func(c *Client) { c.timeout = d } }

// WithRetries sets how many times idempotent requests are retried on
// connection failures and timeouts (default 2). Never applies to non-idempotent calls.
func WithRetries(n int, initialBackoff time.Duration) Option {
	return func(c *Client) { c.retries, c.backoff = n, initialBackoff }
}

// WithLogger sets a structured logger for request tracing (default: discard).
func WithLogger(l *slog.Logger) Option { return func(c *Client) { c.log = l } }

// WithUserAgent sets the User-Agent header.
func WithUserAgent(ua string) Option { return func(c *Client) { c.userAgt = ua } }

// New creates a Client for the endpoint.
func New(ep Endpoint, opts ...Option) *Client {
	c := &Client{
		ep:      ep,
		timeout: 5 * time.Second,
		retries: 2,
		backoff: 200 * time.Millisecond,
		log:     slog.New(slog.DiscardHandler),
		userAgt: "gopro9-cli",
	}
	for _, o := range opts {
		o(c)
	}
	if c.http == nil {
		c.http = &http.Client{Transport: &http.Transport{
			DisableKeepAlives:   true, // the camera closes every connection anyway
			MaxIdleConns:        1,
			DialContext:         (&net.Dialer{Timeout: 2 * time.Second}).DialContext,
			TLSHandshakeTimeout: 0,
			Proxy:               nil, // never route camera traffic through a proxy
		}}
	}
	return c
}

// Endpoint returns the configured endpoint.
func (c *Client) Endpoint() Endpoint { return c.ep }

// Response is a fully read HTTP response.
type Response struct {
	Status      int
	ContentType string
	Body        []byte
}

// Request describes one camera call.
//
// The camera's CGI handler parses query parameters positionally: for
// /gopro/camera/setting the `setting` parameter must precede `option` or
// the camera answers 400. url.Values.Encode sorts keys alphabetically and
// would break that, so Params preserves the caller's order.
type Request struct {
	Path       string     // e.g. "/gopro/camera/state"
	Params     Params     // ordered query parameters (preferred)
	Query      url.Values // unordered query parameters (appended after Params)
	Headers    http.Header
	Idempotent bool // enables retry on transport failures
	// Accept lists status codes that are not errors (default: 2xx only).
	Accept []int
	// Timeout overrides the client's per-request timeout when positive. Slow
	// mode switches (webcam start/exit) need more than the default.
	Timeout time.Duration
}

// Get performs the request and reads the whole body.
func (c *Client) Get(ctx context.Context, req Request) (*Response, error) {
	resp, body, err := c.do(ctx, http.MethodGet, req, nil) //nolint:bodyclose // closed via deferred body.Close
	if err != nil {
		return nil, err
	}
	defer func() { _ = body.Close() }()
	data, rerr := io.ReadAll(body)
	if rerr != nil {
		return nil, &Error{Op: op(http.MethodGet, req.Path), URL: c.url(req), Status: resp.StatusCode, Err: rerr}
	}
	return &Response{Status: resp.StatusCode, ContentType: resp.Header.Get("Content-Type"), Body: data}, nil
}

// Open performs the request and returns the streaming body. The caller must Close it.
// Used for media downloads where the body may be gigabytes.
func (c *Client) Open(ctx context.Context, req Request) (*http.Response, io.ReadCloser, error) {
	return c.do(ctx, http.MethodGet, req, nil)
}

// Post sends a body (multipart or form) and reads the response.
func (c *Client) Post(ctx context.Context, req Request, contentType string, body io.Reader) (*Response, error) {
	if req.Headers == nil {
		req.Headers = http.Header{}
	}
	req.Headers.Set("Content-Type", contentType)
	resp, rc, err := c.do(ctx, http.MethodPost, req, body) //nolint:bodyclose // closed via deferred rc.Close
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()
	data, rerr := io.ReadAll(rc)
	if rerr != nil {
		return nil, &Error{Op: op(http.MethodPost, req.Path), URL: c.url(req), Status: resp.StatusCode, Err: rerr}
	}
	return &Response{Status: resp.StatusCode, ContentType: resp.Header.Get("Content-Type"), Body: data}, nil
}

func (c *Client) url(req Request) string {
	u := c.ep.BaseURL() + req.Path
	q := req.Params.Encode()
	if len(req.Query) > 0 {
		if q != "" {
			q += "&"
		}
		q += req.Query.Encode()
	}
	if q != "" {
		u += "?" + q
	}
	return u
}

// Param is one query parameter.
type Param struct{ Key, Value string }

// Params is an ordered list of query parameters.
type Params []Param

// P builds a Param.
func P(key, value string) Param { return Param{Key: key, Value: value} }

// Encode renders the parameters in order, percent-escaped. Forward slashes
// are left literal: the camera's media endpoints reject "%2F" in path values.
func (p Params) Encode() string {
	var b strings.Builder
	for i, kv := range p {
		if i > 0 {
			b.WriteByte('&')
		}
		b.WriteString(url.QueryEscape(kv.Key))
		b.WriteByte('=')
		b.WriteString(strings.ReplaceAll(url.QueryEscape(kv.Value), "%2F", "/"))
	}
	return b.String()
}

func (c *Client) do(ctx context.Context, method string, req Request, body io.Reader) (*http.Response, io.ReadCloser, error) {
	fullURL := c.url(req)
	opName := op(method, req.Path)
	attempts := 1
	if req.Idempotent && body == nil {
		attempts += c.retries
	}
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 {
			wait := c.backoff << (attempt - 1)
			c.log.DebugContext(ctx, "transport retry", "op", opName, "attempt", attempt, "wait", wait)
			select {
			case <-ctx.Done():
				return nil, nil, &Error{Op: opName, URL: fullURL, Err: ctxErr(ctx)}
			case <-time.After(wait):
			}
		}
		resp, rc, err := c.once(ctx, method, fullURL, opName, req, body)
		if err == nil {
			return resp, rc, nil
		}
		lastErr = err
		if !retryable(err) {
			break
		}
	}
	return nil, nil, lastErr
}

func (c *Client) once(ctx context.Context, method, fullURL, opName string, req Request, body io.Reader) (*http.Response, io.ReadCloser, error) {
	timeout := c.timeout
	if req.Timeout > 0 {
		timeout = req.Timeout
	}
	rctx, cancel := context.WithTimeout(ctx, timeout)
	httpReq, err := http.NewRequestWithContext(rctx, method, fullURL, body)
	if err != nil {
		cancel()
		return nil, nil, &Error{Op: opName, URL: fullURL, Err: err}
	}
	// The camera rejects requests whose Host is not its own address.
	httpReq.Host = c.ep.String()
	httpReq.Header.Set("User-Agent", c.userAgt)
	httpReq.Header.Set("Accept", "*/*")
	for k, vs := range req.Headers {
		for _, v := range vs {
			httpReq.Header.Add(k, v)
		}
	}
	start := time.Now()
	resp, err := c.http.Do(httpReq)
	if err != nil {
		cancel()
		c.log.DebugContext(ctx, "transport error", "op", opName, "err", err, "elapsed", time.Since(start))
		return nil, nil, &Error{Op: opName, URL: fullURL, Err: classify(ctx, err)}
	}
	c.log.DebugContext(ctx, "transport response", "op", opName, "status", resp.StatusCode, "elapsed", time.Since(start))
	if !accepted(resp.StatusCode, req.Accept) {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		_ = resp.Body.Close()
		cancel()
		return nil, nil, &Error{Op: opName, URL: fullURL, Status: resp.StatusCode, Body: snippet, Err: statusErr(resp.StatusCode)}
	}
	// The body outlives this function; tie the timeout cancel to Close.
	return resp, &cancelOnClose{ReadCloser: resp.Body, cancel: cancel}, nil
}

type cancelOnClose struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (c *cancelOnClose) Close() error {
	err := c.ReadCloser.Close()
	c.cancel()
	return err
}

func accepted(status int, accept []int) bool {
	if len(accept) == 0 {
		return status >= 200 && status < 300
	}
	for _, a := range accept {
		if a == status {
			return true
		}
	}
	return false
}

func statusErr(status int) error {
	switch {
	case status == http.StatusNotFound:
		return ErrNotFound
	case status == http.StatusForbidden:
		return ErrForbidden
	case status >= 400 && status < 500:
		return ErrBadRequest
	case status >= 500:
		return ErrServer
	default:
		return ErrBadResponse
	}
}

func classify(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return ctxErr(ctx)
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return fmt.Errorf("%w: %w", ErrTimeout, err)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%w: %w", ErrTimeout, err)
	}
	return fmt.Errorf("%w: %w", ErrUnreachable, err)
}

func ctxErr(ctx context.Context) error {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return fmt.Errorf("%w: %w", ErrTimeout, ctx.Err())
	}
	return ctx.Err()
}

func retryable(err error) bool {
	return errors.Is(err, ErrUnreachable) || errors.Is(err, ErrTimeout) && !errors.Is(err, context.Canceled)
}

func op(method, path string) string { return method + " " + path }

func truncate(b []byte, n int) []byte {
	if len(b) <= n {
		return b
	}
	return append(b[:n:n], []byte("...")...)
}
