## legion api-key create

Create an API key

### Synopsis

Create a key. The secret is shown ONCE, here, and cannot be retrieved later: copy it now and
store it somewhere safe. Use scope full for the CLI and scripts.

```
legion api-key create <name> [flags]
```

### Examples

```
  legion api-key create ci --expires 720h
  legion api-key create bot --scope agent
```

### Options

```
      --expires duration   expire after this long, e.g. 720h (default: never)
  -h, --help               help for create
      --scope string       full or agent (default "full")
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

* [legion api-key](legion_api-key.md)	 - Manage the API keys that sign in to Legion

