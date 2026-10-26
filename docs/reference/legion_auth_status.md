## legion auth status

Show the profile, API URL and who the key belongs to

### Synopsis

Check the saved (or LEGION_API_KEY) key against the API and show the account it belongs to. Exits 3 when not logged in.

```
legion auth status [flags]
```

### Examples

```
  legion auth status
  legion auth status -o json
```

### Options

```
  -h, --help   help for status
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

* [legion auth](legion_auth.md)	 - Log in and out, and see who you are logged in as

