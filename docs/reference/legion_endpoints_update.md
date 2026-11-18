## legion endpoints update

Change an endpoint's access, scaling, hardware, price ceiling or cap

### Synopsis

Change only the settings you name; everything else stays and the URL never changes.
--access public|keys switches who may call it (keys can exist while it is public, so there is no downtime);
--source <cidr> (repeatable) limits callers to those networks in either mode.
Scaling, hardware, --catalog and --image apply to autoscaled endpoints; changes that can raise spend show the new
cost floor and ceiling and ask for confirmation (or pass --yes). Config changes roll one replica at a time with one
surge replica, which counts against the maximum and the monthly cap. --revision refuses the change when the endpoint
was edited meanwhile. Turn autoscaling on or off with 'legion endpoints autoscale'.

```
legion endpoints update <endpoint> [flags]
```

### Examples

```
  legion endpoints update chat --max 4
  legion endpoints update chat --access keys
  legion endpoints update chat --gpu H100 --gpu A100 --max-price 3 --monthly-cap 600
```

### Options

```
      --access string            public (anyone) or keys (callers need an access key)
      --catalog string           switch to this catalog model id
      --gpu stringArray          only use this GPU model, for example H100 (repeatable, unordered: the cheapest fitting one is rented). Default: any
      --gpu-counts string        own image: GPU counts a replica may use, comma separated from 1,2,4,8 (default 1,2,4,8)
  -h, --help                     help for update
      --image string             switch to this container image
      --max int                  maximum replicas (default from the platform)
      --max-price string         highest total price per replica-hour in dollars (all its GPUs), for example 2.50. Default: none
      --min int                  minimum replicas (default from the platform)
      --min-gpu-memory float     own image: minimum total GPU memory per replica in GB
      --monthly-cap string       monthly spend cap in dollars (UTC calendar month)
      --placement string         eligible_hosts (default) or own_hosts
      --revision int             refuse the change unless the endpoint is at this revision
      --scale-to-zero            let the endpoint scale down to no replicas when idle (cold requests then wait for a replica)
      --source stringArray       allowed source CIDR, for example 203.0.113.0/24 (repeatable)
      --spot                     use spot (interruptible) capacity: cheaper, but replicas can be preempted
      --target-concurrency int   concurrent requests per replica before another is added
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

* [legion endpoints](legion_endpoints.md)	 - Serve models and ports behind one URL with access keys

