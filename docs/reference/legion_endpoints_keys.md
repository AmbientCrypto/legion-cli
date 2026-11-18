## legion endpoints keys

Manage an endpoint's access keys

### Synopsis

Access keys (lek_...) let callers reach an endpoint, or a protected port. One key belongs to one endpoint.
The secret is shown once, when the key is created or rotated, and is stored only as a hash. Keys expire after
90 days unless you pick 7d, 30d, 365d or never. Revoking takes effect within seconds.

```
legion endpoints keys [flags]
```

### Examples

```
  legion endpoints keys list chat
  legion endpoints keys create chat --name ci --expires 30d
```

### Options

```
  -h, --help   help for keys
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

* [legion endpoints](legion_endpoints.md)	 - Serve models and ports behind one URL with access keys
* [legion endpoints keys create](legion_endpoints_keys_create.md)	 - Create an access key
* [legion endpoints keys list](legion_endpoints_keys_list.md)	 - List an endpoint's keys
* [legion endpoints keys revoke](legion_endpoints_keys_revoke.md)	 - Revoke a key
* [legion endpoints keys rotate](legion_endpoints_keys_rotate.md)	 - Replace a key with a new one, keeping the old one for a while

