## legion rental backup cancel

Cancel a backup in progress

### Synopsis

Cancel a queued or uploading backup. The charge is refunded.

```
legion rental backup cancel <backup-id> [flags]
```

### Examples

```
  legion rental backup cancel 6f1c2e3a-...
  legion rental backup cancel 6f1c2e3a-... -q
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

* [legion rental backup](legion_rental_backup.md)	 - Back up a rental's workspace and download it

