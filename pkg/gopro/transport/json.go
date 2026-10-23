package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
)

// GetJSON performs req and decodes the body into T.
//
// The camera answers unsupported Open GoPro operations with `404 {}`; those
// surface as ErrNotFound before decoding. An empty 200 body decodes to the
// zero value, which some endpoints (keep_alive, set commands) legitimately return.
func GetJSON[T any](ctx context.Context, c *Client, req Request) (T, error) {
	var out T
	resp, err := c.Get(ctx, req)
	if err != nil {
		return out, err
	}
	body := bytes.TrimSpace(resp.Body)
	if len(body) == 0 {
		return out, nil
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return out, &Error{
			Op: op("GET", req.Path), URL: c.url(req), Status: resp.Status, Body: truncate(body, 200),
			Err: fmt.Errorf("%w: %w", ErrBadResponse, err),
		}
	}
	return out, nil
}
