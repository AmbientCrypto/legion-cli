## legion offer get

Show one offer

### Synopsis

Show a host's GPUs, price, hardware and reliability. <host> is a host id or name from 'legion offer list'.

```
legion offer get <host> [flags]
```

### Examples

```
  legion offer get amber-falcon
  legion offer get 5f0c1c2e-0000-0000-0000-000000000000 -o json
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

* [legion offer](legion_offer.md)	 - Browse GPU offers from hosts

