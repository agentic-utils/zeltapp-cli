# Contributing

## Commit messages

This repo uses [Conventional Commits](https://www.conventionalcommits.org/). Every commit on `main` should be prefixed with a type that determines the next semver bump:

| Type        | Bump  | Use for                                  |
| ----------- | ----- | ---------------------------------------- |
| `feat:`     | minor | New user-visible feature                 |
| `fix:`      | patch | Bug fix                                  |
| `docs:`     | patch | Documentation only                       |
| `style:`    | patch | Formatting, gofmt, lint                  |
| `refactor:` | patch | Code change without user-visible effect  |
| `test:`     | patch | Test changes only                        |
| `chore:`    | patch | Build/tooling/housekeeping               |
| `build:`    | patch | Build system / dependency bumps          |
| `perf:`     | patch | Performance improvement                  |
| `ci:`       | none  | CI config only (excluded from changelog) |

Add `!` after the type, or include `BREAKING CHANGE:` in the body, to force a **major** bump:

```
feat!: rename --user flag to --as

BREAKING CHANGE: --user is no longer recognised. Migrate to --as.
```

Subject style (matches the rest of the repo):

- lowercase, no trailing period
- imperative mood ("add X", not "added X")
- one short line; details go in the body
- no Linear ticket prefixes (this is OSS)

## Versioning

Tags are bumped automatically by [`mathieudutour/github-tag-action`](https://github.com/mathieudutour/github-tag-action) on every push to `main`:

- `feat:` → minor (e.g. v0.1.0 → v0.2.0)
- `fix:` / most others → patch (e.g. v0.1.0 → v0.1.1)
- `feat!:` or `BREAKING CHANGE:` → major (e.g. v0.1.0 → v1.0.0)
- If unsure, the action falls back to **patch** (`default_bump: patch`)
- `ci:` commits are excluded from changelogs but still trigger a patch bump unless they're the only change. Prefer `chore:` for tooling tweaks that should appear in the changelog.

### Deprecation policy

Pre-`v1.0.0` (where we are now) the surface is allowed to shift between minors. From `v1.0.0` onward we follow the same rules `twoctl` / `twoadm` settled on (see [INF-1314](https://linear.app/tillit/issue/INF-1314)):

- Renamed commands or flags keep their old name as a **cobra alias** for at least one minor cycle.
- The alias prints a one-line deprecation note to **stderr** so scripts continue to work but humans see the migration hint.
- Breaking changes go in `feat!:` commits and bump the major version.
- `zeltapp version` and `--version` print the same info; `zeltapp version -o json` is the machine-readable form.

The action also produces grouped changelogs (see `.goreleaser.yaml`):

- 🚀 Features
- 🐛 Bug fixes
- 📚 Documentation
- 🛠 Maintenance
- Other

## Release flow

> ⚠ The `push: branches: [main]` trigger on `ci.yaml` is currently commented out while the kubectl-style refactor stabilises. Releases are manual via `workflow_dispatch` until `v0.1.0` ships. Re-enable the push trigger as part of cutting `v0.1.0`.

Every push to `main` runs `ci.yaml` (once re-enabled):

1. `test.yaml` (build + go test + go vet)
2. Tag bump
3. GoReleaser publishes:
   - GitHub release with darwin/linux × amd64/arm64 archives + checksums
   - Homebrew formula push to [`agentic-utils/homebrew-tap`](https://github.com/agentic-utils/homebrew-tap)

### Manual release

The workflow supports `workflow_dispatch` so you can trigger a release without pushing a no-op commit:

```
gh workflow run ci.yaml --repo agentic-utils/zeltapp-cli
```

Useful for re-running after a failed release once the underlying issue (e.g. a token rotation) is fixed.

## Local development

```
go test -cover ./...
gofmt -s -l .                # report files needing simplification
gofmt -s -w cmd/zeltapp      # apply
go vet ./...
go build -o zeltapp ./cmd/zeltapp
```

CI runs all of the above. Keep `gofmt -s` clean to preserve the Go Report Card A+ grade.
