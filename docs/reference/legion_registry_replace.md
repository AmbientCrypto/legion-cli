## legion registry replace

Replace a credential's token, or rename it

### Synopsis

Replace the token (and optionally the username) of a saved credential, checked against --image
under its host and prefix first, or rename it with --name alone. The host and path prefix never
change: add another credential and remove this one instead. Rentals use the new token from their
next pull.

```
legion registry replace <id> [flags]
```

### Examples

```
  echo "$TOKEN" | legion registry replace 6f1c... --image ghcr.io/acme/model:v3 --token-stdin
  legion registry replace 6f1c... --name "acme on ghcr"
```

### Options

```
  -h, --help              help for replace
      --image string      an image under the credential's host and prefix, to check the new token against
      --name string       a new name ("" clears it)
      --token-stdin       read the new token from stdin
      --username string   a new username, with --token-stdin
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

