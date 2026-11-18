package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/AmbientCrypto/legion-controller/api"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

// ---- registry (LGN-623) ----

// readSecretInput returns the secret from stdin (--token-stdin) or the hidden
// prompt. It never echoes or prints it.
func (a *app) readSecretInput(cmd *cobra.Command, fromStdin bool, helpCmd string) (string, error) {
	var s string
	switch {
	case fromStdin:
		b, err := io.ReadAll(a.env.Stdin)
		if err != nil {
			return "", err
		}
		s = string(b)
	case a.interactive():
		fmt.Fprint(a.env.Stderr, "Token (hidden: nothing is shown), then Enter: ")
		v, err := a.env.ReadSecret(a.ctx(cmd))
		if err != nil {
			return "", err
		}
		s = v
	default:
		return "", usagef("echo $TOKEN | legion "+helpCmd+" --token-stdin", "the token needs a terminal or --token-stdin")
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return "", usagef("legion "+helpCmd+" --help", "no token given")
	}
	return s, nil
}

func parseCredentialID(s string) (uuid.UUID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return id, usagef("legion registry list", "%q is not a credential id", s)
	}
	return id, nil
}

func credRow(c api.RegistryCredential) []string {
	by := "-"
	if c.CreatedBy != nil {
		by = c.CreatedBy.String()
	}
	prefix := c.PathPrefix
	if prefix == "" {
		prefix = "(all)"
	}
	return []string{ptrOr(c.Name, ""), c.Host, prefix, c.Username, tsp(c.LastUsedAt), string(c.LastCheckResult), c.Id.String(), by, ts(c.CreatedAt)}
}

var credCols = []string{"NAME", "HOST", "PREFIX", "USERNAME", "LAST USED", "CHECK"}
var credWide = []string{"ID", "CREATED BY", "CREATED"}

func (a *app) emitCredential(c api.RegistryCredential) error {
	res := listResult([]api.RegistryCredential{c}, credCols, credWide, func(c api.RegistryCredential) string { return c.Id.String() }, credRow)
	res.Data = c
	return a.emit(res)
}

func credLabel(c api.RegistryCredential) string {
	prefix := c.Host
	if c.PathPrefix != "" {
		prefix += "/" + c.PathPrefix
	}
	if c.Name != nil && *c.Name != "" {
		return fmt.Sprintf("%s on %s", san(*c.Name), san(prefix))
	}
	return san(prefix)
}

func (a *app) registryCmd() *cobra.Command {
	cmd := group(&cobra.Command{
		Use:     "registry",
		Aliases: []string{"registries"},
		Short:   "Save registry credentials to rent private images",
		Long: `A registry credential is a username and a read-only token for a container registry. Your
organization's rentals use the one whose host and path prefix best match the image (the longest
prefix), or the one you pick with legion rental create --registry-credential. The token is kept
encrypted, is never shown again, and is never sent to a host: a host gets a short-lived token from
the registry for one repository.

Any registry with token authentication works (Docker Hub, ghcr.io, GitLab, Quay, NGC, Harbor,
Artifactory, Artifact Registry, Azure). AWS ECR is not supported yet: its passwords are 12-hour
tokens made from AWS, so a host could only pull with a long-lived secret. Mirror the image to
another registry. Adding or replacing needs the registry.manage permission (owners have it).`,
		Example: `  echo "$GHCR_TOKEN" | legion registry add --image ghcr.io/acme/model:v3 --username acme-bot --token-stdin
  legion registry list`,
	})

	list := &cobra.Command{
		Use:     "list",
		Short:   "List saved registry credentials",
		Long:    "List the organization's registry credentials: host, path prefix, username, when a host last pulled with it and how its last check went. The token is never shown.",
		Example: "  legion registry list\n  legion registry list -o json",
		Args:    args(0, 0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := a.api()
			if err != nil {
				return err
			}
			resp, err := c.ListRegistryCredentialsWithResponse(a.ctx(cmd))
			if err := a.ok(resp, err); err != nil {
				return err
			}
			l, err := need(resp.JSON200)
			if err != nil {
				return err
			}
			if len(l.Items) == 0 {
				a.info("No registry credentials. Add one: legion registry add --image <image> --username <user> --token-stdin")
			}
			return a.emit(listResult(l.Items, credCols, credWide, func(c api.RegistryCredential) string { return c.Id.String() }, credRow))
		},
	}

	var image, username, name, prefix string
	var tokenStdin bool
	add := &cobra.Command{
		Use:   "add",
		Short: "Save a registry credential",
		Long: `Save a username and a read-only token for the registry of --image. Legion checks the
credential against that image before it saves it. The host comes from the image; the path prefix
defaults to the image's namespace (acme for ghcr.io/acme/model), so another credential can cover
another namespace on the same host. Use --path-prefix "" for every image on the host.

The token is read from a hidden prompt, or from stdin with --token-stdin. It is never a flag.`,
		Example: "  echo \"$TOKEN\" | legion registry add --image ghcr.io/acme/model:v3 --username acme-bot --token-stdin\n  legion registry add --image nvcr.io/acme/model:1 --username '$oauthtoken' --name ngc",
		Args:    args(0, 0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if image == "" {
				return usagef("legion registry add --help", "missing required flag --image: an image the credential is for")
			}
			if username == "" {
				if !a.interactive() {
					return usagef("legion registry add --help", "missing required flag --username")
				}
				u, err := a.prompt.Ask("Username: ")
				if err != nil {
					return err
				}
				if username = strings.TrimSpace(u); username == "" {
					return usagef("legion registry add --help", "no username given")
				}
			}
			secret, err := a.readSecretInput(cmd, tokenStdin, "registry add --image "+image+" --username "+username)
			if err != nil {
				return err
			}
			c, err := a.api()
			if err != nil {
				return err
			}
			req := api.CreateRegistryCredentialRequest{Image: image, Username: username, Secret: &secret}
			if name != "" {
				req.Name = &name
			}
			if cmd.Flags().Changed("path-prefix") {
				req.PathPrefix = &prefix
			}
			resp, err := c.CreateRegistryCredentialWithResponse(a.ctx(cmd), req)
			if err := a.ok(resp, err); err != nil {
				return err
			}
			cred, err := need(resp.JSON201)
			if err != nil {
				return err
			}
			a.info("Saved. Rentals of images under %s now use it (Uses: %s).", san(cred.Host+"/"+cred.PathPrefix), credLabel(*cred))
			a.info("Remove: legion registry remove %s", cred.Id)
			return a.emitCredential(*cred)
		},
	}
	add.Flags().StringVar(&image, "image", "", "an image the credential is for; its registry host is the credential's host")
	add.Flags().StringVar(&username, "username", "", "the registry username ($oauthtoken on nvcr.io, a robot name on quay.io)")
	add.Flags().StringVar(&name, "name", "", "a name for it, shown as \"Uses: <name> on <host>\"")
	add.Flags().StringVar(&prefix, "path-prefix", "", "which images use it: whole path segments (default: the image's namespace; \"\" for every image on the host)")
	add.Flags().BoolVar(&tokenStdin, "token-stdin", false, "read the token from stdin instead of a hidden prompt")

	var rImage, rUser, rName string
	var rStdin bool
	replace := &cobra.Command{
		Use:   "replace <id>",
		Short: "Replace a credential's token, or rename it",
		Long: `Replace the token (and optionally the username) of a saved credential, checked against --image
under its host and prefix first, or rename it with --name alone. The host and path prefix never
change: add another credential and remove this one instead. Rentals use the new token from their
next pull.`,
		Example: "  echo \"$TOKEN\" | legion registry replace 6f1c... --image ghcr.io/acme/model:v3 --token-stdin\n  legion registry replace 6f1c... --name \"acme on ghcr\"",
		Args:    args(1, 1),
		RunE: func(cmd *cobra.Command, as []string) error {
			id, err := parseCredentialID(as[0])
			if err != nil {
				return err
			}
			var req api.UpdateRegistryCredentialRequest
			if cmd.Flags().Changed("name") {
				req.Name = &rName
			}
			switch {
			case rStdin:
				if rImage == "" {
					return usagef("legion registry replace --help", "a new token needs --image: an image under the credential's host and prefix to check it against")
				}
				secret, err := a.readSecretInput(cmd, true, "registry replace")
				if err != nil {
					return err
				}
				req.Secret, req.Image = &secret, &rImage
				if rUser != "" {
					req.Username = &rUser
				}
			case rUser != "" || rImage != "":
				return usagef("legion registry replace --help", "--username and --image go with a new token: add --token-stdin")
			case req.Name == nil:
				return usagef("legion registry replace --help", "nothing to change: pass --token-stdin with --image, or --name")
			}
			c, err := a.api()
			if err != nil {
				return err
			}
			resp, err := c.UpdateRegistryCredentialWithResponse(a.ctx(cmd), id, req)
			if err := a.ok(resp, err); err != nil {
				return err
			}
			cred, err := need(resp.JSON200)
			if err != nil {
				return err
			}
			a.info("Saved %s.", credLabel(*cred))
			return a.emitCredential(*cred)
		},
	}
	replace.Flags().StringVar(&rImage, "image", "", "an image under the credential's host and prefix, to check the new token against")
	replace.Flags().StringVar(&rUser, "username", "", "a new username, with --token-stdin")
	replace.Flags().StringVar(&rName, "name", "", "a new name (\"\" clears it)")
	replace.Flags().BoolVar(&rStdin, "token-stdin", false, "read the new token from stdin")

	remove := &cobra.Command{
		Use:     "remove <id>",
		Aliases: []string{"rm"},
		Short:   "Remove a registry credential",
		Long:    "Delete a credential and its token. Refused while rentals that may still pull with it exist; stop or delete them first. Find ids with: legion registry list -o wide.",
		Example: "  legion registry remove 6f1c... --yes\n  legion registry remove 6f1c...",
		Args:    args(1, 1),
		RunE: func(cmd *cobra.Command, as []string) error {
			id, err := parseCredentialID(as[0])
			if err != nil {
				return err
			}
			c, err := a.api()
			if err != nil {
				return err
			}
			ctx := a.ctx(cmd)
			label := id.String()
			if resp, err := c.GetRegistryCredentialWithResponse(ctx, id); err == nil && resp.JSON200 != nil {
				label = credLabel(*resp.JSON200)
			}
			if err := a.confirm(fmt.Sprintf("Remove the credential %s? Rentals that have to pull their image again can no longer do it.", label),
				"refusing to remove a registry credential without confirmation"); err != nil {
				return err
			}
			return a.removeCredential(ctx, c, id)
		},
	}
	cmd.AddCommand(list, add, replace, remove)
	return cmd
}

func (a *app) removeCredential(ctx context.Context, c *api.ClientWithResponses, id uuid.UUID) error {
	resp, err := c.DeleteRegistryCredentialWithResponse(ctx, id)
	if err := a.ok(resp, err); err != nil {
		return err
	}
	a.info("Removed %s.", id)
	return nil
}
