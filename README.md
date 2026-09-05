# dside events

Event discovery for Athens. One Go binary, one SQLite file, server-rendered
HTML. Product spec: `PROJECT.md`.

## Run locally

Everything goes through `make` (`make help` lists the targets):

    make run                                  # dev server on 127.0.0.1:8080, codes in otp.log
    make poster EMAIL=you@example.com NAME="You"
    make check                                # fmt + vet + unit tests + e2e

Without make:

    ADDR=127.0.0.1:8080 DEV_OTP_FILE=/tmp/otp.log go run . serve

Prints exactly one line to stdout (`listening on http://127.0.0.1:8080`) and
logs to stderr. Open the URL, log in with any email; the six-digit code is
appended to `/tmp/otp.log` as `<RFC3339>\t<email>\t<code>`.

Make someone a curator (works while the server runs):

    go run . add-poster -email maria@example.com -name "Maria P." [-slug maria-p]

Idempotent; promotes an existing user account. Prints `poster <slug> <email>`.

## Configuration (environment)

| Variable | Default | Meaning |
|---|---|---|
| `ADDR` | `:8080` | listen address (`127.0.0.1:0` picks a free port) |
| `DB_PATH` | `events.db` | SQLite file (WAL mode) |
| `BASE_URL` | `http://localhost:8080` | public URL |
| `SECURE_COOKIES` | `false` | set `true` behind HTTPS |
| `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASS`, `SMTP_FROM` | — | when `SMTP_HOST` is set, codes go out by email (port 587 default) |
| `DEV_OTP_FILE` | — | when SMTP is off, append codes to this file |
| `OTP_TTL` | `10m` | code lifetime (Go duration) |
| `OTP_MAX_ATTEMPTS` | `5` | wrong guesses before a code is void |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |

## Docker

    docker compose up --build
    docker compose exec events /dside-events add-poster -email you@example.com -name "You"

The SQLite database lives in the `events-data` volume at `/data/events.db`.

## Layout

    main.go config.go server.go errors.go templates.go auth.go   wiring, routing, middleware
    handlers_*.go event_form.go slug.go tags.go                 HTTP handlers, validation
    templates/                                                  html/template pages
    static/                                                     style.css app.js sw.js manifest icon
    internal/store                                              SQLite (modernc.org/sqlite), migrations
    internal/mail                                               login-code delivery (dev file / SMTP)
    e2e/                                                        black-box HTTP suite (`go test ./e2e/...`)

## Tests

    go test ./...            # unit: store, slug
    go test ./e2e/... -count=1

## Conventions

- Every form works without JavaScript: success → 303, validation failure → 422
  with the submitted values, anonymous → 303 `/login?next=…`, wrong role → 403.
- Times are stored as unix seconds (UTC) and shown in Europe/Athens.
- Event URLs never change after publishing.
