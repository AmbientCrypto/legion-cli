package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fakeGitHub serves a release the way github.com does: /releases/latest
// redirects to the tag page, assets download under /releases/download/<tag>/.
func fakeGitHub(t *testing.T, version string, assets map[string][]byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/releases/latest":
			http.Redirect(w, r, "/releases/tag/v"+version, http.StatusFound)
		case strings.HasPrefix(r.URL.Path, "/releases/tag/"):
			w.Write([]byte("release page"))
		case strings.HasPrefix(r.URL.Path, "/releases/download/v"+version+"/"):
			b, ok := assets[strings.TrimPrefix(r.URL.Path, "/releases/download/v"+version+"/")]
			if !ok {
				http.NotFound(w, r)
				return
			}
			w.Write(b)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func tarGz(t *testing.T, name string, body []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, f := range []struct {
		name string
		body []byte
	}{{"README.md", []byte("readme")}, {name, body}} {
		if err := tw.WriteHeader(&tar.Header{Name: f.name, Mode: 0o755, Size: int64(len(f.body)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		tw.Write(f.body)
	}
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

func zipped(t *testing.T, name string, body []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	f, _ := zw.Create(name)
	f.Write(body)
	zw.Close()
	return buf.Bytes()
}

func sums(files map[string][]byte) []byte {
	var b strings.Builder
	for name, body := range files {
		s := sha256.Sum256(body)
		b.WriteString(hex.EncodeToString(s[:]) + "  " + name + "\n")
	}
	return []byte(b.String())
}

func TestNewer(t *testing.T) {
	for _, c := range []struct {
		latest, current string
		want            bool
	}{
		{"0.2.0", "0.1.9", true},
		{"0.10.0", "0.9.0", true},
		{"1.0.0", "0.99.99", true},
		{"0.1.1", "0.1.1", false},
		{"0.1.0", "0.1.1", false},
		{"0.2.0", "dev", false},
		{"0.2.0", "", false},
		{"garbage", "0.1.0", false},
		{"0.2.0", "v0.1.0", true},
	} {
		if got := Newer(c.latest, c.current); got != c.want {
			t.Errorf("Newer(%q, %q) = %v, want %v", c.latest, c.current, got, c.want)
		}
	}
}

func TestLatestReadsTheRedirect(t *testing.T) {
	srv := fakeGitHub(t, "0.3.1", nil)
	got, err := Source{Base: srv.URL}.Latest(context.Background())
	if err != nil || got != "0.3.1" {
		t.Fatalf("Latest = %q, %v; want 0.3.1", got, err)
	}
}

func TestLatestRefusesANonVersionTag(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/releases/tag/nightly", http.StatusFound)
	}))
	defer srv.Close()
	if _, err := (Source{Base: srv.URL}).Latest(context.Background()); err == nil {
		t.Fatal("a tag that is not vX.Y.Z was accepted")
	}
}

func TestDownloadVerifiesAndExtracts(t *testing.T) {
	bin := []byte("#!new legion binary")
	for _, goos := range []string{"linux", "darwin", "windows"} {
		t.Run(goos, func(t *testing.T) {
			name, archive := "legion", "legion_0.3.1_"+goos+"_arm64.tar.gz"
			var body []byte
			if goos == "windows" {
				name, archive = "legion.exe", "legion_0.3.1_windows_arm64.zip"
				body = zipped(t, name, bin)
			} else {
				body = tarGz(t, name, bin)
			}
			assets := map[string][]byte{archive: body}
			assets["checksums.txt"] = sums(assets)
			srv := fakeGitHub(t, "0.3.1", assets)
			got, err := Source{Base: srv.URL}.Download(context.Background(), "0.3.1", goos, "arm64")
			if err != nil || !bytes.Equal(got, bin) {
				t.Fatalf("Download = %q, %v", got, err)
			}
		})
	}
}

func TestDownloadRefusesABadChecksum(t *testing.T) {
	archive := "legion_0.3.1_linux_amd64.tar.gz"
	assets := map[string][]byte{archive: tarGz(t, "legion", []byte("tampered"))}
	assets["checksums.txt"] = sums(map[string][]byte{archive: []byte("something else")})
	srv := fakeGitHub(t, "0.3.1", assets)
	if _, err := (Source{Base: srv.URL}).Download(context.Background(), "0.3.1", "linux", "amd64"); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("err = %v, want a checksum error", err)
	}
}

func TestDownloadRefusesAMissingChecksumLine(t *testing.T) {
	archive := "legion_0.3.1_linux_amd64.tar.gz"
	assets := map[string][]byte{archive: tarGz(t, "legion", []byte("x")), "checksums.txt": []byte("")}
	srv := fakeGitHub(t, "0.3.1", assets)
	if _, err := (Source{Base: srv.URL}).Download(context.Background(), "0.3.1", "linux", "amd64"); err == nil {
		t.Fatal("an archive with no checksum line was accepted")
	}
}

func TestReplaceSwapsTheBinaryInPlace(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "legion")
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	if err := os.WriteFile(exe, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Replace(exe, []byte("new")); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(exe)
	if string(b) != "new" {
		t.Fatalf("binary = %q, want new", b)
	}
	if runtime.GOOS != "windows" {
		st, _ := os.Stat(exe)
		if st.Mode().Perm()&0o111 == 0 {
			t.Errorf("mode %v is not executable", st.Mode())
		}
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.Name() != filepath.Base(exe) && !strings.HasSuffix(e.Name(), ".old") {
			t.Errorf("left %s behind", e.Name())
		}
	}
}

func TestIsRelease(t *testing.T) {
	for v, want := range map[string]bool{"0.1.0": true, "v1.2.3": true, "dev": false, "": false, "1.2": false, "1.2.3-rc1": false} {
		if IsRelease(v) != want {
			t.Errorf("IsRelease(%q) = %v, want %v", v, !want, want)
		}
	}
}
