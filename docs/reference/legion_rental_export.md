## legion rental export

Export a rental's workspace to a download

### Synopsis

A workspace export uploads the files of a running rental to a download URL. Unlike a backup it is not kept after it expires.

```
legion rental export [flags]
```

### Examples

```
  legion rental export create quiet-river-7k2f
  legion rental export get quiet-river-7k2f <export-id>
```

### Options

```
  -h, --help   help for export
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

* [legion rental](legion_rental.md)	 - Create, connect to, stop and delete rentals
* [legion rental export cancel](legion_rental_export_cancel.md)	 - Cancel an export
* [legion rental export create](legion_rental_export_create.md)	 - Start exporting a workspace
* [legion rental export get](legion_rental_export_get.md)	 - Show one export
* [legion rental export list](legion_rental_export_list.md)	 - List a rental's exports

