package cli

import (
	"net/http"
	"strings"
	"testing"

	"github.com/AmbientCrypto/legion-controller/api"
)

func (h *harness) qwenCatalog() {
	h.on("GET /v1/catalog", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, api.Catalog{Entries: []api.CatalogEntry{
			{Id: "qwen", Name: "Qwen", Kind: "model", Reference: "reg/qwen@sha256:bb", Compatible: ptr(true), MinGpus: ptr(2),
				Serve: &api.CatalogServe{Port: 8000, Protocol: "openai", Model: "Qwen/Qwen3.5-4B"}},
		}})
	})
}

func TestRentalCreateSendsConfiguration(t *testing.T) {
	h := newHarness(t)
	h.stdFixtures()
	h.rentalLifecycle("quiet-river-7k2f", 9)
	h.stdin = "hf_SECRET\n"
	out, errs := h.mustRun(createArgs("--yes", "--async", "--env", "MODE=fast", "--secret-env", "HF_TOKEN", "--command", "python", "--arg", "serve.py", "-o", "json")...)
	req := body[api.CreateRentalRequest](t, h.last("POST", "/v1/rentals"))
	if req.Env == nil || len(*req.Env) != 2 {
		t.Fatalf("%+v", req.Env)
	}
	e := *req.Env
	if e[0].Name != "MODE" || ptrOr(e[0].Value, "") != "fast" || ptrOr(e[0].Secret, false) {
		t.Fatalf("%+v", e[0])
	}
	if e[1].Name != "HF_TOKEN" || ptrOr(e[1].Value, "") != "hf_SECRET" || !ptrOr(e[1].Secret, false) {
		t.Fatalf("%+v", e[1])
	}
	if req.Command == nil || (*req.Command)[0] != "python" || req.Args == nil || (*req.Args)[0] != "serve.py" {
		t.Fatalf("%+v %+v", req.Command, req.Args)
	}
	if strings.Contains(out+errs, "hf_SECRET") {
		t.Fatalf("secret printed: %s %s", out, errs)
	}
}

func TestRentalCreatePreviewHidesSecret(t *testing.T) {
	h := newHarness(t)
	h.stdFixtures()
	h.stdin = "hf_SECRET\n"
	out, errs := h.mustRun(createArgs("--dry-run", "--env", "MODE=fast", "--secret-env", "HF_TOKEN")...)
	contains(t, "preview", out, "MODE = fast", "HF_TOKEN = (secret, hidden)", "provider's machine")
	if strings.Contains(out+errs, "hf_SECRET") {
		t.Fatalf("secret printed: %s %s", out, errs)
	}
	contains(t, "dry run hint", errs, "--secret-env HF_TOKEN")
}

func TestRentalCreateModelSettings(t *testing.T) {
	h := newHarness(t)
	h.stdFixtures()
	h.rentalLifecycle("quiet-river-7k2f", 9)
	h.qwenCatalog()
	h.mustRun("rental", "create", "--offer", uidS(50), "--catalog", "qwen", "--yes", "--async", "--setting", "max_model_len=32768", "--setting", "enable_prefix_caching=true", "--setting", "tool_call_parser=hermes")
	req := body[api.CreateRentalRequest](t, h.last("POST", "/v1/rentals"))
	if req.ModelSettings == nil || len(*req.ModelSettings) != 3 {
		t.Fatalf("%+v", req.ModelSettings)
	}
	if n, _ := (*req.ModelSettings)["max_model_len"].AsModelSettingValue0(); n != 32768 {
		t.Fatalf("%+v", *req.ModelSettings)
	}
	if b, _ := (*req.ModelSettings)["enable_prefix_caching"].AsModelSettingValue1(); !b {
		t.Fatalf("%+v", *req.ModelSettings)
	}
	if s, _ := (*req.ModelSettings)["tool_call_parser"].AsModelSettingValue2(); s != "hermes" {
		t.Fatalf("%+v", *req.ModelSettings)
	}
}

func TestRentalCreateConfigRefusals(t *testing.T) {
	h := newHarness(t)
	h.stdFixtures()
	h.qwenCatalog()
	_, errs, code := h.run(createArgs("--yes", "--setting", "max_model_len=1")...)
	if code != 2 {
		t.Fatalf("setting on own image: exit %d", code)
	}
	contains(t, "stderr", errs, "--catalog")
	_, errs, code = h.run("rental", "create", "--offer", uidS(50), "--catalog", "qwen", "--yes", "--command", "sh")
	if code != 2 {
		t.Fatalf("command on catalog: exit %d", code)
	}
	contains(t, "stderr", errs, "own images")
	if h.posts() != 0 {
		t.Fatal("POST despite refusal")
	}
	_, _, code = h.run(createArgs("--yes", "--env", "LEGION_X=1")...)
	if code != 2 || h.posts() != 0 {
		t.Fatalf("reserved name: exit %d", code)
	}
}

func TestServicesCreateConfiguration(t *testing.T) {
	h := newHarness(t)
	h.svcFixtures()
	h.svcCreated()
	h.stdin = "tok_SECRET\n"
	out, errs := h.mustRun("services", "create", "my-svc", "--image", "ghcr.io/me/serve:1", "--port", "8000", "--monthly-cap", "400", "--yes",
		"--env", "A=1", "--secret-env", "TOK", "--command", "serve", "--arg", "--fast")
	tg := body[api.CreateServiceRequest](t, h.last("POST", "/v1/services")).Target
	if tg.Env == nil || len(*tg.Env) != 2 || !ptrOr((*tg.Env)[1].Secret, false) || ptrOr((*tg.Env)[1].Value, "") != "tok_SECRET" {
		t.Fatalf("%+v", tg.Env)
	}
	if tg.Command == nil || (*tg.Command)[0] != "serve" || (*tg.Args)[0] != "--fast" {
		t.Fatalf("%+v", tg)
	}
	if strings.Contains(out+errs, "tok_SECRET") && !strings.Contains(out, "lek_") {
		t.Fatalf("secret printed")
	}
	if strings.Contains(errs, "tok_SECRET") {
		t.Fatalf("secret printed: %s", errs)
	}

	h2 := newHarness(t)
	h2.svcFixtures()
	h2.svcCreated()
	h2.mustRun(svcCreateArgs("--yes", "--setting", "max_model_len=8192")...)
	tg = body[api.CreateServiceRequest](t, h2.last("POST", "/v1/services")).Target
	if tg.ModelSettings == nil || len(*tg.ModelSettings) != 1 {
		t.Fatalf("%+v", tg.ModelSettings)
	}
	_, _, code := h2.run(svcCreateArgs("--yes", "--command", "x")...)
	if code != 2 {
		t.Fatalf("command on a catalog model: exit %d", code)
	}
}

func TestServicesUpdateEnvMergesAndKeepsSecrets(t *testing.T) {
	h := newHarness(t)
	e := svcFixture(1, "chat", modeAuto)
	e.Target.Template.Env = &[]api.EnvVar{{Name: "KEEP", Value: ptr("1")}, {Name: "DROP", Value: ptr("2")}, {Name: "TOK", Secret: true}}
	h.svcFixtures(e)
	h.reply("GET /v1/services/"+uidS(1), 200, e)
	h.reply("PATCH /v1/services/"+uidS(1), 200, e)
	_, errs := h.mustRun("services", "update", "chat", "--env", "NEW=3", "--unset-env", "DROP", "--yes")
	req := body[api.UpdateServiceRequest](t, h.last("PATCH", "/v1/services/"+uidS(1)))
	if req.Target == nil || req.Target.Env == nil {
		t.Fatalf("%+v", req)
	}
	got := map[string]api.EnvVarInput{}
	for _, v := range *req.Target.Env {
		got[v.Name] = v
	}
	if len(got) != 3 || got["KEEP"].Value == nil || got["NEW"].Value == nil || got["DROP"].Name != "" {
		t.Fatalf("%+v", got)
	}
	if tok := got["TOK"]; !ptrOr(tok.Secret, false) || tok.Value != nil {
		t.Fatalf("stored secret must go back without a value: %+v", tok)
	}
	contains(t, "stderr", errs, "replaced one at a time")
}

func TestServicesUpdateClearAndRefuse(t *testing.T) {
	h := newHarness(t)
	e := svcFixture(1, "chat", modeAuto)
	h.svcFixtures(e, svcFixture(2, "port", modeOff))
	h.reply("GET /v1/services/"+uidS(1), 200, e)
	h.reply("PATCH /v1/services/"+uidS(1), 200, e)
	h.mustRun("services", "update", "chat", "--clear-settings", "--yes")
	req := body[api.UpdateServiceRequest](t, h.last("PATCH", "/v1/services/"+uidS(1)))
	if req.Target == nil || req.Target.ModelSettings == nil || len(*req.Target.ModelSettings) != 0 {
		t.Fatalf("%+v", req.Target)
	}
	h.reply("GET /v1/services/"+uidS(2), 200, svcFixture(2, "port", modeOff))
	_, _, code := h.run("services", "update", "port", "--env", "A=1", "--yes")
	if code != 2 {
		t.Fatalf("env on a port service: exit %d", code)
	}
}

func TestRentalEnvCommands(t *testing.T) {
	h := newHarness(t)
	rs := h.rentals()
	r := rs[0]
	r.Env = &[]api.EnvVar{{Name: "MODE", Value: ptr("slow")}, {Name: "HF_TOKEN", Secret: true}}
	h.reply("GET /v1/rentals/"+r.Id.String(), 200, r)
	h.reply("PATCH /v1/rentals/"+r.Id.String(), 200, r)
	out, _ := h.mustRun("rental", "env", "list", r.Name)
	contains(t, "list", out, "MODE", "slow", "HF_TOKEN", "set, hidden")
	h.stdin = "new_SECRET\n"
	out, errs := h.mustRun("rental", "env", "set", r.Name, "--env", "MODE=fast", "--secret-env", "HF_TOKEN", "--yes")
	req := body[api.UpdateRentalRequest](t, h.last("PATCH", "/v1/rentals/"+r.Id.String()))
	if req.Env == nil || len(*req.Env) != 2 || ptrOr((*req.Env)[0].Value, "") == "" && ptrOr((*req.Env)[1].Value, "") != "new_SECRET" {
		t.Fatalf("%+v", req.Env)
	}
	if strings.Contains(out+errs, "new_SECRET") {
		t.Fatal("secret printed")
	}
	h.mustRun("rental", "env", "unset", r.Name, "MODE", "--yes")
	req = body[api.UpdateRentalRequest](t, h.last("PATCH", "/v1/rentals/"+r.Id.String()))
	if req.Env == nil || len(*req.Env) != 1 || (*req.Env)[0].Name != "HF_TOKEN" || (*req.Env)[0].Value != nil || !ptrOr((*req.Env)[0].Secret, false) {
		t.Fatalf("unset must keep the other secret without a value: %+v", req.Env)
	}
	_, errs, code := h.run("rental", "env", "set", r.Name, "--env", "MODE=x")
	if code != 2 {
		t.Fatalf("no --yes non-tty: exit %d %s", code, errs)
	}
}

func TestRentalDetailShowsEnvNamesNeverSecrets(t *testing.T) {
	h := newHarness(t)
	rs := h.rentals()
	r := rs[0]
	r.Env = &[]api.EnvVar{{Name: "MODE", Value: ptr("fast")}, {Name: "HF_TOKEN", Secret: true}}
	h.reply("GET /v1/rentals/"+r.Id.String(), 200, r)
	out, _ := h.mustRun("rental", "get", r.Name)
	contains(t, "detail", out, "MODE = fast", "HF_TOKEN = (secret, set, hidden)")
}
