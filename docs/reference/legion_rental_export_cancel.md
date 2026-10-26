## legion rental export cancel

Cancel an export

### Synopsis

Cancel an export that is still running.

```
legion rental export cancel <rental> <export-id> [flags]
```

### Examples

```
  legion rental export cancel quiet-river-7k2f 6f1c...
  legion rental export cancel quiet 6f1c... -q
```

### Options

```
  -h, --help   help for cancel
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

