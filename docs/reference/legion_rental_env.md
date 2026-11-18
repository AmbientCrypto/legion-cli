## legion rental env

Show or change a rental's environment variables

### Synopsis

List, set and unset the environment variables of a rental's container. A secret's value is write-only: it is
stored encrypted, listed as "set, hidden" and never shown again. Changing variables restarts the rental's container
(files outside /workspace are reset); a stopped rental takes them at its next start. Anything in the container,
including these variables, can be read by the provider's machine.

### Examples

```
  legion rental env list quiet-river-7k2f
  legion rental env set quiet-river-7k2f --env MODE=fast --secret-env HF_TOKEN
  legion rental env unset quiet-river-7k2f MODE
```

### Options

```
  -h, --help   help for env
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
* [legion rental env list](legion_rental_env_list.md)	 - List a rental's environment variables
* [legion rental env set](legion_rental_env_set.md)	 - Add or change environment variables
* [legion rental env unset](legion_rental_env_unset.md)	 - Remove environment variables

