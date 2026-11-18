## legion rental env list

List a rental's environment variables

### Synopsis

List the variable names of a rental with their values. Secrets show as set and hidden: their values are never returned.

```
legion rental env list <rental> [flags]
```

### Examples

```
  legion rental env list quiet-river-7k2f
  legion rental env list quiet -o json
```

### Options

```
  -h, --help   help for list
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

* [legion rental env](legion_rental_env.md)	 - Show or change a rental's environment variables

