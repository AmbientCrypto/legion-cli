## legion services delete

Delete a service, its replicas and its keys

### Synopsis

Delete the service: replicas stop, every key stops working and the URL goes away. This cannot be undone.
On a terminal you must type the service's name to confirm; in scripts pass --yes. The API may ask for step-up
verification, which needs a browser session.

```
legion services delete <service> [flags]
```

### Examples

```
  legion services delete chat
  legion services delete chat --yes
```

### Options

```
  -h, --help   help for delete
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

