# legion

**Rent GPUs from your terminal.** Find an offer, rent it, SSH in and stop billing, all with `legion`, the command line for [Ambient Legion](https://legion.ambient.xyz) (Ambient GPU Rentals).

```sh
curl -fsSL https://raw.githubusercontent.com/AmbientCrypto/legion-cli/main/scripts/install.sh | bash
```

| | |
|---|---|
| **Docs** | <https://legion.ambient.xyz/docs/cli> |
| **Legion** | <https://legion.ambient.xyz> |
| **API docs** | <https://legion.ambient.xyz/docs/api/overview> |
| **Create an API key** | <https://legion.ambient.xyz/settings/api-keys> |
| **Support** | <https://legion.ambient.xyz/support> |

## Install

`install.sh` detects your OS and CPU, verifies the release's sha256 against `checksums.txt` and installs `legion` to `~/.local/bin` (`LEGION_INSTALL_DIR` to change, `LEGION_VERSION` to pin). On Windows, download the `.zip` from the [releases page](https://github.com/AmbientCrypto/legion-cli/releases). Upgrade with `legion update` (it checks the download against `checksums.txt` before it replaces the binary); on a terminal legion tells you, at most once a day, when a newer release is out (`LEGION_NO_UPDATE_CHECK=1` turns that off). Every merge to `main` is released.

## Quickstart (60 seconds)

```sh
legion auth login                                   # opens your browser; approve "Legion CLI" and you are signed in
legion offer list --gpu-model "RTX 4090"            # cheapest first, prices per GPU-hour
legion rental create --offer amber-falcon --catalog pytorch --gpus 2   # preview, confirm, wait until running
legion ssh quiet-river-7k2f                         # connect (add -- nvidia-smi to run one command)
legion rental stop quiet-river-7k2f                 # ends GPU billing, keeps the workspace
legion rental delete quiet-river-7k2f               # destroys the workspace for good
```

- `auth login` saves an API key named `Legion CLI (<this computer>)` (scope full, 90 days); revoke it in Settings > API keys or with `legion auth logout`. On a remote machine with no local browser use `legion auth login --paste`; in CI pipe a key to `legion auth login --with-token` or set `LEGION_API_KEY`.
- `rental create` shows a preview (host, GPUs, price per GPU-hour and per hour, balance, image, SSH key) and asks first. `--dry-run` stops there; `--async` does not wait. The price you saw is the most you pay (`--max-price` to allow more); if it rose, nothing is rented.
- A `<rental>` is an id, a friendly name or any unique prefix. On a terminal, leave it out to pick from a list.
- `legion rental ssh-config <rental>` prints an `~/.ssh/config` block so `ssh`, `scp` and `rsync` work.
- `legion catalog list` shows the starter images and premade models.

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
legion version|update                  legion help environment|exit-codes
```

Every command has `--help` with examples. **Every flag is in the generated reference: [`docs/reference`](docs/reference).** Nouns are singular with plural aliases (`legion rentals list`).

## Configuration

Precedence, first match wins: **flag > environment > profile > default** (see `legion help environment`).

| Setting | Flag | Environment | Default |
|---|---|---|---|
| profile | `-p/--profile` | `LEGION_PROFILE` | `default` |
| API URL | `--api-url` | `LEGION_API_URL` | `https://api.legion.ambient.xyz` |
| output | `-o/--output` | `LEGION_OUTPUT` | `table` |
| API key | none (keeps it out of `ps`) | `LEGION_API_KEY` | log in |
| config directory | | `LEGION_CONFIG_DIR` | user config dir + `/legion` |

Settings live in `config.yaml` and keys in `credentials.yaml` (mode `0600`). `legion auth status` shows what is in effect. For CI, skip login: set `LEGION_API_KEY`, or `echo "$KEY" | legion auth login --with-token`. See [`legion profile`](docs/reference/legion_profile.md) and [`legion config`](docs/reference/legion_config.md).

## Output and scripting

`-o table` (default), `wide`, `json`, `yaml`, `name` (one name per line) or `jsonpath=<expr>`:

```sh
legion rental list -o json | jq '.[].name'
legion rental list -o 'jsonpath=$[?(@.state=="running")].name'
```

Results go to stdout; progress, prompts and hints go to stderr, so machine formats stay clean. Commands that spend or destroy need confirmation: with no terminal and no `--yes` they refuse (exit 2). `rental create` in a script needs `--offer`, an image (`--image` or `--catalog`) and `--yes`.

## Completion

```sh
source <(legion completion bash)                          # ~/.bashrc
legion completion zsh > "${fpath[1]}/_legion"             # then restart zsh; needs compinit
legion completion fish > ~/.config/fish/completions/legion.fish
```

Completion suggests live rental names, host names, catalog ids, SSH key labels and GPU models. Details: [`legion completion`](docs/reference/legion_completion.md).

## Exit codes

| Code | Meaning |
|---|---|
| 0 | success |
| 1 | error (including a suspended organization or a feature turned off for it) |
| 2 | usage error (bad flag or argument, a request the API rejects as invalid, or nothing to confirm with and no `--yes`) |
| 3 | not logged in, or the key is invalid, expired or lacks scope |
| 4 | not found |
| 5 | conflict or price changed |
| 6 | insufficient credit |
| 130 | interrupted |

Failures print `Error:`, `Code:` and `Hint:` (the next command to run) on stderr.

## Development

```sh
export GOPRIVATE='github.com/AmbientCrypto/*'
go build -o legion ./cmd/legion
go test -race ./... && go vet ./...
go run ./tools/gendocs docs/reference   # regenerate the reference; CI checks it is current
```

Building from source needs access to the private API client module, `github.com/AmbientCrypto/legion-controller/api` (pinned in `go.mod`; never copy the spec or re-run codegen here). Release binaries need none of this. See `AGENTS.md` for conventions.
