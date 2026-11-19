## legion services replicas

List a service's replicas

### Synopsis

List the replicas (rentals) serving the service, live and warm. --all adds the replicas retired in the last 7 days.

```
legion services replicas <service> [flags]
```

### Examples

```
  legion services replicas chat
  legion services replicas chat --all -o wide
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

* [legion services](legion_services.md)	 - Give what you run a public URL, with optional service keys

