package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/AmbientCrypto/legion-controller/api"
	"github.com/spf13/cobra"
)

func (a *app) completionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "completion bash|zsh|fish|powershell",
		Short: "Generate a shell completion script",
		Long: `Print a completion script for your shell. Completion also suggests live values: rental names,
host names, catalog ids, saved SSH key labels and GPU models (it asks the API with a short
timeout and stays silent if that fails).

Install:
  bash        source <(legion completion bash)                       # add to ~/.bashrc
              legion completion bash > /etc/bash_completion.d/legion  # or system-wide (needs bash-completion)
  zsh         legion completion zsh > "${fpath[1]}/_legion"           # then restart the shell
              (if completion is not enabled yet: echo "autoload -U compinit; compinit" >> ~/.zshrc)
  fish        legion completion fish > ~/.config/fish/completions/legion.fish
  powershell  legion completion powershell | Out-String | Invoke-Expression   # add to $PROFILE`,
		Example: `  source <(legion completion bash)
  legion completion zsh > "${fpath[1]}/_legion"`,
		ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
		Args: func(cmd *cobra.Command, as []string) error {
			if len(as) != 1 {
				return usagef("legion completion --help", "choose a shell: bash, zsh, fish or powershell")
			}
			return cobra.OnlyValidArgs(cmd, as)
		},
		RunE: func(cmd *cobra.Command, as []string) error {
			root := cmd.Root()
			switch as[0] {
			case "bash":
				return root.GenBashCompletionV2(a.env.Stdout, true)
			case "zsh":
				return root.GenZshCompletion(a.env.Stdout)
			case "fish":
				return root.GenFishCompletion(a.env.Stdout, true)
			}
			return root.GenPowerShellCompletionWithDesc(a.env.Stdout)
		},
	}
}

type completer func(ctx context.Context, c *api.ClientWithResponses) ([]string, error)

// complete wraps a fetch as a cobra completion func: short timeout, any
// failure (no login, network, API error) yields no suggestions.
func (a *app) complete(maxArgs int, fetch completer) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, as []string, prefix string) ([]string, cobra.ShellCompDirective) {
		none := cobra.ShellCompDirectiveNoFileComp
		if len(as) >= maxArgs {
			return nil, none
		}
		a.timeout = 2 * time.Second
		c, err := a.api()
		if err != nil {
			return nil, none
		}
		ctx, cancel := context.WithTimeout(a.ctx(cmd), 2*time.Second)
		defer cancel()
		vals, err := fetch(ctx, c)
		if err != nil {
			return nil, none
		}
		var out []string
		for _, v := range vals {
			if strings.HasPrefix(v, prefix) {
				out = append(out, v)
			}
		}
		return out, none
	}
}

func (a *app) rentalNamesFor(ctx context.Context, c *api.ClientWithResponses) ([]string, error) {
	rs, err := a.listRentals(ctx, c)
	var out []string
	for _, r := range rs {
		if r.State != api.RentalStateDeleted {
			out = append(out, fmt.Sprintf("%s\t%s, %d GPU", r.Name, r.State, r.GpuCount))
		}
	}
	return out, err
}

func (a *app) offerNamesFor(ctx context.Context, c *api.ClientWithResponses) ([]string, error) {
	os, err := a.listOffers(ctx, c, &api.ListOffersParams{}, 50)
	var out []string
	for _, o := range os {
		out = append(out, fmt.Sprintf("%s\t%dx %s, %d free", o.HostName, o.GpuCount, o.GpuModel, o.GpusFree))
	}
	return out, err
}

func (a *app) gpuModelsFor(ctx context.Context, c *api.ClientWithResponses) ([]string, error) {
	os, err := a.listOffers(ctx, c, &api.ListOffersParams{}, 100)
	seen := map[string]bool{}
	var out []string
	for _, o := range os {
		if !seen[o.GpuModel] {
			seen[o.GpuModel] = true
			out = append(out, o.GpuModel)
		}
	}
	sort.Strings(out)
	return out, err
}

func (a *app) catalogIDsFor(ctx context.Context, c *api.ClientWithResponses) ([]string, error) {
	es, err := a.catalogFor(ctx, c, nil)
	var out []string
	for _, e := range es {
		out = append(out, fmt.Sprintf("%s\t%s", e.Id, e.Name))
	}
	return out, err
}

func (a *app) sshKeyLabelsFor(ctx context.Context, c *api.ClientWithResponses) ([]string, error) {
	ks, err := a.sshKeys(ctx, c)
	var out []string
	for _, k := range ks {
		out = append(out, k.Label+"\t"+k.Fingerprint)
	}
	return out, err
}

// wireCompletions attaches dynamic completion to the commands and flags that
// take a rental, host, catalog id, key label or GPU model.
func (a *app) wireCompletions(root *cobra.Command) {
	set := func(path string, max int, f completer) {
		if c, _, err := root.Find(strings.Fields(path)); err == nil && c != root {
			c.ValidArgsFunction = a.complete(max, f)
		}
	}
	for _, p := range []string{"rental get", "rental stop", "rental start", "rental delete", "rental wait", "rental ssh", "rental ssh-config",
		"rental charges", "rental serve", "ssh", "rental ingress list", "rental ingress add", "rental ingress remove", "rental backup estimate",
		"rental backup create", "rental backup list", "rental export create", "rental export list", "rental export get", "rental export cancel"} {
		set(p, 1, a.rentalNamesFor)
	}
	set("offer get", 1, a.offerNamesFor)
	set("catalog get", 1, a.catalogIDsFor)
	set("ssh-key remove", 1, a.sshKeyLabelsFor)
	flag := func(path, name string, f completer) {
		if c, _, err := root.Find(strings.Fields(path)); err == nil {
			_ = c.RegisterFlagCompletionFunc(name, a.complete(1, f))
		}
	}
	flag("rental create", "offer", a.offerNamesFor)
	flag("rental create", "catalog", a.catalogIDsFor)
	flag("rental create", "ssh-key", a.sshKeyLabelsFor)
	flag("offer list", "gpu-model", a.gpuModelsFor)
	flag("offer list", "catalog", a.catalogIDsFor)
	flag("catalog list", "host", a.offerNamesFor)
}
