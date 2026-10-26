package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/AmbientCrypto/legion-cli/internal/config"
	"github.com/AmbientCrypto/legion-cli/internal/output"
)

func (a *app) configCmd() *cobra.Command {
	cmd := group(&cobra.Command{
		Use:   "config",
		Short: "Read and change settings of the current profile",
		Long: `Settings are stored per profile in config.yaml. Keys: api_url, output.
Precedence is flag, then environment, then profile, then default: 'legion help environment'.`,
		Example: `  legion config list
  legion config set output json`,
	})
	list := &cobra.Command{
		Use:     "list",
		Short:   "Show every setting, its effective value and where it comes from",
		Long:    "Show the effective value of each setting and its source: flag, env, profile or default. The API key is masked.",
		Example: "  legion config list\n  legion config list -p staging -o json",
		Args:    args(0, 0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			r, err := a.conf()
			if err != nil {
				return err
			}
			key := "(not set)"
			if r.APIKey != "" {
				key = short(r.APIKey)
			}
			rows := [][]string{
				{"profile", r.Profile, r.Sources["profile"]},
				{"api_url", r.APIURL, r.Sources["api_url"]},
				{"output", r.Output, r.Sources["output"]},
				{"api_key", key, r.Sources["api_key"]},
			}
			data := map[string]any{}
			for _, row := range rows {
				data[row[0]] = map[string]string{"value": row[1], "source": row[2]}
			}
			return a.emit(output.Result{Data: data, Table: &output.Table{Cols: []string{"KEY", "VALUE", "SOURCE"}, Rows: rows}})
		},
	}
	get := &cobra.Command{
		Use:     "get <key>",
		Short:   "Print the effective value of one setting",
		Long:    "Print one setting's effective value (after flags and environment). Keys: " + strings.Join(config.Keys, ", ") + ".",
		Example: "  legion config get api_url\n  legion config get output -p staging",
		Args:    args(1, 1),
		RunE: func(cmd *cobra.Command, as []string) error {
			r, err := a.conf()
			if err != nil {
				return err
			}
			switch as[0] {
			case "api_url":
				fmt.Fprintln(a.env.Stdout, r.APIURL)
			case "output":
				fmt.Fprintln(a.env.Stdout, r.Output)
			default:
				_, err := (&config.Profile{}).Get(as[0])
				return usagef("legion config list", "%v", err)
			}
			return nil
		},
	}
	set := &cobra.Command{
		Use:     "set <key> <value>",
		Short:   "Change a setting of the current profile",
		Long:    "Store a setting in the current profile (or the one chosen with -p). Keys: " + strings.Join(config.Keys, ", ") + ".",
		Example: "  legion config set output json\n  legion config set api_url https://api.staging.example -p staging",
		Args:    args(2, 2),
		RunE: func(cmd *cobra.Command, as []string) error {
			r, err := a.conf()
			if err != nil {
				return err
			}
			if as[0] == "output" {
				if _, err := output.Parse(as[1]); err != nil {
					return usagef("legion config set --help", "%v", err)
				}
			}
			if err := a.cfg.Ensure(r.Profile).Set(as[0], as[1]); err != nil {
				return usagef("legion config list", "%v", err)
			}
			if err := a.store().SaveConfig(a.cfg); err != nil {
				return err
			}
			a.info("Set %s for profile %s.", as[0], r.Profile)
			return nil
		},
	}
	unset := &cobra.Command{
		Use:     "unset <key>",
		Short:   "Remove a setting from the current profile",
		Long:    "Remove a setting so the default applies again.",
		Example: "  legion config unset output\n  legion config unset api_url -p staging",
		Args:    args(1, 1),
		RunE: func(cmd *cobra.Command, as []string) error {
			r, err := a.conf()
			if err != nil {
				return err
			}
			if _, err := (&config.Profile{}).Get(as[0]); err != nil {
				return usagef("legion config list", "%v", err)
			}
			a.cfg.Ensure(r.Profile).Unset(as[0])
			if err := a.store().SaveConfig(a.cfg); err != nil {
				return err
			}
			a.info("Unset %s for profile %s.", as[0], r.Profile)
			return nil
		},
	}
	cmd.AddCommand(list, get, set, unset)
	return cmd
}

func (a *app) profileCmd() *cobra.Command {
	cmd := group(&cobra.Command{
		Use:   "profile",
		Short: "Manage named profiles (API URL, key, default output)",
		Long: `A profile bundles an API URL, an API key and a default output format, so you can
switch between accounts or environments. Select one with 'legion profile use',
-p/--profile or LEGION_PROFILE.`,
		Example: `  legion profile create staging --api-url https://api.staging.example
  legion profile use staging`,
	})
	list := &cobra.Command{
		Use:     "list",
		Short:   "List profiles",
		Long:    "List the profiles in config.yaml and credentials.yaml. The current one is marked with *.",
		Example: "  legion profile list\n  legion profile list -o name",
		Args:    args(0, 0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			r, err := a.conf()
			if err != nil {
				return err
			}
			names := config.Names(a.cfg, a.creds)
			var rows [][]string
			for _, n := range names {
				mark := ""
				if n == r.Profile {
					mark = "*"
				}
				var p config.Profile
				if a.cfg.Profiles[n] != nil {
					p = *a.cfg.Profiles[n]
				}
				login := "no"
				if a.creds.Profiles[n] != nil && a.creds.Profiles[n].APIKey != "" {
					login = "yes"
				}
				if p.APIURL == "" {
					p.APIURL = config.DefaultAPIURL
				}
				rows = append(rows, []string{mark, n, p.APIURL, login})
			}
			return a.emit(output.Result{Data: names, Names: names, Table: &output.Table{Cols: []string{"", "PROFILE", "API URL", "LOGGED IN"}, Rows: rows}})
		},
	}
	create := &cobra.Command{
		Use:     "create <name>",
		Short:   "Create a profile",
		Long:    "Create a profile. Use --api-url and -o to set its API URL and default output; log in to it with: legion auth login -p <name>.",
		Example: "  legion profile create staging --api-url https://api.staging.example\n  legion profile create work -o wide",
		Args:    args(1, 1),
		RunE: func(cmd *cobra.Command, as []string) error {
			if _, err := a.conf(); err != nil {
				return err
			}
			if a.cfg.Profiles[as[0]] != nil {
				return &cliError{Msg: fmt.Sprintf("profile %q already exists", as[0]), Code: "exists", Hint: "legion profile use " + as[0], Exit: 5}
			}
			p := a.cfg.Ensure(as[0])
			if a.flagURL != "" {
				if err := p.Set("api_url", a.flagURL); err != nil {
					return usagef("legion profile create --help", "%v", err)
				}
			}
			if a.flagOut != "" {
				if _, err := output.Parse(a.flagOut); err != nil {
					return usagef("legion profile create --help", "%v", err)
				}
				p.Output = a.flagOut
			}
			if a.cfg.CurrentProfile == "" {
				a.cfg.CurrentProfile = as[0]
			}
			if err := a.store().SaveConfig(a.cfg); err != nil {
				return err
			}
			a.info("Created profile %s.", as[0])
			a.info("Next: legion auth login --profile %s", as[0])
			return nil
		},
	}
	use := &cobra.Command{
		Use:     "use <name>",
		Short:   "Make a profile the default",
		Long:    "Make a profile the one used when -p and LEGION_PROFILE are not given.",
		Example: "  legion profile use staging\n  legion profile use default",
		Args:    args(1, 1),
		RunE: func(cmd *cobra.Command, as []string) error {
			if _, err := a.conf(); err != nil {
				return err
			}
			if !a.profileExists(as[0]) {
				return notFoundf("legion profile list", "no profile named %q", as[0])
			}
			a.cfg.Ensure(as[0])
			a.cfg.CurrentProfile = as[0]
			if err := a.store().SaveConfig(a.cfg); err != nil {
				return err
			}
			a.info("Now using profile %s.", as[0])
			return nil
		},
	}
	current := &cobra.Command{
		Use:     "current",
		Short:   "Print the profile in use",
		Long:    "Print the profile in effect: -p, then LEGION_PROFILE, then the one set with 'legion profile use'.",
		Example: "  legion profile current\n  LEGION_PROFILE=ci legion profile current",
		Args:    args(0, 0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			r, err := a.conf()
			if err != nil {
				return err
			}
			fmt.Fprintln(a.env.Stdout, r.Profile)
			return nil
		},
	}
	del := &cobra.Command{
		Use:     "delete <name>",
		Short:   "Delete a profile and its saved key",
		Long:    "Delete a profile from config.yaml and its API key from credentials.yaml. The key itself stays valid; revoke it with: legion api-key revoke.",
		Example: "  legion profile delete staging --yes\n  legion profile delete old",
		Args:    args(1, 1),
		RunE: func(cmd *cobra.Command, as []string) error {
			if _, err := a.conf(); err != nil {
				return err
			}
			if !a.profileExists(as[0]) {
				return notFoundf("legion profile list", "no profile named %q", as[0])
			}
			if err := a.confirm(fmt.Sprintf("Delete profile %s and its saved API key?", as[0]), "refusing to delete a profile without confirmation"); err != nil {
				return err
			}
			delete(a.cfg.Profiles, as[0])
			delete(a.creds.Profiles, as[0])
			if a.cfg.CurrentProfile == as[0] {
				a.cfg.CurrentProfile = ""
			}
			if err := a.store().SaveCreds(a.creds); err != nil {
				return err
			}
			if err := a.store().SaveConfig(a.cfg); err != nil {
				return err
			}
			a.info("Deleted profile %s.", as[0])
			return nil
		},
	}
	cmd.AddCommand(list, create, use, current, del)
	return cmd
}

func (a *app) profileExists(name string) bool {
	return a.cfg.Profiles[name] != nil || a.creds.Profiles[name] != nil || name == config.DefaultProfile
}
