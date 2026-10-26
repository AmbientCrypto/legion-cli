## legion rental charges

Show what one rental has cost so far

### Synopsis

Show the billed seconds and the compute, storage and backup charges of one rental. For all rentals: legion billing charges.

```
legion rental charges <rental> [flags]
```

### Examples

```
  legion rental charges quiet-river-7k2f
  legion rental charges quiet -o json
```

### Options

```
  -h, --help   help for charges
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

