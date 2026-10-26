## legion rental list

List your rentals

### Synopsis

List the organization's rentals with state and price. Deleted rentals are hidden unless --all. Use -o wide for host and SSH endpoint, -o name for scripting.

```
legion rental list [flags]
```

### Examples

```
  legion rental list
  legion rental list -o jsonpath='$[?(@.state=="running")].name'
```

### Options

```
      --all    include deleted rentals
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

* [legion rental](legion_rental.md)	 - Create, connect to, stop and delete rentals

