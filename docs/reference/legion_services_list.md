## legion services list

List services

### Synopsis

List the organization's services, on a rental or autoscaled, with access, scaling, status and Service URL. Filter with --mode rental|autoscaled or --rental <rental>. Use -o wide for ids and spend.

```
legion services list [flags]
```

### Examples

```
  legion services list
  legion services list --mode rental -o name
  legion services list --rental quiet-river-7k2f
```

### Options

```
  -h, --help            help for list
      --mode string     only this mode: rental or autoscaled
      --rental string   only the services on this rental
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

