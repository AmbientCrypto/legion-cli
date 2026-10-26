## legion api-key list

List API keys

### Synopsis

List the organization's API keys by prefix, scope and last use. Secrets are never shown after creation.

```
legion api-key list [flags]
```

### Examples

```
  legion api-key list
  legion api-key list -o json
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

* [legion api-key](legion_api-key.md)	 - Manage the API keys that sign in to Legion

