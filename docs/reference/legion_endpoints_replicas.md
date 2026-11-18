## legion endpoints replicas

List an endpoint's replicas

### Synopsis

List the replicas (rentals) serving the endpoint, live and warm. --all adds the replicas retired in the last 7 days.

```
legion endpoints replicas <endpoint> [flags]
```

### Examples

```
  legion endpoints replicas chat
  legion endpoints replicas chat --all -o wide
```

### Options

```
      --all    include replicas retired in the last 7 days
  -h, --help   help for replicas
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

