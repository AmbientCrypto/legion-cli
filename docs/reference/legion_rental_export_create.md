## legion rental export create

Start exporting a workspace

### Synopsis

Start an export of the rental's workspace. Follow it with 'rental export get'; when ready it has a download URL.

```
legion rental export create <rental> [flags]
```

### Examples

```
  legion rental export create quiet-river-7k2f
  legion rental export create quiet -o name
```

### Options

```
  -h, --help   help for create
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

