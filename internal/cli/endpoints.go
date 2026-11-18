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

const endpointsLong = `Endpoints are served models and exposed ports behind one URL and one set of access keys.
An endpoint is either a single rental (scaling off) or an autoscaled pool of replicas (scaling auto);
both keep the same URL and keys when you switch with 'legion endpoints autoscale on|off'.

Replicas bill like rentals, per GPU-hour from start, including loading. Every create and spend change shows
a cost floor and ceiling and asks for confirmation (or pass --yes). The monthly cap (UTC calendar month)
stops serving until the next month or until you raise it. A replica is what Vast and RunPod call a worker.`

func (a *app) endpointsCmd() *cobra.Command {
	cmd := group(&cobra.Command{
		Use:     "endpoints",
		Aliases: []string{"endpoint", "ep"},
		Short:   "Serve models and ports behind one URL with access keys",
		Long:    endpointsLong,
		Example: `  legion endpoints list
  legion endpoints create chat --catalog llama-3-8b --max-price 2.50 --monthly-cap 400
  legion endpoints keys create chat --name ci --expires 30d`,
	})
	cmd.AddCommand(a.epListCmd(), a.epGetCmd(), a.epCreateCmd(), a.epUpdateCmd(), a.epPauseCmd(), a.epResumeCmd(), a.epDeleteCmd(),
		a.epReplicasCmd(), a.epUsageCmd(), a.epEventsCmd(), a.epAutoscaleCmd(), a.epKeysCmd())
	return cmd
}

// ---- fetching and resolving ----

func (a *app) listEndpoints(ctx context.Context, c *api.ClientWithResponses) ([]api.Endpoint, error) {
	return collect(0, func(cur *string, n int) ([]api.Endpoint, *string, error) {
		r, err := c.ListEndpointsWithResponse(ctx, &api.ListEndpointsParams{Cursor: cur, Limit: &n})
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

// pickEndpoint resolves the <endpoint> argument (id, name or unique prefix),
// or offers a picker on a terminal when it is missing.
func (a *app) pickEndpoint(ctx context.Context, c *api.ClientWithResponses, as []string) (*api.Endpoint, error) {
	if len(as) == 0 && !a.interactive() {
		return nil, usagef("legion "+a.cmdPath+" --help", "missing required argument <endpoint>")
	}
	all, err := a.listEndpoints(ctx, c)
	if err != nil {
		return nil, err
	}
	if len(as) == 0 {
		labels := make([]string, len(all))
		for i, e := range all {
			labels[i] = output.Sanitize(fmt.Sprintf("%-24s %-17s %s", e.Name, typeLabel(e.Type), e.Status))
		}
		i, err := a.pickOne("Which endpoint?", labels, "missing required argument <endpoint>", "legion endpoints list")
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
		return nil, notFoundf("legion endpoints list", "no endpoint %q", as[0])
	case 1:
		return &all[hits[0]], nil
	}
	var names []string
	for _, i := range hits {
		names = append(names, all[i].Name)
	}
	return nil, usagef("legion endpoints list", "%q matches several endpoints: %s", as[0], strings.Join(names, ", "))
}

func (a *app) withEndpoint(cmd *cobra.Command, as []string, fn func(ctx context.Context, c *api.ClientWithResponses, e *api.Endpoint) error) error {
	c, err := a.api()
	if err != nil {
		return err
	}
	ctx := a.ctx(cmd)
	e, err := a.pickEndpoint(ctx, c, as)
	if err != nil {
		return err
	}
	return fn(ctx, c, e)
}

// ---- labels ----

func typeLabel(t api.EndpointType) string {
	switch t {
	case api.EndpointTypeModel:
		return "Model"
	case api.EndpointTypeAutoscaledModel:
		return "Autoscaled model"
	case api.EndpointTypePort:
		return "Port"
	}
	return string(t)
}

func epAutoscaled(e api.Endpoint) bool { return e.Target.Kind == api.EndpointTargetAutoscale }

func epScaling(e api.Endpoint) api.EndpointScaling {
	if e.Target.Scaling != nil {
		return *e.Target.Scaling
	}
	return api.EndpointScaling{}
}

func epHardware(e api.Endpoint) api.EndpointHardware {
	if e.Target.Hardware != nil {
		return *e.Target.Hardware
	}
	return api.EndpointHardware{}
}

func epCatalog(e api.Endpoint) *string {
	if e.Target.Template != nil {
		return e.Target.Template.CatalogId
	}
	return nil
}

func scalingLabel(e api.Endpoint) string {
	if !epAutoscaled(e) {
		return "Off, 1 rental"
	}
	s := epScaling(e)
	return fmt.Sprintf("Auto, %d to %d", s.MinReplicas, s.MaxReplicas)
}

func accessLabel(a api.EndpointAccess) string {
	s := string(a.Mode)
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

func hwText(h api.EndpointHardware) string {
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

func statusText(e api.Endpoint) string {
	s := string(e.Status)
	if e.StatusReason != nil && *e.StatusReason != "" {
		s += " (" + san(*e.StatusReason) + ")"
	}
	return s
}

func epResult(es []api.Endpoint) output.Result {
	return listResult(es, []string{"NAME", "TYPE", "ACCESS", "SCALING", "STATUS", "REPLICAS", "URL"}, []string{"ID", "SPEND THIS MONTH"},
		func(e api.Endpoint) string { return e.Name },
		func(e api.Endpoint) []string {
			spend := moneyStr(e.SpendThisMonth.Estimated)
			if e.SpendThisMonth.Cap != nil {
				spend += " of " + moneyStr(*e.SpendThisMonth.Cap)
			}
			return []string{san(e.Name), typeLabel(e.Type), string(e.Access.Mode), scalingLabel(e), string(e.Status),
				fmt.Sprintf("%d/%d", e.Replicas.Ready, e.Replicas.Desired), san(e.Url), e.Id.String(), spend}
		})
}

func epDetail(w io.Writer, e api.Endpoint) {
	fmt.Fprintf(w, "Name:       %s\nID:         %s\nURL:        %s\nType:       %s\nAccess:     %s\nScaling:    %s\nStatus:     %s\nState:      %s\nProtocol:   %s\n",
		san(e.Name), e.Id, san(e.Url), typeLabel(e.Type), accessLabel(e.Access), scalingLabel(e), statusText(e), e.State, e.Protocol)
	t := e.Target
	if epAutoscaled(e) {
		if tp := t.Template; tp != nil {
			src := tp.Image
			if tp.CatalogId != nil && *tp.CatalogId != "" {
				src = *tp.CatalogId
			}
			fmt.Fprintf(w, "Model:      %s (%s on port %d)\n", san(src), san(tp.Model), tp.Port)
		}
		fmt.Fprintf(w, "Hardware:   %s\n", hwText(epHardware(e)))
		s := epScaling(e)
		fmt.Fprintf(w, "Autoscale:  %d to %d replicas, target %d concurrent requests each, scale to zero %s\n",
			s.MinReplicas, s.MaxReplicas, s.TargetConcurrency, map[bool]string{true: "on", false: "off"}[s.ScaleToZero])
	} else if t.RentalId != nil {
		fmt.Fprintf(w, "Rental:     %s port %d\n", t.RentalId, ptrOr(t.Port, 0))
	}
	r := e.Replicas
	fmt.Fprintf(w, "Replicas:   %d ready, %d starting, %d draining, %d warm, %d wanted\n", r.Ready, r.Provisioning, r.Draining, r.Warm, r.Desired)
	fmt.Fprintf(w, "Spend:      %s this month (%s), cap %s\n", moneyStr(e.SpendThisMonth.Estimated), e.SpendThisMonth.Month, moneyPtr(e.SpendThisMonth.Cap))
	fmt.Fprintf(w, "Keys:       legion endpoints keys list %s\n", san(e.Name))
}

// ---- list / get ----

func (a *app) epListCmd() *cobra.Command {
	var typ, rental string
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List endpoints",
		Long:    "List the organization's endpoints: exposed ports, served models and autoscaled models, with access, scaling, status and URL. Filter with --type model|autoscaled|port or --rental <rental>. Use -o wide for ids and spend.",
		Example: "  legion endpoints list\n  legion endpoints list --type port -o name\n  legion endpoints list --rental quiet-river-7k2f",
		Args:    args(0, 0),
		RunE: func(cmd *cobra.Command, _ []string) error {
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
			return a.listEndpointsRun(ctx, c, typ, rid)
		},
	}
	cmd.Flags().StringVar(&typ, "type", "", "only this type: model, autoscaled or port")
	cmd.Flags().StringVar(&rental, "rental", "", "only the endpoints that expose this rental")
	return cmd
}

func (a *app) listEndpointsRun(ctx context.Context, c *api.ClientWithResponses, typ string, rental *api.RentalID) error {
	var want api.EndpointType
	switch typ {
	case "":
	case "model":
		want = api.EndpointTypeModel
	case "autoscaled", "autoscaled_model":
		want = api.EndpointTypeAutoscaledModel
	case "port":
		want = api.EndpointTypePort
	default:
		return usagef("legion endpoints list --help", "--type must be model, autoscaled or port, got %q", typ)
	}
	es, err := a.listEndpoints(ctx, c)
	if err != nil {
		return err
	}
	kept := es[:0:0]
	for _, e := range es {
		if (want == "" || e.Type == want) && (rental == nil || (e.Target.RentalId != nil && *e.Target.RentalId == *rental)) {
			kept = append(kept, e)
		}
	}
	if len(kept) == 0 {
		a.info("No endpoints yet. Create one: legion endpoints create --help")
	}
	return a.emit(epResult(kept))
}

func (a *app) epGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "get <endpoint>",
		Short:   "Show one endpoint",
		Long:    "Show an endpoint's URL, status, hardware, replicas and spend. <endpoint> is an id, a name or a unique prefix.",
		Example: "  legion endpoints get chat\n  legion endpoints get chat -o json",
		Args:    args(0, 1),
		RunE: func(cmd *cobra.Command, as []string) error {
			return a.withEndpoint(cmd, as, func(ctx context.Context, c *api.ClientWithResponses, e *api.Endpoint) error {
				r, err := c.GetEndpointWithResponse(ctx, e.Id)
				if err := a.ok(r, err); err != nil {
					return err
				}
				fresh, err := need(r.JSON200)
				if err != nil {
					return err
				}
				return a.emit(output.Result{Data: fresh, Names: []string{fresh.Name}, Human: func(w io.Writer) { epDetail(w, *fresh) }})
			})
		},
	}
}

// ---- update / pause / resume / delete ----

func (a *app) epUpdateCmd() *cobra.Command {
	var sf scaleFlags
	var hf hwFlags
	var capStr, catalog, image, access string
	var sources []string
	var revision int64
	cmd := &cobra.Command{
		Use:   "update <endpoint>",
		Short: "Change an endpoint's access, scaling, hardware, price ceiling or cap",
		Long: `Change only the settings you name; everything else stays and the URL never changes.
--access public|keys switches who may call it (keys can exist while it is public, so there is no downtime);
--source <cidr> (repeatable) limits callers to those networks in either mode.
Scaling, hardware, --catalog and --image apply to autoscaled endpoints; changes that can raise spend show the new
cost floor and ceiling and ask for confirmation (or pass --yes). Config changes roll one replica at a time with one
surge replica, which counts against the maximum and the monthly cap. --revision refuses the change when the endpoint
was edited meanwhile. Turn autoscaling on or off with 'legion endpoints autoscale'.`,
		Example: "  legion endpoints update chat --max 4\n  legion endpoints update chat --access keys\n  legion endpoints update chat --gpu H100 --gpu A100 --max-price 3 --monthly-cap 600",
		Args:    args(0, 1),
		RunE: func(cmd *cobra.Command, as []string) error {
			fl := cmd.Flags()
			scIn := sf.input(cmd)
			hwChanged := hf.changed(cmd)
			capChanged := fl.Changed("monthly-cap")
			accessChanged := access != "" || len(sources) > 0
			if access != "" && access != "public" && access != "keys" {
				return usagef("legion endpoints update --help", "--access must be public or keys, got %q", access)
			}
			if scIn == nil && !hwChanged && !capChanged && !accessChanged && catalog == "" && image == "" {
				return usagef("legion endpoints update --help", "nothing to change: pass at least one of --access, --source, --min, --max, --scale-to-zero, --target-concurrency, --gpu, --max-price, --spot, --monthly-cap, --catalog, --image")
			}
			return a.withEndpoint(cmd, as, func(ctx context.Context, c *api.ClientWithResponses, e *api.Endpoint) error {
				req := api.UpdateEndpointRequest{}
				if fl.Changed("revision") {
					req.Revision = &revision
				}
				spend := false
				if scIn != nil || hwChanged || catalog != "" || image != "" {
					tc := api.EndpointTargetChange{Scaling: scIn}
					if hwChanged {
						hw, err := hf.apply(cmd, epHardware(*e))
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
					acc := api.EndpointAccess{Mode: e.Access.Mode, Sources: e.Access.Sources}
					if access != "" {
						acc.Mode = api.EndpointAccessMode(access)
					}
					if len(sources) > 0 {
						acc.Sources = &sources
					}
					req.Access = &acc
					if acc.Mode == api.EndpointAccessPublic && e.Access.Mode != api.EndpointAccessPublic {
						if err := a.confirm(fmt.Sprintf("Make %s public? Callers no longer need a key.", e.Name), "refusing to make "+e.Name+" public without confirmation: callers would no longer need a key"); err != nil {
							return err
						}
					}
				}
				if spend && epAutoscaled(*e) {
					var hw api.EndpointHardware
					if req.Target != nil && req.Target.Hardware != nil {
						hw = *req.Target.Hardware
					} else {
						hw = epHardware(*e)
					}
					sc := overlay(epScaling(*e), scIn)
					est, err := a.epEstimate(ctx, c, epCatalog(*e), hw, sc, capM)
					if err != nil {
						return err
					}
					a.summary(e.Name, hw, sc, est, capM)
					q := fmt.Sprintf("Apply these changes to %s? They can raise what it spends.", e.Name)
					if err := a.confirm(q, "refusing to change "+e.Name+" without confirmation: the new floor and ceiling are shown above"); err != nil {
						return err
					}
				}
				r, err := c.UpdateEndpointWithResponse(ctx, e.Id, req)
				if err := a.ok(r, err); err != nil {
					return err
				}
				upd, err := need(r.JSON200)
				if err != nil {
					return err
				}
				a.info("Updated %s.", upd.Name)
				return a.emit(output.Result{Data: upd, Names: []string{upd.Name}, Human: func(w io.Writer) { epDetail(w, *upd) }})
			})
		},
	}
	sf.register(cmd)
	hf.register(cmd)
	fl := cmd.Flags()
	fl.StringVar(&access, "access", "", "public (anyone) or keys (callers need an access key)")
	fl.StringArrayVar(&sources, "source", nil, "allowed source CIDR, for example 203.0.113.0/24 (repeatable)")
	fl.StringVar(&capStr, "monthly-cap", "", "monthly spend cap in dollars (UTC calendar month)")
	fl.StringVar(&catalog, "catalog", "", "switch to this catalog model id")
	fl.StringVar(&image, "image", "", "switch to this container image")
	fl.Int64Var(&revision, "revision", 0, "refuse the change unless the endpoint is at this revision")
	return cmd
}

func (a *app) epPauseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pause <endpoint>",
		Short: "Stop serving and release the replicas",
		Long: `Pause: no replicas run and no GPU cost accrues. The URL and keys are kept and callers get 503
endpoint_paused until you resume. Warm replicas are released after the warm expiry.`,
		Example: "  legion endpoints pause chat\n  legion endpoints pause chat -o json",
		Args:    args(0, 1),
		RunE: func(cmd *cobra.Command, as []string) error {
			return a.withEndpoint(cmd, as, func(ctx context.Context, c *api.ClientWithResponses, e *api.Endpoint) error {
				r, err := c.PauseEndpointWithResponse(ctx, e.Id)
				if err := a.ok(r, err); err != nil {
					return err
				}
				got := ptrOr(r.JSON200, *e)
				a.info("Paused %s. Resume: legion endpoints resume %s", got.Name, got.Name)
				return a.emit(output.Result{Data: got, Names: []string{got.Name}, Human: func(w io.Writer) { epDetail(w, got) }})
			})
		},
	}
}

func (a *app) epResumeCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "resume <endpoint>",
		Short:   "Start serving again",
		Long:    "Resume a paused endpoint. Replicas start again and bill per GPU-hour; the cost floor and ceiling are shown and you confirm (or pass --yes).",
		Example: "  legion endpoints resume chat\n  legion endpoints resume chat --yes",
		Args:    args(0, 1),
		RunE: func(cmd *cobra.Command, as []string) error {
			return a.withEndpoint(cmd, as, func(ctx context.Context, c *api.ClientWithResponses, e *api.Endpoint) error {
				est, err := a.epEstimate(ctx, c, epCatalog(*e), epHardware(*e), epScaling(*e), e.MonthlySpendCap)
				if err != nil {
					return err
				}
				a.summary(e.Name, epHardware(*e), epScaling(*e), est, e.MonthlySpendCap)
				if err := a.confirm(fmt.Sprintf("Resume %s? Replicas start and bill per GPU-hour.", e.Name), "refusing to resume "+e.Name+" without confirmation: it starts billed replicas"); err != nil {
					return err
				}
				r, err := c.ResumeEndpointWithResponse(ctx, e.Id)
				if err := a.ok(r, err); err != nil {
					return err
				}
				got := ptrOr(r.JSON200, *e)
				a.info("Resumed %s: %s", got.Name, san(got.Url))
				return a.emit(output.Result{Data: got, Names: []string{got.Name}, Human: func(w io.Writer) { epDetail(w, got) }})
			})
		},
	}
}

func (a *app) epDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "delete <endpoint>",
		Aliases: []string{"rm"},
		Short:   "Delete an endpoint, its replicas and its keys",
		Long: `Delete the endpoint: replicas stop, every key stops working and the URL goes away. This cannot be undone.
On a terminal you must type the endpoint's name to confirm; in scripts pass --yes. The API may ask for step-up
verification, which needs a browser session.`,
		Example: "  legion endpoints delete chat\n  legion endpoints delete chat --yes",
		Args:    args(0, 1),
		RunE: func(cmd *cobra.Command, as []string) error {
			return a.withEndpoint(cmd, as, func(ctx context.Context, c *api.ClientWithResponses, e *api.Endpoint) error {
				if !a.yes {
					if !a.interactive() {
						return usagef("Add --yes to delete without a prompt.", "refusing to delete %s without confirmation: its replicas stop and all its keys are revoked for good", e.Name)
					}
					fmt.Fprintf(a.env.Stderr, "Deleting %s stops its replicas and revokes all its keys for good.\n", e.Name)
					got, err := a.prompt.Ask(fmt.Sprintf("Type the endpoint name (%s) to confirm:", e.Name))
					if errors.Is(err, context.Canceled) {
						return err
					}
					if err != nil || got != e.Name {
						return &cliError{Msg: "aborted: the name did not match", Exit: client.ExitError}
					}
				}
				r, err := c.DeleteEndpointWithResponse(ctx, e.Id)
				if err := a.ok(r, err); err != nil {
					return err
				}
				a.info("Deleting %s: replicas are stopping and billing ends.", e.Name)
				got := ptrOr(r.JSON202, *e)
				return a.emit(output.Result{Data: got, Names: []string{got.Name}, Human: func(w io.Writer) { epDetail(w, got) }})
			})
		},
	}
}

// ---- replicas / usage / events ----

func (a *app) epReplicasCmd() *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:     "replicas <endpoint>",
		Short:   "List an endpoint's replicas",
		Long:    "List the replicas (rentals) serving the endpoint, live and warm. --all adds the replicas retired in the last 7 days.",
		Example: "  legion endpoints replicas chat\n  legion endpoints replicas chat --all -o wide",
		Args:    args(0, 1),
		RunE: func(cmd *cobra.Command, as []string) error {
			return a.withEndpoint(cmd, as, func(ctx context.Context, c *api.ClientWithResponses, e *api.Endpoint) error {
				rs, err := a.listReplicas(ctx, c, e.Id, all)
				if err != nil {
					return err
				}
				return a.emit(listResult(rs, []string{"REPLICA", "PHASE", "READY", "GPU", "PRICE/HR", "TIER", "IN FLIGHT"}, []string{"RENTAL ID", "HOST", "STATE", "CREATED", "RETIRED"},
					func(r api.EndpointReplica) string { return replicaName(r) },
					func(r api.EndpointReplica) []string {
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

func replicaName(r api.EndpointReplica) string {
	if r.RentalName != nil && *r.RentalName != "" {
		return *r.RentalName
	}
	return r.RentalId.String()
}

func (a *app) listReplicas(ctx context.Context, c *api.ClientWithResponses, id api.EndpointID, retired bool) ([]api.EndpointReplica, error) {
	return collect(0, func(cur *string, n int) ([]api.EndpointReplica, *string, error) {
		p := &api.ListEndpointReplicasParams{Cursor: cur, Limit: &n}
		if retired {
			p.IncludeRetired = &retired
		}
		r, err := c.ListEndpointReplicasWithResponse(ctx, id, p)
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

func (a *app) epUsageCmd() *cobra.Command {
	var days int
	var window string
	cmd := &cobra.Command{
		Use:     "usage <endpoint>",
		Short:   "Show requests, errors, replica-hours and cost per day",
		Long:    "Show the endpoint's usage per day (requests, errors, replica-hours, cost) and this month's spend against the monthly cap. Request-rate and latency points for the --window are in -o json.",
		Example: "  legion endpoints usage chat\n  legion endpoints usage chat --days 7 -o json",
		Args:    args(0, 1),
		RunE: func(cmd *cobra.Command, as []string) error {
			return a.withEndpoint(cmd, as, func(ctx context.Context, c *api.ClientWithResponses, e *api.Endpoint) error {
				p := &api.GetEndpointUsageParams{}
				if cmd.Flags().Changed("days") {
					p.Days = &days
				}
				if window != "" {
					w := api.EndpointMetricsWindow(window)
					if w != api.EndpointMetricsWindow1h && w != api.EndpointMetricsWindow24h && w != api.EndpointMetricsWindow7d {
						return usagef("legion endpoints usage --help", "--window must be 1h, 24h or 7d, got %q", window)
					}
					p.Window = &w
				}
				r, err := c.GetEndpointUsageWithResponse(ctx, e.Id, p)
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

func (a *app) epEventsCmd() *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:     "events <endpoint>",
		Short:   "Show scaling and configuration history",
		Long:    "Show what changed and why: scaling steps, config changes and key activity, newest first.",
		Example: "  legion endpoints events chat\n  legion endpoints events chat --limit 20 -o json",
		Args:    args(0, 1),
		RunE: func(cmd *cobra.Command, as []string) error {
			return a.withEndpoint(cmd, as, func(ctx context.Context, c *api.ClientWithResponses, e *api.Endpoint) error {
				evs, err := collect(limit, func(cur *string, n int) ([]api.EndpointEvent, *string, error) {
					r, err := c.ListEndpointEventsWithResponse(ctx, e.Id, &api.ListEndpointEventsParams{Cursor: cur, Limit: &n})
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
					func(ev api.EndpointEvent) string { return fmt.Sprint(ev.Id) },
					func(ev api.EndpointEvent) []string {
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
