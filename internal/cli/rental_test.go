package cli

import (
	"net/http"
	"strings"
	"testing"

	"github.com/AmbientCrypto/legion-controller/api"
)

// rentals registers GET /v1/rentals with three rentals.
func (h *harness) rentals() []api.Rental {
	rs := []api.Rental{
		rentalFixture(1, "quiet-river-7k2f", api.RentalStateRunning),
		rentalFixture(2, "quiet-meadow-3x9a", api.RentalStateStopped),
		rentalFixture(3, "brave-otter-0aa1", api.RentalStateRunning),
	}
	h.reply("GET /v1/rentals", 200, api.RentalPage{Items: rs})
	for _, r := range rs {
		h.reply("GET /v1/rentals/"+r.Id.String(), 200, r)
	}
	return rs
}

func TestRentalResolution(t *testing.T) {
	h := newHarness(t)
	h.rentals()
	tests := []struct {
		arg  string
		want string // rental name or ""
		code int
		msg  string
	}{
		{"quiet-river-7k2f", "quiet-river-7k2f", 0, ""},
		{uidS(3), "brave-otter-0aa1", 0, ""},
		{"brave", "brave-otter-0aa1", 0, ""},
		{"quiet-r", "quiet-river-7k2f", 0, ""},
		{"quiet", "", 2, "ambiguous"},
		{"nope", "", 4, "no rental"},
	}
	for _, tt := range tests {
		t.Run(tt.arg, func(t *testing.T) {
			out, errs, code := h.run("rental", "get", tt.arg, "-o", "name")
			if code != tt.code {
				t.Fatalf("exit %d: %s", code, errs)
			}
			if tt.want != "" && strings.TrimSpace(out) != tt.want {
				t.Fatalf("got %q", out)
			}
			if tt.msg != "" {
				contains(t, "stderr", errs, tt.msg)
			}
			if code == 4 {
				contains(t, "stderr", errs, "Hint: legion rental list")
			}
			if tt.msg == "ambiguous" {
				contains(t, "stderr", errs, "quiet-river-7k2f", "quiet-meadow-3x9a")
			}
		})
	}
}

func TestRentalResolutionPicker(t *testing.T) {
	h := newHarness(t)
	h.rentals()
	_, errs, code := h.run("rental", "get")
	if code != 2 {
		t.Fatalf("non-tty exit %d", code)
	}
	contains(t, "stderr", errs, "<rental>")
	h.tty = true
	h.stdin = "3\n"
	out, _ := h.mustRun("rental", "get", "-o", "name")
	if strings.TrimSpace(out) != "brave-otter-0aa1" {
		t.Fatalf("%q", out)
	}
}

func TestRentalListFormats(t *testing.T) {
	h := newHarness(t)
	h.rentals()
	out, _ := h.mustRun("rental", "list")
	contains(t, "table", out, "NAME", "STATE", "quiet-river-7k2f", "running", "$2.50")
	excludes(t, "table", out, "ssh.legion.example")
	out, _ = h.mustRun("rental", "list", "-o", "wide")
	contains(t, "wide", out, "ssh.legion.example")
	out, _ = h.mustRun("rental", "list", "-o", "name")
	if out != "quiet-river-7k2f\nquiet-meadow-3x9a\nbrave-otter-0aa1\n" {
		t.Fatalf("%q", out)
	}
	out, _ = h.mustRun("rental", "list", "-o", "jsonpath=$[?(@.state=='stopped')].name")
	if strings.TrimSpace(out) != "quiet-meadow-3x9a" {
		t.Fatalf("jsonpath %q", out)
	}
	out, _ = h.mustRun("rental", "list", "-o", "json")
	contains(t, "json", out, `"name": "quiet-river-7k2f"`)
	// machine formats stay clean even when the list is empty
	h.reply("GET /v1/rentals", 200, api.RentalPage{Items: []api.Rental{}})
	out, errs := h.mustRun("rental", "list", "-o", "json")
	if strings.TrimSpace(out) != "[]" {
		t.Fatalf("empty json %q", out)
	}
	_ = errs
}

func TestRentalListPaginates(t *testing.T) {
	h := newHarness(t)
	h.on("GET /v1/rentals", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("cursor") == "" {
			writeJSON(w, 200, api.RentalPage{Items: []api.Rental{rentalFixture(1, "aaa-bbb-1111", api.RentalStateRunning)}, NextCursor: ptr("c2")})
			return
		}
		writeJSON(w, 200, api.RentalPage{Items: []api.Rental{rentalFixture(2, "ccc-ddd-2222", api.RentalStateRunning)}})
	})
	out, _ := h.mustRun("rental", "list", "-o", "name")
	if out != "aaa-bbb-1111\nccc-ddd-2222\n" {
		t.Fatalf("%q", out)
	}
}

func TestRentalGetShowsConnect(t *testing.T) {
	h := newHarness(t)
	h.rentals()
	out, _ := h.mustRun("rental", "get", "quiet-river-7k2f")
	contains(t, "get", out, "quiet-river-7k2f", "running", "$2.50", "ssh -J legion@ssh.legion.example:2222", "legion ssh quiet-river-7k2f")
}

func TestRentalStop(t *testing.T) {
	h := newHarness(t)
	rs := h.rentals()
	h.reply("POST /v1/rentals/"+rs[0].Id.String()+"/stop", 200, rentalFixture(1, "quiet-river-7k2f", api.RentalStateStopping))
	_, errs, code := h.run("rental", "stop", "quiet-river-7k2f")
	if code != 2 || h.count("POST", "/v1/rentals/"+rs[0].Id.String()+"/stop") != 0 {
		t.Fatalf("non-tty without --yes: %d", code)
	}
	contains(t, "refusal", errs, "--yes")
	_, errs = h.mustRun("rental", "stop", "quiet-river-7k2f", "--yes", "--async")
	contains(t, "stop", errs, "workspace is kept", "per GiB", "legion rental start quiet-river-7k2f", "legion rental delete quiet-river-7k2f")
	if h.count("POST", "/v1/rentals/"+rs[0].Id.String()+"/stop") != 1 {
		t.Fatal("no stop POST")
	}
	// TTY confirmation text states the storage billing.
	h.tty, h.stdin = true, "n\n"
	_, errs, code = h.run("rental", "stop", "quiet-river-7k2f")
	if code == 0 {
		t.Fatal("declined stop succeeded")
	}
	contains(t, "prompt", errs, "billed for storage")
}

func TestRentalStopWaitsForStopped(t *testing.T) {
	h := newHarness(t)
	rs := h.rentals()
	id := rs[0].Id.String()
	h.reply("POST /v1/rentals/"+id+"/stop", 200, rentalFixture(1, "quiet-river-7k2f", api.RentalStateStopping))
	n := 0
	h.on("GET /v1/rentals/"+id, func(w http.ResponseWriter, r *http.Request) {
		n++
		st := api.RentalStateStopping
		if n > 1 {
			st = api.RentalStateStopped
		}
		writeJSON(w, 200, rentalFixture(1, "quiet-river-7k2f", st))
	})
	h.mustRun("rental", "stop", "quiet-river-7k2f", "--yes")
	if n < 2 {
		t.Fatalf("polled %d", n)
	}
}

func TestRentalStart(t *testing.T) {
	h := newHarness(t)
	rs := h.rentals()
	id := rs[1].Id.String()
	h.reply("POST /v1/rentals/"+id+"/start", 200, rentalFixture(2, "quiet-meadow-3x9a", api.RentalStateStarting))
	n := 0
	h.on("GET /v1/rentals/"+id, func(w http.ResponseWriter, r *http.Request) {
		n++
		writeJSON(w, 200, rentalFixture(2, "quiet-meadow-3x9a", api.RentalStateRunning))
	})
	_, errs := h.mustRun("rental", "start", "quiet-meadow-3x9a", "--yes")
	contains(t, "start", errs, "Started", "Connect: legion ssh quiet-meadow-3x9a")
	if h.count("POST", "/v1/rentals/"+id+"/start") != 1 {
		t.Fatal("no start POST")
	}
}

func TestRentalDeleteRequiresName(t *testing.T) {
	h := newHarness(t)
	rs := h.rentals()
	id := rs[0].Id.String()
	h.reply("DELETE /v1/rentals/"+id, 200, rentalFixture(1, "quiet-river-7k2f", api.RentalStateDeleting))
	// non-TTY, no --yes: refuse
	_, errs, code := h.run("rental", "delete", "quiet-river-7k2f")
	if code != 2 || h.count("DELETE", "/v1/rentals/"+id) != 0 {
		t.Fatalf("exit %d", code)
	}
	contains(t, "stderr", errs, "--yes")
	// TTY, wrong name: abort
	h.tty, h.stdin = true, "wrong-name\n"
	_, errs, code = h.run("rental", "delete", "quiet-river-7k2f")
	if code == 0 || h.count("DELETE", "/v1/rentals/"+id) != 0 {
		t.Fatalf("wrong name accepted: %d", code)
	}
	contains(t, "prompt", errs, "destroys the workspace", "Type the rental name")
	// TTY, right name: delete
	h.stdin = "quiet-river-7k2f\n"
	h.mustRun("rental", "delete", "quiet-river-7k2f")
	if h.count("DELETE", "/v1/rentals/"+id) != 1 {
		t.Fatal("not deleted")
	}
	// --yes skips the prompt
	h.tty, h.stdin = false, ""
	h.mustRun("rental", "delete", "quiet-river-7k2f", "--yes")
	if h.count("DELETE", "/v1/rentals/"+id) != 2 {
		t.Fatal("--yes did not delete")
	}
}

func TestRentalWait(t *testing.T) {
	h := newHarness(t)
	rs := h.rentals()
	id := rs[0].Id.String()
	n := 0
	h.on("GET /v1/rentals/"+id, func(w http.ResponseWriter, r *http.Request) {
		n++
		st := api.RentalStateStarting
		if n >= 3 {
			st = api.RentalStateRunning
		}
		writeJSON(w, 200, rentalFixture(1, "quiet-river-7k2f", st))
	})
	h.mustRun("rental", "wait", "quiet-river-7k2f")
	if n != 3 {
		t.Fatalf("polls %d", n)
	}
	// timeout
	h.reply("GET /v1/rentals/"+id, 200, rentalFixture(1, "quiet-river-7k2f", api.RentalStateStarting))
	_, errs, code := h.run("rental", "wait", "quiet-river-7k2f", "--timeout", "30ms")
	if code != 1 {
		t.Fatalf("exit %d", code)
	}
	contains(t, "timeout", errs, "timed out", "starting")
	// --for stopped
	h.reply("GET /v1/rentals/"+id, 200, rentalFixture(1, "quiet-river-7k2f", api.RentalStateStopped))
	h.mustRun("rental", "wait", "quiet-river-7k2f", "--for", "stopped")
	_, _, code = h.run("rental", "wait", "quiet-river-7k2f", "--for", "flying")
	if code != 2 {
		t.Fatal(code)
	}
}

func TestRentalSSHPrint(t *testing.T) {
	h := newHarness(t)
	h.rentals()
	out, _ := h.mustRun("rental", "ssh", "quiet-river-7k2f", "--print")
	want := "ssh -J legion@ssh.legion.example:2222 -o HostKeyAlias=quiet-river-7k2f root@quiet-river-7k2f\n"
	if out != want {
		t.Fatalf("got %q want %q", out, want)
	}
	if len(h.execs) != 0 {
		t.Fatal("exec'd with --print")
	}
	out, _ = h.mustRun("rental", "ssh", "quiet-river-7k2f", "--print", "--identity", "~/.ssh/id_ed25519", "--", "nvidia-smi")
	contains(t, "identity form", out, "-i ~/.ssh/id_ed25519", "IdentitiesOnly=yes", "ProxyCommand=", "-W %h:%p -p 2222 legion@ssh.legion.example", "root@quiet-river-7k2f", "nvidia-smi")
}

func TestRentalSSHExecs(t *testing.T) {
	h := newHarness(t)
	h.rentals()
	h.mustRun("ssh", "quiet-river-7k2f", "--", "nvidia-smi", "-L")
	if len(h.execs) != 1 {
		t.Fatalf("execs %v", h.execs)
	}
	got := strings.Join(h.execs[0], " ")
	if got != "/usr/bin/ssh ssh -J legion@ssh.legion.example:2222 -o HostKeyAlias=quiet-river-7k2f root@quiet-river-7k2f nvidia-smi -L" {
		t.Fatalf("%q", got)
	}
}

func TestRentalSSHWaitsForRunning(t *testing.T) {
	h := newHarness(t)
	rs := h.rentals()
	id := rs[0].Id.String()
	h.reply("GET /v1/rentals", 200, api.RentalPage{Items: []api.Rental{rentalFixture(1, "quiet-river-7k2f", api.RentalStateStarting)}})
	n := 0
	h.on("GET /v1/rentals/"+id, func(w http.ResponseWriter, r *http.Request) {
		n++
		st := api.RentalStateStarting
		if n >= 3 {
			st = api.RentalStateRunning
		}
		writeJSON(w, 200, rentalFixture(1, "quiet-river-7k2f", st))
	})
	h.mustRun("ssh", "quiet-river-7k2f")
	if n < 3 || len(h.execs) != 1 {
		t.Fatalf("polls %d execs %d", n, len(h.execs))
	}
}

func TestRentalSSHStoppedHint(t *testing.T) {
	h := newHarness(t)
	h.rentals()
	_, errs, code := h.run("ssh", "quiet-meadow-3x9a")
	if code != 1 {
		t.Fatalf("exit %d", code)
	}
	contains(t, "stderr", errs, "stopped", "Hint: legion rental start quiet-meadow-3x9a")
}

func TestRentalSSHConfig(t *testing.T) {
	h := newHarness(t)
	h.rentals()
	out, _ := h.mustRun("rental", "ssh-config", "quiet-river-7k2f", "--identity", "~/.ssh/id_ed25519")
	want := `Host quiet-river-7k2f-gateway
  HostName ssh.legion.example
  Port 2222
  User legion
  IdentityFile ~/.ssh/id_ed25519
  IdentitiesOnly yes

Host quiet-river-7k2f
  HostName quiet-river-7k2f
  User root
  HostKeyAlias quiet-river-7k2f
  ProxyJump quiet-river-7k2f-gateway
  IdentityFile ~/.ssh/id_ed25519
  IdentitiesOnly yes
  ServerAliveInterval 60
`
	if out != want {
		t.Fatalf("got:\n%s\nwant:\n%s", out, want)
	}
}

func TestRentalSSHRejectsUnsafeEndpoint(t *testing.T) {
	h := newHarness(t)
	r := rentalFixture(1, "quiet-river-7k2f", api.RentalStateRunning)
	r.Ssh.Host = "-oProxyCommand=evil"
	h.reply("GET /v1/rentals", 200, api.RentalPage{Items: []api.Rental{r}})
	h.reply("GET /v1/rentals/"+uidS(1), 200, r)
	_, _, code := h.run("rental", "ssh", "quiet-river-7k2f", "--print")
	if code == 0 {
		t.Fatal("unsafe host accepted")
	}
}

func TestRentalCharges(t *testing.T) {
	h := newHarness(t)
	rs := h.rentals()
	h.reply("GET /v1/rentals/"+rs[0].Id.String()+"/charges", 200, api.RentalCharges{
		RentalId: rs[0].Id, Seconds: 3725, Charged: api.SignedMoney{AmountMinor: 517, Currency: "USD"}, StorageCharged: api.SignedMoney{AmountMinor: 12, Currency: "USD"},
	})
	out, _ := h.mustRun("rental", "charges", "quiet-river-7k2f")
	contains(t, "charges", out, "$5.17", "$0.12", "1h2m")
}

func TestRentalIngress(t *testing.T) {
	h := newHarness(t)
	rs := h.rentals()
	id := rs[0].Id.String()
	rule := api.IngressRule{Id: uid(80), Protocol: "http", Port: 8000, Sources: []string{}, Hostname: "quiet-river-7k2f.ambi.run", Url: "https://quiet-river-7k2f.ambi.run", Status: "provisioning", CreatedAt: rs[0].CreatedAt}
	h.reply("GET /v1/rentals/"+id+"/network", 200, api.RentalNetwork{Ingress: api.RentalIngress{Available: true, Rules: []api.IngressRule{rule}}})
	h.reply("POST /v1/rentals/"+id+"/ingress-rules", 201, rule)
	h.reply("DELETE /v1/rentals/"+id+"/ingress-rules/"+uidS(80), 204, nil)
	out, _ := h.mustRun("rental", "ingress", "list", "quiet-river-7k2f")
	contains(t, "list", out, "8000", "https://quiet-river-7k2f.ambi.run")
	_, errs := h.mustRun("rental", "ingress", "add", "quiet-river-7k2f", "--port", "8000", "--source", "203.0.113.0/24", "--description", "api")
	c := h.last("POST", "/v1/rentals/"+id+"/ingress-rules")
	req := body[api.CreateIngressRuleRequest](t, c)
	if req.Port != 8000 || req.Protocol != "http" || len(req.Sources) != 1 || req.Sources[0] != "203.0.113.0/24" || c.Header.Get("Idempotency-Key") == "" {
		t.Fatalf("%+v", req)
	}
	contains(t, "add", errs, "https://quiet-river-7k2f.ambi.run")
	_, _, code := h.run("rental", "ingress", "add", "quiet-river-7k2f")
	if code != 2 {
		t.Fatalf("missing --port: %d", code)
	}
	h.mustRun("rental", "ingress", "remove", "quiet-river-7k2f", uidS(80), "--yes")
	if h.count("DELETE", "/v1/rentals/"+id+"/ingress-rules/"+uidS(80)) != 1 {
		t.Fatal("not removed")
	}
	_, _, code = h.run("rental", "ingress", "remove", "quiet-river-7k2f", uidS(80))
	if code != 2 {
		t.Fatalf("remove without --yes on non-tty: %d", code)
	}
}
