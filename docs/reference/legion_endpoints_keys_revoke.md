## legion endpoints keys revoke

Revoke a key

### Synopsis

Revoke a key by id, name or prefix. Callers using it get 401 within seconds. Revoking a legacy direct key removes direct access to the rental; the endpoint URL keeps working.

```
legion endpoints keys revoke <endpoint> <key> [flags]
```

### Examples

```
  legion endpoints keys revoke chat ci
  legion endpoints keys revoke chat ci --yes
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

* [legion endpoints keys](legion_endpoints_keys.md)	 - Manage an endpoint's access keys

