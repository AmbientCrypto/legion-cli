## legion rental backup create

Back up a rental's workspace

### Synopsis

Copy the rental's workspace into a backup kept --retention-days (7 or 30). You are charged the
estimated price shown, so you are asked to confirm (or pass --yes). --delete-rental-after deletes the
rental once the backup is ready.

```
legion rental backup create <rental> [flags]
```

### Examples

```
  legion rental backup create quiet-river-7k2f
  legion rental backup create quiet-river-7k2f --retention-days 30 --yes
```

### Options

```
      --delete-rental-after   delete the rental when the backup is ready
  -h, --help                  help for create
      --retention-days int    keep the backup 7 or 30 days (default 7)
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

