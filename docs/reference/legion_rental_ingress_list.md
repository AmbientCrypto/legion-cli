## legion rental ingress list

List a rental's exposed ports

### Synopsis

List the ingress rules of a rental with their public URL and status.

```
legion rental ingress list <rental> [flags]
```

### Examples

```
  legion rental ingress list quiet-river-7k2f
  legion rental ingress list quiet -o json
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

* [legion rental ingress](legion_rental_ingress.md)	 - Expose a port of a rental on the internet

