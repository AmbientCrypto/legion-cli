## legion billing activity

Show billing activity: usage sessions and balance changes

### Synopsis

Show rental usage sessions and ledger entries together, newest first. --rental narrows to one rental.

```
legion billing activity [flags]
```

### Examples

```
  legion billing activity
  legion billing activity --rental quiet-river-7k2f --limit 20
```

### Options

```
      --account string   balance or earnings (default "balance")
  -h, --help             help for activity
      --limit int        most rows (default 50)
      --rental string    only this rental
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

