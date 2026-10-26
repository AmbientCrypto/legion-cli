## legion config list

Show every setting, its effective value and where it comes from

### Synopsis

Show the effective value of each setting and its source: flag, env, profile or default. The API key is masked.

```
legion config list [flags]
```

### Examples

```
  legion config list
  legion config list -p staging -o json
```

### Options

```
  -h, --help   help for list
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

* [legion config](legion_config.md)	 - Read and change settings of the current profile

