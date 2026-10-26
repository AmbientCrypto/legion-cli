package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/AmbientCrypto/legion-cli/internal/config"
	"github.com/AmbientCrypto/legion-cli/internal/output"
)

func (a *app) configCmd() *cobra.Command {
	keys := strings.Join(config.Keys, ", ")
	cmd := group(&cobra.Command{
		Use:   "config",
		Short: "Read and change settings of the current profile",
		Long: `Settings are stored per profile in config.yaml. Keys: ` + keys + `.
Precedence is flag, then environment, then profile, then default: 'legion help environment'.
'legion auth status' shows everything in effect; 'legion profile list' shows the profiles.`,
		Example: `  legion config get api_url
  legion config set output json`,
	})
	get := &cobra.Command{
		Use:     "get <key>",
		Short:   "Print the effective value of one setting",
		Long:    "Print one setting's effective value (after flags and environment). Keys: " + keys + ".",
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
				return usagef("legion config get --help", "%v", err)
			}
			return nil
		},
	}
	set := &cobra.Command{
		Use:     "set <key> <value>",
		Short:   "Change a setting of the current profile",
		Long:    "Store a setting in the current profile (or the one chosen with -p). Keys: " + keys + ".",
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
				return usagef("legion config set --help", "%v", err)
			}
			if err := a.st.SaveConfig(a.cfg); err != nil {
				return err
			}
			a.info("Set %s for profile %s.", as[0], r.Profile)
			return nil
		},
	}
	cmd.AddCommand(get, set)
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
		Long:    "List the profiles in config.yaml and credentials.yaml. The one in use is marked with *.",
		Example: "  legion profile list\n  legion profile list -o name",
		Args:    args(0, 0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			r, err := a.conf()
			if err != nil {
				return err
			}
			res := listResult(config.Names(a.cfg, a.creds), []string{"", "PROFILE", "API URL", "LOGGED IN"}, nil,
				func(n string) string { return n },
				func(n string) []string {
					mark, url, login := "", config.DefaultAPIURL, "no"
					if n == r.Profile {
						mark = "*"
					}
					if p := a.cfg.Profiles[n]; p != nil && p.APIURL != "" {
						url = p.APIURL
					}
					if c := a.creds.Profiles[n]; c != nil && c.APIKey != "" {
						login = "yes"
					}
					return []string{mark, n, url, login}
				})
			return a.emit(res)
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
				if err := p.Set("output", a.flagOut); err != nil {
					return usagef("legion profile create --help", "%v", err)
				}
			}
			if a.cfg.CurrentProfile == "" {
				a.cfg.CurrentProfile = as[0]
			}
			if err := a.st.SaveConfig(a.cfg); err != nil {
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
			if err := a.st.SaveConfig(a.cfg); err != nil {
				return err
			}
			a.info("Now using profile %s.", as[0])
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
			if err := a.st.SaveCreds(a.creds); err != nil {
				return err
			}
			if err := a.st.SaveConfig(a.cfg); err != nil {
				return err
			}
			a.info("Deleted profile %s.", as[0])
			return nil
		},
	}
	cmd.AddCommand(list, create, use, del)
	return cmd
}

func (a *app) profileExists(name string) bool {
	return a.cfg.Profiles[name] != nil || a.creds.Profiles[name] != nil || name == config.DefaultProfile
}
