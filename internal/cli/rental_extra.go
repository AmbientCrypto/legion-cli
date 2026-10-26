package cli

import (
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
			c, err := a.api()
			if err != nil {
				return err
			}
			r, err := a.pickRental(a.ctx(cmd), c, as)
			if err != nil {
				return err
			}
			resp, err := c.GetRentalNetworkWithResponse(a.ctx(cmd), r.Id)
			if err := a.ok(resp, err); err != nil {
				return err
			}
			rules := resp.JSON200.Ingress.Rules
			if rules == nil {
				rules = []api.IngressRule{}
			}
			if !resp.JSON200.Ingress.Available {
				a.info("Exposed ports are not available for this rental.")
			}
			t := &output.Table{Cols: []string{"PORT", "URL", "STATUS", "SOURCES"}, Wide: []string{"ID", "DESCRIPTION"}}
			names := make([]string, len(rules))
			for i, ru := range rules {
				names[i] = ru.Id.String()
				src := "anyone"
				if len(ru.Sources) > 0 {
					src = strings.Join(ru.Sources, ",")
				}
				t.Rows = append(t.Rows, []string{fmt.Sprint(ru.Port), ru.Url, string(ru.Status), src, ru.Id.String(), ptrOr(ru.Description, "")})
			}
			return a.emit(output.Result{Data: rules, Names: names, Table: t})
		},
	}
	var port int
	var sources []string
	var desc string
	add := &cobra.Command{
		Use:   "add <rental>",
		Short: "Expose a port",
		Long: `Publish the HTTP service listening on --port inside the rental. Without --source anyone on the
internet can reach it; add --source <cidr> (repeatable, up to 16) to limit it.`,
		Example: "  legion rental ingress add quiet-river-7k2f --port 8000\n  legion rental ingress add quiet-river-7k2f --port 8000 --source 203.0.113.0/24 --description api",
		Args:    args(0, 1),
		RunE: func(cmd *cobra.Command, as []string) error {
			if port == 0 {
				return usagef("legion rental ingress add --help", "missing required flag --port")
			}
			c, err := a.api()
			if err != nil {
				return err
			}
			r, err := a.pickRental(a.ctx(cmd), c, as)
			if err != nil {
				return err
			}
			req := api.CreateIngressRuleRequest{Protocol: api.IngressProtocolHTTP, Port: port, Sources: append([]string{}, sources...)}
			if desc != "" {
				req.Description = &desc
			}
			resp, err := c.CreateIngressRuleWithResponse(a.ctx(cmd), r.Id, &api.CreateIngressRuleParams{IdempotencyKey: uuid.NewString()}, req)
			if err := a.ok(resp, err); err != nil {
				return err
			}
			ru := resp.JSON201
			a.info("Exposed port %d at %s (status: %s; it goes live once the service answers).", ru.Port, ru.Url, ru.Status)
			a.info("Remove: legion rental ingress remove %s %s", r.Name, ru.Id)
			return a.emit(output.Result{Data: ru, Names: []string{ru.Url}, Table: &output.Table{Cols: []string{"PORT", "URL", "STATUS"}, Rows: [][]string{{fmt.Sprint(ru.Port), ru.Url, string(ru.Status)}}}})
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
			c, err := a.api()
			if err != nil {
				return err
			}
			r, err := a.pickRental(a.ctx(cmd), c, as[:1])
			if err != nil {
				return err
			}
			if err := a.confirm(fmt.Sprintf("Remove ingress rule %s from %s? Its public URL stops working.", id, r.Name), "refusing to remove an ingress rule without confirmation"); err != nil {
				return err
			}
			resp, err := c.DeleteIngressRuleWithResponse(a.ctx(cmd), r.Id, id)
			if err := a.ok(resp, err); err != nil {
				return err
			}
			a.info("Removed ingress rule %s.", id)
			return nil
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
			c, err := a.api()
			if err != nil {
				return err
			}
			r, err := a.pickRental(a.ctx(cmd), c, as)
			if err != nil {
				return err
			}
			e, err := a.estimate(cmd, c, r.Id, retention())
			if err != nil {
				return err
			}
			return a.emit(output.Result{Data: e, Human: func(w io.Writer) { writeEstimate(w, e) }})
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
			c, err := a.api()
			if err != nil {
				return err
			}
			r, err := a.pickRental(a.ctx(cmd), c, as)
			if err != nil {
				return err
			}
			e, err := a.estimate(cmd, c, r.Id, retention())
			if err != nil {
				return err
			}
			if !a.quiet {
				writeEstimate(a.env.Stderr, e)
			}
			if err := a.confirm(fmt.Sprintf("Back up %s for %s?", r.Name, output.Money(e.Price.AmountMinor, string(e.Price.Currency))), "refusing to start a charged backup without confirmation"); err != nil {
				return err
			}
			body := api.CreateBackupRequest{RetentionDays: retention()}
			if deleteAfter {
				body.DeleteRentalAfter = &deleteAfter
			}
			key := uuid.NewString()
			resp, err := c.CreateRentalBackupWithResponse(a.ctx(cmd), r.Id, &api.CreateRentalBackupParams{IdempotencyKey: &key}, body)
			if err := a.ok(resp, err); err != nil {
				return err
			}
			b := resp.JSON202
			a.info("Backup %s started (state: %s).", b.Id, b.State)
			a.info("Follow it: legion rental backup get %s", b.Id)
			return a.emit(backupResult([]api.Backup{*b}, true))
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
		Long:    "List backups of one rental, or of every rental when none is given.",
		Example: "  legion rental backup list\n  legion rental backup list quiet-river-7k2f --state ready",
		Args:    args(0, 1),
		RunE: func(cmd *cobra.Command, as []string) error {
			c, err := a.api()
			if err != nil {
				return err
			}
			var bs []api.Backup
			if len(as) == 1 {
				r, err := a.pickRental(a.ctx(cmd), c, as)
				if err != nil {
					return err
				}
				bs, err = listAll(func(cur *string) ([]api.Backup, *string, error) {
					resp, err := c.ListRentalBackupsWithResponse(a.ctx(cmd), r.Id, &api.ListRentalBackupsParams{Cursor: cur, Limit: ptr(100)})
					if err := a.ok(resp, err); err != nil {
						return nil, nil, err
					}
					return resp.JSON200.Items, resp.JSON200.NextCursor, nil
				})
				if err != nil {
					return err
				}
			} else {
				p := api.ListBackupsParams{Limit: ptr(100)}
				if state != "" {
					p.State = ptr(api.BackupState(state))
				}
				bs, err = listAll(func(cur *string) ([]api.Backup, *string, error) {
					p.Cursor = cur
					resp, err := c.ListBackupsWithResponse(a.ctx(cmd), &p)
					if err := a.ok(resp, err); err != nil {
						return nil, nil, err
					}
					return resp.JSON200.Items, resp.JSON200.NextCursor, nil
				})
				if err != nil {
					return err
				}
			}
			if bs == nil {
				bs = []api.Backup{}
			}
			return a.emit(backupResult(bs, false))
		},
	}
	list.Flags().StringVar(&state, "state", "", "only backups in this state: queued, uploading, ready, failed, cancelled, expired, deleted")

	byID := func(use, short, long, ex string, run func(cmd *cobra.Command, c *api.ClientWithResponses, id uuid.UUID) error) *cobra.Command {
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
				return run(cmd, c, id)
			},
		}
	}
	get := byID("get <backup-id>", "Show one backup", "Show a backup's state, size, progress, expiry and price.",
		"  legion rental backup get 6f1c2e3a-...\n  legion rental backup get 6f1c2e3a-... -o json",
		func(cmd *cobra.Command, c *api.ClientWithResponses, id uuid.UUID) error {
			r, err := c.GetBackupWithResponse(a.ctx(cmd), id)
			if err := a.ok(r, err); err != nil {
				return err
			}
			if r.JSON200.State == api.BackupStateReady {
				a.info("Download: legion rental backup download-link %s", r.JSON200.Id)
			}
			return a.emit(backupResult([]api.Backup{*r.JSON200}, true))
		})
	del := byID("delete <backup-id>", "Delete a backup", "Delete a backup and its data for good.",
		"  legion rental backup delete 6f1c2e3a-... --yes\n  legion rental backup delete 6f1c2e3a-...",
		func(cmd *cobra.Command, c *api.ClientWithResponses, id uuid.UUID) error {
			if err := a.confirm(fmt.Sprintf("Delete backup %s for good?", id), "refusing to delete a backup without confirmation"); err != nil {
				return err
			}
			r, err := c.DeleteBackupWithResponse(a.ctx(cmd), id)
			if err := a.ok(r, err); err != nil {
				return err
			}
			a.info("Deleted backup %s.", id)
			return nil
		})
	cancel := byID("cancel <backup-id>", "Cancel a backup in progress", "Cancel a queued or uploading backup. The charge is refunded.",
		"  legion rental backup cancel 6f1c2e3a-...\n  legion rental backup cancel 6f1c2e3a-... -q",
		func(cmd *cobra.Command, c *api.ClientWithResponses, id uuid.UUID) error {
			r, err := c.CancelBackupWithResponse(a.ctx(cmd), id)
			if err := a.ok(r, err); err != nil {
				return err
			}
			a.info("Cancelled backup %s.", id)
			return a.emit(backupResult([]api.Backup{*r.JSON200}, true))
		})
	link := byID("download-link <backup-id>", "Make a download link for a ready backup",
		"Create a time-limited URL to download a ready backup. Further links are charged; the charge is printed.",
		"  legion rental backup download-link 6f1c2e3a-...\n  legion rental backup download-link 6f1c2e3a-... -o jsonpath=$.url",
		func(cmd *cobra.Command, c *api.ClientWithResponses, id uuid.UUID) error {
			key := uuid.NewString()
			r, err := c.CreateBackupDownloadLinkWithResponse(a.ctx(cmd), id, &api.CreateBackupDownloadLinkParams{IdempotencyKey: &key})
			if err := a.ok(r, err); err != nil {
				return err
			}
			l := r.JSON201
			a.info("Link %d, charge %s, expires %s.", l.LinkNumber, output.Money(l.Charge.AmountMinor, string(l.Charge.Currency)), ts(l.ExpiresAt))
			return a.emit(output.Result{Data: l, Names: []string{l.Url}, Human: func(w io.Writer) { fmt.Fprintln(w, l.Url) }})
		})
	cmd.AddCommand(estimate, create, list, get, del, cancel, link)
	return cmd
}

func (a *app) estimate(cmd *cobra.Command, c *api.ClientWithResponses, id api.RentalID, days *api.BackupRetentionDays) (*api.BackupEstimate, error) {
	r, err := c.EstimateRentalBackupWithResponse(a.ctx(cmd), id, &api.EstimateRentalBackupParams{RetentionDays: days})
	if err := a.ok(r, err); err != nil {
		return nil, err
	}
	return r.JSON200, nil
}

func writeEstimate(w io.Writer, e *api.BackupEstimate) {
	cur := string(e.Price.Currency)
	fmt.Fprintf(w, "Estimated size:  %s (%s)\nPrice:           %s (storage %s + download %s), kept %d days\n",
		output.Bytes(e.EstimatedSizeBytes), e.EstimateBasis, output.Money(e.Price.AmountMinor, cur), output.Money(e.Storage.AmountMinor, cur), output.Money(e.Download.AmountMinor, cur), e.RetentionDays)
	if !e.FitsUploadWindow {
		fmt.Fprintln(w, "Warning:         this may not finish uploading in time on the host's connection.")
	}
}

func backupResult(bs []api.Backup, detail bool) output.Result {
	t := &output.Table{Cols: []string{"ID", "RENTAL", "STATE", "SIZE", "PRICE", "EXPIRES"}}
	names := make([]string, len(bs))
	for i, b := range bs {
		names[i] = b.Id.String()
		size := "~" + output.Bytes(b.EstimatedSizeBytes)
		if b.SizeBytes != nil {
			size = output.Bytes(*b.SizeBytes)
		}
		t.Rows = append(t.Rows, []string{b.Id.String(), ptrOr(b.RentalName, b.RentalId.String()), string(b.State), size, output.Money(b.Price.Quoted.AmountMinor, string(b.Price.Quoted.Currency)), tsp(b.ExpiresAt)})
	}
	var data any = bs
	if detail && len(bs) == 1 {
		data = bs[0]
	}
	return output.Result{Data: data, Names: names, Table: t}
}

// ---- workspace export ----

func (a *app) exportCmd() *cobra.Command {
	cmd := group(&cobra.Command{
		Use:     "export",
		Aliases: []string{"exports"},
		Short:   "Export a rental's workspace to a download",
		Long:    "A workspace export uploads the files of a running rental to a download URL. Unlike a backup it is not kept after it expires.",
		Example: `  legion rental export create quiet-river-7k2f
  legion rental export get quiet-river-7k2f <export-id>`,
	})
	res := func(es []api.WorkspaceExport) output.Result {
		t := &output.Table{Cols: []string{"ID", "STATE", "SIZE", "EXPIRES"}, Wide: []string{"DOWNLOAD URL"}}
		names := make([]string, len(es))
		for i, e := range es {
			names[i] = e.Id.String()
			size := ""
			if e.SizeBytes != nil {
				size = output.Bytes(*e.SizeBytes)
			}
			t.Rows = append(t.Rows, []string{e.Id.String(), string(e.State), size, tsp(e.ExpiresAt), ptrOr(e.DownloadUrl, "")})
		}
		var data any = es
		if len(es) == 1 {
			data = es[0]
		}
		return output.Result{Data: data, Names: names, Table: t}
	}
	create := &cobra.Command{
		Use:     "create <rental>",
		Short:   "Start exporting a workspace",
		Long:    "Start an export of the rental's workspace. Follow it with 'rental export get'; when ready it has a download URL.",
		Example: "  legion rental export create quiet-river-7k2f\n  legion rental export create quiet -o name",
		Args:    args(0, 1),
		RunE: func(cmd *cobra.Command, as []string) error {
			c, err := a.api()
			if err != nil {
				return err
			}
			r, err := a.pickRental(a.ctx(cmd), c, as)
			if err != nil {
				return err
			}
			resp, err := c.CreateWorkspaceExportWithResponse(a.ctx(cmd), r.Id)
			if err := a.ok(resp, err); err != nil {
				return err
			}
			a.info("Export %s started. Follow it: legion rental export get %s %s", resp.JSON202.Id, r.Name, resp.JSON202.Id)
			return a.emit(res([]api.WorkspaceExport{*resp.JSON202}))
		},
	}
	list := &cobra.Command{
		Use:     "list <rental>",
		Short:   "List a rental's exports",
		Long:    "List the workspace exports of a rental.",
		Example: "  legion rental export list quiet-river-7k2f\n  legion rental export list quiet -o json",
		Args:    args(0, 1),
		RunE: func(cmd *cobra.Command, as []string) error {
			c, err := a.api()
			if err != nil {
				return err
			}
			r, err := a.pickRental(a.ctx(cmd), c, as)
			if err != nil {
				return err
			}
			resp, err := c.ListWorkspaceExportsWithResponse(a.ctx(cmd), r.Id)
			if err := a.ok(resp, err); err != nil {
				return err
			}
			es := resp.JSON200.Items
			if es == nil {
				es = []api.WorkspaceExportSummary{}
			}
			t := &output.Table{Cols: []string{"ID", "STATE", "SIZE", "REQUESTED"}}
			names := make([]string, len(es))
			for i, e := range es {
				names[i] = e.Id.String()
				size := ""
				if e.SizeBytes != nil {
					size = output.Bytes(*e.SizeBytes)
				}
				t.Rows = append(t.Rows, []string{e.Id.String(), string(e.State), size, ts(e.RequestedAt)})
			}
			return a.emit(output.Result{Data: es, Names: names, Table: t})
		},
	}
	exportByID := func(use, short, long, ex string, run func(cmd *cobra.Command, c *api.ClientWithResponses, r *api.Rental, id uuid.UUID) error) *cobra.Command {
		return &cobra.Command{
			Use: use, Short: short, Long: long, Example: ex, Args: args(2, 2),
			RunE: func(cmd *cobra.Command, as []string) error {
				id, err := uuid.Parse(as[1])
				if err != nil {
					return usagef("legion rental export list "+as[0], "%q is not an export id", as[1])
				}
				c, err := a.api()
				if err != nil {
					return err
				}
				r, err := a.pickRental(a.ctx(cmd), c, as[:1])
				if err != nil {
					return err
				}
				return run(cmd, c, r, id)
			},
		}
	}
	get := exportByID("get <rental> <export-id>", "Show one export", "Show an export's state and, when ready, its download URL.",
		"  legion rental export get quiet-river-7k2f 6f1c...\n  legion rental export get quiet 6f1c... -o jsonpath=$.download_url",
		func(cmd *cobra.Command, c *api.ClientWithResponses, r *api.Rental, id uuid.UUID) error {
			resp, err := c.GetWorkspaceExportWithResponse(a.ctx(cmd), r.Id, id)
			if err := a.ok(resp, err); err != nil {
				return err
			}
			return a.emit(res([]api.WorkspaceExport{*resp.JSON200}))
		})
	cancel := exportByID("cancel <rental> <export-id>", "Cancel an export", "Cancel an export that is still running.",
		"  legion rental export cancel quiet-river-7k2f 6f1c...\n  legion rental export cancel quiet 6f1c... -q",
		func(cmd *cobra.Command, c *api.ClientWithResponses, r *api.Rental, id uuid.UUID) error {
			resp, err := c.CancelWorkspaceExportWithResponse(a.ctx(cmd), r.Id, id)
			if err := a.ok(resp, err); err != nil {
				return err
			}
			a.info("Cancelled export %s.", id)
			return nil
		})
	cmd.AddCommand(create, list, get, cancel)
	return cmd
}
