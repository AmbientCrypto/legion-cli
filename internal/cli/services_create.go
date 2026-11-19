package cli

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/AmbientCrypto/legion-controller/api"
	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/AmbientCrypto/legion-cli/internal/output"
)

// ---- shared flags ----

// scaleFlags are the autoscaling knobs create, update and autoscale on share.
type scaleFlags struct {
	min, max, target int
	toZero           bool
}

func (f *scaleFlags) register(cmd *cobra.Command) {
	fl := cmd.Flags()
	fl.IntVar(&f.min, "min", 0, "minimum replicas (default from the platform)")
	fl.IntVar(&f.max, "max", 0, "maximum replicas (default from the platform)")
	fl.BoolVar(&f.toZero, "scale-to-zero", false, "let the service scale down to no replicas when idle (cold requests then wait for a replica)")
	fl.IntVar(&f.target, "target-concurrency", 0, "concurrent requests per replica before another is added")
}

// input is the scaling the person named; nil when they named none.
func (f *scaleFlags) input(cmd *cobra.Command) *api.ServiceScalingInput {
	fl := cmd.Flags()
	in := api.ServiceScalingInput{}
	n := 0
	if fl.Changed("min") {
		in.MinReplicas, n = ptr(f.min), n+1
	}
	if fl.Changed("max") {
		in.MaxReplicas, n = ptr(f.max), n+1
	}
	if fl.Changed("scale-to-zero") {
		in.ScaleToZero, n = ptr(f.toZero), n+1
	}
	if fl.Changed("target-concurrency") {
		in.TargetConcurrency, n = ptr(f.target), n+1
	}
	if n == 0 {
		return nil
	}
	return &in
}

// overlay applies the named scaling over a base.
func overlay(base api.ServiceScaling, in *api.ServiceScalingInput) api.ServiceScaling {
	if in == nil {
		return base
	}
	if in.MinReplicas != nil {
		base.MinReplicas = *in.MinReplicas
	}
	if in.MaxReplicas != nil {
		base.MaxReplicas = *in.MaxReplicas
	}
	if in.ScaleToZero != nil {
		base.ScaleToZero = *in.ScaleToZero
	}
	if in.TargetConcurrency != nil {
		base.TargetConcurrency = *in.TargetConcurrency
	}
	return base
}

// hwFlags are the hardware knobs. The platform picks the cheapest hardware that
// fits the model; these only narrow the search.
type hwFlags struct {
	gpus      []string
	maxPrice  string
	spot      bool
	placement string
	minMem    float64
	counts    string
}

func (f *hwFlags) register(cmd *cobra.Command) {
	fl := cmd.Flags()
	fl.StringArrayVar(&f.gpus, "gpu", nil, "only use this GPU model, for example H100 (repeatable, unordered: the cheapest fitting one is rented). Default: any")
	fl.StringVar(&f.maxPrice, "max-price", "", "highest total price per replica-hour in dollars (all its GPUs), for example 2.50. Default: none")
	fl.BoolVar(&f.spot, "spot", false, "use spot (interruptible) capacity: cheaper, but replicas can be preempted")
	fl.StringVar(&f.placement, "placement", "", "eligible_hosts (default) or own_hosts")
	fl.Float64Var(&f.minMem, "min-gpu-memory", 0, "own image: minimum total GPU memory per replica in GB")
	fl.StringVar(&f.counts, "gpu-counts", "", "own image: GPU counts a replica may use, comma separated from 1,2,4,8 (default 1,2,4,8)")
}

func (f *hwFlags) changed(cmd *cobra.Command) bool {
	fl := cmd.Flags()
	return len(f.gpus) > 0 || fl.Changed("max-price") || fl.Changed("spot") || fl.Changed("placement") || fl.Changed("min-gpu-memory") || fl.Changed("gpu-counts")
}

// apply overlays the named hardware over base.
func (f *hwFlags) apply(cmd *cobra.Command, base api.ServiceHardware) (api.ServiceHardware, error) {
	fl := cmd.Flags()
	base.Strategy = ptr(api.ServiceHardwareCheapestFit)
	if len(f.gpus) > 0 {
		g, err := parseGPUs(f.gpus)
		if err != nil {
			return base, err
		}
		base.GpuModels = &g
	}
	if fl.Changed("max-price") {
		m, err := dollars("--max-price", f.maxPrice)
		if err != nil {
			return base, err
		}
		base.MaxPricePerReplicaHour = &m
	}
	if fl.Changed("spot") {
		t := api.RentalTierOnDemand
		if f.spot {
			t = api.RentalTierInterruptible
		}
		base.Tier = &t
	}
	if fl.Changed("min-gpu-memory") {
		if f.minMem <= 0 {
			return base, usagef("legion "+a0(cmd)+" --help", "--min-gpu-memory must be above 0 GB")
		}
		m := float32(f.minMem)
		base.MinGpuMemoryGb = &m
	}
	if fl.Changed("gpu-counts") {
		var cs []api.ServiceHardwareGpuCounts
		for _, v := range strings.Split(f.counts, ",") {
			n, err := strconv.Atoi(strings.TrimSpace(v))
			if err != nil || (n != 1 && n != 2 && n != 4 && n != 8) {
				return base, usagef("legion "+a0(cmd)+" --help", "--gpu-counts takes counts from 1,2,4,8, for example 1,2,4; got %q", v)
			}
			cs = append(cs, api.ServiceHardwareGpuCounts(n))
		}
		base.GpuCounts = &cs
	}
	if f.placement != "" {
		p := api.ServicePlacement(f.placement)
		if p != api.ServicePlacementEligibleHosts && p != api.ServicePlacementOwnHosts {
			return base, usagef("legion "+a0(cmd)+" --help", "--placement must be eligible_hosts or own_hosts, got %q", f.placement)
		}
		base.Placement = &p
	}
	return base, nil
}

func a0(cmd *cobra.Command) string { return strings.TrimPrefix(cmd.CommandPath(), "legion ") }

// parseGPUs reads the GPU model allow-list (a value may hold several, comma
// separated). A count is not part of it: the platform picks the count.
func parseGPUs(vals []string) ([]string, error) {
	var out []string
	for _, v := range vals {
		for _, s := range strings.Split(v, ",") {
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}
			if strings.Contains(s, ":") {
				return nil, usagef("legion services create --help", "--gpu %q: give a GPU model only, for example H100; the platform picks the GPU count (own images: --gpu-counts)", s)
			}
			out = append(out, s)
		}
	}
	return out, nil
}

func dollars(flagName, s string) (api.Money, error) {
	n, err := output.ParseDollars(s)
	if err != nil {
		return api.Money{}, usagef("legion "+"services --help", "%s: %v", flagName, err)
	}
	return api.Money{AmountMinor: n, Currency: "USD"}, nil
}

// ---- estimate and summary ----

func (a *app) svcEstimate(ctx context.Context, c *api.ClientWithResponses, catalog *string, hw api.ServiceHardware, sc api.ServiceScaling, cap *api.Money) (*api.ServiceEstimate, error) {
	req := api.ServiceEstimateRequest{MonthlySpendCap: cap, Target: &api.ServiceTargetInput{Mode: api.ServiceModeAutoscaled, CatalogId: catalog, Hardware: &hw, Scaling: &api.ServiceScalingInput{
		MinReplicas: &sc.MinReplicas, MaxReplicas: &sc.MaxReplicas, ScaleToZero: &sc.ScaleToZero, TargetConcurrency: &sc.TargetConcurrency}}}
	r, err := c.EstimateServiceWithResponse(ctx, req)
	if err := a.ok(r, err); err != nil {
		return nil, err
	}
	return need(r.JSON200)
}

// summary prints what the service will cost, to stderr, before a confirmation.
func (a *app) summary(name string, hw api.ServiceHardware, sc api.ServiceScaling, est *api.ServiceEstimate, cap *api.Money) {
	w := a.env.Stderr
	fmt.Fprintf(w, "Service:      %s\n", san(name))
	fmt.Fprintf(w, "Hardware:      %s\n", hwText(hw))
	z := "off"
	if sc.ScaleToZero {
		z = "on"
	}
	fmt.Fprintf(w, "Replicas:      %d to %d, up to %d concurrent requests each, scale to zero %s\n", sc.MinReplicas, sc.MaxReplicas, sc.TargetConcurrency, z)
	if sc.MinReplicas == 0 {
		fmt.Fprintf(w, "Cost floor:    %s per hour while idle (no replicas, warm storage only)\n", moneyStr(est.IdlePerHour))
	} else {
		fmt.Fprintf(w, "Cost floor:    %s per hour with the %d minimum replica(s) running\n", moneyStr(est.AtMinPerHour), sc.MinReplicas)
	}
	fmt.Fprintf(w, "Cost ceiling:  %s per hour with all %d replicas running\n", moneyStr(est.AtMaxPerHour), sc.MaxReplicas)
	if cap != nil {
		fmt.Fprintf(w, "Monthly cap:   %s (UTC calendar month; serving stops when it is reached)\n", moneyStr(*cap))
	}
	if lh := est.LikelyHardware; lh != nil {
		fmt.Fprintf(w, "Likely hardware: %dx %s at %s per hour, %d hosts available", lh.GpuCount, san(lh.GpuModel), moneyStr(lh.PricePerHour), est.HostsAvailable)
		if pr := est.PriceRangePerReplicaHour; pr != nil {
			fmt.Fprintf(w, " (a replica costs %s to %s per hour)", moneyStr(pr.Min), moneyStr(pr.Max))
		}
		fmt.Fprintln(w)
	} else if est.HostsAvailable == 0 {
		fmt.Fprintln(w, "Note:          no host fits this model and price right now")
	}
}

// ---- create ----

func (a *app) askDefault(q, def string) (answer string, given bool, err error) {
	if def != "" {
		q += " [" + def + "]"
	}
	ans, err := a.prompt.Ask(q + ":")
	if err != nil {
		return "", false, err
	}
	if ans == "" {
		return def, false, nil
	}
	return ans, true, nil
}

func (a *app) svcCreateCmd() *cobra.Command {
	var sf scaleFlags
	var hf hwFlags
	var catalog, image, kind, servedModel, capStr, keyName, keyExpires, healthPath, rental, access, replicaTerminal string
	var sources []string
	var port int
	var ack bool
	var cf configFlags
	cmd := &cobra.Command{
		Use:   "create [name]",
		Short: "Create a service: expose a rental's port, or serve a model on autoscaled GPUs",
		Long: `A service is a URL (https://<name>.ambi.run), an access policy and a target.

Expose a port of one of your rentals (no extra GPU cost):
  legion services create --rental <rental> --port 8080 --access keys
--access is public (anyone, the default) or keys (callers need a service key); --source <cidr> (repeatable) limits
callers to those networks in either mode. The name defaults to the rental's name.

Serve a model on autoscaled GPUs: --catalog (or --image with --port). Legion rents the cheapest hardware that fits
the model, GPU type and count included; replicas can differ and drift to cheaper hardware as they are replaced.
Narrow it only if you need to: --gpu <model> (repeatable, unordered allow-list), --max-price (total per
replica-hour), --spot, --placement. For your own image state the need: --min-gpu-memory <GB> and --gpu-counts
1,2,4,8. A monthly spend cap (--monthly-cap) is required. Scaling
defaults (min, max, concurrency) come from the platform. You see the cost floor, ceiling and cap and confirm (or
pass --yes). Placement is eligible provider hosts; --ack-host-visibility acknowledges that hosts can see the
workload (needed once per organization). Replicas have no web terminal unless you pass --replica-web-terminal
organization (any member who manages rentals can open it) or creator-and-owners (you and the organization's owners).

On a terminal anything missing is asked for. With keys access the first key is printed ONCE: copy it now.`,
		Example: `  legion services create --rental quiet-river-7k2f --port 8080 --access keys
  legion services create chat --catalog llama-3-8b --max-price 2.50 --monthly-cap 400
  legion services create chat --image ghcr.io/me/serve:1 --port 8000 --min-gpu-memory 24 --gpu-counts 1,2 --max-price 1 --monthly-cap 100 --spot --scale-to-zero --min 0 --yes`,
		Args: args(0, 1),
		RunE: func(cmd *cobra.Command, as []string) error {
			fl := cmd.Flags()
			name := ""
			if len(as) > 0 {
				name = as[0]
			}
			interactive := a.interactive()
			if access != "" && access != "public" && access != "keys" {
				return usagef("legion services create --help", "--access must be public or keys, got %q", access)
			}
			var wt *api.ServiceReplicaWebTerminalInput
			if fl.Changed("replica-web-terminal") {
				var err error
				if wt, err = replicaWebTerminalInput("legion services create --help", replicaTerminal); err != nil {
					return err
				}
				if rental != "" {
					return usagef("legion services create --help", "--replica-web-terminal is for the replicas of a model or image; a rental's port keeps the rental's own web terminal")
				}
			}
			if catalog != "" && image != "" {
				return usagef("legion services create --help", "use --catalog or --image, not both")
			}
			if rental != "" && (catalog != "" || image != "") {
				return usagef("legion services create --help", "--rental exposes a port of a rental; it cannot be combined with --catalog or --image")
			}
			portMode := rental != ""
			if !portMode && catalog == "" && image == "" && interactive {
				i, err := a.prompt.Select("What do you want to serve?", []string{"A port on one of my rentals", "A model or image, autoscaled on GPUs"})
				if err != nil {
					return err
				}
				portMode = i == 0
			}
			if portMode && wt != nil {
				return usagef("legion services create --help", "--replica-web-terminal is for the replicas of a model or image; a rental's port keeps the rental's own web terminal")
			}
			if portMode && cf.any() {
				return usagef("legion services create --help", "--env, --secret-env, --setting, --command and --arg configure the replicas of a model or image; they do not apply to an exposed rental port (set the rental's own with 'legion rental create' or 'legion rental env')")
			}
			if portMode {
				return a.createPortService(cmd, name, rental, port, access, sources, kind, keyName, keyExpires)
			}
			if !interactive {
				var missing []string
				if name == "" {
					missing = append(missing, "<name>")
				}
				if catalog == "" && image == "" {
					missing = append(missing, "--catalog (or --image, or --rental to expose a port)")
				}
				if !fl.Changed("monthly-cap") {
					missing = append(missing, "--monthly-cap")
				}
				if len(missing) > 0 {
					return usagef("legion services create --help", "missing required: %s (run on a terminal to be asked)", strings.Join(missing, ", "))
				}
			}
			if image != "" && port == 0 && !interactive {
				return usagef("legion services create --help", "--image needs --port, the port the service listens on")
			}
			cfg, err := a.resolveConfig(cmd, &cf, "services create")
			if err != nil {
				return err
			}
			c, err := a.api()
			if err != nil {
				return err
			}
			ctx := a.ctx(cmd)
			if name == "" {
				if name, _, err = a.askDefault("Service name (lowercase letters, digits, dashes)", ""); err != nil {
					return err
				}
			}
			if catalog == "" && image == "" {
				if catalog, _, err = a.askDefault("Catalog model id (blank to use your own image)", ""); err != nil {
					return err
				}
				if catalog == "" {
					if image, _, err = a.askDefault("Container image", ""); err != nil {
						return err
					}
					p, _, err := a.askDefault("Port the service listens on", "8000")
					if err != nil {
						return err
					}
					if port, err = strconv.Atoi(p); err != nil {
						return usagef("legion services create --help", "%q is not a port", p)
					}
					if kind, _, err = a.askDefault("Protocol (openai or http)", orDefault(kind, "openai")); err != nil {
						return err
					}
					if !fl.Changed("min-gpu-memory") {
						v, given, err := a.askDefault("Minimum total GPU memory per replica in GB (blank: let the platform decide)", "")
						if err != nil {
							return err
						}
						if given {
							_ = fl.Set("min-gpu-memory", v)
						}
					}
					if !fl.Changed("gpu-counts") {
						v, given, err := a.askDefault("GPU counts a replica may use (1, 2, 4, 8)", "1,2,4,8")
						if err != nil {
							return err
						}
						if given {
							_ = fl.Set("gpu-counts", v)
						}
					}
				}
			}
			if name == "" || (catalog == "" && image == "") {
				return usagef("legion services create --help", "a service needs a name and a --catalog model or an --image")
			}
			var cat *string
			if catalog != "" {
				cat = &catalog
			}
			d, err := a.serviceDefaults(ctx, c, cat)
			if err != nil {
				return err
			}
			if len(hf.gpus) == 0 && interactive {
				v, _, err := a.askDefault("Only these GPU models, comma separated, for example H100,A100 (blank: any, the cheapest that fits is rented)", "")
				if err != nil {
					return err
				}
				if v != "" {
					hf.gpus = []string{v}
				}
			}
			if !fl.Changed("max-price") && interactive {
				v, _, err := a.askDefault("Highest total price per replica-hour in dollars (blank: no limit)", "")
				if err != nil {
					return err
				}
				if v != "" {
					hf.maxPrice = v
					_ = fl.Set("max-price", v)
				}
			}
			if !fl.Changed("spot") && interactive {
				ok, err := a.prompt.Confirm("Use spot (interruptible) capacity? Cheaper, but replicas can be preempted.")
				if err != nil {
					return err
				}
				if ok {
					_ = fl.Set("spot", "true")
				}
			}
			scIn := sf.input(cmd)
			if interactive && !fl.Changed("min") {
				v, given, err := a.askDefault("Minimum replicas", strconv.Itoa(d.Scaling.MinReplicas))
				if err != nil {
					return err
				}
				if given {
					n, err := strconv.Atoi(v)
					if err != nil {
						return usagef("legion services create --help", "%q is not a number", v)
					}
					scIn = withScaling(scIn, func(s *api.ServiceScalingInput) { s.MinReplicas = &n })
				}
			}
			if interactive && !fl.Changed("max") {
				v, given, err := a.askDefault("Maximum replicas", strconv.Itoa(d.Scaling.MaxReplicas))
				if err != nil {
					return err
				}
				if given {
					n, err := strconv.Atoi(v)
					if err != nil {
						return usagef("legion services create --help", "%q is not a number", v)
					}
					scIn = withScaling(scIn, func(s *api.ServiceScalingInput) { s.MaxReplicas = &n })
				}
			}
			hw, err := hf.apply(cmd, api.ServiceHardware{})
			if err != nil {
				return err
			}
			sc := overlay(d.Scaling, scIn)
			var capM *api.Money
			if fl.Changed("monthly-cap") {
				m, err := dollars("--monthly-cap", capStr)
				if err != nil {
					return err
				}
				capM = &m
			}
			est, err := a.svcEstimate(ctx, c, cat, hw, sc, capM)
			if err != nil {
				return err
			}
			if capM == nil {
				def := fmt.Sprintf("%d.%02d", est.SuggestedMonthlySpendCap.AmountMinor/100, est.SuggestedMonthlySpendCap.AmountMinor%100)
				v, _, err := a.askDefault("Monthly spend cap in dollars (required; serving stops when reached)", def)
				if err != nil {
					return err
				}
				m, err := dollars("--monthly-cap", v)
				if err != nil {
					return err
				}
				capM = &m
			}
			if err := checkServiceConfig(cfg, catalog != ""); err != nil {
				return err
			}
			a.summary(name, hw, sc, est, capM)
			if !cfg.empty() && !a.quiet {
				cfg.write(a.env.Stderr)
				if len(cfg.Env) > 0 {
					fmt.Fprintln(a.env.Stderr, "  Note:      anything in the container, including these variables, can be read by the provider's machine.")
				}
			}
			ex, never, err := parseExpiry(keyExpires)
			if err != nil {
				return err
			}
			if never && access != "public" {
				a.info("Warning: the first key never expires. Prefer an expiry unless you rotate it yourself.")
			}
			q := fmt.Sprintf("Create service %s? It spends up to the cap of %s a month.", name, moneyStr(*capM))
			if err := a.confirm(q, "refusing to create "+name+" without confirmation: it spends money, up to the monthly cap shown above"); err != nil {
				return err
			}
			tgt := api.ServiceTargetInput{Mode: api.ServiceModeAutoscaled, CatalogId: cat, Hardware: &hw, Scaling: scIn}
			if image != "" {
				tgt.Image = &image
				tgt.Serve = &api.ServiceServeInput{Port: port}
				if servedModel != "" {
					tgt.Serve.Model = &servedModel
				}
			}
			if fl.Changed("health-path") {
				tgt.HealthPath = &healthPath
			}
			tgt.Env, tgt.ModelSettings, tgt.Command, tgt.Args = cfg.envInputs(), cfg.modelSettings(), cfg.commandOverride(), cfg.argsOverride()
			req := api.CreateServiceRequest{Name: &name, Target: tgt, MonthlySpendCap: capM, ReplicaWebTerminal: wt}
			if kind != "" {
				req.Protocol = ptr(api.ServiceProtocol(kind))
			}
			if access != "" || len(sources) > 0 {
				req.Access = accessInput(access, sources)
			}
			if access != "public" {
				req.InitialKey = &api.CreateServiceKeyRequest{Name: keyName, ExpiresInDays: ex}
				if never {
					req.InitialKey.NoExpiry = ptr(true)
				}
			}
			if ack || (interactive && !a.yes) {
				req.AcknowledgeHostVisibility = ptr(true)
			}
			return a.postService(ctx, c, req)
		},
	}
	hf.register(cmd)
	sf.register(cmd)
	fl := cmd.Flags()
	fl.StringVar(&rental, "rental", "", "expose a port of this rental (name, id or prefix) instead of serving a model")
	fl.StringVar(&access, "access", "", "public (anyone; default for a rental port) or keys (callers need a service key; default for a model)")
	fl.StringVar(&replicaTerminal, "replica-web-terminal", "", "web terminal on the replicas: off (default), organization or creator-and-owners")
	fl.StringArrayVar(&sources, "source", nil, "allowed source CIDR, for example 203.0.113.0/24 (repeatable; either access mode)")
	fl.StringVar(&catalog, "catalog", "", "catalog model id to serve (see: legion catalog list)")
	fl.StringVar(&catalog, "model", "", "alias of --catalog")
	fl.StringVar(&image, "image", "", "your own container image instead of a catalog model (when your organization allows custom images)")
	fl.IntVar(&port, "port", 0, "with --rental: the port to expose; with --image: the port the service listens on")
	fl.StringVar(&kind, "kind", "", "protocol: openai (default for a model) or http")
	fl.StringVar(&servedModel, "served-model", "", "with --image: the model name the service reports")
	fl.StringVar(&healthPath, "health-path", "", "path polled to see a replica is ready (default /v1/models)")
	fl.StringVar(&capStr, "monthly-cap", "", "monthly spend cap in dollars (UTC calendar month), required for a model")
	fl.StringVar(&keyName, "key-name", "default", "name of the first service key")
	fl.StringVar(&keyExpires, "key-expires", "", "first key expiry: 7d, 30d, 90d, 365d or never (default: the platform's, 90d)")
	cf.register(cmd, "your own --image")
	fl.BoolVar(&ack, "ack-host-visibility", false, "acknowledge that provider hosts can see the workload (once per organization)")
	return cmd
}

// checkServiceConfig refuses what the platform would refuse: settings are for
// catalog models, command and args for your own image.
func checkServiceConfig(cfg *containerConfig, catalog bool) error {
	if cfg == nil {
		return nil
	}
	if len(cfg.Settings) > 0 && !catalog {
		return usagef("legion catalog list", "--setting is for a catalog model: use --catalog <model>")
	}
	if (cfg.hasCommand || cfg.hasArgs) && catalog {
		return usagef("Serve your own image with --image to set --command or --arg.", "--command and --arg are only for your own images, not catalog entries")
	}
	return nil
}

func accessInput(mode string, sources []string) *api.ServiceAccess {
	acc := api.ServiceAccess{Mode: api.ServiceAccessMode(orDefault(mode, "public"))}
	if len(sources) > 0 {
		acc.Sources = &sources
	}
	return &acc
}

// createPortService exposes a port of a rental as a service.
func (a *app) createPortService(cmd *cobra.Command, name, rentalArg string, port int, access string, sources []string, kind, keyName, keyExpires string) error {
	c, err := a.api()
	if err != nil {
		return err
	}
	ctx := a.ctx(cmd)
	var as []string
	if rentalArg != "" {
		as = []string{rentalArg}
	}
	r, err := a.pickRental(ctx, c, as)
	if err != nil {
		return err
	}
	if port == 0 {
		if !a.interactive() {
			return usagef("legion services create --help", "missing required flag --port")
		}
		p, _, err := a.askDefault("Port to expose", "")
		if err != nil {
			return err
		}
		if port, err = strconv.Atoi(p); err != nil {
			return usagef("legion services create --help", "%q is not a port", p)
		}
	}
	if access == "" && a.interactive() && !a.yes {
		if access, _, err = a.askDefault("Access: public (anyone) or keys (callers need a service key)", "keys"); err != nil {
			return err
		}
		if access != "public" && access != "keys" {
			return usagef("legion services create --help", "access must be public or keys, got %q", access)
		}
	}
	ex, never, err := parseExpiry(keyExpires)
	if err != nil {
		return err
	}
	who := "anyone on the internet"
	if len(sources) > 0 {
		who = "only " + strings.Join(sources, ", ")
	}
	if access == "keys" {
		who += " who have a service key"
	}
	msg := fmt.Sprintf("port %d of %s will be reachable on the internet by %s", port, r.Name, who)
	if err := a.confirm("Expose it? "+msg+".", "refusing to expose a port without confirmation: "+msg); err != nil {
		return err
	}
	req := api.CreateServiceRequest{Target: api.ServiceTargetInput{Mode: api.ServiceModeRental, RentalId: &r.Id, Port: &port}}
	if name != "" {
		req.Name = &name
	}
	if kind != "" {
		req.Protocol = ptr(api.ServiceProtocol(kind))
	}
	if access != "" || len(sources) > 0 {
		req.Access = accessInput(access, sources)
	}
	if access == "keys" {
		req.InitialKey = &api.CreateServiceKeyRequest{Name: keyName, ExpiresInDays: ex}
		if never {
			req.InitialKey.NoExpiry = ptr(true)
			a.info("Warning: the first key never expires.")
		}
	}
	return a.postService(ctx, c, req)
}

func (a *app) postService(ctx context.Context, c *api.ClientWithResponses, req api.CreateServiceRequest) error {
	r, err := c.CreateServiceWithResponse(ctx, &api.CreateServiceParams{IdempotencyKey: uuid.NewString()}, req)
	if err := a.ok(r, err); err != nil {
		return err
	}
	ec, err := need(r.JSON201)
	if err != nil {
		return err
	}
	e := ec.Service
	a.info("Created service %s at %s", san(e.Name), san(e.Url))
	// A public service makes no key: initial_key is absent (LGN-639).
	secret := ""
	if ec.InitialKey != nil {
		secret = ec.InitialKey.Secret
	}
	if secret != "" {
		a.info("Warning: this is the only time the key secret is shown. Copy it now.")
		a.info("Call it: curl %s -H 'Authorization: Bearer <key>'", san(e.Url))
	} else {
		a.info("It is public: anyone can call %s. Protect it with: legion services update %s --access keys", san(e.Url), san(e.Name))
	}
	a.info("Watch it: legion services get %s", san(e.Name))
	return a.emit(output.Result{Data: ec, Names: []string{e.Name}, Human: func(w io.Writer) {
		fmt.Fprintf(w, "Name:    %s\nURL:     %s\nAccess:  %s\n", san(e.Name), san(e.Url), accessLabel(e.Access))
		if secret != "" {
			fmt.Fprintf(w, "Key:     %s (%s)\nSecret:  %s\n", san(ec.InitialKey.Key.Name), san(ec.InitialKey.Key.Prefix), secret)
		}
	}})
}

func withScaling(in *api.ServiceScalingInput, f func(*api.ServiceScalingInput)) *api.ServiceScalingInput {
	if in == nil {
		in = &api.ServiceScalingInput{}
	}
	f(in)
	return in
}

func (a *app) serviceDefaults(ctx context.Context, c *api.ClientWithResponses, catalog *string) (*api.ServiceDefaults, error) {
	r, err := c.GetServiceDefaultsWithResponse(ctx, &api.GetServiceDefaultsParams{CatalogId: catalog})
	if err := a.ok(r, err); err != nil {
		return nil, err
	}
	return need(r.JSON200)
}

// parseExpiry reads 7d, 30d, 90d, 365d or never. Empty means the platform default.
func parseExpiry(s string) (days *api.CreateServiceKeyRequestExpiresInDays, never bool, err error) {
	switch s {
	case "":
		return nil, false, nil
	case "never":
		return nil, true, nil
	case "7d", "30d", "90d", "365d":
		n, _ := strconv.Atoi(strings.TrimSuffix(s, "d"))
		d := api.CreateServiceKeyRequestExpiresInDays(n)
		return &d, false, nil
	}
	return nil, false, usagef("legion services keys create --help", "expiry must be 7d, 30d, 90d, 365d or never, got %q", s)
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
