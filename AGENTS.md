# Agent guidance — legion-cli

Read by Claude Code (CLAUDE.md is a symlink to this file), Codex, and other coding agents.

`legion` is the customer-facing command line for Ambient Legion (Ambient GPU Rentals): rent GPUs, SSH in, stop billing. Jira project **LGN** (epic LGN-481). It is a thin client: every request goes through the generated Go client `github.com/AmbientCrypto/legion-controller/api`, pinned in `go.mod`. Never copy the OpenAPI spec, never re-run codegen, never hand-write request/response structs the api package already has. The spec is `git show <tag>:api/openapi.yaml` in the legion-controller repo.

- Project key is **LGN** (e.g. `LGN-481`, branch `lgn-481-cli`): wherever the conventions below say ENG, use LGN. Put the key in every commit subject.
- Layout: `cmd/legion` (main), `internal/cli` (commands), `internal/client` (auth header, UA, debug log, problem+json), `internal/config`, `internal/output`, `internal/ui`.
- TDD: tests first, against an `httptest` fake controller. `go test -race ./...` and `go vet ./...` must pass.
- `docs/reference/*.md` is generated: `go run ./tools/gendocs docs/reference`; CI fails when it is stale.
- Never print or log the API key or the `Authorization` header.
- Bumping the controller API: `go get github.com/AmbientCrypto/legion-controller/api@<tag>`, fix compile errors, run the tests.
- Private module: `export GOPRIVATE='github.com/AmbientCrypto/*'`.

## Jira & branch conventions (Atlassian-enabled)

All engineering work is tracked in the Jira project **ENG** — one project for all of engineering, kanban board. Full process: https://ambientcrypto.github.io/engineering-process/

- **No ticket, no work.** If asked to do something with no ENG issue, create one first via the Atlassian MCP (title + one sentence of context is enough), and post the issue link back into the conversation or Slack thread.
- **Branches carry the issue key:** `eng-142-short-slug` (lowercase key + short slug). GitHub-for-Jira auto-links the branch and PR to the ticket — no key in the branch means no linkage.
- **PR titles start with the key:** `ENG-142: what changed`.
- **When a PR opens,** comment a 2–4 line summary on the Jira issue: what changed and how it will be verified.
- **Statuses you may set:** To Do → In Progress (when work starts) → In Review (PR open) → Staged (merged and running in staging/tooling). **Never move a ticket to Released** — that is a human verification step (live in prod, acceptance criteria confirmed).
- **Labels:** `ktlo` for operational/interrupt work, `incident-followup` for postmortem action items.
- **Only clickable links.** Every reference you write into a Jira ticket, Jira comment, PR description, PR comment, commit message or Slack post must be a full `https://` URL the reader can open from their own browser. Use Markdown link syntax, e.g. `[ENG-142](https://ambientcrypto.atlassian.net/browse/ENG-142)`. The reader is not in your terminal and has not checked out your branch.
  - **Do:** GitHub PR, issue, commit and file links, using a commit-SHA permalink with a `#L12` line anchor for code (`https://github.com/AmbientCrypto/<repo>/blob/<sha>/path/to/file.go#L12`). Also Jira and Confluence URLs, and Grafana or other dashboard URLs the team normally reaches.
  - **Don't:** local paths (`/Users/...`, `~/...`, `file://`), repo-relative paths like `internal/foo.go:12`, bare `#76` / `PR 76` / `owner/repo#76`, short SHAs, `localhost` or `kubectl port-forward` URLs, pod, node or tailnet IPs, and files that only exist in your workspace or scratchpad.
  - If there is no clickable URL for something, such as a log line or command output, paste the relevant excerpt inline instead of linking to it.
- **Epic briefs:** an epic's description holds the five-part brief (problem, outcome, acceptance criteria, non-goals, size). Don't start epic work whose brief is missing — flag it instead.
