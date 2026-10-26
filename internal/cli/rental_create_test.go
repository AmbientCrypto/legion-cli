package cli

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/AmbientCrypto/legion-controller/api"
)

func createArgs(extra ...string) []string {
	return append([]string{"rental", "create", "--offer", uidS(50), "--image", "pytorch/pytorch:2", "--gpus", "2"}, extra...)
}

func TestRentalCreateHappyPath(t *testing.T) {
	h := newHarness(t)
	h.stdFixtures()
	h.rentalLifecycle("quiet-river-7k2f", 9)
	out, errs := h.mustRun(createArgs("--yes", "-o", "name")...)
	if strings.TrimSpace(out) != "quiet-river-7k2f" {
		t.Fatalf("stdout %q", out)
	}
	c := h.last("POST", "/v1/rentals")
	if c == nil {
		t.Fatal("no POST")
	}
	req := body[api.CreateRentalRequest](t, c)
	if req.HostId.String() != uidS(50) || req.GpuCount != 2 || req.Image != "pytorch/pytorch:2" || req.SshPublicKey != "ssh-ed25519 AAAAC3Nza laptop" {
		t.Fatalf("%+v", req)
	}
	if req.MaxPricePerGpuHour == nil || req.MaxPricePerGpuHour.AmountMinor != 250 || req.MaxPricePerGpuHour.Currency != "USD" {
		t.Fatalf("max price not sent: %+v", req.MaxPricePerGpuHour)
	}
	if c.Header.Get("Idempotency-Key") == "" {
		t.Fatal("no Idempotency-Key")
	}
	contains(t, "stderr", errs, "Created rental quiet-river-7k2f", "Connect: legion ssh quiet-river-7k2f", "Stop billing: legion rental stop quiet-river-7k2f")
	if h.count("GET", "/v1/rentals/"+uidS(9)) < 2 {
		t.Fatal("did not wait for running")
	}
}

func TestRentalCreateFreshIdempotencyKeys(t *testing.T) {
	h := newHarness(t)
	h.stdFixtures()
	h.rentalLifecycle("quiet-river-7k2f", 9)
	h.mustRun(createArgs("--yes", "--async")...)
	k1 := h.last("POST", "/v1/rentals").Header.Get("Idempotency-Key")
	h.mustRun(createArgs("--yes", "--async")...)
	k2 := h.last("POST", "/v1/rentals").Header.Get("Idempotency-Key")
	if k1 == "" || k1 == k2 {
		t.Fatalf("keys %q %q", k1, k2)
	}
}

func TestRentalCreateAsyncDoesNotWait(t *testing.T) {
	h := newHarness(t)
	h.stdFixtures()
	h.rentalLifecycle("quiet-river-7k2f", 9)
	_, errs := h.mustRun(createArgs("--yes", "--async")...)
	if h.count("GET", "/v1/rentals/"+uidS(9)) != 0 {
		t.Fatal("polled in --async")
	}
	contains(t, "stderr", errs, "legion rental wait quiet-river-7k2f")
}

func TestRentalCreateDryRun(t *testing.T) {
	h := newHarness(t)
	h.stdFixtures()
	out, _ := h.mustRun(createArgs("--dry-run")...)
	contains(t, "preview", out, "amber-falcon", "RTX 4090", "GPUs:      2", "$2.50", "$5.00", "Balance", "$50.00", "pytorch/pytorch:2", "laptop")
	if h.posts() != 0 {
		t.Fatal("dry-run sent a write")
	}
}

func TestRentalCreateNonTTYRefusesWithoutYes(t *testing.T) {
	h := newHarness(t)
	h.stdFixtures()
	_, errs, code := h.run(createArgs()...)
	if code != 2 {
		t.Fatalf("exit %d", code)
	}
	contains(t, "stderr", errs, "--yes", "Hint:")
	if h.posts() != 0 {
		t.Fatal("POST without confirmation")
	}
}

func TestRentalCreateTTYPrompt(t *testing.T) {
	h := newHarness(t)
	h.stdFixtures()
	h.rentalLifecycle("quiet-river-7k2f", 9)
	h.tty = true
	h.stdin = "n\n"
	_, errs, code := h.run(createArgs()...)
	if code != 1 || h.posts() != 0 {
		t.Fatalf("decline: exit %d posts %d", code, h.posts())
	}
	contains(t, "preview on stderr", errs, "amber-falcon", "$5.00", "Rent these GPUs")
	h.stdin = "y\n"
	h.mustRun(createArgs("--async")...)
	if h.count("POST", "/v1/rentals") != 1 {
		t.Fatal("not posted after y")
	}
}

func TestRentalCreateMaxPrice(t *testing.T) {
	h := newHarness(t)
	h.stdFixtures()
	h.rentalLifecycle("quiet-river-7k2f", 9)
	h.mustRun(createArgs("--yes", "--async", "--max-price", "3")...)
	req := body[api.CreateRentalRequest](t, h.last("POST", "/v1/rentals"))
	if req.MaxPricePerGpuHour.AmountMinor != 300 {
		t.Fatalf("%+v", req.MaxPricePerGpuHour)
	}
	_, _, code := h.run(createArgs("--yes", "--max-price", "abc")...)
	if code != 2 {
		t.Fatal(code)
	}
}

func TestRentalCreatePriceChanged(t *testing.T) {
	h := newHarness(t)
	h.stdFixtures()
	h.on("POST /v1/rentals", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(409)
		w.Write([]byte(`{"type":"about:blank","title":"Price changed","status":409,"code":"price_changed","current_price_per_gpu_hour":{"amount_minor":300,"currency":"USD"}}`))
	})
	_, errs, code := h.run(createArgs("--yes")...)
	if code != 5 {
		t.Fatalf("exit %d", code)
	}
	contains(t, "stderr", errs, "price changed", "$3.00", "Code: price_changed", "Hint: Re-run")
}

func TestRentalCreateInsufficientCredit(t *testing.T) {
	h := newHarness(t)
	h.stdFixtures()
	h.fail("POST /v1/rentals", 402, "insufficient_credit", "The organization has no credit.")
	_, errs, code := h.run(createArgs("--yes")...)
	if code != 6 {
		t.Fatalf("exit %d", code)
	}
	contains(t, "stderr", errs, "Code: insufficient_credit", "legion billing balance")
}

func TestRentalCreateFromCatalogModel(t *testing.T) {
	h := newHarness(t)
	h.stdFixtures()
	h.rentalLifecycle("quiet-river-7k2f", 9)
	h.on("GET /v1/catalog", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, api.Catalog{Entries: []api.CatalogEntry{
			{Id: "plain", Name: "Plain", Kind: "image", Reference: "reg/plain@sha256:aa", Default: true},
			{Id: "qwen", Name: "Qwen", Kind: "model", Reference: "reg/qwen@sha256:bb", Compatible: ptr(true), MinGpus: ptr(2),
				Serve: &api.CatalogServe{Port: 8000, Protocol: "openai", Model: "Qwen/Qwen3.5-4B"}},
		}})
	})
	h.mustRun("rental", "create", "--offer", uidS(50), "--catalog", "qwen", "--yes", "--async")
	if got := h.last("GET", "/v1/catalog").Query["host_id"]; len(got) != 1 || got[0] != uidS(50) {
		t.Fatalf("catalog not asked per host: %v", got)
	}
	req := body[api.CreateRentalRequest](t, h.last("POST", "/v1/rentals"))
	if req.Image != "reg/qwen@sha256:bb" || req.GpuCount != 2 || req.Serve == nil || req.Serve.Port != 8000 || req.Serve.Kind != "openai" || req.Serve.Model != "Qwen/Qwen3.5-4B" || req.Serve.CatalogId == nil || *req.Serve.CatalogId != "qwen" {
		t.Fatalf("%+v serve=%+v", req, req.Serve)
	}
}

func TestRentalCreateCatalogIncompatible(t *testing.T) {
	h := newHarness(t)
	h.stdFixtures()
	h.reply("GET /v1/catalog", 200, api.Catalog{Entries: []api.CatalogEntry{
		{Id: "big", Kind: "model", Reference: "r", Compatible: ptr(false), Incompatibilities: &[]api.CatalogIncompatibility{{Code: "gpu_memory", Detail: "needs 80 GB"}}},
	}})
	_, errs, code := h.run("rental", "create", "--offer", uidS(50), "--catalog", "big", "--yes")
	if code != 1 || h.posts() != 0 {
		t.Fatalf("exit %d", code)
	}
	contains(t, "stderr", errs, "needs 80 GB", "legion offer list --catalog big")
}

func TestRentalCreateSSHKeySelection(t *testing.T) {
	// No saved key: fall back to ~/.ssh/id_ed25519.pub with a hint.
	h := newHarness(t)
	h.stdFixtures()
	h.reply("GET /v1/ssh-keys", 200, api.SshKeyList{Items: []api.SshKey{}})
	h.rentalLifecycle("quiet-river-7k2f", 9)
	_, _, code := h.run(createArgs("--yes", "--async")...)
	if code != 2 {
		t.Fatalf("no key anywhere should be a usage error, got %d", code)
	}
	h.writeKey("id_ed25519.pub", "ssh-ed25519 AAAAFILE me@box\n")
	_, errs := h.mustRun(createArgs("--yes", "--async")...)
	req := body[api.CreateRentalRequest](t, h.last("POST", "/v1/rentals"))
	if req.SshPublicKey != "ssh-ed25519 AAAAFILE me@box" {
		t.Fatalf("%q", req.SshPublicKey)
	}
	contains(t, "hint", errs, "legion ssh-key add")

	// --ssh-key by label picks the saved key; a path reads the file.
	h2 := newHarness(t)
	h2.stdFixtures()
	h2.rentalLifecycle("quiet-river-7k2f", 9)
	h2.mustRun(createArgs("--yes", "--async", "--ssh-key", "laptop")...)
	req = body[api.CreateRentalRequest](t, h2.last("POST", "/v1/rentals"))
	if req.SshPublicKey != "ssh-ed25519 AAAAC3Nza laptop" {
		t.Fatalf("%q", req.SshPublicKey)
	}
	h2.writeKey("other.pub", "ssh-ed25519 AAAAOTHER x\n")
	h2.mustRun(createArgs("--yes", "--async", "--ssh-key", h2.home+"/.ssh/other.pub")...)
	req = body[api.CreateRentalRequest](t, h2.last("POST", "/v1/rentals"))
	if req.SshPublicKey != "ssh-ed25519 AAAAOTHER x" {
		t.Fatalf("%q", req.SshPublicKey)
	}
	// An unknown label is a not-found-ish usage error, not a silent fallback.
	_, _, code = h2.run(createArgs("--yes", "--ssh-key", "nope")...)
	if code == 0 {
		t.Fatal("unknown key accepted")
	}
}

func TestRentalCreateNeedsOfferNonTTY(t *testing.T) {
	h := newHarness(t)
	h.stdFixtures()
	_, errs, code := h.run("rental", "create", "--image", "x", "--yes")
	if code != 2 {
		t.Fatalf("exit %d", code)
	}
	contains(t, "stderr", errs, "--offer", "legion offer list")
}

func TestRentalCreateInteractivePicks(t *testing.T) {
	h := newHarness(t)
	h.stdFixtures()
	h.rentalLifecycle("quiet-river-7k2f", 9)
	h.tty = true
	h.reply("GET /v1/offers", 200, api.OfferPage{Items: []api.Offer{offerFixture()}})
	h.reply("GET /v1/catalog", 200, api.Catalog{Entries: []api.CatalogEntry{{Id: "plain", Name: "Plain", Kind: "image", Reference: "reg/plain@sha256:aa", Default: true, Compatible: ptr(true), MinGpus: ptr(1)}}})
	h.stdin = "1\n1\ny\n" // offer, image, confirm
	h.mustRun("rental", "create", "--gpus", "1", "--async")
	req := body[api.CreateRentalRequest](t, h.last("POST", "/v1/rentals"))
	if req.HostId.String() != uidS(50) || req.Image != "reg/plain@sha256:aa" {
		t.Fatalf("%+v", req)
	}
}

func TestRentalCreateNotEnoughGPUs(t *testing.T) {
	h := newHarness(t)
	h.stdFixtures()
	_, errs, code := h.run(createArgs("--yes", "--gpus", "9")...)
	if code != 2 || h.posts() != 0 {
		t.Fatalf("exit %d", code)
	}
	contains(t, "stderr", errs, "4 GPUs")
}

func TestRentalCreateWaitFailure(t *testing.T) {
	h := newHarness(t)
	h.stdFixtures()
	h.reply("POST /v1/rentals", 201, rentalFixture(9, "quiet-river-7k2f", api.RentalStateStarting))
	failed := rentalFixture(9, "quiet-river-7k2f", api.RentalStateFailed)
	failed.FailureReason = ptr("image_pull_failed")
	h.reply("GET /v1/rentals/"+uidS(9), 200, failed)
	_, errs, code := h.run(createArgs("--yes")...)
	if code != 1 {
		t.Fatalf("exit %d", code)
	}
	contains(t, "stderr", errs, "image_pull_failed", "legion rental delete quiet-river-7k2f")
}

func TestRentalCreateInterrupted(t *testing.T) {
	h := newHarness(t)
	h.stdFixtures()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	h.ctx = ctx
	_, errs, code := h.run(createArgs("--yes")...)
	if code != 130 {
		t.Fatalf("exit %d: %s", code, errs)
	}
	contains(t, "stderr", errs, "interrupted")
}

func TestRentalCreateNetworkErrorWarnsAboutDuplicates(t *testing.T) {
	h := newHarness(t)
	h.stdFixtures()
	h.on("POST /v1/rentals", func(w http.ResponseWriter, r *http.Request) {
		hj, _ := w.(http.Hijacker)
		conn, _, _ := hj.Hijack()
		conn.Close() // the connection drops after the request was sent
	})
	_, errs, code := h.run(createArgs("--yes")...)
	if code != 1 {
		t.Fatalf("exit %d", code)
	}
	contains(t, "stderr", errs, "Code: network_error", "legion rental list")
}
