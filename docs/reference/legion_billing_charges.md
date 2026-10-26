## legion billing charges

Show what each rental has cost

### Synopsis

Show compute, storage and backup charges per rental, newest first.

```
legion billing charges [flags]
```

### Examples

```
  legion billing charges
  legion billing charges --limit 10 -o json
```

### Options

```
  -h, --help        help for charges
      --limit int   most rows (default 50)
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

* [legion billing](legion_billing.md)	 - See your balance, charges and ledger

