package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AmbientCrypto/legion-controller/api"
	"github.com/spf13/cobra"
)

func TestOfferListFilters(t *testing.T) {
	h := newHarness(t)
	h.reply("GET /v1/offers", 200, api.OfferPage{Items: []api.Offer{offerFixture()}})
	out, _ := h.mustRun("offer", "list", "--gpu-model", "RTX 4090", "--min-gpus", "2", "--max-price", "3.25", "--region", "us-east-1", "--max-latency", "80", "--catalog", "qwen", "--sort", "reliability", "--country", "US")
	contains(t, "table", out, "HOST", "amber-falcon", "RTX 4090", "4/4", "$2.50")
	q := h.last("GET", "/v1/offers").Query
	want := map[string]string{"gpu_model": "RTX 4090", "min_gpus": "2", "max_price_per_gpu_hour_cents": "325", "latency_region": "us-east-1", "max_latency_ms": "80", "catalog_id": "qwen", "sort": "reliability", "countries": "US"}
	for k, v := range want {
		if len(q[k]) != 1 || q[k][0] != v {
			t.Errorf("param %s = %v, want %s", k, q[k], v)
		}
	}
	out, _ = h.mustRun("offer", "list", "-o", "name")
	if out != "amber-falcon\n" {
		t.Fatalf("%q", out)
	}
	out, _ = h.mustRun("offer", "list", "-o", "wide")
	contains(t, "wide", out, uidS(50))
	_, _, code := h.run("offer", "list", "--max-price", "cheap")
	if code != 2 {
		t.Fatal(code)
	}
	h.mustRun("offer", "list", "--gpu-model", "A", "--gpu-model", "B")
	if got := h.last("GET", "/v1/offers").Query["gpu_models"]; len(got) != 2 {
		t.Fatalf("gpu_models %v", got)
	}
}

func TestOfferGetByNameAndID(t *testing.T) {
	h := newHarness(t)
	h.stdFixtures()
	h.reply("GET /v1/offers", 200, api.OfferPage{Items: []api.Offer{offerFixture()}})
	out, _ := h.mustRun("offer", "get", uidS(50))
	contains(t, "get", out, "amber-falcon", "4 x RTX 4090", "$2.50", "legion rental create --offer amber-falcon")
	out, _ = h.mustRun("offer", "get", "amber-falcon", "-o", "name")
	if strings.TrimSpace(out) != "amber-falcon" {
		t.Fatal(out)
	}
	_, _, code := h.run("offer", "get", "ghost")
	if code != 4 {
		t.Fatal(code)
	}
}

func TestCatalog(t *testing.T) {
	h := newHarness(t)
	h.reply("GET /v1/catalog", 200, api.Catalog{Entries: []api.CatalogEntry{
		{Id: "plain", Name: "Plain", Kind: "image", Reference: "r/plain", Default: true},
		{Id: "qwen", Name: "Qwen", Kind: "model", Reference: "r/qwen", Serve: &api.CatalogServe{Port: 8000, Protocol: "openai", Model: "Qwen/Qwen3.5-4B", MinVramGb: 16, License: "apache-2.0"}},
	}})
	out, _ := h.mustRun("catalog", "list")
	contains(t, "all", out, "plain", "qwen", "16 GB")
	out, _ = h.mustRun("catalog", "list", "--kind", "model", "-o", "name")
	if out != "qwen\n" {
		t.Fatalf("%q", out)
	}
	_, _, code := h.run("catalog", "list", "--kind", "bogus")
	if code != 2 {
		t.Fatal(code)
	}
	out, _ = h.mustRun("catalog", "get", "qwen")
	contains(t, "get", out, "Qwen/Qwen3.5-4B", "8000", "legion rental create --offer <host> --catalog qwen")
	_, errs, code := h.run("catalog", "get", "nope")
	if code != 4 {
		t.Fatal(code)
	}
	contains(t, "hint", errs, "Hint: legion catalog list")
}

func TestBilling(t *testing.T) {
	h := newHarness(t)
	h.reply("GET /v1/billing/account", 200, api.BillingAccount{Balance: api.SignedMoney{AmountMinor: -125, Currency: "USD"}, HasCredit: false})
	out, errs := h.mustRun("billing", "balance")
	contains(t, "balance", out, "-$1.25", "none")
	contains(t, "hint", errs, "add credit")
	now := time.Now()
	h.reply("GET /v1/billing/ledger", 200, api.LedgerEntryPage{Items: []api.LedgerEntry{{Id: "l1", Account: "balance", Cause: "top_up", Amount: api.SignedMoney{AmountMinor: 5000, Currency: "USD"}, OccurredAt: now, RecordedAt: now}}})
	out, _ = h.mustRun("billing", "ledger")
	contains(t, "ledger", out, "top_up", "$50.00")
	h.reply("GET /v1/billing/activity", 200, api.BillingActivityPage{Items: []api.BillingActivityItem{{
		Id: "a1", At: now, Kind: "rental_session", Amount: api.SignedMoney{AmountMinor: -517, Currency: "USD"},
		Session: &api.RentalSession{RentalName: ptr("quiet-river-7k2f"), GpuCount: 2, GpuModel: "RTX 4090", Seconds: 3725},
	}}})
	out, _ = h.mustRun("billing", "activity")
	contains(t, "activity", out, "quiet-river-7k2f", "2x RTX 4090", "1h2m", "-$5.17")
	if got := h.last("GET", "/v1/billing/activity").Query["account"]; len(got) != 1 || got[0] != "balance" {
		t.Fatalf("account param %v", got)
	}
	h.reply("GET /v1/billing/charges", 200, api.RentalChargesPage{Items: []api.RentalCharges{{RentalId: uid(1), Seconds: 60, Charged: api.SignedMoney{AmountMinor: 8, Currency: "USD"}, StorageCharged: api.SignedMoney{Currency: "USD"}}}})
	h.reply("GET /v1/rentals", 200, api.RentalPage{Items: []api.Rental{rentalFixture(1, "quiet-river-7k2f", api.RentalStateRunning)}})
	out, _ = h.mustRun("billing", "charges")
	contains(t, "charges", out, "quiet-river-7k2f", "$0.08")
}

func TestSSHKeyCommands(t *testing.T) {
	h := newHarness(t)
	h.reply("GET /v1/ssh-keys", 200, api.SshKeyList{Items: []api.SshKey{sshKeyFixture()}})
	out, _ := h.mustRun("ssh-key", "list")
	contains(t, "list", out, "laptop", "SHA256:fp")
	h.writeKey("id_ed25519.pub", "ssh-ed25519 AAAAC3Nza me@box\n")
	k := sshKeyFixture()
	k.Label = "me@box"
	h.reply("POST /v1/ssh-keys", 201, k)
	h.mustRun("ssh-key", "add")
	req := body[api.AddSshKeyRequest](t, h.last("POST", "/v1/ssh-keys"))
	if req.Label != "me@box" || req.PublicKey != "ssh-ed25519 AAAAC3Nza me@box" {
		t.Fatalf("%+v", req)
	}
	h.writeKey("work.pub", "ssh-ed25519 AAAAWORK\n")
	h.mustRun("ssh-key", "add", "--file", filepath.Join(h.home, ".ssh", "work.pub"))
	if req = body[api.AddSshKeyRequest](t, h.last("POST", "/v1/ssh-keys")); req.Label != "work" {
		t.Fatalf("label from file name: %q", req.Label)
	}
	h.writeKey("id_priv", "-----BEGIN OPENSSH PRIVATE KEY-----\nabc\n")
	_, errs, code := h.run("ssh-key", "add", "--file", filepath.Join(h.home, ".ssh", "id_priv"))
	if code != 2 {
		t.Fatalf("private key accepted: %d", code)
	}
	contains(t, "stderr", errs, "private key")
	h.reply("DELETE /v1/ssh-keys/"+uidS(70), 204, nil)
	h.mustRun("ssh-key", "remove", "laptop")
	if h.count("DELETE", "/v1/ssh-keys/"+uidS(70)) != 1 {
		t.Fatal("not deleted")
	}
	_, _, code = h.run("ssh-key", "remove", "ghost")
	if code != 4 {
		t.Fatal(code)
	}
}

func TestAPIKeyCommands(t *testing.T) {
	h := newHarness(t)
	k := api.ApiKey{Id: uid(90), Name: "ci", Prefix: "lgn_3Fq9", Scope: "full", CreatedAt: time.Now()}
	h.reply("GET /v1/api-keys", 200, api.ApiKeyPage{Items: []api.ApiKey{k}})
	out, _ := h.mustRun("api-key", "list")
	contains(t, "list", out, "ci", "lgn_3Fq9", "full")
	h.reply("POST /v1/api-keys", 201, api.ApiKeyCreated{ApiKey: k, Token: "lgn_3Fq9_the_whole_secret"})
	out, errs := h.mustRun("api-key", "create", "ci", "--expires", "24h")
	contains(t, "secret shown once", out, "lgn_3Fq9_the_whole_secret")
	contains(t, "warning", errs, "only time")
	req := body[api.CreateApiKeyRequest](t, h.last("POST", "/v1/api-keys"))
	if req.Name != "ci" || req.Scope == nil || *req.Scope != "full" || req.ExpiresAt == nil {
		t.Fatalf("%+v", req)
	}
	out, _ = h.mustRun("api-key", "create", "ci", "-o", "json")
	var created api.ApiKeyCreated
	if err := json.Unmarshal([]byte(out), &created); err != nil || created.Token == "" {
		t.Fatalf("json stdout not clean: %q %v", out, err)
	}
	h.reply("DELETE /v1/api-keys/"+uidS(90), 204, nil)
	_, _, code := h.run("api-key", "revoke", "ci")
	if code != 2 {
		t.Fatalf("revoke without --yes on non-tty: %d", code)
	}
	h.mustRun("api-key", "revoke", "ci", "--yes")
	if h.count("DELETE", "/v1/api-keys/"+uidS(90)) != 1 {
		t.Fatal("not revoked")
	}
}

func TestBackupCommands(t *testing.T) {
	h := newHarness(t)
	rs := h.rentals()
	id := rs[0].Id.String()
	h.reply("GET /v1/rentals/"+id+"/backup-estimate", 200, api.BackupEstimate{EstimatedSizeBytes: 5 << 30, EstimateBasis: "disk_usage", Price: money(120), Storage: money(100), Download: money(20), RetentionDays: 7, FitsUploadWindow: true})
	out, _ := h.mustRun("rental", "backup", "estimate", "quiet-river-7k2f")
	contains(t, "estimate", out, "5.0 GiB", "$1.20", "7 days")
	b := api.Backup{Id: uid(95), RentalId: rs[0].Id, RentalName: ptr("quiet-river-7k2f"), State: "queued", EstimatedSizeBytes: 5 << 30, Price: api.BackupPrice{Quoted: money(120)}}
	h.reply("POST /v1/rentals/"+id+"/backups", 202, b)
	_, _, code := h.run("rental", "backup", "create", "quiet-river-7k2f")
	if code != 2 || h.count("POST", "/v1/rentals/"+id+"/backups") != 0 {
		t.Fatalf("charged backup without confirmation: %d", code)
	}
	h.mustRun("rental", "backup", "create", "quiet-river-7k2f", "--yes", "--retention-days", "30")
	c := h.last("POST", "/v1/rentals/"+id+"/backups")
	req := body[api.CreateBackupRequest](t, c)
	if req.RetentionDays == nil || *req.RetentionDays != 30 || c.Header.Get("Idempotency-Key") == "" {
		t.Fatalf("%+v", req)
	}
	h.reply("GET /v1/rentals/"+id+"/backups", 200, api.BackupPage{Items: []api.Backup{b}})
	out, _ = h.mustRun("rental", "backup", "list", "quiet-river-7k2f")
	contains(t, "list", out, uidS(95), "queued", "$1.20")
	h.reply("GET /v1/backups/"+uidS(95), 200, b)
	h.reply("DELETE /v1/backups/"+uidS(95), 204, nil)
	h.reply("POST /v1/backups/"+uidS(95)+"/cancel", 200, b)
	h.reply("POST /v1/backups/"+uidS(95)+"/download-links", 201, api.BackupDownloadLink{Url: "https://dl.example/x", LinkNumber: 1, Charge: money(0), ExpiresAt: time.Now().Add(time.Hour)})
	h.mustRun("rental", "backup", "get", uidS(95))
	h.mustRun("rental", "backup", "cancel", uidS(95))
	out, _ = h.mustRun("rental", "backup", "download-link", uidS(95))
	if strings.TrimSpace(out) != "https://dl.example/x" {
		t.Fatalf("%q", out)
	}
	_, _, code = h.run("rental", "backup", "delete", uidS(95))
	if code != 2 {
		t.Fatalf("delete without --yes: %d", code)
	}
	h.mustRun("rental", "backup", "delete", uidS(95), "--yes")
}

func TestExportCommands(t *testing.T) {
	h := newHarness(t)
	rs := h.rentals()
	id := rs[0].Id.String()
	e := api.WorkspaceExport{Id: uid(96), State: "queued"}
	h.reply("POST /v1/rentals/"+id+"/workspace-exports", 202, e)
	h.reply("GET /v1/rentals/"+id+"/workspace-exports", 200, api.WorkspaceExportList{Items: []api.WorkspaceExportSummary{{Id: uid(96), State: "queued", RequestedAt: time.Now()}}})
	h.reply("GET /v1/rentals/"+id+"/workspace-exports/"+uidS(96), 200, e)
	h.reply("POST /v1/rentals/"+id+"/workspace-exports/"+uidS(96)+"/cancel", 200, e)
	_, errs := h.mustRun("rental", "export", "create", "quiet-river-7k2f")
	contains(t, "create", errs, "legion rental export get quiet-river-7k2f "+uidS(96))
	out, _ := h.mustRun("rental", "export", "list", "quiet-river-7k2f")
	contains(t, "list", out, uidS(96))
	h.mustRun("rental", "export", "get", "quiet-river-7k2f", uidS(96))
	h.mustRun("rental", "export", "cancel", "quiet-river-7k2f", uidS(96))
}

func TestQuietAndStdoutCleanliness(t *testing.T) {
	h := newHarness(t)
	rs := h.rentals()
	id := rs[0].Id.String()
	h.reply("POST /v1/rentals/"+id+"/stop", 200, rentalFixture(1, "quiet-river-7k2f", api.RentalStateStopping))
	out, errs := h.mustRun("rental", "stop", "quiet-river-7k2f", "--yes", "--async", "-o", "json")
	var r api.Rental
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatalf("stdout is not pure JSON: %q", out)
	}
	if errs == "" {
		t.Fatal("expected next-step text on stderr")
	}
	_, errs = h.mustRun("rental", "stop", "quiet-river-7k2f", "--yes", "--async", "-q", "-o", "name")
	if errs != "" {
		t.Fatalf("-q still printed: %q", errs)
	}
}

func TestNoANSIOnPipes(t *testing.T) {
	h := newHarness(t)
	h.rentals()
	out, errs := h.mustRun("rental", "list")
	if strings.Contains(out, "\x1b") || strings.Contains(errs, "\x1b") {
		t.Fatal("escape codes on a non-terminal")
	}
}

func TestCompletionScripts(t *testing.T) {
	h := newHarness(t)
	for sh, marker := range map[string]string{"bash": "bash completion", "zsh": "#compdef", "fish": "complete -c legion", "powershell": "Register-ArgumentCompleter"} {
		out, _ := h.mustRun("completion", sh)
		contains(t, sh, out, marker)
	}
	_, _, code := h.run("completion", "tcsh")
	if code != 2 {
		t.Fatal(code)
	}
	out, _ := h.mustRun("completion", "--help")
	contains(t, "help", out, "source <(legion completion bash)", "fish/completions")
}

func TestDynamicCompletion(t *testing.T) {
	h := newHarness(t)
	h.rentals()
	h.reply("GET /v1/offers", 200, api.OfferPage{Items: []api.Offer{offerFixture()}})
	h.reply("GET /v1/catalog", 200, api.Catalog{Entries: []api.CatalogEntry{{Id: "plain", Name: "Plain", Kind: "image", Reference: "r"}}})
	h.reply("GET /v1/ssh-keys", 200, api.SshKeyList{Items: []api.SshKey{sshKeyFixture()}})
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"__complete", "rental", "get", "qui"}, "quiet-river-7k2f"},
		{[]string{"__complete", "ssh", ""}, "brave-otter-0aa1"},
		{[]string{"__complete", "offer", "get", ""}, "amber-falcon"},
		{[]string{"__complete", "rental", "create", "--offer", ""}, "amber-falcon"},
		{[]string{"__complete", "rental", "create", "--catalog", ""}, "plain"},
		{[]string{"__complete", "catalog", "get", ""}, "plain"},
		{[]string{"__complete", "ssh-key", "remove", ""}, "laptop"},
		{[]string{"__complete", "offer", "list", "--gpu-model", ""}, "RTX 4090"},
	}
	for _, tc := range cases {
		out, _ := h.mustRun(tc.args...)
		if !strings.Contains(out, tc.want) {
			t.Errorf("%v: %q missing %q", tc.args, out, tc.want)
		}
	}
	out, _ := h.mustRun("__complete", "rental", "get", "brave", "")
	excludes(t, "second arg", out, "quiet-river")
}

func TestDynamicCompletionDegradesSilently(t *testing.T) {
	h := newHarness(t)
	h.fail("GET /v1/rentals", 500, "boom", "down")
	out, errs := h.mustRun("__complete", "rental", "get", "")
	excludes(t, "stdout", out, "boom", "Error")
	excludes(t, "stderr", errs, "Error:", "boom")
	h.noKey = true
	out, _ = h.mustRun("__complete", "rental", "get", "")
	excludes(t, "no key", out, "Error")
}

// Every command, group or leaf, documents itself with a Short, a Long and at
// least two examples that start with the program name.
func TestEveryCommandHasExamples(t *testing.T) {
	a := &app{env: Env{Stdin: strings.NewReader(""), Stdout: os.Stderr, Stderr: os.Stderr, Getenv: func(string) string { return "" }}}
	checked := 0
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		if c.HasParent() && c.Name() != "help" && (c.Runnable() || c.HasSubCommands()) {
			checked++
			if c.Short == "" || c.Long == "" {
				t.Errorf("%s: missing Short or Long", c.CommandPath())
			}
			n := 0
			for _, l := range strings.Split(c.Example, "\n") {
				if strings.Contains(l, "legion ") {
					n++
				}
			}
			if n < 2 {
				t.Errorf("%s: %d examples, want at least 2", c.CommandPath(), n)
			}
		}
		for _, s := range c.Commands() {
			walk(s)
		}
	}
	walk(a.root())
	if checked < 50 {
		t.Fatalf("only %d commands walked: the test is not looking at the tree", checked)
	}
}

func TestDocsGeneration(t *testing.T) {
	h := newHarness(t)
	dir := t.TempDir()
	h.mustRun("docs", "--dir", dir)
	b, err := os.ReadFile(filepath.Join(dir, "legion_rental_create.md"))
	if err != nil || !strings.Contains(string(b), "--dry-run") {
		t.Fatalf("%v %s", err, b)
	}
	if strings.Contains(string(b), "Auto generated by spf13/cobra on") {
		t.Fatal("docs carry a date: not reproducible")
	}
	man := t.TempDir()
	h.mustRun("docs", "--dir", man, "--format", "man")
	if _, err := os.Stat(filepath.Join(man, "legion-rental-create.1")); err != nil {
		t.Fatal(err)
	}
}
