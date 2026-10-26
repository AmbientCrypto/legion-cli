## legion profile current

Print the profile in use

### Synopsis

Print the profile in effect: -p, then LEGION_PROFILE, then the one set with 'legion profile use'.

```
legion profile current [flags]
```

### Examples

```
  legion profile current
  LEGION_PROFILE=ci legion profile current
```

### Options

```
  -h, --help   help for current
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

