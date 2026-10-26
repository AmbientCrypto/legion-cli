## legion rental create

Rent GPUs and start a container

### Synopsis

Rent GPUs on one host and start a container on them. legion first shows a preview - host,
GPUs, price per GPU-hour and per hour, your balance, image and SSH key - and asks you to confirm.
The price you saw is sent as the most you will pay: if the host's price rises first, nothing is
rented and you are told the new price. Billing is per second while the rental runs.

By default legion waits until the rental is running; --async returns at once.
Missing flags are asked for on a terminal. In scripts pass --offer, an image (--image or
--catalog) and --yes. --dry-run prints the preview and exits without renting.

```
legion rental create [flags]
```

### Examples

```
  legion rental create --offer amber-falcon --catalog pytorch --gpus 2
  legion rental create --offer 5f0c1c2e-... --image pytorch/pytorch:2.4.0-cuda12.4-cudnn9-runtime --max-price 2.60
  legion rental create --offer amber-falcon --catalog qwen3-5-4b --dry-run
```

### Options

```
      --async                   return as soon as the rental is created (same as --wait=false)
      --catalog string          starter image or premade model id from 'legion catalog list'
      --dry-run                 print the preview and exit without renting
      --gpus int                number of GPUs (default 1, or what the catalog entry needs)
  -h, --help                    help for create
      --image string            container image, for example pytorch/pytorch:2.4.0-cuda12.4-cudnn9-runtime
      --max-price string        most you will pay per GPU-hour in dollars, for example 2.60 (default: the price shown)
      --offer string            host to rent: id or name from 'legion offer list'
      --ssh-key string          saved SSH key (id, label or fingerprint) or a path to a .pub file
      --wait                    wait until the rental is running (default true)
      --wait-timeout duration   how long to wait for the rental to run (default 10m0s)
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

