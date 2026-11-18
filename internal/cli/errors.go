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
	After           string // printed after the Hint (a preview of what was not done)
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
	Message, Code, Hint, After string
	Exit                       int
}

// Describe turns any error into the three lines the CLI prints. cmdPath is
// the command that failed ("rental get"); it picks the right list command
// for not-found hints.
func Describe(err error, cmdPath string) Described {
	d := describe(err, cmdPath)
	d.Message, d.Code, d.Hint = output.Sanitize(d.Message), output.Sanitize(d.Code), output.Sanitize(d.Hint)
	return d
}

func describe(err error, cmdPath string) Described {
	var ce *cliError
	if errors.As(err, &ce) {
		return Described{ce.Msg, ce.Code, ce.Hint, ce.After, ce.Exit}
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
	if e.Status == 401 {
		d.Message = "Your API key is invalid, expired or revoked."
	}
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
	case e.Code == "linked_host":
		d.Message = "Your organization cannot rent this host."
		d.Hint = "Find another host: legion offer list"
	case e.Code == "own_host":
		d.Message = "You cannot bid on a host your organization owns."
		d.Hint = "Rent it for your own use at Ambient's fee: legion rental create --offer <host>"
	case e.Code == "gpus_unavailable" || e.Code == "gpus_interruptible":
		d.Hint = "Those GPUs were just taken. Find another host: legion offer list"
	case e.Code == "catalog_entry_incompatible":
		d.Hint = "Pick a host that can run it: legion offer list --catalog <id>"
	case e.Code == "org_suspended":
		d.Hint = "Your organization is suspended. Contact Ambient support."
	case e.Code == "custom_images_disabled":
		d.Hint = "Custom images are turned off for your organization. Use a catalog image: legion catalog list"
	case e.Code == "catalog_entry_disabled":
		d.Hint = "That catalog entry is turned off. Pick another: legion catalog list"
	case e.Code == "access_not_approved" || e.Code == "terms_not_accepted" || e.Code == "email_unverified":
		d.Hint = "Finish setting up your account in the web console: https://legion.ambient.xyz"
	case registryProblem(e, &d):
	case e.Code == "link_used" || e.Status == 410:
		d.Message = "This download link was already used or has expired."
		if strings.Contains(path, "backup") {
			d.Hint = "Make a new link: legion rental backup download-link <backup-id>"
		} else {
			d.Hint = "Make a new link: legion rental export link <rental> <export-id>"
		}
	case e.Code == "export_not_ready":
		d.Hint = "Wait until it is ready: legion rental export get <rental> <export-id>"
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

// registryProblem words the registry credential refusals (LGN-623). It
// reports whether it handled e; the secret is never part of any of them.
func registryProblem(e *client.Error, d *Described) bool {
	host := e.RegistryHost
	if host == "" {
		host = "the registry"
	}
	switch e.Code {
	case "registry_ecr_unsupported":
		d.Message = "AWS ECR is not supported yet. ECR passwords last 12 hours and are made from AWS, so the only way for a host to pull the image would be with a long-lived AWS secret, which Legion never gives a host."
		d.Hint = "Use a registry that issues pull tokens (Docker Hub, ghcr.io, GitLab, Quay, NGC, Harbor, Artifact Registry or Azure), or mirror the image to one."
	case "registry_auth_unsupported":
		d.Message = host + " does not use a token service (it asks for a plain username and password), so Legion cannot give a host a short-lived token for it."
		d.Hint = "Use a registry with token authentication, or mirror the image to one."
	case "registry_host_not_allowed":
		d.Message = host + " is not a registry Legion sends credentials to. Addresses, local names and non-standard ports are refused."
		d.Hint = "Use the registry's public host name on the standard port, or mirror the image to another registry."
	case "registry_realm_not_allowed":
		d.Message = host + " points to a token service on another host, so Legion did not send the credential there. Nothing was saved."
		d.Hint = "Use a registry whose token service is on its own host."
	case "registry_credential_rejected":
		d.Message = host + " refused this credential, or it cannot read that image. Nothing was saved."
		d.Hint = "Check the username and the token, and that the token can read the repository."
	case "image_outside_prefix":
		d.Hint = "Pass an image under the path prefix, or change --path-prefix."
	case "registry_credential_exists":
		d.Message = "A credential for that host and path prefix already exists."
		if e.ResourceID != "" {
			d.Hint = "Replace its secret: legion registry replace " + e.ResourceID + " --image <image> --token-stdin"
		} else {
			d.Hint = "See it with: legion registry list"
		}
	case "registry_credential_limit":
		d.Message = "Your organization already has 20 registry credentials."
		d.Hint = "Remove one you no longer need: legion registry remove <id>"
	case "registry_credential_in_use":
		d.Message = "Rentals still use this credential."
		if len(e.Rentals) > 0 {
			var parts []string
			for _, r := range e.Rentals {
				parts = append(parts, output.Sanitize(r.Name)+" ("+string(r.State)+")")
			}
			d.Message += " " + strings.Join(parts, ", ") + "."
		}
		d.Hint = "Stop or delete those rentals first: legion rental stop <rental>"
	case "registry_credential_not_found":
		d.Hint = "See your credentials with: legion registry list"
	case "registry_credential_host_mismatch":
		d.Hint = "That credential is for another host. Pick one for the image's host: legion registry list"
	case "image_private":
		d.Hint = "Save a read-only credential for the registry, then rent again: legion registry add --image <image>"
	case "registry_credentials_unavailable":
		d.Hint = "Registry credentials are not available right now. Try again later."
	case "image_not_found", "image_registry_unavailable":
		return false
	default:
		return false
	}
	return true
}
