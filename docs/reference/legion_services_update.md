## legion services update

Change a service's access, scaling, hardware, price ceiling or cap

### Synopsis

Change only the settings you name; everything else stays and the URL never changes.
--access public|keys switches who may call it (keys can exist while it is public, so there is no downtime);
--source <cidr> (repeatable) limits callers to those networks in either mode.
Scaling, hardware, --catalog and --image apply to autoscaled services; changes that can raise spend show the new
cost floor and ceiling and ask for confirmation (or pass --yes). Config changes roll one replica at a time with one
surge replica, which counts against the maximum and the monthly cap. --revision refuses the change when the service
was edited meanwhile. Turn autoscaling on or off with 'legion services autoscale'.

Container configuration (autoscaled services): --env and --secret-env add or change variables and --unset-env removes
them; the others stay. A secret you do not name keeps its stored value. --setting changes the named model settings,
--command and --arg replace the entrypoint and arguments of your own image; --clear-settings, --clear-command and
--clear-args remove them. Any of these changes the replica template, so replicas are replaced one at a time with one
surge replica and a running replica restarts. Anything in the container can be read by the provider's machine.

--replica-web-terminal off|organization|creator-and-owners (autoscaled services) never replaces a replica: who may
open the terminal (organization or creator-and-owners) changes on the running replicas at once, while turning it on
or off applies to replicas started from now on (a rental's web terminal is fixed when it is created). Only the
service's creator or an owner may change who may open it.

```
legion services update <service> [flags]
```

### Examples

```
  legion services update chat --max 4
  legion services update chat --access keys
  legion services update chat --gpu H100 --gpu A100 --max-price 3 --monthly-cap 600
```

### Options

```
      --access string                 public (anyone) or keys (callers need a service key)
      --arg stringArray               replace the image's default arguments of your own image: one item per flag (repeatable)
      --catalog string                switch to this catalog model id
      --clear-args                    remove the arguments override
      --clear-command                 remove the entrypoint override
      --clear-settings                remove all model settings (the image defaults apply)
      --command stringArray           replace the image entrypoint of your own image: one item per flag, for example --command python --command server.py (repeatable)
      --env stringArray               environment variable NAME=value for the container (repeatable). Not for secrets: they would show in your shell history; use --secret-env
      --gpu stringArray               only use this GPU model, for example H100 (repeatable, unordered: the cheapest fitting one is rented). Default: any
      --gpu-counts string             own image: GPU counts a replica may use, comma separated from 1,2,4,8 (default 1,2,4,8)
  -h, --help                          help for update
      --image string                  switch to this container image
      --max int                       maximum replicas (default from the platform)
      --max-price string              highest total price per replica-hour in dollars (all its GPUs), for example 2.50. Default: none
      --min int                       minimum replicas (default from the platform)
      --min-gpu-memory float          own image: minimum total GPU memory per replica in GB
      --monthly-cap string            monthly spend cap in dollars (UTC calendar month)
      --placement string              eligible_hosts (default) or own_hosts
      --replica-web-terminal string   web terminal on the replicas: off, organization or creator-and-owners (applies in place)
      --revision int                  refuse the change unless the service is at this revision
      --scale-to-zero                 let the service scale down to no replicas when idle (cold requests then wait for a replica)
      --secret-env stringArray        secret environment variable NAME (repeatable). The value is read from stdin (one line per secret) or a hidden prompt, stored encrypted and never shown again
      --setting stringArray           model setting key=value for a catalog model, for example max_model_len=32768 (repeatable; see the catalog entry's settings)
      --source stringArray            allowed source CIDR, for example 203.0.113.0/24 (repeatable)
      --spot                          use spot (interruptible) capacity: cheaper, but replicas can be preempted
      --target-concurrency int        concurrent requests per replica before another is added
      --unset-env stringArray         remove this environment variable (repeatable)
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

