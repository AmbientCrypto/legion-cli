## legion services keys revoke

Revoke a key

### Synopsis

Revoke a key by id, name or prefix. Callers using it get 401 within seconds. Revoking a legacy direct key removes direct access to the rental; the service URL keeps working.

```
legion services keys revoke <service> <key> [flags]
```

### Examples

```
  legion services keys revoke chat ci
  legion services keys revoke chat ci --yes
```

### Options

```
  -h, --help   help for revoke
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

