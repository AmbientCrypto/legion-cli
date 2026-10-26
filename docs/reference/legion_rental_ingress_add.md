## legion rental ingress add

Expose a port

### Synopsis

Publish the HTTP service listening on --port inside the rental. Without --source anyone on the
internet can reach it; add --source <cidr> (repeatable, up to 16) to limit it.

```
legion rental ingress add <rental> [flags]
```

### Examples

```
  legion rental ingress add quiet-river-7k2f --port 8000
  legion rental ingress add quiet-river-7k2f --port 8000 --source 203.0.113.0/24 --description api
```

### Options

```
      --description string   a note shown with the rule
  -h, --help                 help for add
      --port int             port the service listens on in the rental (not 22 or 22022)
      --source stringArray   allowed source CIDR, e.g. 203.0.113.0/24 (repeatable; default anyone)
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

