## legion endpoints autoscale on

Turn on autoscaling for a single-rental endpoint

### Synopsis

Turn a single-rental endpoint into an autoscaled one. Hardware starts as the rental's own GPU type and count
(narrow it with --gpu models, --max-price per replica-hour and --spot); scaling defaults to the platform's
(override --min, --max, --scale-to-zero, --target-concurrency); --monthly-cap is required.

--keep-rental (default) makes the current rental the first replica and moves its lifecycle to the endpoint.
--replace starts fresh replicas on other hosts and stops this rental once they are ready. You see the cost
floor and ceiling and confirm (or pass --yes).

```
legion endpoints autoscale on <endpoint> [flags]
```

### Examples

```
  legion endpoints autoscale on chat --monthly-cap 300 --max 3
  legion endpoints autoscale on chat --gpu H100 --max-price 2.50 --monthly-cap 300 --replace --yes
```

### Options

```
      --gpu stringArray          only use this GPU model, for example H100 (repeatable, unordered: the cheapest fitting one is rented). Default: any
      --gpu-counts string        own image: GPU counts a replica may use, comma separated from 1,2,4,8 (default 1,2,4,8)
  -h, --help                     help for on
      --keep-rental              keep the current rental as the first replica (default)
      --max int                  maximum replicas (default from the platform)
      --max-price string         highest total price per replica-hour in dollars (all its GPUs), for example 2.50. Default: none
      --min int                  minimum replicas (default from the platform)
      --min-gpu-memory float     own image: minimum total GPU memory per replica in GB
      --monthly-cap string       monthly spend cap in dollars (UTC calendar month), required
      --placement string         eligible_hosts (default) or own_hosts
      --replace                  start fresh replicas and stop the current rental when they are ready
      --scale-to-zero            let the endpoint scale down to no replicas when idle (cold requests then wait for a replica)
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

* [legion endpoints autoscale](legion_endpoints_autoscale.md)	 - Turn autoscaling on or off for an endpoint

