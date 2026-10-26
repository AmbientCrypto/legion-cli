## legion profile use

Make a profile the default

### Synopsis

Make a profile the one used when -p and LEGION_PROFILE are not given.

```
legion profile use <name> [flags]
```

### Examples

```
  legion profile use staging
  legion profile use default
```

### Options

```
  -h, --help   help for use
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

* [legion profile](legion_profile.md)	 - Manage named profiles (API URL, key, default output)

