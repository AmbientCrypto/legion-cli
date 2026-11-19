## legion services autoscale

Turn autoscaling on or off for a service

### Synopsis

Switch a service between one rental (scaling off) and an autoscaled pool (scaling on). The URL and the
keys stay the same either way.

```
legion services autoscale [flags]
```

### Examples

```
  legion services autoscale on chat --monthly-cap 300
  legion services autoscale off chat --keep-replica quiet-river-7k2f
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

* [legion services](legion_services.md)	 - Give what you run a public URL, with optional service keys
* [legion services autoscale off](legion_services_autoscale_off.md)	 - Turn off autoscaling and keep one replica as a single rental
* [legion services autoscale on](legion_services_autoscale_on.md)	 - Turn on autoscaling for a single-rental service

