package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/AmbientCrypto/legion-controller/api"
	"github.com/spf13/cobra"
)

// deprecated tells the person the command that replaces an old one, on stderr
// (always: it is not progress, it is a notice).
func (a *app) deprecated(old, replacement string) {
	fmt.Fprintf(a.env.Stderr, "Deprecated: 'legion rental ingress %s' is now: %s\n", old, replacement)
}

// ingressCmd is the old 'legion rental ingress': an exposed port is an
// endpoint now, so each command calls the endpoint code.
func (a *app) ingressCmd() *cobra.Command {
	cmd := group(&cobra.Command{
		Use:   "ingress",
		Short: "Deprecated: use 'legion endpoints' to expose a port",
		Long: `Deprecated. An exposed port is an endpoint now: use 'legion endpoints create --rental <rental> --port <port>'.
These commands still work: each one does what its replacement does and prints the new command on stderr.`,
		Example: `  legion rental ingress add quiet-river-7k2f --port 8000
  legion rental ingress list quiet-river-7k2f`,
	})
	list := &cobra.Command{
		Use:     "list <rental>",
		Short:   "Deprecated: list a rental's exposed ports",
		Long:    "Deprecated: same as 'legion endpoints list --rental <rental>'.",
		Example: "  legion rental ingress list quiet-river-7k2f\n  legion rental ingress list quiet -o json",
		Args:    args(0, 1),
		RunE: func(cmd *cobra.Command, as []string) error {
			return a.withRental(cmd, as, func(ctx context.Context, c *api.ClientWithResponses, r *api.Rental) error {
				a.deprecated("list", "legion endpoints list --rental "+r.Name)
				return a.listEndpointsRun(ctx, c, "", &r.Id)
			})
		},
	}
	var port int
	var sources []string
	var desc string
	add := &cobra.Command{
		Use:   "add <rental>",
		Short: "Deprecated: expose a port",
		Long: `Deprecated: same as 'legion endpoints create --rental <rental> --port <port> --access public'. Without --source
anyone on the internet can reach the port; add --source <cidr> (repeatable) to limit it. You are asked to confirm
(or pass --yes).`,
		Example: "  legion rental ingress add quiet-river-7k2f --port 8000\n  legion rental ingress add quiet-river-7k2f --port 8000 --source 203.0.113.0/24",
		Args:    args(0, 1),
		RunE: func(cmd *cobra.Command, as []string) error {
			if port == 0 {
				return usagef("legion rental ingress add --help", "missing required flag --port")
			}
			return a.withRental(cmd, as, func(ctx context.Context, c *api.ClientWithResponses, r *api.Rental) error {
				repl := fmt.Sprintf("legion endpoints create --rental %s --port %d --access public", r.Name, port)
				for _, s := range sources {
					repl += " --source " + s
				}
				a.deprecated("add", repl)
				return a.createPortEndpoint(cmd, "", r.Id.String(), port, "public", sources, "", "default", "")
			})
		},
	}
	add.Flags().IntVar(&port, "port", 0, "port the service listens on in the rental (not 22 or 22022)")
	add.Flags().StringArrayVar(&sources, "source", nil, "allowed source CIDR, e.g. 203.0.113.0/24 (repeatable; default anyone)")
	add.Flags().StringVar(&desc, "description", "", "ignored: endpoints have no description")
	_ = add.Flags().MarkHidden("description")
	remove := &cobra.Command{
		Use:     "remove <rental> <rule-id>",
		Aliases: []string{"rm"},
		Short:   "Deprecated: stop exposing a port",
		Long:    "Deprecated: same as 'legion endpoints delete <endpoint>'. <rule-id> is the old rule id (see 'legion rental ingress list -o wide'), or an endpoint name or id.",
		Example: "  legion rental ingress remove quiet-river-7k2f quiet-river-7k2f --yes\n  legion rental ingress remove quiet 6f1c... --yes",
		Args:    args(2, 2),
		RunE: func(cmd *cobra.Command, as []string) error {
			return a.withRental(cmd, as[:1], func(ctx context.Context, c *api.ClientWithResponses, r *api.Rental) error {
				e, err := a.endpointForRule(ctx, c, r, as[1])
				if err != nil {
					return err
				}
				a.deprecated("remove", "legion endpoints delete "+e.Name)
				if err := a.confirm(fmt.Sprintf("Remove endpoint %s from %s? Its public URL stops working.", e.Name, r.Name), "refusing to remove an exposed port without confirmation"); err != nil {
					return err
				}
				resp, err := c.DeleteEndpointWithResponse(ctx, e.Id)
				if err := a.ok(resp, err); err != nil {
					return err
				}
				a.info("Removed endpoint %s.", e.Name)
				return nil
			})
		},
	}
	cmd.AddCommand(list, add, remove)
	return cmd
}

// endpointForRule finds the endpoint behind an old ingress rule id of rental
// r: the backfilled endpoint is named after the rule's hostname. An endpoint
// name or id works too.
func (a *app) endpointForRule(ctx context.Context, c *api.ClientWithResponses, r *api.Rental, arg string) (*api.Endpoint, error) {
	if resp, err := c.GetRentalNetworkWithResponse(ctx, r.Id); err == nil && resp.JSON200 != nil {
		for _, ru := range resp.JSON200.Ingress.Rules {
			if ru.Id.String() == strings.ToLower(arg) {
				arg = strings.SplitN(ru.Hostname, ".", 2)[0]
			}
		}
	}
	return a.pickEndpoint(ctx, c, []string{arg})
}
