## legion rental backup list

List backups

### Synopsis

List backups of one rental, or of every rental when none is given. --state keeps only backups in that state.

```
legion rental backup list [rental] [flags]
```

### Examples

```
  legion rental backup list
  legion rental backup list quiet-river-7k2f --state ready
```

### Options

```
  -h, --help           help for list
      --state string   only backups in this state: queued, uploading, ready, failed, cancelled, expired, deleted
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

