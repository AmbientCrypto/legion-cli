## legion rental export get

Show one export

### Synopsis

Show an export's state and, when ready, its download URL.

```
legion rental export get <rental> <export-id> [flags]
```

### Examples

```
  legion rental export get quiet-river-7k2f 6f1c...
  legion rental export get quiet 6f1c... -o jsonpath=$.download_url
```

### Options

```
  -h, --help   help for get
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

* [legion rental export](legion_rental_export.md)	 - Export a rental's workspace to a download

