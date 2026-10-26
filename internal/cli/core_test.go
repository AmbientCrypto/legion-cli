package cli

import (
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/AmbientCrypto/legion-cli/internal/client"
)

func TestVersion(t *testing.T) {
	h := newHarness(t)
	out, _ := h.mustRun("version")
	contains(t, "version", out, "legion version 1.2.3", "abc1234", "2026-10-26")
}

func TestRootHelp(t *testing.T) {
	h := newHarness(t)
	out, _ := h.mustRun("--help")
	contains(t, "help", out, "Getting started", "Compute", "Account", "Other", "Quickstart", "legion auth login", "legion offer list", "legion rental create", "legion ssh", "legion help environment")
}

func TestHelpTopics(t *testing.T) {
	h := newHarness(t)
	out, _ := h.mustRun("help", "environment")
	contains(t, "environment", out, "LEGION_API_KEY", "LEGION_API_URL", "LEGION_PROFILE", "LEGION_OUTPUT", "LEGION_CONFIG_DIR", "flag", "profile", "default")
	out, _ = h.mustRun("help", "exit-codes")
	contains(t, "exit-codes", out, "130", "price changed", "insufficient credit", "usage")
}

func TestUnknownCommandSuggests(t *testing.T) {
	h := newHarness(t)
	_, e, c := h.run("rentl", "list")
	if c != client.ExitUsage {
		t.Fatalf("exit %d", c)
	}
	contains(t, "stderr", e, "Error:", "Did you mean", "rental", "Hint: legion help")
}

func TestUsageErrorExit2(t *testing.T) {
	h := newHarness(t)
	_, e, c := h.run("rental", "get")
	if c != 2 {
		t.Fatalf("exit %d: %s", c, e)
	}
	contains(t, "stderr", e, "Error:", "Hint: legion rental get --help")
	_, _, c = h.run("rental", "list", "--nonsense")
	if c != 2 {
		t.Fatalf("bad flag exit %d", c)
	}
}

func TestNotLoggedIn(t *testing.T) {
	h := newHarness(t)
	h.noKey = true
	_, e, c := h.run("rental", "list")
	if c != client.ExitAuth {
		t.Fatalf("exit %d", c)
	}
	contains(t, "stderr", e, "Error: not logged in", "Code: not_logged_in", "Hint: legion auth login")
	if len(h.calls) != 0 {
		t.Fatal("must not call the API without a key")
	}
}

func TestErrorShape(t *testing.T) {
	h := newHarness(t)
	h.fail("GET /v1/rentals", 401, "unauthorized", "bad token")
	_, e, c := h.run("rental", "list")
	if c != 3 {
		t.Fatal(c)
	}
	contains(t, "stderr", e, "Error: bad token", "Code: unauthorized", "Hint: legion auth login")
}

func TestDescribeMapping(t *testing.T) {
	tests := []struct {
		name    string
		err     *client.Error
		path    string
		exit    int
		hint    string
		hintNot string
	}{
		{"unauthorized", &client.Error{Status: 401, Code: "unauthorized"}, "rental list", 3, "legion auth login", ""},
		{"scope", &client.Error{Status: 403, Code: "api_key_scope"}, "rental list", 3, "scope full", ""},
		{"credit", &client.Error{Status: 402, Code: "insufficient_credit"}, "rental create", 6, "legion billing balance", ""},
		{"price", &client.Error{Status: 409, Code: "price_changed"}, "rental create", 5, "Re-run", ""},
		{"notfound rental", &client.Error{Status: 404, Code: "not_found"}, "rental get", 4, "legion rental list", ""},
		{"notfound offer", &client.Error{Status: 404, Code: "not_found"}, "offer get", 4, "legion offer list", ""},
		{"notfound sshkey", &client.Error{Status: 404, Code: "not_found"}, "ssh-key remove", 4, "legion ssh-key list", ""},
		{"validation", &client.Error{Status: 400, Code: "invalid_request", Detail: "gpu_count: must be >= 1"}, "rental create", 1, "legion rental create --help", ""},
		{"gpus", &client.Error{Status: 409, Code: "gpus_unavailable"}, "rental create", 5, "legion offer list", ""},
		{"rate", &client.Error{Status: 429, Code: "rate_limited"}, "offer list", 1, "Wait", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := Describe(tt.err, tt.path)
			if d.Exit != tt.exit || !strings.Contains(d.Hint, tt.hint) || d.Code != tt.err.Code {
				t.Fatalf("%+v", d)
			}
		})
	}
	pc := Describe(&client.Error{Status: 409, Code: "price_changed", CurrentPrice: &moneyUSD3}, "rental create")
	contains(t, "price msg", pc.Message, "$3.00")
}

func TestAuthLoginWithToken(t *testing.T) {
	h := newHarness(t)
	h.noKey = true
	me := meFixture()
	h.on("GET /v1/me", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer lgn_pasted" {
			problem(w, 401, "unauthorized", "no")
			return
		}
		writeJSON(w, 200, me)
	})
	h.stdin = "lgn_pasted\n"
	_, e := h.mustRun("auth", "login", "--with-token")
	contains(t, "stderr", e, "Logged in as Ann Lee (org Acme)")
	b, err := os.ReadFile(filepath.Join(h.dir, "credentials.yaml"))
	if err != nil || !strings.Contains(string(b), "lgn_pasted") {
		t.Fatalf("creds: %s %v", b, err)
	}
	if runtime.GOOS != "windows" {
		st, _ := os.Stat(filepath.Join(h.dir, "credentials.yaml"))
		if st.Mode().Perm() != 0o600 {
			t.Fatal(st.Mode())
		}
	}
	// Now logged in without env: whoami works from the stored key.
	h.stdin = ""
	out, _ := h.mustRun("whoami")
	contains(t, "whoami", out, "ann@acme.example", "Acme")
}

func TestAuthLoginRejectsBadToken(t *testing.T) {
	h := newHarness(t)
	h.noKey = true
	h.fail("GET /v1/me", 401, "unauthorized", "invalid API key")
	h.stdin = "lgn_bad\n"
	_, e, c := h.run("auth", "login", "--with-token")
	if c != 3 {
		t.Fatal(c)
	}
	contains(t, "stderr", e, "invalid API key")
	if _, err := os.Stat(filepath.Join(h.dir, "credentials.yaml")); err == nil {
		t.Fatal("bad token was saved")
	}
}

func TestAuthLoginInteractive(t *testing.T) {
	h := newHarness(t)
	h.noKey = true
	h.reply("GET /v1/me", 200, meFixture())
	h.tty, h.secret = true, "lgn_typed"
	_, e := h.mustRun("auth", "login")
	if len(h.opened) != 1 || h.opened[0] != "https://legion.ambient.xyz/settings/api-keys" {
		t.Fatalf("opened %v", h.opened)
	}
	contains(t, "stderr", e, "full", "Logged in as")
	h2 := newHarness(t)
	h2.noKey = true
	h2.reply("GET /v1/me", 200, meFixture())
	h2.tty, h2.secret = true, "lgn_typed"
	h2.mustRun("auth", "login", "--no-browser")
	if len(h2.opened) != 0 {
		t.Fatal("browser opened despite --no-browser")
	}
}

func TestAuthLoginNonTTYNeedsToken(t *testing.T) {
	h := newHarness(t)
	h.noKey = true
	_, e, c := h.run("auth", "login")
	if c != 2 {
		t.Fatal(c)
	}
	contains(t, "stderr", e, "--with-token")
}

func TestAuthLogoutAndStatus(t *testing.T) {
	h := newHarness(t)
	h.noKey = true
	h.reply("GET /v1/me", 200, meFixture())
	h.stdin = "lgn_k_long_secret_value\n"
	h.mustRun("auth", "login", "--with-token")
	out, _ := h.mustRun("auth", "status")
	contains(t, "status", out, "Ann Lee", "Acme", "default", "full")
	excludes(t, "status", out, "lgn_k_long")
	h.mustRun("auth", "logout")
	_, e, c := h.run("auth", "status")
	if c != 3 {
		t.Fatalf("status after logout = %d", c)
	}
	contains(t, "stderr", e, "not logged in")
}

func TestDebugNeverLeaksKey(t *testing.T) {
	h := newHarness(t)
	h.reply("GET /v1/me", 200, meFixture())
	_, e := h.mustRun("--debug", "whoami")
	contains(t, "debug", e, "GET /v1/me 200")
	excludes(t, "debug", e, "lgn_testkey", "Authorization", "Bearer")
}

func TestAPIURLPrecedence(t *testing.T) {
	h := newHarness(t)
	other := newHarness(t)
	other.reply("GET /v1/me", 200, meFixture())
	h.fail("GET /v1/me", 500, "boom", "env server used")
	// flag beats LEGION_API_URL
	h.mustRun("--api-url", other.srv.URL, "whoami")
	if other.count("GET", "/v1/me") != 1 || h.count("GET", "/v1/me") != 0 {
		t.Fatal("flag did not win over env")
	}
	// profile url used when env absent
	h.noKey = false
	h.env["LEGION_API_URL"] = ""
	h.mustRun("config", "set", "api_url", other.srv.URL)
	h.mustRun("whoami")
	if other.count("GET", "/v1/me") != 2 {
		t.Fatal("profile url not used")
	}
}

func TestConfigCommands(t *testing.T) {
	h := newHarness(t)
	h.mustRun("config", "set", "output", "json")
	out, _ := h.mustRun("config", "get", "output")
	if strings.TrimSpace(out) != "json" {
		t.Fatalf("get: %q", out)
	}
	out, _ = h.mustRun("config", "list")
	contains(t, "list", out, "output", "json", "profile", "api_url", "default")
	h.mustRun("config", "unset", "output")
	out, _ = h.mustRun("config", "get", "output")
	if strings.TrimSpace(out) != "table" {
		t.Fatalf("after unset: %q", out)
	}
	_, e, c := h.run("config", "set", "api_url", "nonsense")
	if c != 2 {
		t.Fatal(c, e)
	}
	_, _, c = h.run("config", "get", "bogus")
	if c != 2 {
		t.Fatal(c)
	}
}

func TestProfileCommands(t *testing.T) {
	h := newHarness(t)
	h.mustRun("profile", "create", "staging", "--api-url", "https://staging.example")
	h.mustRun("profile", "create", "prod")
	out, _ := h.mustRun("profile", "list")
	contains(t, "list", out, "staging", "prod", "https://staging.example")
	h.mustRun("profile", "use", "staging")
	out, _ = h.mustRun("profile", "current")
	if strings.TrimSpace(out) != "staging" {
		t.Fatal(out)
	}
	_, _, c := h.run("profile", "use", "ghost")
	if c != 4 {
		t.Fatalf("use unknown = %d", c)
	}
	_, _, c = h.run("profile", "delete", "staging")
	if c != 2 {
		t.Fatalf("delete current without --yes = %d", c)
	}
	h.mustRun("profile", "delete", "prod", "--yes")
	out, _ = h.mustRun("profile", "list", "-o", "name")
	if strings.TrimSpace(out) != "staging" {
		t.Fatalf("names: %q", out)
	}
}

func TestOutputEnvAndFlag(t *testing.T) {
	h := newHarness(t)
	h.reply("GET /v1/me", 200, meFixture())
	h.env["LEGION_OUTPUT"] = "json"
	out, _ := h.mustRun("whoami")
	contains(t, "json", out, `"name": "Ann Lee"`)
	out, _ = h.mustRun("whoami", "-o", "jsonpath=$.org.name")
	if strings.TrimSpace(out) != "Acme" {
		t.Fatalf("%q", out)
	}
	_, _, c := h.run("whoami", "-o", "xml")
	if c != 2 {
		t.Fatal(c)
	}
}

func TestColour(t *testing.T) {
	h := newHarness(t)
	h.tty = true // stderr is a terminal
	_, e, _ := h.run("rental", "frob")
	contains(t, "tty errors are coloured", e, "\x1b[")
	_, e, _ = h.run("rental", "frob", "--no-color")
	excludes(t, "--no-color", e, "\x1b[")
	h.env["NO_COLOR"] = "1"
	_, e, _ = h.run("rental", "frob")
	excludes(t, "NO_COLOR", e, "\x1b[")
}
