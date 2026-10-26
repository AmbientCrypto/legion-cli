package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/AmbientCrypto/legion-controller/api"
	"github.com/spf13/cobra"

	"github.com/AmbientCrypto/legion-cli/internal/config"
	"github.com/AmbientCrypto/legion-cli/internal/output"
)

const apiKeysURL = "https://legion.ambient.xyz/settings/api-keys"

func (a *app) authCmd() *cobra.Command {
	cmd := group(&cobra.Command{
		Use:   "auth",
		Short: "Log in and out, and see who you are logged in as",
		Long:  "Manage the API key legion uses. Keys are stored per profile in credentials.yaml (mode 0600).",
		Example: `  legion auth login
  legion auth status`,
	})

	var noBrowser, withToken bool
	login := &cobra.Command{
		Use:   "login",
		Short: "Save an API key and check that it works",
		Long: `Log in with an API key. legion opens ` + apiKeysURL + ` in your browser:
create a key with scope "full" there, copy it, and paste it at the hidden prompt.
The key is checked against the API before it is saved.

For scripts and CI pipe the key in with --with-token, or skip login and set LEGION_API_KEY.`,
		Example: `  legion auth login
  legion auth login --no-browser
  echo "$LEGION_KEY" | legion auth login --with-token --profile ci`,
		Args: args(0, 0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			r, err := a.conf()
			if err != nil {
				return err
			}
			var key string
			switch {
			case withToken:
				b, err := io.ReadAll(a.env.Stdin)
				if err != nil {
					return err
				}
				key = strings.TrimSpace(string(b))
			case !a.interactive():
				return usagef("echo $KEY | legion auth login --with-token", "login needs a terminal to paste the key; pass it on stdin with --with-token")
			default:
				if !noBrowser {
					if err := a.env.OpenBrowser(apiKeysURL); err != nil {
						a.info("Could not open a browser (%v).", err)
					}
				}
				fmt.Fprintf(a.env.Stderr, "Open %s and create an API key with scope \"full\".\nPaste the key (input is hidden): ", apiKeysURL)
				key, err = a.env.ReadSecret()
				if err != nil {
					return err
				}
				key = strings.TrimSpace(key)
			}
			if key == "" {
				return usagef("legion auth login --help", "no API key given")
			}
			me, err := a.whoami(cmd, r.APIURL, key)
			if err != nil {
				return err
			}
			if err := a.saveKey(r.Profile, key); err != nil {
				return err
			}
			a.info("Logged in as %s (org %s)", me.User.Name, me.Org.Name)
			if me.Credential != nil && me.Credential.Scope != api.ApiKeyScopeFull {
				a.info("Warning: this key has scope %q; most commands need scope \"full\".", me.Credential.Scope)
			}
			a.info("Next: legion offer list")
			return nil
		},
	}
	login.Flags().BoolVar(&noBrowser, "no-browser", false, "do not open the browser")
	login.Flags().BoolVar(&withToken, "with-token", false, "read the API key from stdin")

	logout := &cobra.Command{
		Use:     "logout",
		Short:   "Forget the saved API key",
		Long:    "Remove the API key of the current profile from credentials.yaml. The key itself stays valid; revoke it with: legion api-key revoke.",
		Example: "  legion auth logout\n  legion auth logout --profile staging",
		Args:    args(0, 0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			r, err := a.conf()
			if err != nil {
				return err
			}
			if c := a.creds.Profiles[r.Profile]; c != nil {
				delete(a.creds.Profiles, r.Profile)
				if err := a.store().SaveCreds(a.creds); err != nil {
					return err
				}
			}
			a.info("Logged out of profile %s.", r.Profile)
			if a.env.Getenv("LEGION_API_KEY") != "" {
				a.info("LEGION_API_KEY is still set in your environment.")
			}
			return nil
		},
	}

	status := &cobra.Command{
		Use:     "status",
		Short:   "Show the profile, API URL and who the key belongs to",
		Long:    "Check the saved (or LEGION_API_KEY) key against the API and show the account it belongs to. Exits 3 when not logged in.",
		Example: "  legion auth status\n  legion auth status -o json",
		Args:    args(0, 0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			r, err := a.conf()
			if err != nil {
				return err
			}
			if r.APIKey == "" {
				return errNotLoggedIn
			}
			me, err := a.whoami(cmd, r.APIURL, r.APIKey)
			if err != nil {
				return err
			}
			src := "credentials.yaml"
			if r.Sources["api_key"] == "env" {
				src = "LEGION_API_KEY"
			}
			scope := ""
			if me.Credential != nil {
				scope = string(me.Credential.Scope)
			}
			data := map[string]any{"profile": r.Profile, "api_url": r.APIURL, "key": short(r.APIKey), "key_source": src, "scope": scope, "me": me}
			return a.emit(output.Result{Data: data, Human: func(w io.Writer) {
				fmt.Fprintf(w, "Profile:  %s\nAPI URL:  %s\nKey:      %s (%s)\nScope:    %s\n", r.Profile, r.APIURL, short(r.APIKey), src, scope)
				fmt.Fprintf(w, "User:     %s <%s>\nOrg:      %s (%s)\n", me.User.Name, me.User.Email, me.Org.Name, me.Role)
			}})
		},
	}
	cmd.AddCommand(login, logout, status)
	return cmd
}

func (a *app) whoamiCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "whoami",
		Short:   "Show the user and organization you are logged in as",
		Long:    "Show the user, organization and role of the current API key. Shortcut for the identity part of 'legion auth status'.",
		Example: "  legion whoami\n  legion whoami -o jsonpath=$.org.name",
		Args:    args(0, 0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			r, err := a.conf()
			if err != nil {
				return err
			}
			if r.APIKey == "" {
				return errNotLoggedIn
			}
			me, err := a.whoami(cmd, r.APIURL, r.APIKey)
			if err != nil {
				return err
			}
			return a.emit(output.Result{Data: me, Names: []string{string(me.User.Email)}, Human: func(w io.Writer) {
				fmt.Fprintf(w, "%s <%s>\nOrg: %s (%s)\n", me.User.Name, me.User.Email, me.Org.Name, me.Role)
			}})
		},
	}
}

// whoami calls getMe with the given key.
func (a *app) whoami(cmd *cobra.Command, url, key string) (*api.Me, error) {
	c, err := a.newClient(url, key)
	if err != nil {
		return nil, err
	}
	resp, err := c.GetMeWithResponse(a.ctx(cmd))
	if err := a.ok(resp, err); err != nil {
		return nil, err
	}
	return resp.JSON200, nil
}

func (a *app) saveKey(profile, key string) error {
	if a.creds.Profiles == nil {
		a.creds.Profiles = map[string]*config.Cred{}
	}
	a.creds.Profiles[profile] = &config.Cred{APIKey: key}
	if err := a.store().SaveCreds(a.creds); err != nil {
		return err
	}
	a.cfg.Ensure(profile)
	if a.cfg.CurrentProfile == "" {
		a.cfg.CurrentProfile = profile
	}
	return a.store().SaveConfig(a.cfg)
}
