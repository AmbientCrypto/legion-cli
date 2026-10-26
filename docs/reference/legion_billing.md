## legion billing

See your balance, charges and ledger

### Synopsis

Renting spends from a prepaid balance. Amounts are in US dollars.

```
legion billing [flags]
```

### Examples

```
  legion billing balance
  legion billing charges
```

### Options

```
  -h, --help   help for billing
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

* [legion](legion.md)	 - Rent GPUs on Ambient Legion
* [legion billing activity](legion_billing_activity.md)	 - Show billing activity: usage sessions and balance changes
* [legion billing balance](legion_billing_balance.md)	 - Show your prepaid balance
* [legion billing charges](legion_billing_charges.md)	 - Show what each rental has cost
* [legion billing ledger](legion_billing_ledger.md)	 - Show the ledger of balance changes

