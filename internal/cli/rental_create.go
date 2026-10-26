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
	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/AmbientCrypto/legion-cli/internal/client"
	"github.com/AmbientCrypto/legion-cli/internal/output"
)

// Preview is what a rental will cost, built client-side from getOffer and
// getBillingAccount before anything is spent (the same fields the MCP
// confirm flow shows).
type Preview struct {
	HostID         string          `json:"host_id"`
	HostName       string          `json:"host_name"`
	GPUModel       string          `json:"gpu_model"`
	GPUCount       int             `json:"gpu_count"`
	GPUMemoryMiB   int64           `json:"gpu_memory_mib"`
	PricePerGPUHr  api.Money       `json:"price_per_gpu_hour"`
	HourlyPrice    api.Money       `json:"hourly_price"`
	MaxPricePerGPU api.Money       `json:"max_price_per_gpu_hour"`
	StoragePerGiB  *api.Money      `json:"storage_price_per_gib_month,omitempty"`
	Balance        api.SignedMoney `json:"balance"`
	HasCredit      bool            `json:"has_credit"`
	Image          string          `json:"image"`
	CatalogID      string          `json:"catalog_id,omitempty"`
	SSHKey         string          `json:"ssh_key"`
	SSHFingerprint string          `json:"ssh_key_fingerprint,omitempty"`
}

func (p Preview) write(w io.Writer) {
	cur := string(p.PricePerGPUHr.Currency)
	fmt.Fprintf(w, "Rental preview\n")
	fmt.Fprintf(w, "  Host:      %s (%s)\n", p.HostName, p.HostID)
	fmt.Fprintf(w, "  GPUs:      %d x %s, %d GiB each\n", p.GPUCount, p.GPUModel, p.GPUMemoryMiB/1024)
	fmt.Fprintf(w, "  Price:     %s per GPU-hour = %s per hour", output.Money(p.PricePerGPUHr.AmountMinor, cur), output.Money(p.HourlyPrice.AmountMinor, cur))
	if p.MaxPricePerGPU.AmountMinor != p.PricePerGPUHr.AmountMinor {
		fmt.Fprintf(w, " (you accept up to %s per GPU-hour)", output.Money(p.MaxPricePerGPU.AmountMinor, cur))
	}
	fmt.Fprintln(w)
	if p.StoragePerGiB != nil {
		fmt.Fprintf(w, "  Storage:   %s per GiB-month while stopped\n", output.Money(p.StoragePerGiB.AmountMinor, cur))
	}
	fmt.Fprintf(w, "  Balance:   %s", signed(p.Balance))
	switch {
	case !p.HasCredit:
		fmt.Fprint(w, " (no credit: the rental will be refused)")
	case p.HourlyPrice.AmountMinor > 0:
		fmt.Fprintf(w, " (about %s at this rate)", output.Duration(p.Balance.AmountMinor*3600/p.HourlyPrice.AmountMinor))
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  Image:     %s\n", p.Image)
	fmt.Fprintf(w, "  SSH key:   %s", p.SSHKey)
	if p.SSHFingerprint != "" {
		fmt.Fprintf(w, " (%s)", p.SSHFingerprint)
	}
	fmt.Fprintln(w)
}

func (a *app) rentalCreateCmd() *cobra.Command {
	var (
		offerArg, image, catalogID, sshKeyArg, maxPrice string
		gpus                                            int
		async, dryRun                                   bool
		wait                                            = true
		waitTimeout                                     time.Duration
	)
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Rent GPUs and start a container",
		Long: `Rent GPUs on one host and start a container on them. legion first shows a preview - host,
GPUs, price per GPU-hour and per hour, your balance, image and SSH key - and asks you to confirm.
The price you saw is sent as the most you will pay: if the host's price rises first, nothing is
rented and you are told the new price. Billing is per second while the rental runs.

By default legion waits until the rental is running; --async returns at once.
Missing flags are asked for on a terminal. In scripts pass --offer, an image (--image or
--catalog) and --yes. --dry-run prints the preview and exits without renting.`,
		Example: `  legion rental create --offer amber-falcon --catalog pytorch --gpus 2
  legion rental create --offer 5f0c1c2e-... --image pytorch/pytorch:2.4.0-cuda12.4-cudnn9-runtime --max-price 2.60
  legion rental create --offer amber-falcon --catalog qwen3-5-4b --dry-run`,
		Args: args(0, 0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if async {
				wait = false
			}
			if image != "" && catalogID != "" {
				return usagef("legion rental create --help", "use either --image or --catalog, not both")
			}
			c, err := a.api()
			if err != nil {
				return err
			}
			ctx := a.ctx(cmd)
			var maxCents int64
			if maxPrice != "" {
				if maxCents, err = output.ParseDollars(maxPrice); err != nil {
					return usagef("legion rental create --help", "--max-price: %v", err)
				}
			}

			offer, err := a.chooseOffer(ctx, c, offerArg, catalogID)
			if err != nil {
				return err
			}
			req := api.CreateRentalRequest{HostId: offer.HostId, GpuCount: gpus}
			pv := Preview{HostID: offer.HostId.String(), HostName: offer.HostName, GPUModel: offer.GpuModel, GPUMemoryMiB: offer.GpuMemoryMib,
				PricePerGPUHr: offer.PricePerGpuHour, StoragePerGiB: offer.StoragePricePerGibMonth}

			if err := a.chooseImage(ctx, c, offer, image, catalogID, &req, &pv); err != nil {
				return err
			}
			if req.GpuCount == 0 {
				req.GpuCount = 1
			}
			if req.GpuCount > offer.GpusFree {
				return usagef("legion offer get "+offer.HostId.String(), "%s has only %d GPUs free, you asked for %d", offer.HostName, offer.GpusFree, req.GpuCount)
			}
			key, err := a.chooseSSHKey(ctx, c, sshKeyArg)
			if err != nil {
				return err
			}
			req.SshPublicKey, pv.SSHKey, pv.SSHFingerprint = key.pub, key.label, key.fingerprint

			bal, err := a.balance(ctx, c)
			if err != nil {
				return err
			}
			max := offer.PricePerGpuHour
			if maxCents > 0 {
				max = api.Money{AmountMinor: maxCents, Currency: offer.PricePerGpuHour.Currency}
			}
			req.MaxPricePerGpuHour = &max
			pv.GPUCount, pv.Image = req.GpuCount, req.Image
			pv.HourlyPrice = api.Money{AmountMinor: offer.PricePerGpuHour.AmountMinor * int64(req.GpuCount), Currency: offer.PricePerGpuHour.Currency}
			pv.MaxPricePerGPU, pv.Balance, pv.HasCredit = max, bal.Balance, bal.HasCredit

			if dryRun {
				return a.emit(output.Result{Data: pv, Names: []string{pv.HostName}, Human: pv.write})
			}
			if !a.quiet {
				pv.write(a.env.Stderr)
			}
			if err := a.confirm("Rent these GPUs and start billing?", "refusing to start billing without confirmation: this rental will cost "+output.Money(pv.HourlyPrice.AmountMinor, string(pv.HourlyPrice.Currency))+" per hour"); err != nil {
				return err
			}

			resp, err := c.CreateRentalWithResponse(ctx, &api.CreateRentalParams{IdempotencyKey: uuid.NewString()}, req)
			if err != nil {
				return a.netErr(err)
			}
			if err := client.Check(resp.HTTPResponse, resp.Body); err != nil {
				return err
			}
			r := resp.JSON201
			a.info("Created rental %s (%s).", r.Name, r.Id)
			if !wait {
				a.info("It is starting. Wait for it: legion rental wait %s", r.Name)
				a.info("Connect when running: legion ssh %s", r.Name)
				a.info("Stop billing: legion rental stop %s", r.Name)
				return a.emit(one(*r))
			}
			final, err := a.waitFor(ctx, c, r.Id, "running", waitTimeout, true)
			if err != nil {
				return err
			}
			a.info("Running.")
			a.info("Connect: legion ssh %s", final.Name)
			if final.Serve != nil {
				a.info("Model endpoint and key: legion rental serve %s", final.Name)
			}
			a.info("Stop billing: legion rental stop %s", final.Name)
			return a.emit(one(*final))
		},
	}
	f := cmd.Flags()
	f.StringVar(&offerArg, "offer", "", "host to rent: id or name from 'legion offer list'")
	f.StringVar(&offerArg, "host", "", "alias of --offer")
	f.IntVar(&gpus, "gpus", 0, "number of GPUs (default 1, or what the catalog entry needs)")
	f.StringVar(&image, "image", "", "container image, for example pytorch/pytorch:2.4.0-cuda12.4-cudnn9-runtime")
	f.StringVar(&catalogID, "catalog", "", "starter image or premade model id from 'legion catalog list'")
	f.StringVar(&sshKeyArg, "ssh-key", "", "saved SSH key (id, label or fingerprint) or a path to a .pub file")
	f.StringVar(&maxPrice, "max-price", "", "most you will pay per GPU-hour in dollars, for example 2.60 (default: the price shown)")
	f.BoolVar(&wait, "wait", true, "wait until the rental is running")
	f.BoolVar(&async, "async", false, "return as soon as the rental is created (same as --wait=false)")
	f.DurationVar(&waitTimeout, "wait-timeout", 10*time.Minute, "how long to wait for the rental to run")
	f.BoolVar(&dryRun, "dry-run", false, "print the preview and exit without renting")
	_ = cmd.Flags().MarkHidden("host")
	return cmd
}

func (a *app) balance(ctx context.Context, c *api.ClientWithResponses) (*api.BillingAccount, error) {
	r, err := c.GetBillingAccountWithResponse(ctx)
	if err != nil {
		return nil, a.netErr(err)
	}
	if err := client.Check(r.HTTPResponse, r.Body); err != nil {
		return nil, err
	}
	return r.JSON200, nil
}

// chooseOffer resolves --offer (id or host name), or picks one on a terminal.
func (a *app) chooseOffer(ctx context.Context, c *api.ClientWithResponses, arg, catalogID string) (*api.Offer, error) {
	if arg != "" {
		return a.findOffer(ctx, c, arg)
	}
	if !a.interactive() {
		return nil, usagef("Find one with: legion offer list", "missing --offer: which host to rent")
	}
	params := &api.ListOffersParams{Sort: ptr(api.OfferSortPrice)}
	if catalogID != "" {
		params.CatalogId = &catalogID
	}
	offers, err := a.listOffers(ctx, c, params, 30)
	if err != nil {
		return nil, err
	}
	labels := make([]string, len(offers))
	for i, o := range offers {
		labels[i] = fmt.Sprintf("%-18s %dx %-18s %d free  %s/GPU-hr", o.HostName, o.GpuCount, o.GpuModel, o.GpusFree, output.Money(o.PricePerGpuHour.AmountMinor, string(o.PricePerGpuHour.Currency)))
	}
	i, err := a.pickOne("Which host?", labels, "", "")
	if err != nil {
		return nil, err
	}
	return a.getOffer(ctx, c, offers[i].HostId)
}

// findOffer reads one offer by host id, host name or unique name prefix.
func (a *app) findOffer(ctx context.Context, c *api.ClientWithResponses, arg string) (*api.Offer, error) {
	if id, err := uuid.Parse(arg); err == nil {
		return a.getOffer(ctx, c, id)
	}
	offers, err := a.listOffers(ctx, c, &api.ListOffersParams{}, 500)
	if err != nil {
		return nil, err
	}
	var hits []api.Offer
	for _, o := range offers {
		if o.HostName == arg {
			return a.getOffer(ctx, c, o.HostId)
		}
		if strings.HasPrefix(o.HostName, arg) {
			hits = append(hits, o)
		}
	}
	if len(hits) != 1 {
		return nil, notFoundf("legion offer list", "no single offer matches host %q", arg)
	}
	return a.getOffer(ctx, c, hits[0].HostId)
}

func (a *app) getOffer(ctx context.Context, c *api.ClientWithResponses, id uuid.UUID) (*api.Offer, error) {
	r, err := c.GetOfferWithResponse(ctx, id)
	if err != nil {
		return nil, a.netErr(err)
	}
	if err := client.Check(r.HTTPResponse, r.Body); err != nil {
		return nil, err
	}
	return &r.JSON200.Offer, nil
}

// chooseImage fills req.Image (and serve for a premade model) from --image,
// --catalog or a picker.
func (a *app) chooseImage(ctx context.Context, c *api.ClientWithResponses, offer *api.Offer, image, catalogID string, req *api.CreateRentalRequest, pv *Preview) error {
	if image != "" {
		req.Image = image
		return nil
	}
	if catalogID == "" && !a.interactive() {
		return usagef("See: legion catalog list", "missing --image or --catalog: what to run")
	}
	entries, err := a.catalogFor(ctx, c, &offer.HostId)
	if err != nil {
		return err
	}
	var e *api.CatalogEntry
	if catalogID != "" {
		for i := range entries {
			if entries[i].Id == catalogID {
				e = &entries[i]
			}
		}
		if e == nil {
			return notFoundf("legion catalog list", "no catalog entry %q", catalogID)
		}
	} else {
		var labels []string
		var ok []int
		for i, en := range entries {
			if en.Compatible != nil && !*en.Compatible {
				continue
			}
			ok = append(ok, i)
			labels = append(labels, fmt.Sprintf("%-22s %-6s %s", en.Id, en.Kind, en.Name))
		}
		i, err := a.pickOne("Which image?", labels, "", "")
		if err != nil {
			return err
		}
		e = &entries[ok[i]]
	}
	if e.Compatible != nil && !*e.Compatible {
		var why []string
		if e.Incompatibilities != nil {
			for _, inc := range *e.Incompatibilities {
				why = append(why, inc.Detail)
			}
		}
		return errorf("legion offer list --catalog "+e.Id, "%s cannot run on %s: %s", e.Id, offer.HostName, strings.Join(why, "; "))
	}
	req.Image, pv.CatalogID = e.Reference, e.Id
	if req.GpuCount == 0 && e.MinGpus != nil {
		req.GpuCount = *e.MinGpus
	}
	if e.MinGpus != nil && req.GpuCount < *e.MinGpus {
		return usagef("legion rental create --gpus "+fmt.Sprint(*e.MinGpus), "%s needs at least %d GPUs on this host", e.Id, *e.MinGpus)
	}
	if e.Kind == "model" && e.Serve != nil {
		req.Serve = &api.RentalServeRequest{Port: e.Serve.Port, Kind: api.RentalServeKind(e.Serve.Protocol), Model: e.Serve.Model, CatalogId: &e.Id}
	}
	return nil
}

func (a *app) catalogFor(ctx context.Context, c *api.ClientWithResponses, host *uuid.UUID) ([]api.CatalogEntry, error) {
	r, err := c.ListCatalogWithResponse(ctx, &api.ListCatalogParams{HostId: host})
	if err != nil {
		return nil, a.netErr(err)
	}
	if err := client.Check(r.HTTPResponse, r.Body); err != nil {
		return nil, err
	}
	return r.JSON200.Entries, nil
}

type chosenKey struct{ pub, label, fingerprint string }

// chooseSSHKey picks the key the rental authorizes: --ssh-key (saved key or
// .pub file), else the saved keys, else ~/.ssh/id_ed25519.pub.
func (a *app) chooseSSHKey(ctx context.Context, c *api.ClientWithResponses, arg string) (chosenKey, error) {
	r, err := c.ListSshKeysWithResponse(ctx)
	if err != nil {
		return chosenKey{}, a.netErr(err)
	}
	if err := client.Check(r.HTTPResponse, r.Body); err != nil {
		return chosenKey{}, err
	}
	saved := r.JSON200.Items
	from := func(k api.SshKey) chosenKey { return chosenKey{k.PublicKey, k.Label, k.Fingerprint} }
	if arg != "" {
		for _, k := range saved {
			if k.Id.String() == arg || k.Label == arg || k.Fingerprint == arg {
				return from(k), nil
			}
		}
		if b, err := os.ReadFile(expandHome(arg, a.env.Home)); err == nil {
			return chosenKey{pub: strings.TrimSpace(string(b)), label: arg}, nil
		}
		return chosenKey{}, notFoundf("legion ssh-key list", "no saved SSH key or .pub file matches %q", arg)
	}
	switch len(saved) {
	case 1:
		return from(saved[0]), nil
	case 0:
		p := filepath.Join(a.env.Home, ".ssh", "id_ed25519.pub")
		b, err := os.ReadFile(p)
		if err != nil {
			return chosenKey{}, usagef("ssh-keygen -t ed25519, then: legion ssh-key add", "no SSH key: none saved and %s does not exist", "~/.ssh/id_ed25519.pub")
		}
		a.info("Using ~/.ssh/id_ed25519.pub. Save it for next time with: legion ssh-key add")
		return chosenKey{pub: strings.TrimSpace(string(b)), label: "~/.ssh/id_ed25519.pub"}, nil
	}
	labels := make([]string, len(saved))
	for i, k := range saved {
		labels[i] = fmt.Sprintf("%-20s %s", k.Label, k.Fingerprint)
	}
	i, err := a.pickOne("Which SSH key?", labels, "several SSH keys are saved: choose one with --ssh-key <label>", "legion ssh-key list")
	if err != nil {
		return chosenKey{}, err
	}
	return from(saved[i]), nil
}

func expandHome(p, home string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		return filepath.Join(home, strings.TrimPrefix(p, "~"))
	}
	return p
}
