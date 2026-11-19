package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/AmbientCrypto/legion-controller/api"
)

// ---- container configuration on the wire (LGN-628) ----
//
// An update REPLACES the lists on the server, and a secret sent without a
// value keeps its stored value. So env changes are read-modify-write here:
// the stored names come back, plain ones with values, secrets without, and
// only what the person named changes.

func (c *containerConfig) envInputs() *[]api.EnvVarInput {
	if c == nil || len(c.Env) == 0 {
		return nil
	}
	return ptr(envInputs(c.Env))
}

func envInputs(vs []envVar) []api.EnvVarInput {
	out := make([]api.EnvVarInput, 0, len(vs))
	for _, v := range vs {
		in := api.EnvVarInput{Name: v.Name, Value: ptr(v.Value)}
		if v.Secret {
			in.Secret = ptr(true)
		}
		out = append(out, in)
	}
	return out
}

func settingsToAPI(m map[string]any) api.ModelSettings {
	out := api.ModelSettings{}
	for k, v := range m {
		var mv api.ModelSettingValue
		switch x := v.(type) {
		case int64:
			_ = mv.FromModelSettingValue0(x)
		case bool:
			_ = mv.FromModelSettingValue1(x)
		default:
			_ = mv.FromModelSettingValue2(fmt.Sprint(x))
		}
		out[k] = mv
	}
	return out
}

func (c *containerConfig) modelSettings() *api.ModelSettings {
	if c == nil || len(c.Settings) == 0 {
		return nil
	}
	return ptr(settingsToAPI(c.Settings))
}

func (c *containerConfig) commandOverride() *api.CommandOverride {
	if c == nil || !c.hasCommand {
		return nil
	}
	return ptr(api.CommandOverride(c.Command))
}

func (c *containerConfig) argsOverride() *api.CommandOverride {
	if c == nil || !c.hasArgs {
		return nil
	}
	return ptr(api.CommandOverride(c.Args))
}

// mergeEnv applies set and unset over the stored variables. A stored secret
// that is not replaced goes back without a value, which keeps it.
func mergeEnv(cur []api.EnvVar, set []envVar, unset []string) []api.EnvVarInput {
	drop := map[string]bool{}
	for _, n := range unset {
		drop[n] = true
	}
	for _, v := range set {
		drop[v.Name] = true
	}
	out := []api.EnvVarInput{}
	for _, e := range cur {
		if drop[e.Name] {
			continue
		}
		in := api.EnvVarInput{Name: e.Name}
		if e.Secret {
			in.Secret = ptr(true)
		} else {
			in.Value = e.Value
			if in.Value == nil {
				in.Value = ptr("")
			}
		}
		out = append(out, in)
	}
	return append(out, envInputs(set)...)
}

// mergeSettings overlays set on the stored settings.
func mergeSettings(cur *api.ModelSettings, set map[string]any) api.ModelSettings {
	out := api.ModelSettings{}
	if cur != nil {
		for k, v := range *cur {
			out[k] = v
		}
	}
	for k, v := range settingsToAPI(set) {
		out[k] = v
	}
	return out
}

func settingText(v api.ModelSettingValue) string {
	b, _ := json.Marshal(v)
	return strings.Trim(string(b), `"`)
}

// writeStoredConfig prints what is stored: variable names with the secret
// flag (a secret has no value to print), settings, command and args.
func writeStoredConfig(w io.Writer, env *[]api.EnvVar, ms *api.ModelSettings, cmd, args *api.CommandOverride) {
	if env != nil {
		for _, e := range *env {
			switch {
			case e.Secret:
				fmt.Fprintf(w, "Env:        %s = (secret, set, hidden)\n", san(e.Name))
			default:
				fmt.Fprintf(w, "Env:        %s = %s\n", san(e.Name), san(ptrOr(e.Value, "")))
			}
		}
	}
	if ms != nil {
		keys := make([]string, 0, len(*ms))
		for k := range *ms {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(w, "Setting:    %s = %s\n", san(k), san(settingText((*ms)[k])))
		}
	}
	if cmd != nil && len(*cmd) > 0 {
		fmt.Fprintf(w, "Command:    %s\n", san(strings.Join(*cmd, " ")))
	}
	if args != nil && len(*args) > 0 {
		fmt.Fprintf(w, "Args:       %s\n", san(strings.Join(*args, " ")))
	}
}

func ptrOrSlice[T any](p *[]T) []T {
	if p == nil {
		return nil
	}
	return *p
}
