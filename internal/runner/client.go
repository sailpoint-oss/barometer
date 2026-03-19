// Package runner provides HTTP client and execution for contract tests and Arazzo workflows.
package runner

import (
	"context"
	"crypto/tls"
	"net/http"
	"time"
)

// Client is an HTTP client configured for contract test runs (timeouts, optional retries, auth).
type Client struct {
	*http.Client
	// Auth adds headers (e.g. Authorization, X-API-Key) to every request when set.
	Auth func(*http.Request)
}

// Config configures the runner client.
type Config struct {
	Timeout       time.Duration // request timeout; 0 = 30s default
	MaxConns      int           // max idle conns per host; 0 = default
	SkipTLSVerify bool          // skip TLS verification (insecure)
	// Auth applies to every request (e.g. Bearer token, API key header).
	Auth func(*http.Request)
}

// NewClient returns a new runner client with the given config.
func NewClient(cfg *Config) *Client {
	if cfg == nil {
		cfg = &Config{}
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	transport := &http.Transport{
		MaxIdleConnsPerHost: cfg.MaxConns,
	}
	if transport.MaxIdleConnsPerHost == 0 {
		transport.MaxIdleConnsPerHost = 10
	}
	client := &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}
	if cfg.SkipTLSVerify {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	return &Client{Client: client, Auth: cfg.Auth}
}

// Do sends the request and returns the response. It respects context cancellation and applies Auth if set.
func (c *Client) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	req = req.Clone(ctx)
	if c.Auth != nil {
		c.Auth(req)
	}
	return c.Client.Do(req)
}
