## legion services

Give what you run a public URL, with optional service keys

### Synopsis

A service is a public URL for something you run on Legion. Point it at a port on one of your rentals, or let Legion run your model on rented GPUs and scale it with traffic, down to zero when idle.
A service is either on a rental (autoscaling off, one rental) or autoscaled (a pool of replicas);
both keep the same Service URL and service keys when you switch with 'legion services autoscale on|off'.

Replicas bill like rentals, per GPU-hour from start, including loading. Every create and spend change shows
a cost floor and ceiling and asks for confirmation (or pass --yes). The monthly cap (UTC calendar month)
stops serving until the next month or until you raise it. A replica is what Vast and RunPod call a worker.

```
legion services [flags]
```

### Examples

```
  legion services list
  legion services create chat --catalog llama-3-8b --max-price 2.50 --monthly-cap 400
  legion services keys create chat --name ci --expires 30d
```

### Options

```
  -h, --help   help for services
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
* [legion services autoscale](legion_services_autoscale.md)	 - Turn autoscaling on or off for a service
* [legion services create](legion_services_create.md)	 - Create a service: expose a rental's port, or serve a model on autoscaled GPUs
* [legion services delete](legion_services_delete.md)	 - Delete a service, its replicas and its keys
* [legion services events](legion_services_events.md)	 - Show scaling and configuration history
* [legion services get](legion_services_get.md)	 - Show one service
* [legion services keys](legion_services_keys.md)	 - Manage service keys
* [legion services list](legion_services_list.md)	 - List services
* [legion services logs](legion_services_logs.md)	 - Show a service's logs from every replica, or follow them live
* [legion services pause](legion_services_pause.md)	 - Stop serving and release the replicas
* [legion services replicas](legion_services_replicas.md)	 - List a service's replicas
* [legion services resume](legion_services_resume.md)	 - Start serving again
* [legion services update](legion_services_update.md)	 - Change a service's access, scaling, hardware, price ceiling or cap
* [legion services usage](legion_services_usage.md)	 - Show requests, errors, replica-hours and cost per day

