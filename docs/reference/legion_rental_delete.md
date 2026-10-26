## legion rental delete

Delete a rental and destroy its workspace

### Synopsis

Delete the rental: the container AND its workspace are destroyed for good and all billing ends.
Make a backup first with: legion rental backup create. On a terminal you must type the rental's
name to confirm; in scripts pass --yes.

```
legion rental delete <rental> [flags]
```

### Examples

```
  legion rental delete quiet-river-7k2f
  legion rental delete quiet-river-7k2f --yes
```

### Options

```
  -h, --help   help for delete
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

