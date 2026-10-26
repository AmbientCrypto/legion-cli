# legion

The command line for [Ambient Legion](https://legion.ambient.xyz) (Ambient GPU Rentals): find GPUs,
rent them, SSH in, and stop billing when you are done.

```
$ legion version
$ legion auth login
$ legion offer list --gpu-model "NVIDIA GeForce RTX 4090"
$ legion rental create --offer amber-falcon --catalog pytorch --gpus 2
$ legion ssh quiet-river-7k2f
$ legion rental stop quiet-river-7k2f
```

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/AmbientCrypto/legion-cli/main/scripts/install.sh | sh
```

`install.sh` detects your OS and CPU, downloads the release archive, verifies its sha256 against
`checksums.txt` and installs `legion` to `~/.local/bin` (`LEGION_INSTALL_DIR` to change,
`LEGION_VERSION` to pin). On Windows, download the `.zip` from the releases page. Homebrew will
follow once a public tap exists (see the commented `brews` block in `.goreleaser.yaml`).

Check it worked with `legion version`. Until the releases are public, the installer and `go install`
need read access to the `AmbientCrypto/legion-cli` repository (set `LEGION_GITHUB_TOKEN` for the installer).

From source: `go install github.com/AmbientCrypto/legion-cli/cmd/legion@latest`
(needs `GOPRIVATE=github.com/AmbientCrypto/*` while the API client module is private).

## Quickstart

1. **Log in.** `legion auth login` opens <https://legion.ambient.xyz/settings/api-keys>; create a key
   with scope **full**, paste it at the hidden prompt. In CI: `echo "$KEY" | legion auth login --with-token`
   or just set `LEGION_API_KEY`.
2. **Find GPUs.** `legion offer list --max-price 2.50` (prices are per GPU-hour). `legion catalog list`
   shows the starter images and premade models you can run.
3. **Rent.** `legion rental create --offer <host> --catalog <id>`. legion shows a preview (host, GPUs,
   price per GPU-hour and per hour, your balance, image, SSH key), asks you to confirm, then waits until
   the rental is running. `--dry-run` shows the preview and stops; `--async` does not wait.
   The price you saw is sent as the most you will pay (`--max-price` to allow more); if it rose, nothing is rented.
4. **Connect.** `legion ssh <rental>` (or `legion ssh <rental> -- nvidia-smi`). `legion rental ssh-config <rental>`
   prints an `~/.ssh/config` block so plain `ssh`, `scp` and `rsync` work.
5. **Stop billing.** `legion rental stop <rental>` ends GPU billing and keeps the workspace (billed for
   storage per GiB until you `legion rental delete` it, which destroys it for good).

A `<rental>` is an id, a friendly name or any unique prefix. On a terminal, leave it out to pick from a list.

## Commands

```
legion auth login|logout|status        legion whoami
legion config get|set                  legion profile list|create|use|delete
legion catalog list|get                legion offer list|get
legion rental create|list|get|stop|start|delete|wait|ssh|ssh-config|charges|serve
legion rental ingress list|add|remove  legion rental backup estimate|create|list|get|delete|cancel|download-link
legion rental export create|list|get|cancel
legion ssh <rental>                    legion ssh-key list|add|remove
legion billing balance|charges|activity|ledger
legion api-key list|create|revoke      legion completion bash|zsh|fish|powershell
legion version                         legion help environment|exit-codes
```

Every command has `--help` with examples; the generated reference is in [`docs/reference`](docs/reference).
Nouns are singular with plural aliases (`legion rentals list`).

## Configuration

Precedence, first match wins: **flag > environment > profile > default** (also `legion help environment`).

| Setting | Flag | Environment | Profile key | Default |
|---|---|---|---|---|
| profile | `-p/--profile` | `LEGION_PROFILE` | `current_profile` | `default` |
| API URL | `--api-url` | `LEGION_API_URL` | `api_url` | `https://api.legion.ambient.xyz` |
| output | `-o/--output` | `LEGION_OUTPUT` | `output` | `table` |
| API key | (none, so it stays out of `ps`) | `LEGION_API_KEY` | credentials | log in |
| config directory | | `LEGION_CONFIG_DIR` | | user config dir + `/legion` |

Files: `config.yaml` (profiles) and `credentials.yaml` (API keys, mode `0600`, directory `0700`; legion
warns if the permissions are looser). Profiles: `legion profile create staging --api-url https://...`,
`legion profile use staging`, or `-p staging` per command. `legion auth status` shows what is in effect;
the API URL must be https (http only for localhost).

Other global flags: `--no-color` (also `NO_COLOR`, and automatic when not a terminal), `-q/--quiet`
(results only), `--debug` (method, path, status and duration of each request on stderr; never the key),
`--timeout` (per request), `-y/--yes`.

## Output

`-o table` (default), `wide` (more columns), `json`, `yaml`, `name` (one name per line, for scripts),
`jsonpath=<expr>`:

```sh
legion rental list -o json | jq '.[].name'
legion rental list -o 'jsonpath=$[?(@.state=="running")].name'
legion rental get quiet-river-7k2f -o jsonpath=$.ssh.host
```

Results go to stdout; progress, prompts and "what to do next" hints go to stderr (progress only on a
terminal), so machine formats stay clean. Money is US dollars, shown from the API's minor units ($2.50).

## Completion

```sh
source <(legion completion bash)                          # ~/.bashrc
legion completion zsh > "${fpath[1]}/_legion"             # then restart zsh; needs compinit
                                                          # (echo "autoload -U compinit; compinit" >> ~/.zshrc)
legion completion fish > ~/.config/fish/completions/legion.fish
legion completion powershell | Out-String | Invoke-Expression
```

Completion suggests live rental names, host names, catalog ids, saved SSH key labels and GPU models
(short timeout; silent when offline or logged out).

## Exit codes

| Code | Meaning |
|---|---|
| 0 | success |
| 1 | error (including a suspended organization or a feature turned off for it) |
| 2 | usage error (bad flag or argument, a request the API rejects as invalid; nothing to confirm with and no `--yes`) |
| 3 | not logged in, or the key is invalid, expired or lacks scope |
| 4 | not found |
| 5 | conflict or price changed |
| 6 | insufficient credit |
| 130 | interrupted |

Failures print `Error:`, `Code:` (the API's stable error code) and `Hint:` (the next command) on stderr.

## Scripts and automation

Commands that spend or destroy need confirmation. With no terminal and no `--yes` they refuse (exit 2)
instead of guessing. `rental create` needs `--offer`, an image (`--image` or `--catalog`) and `--yes`.
Creating a rental sends a fresh random `Idempotency-Key`.

## Development

```sh
export GOPRIVATE='github.com/AmbientCrypto/*'
go test -race ./... && go vet ./...
go run ./tools/gendocs docs/reference   # regenerate the reference; CI checks it is current
```

The API client is the generated `github.com/AmbientCrypto/legion-controller/api`, pinned in `go.mod`
(never copy the spec or re-run codegen here). See `AGENTS.md` for conventions.
