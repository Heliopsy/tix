// Package client implements core.Service over the tix HTTP API.
//
// It marshals, sends and reconstructs. Validation and business rules live on
// the server, so a rule added here would duplicate it and be bypassable.
package client

import (
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/thereisnotime/tix/internal/core"
)

var _ core.Service = (*Client)(nil)

// DefaultTimeout bounds a single unary request when no timeout is configured.
const DefaultTimeout = 30 * time.Second

// DefaultUserAgent identifies the client to the server.
const DefaultUserAgent = "tix-client"

// Client is a remote core.Service backed by a tix server.
type Client struct {
	base      *url.URL
	token     string
	cookie    string
	userAgent string
	timeout   time.Duration
	http      *http.Client

	mu      sync.Mutex
	closers []func()
}

// Option configures a Client.
type Option func(*Client)

// WithHTTPClient uses the given HTTP client for every request.
func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) {
		if h != nil {
			c.http = h
		}
	}
}

// WithTimeout bounds each unary request. Zero disables the bound.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) { c.timeout = d }
}

// WithUserAgent sets the User-Agent header.
func WithUserAgent(ua string) Option {
	return func(c *Client) {
		if ua != "" {
			c.userAgent = ua
		}
	}
}

// WithSessionCookie authenticates with a session cookie instead of a bearer token.
func WithSessionCookie(value string) Option {
	return func(c *Client) { c.cookie = value }
}

// New builds a client for the server at baseURL authenticating with token.
func New(baseURL, token string, opts ...Option) (*Client, error) {
	trimmed := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if trimmed == "" {
		return nil, core.Invalid("server url is required")
	}
	u, err := url.Parse(trimmed)
	if err != nil {
		return nil, core.Invalid("parsing server url %q: %v", baseURL, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, core.Invalid("server url %q must use http or https", baseURL)
	}

	c := &Client{
		base:      u,
		token:     token,
		userAgent: DefaultUserAgent,
		timeout:   DefaultTimeout,
		http:      &http.Client{},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c, nil
}

// Close releases client-side resources. The server is unaffected.
func (c *Client) Close() error {
	c.mu.Lock()
	closers := c.closers
	c.closers = nil
	c.mu.Unlock()

	for _, closer := range closers {
		closer()
	}
	c.http.CloseIdleConnections()
	return nil
}

func (c *Client) track(closer func()) {
	c.mu.Lock()
	c.closers = append(c.closers, closer)
	c.mu.Unlock()
}
