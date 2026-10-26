package client

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHeadersAndDebug(t *testing.T) {
	var gotAuth, gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth, gotUA = r.Header.Get("Authorization"), r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"org":{"id":"11111111-1111-1111-1111-111111111111","name":"Acme"},"permissions":[],"role":"owner","user":{"email":"a@b.co","id":"11111111-1111-1111-1111-111111111111","name":"Ann"}}`))
	}))
	defer srv.Close()
	var dbg bytes.Buffer
	c, err := New(Options{BaseURL: srv.URL, Token: "lgn_supersecret", Version: "1.2.3", Debug: &dbg})
	if err != nil {
		t.Fatal(err)
	}
	r, err := c.GetMeWithResponse(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if e := Check(r.HTTPResponse, r.Body); e != nil {
		t.Fatal(e)
	}
	if gotAuth != "Bearer lgn_supersecret" {
		t.Fatalf("auth %q", gotAuth)
	}
	if gotUA != "legion-cli/1.2.3" {
		t.Fatalf("ua %q", gotUA)
	}
	out := dbg.String()
	if !strings.Contains(out, "GET /v1/me 200") {
		t.Fatalf("debug line missing: %q", out)
	}
	if strings.Contains(out, "lgn_supersecret") || strings.Contains(strings.ToLower(out), "authorization") {
		t.Fatalf("debug leaked credentials: %q", out)
	}
}

func TestNoTokenNoHeader(t *testing.T) {
	var has bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, has = r.Header["Authorization"]
		w.WriteHeader(204)
	}))
	defer srv.Close()
	c, _ := New(Options{BaseURL: srv.URL})
	if _, err := c.GetMeWithResponse(context.Background()); err != nil {
		t.Fatal(err)
	}
	if has {
		t.Fatal("Authorization sent without a token")
	}
}

func TestCheckProblem(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(409)
		w.Write([]byte(`{"type":"about:blank","title":"Price changed","status":409,"code":"price_changed","detail":"host now costs more","current_price_per_gpu_hour":{"amount_minor":300,"currency":"USD"}}`))
	}))
	defer srv.Close()
	c, _ := New(Options{BaseURL: srv.URL})
	r, _ := c.GetMeWithResponse(context.Background())
	err := Check(r.HTTPResponse, r.Body)
	var e *Error
	if !errors.As(err, &e) {
		t.Fatalf("not *Error: %v", err)
	}
	if e.Status != 409 || e.Code != "price_changed" || e.Detail != "host now costs more" || e.CurrentPrice == nil || e.CurrentPrice.AmountMinor != 300 {
		t.Fatalf("%+v", e)
	}
	if !strings.Contains(e.Error(), "host now costs more") {
		t.Fatal(e.Error())
	}
}

func TestCheckNonProblemBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(502)
		w.Write([]byte("<html>bad gateway</html>"))
	}))
	defer srv.Close()
	c, _ := New(Options{BaseURL: srv.URL})
	r, _ := c.GetMeWithResponse(context.Background())
	var e *Error
	if err := Check(r.HTTPResponse, r.Body); !errors.As(err, &e) || e.Status != 502 || e.Code != "" {
		t.Fatalf("%v", err)
	}
}

func TestTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { time.Sleep(300 * time.Millisecond) }))
	defer srv.Close()
	c, _ := New(Options{BaseURL: srv.URL, Timeout: 30 * time.Millisecond})
	_, err := c.GetMeWithResponse(context.Background())
	if err == nil {
		t.Fatal("want timeout")
	}
	if !strings.Contains(NetworkError(err, srv.URL).Error(), "cannot reach") {
		t.Fatal(NetworkError(err, srv.URL))
	}
}

func TestExitCodes(t *testing.T) {
	for _, tc := range []struct {
		status int
		code   string
		want   int
	}{
		{500, "", 1}, {401, "unauthorized", 3}, {401, "", 3}, {403, "api_key_scope", 3}, {404, "not_found", 4},
		{409, "price_changed", 5}, {409, "gpus_unavailable", 5}, {402, "insufficient_credit", 6}, {400, "invalid_request", 1},
	} {
		if got := (&Error{Status: tc.status, Code: tc.code}).ExitCode(); got != tc.want {
			t.Errorf("%d %s: got %d want %d", tc.status, tc.code, got, tc.want)
		}
	}
}
