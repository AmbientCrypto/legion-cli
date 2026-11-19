package cli

import (
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// ---- container configuration flags (LGN-628) ----
//
// --env, --secret-env, --setting, --command and --arg are shared by rental
// create and services create/update. The checks here are the cheap ones that
// save a round trip; the controller is authoritative and its refusals are
// shown as they come.

const (
	maxEnvVars     = 64
	maxEnvValue    = 32 * 1024
	maxEnvTotal    = 64 * 1024
	maxArgvItems   = 64
	maxArgvItemLen = 4 * 1024
)

var envNameRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,127}$`)

// runtimeEnvNames are set by the runtime itself and cannot be overridden.
var runtimeEnvNames = map[string]bool{"PATH": true, "HOME": true, "HOSTNAME": true, "CUDA_VISIBLE_DEVICES": true}

// envVar is one variable as the person gave it. Value is empty for a secret
// that was not given a value (never printed).
type envVar struct {
	Name   string
	Value  string
	Secret bool
}

// checkEnvName reports why name cannot be a variable name, or nil.
func checkEnvName(name string) error {
	switch {
	case !envNameRE.MatchString(name):
		return fmt.Errorf("%q is not a valid name: use letters, digits and underscores, not starting with a digit, up to 128 characters", name)
	case strings.HasPrefix(name, "LEGION_"):
		return fmt.Errorf("%s: names starting with LEGION_ are reserved", name)
	case runtimeEnvNames[name] || strings.HasPrefix(name, "NVIDIA_"):
		return fmt.Errorf("%s is set by the runtime and cannot be changed", name)
	}
	return nil
}

// containerConfig is what the configuration flags ask for.
type containerConfig struct {
	Env        []envVar
	Settings   map[string]any // bool, int64 or string, by what the value looks like
	Command    []string
	Args       []string
	hasCommand bool
	hasArgs    bool
}

func (c *containerConfig) empty() bool {
	return c == nil || (len(c.Env) == 0 && len(c.Settings) == 0 && !c.hasCommand && !c.hasArgs)
}

// configFlags are the raw flag values.
type configFlags struct {
	env, secretEnv, setting, command, arg []string
}

func (f *configFlags) register(cmd *cobra.Command, own string) {
	fl := cmd.Flags()
	fl.StringArrayVar(&f.env, "env", nil, "environment variable NAME=value for the container (repeatable). Not for secrets: they would show in your shell history; use --secret-env")
	fl.StringArrayVar(&f.secretEnv, "secret-env", nil, "secret environment variable NAME (repeatable). The value is read from stdin (one line per secret) or a hidden prompt, stored encrypted and never shown again")
	fl.StringArrayVar(&f.setting, "setting", nil, "model setting key=value for a catalog model, for example max_model_len=32768 (repeatable; see the catalog entry's settings)")
	fl.StringArrayVar(&f.command, "command", nil, "replace the image entrypoint of "+own+": one item per flag, for example --command python --command server.py (repeatable)")
	fl.StringArrayVar(&f.arg, "arg", nil, "replace the image's default arguments of "+own+": one item per flag (repeatable)")
}

// registerEnvOnly registers just the variable flags (rental env set).
func (f *configFlags) registerEnvOnly(cmd *cobra.Command) {
	fl := cmd.Flags()
	fl.StringArrayVar(&f.env, "env", nil, "environment variable NAME=value (repeatable). Not for secrets: use --secret-env")
	fl.StringArrayVar(&f.secretEnv, "secret-env", nil, "secret environment variable NAME (repeatable); the value is read from stdin (one line per secret) or a hidden prompt")
	for _, n := range []string{"setting", "command", "arg"} {
		fl.StringArrayVar(map[string]*[]string{"setting": &f.setting, "command": &f.command, "arg": &f.arg}[n], n, nil, "")
		_ = fl.MarkHidden(n)
	}
}

func (f *configFlags) any() bool {
	return len(f.env)+len(f.secretEnv)+len(f.setting)+len(f.command)+len(f.arg) > 0
}

func parseSetting(s string) (string, any, error) {
	k, v, ok := strings.Cut(s, "=")
	k = strings.TrimSpace(k)
	if !ok || k == "" || strings.TrimSpace(v) == "" {
		return "", nil, fmt.Errorf("--setting %q: use key=value, for example max_model_len=32768", s)
	}
	v = strings.TrimSpace(v)
	switch v {
	case "true":
		return k, true, nil
	case "false":
		return k, false, nil
	}
	if n, err := strconv.ParseInt(v, 10, 64); err == nil {
		return k, n, nil
	}
	return k, v, nil
}

func checkArgv(flagName string, items []string) error {
	if len(items) > maxArgvItems {
		return fmt.Errorf("%s: at most %d items, got %d", flagName, maxArgvItems, len(items))
	}
	for _, it := range items {
		if len(it) > maxArgvItemLen {
			return fmt.Errorf("%s: an item is longer than %d bytes", flagName, maxArgvItemLen)
		}
	}
	return nil
}

// resolve checks the flags and reads secret values: from stdin when it is not
// a terminal (one line per --secret-env, in flag order; with one secret the
// whole input), else a hidden prompt per name. help is the command to point
// at in errors, for example "rental create".
func (a *app) resolveConfig(cmd *cobra.Command, f *configFlags, help string) (*containerConfig, error) {
	if !f.any() {
		return nil, nil
	}
	bad := func(err error) error { return usagef("legion "+help+" --help", "%v", err) }
	cfg := &containerConfig{}
	seen := map[string]bool{}
	total := 0
	add := func(v envVar) error {
		if err := checkEnvName(v.Name); err != nil {
			return err
		}
		if seen[v.Name] {
			return fmt.Errorf("%s is given twice", v.Name)
		}
		seen[v.Name] = true
		if len(v.Value) > maxEnvValue {
			return fmt.Errorf("%s: the value is longer than %d KiB", v.Name, maxEnvValue/1024)
		}
		total += len(v.Name) + len(v.Value)
		cfg.Env = append(cfg.Env, v)
		return nil
	}
	for _, e := range f.env {
		n, v, ok := strings.Cut(e, "=")
		if !ok {
			return nil, bad(fmt.Errorf("--env %q: use NAME=value (for a secret use --secret-env NAME)", e))
		}
		if err := add(envVar{Name: n, Value: v}); err != nil {
			return nil, bad(err)
		}
	}
	for _, n := range f.secretEnv {
		if strings.Contains(n, "=") {
			// Never echo the argument: it may hold the secret.
			return nil, bad(fmt.Errorf("--secret-env takes only the NAME; the value is read from stdin or a hidden prompt so it stays out of your shell history"))
		}
		if err := checkEnvName(n); err != nil {
			return nil, bad(err)
		}
	}
	if len(f.secretEnv) > 0 {
		vals, err := a.readSecretValues(cmd, f.secretEnv)
		if err != nil {
			return nil, err
		}
		for i, n := range f.secretEnv {
			if err := add(envVar{Name: n, Value: vals[i], Secret: true}); err != nil {
				return nil, bad(err)
			}
		}
	}
	if len(cfg.Env) > maxEnvVars {
		return nil, bad(fmt.Errorf("at most %d environment variables, got %d", maxEnvVars, len(cfg.Env)))
	}
	if total > maxEnvTotal {
		return nil, bad(fmt.Errorf("environment values add up to more than %d KiB", maxEnvTotal/1024))
	}
	if len(f.setting) > 0 {
		cfg.Settings = map[string]any{}
		for _, s := range f.setting {
			k, v, err := parseSetting(s)
			if err != nil {
				return nil, bad(err)
			}
			if _, dup := cfg.Settings[k]; dup {
				return nil, bad(fmt.Errorf("--setting %s is given twice", k))
			}
			cfg.Settings[k] = v
		}
	}
	for _, p := range []struct {
		name  string
		items []string
		dst   *[]string
		has   *bool
	}{{"--command", f.command, &cfg.Command, &cfg.hasCommand}, {"--arg", f.arg, &cfg.Args, &cfg.hasArgs}} {
		if len(p.items) == 0 {
			continue
		}
		if err := checkArgv(p.name, p.items); err != nil {
			return nil, bad(err)
		}
		*p.dst, *p.has = append([]string(nil), p.items...), true
	}
	return cfg, nil
}

// readSecretValues returns one value per name, never echoing them.
func (a *app) readSecretValues(cmd *cobra.Command, names []string) ([]string, error) {
	vals := make([]string, len(names))
	if !a.interactive() {
		b, err := io.ReadAll(a.env.Stdin)
		if err != nil {
			return nil, err
		}
		in := string(b)
		if len(names) == 1 {
			vals[0] = strings.TrimRight(in, "\r\n")
		} else {
			lines := strings.Split(strings.ReplaceAll(in, "\r\n", "\n"), "\n")
			if len(lines) > 0 && lines[len(lines)-1] == "" {
				lines = lines[:len(lines)-1]
			}
			if len(lines) != len(names) {
				return nil, usagef("printf '%s\\n%s\\n' \"$A\" \"$B\" | legion ... --secret-env A --secret-env B", "%d --secret-env given but stdin has %d lines: send one line per secret, in flag order", len(names), len(lines))
			}
			copy(vals, lines)
		}
	} else {
		for i, n := range names {
			fmt.Fprintf(a.env.Stderr, "Value for %s (hidden: nothing is shown), then Enter: ", n)
			v, err := a.env.ReadSecret(a.ctx(cmd))
			if err != nil {
				return nil, err
			}
			vals[i] = v
		}
	}
	for i, v := range vals {
		if v == "" {
			return nil, usagef("legion "+a0(cmd)+" --help", "no value given for %s", names[i])
		}
	}
	return vals, nil
}

// envNames lists variable names for a preview, secrets marked, no values.
func (c *containerConfig) envNames() []string {
	if c == nil {
		return nil
	}
	var out []string
	for _, e := range c.Env {
		if e.Secret {
			out = append(out, e.Name+" (secret)")
		} else {
			out = append(out, e.Name)
		}
	}
	return out
}

// write prints what will be configured. Secret values are never printed;
// plain values are, since the person typed them and the API returns them.
func (c *containerConfig) write(w io.Writer) {
	if c.empty() {
		return
	}
	for _, e := range c.Env {
		if e.Secret {
			fmt.Fprintf(w, "  Env:       %s = (secret, hidden)\n", san(e.Name))
		} else {
			fmt.Fprintf(w, "  Env:       %s = %s\n", san(e.Name), san(e.Value))
		}
	}
	keys := make([]string, 0, len(c.Settings))
	for k := range c.Settings {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(w, "  Setting:   %s = %s\n", san(k), san(fmt.Sprint(c.Settings[k])))
	}
	if c.hasCommand {
		fmt.Fprintf(w, "  Command:   %s\n", san(strings.Join(c.Command, " ")))
	}
	if c.hasArgs {
		fmt.Fprintf(w, "  Args:      %s\n", san(strings.Join(c.Args, " ")))
	}
}
