// Package selfupdate finds the latest legion release on GitHub, downloads the
// archive for this platform, checks it against the release's checksums.txt
// and swaps the running binary for it (LGN-501). It reads the same files
// install.sh does, from the same place.
package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// DefaultBase is the repository whose releases legion updates from.
const DefaultBase = "https://github.com/AmbientCrypto/legion-cli"

// maxArchive bounds a download: a legion archive is a few MB.
const maxArchive = 64 << 20

var tagPattern = regexp.MustCompile(`/releases/tag/v(\d+\.\d+\.\d+)$`)

// Source is where releases come from.
type Source struct {
	Base string       // DefaultBase, or a test server
	HTTP *http.Client // nil: a client with a 60 s timeout
}

func (s Source) client() *http.Client {
	if s.HTTP != nil {
		return s.HTTP
	}
	return &http.Client{Timeout: 60 * time.Second}
}

// Latest is the version (without the v) of the latest release: where
// /releases/latest redirects, as install.sh reads it. No API token, no rate limit.
func (s Source) Latest(ctx context.Context) (string, error) {
	c := *s.client()
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.Base+"/releases/latest", nil)
	if err != nil {
		return "", err
	}
	resp, err := c.Do(req)
	if err != nil {
		return "", fmt.Errorf("find the latest release: %w", err)
	}
	resp.Body.Close()
	m := tagPattern.FindStringSubmatch(resp.Header.Get("Location"))
	if m == nil {
		return "", fmt.Errorf("find the latest release: unexpected answer %s", resp.Status)
	}
	return m[1], nil
}

// Download returns the legion binary of version for goos/goarch, verified
// against the release's checksums.txt.
func (s Source) Download(ctx context.Context, version, goos, goarch string) ([]byte, error) {
	ext, bin := ".tar.gz", "legion"
	if goos == "windows" {
		ext, bin = ".zip", "legion.exe"
	}
	archive := fmt.Sprintf("legion_%s_%s_%s%s", version, goos, goarch, ext)
	sums, err := s.get(ctx, version, "checksums.txt")
	if err != nil {
		return nil, err
	}
	want, err := checksumOf(sums, archive)
	if err != nil {
		return nil, err
	}
	body, err := s.get(ctx, version, archive)
	if err != nil {
		return nil, err
	}
	got := sha256.Sum256(body)
	if hex.EncodeToString(got[:]) != want {
		return nil, fmt.Errorf("%s does not match its checksum in checksums.txt: not installing it", archive)
	}
	if goos == "windows" {
		return fromZip(body, bin)
	}
	return fromTarGz(body, bin)
}

func (s Source) get(ctx context.Context, version, name string) ([]byte, error) {
	u := s.Base + "/releases/download/v" + version + "/" + name
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.client().Do(req)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: %s", name, resp.Status)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxArchive+1))
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", name, err)
	}
	if len(b) > maxArchive {
		return nil, fmt.Errorf("download %s: larger than %d bytes", name, maxArchive)
	}
	return b, nil
}

func checksumOf(sums []byte, name string) (string, error) {
	sc := bufio.NewScanner(bytes.NewReader(sums))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) == 2 && f[1] == name && len(f[0]) == 64 {
			return strings.ToLower(f[0]), nil
		}
	}
	return "", fmt.Errorf("checksums.txt has no line for %s", name)
}

func fromTarGz(b []byte, name string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("the archive has no %s", name)
		}
		if err != nil {
			return nil, err
		}
		if h.Typeflag == tar.TypeReg && path.Base(h.Name) == name {
			return io.ReadAll(io.LimitReader(tr, maxArchive))
		}
	}
}

func fromZip(b []byte, name string) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		return nil, err
	}
	for _, f := range zr.File {
		if path.Base(f.Name) == name && !f.FileInfo().IsDir() {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return io.ReadAll(io.LimitReader(rc, maxArchive))
		}
	}
	return nil, fmt.Errorf("the archive has no %s", name)
}

// Newer is whether latest is a higher version than current. A build that
// is not a release ("dev", empty) is never told to update.
func Newer(latest, current string) bool {
	l, ok1 := parse(latest)
	c, ok2 := parse(strings.TrimPrefix(current, "v"))
	if !ok1 || !ok2 {
		return false
	}
	for i := range l {
		if l[i] != c[i] {
			return l[i] > c[i]
		}
	}
	return false
}

// IsRelease is whether v is a release version (x.y.z, an optional v), not "dev".
func IsRelease(v string) bool {
	_, ok := parse(strings.TrimPrefix(v, "v"))
	return ok
}

func parse(v string) ([3]int, bool) {
	var out [3]int
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return out, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return out, false
		}
		out[i] = n
	}
	return out, true
}

// Replace swaps the binary at exe for bin: written next to it and renamed
// over it, so a failure leaves the old binary in place. Windows cannot
// replace a running executable, so the old one is moved aside first.
func Replace(exe string, bin []byte) error {
	dir := filepath.Dir(exe)
	tmp, err := os.CreateTemp(dir, ".legion-update-*")
	if err != nil {
		return fmt.Errorf("cannot write next to %s (%w): reinstall with install.sh, or run legion update with permission to write there", exe, err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(bin); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o755); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		old := exe + ".old"
		_ = os.Remove(old)
		if err := os.Rename(exe, old); err != nil {
			return err
		}
	}
	return os.Rename(tmp.Name(), exe)
}
