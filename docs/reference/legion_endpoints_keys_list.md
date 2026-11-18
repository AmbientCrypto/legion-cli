## legion endpoints keys list

List an endpoint's keys

### Synopsis

List keys with prefix, status, expiry and last use. Secrets and hashes are never shown. A key marked legacy is a direct key of a rental created before endpoints; its direct URL is in -o wide.

```
legion endpoints keys list <endpoint> [flags]
```

### Examples

```
  legion endpoints keys list chat
  legion endpoints keys list chat -o wide
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

* [legion endpoints keys](legion_endpoints_keys.md)	 - Manage an endpoint's access keys

