## legion update

Update legion to the latest release

### Synopsis

Download the latest legion release for this computer from GitHub, check it
against the release's checksums.txt, and replace this binary with it.

With --check, only say whether a newer release exists. On a terminal, legion
also says so after other commands, at most once a day; set
LEGION_NO_UPDATE_CHECK=1 to turn that off.

```
legion update [flags]
```

### Examples

```
  legion update
  legion update --check
  legion update --check -o json
```

### Options

```
      --check   only say whether a newer release exists
  -h, --help    help for update
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

