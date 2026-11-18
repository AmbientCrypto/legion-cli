## legion endpoints

Serve models and ports behind one URL with access keys

### Synopsis

Endpoints are served models and exposed ports behind one URL and one set of access keys.
An endpoint is either a single rental (scaling off) or an autoscaled pool of replicas (scaling auto);
both keep the same URL and keys when you switch with 'legion endpoints autoscale on|off'.

Replicas bill like rentals, per GPU-hour from start, including loading. Every create and spend change shows
a cost floor and ceiling and asks for confirmation (or pass --yes). The monthly cap (UTC calendar month)
stops serving until the next month or until you raise it. A replica is what Vast and RunPod call a worker.

```
legion endpoints [flags]
```

### Examples

```
  legion endpoints list
  legion endpoints create chat --catalog llama-3-8b --max-price 2.50 --monthly-cap 400
  legion endpoints keys create chat --name ci --expires 30d
```

### Options

```
  -h, --help   help for endpoints
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

* [legion](legion.md)	 - Rent GPUs on Ambient Legion
* [legion endpoints autoscale](legion_endpoints_autoscale.md)	 - Turn autoscaling on or off for an endpoint
* [legion endpoints create](legion_endpoints_create.md)	 - Create an endpoint: expose a rental's port, or serve a model on autoscaled GPUs
* [legion endpoints delete](legion_endpoints_delete.md)	 - Delete an endpoint, its replicas and its keys
* [legion endpoints events](legion_endpoints_events.md)	 - Show scaling and configuration history
* [legion endpoints get](legion_endpoints_get.md)	 - Show one endpoint
* [legion endpoints keys](legion_endpoints_keys.md)	 - Manage an endpoint's access keys
* [legion endpoints list](legion_endpoints_list.md)	 - List endpoints
* [legion endpoints pause](legion_endpoints_pause.md)	 - Stop serving and release the replicas
* [legion endpoints replicas](legion_endpoints_replicas.md)	 - List an endpoint's replicas
* [legion endpoints resume](legion_endpoints_resume.md)	 - Start serving again
* [legion endpoints update](legion_endpoints_update.md)	 - Change an endpoint's access, scaling, hardware, price ceiling or cap
* [legion endpoints usage](legion_endpoints_usage.md)	 - Show requests, errors, replica-hours and cost per day

