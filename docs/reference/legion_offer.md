## legion offer

Browse GPU offers from hosts

### Synopsis

An offer is a host with free GPUs at a price per GPU-hour. Find one, then rent it with: legion rental create --offer <host>.

```
legion offer [flags]
```

### Examples

```
  legion offer list --gpu-model "NVIDIA GeForce RTX 4090" --max-price 2
  legion offer get amber-falcon
```

### Options

```
  -h, --help   help for offer
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
* [legion offer get](legion_offer_get.md)	 - Show one offer
* [legion offer list](legion_offer_list.md)	 - List offers, cheapest first

