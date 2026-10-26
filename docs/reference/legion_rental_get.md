## legion rental get

Show one rental

### Synopsis

Show a rental's state, price, resources and how to connect. <rental> is an id, a name or a unique prefix.

```
legion rental get <rental> [flags]
```

### Examples

```
  legion rental get quiet-river-7k2f
  legion rental get quiet -o json
```

### Options

```
  -h, --help   help for get
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

