package cli

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/AmbientCrypto/legion-controller/api"
	"github.com/spf13/cobra"

	"github.com/AmbientCrypto/legion-cli/internal/client"
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

	var noBrowser, withToken, paste bool
	login := &cobra.Command{
		Use:   "login",
		Short: "Sign in through the browser and save an API key",
		Long: `Sign in through the browser. legion opens Legion's sign-in page, you approve
"Legion CLI" there, and legion saves the API key the approval creates: scope
"full", named "Legion CLI (<this computer>)", valid for 90 days. It is listed in
Settings → API keys, where you can revoke it; legion auth logout revokes it too.

The browser must run on this computer (the sign-in returns to a port on
127.0.0.1). On a remote machine use --paste: create a key in ` + apiKeysURL + `
and paste it at the hidden prompt.

For scripts and CI pipe a key in with --with-token, or skip login and set LEGION_API_KEY.`,
		Example: `  legion auth login
  legion auth login --no-browser
  legion auth login --paste
  echo "$LEGION_KEY" | legion auth login --with-token --profile ci`,
		Args: args(0, 0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			r, err := a.conf()
			if err != nil {
				return err
			}
			var key, minted string
			switch {
			case withToken:
				b, err := io.ReadAll(a.env.Stdin)
				if err != nil {
					return err
				}
				key = strings.TrimSpace(string(b))
			case !a.interactive():
				return usagef("echo $KEY | legion auth login --with-token", "login needs a terminal; pass a key on stdin with --with-token")
			case !paste:
				if key, minted, err = a.browserLogin(cmd, r.APIURL, !noBrowser); err != nil {
					return err
				}
			default:
				if !noBrowser {
					if err := a.env.OpenBrowser(apiKeysURL); err != nil {
						a.info("Could not open a browser (%v).", err)
					}
				}
				fmt.Fprintf(a.env.Stderr, "Open %s and create an API key with scope \"full\".\nPaste the key (it is hidden: nothing is shown), then press Enter: ", apiKeysURL)
				key, err = a.env.ReadSecret(a.ctx(cmd))
				if err != nil {
					return err
				}
				key = strings.TrimSpace(key)
			}
			if key == "" {
				return usagef("legion auth login --help", "no API key given")
			}
			if r.APIURL != config.DefaultAPIURL {
				a.info("Sending your key to %s", hostOf(r.APIURL))
			}
			me, err := a.whoami(cmd, r.APIURL, key)
			if err != nil {
				return err
			}
			if err := a.saveKey(r.Profile, key, minted != ""); err != nil {
				return err
			}
			a.info("Logged in as %s (org %s)", san(me.User.Name), san(me.Org.Name))
			if minted != "" {
				a.info("Saved the API key %q (expires in 90 days; revoke it in Settings → API keys or with legion auth logout).", san(minted))
			}
			if sc := a.keyScope(cmd, r.APIURL, key, me); sc != "" && sc != string(api.ApiKeyScopeFull) {
				a.info("Warning: this key has scope %q; most commands need scope \"full\".", sc)
			}
			a.info("Next: legion offer list")
			return nil
		},
	}
	login.Flags().BoolVar(&noBrowser, "no-browser", false, "print the sign-in URL instead of opening a browser")
	login.Flags().BoolVar(&paste, "paste", false, "paste an API key made in Settings → API keys instead of signing in through the browser")
	login.Flags().BoolVar(&withToken, "with-token", false, "read the API key from stdin")

	logout := &cobra.Command{
		Use:     "logout",
		Short:   "Revoke the key legion auth login made and forget it",
		Long:    "Remove the API key of the current profile from credentials.yaml. A key made by browser sign-in is revoked first; a pasted key stays valid (revoke it with: legion api-key revoke).",
		Example: "  legion auth logout\n  legion auth logout --profile staging",
		Args:    args(0, 0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			r, err := a.conf()
			if err != nil {
				return err
			}
			if c := a.creds.Profiles[r.Profile]; c != nil {
				if c.BrowserLogin {
					a.revokeOwnKey(cmd, r.APIURL, c.APIKey)
				}
				delete(a.creds.Profiles, r.Profile)
				if err := a.st.SaveCreds(a.creds); err != nil {
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
			scope := a.keyScope(cmd, r.APIURL, r.APIKey, me)
			if scope == "" {
				scope = "unknown"
			}
			data := map[string]any{"profile": r.Profile, "api_url": r.APIURL, "key": short(r.APIKey), "key_source": src, "scope": scope, "me": me}
			return a.emit(output.Result{Data: data, Human: func(w io.Writer) {
				fmt.Fprintf(w, "Profile:  %s\nAPI URL:  %s\nKey:      %s (%s)\nScope:    %s\n", r.Profile, r.APIURL, short(r.APIKey), src, scope)
				fmt.Fprintf(w, "User:     %s <%s>\nOrg:      %s (%s)\n", san(me.User.Name), san(string(me.User.Email)), san(me.Org.Name), me.Role)
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
				fmt.Fprintf(w, "%s <%s>\nOrg: %s (%s)\n", san(me.User.Name), san(string(me.User.Email)), san(me.Org.Name), me.Role)
			}})
		},
	}
}

func hostOf(raw string) string {
	if u, err := url.Parse(raw); err == nil {
		return u.Host
	}
	return raw
}

// keyScope is the scope of the key in use: from /v1/me, else looked up in the
// key list by id. Empty when it cannot be found.
func (a *app) keyScope(cmd *cobra.Command, apiURL, key string, me *api.Me) string {
	cr := me.Credential
	if cr == nil {
		return ""
	}
	if cr.Scope != "" {
		return string(cr.Scope)
	}
	if cr.ApiKeyId == nil {
		return ""
	}
	c, err := a.newClient(apiURL, key)
	if err != nil {
		return ""
	}
	keys, err := a.apiKeys(a.ctx(cmd), c)
	if err != nil {
		return ""
	}
	for _, k := range keys {
		if k.Id == *cr.ApiKeyId {
			return string(k.Scope)
		}
	}
	return ""
}

// whoami calls getMe with the given key.
func (a *app) whoami(cmd *cobra.Command, apiURL, key string) (*api.Me, error) {
	c, err := a.newClient(apiURL, key)
	if err != nil {
		return nil, err
	}
	resp, err := c.GetMeWithResponse(a.ctx(cmd))
	if err := a.ok(resp, err); err != nil {
		return nil, err
	}
	return need(resp.JSON200)
}

// revokeOwnKey revokes the key browser sign-in made. Best effort: logout
// forgets the key either way, and says how to revoke it by hand when it can't.
func (a *app) revokeOwnKey(cmd *cobra.Command, apiURL, key string) {
	me, err := a.whoami(cmd, apiURL, key)
	if err != nil {
		var ce *client.Error
		if errors.As(err, &ce) && ce.Status == 401 {
			return // already revoked or expired
		}
		a.info("Could not revoke the key (%v); revoke it in %s.", err, apiKeysURL)
		return
	}
	if me.Credential == nil || me.Credential.ApiKeyId == nil {
		a.info("Could not tell which key this is; revoke it in %s.", apiKeysURL)
		return
	}
	c, err := a.newClient(apiURL, key)
	if err == nil {
		var resp *api.RevokeApiKeyResult
		resp, err = c.RevokeApiKeyWithResponse(a.ctx(cmd), *me.Credential.ApiKeyId)
		err = a.ok(resp, err)
	}
	if err != nil {
		a.info("Could not revoke the key (%v); revoke it in %s.", err, apiKeysURL)
		return
	}
	a.info("Revoked the API key %s.", short(key))
}

func (a *app) saveKey(profile, key string, browserLogin bool) error {
	if a.creds.Profiles == nil {
		a.creds.Profiles = map[string]*config.Cred{}
	}
	a.creds.Profiles[profile] = &config.Cred{APIKey: key, BrowserLogin: browserLogin}
	if err := a.st.SaveCreds(a.creds); err != nil {
		return err
	}
	a.cfg.Ensure(profile)
	if a.cfg.CurrentProfile == "" {
		a.cfg.CurrentProfile = profile
	}
	return a.st.SaveConfig(a.cfg)
}
