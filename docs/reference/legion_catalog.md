## legion catalog

Browse Legion's starter images and premade models

### Synopsis

The catalog lists images that are ready to work in over SSH and premade models that serve
themselves (an OpenAI-compatible endpoint, for example). Rent one with:
legion rental create --offer <host> --catalog <id>.

```
legion catalog [flags]
```

### Examples

```
  legion catalog list
  legion catalog get qwen3-5-4b
```

### Options

```
  -h, --help   help for catalog
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
* [legion catalog get](legion_catalog_get.md)	 - Show one catalog entry
* [legion catalog list](legion_catalog_list.md)	 - List starter images and premade models

