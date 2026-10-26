## legion whoami

Show the user and organization you are logged in as

### Synopsis

Show the user, organization and role of the current API key. Shortcut for the identity part of 'legion auth status'.

```
legion whoami [flags]
```

### Examples

```
  legion whoami
  legion whoami -o jsonpath=$.org.name
```

### Options

```
  -h, --help   help for whoami
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

