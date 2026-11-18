package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/AmbientCrypto/legion-controller/api"
	"github.com/spf13/cobra"

	"github.com/AmbientCrypto/legion-cli/internal/output"
)

// ---- rental env (LGN-628) ----

func (a *app) rentalEnvCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "env",
		Short: "Show or change a rental's environment variables",
		Long: `List, set and unset the environment variables of a rental's container. A secret's value is write-only: it is
stored encrypted, listed as "set, hidden" and never shown again. Changing variables restarts the rental's container
(files outside /workspace are reset); a stopped rental takes them at its next start. Anything in the container,
including these variables, can be read by the provider's machine.`,
		Example: "  legion rental env list quiet-river-7k2f\n  legion rental env set quiet-river-7k2f --env MODE=fast --secret-env HF_TOKEN\n  legion rental env unset quiet-river-7k2f MODE",
	}
	cmd.AddCommand(a.rentalEnvListCmd(), a.rentalEnvSetCmd(), a.rentalEnvUnsetCmd())
	return cmd
}

func (a *app) rentalEnvListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list <rental>",
		Aliases: []string{"ls"},
		Short:   "List a rental's environment variables",
		Long:    "List the variable names of a rental with their values. Secrets show as set and hidden: their values are never returned.",
		Example: "  legion rental env list quiet-river-7k2f\n  legion rental env list quiet -o json",
		Args:    args(0, 1),
		RunE: func(cmd *cobra.Command, as []string) error {
			return a.withRental(cmd, as, func(ctx context.Context, c *api.ClientWithResponses, r *api.Rental) error {
				fresh, err := a.getRental(ctx, c, r.Id)
				if err != nil {
					return err
				}
				return a.emit(envResult(fresh.Name, ptrOrSlice(fresh.Env)))
			})
		},
	}
}

func envResult(owner string, env []api.EnvVar) output.Result {
	rows := make([][]string, 0, len(env))
	names := make([]string, 0, len(env))
	for _, e := range env {
		v := san(ptrOr(e.Value, ""))
		if e.Secret {
			v = "set, hidden"
		}
		rows = append(rows, []string{san(e.Name), map[bool]string{true: "yes", false: "no"}[e.Secret], v})
		names = append(names, e.Name)
	}
	res := output.Result{Data: env, Names: names, Table: &output.Table{Cols: []string{"NAME", "SECRET", "VALUE"}, Rows: rows}}
	if len(env) == 0 {
		res.Human = func(w io.Writer) { fmt.Fprintf(w, "%s has no environment variables.\n", san(owner)) }
	}
	return res
}

func (a *app) rentalEnvSetCmd() *cobra.Command {
	var cf configFlags
	cmd := &cobra.Command{
		Use:   "set <rental> --env NAME=value | --secret-env NAME",
		Short: "Add or change environment variables",
		Long: `Add or change variables; the others stay. --env NAME=value sets a plain variable. --secret-env NAME reads the
value from stdin (one line per secret) or a hidden prompt, and stores it encrypted and write-only. This restarts the
rental's container (files outside /workspace are reset), so legion asks first; a stopped rental takes the change at
its next start.`,
		Example: "  legion rental env set quiet-river-7k2f --env MODE=fast\n  printf %s \"$HF_TOKEN\" | legion rental env set quiet-river-7k2f --secret-env HF_TOKEN --yes",
		Args:    args(0, 1),
		RunE: func(cmd *cobra.Command, as []string) error {
			if len(cf.env)+len(cf.secretEnv) == 0 || len(cf.setting)+len(cf.command)+len(cf.arg) > 0 {
				return usagef("legion rental env set --help", "pass --env NAME=value or --secret-env NAME (model settings and command cannot be changed on a rental)")
			}
			cfg, err := a.resolveConfig(cmd, &cf, "rental env set")
			if err != nil {
				return err
			}
			return a.withRental(cmd, as, func(ctx context.Context, c *api.ClientWithResponses, r *api.Rental) error {
				fresh, err := a.getRental(ctx, c, r.Id)
				if err != nil {
					return err
				}
				return a.putRentalEnv(ctx, c, fresh, mergeEnv(ptrOrSlice(fresh.Env), cfg.Env, nil))
			})
		},
	}
	cf.registerEnvOnly(cmd)
	return cmd
}

func (a *app) rentalEnvUnsetCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "unset <rental> NAME...",
		Short:   "Remove environment variables",
		Long:    "Remove the named variables; the others stay. This restarts the rental's container (files outside /workspace are reset), so legion asks first.",
		Example: "  legion rental env unset quiet-river-7k2f MODE\n  legion rental env unset quiet-river-7k2f MODE HF_TOKEN --yes",
		Args:    args(2, 64),
		RunE: func(cmd *cobra.Command, as []string) error {
			names := as[1:]
			for _, n := range names {
				if err := checkEnvName(n); err != nil {
					return usagef("legion rental env list "+as[0], "%v", err)
				}
			}
			return a.withRental(cmd, as[:1], func(ctx context.Context, c *api.ClientWithResponses, r *api.Rental) error {
				fresh, err := a.getRental(ctx, c, r.Id)
				if err != nil {
					return err
				}
				return a.putRentalEnv(ctx, c, fresh, mergeEnv(ptrOrSlice(fresh.Env), nil, names))
			})
		},
	}
}

func (a *app) putRentalEnv(ctx context.Context, c *api.ClientWithResponses, r *api.Rental, env []api.EnvVarInput) error {
	q := fmt.Sprintf("Change the environment of %s? Its container restarts if it may be running; files outside /workspace are reset.", r.Name)
	if err := a.confirm(q, "refusing to change the environment of "+r.Name+" without confirmation: its container restarts and files outside /workspace are reset"); err != nil {
		return err
	}
	resp, err := c.UpdateRentalWithResponse(ctx, r.Id, api.UpdateRentalRequest{Env: &env})
	if err := a.ok(resp, err); err != nil {
		return err
	}
	upd, err := need(resp.JSON200)
	if err != nil {
		return err
	}
	a.info("Updated the environment of %s.", upd.Name)
	return a.emit(envResult(upd.Name, ptrOrSlice(upd.Env)))
}
