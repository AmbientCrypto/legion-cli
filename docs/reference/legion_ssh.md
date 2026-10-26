## legion ssh

SSH into a rental (shortcut for 'legion rental ssh')

### Synopsis

Wait until the rental is running, then run the system ssh to connect through Legion's
gateway as root. Everything after -- runs on the rental instead of a shell.

The gateway offers your SSH agent's keys and your default key files. To use one specific key
instead, pass --identity. --print shows the command without running it; 'legion rental
ssh-config' prints an ~/.ssh/config block so plain 'ssh <rental>' works, and so do scp and rsync.

```
legion ssh <rental> [-- command...] [flags]
```

### Examples

```
  legion ssh quiet-river-7k2f
  legion ssh quiet-river-7k2f -- nvidia-smi
  legion rental ssh quiet-river-7k2f --print --identity ~/.ssh/id_ed25519
```

### Options

```
  -h, --help                    help for ssh
  -i, --identity string         use this private key for the gateway and the rental (path like ~/.ssh/id_ed25519)
      --print                   print the ssh command instead of running it
      --wait-timeout duration   how long to wait for the rental to run (default 10m0s)
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

