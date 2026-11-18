## legion registry add

Save a registry credential

### Synopsis

Save a username and a read-only token for the registry of --image. Legion checks the
credential against that image before it saves it. The host comes from the image; the path prefix
defaults to the image's namespace (acme for ghcr.io/acme/model), so another credential can cover
another namespace on the same host. Use --path-prefix "" for every image on the host.

The token is read from a hidden prompt, or from stdin with --token-stdin. It is never a flag.

```
legion registry add [flags]
```

### Examples

```
  echo "$TOKEN" | legion registry add --image ghcr.io/acme/model:v3 --username acme-bot --token-stdin
  legion registry add --image nvcr.io/acme/model:1 --username '$oauthtoken' --name ngc
```

### Options

```
  -h, --help                 help for add
      --image string         an image the credential is for; its registry host is the credential's host
      --name string          a name for it, shown as "Uses: <name> on <host>"
      --path-prefix string   which images use it: whole path segments (default: the image's namespace; "" for every image on the host)
      --token-stdin          read the token from stdin instead of a hidden prompt
      --username string      the registry username ($oauthtoken on nvcr.io, a robot name on quay.io)
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

* [legion registry](legion_registry.md)	 - Save registry credentials to rent private images

