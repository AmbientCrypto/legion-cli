package cli

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/AmbientCrypto/legion-controller/api"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

type svcMode int

const (
	modeAuto svcMode = iota
	modeOff
)

func svcFixture(n int, name string, mode svcMode) api.Service {
	e := api.Service{
		Id: uid(n), Name: name, Url: "https://" + name + ".ambi.run", Mode: api.ServiceModeAutoscaled, Protocol: api.ServiceProtocolOpenai,
		State: api.ServiceStateActive, Status: api.ServiceStatusReady, Revision: 4,
		Access:   api.ServiceAccess{Mode: api.ServiceAccessKeys},
		Replicas: api.ServiceReplicaCounts{Ready: 1, Desired: 1},
		Target: api.ServiceTarget{Mode: api.ServiceModeAutoscaled,
			Hardware: &api.ServiceHardware{GpuModels: &[]string{"H100"}, MaxPricePerReplicaHour: ptr(money(250))},
			Scaling:  &api.ServiceScaling{MinReplicas: 1, MaxReplicas: 3, TargetConcurrency: 8},
			Template: &api.ServiceTemplate{CatalogId: ptr("llama-3-8b"), Model: "llama", Port: 8000}},
		SpendThisMonth:  api.ServiceSpend{Estimated: money(1234), Cap: ptr(money(5000)), Month: "2026-11"},
		MonthlySpendCap: ptr(money(5000)),
	}
	if mode == modeOff {
		e.Mode = api.ServiceModeRental
		e.Target = api.ServiceTarget{Mode: api.ServiceModeRental, RentalId: ptr(uid(30)), Port: ptr(8000),
			Hardware: &api.ServiceHardware{GpuModels: &[]string{"H100"}, MaxPricePerReplicaHour: ptr(money(250))}}
	}
	return e
}

func keyFixture(n int, name string) api.ServiceKey {
	return api.ServiceKey{Id: uid(n), Name: name, Prefix: "lek_ab12cd", Kind: api.ServiceKeyKindService, Status: api.ServiceKeyStatusActive, CreatedAt: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)}
}

// svcFixtures wires defaults, estimate and a service list.
func (h *harness) svcFixtures(eps ...api.Service) {
	h.reply("GET /v1/services", 200, api.ServicePage{Items: eps})
	h.reply("GET /v1/services/defaults", 200, api.ServiceDefaults{
		KeyExpiryDays: 90, KeyExpiryOptions: []int{7, 30, 90, 365}, MinGpus: 1,
		Scaling: api.ServiceScaling{MinReplicas: 1, MaxReplicas: 2, TargetConcurrency: 8},
	})
	h.reply("POST /v1/services/estimate", 200, api.ServiceEstimate{
		AtMinPerHour: money(250), AtMaxPerHour: money(500), IdlePerHour: money(2), SuggestedMonthlySpendCap: money(40000), HostsAvailable: 14,
		LikelyHardware: &struct {
			GpuCount     int       `json:"gpu_count"`
			GpuModel     string    `json:"gpu_model"`
			PricePerHour api.Money `json:"price_per_hour"`
		}{GpuCount: 2, GpuModel: "RTX 4090", PricePerHour: money(84)},
	})
}

func svcCreateArgs(extra ...string) []string {
	return append([]string{"services", "create", "my-svc", "--catalog", "llama-3-8b", "--gpu", "H100", "--gpu", "A100", "--max-price", "2.50", "--monthly-cap", "400"}, extra...)
}

func (h *harness) svcCreated() {
	h.reply("POST /v1/services", 201, api.ServiceCreated{Service: svcFixture(7, "my-svc", modeAuto),
		InitialKey: api.ServiceKeyCreated{Key: keyFixture(8, "default"), Secret: "lek_SECRETVALUE123"}})
}

func TestServicesListColumnsAndFilter(t *testing.T) {
	h := newHarness(t)
	port := svcFixture(2, "pg-port", modeOff)
	port.Target.RentalId = ptr(uid(31))
	h.svcFixtures(svcFixture(1, "chat", modeAuto), svcFixture(3, "single", modeOff), port)
	out, _ := h.mustRun("services", "list")
	contains(t, "list", out, "NAME", "MODE", "ACCESS", "SCALING", "STATUS", "Autoscaled", "On a rental", "API key required", "Auto, 1 to 3", "Off, 1 rental", "pg-port")
	if strings.Contains(out, "TYPE") || strings.Contains(out, "Autoscaled model") {
		t.Fatalf("old type column is back: %s", out)
	}
	out, _ = h.mustRun("services", "list", "--mode", "rental", "-o", "name")
	if strings.TrimSpace(out) != "pg-port\nsingle" && strings.TrimSpace(out) != "single\npg-port" {
		t.Fatalf("mode filter: %q", out)
	}
	out, _ = h.mustRun("svc", "list", "--mode", "autoscaled", "-o", "name")
	if strings.TrimSpace(out) != "chat" {
		t.Fatalf("mode filter autoscaled: %q", out)
	}
	h.reply("GET /v1/rentals", 200, api.RentalPage{Items: []api.Rental{rentalFixture(30, "quiet-river", api.RentalStateRunning)}})
	h.reply("GET /v1/rentals/"+uidS(30), 200, rentalFixture(30, "quiet-river", api.RentalStateRunning))
	h.on("GET /v1/services", func(w http.ResponseWriter, r *http.Request) {
		items := []api.Service{svcFixture(3, "single", modeOff)}
		if r.URL.Query().Get("rental_id") != uidS(30) {
			items = append(items, svcFixture(1, "chat", modeAuto))
		}
		writeJSON(w, 200, api.ServicePage{Items: items})
	})
	out, _ = h.mustRun("services", "list", "--rental", "quiet-river", "-o", "name")
	if strings.TrimSpace(out) != "single" {
		t.Fatalf("rental filter: %q", out)
	}
	for _, bad := range [][]string{{"--mode", "bogus"}, {"--type", "port"}} {
		_, e, c := h.run(append([]string{"services", "list"}, bad...)...)
		if c != 2 {
			t.Fatalf("%v: exit %d %s", bad, c, e)
		}
	}
}

func TestServicesGetByNameAndPrefix(t *testing.T) {
	h := newHarness(t)
	h.svcFixtures(svcFixture(1, "chat-prod", modeAuto))
	h.reply("GET /v1/services/"+uidS(1), 200, svcFixture(1, "chat-prod", modeAuto))
	out, _ := h.mustRun("services", "get", "chat")
	contains(t, "get", out, "chat-prod", "Service URL:", "https://chat-prod.ambi.run", "Mode:", "Autoscaled", "API key required", "Auto", "ready")
	_, e, c := h.run("services", "get", "nope")
	if c != 4 {
		t.Fatalf("exit %d %s", c, e)
	}
	contains(t, "hint", e, "legion services list")
}

func TestServicesCreateSendsRequest(t *testing.T) {
	h := newHarness(t)
	h.svcFixtures()
	h.svcCreated()
	out, errs := h.mustRun(svcCreateArgs("--yes", "--spot", "--min", "0", "--max", "4", "--scale-to-zero", "--target-concurrency", "16", "--key-expires", "30d")...)
	c := h.last("POST", "/v1/services")
	if c == nil {
		t.Fatal("no POST")
	}
	if c.Header.Get("Idempotency-Key") == "" {
		t.Fatal("no Idempotency-Key")
	}
	req := body[api.CreateServiceRequest](t, c)
	tg := req.Target
	if ptrOr(req.Name, "") != "my-svc" || tg.Mode != api.ServiceModeAutoscaled || ptrOr(tg.CatalogId, "") != "llama-3-8b" || tg.Hardware == nil {
		t.Fatalf("%+v", req)
	}
	hw := tg.Hardware
	if hw.GpuModels == nil || len(*hw.GpuModels) != 2 || (*hw.GpuModels)[0] != "H100" || (*hw.GpuModels)[1] != "A100" {
		t.Fatalf("gpu allow-list %+v", hw.GpuModels)
	}
	if hw.Strategy == nil || *hw.Strategy != api.ServiceHardwareCheapestFit || hw.GpuCounts != nil || hw.MinGpuMemoryGb != nil {
		t.Fatalf("hardware %+v", hw)
	}
	if tg.Hardware.MaxPricePerReplicaHour == nil || tg.Hardware.MaxPricePerReplicaHour.AmountMinor != 250 || tg.Hardware.Tier == nil || *tg.Hardware.Tier != api.RentalTierInterruptible {
		t.Fatalf("hardware %+v", tg.Hardware)
	}
	s := tg.Scaling
	if s == nil || ptrOr(s.MinReplicas, -1) != 0 || ptrOr(s.MaxReplicas, -1) != 4 || !ptrOr(s.ScaleToZero, false) || ptrOr(s.TargetConcurrency, -1) != 16 {
		t.Fatalf("scaling %+v", s)
	}
	if req.MonthlySpendCap == nil || req.MonthlySpendCap.AmountMinor != 40000 {
		t.Fatalf("cap %+v", req.MonthlySpendCap)
	}
	if req.InitialKey == nil || req.InitialKey.ExpiresInDays == nil || int(*req.InitialKey.ExpiresInDays) != 30 {
		t.Fatalf("key %+v", req.InitialKey)
	}
	contains(t, "stdout", out, "lek_SECRETVALUE123", "https://my-svc.ambi.run")
	contains(t, "stderr", errs, "only time", "Cost floor", "$2.50", "Cost ceiling", "$5.00", "Monthly cap", "$400.00", "Likely hardware: 2x RTX 4090 at $0.84 per hour, 14 hosts available")
}

func TestServicesCreateNoYesNonInteractiveRefuses(t *testing.T) {
	h := newHarness(t)
	h.svcFixtures()
	h.svcCreated()
	_, e, c := h.run(svcCreateArgs()...)
	if c != 2 || h.posts() != 1 { // the estimate is the only POST
		t.Fatalf("exit %d posts %d: %s", c, h.posts(), e)
	}
	contains(t, "stderr", e, "--yes", "Cost floor")
	if h.count("POST", "/v1/services") != 0 {
		t.Fatal("created without confirmation")
	}
}

func TestServicesCreateGPUAllowListNoCounts(t *testing.T) {
	h := newHarness(t)
	h.svcFixtures()
	_, e, c := h.run(svcCreateArgs("--gpu", "H100:2", "--yes")...)
	if c != 2 {
		t.Fatalf("exit %d", c)
	}
	contains(t, "stderr", e, "GPU model only", "--gpu-counts")
	_, e, c = h.run(svcCreateArgs("--gpu-counts", "1,3", "--yes")...)
	if c != 2 {
		t.Fatalf("bad counts exit %d", c)
	}
	contains(t, "stderr", e, "1,2,4,8")
}

func TestServicesCreateCheapestFitNeedsNoHardware(t *testing.T) {
	h := newHarness(t)
	h.svcFixtures()
	h.svcCreated()
	h.mustRun("services", "create", "my-svc", "--catalog", "llama-3-8b", "--monthly-cap", "400", "--yes")
	req := body[api.CreateServiceRequest](t, h.last("POST", "/v1/services"))
	hw := req.Target.Hardware
	if hw == nil || hw.Strategy == nil || *hw.Strategy != api.ServiceHardwareCheapestFit || hw.GpuModels != nil || hw.MaxPricePerReplicaHour != nil {
		t.Fatalf("%+v", hw)
	}
}

func TestServicesCreateOwnImageNeed(t *testing.T) {
	h := newHarness(t)
	h.svcFixtures()
	h.svcCreated()
	h.mustRun("services", "create", "my-svc", "--image", "ghcr.io/me/serve:1", "--port", "8000", "--min-gpu-memory", "48", "--gpu-counts", "1,2,4", "--monthly-cap", "400", "--yes")
	req := body[api.CreateServiceRequest](t, h.last("POST", "/v1/services"))
	hw := req.Target.Hardware
	if hw.MinGpuMemoryGb == nil || *hw.MinGpuMemoryGb != 48 || hw.GpuCounts == nil || len(*hw.GpuCounts) != 3 || (*hw.GpuCounts)[2] != 4 {
		t.Fatalf("%+v", hw)
	}
	if req.Target.Image == nil || req.Target.Serve == nil || req.Target.Serve.Port != 8000 {
		t.Fatalf("%+v", req.Target)
	}
}

func TestServicesCreateMissingFlagsNonInteractive(t *testing.T) {
	h := newHarness(t)
	h.svcFixtures()
	_, e, c := h.run("services", "create", "my-svc", "--yes")
	if c != 2 {
		t.Fatalf("exit %d", c)
	}
	contains(t, "stderr", e, "--catalog", "--monthly-cap")
	excludes(t, "stderr", e, "--gpu", "--max-price")
	if h.posts() != 0 {
		t.Fatal("sent something")
	}
}

func TestServicesCreateInteractive(t *testing.T) {
	h := newHarness(t)
	h.svcFixtures()
	h.svcCreated()
	h.tty = true
	// kind (2 = autoscaled model), name, catalog id, gpus, max price, spot, min, max, monthly cap, confirm
	h.stdin = "2\nmy-svc\nllama-3-8b\nH100, A100\n2.50\nn\n\n\n400\ny\n"
	out, errs := h.mustRun("services", "create")
	req := body[api.CreateServiceRequest](t, h.last("POST", "/v1/services"))
	if ptrOr(req.Name, "") != "my-svc" || len(*req.Target.Hardware.GpuModels) != 2 || req.MonthlySpendCap.AmountMinor != 40000 || req.Target.Hardware.MaxPricePerReplicaHour.AmountMinor != 250 {
		t.Fatalf("%+v", req)
	}
	if req.Target.Hardware.Tier != nil && *req.Target.Hardware.Tier == api.RentalTierInterruptible {
		t.Fatal("spot chosen")
	}
	contains(t, "stderr", errs, "Cost floor", "[1]", "[2]") // defaults shown from /v1/services/defaults
	contains(t, "stdout", out, "lek_SECRETVALUE123")
	if h.count("GET", "/v1/services/defaults") == 0 {
		t.Fatal("defaults not read")
	}
}

func TestServicesCreateJSONHasSecretOnce(t *testing.T) {
	h := newHarness(t)
	h.svcFixtures()
	h.svcCreated()
	out, _ := h.mustRun(svcCreateArgs("--yes", "-o", "json")...)
	contains(t, "json", out, `"secret": "lek_SECRETVALUE123"`)
}

func TestServicesCreateAckHostVisibility(t *testing.T) {
	h := newHarness(t)
	h.svcFixtures()
	h.svcCreated()
	h.mustRun(svcCreateArgs("--yes", "--ack-host-visibility")...)
	req := body[api.CreateServiceRequest](t, h.last("POST", "/v1/services"))
	if !ptrOr(req.AcknowledgeHostVisibility, false) {
		t.Fatal("ack not sent")
	}
}

func TestServicesUpdateOnlyChangedFields(t *testing.T) {
	h := newHarness(t)
	h.svcFixtures(svcFixture(1, "chat", modeAuto))
	h.reply("GET /v1/services/"+uidS(1), 200, svcFixture(1, "chat", modeAuto))
	h.reply("PATCH /v1/services/"+uidS(1), 200, svcFixture(1, "chat", modeAuto))
	_, errs := h.mustRun("services", "update", "chat", "--max", "5", "--yes")
	req := body[api.UpdateServiceRequest](t, h.last("PATCH", "/v1/services/"+uidS(1)))
	if req.Target == nil || req.Target.Scaling == nil || ptrOr(req.Target.Scaling.MaxReplicas, -1) != 5 || req.Target.Scaling.MinReplicas != nil || req.Target.Hardware != nil || req.MonthlySpendCap != nil || req.Access != nil {
		t.Fatalf("%+v", req)
	}
	contains(t, "stderr", errs, "Cost ceiling")
	_, e, c := h.run("services", "update", "chat", "--yes")
	if c != 2 {
		t.Fatalf("no-op update exit %d %s", c, e)
	}
}

func TestServicesPauseResume(t *testing.T) {
	h := newHarness(t)
	h.svcFixtures(svcFixture(1, "chat", modeAuto))
	h.reply("GET /v1/services/"+uidS(1), 200, svcFixture(1, "chat", modeAuto))
	h.reply("POST /v1/services/"+uidS(1)+"/pause", 200, svcFixture(1, "chat", modeAuto))
	h.reply("POST /v1/services/"+uidS(1)+"/resume", 200, svcFixture(1, "chat", modeAuto))
	h.mustRun("services", "pause", "chat")
	if h.count("POST", "/v1/services/"+uidS(1)+"/pause") != 1 {
		t.Fatal("not paused")
	}
	_, e, c := h.run("services", "resume", "chat")
	if c != 2 || h.count("POST", "/v1/services/"+uidS(1)+"/resume") != 0 {
		t.Fatalf("resume without confirmation exit %d: %s", c, e)
	}
	contains(t, "stderr", e, "Cost floor", "--yes")
	_, errs := h.mustRun("services", "resume", "chat", "--yes")
	contains(t, "stderr", errs, "Cost floor")
	if h.count("POST", "/v1/services/"+uidS(1)+"/resume") != 1 {
		t.Fatal("not resumed")
	}
}

func TestServicesDelete(t *testing.T) {
	h := newHarness(t)
	h.svcFixtures(svcFixture(1, "chat", modeAuto))
	h.reply("DELETE /v1/services/"+uidS(1), 202, svcFixture(1, "chat", modeAuto))
	_, e, c := h.run("services", "delete", "chat")
	if c != 2 || h.count("DELETE", "/v1/services/"+uidS(1)) != 0 {
		t.Fatalf("exit %d: %s", c, e)
	}
	contains(t, "stderr", e, "--yes")
	h.tty = true
	h.stdin = "wrong\n"
	_, _, c = h.run("services", "delete", "chat")
	if c == 0 || h.count("DELETE", "/v1/services/"+uidS(1)) != 0 {
		t.Fatal("deleted on wrong name")
	}
	h.stdin = "chat\n"
	h.mustRun("services", "delete", "chat")
	if h.count("DELETE", "/v1/services/"+uidS(1)) != 1 {
		t.Fatal("not deleted")
	}
	h.tty = false
	h.mustRun("services", "delete", "chat", "--yes")
}

func TestServicesKeys(t *testing.T) {
	h := newHarness(t)
	h.svcFixtures(svcFixture(1, "chat", modeAuto))
	legacy := keyFixture(11, "Direct key (legacy)")
	legacy.Kind = api.ServiceKeyKindLegacyDirect
	legacy.DirectUrl = ptr("https://old.ambi.run")
	h.reply("GET /v1/services/"+uidS(1)+"/keys", 200, api.ServiceKeyPage{Items: []api.ServiceKey{keyFixture(10, "ci"), legacy}})
	out, _ := h.mustRun("services", "keys", "list", "chat")
	contains(t, "keys", out, "ci", "lek_ab12cd", "legacy")
	excludes(t, "keys", out, "SECRET")

	h.reply("POST /v1/services/"+uidS(1)+"/keys", 201, api.ServiceKeyCreated{Key: keyFixture(12, "bot"), Secret: "lek_NEWSECRET"})
	out, errs := h.mustRun("services", "keys", "create", "chat", "--name", "bot", "--expires", "7d")
	contains(t, "create out", out, "lek_NEWSECRET")
	contains(t, "create err", errs, "only time")
	req := body[api.CreateServiceKeyRequest](t, h.last("POST", "/v1/services/"+uidS(1)+"/keys"))
	if req.Name != "bot" || req.ExpiresInDays == nil || int(*req.ExpiresInDays) != 7 || req.NoExpiry != nil {
		t.Fatalf("%+v", req)
	}
	if h.last("POST", "/v1/services/"+uidS(1)+"/keys").Header.Get("Idempotency-Key") == "" {
		t.Fatal("no Idempotency-Key")
	}
	// -q prints only the secret
	out, _ = h.mustRun("services", "keys", "create", "chat", "--name", "bot", "-q", "-o", "name")
	if strings.TrimSpace(out) != "lek_NEWSECRET" {
		t.Fatalf("name out %q", out)
	}
	// never expires: warns, sends no_expiry
	_, errs = h.mustRun("services", "keys", "create", "chat", "--name", "forever", "--expires", "never")
	req = body[api.CreateServiceKeyRequest](t, h.last("POST", "/v1/services/"+uidS(1)+"/keys"))
	if req.ExpiresInDays != nil || !ptrOr(req.NoExpiry, false) {
		t.Fatalf("%+v", req)
	}
	contains(t, "never warning", errs, "never expires")
	_, e, c := h.run("services", "keys", "create", "chat", "--name", "x", "--expires", "5d")
	if c != 2 {
		t.Fatalf("bad expiry exit %d %s", c, e)
	}
	contains(t, "bad expiry", e, "7d", "365d", "never")
}

func TestServicesKeysRevoke(t *testing.T) {
	h := newHarness(t)
	h.svcFixtures(svcFixture(1, "chat", modeAuto))
	h.reply("GET /v1/services/"+uidS(1)+"/keys", 200, api.ServiceKeyPage{Items: []api.ServiceKey{keyFixture(10, "ci")}})
	h.reply("DELETE /v1/services/"+uidS(1)+"/keys/"+uidS(10), 204, nil)
	_, e, c := h.run("services", "keys", "revoke", "chat", "ci")
	if c != 2 || h.count("DELETE", "/v1/services/"+uidS(1)+"/keys/"+uidS(10)) != 0 {
		t.Fatalf("exit %d %s", c, e)
	}
	h.mustRun("services", "keys", "revoke", "chat", "ci", "--yes")
	if h.count("DELETE", "/v1/services/"+uidS(1)+"/keys/"+uidS(10)) != 1 {
		t.Fatal("not revoked")
	}
	_, _, c = h.run("services", "keys", "revoke", "chat", "missing", "--yes")
	if c != 4 {
		t.Fatalf("exit %d", c)
	}
}

func TestServicesKeysRotate(t *testing.T) {
	h := newHarness(t)
	h.svcFixtures(svcFixture(1, "chat", modeAuto))
	h.reply("GET /v1/services/"+uidS(1)+"/keys", 200, api.ServiceKeyPage{Items: []api.ServiceKey{keyFixture(10, "ci")}})
	h.reply("POST /v1/services/"+uidS(1)+"/keys/"+uidS(10)+"/rotate", 201, api.ServiceKeyRotated{NewKey: api.ServiceKeyCreated{Key: keyFixture(13, "ci"), Secret: "lek_ROTATED"}, OldKey: keyFixture(10, "ci")})
	out, errs := h.mustRun("services", "keys", "rotate", "chat", "ci", "--overlap-hours", "12", "--yes")
	contains(t, "rotate", out, "lek_ROTATED")
	contains(t, "rotate", errs, "only time")
}

func TestServicesReplicasUsageEvents(t *testing.T) {
	h := newHarness(t)
	h.svcFixtures(svcFixture(1, "chat", modeAuto))
	h.reply("GET /v1/services/"+uidS(1)+"/replicas", 200, api.ServiceReplicaPage{Items: []api.ServiceReplica{{
		RentalId: uid(20), RentalName: ptr("quiet-river"), Phase: "ready", Ready: true, GpuCount: 1, Tier: api.RentalTierOnDemand,
		PricePerGpuHour: money(250), Host: api.ServiceReplicaHost{GpuModel: "H100"}, RentalState: "running"}}})
	out, _ := h.mustRun("services", "replicas", "chat")
	contains(t, "replicas", out, "quiet-river", "H100", "$2.50")
	h.reply("GET /v1/services/"+uidS(1)+"/usage", 200, api.ServiceUsage{
		Days:           []api.ServiceUsageDay{{Date: dateOf(2026, 11, 17), Requests: 120, Errors: 3, ReplicaSeconds: 7200, GpuSeconds: 7200, Cost: money(500)}},
		SpendThisMonth: api.ServiceSpend{Estimated: money(1234), Cap: ptr(money(5000)), Month: "2026-11"}})
	out, _ = h.mustRun("services", "usage", "chat", "--days", "7")
	contains(t, "usage", out, "2026-11-17", "120", "$5.00", "$12.34", "$50.00")
	if q := h.last("GET", "/v1/services/"+uidS(1)+"/usage").Query["days"]; len(q) != 1 || q[0] != "7" {
		t.Fatalf("days %v", q)
	}
	h.reply("GET /v1/services/"+uidS(1)+"/events", 200, api.ServiceEventPage{Items: []api.ServiceEvent{{Id: 1, At: time.Date(2026, 11, 17, 10, 0, 0, 0, time.UTC), Kind: "scaled_up", Reason: ptr("queue wait over 4s"), ReplicasBefore: ptr(1), ReplicasAfter: ptr(2)}}})
	out, _ = h.mustRun("services", "events", "chat")
	contains(t, "events", out, "scaled_up", "queue wait over 4s", "1 -> 2")
}

func TestServicesAutoscaleOn(t *testing.T) {
	h := newHarness(t)
	ep := svcFixture(1, "single", modeOff)
	h.svcFixtures(ep)
	h.reply("PATCH /v1/services/"+uidS(1), 200, svcFixture(1, "single", modeAuto))
	_, errs := h.mustRun("services", "autoscale", "on", "single", "--gpu", "H100", "--min", "1", "--max", "3", "--monthly-cap", "300", "--max-price", "2.50", "--yes")
	req := body[api.UpdateServiceRequest](t, h.last("PATCH", "/v1/services/"+uidS(1)))
	tc := req.Target
	if tc == nil || tc.Mode == nil || *tc.Mode != api.ServiceModeAutoscaled || tc.Scaling == nil || ptrOr(tc.Scaling.MaxReplicas, 0) != 3 {
		t.Fatalf("%+v", tc)
	}
	if tc.Strategy == nil || *tc.Strategy != api.ServiceSwitchKeepRental {
		t.Fatalf("strategy %+v", tc.Strategy)
	}
	if req.MonthlySpendCap == nil || req.MonthlySpendCap.AmountMinor != 30000 || tc.Hardware == nil || tc.Hardware.GpuModels == nil || len(*tc.Hardware.GpuModels) != 1 {
		t.Fatalf("%+v", req)
	}
	contains(t, "stderr", errs, "Cost floor", "Cost ceiling", "keep")
	h.mustRun("services", "autoscale", "on", "single", "--replace", "--monthly-cap", "300", "--yes")
	req = body[api.UpdateServiceRequest](t, h.last("PATCH", "/v1/services/"+uidS(1)))
	if req.Target == nil || req.Target.Strategy == nil || *req.Target.Strategy != api.ServiceSwitchReplace {
		t.Fatalf("%+v", req.Target)
	}
	// prefilled from the rental's own hardware
	if req.Target.Hardware == nil || req.Target.Hardware.GpuModels == nil || (*req.Target.Hardware.GpuModels)[0] != "H100" {
		t.Fatalf("not prefilled: %+v", req.Target.Hardware)
	}
	_, _, c := h.run("services", "autoscale", "on", "single", "--keep-rental", "--replace", "--monthly-cap", "300", "--yes")
	if c != 2 {
		t.Fatalf("both flags exit %d", c)
	}
	_, e, c := h.run("services", "autoscale", "on", "single", "--yes")
	if c != 2 {
		t.Fatalf("missing cap exit %d", c)
	}
	contains(t, "stderr", e, "--monthly-cap")
}

func TestServicesAutoscaleOff(t *testing.T) {
	h := newHarness(t)
	h.svcFixtures(svcFixture(1, "chat", modeAuto))
	h.reply("PATCH /v1/services/"+uidS(1), 200, svcFixture(1, "chat", modeOff))
	h.reply("GET /v1/services/"+uidS(1)+"/replicas", 200, api.ServiceReplicaPage{Items: []api.ServiceReplica{
		{RentalId: uid(20), RentalName: ptr("a"), Ready: true, Phase: "ready"}, {RentalId: uid(21), RentalName: ptr("b"), Ready: true, Phase: "ready"}}})
	_, e, c := h.run("services", "autoscale", "off", "chat", "--yes")
	if c != 2 {
		t.Fatalf("exit %d %s", c, e)
	}
	contains(t, "stderr", e, "--keep-replica")
	_, errs := h.mustRun("services", "autoscale", "off", "chat", "--keep-replica", uidS(21), "--yes")
	req := body[api.UpdateServiceRequest](t, h.last("PATCH", "/v1/services/"+uidS(1)))
	if req.Target == nil || req.Target.Mode == nil || *req.Target.Mode != api.ServiceModeRental || req.Target.KeepRentalId == nil || req.Target.KeepRentalId.String() != uidS(21) {
		t.Fatalf("%+v", req)
	}
	contains(t, "stderr", errs, "drain")
	// interactive pick
	h.tty = true
	h.stdin = "1\ny\n"
	h.mustRun("services", "autoscale", "off", "chat")
	req = body[api.UpdateServiceRequest](t, h.last("PATCH", "/v1/services/"+uidS(1)))
	if req.Target.KeepRentalId.String() != uidS(20) {
		t.Fatalf("picked %v", req.Target.KeepRentalId)
	}
}

func (h *harness) portFixtures() {
	h.reply("GET /v1/rentals", 200, api.RentalPage{Items: []api.Rental{rentalFixture(9, "quiet-river-7k2f", api.RentalStateRunning)}})
	h.reply("GET /v1/rentals/"+uidS(9), 200, rentalFixture(9, "quiet-river-7k2f", api.RentalStateRunning))
}

func portService(access api.ServiceAccessMode) api.Service {
	e := svcFixture(41, "quiet-river-7k2f", modeOff)
	e.Mode, e.Access = api.ServiceModeRental, api.ServiceAccess{Mode: access}
	e.Target.RentalId = ptr(uid(9))
	return e
}

func TestServicesCreateRentalPort(t *testing.T) {
	h := newHarness(t)
	h.portFixtures()
	h.reply("POST /v1/services", 201, api.ServiceCreated{Service: portService(api.ServiceAccessKeys), InitialKey: api.ServiceKeyCreated{Key: keyFixture(8, "default"), Secret: "lek_PORTSECRET"}})
	out, errs := h.mustRun("services", "create", "--rental", "quiet-river-7k2f", "--port", "8080", "--access", "keys", "--source", "203.0.113.0/24", "--yes")
	c := h.last("POST", "/v1/services")
	req := body[api.CreateServiceRequest](t, c)
	if req.Target.Mode != api.ServiceModeRental || req.Target.RentalId == nil || req.Target.RentalId.String() != uidS(9) || ptrOr(req.Target.Port, 0) != 8080 {
		t.Fatalf("%+v", req.Target)
	}
	if req.Access == nil || req.Access.Mode != api.ServiceAccessKeys || req.Access.Sources == nil || (*req.Access.Sources)[0] != "203.0.113.0/24" {
		t.Fatalf("access %+v", req.Access)
	}
	if req.InitialKey == nil || req.Name != nil || c.Header.Get("Idempotency-Key") == "" {
		t.Fatalf("%+v", req)
	}
	if h.count("POST", "/v1/services/estimate") != 0 {
		t.Fatal("a port costs no GPU: no estimate")
	}
	contains(t, "stdout", out, "lek_PORTSECRET", "https://quiet-river-7k2f.ambi.run")
	contains(t, "stderr", errs, "only time")
}

func TestServicesCreateRentalPortPublicNeedsConfirmation(t *testing.T) {
	h := newHarness(t)
	h.portFixtures()
	h.reply("POST /v1/services", 201, api.ServiceCreated{Service: portService(api.ServiceAccessPublic)})
	_, e, c := h.run("services", "create", "--rental", "quiet-river-7k2f", "--port", "8080")
	if c != 2 || h.count("POST", "/v1/services") != 0 {
		t.Fatalf("exit %d: %s", c, e)
	}
	contains(t, "stderr", e, "internet")
	_, errs := h.mustRun("services", "create", "--rental", "quiet-river-7k2f", "--port", "8080", "--yes")
	req := body[api.CreateServiceRequest](t, h.last("POST", "/v1/services"))
	if req.Access != nil || req.InitialKey != nil {
		t.Fatalf("public default should send no access or key: %+v", req)
	}
	contains(t, "stderr", errs, "public", "--access keys")
	_, _, c = h.run("services", "create", "--rental", "quiet-river-7k2f", "--yes")
	if c != 2 {
		t.Fatalf("missing port exit %d", c)
	}
	_, _, c = h.run("services", "create", "--rental", "x", "--catalog", "m", "--port", "1", "--yes")
	if c != 2 {
		t.Fatalf("mixed exit %d", c)
	}
}

func TestServicesCreateInteractivePort(t *testing.T) {
	h := newHarness(t)
	h.portFixtures()
	h.reply("POST /v1/services", 201, api.ServiceCreated{Service: portService(api.ServiceAccessKeys), InitialKey: api.ServiceKeyCreated{Key: keyFixture(8, "default"), Secret: "lek_X"}})
	h.tty = true
	// kind 1 (a port), rental 1, port, access (default keys), confirm
	h.stdin = "1\n1\n8080\n\ny\n"
	h.mustRun("services", "create")
	req := body[api.CreateServiceRequest](t, h.last("POST", "/v1/services"))
	if ptrOr(req.Target.Port, 0) != 8080 || req.Access == nil || req.Access.Mode != api.ServiceAccessKeys {
		t.Fatalf("%+v", req)
	}
}

func TestServicesUpdateAccess(t *testing.T) {
	h := newHarness(t)
	ep := portService(api.ServiceAccessPublic)
	h.svcFixtures(ep)
	h.reply("PATCH /v1/services/"+uidS(41), 200, ep)
	h.mustRun("services", "update", "quiet-river-7k2f", "--access", "keys")
	req := body[api.UpdateServiceRequest](t, h.last("PATCH", "/v1/services/"+uidS(41)))
	if req.Access == nil || req.Access.Mode != api.ServiceAccessKeys || req.Target != nil {
		t.Fatalf("%+v", req)
	}
	// switching to public from keys asks first
	ek := portService(api.ServiceAccessKeys)
	h.svcFixtures(ek)
	_, e, c := h.run("services", "update", "quiet-river-7k2f", "--access", "public")
	if c != 2 {
		t.Fatalf("exit %d %s", c, e)
	}
	h.mustRun("services", "update", "quiet-river-7k2f", "--access", "public", "--source", "10.0.0.0/8", "--yes")
	req = body[api.UpdateServiceRequest](t, h.last("PATCH", "/v1/services/"+uidS(41)))
	if req.Access.Mode != api.ServiceAccessPublic || req.Access.Sources == nil || (*req.Access.Sources)[0] != "10.0.0.0/8" {
		t.Fatalf("%+v", req.Access)
	}
	_, _, c = h.run("services", "update", "quiet-river-7k2f", "--access", "bogus")
	if c != 2 {
		t.Fatalf("bogus exit %d", c)
	}
}

func TestEndpointsAndIngressCommandsAreGone(t *testing.T) {
	h := newHarness(t)
	for _, cmd := range [][]string{{"endpoints", "list"}, {"endpoint", "list"}, {"ep", "list"}, {"rental", "ingress", "list", "quiet-river-7k2f"}, {"ports", "list"}} {
		_, _, c := h.run(cmd...)
		if c != 2 {
			t.Fatalf("%v should be an unknown command, exit %d", cmd, c)
		}
	}
	if len(h.calls) != 0 {
		t.Fatalf("a removed command called the API: %v", h.calls)
	}
}

func TestServiceProblemCodesAreWorded(t *testing.T) {
	cases := []struct {
		code int
		name string
		want []string
	}{
		{409, "service_name_taken", []string{"legion services list"}},
		{409, "too_many_services", []string{"legion services delete"}},
		{400, "invalid_service_name", []string{"legion services create --help"}},
		{404, "service_not_found", []string{"legion services list"}},
		{409, "rental_managed_by_service", []string{"legion services update"}},
		{403, "service_unavailable", []string{"cannot give this port a public URL"}},
		{409, "service_limit", []string{"reached the limit of services", "legion services delete"}},
	}
	for _, tc := range cases {
		h := newHarness(t)
		h.svcFixtures()
		h.fail("POST /v1/services", tc.code, tc.name, "server detail")
		_, e, _ := h.run(append(svcCreateArgs("--yes"), "--key-name", "k")...)
		contains(t, tc.name, e, tc.want...)
	}
}

var _ = http.StatusOK

func dateOf(y int, m time.Month, d int) openapi_types.Date {
	return openapi_types.Date{Time: time.Date(y, m, d, 0, 0, 0, 0, time.UTC)}
}
