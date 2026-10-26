## legion

Rent GPUs on Ambient Legion

### Synopsis

legion rents GPUs on Ambient Legion (Ambient GPU Rentals): find an offer,
start a rental, SSH in, and stop billing when you are done.

Quickstart:
  legion auth login                     # sign in through your browser
  legion offer list --gpu-model "RTX 4090"      # case-insensitive substring of the model name
  legion rental create --offer <host-id> --catalog <id>
  legion ssh <rental>                   # connect;  legion rental stop <rental>  to stop billing

Settings come from flags, then environment, then the profile, then defaults:
see 'legion help environment'. Exit codes: 'legion help exit-codes'.

```
legion [flags]
```

### Options

```
      --api-url string     API base URL (env LEGION_API_URL)
      --debug              log each request's method, path, status and duration to stderr
  -h, --help               help for legion
      --no-color           disable colour (also NO_COLOR)
  -o, --output string      output format: table|wide|json|yaml|name|jsonpath=<expr> (env LEGION_OUTPUT)
  -p, --profile string     profile to use (env LEGION_PROFILE)
  -q, --quiet              print only results, no progress or next-step hints
      --timeout duration   timeout for each API request (default 30s)
  -y, --yes                answer yes to confirmation prompts
```

### SEE ALSO

* [legion api-key](legion_api-key.md)	 - Manage the API keys that sign in to Legion
* [legion auth](legion_auth.md)	 - Log in and out, and see who you are logged in as
* [legion billing](legion_billing.md)	 - See your balance, charges and ledger
* [legion catalog](legion_catalog.md)	 - Browse Legion's starter images and premade models
* [legion completion](legion_completion.md)	 - Generate a shell completion script
* [legion config](legion_config.md)	 - Read and change settings of the current profile
* [legion offer](legion_offer.md)	 - Browse GPU offers from hosts
* [legion profile](legion_profile.md)	 - Manage named profiles (API URL, key, default output)
* [legion rental](legion_rental.md)	 - Create, connect to, stop and delete rentals
* [legion ssh](legion_ssh.md)	 - SSH into a rental (shortcut for 'legion rental ssh')
* [legion ssh-key](legion_ssh-key.md)	 - Save the SSH public keys you rent with
* [legion update](legion_update.md)	 - Update legion to the latest release
* [legion version](legion_version.md)	 - Print the legion version
* [legion whoami](legion_whoami.md)	 - Show the user and organization you are logged in as

