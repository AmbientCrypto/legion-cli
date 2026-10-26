package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/AmbientCrypto/legion-controller/api"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/AmbientCrypto/legion-cli/internal/output"
)

func (a *app) offerCmd() *cobra.Command {
	cmd := group(&cobra.Command{
		Use:     "offer",
		Aliases: []string{"offers"},
		Short:   "Browse GPU offers from hosts",
		Long:    "An offer is a host with free GPUs at a price per GPU-hour. Find one, then rent it with: legion rental create --offer <host>.",
		Example: `  legion offer list --gpu-model "NVIDIA GeForce RTX 4090" --max-price 2
  legion offer get amber-falcon`,
	})
	var (
		models, countries               []string
		minGPUs, maxLatency, limit      int
		maxPrice, region, catalog, sort string
	)
	list := &cobra.Command{
		Use:   "list",
		Short: "List offers, cheapest first",
		Long: `List hosts with free GPUs, filtered and sorted. Prices are per GPU-hour. The default sort is price
(cheapest first); others: score, reliability, gpu_memory, price_per_tflops. Use -o wide for host ids and
hardware, -o name for host names. --catalog shows only hosts that can run that starter image or model.`,
		Example: `  legion offer list
  legion offer list --gpu-model "NVIDIA GeForce RTX 4090" --min-gpus 2 --max-price 2.50
  legion offer list --catalog qwen3-5-4b --sort reliability -o wide`,
		Args: args(0, 0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := a.api()
			if err != nil {
				return err
			}
			// The API only matches a GPU model exactly, so --gpu-model is matched
			// here, as a case-insensitive substring, over every offer.
			p := &api.ListOffersParams{}
			if minGPUs > 0 {
				p.MinGpus = &minGPUs
			}
			if maxPrice != "" {
				cents, err := output.ParseDollars(maxPrice)
				if err != nil {
					return usagef("legion offer list --help", "--max-price: %v", err)
				}
				p.MaxPricePerGpuHourCents = &cents
			}
			if region != "" {
				p.LatencyRegion = ptr(api.LatencyRegion(region))
			}
			if maxLatency > 0 {
				p.MaxLatencyMs = &maxLatency
			}
			if len(countries) > 0 {
				p.Countries = &countries
			}
			if catalog != "" {
				p.CatalogId = &catalog
			}
			if sort != "" {
				p.Sort = ptr(api.OfferSort(sort))
			}
			fetch := limit
			if len(models) > 0 {
				fetch = 1000
			}
			offers, err := a.listOffers(a.ctx(cmd), c, p, fetch)
			if err != nil {
				return err
			}
			if len(models) > 0 {
				offers = filterModels(offers, models)
			}
			if limit > 0 && len(offers) > limit {
				offers = offers[:limit]
			}
			if len(offers) == 0 {
				var applied []string
				cmd.Flags().Visit(func(f *pflag.Flag) {
					if f.Name != "limit" {
						applied = append(applied, "--"+f.Name+" "+strings.Trim(f.Value.String(), "[]"))
					}
				})
				if len(applied) == 0 {
					a.info("No offers are listed right now.")
				} else {
					a.info("No offers match (filters: %s). Try without them: legion offer list", strings.Join(applied, ", "))
				}
			} else {
				a.info("Rent one: legion rental create --offer <host> --catalog <id>")
			}
			return a.emit(offerResult(offers))
		},
	}
	f := list.Flags()
	f.StringArrayVar(&models, "gpu-model", nil, "GPU model, a case-insensitive part of its name, e.g. \"rtx 4090\" (repeatable)")
	f.IntVar(&minGPUs, "min-gpus", 0, "at least this many free GPUs")
	f.StringVar(&maxPrice, "max-price", "", "most per GPU-hour in dollars, e.g. 2.50")
	f.StringVar(&region, "region", "", "reference region for latency: us-east-1, us-west-2, eu-central-1, ap-northeast-1, ap-southeast-1")
	f.IntVar(&maxLatency, "max-latency", 0, "most round-trip milliseconds to --region (or to the host's nearest region)")
	f.StringArrayVar(&countries, "country", nil, "host country code, e.g. US (repeatable)")
	f.StringVar(&catalog, "catalog", "", "only hosts that can run this catalog id")
	f.StringVar(&sort, "sort", "", "price (default), score, reliability, gpu_memory, price_per_tflops")
	f.IntVar(&limit, "limit", 50, "most offers to list")

	get := &cobra.Command{
		Use:     "get <host>",
		Short:   "Show one offer",
		Long:    "Show a host's GPUs, price, hardware and reliability. <host> is a host id or name from 'legion offer list'.",
		Example: "  legion offer get amber-falcon\n  legion offer get 5f0c1c2e-0000-0000-0000-000000000000 -o json",
		Args:    args(1, 1),
		RunE: func(cmd *cobra.Command, as []string) error {
			c, err := a.api()
			if err != nil {
				return err
			}
			o, err := a.findOffer(a.ctx(cmd), c, as[0])
			if err != nil {
				return err
			}
			res := offerResult([]api.Offer{*o})
			res.Data, res.Human = o, func(w io.Writer) { offerDetail(w, *o) }
			return a.emit(res)
		},
	}
	cmd.AddCommand(list, get)
	return cmd
}

// filterModels keeps offers whose GPU model contains any of the given texts.
func filterModels(offers []api.Offer, models []string) []api.Offer {
	var keep []api.Offer
	for _, o := range offers {
		for _, m := range models {
			if strings.Contains(strings.ToLower(o.GpuModel), strings.ToLower(strings.TrimSpace(m))) {
				keep = append(keep, o)
				break
			}
		}
	}
	return keep
}

func offerResult(os []api.Offer) output.Result {
	return listResult(os, []string{"HOST", "GPU", "FREE/TOTAL", "VRAM", "PRICE/GPU-HR"}, []string{"HOST ID", "COUNTRY", "CPU/GPU", "RAM/GPU", "DISK/GPU", "DRIVER"},
		func(o api.Offer) string { return o.HostName },
		func(o api.Offer) []string {
			return []string{o.HostName, o.GpuModel, fmt.Sprintf("%d/%d", o.GpusFree, o.GpuCount), fmt.Sprintf("%d GiB", o.GpuMemoryMib/1024), signed(api.SignedMoney(o.PricePerGpuHour)),
				o.HostId.String(), ptrOr(o.Country, ""), fmt.Sprint(o.PerGpu.CpuCores), output.Bytes(o.PerGpu.MemoryMib << 20), fmt.Sprintf("%d GiB", o.PerGpu.DiskGib), ptrOr(o.DriverVersion, "")}
		})
}

func offerDetail(w io.Writer, o api.Offer) {
	cur := string(o.PricePerGpuHour.Currency)
	fmt.Fprintf(w, "Host:       %s (%s)\nGPU:        %d x %s, %d GiB each, %d free\n", san(o.HostName), o.HostId, o.GpuCount, san(o.GpuModel), o.GpuMemoryMib/1024, o.GpusFree)
	fmt.Fprintf(w, "Price:      %s per GPU-hour\n", output.Money(o.PricePerGpuHour.AmountMinor, cur))
	if o.StoragePricePerGibMonth != nil {
		fmt.Fprintf(w, "Storage:    %s per GiB-month while stopped\n", output.Money(o.StoragePricePerGibMonth.AmountMinor, cur))
	}
	fmt.Fprintf(w, "Per GPU:    %d CPU cores, %s RAM, %d GiB disk\n", o.PerGpu.CpuCores, output.Bytes(o.PerGpu.MemoryMib<<20), o.PerGpu.DiskGib)
	if o.Country != nil {
		fmt.Fprintf(w, "Country:    %s\n", san(*o.Country))
	}
	if o.DriverVersion != nil {
		fmt.Fprintf(w, "Driver:     %s\n", san(*o.DriverVersion))
	}
	fmt.Fprintf(w, "Rent:       legion rental create --offer %s --catalog <id>\n", san(o.HostName))
}

// listOffers reads up to max offers (default 50).
func (a *app) listOffers(ctx context.Context, c *api.ClientWithResponses, p *api.ListOffersParams, max int) ([]api.Offer, error) {
	if max <= 0 {
		max = 50
	}
	return collect(max, func(cur *string, n int) ([]api.Offer, *string, error) {
		p.Cursor, p.Limit = cur, &n
		r, err := c.ListOffersWithResponse(ctx, p)
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
