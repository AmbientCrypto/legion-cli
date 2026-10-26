package cli

import (
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/AmbientCrypto/legion-controller/api"

	"github.com/AmbientCrypto/legion-cli/internal/client"
)

type clientErr = client.Error

func TestBadOutputRejectedBeforeAnyCall(t *testing.T) {
	h := newHarness(t)
	h.stdFixtures()
	h.rentalLifecycle("quiet-river-7k2f", 9)
	_, e, c := h.run(createArgs("--yes", "-o", "bogus")...)
	if c != 2 || len(h.calls) != 0 {
		t.Fatalf("exit %d, calls %d", c, len(h.calls))
	}
	contains(t, "hint", e, "table", "jsonpath")
	_, _, c = h.run(createArgs("--yes", "-o", "jsonpath=$[[")...)
	if c != 2 || len(h.calls) != 0 {
		t.Fatalf("bad jsonpath: exit %d, calls %d", c, len(h.calls))
	}
	h.noKey = true
	if _, _, c = h.run("rental", "list", "-o", "bogus"); c != 2 {
		t.Fatalf("logged out + bad format = %d, want 2", c)
	}
}

func TestAPIURLMustBeHTTPSOrLocal(t *testing.T) {
	h := newHarness(t)
	for _, args := range [][]string{{"--api-url", "http://api.example.com", "whoami"}, {"config", "set", "api_url", "http://api.example.com"}} {
		if _, _, c := h.run(args...); c != 2 {
			t.Fatalf("%v = %d", args, c)
		}
	}
	h.env["LEGION_API_URL"] = "http://api.example.com"
	if _, e, c := h.run("whoami"); c != 2 || len(h.calls) != 0 {
		t.Fatalf("env url: %d %s", c, e)
	}
	h.env["LEGION_API_URL"] = h.srv.URL
	h.noKey = true
	h.reply("GET /v1/me", 200, meFixture())
	h.stdin = "lgn_k\n"
	_, e := h.mustRun("auth", "login", "--with-token")
	contains(t, "login", e, "Sending your key to 127.0.0.1")
}

func TestNoConfigDirFallbackToCwd(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	h := newHarness(t)
	h.env["LEGION_CONFIG_DIR"] = ""
	h.env["XDG_CONFIG_HOME"] = ""
	_, e, c := h.run("config", "set", "output", "json")
	if c != 1 {
		t.Fatalf("exit %d: %s", c, e)
	}
	if _, err := os.Stat("config.yaml"); err == nil {
		os.Remove("config.yaml")
		t.Fatal("wrote config.yaml into the working directory")
	}
}

func TestBackupDeleteAfterWarns(t *testing.T) {
	h := newHarness(t)
	rs := h.rentals()
	id := rs[0].Id.String()
	h.reply("GET /v1/rentals/"+id+"/backup-estimate", 200, api.BackupEstimate{EstimatedSizeBytes: 1 << 30, EstimateBasis: "x", Price: money(10), Storage: money(5), Download: money(5), RetentionDays: 7, FitsUploadWindow: true})
	_, e, c := h.run("rental", "backup", "create", "quiet-river-7k2f", "--delete-rental-after")
	if c != 2 {
		t.Fatal(c)
	}
	contains(t, "refusal", e, "deleted after the backup")
	h.tty, h.stdin = true, "n\n"
	_, e, _ = h.run("rental", "backup", "create", "quiet-river-7k2f", "--delete-rental-after")
	contains(t, "prompt", e, "deleted after the backup")
}

type blockingReader struct{ done chan struct{} }

func (b blockingReader) Read([]byte) (int, error) { <-b.done; return 0, io.EOF }

func TestPromptReturnsOnInterrupt(t *testing.T) {
	h := newHarness(t)
	h.rentals()
	h.tty = true
	br := blockingReader{make(chan struct{})}
	defer close(br.done)
	h.stdinR = br
	ctx, cancel := context.WithCancel(context.Background())
	h.ctx = ctx
	go func() { time.Sleep(50 * time.Millisecond); cancel() }()
	done := make(chan int)
	go func() { _, _, c := h.run("rental", "delete", "quiet-river-7k2f"); done <- c }()
	select {
	case c := <-done:
		if c != 130 {
			t.Fatalf("exit %d", c)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("prompt did not return on interrupt")
	}
}

func TestSelectDefaultsAndGivesUp(t *testing.T) {
	h := newHarness(t)
	h.rentals()
	h.reply("GET /v1/rentals", 200, api.RentalPage{Items: []api.Rental{rentalFixture(1, "quiet-river-7k2f", api.RentalStateRunning)}})
	h.tty, h.stdin = true, "\n"
	out, _ := h.mustRun("rental", "get", "-o", "name")
	if strings.TrimSpace(out) != "quiet-river-7k2f" {
		t.Fatalf("Enter did not pick the only choice: %q", out)
	}
	h.reply("GET /v1/rentals", 200, api.RentalPage{Items: []api.Rental{rentalFixture(1, "quiet-river-7k2f", api.RentalStateRunning), rentalFixture(3, "brave-otter-0aa1", api.RentalStateRunning)}})
	h.stdin = "x\ny\nz\n1\n"
	if _, e, c := h.run("rental", "get"); c != 2 {
		t.Fatalf("exit %d %s", c, e)
	}
}

func TestMutationsSurviveEmptyBodies(t *testing.T) {
	h := newHarness(t)
	rs := h.rentals()
	id := rs[0].Id.String()
	h.reply("DELETE /v1/rentals/"+id, 204, nil)
	out, e := h.mustRun("rental", "delete", "quiet-river-7k2f", "--yes")
	excludes(t, "delete stdout", out, "State:", "running")
	contains(t, "delete", e, "Deleting quiet-river-7k2f", "legion rental list")
	h.reply("POST /v1/rentals/"+id+"/stop", 204, nil)
	h.mustRun("rental", "stop", "quiet-river-7k2f", "--yes", "--async")
	h.reply("POST /v1/rentals", 201, nil)
	h.stdFixtures()
	if _, _, c := h.run(createArgs("--yes")...); c == 0 {
		t.Fatal("create with empty answer reported success")
	}
}

func TestStopDeleteOneLine(t *testing.T) {
	h := newHarness(t)
	rs := h.rentals()
	id := rs[0].Id.String()
	h.reply("POST /v1/rentals/"+id+"/stop", 200, rentalFixture(1, "quiet-river-7k2f", api.RentalStateStopping))
	h.reply("DELETE /v1/rentals/"+id, 200, rentalFixture(1, "quiet-river-7k2f", api.RentalStateDeleting))
	out, e := h.mustRun("rental", "stop", "quiet-river-7k2f", "--yes", "--async")
	if out != "" || strings.Count(e, "legion rental start") != 1 || strings.Count(e, "legion rental delete") != 1 {
		t.Fatalf("stdout %q stderr %q", out, e)
	}
	out, _ = h.mustRun("rental", "delete", "quiet-river-7k2f", "--yes")
	if out != "" {
		t.Fatalf("detail block printed: %q", out)
	}
}

func TestCreateUsesInterruptibleGPUs(t *testing.T) {
	h := newHarness(t)
	h.stdFixtures()
	h.rentalLifecycle("quiet-river-7k2f", 9)
	o := offerFixture()
	o.GpusFree, o.GpusInterruptible = 1, ptr(3)
	h.reply("GET /v1/offers/"+uidS(50), 200, api.OfferDetail{Offer: o, UptimeDaily: []api.DayUptime{}})
	h.mustRun(createArgs("--yes", "--gpus", "4", "--async")...)
	if _, _, c := h.run(createArgs("--yes", "--gpus", "5", "--async")...); c != 2 {
		t.Fatal(c)
	}
}

func TestMatchRentalSkipsDeletedAndUsesUUID(t *testing.T) {
	h := newHarness(t)
	del := rentalFixture(1, "quiet-old-aaaa", api.RentalStateDeleted)
	live := rentalFixture(2, "quiet-new-bbbb", api.RentalStateRunning)
	h.reply("GET /v1/rentals", 200, api.RentalPage{Items: []api.Rental{del, live}})
	h.reply("GET /v1/rentals/"+uidS(2), 200, live)
	h.reply("GET /v1/rentals/"+uidS(1), 200, del)
	out, _ := h.mustRun("rental", "get", "quiet", "-o", "name")
	if strings.TrimSpace(out) != "quiet-new-bbbb" {
		t.Fatalf("%q", out)
	}
	h.mustRun("rental", "get", "quiet-old-aaaa", "-o", "name")
	before := h.count("GET", "/v1/rentals")
	h.mustRun("rental", "get", uidS(2), "-o", "name")
	if h.count("GET", "/v1/rentals") != before {
		t.Fatal("listed rentals for a UUID")
	}
}

func TestRentalListHidesDeleted(t *testing.T) {
	h := newHarness(t)
	h.reply("GET /v1/rentals", 200, api.RentalPage{Items: []api.Rental{rentalFixture(1, "quiet-old-aaaa", api.RentalStateDeleted), rentalFixture(2, "quiet-new-bbbb", api.RentalStateRunning)}})
	out, _ := h.mustRun("rental", "list", "-o", "name")
	if strings.TrimSpace(out) != "quiet-new-bbbb" {
		t.Fatalf("%q", out)
	}
	out, _ = h.mustRun("rental", "list", "--all", "-o", "name")
	contains(t, "all", out, "quiet-old-aaaa")
}

func TestRevokePagesThroughKeys(t *testing.T) {
	h := newHarness(t)
	k1 := api.ApiKey{Id: uid(90), Name: "a", Prefix: "lgn_aaaa", Scope: "full", CreatedAt: time.Now()}
	k2 := api.ApiKey{Id: uid(91), Name: "ci", Prefix: "lgn_bbbb", Scope: "full", CreatedAt: time.Now()}
	h.on("GET /v1/api-keys", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("cursor") == "" {
			writeJSON(w, 200, api.ApiKeyPage{Items: []api.ApiKey{k1}, NextCursor: ptr("p2")})
			return
		}
		writeJSON(w, 200, api.ApiKeyPage{Items: []api.ApiKey{k2}})
	})
	h.reply("DELETE /v1/api-keys/"+uidS(91), 204, nil)
	h.mustRun("api-key", "revoke", "ci", "--yes")
	if h.count("DELETE", "/v1/api-keys/"+uidS(91)) != 1 {
		t.Fatal("key on page 2 not found")
	}
}

func TestStillBillingNotice(t *testing.T) {
	h := newHarness(t)
	h.stdFixtures()
	h.reply("POST /v1/rentals", 201, rentalFixture(9, "quiet-river-7k2f", api.RentalStateStarting))
	h.reply("GET /v1/rentals/"+uidS(9), 200, rentalFixture(9, "quiet-river-7k2f", api.RentalStateStarting))
	_, e, c := h.run(createArgs("--yes", "--wait-timeout", "30ms")...)
	if c != 1 {
		t.Fatal(c)
	}
	contains(t, "timeout", e, "The rental is still billing. Stop it: legion rental stop quiet-river-7k2f")
	ctx, cancel := context.WithCancel(context.Background())
	h.ctx = ctx
	h.on("GET /v1/rentals/"+uidS(9), func(w http.ResponseWriter, r *http.Request) {
		cancel()
		writeJSON(w, 200, rentalFixture(9, "quiet-river-7k2f", api.RentalStateStarting))
	})
	_, e, c = h.run(createArgs("--yes")...)
	if c != 130 {
		t.Fatal(c)
	}
	contains(t, "interrupt", e, "still billing")
}

func TestStartAndIngressConfirm(t *testing.T) {
	h := newHarness(t)
	rs := h.rentals()
	id := rs[1].Id.String()
	h.reply("POST /v1/rentals/"+id+"/start", 200, rentalFixture(2, "quiet-meadow-3x9a", api.RentalStateStarting))
	_, e, c := h.run("rental", "start", "quiet-meadow-3x9a", "--async")
	if c != 2 || h.count("POST", "/v1/rentals/"+id+"/start") != 0 {
		t.Fatalf("start without --yes: %d", c)
	}
	contains(t, "start refusal", e, "$5.00 per hour")
	h.tty, h.stdin = true, "y\n"
	_, e = h.mustRun("rental", "start", "quiet-meadow-3x9a", "--async")
	contains(t, "start prompt", e, "$5.00 per hour")
	h.tty = false
	rid := rs[0].Id.String()
	h.reply("POST /v1/rentals/"+rid+"/ingress-rules", 201, api.IngressRule{Id: uid(80), Port: 8000, Url: "https://x.example", Status: "provisioning", Sources: []string{}})
	_, e, c = h.run("rental", "ingress", "add", "quiet-river-7k2f", "--port", "8000")
	if c != 2 || h.count("POST", "/v1/rentals/"+rid+"/ingress-rules") != 0 {
		t.Fatalf("ingress add without --yes: %d", c)
	}
	contains(t, "ingress", e, "internet")
}

func TestEscapeSequencesAreStripped(t *testing.T) {
	h := newHarness(t)
	r := rentalFixture(1, "quiet-river-7k2f", api.RentalStateFailed)
	r.Image = "evil\x1b]0;pwn\x07img"
	r.FailureReason = ptr("boom\x1b[2Jreason\u009b")
	h.reply("GET /v1/rentals", 200, api.RentalPage{Items: []api.Rental{r}})
	h.reply("GET /v1/rentals/"+uidS(1), 200, r)
	o := offerFixture()
	o.GpuModel = "RTX\x1b[31m4090"
	h.reply("GET /v1/offers", 200, api.OfferPage{Items: []api.Offer{o}})
	h.reply("GET /v1/offers/"+uidS(50), 200, api.OfferDetail{Offer: o})
	h.fail("GET /v1/billing/account", 500, "boom", "bad\x1b[2Jnews")
	for _, args := range [][]string{{"rental", "list"}, {"rental", "get", "quiet"}, {"offer", "list"}, {"offer", "get", "amber-falcon"}, {"billing", "balance"}} {
		out, e, _ := h.run(args...)
		excludes(t, strings.Join(args, " "), out+e, "\x1b", "\u009b", "\x07")
	}
	_, _, _ = h.run("rental", "list", "-o", "json")
}

func TestSSHFingerprintAndPercent(t *testing.T) {
	h := newHarness(t)
	h.rentals()
	_, e := h.mustRun("ssh", "quiet-river-7k2f")
	contains(t, "fingerprint", e, "Host key should be SHA256:abc", "ssh will ask you to confirm it on first connect")
	_, e = h.mustRun("ssh", "quiet-river-7k2f", "-q")
	excludes(t, "-q", e, "Host key")
	out, _ := h.mustRun("rental", "ssh-config", "quiet-river-7k2f")
	contains(t, "config", out, "# Gateway host key: SHA256:abc")
	if _, _, c := h.run("ssh", "quiet-river-7k2f", "--print", "-i", "~/a%h"); c != 2 {
		t.Fatalf("%% in identity accepted: %d", c)
	}
}

func TestInstallScriptHardening(t *testing.T) {
	b, err := os.ReadFile("../../scripts/install.sh")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, l := range strings.Split(s, "\n") {
		if strings.Contains(l, "curl -") && !strings.HasPrefix(strings.TrimSpace(l), "#") && !strings.Contains(l, "--proto '=https' --tlsv1.2") {
			t.Errorf("curl without https-only: %s", l)
		}
	}
	contains(t, "version check", s, "^v?[0-9A-Za-z.+-]+$")
}

func TestForbiddenExitCodes(t *testing.T) {
	for code, want := range map[string]int{"forbidden": 3, "api_key_scope": 3, "api_key_required": 3, "org_suspended": 1, "custom_images_disabled": 1, "catalog_entry_disabled": 1} {
		d := Describe(&clientErr{Status: 403, Code: code}, "rental create")
		if d.Exit != want {
			t.Errorf("%s: exit %d want %d", code, d.Exit, want)
		}
	}
	if d := Describe(&clientErr{Status: 422, Code: "invalid_request"}, "rental create"); d.Exit != 2 {
		t.Errorf("422 = %d", d.Exit)
	}
	for _, code := range []string{"custom_images_disabled", "catalog_entry_disabled"} {
		if Describe(&clientErr{Status: 403, Code: code}, "rental create").Hint == "" {
			t.Errorf("%s has no hint", code)
		}
	}
}

func TestUnauthorizedMessage(t *testing.T) {
	h := newHarness(t)
	h.fail("GET /v1/rentals", 401, "unauthorized", "security requirements failed: missing or invalid credentials")
	_, e, c := h.run("rental", "list")
	if c != 3 {
		t.Fatal(c)
	}
	contains(t, "401", e, "Your API key is invalid, expired or revoked.", "Hint: legion auth login")
	excludes(t, "401", e, "security requirements")
}

func TestBackupListStateFilter(t *testing.T) {
	h := newHarness(t)
	rs := h.rentals()
	b1 := api.Backup{Id: uid(95), RentalId: rs[0].Id, State: "ready"}
	b2 := api.Backup{Id: uid(96), RentalId: rs[0].Id, State: "failed"}
	h.reply("GET /v1/rentals/"+rs[0].Id.String()+"/backups", 200, api.BackupPage{Items: []api.Backup{b1, b2}})
	out, _ := h.mustRun("rental", "backup", "list", "quiet-river-7k2f", "--state", "ready", "-o", "name")
	if strings.TrimSpace(out) != uidS(95) {
		t.Fatalf("%q", out)
	}
}

func TestOfferAmbiguousPrefix(t *testing.T) {
	h := newHarness(t)
	a, b := offerFixture(), offerFixture()
	b.HostId, b.HostName = uid(51), "amber-fox"
	h.reply("GET /v1/offers", 200, api.OfferPage{Items: []api.Offer{a, b}})
	_, e, c := h.run("offer", "get", "amber")
	if c != 2 {
		t.Fatalf("exit %d", c)
	}
	contains(t, "ambiguous", e, "amber-falcon", "amber-fox")
}

func TestMaxPriceBelowCurrent(t *testing.T) {
	h := newHarness(t)
	h.stdFixtures()
	for _, extra := range []string{"--yes", "--dry-run"} {
		_, e, c := h.run(createArgs(extra, "--max-price", "1.00")...)
		if c != 2 || h.posts() != 0 {
			t.Fatalf("%s: exit %d", extra, c)
		}
		contains(t, "max price", e, "$2.50", "--max-price")
		excludes(t, "no preview", e, "Rental preview")
	}
}

func TestGPUModelSubstringFilter(t *testing.T) {
	h := newHarness(t)
	a, b := offerFixture(), offerFixture()
	b.HostName, b.GpuModel = "other", "NVIDIA H100"
	a.GpuModel = "NVIDIA GeForce RTX 4090"
	h.reply("GET /v1/offers", 200, api.OfferPage{Items: []api.Offer{a, b}})
	out, _ := h.mustRun("offer", "list", "--gpu-model", "rtx 4090", "-o", "name")
	if strings.TrimSpace(out) != "amber-falcon" {
		t.Fatalf("%q", out)
	}
	if len(h.last("GET", "/v1/offers").Query["gpu_model"]) != 0 {
		t.Fatal("exact API filter sent")
	}
	_, e := h.mustRun("offer", "list", "--gpu-model", "nothing", "--min-gpus", "2")
	contains(t, "empty", e, "gpu-model", "nothing", "min-gpus", "legion offer list")
	h.mustRun("offer", "list", "--catalog", "qwen")
	if h.last("GET", "/v1/offers").Query["catalog_id"][0] != "qwen" {
		t.Fatal("catalog_id not sent")
	}
}

func TestJSONPathNoMatch(t *testing.T) {
	h := newHarness(t)
	h.reply("GET /v1/rentals", 200, api.RentalPage{Items: []api.Rental{rentalFixture(1, "quiet-river-7k2f", api.RentalStateRunning)}})
	_, e, c := h.run("rental", "list", "-o", "jsonpath=$.name")
	if c != 1 {
		t.Fatal(c)
	}
	contains(t, "nomatch", e, "jsonpath matched nothing (list output is an array: try '$[*].name')")
}

func TestDryRunAndPreviewText(t *testing.T) {
	h := newHarness(t)
	h.stdFixtures()
	h.reply("GET /v1/offers", 200, api.OfferPage{Items: []api.Offer{offerFixture()}})
	out, e := h.mustRun("rental", "create", "--offer", "amber-falcon", "--image", "pytorch/pytorch:2", "--dry-run")
	contains(t, "dry", out, "GPUs:      1 (change with --gpus N)")
	contains(t, "dry", e, "Dry run: nothing rented. To rent: legion rental create --offer amber-falcon --image pytorch/pytorch:2 --gpus 1 --yes")
	_, e, c := h.run("rental", "create", "--offer", "amber-falcon", "--image", "pytorch/pytorch:2")
	if c != 2 {
		t.Fatal(c)
	}
	if i, j := strings.Index(e, "Error:"), strings.Index(e, "Preview (nothing rented):"); i < 0 || j < i {
		t.Fatalf("error must come first, then the preview:\n%s", e)
	}
}

func TestLoginPromptWording(t *testing.T) {
	h := newHarness(t)
	h.noKey = true
	h.reply("GET /v1/me", 200, meFixture())
	h.tty, h.secret = true, "lgn_x"
	_, e := h.mustRun("auth", "login", "--paste", "--no-browser")
	contains(t, "prompt", e, "hidden", "press Enter")
}

func TestLedgerNewestFirstAndScope(t *testing.T) {
	h := newHarness(t)
	t0 := time.Now().Add(-time.Hour)
	h.reply("GET /v1/billing/ledger", 200, api.LedgerEntryPage{Items: []api.LedgerEntry{
		{Id: "old", Cause: "top_up", Amount: api.SignedMoney{AmountMinor: 100, Currency: "USD"}, OccurredAt: t0},
		{Id: "new", Cause: "usage", Amount: api.SignedMoney{AmountMinor: -5, Currency: "USD"}, OccurredAt: t0.Add(time.Minute)}}})
	out, _ := h.mustRun("billing", "ledger", "-o", "wide")
	if strings.Index(out, "new") > strings.Index(out, "old") {
		t.Fatalf("not newest first:\n%s", out)
	}
	me := meFixture()
	me.Credential.Scope = ""
	me.Credential.ApiKeyId = ptr(uid(90))
	h.reply("GET /v1/me", 200, me)
	h.reply("GET /v1/api-keys", 200, api.ApiKeyPage{Items: []api.ApiKey{{Id: uid(90), Name: "ci", Prefix: "lgn_x", Scope: "full", CreatedAt: t0}}})
	out, _ = h.mustRun("auth", "status")
	contains(t, "scope", out, "Scope:    full")
}

func TestNoSSHLinesUnlessRunning(t *testing.T) {
	h := newHarness(t)
	r := rentalFixture(1, "quiet-river-7k2f", api.RentalStateDeleting)
	r.Ssh = &api.SSHEndpoint{Host: "ssh.legion.example", Port: 2222}
	h.reply("GET /v1/rentals", 200, api.RentalPage{Items: []api.Rental{r}})
	h.reply("GET /v1/rentals/"+uidS(1), 200, r)
	out, _ := h.mustRun("rental", "get", "quiet-river-7k2f")
	excludes(t, "get", out, "SSH:", "Connect:")
}

func TestStartQuiet(t *testing.T) {
	h := newHarness(t)
	rs := h.rentals()
	id := rs[1].Id.String()
	h.reply("POST /v1/rentals/"+id+"/start", 200, rentalFixture(2, "quiet-meadow-3x9a", api.RentalStateStarting))
	h.reply("GET /v1/rentals/"+id, 200, rentalFixture(2, "quiet-meadow-3x9a", api.RentalStateRunning))
	out, e := h.mustRun("rental", "start", "quiet-meadow-3x9a", "--yes", "-q")
	if strings.TrimSpace(out) != "quiet-meadow-3x9a" || e != "" {
		t.Fatalf("stdout %q stderr %q", out, e)
	}
}
