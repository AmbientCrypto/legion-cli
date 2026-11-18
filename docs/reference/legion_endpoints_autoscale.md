## legion endpoints autoscale

Turn autoscaling on or off for an endpoint

### Synopsis

Switch an endpoint between one rental (scaling off) and an autoscaled pool (scaling on). The URL and the
keys stay the same either way.

```
legion endpoints autoscale [flags]
```

### Examples

```
  legion endpoints autoscale on chat --monthly-cap 300
  legion endpoints autoscale off chat --keep-replica quiet-river-7k2f
```

### Options

```
  -h, --help   help for autoscale
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
* [legion endpoints autoscale off](legion_endpoints_autoscale_off.md)	 - Turn off autoscaling and keep one replica as a single rental
* [legion endpoints autoscale on](legion_endpoints_autoscale_on.md)	 - Turn on autoscaling for a single-rental endpoint

