package cli

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/AmbientCrypto/legion-controller/api"
)

// Browser sign-in (LGN-490, ADR-0014): legion listens on a loopback port, the
// person approves in the browser, the controller redirects there with a code,
// and legion redeems it with its PKCE verifier for an API key.

// fakeSignIn makes the fake controller an authorization server for legion-cli
// and the harness's browser a person who approves: GET authorize answers with
// the loopback redirect (as the consent page would after approval), and
// POST /v1/oauth/cli-token checks the code and verifier and returns the key.
func fakeSignIn(t *testing.T, h *harness, iss func(string) string) {
	t.Helper()
	const code = "the-code"
	var challenge, redirect string
	h.on("GET /v1/oauth/authorize", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("client_id") != "legion-cli" || q.Get("response_type") != "code" || q.Get("code_challenge_method") != "S256" || len(q.Get("code_challenge")) != 43 || q.Get("state") == "" {
			t.Errorf("authorize query %v", q)
		}
		challenge, redirect = q.Get("code_challenge"), q.Get("redirect_uri")
		u, err := url.Parse(redirect)
		if err != nil || u.Scheme != "http" || !strings.HasPrefix(u.Host, "127.0.0.1:") || u.Path != "/callback" {
			t.Errorf("redirect_uri %q, want http://127.0.0.1:<port>/callback", redirect)
		}
		v := url.Values{"code": {code}, "state": {q.Get("state")}, "iss": {iss(h.srv.URL)}}
		http.Redirect(w, r, redirect+"?"+v.Encode(), http.StatusFound)
	})
	h.on("POST /v1/oauth/cli-token", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Error("the redemption carried a credential")
		}
		var body api.ExchangeCliLoginCodeRequest
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Code != code || body.RedirectUri != redirect || s256(body.CodeVerifier) != challenge {
			problem(w, 400, "invalid_grant", "bad code")
			return
		}
		writeJSON(w, 201, api.ApiKeyCreated{Token: "lgn_minted", ApiKey: api.ApiKey{Id: uid(77), Name: "Legion CLI (" + ptrOr(body.DeviceName, "") + ")", Scope: api.ApiKeyScopeFull}})
	})
	h.on("GET /v1/me", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer lgn_minted" {
			problem(w, 401, "unauthorized", "no")
			return
		}
		me := meFixture()
		me.Credential.ApiKeyId = ptr(uid(77))
		writeJSON(w, 200, me)
	})
	h.browser = func(u string) {
		resp, err := http.Get(u) // follows the redirect to legion's listener, like a browser
		if err != nil {
			return
		}
		_ = resp.Body.Close()
	}
}

func sameIssuer(s string) string { return s }

func TestAuthLoginSignsInThroughTheBrowser(t *testing.T) {
	h := newHarness(t)
	h.noKey, h.tty, h.hostname = true, true, "build-01"
	fakeSignIn(t, h, sameIssuer)
	_, e := h.mustRun("auth", "login")
	if len(h.opened) != 1 || !strings.HasPrefix(h.opened[0], h.srv.URL+"/v1/oauth/authorize?") {
		t.Fatalf("opened %v, want the authorize URL", h.opened)
	}
	contains(t, "stderr", e, h.opened[0], "Logged in as Ann Lee (org Acme)", "Legion CLI (build-01)")
	excludes(t, "stderr", e, "lgn_minted", "the-code")
	b, err := os.ReadFile(filepath.Join(h.dir, "credentials.yaml"))
	if err != nil || !strings.Contains(string(b), "lgn_minted") || !strings.Contains(string(b), "browser_login: true") {
		t.Fatalf("creds: %s %v", b, err)
	}
	if got := h.last("POST", "/v1/oauth/cli-token"); got == nil {
		t.Fatal("the code was never redeemed")
	}
}

func TestAuthLoginNoBrowserPrintsTheURL(t *testing.T) {
	h := newHarness(t)
	h.noKey, h.tty = true, true
	fakeSignIn(t, h, sameIssuer)
	browser := h.browser
	h.browser = nil
	// Nothing is opened: the person reads the printed URL and opens it themselves.
	var once sync.Once
	h.reads = func(text string) {
		i := strings.Index(text, h.srv.URL+"/v1/oauth/authorize?")
		if i < 0 {
			return
		}
		u := strings.Fields(text[i:])[0]
		once.Do(func() { go browser(u) })
	}
	_, e := h.mustRun("auth", "login", "--no-browser")
	contains(t, "stderr", e, "Logged in as")
	if len(h.opened) != 0 {
		t.Fatal("a browser was opened despite --no-browser")
	}
}

func TestAuthLoginRefusesAWrongIssuerOrState(t *testing.T) {
	h := newHarness(t)
	h.noKey, h.tty = true, true
	fakeSignIn(t, h, func(string) string { return "https://evil.example" })
	_, e, c := h.run("auth", "login")
	if c == 0 {
		t.Fatal("logged in with a code from another issuer")
	}
	contains(t, "stderr", e, "issuer")
	if h.count("POST", "/v1/oauth/cli-token") != 0 {
		t.Error("a code from another issuer was redeemed")
	}
}

func TestAuthLoginDeniedInTheBrowser(t *testing.T) {
	h := newHarness(t)
	h.noKey, h.tty = true, true
	h.on("GET /v1/oauth/authorize", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		v := url.Values{"error": {"access_denied"}, "state": {q.Get("state")}, "iss": {h.srv.URL}}
		http.Redirect(w, r, q.Get("redirect_uri")+"?"+v.Encode(), http.StatusFound)
	})
	h.browser = func(u string) {
		if resp, err := http.Get(u); err == nil {
			_ = resp.Body.Close()
		}
	}
	_, e, c := h.run("auth", "login")
	if c == 0 {
		t.Fatal("exit 0 after the person denied")
	}
	contains(t, "stderr", e, "denied")
}

func TestAuthLoginPasteStillWorks(t *testing.T) {
	h := newHarness(t)
	h.noKey = true
	h.reply("GET /v1/me", 200, meFixture())
	h.tty, h.secret = true, "lgn_typed"
	_, e := h.mustRun("auth", "login", "--paste", "--no-browser")
	contains(t, "stderr", e, "hidden", "Logged in as")
	if len(h.opened) != 0 {
		t.Fatal("browser opened despite --no-browser")
	}
}

func TestAuthLogoutRevokesABrowserLoginKey(t *testing.T) {
	h := newHarness(t)
	h.noKey, h.tty = true, true
	fakeSignIn(t, h, sameIssuer)
	h.mustRun("auth", "login")
	h.on("DELETE /v1/api-keys/"+uid(77).String(), func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer lgn_minted" {
			problem(w, 401, "unauthorized", "no")
			return
		}
		w.WriteHeader(204)
	})
	_, e := h.mustRun("auth", "logout")
	contains(t, "stderr", e, "Revoked the API key")
	if h.count("DELETE", "/v1/api-keys/"+uid(77).String()) != 1 {
		t.Fatal("the browser-login key was not revoked")
	}
}

func TestAuthLogoutKeepsAPastedKey(t *testing.T) {
	h := newHarness(t)
	h.noKey = true
	h.reply("GET /v1/me", 200, meFixture())
	h.stdin = "lgn_pasted\n"
	h.mustRun("auth", "login", "--with-token")
	h.mustRun("auth", "logout")
	if n := h.posts(); n != 0 {
		t.Fatalf("logout of a pasted key sent %d writes, want none", n)
	}
}

func s256(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
