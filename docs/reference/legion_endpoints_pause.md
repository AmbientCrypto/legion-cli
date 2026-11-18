## legion endpoints pause

Stop serving and release the replicas

### Synopsis

Pause: no replicas run and no GPU cost accrues. The URL and keys are kept and callers get 503
endpoint_paused until you resume. Warm replicas are released after the warm expiry.

```
legion endpoints pause <endpoint> [flags]
```

### Examples

```
  legion endpoints pause chat
  legion endpoints pause chat -o json
```

### Options

```
  -h, --help   help for pause
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

