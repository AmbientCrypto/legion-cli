package cli

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/AmbientCrypto/legion-controller/api"
	"github.com/spf13/cobra"

	"github.com/AmbientCrypto/legion-cli/internal/client"
)

const gatewayUser = "legion"

var (
	nameRE = regexp.MustCompile(`^[a-z][a-z0-9-]{0,61}[a-z0-9]$`)
	hostRE = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9.-]*[A-Za-z0-9])?$`)
	pathRE = regexp.MustCompile(`^[A-Za-z0-9_@+=:,./~-]+$`)
)

// sshEndpoint checks the pieces that go into an ssh command line: they come
// from the API and must not be able to smuggle in an option.
func sshEndpoint(r api.Rental) (host string, port int, err error) {
	if r.Ssh == nil {
		return "", 0, errorf("legion rental wait "+r.Name, "%s has no SSH endpoint yet (state: %s)", r.Name, r.State)
	}
	if !nameRE.MatchString(r.Name) || !hostRE.MatchString(r.Ssh.Host) || r.Ssh.Port < 1 || r.Ssh.Port > 65535 {
		return "", 0, errorf("", "refusing to build an ssh command from an unexpected endpoint (%q port %d)", r.Ssh.Host, r.Ssh.Port)
	}
	return r.Ssh.Host, r.Ssh.Port, nil
}

func checkIdentity(path string) error {
	if path != "" && (!pathRE.MatchString(path) || strings.HasPrefix(path, "-")) {
		return usagef("legion rental ssh --help", "--identity %q has characters that are not safe in an ssh command; use a plain path like ~/.ssh/id_ed25519", path)
	}
	return nil
}

// sshArgv is the ssh command line (without the leading "ssh") for a rental,
// as documented in the web docs "Connecting" page: jump through the gateway
// to root@<rental name>, pinning the rental's host key under its name. With
// an identity file the gateway hop is a ProxyCommand that names the key too.
func sshArgv(r api.Rental, identity string) ([]string, error) {
	host, port, err := sshEndpoint(r)
	if err != nil {
		return nil, err
	}
	if err := checkIdentity(identity); err != nil {
		return nil, err
	}
	pin := "HostKeyAlias=" + r.Name
	dest := "root@" + r.Name
	if identity == "" {
		return []string{"-J", fmt.Sprintf("%s@%s:%d", gatewayUser, host, port), "-o", pin, dest}, nil
	}
	proxy := fmt.Sprintf("ssh -i %s -o IdentitiesOnly=yes -W %%h:%%p -p %d %s@%s", identity, port, gatewayUser, host)
	return []string{"-i", identity, "-o", "IdentitiesOnly=yes", "-o", pin, "-o", "ProxyCommand=" + proxy, dest}, nil
}

// hostKey is the SHA256 fingerprint of the gateway's host key from the API,
// cleaned for display; empty when the controller does not publish one.
func hostKey(r *api.Rental) string {
	if r.Ssh == nil || r.Ssh.HostKeyFingerprint == nil {
		return ""
	}
	return san(*r.Ssh.HostKeyFingerprint)
}

func shellJoin(argv []string) string {
	q := make([]string, len(argv))
	for i, s := range argv {
		if pathRE.MatchString(s) {
			q[i] = s
		} else {
			q[i] = "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
		}
	}
	return strings.Join(q, " ")
}

func (a *app) rentalSSHCmd(top bool) *cobra.Command {
	var printOnly bool
	var identity string
	var waitTimeout time.Duration
	use := "ssh <rental> [-- command...]"
	cmd := &cobra.Command{
		Use:   use,
		Short: "SSH into a rental",
		Long: `Wait until the rental is running, then run the system ssh to connect through Legion's
gateway as root. Everything after -- runs on the rental instead of a shell.

The gateway offers your SSH agent's keys and your default key files. To use one specific key
instead, pass --identity. --print shows the command without running it; 'legion rental
ssh-config' prints an ~/.ssh/config block so plain 'ssh <rental>' works, and so do scp and rsync.`,
		Example: `  legion ssh quiet-river-7k2f
  legion ssh quiet-river-7k2f -- nvidia-smi
  legion rental ssh quiet-river-7k2f --print --identity ~/.ssh/id_ed25519`,
		Args: args(0, -1),
		RunE: func(cmd *cobra.Command, as []string) error {
			remote := []string(nil)
			if d := cmd.ArgsLenAtDash(); d >= 0 {
				as, remote = as[:d], as[d:]
			}
			if len(as) > 1 {
				return usagef("legion ssh --help", "too many arguments before --: %q", strings.Join(as[1:], " "))
			}
			return a.withRental(cmd, as, func(ctx context.Context, c *api.ClientWithResponses, r *api.Rental) error {
				switch r.State {
				case api.RentalStateStopped, api.RentalStateStopping:
					return &cliError{Msg: fmt.Sprintf("%s is %s", r.Name, r.State), Code: "rental_not_running", Hint: "legion rental start " + r.Name, Exit: client.ExitError}
				case api.RentalStateFailed, api.RentalStateDeleted, api.RentalStateDeleting:
					return &cliError{Msg: fmt.Sprintf("%s is %s", r.Name, r.State), Code: "rental_not_running", Hint: "legion rental list", Exit: client.ExitError}
				}
				var err error
				if r.State != api.RentalStateRunning || r.Ssh == nil {
					a.info("Waiting for %s to be running...", r.Name)
					if r, err = a.waitFor(ctx, c, r.Id, "running", waitTimeout, true); err != nil {
						return err
					}
				}
				argv, err := sshArgv(*r, identity)
				if err != nil {
					return err
				}
				argv = append(argv, remote...)
				if fp := hostKey(r); fp != "" {
					a.info("Host key should be %s - ssh will ask you to confirm it on first connect.", fp)
				}
				if printOnly {
					fmt.Fprintln(a.env.Stdout, shellJoin(append([]string{"ssh"}, argv...)))
					return nil
				}
				path, err := a.env.LookPath("ssh")
				if err != nil {
					return errorf("legion ssh "+r.Name+" --print", "the ssh program was not found on your PATH; install OpenSSH, or run the command from --print yourself")
				}
				return a.env.Exec(path, append([]string{"ssh"}, argv...))
			})
		},
	}
	cmd.Flags().BoolVar(&printOnly, "print", false, "print the ssh command instead of running it")
	cmd.Flags().StringVarP(&identity, "identity", "i", "", "use this private key for the gateway and the rental (path like ~/.ssh/id_ed25519)")
	cmd.Flags().DurationVar(&waitTimeout, "wait-timeout", 10*time.Minute, "how long to wait for the rental to run")
	if top {
		cmd.GroupID = grpStart
	}
	return cmd
}

func (a *app) rentalSSHConfigCmd() *cobra.Command {
	var identity string
	cmd := &cobra.Command{
		Use:   "ssh-config <rental>",
		Short: "Print an ~/.ssh/config block for a rental",
		Long: `Print the Host blocks for ~/.ssh/config that let you connect with plain 'ssh <rental>',
scp and rsync. Append them yourself, for example:  legion rental ssh-config <rental> >> ~/.ssh/config`,
		Example: `  legion rental ssh-config quiet-river-7k2f
  legion rental ssh-config quiet-river-7k2f --identity ~/.ssh/id_ed25519 >> ~/.ssh/config`,
		Args: args(0, 1),
		RunE: func(cmd *cobra.Command, as []string) error {
			return a.withRental(cmd, as, func(ctx context.Context, c *api.ClientWithResponses, r *api.Rental) error {
				host, port, err := sshEndpoint(*r)
				if err != nil {
					return err
				}
				if err := checkIdentity(identity); err != nil {
					return err
				}
				id := ""
				if identity != "" {
					id = "  IdentityFile " + identity + "\n  IdentitiesOnly yes\n"
				}
				if fp := hostKey(r); fp != "" {
					fmt.Fprintf(a.env.Stdout, "# Gateway host key: %s (ssh asks you to confirm it on first connect)\n", fp)
				}
				fmt.Fprintf(a.env.Stdout, "Host %[1]s-gateway\n  HostName %[2]s\n  Port %[3]d\n  User %[4]s\n%[5]s\nHost %[1]s\n  HostName %[1]s\n  User root\n  HostKeyAlias %[1]s\n  ProxyJump %[1]s-gateway\n%[5]s  ServerAliveInterval 60\n",
					r.Name, host, port, gatewayUser, id)
				a.info("Append this to ~/.ssh/config, then connect with: ssh %s", r.Name)
				return nil
			})
		},
	}
	cmd.Flags().StringVarP(&identity, "identity", "i", "", "private key to use (path like ~/.ssh/id_ed25519)")
	return cmd
}
