# Staatus

Staatus is a free, GitHub-native status page foundation. It is designed to live in the user's repository: config, incidents, history, generated public API data, branding, and the static status page all stay forkable and reviewable.

This v0 is intentionally small and boring:

- Go CLI for validating `staatus.yml` and rendering static public JSON.
- Vue 3 + Vite status page in `web/`.
- Static API files at `web/public/api/*.json`.
- JSON/JSONL source data for incidents and history.
- JSON Schema drafts for the config and public API contracts.
- Placeholders for a future GitHub Action wrapper and forkable template.

The public status page does not call the GitHub API at runtime. It only reads local static files like `/api/status.json`, `/api/components.json`, and `/api/incidents.json`, which makes it CDN-friendly and static-host friendly.

## Current v0 status

Staatus can validate a YAML config, read incident/history data from the repo, render public API JSON, and display a polished static status page from that JSON. Checks are modeled in Go, with a basic HTTP check runner available internally, but scheduled GitHub Actions execution is still future work.

The rendered component API includes a 90-day daily timeline per component, so the public page can show reliability history without making runtime API calls.

For new projects, `settings.missing_history: operational` fills days without check history as green operational days. Teams that prefer stricter reporting can set `settings.missing_history: unknown` to show missing days as neutral instead.

The header is intentionally minimal. Set `page.logo` for an optional logo and `page.contact` for a right-aligned call-to-action link.

## CLI

Run the CLI from the repository root:

```sh
go run ./cmd/staatus version
```

Validate the sample config:

```sh
go run ./cmd/staatus validate
go run ./cmd/staatus validate --json
```

Render the public API files:

```sh
go run ./cmd/staatus render
```

By default, `render` reads:

- `staatus.yml`
- `data/incidents/*.json`
- `data/history/*.jsonl`

And writes:

- `web/public/api/status.json`
- `web/public/api/components.json`
- `web/public/api/incidents.json`

## Web UI

Install and run the status page locally:

```sh
cd web
npm install
npm run dev
```

Build the static site:

```sh
cd web
npm run build
```

The Vite app fetches only `/api/*.json` from its own public directory. Re-run `go run ./cmd/staatus render` after changing `staatus.yml`, incidents, or history.

## Data layout

```text
staatus.yml                 # User-owned page, component, and check config
data/incidents/*.json       # Canonical incident records
data/history/*.jsonl        # Repo-friendly check/history points
web/public/api/*.json       # Generated public API consumed by the status page
schemas/*.schema.json       # Contract drafts for config and public output
```

## Development

```sh
go test ./...
cd web && npm run build
```

Staatus is early. The next natural steps are a GitHub Action wrapper, check execution commands, release binaries, and a dedicated forkable template repo once the core contracts settle.
