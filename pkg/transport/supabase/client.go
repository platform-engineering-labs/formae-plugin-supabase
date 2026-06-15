// © 2026 Platform Engineering Labs Inc.
//
// SPDX-License-Identifier: FSL-1.1-ALv2

// Package supabase is a minimal HTTP client for the Supabase Management API.
//
// Scope is deliberately narrow: Bearer-token auth, JSON in / JSON out,
// status-code-driven errors. Higher-level resource logic lives in
// pkg/resources/*.
package supabase

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	DefaultBaseURL = "https://api.supabase.com"
	defaultTimeout = 30 * time.Second
)

// Config configures the HTTP client.
type Config struct {
	BaseURL     string
	AccessToken string
	HTTPClient  *http.Client
	UserAgent   string
}

// Client talks to the Supabase Management API.
type Client struct {
	baseURL     string
	accessToken string
	http        *http.Client
	userAgent   string
}

// NewClient constructs a client. AccessToken is required.
func NewClient(cfg Config) (*Client, error) {
	if cfg.AccessToken == "" {
		return nil, fmt.Errorf("supabase: AccessToken is required")
	}
	base := cfg.BaseURL
	if base == "" {
		base = DefaultBaseURL
	}
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: defaultTimeout}
	}
	ua := cfg.UserAgent
	if ua == "" {
		ua = "formae-plugin-supabase/0.1.0"
	}
	return &Client{
		baseURL:     base,
		accessToken: cfg.AccessToken,
		http:        hc,
		userAgent:   ua,
	}, nil
}

// Request describes one API call.
type Request struct {
	Method string
	Path   string      // begins with "/" e.g. "/v1/projects"
	Body   interface{} // marshalled to JSON if non-nil
	Query  map[string]string
}

// Do executes a request. On 2xx, decodes the JSON body into out (if non-nil).
// On non-2xx, returns *APIError.
func (c *Client) Do(ctx context.Context, req Request, out interface{}) error {
	var bodyReader io.Reader
	if req.Body != nil {
		b, err := json.Marshal(req.Body)
		if err != nil {
			return fmt.Errorf("marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	url := c.baseURL + req.Path
	httpReq, err := http.NewRequestWithContext(ctx, req.Method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.accessToken)
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("User-Agent", c.userAgent)
	if req.Body != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}
	if len(req.Query) > 0 {
		q := httpReq.URL.Query()
		for k, v := range req.Query {
			q.Set(k, v)
		}
		httpReq.URL.RawQuery = q.Encode()
	}

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return fmt.Errorf("http: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    extractMessage(respBody),
			Body:       string(respBody),
		}
	}

	if out == nil || len(respBody) == 0 {
		return nil
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

// extractMessage best-effort pulls a "message" or "error" field from a
// Supabase error JSON body. Returns empty string if it cannot.
func extractMessage(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	var probe struct {
		Message string `json:"message"`
		Error   string `json:"error"`
		Msg     string `json:"msg"`
	}
	if err := json.Unmarshal(body, &probe); err != nil {
		return ""
	}
	switch {
	case probe.Message != "":
		return probe.Message
	case probe.Error != "":
		return probe.Error
	case probe.Msg != "":
		return probe.Msg
	}
	return ""
}
