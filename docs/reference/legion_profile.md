## legion profile

Manage named profiles (API URL, key, default output)

### Synopsis

A profile bundles an API URL, an API key and a default output format, so you can
switch between accounts or environments. Select one with 'legion profile use',
-p/--profile or LEGION_PROFILE.

```
legion profile [flags]
```

### Examples

```
  legion profile create staging --api-url https://api.staging.example
  legion profile use staging
```

### Options

```
  -h, --help   help for profile
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
* [legion profile create](legion_profile_create.md)	 - Create a profile
* [legion profile delete](legion_profile_delete.md)	 - Delete a profile and its saved key
* [legion profile list](legion_profile_list.md)	 - List profiles
* [legion profile use](legion_profile_use.md)	 - Make a profile the default

