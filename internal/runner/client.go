// Package runner provides HTTP client and execution for contract tests and Arazzo workflows.
package runner

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
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
	// TLSClientCertFile and TLSClientKeyFile load an optional client certificate (mTLS).
	TLSClientCertFile string
	TLSClientKeyFile  string
	// TLSCACertFile optionally adds a PEM CA bundle for verifying the server (e.g. private PKI).
	TLSCACertFile string
	// Auth applies to every request (e.g. Bearer token, API key header).
	Auth func(*http.Request)
}

// NewClient returns a new runner client with the given config.
func NewClient(cfg *Config) (*Client, error) {
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
	tlsConfig := &tls.Config{
		InsecureSkipVerify: cfg.SkipTLSVerify,
	}
	if cfg.TLSClientCertFile != "" || cfg.TLSClientKeyFile != "" {
		if cfg.TLSClientCertFile == "" || cfg.TLSClientKeyFile == "" {
			return nil, fmt.Errorf("TLS client cert requires both tlsClientCertFile and tlsClientKeyFile")
		}
		cert, err := tls.LoadX509KeyPair(cfg.TLSClientCertFile, cfg.TLSClientKeyFile)
		if err != nil {
			return nil, fmt.Errorf("load TLS client certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}
	if cfg.TLSCACertFile != "" {
		pemData, err := os.ReadFile(cfg.TLSCACertFile)
		if err != nil {
			return nil, fmt.Errorf("read TLS CA bundle: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pemData) {
			return nil, fmt.Errorf("parse TLS CA bundle %q: no certificates", cfg.TLSCACertFile)
		}
		tlsConfig.RootCAs = pool
	}
	if tlsConfig.InsecureSkipVerify || len(tlsConfig.Certificates) > 0 || tlsConfig.RootCAs != nil {
		transport.TLSClientConfig = tlsConfig
	}
	client := &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}
	return &Client{Client: client, Auth: cfg.Auth}, nil
}

// Do sends the request and returns the response. It respects context cancellation and applies Auth if set.
func (c *Client) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	req = req.Clone(ctx)
	if c.Auth != nil {
		c.Auth(req)
	}
	return c.Client.Do(req)
}
