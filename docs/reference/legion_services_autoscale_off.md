## legion services autoscale off

Turn off autoscaling and keep one replica as a single rental

### Synopsis

Turn an autoscaled service back into one rental. You pick one ready replica to keep (--keep-replica, a rental
name or id; on a terminal you are asked); the other replicas drain and stop. The URL and keys stay the same.

```
legion services autoscale off <service> [flags]
```

### Examples

```
  legion services autoscale off chat --keep-replica quiet-river-7k2f --yes
  legion services autoscale off chat
```

### Options

```
  -h, --help                  help for off
      --keep-replica string   rental name or id of the replica to keep
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

* [legion services autoscale](legion_services_autoscale.md)	 - Turn autoscaling on or off for a service

