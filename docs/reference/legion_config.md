## legion config

Read and change settings of the current profile

### Synopsis

Settings are stored per profile in config.yaml. Keys: api_url, output.
Precedence is flag, then environment, then profile, then default: 'legion help environment'.

```
legion config [flags]
```

### Examples

```
  legion config list
  legion config set output json
```

### Options

```
  -h, --help   help for config
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
* [legion config get](legion_config_get.md)	 - Print the effective value of one setting
* [legion config list](legion_config_list.md)	 - Show every setting, its effective value and where it comes from
* [legion config set](legion_config_set.md)	 - Change a setting of the current profile
* [legion config unset](legion_config_unset.md)	 - Remove a setting from the current profile

