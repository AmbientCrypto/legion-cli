package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Self-update and the outdated notice (LGN-501). The harness's legion is
// version 1.2.3; the fake GitHub publishes latest.

type fakeReleases struct {
	srv     *httptest.Server
	latests atomic.Int32 // /releases/latest requests
}

func newReleases(t *testing.T, h *harness, latest string, bin []byte) *fakeReleases {
	t.Helper()
	ext, name := ".tar.gz", "legion"
	if runtime.GOOS == "windows" {
		t.Skip("the fake release archive is a tarball")
	}
	archive := "legion_" + latest + "_" + runtime.GOOS + "_" + runtime.GOARCH + ext
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(bin)), Typeflag: tar.TypeReg})
	tw.Write(bin)
	tw.Close()
	gz.Close()
	sum := sha256.Sum256(buf.Bytes())
	sums := hex.EncodeToString(sum[:]) + "  " + archive + "\n"
	f := &fakeReleases{}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/releases/latest":
			f.latests.Add(1)
			http.Redirect(w, r, "/releases/tag/v"+latest, http.StatusFound)
		case "/releases/download/v" + latest + "/" + archive:
			w.Write(buf.Bytes())
		case "/releases/download/v" + latest + "/checksums.txt":
			w.Write([]byte(sums))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(f.srv.Close)
	h.releases = f.srv.URL
	h.exe = filepath.Join(t.TempDir(), "legion")
	if err := os.WriteFile(h.exe, []byte("old binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestUpdateCheckSaysANewerVersionExists(t *testing.T) {
	h := newHarness(t)
	newReleases(t, h, "1.3.0", []byte("new"))
	out, _ := h.mustRun("update", "--check")
	contains(t, "stdout", out, "1.3.0", "1.2.3", "legion update")
	out, _ = h.mustRun("update", "--check", "-o", "json")
	var v map[string]any
	if err := json.Unmarshal([]byte(out), &v); err != nil || v["latest"] != "1.3.0" || v["current"] != "1.2.3" || v["update_available"] != true {
		t.Fatalf("json = %s (%v)", out, err)
	}
	if b, _ := os.ReadFile(h.exe); string(b) != "old binary" {
		t.Fatal("--check replaced the binary")
	}
}

func TestUpdateReplacesTheBinary(t *testing.T) {
	h := newHarness(t)
	newReleases(t, h, "1.3.0", []byte("new binary"))
	_, e := h.mustRun("update")
	contains(t, "stderr", e, "Updated legion 1.2.3 → 1.3.0")
	if b, _ := os.ReadFile(h.exe); string(b) != "new binary" {
		t.Fatalf("binary = %q, want the release's", b)
	}
}

func TestUpdateWhenUpToDate(t *testing.T) {
	h := newHarness(t)
	newReleases(t, h, "1.2.3", []byte("same"))
	_, e := h.mustRun("update")
	contains(t, "stderr", e, "up to date")
	if b, _ := os.ReadFile(h.exe); string(b) != "old binary" {
		t.Fatal("an up-to-date legion replaced its binary")
	}
}

func TestUpdateRefusesADevBuild(t *testing.T) {
	h := newHarness(t)
	newReleases(t, h, "1.3.0", []byte("new"))
	h.version = "dev"
	_, e, c := h.run("update")
	if c == 0 {
		t.Fatal("a dev build updated itself")
	}
	contains(t, "stderr", e, "not a release")
}

func TestOutdatedNoticeOncePerDay(t *testing.T) {
	h := newHarness(t)
	rel := newReleases(t, h, "1.3.0", []byte("new"))
	h.tty = true
	h.reply("GET /v1/me", 200, meFixture())
	_, e := h.mustRun("whoami")
	contains(t, "stderr", e, "legion 1.3.0 is available (you have 1.2.3)", "legion update")
	// Within a day the cached answer is used: the notice, no request.
	h.now = h.now.Add(23 * time.Hour)
	_, e = h.mustRun("whoami")
	contains(t, "stderr", e, "legion 1.3.0 is available")
	if n := rel.latests.Load(); n != 1 {
		t.Fatalf("%d release checks in a day, want 1", n)
	}
	h.now = h.now.Add(2 * time.Hour)
	h.mustRun("whoami")
	if n := rel.latests.Load(); n != 2 {
		t.Fatalf("%d release checks after a day, want 2", n)
	}
}

func TestOutdatedNoticeStaysOutOfTheWay(t *testing.T) {
	for _, c := range []struct {
		name string
		set  func(h *harness)
		args []string
	}{
		{"no terminal", func(h *harness) { h.tty = false }, []string{"whoami"}},
		{"quiet", func(h *harness) {}, []string{"whoami", "-q"}},
		{"opted out", func(h *harness) { h.env["LEGION_NO_UPDATE_CHECK"] = "1" }, []string{"whoami"}},
		{"CI", func(h *harness) { h.env["CI"] = "true" }, []string{"whoami"}},
		{"version", func(h *harness) {}, []string{"version"}},
		{"a dev build", func(h *harness) { h.version = "dev" }, []string{"whoami"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			h := newHarness(t)
			rel := newReleases(t, h, "1.3.0", []byte("new"))
			h.tty = true
			h.reply("GET /v1/me", 200, meFixture())
			c.set(h)
			_, e := h.mustRun(c.args...)
			excludes(t, "stderr", e, "is available")
			if strings.Contains(c.name, "terminal") || c.name == "opted out" || c.name == "CI" || c.name == "a dev build" {
				if n := rel.latests.Load(); n != 0 {
					t.Errorf("%d release checks, want none", n)
				}
			}
		})
	}
}

func TestOutdatedNoticeNeverFailsACommand(t *testing.T) {
	h := newHarness(t)
	h.tty = true
	h.releases = "http://127.0.0.1:1" // nothing listens
	h.reply("GET /v1/me", 200, meFixture())
	_, e := h.mustRun("whoami")
	excludes(t, "stderr", e, "is available", "Error")
}
