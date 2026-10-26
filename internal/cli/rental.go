package cli

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/AmbientCrypto/legion-controller/api"
	"github.com/spf13/cobra"

	"github.com/AmbientCrypto/legion-cli/internal/client"
	"github.com/AmbientCrypto/legion-cli/internal/output"
	"github.com/AmbientCrypto/legion-cli/internal/ui"
)

func (a *app) rentalCmd() *cobra.Command {
	cmd := group(&cobra.Command{
		Use:     "rental",
		Aliases: []string{"rentals"},
		Short:   "Create, connect to, stop and delete rentals",
		Long: `A rental is a container on a host's GPUs, billed per second while it runs.
A rental is named by id, friendly name (quiet-river-7k2f) or any unique prefix of either.
Leave the name out on a terminal to pick from a list.`,
		Example: `  legion rental create --offer <host-id> --catalog <id>
  legion rental list`,
	})
	cmd.AddCommand(a.rentalListCmd(), a.rentalGetCmd(), a.rentalCreateCmd(), a.rentalStopCmd(), a.rentalStartCmd(),
		a.rentalDeleteCmd(), a.rentalWaitCmd(), a.rentalSSHCmd(false), a.rentalSSHConfigCmd(), a.rentalChargesCmd(),
		a.rentalServeCmd(), a.ingressCmd(), a.backupCmd(), a.exportCmd())
	return cmd
}

// ---- fetching and resolving ----

func (a *app) listRentals(ctx context.Context, c *api.ClientWithResponses) ([]api.Rental, error) {
	return listAll(func(cur *string) ([]api.Rental, *string, error) {
		r, err := c.ListRentalsWithResponse(ctx, &api.ListRentalsParams{Cursor: cur, Limit: ptr(100)})
		if err := a.ok(r, err); err != nil {
			return nil, nil, err
		}
		return r.JSON200.Items, r.JSON200.NextCursor, nil
	})
}

func (a *app) getRental(ctx context.Context, c *api.ClientWithResponses, id api.RentalID) (*api.Rental, error) {
	r, err := c.GetRentalWithResponse(ctx, id)
	if err := a.ok(r, err); err != nil {
		return nil, err
	}
	return r.JSON200, nil
}

// pickRental resolves the <rental> argument, or offers a picker when it is
// missing on a terminal. Deleted rentals are skipped unless named exactly.
func (a *app) pickRental(ctx context.Context, c *api.ClientWithResponses, as []string) (*api.Rental, error) {
	if len(as) == 0 && !a.interactive() {
		return nil, usagef("legion "+a.cmdPath+" --help", "missing required argument <rental>")
	}
	all, err := a.listRentals(ctx, c)
	if err != nil {
		return nil, err
	}
	if len(as) > 0 {
		return matchRental(all, as[0])
	}
	var live []api.Rental
	for _, r := range all {
		if r.State != api.RentalStateDeleted {
			live = append(live, r)
		}
	}
	labels := make([]string, len(live))
	for i, r := range live {
		labels[i] = fmt.Sprintf("%-22s %-9s %d GPU  %s", r.Name, r.State, r.GpuCount, r.Image)
	}
	i, err := a.pickOne("Which rental?", labels, "missing required argument <rental>", "legion rental list")
	if err != nil {
		return nil, err
	}
	return &live[i], nil
}

// matchRental finds one rental by id, name or unique prefix of either.
func matchRental(all []api.Rental, arg string) (*api.Rental, error) {
	arg = strings.ToLower(strings.TrimSpace(arg))
	var prefix []api.Rental
	for i := range all {
		r := all[i]
		if r.Id.String() == arg || r.Name == arg {
			return &r, nil
		}
		if strings.HasPrefix(r.Name, arg) || strings.HasPrefix(r.Id.String(), arg) {
			prefix = append(prefix, r)
		}
	}
	switch len(prefix) {
	case 0:
		return nil, notFoundf("legion rental list", "no rental matches %q", arg)
	case 1:
		return &prefix[0], nil
	}
	names := make([]string, len(prefix))
	for i, r := range prefix {
		names[i] = r.Name
	}
	sort.Strings(names)
	return nil, usagef("Use a longer prefix or the full name.", "%q is ambiguous: it matches %s", arg, strings.Join(names, ", "))
}

// ---- rendering ----

func hourly(r api.Rental) int64 { return r.PricePerGpuHour.AmountMinor * int64(r.GpuCount) }

func rentalTable(rs []api.Rental) *output.Table {
	t := &output.Table{Cols: []string{"NAME", "STATE", "GPUS", "PRICE/GPU-HR", "IMAGE", "CREATED"}, Wide: []string{"ID", "HOST", "SSH"}}
	for _, r := range rs {
		ssh := ""
		if r.Ssh != nil {
			ssh = fmt.Sprintf("%s:%d", r.Ssh.Host, r.Ssh.Port)
		}
		t.Rows = append(t.Rows, []string{r.Name, string(r.State), fmt.Sprint(r.GpuCount), output.Money(r.PricePerGpuHour.AmountMinor, string(r.PricePerGpuHour.Currency)),
			r.Image, r.CreatedAt.Local().Format("2006-01-02 15:04"), r.Id.String(), r.HostId.String(), ssh})
	}
	return t
}

func rentalNames(rs []api.Rental) []string {
	n := make([]string, len(rs))
	for i, r := range rs {
		n[i] = r.Name
	}
	return n
}

func (a *app) one(r api.Rental) output.Result {
	return output.Result{Data: r, Names: []string{r.Name}, Table: rentalTable([]api.Rental{r}), Human: func(w io.Writer) { rentalDetail(w, a.style(), r) }}
}

func rentalDetail(w io.Writer, st ui.Style, r api.Rental) {
	state := string(r.State)
	switch r.State {
	case api.RentalStateRunning:
		state = st.Green(state)
	case api.RentalStateFailed:
		state = st.Red(state)
	}
	fmt.Fprintf(w, "Name:       %s\nID:         %s\nState:      %s\n", st.Bold(r.Name), r.Id, state)
	if r.FailureReason != nil {
		fmt.Fprintf(w, "Failure:    %s\n", *r.FailureReason)
	}
	fmt.Fprintf(w, "Image:      %s\nGPUs:       %d (host %s)\n", r.Image, r.GpuCount, r.HostId)
	fmt.Fprintf(w, "Price:      %s per GPU-hour (%s per hour)\n", output.Money(r.PricePerGpuHour.AmountMinor, string(r.PricePerGpuHour.Currency)), output.Money(hourly(r), string(r.PricePerGpuHour.Currency)))
	if r.StoragePricePerGibMonth != nil {
		fmt.Fprintf(w, "Storage:    %s per GiB-month while stopped\n", output.Money(r.StoragePricePerGibMonth.AmountMinor, string(r.StoragePricePerGibMonth.Currency)))
	}
	fmt.Fprintf(w, "Resources:  %d CPU cores, %s RAM, %d GiB disk\n", r.Resources.CpuCores, output.Bytes(r.Resources.MemoryMib<<20), r.Resources.DiskGib)
	fmt.Fprintf(w, "Created:    %s\n", r.CreatedAt.Local().Format(time.RFC3339))
	if r.Serve != nil {
		fmt.Fprintf(w, "Serving:    %s (%s)  see: legion rental serve %s\n", r.Serve.Model, r.Serve.Kind, r.Name)
	}
	if r.Ssh != nil {
		if argv, err := sshArgv(r, ""); err == nil {
			fmt.Fprintf(w, "SSH:        %s\n", shellJoin(append([]string{"ssh"}, argv...)))
		}
		fmt.Fprintf(w, "Connect:    legion ssh %s\n", r.Name)
	}
}

// ---- list / get ----

func (a *app) rentalListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List your rentals",
		Long:    "List the organization's rentals with state and price. Use -o wide for host and SSH endpoint, -o name for scripting.",
		Example: `  legion rental list
  legion rental list -o jsonpath='$[?(@.state=="running")].name'`,
		Args: args(0, 0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := a.api()
			if err != nil {
				return err
			}
			rs, err := a.listRentals(a.ctx(cmd), c)
			if err != nil {
				return err
			}
			if rs == nil {
				rs = []api.Rental{}
			}
			if len(rs) == 0 {
				a.info("No rentals yet. Start one: legion offer list")
			}
			return a.emit(output.Result{Data: rs, Names: rentalNames(rs), Table: rentalTable(rs)})
		},
	}
}

func (a *app) rentalGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "get <rental>",
		Short:   "Show one rental",
		Long:    "Show a rental's state, price, resources and how to connect. <rental> is an id, a name or a unique prefix.",
		Example: "  legion rental get quiet-river-7k2f\n  legion rental get quiet -o json",
		Args:    args(0, 1),
		RunE: func(cmd *cobra.Command, as []string) error {
			c, err := a.api()
			if err != nil {
				return err
			}
			r, err := a.pickRental(a.ctx(cmd), c, as)
			if err != nil {
				return err
			}
			fresh, err := a.getRental(a.ctx(cmd), c, r.Id)
			if err != nil {
				return err
			}
			return a.emit(a.one(*fresh))
		},
	}
}

// ---- stop / start / delete ----

func (a *app) rentalStopCmd() *cobra.Command {
	var async bool
	var waitTimeout time.Duration
	cmd := &cobra.Command{
		Use:   "stop <rental>",
		Short: "Stop the workload; keep the workspace",
		Long: `Stop the rental's container and release its GPUs. Billing for GPU time ends.
The workspace is kept on the host and billed for storage, per GiB, until you delete the rental.
Start it again with: legion rental start. Waits until stopped unless --async.`,
		Example: "  legion rental stop quiet-river-7k2f\n  legion rental stop quiet --yes --async",
		Args:    args(0, 1),
		RunE: func(cmd *cobra.Command, as []string) error {
			c, err := a.api()
			if err != nil {
				return err
			}
			r, err := a.pickRental(a.ctx(cmd), c, as)
			if err != nil {
				return err
			}
			q := fmt.Sprintf("Stop %s? The workload stops and the workspace is kept, billed for storage per GiB until deleted.", r.Name)
			if err := a.confirm(q, fmt.Sprintf("refusing to stop %s without confirmation: its workload stops and the workspace is billed for storage until deleted", r.Name)); err != nil {
				return err
			}
			resp, err := c.StopRentalWithResponse(a.ctx(cmd), r.Id, &api.StopRentalParams{})
			if err := a.ok(resp, err); err != nil {
				return err
			}
			final := resp.JSON200
			if !async {
				if final, err = a.waitFor(a.ctx(cmd), c, r.Id, "stopped", waitTimeout, false); err != nil {
					return err
				}
			}
			a.info("Stopped %s: the workload is stopped and GPU billing has ended. The workspace is kept and billed for storage per GiB until deleted.", r.Name)
			a.info("Resume: legion rental start %s", r.Name)
			a.info("Destroy the workspace and end all billing: legion rental delete %s", r.Name)
			return a.emit(a.one(*final))
		},
	}
	cmd.Flags().BoolVar(&async, "async", false, "return without waiting for the rental to stop")
	cmd.Flags().DurationVar(&waitTimeout, "wait-timeout", 5*time.Minute, "how long to wait for the rental to stop")
	return cmd
}

func (a *app) rentalStartCmd() *cobra.Command {
	var async bool
	var waitTimeout time.Duration
	cmd := &cobra.Command{
		Use:   "start <rental>",
		Short: "Start a stopped rental",
		Long: `Start a stopped rental again on its host's GPUs. GPU billing starts when it is running.
Needs credit and free GPUs on the host. Waits until running unless --async.`,
		Example: "  legion rental start quiet-river-7k2f\n  legion rental start quiet --async",
		Args:    args(0, 1),
		RunE: func(cmd *cobra.Command, as []string) error {
			c, err := a.api()
			if err != nil {
				return err
			}
			r, err := a.pickRental(a.ctx(cmd), c, as)
			if err != nil {
				return err
			}
			resp, err := c.StartRentalWithResponse(a.ctx(cmd), r.Id)
			if err := a.ok(resp, err); err != nil {
				return err
			}
			final := resp.JSON200
			if !async {
				if final, err = a.waitFor(a.ctx(cmd), c, r.Id, "running", waitTimeout, true); err != nil {
					return err
				}
				a.info("Started %s. GPU billing is running.", r.Name)
				a.info("Connect: legion ssh %s", r.Name)
			} else {
				a.info("Starting %s. Wait for it: legion rental wait %s", r.Name, r.Name)
			}
			a.info("Stop billing: legion rental stop %s", r.Name)
			return a.emit(a.one(*final))
		},
	}
	cmd.Flags().BoolVar(&async, "async", false, "return without waiting for the rental to run")
	cmd.Flags().DurationVar(&waitTimeout, "wait-timeout", 10*time.Minute, "how long to wait for the rental to run")
	return cmd
}

func (a *app) rentalDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "delete <rental>",
		Aliases: []string{"rm"},
		Short:   "Delete a rental and destroy its workspace",
		Long: `Delete the rental: the container AND its workspace are destroyed for good and all billing ends.
Make a backup first with: legion rental backup create. On a terminal you must type the rental's
name to confirm; in scripts pass --yes.`,
		Example: "  legion rental delete quiet-river-7k2f\n  legion rental delete quiet-river-7k2f --yes",
		Args:    args(0, 1),
		RunE: func(cmd *cobra.Command, as []string) error {
			c, err := a.api()
			if err != nil {
				return err
			}
			r, err := a.pickRental(a.ctx(cmd), c, as)
			if err != nil {
				return err
			}
			if !a.yes {
				if !a.interactive() {
					return usagef("Add --yes to delete without a prompt.", "refusing to delete %s without confirmation: deleting destroys the workspace for good", r.Name)
				}
				fmt.Fprintf(a.env.Stderr, "Deleting %s destroys the workspace for good. Back it up first if you need it: legion rental backup create %s\n", r.Name, r.Name)
				got, err := a.prompt.Ask(fmt.Sprintf("Type the rental name (%s) to confirm:", r.Name))
				if err != nil || got != r.Name {
					return &cliError{Msg: "aborted: the name did not match", Exit: client.ExitError}
				}
			}
			resp, err := c.DeleteRentalWithResponse(a.ctx(cmd), r.Id, &api.DeleteRentalParams{})
			if err := a.ok(resp, err); err != nil {
				return err
			}
			a.info("Deleting %s: the workspace is being destroyed and billing has ended.", r.Name)
			a.info("Check: legion rental list")
			return a.emit(a.one(*resp.JSON200))
		},
	}
}

// ---- wait ----

func (a *app) rentalWaitCmd() *cobra.Command {
	var forState string
	var timeout time.Duration
	cmd := &cobra.Command{
		Use:   "wait <rental>",
		Short: "Wait until a rental is running or stopped",
		Long: `Poll a rental until it reaches a state, then exit 0. Exits 1 on failure or timeout.
Use it in scripts after 'legion rental create --async'.`,
		Example: "  legion rental wait quiet-river-7k2f\n  legion rental wait quiet-river-7k2f --for stopped --timeout 2m",
		Args:    args(0, 1),
		RunE: func(cmd *cobra.Command, as []string) error {
			if forState != "running" && forState != "stopped" {
				return usagef("legion rental wait --help", "--for must be running or stopped, got %q", forState)
			}
			c, err := a.api()
			if err != nil {
				return err
			}
			r, err := a.pickRental(a.ctx(cmd), c, as)
			if err != nil {
				return err
			}
			final, err := a.waitFor(a.ctx(cmd), c, r.Id, forState, timeout, forState == "running")
			if err != nil {
				return err
			}
			return a.emit(a.one(*final))
		},
	}
	cmd.Flags().StringVar(&forState, "for", "running", "state to wait for: running or stopped")
	cmd.Flags().DurationVar(&timeout, "timeout", 10*time.Minute, "give up after this long")
	return cmd
}

// waitFor polls until the rental is running (with an SSH endpoint when
// needSSH) or stopped. It stops early on a state that cannot recover.
func (a *app) waitFor(ctx context.Context, c *api.ClientWithResponses, id api.RentalID, target string, timeout time.Duration, needSSH bool) (*api.Rental, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	sp := a.spinner()
	sp.Start("waiting for the rental to be " + target)
	defer sp.Stop()
	var last *api.Rental
	for {
		r, err := a.getRental(ctx, c, id)
		if err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				break
			}
			return nil, err
		}
		last = r
		sp.Update(fmt.Sprintf("%s is %s", r.Name, r.State))
		switch {
		case string(r.State) == target && (!needSSH || r.Ssh != nil):
			return r, nil
		case r.State == api.RentalStateFailed:
			reason := ptrOr(r.FailureReason, "no reason given")
			return nil, &cliError{Msg: fmt.Sprintf("%s failed: %s", r.Name, reason), Code: "rental_failed", Hint: "legion rental delete " + r.Name, Exit: client.ExitError}
		case r.State == api.RentalStateDeleted || r.State == api.RentalStateDeleting:
			return nil, &cliError{Msg: fmt.Sprintf("%s is %s", r.Name, r.State), Code: "rental_deleted", Hint: "legion rental list", Exit: client.ExitError}
		}
		select {
		case <-ctx.Done():
		case <-time.After(a.env.Poll):
			continue
		}
		break
	}
	if ctx.Err() == context.Canceled {
		return nil, context.Canceled
	}
	state, name := "unknown", id.String()
	if last != nil {
		state, name = string(last.State), last.Name
	}
	return nil, &cliError{Msg: fmt.Sprintf("timed out after %s waiting for %s to be %s (last state: %s)", timeout, name, target, state), Code: "wait_timeout", Hint: "legion rental get " + name, Exit: client.ExitError}
}

// ---- charges / serve ----

func (a *app) rentalChargesCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "charges <rental>",
		Short:   "Show what one rental has cost so far",
		Long:    "Show the billed seconds and the compute, storage and backup charges of one rental. For all rentals: legion billing charges.",
		Example: "  legion rental charges quiet-river-7k2f\n  legion rental charges quiet -o json",
		Args:    args(0, 1),
		RunE: func(cmd *cobra.Command, as []string) error {
			c, err := a.api()
			if err != nil {
				return err
			}
			r, err := a.pickRental(a.ctx(cmd), c, as)
			if err != nil {
				return err
			}
			resp, err := c.GetRentalChargesWithResponse(a.ctx(cmd), r.Id)
			if err := a.ok(resp, err); err != nil {
				return err
			}
			ch := resp.JSON200
			return a.emit(output.Result{Data: ch, Human: func(w io.Writer) {
				total := ch.Charged.AmountMinor + ch.StorageCharged.AmountMinor
				fmt.Fprintf(w, "Rental:   %s\nBilled:   %s\nCompute:  %s\nStorage:  %s\n", r.Name, output.Duration(ch.Seconds), signed(ch.Charged), signed(ch.StorageCharged))
				if ch.BackupCharged != nil {
					total += ch.BackupCharged.AmountMinor
					fmt.Fprintf(w, "Backups:  %s\n", signed(*ch.BackupCharged))
				}
				fmt.Fprintf(w, "Total:    %s\n", output.Money(total, string(ch.Charged.Currency)))
			}})
		},
	}
}

func signed(m api.SignedMoney) string { return output.Money(m.AmountMinor, string(m.Currency)) }

func (a *app) rentalServeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve <rental>",
		Short: "Show the endpoint and API key of a premade model rental",
		Long: `For a rental created from a premade model (legion rental create --catalog <model>), show the
public URL, its status and the service API key (lsk_...). The key is a secret: it is shown here only
when you ask, never in 'rental get'.`,
		Example: "  legion rental serve quiet-river-7k2f\n  legion rental serve quiet-river-7k2f -o jsonpath=$.url",
		Args:    args(0, 1),
		RunE: func(cmd *cobra.Command, as []string) error {
			c, err := a.api()
			if err != nil {
				return err
			}
			r, err := a.pickRental(a.ctx(cmd), c, as)
			if err != nil {
				return err
			}
			resp, err := c.GetRentalServeWithResponse(a.ctx(cmd), r.Id)
			if err := a.ok(resp, err); err != nil {
				return err
			}
			s := resp.JSON200
			return a.emit(output.Result{Data: s, Human: func(w io.Writer) {
				fmt.Fprintf(w, "Model:    %s (%s)\nStatus:   %s", s.Model, s.Kind, s.Status)
				if s.StatusReason != nil {
					fmt.Fprintf(w, " (%s)", *s.StatusReason)
				}
				fmt.Fprintf(w, "\nURL:      %s\nAPI key:  %s\n", ptrOr(s.Url, "(not ready yet)"), s.ApiKey)
			}})
		},
	}
}
