// Package cli is the legion command tree.
package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/AmbientCrypto/legion-controller/api"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/AmbientCrypto/legion-cli/internal/client"
	"github.com/AmbientCrypto/legion-cli/internal/config"
	"github.com/AmbientCrypto/legion-cli/internal/output"
	"github.com/AmbientCrypto/legion-cli/internal/ui"
)

// Env is everything the CLI touches outside its own memory, so tests can
// replace it.
type Env struct {
	Ctx                            context.Context
	Stdin                          io.Reader
	Stdout, Stderr                 io.Writer
	Getenv                         func(string) string
	StdinTTY, StdoutTTY, StderrTTY bool
	ReadSecret                     func() (string, error) // hidden input from the terminal
	OpenBrowser                    func(url string) error
	LookPath                       func(string) (string, error)
	Exec                           func(path string, argv []string) error // replaces the process on unix
	Home                           string
	Poll                           time.Duration // wait loop interval
	Version, Commit, Date          string
}

// DefaultEnv is the real process environment.
func DefaultEnv(ctx context.Context, version, commit, date string) Env {
	home, _ := os.UserHomeDir()
	return Env{
		Ctx: ctx, Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr, Getenv: os.Getenv,
		StdinTTY:  term.IsTerminal(int(os.Stdin.Fd())),
		StdoutTTY: term.IsTerminal(int(os.Stdout.Fd())),
		StderrTTY: term.IsTerminal(int(os.Stderr.Fd())),
		ReadSecret: func() (string, error) {
			b, err := term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Fprintln(os.Stderr)
			return string(b), err
		},
		OpenBrowser: openBrowser, LookPath: exec.LookPath, Exec: execReplace,
		Home: home, Poll: 3 * time.Second, Version: version, Commit: commit, Date: date,
	}
}

func openBrowser(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	}
	return exec.Command("xdg-open", url).Start()
}

// app holds the global flags and per-run state.
type app struct {
	env                           Env
	flagProfile, flagURL, flagOut string
	noColor, quiet, debug, yes    bool
	timeout                       time.Duration
	cmdPath                       string

	prompt   *ui.Prompter
	cfg      *config.File
	creds    *config.Credentials
	resolved *config.Resolved
}

// Execute runs legion with args and returns the exit code.
func Execute(env Env, args []string) int {
	if env.Ctx == nil {
		env.Ctx = context.Background()
	}
	a := &app{env: env}
	a.prompt = &ui.Prompter{In: bufio.NewReader(env.Stdin), Out: env.Stderr}
	root := a.root()
	root.SetArgs(args)
	root.SetOut(env.Stdout)
	root.SetErr(env.Stderr)
	err := root.ExecuteContext(env.Ctx)
	if err == nil {
		return client.ExitOK
	}
	d := Describe(err, a.cmdPath)
	fmt.Fprintf(env.Stderr, "Error: %s\n", d.Message)
	if d.Code != "" {
		fmt.Fprintf(env.Stderr, "Code: %s\n", d.Code)
	}
	if d.Hint != "" {
		fmt.Fprintf(env.Stderr, "Hint: %s\n", d.Hint)
	}
	return d.Exit
}

// ---- config and client plumbing ----

func (a *app) store() config.Store {
	dir, err := config.Dir(a.env.Getenv)
	if err != nil {
		dir = "."
	}
	return config.Store{Dir: dir, Warn: a.env.Stderr}
}

func (a *app) load() error {
	if a.resolved != nil {
		return nil
	}
	var err error
	if a.cfg, err = a.store().LoadConfig(); err != nil {
		return err
	}
	if a.creds, err = a.store().LoadCreds(); err != nil {
		return err
	}
	r := config.Resolve(a.cfg, a.creds, config.Inputs{Profile: a.flagProfile, APIURL: a.flagURL, Output: a.flagOut, Getenv: a.env.Getenv})
	a.resolved = &r
	return nil
}

func (a *app) conf() (*config.Resolved, error) {
	if err := a.load(); err != nil {
		return nil, err
	}
	return a.resolved, nil
}

// newClient builds a client for the given key and URL.
func (a *app) newClient(url, key string) (*api.ClientWithResponses, error) {
	var dbg io.Writer
	if a.debug {
		dbg = a.env.Stderr
	}
	return client.New(client.Options{BaseURL: url, Token: key, Version: a.env.Version, Debug: dbg, Timeout: a.timeout})
}

// api returns a client for the current profile; it needs a key.
func (a *app) api() (*api.ClientWithResponses, error) {
	r, err := a.conf()
	if err != nil {
		return nil, err
	}
	if r.APIKey == "" {
		return nil, errNotLoggedIn
	}
	return a.newClient(r.APIURL, r.APIKey)
}

// netErr words a transport error with the API URL.
func (a *app) netErr(err error) error {
	if errors.Is(err, context.Canceled) {
		return err
	}
	u := config.DefaultAPIURL
	if r, e := a.conf(); e == nil {
		u = r.APIURL
	}
	return &cliError{Msg: client.NetworkError(err, u).Error(), Code: "network_error", Hint: "Check your connection, or the API URL (legion config get api_url)", Exit: client.ExitError}
}

// ---- output ----

func (a *app) format() (output.Format, error) {
	r, err := a.conf()
	if err != nil {
		return output.Format{}, err
	}
	f, err := output.Parse(r.Output)
	if err != nil {
		return f, usagef("legion --help", "%v", err)
	}
	return f, nil
}

// emit writes a command's result to stdout in the chosen format.
func (a *app) emit(res output.Result) error {
	f, err := a.format()
	if err != nil {
		return err
	}
	return output.Write(a.env.Stdout, f, res)
}

// info prints a status line to stderr unless -q.
func (a *app) info(format string, args ...any) {
	if !a.quiet {
		fmt.Fprintf(a.env.Stderr, format+"\n", args...)
	}
}

func (a *app) style() ui.Style {
	return ui.Style{On: ui.ColorEnabled(a.noColor, a.env.Getenv, a.env.StdoutTTY)}
}

// spinner is on only for a terminal, never with -q, never with machine output.
func (a *app) spinner() *ui.Spinner {
	return ui.NewSpinner(a.env.Stderr, a.env.StderrTTY && !a.quiet)
}

// interactive reports whether we may prompt.
func (a *app) interactive() bool { return a.env.StdinTTY }

// confirm asks for a yes. --yes answers it; with no terminal and no --yes it
// refuses instead of guessing.
func (a *app) confirm(question, whatIfNot string) error {
	if a.yes {
		return nil
	}
	if !a.interactive() {
		return usagef("Add --yes to confirm without a prompt.", "%s", whatIfNot)
	}
	ok, _ := a.prompt.Confirm(question)
	if !ok {
		return &cliError{Msg: "aborted", Exit: client.ExitError}
	}
	return nil
}

// pickOne offers a numbered choice on a terminal; elsewhere it is a usage error.
func (a *app) pickOne(label string, opts []string, missing, hint string) (int, error) {
	if !a.interactive() {
		return 0, usagef(hint, "%s", missing)
	}
	if len(opts) == 0 {
		return 0, errorf("", "nothing to choose from")
	}
	return a.prompt.Select(label, opts)
}

func (a *app) ctx(cmd *cobra.Command) context.Context {
	if c := cmd.Context(); c != nil {
		return c
	}
	return context.Background()
}

// short reports the first 8 characters of a secret, for display.
func short(s string) string {
	if len(s) <= 12 {
		return s[:min(4, len(s))] + "…"
	}
	return s[:8] + "…"
}

func ptr[T any](v T) *T { return &v }

func ptrOr[T any](p *T, def T) T {
	if p == nil {
		return def
	}
	return *p
}

func joinNonEmpty(sep string, parts ...string) string {
	var out []string
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, sep)
}

// listAll follows next_cursor to the end.
func listAll[T any](fetch func(cursor *string) ([]T, *string, error)) ([]T, error) {
	var all []T
	var cur *string
	for i := 0; i < 1000; i++ {
		items, next, err := fetch(cur)
		if err != nil {
			return nil, err
		}
		all = append(all, items...)
		if next == nil || *next == "" {
			return all, nil
		}
		cur = next
	}
	return all, nil
}
