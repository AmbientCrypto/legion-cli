package cli

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/AmbientCrypto/legion-controller/api"
	"github.com/spf13/cobra"
)

// Browser sign-in (LGN-490, ADR-0014): authorization code + PKCE S256 over a
// loopback redirect (RFC 8252). The person approves on Legion's consent page,
// which mints an ordinary full API key; legion redeems the code for it with
// its verifier. The code only ever reaches this computer.

const (
	cliClientID  = "legion-cli"
	loginTimeout = 5 * time.Minute
)

// callbackPage is all the browser is shown: nothing from the request is echoed.
const callbackPage = `<!doctype html><meta charset="utf-8"><title>Legion CLI</title>
<body style="font-family:system-ui,sans-serif;margin:4rem auto;max-width:32rem;text-align:center">
<h1 style="font-size:1.25rem">%s</h1><p>You can close this tab and go back to the terminal.</p></body>`

// callback is what the loopback listener received.
type callback struct {
	code string
	err  error
}

func randomString() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// origin is scheme://host of u, the issuer the controller names in `iss`.
func origin(u string) string {
	p, err := url.Parse(u)
	if err != nil {
		return u
	}
	return p.Scheme + "://" + p.Host
}

// browserLogin runs the sign-in and returns the new key and its name.
func (a *app) browserLogin(cmd *cobra.Command, apiURL string, openBrowser bool) (string, string, error) {
	verifier, err := randomString()
	if err != nil {
		return "", "", err
	}
	state, err := randomString()
	if err != nil {
		return "", "", err
	}
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", "", errorf("legion auth login --paste", "cannot listen on this computer for the sign-in: %v", err)
	}
	redirect := fmt.Sprintf("http://127.0.0.1:%d/callback", ln.Addr().(*net.TCPAddr).Port)
	issuer := origin(apiURL)
	got := make(chan callback, 1)
	srv := &http.Server{ReadHeaderTimeout: 10 * time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		// Anything without our state is not the answer to our request: ignore it and keep waiting.
		if r.URL.Path != "/callback" || subtle.ConstantTimeCompare([]byte(q.Get("state")), []byte(state)) != 1 {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		var cb callback
		switch {
		case q.Get("iss") != issuer:
			cb.err = errorf("legion auth login", "the sign-in answer came from issuer %q, not %s; nothing was saved", san(q.Get("iss")), issuer)
		case q.Get("error") == "access_denied":
			cb.err = errorf("legion auth login", "sign-in denied in the browser")
		case q.Get("error") != "":
			cb.err = errorf("legion auth login", "sign-in failed: %s", san(q.Get("error")))
		case q.Get("code") == "":
			cb.err = errorf("legion auth login", "the sign-in answer had no code")
		default:
			cb.code = q.Get("code")
		}
		title := "Signed in to Legion"
		if cb.err != nil {
			title = "Sign-in did not complete"
		}
		fmt.Fprintf(w, callbackPage, title)
		select {
		case got <- cb:
		default:
		}
	})}
	go func() { _ = srv.Serve(ln) }()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	q := url.Values{"response_type": {"code"}, "client_id": {cliClientID}, "redirect_uri": {redirect}, "state": {state},
		"code_challenge": {challenge}, "code_challenge_method": {"S256"}}
	authURL := apiURL + "/v1/oauth/authorize?" + q.Encode()
	if openBrowser {
		if err := a.env.OpenBrowser(authURL); err != nil {
			a.info("Could not open a browser (%v).", err)
		}
		a.info("Approve the sign-in in your browser. If it did not open, open this URL:\n  %s", authURL)
	} else {
		a.info("Open this URL in a browser on this computer and approve the sign-in:\n  %s", authURL)
	}
	a.info("Waiting for the browser (Ctrl-C to cancel)...")

	ctx, cancel := context.WithTimeout(a.ctx(cmd), loginTimeout)
	defer cancel()
	var cb callback
	select {
	case cb = <-got:
	case <-ctx.Done():
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return "", "", errorf("legion auth login", "no answer from the browser in %s", loginTimeout)
		}
		return "", "", ctx.Err()
	}
	if cb.err != nil {
		return "", "", cb.err
	}

	c, err := a.newClient(apiURL, "")
	if err != nil {
		return "", "", err
	}
	body := api.ExchangeCliLoginCodeRequest{Code: cb.code, CodeVerifier: verifier, RedirectUri: redirect}
	if host, err := a.env.Hostname(); err == nil && strings.TrimSpace(host) != "" {
		body.DeviceName = &host
	}
	resp, err := c.ExchangeCliLoginCodeWithResponse(a.ctx(cmd), body)
	if err := a.ok(resp, err); err != nil {
		return "", "", err
	}
	created, err := need(resp.JSON201)
	if err != nil {
		return "", "", err
	}
	return created.Token, created.ApiKey.Name, nil
}
