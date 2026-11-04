## legion rental export link

Make a single-use download link for a ready export

### Synopsis

Make a link to download a ready export. Each link is charged by the GiB (the charge is shown first and you confirm, or pass --yes), is not refunded, works once and expires after about an hour. Make a new link if one is used or has expired.

```
legion rental export link <rental> <export-id> [flags]
```

### Examples

```
  legion rental export link quiet-river-7k2f 6f1c...
  legion rental export link quiet 6f1c... --yes -o jsonpath=$.url
```

### Options

```
  -h, --help   help for link
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

