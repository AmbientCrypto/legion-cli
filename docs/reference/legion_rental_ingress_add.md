## legion rental ingress add

Deprecated: expose a port

### Synopsis

Deprecated: same as 'legion endpoints create --rental <rental> --port <port> --access public'. Without --source
anyone on the internet can reach the port; add --source <cidr> (repeatable) to limit it. You are asked to confirm
(or pass --yes).

```
legion rental ingress add <rental> [flags]
```

### Examples

```
  legion rental ingress add quiet-river-7k2f --port 8000
  legion rental ingress add quiet-river-7k2f --port 8000 --source 203.0.113.0/24
```

### Options

```
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

* [legion rental ingress](legion_rental_ingress.md)	 - Deprecated: use 'legion endpoints' to expose a port

