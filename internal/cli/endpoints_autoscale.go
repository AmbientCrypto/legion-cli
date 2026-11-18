package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/AmbientCrypto/legion-controller/api"
	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/AmbientCrypto/legion-cli/internal/output"
)

func (a *app) epAutoscaleCmd() *cobra.Command {
	cmd := group(&cobra.Command{
		Use:   "autoscale",
		Short: "Turn autoscaling on or off for an endpoint",
		Long: `Switch an endpoint between one rental (scaling off) and an autoscaled pool (scaling on). The URL and the
keys stay the same either way.`,
		Example: "  legion endpoints autoscale on chat --monthly-cap 300\n  legion endpoints autoscale off chat --keep-replica quiet-river-7k2f",
	})
	cmd.AddCommand(a.epAutoscaleOnCmd(), a.epAutoscaleOffCmd())
	return cmd
}

func (a *app) epAutoscaleOnCmd() *cobra.Command {
	var sf scaleFlags
	var hf hwFlags
	var capStr string
	var keep, replace bool
	cmd := &cobra.Command{
		Use:   "on <endpoint>",
		Short: "Turn on autoscaling for a single-rental endpoint",
		Long: `Turn a single-rental endpoint into an autoscaled one. Hardware starts as the rental's own GPU type and count
(narrow it with --gpu models, --max-price per replica-hour and --spot); scaling defaults to the platform's
(override --min, --max, --scale-to-zero, --target-concurrency); --monthly-cap is required.

--keep-rental (default) makes the current rental the first replica and moves its lifecycle to the endpoint.
--replace starts fresh replicas on other hosts and stops this rental once they are ready. You see the cost
floor and ceiling and confirm (or pass --yes).`,
		Example: "  legion endpoints autoscale on chat --monthly-cap 300 --max 3\n  legion endpoints autoscale on chat --gpu H100 --max-price 2.50 --monthly-cap 300 --replace --yes",
		Args:    args(0, 1),
		RunE: func(cmd *cobra.Command, as []string) error {
			if keep && replace {
				return usagef("legion endpoints autoscale on --help", "use --keep-rental or --replace, not both")
			}
			fl := cmd.Flags()
			if !fl.Changed("monthly-cap") && !a.interactive() {
				return usagef("legion endpoints autoscale on --help", "missing required flag --monthly-cap: autoscaling needs a monthly spend cap")
			}
			return a.withEndpoint(cmd, as, func(ctx context.Context, c *api.ClientWithResponses, e *api.Endpoint) error {
				if epAutoscaled(*e) {
					return usagef("legion endpoints get "+e.Name, "%s already autoscales", e.Name)
				}
				hw, err := hf.apply(cmd, epHardware(*e))
				if err != nil {
					return err
				}
				cat := epCatalog(*e)
				d, err := a.endpointDefaults(ctx, c, cat)
				if err != nil {
					return err
				}
				in := withScaling(sf.input(cmd), func(s *api.EndpointScalingInput) {
					if s.MinReplicas == nil {
						s.MinReplicas = ptr(d.Scaling.MinReplicas)
					}
					if s.MaxReplicas == nil {
						s.MaxReplicas = ptr(d.Scaling.MaxReplicas)
					}
				})
				sc := overlay(d.Scaling, in)
				var capM *api.Money
				if fl.Changed("monthly-cap") {
					m, err := dollars("--monthly-cap", capStr)
					if err != nil {
						return err
					}
					capM = &m
				}
				est, err := a.epEstimate(ctx, c, cat, hw, sc, capM)
				if err != nil {
					return err
				}
				if capM == nil {
					def := fmt.Sprintf("%d.%02d", est.SuggestedMonthlySpendCap.AmountMinor/100, est.SuggestedMonthlySpendCap.AmountMinor%100)
					v, _, err := a.askDefault("Monthly spend cap in dollars (required)", def)
					if err != nil {
						return err
					}
					m, err := dollars("--monthly-cap", v)
					if err != nil {
						return err
					}
					capM = &m
				}
				strategy := api.EndpointSwitchKeepRental
				if replace {
					strategy = api.EndpointSwitchReplace
				}
				a.summary(e.Name, hw, sc, est, capM)
				if replace {
					fmt.Fprintln(a.env.Stderr, "Rental:        fresh replicas start on other hosts, then the current rental is stopped (make-before-break)")
				} else {
					fmt.Fprintln(a.env.Stderr, "Rental:        keep the current rental as the first replica; its lifecycle moves to the endpoint")
				}
				q := fmt.Sprintf("Turn on autoscaling for %s? It can spend up to %s a month.", e.Name, moneyStr(*capM))
				if err := a.confirm(q, "refusing to turn on autoscaling for "+e.Name+" without confirmation: it spends money, up to the cap shown above"); err != nil {
					return err
				}
				kind := api.EndpointTargetAutoscale
				req := api.UpdateEndpointRequest{MonthlySpendCap: capM, Target: &api.EndpointTargetChange{Kind: &kind, Hardware: &hw, Scaling: in, Strategy: &strategy}}
				r, err := c.UpdateEndpointWithResponse(ctx, e.Id, req)
				if err := a.ok(r, err); err != nil {
					return err
				}
				upd, err := need(r.JSON200)
				if err != nil {
					return err
				}
				a.info("Autoscaling is on for %s. URL and keys are unchanged.", upd.Name)
				return a.emit(output.Result{Data: upd, Names: []string{upd.Name}, Human: func(w io.Writer) { epDetail(w, *upd) }})
			})
		},
	}
	hf.register(cmd)
	sf.register(cmd)
	cmd.Flags().StringVar(&capStr, "monthly-cap", "", "monthly spend cap in dollars (UTC calendar month), required")
	cmd.Flags().BoolVar(&keep, "keep-rental", false, "keep the current rental as the first replica (default)")
	cmd.Flags().BoolVar(&replace, "replace", false, "start fresh replicas and stop the current rental when they are ready")
	return cmd
}

func (a *app) epAutoscaleOffCmd() *cobra.Command {
	var keep string
	cmd := &cobra.Command{
		Use:   "off <endpoint>",
		Short: "Turn off autoscaling and keep one replica as a single rental",
		Long: `Turn an autoscaled endpoint back into one rental. You pick one ready replica to keep (--keep-replica, a rental
name or id; on a terminal you are asked); the other replicas drain and stop. The URL and keys stay the same.`,
		Example: "  legion endpoints autoscale off chat --keep-replica quiet-river-7k2f --yes\n  legion endpoints autoscale off chat",
		Args:    args(0, 1),
		RunE: func(cmd *cobra.Command, as []string) error {
			if keep == "" && !a.interactive() {
				return usagef("legion endpoints autoscale off --help", "missing required flag --keep-replica: say which replica to keep (see: legion endpoints replicas <endpoint>)")
			}
			return a.withEndpoint(cmd, as, func(ctx context.Context, c *api.ClientWithResponses, e *api.Endpoint) error {
				if !epAutoscaled(*e) {
					return usagef("legion endpoints get "+e.Name, "%s already runs as a single rental", e.Name)
				}
				var keepID uuid.UUID
				var keepName string
				if id, err := uuid.Parse(keep); err == nil {
					keepID, keepName = id, keep
				} else {
					rs, err := a.listReplicas(ctx, c, e.Id, false)
					if err != nil {
						return err
					}
					var ready []api.EndpointReplica
					for _, r := range rs {
						if r.Ready {
							ready = append(ready, r)
						}
					}
					if keep != "" {
						found := false
						for _, r := range ready {
							if replicaName(r) == keep {
								keepID, keepName, found = r.RentalId, keep, true
							}
						}
						if !found {
							return notFoundf("legion endpoints replicas "+e.Name, "no ready replica %q on %s", keep, e.Name)
						}
					} else {
						labels := make([]string, len(ready))
						for i, r := range ready {
							labels[i] = san(fmt.Sprintf("%-24s %s x%d  %s", replicaName(r), r.Host.GpuModel, r.GpuCount, moneyStr(r.PricePerGpuHour)))
						}
						i, err := a.pickOne("Which replica should stay?", labels, "missing --keep-replica", "legion endpoints replicas "+e.Name)
						if err != nil {
							return err
						}
						keepID, keepName = ready[i].RentalId, replicaName(ready[i])
					}
				}
				fmt.Fprintf(a.env.Stderr, "Keep:          %s as the single rental of %s\nThe other replicas drain and stop; the URL and keys stay the same.\n", san(keepName), san(e.Name))
				if err := a.confirm(fmt.Sprintf("Turn off autoscaling for %s?", e.Name), "refusing to turn off autoscaling for "+e.Name+" without confirmation: the other replicas stop"); err != nil {
					return err
				}
				kind := api.EndpointTargetRental
				req := api.UpdateEndpointRequest{Target: &api.EndpointTargetChange{Kind: &kind, KeepRentalId: &keepID}}
				r, err := c.UpdateEndpointWithResponse(ctx, e.Id, req)
				if err := a.ok(r, err); err != nil {
					return err
				}
				upd, err := need(r.JSON200)
				if err != nil {
					return err
				}
				a.info("Autoscaling is off for %s; %s is now its only rental.", upd.Name, keepName)
				return a.emit(output.Result{Data: upd, Names: []string{upd.Name}, Human: func(w io.Writer) { epDetail(w, *upd) }})
			})
		},
	}
	cmd.Flags().StringVar(&keep, "keep-replica", "", "rental name or id of the replica to keep")
	return cmd
}
