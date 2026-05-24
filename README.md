# zeltapp-cli

[![CI](https://github.com/agentic-utils/zeltapp-cli/actions/workflows/ci.yaml/badge.svg)](https://github.com/agentic-utils/zeltapp-cli/actions/workflows/ci.yaml)
[![Go Reference](https://pkg.go.dev/badge/github.com/agentic-utils/zeltapp-cli.svg)](https://pkg.go.dev/github.com/agentic-utils/zeltapp-cli)
[![Go Report Card](https://goreportcard.com/badge/github.com/agentic-utils/zeltapp-cli)](https://goreportcard.com/report/github.com/agentic-utils/zeltapp-cli)
[![Release](https://img.shields.io/github/v/release/agentic-utils/zeltapp-cli?sort=semver)](https://github.com/agentic-utils/zeltapp-cli/releases/latest)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

kubectl-style unofficial CLI for [Zelt](https://zelt.app), reverse-engineered from the web app's `/apiv2/...` surface.

cookie-based session with email MFA, macOS Keychain credential storage, on-disk TTL cache, and table/json/yaml/wide/name output formats. session lives at `~/.config/zeltapp-cli/session.json` (chmod 600).

## install

**homebrew** (macOS / linux):

```
brew install agentic-utils/tap/zeltapp
```

**go install**:

```
go install github.com/agentic-utils/zeltapp-cli/cmd/zeltapp@latest
```

**from source**:

```
git clone https://github.com/agentic-utils/zeltapp-cli && cd zeltapp-cli
go install ./cmd/zeltapp
```

**prebuilt binaries**: see [releases](https://github.com/agentic-utils/zeltapp-cli/releases).

## usage

each top-level command is a **resource**; subcommands are **verbs** on it (e.g. `zeltapp people list`, `zeltapp absence book`).

### auth

```
zeltapp auth login                   # prompts for email, password, MFA; saves password to Keychain
zeltapp auth login --remember=false  # don't persist the password
zeltapp auth whoami
zeltapp auth logout [--forget]       # --forget also wipes the Keychain entry
```

### people

```
zeltapp people list                                 # active members
zeltapp people list --all                           # include inactive/terminated
zeltapp people list -l department=Engineering,site=London
zeltapp people get me                               # or numeric id, or email
zeltapp people get alice@example.com                # case-insensitive email match
zeltapp people search "alice"                       # substring across name/email/dept/role
```

### absence

```
zeltapp absence list [--calendar past|current|future]
zeltapp absence policies
zeltapp absence balance
zeltapp absence check --policy 512 --start 2026-06-01     # dry-run (overlap + cost)
zeltapp absence book  --policy 512 --start 2026-06-01     # interactive confirm
zeltapp absence book  --policy 512 --start 2026-06-01 --yes
```

### finance / money

```
zeltapp payslip list                                # payrolls + payslips
zeltapp compensation show
zeltapp equity show
zeltapp pension show
zeltapp benefit list
zeltapp contract list                               # history + current
zeltapp contract current                            # current only
zeltapp expense list
zeltapp invoice list
```

each accepts `--user <id|email|me>` to read someone else's (subject to permissions).

### ops

```
zeltapp device list                                 # assigned + in-transit + orders
zeltapp attendance show [--date YYYY-MM-DD] [--user me]
zeltapp calendar show [--start YYYY-MM-DD] [--end YYYY-MM-DD]
```

### performance

```
zeltapp review list                                 # ongoing cycles
zeltapp review get <uuid>                           # one cycle
zeltapp review describe <uuid>                      # cycle + progress + participation in one
zeltapp review progress <uuid>
zeltapp review participation <uuid>
zeltapp review result <uuid> [--user me|<id>]
zeltapp review entry [--user me|<id>]
zeltapp goal list [--user me|<id>]
```

### company / org

```
zeltapp company show                                # config
zeltapp company describe                            # config + settings + departments + sites + jobs
zeltapp company department list
zeltapp company site list
zeltapp company job-position list
```

### config

```
zeltapp config view                                 # current session, scopes, paths
zeltapp config cache list
zeltapp config cache clear
```

### escape hatch

```
zeltapp raw GET  /apiv2/users/cache
zeltapp raw POST /apiv2/foo '{"x":1}'
```

### global flags

```
-o, --output FORMAT   table|json|yaml|wide|name (default: table)
    --json            shorthand for -o json
-v, --verbose         log request/response to stderr
    --no-cache        bypass the local TTL cache for this invocation
```

## output formats

```
$ zeltapp get absence-policies
ID   NAME           TYPE
512  Annual Leave   annual
513  Sick Leave     sick

$ zeltapp get absence-policies -o wide       # extra columns
ID   NAME           TYPE    ACCRUAL  CAP
512  Annual Leave   annual  monthly  25
513  Sick Leave     sick    none     -

$ zeltapp get absence-policies -o json | jq '.[] | select(.type=="sick")'
{"id":513,"name":"Sick Leave","type":"sick"}

$ zeltapp get absence-policies -o yaml
- id: 512
  name: Annual Leave
  type: annual

$ zeltapp get people -o name
Alice Smith
Bob Jones
```

## privacy / telemetry

**zeltapp sends no telemetry, no usage metrics, no anonymous pings.** The only outbound network calls are:

- `https://go.zelt.app/apiv2/*` — your Zelt instance, only when you invoke a command that needs the API
- (planned, currently absent) `api.github.com/repos/agentic-utils/zeltapp-cli/releases/latest` — for an opt-in `zeltapp upgrade` check, throttled to 24h

No PostHog, no Sentry, no analytics. You can verify by grepping the source: there are no third-party network destinations hard-coded.

## exit codes

`zeltapp` follows the convention used by `twoctl` / `twoadm`:

| code | meaning                                          |
| ---- | ------------------------------------------------ |
| 0    | success                                          |
| 1    | generic failure                                  |
| 2    | usage error (bad flag, missing argument)         |
| 3    | authentication / authorization (401/403)         |
| 4    | not found (404)                                  |
| 5    | rate limited (429) — after retries are exhausted |
| 6    | server error (5xx) — after retries are exhausted |
| 7    | network / transport error                        |

## reliability

- **Retries:** GET / write requests that return 429, 408, or 5xx are retried up to 3 times with exponential backoff + full jitter (500 ms base, 10 s cap). `Retry-After` is honoured when present.
- **Idempotency:** every POST / PUT / PATCH / DELETE sends a fresh `Idempotency-Key` so a retried write never creates a duplicate.
- **Request tracing:** the upstream request id (`X-Request-Id` / `X-Trace-Id` / `X-Amz-Cf-Id` / `Cf-Ray`, whichever is present) is captured and surfaced in error messages as `(request_id=…)` for support tickets.
- **Body cap:** responses are read up to 4 MB so a misbehaving server can't OOM the CLI.
- **Timeout:** 60 s default per request.

## pagination

Endpoints that paginate (`absence list`, `expense list`) default to a single page. Use:

```
--all              # drain every page
--no-paginate      # explicit single page (default behaviour, makes intent clear in scripts)
--page-size N      # upstream page size (default 50)
--limit N          # cap on total items across all pages (0 = no cap)
```

Pages are detected by the `items` / `data` / `results` / `rows` / `absences` shape and stop when a short page is returned. `-o json|yaml` emit the concatenated items; `-o table` does too with the inferred columns.

## label selectors

`-l`/`--selector` works on listed resources where columns map naturally to label keys. values match as case-insensitive substrings, keys are exact (case-insensitive).

```
zeltapp get people -l department=Engineering
zeltapp get people -l site=London,jobPosition=engineer
```

## shell completion

zeltapp ships completion scripts via cobra. install once per shell:

**zsh** (most common on macOS):

```
mkdir -p ~/.zsh/completions
zeltapp completion zsh > ~/.zsh/completions/_zeltapp
# add to ~/.zshrc if not already:
#   fpath=(~/.zsh/completions $fpath)
#   autoload -Uz compinit && compinit
```

restart your shell or run `compinit` to pick it up.

**bash**:

```
zeltapp completion bash > /usr/local/etc/bash_completion.d/zeltapp     # macOS via brew install bash-completion@2
# or:
zeltapp completion bash > /etc/bash_completion.d/zeltapp               # linux
```

**fish**:

```
zeltapp completion fish > ~/.config/fish/completions/zeltapp.fish
```

**powershell**:

```
zeltapp completion powershell > zeltapp.ps1
# then source zeltapp.ps1 from your profile
```

## notes

- email MFA: zelt emails a 6-digit code; paste it at the prompt.
- access token expires after 15 min; refresh token lasts 30 days. The CLI auto-refreshes when the access token expires.
- password is stored in macOS Keychain by default (service=`zeltapp-cli`). If the refresh token also expires (~30 days idle), the CLI re-logs in using the stored password and prompts only for the new MFA code. Pass `--remember=false` to `auth login` to opt out.
- `create absence` runs `verify-overlap` + `request-value-and-balance2` first and shows a confirmation prompt unless `--yes` is passed.
- some endpoints are cached on disk at `~/.config/zeltapp-cli/cache/` with per-endpoint TTLs (`users/cache` 5 min, `companies/*` and `job-positions` 1 hour, etc.). Pass `--no-cache` to bypass for one call, or run `zeltapp config cache clear` to wipe everything. User-specific endpoints (`/users/<id>/*`, `auth/me`, absences, expenses, payroll) are never cached.
