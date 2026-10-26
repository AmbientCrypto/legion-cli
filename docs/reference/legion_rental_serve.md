## legion rental serve

Show the endpoint and API key of a premade model rental

### Synopsis

For a rental created from a premade model (legion rental create --catalog <model>), show the
public URL, its status and the service API key (lsk_...). The key is a secret: it is shown here only
when you ask, never in 'rental get'.

```
legion rental serve <rental> [flags]
```

### Examples

```
  legion rental serve quiet-river-7k2f
  legion rental serve quiet-river-7k2f -o jsonpath=$.url
```

### Options

```
  -h, --help   help for serve
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

