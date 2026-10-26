## legion completion

Generate a shell completion script

### Synopsis

Print a completion script for your shell. Completion also suggests live values: rental names,
host names, catalog ids, saved SSH key labels and GPU models (it asks the API with a short
timeout and stays silent if that fails).

Install:
  bash        source <(legion completion bash)                       # add to ~/.bashrc
              legion completion bash > /etc/bash_completion.d/legion  # or system-wide (needs bash-completion)
  zsh         legion completion zsh > "${fpath[1]}/_legion"           # then restart the shell
              (if completion is not enabled yet: echo "autoload -U compinit; compinit" >> ~/.zshrc)
  fish        legion completion fish > ~/.config/fish/completions/legion.fish
  powershell  legion completion powershell | Out-String | Invoke-Expression   # add to $PROFILE

```
legion completion bash|zsh|fish|powershell [flags]
```

### Examples

```
  source <(legion completion bash)
  legion completion zsh > "${fpath[1]}/_legion"
```

### Options

```
  -h, --help   help for completion
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

