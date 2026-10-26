## legion ssh-key

Save the SSH public keys you rent with

### Synopsis

Saved public keys are offered when you create a rental, so you do not pass one each time. A key only grants access to rentals created after it was saved with it.

```
legion ssh-key [flags]
```

### Examples

```
  legion ssh-key add
  legion ssh-key list
```

### Options

```
  -h, --help   help for ssh-key
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
* [legion ssh-key add](legion_ssh-key_add.md)	 - Save an SSH public key
* [legion ssh-key list](legion_ssh-key_list.md)	 - List saved SSH keys
* [legion ssh-key remove](legion_ssh-key_remove.md)	 - Forget a saved SSH key

