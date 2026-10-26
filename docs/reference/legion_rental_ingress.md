## legion rental ingress

Expose a port of a rental on the internet

### Synopsis

An ingress rule publishes an HTTP service in the rental at https://<rental>.<domain>, optionally
limited to some source networks. SSH needs no rule.

```
legion rental ingress [flags]
```

### Examples

```
  legion rental ingress add quiet-river-7k2f --port 8000
  legion rental ingress list quiet-river-7k2f
```

### Options

```
  -h, --help   help for ingress
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
* [legion rental ingress add](legion_rental_ingress_add.md)	 - Expose a port
* [legion rental ingress list](legion_rental_ingress_list.md)	 - List a rental's exposed ports
* [legion rental ingress remove](legion_rental_ingress_remove.md)	 - Stop exposing a port

