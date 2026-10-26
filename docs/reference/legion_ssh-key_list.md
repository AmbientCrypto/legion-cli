## legion ssh-key list

List saved SSH keys

### Synopsis

List the SSH public keys saved for your organization.

```
legion ssh-key list [flags]
```

### Examples

```
  legion ssh-key list
  legion ssh-key list -o name
```

### Options

```
  -h, --help   help for list
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

* [legion ssh-key](legion_ssh-key.md)	 - Save the SSH public keys you rent with

