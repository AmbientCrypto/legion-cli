## legion endpoints list

List endpoints

### Synopsis

List the organization's endpoints: exposed ports, served models and autoscaled models, with access, scaling, status and URL. Filter with --type model|autoscaled|port or --rental <rental>. Use -o wide for ids and spend.

```
legion endpoints list [flags]
```

### Examples

```
  legion endpoints list
  legion endpoints list --type port -o name
  legion endpoints list --rental quiet-river-7k2f
```

### Options

```
  -h, --help            help for list
      --rental string   only the endpoints that expose this rental
      --type string     only this type: model, autoscaled or port
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

