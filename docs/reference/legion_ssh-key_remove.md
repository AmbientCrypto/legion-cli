## legion ssh-key remove

Forget a saved SSH key

### Synopsis

Remove a saved key by label, fingerprint or id. Rentals already created with it keep working.

```
legion ssh-key remove <key> [flags]
```

### Examples

```
  legion ssh-key remove laptop
  legion ssh-key remove SHA256:abc... --yes
```

### Options

```
  -h, --help   help for remove
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

* [legion ssh-key](legion_ssh-key.md)	 - Save the SSH public keys you rent with

