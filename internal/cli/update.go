package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/spf13/cobra"

	"github.com/AmbientCrypto/legion-cli/internal/config"
	"github.com/AmbientCrypto/legion-cli/internal/output"
	"github.com/AmbientCrypto/legion-cli/internal/selfupdate"
)

// Self-update and the outdated notice (LGN-501): legion update installs the
// latest release from GitHub (the archive install.sh would fetch, checked
// against checksums.txt); every other command on a terminal says, at most
// once a day per check, when a newer release exists.

const (
	updateCheckEvery   = 24 * time.Hour
	updateCheckTimeout = 2 * time.Second
	updateCheckFile    = "update-check.json"
)

func (a *app) source() selfupdate.Source { return selfupdate.Source{Base: a.env.ReleasesURL} }

// isRelease is whether this binary is a release that can compare versions.
func (a *app) isRelease() bool { return selfupdate.IsRelease(a.env.Version) }

func (a *app) updateCmd() *cobra.Command {
	var check bool
	cmd := &cobra.Command{
		Use:     "update",
		Aliases: []string{"upgrade"},
		Short:   "Update legion to the latest release",
		Long: `Download the latest legion release for this computer from GitHub, check it
against the release's checksums.txt, and replace this binary with it.

With --check, only say whether a newer release exists. On a terminal, legion
also says so after other commands, at most once a day; set
LEGION_NO_UPDATE_CHECK=1 to turn that off.`,
		Example: "  legion update\n  legion update --check\n  legion update --check -o json",
		Args:    args(0, 0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !a.isRelease() {
				return usagef("Reinstall with: curl -fsSL https://raw.githubusercontent.com/AmbientCrypto/legion-cli/main/scripts/install.sh | bash",
					"this legion (%s) is not a release build, so it cannot update itself", a.env.Version)
			}
			ctx := a.ctx(cmd)
			latest, err := a.source().Latest(ctx)
			if err != nil {
				return errorf("legion update --check", "%v", err)
			}
			a.saveUpdateCheck(latest)
			newer := selfupdate.Newer(latest, a.env.Version)
			if check {
				data := map[string]any{"current": a.env.Version, "latest": latest, "update_available": newer}
				return a.emit(output.Result{Data: data, Human: func(w io.Writer) {
					if newer {
						fmt.Fprintf(w, "legion %s is available (you have %s). Update with: legion update\n", latest, a.env.Version)
					} else {
						fmt.Fprintf(w, "legion %s is up to date.\n", a.env.Version)
					}
				}})
			}
			if !newer {
				a.info("legion %s is up to date.", a.env.Version)
				return nil
			}
			exe, err := a.env.Executable()
			if err == nil {
				exe, err = filepath.EvalSymlinks(exe)
			}
			if err != nil {
				return errorf("", "cannot find this legion binary: %v", err)
			}
			a.info("Downloading legion %s...", latest)
			bin, err := a.source().Download(ctx, latest, runtime.GOOS, runtime.GOARCH)
			if err != nil {
				return errorf("legion update", "%v", err)
			}
			if err := selfupdate.Replace(exe, bin); err != nil {
				return errorf("", "%v", err)
			}
			a.info("Updated legion %s → %s (%s).", a.env.Version, latest, exe)
			return nil
		},
	}
	cmd.Flags().BoolVar(&check, "check", false, "only say whether a newer release exists")
	return cmd
}

// updateCheck is update-check.json in the config directory.
type updateCheck struct {
	CheckedAt time.Time `json:"checked_at"`
	Latest    string    `json:"latest,omitempty"`
}

func (a *app) updateCheckPath() (string, error) {
	dir, err := config.Dir(a.env.Getenv)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, updateCheckFile), nil
}

func (a *app) loadUpdateCheck() updateCheck {
	var c updateCheck
	if p, err := a.updateCheckPath(); err == nil {
		if b, err := os.ReadFile(p); err == nil {
			_ = json.Unmarshal(b, &c)
		}
	}
	return c
}

// saveUpdateCheck records an answer (latest "" keeps the previous one: a failed
// check still waits a day before the next).
func (a *app) saveUpdateCheck(latest string) {
	c := a.loadUpdateCheck()
	c.CheckedAt = a.env.Now()
	if latest != "" {
		c.Latest = latest
	}
	p, err := a.updateCheckPath()
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return
	}
	if b, err := json.Marshal(c); err == nil {
		_ = os.WriteFile(p, b, 0o600)
	}
}

// outdatedNotice says, after a command on a terminal, that a newer release
// exists. It checks GitHub at most once a day, waits at most two seconds, and
// never changes the command's outcome.
func (a *app) outdatedNotice() {
	switch {
	case !a.env.StderrTTY, a.quiet, a.env.Getenv("LEGION_NO_UPDATE_CHECK") != "", a.env.Getenv("CI") != "", !a.isRelease():
		return
	}
	switch a.cmdPath {
	case "", "update", "version", "completion", "help", "__complete":
		return
	}
	c := a.loadUpdateCheck()
	if a.env.Now().Sub(c.CheckedAt) >= updateCheckEvery {
		ctx, cancel := context.WithTimeout(a.env.Ctx, updateCheckTimeout)
		latest, _ := a.source().Latest(ctx)
		cancel()
		a.saveUpdateCheck(latest)
		if latest != "" {
			c.Latest = latest
		}
	}
	if selfupdate.Newer(c.Latest, a.env.Version) {
		st := a.style()
		fmt.Fprintf(a.env.Stderr, "\n%s legion %s is available (you have %s). Update with: legion update\n", st.Bold("Update:"), c.Latest, a.env.Version)
	}
}
