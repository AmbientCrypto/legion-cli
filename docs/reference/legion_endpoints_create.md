## legion endpoints create

Create an endpoint: expose a rental's port, or serve a model on autoscaled GPUs

### Synopsis

An endpoint is a URL (https://<name>.ambi.run), an access policy and a target.

Expose a port of one of your rentals (no extra GPU cost):
  legion endpoints create --rental <rental> --port 8080 --access keys
--access is public (anyone, the default) or keys (callers need an access key); --source <cidr> (repeatable) limits
callers to those networks in either mode. The name defaults to the rental's name.

Serve a model on autoscaled GPUs: --catalog (or --image with --port). Legion rents the cheapest hardware that fits
the model, GPU type and count included; replicas can differ and drift to cheaper hardware as they are replaced.
Narrow it only if you need to: --gpu <model> (repeatable, unordered allow-list), --max-price (total per
replica-hour), --spot, --placement. For your own image state the need: --min-gpu-memory <GB> and --gpu-counts
1,2,4,8. A monthly spend cap (--monthly-cap) is required. Scaling
defaults (min, max, concurrency) come from the platform. You see the cost floor, ceiling and cap and confirm (or
pass --yes). Placement is eligible provider hosts; --ack-host-visibility acknowledges that hosts can see the
workload (needed once per organization).

On a terminal anything missing is asked for. With keys access the first key is printed ONCE: copy it now.

```
legion endpoints create [name] [flags]
```

### Examples

```
  legion endpoints create --rental quiet-river-7k2f --port 8080 --access keys
  legion endpoints create chat --catalog llama-3-8b --max-price 2.50 --monthly-cap 400
  legion endpoints create chat --image ghcr.io/me/serve:1 --port 8000 --min-gpu-memory 24 --gpu-counts 1,2 --max-price 1 --monthly-cap 100 --spot --scale-to-zero --min 0 --yes
```

### Options

```
      --access string            public (anyone; default for a rental port) or keys (callers need an access key; default for a model)
      --ack-host-visibility      acknowledge that provider hosts can see the workload (once per organization)
      --arg stringArray          replace the image's default arguments of your own --image: one item per flag (repeatable)
      --catalog string           catalog model id to serve (see: legion catalog list)
      --command stringArray      replace the image entrypoint of your own --image: one item per flag, for example --command python --command server.py (repeatable)
      --env stringArray          environment variable NAME=value for the container (repeatable). Not for secrets: they would show in your shell history; use --secret-env
      --gpu stringArray          only use this GPU model, for example H100 (repeatable, unordered: the cheapest fitting one is rented). Default: any
      --gpu-counts string        own image: GPU counts a replica may use, comma separated from 1,2,4,8 (default 1,2,4,8)
      --health-path string       path polled to see a replica is ready (default /v1/models)
  -h, --help                     help for create
      --image string             your own container image instead of a catalog model (when your organization allows custom images)
      --key-expires string       first key expiry: 7d, 30d, 90d, 365d or never (default: the platform's, 90d)
      --key-name string          name of the first access key (default "default")
      --kind string              protocol: openai (default for a model) or http
      --max int                  maximum replicas (default from the platform)
      --max-price string         highest total price per replica-hour in dollars (all its GPUs), for example 2.50. Default: none
      --min int                  minimum replicas (default from the platform)
      --min-gpu-memory float     own image: minimum total GPU memory per replica in GB
      --model string             alias of --catalog
      --monthly-cap string       monthly spend cap in dollars (UTC calendar month), required for a model
      --placement string         eligible_hosts (default) or own_hosts
      --port int                 with --rental: the port to expose; with --image: the port the service listens on
      --rental string            expose a port of this rental (name, id or prefix) instead of serving a model
      --scale-to-zero            let the endpoint scale down to no replicas when idle (cold requests then wait for a replica)
      --secret-env stringArray   secret environment variable NAME (repeatable). The value is read from stdin (one line per secret) or a hidden prompt, stored encrypted and never shown again
      --served-model string      with --image: the model name the service reports
      --setting stringArray      model setting key=value for a catalog model, for example max_model_len=32768 (repeatable; see the catalog entry's settings)
      --source stringArray       allowed source CIDR, for example 203.0.113.0/24 (repeatable; either access mode)
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

