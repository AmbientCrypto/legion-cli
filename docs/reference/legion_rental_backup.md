## legion rental backup

Back up a rental's workspace and download it

### Synopsis

A backup copies the workspace off the host and keeps it for 7 or 30 days. It is charged by size, so
estimate first. Download links are charged too.

```
legion rental backup [flags]
```

### Examples

```
  legion rental backup estimate quiet-river-7k2f
  legion rental backup create quiet-river-7k2f --retention-days 30
```

### Options

```
  -h, --help   help for backup
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
* [legion rental backup cancel](legion_rental_backup_cancel.md)	 - Cancel a backup in progress
* [legion rental backup create](legion_rental_backup_create.md)	 - Back up a rental's workspace
* [legion rental backup delete](legion_rental_backup_delete.md)	 - Delete a backup
* [legion rental backup download-link](legion_rental_backup_download-link.md)	 - Make a download link for a ready backup
* [legion rental backup estimate](legion_rental_backup_estimate.md)	 - Estimate a backup's size and price
* [legion rental backup get](legion_rental_backup_get.md)	 - Show one backup
* [legion rental backup list](legion_rental_backup_list.md)	 - List backups

