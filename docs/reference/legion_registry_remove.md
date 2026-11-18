## legion registry remove

Remove a registry credential

### Synopsis

Delete a credential and its token. Refused while rentals that may still pull with it exist; stop or delete them first. Find ids with: legion registry list -o wide.

```
legion registry remove <id> [flags]
```

### Examples

```
  legion registry remove 6f1c... --yes
  legion registry remove 6f1c...
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

* [legion registry](legion_registry.md)	 - Save registry credentials to rent private images

