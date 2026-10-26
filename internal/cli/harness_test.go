package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/AmbientCrypto/legion-controller/api"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// call is one request the fake controller saw.
type call struct {
	Method, Path string
	Query        map[string][]string
	Header       http.Header
	Body         []byte
}

// harness is a fake controller plus an Env wired to it.
type harness struct {
	t        *testing.T
	srv      *httptest.Server
	mu       sync.Mutex
	calls    []call
	handlers map[string]http.HandlerFunc
	dir      string // LEGION_CONFIG_DIR
	home     string
	stdin    string
	stdinR   io.Reader // overrides stdin
	tty      bool
	secret   string
	noKey    bool
	env      map[string]string
	opened   []string
	browser  func(url string) // the person's browser, run when the CLI opens a URL
	hostname string
	reads    func(text string) // the person reading stderr as it is written
	execs    [][]string
	ctx      context.Context
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	h := &harness{t: t, handlers: map[string]http.HandlerFunc{}, dir: filepath.Join(t.TempDir(), "cfg"), home: t.TempDir(), env: map[string]string{}}
	h.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		h.mu.Lock()
		h.calls = append(h.calls, call{r.Method, r.URL.Path, r.URL.Query(), r.Header.Clone(), b})
		fn := h.handlers[r.Method+" "+r.URL.Path]
		h.mu.Unlock()
		if fn == nil {
			problem(w, 404, "not_found", "no such route in fake: "+r.Method+" "+r.URL.Path)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(b))
		fn(w, r)
	}))
	t.Cleanup(h.srv.Close)
	return h
}

func problem(w http.ResponseWriter, status int, code, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{"type": "about:blank", "title": code, "status": status, "code": code, "detail": detail})
}

// on registers a handler for "METHOD /path".
func (h *harness) on(route string, fn http.HandlerFunc) {
	h.mu.Lock()
	h.handlers[route] = fn
	h.mu.Unlock()
}

// reply answers route with a JSON body.
func (h *harness) reply(route string, status int, body any) {
	h.on(route, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if body != nil {
			json.NewEncoder(w).Encode(body)
		}
	})
}

func (h *harness) fail(route string, status int, code, detail string) {
	h.on(route, func(w http.ResponseWriter, r *http.Request) { problem(w, status, code, detail) })
}

func (h *harness) count(method, path string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := 0
	for _, c := range h.calls {
		if c.Method == method && c.Path == path {
			n++
		}
	}
	return n
}

func (h *harness) last(method, path string) *call {
	h.mu.Lock()
	defer h.mu.Unlock()
	for i := len(h.calls) - 1; i >= 0; i-- {
		if h.calls[i].Method == method && h.calls[i].Path == path {
			c := h.calls[i]
			return &c
		}
	}
	return nil
}

func (h *harness) posts() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := 0
	for _, c := range h.calls {
		if c.Method != "GET" {
			n++
		}
	}
	return n
}

// run executes the CLI and returns stdout, stderr and the exit code.
func (h *harness) run(args ...string) (string, string, int) {
	h.t.Helper()
	var out bytes.Buffer
	errb := &watched{fn: h.reads}
	env := map[string]string{"LEGION_CONFIG_DIR": h.dir, "LEGION_API_URL": h.srv.URL}
	if !h.noKey {
		env["LEGION_API_KEY"] = "lgn_testkey"
	}
	for k, v := range h.env {
		env[k] = v
	}
	var in io.Reader = strings.NewReader(h.stdin)
	if h.stdinR != nil {
		in = h.stdinR
	}
	e := Env{
		Ctx:   h.ctx,
		Stdin: in, Stdout: &out, Stderr: errb,
		Getenv:   func(k string) string { return env[k] },
		StdinTTY: h.tty, StdoutTTY: false, StderrTTY: h.tty,
		ReadSecret: func(context.Context) (string, error) { return h.secret, nil },
		OpenBrowser: func(u string) error {
			h.opened = append(h.opened, u)
			if h.browser != nil {
				go h.browser(u)
			}
			return nil
		},
		Hostname: func() (string, error) { return h.hostname, nil },
		LookPath: func(s string) (string, error) { return "/usr/bin/" + s, nil },
		Exec: func(path string, argv []string) error {
			h.execs = append(h.execs, append([]string{path}, argv...))
			return nil
		},
		Home: h.home, Poll: time.Millisecond, Version: "1.2.3", Commit: "abc1234", Date: "2026-10-26",
	}
	code := Execute(e, args)
	return out.String(), errb.String(), code
}

// watched is stderr as a person sees it: each write is shown to fn.
type watched struct {
	mu  sync.Mutex
	buf bytes.Buffer
	fn  func(string)
}

func (w *watched) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.fn != nil {
		w.fn(string(p))
	}
	return w.buf.Write(p)
}

func (w *watched) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}

// mustRun fails the test unless the exit code is 0.
func (h *harness) mustRun(args ...string) (string, string) {
	h.t.Helper()
	o, e, c := h.run(args...)
	if c != 0 {
		h.t.Fatalf("legion %v exited %d\nstdout: %s\nstderr: %s", args, c, o, e)
	}
	return o, e
}

func (h *harness) writeKey(name, content string) {
	h.t.Helper()
	p := filepath.Join(h.home, ".ssh", name)
	os.MkdirAll(filepath.Dir(p), 0o700)
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		h.t.Fatal(err)
	}
}

func contains(t *testing.T, label, got string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(got, w) {
			t.Errorf("%s missing %q:\n%s", label, w, got)
		}
	}
}

func excludes(t *testing.T, label, got string, nots ...string) {
	t.Helper()
	for _, w := range nots {
		if strings.Contains(got, w) {
			t.Errorf("%s should not contain %q:\n%s", label, w, got)
		}
	}
}

func uid(n int) openapi_types.UUID {
	var u openapi_types.UUID
	u[15] = byte(n)
	u[0] = 0x11
	return u
}

func money(c int64) api.Money { return api.Money{AmountMinor: c, Currency: "USD"} }

func meFixture() api.Me {
	return api.Me{
		Org:         api.MeOrg{Id: uid(1), Name: "Acme"},
		User:        api.User{Id: uid(2), Name: "Ann Lee", Email: "ann@acme.example"},
		Role:        "owner",
		Permissions: []api.Permission{},
		Credential:  &api.MeCredential{Kind: "api_key", Scope: api.ApiKeyScopeFull, Prefix: ptr("lgn_test")},
	}
}

func rentalFixture(n int, name string, state api.RentalState) api.Rental {
	r := api.Rental{
		Id: uid(n), Name: name, HostId: uid(50), State: state, GpuCount: 2, Image: "pytorch/pytorch:2",
		PricePerGpuHour: money(250), Resources: api.Resources{CpuCores: 16, DiskGib: 200, MemoryMib: 65536},
		CreatedAt: time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC), Tier: "on_demand",
		StoragePricePerGibMonth: ptr(money(10)),
	}
	if state == api.RentalStateRunning {
		r.Ssh = &api.SSHEndpoint{Host: "ssh.legion.example", Port: 2222, HostKeyFingerprint: ptr("SHA256:abc")}
	}
	return r
}

func offerFixture() api.Offer {
	return api.Offer{
		HostId: uid(50), HostName: "amber-falcon", GpuModel: "RTX 4090", GpuCount: 4, GpusFree: 4,
		PricePerGpuHour: money(250), GpuMemoryMib: 24576, StoragePricePerGibMonth: ptr(money(10)),
	}
}

func sshKeyFixture() api.SshKey {
	return api.SshKey{Id: uid(70), Label: "laptop", PublicKey: "ssh-ed25519 AAAAC3Nza laptop", Fingerprint: "SHA256:fp", KeyType: "ssh-ed25519", CreatedAt: time.Now()}
}

var moneyUSD3 = money(300)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func uidS(n int) string { return uid(n).String() }

func (h *harness) stdFixtures() {
	h.reply("GET /v1/offers/"+uidS(50), 200, api.OfferDetail{Offer: offerFixture(), UptimeDaily: []api.DayUptime{}})
	h.reply("GET /v1/billing/account", 200, api.BillingAccount{Balance: api.SignedMoney{AmountMinor: 5000, Currency: "USD"}, HasCredit: true})
	h.reply("GET /v1/ssh-keys", 200, api.SshKeyList{Items: []api.SshKey{sshKeyFixture()}})
}

// created answers POST /v1/rentals with a starting rental and GET with it
// starting once, then running.
func (h *harness) rentalLifecycle(name string, n int) {
	var polls int
	h.reply("POST /v1/rentals", 201, rentalFixture(n, name, api.RentalStateStarting))
	h.on("GET /v1/rentals/"+uidS(n), func(w http.ResponseWriter, r *http.Request) {
		polls++
		st := api.RentalStateStarting
		if polls > 1 {
			st = api.RentalStateRunning
		}
		writeJSON(w, 200, rentalFixture(n, name, st))
	})
}

func body[T any](t *testing.T, c *call) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(c.Body, &v); err != nil {
		t.Fatalf("body %s: %v", c.Body, err)
	}
	return v
}
