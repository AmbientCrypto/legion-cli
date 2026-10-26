## legion offer list

List offers, cheapest first

### Synopsis

List hosts with free GPUs, filtered and sorted. Prices are per GPU-hour. The default sort is price
(cheapest first); others: score, reliability, gpu_memory, price_per_tflops. Use -o wide for host ids and
hardware, -o name for host names. --catalog shows only hosts that can run that starter image or model.

```
legion offer list [flags]
```

### Examples

```
  legion offer list
  legion offer list --gpu-model "NVIDIA GeForce RTX 4090" --min-gpus 2 --max-price 2.50
  legion offer list --catalog qwen3-5-4b --sort reliability -o wide
```

### Options

```
      --catalog string          only hosts that can run this catalog id
      --country stringArray     host country code, e.g. US (repeatable)
      --gpu-model stringArray   GPU model, a case-insensitive part of its name, e.g. "rtx 4090" (repeatable)
  -h, --help                    help for list
      --limit int               most offers to list (default 50)
      --max-latency int         most round-trip milliseconds to --region (or to the host's nearest region)
      --max-price string        most per GPU-hour in dollars, e.g. 2.50
      --min-gpus int            at least this many free GPUs
      --region string           reference region for latency: us-east-1, us-west-2, eu-central-1, ap-northeast-1, ap-southeast-1
      --sort string             price (default), score, reliability, gpu_memory, price_per_tflops
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

* [legion offer](legion_offer.md)	 - Browse GPU offers from hosts

