package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/AmbientCrypto/legion-controller/api"
	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/AmbientCrypto/legion-cli/internal/output"
)

func (a *app) svcKeysCmd() *cobra.Command {
	cmd := group(&cobra.Command{
		Use:     "keys",
		Aliases: []string{"key"},
		Short:   "Manage service keys",
		Long: `Service keys (lek_...) let callers reach a service, or a protected port. One key belongs to one service.
The secret is shown once, when the key is created or rotated, and is stored only as a hash. Keys expire after
90 days unless you pick 7d, 30d, 365d or never. Revoking takes effect within seconds.`,
		Example: "  legion services keys list chat\n  legion services keys create chat --name ci --expires 30d",
	})
	cmd.AddCommand(a.svcKeysListCmd(), a.svcKeysCreateCmd(), a.svcKeysRotateCmd(), a.svcKeysRevokeCmd())
	return cmd
}

func (a *app) listKeys(ctx context.Context, c *api.ClientWithResponses, id api.ServiceID) ([]api.ServiceKey, error) {
	return collect(0, func(cur *string, n int) ([]api.ServiceKey, *string, error) {
		r, err := c.ListServiceKeysWithResponse(ctx, id, &api.ListServiceKeysParams{Cursor: cur, Limit: &n})
		if err := a.ok(r, err); err != nil {
			return nil, nil, err
		}
		p, err := need(r.JSON200)
		if err != nil {
			return nil, nil, err
		}
		return p.Items, p.NextCursor, nil
	})
}

// findKey resolves a key by id, name or prefix among the keys that are not revoked.
func findKey(keys []api.ServiceKey, arg, service string) (*api.ServiceKey, error) {
	var hits []int
	for i, k := range keys {
		if k.Id.String() == arg {
			return &keys[i], nil
		}
		if k.Status == api.ServiceKeyStatusRevoked {
			continue
		}
		if k.Name == arg || k.Prefix == arg {
			hits = append(hits, i)
		}
	}
	if len(hits) == 0 {
		for i, k := range keys {
			if k.Status != api.ServiceKeyStatusRevoked && (strings.HasPrefix(k.Name, arg) || strings.HasPrefix(k.Prefix, arg)) {
				hits = append(hits, i)
			}
		}
	}
	switch len(hits) {
	case 0:
		return nil, notFoundf("legion services keys list "+service, "no key %q on %s", arg, service)
	case 1:
		return &keys[hits[0]], nil
	}
	return nil, usagef("legion services keys list "+service+" -o wide", "%q matches several keys; use the key id", arg)
}

func keyDate(t *api.ServiceKey, which string) string {
	switch which {
	case "expires":
		if t.ExpiresAt == nil {
			return "never"
		}
		return t.ExpiresAt.Format("2006-01-02")
	case "used":
		if t.LastUsedAt == nil {
			return "-"
		}
		return t.LastUsedAt.Format("2006-01-02 15:04")
	}
	return ""
}

func (a *app) svcKeysListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list <service>",
		Aliases: []string{"ls"},
		Short:   "List a service's keys",
		Long:    "List keys with prefix, status, expiry and last use. Secrets and hashes are never shown. A key marked legacy is a direct key of a rental created before services; its direct URL is in -o wide.",
		Example: "  legion services keys list chat\n  legion services keys list chat -o wide",
		Args:    args(0, 1),
		RunE: func(cmd *cobra.Command, as []string) error {
			return a.withService(cmd, as, func(ctx context.Context, c *api.ClientWithResponses, e *api.Service) error {
				ks, err := a.listKeys(ctx, c, e.Id)
				if err != nil {
					return err
				}
				return a.emit(listResult(ks, []string{"NAME", "PREFIX", "STATUS", "EXPIRES", "LAST USED"}, []string{"ID", "KIND", "CREATED", "DIRECT URL"},
					func(k api.ServiceKey) string { return k.Name },
					func(k api.ServiceKey) []string {
						return []string{san(k.Name), san(k.Prefix), string(k.Status), keyDate(&k, "expires"), keyDate(&k, "used"),
							k.Id.String(), string(k.Kind), k.CreatedAt.Format("2006-01-02"), san(ptrOr(k.DirectUrl, ""))}
					}))
			})
		},
	}
}

func showSecret(a *app, k api.ServiceKeyCreated, service string) output.Result {
	a.info("Warning: this is the only time the secret is shown. Copy it now.")
	return output.Result{Data: k, Names: []string{k.Secret}, Human: func(w io.Writer) {
		fmt.Fprintf(w, "Name:     %s\nPrefix:   %s\nExpires:  %s\nSecret:   %s\n", san(k.Key.Name), san(k.Key.Prefix), keyDate(&k.Key, "expires"), k.Secret)
	}}
}

func (a *app) svcKeysCreateCmd() *cobra.Command {
	var name, expires string
	var rate int
	cmd := &cobra.Command{
		Use:   "create <service>",
		Short: "Create a service key",
		Long: `Create a key. The secret is shown ONCE, here, and cannot be retrieved later: copy it now. --expires takes 7d,
30d, 90d, 365d or never (default 90d); a key that never expires may need step-up verification. At most 10 active
keys per service. -o name prints only the secret.`,
		Example: "  legion services keys create chat --name ci --expires 30d\n  legion services keys create chat --name bot -q -o name",
		Args:    args(0, 1),
		RunE: func(cmd *cobra.Command, as []string) error {
			if name == "" {
				return usagef("legion services keys create --help", "missing required flag --name")
			}
			ex, never, err := parseExpiry(expires)
			if err != nil {
				return err
			}
			return a.withService(cmd, as, func(ctx context.Context, c *api.ClientWithResponses, e *api.Service) error {
				req := api.CreateServiceKeyRequest{Name: name, ExpiresInDays: ex}
				if never {
					req.NoExpiry = ptr(true)
					a.info("Warning: this key never expires. Anyone who gets it can use %s until you revoke it.", e.Name)
				}
				if cmd.Flags().Changed("rate-limit") {
					req.RateLimitPerMinute = &rate
				}
				r, err := c.CreateServiceKeyWithResponse(ctx, e.Id, &api.CreateServiceKeyParams{IdempotencyKey: uuid.NewString()}, req)
				if err := a.ok(r, err); err != nil {
					return err
				}
				k, err := need(r.JSON201)
				if err != nil {
					return err
				}
				return a.emit(showSecret(a, *k, e.Name))
			})
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "a name for the key, for example ci")
	cmd.Flags().StringVar(&expires, "expires", "", "7d, 30d, 90d, 365d or never (default 90d)")
	cmd.Flags().IntVar(&rate, "rate-limit", 0, "requests per minute this key may make (default: the service's)")
	return cmd
}

func (a *app) svcKeysRotateCmd() *cobra.Command {
	var overlapHours int
	var expires string
	cmd := &cobra.Command{
		Use:   "rotate <service> <key>",
		Short: "Replace a key with a new one, keeping the old one for a while",
		Long: `Make a new key with the same name and show its secret ONCE. The old key keeps working for the overlap
(default 24 hours) so you can move callers over, then stops. Revoke the old key at once with 'keys revoke'.`,
		Example: "  legion services keys rotate chat ci\n  legion services keys rotate chat ci --overlap-hours 12 --expires 30d",
		Args:    args(2, 2),
		RunE: func(cmd *cobra.Command, as []string) error {
			ex, never, err := parseExpiry(expires)
			if err != nil {
				return err
			}
			return a.withService(cmd, as[:1], func(ctx context.Context, c *api.ClientWithResponses, e *api.Service) error {
				ks, err := a.listKeys(ctx, c, e.Id)
				if err != nil {
					return err
				}
				k, err := findKey(ks, as[1], e.Name)
				if err != nil {
					return err
				}
				if err := a.confirm(fmt.Sprintf("Rotate key %s of %s? The old key stops after the overlap.", k.Name, e.Name), "refusing to rotate a key without confirmation"); err != nil {
					return err
				}
				var req api.RotateServiceKeyRequest
				if ex != nil {
					d := api.RotateServiceKeyRequestExpiresInDays(*ex)
					req.ExpiresInDays = &d
				}
				if never {
					req.NoExpiry = ptr(true)
				}
				if cmd.Flags().Changed("overlap-hours") {
					req.OverlapHours = &overlapHours
				}
				r, err := c.RotateServiceKeyWithResponse(ctx, e.Id, k.Id, req)
				if err := a.ok(r, err); err != nil {
					return err
				}
				rot, err := need(r.JSON201)
				if err != nil {
					return err
				}
				a.info("The old key %s stays valid until its overlap ends.", san(rot.OldKey.Prefix))
				return a.emit(showSecret(a, rot.NewKey, e.Name))
			})
		},
	}
	cmd.Flags().IntVar(&overlapHours, "overlap-hours", 24, "hours the old key keeps working")
	cmd.Flags().StringVar(&expires, "expires", "", "expiry of the new key: 7d, 30d, 90d, 365d or never (default 90d)")
	return cmd
}

func (a *app) svcKeysRevokeCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "revoke <service> <key>",
		Aliases: []string{"rm"},
		Short:   "Revoke a key",
		Long:    "Revoke a key by id, name or prefix. Callers using it get 401 within seconds. Revoking a legacy direct key removes direct access to the rental; the service URL keeps working.",
		Example: "  legion services keys revoke chat ci\n  legion services keys revoke chat ci --yes",
		Args:    args(2, 2),
		RunE: func(cmd *cobra.Command, as []string) error {
			return a.withService(cmd, as[:1], func(ctx context.Context, c *api.ClientWithResponses, e *api.Service) error {
				ks, err := a.listKeys(ctx, c, e.Id)
				if err != nil {
					return err
				}
				k, err := findKey(ks, as[1], e.Name)
				if err != nil {
					return err
				}
				if err := a.confirm(fmt.Sprintf("Revoke key %s (%s) of %s? Anything using it stops working.", k.Name, k.Prefix, e.Name), "refusing to revoke a key without confirmation"); err != nil {
					return err
				}
				r, err := c.RevokeServiceKeyWithResponse(ctx, e.Id, k.Id)
				if err := a.ok(r, err); err != nil {
					return err
				}
				a.info("Revoked key %s.", san(k.Name))
				return nil
			})
		},
	}
}
