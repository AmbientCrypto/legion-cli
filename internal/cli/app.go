// Package cli is the legion command tree.
package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"reflect"
	"runtime"
	"time"

	"github.com/AmbientCrypto/legion-controller/api"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/AmbientCrypto/legion-cli/internal/client"
	"github.com/AmbientCrypto/legion-cli/internal/config"
	"github.com/AmbientCrypto/legion-cli/internal/output"
	"github.com/AmbientCrypto/legion-cli/internal/selfupdate"
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
	ReadSecret                     func(context.Context) (string, error) // hidden input from the terminal
	OpenBrowser                    func(url string) error
	Hostname                       func() (string, error) // names the key a browser sign-in makes
	LookPath                       func(string) (string, error)
	Exec                           func(path string, argv []string) error // replaces the process on unix
	ReleasesURL                    string                                 // where legion update looks (selfupdate.DefaultBase)
	Executable                     func() (string, error)                 // the binary legion update replaces
	Now                            func() time.Time                       // the update check's clock
	Home                           string
	Poll                           time.Duration // wait loop interval
	Version, Commit, Date          string
}

// DefaultEnv is the real process environment.
func DefaultEnv(ctx context.Context, version, commit, date string) Env {
	home, _ := os.UserHomeDir()
	return Env{
		Ctx: ctx, Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr, Getenv: os.Getenv,
		StdinTTY:    term.IsTerminal(int(os.Stdin.Fd())),
		StdoutTTY:   term.IsTerminal(int(os.Stdout.Fd())),
		StderrTTY:   term.IsTerminal(int(os.Stderr.Fd())),
		ReadSecret:  readSecret,
		OpenBrowser: openBrowser, Hostname: os.Hostname, LookPath: exec.LookPath, Exec: execReplace,
		ReleasesURL: selfupdate.DefaultBase, Executable: os.Executable, Now: time.Now,
		Home: home, Poll: 3 * time.Second, Version: version, Commit: commit, Date: date,
	}
}

// readSecret reads hidden input but gives up on Ctrl-C, putting the terminal
// back the way it was.
func readSecret(ctx context.Context) (string, error) {
	fd := int(os.Stdin.Fd())
	saved, _ := term.GetState(fd)
	type res struct {
		s   string
		err error
	}
	ch := make(chan res, 1)
	go func() { b, err := term.ReadPassword(fd); ch <- res{string(b), err} }()
	select {
	case r := <-ch:
		fmt.Fprintln(os.Stderr)
		return r.s, r.err
	case <-ctx.Done():
		if saved != nil {
			_ = term.Restore(fd, saved)
		}
		fmt.Fprintln(os.Stderr)
		return "", ctx.Err()
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
	st       config.Store
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
	a.prompt = &ui.Prompter{In: bufio.NewReader(env.Stdin), Out: env.Stderr, Ctx: env.Ctx}
	root := a.root()
	root.SetArgs(args)
	root.SetOut(env.Stdout)
	root.SetErr(env.Stderr)
	err := root.ExecuteContext(env.Ctx)
	a.outdatedNotice()
	if err == nil {
		return client.ExitOK
	}
	d := Describe(err, a.cmdPath)
	st := ui.Style{On: ui.ColorEnabled(a.noColor, env.Getenv, env.StderrTTY)}
	fmt.Fprintf(env.Stderr, "%s %s\n", st.Red(st.Bold("Error:")), d.Message)
	if d.Code != "" {
		fmt.Fprintf(env.Stderr, "%s %s\n", st.Dim("Code:"), d.Code)
	}
	if d.Hint != "" {
		fmt.Fprintf(env.Stderr, "%s %s\n", st.Bold("Hint:"), d.Hint)
	}
	if d.After != "" {
		fmt.Fprint(env.Stderr, d.After)
	}
	return d.Exit
}

// ---- config and client plumbing ----

// load reads config and credentials once. Every command calls it (through
// conf) before it touches a.st, a.cfg or a.creds.
func (a *app) load() error {
	if a.resolved != nil {
		return nil
	}
	dir, err := config.Dir(a.env.Getenv)
	if err != nil {
		return err
	}
	a.st = config.Store{Dir: dir, Warn: a.env.Stderr}
	if a.cfg, err = a.st.LoadConfig(); err != nil {
		return err
	}
	if a.creds, err = a.st.LoadCreds(); err != nil {
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
	if err := config.CheckAPIURL(url); err != nil {
		return nil, usagef("legion config set api_url https://...", "%v", err)
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
	err = output.Write(a.env.Stdout, f, res)
	if errors.Is(err, output.ErrNoMatch) {
		return errorf("", "%v", err)
	}
	return err
}

// need turns a missing answer body (204, empty 200) into an error instead of
// a nil dereference.
func need[T any](p *T) (*T, error) {
	if p == nil {
		return nil, errorf("Check the result with: legion rental list", "the API answered without the expected body")
	}
	return p, nil
}

// withRental resolves <rental> (id, name, prefix, or a picker) and runs fn.
func (a *app) withRental(cmd *cobra.Command, as []string, fn func(ctx context.Context, c *api.ClientWithResponses, r *api.Rental) error) error {
	c, err := a.api()
	if err != nil {
		return err
	}
	ctx := a.ctx(cmd)
	r, err := a.pickRental(ctx, c, as)
	if err != nil {
		return err
	}
	return fn(ctx, c, r)
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
	ok, err := a.prompt.Confirm(question)
	if err != nil {
		return err
	}
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
	i, err := a.prompt.Select(label, opts)
	if errors.Is(err, ui.ErrNoChoice) {
		return 0, usagef(hint, "no valid choice given")
	}
	return i, err
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

// san cleans API text before it reaches the terminal.
func san(s string) string { return output.Sanitize(s) }

func ptr[T any](v T) *T { return &v }

func ptrOr[T any](p *T, def T) T {
	if p == nil {
		return def
	}
	return *p
}

// collect follows next_cursor until max items are read (max <= 0: all of
// them). fetch gets the cursor and the page size to ask for.
func collect[T any](max int, fetch func(cur *string, n int) ([]T, *string, error)) ([]T, error) {
	all := []T{}
	var cur *string
	for i := 0; i < 1000 && (max <= 0 || len(all) < max); i++ {
		n := 100
		if max > 0 {
			n = min(max-len(all), 100)
		}
		items, next, err := fetch(cur, n)
		if err != nil {
			return nil, err
		}
		all = append(all, items...)
		if next == nil || *next == "" {
			break
		}
		cur = next
	}
	return all, nil
}

// listResult is the shared table builder: one row per item, names for -o name.
func listResult[T any](items []T, cols, wide []string, name func(T) string, row func(T) []string) output.Result {
	if items == nil {
		items = []T{}
	}
	t := &output.Table{Cols: cols, Wide: wide}
	names := make([]string, len(items))
	for i, it := range items {
		names[i] = name(it)
		t.Rows = append(t.Rows, row(it))
	}
	return output.Result{Data: items, Names: names, Table: t}
}

// ok is the tail of every API call: res is the generated *XxxResult. It
// reports a transport error, or the problem+json answer of a non-2xx status.
func (a *app) ok(res any, err error) error {
	if err != nil {
		return a.netErr(err)
	}
	v := reflect.ValueOf(res).Elem()
	resp, _ := v.FieldByName("HTTPResponse").Interface().(*http.Response)
	body, _ := v.FieldByName("Body").Interface().([]byte)
	return client.Check(resp, body)
}
