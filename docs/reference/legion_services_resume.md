## legion services resume

Start serving again

### Synopsis

Resume a paused service. Replicas start again and bill per GPU-hour; the cost floor and ceiling are shown and you confirm (or pass --yes).

```
legion services resume <service> [flags]
```

### Examples

```
  legion services resume chat
  legion services resume chat --yes
```

### Options

```
  -h, --help   help for resume
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

* [legion services](legion_services.md)	 - Give what you run a public URL, with optional service keys

