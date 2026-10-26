## legion rental wait

Wait until a rental is running or stopped

### Synopsis

Poll a rental until it reaches a state, then exit 0. Exits 1 on failure or timeout.
Use it in scripts after 'legion rental create --async'.

```
legion rental wait <rental> [flags]
```

### Examples

```
  legion rental wait quiet-river-7k2f
  legion rental wait quiet-river-7k2f --for stopped --timeout 2m
```

### Options

```
      --for string         state to wait for: running or stopped (default "running")
  -h, --help               help for wait
      --timeout duration   give up after this long (default 10m0s)
```

### Options inherited from parent commands

```
      --api-url string   API base URL (env LEGION_API_URL)
      --debug            log each request's method, path, status and duration to stderr
      --no-color         disable colour (also NO_COLOR)
  -o, --output string    output format: table|wide|json|yaml|name|jsonpath=<expr> (env LEGION_OUTPUT)
  -p, --profile string   profile to use (env LEGION_PROFILE)
  -q, --quiet            print only results, no progress or next-step hints
  -y, --yes              answer yes to confirmation prompts
```

### SEE ALSO

* [legion rental](legion_rental.md)	 - Create, connect to, stop and delete rentals

