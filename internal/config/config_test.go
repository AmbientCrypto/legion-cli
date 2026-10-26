package config

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestDir(t *testing.T) {
	d, err := Dir(env(map[string]string{"LEGION_CONFIG_DIR": "/x/y"}))
	if err != nil || d != "/x/y" {
		t.Fatalf("Dir = %q, %v", d, err)
	}
	d, err = Dir(env(map[string]string{"HOME": "/h", "XDG_CONFIG_HOME": "/h/.cfg"}))
	if err != nil || !strings.HasSuffix(d, "legion") {
		t.Fatalf("default Dir = %q, %v", d, err)
	}
}

func TestResolvePrecedence(t *testing.T) {
	f := &File{CurrentProfile: "work", Profiles: map[string]*Profile{
		"work":  {APIURL: "https://work.example", Output: "wide"},
		"other": {APIURL: "https://other.example"},
	}}
	c := &Credentials{Profiles: map[string]*Cred{"work": {APIKey: "lgn_work"}, "other": {APIKey: "lgn_other"}}}

	tests := []struct {
		name                           string
		in                             Inputs
		profile, url, key, out, srcURL string
	}{
		{"defaults", Inputs{Getenv: env(nil)}, "work", "https://work.example", "lgn_work", "wide", "profile"},
		{"env profile", Inputs{Getenv: env(map[string]string{"LEGION_PROFILE": "other"})}, "other", "https://other.example", "lgn_other", "table", "profile"},
		{"flag profile beats env", Inputs{Profile: "work", Getenv: env(map[string]string{"LEGION_PROFILE": "other"})}, "work", "https://work.example", "lgn_work", "wide", "profile"},
		{"env url beats profile", Inputs{Getenv: env(map[string]string{"LEGION_API_URL": "https://env.example", "LEGION_OUTPUT": "json", "LEGION_API_KEY": "lgn_env"})}, "work", "https://env.example", "lgn_env", "json", "env"},
		{"flag beats env", Inputs{APIURL: "https://flag.example", Output: "yaml", Getenv: env(map[string]string{"LEGION_API_URL": "https://env.example"})}, "work", "https://flag.example", "lgn_work", "yaml", "flag"},
		{"unknown profile falls to default url", Inputs{Profile: "nope", Getenv: env(nil)}, "nope", DefaultAPIURL, "", "table", "default"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := Resolve(f, c, tt.in)
			if r.Profile != tt.profile || r.APIURL != tt.url || r.APIKey != tt.key || r.Output != tt.out || r.Sources["api_url"] != tt.srcURL {
				t.Fatalf("got %+v", r)
			}
		})
	}
}

func TestResolveEmpty(t *testing.T) {
	r := Resolve(&File{}, &Credentials{}, Inputs{Getenv: env(nil)})
	if r.Profile != "default" || r.APIURL != DefaultAPIURL || r.APIKey != "" || r.Output != "table" || r.Sources["api_key"] != "none" {
		t.Fatalf("got %+v", r)
	}
}

func TestResolveTrimsTrailingSlash(t *testing.T) {
	r := Resolve(&File{}, &Credentials{}, Inputs{APIURL: "https://x.example/", Getenv: env(nil)})
	if r.APIURL != "https://x.example" {
		t.Fatal(r.APIURL)
	}
}

func TestStoreRoundTripAndPerms(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("posix perms")
	}
	dir := filepath.Join(t.TempDir(), "legion")
	var warn bytes.Buffer
	s := Store{Dir: dir, Warn: &warn}
	c := &Credentials{Profiles: map[string]*Cred{"default": {APIKey: "lgn_secret"}}}
	if err := s.SaveCreds(c); err != nil {
		t.Fatal(err)
	}
	st, _ := os.Stat(filepath.Join(dir, "credentials.yaml"))
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("creds mode %v", st.Mode().Perm())
	}
	st, _ = os.Stat(dir)
	if st.Mode().Perm() != 0o700 {
		t.Fatalf("dir mode %v", st.Mode().Perm())
	}
	got, err := s.LoadCreds()
	if err != nil || got.Profiles["default"].APIKey != "lgn_secret" {
		t.Fatalf("load: %+v %v", got, err)
	}
	if warn.Len() != 0 {
		t.Fatalf("unexpected warning %q", warn.String())
	}
	// Loosen and expect a warning, still loading.
	if err := os.Chmod(filepath.Join(dir, "credentials.yaml"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.LoadCreds(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(warn.String(), "chmod 600") {
		t.Fatalf("no perms warning: %q", warn.String())
	}
}

func TestLoadMissingIsEmpty(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	f, err := s.LoadConfig()
	if err != nil || f == nil || len(f.Profiles) != 0 {
		t.Fatal(f, err)
	}
	c, err := s.LoadCreds()
	if err != nil || c == nil {
		t.Fatal(c, err)
	}
}

func TestConfigSaveLoad(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	f := &File{CurrentProfile: "a"}
	f.Ensure("a").APIURL = "https://a.example"
	if err := s.SaveConfig(f); err != nil {
		t.Fatal(err)
	}
	g, err := s.LoadConfig()
	if err != nil || g.CurrentProfile != "a" || g.Profiles["a"].APIURL != "https://a.example" {
		t.Fatal(g, err)
	}
}

func TestSetGet(t *testing.T) {
	p := &Profile{}
	if err := p.Set("api_url", "https://x.example/"); err != nil {
		t.Fatal(err)
	}
	if v, _ := p.Get("api_url"); v != "https://x.example" {
		t.Fatal(v)
	}
	if err := p.Set("api_url", "ftp://x"); err == nil {
		t.Fatal("want url error")
	}
	if err := p.Set("bogus", "1"); err == nil {
		t.Fatal("want key error")
	}
	if err := p.Set("output", "wide"); err != nil {
		t.Fatal(err)
	}
	for u, ok := range map[string]bool{"https://a.example": true, "http://localhost:8080": true, "http://127.0.0.1:1": true, "http://[::1]:1": true, "http://a.example": false, "http://localhost.evil.example": false} {
		if (CheckAPIURL(u) == nil) != ok {
			t.Errorf("CheckAPIURL(%q) ok=%v", u, !ok)
		}
	}
}
