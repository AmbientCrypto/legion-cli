## legion rental ingress remove

Stop exposing a port

### Synopsis

Delete an ingress rule. The public URL stops answering at once. Find ids with: legion rental ingress list -o wide.

```
legion rental ingress remove <rental> <rule-id> [flags]
```

### Examples

```
  legion rental ingress remove quiet-river-7k2f 6f1c... --yes
  legion rental ingress remove quiet 6f1c...
```

### Options

```
  -h, --help   help for remove
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

* [legion rental ingress](legion_rental_ingress.md)	 - Expose a port of a rental on the internet

