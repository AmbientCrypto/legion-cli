## legion registry list

List saved registry credentials

### Synopsis

List the organization's registry credentials: host, path prefix, username, when a host last pulled with it and how its last check went. The token is never shown.

```
legion registry list [flags]
```

### Examples

```
  legion registry list
  legion registry list -o json
```

### Options

```
  -h, --help   help for list
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

