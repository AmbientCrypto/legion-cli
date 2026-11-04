## legion rental export estimate

Show what downloading a workspace export will cost

### Synopsis

Show the estimated size of the rental's workspace and the price of one download link, without making an export. A link is charged by the GiB every time you make one and works once.

```
legion rental export estimate <rental> [flags]
```

### Examples

```
  legion rental export estimate quiet-river-7k2f
  legion rental export estimate quiet -o json
```

### Options

```
  -h, --help   help for estimate
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

* [legion rental export](legion_rental_export.md)	 - Export a stopped rental's workspace and download it

