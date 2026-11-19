## legion services logs

Show a service's logs from every replica, or follow them live

### Synopsis

Lines come oldest first: the container's output (stdout, stderr), for a Legion model image also the
model server (model_server) and Legion's download and start log (legion_serve), and Legion's own events
about the rental (legion: image pulled, container started, health check passed, stopped, failed).
Without --follow it prints the newest lines of the range (the last hour by default). With --follow it
keeps printing new lines every 2 seconds until Ctrl-C. Logs are kept 14 days; values of the rental's
environment variables and secrets are shown as [redacted].
A service's lines come from all its replicas, merged by time; --replica keeps one.

```
legion services logs <service> [flags]
```

### Examples

```
  legion services logs chat
  legion services logs chat -f --source model_server
  legion services logs chat --replica quiet-river-7k2f --since 30m
```

### Options

```
  -f, --follow           keep printing new lines until Ctrl-C
      --grep string      only lines whose message contains this text (plain text, not a pattern)
  -h, --help             help for logs
      --json             print one JSON object per line (NDJSON)
      --level string     the lowest level to show: debug, info, warn or error (default info)
      --limit int        how many lines at most, 1 to 1000 (default 200)
      --replica string   only this replica's lines (its rental id or name, as 'legion services replicas' lists it)
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

* [legion services](legion_services.md)	 - Give what you run a public URL, with optional service keys

