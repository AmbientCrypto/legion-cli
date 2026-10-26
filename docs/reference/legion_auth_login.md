## legion auth login

Sign in through the browser and save an API key

### Synopsis

Sign in through the browser. legion opens Legion's sign-in page, you approve
"Legion CLI" there, and legion saves the API key the approval creates: scope
"full", named "Legion CLI (<this computer>)", valid for 90 days. It is listed in
Settings → API keys, where you can revoke it; legion auth logout revokes it too.

The browser must run on this computer (the sign-in returns to a port on
127.0.0.1). On a remote machine use --paste: create a key in https://legion.ambient.xyz/settings/api-keys
and paste it at the hidden prompt.

For scripts and CI pipe a key in with --with-token, or skip login and set LEGION_API_KEY.

```
legion auth login [flags]
```

### Examples

```
  legion auth login
  legion auth login --no-browser
  legion auth login --paste
  echo "$LEGION_KEY" | legion auth login --with-token --profile ci
```

### Options

```
  -h, --help         help for login
      --no-browser   print the sign-in URL instead of opening a browser
      --paste        paste an API key made in Settings → API keys instead of signing in through the browser
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

