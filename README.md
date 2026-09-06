# dside events

Event discovery for Athens. One Go binary, one SQLite file, server-rendered
HTML. Product spec: `PROJECT.md`.

## Run locally

Everything goes through `make` (`make help` lists the targets):

    make run                                  # dev server on 127.0.0.1:8080
    make poster NAME="Maria P." [SLUG=maria]  # a curator; prints their secret link
    make poster-link SLUG=maria               # a new link for an existing curator
    make check                                # fmt + vet + unit tests + e2e

Without make:

    ADDR=127.0.0.1:8080 go run . serve

Prints exactly one line to stdout (`listening on http://127.0.0.1:8080`) and
logs to stderr.

## Pages

| Path | What it shows |
|---|---|
| `/`, `/?tag=x` | home: "My feed" (the events you follow) next to the next 10 upcoming events (a fixed founder decision, not a setting), side by side; "All upcoming events →" leads to the full list |
| `/upcoming`, `/upcoming?tag=x` | all upcoming events, day-grouped |
| `/mine` | My feed: your Upcoming, Past and Hidden events |
| `/e/{slug}` | one event; Follow / Hide, and how many follow it |
| `/p/{slug}` | a curator's events |
| `/account` | your secret link, forget / delete |

Both home columns always render; with no rows, My feed says why it is empty,
which is also where a visitor without a session is told what Follow costs.

Every upcoming row, on every page, carries the Follow toggle
(`POST /e/{slug}/follow` with `state=follow|hide|clear`); a pressed one reads
`✓ Following`. Hiding is offered on every row and on the event page, and stays
private. Unknown `tag` values are 404. An event carries at most 3 links.

| Route | Form fields |
|---|---|
| `POST /e/{slug}/follow` | `state` = `follow`, `hide` or `clear`; `back` |

## Accounts

There is no sign-up and no email. Everyone browses; the first press of
Follow of an event creates an anonymous account and keeps it in a cookie
(`session`, HttpOnly, SameSite=Lax, 365 days). The account page shows a
**secret link** (`/k/<key>`): opening it on another device continues the same
account there, and opening it while another account's cookie is present
switches to the link's account. "Get a new link" replaces the key (the old
link stops working; devices already using the account stay in). "Forget this
device" drops the cookie only; the link still opens the account. "Delete
account" removes the account, its follows, hidden events and sessions.

Curators (posters) are created by hand and log in with the same kind of link:

    go run . add-poster -name "Maria P." [-slug maria]

prints two lines, `poster <slug>` and `link <BASE_URL>/k/<key>`. Send the link
to the curator; opening it logs them in and lands on `/mine`. Re-running for a
slug that already exists changes nothing and prints `poster <slug>` followed by
`link (unchanged, run poster-link to get a new one)`. To replace a curator's
link (lost, leaked, or never received):

    go run . poster-link -slug maria

prints `link <BASE_URL>/k/<key>`; an unknown slug exits 1. Both commands work
while the server runs (SQLite WAL) and need the same `DB_PATH` and `BASE_URL`.

Keys are 32 random bytes (base64url, 43 characters) stored as-is in the
database, which is the trust boundary; sessions are stored hashed as before.

## Configuration (environment)

| Variable | Default | Meaning |
|---|---|---|
| `ADDR` | `:8080` | listen address (`127.0.0.1:0` picks a free port) |
| `DB_PATH` | `events.db` | SQLite file (WAL mode) |
| `BASE_URL` | `http://localhost:8080` | public URL; secret links are built from it. `serve` without it uses the address it actually listens on (so `ADDR=127.0.0.1:0` still prints working links); the CLI uses the default |
| `SECURE_COOKIES` | `false` | set `true` behind HTTPS |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |

## Docker

Local build:

    docker compose up --build
    docker compose exec events /dside-events add-poster -name "You"

Production (Watchtower-friendly): every push to `main` runs `make check` and
publishes `ghcr.io/george-spanos/dside-events:latest` (plus a `:<sha>` tag) via
`.github/workflows/build.yml` (no secrets needed: it logs in to GHCR with the workflow's own `GITHUB_TOKEN`). On the server:

    BASE_URL=https://events.example.com make prod-up
    make prod-curators                        # creates the two curators, prints their secret links
    make prod-poster-links                    # list every curator's secret link (also saved to curators.txt)

`prod-curators` is idempotent: the names live in the `CURATORS` variable of the
Makefile; re-running prints `link (unchanged, …)` for curators that already exist.

The service has `restart: always`, so
Watchtower pulls the new image and restarts the container on its own.

Backups and schema changes:

    make prod-backup                          # consistent snapshot (VACUUM INTO) copied to backups/
    make prod-restore FILE=backups/events-<time>.db

Schema changes are plain SQL files in `internal/store/migrations/NNN_name.sql`, applied
in order by the server at startup and recorded in `schema_version`. Deploying a new
image is how a migration runs in production: Watchtower restarts the container and
the new binary applies whatever is pending before it serves. Never edit a migration
that has been applied somewhere; add the next number. Take `make prod-backup` first.

In both cases the SQLite database lives in the `events-data` volume at
`/data/events.db`. `BASE_URL` must be the public address, because it is
printed inside the curators' secret links.

## Layout

    main.go config.go server.go errors.go templates.go auth.go   wiring, routing, middleware, sessions
    handlers_*.go event_form.go slug.go tags.go                 HTTP handlers, validation (handlers_feed.go: home, upcoming, mine, event, poster)
    templates/                                                  html/template pages; _list.html is the one row markup, _filters.html the filter row
    static/                                                     style.css app.js sw.js manifest icon
    internal/store                                              SQLite (modernc.org/sqlite), migrations
    e2e/                                                        black-box HTTP suite (`go test ./e2e/...`)

## Tests

    go test ./...            # unit: store, slug
    go test ./e2e/... -count=1

## Conventions

- Every form works without JavaScript: success → 303, validation failure → 422
  with the submitted values, wrong role (including no account) → 403.
- Follow never needs a prior login: without a session it
  creates the account first, then acts. Read pages never redirect; without a session
  `/mine` and `/account` show an empty state.
- Times are stored as unix seconds (UTC) and shown in Europe/Athens.
- Event URLs never change after publishing.

## Credits

The app icon is the `calendar-event` glyph from [Tabler Icons](https://tabler.io/icons) (MIT, © Paweł Kuna) on the site's ochre tile.
