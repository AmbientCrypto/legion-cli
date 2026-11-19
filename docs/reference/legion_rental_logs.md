## legion rental logs

Show a rental's logs, or follow them live

### Synopsis

Lines come oldest first: the container's output (stdout, stderr), for a Legion model image also the
model server (model_server) and Legion's download and start log (legion_serve), and Legion's own events
about the rental (legion: image pulled, container started, health check passed, stopped, failed).
Without --follow it prints the newest lines of the range (the last hour by default). With --follow it
keeps printing new lines every 2 seconds until Ctrl-C. Logs are kept 14 days; values of the rental's
environment variables and secrets are shown as [redacted].

```
legion rental logs <rental> [flags]
```

### Examples

```
  legion rental logs quiet-river-7k2f
  legion rental logs quiet-river-7k2f -f --source stdout,stderr
  legion rental logs quiet-river-7k2f --since 6h --level warn --grep "out of memory"
  legion rental logs quiet-river-7k2f --json | jq -r .message
```

### Options

```
  -f, --follow           keep printing new lines until Ctrl-C
      --grep string      only lines whose message contains this text (plain text, not a pattern)
  -h, --help             help for logs
      --json             print one JSON object per line (NDJSON)
      --level string     the lowest level to show: debug, info, warn or error (default info)
      --limit int        how many lines at most, 1 to 1000 (default 200)
      --since string     how far back to read, like 30m, 2h or 3d (at most 14d), or a time (RFC 3339); default the last hour
      --source strings   only these streams: stdout, stderr, model_server, legion_serve, legion (comma separated or repeated; default all)
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

* [legion rental](legion_rental.md)	 - Create, connect to, stop and delete rentals

