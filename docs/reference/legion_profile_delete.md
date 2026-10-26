## legion profile delete

Delete a profile and its saved key

### Synopsis

Delete a profile from config.yaml and its API key from credentials.yaml. The key itself stays valid; revoke it with: legion api-key revoke.

```
legion profile delete <name> [flags]
```

### Examples

```
  legion profile delete staging --yes
  legion profile delete old
```

### Options

```
  -h, --help   help for delete
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

