package cli

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/AmbientCrypto/legion-controller/api"
)

func exportFixture(h *harness, size int64) (rentalID string) {
	rs := h.rentals()
	id := rs[0].Id.String()
	s := size
	h.reply("GET /v1/rentals/"+id+"/workspace-exports/"+uidS(96), 200, api.WorkspaceExport{Id: uid(96), State: "ready", SizeBytes: &s})
	h.reply("GET /v1/rentals/"+id+"/workspace-exports/estimate", 200, api.WorkspaceExportEstimate{
		EstimatedBytes: 2 << 30, Basis: "disk_used", PriceCentsPerGib: 15, EstimatedDownloadCents: 30, LinkTtlSeconds: 3600, SingleUse: true})
	return id
}

func TestExportEstimate(t *testing.T) {
	h := newHarness(t)
	id := exportFixture(h, 1<<30)
	out, _ := h.mustRun("rental", "export", "estimate", "quiet-river-7k2f")
	contains(t, "estimate", out, "2.0 GiB", "$0.15", "$0.30", "once", "60 minutes")
	if h.count("GET", "/v1/rentals/"+id+"/workspace-exports/estimate") != 1 {
		t.Fatal("estimate not asked")
	}
	j, _ := h.mustRun("rental", "export", "estimate", "quiet-river-7k2f", "-o", "json")
	var e api.WorkspaceExportEstimate
	if err := json.Unmarshal([]byte(j), &e); err != nil || e.EstimatedDownloadCents != 30 || !e.SingleUse {
		t.Fatalf("%v %s", err, j)
	}
}

func TestExportLinkConfirmsAndPrintsSingleUseURL(t *testing.T) {
	h := newHarness(t)
	id := exportFixture(h, 3<<30) // 3 GiB at 15 cents = 45 cents
	route := "/v1/rentals/" + id + "/workspace-exports/" + uidS(96) + "/download-links"
	h.reply("POST "+route, 201, api.WorkspaceExportDownloadLink{Url: "https://api.example/v1/downloads/tok", ExpiresAt: time.Now().Add(time.Hour), ChargeCents: 45, Number: 2})

	// No --yes and no terminal: nothing is charged.
	_, errs, code := h.run("rental", "export", "link", "quiet-river-7k2f", uidS(96))
	if code != 2 || h.count("POST", route) != 0 {
		t.Fatalf("charged without confirmation: code %d", code)
	}
	contains(t, "preview", errs, "$0.45", "3.0 GiB", "once")

	out, errs := h.mustRun("rental", "export", "link", "quiet-river-7k2f", uidS(96), "--yes")
	if strings.TrimSpace(out) != "https://api.example/v1/downloads/tok" {
		t.Fatalf("stdout must be only the URL: %q", out)
	}
	contains(t, "info", errs, "$0.45", "once", "Link 2")
	c := h.last("POST", route)
	if c == nil || c.Header.Get("Idempotency-Key") == "" {
		t.Fatal("no Idempotency-Key")
	}
	j, _ := h.mustRun("rental", "export", "link", "quiet-river-7k2f", uidS(96), "--yes", "-o", "json")
	var l api.WorkspaceExportDownloadLink
	if err := json.Unmarshal([]byte(j), &l); err != nil || l.ChargeCents != 45 {
		t.Fatalf("%v %s", err, j)
	}
	if h.last("POST", route).Header.Get("Idempotency-Key") == c.Header.Get("Idempotency-Key") {
		t.Fatal("two separate links reused one key")
	}
}

func TestExportLinkNotReadyAndNoCredit(t *testing.T) {
	h := newHarness(t)
	id := exportFixture(h, 1<<30)
	h.reply("GET /v1/rentals/"+id+"/workspace-exports/"+uidS(96), 200, api.WorkspaceExport{Id: uid(96), State: "pending"})
	_, errs, code := h.run("rental", "export", "link", "quiet-river-7k2f", uidS(96), "--yes")
	if code == 0 {
		t.Fatal("link for a pending export succeeded")
	}
	contains(t, "pending", errs, "not ready", "rental export get")
	if h.count("POST", "/v1/rentals/"+id+"/workspace-exports/"+uidS(96)+"/download-links") != 0 {
		t.Fatal("asked for a link of a pending export")
	}

	h.reply("GET /v1/rentals/"+id+"/workspace-exports/"+uidS(96), 200, api.WorkspaceExport{Id: uid(96), State: "ready", SizeBytes: ptr(int64(1 << 30))})
	h.fail("POST /v1/rentals/"+id+"/workspace-exports/"+uidS(96)+"/download-links", 402, "insufficient_credit", "no credit")
	_, errs, code = h.run("rental", "export", "link", "quiet-river-7k2f", uidS(96), "--yes")
	if code != 6 {
		t.Fatalf("exit code for 402: %d", code)
	}
	contains(t, "402", errs, "billing balance")
}

func TestExportGetNoLongerPromisesAURL(t *testing.T) {
	h := newHarness(t)
	id := exportFixture(h, 1<<30)
	out, errs := h.mustRun("rental", "export", "get", "quiet-river-7k2f", uidS(96))
	if strings.Contains(strings.ToLower(out), "download url") {
		t.Fatalf("still shows a download URL column:\n%s", out)
	}
	contains(t, "get", errs, "legion rental export link quiet-river-7k2f "+uidS(96), "charged")
	h.reply("POST /v1/rentals/"+id+"/workspace-exports", 202, api.WorkspaceExport{Id: uid(96), State: "pending"})
	_, errs = h.mustRun("rental", "export", "create", "quiet-river-7k2f")
	contains(t, "create", errs, "rental export estimate")
}

func TestExportCreateHelpMentionsLink(t *testing.T) {
	h := newHarness(t)
	out, _ := h.mustRun("rental", "export", "--help")
	contains(t, "help", out, "estimate", "link", "once")
	if strings.Contains(out, "download URL") {
		t.Fatalf("help still promises a download URL:\n%s", out)
	}
}

func TestBackupLinkIncludedChargedAndUsed(t *testing.T) {
	h := newHarness(t)
	route := "/v1/backups/" + uidS(95) + "/download-links"
	h.reply("POST "+route, 201, api.BackupDownloadLink{Url: "https://api.example/v1/downloads/a", LinkNumber: 1, Charge: money(0), ExpiresAt: time.Now().Add(time.Hour)})
	_, errs := h.mustRun("rental", "backup", "download-link", uidS(95))
	contains(t, "first", errs, "included", "once")
	h.reply("POST "+route, 201, api.BackupDownloadLink{Url: "https://api.example/v1/downloads/b", LinkNumber: 2, Charge: money(36), ExpiresAt: time.Now().Add(time.Hour)})
	_, errs = h.mustRun("rental", "backup", "download-link", uidS(95))
	contains(t, "second", errs, "charged $0.36", "once")

	h.fail("POST "+route, 410, "link_used", "This download link was already used or has expired. Make a new link.")
	_, errs, code := h.run("rental", "backup", "download-link", uidS(95))
	if code != 1 {
		t.Fatalf("exit %d", code)
	}
	contains(t, "410", errs, "already used or has expired", "legion rental backup download-link")
}

func TestExportLinkGoneHint(t *testing.T) {
	h := newHarness(t)
	id := exportFixture(h, 1<<30)
	h.fail("POST /v1/rentals/"+id+"/workspace-exports/"+uidS(96)+"/download-links", 410, "link_used", "gone")
	_, errs, code := h.run("rental", "export", "link", "quiet-river-7k2f", uidS(96), "--yes")
	if code != 1 {
		t.Fatalf("exit %d", code)
	}
	contains(t, "410", errs, "already used or has expired", "legion rental export link")
}
