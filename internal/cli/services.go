package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/AmbientCrypto/legion-controller/api"
	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/AmbientCrypto/legion-cli/internal/client"
	"github.com/AmbientCrypto/legion-cli/internal/output"
)

const servicesLong = `A service is a public URL for something you run on Legion. Point it at a port on one of your rentals, or let Legion run your model on rented GPUs and scale it with traffic, down to zero when idle.
A service is either on a rental (autoscaling off, one rental) or autoscaled (a pool of replicas);
both keep the same Service URL and service keys when you switch with 'legion services autoscale on|off'.

Replicas bill like rentals, per GPU-hour from start, including loading. Every create and spend change shows
a cost floor and ceiling and asks for confirmation (or pass --yes). The monthly cap (UTC calendar month)
stops serving until the next month or until you raise it. A replica is what Vast and RunPod call a worker.`

func (a *app) servicesCmd() *cobra.Command {
	cmd := group(&cobra.Command{
		Use:     "services",
		Aliases: []string{"svc"},
		Short:   "Give what you run a public URL, with optional service keys",
		Long:    servicesLong,
		Example: `  legion services list
  legion services create chat --catalog llama-3-8b --max-price 2.50 --monthly-cap 400
  legion services keys create chat --name ci --expires 30d`,
	})
	cmd.AddCommand(a.svcListCmd(), a.svcGetCmd(), a.svcCreateCmd(), a.svcUpdateCmd(), a.svcPauseCmd(), a.svcResumeCmd(), a.svcDeleteCmd(),
		a.svcReplicasCmd(), a.svcUsageCmd(), a.svcEventsCmd(), a.svcAutoscaleCmd(), a.svcKeysCmd())
	return cmd
}

// ---- fetching and resolving ----

func (a *app) listServices(ctx context.Context, c *api.ClientWithResponses, rental *api.RentalID) ([]api.Service, error) {
	return collect(0, func(cur *string, n int) ([]api.Service, *string, error) {
		r, err := c.ListServicesWithResponse(ctx, &api.ListServicesParams{RentalId: rental, Cursor: cur, Limit: &n})
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

// pickService resolves the <service> argument (id, name or unique prefix),
// or offers a picker on a terminal when it is missing.
func (a *app) pickService(ctx context.Context, c *api.ClientWithResponses, as []string) (*api.Service, error) {
	if len(as) == 0 && !a.interactive() {
		return nil, usagef("legion "+a.cmdPath+" --help", "missing required argument <service>")
	}
	all, err := a.listServices(ctx, c, nil)
	if err != nil {
		return nil, err
	}
	if len(as) == 0 {
		labels := make([]string, len(all))
		for i, e := range all {
			labels[i] = output.Sanitize(fmt.Sprintf("%-24s %-17s %s", e.Name, modeLabel(e.Mode), e.Status))
		}
		i, err := a.pickOne("Which service?", labels, "missing required argument <service>", "legion services list")
		if err != nil {
			return nil, err
		}
		return &all[i], nil
	}
	arg := strings.ToLower(strings.TrimSpace(as[0]))
	var hits []int
	for i, e := range all {
		if e.Id.String() == arg || e.Name == arg {
			return &all[i], nil
		}
		if strings.HasPrefix(e.Name, arg) || strings.HasPrefix(e.Id.String(), arg) {
			hits = append(hits, i)
		}
	}
	switch len(hits) {
	case 0:
		return nil, notFoundf("legion services list", "no service %q", as[0])
	case 1:
		return &all[hits[0]], nil
	}
	var names []string
	for _, i := range hits {
		names = append(names, all[i].Name)
	}
	return nil, usagef("legion services list", "%q matches several services: %s", as[0], strings.Join(names, ", "))
}

func (a *app) withService(cmd *cobra.Command, as []string, fn func(ctx context.Context, c *api.ClientWithResponses, e *api.Service) error) error {
	c, err := a.api()
	if err != nil {
		return err
	}
	ctx := a.ctx(cmd)
	e, err := a.pickService(ctx, c, as)
	if err != nil {
		return err
	}
	return fn(ctx, c, e)
}

// ---- labels ----

func modeLabel(m api.ServiceMode) string {
	switch m {
	case api.ServiceModeRental:
		return "On a rental"
	case api.ServiceModeAutoscaled:
		return "Autoscaled"
	}
	return string(m)
}

// accessWord is the label for an access mode: the --access flag values stay
// public and keys, the words shown to people are Public and API key required.
func accessWord(m api.ServiceAccessMode) string {
	switch m {
	case api.ServiceAccessPublic:
		return "Public"
	case api.ServiceAccessKeys:
		return "API key required"
	}
	return string(m)
}

func svcAutoscaled(e api.Service) bool { return e.Target.Mode == api.ServiceModeAutoscaled }

func svcScaling(e api.Service) api.ServiceScaling {
	if e.Target.Scaling != nil {
		return *e.Target.Scaling
	}
	return api.ServiceScaling{}
}

func svcHardware(e api.Service) api.ServiceHardware {
	if e.Target.Hardware != nil {
		return *e.Target.Hardware
	}
	return api.ServiceHardware{}
}

func svcCatalog(e api.Service) *string {
	if e.Target.Template != nil {
		return e.Target.Template.CatalogId
	}
	return nil
}

func scalingLabel(e api.Service) string {
	if !svcAutoscaled(e) {
		return "Off, 1 rental"
	}
	s := svcScaling(e)
	return fmt.Sprintf("Auto, %d to %d", s.MinReplicas, s.MaxReplicas)
}

func accessLabel(a api.ServiceAccess) string {
	s := accessWord(a.Mode)
	if a.Sources != nil && len(*a.Sources) > 0 {
		s += ", limited to " + strings.Join(*a.Sources, ",")
	}
	return s
}

func moneyStr(m api.Money) string { return output.Money(m.AmountMinor, string(m.Currency)) }

func moneyPtr(m *api.Money) string {
	if m == nil {
		return "-"
	}
	return moneyStr(*m)
}

func tierLabel(t *api.RentalTier) string {
	if t != nil && *t == api.RentalTierInterruptible {
		return "spot"
	}
	return "on demand"
}

func hwText(h api.ServiceHardware) string {
	s := "cheapest that fits"
	if h.GpuModels != nil && len(*h.GpuModels) > 0 {
		s += ", only " + san(strings.Join(*h.GpuModels, ", "))
	}
	if h.MinGpuMemoryGb != nil {
		s += fmt.Sprintf(", at least %.0f GB GPU memory", *h.MinGpuMemoryGb)
	}
	if h.GpuCounts != nil && len(*h.GpuCounts) > 0 {
		var cs []string
		for _, c := range *h.GpuCounts {
			cs = append(cs, fmt.Sprint(int(c)))
		}
		s += ", GPU counts " + strings.Join(cs, ",")
	}
	s += ", " + tierLabel(h.Tier)
	if h.MaxPricePerReplicaHour != nil {
		s += ", up to " + moneyStr(*h.MaxPricePerReplicaHour) + " per replica-hour"
	}
	return s
}

func statusText(e api.Service) string {
	s := string(e.Status)
	if e.StatusReason != nil && *e.StatusReason != "" {
		s += " (" + reasonText(*e.StatusReason) + ")"
	}
	return s
}

// reasonText words a status reason in plain language; reasons it does not
// know are shown as the API sent them.
func reasonText(r string) string {
	switch r {
	case "service_unavailable":
		return "Legion cannot give this port a public URL right now"
	case "service_limit":
		return "limit of services reached"
	}
	return san(r)
}

func svcResult(es []api.Service) output.Result {
	return listResult(es, []string{"NAME", "MODE", "ACCESS", "SCALING", "STATUS", "REPLICAS", "URL"}, []string{"ID", "SPEND THIS MONTH"},
		func(e api.Service) string { return e.Name },
		func(e api.Service) []string {
			spend := moneyStr(e.SpendThisMonth.Estimated)
			if e.SpendThisMonth.Cap != nil {
				spend += " of " + moneyStr(*e.SpendThisMonth.Cap)
			}
			return []string{san(e.Name), modeLabel(e.Mode), accessWord(e.Access.Mode), scalingLabel(e), string(e.Status),
				fmt.Sprintf("%d/%d", e.Replicas.Ready, e.Replicas.Desired), san(e.Url), e.Id.String(), spend}
		})
}

func svcDetail(w io.Writer, e api.Service) {
	fmt.Fprintf(w, "Name:       %s\nID:         %s\nService URL: %s\nMode:       %s\nAccess:     %s\nScaling:    %s\nStatus:     %s\nState:      %s\nProtocol:   %s\n",
		san(e.Name), e.Id, san(e.Url), modeLabel(e.Mode), accessLabel(e.Access), scalingLabel(e), statusText(e), e.State, e.Protocol)
	t := e.Target
	if svcAutoscaled(e) {
		if tp := t.Template; tp != nil {
			defer writeStoredConfig(w, tp.Env, tp.ModelSettings, tp.Command, tp.Args)
			src := tp.Image
			if tp.CatalogId != nil && *tp.CatalogId != "" {
				src = *tp.CatalogId
			}
			fmt.Fprintf(w, "Model:      %s (%s on port %d)\n", san(src), san(tp.Model), tp.Port)
		}
		fmt.Fprintf(w, "Hardware:   %s\n", hwText(svcHardware(e)))
		s := svcScaling(e)
		fmt.Fprintf(w, "Autoscale:  %d to %d replicas, target %d concurrent requests each, scale to zero %s\n",
			s.MinReplicas, s.MaxReplicas, s.TargetConcurrency, map[bool]string{true: "on", false: "off"}[s.ScaleToZero])
	} else if t.RentalId != nil {
		fmt.Fprintf(w, "Rental:     %s port %d\n", t.RentalId, ptrOr(t.Port, 0))
	}
	r := e.Replicas
	fmt.Fprintf(w, "Replicas:   %d ready, %d starting, %d draining, %d warm, %d wanted\n", r.Ready, r.Provisioning, r.Draining, r.Warm, r.Desired)
	fmt.Fprintf(w, "Spend:      %s this month (%s), cap %s\n", moneyStr(e.SpendThisMonth.Estimated), e.SpendThisMonth.Month, moneyPtr(e.SpendThisMonth.Cap))
	fmt.Fprintf(w, "Keys:       legion services keys list %s\n", san(e.Name))
}

// ---- list / get ----

func (a *app) svcListCmd() *cobra.Command {
	var mode, rental string
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List services",
		Long:    "List the organization's services, on a rental or autoscaled, with access, scaling, status and Service URL. Filter with --mode rental|autoscaled or --rental <rental>. Use -o wide for ids and spend.",
		Example: "  legion services list\n  legion services list --mode rental -o name\n  legion services list --rental quiet-river-7k2f",
		Args:    args(0, 0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			var want api.ServiceMode
			switch mode {
			case "":
			case "rental":
				want = api.ServiceModeRental
			case "autoscaled":
				want = api.ServiceModeAutoscaled
			default:
				return usagef("legion services list --help", "--mode must be rental or autoscaled, got %q", mode)
			}
			c, err := a.api()
			if err != nil {
				return err
			}
			ctx := a.ctx(cmd)
			var rid *api.RentalID
			if rental != "" {
				r, err := a.pickRental(ctx, c, []string{rental})
				if err != nil {
					return err
				}
				rid = &r.Id
			}
			ss, err := a.listServices(ctx, c, rid)
			if err != nil {
				return err
			}
			kept := ss[:0:0]
			for _, s := range ss {
				if want == "" || s.Mode == want {
					kept = append(kept, s)
				}
			}
			if len(kept) == 0 {
				a.info("No services yet. Create one: legion services create --help")
			}
			return a.emit(svcResult(kept))
		},
	}
	cmd.Flags().StringVar(&mode, "mode", "", "only this mode: rental or autoscaled")
	cmd.Flags().StringVar(&rental, "rental", "", "only the services on this rental")
	return cmd
}

func (a *app) svcGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "get <service>",
		Short:   "Show one service",
		Long:    "Show a service's URL, status, hardware, replicas and spend. <service> is an id, a name or a unique prefix.",
		Example: "  legion services get chat\n  legion services get chat -o json",
		Args:    args(0, 1),
		RunE: func(cmd *cobra.Command, as []string) error {
			return a.withService(cmd, as, func(ctx context.Context, c *api.ClientWithResponses, e *api.Service) error {
				r, err := c.GetServiceWithResponse(ctx, e.Id)
				if err := a.ok(r, err); err != nil {
					return err
				}
				fresh, err := need(r.JSON200)
				if err != nil {
					return err
				}
				return a.emit(output.Result{Data: fresh, Names: []string{fresh.Name}, Human: func(w io.Writer) { svcDetail(w, *fresh) }})
			})
		},
	}
}

// ---- update / pause / resume / delete ----

func (a *app) svcUpdateCmd() *cobra.Command {
	var sf scaleFlags
	var hf hwFlags
	var capStr, catalog, image, access string
	var sources []string
	var revision int64
	var cf configFlags
	var unsetEnv []string
	var clearCommand, clearArgs, clearSettings bool
	cmd := &cobra.Command{
		Use:   "update <service>",
		Short: "Change a service's access, scaling, hardware, price ceiling or cap",
		Long: `Change only the settings you name; everything else stays and the URL never changes.
--access public|keys switches who may call it (keys can exist while it is public, so there is no downtime);
--source <cidr> (repeatable) limits callers to those networks in either mode.
Scaling, hardware, --catalog and --image apply to autoscaled services; changes that can raise spend show the new
cost floor and ceiling and ask for confirmation (or pass --yes). Config changes roll one replica at a time with one
surge replica, which counts against the maximum and the monthly cap. --revision refuses the change when the service
was edited meanwhile. Turn autoscaling on or off with 'legion services autoscale'.

Container configuration (autoscaled services): --env and --secret-env add or change variables and --unset-env removes
them; the others stay. A secret you do not name keeps its stored value. --setting changes the named model settings,
--command and --arg replace the entrypoint and arguments of your own image; --clear-settings, --clear-command and
--clear-args remove them. Any of these changes the replica template, so replicas are replaced one at a time with one
surge replica and a running replica restarts. Anything in the container can be read by the provider's machine.`,
		Example: "  legion services update chat --max 4\n  legion services update chat --access keys\n  legion services update chat --gpu H100 --gpu A100 --max-price 3 --monthly-cap 600",
		Args:    args(0, 1),
		RunE: func(cmd *cobra.Command, as []string) error {
			fl := cmd.Flags()
			scIn := sf.input(cmd)
			hwChanged := hf.changed(cmd)
			capChanged := fl.Changed("monthly-cap")
			accessChanged := access != "" || len(sources) > 0
			cfgChanged := cf.any() || len(unsetEnv) > 0 || clearCommand || clearArgs || clearSettings
			if (clearCommand && len(cf.command) > 0) || (clearArgs && len(cf.arg) > 0) || (clearSettings && len(cf.setting) > 0) {
				return usagef("legion services update --help", "do not combine --clear-command, --clear-args or --clear-settings with the flag that sets the same thing")
			}
			if access != "" && access != "public" && access != "keys" {
				return usagef("legion services update --help", "--access must be public or keys, got %q", access)
			}
			if scIn == nil && !hwChanged && !capChanged && !accessChanged && catalog == "" && image == "" && !cfgChanged {
				return usagef("legion services update --help", "nothing to change: pass at least one of --env, --secret-env, --unset-env, --setting, --command, --arg, --access, --source, --min, --max, --scale-to-zero, --target-concurrency, --gpu, --max-price, --spot, --monthly-cap, --catalog, --image")
			}
			cfg, err := a.resolveConfig(cmd, &cf, "services update")
			if err != nil {
				return err
			}
			for _, n := range unsetEnv {
				if err := checkEnvName(n); err != nil {
					return usagef("legion services update --help", "--unset-env: %v", err)
				}
			}
			return a.withService(cmd, as, func(ctx context.Context, c *api.ClientWithResponses, e *api.Service) error {
				req := api.UpdateServiceRequest{}
				if fl.Changed("revision") {
					req.Revision = &revision
				}
				spend := false
				if cfgChanged && !svcAutoscaled(*e) {
					return usagef("legion services get "+e.Name, "%s exposes a rental's port; container configuration belongs to autoscaled services (for a rental: legion rental env)", e.Name)
				}
				if scIn != nil || hwChanged || catalog != "" || image != "" || cfgChanged {
					tc := api.ServiceTargetChange{Scaling: scIn}
					if cfgChanged {
						tp := e.Target.Template
						isCatalog := catalog != "" || (image == "" && tp != nil && tp.CatalogId != nil && *tp.CatalogId != "")
						if err := checkServiceConfig(cfg, isCatalog); err != nil {
							return err
						}
						var curEnv []api.EnvVar
						var curSet *api.ModelSettings
						if tp != nil {
							curEnv, curSet = ptrOrSlice(tp.Env), tp.ModelSettings
						}
						if cf.any() || len(unsetEnv) > 0 {
							var set []envVar
							if cfg != nil {
								set = cfg.Env
							}
							if len(set) > 0 || len(unsetEnv) > 0 {
								tc.Env = ptr(mergeEnv(curEnv, set, unsetEnv))
							}
							if cfg != nil && len(cfg.Settings) > 0 {
								tc.ModelSettings = ptr(mergeSettings(curSet, cfg.Settings))
							}
							tc.Command, tc.Args = cfg.commandOverride(), cfg.argsOverride()
						}
						if clearSettings {
							tc.ModelSettings = &api.ModelSettings{}
						}
						if clearCommand {
							tc.Command = ptr(api.CommandOverride{})
						}
						if clearArgs {
							tc.Args = ptr(api.CommandOverride{})
						}
						if !a.quiet {
							w := a.env.Stderr
							if tc.Env != nil {
								for _, v := range *tc.Env {
									if ptrOr(v.Secret, false) {
										fmt.Fprintf(w, "  Env:       %s = (secret, hidden)\n", san(v.Name))
									} else {
										fmt.Fprintf(w, "  Env:       %s = %s\n", san(v.Name), san(ptrOr(v.Value, "")))
									}
								}
							}
							fmt.Fprintln(w, "This changes the replica template: replicas are replaced one at a time, so a running replica restarts.")
						}
					}
					if hwChanged {
						hw, err := hf.apply(cmd, svcHardware(*e))
						if err != nil {
							return err
						}
						tc.Hardware = &hw
					}
					if catalog != "" {
						tc.CatalogId = &catalog
					}
					if image != "" {
						tc.Image = &image
					}
					req.Target = &tc
					spend = true
				}
				var capM *api.Money
				if capChanged {
					m, err := dollars("--monthly-cap", capStr)
					if err != nil {
						return err
					}
					capM, req.MonthlySpendCap, spend = &m, &m, true
				} else {
					capM = e.MonthlySpendCap
				}
				if accessChanged {
					acc := api.ServiceAccess{Mode: e.Access.Mode, Sources: e.Access.Sources}
					if access != "" {
						acc.Mode = api.ServiceAccessMode(access)
					}
					if len(sources) > 0 {
						acc.Sources = &sources
					}
					req.Access = &acc
					if acc.Mode == api.ServiceAccessPublic && e.Access.Mode != api.ServiceAccessPublic {
						if err := a.confirm(fmt.Sprintf("Make %s public? Callers no longer need a key.", e.Name), "refusing to make "+e.Name+" public without confirmation: callers would no longer need a key"); err != nil {
							return err
						}
					}
				}
				if spend && svcAutoscaled(*e) {
					var hw api.ServiceHardware
					if req.Target != nil && req.Target.Hardware != nil {
						hw = *req.Target.Hardware
					} else {
						hw = svcHardware(*e)
					}
					sc := overlay(svcScaling(*e), scIn)
					est, err := a.svcEstimate(ctx, c, svcCatalog(*e), hw, sc, capM)
					if err != nil {
						return err
					}
					a.summary(e.Name, hw, sc, est, capM)
					q := fmt.Sprintf("Apply these changes to %s? They can raise what it spends.", e.Name)
					if err := a.confirm(q, "refusing to change "+e.Name+" without confirmation: the new floor and ceiling are shown above"); err != nil {
						return err
					}
				}
				r, err := c.UpdateServiceWithResponse(ctx, e.Id, req)
				if err := a.ok(r, err); err != nil {
					return err
				}
				upd, err := need(r.JSON200)
				if err != nil {
					return err
				}
				a.info("Updated %s.", upd.Name)
				return a.emit(output.Result{Data: upd, Names: []string{upd.Name}, Human: func(w io.Writer) { svcDetail(w, *upd) }})
			})
		},
	}
	sf.register(cmd)
	hf.register(cmd)
	fl := cmd.Flags()
	fl.StringVar(&access, "access", "", "public (anyone) or keys (callers need a service key)")
	fl.StringArrayVar(&sources, "source", nil, "allowed source CIDR, for example 203.0.113.0/24 (repeatable)")
	fl.StringVar(&capStr, "monthly-cap", "", "monthly spend cap in dollars (UTC calendar month)")
	fl.StringVar(&catalog, "catalog", "", "switch to this catalog model id")
	fl.StringVar(&image, "image", "", "switch to this container image")
	fl.Int64Var(&revision, "revision", 0, "refuse the change unless the service is at this revision")
	cf.register(cmd, "your own image")
	fl.StringArrayVar(&unsetEnv, "unset-env", nil, "remove this environment variable (repeatable)")
	fl.BoolVar(&clearSettings, "clear-settings", false, "remove all model settings (the image defaults apply)")
	fl.BoolVar(&clearCommand, "clear-command", false, "remove the entrypoint override")
	fl.BoolVar(&clearArgs, "clear-args", false, "remove the arguments override")
	return cmd
}

func (a *app) svcPauseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pause <service>",
		Short: "Stop serving and release the replicas",
		Long: `Pause: no replicas run and no GPU cost accrues. The URL and keys are kept and callers get 503
service_paused until you resume. Warm replicas are released after the warm expiry.`,
		Example: "  legion services pause chat\n  legion services pause chat -o json",
		Args:    args(0, 1),
		RunE: func(cmd *cobra.Command, as []string) error {
			return a.withService(cmd, as, func(ctx context.Context, c *api.ClientWithResponses, e *api.Service) error {
				r, err := c.PauseServiceWithResponse(ctx, e.Id)
				if err := a.ok(r, err); err != nil {
					return err
				}
				got := ptrOr(r.JSON200, *e)
				a.info("Paused %s. Resume: legion services resume %s", got.Name, got.Name)
				return a.emit(output.Result{Data: got, Names: []string{got.Name}, Human: func(w io.Writer) { svcDetail(w, got) }})
			})
		},
	}
}

func (a *app) svcResumeCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "resume <service>",
		Short:   "Start serving again",
		Long:    "Resume a paused service. Replicas start again and bill per GPU-hour; the cost floor and ceiling are shown and you confirm (or pass --yes).",
		Example: "  legion services resume chat\n  legion services resume chat --yes",
		Args:    args(0, 1),
		RunE: func(cmd *cobra.Command, as []string) error {
			return a.withService(cmd, as, func(ctx context.Context, c *api.ClientWithResponses, e *api.Service) error {
				est, err := a.svcEstimate(ctx, c, svcCatalog(*e), svcHardware(*e), svcScaling(*e), e.MonthlySpendCap)
				if err != nil {
					return err
				}
				a.summary(e.Name, svcHardware(*e), svcScaling(*e), est, e.MonthlySpendCap)
				if err := a.confirm(fmt.Sprintf("Resume %s? Replicas start and bill per GPU-hour.", e.Name), "refusing to resume "+e.Name+" without confirmation: it starts billed replicas"); err != nil {
					return err
				}
				r, err := c.ResumeServiceWithResponse(ctx, e.Id)
				if err := a.ok(r, err); err != nil {
					return err
				}
				got := ptrOr(r.JSON200, *e)
				a.info("Resumed %s: %s", got.Name, san(got.Url))
				return a.emit(output.Result{Data: got, Names: []string{got.Name}, Human: func(w io.Writer) { svcDetail(w, got) }})
			})
		},
	}
}

func (a *app) svcDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "delete <service>",
		Aliases: []string{"rm"},
		Short:   "Delete a service, its replicas and its keys",
		Long: `Delete the service: replicas stop, every key stops working and the URL goes away. This cannot be undone.
On a terminal you must type the service's name to confirm; in scripts pass --yes. The API may ask for step-up
verification, which needs a browser session.`,
		Example: "  legion services delete chat\n  legion services delete chat --yes",
		Args:    args(0, 1),
		RunE: func(cmd *cobra.Command, as []string) error {
			return a.withService(cmd, as, func(ctx context.Context, c *api.ClientWithResponses, e *api.Service) error {
				if !a.yes {
					if !a.interactive() {
						return usagef("Add --yes to delete without a prompt.", "refusing to delete %s without confirmation: its replicas stop and all its keys are revoked for good", e.Name)
					}
					fmt.Fprintf(a.env.Stderr, "Deleting %s stops its replicas and revokes all its keys for good.\n", e.Name)
					got, err := a.prompt.Ask(fmt.Sprintf("Type the service name (%s) to confirm:", e.Name))
					if errors.Is(err, context.Canceled) {
						return err
					}
					if err != nil || got != e.Name {
						return &cliError{Msg: "aborted: the name did not match", Exit: client.ExitError}
					}
				}
				r, err := c.DeleteServiceWithResponse(ctx, e.Id)
				if err := a.ok(r, err); err != nil {
					return err
				}
				a.info("Deleting %s: replicas are stopping and billing ends.", e.Name)
				got := ptrOr(r.JSON202, *e)
				return a.emit(output.Result{Data: got, Names: []string{got.Name}, Human: func(w io.Writer) { svcDetail(w, got) }})
			})
		},
	}
}

// ---- replicas / usage / events ----

func (a *app) svcReplicasCmd() *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:     "replicas <service>",
		Short:   "List a service's replicas",
		Long:    "List the replicas (rentals) serving the service, live and warm. --all adds the replicas retired in the last 7 days.",
		Example: "  legion services replicas chat\n  legion services replicas chat --all -o wide",
		Args:    args(0, 1),
		RunE: func(cmd *cobra.Command, as []string) error {
			return a.withService(cmd, as, func(ctx context.Context, c *api.ClientWithResponses, e *api.Service) error {
				rs, err := a.listReplicas(ctx, c, e.Id, all)
				if err != nil {
					return err
				}
				return a.emit(listResult(rs, []string{"REPLICA", "PHASE", "READY", "GPU", "PRICE/HR", "TIER", "IN FLIGHT"}, []string{"RENTAL ID", "HOST", "STATE", "CREATED", "RETIRED"},
					func(r api.ServiceReplica) string { return replicaName(r) },
					func(r api.ServiceReplica) []string {
						host := ptrOr(r.Host.Name, "")
						if r.Host.Country != nil {
							host += " (" + *r.Host.Country + ")"
						}
						return []string{san(replicaName(r)), string(r.Phase), map[bool]string{true: "yes", false: "no"}[r.Ready],
							fmt.Sprintf("%dx %s", r.GpuCount, san(r.Host.GpuModel)), moneyStr(api.Money{AmountMinor: r.PricePerGpuHour.AmountMinor * int64(r.GpuCount), Currency: r.PricePerGpuHour.Currency}), tierLabel(&r.Tier), fmt.Sprint(ptrOr(r.InFlight, 0)),
							r.RentalId.String(), san(host), san(r.RentalState), r.CreatedAt.Format("2006-01-02 15:04"), san(ptrOr(r.RetireReason, ""))}
					}))
			})
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "include replicas retired in the last 7 days")
	return cmd
}

func replicaName(r api.ServiceReplica) string {
	if r.RentalName != nil && *r.RentalName != "" {
		return *r.RentalName
	}
	return r.RentalId.String()
}

func (a *app) listReplicas(ctx context.Context, c *api.ClientWithResponses, id api.ServiceID, retired bool) ([]api.ServiceReplica, error) {
	return collect(0, func(cur *string, n int) ([]api.ServiceReplica, *string, error) {
		p := &api.ListServiceReplicasParams{Cursor: cur, Limit: &n}
		if retired {
			p.IncludeRetired = &retired
		}
		r, err := c.ListServiceReplicasWithResponse(ctx, id, p)
		if err := a.ok(r, err); err != nil {
			return nil, nil, err
		}
		pg, err := need(r.JSON200)
		if err != nil {
			return nil, nil, err
		}
		return pg.Items, pg.NextCursor, nil
	})
}

func (a *app) svcUsageCmd() *cobra.Command {
	var days int
	var window string
	cmd := &cobra.Command{
		Use:     "usage <service>",
		Short:   "Show requests, errors, replica-hours and cost per day",
		Long:    "Show the service's usage per day (requests, errors, replica-hours, cost) and this month's spend against the monthly cap. Request-rate and latency points for the --window are in -o json.",
		Example: "  legion services usage chat\n  legion services usage chat --days 7 -o json",
		Args:    args(0, 1),
		RunE: func(cmd *cobra.Command, as []string) error {
			return a.withService(cmd, as, func(ctx context.Context, c *api.ClientWithResponses, e *api.Service) error {
				p := &api.GetServiceUsageParams{}
				if cmd.Flags().Changed("days") {
					p.Days = &days
				}
				if window != "" {
					w := api.ServiceMetricsWindow(window)
					if w != api.ServiceMetricsWindow1h && w != api.ServiceMetricsWindow24h && w != api.ServiceMetricsWindow7d {
						return usagef("legion services usage --help", "--window must be 1h, 24h or 7d, got %q", window)
					}
					p.Window = &w
				}
				r, err := c.GetServiceUsageWithResponse(ctx, e.Id, p)
				if err := a.ok(r, err); err != nil {
					return err
				}
				u, err := need(r.JSON200)
				if err != nil {
					return err
				}
				return a.emit(output.Result{Data: u, Human: func(w io.Writer) {
					tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
					fmt.Fprintln(tw, "DATE\tREQUESTS\tERRORS\tREPLICA-HOURS\tGPU-HOURS\tCOST")
					for _, d := range u.Days {
						fmt.Fprintf(tw, "%s\t%d\t%d\t%.1f\t%.1f\t%s\n", d.Date.Format("2006-01-02"), d.Requests, d.Errors, float64(d.ReplicaSeconds)/3600, float64(d.GpuSeconds)/3600, moneyStr(d.Cost))
					}
					_ = tw.Flush()
					s := u.SpendThisMonth
					fmt.Fprintf(w, "\nThis month (%s, UTC): %s estimated of a %s cap", s.Month, moneyStr(s.Estimated), moneyPtr(s.Cap))
					if s.AlertedPct > 0 {
						fmt.Fprintf(w, ", alerted at %d%%", int(s.AlertedPct))
					}
					fmt.Fprintln(w)
				}})
			})
		},
	}
	cmd.Flags().IntVar(&days, "days", 30, "how many days back")
	cmd.Flags().StringVar(&window, "window", "", "traffic window for the request-rate points in -o json: 1h, 24h or 7d")
	return cmd
}

func (a *app) svcEventsCmd() *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:     "events <service>",
		Short:   "Show scaling and configuration history",
		Long:    "Show what changed and why: scaling steps, config changes and key activity, newest first.",
		Example: "  legion services events chat\n  legion services events chat --limit 20 -o json",
		Args:    args(0, 1),
		RunE: func(cmd *cobra.Command, as []string) error {
			return a.withService(cmd, as, func(ctx context.Context, c *api.ClientWithResponses, e *api.Service) error {
				evs, err := collect(limit, func(cur *string, n int) ([]api.ServiceEvent, *string, error) {
					r, err := c.ListServiceEventsWithResponse(ctx, e.Id, &api.ListServiceEventsParams{Cursor: cur, Limit: &n})
					if err := a.ok(r, err); err != nil {
						return nil, nil, err
					}
					p, err := need(r.JSON200)
					if err != nil {
						return nil, nil, err
					}
					return p.Items, p.NextCursor, nil
				})
				if err != nil {
					return err
				}
				return a.emit(listResult(evs, []string{"WHEN", "EVENT", "REPLICAS", "REASON"}, []string{"ID", "ACTOR", "RENTAL"},
					func(ev api.ServiceEvent) string { return fmt.Sprint(ev.Id) },
					func(ev api.ServiceEvent) []string {
						chg := ""
						if ev.ReplicasBefore != nil && ev.ReplicasAfter != nil {
							chg = fmt.Sprintf("%d -> %d", *ev.ReplicasBefore, *ev.ReplicasAfter)
						}
						rental := ""
						if ev.RentalId != nil {
							rental = ev.RentalId.String()
						}
						return []string{ev.At.Format("2006-01-02 15:04:05"), san(ev.Kind), chg, san(ptrOr(ev.Reason, "")), fmt.Sprint(ev.Id), string(ev.Actor.Kind), rental}
					}))
			})
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "how many events at most (0 for all)")
	return cmd
}

var _ = uuid.Nil
