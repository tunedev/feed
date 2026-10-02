// Package httpjson is the one bounded, timed JSON GET every fetcher shares.
package httpjson

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Client sends every request with the same User-Agent and bounds every
// response body. The timeout is the http.Client's own.
type Client struct {
	http      *http.Client
	maxBytes  int64
	userAgent string
}

func New(c *http.Client, maxBytes int64, userAgent string) *Client {
	return &Client{http: c, maxBytes: maxBytes, userAgent: userAgent}
}

// Get fetches url and decodes its JSON body into v. A non-2xx status, or a
// body over the size bound, fails rather than decoding partial data. It reads
// one byte past the bound so a body exactly at it is told apart from one over.
func (c *Client) Get(ctx context.Context, url string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("httpjson: build request: %w", err)
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("httpjson: GET %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("httpjson: GET %s returned %d", url, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, c.maxBytes+1))
	if err != nil {
		return fmt.Errorf("httpjson: read %s: %w", url, err)
	}
	if int64(len(body)) > c.maxBytes {
		return fmt.Errorf("httpjson: %s exceeds max size of %d bytes", url, c.maxBytes)
	}
	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("httpjson: decode %s: %w", url, err)
	}
	return nil
}
