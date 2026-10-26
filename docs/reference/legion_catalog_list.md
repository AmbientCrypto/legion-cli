## legion catalog list

List starter images and premade models

### Synopsis

List catalog entries. --kind narrows to images or models. With --host, each entry says whether
that host can run it and how many of its GPUs it needs.

```
legion catalog list [flags]
```

### Examples

```
  legion catalog list --kind model
  legion catalog list --host amber-falcon -o wide
```

### Options

```
  -h, --help          help for list
      --host string   host id or name: show whether it can run each entry
      --kind string   image or model
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

