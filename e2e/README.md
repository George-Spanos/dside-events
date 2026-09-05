# e2e — black-box end-to-end suite

Go stdlib only, package `e2e` (external test package). It builds the real
binary, seeds two curators through the CLI, starts `serve` on a random port
and drives it over plain HTTP the way a browser with JavaScript disabled
would: no redirect following, one cookie jar per persona, form posts as
`application/x-www-form-urlencoded`.

## Run

```
go test ./e2e/... -count=1          # from the module root
go test ./e2e/... -count=1 -run TestLogin -v
DSIDE_E2E_BIN=/path/to/dside-events go test ./e2e/... -count=1   # skip the build
```

Tests run sequentially against one shared server. A few tests start their own
server to override `OTP_TTL` / `OTP_MAX_ATTEMPTS`. Server stdout/stderr is
attached to the test log when a test fails. Every email, title and venue is
unique per test, and feed assertions are "contains my title / lacks my other
title", never exact counts, so tests do not interfere with each other.

## How the harness talks to the binary

| Step | What the harness does |
|---|---|
| build | `go build -o <tmp>/dside-events .` from the module root, unless `DSIDE_E2E_BIN` is set |
| seed | `<bin> add-poster -email poster1@example.test -name "Maria P."` (and `poster2@example.test` / "Nikos K.") with `DB_PATH`; parses `poster <slug> <email>` from stdout |
| start | `<bin> serve` with env `ADDR=127.0.0.1:0 DB_PATH=… DEV_OTP_FILE=…`; waits for the single stdout line `listening on http://127.0.0.1:PORT`, then polls `/healthz` until 200 |
| login | `POST /login` (email) → reads the last `<RFC3339>\t<email>\t<code>` line for that email from `DEV_OTP_FILE` → `POST /login/code` (code) |
| stop | SIGTERM, `Kill` after 5 s, temp dir removed |

Environment knobs the suite relies on: `ADDR`, `DB_PATH`, `DEV_OTP_FILE`,
`OTP_TTL` (e.g. `1s`), `OTP_MAX_ATTEMPTS` (e.g. `3`). `SMTP_HOST` is forced
empty so codes always land in the dev file.

## HTTP contract under test

| Method | Path | Auth | Result |
|---|---|---|---|
| GET | `/`, `/?tag={tag}` | anon | upcoming feed (from Athens midnight today) grouped by day; unknown tag → 404; logged in: hides `not_interested` |
| GET | `/following` | user | feed narrowed to followed tags OR followed posters; anon → 303 `/login?next=/following` |
| GET | `/e/{slug}` | anon | event page; anon: counter + "Log in to mark interested" link to `/login?next=/e/{slug}`; user: two toggle buttons; owner: Edit link + Delete form; past: "This event has passed.", no buttons |
| POST | `/e/{slug}/interest` | user | `state=interested\|not_interested\|clear`, `back`; 303 → `back` (local path only) else `/e/{slug}`; bad state → 400; unknown slug → 404 |
| GET | `/p/{slug}` | anon | poster name, follow toggle (user only), Upcoming then Past lists; not a poster → 404 |
| POST | `/follow` | user | `kind=tag\|poster`, `key`, `on=1\|0`, `back`; 303; unknown key → 404; idempotent |
| GET | `/mine` | user | sections Upcoming (asc), Past (desc), Hidden (not_interested, with clear); empty state |
| GET | `/login` | anon | email form (+ hidden `next`); logged in → 303 `/` |
| POST | `/login` | anon | `email`, `next`; invalid → 422 "Enter a valid email address."; 4th code in 15 min → 422; OK → cookie `otp_email` (Path=/login), 303 `/login/code?next=…` |
| GET | `/login/code` | anon | code form; without cookie → 303 `/login` |
| POST | `/login/code` | anon | `code`, `next`; wrong → 422 "didn't match"; expired → 422 "expired"; exhausted → 422 "Too many attempts"; OK → `Set-Cookie: session=…; Path=/; HttpOnly; SameSite=Lax`, otp cookie cleared, 303 `next` (local only) else `/` |
| POST | `/logout` | user | deletes the session row, clears cookie, 303 `/`; GET → 405 |
| GET | `/account` | user | email, role, followed tags/posters with unfollow buttons, all tags with follow buttons, logout form, delete form (users only; posters see "removed by hand") |
| POST | `/account/delete` | user | `confirm=1` required (else 422); poster → 403; 303 `/`; cascades sessions, interests, follows |
| GET/POST | `/new` | poster | event form; user → 403; OK → 303 `/e/{slug}`; invalid → 422 re-render with values and `role="alert"` summary; duplicate (same poster, normalised title, Athens day) → 422 "already posted" |
| GET/POST | `/e/{slug}/edit` | owner | prefilled; slug never changes; other poster/user → 403; unknown → 404 |
| POST | `/e/{slug}/delete` | owner | hard delete with cascade, 303 `/`; slug → 404 afterwards |
| GET | `/healthz` | anon | `200 ok` text/plain |
| GET | `/manifest.webmanifest` | anon | `application/manifest+json` with name, start_url, display, icons |
| GET | `/sw.js` | anon | `text/javascript`, `Cache-Control: no-cache`, handles `'install'` and `'fetch'` |
| GET | `/static/*`, `/icon.svg`, `/icon-{180,192,512}.png`, `/offline` | anon | embedded; `/static/*?v=` served `immutable` |
| * | anything else | | custom 404 page ("Page not found", link to `/`); HTML pages `Cache-Control: no-store` |

Event form fields: `title`, `date`, `time`, `venue`, `price`, `tag` (repeated),
`link_label_1..5`, `link_url_1..5`, `description`. There is no sixth link
field, so `max_links` is checked as "the form offers exactly five slots"
rather than by posting a `link_url_6` the contract does not define. Slugs match
`^[a-z0-9-]+-\d{4}-\d{2}-\d{2}(-\d+)?$`; Greek titles are transliterated
("Ταξίδι στη Χώρα των Ήχων" → `taxidi-sti-chora-ton-ichon-…`).

Counter copy: "Nobody yet interested", "N interested", past events "N were
interested". Hidden state copy: "Hidden from your feed".

## `// spec:` convention

Every test function starts with a header comment naming the rules, surfaces
and invariants of `spec/dside-events.allium` it covers, e.g.

```go
// spec: MarkNotInterested, Feed, EventDetailForUser
func TestNotInterested_HiddenFromOwnFeed_InvisibleToOthers(t *testing.T) {
```

The weed phase greps these against `^rule|^surface|^invariant` in the spec to
find uncovered obligations. `SessionExpires` (90-day sessions) is deliberately
not covered by this suite: it cannot be observed in a black-box run.

## Files

| File | Contents |
|---|---|
| `main_test.go` | `TestMain`: build, temp dir, seed posters, shared server, teardown |
| `harness_test.go` | `server`, `startServer`, `launchServer`, `waitHealthy`, `addPoster`, `readOTP`/`waitOTP` |
| `client_test.go` | `client` (cookie jar, no redirects), `get`/`postForm`/`follow`/`cookie`/`setRawCookie`, assertions, HTML helpers |
| `fixtures_test.go` | `uniqEmail`, `uniqTitle`, `loginAs`/`asPoster`, `eventForm`/`validEvent`/`createEvent`, `tomorrow`/`yesterday`, `interestedCount`, `setInterest`, `follow` |
| `browse_test.go` | public feed, tag filter, event and poster pages, anonymous limits |
| `auth_test.go` | OTP request/verify, expiry, attempts, supersede, rate limit, normalisation, `next`, logout |
| `events_test.go` | create/edit/delete, validation, duplicates, slugs, owner checks, `add-poster` CLI |
| `follow_test.go` | tag/poster follow toggles, `/following` semantics |
| `interest_test.go` | interested / not_interested / clear, counters, `/mine` |
| `account_test.go` | account page, self-deletion |
| `pwa_test.go` | manifest, service worker, static assets, 404/405, no-JS guarantees |
