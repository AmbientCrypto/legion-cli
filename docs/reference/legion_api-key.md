## legion api-key

Manage the API keys that sign in to Legion

### Synopsis

API keys (lgn_...) let scripts and legion itself call the API. A full-scope key can do everything its creator can; an agent-scope key is limited to the renter tools of the MCP server.

```
legion api-key [flags]
```

### Examples

```
  legion api-key list
  legion api-key create ci --scope full --expires 720h
```

### Options

```
  -h, --help   help for api-key
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

* [legion](legion.md)	 - Rent GPUs on Ambient Legion
* [legion api-key create](legion_api-key_create.md)	 - Create an API key
* [legion api-key list](legion_api-key_list.md)	 - List API keys
* [legion api-key revoke](legion_api-key_revoke.md)	 - Revoke an API key

