## legion config get

Print the effective value of one setting

### Synopsis

Print one setting's effective value (after flags and environment). Keys: api_url, output.

```
legion config get <key> [flags]
```

### Examples

```
  legion config get api_url
  legion config get output -p staging
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

* [legion config](legion_config.md)	 - Read and change settings of the current profile

