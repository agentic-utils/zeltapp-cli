# zeltapp-cli

[![CI](https://github.com/agentic-utils/zeltapp-cli/actions/workflows/ci.yaml/badge.svg)](https://github.com/agentic-utils/zeltapp-cli/actions/workflows/ci.yaml)
[![Go Report Card](https://goreportcard.com/badge/github.com/agentic-utils/zeltapp-cli)](https://goreportcard.com/report/github.com/agentic-utils/zeltapp-cli)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

unofficial CLI for [Zelt](https://zelt.app). reverse-engineered from the web app's `/apiv2/...` surface.

cookie-based session, email MFA prompted interactively on `login`. session lives at `~/.config/zeltapp-cli/session.json` (chmod 600).

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

```
zeltapp login                   # prompts for email, password, MFA code; saves password to Keychain
zeltapp login --remember=false  # same, but don't save the password
zeltapp whoami
zeltapp logout

zeltapp me profile            # basic + personal + about
zeltapp me contact            # address + emergency + work contact
zeltapp me employment         # role + contracts + lifecycle
zeltapp me compensation
zeltapp me bank
zeltapp me equity
zeltapp me pension
zeltapp me payslips
zeltapp me devices
zeltapp me benefits

zeltapp leave list
zeltapp leave balance
zeltapp leave book --policy 512 --start 2026-06-01 [--end 2026-06-02] [--notes "..."]
zeltapp leave check --policy 512 --start 2026-06-01    # dry-run, shows cost + overlaps

zeltapp attendance today
zeltapp attendance week

zeltapp calendar team [--start YYYY-MM-DD] [--end YYYY-MM-DD]

zeltapp expenses list

zeltapp company config
zeltapp company departments
zeltapp company sites
zeltapp company jobs

zeltapp people list                              # company directory (active only)
zeltapp people list --all                        # include inactive/terminated
zeltapp people search "alice"                    # case-insensitive substring across name/email/dept/role
zeltapp people get 6380                          # one person by userId
zeltapp people get alice@example.com             # or by email

zeltapp reviews list                             # ongoing review cycles
zeltapp reviews mine                             # my results across cycles
zeltapp reviews cycle <uuid>                     # details + navigation for a cycle
zeltapp reviews progress <uuid>                  # cycle + result progress
zeltapp reviews participation <uuid>             # participants in a cycle
zeltapp reviews result <uuid> [--user N]         # user-level result (default: self)
zeltapp reviews entry [--user N]                 # review entry (default: self)

zeltapp goals list
zeltapp goals mine

zeltapp cache list                               # show cached entries + TTLs
zeltapp cache clear                              # wipe local cache

zeltapp raw GET /apiv2/users/cache               # escape hatch
zeltapp raw POST /apiv2/foo '{"x":1}'
```

global flags:

```
--json          JSON output instead of human-readable tables
-v, --verbose   print request/response info to stderr
--no-cache      bypass the local TTL cache for this invocation
```

## output format

by default, output is rendered as aligned tables (for lists) or key=value (for single records). pass `--json` to get the raw JSON response instead - useful for piping into `jq` or scripting.

```
$ zeltapp leave policies
ID   NAME           TYPE
512  Annual Leave   annual
513  Sick Leave     sick

$ zeltapp leave policies --json
[
  {"id": 512, "name": "Annual Leave", "type": "annual"},
  ...
]
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
- password is stored in macOS Keychain by default (service=`zeltapp-cli`). If the refresh token also expires (~30 days idle), the CLI re-logs in using the stored password and prompts only for the new MFA code. Pass `--remember=false` to login to opt out.
- `leave book` will run `verify-overlap` + `request-value-and-balance2` first and show a confirmation prompt unless `--yes` is passed.
- some endpoints are cached on disk at `~/.config/zeltapp-cli/cache/` with per-endpoint TTLs (e.g. `users/cache` 5 min, `companies/*` and `job-positions` 1 hour). pass `--no-cache` to bypass for one call, or run `zeltapp cache clear` to wipe everything. User-specific endpoints (`/users/<id>/*`, `auth/me`, absences, expenses, payroll) are never cached.
