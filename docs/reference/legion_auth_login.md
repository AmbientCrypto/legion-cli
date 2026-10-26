## legion auth login

Save an API key and check that it works

### Synopsis

Log in with an API key. legion opens https://legion.ambient.xyz/settings/api-keys in your browser:
create a key with scope "full" there, copy it, and paste it at the hidden prompt.
The key is checked against the API before it is saved.

For scripts and CI pipe the key in with --with-token, or skip login and set LEGION_API_KEY.

```
legion auth login [flags]
```

### Examples

```
  legion auth login
  legion auth login --no-browser
  echo "$LEGION_KEY" | legion auth login --with-token --profile ci
```

### Options

```
  -h, --help         help for login
      --no-browser   do not open the browser
      --with-token   read the API key from stdin
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

