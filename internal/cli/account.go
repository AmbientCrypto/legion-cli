package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AmbientCrypto/legion-controller/api"
	"github.com/spf13/cobra"

	"github.com/AmbientCrypto/legion-cli/internal/client"
	"github.com/AmbientCrypto/legion-cli/internal/output"
)

func ts(t time.Time) string { return t.Local().Format("2006-01-02 15:04") }

func tsp(t *time.Time) string {
	if t == nil {
		return ""
	}
	return ts(*t)
}

// ---- ssh-key ----

func (a *app) sshKeyCmd() *cobra.Command {
	cmd := group(&cobra.Command{
		Use:     "ssh-key",
		Aliases: []string{"ssh-keys", "sshkey"},
		Short:   "Save the SSH public keys you rent with",
		Long:    "Saved public keys are offered when you create a rental, so you do not pass one each time. A key only grants access to rentals created after it was saved with it.",
		Example: `  legion ssh-key add
  legion ssh-key list`,
	})
	list := &cobra.Command{
		Use:     "list",
		Short:   "List saved SSH keys",
		Long:    "List the SSH public keys saved for your organization.",
		Example: "  legion ssh-key list\n  legion ssh-key list -o name",
		Args:    args(0, 0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := a.api()
			if err != nil {
				return err
			}
			keys, err := a.sshKeys(a.ctx(cmd), c)
			if err != nil {
				return err
			}
			t := &output.Table{Cols: []string{"LABEL", "TYPE", "FINGERPRINT", "CREATED"}, Wide: []string{"ID"}}
			names := make([]string, len(keys))
			for i, k := range keys {
				names[i] = k.Label
				t.Rows = append(t.Rows, []string{k.Label, string(k.KeyType), k.Fingerprint, ts(k.CreatedAt), k.Id.String()})
			}
			if len(keys) == 0 {
				a.info("No saved keys. Add one: legion ssh-key add")
			}
			return a.emit(output.Result{Data: keys, Names: names, Table: t})
		},
	}
	var file, label string
	add := &cobra.Command{
		Use:   "add",
		Short: "Save an SSH public key",
		Long: `Save a public key (default ~/.ssh/id_ed25519.pub). Only the public half is sent; a private key
file is refused. Without --label the key's comment, then the file name, is the label.`,
		Example: "  legion ssh-key add\n  legion ssh-key add --file ~/.ssh/work.pub --label work",
		Args:    args(0, 0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if file == "" {
				file = "~/.ssh/id_ed25519.pub"
			}
			b, err := os.ReadFile(expandHome(file, a.env.Home))
			if err != nil {
				return usagef("ssh-keygen -t ed25519, or: legion ssh-key add --file <path>.pub", "cannot read %s: %v", file, err)
			}
			pub := strings.TrimSpace(string(b))
			if strings.Contains(pub, "PRIVATE KEY") {
				return usagef("legion ssh-key add --file <path>.pub", "%s is a private key; pass the .pub file", file)
			}
			if label == "" {
				if f := strings.Fields(pub); len(f) >= 3 {
					label = f[2]
				} else {
					label = strings.TrimSuffix(filepath.Base(file), ".pub")
				}
			}
			c, err := a.api()
			if err != nil {
				return err
			}
			r, err := c.AddSshKeyWithResponse(a.ctx(cmd), api.AddSshKeyRequest{Label: label, PublicKey: pub})
			if err != nil {
				return a.netErr(err)
			}
			if err := client.Check(r.HTTPResponse, r.Body); err != nil {
				return err
			}
			k := r.JSON201
			a.info("Saved SSH key %s (%s).", k.Label, k.Fingerprint)
			a.info("Next: legion rental create --ssh-key %s", k.Label)
			return a.emit(output.Result{Data: k, Names: []string{k.Label}, Table: &output.Table{Cols: []string{"LABEL", "TYPE", "FINGERPRINT"}, Rows: [][]string{{k.Label, string(k.KeyType), k.Fingerprint}}}})
		},
	}
	add.Flags().StringVar(&file, "file", "", "public key file (default ~/.ssh/id_ed25519.pub)")
	add.Flags().StringVar(&label, "label", "", "name for the key")
	remove := &cobra.Command{
		Use:     "remove <key>",
		Aliases: []string{"rm", "delete"},
		Short:   "Forget a saved SSH key",
		Long:    "Remove a saved key by label, fingerprint or id. Rentals already created with it keep working.",
		Example: "  legion ssh-key remove laptop\n  legion ssh-key remove SHA256:abc... --yes",
		Args:    args(1, 1),
		RunE: func(cmd *cobra.Command, as []string) error {
			c, err := a.api()
			if err != nil {
				return err
			}
			keys, err := a.sshKeys(a.ctx(cmd), c)
			if err != nil {
				return err
			}
			for _, k := range keys {
				if k.Label == as[0] || k.Fingerprint == as[0] || k.Id.String() == as[0] {
					r, err := c.DeleteSshKeyWithResponse(a.ctx(cmd), k.Id)
					if err != nil {
						return a.netErr(err)
					}
					if err := client.Check(r.HTTPResponse, r.Body); err != nil {
						return err
					}
					a.info("Removed SSH key %s.", k.Label)
					return nil
				}
			}
			return notFoundf("legion ssh-key list", "no saved SSH key %q", as[0])
		},
	}
	cmd.AddCommand(list, add, remove)
	return cmd
}

func (a *app) sshKeys(ctx context.Context, c *api.ClientWithResponses) ([]api.SshKey, error) {
	r, err := c.ListSshKeysWithResponse(ctx)
	if err != nil {
		return nil, a.netErr(err)
	}
	if err := client.Check(r.HTTPResponse, r.Body); err != nil {
		return nil, err
	}
	return r.JSON200.Items, nil
}

// ---- api-key ----

func (a *app) apiKeyCmd() *cobra.Command {
	cmd := group(&cobra.Command{
		Use:     "api-key",
		Aliases: []string{"api-keys", "apikey"},
		Short:   "Manage the API keys that sign in to Legion",
		Long:    "API keys (lgn_...) let scripts and legion itself call the API. A full-scope key can do everything its creator can; an agent-scope key is limited to the renter tools of the MCP server.",
		Example: `  legion api-key list
  legion api-key create ci --scope full --expires 720h`,
	})
	list := &cobra.Command{
		Use:     "list",
		Short:   "List API keys",
		Long:    "List the organization's API keys by prefix, scope and last use. Secrets are never shown after creation.",
		Example: "  legion api-key list\n  legion api-key list -o json",
		Args:    args(0, 0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := a.api()
			if err != nil {
				return err
			}
			keys, err := listAll(func(cur *string) ([]api.ApiKey, *string, error) {
				r, err := c.ListApiKeysWithResponse(a.ctx(cmd), &api.ListApiKeysParams{Cursor: cur, Limit: ptr(100)})
				if err != nil {
					return nil, nil, a.netErr(err)
				}
				if err := client.Check(r.HTTPResponse, r.Body); err != nil {
					return nil, nil, err
				}
				return r.JSON200.Items, r.JSON200.NextCursor, nil
			})
			if err != nil {
				return err
			}
			t := &output.Table{Cols: []string{"NAME", "PREFIX", "SCOPE", "CREATED", "LAST USED"}, Wide: []string{"EXPIRES", "ID"}}
			names := make([]string, len(keys))
			for i, k := range keys {
				names[i] = k.Name
				t.Rows = append(t.Rows, []string{k.Name, k.Prefix, string(k.Scope), ts(k.CreatedAt), tsp(k.LastUsedAt), tsp(k.ExpiresAt), k.Id.String()})
			}
			return a.emit(output.Result{Data: keys, Names: names, Table: t})
		},
	}
	var scope string
	var expires time.Duration
	create := &cobra.Command{
		Use:   "create <name>",
		Short: "Create an API key",
		Long: `Create a key. The secret is shown ONCE, here, and cannot be retrieved later: copy it now and
store it somewhere safe. Use scope full for the CLI and scripts.`,
		Example: "  legion api-key create ci --expires 720h\n  legion api-key create bot --scope agent",
		Args:    args(1, 1),
		RunE: func(cmd *cobra.Command, as []string) error {
			c, err := a.api()
			if err != nil {
				return err
			}
			req := api.CreateApiKeyRequest{Name: as[0]}
			if scope != "" {
				if scope != "full" && scope != "agent" {
					return usagef("legion api-key create --help", "--scope must be full or agent, got %q", scope)
				}
				req.Scope = ptr(api.ApiKeyScope(scope))
			}
			if expires > 0 {
				req.ExpiresAt = ptr(time.Now().Add(expires).UTC())
			}
			r, err := c.CreateApiKeyWithResponse(a.ctx(cmd), req)
			if err != nil {
				return a.netErr(err)
			}
			if err := client.Check(r.HTTPResponse, r.Body); err != nil {
				return err
			}
			k := r.JSON201
			a.info("Warning: this is the only time the secret is shown. Copy it now.")
			a.info("Use it: echo <key> | legion auth login --with-token   (or export LEGION_API_KEY)")
			return a.emit(output.Result{Data: k, Names: []string{k.Token}, Human: func(w io.Writer) {
				fmt.Fprintf(w, "Name:    %s\nScope:   %s\nPrefix:  %s\nSecret:  %s\n", k.ApiKey.Name, k.ApiKey.Scope, k.ApiKey.Prefix, k.Token)
			}})
		},
	}
	create.Flags().StringVar(&scope, "scope", "full", "full or agent")
	create.Flags().DurationVar(&expires, "expires", 0, "expire after this long, e.g. 720h (default: never)")
	revoke := &cobra.Command{
		Use:     "revoke <key>",
		Short:   "Revoke an API key",
		Long:    "Revoke a key by name, prefix or id. Anything using it stops working at once. Revoking the key legion is logged in with signs you out.",
		Example: "  legion api-key revoke ci\n  legion api-key revoke lgn_3Fq9 --yes",
		Args:    args(1, 1),
		RunE: func(cmd *cobra.Command, as []string) error {
			c, err := a.api()
			if err != nil {
				return err
			}
			rr, err := c.ListApiKeysWithResponse(a.ctx(cmd), &api.ListApiKeysParams{Limit: ptr(100)})
			if err != nil {
				return a.netErr(err)
			}
			if err := client.Check(rr.HTTPResponse, rr.Body); err != nil {
				return err
			}
			var hits []api.ApiKey
			for _, k := range rr.JSON200.Items {
				if k.Id.String() == as[0] || k.Name == as[0] || k.Prefix == as[0] {
					hits = append(hits, k)
				}
			}
			if len(hits) == 0 {
				return notFoundf("legion api-key list", "no API key matches %q", as[0])
			}
			if len(hits) > 1 {
				return usagef("Use the key id: legion api-key list -o wide", "%q matches %d keys", as[0], len(hits))
			}
			k := hits[0]
			if err := a.confirm(fmt.Sprintf("Revoke API key %s (%s)? Anything using it stops working.", k.Name, k.Prefix), "refusing to revoke an API key without confirmation"); err != nil {
				return err
			}
			r, err := c.RevokeApiKeyWithResponse(a.ctx(cmd), k.Id)
			if err != nil {
				return a.netErr(err)
			}
			if err := client.Check(r.HTTPResponse, r.Body); err != nil {
				return err
			}
			a.info("Revoked API key %s.", k.Name)
			return nil
		},
	}
	cmd.AddCommand(list, create, revoke)
	return cmd
}

// ---- billing ----

func (a *app) billingCmd() *cobra.Command {
	cmd := group(&cobra.Command{
		Use:   "billing",
		Short: "See your balance, charges and ledger",
		Long:  "Renting spends from a prepaid balance. Amounts are in US dollars.",
		Example: `  legion billing balance
  legion billing charges`,
	})
	balance := &cobra.Command{
		Use:     "balance",
		Short:   "Show your prepaid balance",
		Long:    "Show the organization's balance. A rental needs credit to be created or started.",
		Example: "  legion billing balance\n  legion billing balance -o jsonpath=$.balance.amount_minor",
		Args:    args(0, 0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := a.api()
			if err != nil {
				return err
			}
			b, err := a.balance(a.ctx(cmd), c)
			if err != nil {
				return err
			}
			if !b.HasCredit {
				a.info("No credit: rentals cannot be created or started. To add credit, open Billing in the web console (https://legion.ambient.xyz) or ask your Ambient contact.")
			}
			return a.emit(output.Result{Data: b, Human: func(w io.Writer) {
				fmt.Fprintf(w, "Balance:  %s\nCredit:   %s\n", signed(b.Balance), map[bool]string{true: "available", false: "none"}[b.HasCredit])
			}})
		},
	}
	var limit int
	charges := &cobra.Command{
		Use:     "charges",
		Short:   "Show what each rental has cost",
		Long:    "Show compute, storage and backup charges per rental, newest first.",
		Example: "  legion billing charges\n  legion billing charges --limit 10 -o json",
		Args:    args(0, 0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := a.api()
			if err != nil {
				return err
			}
			items, err := pages(limit, func(cur *string, n int) ([]api.RentalCharges, *string, error) {
				r, err := c.ListRentalChargesWithResponse(a.ctx(cmd), &api.ListRentalChargesParams{Cursor: cur, Limit: &n})
				if err != nil {
					return nil, nil, a.netErr(err)
				}
				if err := client.Check(r.HTTPResponse, r.Body); err != nil {
					return nil, nil, err
				}
				return r.JSON200.Items, r.JSON200.NextCursor, nil
			})
			if err != nil {
				return err
			}
			names := map[string]string{}
			if rs, err := a.listRentals(a.ctx(cmd), c); err == nil {
				for _, r := range rs {
					names[r.Id.String()] = r.Name
				}
			}
			t := &output.Table{Cols: []string{"RENTAL", "BILLED", "COMPUTE", "STORAGE", "BACKUPS"}}
			for _, ch := range items {
				n := names[ch.RentalId.String()]
				if n == "" {
					n = ch.RentalId.String()
				}
				bk := ""
				if ch.BackupCharged != nil {
					bk = signed(*ch.BackupCharged)
				}
				t.Rows = append(t.Rows, []string{n, output.Duration(ch.Seconds), signed(ch.Charged), signed(ch.StorageCharged), bk})
			}
			return a.emit(output.Result{Data: items, Table: t})
		},
	}
	charges.Flags().IntVar(&limit, "limit", 50, "most rows")
	var account, rental string
	activity := &cobra.Command{
		Use:     "activity",
		Short:   "Show billing activity: usage sessions and balance changes",
		Long:    "Show rental usage sessions and ledger entries together, newest first. --rental narrows to one rental.",
		Example: "  legion billing activity\n  legion billing activity --rental quiet-river-7k2f --limit 20",
		Args:    args(0, 0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := a.api()
			if err != nil {
				return err
			}
			p := api.ListBillingActivityParams{Account: api.LedgerAccount(account)}
			if rental != "" {
				r, err := a.pickRental(a.ctx(cmd), c, []string{rental})
				if err != nil {
					return err
				}
				p.RentalId = &r.Id
			}
			items, err := pages(limit, func(cur *string, n int) ([]api.BillingActivityItem, *string, error) {
				p.Cursor, p.Limit = cur, &n
				r, err := c.ListBillingActivityWithResponse(a.ctx(cmd), &p)
				if err != nil {
					return nil, nil, a.netErr(err)
				}
				if err := client.Check(r.HTTPResponse, r.Body); err != nil {
					return nil, nil, err
				}
				return r.JSON200.Items, r.JSON200.NextCursor, nil
			})
			if err != nil {
				return err
			}
			t := &output.Table{Cols: []string{"TIME", "WHAT", "DETAIL", "AMOUNT"}}
			for _, it := range items {
				what, detail := string(it.Kind), ""
				switch {
				case it.Session != nil:
					s := it.Session
					what = "usage"
					detail = fmt.Sprintf("%s: %dx %s for %s", ptrOr(s.RentalName, s.RentalId.String()), s.GpuCount, s.GpuModel, output.Duration(s.Seconds))
					if s.Running {
						detail += " (running)"
					}
				case it.Entry != nil:
					what, detail = string(it.Entry.Cause), ptrOr(it.Entry.Reason, "")
				}
				t.Rows = append(t.Rows, []string{ts(it.At), what, detail, signed(it.Amount)})
			}
			return a.emit(output.Result{Data: items, Table: t})
		},
	}
	activity.Flags().StringVar(&account, "account", "balance", "balance or earnings")
	activity.Flags().StringVar(&rental, "rental", "", "only this rental")
	activity.Flags().IntVar(&limit, "limit", 50, "most rows")
	ledger := &cobra.Command{
		Use:     "ledger",
		Short:   "Show the ledger of balance changes",
		Long:    "Show every change to the balance: usage, storage, backups, top-ups, refunds and adjustments, newest first.",
		Example: "  legion billing ledger\n  legion billing ledger --limit 100 -o json",
		Args:    args(0, 0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := a.api()
			if err != nil {
				return err
			}
			p := api.ListLedgerEntriesParams{Account: ptr(api.LedgerAccount(account))}
			items, err := pages(limit, func(cur *string, n int) ([]api.LedgerEntry, *string, error) {
				p.Cursor, p.Limit = cur, &n
				r, err := c.ListLedgerEntriesWithResponse(a.ctx(cmd), &p)
				if err != nil {
					return nil, nil, a.netErr(err)
				}
				if err := client.Check(r.HTTPResponse, r.Body); err != nil {
					return nil, nil, err
				}
				return r.JSON200.Items, r.JSON200.NextCursor, nil
			})
			if err != nil {
				return err
			}
			t := &output.Table{Cols: []string{"TIME", "CAUSE", "AMOUNT", "REASON"}, Wide: []string{"RENTAL ID", "ID"}}
			for _, e := range items {
				rid := ""
				if e.RentalId != nil {
					rid = e.RentalId.String()
				}
				t.Rows = append(t.Rows, []string{ts(e.OccurredAt), string(e.Cause), signed(e.Amount), ptrOr(e.Reason, ""), rid, e.Id})
			}
			return a.emit(output.Result{Data: items, Table: t})
		},
	}
	ledger.Flags().StringVar(&account, "account", "balance", "balance or earnings")
	ledger.Flags().IntVar(&limit, "limit", 50, "most rows")
	cmd.AddCommand(balance, charges, activity, ledger)
	return cmd
}

// pages collects up to max items from a cursor-paged list.
func pages[T any](max int, fetch func(cur *string, n int) ([]T, *string, error)) ([]T, error) {
	if max <= 0 {
		max = 50
	}
	all := []T{}
	var cur *string
	for len(all) < max {
		items, next, err := fetch(cur, min(max-len(all), 100))
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
