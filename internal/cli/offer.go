package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/AmbientCrypto/legion-controller/api"
	"github.com/spf13/cobra"

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
		models                          []string
		minGPUs, maxLatency, limit      int
		maxPrice, region, catalog, sort string
		countries                       []string
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
			p := &api.ListOffersParams{}
			switch len(models) {
			case 0:
			case 1:
				p.GpuModel = &models[0]
			default:
				p.GpuModels = &models
			}
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
			offers, err := a.listOffers(a.ctx(cmd), c, p, limit)
			if err != nil {
				return err
			}
			if len(offers) == 0 {
				a.info("No offers match. Loosen a filter, for example --max-price or --min-gpus.")
			} else {
				a.info("Rent one: legion rental create --offer <host> --catalog <id>")
			}
			return a.emit(output.Result{Data: offers, Names: offerNames(offers), Table: offerTable(offers)})
		},
	}
	f := list.Flags()
	f.StringArrayVar(&models, "gpu-model", nil, "exact GPU model, e.g. \"NVIDIA GeForce RTX 4090\" (repeatable)")
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
			return a.emit(output.Result{Data: o, Names: []string{o.HostName}, Table: offerTable([]api.Offer{*o}), Human: func(w io.Writer) { offerDetail(w, *o) }})
		},
	}
	cmd.AddCommand(list, get)
	return cmd
}

func offerNames(os []api.Offer) []string {
	n := make([]string, len(os))
	for i, o := range os {
		n[i] = o.HostName
	}
	return n
}

func offerTable(os []api.Offer) *output.Table {
	t := &output.Table{Cols: []string{"HOST", "GPU", "FREE/TOTAL", "VRAM", "PRICE/GPU-HR"}, Wide: []string{"HOST ID", "COUNTRY", "CPU/GPU", "RAM/GPU", "DISK/GPU", "DRIVER"}}
	for _, o := range os {
		t.Rows = append(t.Rows, []string{o.HostName, o.GpuModel, fmt.Sprintf("%d/%d", o.GpusFree, o.GpuCount), fmt.Sprintf("%d GiB", o.GpuMemoryMib/1024),
			output.Money(o.PricePerGpuHour.AmountMinor, string(o.PricePerGpuHour.Currency)),
			o.HostId.String(), ptrOr(o.Country, ""), fmt.Sprint(o.PerGpu.CpuCores), output.Bytes(o.PerGpu.MemoryMib << 20), fmt.Sprintf("%d GiB", o.PerGpu.DiskGib), ptrOr(o.DriverVersion, "")})
	}
	return t
}

func offerDetail(w io.Writer, o api.Offer) {
	cur := string(o.PricePerGpuHour.Currency)
	fmt.Fprintf(w, "Host:       %s (%s)\nGPU:        %d x %s, %d GiB each, %d free\n", o.HostName, o.HostId, o.GpuCount, o.GpuModel, o.GpuMemoryMib/1024, o.GpusFree)
	fmt.Fprintf(w, "Price:      %s per GPU-hour\n", output.Money(o.PricePerGpuHour.AmountMinor, cur))
	if o.StoragePricePerGibMonth != nil {
		fmt.Fprintf(w, "Storage:    %s per GiB-month while stopped\n", output.Money(o.StoragePricePerGibMonth.AmountMinor, cur))
	}
	fmt.Fprintf(w, "Per GPU:    %d CPU cores, %s RAM, %d GiB disk\n", o.PerGpu.CpuCores, output.Bytes(o.PerGpu.MemoryMib<<20), o.PerGpu.DiskGib)
	if o.Country != nil {
		fmt.Fprintf(w, "Country:    %s\n", *o.Country)
	}
	if o.DriverVersion != nil {
		fmt.Fprintf(w, "Driver:     %s\n", *o.DriverVersion)
	}
	fmt.Fprintf(w, "Rent:       legion rental create --offer %s --catalog <id>\n", o.HostName)
}

// listOffers pages through listOffers until max offers are collected.
func (a *app) listOffers(ctx context.Context, c *api.ClientWithResponses, p *api.ListOffersParams, max int) ([]api.Offer, error) {
	if max <= 0 {
		max = 50
	}
	page := min(max, 100)
	p.Limit = &page
	var all []api.Offer
	var cur *string
	for len(all) < max {
		p.Cursor = cur
		r, err := c.ListOffersWithResponse(ctx, p)
		if err := a.ok(r, err); err != nil {
			return nil, err
		}
		all = append(all, r.JSON200.Items...)
		if r.JSON200.NextCursor == nil || *r.JSON200.NextCursor == "" {
			break
		}
		cur = r.JSON200.NextCursor
	}
	if len(all) > max {
		all = all[:max]
	}
	if all == nil {
		all = []api.Offer{}
	}
	return all, nil
}
