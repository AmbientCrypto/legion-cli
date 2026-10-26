package cli

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/AmbientCrypto/legion-cli/internal/client"
	"github.com/AmbientCrypto/legion-cli/internal/output"
)

// cliError is an error the CLI words itself: message, stable code, next step.
type cliError struct {
	Msg, Code, Hint string
	Exit            int
}

func (e *cliError) Error() string { return e.Msg }

func usagef(hint, format string, a ...any) error {
	return &cliError{Msg: fmt.Sprintf(format, a...), Hint: hint, Exit: client.ExitUsage}
}

func notFoundf(hint, format string, a ...any) error {
	return &cliError{Msg: fmt.Sprintf(format, a...), Code: "not_found", Hint: hint, Exit: client.ExitNotFound}
}

func errorf(hint, format string, a ...any) error {
	return &cliError{Msg: fmt.Sprintf(format, a...), Hint: hint, Exit: client.ExitError}
}

var errNotLoggedIn = &cliError{Msg: "not logged in", Code: "not_logged_in", Hint: "legion auth login", Exit: client.ExitAuth}

// Described is an error ready to print: Error/Code/Hint and the exit code.
type Described struct {
	Message, Code, Hint string
	Exit                int
}

// Describe turns any error into the three lines the CLI prints. cmdPath is
// the command that failed ("rental get"); it picks the right list command
// for not-found hints.
func Describe(err error, cmdPath string) Described {
	var ce *cliError
	if errors.As(err, &ce) {
		return Described{ce.Msg, ce.Code, ce.Hint, ce.Exit}
	}
	var e *client.Error
	if errors.As(err, &e) {
		return describeAPI(e, cmdPath)
	}
	if errors.Is(err, context.Canceled) {
		return Described{Message: "interrupted", Exit: client.ExitInterrupted}
	}
	var ue *url.Error
	if errors.As(err, &ue) {
		return Described{Message: err.Error(), Code: "network_error", Hint: "Check your connection, or the API URL (legion config get api_url)", Exit: client.ExitError}
	}
	msg := err.Error()
	for _, p := range []string{"unknown command", "unknown flag", "unknown shorthand flag", "flag needs an argument", "invalid argument", "required flag", "accepts ", "invalid command"} {
		if strings.HasPrefix(msg, p) {
			hint := "legion help"
			if cmdPath != "" && !strings.HasPrefix(msg, "unknown command") {
				hint = "legion " + cmdPath + " --help"
			}
			return Described{Message: strings.Join(strings.Fields(strings.ReplaceAll(msg, "\n\t", " / ")), " "), Hint: hint, Exit: client.ExitUsage}
		}
	}
	return Described{Message: msg, Exit: client.ExitError}
}

func describeAPI(e *client.Error, path string) Described {
	d := Described{Message: e.Error(), Code: e.Code, Exit: e.ExitCode()}
	switch {
	case e.Code == "api_key_scope":
		d.Hint = "This API key cannot do that. Create a key with scope full and log in again: legion auth login"
	case e.Status == 401 || e.Code == "unauthorized" || e.Code == "api_key_required":
		d.Hint = "legion auth login"
	case e.Code == "insufficient_credit" || e.Status == 402:
		d.Hint = "Check your balance with: legion billing balance. To add credit, open Billing in the web console (https://legion.ambient.xyz) or ask your Ambient contact for credit."
	case e.Code == "price_changed":
		if e.CurrentPrice != nil {
			d.Message = fmt.Sprintf("The price changed: this host now charges %s per GPU-hour, above the maximum you agreed to.", output.Money(e.CurrentPrice.AmountMinor, string(e.CurrentPrice.Currency)))
		}
		d.Hint = "Re-run the command to review the new price (add --max-price to accept a higher one)."
	case e.Code == "gpus_unavailable" || e.Code == "gpus_interruptible":
		d.Hint = "Those GPUs were just taken. Find another host: legion offer list"
	case e.Code == "catalog_entry_incompatible":
		d.Hint = "Pick a host that can run it: legion offer list --catalog <id>"
	case e.Code == "org_suspended":
		d.Hint = "Your organization is suspended. Contact Ambient support."
	case e.Code == "rate_limited" || e.Status == 429:
		d.Hint = "Wait a moment and retry."
	case e.Status == 404:
		d.Hint = listHint(path)
	case e.Status == 400 || e.Status == 422:
		d.Hint = "Check the values you passed: legion " + path + " --help"
	}
	return d
}

func listHint(path string) string {
	words := strings.Fields(path)
	if len(words) == 0 {
		return "legion rental list"
	}
	switch {
	case strings.Contains(path, "ingress"):
		return "legion rental ingress list <rental>"
	case strings.Contains(path, "backup"):
		return "legion rental backup list"
	case strings.Contains(path, "export"):
		return "legion rental export list <rental>"
	}
	switch words[0] {
	case "offer", "catalog", "ssh-key", "api-key":
		return "legion " + words[0] + " list"
	}
	return "legion rental list"
}
