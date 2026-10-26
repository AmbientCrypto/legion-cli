package cli

import (
	"fmt"
	"io"

	"github.com/AmbientCrypto/legion-controller/api"
	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/AmbientCrypto/legion-cli/internal/output"
)

func (a *app) catalogCmd() *cobra.Command {
	cmd := group(&cobra.Command{
		Use:   "catalog",
		Short: "Browse Legion's starter images and premade models",
		Long: `The catalog lists images that are ready to work in over SSH and premade models that serve
themselves (an OpenAI-compatible endpoint, for example). Rent one with:
legion rental create --offer <host> --catalog <id>.`,
		Example: `  legion catalog list
  legion catalog get qwen3-5-4b`,
	})
	var kind, host string
	list := &cobra.Command{
		Use:   "list",
		Short: "List starter images and premade models",
		Long: `List catalog entries. --kind narrows to images or models. With --host, each entry says whether
that host can run it and how many of its GPUs it needs.`,
		Example: `  legion catalog list --kind model
  legion catalog list --host amber-falcon -o wide`,
		Args: args(0, 0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			es, err := a.catalogEntries(cmd, host)
			if err != nil {
				return err
			}
			if kind != "" {
				if kind != "image" && kind != "model" {
					return usagef("legion catalog list --help", "--kind must be image or model, got %q", kind)
				}
				var keep []api.CatalogEntry
				for _, e := range es {
					if string(e.Kind) == kind {
						keep = append(keep, e)
					}
				}
				es = keep
			}
			if es == nil {
				es = []api.CatalogEntry{}
			}
			return a.emit(output.Result{Data: es, Names: catalogIDs(es), Table: catalogTable(es)})
		},
	}
	list.Flags().StringVar(&kind, "kind", "", "image or model")
	list.Flags().StringVar(&host, "host", "", "host id or name: show whether it can run each entry")
	get := &cobra.Command{
		Use:     "get <id>",
		Short:   "Show one catalog entry",
		Long:    "Show an entry's image reference, requirements and, for a model, its serving details and licence.",
		Example: "  legion catalog get qwen3-5-4b\n  legion catalog get pytorch -o yaml",
		Args:    args(1, 1),
		RunE: func(cmd *cobra.Command, as []string) error {
			es, err := a.catalogEntries(cmd, "")
			if err != nil {
				return err
			}
			for _, e := range es {
				if e.Id == as[0] {
					return a.emit(output.Result{Data: e, Names: []string{e.Id}, Table: catalogTable([]api.CatalogEntry{e}), Human: func(w io.Writer) { catalogDetail(w, e) }})
				}
			}
			return notFoundf("legion catalog list", "no catalog entry %q", as[0])
		},
	}
	cmd.AddCommand(list, get)
	return cmd
}

func (a *app) catalogEntries(cmd *cobra.Command, host string) ([]api.CatalogEntry, error) {
	c, err := a.api()
	if err != nil {
		return nil, err
	}
	var hp *uuid.UUID
	if host != "" {
		o, err := a.findOffer(a.ctx(cmd), c, host)
		if err != nil {
			return nil, err
		}
		hp = &o.HostId
	}
	return a.catalogFor(a.ctx(cmd), c, hp)
}

func catalogIDs(es []api.CatalogEntry) []string {
	n := make([]string, len(es))
	for i, e := range es {
		n[i] = e.Id
	}
	return n
}

func catalogTable(es []api.CatalogEntry) *output.Table {
	t := &output.Table{Cols: []string{"ID", "KIND", "NAME", "MIN VRAM", "DEFAULT"}, Wide: []string{"SIZE", "CUDA", "PROTOCOL", "FITS HOST"}}
	for _, e := range es {
		vram, proto := "", ""
		if e.Serve != nil {
			vram, proto = fmt.Sprintf("%.0f GB", e.Serve.MinVramGb), string(e.Serve.Protocol)
		}
		def := ""
		if e.Default {
			def = "yes"
		}
		fits := ""
		if e.Compatible != nil {
			fits = map[bool]string{true: "yes", false: "no"}[*e.Compatible]
		}
		t.Rows = append(t.Rows, []string{e.Id, string(e.Kind), e.Name, vram, def, fmt.Sprintf("%.1f GB", e.SizeGb), e.Cuda, proto, fits})
	}
	return t
}

func catalogDetail(w io.Writer, e api.CatalogEntry) {
	fmt.Fprintf(w, "ID:         %s\nName:       %s\nKind:       %s\n%s\n", e.Id, e.Name, e.Kind, e.Description)
	fmt.Fprintf(w, "Image:      %s\nNeeds:      NVIDIA driver %s+, compute capability %s+, CUDA %s, %.1f GB image\n", e.Reference, e.MinDriverVersion, e.MinComputeCapability, e.Cuda, e.SizeGb)
	if s := e.Serve; s != nil {
		fmt.Fprintf(w, "Serves:     %s on port %d (%s, %s)\nVRAM:       %.0f GB minimum, %.0f GB recommended, weights %.0f GB\nLicence:    %s %s\n", s.Model, s.Port, s.Protocol, s.Category, s.MinVramGb, s.RecommendedVramGb, s.WeightsGb, s.License, s.LicenseUrl)
		if !s.CommercialOk {
			fmt.Fprintln(w, "            not licensed for commercial use")
		}
		if s.Notice != nil {
			fmt.Fprintf(w, "Notice:     %s\n", *s.Notice)
		}
	}
	fmt.Fprintf(w, "Rent:       legion rental create --offer <host> --catalog %s\n", e.Id)
}
