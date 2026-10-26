## legion rental ssh-config

Print an ~/.ssh/config block for a rental

### Synopsis

Print the Host blocks for ~/.ssh/config that let you connect with plain 'ssh <rental>',
scp and rsync. Append them yourself, for example:  legion rental ssh-config <rental> >> ~/.ssh/config

```
legion rental ssh-config <rental> [flags]
```

### Examples

```
  legion rental ssh-config quiet-river-7k2f
  legion rental ssh-config quiet-river-7k2f --identity ~/.ssh/id_ed25519 >> ~/.ssh/config
```

### Options

```
  -h, --help              help for ssh-config
  -i, --identity string   private key to use (path like ~/.ssh/id_ed25519)
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

* [legion rental](legion_rental.md)	 - Create, connect to, stop and delete rentals

