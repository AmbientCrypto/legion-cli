## legion rental stop

Stop the workload; keep the workspace

### Synopsis

Stop the rental's container and release its GPUs. Billing for GPU time ends.
The workspace is kept on the host and billed for storage, per GiB, until you delete the rental.
Start it again with: legion rental start. Waits until stopped unless --async.

```
legion rental stop <rental> [flags]
```

### Examples

```
  legion rental stop quiet-river-7k2f
  legion rental stop quiet --yes --async
```

### Options

```
      --async                   return without waiting for the rental to stop
  -h, --help                    help for stop
      --wait-timeout duration   how long to wait for the rental to stop (default 5m0s)
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

