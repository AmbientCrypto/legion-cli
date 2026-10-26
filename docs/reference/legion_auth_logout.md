## legion auth logout

Revoke the key legion auth login made and forget it

### Synopsis

Remove the API key of the current profile from credentials.yaml. A key made by browser sign-in is revoked first; a pasted key stays valid (revoke it with: legion api-key revoke).

```
legion auth logout [flags]
```

### Examples

```
  legion auth logout
  legion auth logout --profile staging
```

### Options

```
  -h, --help   help for logout
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

