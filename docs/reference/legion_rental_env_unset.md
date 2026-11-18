## legion rental env unset

Remove environment variables

### Synopsis

Remove the named variables; the others stay. This restarts the rental's container (files outside /workspace are reset), so legion asks first.

```
legion rental env unset <rental> NAME... [flags]
```

### Examples

```
  legion rental env unset quiet-river-7k2f MODE
  legion rental env unset quiet-river-7k2f MODE HF_TOKEN --yes
```

### Options

```
  -h, --help   help for unset
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

