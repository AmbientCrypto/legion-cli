## legion rental

Create, connect to, stop and delete rentals

### Synopsis

A rental is a container on a host's GPUs, billed per second while it runs.
A rental is named by id, friendly name (quiet-river-7k2f) or any unique prefix of either.
Leave the name out on a terminal to pick from a list.

```
legion rental [flags]
```

### Examples

```
  legion rental create --offer <host-id> --catalog <id>
  legion rental list
```

### Options

```
  -h, --help   help for rental
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
* [legion rental backup](legion_rental_backup.md)	 - Back up a rental's workspace and download it
* [legion rental charges](legion_rental_charges.md)	 - Show what one rental has cost so far
* [legion rental create](legion_rental_create.md)	 - Rent GPUs and start a container
* [legion rental delete](legion_rental_delete.md)	 - Delete a rental and destroy its workspace
* [legion rental export](legion_rental_export.md)	 - Export a rental's workspace to a download
* [legion rental get](legion_rental_get.md)	 - Show one rental
* [legion rental ingress](legion_rental_ingress.md)	 - Expose a port of a rental on the internet
* [legion rental list](legion_rental_list.md)	 - List your rentals
* [legion rental serve](legion_rental_serve.md)	 - Show the endpoint and API key of a premade model rental
* [legion rental ssh](legion_rental_ssh.md)	 - SSH into a rental
* [legion rental ssh-config](legion_rental_ssh-config.md)	 - Print an ~/.ssh/config block for a rental
* [legion rental start](legion_rental_start.md)	 - Start a stopped rental
* [legion rental stop](legion_rental_stop.md)	 - Stop the workload; keep the workspace
* [legion rental wait](legion_rental_wait.md)	 - Wait until a rental is running or stopped

