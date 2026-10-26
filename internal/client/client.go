// Package client builds the generated Legion API client with the CLI's
// transport (bearer auth, user agent, debug log) and turns problem+json
// answers into *Error.
package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/AmbientCrypto/legion-controller/api"

	"github.com/AmbientCrypto/legion-cli/internal/config"
)

// Exit codes, documented by `legion help exit-codes`.
const (
	ExitOK          = 0
	ExitError       = 1
	ExitUsage       = 2
	ExitAuth        = 3
	ExitNotFound    = 4
	ExitConflict    = 5
	ExitCredit      = 6
	ExitInterrupted = 130
)

// Options configures New.
type Options struct {
	BaseURL string
	Token   string
	Version string
	Debug   io.Writer     // method, path, status and duration only
	Timeout time.Duration // per request; 0 means none
	HTTP    *http.Client  // tests
}

// New returns the generated client over the CLI transport.
func New(o Options) (*api.ClientWithResponses, error) {
	hc := o.HTTP
	if hc == nil {
		hc = &http.Client{}
	}
	hc.Timeout = o.Timeout
	base := hc.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	u, err := url.Parse(o.BaseURL)
	if err != nil {
		return nil, err
	}
	if err := config.CheckAPIURL(o.BaseURL); err != nil {
		return nil, err
	}
	hc.Transport = &transport{base: base, token: o.Token, host: u.Host, ua: "legion-cli/" + o.Version, debug: o.Debug}
	// The key must never follow a redirect to another host.
	hc.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if req.URL.Scheme != u.Scheme || req.URL.Host != u.Host {
			return fmt.Errorf("refusing to follow a redirect to %s", req.URL.Host)
		}
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		return nil
	}
	return api.NewClientWithResponses(strings.TrimRight(o.BaseURL, "/"), api.WithHTTPClient(hc))
}

type transport struct {
	base  http.RoundTripper
	token string
	host  string // the only host that gets the token
	ua    string
	debug io.Writer
}

func (t *transport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("User-Agent", t.ua)
	if t.token != "" && r.URL.Host == t.host {
		r.Header.Set("Authorization", "Bearer "+t.token)
	}
	start := time.Now()
	resp, err := t.base.RoundTrip(r)
	if t.debug != nil {
		dur := time.Since(start).Round(time.Millisecond)
		if err != nil {
			fmt.Fprintf(t.debug, "debug: %s %s failed after %s: %v\n", r.Method, r.URL.Path, dur, err)
		} else {
			fmt.Fprintf(t.debug, "debug: %s %s %d (%s)\n", r.Method, r.URL.Path, resp.StatusCode, dur)
		}
	}
	return resp, err
}

// Error is an RFC 9457 problem answer from the controller.
type Error struct {
	Status       int
	Code         string
	Title        string
	Detail       string
	CurrentPrice *api.Money // price_changed: what the host charges now
}

func (e *Error) Error() string {
	switch {
	case e.Detail != "":
		return e.Detail
	case e.Title != "":
		return e.Title
	}
	return fmt.Sprintf("the API answered HTTP %d", e.Status)
}

// authCodes are the 403 codes that mean "your credential is wrong"; the other
// 403s (org_suspended, custom_images_disabled ...) are plain errors.
var authCodes = map[string]bool{"forbidden": true, "unauthorized": true, "api_key_required": true, "api_key_scope": true, "session_required": true, "oauth_token_mcp_only": true}

// ExitCode maps the answer onto the CLI's exit codes.
func (e *Error) ExitCode() int {
	switch {
	case e.Status == 401 || (e.Status == 403 && authCodes[e.Code]):
		return ExitAuth
	case e.Status == 400 || e.Status == 422:
		return ExitUsage
	case e.Status == 404:
		return ExitNotFound
	case e.Status == 402 || e.Code == "insufficient_credit":
		return ExitCredit
	case e.Status == 409:
		return ExitConflict
	}
	return ExitError
}

// Check returns nil for a 2xx answer and an *Error otherwise.
func Check(resp *http.Response, body []byte) error {
	if resp == nil || (resp.StatusCode >= 200 && resp.StatusCode < 300) {
		return nil
	}
	e := &Error{Status: resp.StatusCode}
	var p api.Problem
	if json.Unmarshal(body, &p) == nil && (p.Code != "" || p.Title != "") {
		e.Code, e.Title, e.CurrentPrice = p.Code, p.Title, p.CurrentPricePerGpuHour
		if p.Detail != nil {
			e.Detail = *p.Detail
		}
	} else {
		e.Title = http.StatusText(resp.StatusCode)
	}
	return e
}

// NetworkError words a transport failure.
func NetworkError(err error, baseURL string) error {
	return fmt.Errorf("cannot reach the Legion API at %s: %w", baseURL, err)
}
