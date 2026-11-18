## legion registry

Save registry credentials to rent private images

### Synopsis

A registry credential is a username and a read-only token for a container registry. Your
organization's rentals use the one whose host and path prefix best match the image (the longest
prefix), or the one you pick with legion rental create --registry-credential. The token is kept
encrypted, is never shown again, and is never sent to a host: a host gets a short-lived token from
the registry for one repository.

Any registry with token authentication works (Docker Hub, ghcr.io, GitLab, Quay, NGC, Harbor,
Artifactory, Artifact Registry, Azure). AWS ECR is not supported yet: its passwords are 12-hour
tokens made from AWS, so a host could only pull with a long-lived secret. Mirror the image to
another registry. Adding or replacing needs the registry.manage permission (owners have it).

```
legion registry [flags]
```

### Examples

```
  echo "$GHCR_TOKEN" | legion registry add --image ghcr.io/acme/model:v3 --username acme-bot --token-stdin
  legion registry list
```

### Options

```
  -h, --help   help for registry
```

### Options inherited from parent commands

```
      --api-url string     API base URL (env LEGION_API_URL)
      --debug              log each request's method, path, status and duration to stderr
      --no-color           disable colour (also NO_COLOR)
  -o, --output string      output format: table|wide|json|yaml|name|jsonpath=<expr> (env LEGION_OUTPUT)
  -p, --profile string     profile to use (env LEGION_PROFILE)
  -q, --quiet              print only results, no progress or next-step hints
      --timeout duration   timeout for each API request (default 30s)
  -y, --yes                answer yes to confirmation prompts
```

### SEE ALSO

* [legion](legion.md)	 - Rent GPUs on Ambient Legion
* [legion registry add](legion_registry_add.md)	 - Save a registry credential
* [legion registry list](legion_registry_list.md)	 - List saved registry credentials
* [legion registry remove](legion_registry_remove.md)	 - Remove a registry credential
* [legion registry replace](legion_registry_replace.md)	 - Replace a credential's token, or rename it

