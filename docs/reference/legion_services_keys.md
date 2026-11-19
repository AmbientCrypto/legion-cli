## legion services keys

Manage service keys

### Synopsis

Service keys (lek_...) let callers reach a service, or a protected port. One key belongs to one service.
The secret is shown once, when the key is created or rotated, and is stored only as a hash. Keys expire after
90 days unless you pick 7d, 30d, 365d or never. Revoking takes effect within seconds.

```
legion services keys [flags]
```

### Examples

```
  legion services keys list chat
  legion services keys create chat --name ci --expires 30d
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

* [legion services](legion_services.md)	 - Give what you run a public URL, with optional service keys
* [legion services keys create](legion_services_keys_create.md)	 - Create a service key
* [legion services keys list](legion_services_keys_list.md)	 - List a service's keys
* [legion services keys revoke](legion_services_keys_revoke.md)	 - Revoke a key
* [legion services keys rotate](legion_services_keys_rotate.md)	 - Replace a key with a new one, keeping the old one for a while

