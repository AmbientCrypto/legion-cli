package cli

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/AmbientCrypto/legion-cli/internal/output"
)

const (
	grpStart   = "start"
	grpCompute = "compute"
	grpAccount = "account"
	grpOther   = "other"
)

const rootLong = `legion rents GPUs on Ambient Legion (Ambient GPU Rentals): find an offer,
start a rental, SSH in, and stop billing when you are done.

Quickstart:
  legion auth login                     # sign in through your browser
  legion offer list --gpu-model "RTX 4090"      # case-insensitive substring of the model name
  legion rental create --offer <host-id> --catalog <id>
  legion ssh <rental>                   # connect;  legion rental stop <rental>  to stop billing

Settings come from flags, then environment, then the profile, then defaults:
see 'legion help environment'. Exit codes: 'legion help exit-codes'.`

// Root is the command tree without a running app: for docs generation.
func Root() *cobra.Command { return (&app{}).root() }

func (a *app) root() *cobra.Command {
	root := &cobra.Command{
		Use:           "legion",
		Short:         "Rent GPUs on Ambient Legion",
		Long:          rootLong,
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.ArbitraryArgs,
		RunE:          func(cmd *cobra.Command, args []string) error { return a.unknown(cmd, args) },
		// Reject a bad -o before anything is sent: `rental create -o bogus`
		// must not bill.
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			a.cmdPath = strings.TrimPrefix(cmd.CommandPath(), "legion ")
			switch strings.Fields(a.cmdPath + " ")[0] {
			case "config", "profile", "help", "completion", "__complete", "version", "update":
				return nil
			}
			_, err := a.format()
			return err
		},
	}
	root.CompletionOptions.DisableDefaultCmd = true
	root.AddGroup(
		&cobra.Group{ID: grpStart, Title: "Getting started:"},
		&cobra.Group{ID: grpCompute, Title: "Compute:"},
		&cobra.Group{ID: grpAccount, Title: "Account:"},
		&cobra.Group{ID: grpOther, Title: "Other:"},
	)
	root.SetHelpCommandGroupID(grpOther)
	root.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		a.cmdPath = strings.TrimPrefix(cmd.CommandPath(), "legion ")
		return usagef("legion "+a.cmdPath+" --help", "%v", err)
	})

	f := root.PersistentFlags()
	f.StringVarP(&a.flagProfile, "profile", "p", "", "profile to use (env LEGION_PROFILE)")
	f.StringVar(&a.flagURL, "api-url", "", "API base URL (env LEGION_API_URL)")
	f.StringVarP(&a.flagOut, "output", "o", "", "output format: "+output.Formats+" (env LEGION_OUTPUT)")
	f.BoolVar(&a.noColor, "no-color", false, "disable colour (also NO_COLOR)")
	f.BoolVarP(&a.quiet, "quiet", "q", false, "print only results, no progress or next-step hints")
	f.BoolVar(&a.debug, "debug", false, "log each request's method, path, status and duration to stderr")
	f.DurationVar(&a.timeout, "timeout", 30*time.Second, "timeout for each API request")
	f.BoolVarP(&a.yes, "yes", "y", false, "answer yes to confirmation prompts")

	add := func(group string, cmds ...*cobra.Command) {
		for _, c := range cmds {
			c.GroupID = group
			root.AddCommand(c)
		}
	}
	ssh := a.rentalSSHCmd(true)
	ssh.Use, ssh.Short = "ssh <rental> [-- command...]", "SSH into a rental (shortcut for 'legion rental ssh')"
	add(grpStart, a.authCmd(), a.whoamiCmd(), a.offerCmd(), a.catalogCmd(), ssh)
	add(grpCompute, a.rentalCmd())
	add(grpAccount, a.sshKeyCmd(), a.billingCmd(), a.apiKeyCmd())
	add(grpOther, a.configCmd(), a.profileCmd(), a.versionCmd(), a.updateCmd(), a.completionCmd())
	root.AddCommand(topic("environment", "How flags, environment variables and profiles combine", environmentHelp))
	root.AddCommand(topic("exit-codes", "What legion's exit codes mean", exitCodesHelp))
	a.wireCompletions(root)
	return root
}

// unknown handles `legion <typo>`: a usage error with suggestions, or help.
func (a *app) unknown(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return cmd.Help()
	}
	return unknownErr(cmd, args[0])
}

func unknownErr(cmd *cobra.Command, arg string) error {
	cmd.SuggestionsMinimumDistance = 2
	msg := fmt.Sprintf("unknown command %q for %q", arg, cmd.CommandPath())
	if s := cmd.SuggestionsFor(arg); len(s) > 0 {
		msg += ". Did you mean: " + strings.Join(s, ", ") + "?"
	}
	hint := "legion help"
	if cmd.HasParent() {
		hint = "legion " + strings.TrimPrefix(cmd.CommandPath(), "legion ") + " --help"
	}
	return usagef(hint, "%s", msg)
}

// group makes a parent command: bare it prints help, a typo suggests.
func group(cmd *cobra.Command) *cobra.Command {
	cmd.Args = cobra.ArbitraryArgs
	cmd.RunE = func(c *cobra.Command, args []string) error {
		if len(args) > 0 {
			return unknownErr(c, args[0])
		}
		return c.Help()
	}
	return cmd
}

// topic is a help-only page: `legion help <name>`.
func topic(name, short, long string) *cobra.Command {
	return &cobra.Command{Use: name, Short: short, Long: long}
}

// args checks positional arguments; the error names the missing <arg> from Use.
func args(min, max int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, got []string) error {
		hint := "legion " + strings.TrimPrefix(cmd.CommandPath(), "legion ") + " --help"
		if len(got) < min {
			var names []string
			for _, w := range strings.Fields(cmd.Use)[1:] {
				if strings.HasPrefix(w, "<") {
					names = append(names, w)
				}
			}
			missing := "missing required argument"
			if len(got) < len(names) {
				missing += " " + names[len(got)]
			}
			return usagef(hint, "%s", missing)
		}
		if max >= 0 && len(got) > max {
			return usagef(hint, "too many arguments: %q", strings.Join(got[max:], " "))
		}
		return nil
	}
}

func (a *app) versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the legion version",
		Long:  "Print the version, commit and build date of this legion binary.",
		Example: `  legion version
  legion version -o json`,
		Args: args(0, 0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			v := map[string]string{"version": a.env.Version, "commit": a.env.Commit, "date": a.env.Date}
			return a.emit(output.Result{Data: v, Human: func(w io.Writer) {
				fmt.Fprintf(w, "legion version %s (commit %s, built %s)\n", a.env.Version, a.env.Commit, a.env.Date)
			}})
		},
	}
}

const environmentHelp = `Settings are resolved in this order, first match wins:

  1. command-line flag
  2. environment variable
  3. the selected profile (config.yaml / credentials.yaml)
  4. built-in default

  Setting      Flag           Environment       Profile key      Default
  profile      -p/--profile   LEGION_PROFILE    current_profile  default
  API URL      --api-url      LEGION_API_URL    api_url          https://api.legion.ambient.xyz
  output       -o/--output    LEGION_OUTPUT     output           table
  API key      (none)         LEGION_API_KEY    credentials.yaml (log in with: legion auth login)

The API URL must be https (http only for localhost, 127.0.0.1 and [::1]).
See the effective values with 'legion auth status' and 'legion profile list'.

The API key has no flag so it never appears in the process list.

Files live in $LEGION_CONFIG_DIR, else $XDG_CONFIG_HOME/legion, else your user
config directory (for example ~/.config/legion):
  config.yaml        current_profile and, per profile, api_url and output
  credentials.yaml   API keys, mode 0600 (the directory is 0700)

Colour is off with --no-color, NO_COLOR, TERM=dumb or when output is not a terminal.

On a terminal legion says, at most once a day, when a newer release exists
(update with 'legion update'). LEGION_NO_UPDATE_CHECK=1, or CI set, turns that off.
Progress and prompts go to stderr, results to stdout.`

const exitCodesHelp = `legion exits with:

    0  success
    1  error (the API or the network failed, the account is suspended or a feature is
       turned off for it, or anything not listed below)
    2  usage error (bad flag, missing argument, a request the API rejected as invalid,
       nothing to confirm with and no --yes)
    3  authentication failed (not logged in, bad, expired or under-scoped API key)
    4  not found (rental, offer, key, backup ...)
    5  conflict or price changed (the host's price rose above --max-price, GPUs just taken)
    6  insufficient credit (check: legion billing balance)
  130  interrupted (Ctrl-C)

On failure legion prints three lines to stderr: Error, Code (the API's stable
error code, when there is one) and Hint (the next command to run).`
