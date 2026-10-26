## legion catalog get

Show one catalog entry

### Synopsis

Show an entry's image reference, requirements and, for a model, its serving details and licence.

```
legion catalog get <id> [flags]
```

### Examples

```
  legion catalog get qwen3-5-4b
  legion catalog get pytorch -o yaml
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

* [legion catalog](legion_catalog.md)	 - Browse Legion's starter images and premade models

