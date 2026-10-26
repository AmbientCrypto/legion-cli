## legion billing balance

Show your prepaid balance

### Synopsis

Show the organization's balance. A rental needs credit to be created or started.

```
legion billing balance [flags]
```

### Examples

```
  legion billing balance
  legion billing balance -o jsonpath=$.balance.amount_minor
```

### Options

```
  -h, --help   help for balance
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

