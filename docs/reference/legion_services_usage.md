## legion services usage

Show requests, errors, replica-hours and cost per day

### Synopsis

Show the service's usage per day (requests, errors, replica-hours, cost) and this month's spend against the monthly cap. Request-rate and latency points for the --window are in -o json.

```
legion services usage <service> [flags]
```

### Examples

```
  legion services usage chat
  legion services usage chat --days 7 -o json
```

### Options

```
      --days int        how many days back (default 30)
  -h, --help            help for usage
      --window string   traffic window for the request-rate points in -o json: 1h, 24h or 7d
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

