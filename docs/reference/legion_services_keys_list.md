## legion services keys list

List a service's keys

### Synopsis

List keys with prefix, status, expiry and last use. Secrets and hashes are never shown. A key marked legacy is a direct key of a rental created before services; its direct URL is in -o wide.

```
legion services keys list <service> [flags]
```

### Examples

```
  legion services keys list chat
  legion services keys list chat -o wide
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

* [legion services keys](legion_services_keys.md)	 - Manage service keys

