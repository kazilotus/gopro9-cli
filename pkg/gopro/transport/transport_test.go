package transport

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newServer(t *testing.T, h http.HandlerFunc) Endpoint {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	return endpointOf(t, u.Host)
}

func endpointOf(t *testing.T, hostport string) Endpoint {
	t.Helper()
	host, portStr, err := net.SplitHostPort(hostport)
	require.NoError(t, err)
	port, err := strconv.Atoi(portStr)
	require.NoError(t, err)
	return Endpoint{Host: host, Port: port}
}

// closedEndpoint returns an address nothing listens on (connection refused).
func closedEndpoint(t *testing.T) Endpoint {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	require.NoError(t, ln.Close())
	return endpointOf(t, ln.Addr().String())
}

func TestEndpoint(t *testing.T) {
	t.Parallel()
	ep := Endpoint{Host: "172.24.135.51", Port: 8080}
	assert.Equal(t, "172.24.135.51:8080", ep.String())
	assert.Equal(t, "http://172.24.135.51:8080", ep.BaseURL())
}

func TestGetSetsHostHeaderAndUserAgent(t *testing.T) {
	t.Parallel()
	var gotHost, gotUA string
	ep := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotHost, gotUA = r.Host, r.UserAgent()
		_, _ = io.WriteString(w, `{"version":"2.0"}`)
	})
	c := New(ep, WithUserAgent("test-agent"))
	resp, err := c.Get(context.Background(), Request{Path: "/gopro/version"})
	require.NoError(t, err)
	assert.Equal(t, 200, resp.Status)
	assert.Equal(t, ep.String(), gotHost)
	assert.Equal(t, "test-agent", gotUA)
	assert.JSONEq(t, `{"version":"2.0"}`, string(resp.Body))
}

func TestGetJSON(t *testing.T) {
	t.Parallel()
	ep := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok":
			_, _ = io.WriteString(w, `{"version" : "2.0"}`)
		case "/empty":
			w.WriteHeader(http.StatusOK)
		case "/garbage":
			_, _ = io.WriteString(w, `<html>`)
		case "/missing":
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{}`)
		}
	})
	type v struct {
		Version string `json:"version"`
	}
	c := New(ep)
	ctx := context.Background()

	got, err := GetJSON[v](ctx, c, Request{Path: "/ok"})
	require.NoError(t, err)
	assert.Equal(t, "2.0", got.Version)

	got, err = GetJSON[v](ctx, c, Request{Path: "/empty"})
	require.NoError(t, err)
	assert.Empty(t, got.Version)

	_, err = GetJSON[v](ctx, c, Request{Path: "/garbage"})
	require.ErrorIs(t, err, ErrBadResponse)

	_, err = GetJSON[v](ctx, c, Request{Path: "/missing"})
	require.ErrorIs(t, err, ErrNotFound)
	var te *Error
	require.ErrorAs(t, err, &te)
	assert.Equal(t, 404, te.Status)
	assert.Contains(t, te.Error(), "HTTP 404")
}

func TestStatusMapping(t *testing.T) {
	t.Parallel()
	ep := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/403":
			http.Error(w, "Connection forbidden", http.StatusForbidden)
		case "/400":
			w.WriteHeader(http.StatusBadRequest)
		case "/500":
			w.WriteHeader(http.StatusInternalServerError)
		case "/206":
			w.WriteHeader(http.StatusPartialContent)
		}
	})
	c := New(ep)
	ctx := context.Background()
	_, err := c.Get(ctx, Request{Path: "/403"})
	require.ErrorIs(t, err, ErrForbidden)
	_, err = c.Get(ctx, Request{Path: "/400"})
	require.ErrorIs(t, err, ErrBadRequest)
	_, err = c.Get(ctx, Request{Path: "/500"})
	require.ErrorIs(t, err, ErrServer)
	resp, err := c.Get(ctx, Request{Path: "/206"})
	require.NoError(t, err)
	assert.Equal(t, 206, resp.Status)
	_, err = c.Get(ctx, Request{Path: "/403", Accept: []int{403}})
	require.NoError(t, err, "explicitly accepted status is not an error")
}

func TestQueryEncoding(t *testing.T) {
	t.Parallel()
	var got string
	ep := newServer(t, func(_ http.ResponseWriter, r *http.Request) { got = r.URL.RawQuery })
	c := New(ep)
	// Ordered params keep caller order (the camera parses them positionally).
	_, err := c.Get(context.Background(), Request{Path: "/gopro/camera/setting", Params: Params{P("setting", "2"), P("option", "9")}})
	require.NoError(t, err)
	assert.Equal(t, "setting=2&option=9", got)
	// Unordered values are appended after ordered ones, sorted.
	q := url.Values{}
	q.Set("z", "1")
	q.Set("a", "x y")
	_, err = c.Get(context.Background(), Request{Path: "/x", Params: Params{P("first", "1")}, Query: q})
	require.NoError(t, err)
	assert.Equal(t, "first=1&a=x+y&z=1", got)
	assert.Equal(t, "path=100GOPRO/GH010397.MP4", Params{P("path", "100GOPRO/GH010397.MP4")}.Encode(), "camera rejects %2F")
	assert.Equal(t, "q=a+b%26c", Params{P("q", "a b&c")}.Encode())
	assert.Empty(t, Params(nil).Encode())
}

func TestRetryOnlyForIdempotent(t *testing.T) {
	t.Parallel()
	// A listener that accepts nothing: connect succeeds on the OS level but no HTTP server answers.
	ep := closedEndpoint(t)

	var dials atomic.Int32
	countingDial := func(ctx context.Context, network, addr string) (net.Conn, error) {
		dials.Add(1)
		return (&net.Dialer{}).DialContext(ctx, network, addr)
	}
	hc := &http.Client{Transport: &http.Transport{DialContext: countingDial}}
	c := New(ep, WithHTTPClient(hc), WithRetries(2, time.Millisecond))

	_, err := c.Get(context.Background(), Request{Path: "/x", Idempotent: true})
	require.ErrorIs(t, err, ErrUnreachable)
	assert.Equal(t, int32(3), dials.Load(), "1 attempt + 2 retries")

	dials.Store(0)
	_, err = c.Get(context.Background(), Request{Path: "/x"})
	require.ErrorIs(t, err, ErrUnreachable)
	assert.Equal(t, int32(1), dials.Load(), "non-idempotent: no retry")
}

func TestTimeout(t *testing.T) {
	t.Parallel()
	ep := newServer(t, func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	})
	c := New(ep, WithTimeout(50*time.Millisecond), WithRetries(0, 0))
	_, err := c.Get(context.Background(), Request{Path: "/slow"})
	require.ErrorIs(t, err, ErrTimeout)
	// A per-request override extends the deadline (server answers after 2s; use a fast path instead).
	_, err = c.Get(context.Background(), Request{Path: "/slow", Timeout: 80 * time.Millisecond})
	require.ErrorIs(t, err, ErrTimeout)
}

func TestContextCancelDuringRetryBackoff(t *testing.T) {
	t.Parallel()
	c := New(closedEndpoint(t), WithRetries(5, time.Hour))
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(20 * time.Millisecond); cancel() }()
	_, err := c.Get(ctx, Request{Path: "/x", Idempotent: true})
	require.ErrorIs(t, err, context.Canceled)
}

func TestOpenStreamsBody(t *testing.T) {
	t.Parallel()
	payload := strings.Repeat("x", 10_000)
	ep := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "bytes=0-99", r.Header.Get("Range"))
		w.WriteHeader(http.StatusPartialContent)
		_, _ = io.WriteString(w, payload[:100])
	})
	c := New(ep)
	h := http.Header{}
	h.Set("Range", "bytes=0-99")
	resp, body, err := c.Open(context.Background(), Request{Path: "/videos/DCIM/100GOPRO/X.MP4", Headers: h}) //nolint:bodyclose // closed below
	require.NoError(t, err)
	defer func() { _ = body.Close() }()
	assert.Equal(t, 206, resp.StatusCode)
	b, err := io.ReadAll(body)
	require.NoError(t, err)
	assert.Len(t, b, 100)
}

func TestPost(t *testing.T) {
	t.Parallel()
	var ct, got string
	ep := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		ct = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		got = string(b)
		_, _ = io.WriteString(w, "ok")
	})
	c := New(ep)
	resp, err := c.Post(context.Background(), Request{Path: "/gp/gpSoftUpdate"}, "text/plain", strings.NewReader("hello"))
	require.NoError(t, err)
	assert.Equal(t, "text/plain", ct)
	assert.Equal(t, "hello", got)
	assert.Equal(t, "ok", string(resp.Body))
}

func TestErrorString(t *testing.T) {
	t.Parallel()
	e := &Error{Op: "GET /x", Err: ErrUnreachable}
	assert.Equal(t, "GET /x: transport: camera unreachable", e.Error())
	e = &Error{Op: "GET /x", Status: 500, Body: []byte(strings.Repeat("a", 200))}
	assert.Contains(t, e.Error(), "...")
	assert.ErrorIs(t, &Error{Err: ErrTimeout}, ErrTimeout)
}

func TestRetryable(t *testing.T) {
	t.Parallel()
	assert.True(t, retryable(ErrUnreachable))
	assert.True(t, retryable(ErrTimeout))
	assert.False(t, retryable(ErrNotFound))
	assert.False(t, retryable(context.Canceled))
}
