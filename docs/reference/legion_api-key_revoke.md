## legion api-key revoke

Revoke an API key

### Synopsis

Revoke a key by name, prefix or id. Anything using it stops working at once. Revoking the key legion is logged in with signs you out.

```
legion api-key revoke <key> [flags]
```

### Examples

```
  legion api-key revoke ci
  legion api-key revoke lgn_3Fq9 --yes
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

* [legion api-key](legion_api-key.md)	 - Manage the API keys that sign in to Legion

