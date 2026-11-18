package cli

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/AmbientCrypto/legion-controller/api"
)

const secretToken = "ghp_SUPERSECRETvalue0123456789"

func credFixture(n int, host, prefix string) api.RegistryCredential {
	by := uid(2)
	used := time.Date(2026, 11, 17, 9, 0, 0, 0, time.UTC)
	name := "acme on ghcr"
	return api.RegistryCredential{
		Id: uid(n), Name: &name, Host: host, PathPrefix: prefix, Username: "acme-legion",
		TokenRealm: "https://" + host + "/token", CreatedBy: &by, CreatedAt: time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC), LastCheckedAt: used, LastUsedAt: &used,
		LastCheckResult: "ok",
	}
}

func TestRegistryAddFromStdinSendsSecretOnlyInTheBody(t *testing.T) {
	h := newHarness(t)
	h.reply("POST /v1/registry-credentials", 201, credFixture(70, "ghcr.io", "acme"))
	h.stdin = secretToken + "\n"
	out, errs := h.mustRun("registry", "add", "--image", "ghcr.io/acme/model:v3", "--username", "acme-legion", "--name", "acme on ghcr", "--token-stdin")
	excludes(t, "stdout", out, secretToken)
	excludes(t, "stderr", errs, secretToken)
	contains(t, "info", errs, "ghcr.io", "acme", "Uses")
	var body map[string]any
	if err := json.Unmarshal(h.last("POST", "/v1/registry-credentials").Body, &body); err != nil {
		t.Fatal(err)
	}
	if body["image"] != "ghcr.io/acme/model:v3" || body["username"] != "acme-legion" || body["secret"] != secretToken || body["name"] != "acme on ghcr" {
		t.Fatalf("body %v", body)
	}
	if _, ok := body["path_prefix"]; ok {
		t.Fatal("path_prefix sent without --path-prefix")
	}
}

func TestRegistryAddPathPrefixEmptyMeansWholeHost(t *testing.T) {
	h := newHarness(t)
	h.reply("POST /v1/registry-credentials", 201, credFixture(70, "ghcr.io", ""))
	h.stdin = secretToken
	h.mustRun("registry", "add", "--image", "ghcr.io/acme/model", "--username", "u", "--token-stdin", "--path-prefix", "")
	var body map[string]any
	_ = json.Unmarshal(h.last("POST", "/v1/registry-credentials").Body, &body)
	if p, ok := body["path_prefix"]; !ok || p != "" {
		t.Fatalf("an explicit empty --path-prefix must be sent: %v", body)
	}
}

func TestRegistryAddPromptsHiddenOnATerminal(t *testing.T) {
	h := newHarness(t)
	h.reply("POST /v1/registry-credentials", 201, credFixture(70, "nvcr.io", "acme"))
	h.tty, h.secret = true, secretToken
	h.stdin = "acme-user\n"
	out, errs := h.mustRun("registry", "add", "--image", "nvcr.io/acme/model:1")
	excludes(t, "output", out+errs, secretToken)
	var body map[string]any
	_ = json.Unmarshal(h.last("POST", "/v1/registry-credentials").Body, &body)
	if body["secret"] != secretToken || body["username"] != "acme-user" {
		t.Fatalf("body %v", body)
	}
}

func TestRegistryAddNeedsTokenSourceAndImage(t *testing.T) {
	h := newHarness(t)
	_, errs, code := h.run("registry", "add", "--image", "ghcr.io/acme/model", "--username", "u")
	if code != 2 {
		t.Fatalf("exit %d", code)
	}
	contains(t, "no token", errs, "--token-stdin")
	_, errs, code = h.run("registry", "add", "--username", "u", "--token-stdin")
	if code != 2 {
		t.Fatalf("exit %d", code)
	}
	contains(t, "no image", errs, "--image")
	if h.posts() != 0 {
		t.Fatal("sent a request")
	}
}

func TestRegistryAddEmptyStdinIsRefused(t *testing.T) {
	h := newHarness(t)
	h.stdin = "  \n"
	_, _, code := h.run("registry", "add", "--image", "ghcr.io/acme/model", "--username", "u", "--token-stdin")
	if code != 2 || h.posts() != 0 {
		t.Fatalf("exit %d posts %d", code, h.posts())
	}
}

func TestRegistryAddExistsPointsAtReplace(t *testing.T) {
	h := newHarness(t)
	h.on("POST /v1/registry-credentials", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(409)
		json.NewEncoder(w).Encode(map[string]any{"status": 409, "title": "exists", "code": "registry_credential_exists", "detail": "exists", "resource_id": uidS(70)})
	})
	h.stdin = secretToken
	_, errs, code := h.run("registry", "add", "--image", "ghcr.io/acme/model", "--username", "u", "--token-stdin")
	if code == 0 {
		t.Fatal("succeeded")
	}
	contains(t, "exists", errs, uidS(70), "legion registry replace")
	excludes(t, "exists", errs, secretToken)
}

func TestRegistryProblemWords(t *testing.T) {
	cases := []struct {
		status int
		code   string
		wants  []string
	}{
		{422, "registry_ecr_unsupported", []string{"ECR", "12 hours", "mirror"}},
		{422, "registry_auth_unsupported", []string{"token"}},
		{422, "registry_host_not_allowed", []string{"registry"}},
		{422, "registry_realm_not_allowed", []string{"token service"}},
		{422, "registry_credential_rejected", []string{"refused", "read"}},
		{400, "image_outside_prefix", []string{"--path-prefix"}},
		{409, "registry_credential_limit", []string{"20", "legion registry remove"}},
		{422, "registry_credential_not_found", []string{"legion registry list"}},
		{422, "registry_credential_host_mismatch", []string{"host"}},
		{422, "image_private", []string{"legion registry add --image"}},
	}
	for _, c := range cases {
		h := newHarness(t)
		h.fail("POST /v1/registry-credentials", c.status, c.code, "registry said no")
		h.stdin = secretToken
		_, errs, code := h.run("registry", "add", "--image", "ghcr.io/acme/model", "--username", "u", "--token-stdin")
		if code == 0 {
			t.Fatalf("%s: succeeded", c.code)
		}
		contains(t, c.code, errs, c.wants...)
		excludes(t, c.code, errs, "—", "–")
	}
}

func TestRegistryList(t *testing.T) {
	h := newHarness(t)
	h.reply("GET /v1/registry-credentials", 200, api.RegistryCredentialList{Items: []api.RegistryCredential{credFixture(70, "ghcr.io", "acme"), credFixture(71, "docker.io", "")}})
	out, _ := h.mustRun("registry", "list")
	contains(t, "table", out, "NAME", "HOST", "PREFIX", "USERNAME", "LAST USED", "CHECK", "ghcr.io", "acme", "docker.io", "acme-legion", "ok")
	excludes(t, "table", out, "token_realm", "secret")
	j, _ := h.mustRun("registry", "list", "-o", "json")
	var items []api.RegistryCredential
	if err := json.Unmarshal([]byte(j), &items); err != nil || len(items) != 2 {
		t.Fatalf("%v %s", err, j)
	}
	n, _ := h.mustRun("registry", "list", "-o", "name")
	if !strings.Contains(n, uidS(70)) {
		t.Fatalf("names should be ids: %q", n)
	}
}

func TestRegistryListEmptySaysHowToAdd(t *testing.T) {
	h := newHarness(t)
	h.reply("GET /v1/registry-credentials", 200, api.RegistryCredentialList{Items: []api.RegistryCredential{}})
	_, errs := h.mustRun("registry", "list")
	contains(t, "empty", errs, "legion registry add --image")
}

func TestRegistryRemoveConfirmsAndDeletes(t *testing.T) {
	h := newHarness(t)
	route := "/v1/registry-credentials/" + uidS(70)
	h.reply("GET "+route, 200, credFixture(70, "ghcr.io", "acme"))
	h.reply("DELETE "+route, 204, nil)
	_, errs, code := h.run("registry", "remove", uidS(70))
	if code != 2 || h.count("DELETE", route) != 0 {
		t.Fatalf("deleted without confirmation: %d", code)
	}
	contains(t, "refuse", errs, "--yes")
	h.mustRun("registry", "remove", uidS(70), "--yes")
	if h.count("DELETE", route) != 1 {
		t.Fatal("not deleted")
	}
}

func TestRegistryRemoveInUseNamesTheRentals(t *testing.T) {
	h := newHarness(t)
	route := "/v1/registry-credentials/" + uidS(70)
	h.on("DELETE "+route, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(409)
		json.NewEncoder(w).Encode(map[string]any{"status": 409, "title": "in use", "code": "registry_credential_in_use", "detail": "still used",
			"registry_credential_rentals": []map[string]any{{"id": uidS(5), "name": "quiet-river-7k2f", "state": "running"}, {"id": uidS(6), "name": "calm-lake-1x9z", "state": "starting"}}})
	})
	_, errs, code := h.run("registry", "remove", uidS(70), "--yes")
	if code != 5 && code == 0 {
		t.Fatalf("exit %d", code)
	}
	contains(t, "in use", errs, "quiet-river-7k2f", "running", "calm-lake-1x9z", "legion rental stop")
}

func TestRegistryReplaceSendsSecretAndImage(t *testing.T) {
	h := newHarness(t)
	route := "/v1/registry-credentials/" + uidS(70)
	h.reply("PATCH "+route, 200, credFixture(70, "ghcr.io", "acme"))
	h.stdin = secretToken
	out, errs := h.mustRun("registry", "replace", uidS(70), "--image", "ghcr.io/acme/model:v3", "--token-stdin", "--username", "new-user")
	excludes(t, "output", out+errs, secretToken)
	var body map[string]any
	_ = json.Unmarshal(h.last("PATCH", route).Body, &body)
	if body["secret"] != secretToken || body["image"] != "ghcr.io/acme/model:v3" || body["username"] != "new-user" {
		t.Fatalf("body %v", body)
	}
}

func TestRegistryReplaceRenameOnly(t *testing.T) {
	h := newHarness(t)
	route := "/v1/registry-credentials/" + uidS(70)
	h.reply("PATCH "+route, 200, credFixture(70, "ghcr.io", "acme"))
	h.mustRun("registry", "replace", uidS(70), "--name", "team ghcr")
	var body map[string]any
	_ = json.Unmarshal(h.last("PATCH", route).Body, &body)
	if body["name"] != "team ghcr" || len(body) != 1 {
		t.Fatalf("body %v", body)
	}
	_, _, code := h.run("registry", "replace", uidS(70))
	if code != 2 {
		t.Fatalf("nothing to change must be a usage error: %d", code)
	}
	_, _, code = h.run("registry", "replace", uidS(70), "--token-stdin")
	if code != 2 {
		t.Fatalf("a secret needs --image: %d", code)
	}
}

func TestRentalCreateRegistryCredentialFlag(t *testing.T) {
	h := newHarness(t)
	h.stdFixtures()
	h.rentalLifecycle("quiet-river-7k2f", 9)
	h.mustRun(createArgs("--yes", "-o", "name", "--registry-credential", uidS(70))...)
	req := body[api.CreateRentalRequest](t, h.last("POST", "/v1/rentals"))
	if req.RegistryCredentialId == nil || req.RegistryCredentialId.String() != uidS(70) {
		t.Fatalf("%+v", req.RegistryCredentialId)
	}
	h2 := newHarness(t)
	h2.stdFixtures()
	_, _, code := h2.run(createArgs("--yes", "--registry-credential", "nope")...)
	if code != 2 || h2.posts() != 0 {
		t.Fatalf("a bad id is a usage error: %d", code)
	}
}

func TestRentalCreateWithoutFlagSendsNoCredential(t *testing.T) {
	h := newHarness(t)
	h.stdFixtures()
	h.rentalLifecycle("quiet-river-7k2f", 9)
	h.mustRun(createArgs("--yes", "-o", "name")...)
	if strings.Contains(string(h.last("POST", "/v1/rentals").Body), "registry_credential_id") {
		t.Fatal("sent a credential nobody chose")
	}
}
