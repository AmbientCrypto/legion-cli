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

// ---- ingress ----

func (a *app) ingressCmd() *cobra.Command {
	cmd := group(&cobra.Command{
		Use:   "ingress",
		Short: "Expose a port of a rental on the internet",
		Long: `An ingress rule publishes an HTTP service in the rental at https://<rental>.<domain>, optionally
limited to some source networks. SSH needs no rule.`,
		Example: `  legion rental ingress add quiet-river-7k2f --port 8000
  legion rental ingress list quiet-river-7k2f`,
	})
	list := &cobra.Command{
		Use:     "list <rental>",
		Short:   "List a rental's exposed ports",
		Long:    "List the ingress rules of a rental with their public URL and status.",
		Example: "  legion rental ingress list quiet-river-7k2f\n  legion rental ingress list quiet -o json",
		Args:    args(0, 1),
		RunE: func(cmd *cobra.Command, as []string) error {
			return a.withRental(cmd, as, func(ctx context.Context, c *api.ClientWithResponses, r *api.Rental) error {
				resp, err := c.GetRentalNetworkWithResponse(ctx, r.Id)
				if err := a.ok(resp, err); err != nil {
					return err
				}
				n, err := need(resp.JSON200)
				if err != nil {
					return err
				}
				if !n.Ingress.Available {
					a.info("Exposed ports are not available for this rental.")
				}
				return a.emit(listResult(n.Ingress.Rules, []string{"PORT", "URL", "STATUS", "SOURCES"}, []string{"ID", "DESCRIPTION"},
					func(ru api.IngressRule) string { return ru.Id.String() },
					func(ru api.IngressRule) []string {
						src := "anyone"
						if len(ru.Sources) > 0 {
							src = strings.Join(ru.Sources, ",")
						}
						return []string{fmt.Sprint(ru.Port), ru.Url, string(ru.Status), src, ru.Id.String(), ptrOr(ru.Description, "")}
					}))
			})
		},
	}
	var port int
	var sources []string
	var desc string
	add := &cobra.Command{
		Use:   "add <rental>",
		Short: "Expose a port",
		Long: `Publish the HTTP service listening on --port inside the rental. Without --source anyone on the
internet can reach it; add --source <cidr> (repeatable, up to 16) to limit it. You are asked to confirm
(or pass --yes).`,
		Example: "  legion rental ingress add quiet-river-7k2f --port 8000\n  legion rental ingress add quiet-river-7k2f --port 8000 --source 203.0.113.0/24 --description api",
		Args:    args(0, 1),
		RunE: func(cmd *cobra.Command, as []string) error {
			if port == 0 {
				return usagef("legion rental ingress add --help", "missing required flag --port")
			}
			return a.withRental(cmd, as, func(ctx context.Context, c *api.ClientWithResponses, r *api.Rental) error {
				who := "anyone on the internet"
				if len(sources) > 0 {
					who = "only " + strings.Join(sources, ", ")
				}
				msg := fmt.Sprintf("port %d of %s will be reachable on the internet by %s", port, r.Name, who)
				if err := a.confirm("Expose it? "+msg+".", "refusing to expose a port without confirmation: "+msg); err != nil {
					return err
				}
				req := api.CreateIngressRuleRequest{Protocol: api.IngressProtocolHTTP, Port: port, Sources: append([]string{}, sources...)}
				if desc != "" {
					req.Description = &desc
				}
				resp, err := c.CreateIngressRuleWithResponse(ctx, r.Id, &api.CreateIngressRuleParams{IdempotencyKey: uuid.NewString()}, req)
				if err := a.ok(resp, err); err != nil {
					return err
				}
				ru, err := need(resp.JSON201)
				if err != nil {
					return err
				}
				a.info("Exposed port %d at %s (status: %s; it goes live once the service answers).", ru.Port, san(ru.Url), ru.Status)
				a.info("Remove: legion rental ingress remove %s %s", r.Name, ru.Id)
				res := listResult([]api.IngressRule{*ru}, []string{"PORT", "URL", "STATUS"}, nil, func(ru api.IngressRule) string { return ru.Url },
					func(ru api.IngressRule) []string { return []string{fmt.Sprint(ru.Port), ru.Url, string(ru.Status)} })
				res.Data = ru
				return a.emit(res)
			})
		},
	}
	add.Flags().IntVar(&port, "port", 0, "port the service listens on in the rental (not 22 or 22022)")
	add.Flags().StringArrayVar(&sources, "source", nil, "allowed source CIDR, e.g. 203.0.113.0/24 (repeatable; default anyone)")
	add.Flags().StringVar(&desc, "description", "", "a note shown with the rule")
	remove := &cobra.Command{
		Use:     "remove <rental> <rule-id>",
		Aliases: []string{"rm"},
		Short:   "Stop exposing a port",
		Long:    "Delete an ingress rule. The public URL stops answering at once. Find ids with: legion rental ingress list -o wide.",
		Example: "  legion rental ingress remove quiet-river-7k2f 6f1c... --yes\n  legion rental ingress remove quiet 6f1c...",
		Args:    args(2, 2),
		RunE: func(cmd *cobra.Command, as []string) error {
			id, err := uuid.Parse(as[1])
			if err != nil {
				return usagef("legion rental ingress list "+as[0]+" -o wide", "%q is not a rule id", as[1])
			}
			return a.withRental(cmd, as[:1], func(ctx context.Context, c *api.ClientWithResponses, r *api.Rental) error {
				if err := a.confirm(fmt.Sprintf("Remove ingress rule %s from %s? Its public URL stops working.", id, r.Name), "refusing to remove an ingress rule without confirmation"); err != nil {
					return err
				}
				resp, err := c.DeleteIngressRuleWithResponse(ctx, r.Id, id)
				if err := a.ok(resp, err); err != nil {
					return err
				}
				a.info("Removed ingress rule %s.", id)
				return nil
			})
		},
	}
	cmd.AddCommand(list, add, remove)
	return cmd
}

// ---- backup ----

func (a *app) backupCmd() *cobra.Command {
	cmd := group(&cobra.Command{
		Use:     "backup",
		Aliases: []string{"backups"},
		Short:   "Back up a rental's workspace and download it",
		Long: `A backup copies the workspace off the host and keeps it for 7 or 30 days. It is charged by size, so
estimate first. Download links are charged too.`,
		Example: `  legion rental backup estimate quiet-river-7k2f
  legion rental backup create quiet-river-7k2f --retention-days 30`,
	})
	var days int
	retention := func() *api.BackupRetentionDays {
		if days == 0 {
			return nil
		}
		d := api.BackupRetentionDays(days)
		return &d
	}
	estimate := &cobra.Command{
		Use:     "estimate <rental>",
		Short:   "Estimate a backup's size and price",
		Long:    "Show the estimated size and price of backing up a rental's workspace, without making one.",
		Example: "  legion rental backup estimate quiet-river-7k2f\n  legion rental backup estimate quiet-river-7k2f --retention-days 30",
		Args:    args(0, 1),
		RunE: func(cmd *cobra.Command, as []string) error {
			return a.withRental(cmd, as, func(ctx context.Context, c *api.ClientWithResponses, r *api.Rental) error {
				e, err := a.estimate(ctx, c, r.Id, retention())
				if err != nil {
					return err
				}
				return a.emit(output.Result{Data: e, Human: func(w io.Writer) { writeEstimate(w, e) }})
			})
		},
	}
	var deleteAfter bool
	create := &cobra.Command{
		Use:   "create <rental>",
		Short: "Back up a rental's workspace",
		Long: `Copy the rental's workspace into a backup kept --retention-days (7 or 30). You are charged the
estimated price shown, so you are asked to confirm (or pass --yes). --delete-rental-after deletes the
rental once the backup is ready.`,
		Example: "  legion rental backup create quiet-river-7k2f\n  legion rental backup create quiet-river-7k2f --retention-days 30 --yes",
		Args:    args(0, 1),
		RunE: func(cmd *cobra.Command, as []string) error {
			return a.withRental(cmd, as, func(ctx context.Context, c *api.ClientWithResponses, r *api.Rental) error {
				e, err := a.estimate(ctx, c, r.Id, retention())
				if err != nil {
					return err
				}
				if !a.quiet {
					writeEstimate(a.env.Stderr, e)
				}
				q := fmt.Sprintf("Back up %s for %s?", r.Name, output.Money(e.Price.AmountMinor, string(e.Price.Currency)))
				refuse := "refusing to start a charged backup without confirmation"
				if deleteAfter {
					note := fmt.Sprintf(" The rental %s will be deleted after the backup is ready.", r.Name)
					q, refuse = q+note, refuse+":"+note
				}
				if err := a.confirm(q, refuse); err != nil {
					return err
				}
				body := api.CreateBackupRequest{RetentionDays: retention()}
				if deleteAfter {
					body.DeleteRentalAfter = &deleteAfter
				}
				key := uuid.NewString()
				resp, err := c.CreateRentalBackupWithResponse(ctx, r.Id, &api.CreateRentalBackupParams{IdempotencyKey: &key}, body)
				if err := a.ok(resp, err); err != nil {
					return err
				}
				b, err := need(resp.JSON202)
				if err != nil {
					return err
				}
				a.info("Backup %s started (state: %s).", b.Id, b.State)
				a.info("Follow it: legion rental backup get %s", b.Id)
				return a.emit(backupResult([]api.Backup{*b}, true))
			})
		},
	}
	for _, c := range []*cobra.Command{estimate, create} {
		c.Flags().IntVar(&days, "retention-days", 0, "keep the backup 7 or 30 days (default 7)")
	}
	create.Flags().BoolVar(&deleteAfter, "delete-rental-after", false, "delete the rental when the backup is ready")

	var state string
	list := &cobra.Command{
		Use:     "list [rental]",
		Short:   "List backups",
		Long:    "List backups of one rental, or of every rental when none is given. --state keeps only backups in that state.",
		Example: "  legion rental backup list\n  legion rental backup list quiet-river-7k2f --state ready",
		Args:    args(0, 1),
		RunE: func(cmd *cobra.Command, as []string) error {
			c, err := a.api()
			if err != nil {
				return err
			}
			ctx := a.ctx(cmd)
			var bs []api.Backup
			if len(as) == 1 {
				r, err := a.pickRental(ctx, c, as)
				if err != nil {
					return err
				}
				bs, err = collect(0, func(cur *string, n int) ([]api.Backup, *string, error) {
					resp, err := c.ListRentalBackupsWithResponse(ctx, r.Id, &api.ListRentalBackupsParams{Cursor: cur, Limit: &n})
					if err := a.ok(resp, err); err != nil {
						return nil, nil, err
					}
					p, err := need(resp.JSON200)
					if err != nil {
						return nil, nil, err
					}
					return p.Items, p.NextCursor, nil
				})
				if err != nil {
					return err
				}
				if state != "" { // this endpoint has no state filter
					var keep []api.Backup
					for _, b := range bs {
						if string(b.State) == state {
							keep = append(keep, b)
						}
					}
					bs = keep
				}
			} else {
				p := api.ListBackupsParams{}
				if state != "" {
					p.State = ptr(api.BackupState(state))
				}
				bs, err = collect(0, func(cur *string, n int) ([]api.Backup, *string, error) {
					p.Cursor, p.Limit = cur, &n
					resp, err := c.ListBackupsWithResponse(ctx, &p)
					if err := a.ok(resp, err); err != nil {
						return nil, nil, err
					}
					pg, err := need(resp.JSON200)
					if err != nil {
						return nil, nil, err
					}
					return pg.Items, pg.NextCursor, nil
				})
				if err != nil {
					return err
				}
			}
			return a.emit(backupResult(bs, false))
		},
	}
	list.Flags().StringVar(&state, "state", "", "only backups in this state: queued, uploading, ready, failed, cancelled, expired, deleted")

	byID := func(use, short, long, ex string, run func(ctx context.Context, c *api.ClientWithResponses, id uuid.UUID) error) *cobra.Command {
		return &cobra.Command{
			Use: use, Short: short, Long: long, Example: ex, Args: args(1, 1),
			RunE: func(cmd *cobra.Command, as []string) error {
				id, err := uuid.Parse(as[0])
				if err != nil {
					return usagef("legion rental backup list", "%q is not a backup id", as[0])
				}
				c, err := a.api()
				if err != nil {
					return err
				}
				return run(a.ctx(cmd), c, id)
			},
		}
	}
	get := byID("get <backup-id>", "Show one backup", "Show a backup's state, size, progress, expiry and price.",
		"  legion rental backup get 6f1c2e3a-...\n  legion rental backup get 6f1c2e3a-... -o json",
		func(ctx context.Context, c *api.ClientWithResponses, id uuid.UUID) error {
			r, err := c.GetBackupWithResponse(ctx, id)
			if err := a.ok(r, err); err != nil {
				return err
			}
			b, err := need(r.JSON200)
			if err != nil {
				return err
			}
			if b.State == api.BackupStateReady {
				a.info("Download: legion rental backup download-link %s", b.Id)
			}
			return a.emit(backupResult([]api.Backup{*b}, true))
		})
	del := byID("delete <backup-id>", "Delete a backup", "Delete a backup and its data for good.",
		"  legion rental backup delete 6f1c2e3a-... --yes\n  legion rental backup delete 6f1c2e3a-...",
		func(ctx context.Context, c *api.ClientWithResponses, id uuid.UUID) error {
			if err := a.confirm(fmt.Sprintf("Delete backup %s for good?", id), "refusing to delete a backup without confirmation"); err != nil {
				return err
			}
			r, err := c.DeleteBackupWithResponse(ctx, id)
			if err := a.ok(r, err); err != nil {
				return err
			}
			a.info("Deleted backup %s.", id)
			return nil
		})
	cancel := byID("cancel <backup-id>", "Cancel a backup in progress", "Cancel a queued or uploading backup. The charge is refunded.",
		"  legion rental backup cancel 6f1c2e3a-...\n  legion rental backup cancel 6f1c2e3a-... -q",
		func(ctx context.Context, c *api.ClientWithResponses, id uuid.UUID) error {
			r, err := c.CancelBackupWithResponse(ctx, id)
			if err := a.ok(r, err); err != nil {
				return err
			}
			a.info("Cancelled backup %s.", id)
			if r.JSON200 == nil {
				return nil
			}
			return a.emit(backupResult([]api.Backup{*r.JSON200}, true))
		})
	link := byID("download-link <backup-id>", "Make a single-use download link for a ready backup",
		"Make a link to download a ready backup. The link works once and expires after about an hour. The first link is included in the backup's price; every further link is charged by the GiB and not refunded, and the command says which this one is. If a link was already used or has expired, make a new one.",
		"  legion rental backup download-link 6f1c2e3a-...\n  legion rental backup download-link 6f1c2e3a-... -o jsonpath=$.url",
		func(ctx context.Context, c *api.ClientWithResponses, id uuid.UUID) error {
			key := uuid.NewString()
			r, err := c.CreateBackupDownloadLinkWithResponse(ctx, id, &api.CreateBackupDownloadLinkParams{IdempotencyKey: &key})
			if err := a.ok(r, err); err != nil {
				return err
			}
			l, err := need(r.JSON201)
			if err != nil {
				return err
			}
			cost := "included in the backup's price"
			if l.Charge.AmountMinor > 0 {
				cost = "charged " + output.Money(l.Charge.AmountMinor, string(l.Charge.Currency)) + ", not refunded"
			}
			a.info("Link %d: %s. It works once and expires %s.", l.LinkNumber, cost, ts(l.ExpiresAt))
			return a.emit(output.Result{Data: l, Names: []string{l.Url}, Human: func(w io.Writer) { fmt.Fprintln(w, san(l.Url)) }})
		})
	cmd.AddCommand(estimate, create, list, get, del, cancel, link)
	return cmd
}

func (a *app) estimate(ctx context.Context, c *api.ClientWithResponses, id api.RentalID, days *api.BackupRetentionDays) (*api.BackupEstimate, error) {
	r, err := c.EstimateRentalBackupWithResponse(ctx, id, &api.EstimateRentalBackupParams{RetentionDays: days})
	if err := a.ok(r, err); err != nil {
		return nil, err
	}
	return need(r.JSON200)
}

func writeEstimate(w io.Writer, e *api.BackupEstimate) {
	cur := string(e.Price.Currency)
	fmt.Fprintf(w, "Estimated size:  %s (%s)\nPrice:           %s (storage %s + download %s), kept %d days\n",
		output.Bytes(e.EstimatedSizeBytes), san(string(e.EstimateBasis)), output.Money(e.Price.AmountMinor, cur), output.Money(e.Storage.AmountMinor, cur), output.Money(e.Download.AmountMinor, cur), e.RetentionDays)
	if !e.FitsUploadWindow {
		fmt.Fprintln(w, "Warning:         this may not finish uploading in time on the host's connection.")
	}
}

// bytesOf formats an optional size.
func bytesOf(n *int64) string {
	if n == nil {
		return ""
	}
	return output.Bytes(*n)
}

// detail is the data of a one-item result: the item itself, not a list.
func detail(res output.Result, one any) output.Result {
	res.Data = one
	return res
}

func backupResult(bs []api.Backup, single bool) output.Result {
	res := listResult(bs, []string{"ID", "RENTAL", "STATE", "SIZE", "PRICE", "EXPIRES"}, nil,
		func(b api.Backup) string { return b.Id.String() },
		func(b api.Backup) []string {
			size := "~" + output.Bytes(b.EstimatedSizeBytes)
			if b.SizeBytes != nil {
				size = bytesOf(b.SizeBytes)
			}
			return []string{b.Id.String(), ptrOr(b.RentalName, b.RentalId.String()), string(b.State), size, signed(api.SignedMoney(b.Price.Quoted)), tsp(b.ExpiresAt)}
		})
	if single && len(bs) == 1 {
		return detail(res, bs[0])
	}
	return res
}

// ---- workspace export ----

// cents is a whole-cent amount in US dollars.
func cents(n int64) string { return output.Money(n, "USD") }

// linkCents is what one download of a size costs: whole cents, rounded up, at
// least one cent unless the price is zero (the controller's rule, shown here
// before the person confirms; the controller charges the real figure).
func linkCents(sizeBytes, centsPerGiB int64) int64 {
	if centsPerGiB <= 0 {
		return 0
	}
	const gib = int64(1) << 30
	whole, rest := sizeBytes/gib, sizeBytes%gib
	c := whole * centsPerGiB
	if rest > 0 {
		c += (rest*centsPerGiB + gib - 1) / gib
	}
	if c < 1 {
		c = 1
	}
	return c
}

func minutes(sec int64) string { return fmt.Sprintf("%d minutes", (sec+59)/60) }

func writeExportEstimate(w io.Writer, e *api.WorkspaceExportEstimate) {
	fmt.Fprintf(w, "Workspace size:  %s (%s)\nPrice:           %s per GiB for each download link\nOne download:    about %s\nLinks:           work once, for %s after you make them\nMaking the export itself is free.\n",
		output.Bytes(e.EstimatedBytes), san(string(e.Basis)), cents(e.PriceCentsPerGib), cents(e.EstimatedDownloadCents), minutes(e.LinkTtlSeconds))
}

func (a *app) exportCmd() *cobra.Command {
	cmd := group(&cobra.Command{
		Use:     "export",
		Aliases: []string{"exports"},
		Short:   "Export a stopped rental's workspace and download it",
		Long: `A workspace export copies the files of a stopped rental into a file you can download. Making the export is free.
Downloading it is charged by the GiB, every time you make a download link, and each link works once.
See the price first with 'rental export estimate'; make a link with 'rental export link'.`,
		Example: `  legion rental export estimate quiet-river-7k2f
  legion rental export create quiet-river-7k2f
  legion rental export get quiet-river-7k2f <export-id>
  legion rental export link quiet-river-7k2f <export-id>`,
	})
	res := func(e api.WorkspaceExport) output.Result {
		return detail(listResult([]api.WorkspaceExport{e}, []string{"ID", "STATE", "SIZE"}, nil,
			func(e api.WorkspaceExport) string { return e.Id.String() },
			func(e api.WorkspaceExport) []string {
				return []string{e.Id.String(), string(e.State), bytesOf(e.SizeBytes)}
			}), e)
	}
	estimate := &cobra.Command{
		Use:     "estimate <rental>",
		Short:   "Show what downloading a workspace export will cost",
		Long:    "Show the estimated size of the rental's workspace and the price of one download link, without making an export. A link is charged by the GiB every time you make one and works once.",
		Example: "  legion rental export estimate quiet-river-7k2f\n  legion rental export estimate quiet -o json",
		Args:    args(0, 1),
		RunE: func(cmd *cobra.Command, as []string) error {
			return a.withRental(cmd, as, func(ctx context.Context, c *api.ClientWithResponses, r *api.Rental) error {
				resp, err := c.EstimateWorkspaceExportWithResponse(ctx, r.Id)
				if err := a.ok(resp, err); err != nil {
					return err
				}
				e, err := need(resp.JSON200)
				if err != nil {
					return err
				}
				return a.emit(output.Result{Data: e, Human: func(w io.Writer) { writeExportEstimate(w, e) }})
			})
		},
	}
	create := &cobra.Command{
		Use:     "create <rental>",
		Short:   "Start exporting a workspace",
		Long:    "Start an export of the stopped rental's workspace. Making it is free. Follow it with 'rental export get'; when it is ready, 'rental export link' makes a single-use download link, which is charged by the GiB.",
		Example: "  legion rental export create quiet-river-7k2f\n  legion rental export create quiet -o name",
		Args:    args(0, 1),
		RunE: func(cmd *cobra.Command, as []string) error {
			return a.withRental(cmd, as, func(ctx context.Context, c *api.ClientWithResponses, r *api.Rental) error {
				resp, err := c.CreateWorkspaceExportWithResponse(ctx, r.Id)
				if err := a.ok(resp, err); err != nil {
					return err
				}
				e, err := need(resp.JSON202)
				if err != nil {
					return err
				}
				a.info("Export %s started. Follow it: legion rental export get %s %s", e.Id, r.Name, e.Id)
				a.info("Downloading it is charged by the GiB: legion rental export estimate %s", r.Name)
				return a.emit(res(*e))
			})
		},
	}
	list := &cobra.Command{
		Use:     "list <rental>",
		Short:   "List a rental's exports",
		Long:    "List the workspace exports of a rental.",
		Example: "  legion rental export list quiet-river-7k2f\n  legion rental export list quiet -o json",
		Args:    args(0, 1),
		RunE: func(cmd *cobra.Command, as []string) error {
			return a.withRental(cmd, as, func(ctx context.Context, c *api.ClientWithResponses, r *api.Rental) error {
				resp, err := c.ListWorkspaceExportsWithResponse(ctx, r.Id)
				if err := a.ok(resp, err); err != nil {
					return err
				}
				l, err := need(resp.JSON200)
				if err != nil {
					return err
				}
				return a.emit(listResult(l.Items, []string{"ID", "STATE", "SIZE", "REQUESTED"}, nil,
					func(e api.WorkspaceExportSummary) string { return e.Id.String() },
					func(e api.WorkspaceExportSummary) []string {
						return []string{e.Id.String(), string(e.State), bytesOf(e.SizeBytes), ts(e.RequestedAt)}
					}))
			})
		},
	}
	exportByID := func(use, short, long, ex string, run func(ctx context.Context, c *api.ClientWithResponses, r *api.Rental, id uuid.UUID) error) *cobra.Command {
		return &cobra.Command{
			Use: use, Short: short, Long: long, Example: ex, Args: args(2, 2),
			RunE: func(cmd *cobra.Command, as []string) error {
				id, err := uuid.Parse(as[1])
				if err != nil {
					return usagef("legion rental export list "+as[0], "%q is not an export id", as[1])
				}
				return a.withRental(cmd, as[:1], func(ctx context.Context, c *api.ClientWithResponses, r *api.Rental) error { return run(ctx, c, r, id) })
			},
		}
	}
	get := exportByID("get <rental> <export-id>", "Show one export", "Show an export's state and size. When it is ready, make a download link with 'rental export link' (charged by the GiB, works once).",
		"  legion rental export get quiet-river-7k2f 6f1c...\n  legion rental export get quiet 6f1c... -o jsonpath=$.state",
		func(ctx context.Context, c *api.ClientWithResponses, r *api.Rental, id uuid.UUID) error {
			resp, err := c.GetWorkspaceExportWithResponse(ctx, r.Id, id)
			if err := a.ok(resp, err); err != nil {
				return err
			}
			e, err := need(resp.JSON200)
			if err != nil {
				return err
			}
			if e.State == api.WorkspaceExportReady {
				a.info("Ready. Download it: legion rental export link %s %s (charged by the GiB, the link works once)", r.Name, e.Id)
			}
			return a.emit(res(*e))
		})
	link := exportByID("link <rental> <export-id>", "Make a single-use download link for a ready export",
		"Make a link to download a ready export. Each link is charged by the GiB (the charge is shown first and you confirm, or pass --yes), is not refunded, works once and expires after about an hour. Make a new link if one is used or has expired.",
		"  legion rental export link quiet-river-7k2f 6f1c...\n  legion rental export link quiet 6f1c... --yes -o jsonpath=$.url",
		func(ctx context.Context, c *api.ClientWithResponses, r *api.Rental, id uuid.UUID) error {
			gr, err := c.GetWorkspaceExportWithResponse(ctx, r.Id, id)
			if err := a.ok(gr, err); err != nil {
				return err
			}
			e, err := need(gr.JSON200)
			if err != nil {
				return err
			}
			if e.State != api.WorkspaceExportReady {
				return errorf(fmt.Sprintf("legion rental export get %s %s", r.Name, e.Id), "export %s is not ready (state: %s)", e.Id, e.State)
			}
			er, err := c.EstimateWorkspaceExportWithResponse(ctx, r.Id)
			if err := a.ok(er, err); err != nil {
				return err
			}
			est, err := need(er.JSON200)
			if err != nil {
				return err
			}
			var size int64
			if e.SizeBytes != nil {
				size = *e.SizeBytes
			}
			charge := linkCents(size, est.PriceCentsPerGib)
			if !a.quiet {
				fmt.Fprintf(a.env.Stderr, "Export size:     %s\nThis link:       %s (%s per GiB), not refunded\nThe link works once and expires %s after you make it.\n",
					output.Bytes(size), cents(charge), cents(est.PriceCentsPerGib), minutes(est.LinkTtlSeconds))
			}
			if err := a.confirm(fmt.Sprintf("Make a single-use download link for %s?", cents(charge)), "refusing to make a charged download link without confirmation"); err != nil {
				return err
			}
			key := uuid.NewString()
			lr, err := c.CreateWorkspaceExportDownloadLinkWithResponse(ctx, r.Id, id, &api.CreateWorkspaceExportDownloadLinkParams{IdempotencyKey: key})
			if err := a.ok(lr, err); err != nil {
				return err
			}
			l, err := need(lr.JSON201)
			if err != nil {
				return err
			}
			a.info("Link %d, charged %s. It works once and expires %s.", l.Number, cents(l.ChargeCents), ts(l.ExpiresAt))
			return a.emit(output.Result{Data: l, Names: []string{l.Url}, Human: func(w io.Writer) { fmt.Fprintln(w, san(l.Url)) }})
		})
	cancel := exportByID("cancel <rental> <export-id>", "Cancel an export", "Cancel an export that is still running.",
		"  legion rental export cancel quiet-river-7k2f 6f1c...\n  legion rental export cancel quiet 6f1c... -q",
		func(ctx context.Context, c *api.ClientWithResponses, r *api.Rental, id uuid.UUID) error {
			resp, err := c.CancelWorkspaceExportWithResponse(ctx, r.Id, id)
			if err := a.ok(resp, err); err != nil {
				return err
			}
			a.info("Cancelled export %s.", id)
			return nil
		})
	cmd.AddCommand(estimate, create, list, get, link, cancel)
	return cmd
}
