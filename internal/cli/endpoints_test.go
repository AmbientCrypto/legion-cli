package cli

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/AmbientCrypto/legion-controller/api"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

type epMode int

const (
	modeAuto epMode = iota
	modeOff
)

func epFixture(n int, name string, mode epMode) api.Endpoint {
	e := api.Endpoint{
		Id: uid(n), Name: name, Url: "https://" + name + ".ambi.run", Type: api.EndpointTypeAutoscaledModel, Protocol: api.EndpointProtocolOpenai,
		State: api.EndpointStateActive, Status: api.EndpointStatusReady, Revision: 4,
		Access:   api.EndpointAccess{Mode: api.EndpointAccessKeys},
		Replicas: api.EndpointReplicaCounts{Ready: 1, Desired: 1},
		Target: api.EndpointTarget{Kind: api.EndpointTargetAutoscale,
			Hardware: &api.EndpointHardware{GpuModels: &[]string{"H100"}, MaxPricePerReplicaHour: ptr(money(250))},
			Scaling:  &api.EndpointScaling{MinReplicas: 1, MaxReplicas: 3, TargetConcurrency: 8},
			Template: &api.EndpointTemplate{CatalogId: ptr("llama-3-8b"), Model: "llama", Port: 8000}},
		SpendThisMonth:  api.EndpointSpend{Estimated: money(1234), Cap: ptr(money(5000)), Month: "2026-11"},
		MonthlySpendCap: ptr(money(5000)),
	}
	if mode == modeOff {
		e.Type = api.EndpointTypeModel
		e.Target = api.EndpointTarget{Kind: api.EndpointTargetRental, RentalId: ptr(uid(30)), Port: ptr(8000),
			Hardware: &api.EndpointHardware{GpuModels: &[]string{"H100"}, MaxPricePerReplicaHour: ptr(money(250))}}
	}
	return e
}

func keyFixture(n int, name string) api.EndpointKey {
	return api.EndpointKey{Id: uid(n), Name: name, Prefix: "lek_ab12cd", Kind: api.EndpointKeyKindEndpoint, Status: api.EndpointKeyStatusActive, CreatedAt: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)}
}

// epFixtures wires defaults, estimate and an endpoint list.
func (h *harness) epFixtures(eps ...api.Endpoint) {
	h.reply("GET /v1/endpoints", 200, api.EndpointPage{Items: eps})
	h.reply("GET /v1/endpoints/defaults", 200, api.EndpointDefaults{
		KeyExpiryDays: 90, KeyExpiryOptions: []int{7, 30, 90, 365}, MinGpus: 1,
		Scaling: api.EndpointScaling{MinReplicas: 1, MaxReplicas: 2, TargetConcurrency: 8},
	})
	h.reply("POST /v1/endpoints/estimate", 200, api.EndpointEstimate{
		AtMinPerHour: money(250), AtMaxPerHour: money(500), IdlePerHour: money(2), SuggestedMonthlySpendCap: money(40000), HostsAvailable: 14,
		LikelyHardware: &struct {
			GpuCount     int       `json:"gpu_count"`
			GpuModel     string    `json:"gpu_model"`
			PricePerHour api.Money `json:"price_per_hour"`
		}{GpuCount: 2, GpuModel: "RTX 4090", PricePerHour: money(84)},
	})
}

func epCreateArgs(extra ...string) []string {
	return append([]string{"endpoints", "create", "my-ep", "--catalog", "llama-3-8b", "--gpu", "H100", "--gpu", "A100", "--max-price", "2.50", "--monthly-cap", "400"}, extra...)
}

func (h *harness) epCreated() {
	h.reply("POST /v1/endpoints", 201, api.EndpointCreated{Endpoint: epFixture(7, "my-ep", modeAuto),
		InitialKey: api.EndpointKeyCreated{Key: keyFixture(8, "default"), Secret: "lek_SECRETVALUE123"}})
}

func TestEndpointsListColumnsAndFilter(t *testing.T) {
	h := newHarness(t)
	port := epFixture(2, "pg-port", modeOff)
	port.Type = api.EndpointTypePort
	port.Target.RentalId = ptr(uid(31))
	h.epFixtures(epFixture(1, "chat", modeAuto), epFixture(3, "single", modeOff), port)
	out, _ := h.mustRun("endpoints", "list")
	contains(t, "list", out, "NAME", "TYPE", "ACCESS", "SCALING", "STATUS", "Autoscaled model", "Auto, 1 to 3", "Off, 1 rental", "Port", "pg-port")
	out, _ = h.mustRun("endpoints", "list", "--type", "port", "-o", "name")
	if strings.TrimSpace(out) != "pg-port" {
		t.Fatalf("filter: %q", out)
	}
	h.reply("GET /v1/rentals", 200, api.RentalPage{Items: []api.Rental{rentalFixture(30, "quiet-river", api.RentalStateRunning)}})
	h.reply("GET /v1/rentals/"+uidS(30), 200, rentalFixture(30, "quiet-river", api.RentalStateRunning))
	out, _ = h.mustRun("ep", "list", "--rental", "quiet-river", "-o", "name")
	if strings.TrimSpace(out) != "single" {
		t.Fatalf("rental filter: %q", out)
	}
	_, e, c := h.run("endpoints", "list", "--type", "bogus")
	if c != 2 {
		t.Fatalf("exit %d %s", c, e)
	}
}

func TestEndpointsGetByNameAndPrefix(t *testing.T) {
	h := newHarness(t)
	h.epFixtures(epFixture(1, "chat-prod", modeAuto))
	h.reply("GET /v1/endpoints/"+uidS(1), 200, epFixture(1, "chat-prod", modeAuto))
	out, _ := h.mustRun("endpoints", "get", "chat")
	contains(t, "get", out, "chat-prod", "https://chat-prod.ambi.run", "Auto", "ready")
	_, e, c := h.run("endpoints", "get", "nope")
	if c != 4 {
		t.Fatalf("exit %d %s", c, e)
	}
	contains(t, "hint", e, "legion endpoints list")
}

func TestEndpointsCreateSendsRequest(t *testing.T) {
	h := newHarness(t)
	h.epFixtures()
	h.epCreated()
	out, errs := h.mustRun(epCreateArgs("--yes", "--spot", "--min", "0", "--max", "4", "--scale-to-zero", "--target-concurrency", "16", "--key-expires", "30d")...)
	c := h.last("POST", "/v1/endpoints")
	if c == nil {
		t.Fatal("no POST")
	}
	if c.Header.Get("Idempotency-Key") == "" {
		t.Fatal("no Idempotency-Key")
	}
	req := body[api.CreateEndpointRequest](t, c)
	tg := req.Target
	if ptrOr(req.Name, "") != "my-ep" || tg.Kind != api.EndpointTargetAutoscale || ptrOr(tg.CatalogId, "") != "llama-3-8b" || tg.Hardware == nil {
		t.Fatalf("%+v", req)
	}
	hw := tg.Hardware
	if hw.GpuModels == nil || len(*hw.GpuModels) != 2 || (*hw.GpuModels)[0] != "H100" || (*hw.GpuModels)[1] != "A100" {
		t.Fatalf("gpu allow-list %+v", hw.GpuModels)
	}
	if hw.Strategy == nil || *hw.Strategy != api.EndpointHardwareCheapestFit || hw.GpuCounts != nil || hw.MinGpuMemoryGb != nil {
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
	contains(t, "stdout", out, "lek_SECRETVALUE123", "https://my-ep.ambi.run")
	contains(t, "stderr", errs, "only time", "Cost floor", "$2.50", "Cost ceiling", "$5.00", "Monthly cap", "$400.00", "Likely hardware: 2x RTX 4090 at $0.84 per hour, 14 hosts available")
}

func TestEndpointsCreateNoYesNonInteractiveRefuses(t *testing.T) {
	h := newHarness(t)
	h.epFixtures()
	h.epCreated()
	_, e, c := h.run(epCreateArgs()...)
	if c != 2 || h.posts() != 1 { // the estimate is the only POST
		t.Fatalf("exit %d posts %d: %s", c, h.posts(), e)
	}
	contains(t, "stderr", e, "--yes", "Cost floor")
	if h.count("POST", "/v1/endpoints") != 0 {
		t.Fatal("created without confirmation")
	}
}

func TestEndpointsCreateGPUAllowListNoCounts(t *testing.T) {
	h := newHarness(t)
	h.epFixtures()
	_, e, c := h.run(epCreateArgs("--gpu", "H100:2", "--yes")...)
	if c != 2 {
		t.Fatalf("exit %d", c)
	}
	contains(t, "stderr", e, "GPU model only", "--gpu-counts")
	_, e, c = h.run(epCreateArgs("--gpu-counts", "1,3", "--yes")...)
	if c != 2 {
		t.Fatalf("bad counts exit %d", c)
	}
	contains(t, "stderr", e, "1,2,4,8")
}

func TestEndpointsCreateCheapestFitNeedsNoHardware(t *testing.T) {
	h := newHarness(t)
	h.epFixtures()
	h.epCreated()
	h.mustRun("endpoints", "create", "my-ep", "--catalog", "llama-3-8b", "--monthly-cap", "400", "--yes")
	req := body[api.CreateEndpointRequest](t, h.last("POST", "/v1/endpoints"))
	hw := req.Target.Hardware
	if hw == nil || hw.Strategy == nil || *hw.Strategy != api.EndpointHardwareCheapestFit || hw.GpuModels != nil || hw.MaxPricePerReplicaHour != nil {
		t.Fatalf("%+v", hw)
	}
}

func TestEndpointsCreateOwnImageNeed(t *testing.T) {
	h := newHarness(t)
	h.epFixtures()
	h.epCreated()
	h.mustRun("endpoints", "create", "my-ep", "--image", "ghcr.io/me/serve:1", "--port", "8000", "--min-gpu-memory", "48", "--gpu-counts", "1,2,4", "--monthly-cap", "400", "--yes")
	req := body[api.CreateEndpointRequest](t, h.last("POST", "/v1/endpoints"))
	hw := req.Target.Hardware
	if hw.MinGpuMemoryGb == nil || *hw.MinGpuMemoryGb != 48 || hw.GpuCounts == nil || len(*hw.GpuCounts) != 3 || (*hw.GpuCounts)[2] != 4 {
		t.Fatalf("%+v", hw)
	}
	if req.Target.Image == nil || req.Target.Serve == nil || req.Target.Serve.Port != 8000 {
		t.Fatalf("%+v", req.Target)
	}
}

func TestEndpointsCreateMissingFlagsNonInteractive(t *testing.T) {
	h := newHarness(t)
	h.epFixtures()
	_, e, c := h.run("endpoints", "create", "my-ep", "--yes")
	if c != 2 {
		t.Fatalf("exit %d", c)
	}
	contains(t, "stderr", e, "--catalog", "--monthly-cap")
	excludes(t, "stderr", e, "--gpu", "--max-price")
	if h.posts() != 0 {
		t.Fatal("sent something")
	}
}

func TestEndpointsCreateInteractive(t *testing.T) {
	h := newHarness(t)
	h.epFixtures()
	h.epCreated()
	h.tty = true
	// kind (2 = autoscaled model), name, catalog id, gpus, max price, spot, min, max, monthly cap, confirm
	h.stdin = "2\nmy-ep\nllama-3-8b\nH100, A100\n2.50\nn\n\n\n400\ny\n"
	out, errs := h.mustRun("endpoints", "create")
	req := body[api.CreateEndpointRequest](t, h.last("POST", "/v1/endpoints"))
	if ptrOr(req.Name, "") != "my-ep" || len(*req.Target.Hardware.GpuModels) != 2 || req.MonthlySpendCap.AmountMinor != 40000 || req.Target.Hardware.MaxPricePerReplicaHour.AmountMinor != 250 {
		t.Fatalf("%+v", req)
	}
	if req.Target.Hardware.Tier != nil && *req.Target.Hardware.Tier == api.RentalTierInterruptible {
		t.Fatal("spot chosen")
	}
	contains(t, "stderr", errs, "Cost floor", "[1]", "[2]") // defaults shown from /v1/endpoints/defaults
	contains(t, "stdout", out, "lek_SECRETVALUE123")
	if h.count("GET", "/v1/endpoints/defaults") == 0 {
		t.Fatal("defaults not read")
	}
}

func TestEndpointsCreateJSONHasSecretOnce(t *testing.T) {
	h := newHarness(t)
	h.epFixtures()
	h.epCreated()
	out, _ := h.mustRun(epCreateArgs("--yes", "-o", "json")...)
	contains(t, "json", out, `"secret": "lek_SECRETVALUE123"`)
}

func TestEndpointsCreateAckHostVisibility(t *testing.T) {
	h := newHarness(t)
	h.epFixtures()
	h.epCreated()
	h.mustRun(epCreateArgs("--yes", "--ack-host-visibility")...)
	req := body[api.CreateEndpointRequest](t, h.last("POST", "/v1/endpoints"))
	if !ptrOr(req.AcknowledgeHostVisibility, false) {
		t.Fatal("ack not sent")
	}
}

func TestEndpointsUpdateOnlyChangedFields(t *testing.T) {
	h := newHarness(t)
	h.epFixtures(epFixture(1, "chat", modeAuto))
	h.reply("GET /v1/endpoints/"+uidS(1), 200, epFixture(1, "chat", modeAuto))
	h.reply("PATCH /v1/endpoints/"+uidS(1), 200, epFixture(1, "chat", modeAuto))
	_, errs := h.mustRun("endpoints", "update", "chat", "--max", "5", "--yes")
	req := body[api.UpdateEndpointRequest](t, h.last("PATCH", "/v1/endpoints/"+uidS(1)))
	if req.Target == nil || req.Target.Scaling == nil || ptrOr(req.Target.Scaling.MaxReplicas, -1) != 5 || req.Target.Scaling.MinReplicas != nil || req.Target.Hardware != nil || req.MonthlySpendCap != nil || req.Access != nil {
		t.Fatalf("%+v", req)
	}
	contains(t, "stderr", errs, "Cost ceiling")
	_, e, c := h.run("endpoints", "update", "chat", "--yes")
	if c != 2 {
		t.Fatalf("no-op update exit %d %s", c, e)
	}
}

func TestEndpointsPauseResume(t *testing.T) {
	h := newHarness(t)
	h.epFixtures(epFixture(1, "chat", modeAuto))
	h.reply("GET /v1/endpoints/"+uidS(1), 200, epFixture(1, "chat", modeAuto))
	h.reply("POST /v1/endpoints/"+uidS(1)+"/pause", 200, epFixture(1, "chat", modeAuto))
	h.reply("POST /v1/endpoints/"+uidS(1)+"/resume", 200, epFixture(1, "chat", modeAuto))
	h.mustRun("endpoints", "pause", "chat")
	if h.count("POST", "/v1/endpoints/"+uidS(1)+"/pause") != 1 {
		t.Fatal("not paused")
	}
	_, e, c := h.run("endpoints", "resume", "chat")
	if c != 2 || h.count("POST", "/v1/endpoints/"+uidS(1)+"/resume") != 0 {
		t.Fatalf("resume without confirmation exit %d: %s", c, e)
	}
	contains(t, "stderr", e, "Cost floor", "--yes")
	_, errs := h.mustRun("endpoints", "resume", "chat", "--yes")
	contains(t, "stderr", errs, "Cost floor")
	if h.count("POST", "/v1/endpoints/"+uidS(1)+"/resume") != 1 {
		t.Fatal("not resumed")
	}
}

func TestEndpointsDelete(t *testing.T) {
	h := newHarness(t)
	h.epFixtures(epFixture(1, "chat", modeAuto))
	h.reply("DELETE /v1/endpoints/"+uidS(1), 202, epFixture(1, "chat", modeAuto))
	_, e, c := h.run("endpoints", "delete", "chat")
	if c != 2 || h.count("DELETE", "/v1/endpoints/"+uidS(1)) != 0 {
		t.Fatalf("exit %d: %s", c, e)
	}
	contains(t, "stderr", e, "--yes")
	h.tty = true
	h.stdin = "wrong\n"
	_, _, c = h.run("endpoints", "delete", "chat")
	if c == 0 || h.count("DELETE", "/v1/endpoints/"+uidS(1)) != 0 {
		t.Fatal("deleted on wrong name")
	}
	h.stdin = "chat\n"
	h.mustRun("endpoints", "delete", "chat")
	if h.count("DELETE", "/v1/endpoints/"+uidS(1)) != 1 {
		t.Fatal("not deleted")
	}
	h.tty = false
	h.mustRun("endpoints", "delete", "chat", "--yes")
}

func TestEndpointsKeys(t *testing.T) {
	h := newHarness(t)
	h.epFixtures(epFixture(1, "chat", modeAuto))
	legacy := keyFixture(11, "Direct key (legacy)")
	legacy.Kind = api.EndpointKeyKindLegacyDirect
	legacy.DirectUrl = ptr("https://old.ambi.run")
	h.reply("GET /v1/endpoints/"+uidS(1)+"/keys", 200, api.EndpointKeyPage{Items: []api.EndpointKey{keyFixture(10, "ci"), legacy}})
	out, _ := h.mustRun("endpoints", "keys", "list", "chat")
	contains(t, "keys", out, "ci", "lek_ab12cd", "legacy")
	excludes(t, "keys", out, "SECRET")

	h.reply("POST /v1/endpoints/"+uidS(1)+"/keys", 201, api.EndpointKeyCreated{Key: keyFixture(12, "bot"), Secret: "lek_NEWSECRET"})
	out, errs := h.mustRun("endpoints", "keys", "create", "chat", "--name", "bot", "--expires", "7d")
	contains(t, "create out", out, "lek_NEWSECRET")
	contains(t, "create err", errs, "only time")
	req := body[api.CreateEndpointKeyRequest](t, h.last("POST", "/v1/endpoints/"+uidS(1)+"/keys"))
	if req.Name != "bot" || req.ExpiresInDays == nil || int(*req.ExpiresInDays) != 7 || req.NoExpiry != nil {
		t.Fatalf("%+v", req)
	}
	if h.last("POST", "/v1/endpoints/"+uidS(1)+"/keys").Header.Get("Idempotency-Key") == "" {
		t.Fatal("no Idempotency-Key")
	}
	// -q prints only the secret
	out, _ = h.mustRun("endpoints", "keys", "create", "chat", "--name", "bot", "-q", "-o", "name")
	if strings.TrimSpace(out) != "lek_NEWSECRET" {
		t.Fatalf("name out %q", out)
	}
	// never expires: warns, sends no_expiry
	_, errs = h.mustRun("endpoints", "keys", "create", "chat", "--name", "forever", "--expires", "never")
	req = body[api.CreateEndpointKeyRequest](t, h.last("POST", "/v1/endpoints/"+uidS(1)+"/keys"))
	if req.ExpiresInDays != nil || !ptrOr(req.NoExpiry, false) {
		t.Fatalf("%+v", req)
	}
	contains(t, "never warning", errs, "never expires")
	_, e, c := h.run("endpoints", "keys", "create", "chat", "--name", "x", "--expires", "5d")
	if c != 2 {
		t.Fatalf("bad expiry exit %d %s", c, e)
	}
	contains(t, "bad expiry", e, "7d", "365d", "never")
}

func TestEndpointsKeysRevoke(t *testing.T) {
	h := newHarness(t)
	h.epFixtures(epFixture(1, "chat", modeAuto))
	h.reply("GET /v1/endpoints/"+uidS(1)+"/keys", 200, api.EndpointKeyPage{Items: []api.EndpointKey{keyFixture(10, "ci")}})
	h.reply("DELETE /v1/endpoints/"+uidS(1)+"/keys/"+uidS(10), 204, nil)
	_, e, c := h.run("endpoints", "keys", "revoke", "chat", "ci")
	if c != 2 || h.count("DELETE", "/v1/endpoints/"+uidS(1)+"/keys/"+uidS(10)) != 0 {
		t.Fatalf("exit %d %s", c, e)
	}
	h.mustRun("endpoints", "keys", "revoke", "chat", "ci", "--yes")
	if h.count("DELETE", "/v1/endpoints/"+uidS(1)+"/keys/"+uidS(10)) != 1 {
		t.Fatal("not revoked")
	}
	_, _, c = h.run("endpoints", "keys", "revoke", "chat", "missing", "--yes")
	if c != 4 {
		t.Fatalf("exit %d", c)
	}
}

func TestEndpointsKeysRotate(t *testing.T) {
	h := newHarness(t)
	h.epFixtures(epFixture(1, "chat", modeAuto))
	h.reply("GET /v1/endpoints/"+uidS(1)+"/keys", 200, api.EndpointKeyPage{Items: []api.EndpointKey{keyFixture(10, "ci")}})
	h.reply("POST /v1/endpoints/"+uidS(1)+"/keys/"+uidS(10)+"/rotate", 201, api.EndpointKeyRotated{NewKey: api.EndpointKeyCreated{Key: keyFixture(13, "ci"), Secret: "lek_ROTATED"}, OldKey: keyFixture(10, "ci")})
	out, errs := h.mustRun("endpoints", "keys", "rotate", "chat", "ci", "--overlap-hours", "12", "--yes")
	contains(t, "rotate", out, "lek_ROTATED")
	contains(t, "rotate", errs, "only time")
}

func TestEndpointsReplicasUsageEvents(t *testing.T) {
	h := newHarness(t)
	h.epFixtures(epFixture(1, "chat", modeAuto))
	h.reply("GET /v1/endpoints/"+uidS(1)+"/replicas", 200, api.EndpointReplicaPage{Items: []api.EndpointReplica{{
		RentalId: uid(20), RentalName: ptr("quiet-river"), Phase: "ready", Ready: true, GpuCount: 1, Tier: api.RentalTierOnDemand,
		PricePerGpuHour: money(250), Host: api.EndpointReplicaHost{GpuModel: "H100"}, RentalState: "running"}}})
	out, _ := h.mustRun("endpoints", "replicas", "chat")
	contains(t, "replicas", out, "quiet-river", "H100", "$2.50")
	h.reply("GET /v1/endpoints/"+uidS(1)+"/usage", 200, api.EndpointUsage{
		Days:           []api.EndpointUsageDay{{Date: dateOf(2026, 11, 17), Requests: 120, Errors: 3, ReplicaSeconds: 7200, GpuSeconds: 7200, Cost: money(500)}},
		SpendThisMonth: api.EndpointSpend{Estimated: money(1234), Cap: ptr(money(5000)), Month: "2026-11"}})
	out, _ = h.mustRun("endpoints", "usage", "chat", "--days", "7")
	contains(t, "usage", out, "2026-11-17", "120", "$5.00", "$12.34", "$50.00")
	if q := h.last("GET", "/v1/endpoints/"+uidS(1)+"/usage").Query["days"]; len(q) != 1 || q[0] != "7" {
		t.Fatalf("days %v", q)
	}
	h.reply("GET /v1/endpoints/"+uidS(1)+"/events", 200, api.EndpointEventPage{Items: []api.EndpointEvent{{Id: 1, At: time.Date(2026, 11, 17, 10, 0, 0, 0, time.UTC), Kind: "scaled_up", Reason: ptr("queue wait over 4s"), ReplicasBefore: ptr(1), ReplicasAfter: ptr(2)}}})
	out, _ = h.mustRun("endpoints", "events", "chat")
	contains(t, "events", out, "scaled_up", "queue wait over 4s", "1 -> 2")
}

func TestEndpointsAutoscaleOn(t *testing.T) {
	h := newHarness(t)
	ep := epFixture(1, "single", modeOff)
	h.epFixtures(ep)
	h.reply("PATCH /v1/endpoints/"+uidS(1), 200, epFixture(1, "single", modeAuto))
	_, errs := h.mustRun("endpoints", "autoscale", "on", "single", "--gpu", "H100", "--min", "1", "--max", "3", "--monthly-cap", "300", "--max-price", "2.50", "--yes")
	req := body[api.UpdateEndpointRequest](t, h.last("PATCH", "/v1/endpoints/"+uidS(1)))
	tc := req.Target
	if tc == nil || tc.Kind == nil || *tc.Kind != api.EndpointTargetAutoscale || tc.Scaling == nil || ptrOr(tc.Scaling.MaxReplicas, 0) != 3 {
		t.Fatalf("%+v", tc)
	}
	if tc.Strategy == nil || *tc.Strategy != api.EndpointSwitchKeepRental {
		t.Fatalf("strategy %+v", tc.Strategy)
	}
	if req.MonthlySpendCap == nil || req.MonthlySpendCap.AmountMinor != 30000 || tc.Hardware == nil || tc.Hardware.GpuModels == nil || len(*tc.Hardware.GpuModels) != 1 {
		t.Fatalf("%+v", req)
	}
	contains(t, "stderr", errs, "Cost floor", "Cost ceiling", "keep")
	h.mustRun("endpoints", "autoscale", "on", "single", "--replace", "--monthly-cap", "300", "--yes")
	req = body[api.UpdateEndpointRequest](t, h.last("PATCH", "/v1/endpoints/"+uidS(1)))
	if req.Target == nil || req.Target.Strategy == nil || *req.Target.Strategy != api.EndpointSwitchReplace {
		t.Fatalf("%+v", req.Target)
	}
	// prefilled from the rental's own hardware
	if req.Target.Hardware == nil || req.Target.Hardware.GpuModels == nil || (*req.Target.Hardware.GpuModels)[0] != "H100" {
		t.Fatalf("not prefilled: %+v", req.Target.Hardware)
	}
	_, _, c := h.run("endpoints", "autoscale", "on", "single", "--keep-rental", "--replace", "--monthly-cap", "300", "--yes")
	if c != 2 {
		t.Fatalf("both flags exit %d", c)
	}
	_, e, c := h.run("endpoints", "autoscale", "on", "single", "--yes")
	if c != 2 {
		t.Fatalf("missing cap exit %d", c)
	}
	contains(t, "stderr", e, "--monthly-cap")
}

func TestEndpointsAutoscaleOff(t *testing.T) {
	h := newHarness(t)
	h.epFixtures(epFixture(1, "chat", modeAuto))
	h.reply("PATCH /v1/endpoints/"+uidS(1), 200, epFixture(1, "chat", modeOff))
	h.reply("GET /v1/endpoints/"+uidS(1)+"/replicas", 200, api.EndpointReplicaPage{Items: []api.EndpointReplica{
		{RentalId: uid(20), RentalName: ptr("a"), Ready: true, Phase: "ready"}, {RentalId: uid(21), RentalName: ptr("b"), Ready: true, Phase: "ready"}}})
	_, e, c := h.run("endpoints", "autoscale", "off", "chat", "--yes")
	if c != 2 {
		t.Fatalf("exit %d %s", c, e)
	}
	contains(t, "stderr", e, "--keep-replica")
	_, errs := h.mustRun("endpoints", "autoscale", "off", "chat", "--keep-replica", uidS(21), "--yes")
	req := body[api.UpdateEndpointRequest](t, h.last("PATCH", "/v1/endpoints/"+uidS(1)))
	if req.Target == nil || req.Target.Kind == nil || *req.Target.Kind != api.EndpointTargetRental || req.Target.KeepRentalId == nil || req.Target.KeepRentalId.String() != uidS(21) {
		t.Fatalf("%+v", req)
	}
	contains(t, "stderr", errs, "drain")
	// interactive pick
	h.tty = true
	h.stdin = "1\ny\n"
	h.mustRun("endpoints", "autoscale", "off", "chat")
	req = body[api.UpdateEndpointRequest](t, h.last("PATCH", "/v1/endpoints/"+uidS(1)))
	if req.Target.KeepRentalId.String() != uidS(20) {
		t.Fatalf("picked %v", req.Target.KeepRentalId)
	}
}

func (h *harness) portFixtures() {
	h.reply("GET /v1/rentals", 200, api.RentalPage{Items: []api.Rental{rentalFixture(9, "quiet-river-7k2f", api.RentalStateRunning)}})
	h.reply("GET /v1/rentals/"+uidS(9), 200, rentalFixture(9, "quiet-river-7k2f", api.RentalStateRunning))
}

func portEndpoint(access api.EndpointAccessMode) api.Endpoint {
	e := epFixture(41, "quiet-river-7k2f", modeOff)
	e.Type, e.Access = api.EndpointTypePort, api.EndpointAccess{Mode: access}
	e.Target.RentalId = ptr(uid(9))
	return e
}

func TestEndpointsCreateRentalPort(t *testing.T) {
	h := newHarness(t)
	h.portFixtures()
	h.reply("POST /v1/endpoints", 201, api.EndpointCreated{Endpoint: portEndpoint(api.EndpointAccessKeys), InitialKey: api.EndpointKeyCreated{Key: keyFixture(8, "default"), Secret: "lek_PORTSECRET"}})
	out, errs := h.mustRun("ep", "create", "--rental", "quiet-river-7k2f", "--port", "8080", "--access", "keys", "--source", "203.0.113.0/24", "--yes")
	c := h.last("POST", "/v1/endpoints")
	req := body[api.CreateEndpointRequest](t, c)
	if req.Target.Kind != api.EndpointTargetRental || req.Target.RentalId == nil || req.Target.RentalId.String() != uidS(9) || ptrOr(req.Target.Port, 0) != 8080 {
		t.Fatalf("%+v", req.Target)
	}
	if req.Access == nil || req.Access.Mode != api.EndpointAccessKeys || req.Access.Sources == nil || (*req.Access.Sources)[0] != "203.0.113.0/24" {
		t.Fatalf("access %+v", req.Access)
	}
	if req.InitialKey == nil || req.Name != nil || c.Header.Get("Idempotency-Key") == "" {
		t.Fatalf("%+v", req)
	}
	if h.count("POST", "/v1/endpoints/estimate") != 0 {
		t.Fatal("a port costs no GPU: no estimate")
	}
	contains(t, "stdout", out, "lek_PORTSECRET", "https://quiet-river-7k2f.ambi.run")
	contains(t, "stderr", errs, "only time")
}

func TestEndpointsCreateRentalPortPublicNeedsConfirmation(t *testing.T) {
	h := newHarness(t)
	h.portFixtures()
	h.reply("POST /v1/endpoints", 201, api.EndpointCreated{Endpoint: portEndpoint(api.EndpointAccessPublic)})
	_, e, c := h.run("ep", "create", "--rental", "quiet-river-7k2f", "--port", "8080")
	if c != 2 || h.count("POST", "/v1/endpoints") != 0 {
		t.Fatalf("exit %d: %s", c, e)
	}
	contains(t, "stderr", e, "internet")
	_, errs := h.mustRun("ep", "create", "--rental", "quiet-river-7k2f", "--port", "8080", "--yes")
	req := body[api.CreateEndpointRequest](t, h.last("POST", "/v1/endpoints"))
	if req.Access != nil || req.InitialKey != nil {
		t.Fatalf("public default should send no access or key: %+v", req)
	}
	contains(t, "stderr", errs, "public", "--access keys")
	_, _, c = h.run("ep", "create", "--rental", "quiet-river-7k2f", "--yes")
	if c != 2 {
		t.Fatalf("missing port exit %d", c)
	}
	_, _, c = h.run("ep", "create", "--rental", "x", "--catalog", "m", "--port", "1", "--yes")
	if c != 2 {
		t.Fatalf("mixed exit %d", c)
	}
}

func TestEndpointsCreateInteractivePort(t *testing.T) {
	h := newHarness(t)
	h.portFixtures()
	h.reply("POST /v1/endpoints", 201, api.EndpointCreated{Endpoint: portEndpoint(api.EndpointAccessKeys), InitialKey: api.EndpointKeyCreated{Key: keyFixture(8, "default"), Secret: "lek_X"}})
	h.tty = true
	// kind 1 (a port), rental 1, port, access (default keys), confirm
	h.stdin = "1\n1\n8080\n\ny\n"
	h.mustRun("endpoints", "create")
	req := body[api.CreateEndpointRequest](t, h.last("POST", "/v1/endpoints"))
	if ptrOr(req.Target.Port, 0) != 8080 || req.Access == nil || req.Access.Mode != api.EndpointAccessKeys {
		t.Fatalf("%+v", req)
	}
}

func TestEndpointsUpdateAccess(t *testing.T) {
	h := newHarness(t)
	ep := portEndpoint(api.EndpointAccessPublic)
	h.epFixtures(ep)
	h.reply("PATCH /v1/endpoints/"+uidS(41), 200, ep)
	h.mustRun("ep", "update", "quiet-river-7k2f", "--access", "keys")
	req := body[api.UpdateEndpointRequest](t, h.last("PATCH", "/v1/endpoints/"+uidS(41)))
	if req.Access == nil || req.Access.Mode != api.EndpointAccessKeys || req.Target != nil {
		t.Fatalf("%+v", req)
	}
	// switching to public from keys asks first
	ek := portEndpoint(api.EndpointAccessKeys)
	h.epFixtures(ek)
	_, e, c := h.run("ep", "update", "quiet-river-7k2f", "--access", "public")
	if c != 2 {
		t.Fatalf("exit %d %s", c, e)
	}
	h.mustRun("ep", "update", "quiet-river-7k2f", "--access", "public", "--source", "10.0.0.0/8", "--yes")
	req = body[api.UpdateEndpointRequest](t, h.last("PATCH", "/v1/endpoints/"+uidS(41)))
	if req.Access.Mode != api.EndpointAccessPublic || req.Access.Sources == nil || (*req.Access.Sources)[0] != "10.0.0.0/8" {
		t.Fatalf("%+v", req.Access)
	}
	_, _, c = h.run("ep", "update", "quiet-river-7k2f", "--access", "bogus")
	if c != 2 {
		t.Fatalf("bogus exit %d", c)
	}
}

func TestRentalIngressIsDeprecatedAliasOfEndpoints(t *testing.T) {
	h := newHarness(t)
	h.portFixtures()
	ep := portEndpoint(api.EndpointAccessPublic)
	h.epFixtures(ep, epFixture(1, "other", modeAuto))
	h.reply("POST /v1/endpoints", 201, api.EndpointCreated{Endpoint: ep})
	h.reply("DELETE /v1/endpoints/"+uidS(41), 202, ep)
	rule := api.IngressRule{Id: uid(80), Port: 8000, Protocol: "http", Hostname: "quiet-river-7k2f.ambi.run", Url: "https://quiet-river-7k2f.ambi.run", Status: "active", Sources: []string{}}
	h.reply("GET /v1/rentals/"+uidS(9)+"/network", 200, api.RentalNetwork{Ingress: api.RentalIngress{Available: true, Rules: []api.IngressRule{rule}}})

	out, errs := h.mustRun("rental", "ingress", "list", "quiet-river-7k2f", "-o", "name")
	contains(t, "list", errs, "Deprecated", "legion endpoints list --rental quiet-river-7k2f")
	if strings.TrimSpace(out) != "quiet-river-7k2f" {
		t.Fatalf("list %q", out)
	}
	_, errs = h.mustRun("rental", "ingress", "add", "quiet-river-7k2f", "--port", "8000", "--source", "203.0.113.0/24", "--yes")
	contains(t, "add", errs, "Deprecated", "legion endpoints create --rental quiet-river-7k2f --port 8000")
	req := body[api.CreateEndpointRequest](t, h.last("POST", "/v1/endpoints"))
	if req.Target.Kind != api.EndpointTargetRental || ptrOr(req.Target.Port, 0) != 8000 || req.Access == nil || req.Access.Mode != api.EndpointAccessPublic {
		t.Fatalf("%+v", req)
	}
	if h.count("POST", "/v1/rentals/"+uidS(9)+"/ingress-rules") != 0 {
		t.Fatal("the old route was called")
	}
	_, e, c := h.run("rental", "ingress", "remove", "quiet-river-7k2f", uidS(80))
	if c != 2 || h.count("DELETE", "/v1/endpoints/"+uidS(41)) != 0 {
		t.Fatalf("remove without --yes: %d %s", c, e)
	}
	_, errs = h.mustRun("rental", "ingress", "remove", "quiet-river-7k2f", uidS(80), "--yes")
	contains(t, "remove", errs, "legion endpoints delete quiet-river-7k2f")
	if h.count("DELETE", "/v1/endpoints/"+uidS(41)) != 1 {
		t.Fatal("endpoint not deleted")
	}
	_, _, c = h.run("ports", "list")
	if c != 2 {
		t.Fatalf("ports should be gone, exit %d", c)
	}
}

var _ = http.StatusOK

func dateOf(y int, m time.Month, d int) openapi_types.Date {
	return openapi_types.Date{Time: time.Date(y, m, d, 0, 0, 0, 0, time.UTC)}
}
