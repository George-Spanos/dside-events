# e2e — black-box end-to-end suite

Go stdlib only, package `e2e` (external test package). It builds the real
binary, starts `serve` on a random port, seeds two curators through the CLI
(which prints their secret login links) and drives the server over plain HTTP
the way a browser with JavaScript disabled would: no redirect following, one
cookie jar per persona, form posts as `application/x-www-form-urlencoded`.

There is no email and no login form anywhere. An account is a cookie that
appears on the first Interested / Follow; it travels between devices by a
secret link (`/k/{key}`); posters log in with the secret link the CLI prints.

## Run

```
go test ./e2e/... -count=1          # from the module root
go test ./e2e/... -count=1 -run TestSecret -v
DSIDE_E2E_BIN=/path/to/dside-events go test ./e2e/... -count=1   # skip the build
```

Tests run sequentially against one shared server. The CLI tests start their
own server so rotating keys never touches the two shared posters. Server
stdout/stderr is attached to the test log when a test fails. Every title,
venue and poster slug is unique per test, and feed assertions are "contains my
title / lacks my other title", never exact counts, so tests do not interfere
with each other.

## How the harness talks to the binary

| Step | What the harness does |
|---|---|
| build | `go build -o <tmp>/dside-events .` from the module root, unless `DSIDE_E2E_BIN` is set |
| start | `<bin> serve` with env `ADDR=127.0.0.1:0 DB_PATH=…` (nothing else); waits for the single stdout line `listening on http://127.0.0.1:PORT`, then polls `/healthz` until 200 |
| seed | after the server is up: `<bin> add-poster -name "Maria P."` and `-name "Nikos K."` with env `DB_PATH=… BASE_URL=http://127.0.0.1:PORT`; parses the two stdout lines `poster <slug>` and `link <BASE_URL>/k/<key>` |
| poster login | fresh client, `GET /k/<key>` → must be 303 `/mine` with a `session` cookie (`asPoster`) |
| user | fresh client whose first `POST /follow` creates the account and the cookie, then unfollows (`newUser`) |
| secret link | `GET /account`, parse `href="…/k/([A-Za-z0-9_-]{43})"` (`secretLink`); links are opened by path so any `BASE_URL` works (`openLink`) |
| stop | SIGTERM, `Kill` after 5 s, temp dir removed |

## CLI contract

| Command | Env | Stdout |
|---|---|---|
| `serve` | `ADDR`, `DB_PATH` (also honours `BASE_URL`, `SECURE_COOKIES`, `LOG_LEVEL`) | exactly one line `listening on http://127.0.0.1:PORT` once bound |
| `add-poster -name "Maria P." [-slug maria]` | `DB_PATH`, `BASE_URL` | new poster: `poster <slug>` and `link <BASE_URL>/k/<key>`; existing slug: `poster <slug>` and `link (unchanged, run poster-link to get a new one)` — a no-op, name untouched |
| `poster-link -slug maria` | `DB_PATH`, `BASE_URL` | `link <BASE_URL>/k/<key>` with a fresh key; the old link dies, sessions survive; unknown slug → non-zero exit, no link line |

Both CLI commands work while the server runs (WAL + busy_timeout). Keys are
32 random bytes as base64url (43 characters). There are no OTP/SMTP knobs.

## HTTP contract under test

| Method | Path | Who | Result |
|---|---|---|---|
| GET | `/`, `/?tag={tag}` | anyone | upcoming feed (from Athens midnight today) grouped by day; filters always include `following`; unknown tag → 404; with a session hides `not_interested` |
| GET | `/following` | anyone | 200; feed narrowed to followed tags OR posters; no session / nothing followed → "You're not following anything yet. Pick a tag above and press Follow, or follow a curator from an event page." |
| GET | `/mine` | anyone | 200; Upcoming (asc), Past (desc), Hidden (with clear); no session / nothing marked → h1 "Mine" + "Nothing here yet. Press Interested on an event and it shows up here." |
| GET | `/e/{slug}` | anyone | event page, identical for anon and user: Interested / Not interested buttons + counter; owner: Edit link + Delete form; past: "This event has passed.", no buttons; no login link |
| POST | `/e/{slug}/interest` | anyone | no session → creates `Account{role user}` + session, sets cookie, then acts; `state=interested\|not_interested\|clear`, `back`; 303 → `back` (local path only) else `/e/{slug}`; bad state → 400; unknown slug → 404 |
| GET | `/p/{slug}` | anyone | poster name, follow toggle (everyone), Upcoming then Past lists; not a poster → 404 |
| POST | `/follow` | anyone | lazy account as above; `kind=tag\|poster`, `key`, `on=1\|0`, `back`; 303; unknown key → 404 (no account created); idempotent |
| GET | `/k/{key}` | anyone | valid → new session for that account (also when another cookie is present: switches), `Set-Cookie: session=…; Path=/; HttpOnly; SameSite=Lax; Max-Age=31536000`, 303 `/mine`; unknown → 404 "This link doesn't work. It may have been replaced with a new one." |
| GET | `/account` | anyone | 200, two variants. No session: h1 "Account" + "This device has no list yet. Press Interested on an event, or follow a tag or a curator, and your account starts here. No sign-up, no email." + "Opened a secret link before? Open it again on this device to continue there." With session: `<h2>Your secret link</h2>` `<a href="{link}">{link}</a>`, form `/account/key` "Get a new link", `<h2>Following</h2>` toggles, `<h2>This device</h2>` form `/forget` "Forget this device", `<h2>Delete account</h2>` form with `confirm` (posters: "Your curator page: …" and "removed by hand" instead) |
| POST | `/account/key` | session | rotate key, 303 `/account`; old link → 404, existing sessions untouched; no session → 303 `/account`; GET → 405 |
| POST | `/forget` | session | delete the session row, clear cookie, 303 `/`; account and data stay, link restores them; no session → 303 `/account`; GET → 405 |
| POST | `/account/delete` | session user | `confirm=1` required (else 422); poster → 403 "by hand"; 303 `/`; cascades sessions, interests, follows, key (link → 404); no session → 303 `/account` |
| GET/POST | `/new` | poster | event form; non-poster incl. anon → 403; OK → 303 `/e/{slug}`; invalid → 422 re-render with values and `role="alert"`; duplicate (same poster, normalised title, Athens day) → 422 "already posted" |
| GET/POST | `/e/{slug}/edit` | owner | prefilled; slug never changes; other poster / user / anon → 403; unknown → 404 |
| POST | `/e/{slug}/delete` | owner | hard delete with cascade, 303 `/`; slug → 404 afterwards; others → 403 |
| GET | `/healthz` | anyone | `200 ok` text/plain |
| GET | `/manifest.webmanifest` | anyone | `application/manifest+json` with name, start_url, display, icons |
| GET | `/sw.js` | anyone | `text/javascript`, `Cache-Control: no-cache`, handles `'install'` and `'fetch'` |
| GET | `/static/*`, `/icon.svg`, `/icon-{180,192,512}.png`, `/offline` | anyone | embedded; `/static/*?v=` served `immutable` |
| any | `/login`, `/login/code`, `/logout` | | gone → 404 |
| * | anything else | | custom 404 page ("Page not found", link to `/`); HTML pages `Cache-Control: no-store` |

Nav is always `mine · account` (+ `new` for posters); no page links to a
login. Read pages never redirect and never set a cookie.

Event form fields: `title`, `date`, `time`, `venue`, `price`, `tag` (repeated),
`link_label_1..5`, `link_url_1..5`, `description`. There is no sixth link
field, so `max_links` is checked as "the form offers exactly five slots"
rather than by posting a `link_url_6` the contract does not define. Slugs match
`^[a-z0-9-]+-\d{4}-\d{2}-\d{2}(-\d+)?$`; Greek titles are transliterated
("Ταξίδι στη Χώρα των Ήχων" → `taxidi-sti-chora-ton-ichon-…`).

Counter copy: "Nobody yet interested", "N interested", past events "N were
interested". Hidden state copy: "Hidden from your feed". Copy assertions
unescape HTML and normalise curly apostrophes (`assertCopy`), so templates may
emit `&#39;` or `’`.

## `// spec:` convention

Every test function starts with a header comment naming the rules, surfaces,
invariants (and, where natural, entities or fields) of
`spec/dside-events.allium` it covers, e.g.

```go
// spec: OpenSecretLink, SecretLink, Session, Account, MyEvents, Visitor
func TestOpenSecretLink_SameListOnAnotherDevice(t *testing.T) {
```

The weed phase greps these against `^rule|^surface|^invariant` in the spec to
find uncovered obligations.

## Not black-box testable

- `SessionExpires` (`session_duration = 365.days`): the suite only checks
  that the cookie is persistent with `Max-Age=31536000` (or an `Expires`);
  it cannot wait a year.
- `SECURE_COOKIES` / the `Secure` flag: the harness speaks plain HTTP.
- That the key is stored hashed (`key_hash`): only observable as "the old link
  stops working after rotate/delete", which is asserted.
- Rate limiting of cookieless POSTs (OQ-17): not specified, not tested.

## Files

| File | Contents |
|---|---|
| `main_test.go` | `TestMain`: build, temp dir, shared server, seed posters via `add-poster` (server first, then CLI with `BASE_URL`), teardown |
| `harness_test.go` | `server`, `startServer`, `launchServer`, `waitHealthy`, `runCLI`, `addPoster`/`addPosterRaw`, `posterLink`, `parseLink` |
| `client_test.go` | `client` (cookie jar, no redirects), `get`/`postForm`/`follow`/`cookie`/`setRawCookie`, `sessionSetCookie`, assertions incl. `assertCopy` and `assertSessionCookieFlags`, HTML helpers |
| `fixtures_test.go` | `uid`/`uniqTitle`/`uniqSlug`, `asPoster`/`newUser`/`openLink`/`secretLink`/`keyPath`/`randomKey`, `eventForm`/`validEvent`/`createEvent`, `tomorrow`/`yesterday`, `interestedCount`, `setInterest`, `follow` |
| `browse_test.go` | public feed, tag + following filters, event and poster pages, anonymous event page with buttons, 403/200 route table |
| `secret_test.go` | lazy account creation, cookie flags, no-session read pages, account page with link and forms, open / unknown / switch link, rotate, forget, session-less POSTs, login routes gone, poster link login, `poster-link` CLI |
| `account_test.go` | follows on the account page, poster variant, self-deletion (data, sessions, link), confirm, poster 403 |
| `events_test.go` | create/edit/delete, validation, duplicates, slugs, owner checks, `add-poster` CLI (link, no-op for existing slug, unique slugs/keys) |
| `follow_test.go` | tag/poster follow toggles, anon follow from a poster page, `/following` semantics incl. anon 200 |
| `interest_test.go` | interested / not_interested / clear, counters, `/mine`, anon mark starts an account |
| `pwa_test.go` | manifest, service worker, static assets, 404/405, no-JS guarantees (every mutation 303, forms well-formed, `/k/` is the one state-changing GET) |
