## legion endpoints keys rotate

Replace a key with a new one, keeping the old one for a while

### Synopsis

Make a new key with the same name and show its secret ONCE. The old key keeps working for the overlap
(default 24 hours) so you can move callers over, then stops. Revoke the old key at once with 'keys revoke'.

```
legion endpoints keys rotate <endpoint> <key> [flags]
```

### Examples

```
  legion endpoints keys rotate chat ci
  legion endpoints keys rotate chat ci --overlap-hours 12 --expires 30d
```

### Options

```
      --expires string      expiry of the new key: 7d, 30d, 90d, 365d or never (default 90d)
  -h, --help                help for rotate
      --overlap-hours int   hours the old key keeps working (default 24)
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

* [legion endpoints keys](legion_endpoints_keys.md)	 - Manage an endpoint's access keys

