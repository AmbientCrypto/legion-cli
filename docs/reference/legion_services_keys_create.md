## legion services keys create

Create a service key

### Synopsis

Create a key. The secret is shown ONCE, here, and cannot be retrieved later: copy it now. --expires takes 7d,
30d, 90d, 365d or never (default 90d); a key that never expires may need step-up verification. At most 10 active
keys per service. -o name prints only the secret.

```
legion services keys create <service> [flags]
```

### Examples

```
  legion services keys create chat --name ci --expires 30d
  legion services keys create chat --name bot -q -o name
```

### Options

```
      --expires string   7d, 30d, 90d, 365d or never (default 90d)
  -h, --help             help for create
      --name string      a name for the key, for example ci
      --rate-limit int   requests per minute this key may make (default: the service's)
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

