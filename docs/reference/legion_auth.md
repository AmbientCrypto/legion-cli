## legion auth

Log in and out, and see who you are logged in as

### Synopsis

Manage the API key legion uses. Keys are stored per profile in credentials.yaml (mode 0600).

```
legion auth [flags]
```

### Examples

```
  legion auth login
  legion auth status
```

### Options

```
  -h, --help   help for auth
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
* [legion auth login](legion_auth_login.md)	 - Sign in through the browser and save an API key
* [legion auth logout](legion_auth_logout.md)	 - Revoke the key legion auth login made and forget it
* [legion auth status](legion_auth_status.md)	 - Show the profile, API URL and who the key belongs to

