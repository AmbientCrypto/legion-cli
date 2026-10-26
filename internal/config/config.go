// Package config reads and writes the CLI's profiles and credentials and
// resolves the effective settings: flag, then environment, then profile,
// then the built-in default.
package config

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	DefaultAPIURL  = "https://api.legion.ambient.xyz"
	DefaultProfile = "default"
	DefaultOutput  = "table"
)

// Keys are the settings `legion config` can change on a profile.
var Keys = []string{"api_url", "output"}

// Profile is one named set of non-secret settings.
type Profile struct {
	APIURL string `yaml:"api_url,omitempty"`
	Output string `yaml:"output,omitempty"`
}

// File is config.yaml.
type File struct {
	CurrentProfile string              `yaml:"current_profile,omitempty"`
	Profiles       map[string]*Profile `yaml:"profiles,omitempty"`
}

// Cred is one profile's secret.
type Cred struct {
	APIKey string `yaml:"api_key,omitempty"`
	// BrowserLogin is set when legion auth login made the key itself (LGN-490):
	// logout then revokes it. A pasted key is left alone.
	BrowserLogin bool `yaml:"browser_login,omitempty"`
}

// Credentials is credentials.yaml, kept apart so it can be 0600.
type Credentials struct {
	Profiles map[string]*Cred `yaml:"profiles,omitempty"`
}

// Ensure returns the named profile, creating it.
func (f *File) Ensure(name string) *Profile {
	if f.Profiles == nil {
		f.Profiles = map[string]*Profile{}
	}
	if f.Profiles[name] == nil {
		f.Profiles[name] = &Profile{}
	}
	return f.Profiles[name]
}

// Names lists the known profiles, including ones that only have credentials.
func Names(f *File, c *Credentials) []string {
	set := map[string]bool{}
	for n := range f.Profiles {
		set[n] = true
	}
	for n := range c.Profiles {
		set[n] = true
	}
	out := make([]string, 0, len(set))
	for n := range set {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// Get returns a setting by key.
func (p *Profile) Get(key string) (string, error) {
	switch key {
	case "api_url":
		return p.APIURL, nil
	case "output":
		return p.Output, nil
	}
	return "", unknownKey(key)
}

// Set validates and stores a setting.
func (p *Profile) Set(key, val string) error {
	switch key {
	case "api_url":
		if err := CheckAPIURL(val); err != nil {
			return err
		}
		p.APIURL = strings.TrimRight(val, "/")
	case "output":
		p.Output = val
	default:
		return unknownKey(key)
	}
	return nil
}

func unknownKey(k string) error {
	return fmt.Errorf("unknown config key %q (valid: %s)", k, strings.Join(Keys, ", "))
}

// CheckAPIURL requires https, except for a local development server, so an
// API key is never sent over plain text.
func CheckAPIURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return fmt.Errorf("api_url must be an https URL, got %q", raw)
	}
	switch h := u.Hostname(); {
	case u.Scheme == "https":
	case u.Scheme == "http" && (h == "localhost" || h == "127.0.0.1" || h == "::1"):
	default:
		return fmt.Errorf("api_url %q must use https (http is allowed only for localhost, 127.0.0.1 and [::1])", raw)
	}
	return nil
}

// Dir is $LEGION_CONFIG_DIR, else <user config dir>/legion.
func Dir(getenv func(string) string) (string, error) {
	if d := getenv("LEGION_CONFIG_DIR"); d != "" {
		return d, nil
	}
	if x := getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "legion"), nil
	}
	d, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("cannot find a config directory (set LEGION_CONFIG_DIR): %w", err)
	}
	return filepath.Join(d, "legion"), nil
}

// Store reads and writes the two files in Dir. Warn gets permission warnings.
type Store struct {
	Dir  string
	Warn io.Writer
}

func (s Store) path(name string) string { return filepath.Join(s.Dir, name) }

func (s Store) LoadConfig() (*File, error) {
	f := &File{}
	if err := s.read("config.yaml", f); err != nil {
		return nil, err
	}
	return f, nil
}

func (s Store) SaveConfig(f *File) error { return s.write("config.yaml", f, 0o644) }

func (s Store) LoadCreds() (*Credentials, error) {
	c := &Credentials{}
	p := s.path("credentials.yaml")
	if st, err := os.Stat(p); err == nil && s.Warn != nil && st.Mode().Perm()&0o077 != 0 {
		fmt.Fprintf(s.Warn, "Warning: %s is readable by other users (mode %04o); run: chmod 600 %s\n", p, st.Mode().Perm(), p)
	}
	if st, err := os.Stat(s.Dir); err == nil && s.Warn != nil && st.Mode().Perm()&0o077 != 0 {
		fmt.Fprintf(s.Warn, "Warning: %s is accessible by other users (mode %04o); run: chmod 700 %s\n", s.Dir, st.Mode().Perm(), s.Dir)
	}
	if err := s.read("credentials.yaml", c); err != nil {
		return nil, err
	}
	return c, nil
}

func (s Store) SaveCreds(c *Credentials) error { return s.write("credentials.yaml", c, 0o600) }

func (s Store) read(name string, into any) error {
	b, err := os.ReadFile(s.path(name))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := yaml.Unmarshal(b, into); err != nil {
		return fmt.Errorf("%s: %w", s.path(name), err)
	}
	return nil
}

func (s Store) write(name string, v any, mode os.FileMode) error {
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return err
	}
	b, err := yaml.Marshal(v)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(s.Dir, name+".*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), s.path(name))
}

// Inputs are the per-invocation overrides.
type Inputs struct {
	Profile, APIURL, Output string
	Getenv                  func(string) string
}

// Resolved is the effective configuration and where each value came from.
type Resolved struct {
	Profile, APIURL, APIKey, Output string
	Sources                         map[string]string // key -> flag|env|profile|default|none
}

// Resolve applies flag > env > profile > default. The API key has no flag
// (it would show in the process list): env > profile.
func Resolve(f *File, c *Credentials, in Inputs) Resolved {
	r := Resolved{Sources: map[string]string{}}
	pick := func(key string, flag string, envName string, prof string, def string) string {
		switch {
		case flag != "":
			r.Sources[key] = "flag"
			return flag
		case in.Getenv(envName) != "":
			r.Sources[key] = "env"
			return in.Getenv(envName)
		case prof != "":
			r.Sources[key] = "profile"
			return prof
		}
		if def == "" {
			r.Sources[key] = "none"
		} else {
			r.Sources[key] = "default"
		}
		return def
	}
	r.Profile = pick("profile", in.Profile, "LEGION_PROFILE", f.CurrentProfile, DefaultProfile)
	var p Profile
	if f.Profiles[r.Profile] != nil {
		p = *f.Profiles[r.Profile]
	}
	var key string
	if c.Profiles[r.Profile] != nil {
		key = c.Profiles[r.Profile].APIKey
	}
	r.APIURL = strings.TrimRight(pick("api_url", in.APIURL, "LEGION_API_URL", p.APIURL, DefaultAPIURL), "/")
	r.Output = pick("output", in.Output, "LEGION_OUTPUT", p.Output, DefaultOutput)
	r.APIKey = pick("api_key", "", "LEGION_API_KEY", key, "")
	return r
}
