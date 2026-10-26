## legion rental start

Start a stopped rental

### Synopsis

Start a stopped rental again on its host's GPUs. GPU billing starts when it is running, at the
price shown; you are asked to confirm (or pass --yes). Needs credit and free GPUs on the host.
Waits until running unless --async.

```
legion rental start <rental> [flags]
```

### Examples

```
  legion rental start quiet-river-7k2f
  legion rental start quiet --yes --async
```

### Options

```
      --async                   return without waiting for the rental to run
  -h, --help                    help for start
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

* [legion rental](legion_rental.md)	 - Create, connect to, stop and delete rentals

