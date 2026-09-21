package sync

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

// Doer performs HTTP requests. The standard client satisfies it; a test
// supplies a recorded transport instead.
type Doer interface {
	Do(req *http.Request) (*http.Response, error)
}

// HTTPOptions configures a retrying client for one external system.
type HTTPOptions struct {
	System  string
	BaseURL string
	Config  SourceConfig

	Client     Doer
	Sleep      func(time.Duration)
	MaxRetries int
	BaseDelay  time.Duration
	MaxDelay   time.Duration
}

// Retry defaults, chosen so a throttled source is waited out rather than
// hammered, and a dead one gives up in bounded time.
const (
	DefaultMaxRetries = 5
	DefaultBaseDelay  = 250 * time.Millisecond
	DefaultMaxDelay   = 30 * time.Second
)

// HTTPClient issues authenticated JSON requests, pages politely, and retries a
// throttled or transiently failing source with exponential backoff.
type HTTPClient struct {
	system  string
	baseURL string
	cfg     SourceConfig

	client     Doer
	sleep      func(time.Duration)
	maxRetries int
	baseDelay  time.Duration
	maxDelay   time.Duration

	attempts int
	waited   time.Duration
}

// NewHTTPClient builds a client for one external system.
func NewHTTPClient(o HTTPOptions) (*HTTPClient, error) {
	base := strings.TrimRight(strings.TrimSpace(o.BaseURL), "/")
	if base == "" {
		base = strings.TrimRight(strings.TrimSpace(o.Config.BaseURL), "/")
	}
	if base == "" {
		return nil, core.Invalid("%s source has no base url configured; set %s",
			o.System, EnvPrefix+"<SOURCE>_"+EnvURL)
	}
	if _, err := url.Parse(base); err != nil {
		return nil, core.Invalid("%s base url is not a valid url", o.System)
	}
	c := &HTTPClient{
		system:     o.System,
		baseURL:    base,
		cfg:        o.Config,
		client:     o.Client,
		sleep:      o.Sleep,
		maxRetries: o.MaxRetries,
		baseDelay:  o.BaseDelay,
		maxDelay:   o.MaxDelay,
	}
	if c.client == nil {
		c.client = &http.Client{Timeout: time.Minute, CheckRedirect: sameOriginRedirect}
	}
	if c.sleep == nil {
		c.sleep = time.Sleep
	}
	if c.maxRetries <= 0 {
		c.maxRetries = DefaultMaxRetries
	}
	if c.baseDelay <= 0 {
		c.baseDelay = DefaultBaseDelay
	}
	if c.maxDelay <= 0 {
		c.maxDelay = DefaultMaxDelay
	}
	return c, nil
}

// Attempts reports how many requests the client has issued, retries included.
func (c *HTTPClient) Attempts() int { return c.attempts }

// Waited reports how long the client has spent backing off.
func (c *HTTPClient) Waited() time.Duration { return c.waited }

// GetJSON fetches a path and decodes the response into out, retrying a
// throttled or transiently failing source before giving up.
func (c *HTTPClient) GetJSON(ctx context.Context, path string, query url.Values, out any) error {
	target := c.baseURL + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}

	var last error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		body, retryAfter, err := c.once(ctx, target)
		switch {
		case err == nil:
			if out == nil {
				return nil
			}
			if err := json.Unmarshal(body, out); err != nil {
				return core.Internal("decoding %s response from %s", c.system, path)
			}
			return nil
		case retryAfter < 0:
			return err
		}
		last = err
		if attempt == c.maxRetries {
			break
		}
		if err := c.wait(ctx, attempt, retryAfter); err != nil {
			return err
		}
	}
	return core.Internal("%s did not answer %s after %d attempts", c.system, path, c.maxRetries+1).Wrap(last)
}

// once issues one request. A negative retryAfter means the failure is final.
func (c *HTTPClient) once(ctx context.Context, target string) ([]byte, time.Duration, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, -1, core.Invalid("%s request url is not usable", c.system)
	}
	req.Header.Set("Accept", "application/json")
	c.authenticate(req)

	c.attempts++
	resp, err := c.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, -1, ctx.Err()
		}
		return nil, 0, core.Internal("reaching %s", c.system)
	}
	defer func() { _, _ = io.Copy(io.Discard, resp.Body); _ = resp.Body.Close() }()

	body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		return nil, retryAfter(resp.Header.Get("Retry-After")), core.Internal("%s rate limited the import", c.system)
	case resp.StatusCode >= 500:
		return nil, retryAfter(resp.Header.Get("Retry-After")), core.Internal("%s answered %d", c.system, resp.StatusCode)
	case resp.StatusCode == http.StatusUnauthorized, resp.StatusCode == http.StatusForbidden:
		return nil, -1, core.Unauthenticated("%s rejected the configured credential (http %d)", c.system, resp.StatusCode)
	case resp.StatusCode >= 400:
		return nil, -1, core.Invalid("%s answered %d for the configured query", c.system, resp.StatusCode)
	case readErr != nil:
		return nil, 0, core.Internal("reading %s response", c.system)
	default:
		return body, 0, nil
	}
}

// maxResponseBytes bounds a single page, so a hostile source cannot exhaust
// memory during an import.
const maxResponseBytes = 32 << 20

// maxRedirects bounds a redirect chain the source is allowed to walk.
const maxRedirects = 5

// sameOriginRedirect keeps a redirect inside the origin the operator
// configured. A source is operator configuration rather than tenant input, so
// redirects are not refused outright the way webhook delivery refuses them;
// what is refused is a hop that leaves the configured scheme and host. That is
// what would otherwise carry the configured credential, or a request the
// operator believes is going to their tracker, to an arbitrary address such as
// the cloud metadata endpoint.
func sameOriginRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= maxRedirects {
		return core.Invalid("source redirected more than %d times", maxRedirects)
	}
	first := via[0].URL
	if req.URL.Scheme != first.Scheme || !strings.EqualFold(req.URL.Host, first.Host) {
		return core.Invalid("source redirected to %s://%s, which is not the configured origin",
			req.URL.Scheme, req.URL.Host)
	}
	return nil
}

// authenticate applies the configured credential. The value is written to the
// request and nowhere else.
func (c *HTTPClient) authenticate(req *http.Request) {
	if token := c.cfg.Token(); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
		return
	}
	if user, password := c.cfg.BasicAuth(); user != "" || password != "" {
		req.SetBasicAuth(user, password)
	}
}

// wait backs off, honouring a Retry-After the source supplied.
func (c *HTTPClient) wait(ctx context.Context, attempt int, hinted time.Duration) error {
	delay := c.baseDelay << attempt
	if hinted > 0 {
		delay = hinted
	}
	if delay > c.maxDelay {
		delay = c.maxDelay
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	c.waited += delay
	c.sleep(delay)
	return ctx.Err()
}

// retryAfter reads a Retry-After header written as seconds or as a date.
func retryAfter(raw string) time.Duration {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	if secs, err := strconv.Atoi(raw); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second
	}
	if when, err := http.ParseTime(raw); err == nil {
		if d := time.Until(when); d > 0 {
			return d
		}
	}
	return 0
}
