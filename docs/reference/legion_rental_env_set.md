## legion rental env set

Add or change environment variables

### Synopsis

Add or change variables; the others stay. --env NAME=value sets a plain variable. --secret-env NAME reads the
value from stdin (one line per secret) or a hidden prompt, and stores it encrypted and write-only. This restarts the
rental's container (files outside /workspace are reset), so legion asks first; a stopped rental takes the change at
its next start.

```
legion rental env set <rental> --env NAME=value | --secret-env NAME [flags]
```

### Examples

```
  legion rental env set quiet-river-7k2f --env MODE=fast
  printf %s "$HF_TOKEN" | legion rental env set quiet-river-7k2f --secret-env HF_TOKEN --yes
```

### Options

```
      --env stringArray          environment variable NAME=value (repeatable). Not for secrets: use --secret-env
  -h, --help                     help for set
      --secret-env stringArray   secret environment variable NAME (repeatable); the value is read from stdin (one line per secret) or a hidden prompt
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

* [legion rental env](legion_rental_env.md)	 - Show or change a rental's environment variables

