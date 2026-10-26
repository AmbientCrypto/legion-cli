## legion ssh-key add

Save an SSH public key

### Synopsis

Save a public key (default ~/.ssh/id_ed25519.pub). Only the public half is sent; a private key
file is refused. Without --label the key's comment, then the file name, is the label.

```
legion ssh-key add [flags]
```

### Examples

```
  legion ssh-key add
  legion ssh-key add --file ~/.ssh/work.pub --label work
```

### Options

```
      --file string    public key file (default ~/.ssh/id_ed25519.pub)
  -h, --help           help for add
      --label string   name for the key
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

