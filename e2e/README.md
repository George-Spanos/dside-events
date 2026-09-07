# e2e — black-box end-to-end suite

Go stdlib only, package `e2e` (external test package). It builds the real
binary, starts `serve` on a random port, seeds two curators through the CLI
(which prints their secret login links) and drives the server over plain HTTP
the way a browser with JavaScript disabled would: no redirect following, one
cookie jar per persona, form posts as `application/x-www-form-urlencoded`.

There is no email and no login form anywhere. An account is a cookie that
appears on the first Follow of an event; it travels between devices by a
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
venue and poster slug is unique per test, and list assertions are "contains my
title / lacks my other title", never exact counts, so tests do not interfere
with each other.

Because the shared server accumulates events, "the list shows my event" is
asserted on `/upcoming` (all upcoming events) or `/upcoming?tag=x` through
`assertListed(t, c, path, title)` /
`assertNotListed`, never on `/`: the home page shows only the next ten, so a
title created by a test may legitimately be beyond the tenth. Home-page tests
(`home_test.go`) instead post events about a thousand days out, so they are
last by date whatever else exists, and check row counts inside the
`<section class="upcoming">` column.

## How the harness talks to the binary

| Step | What the harness does |
|---|---|
| build | `go build -o <tmp>/dside-events .` from the module root, unless `DSIDE_E2E_BIN` is set |
| start | `<bin> serve` with env `ADDR=127.0.0.1:0 DB_PATH=…` (nothing else); waits for the single stdout line `listening on http://127.0.0.1:PORT`, then polls `/healthz` until 200 |
| seed | after the server is up: `<bin> add-poster -name "Maria P."` and `-name "Nikos K."` with env `DB_PATH=… BASE_URL=http://127.0.0.1:PORT`; parses the two stdout lines `poster <slug>` and `link <BASE_URL>/k/<key>` |
| poster login | fresh client, `GET /k/<key>` → must be 303 `/mine` with a `session` cookie (`asPoster`) |
| user | fresh client whose first `POST /e/{slug}/follow` creates the account and the cookie, then clears that follow (`newUser`) |
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
| GET | `/`, `/?tag={tag}` | anyone | home: `<body class="wide">`, filters row (unknown tag → 404), then `<div class="columns">` with `<section class="mine">` (always rendered, narrowed by `?tag=` like Upcoming: `<h2>My feed</h2>`, at most 10 soonest, `<a href="/mine">All of my feed →</a>` once there is an account; with no rows one line instead — anon "Follow an event and it starts a list on this device. No sign-up, no email.", session "Nothing here yet. Follow an event and it shows up here.", under a filter "Nothing of yours tagged {tag}.") before `<section class="upcoming">` (`<h2>Upcoming</h2>`, the next 10 upcoming from Athens midnight today, `hidden` events left out, `<a href="/upcoming{?tag=…}">All upcoming events →</a>`); the 10 is hardcoded, no paging |
| GET | `/upcoming`, `/upcoming?tag={tag}` | anyone | 200, h1 "Upcoming", `<title>Upcoming · dside events</title>`; ALL upcoming events grouped by day, one column, same filters row; unknown tag → 404; with a session leaves out `hidden` events |
| GET | `/mine` | anyone | 200; Upcoming (asc), Past (desc), Hidden ("Events you hid. They stay out of Upcoming.", each row with a "Show again" `state=clear` button); a followed or hidden repeating event lists every one of its dates; no session / nothing followed or hidden → h1 "My feed" + "Nothing here yet. Follow an event and it shows up here." |
| GET | `/e/{slug}` | anyone | event page, identical for anon and user: Follow / Hide buttons + counter; the pressed one reads `Following` / `Hidden` with `aria-pressed="true"` (`data-on`/`data-off`, the check mark is CSS), a hidden event adds "Hidden from Upcoming."; a date of a repeating event adds `<small class="series">One of N dates</small>` to the details and `<small>Follow or hide applies to all N dates.</small>` after the counter (N ≥ 2, the dates still posted); owner: Edit link + Delete form; past: "This event has passed.", no buttons; no login link |
| POST | `/e/{slug}/follow` | anyone | no session → creates `Account{role user}` + session, sets cookie, then acts; `state=follow\|hide\|clear` (follow → `following`, hide → `hidden`, clear → row deleted), `back`; on a date of a repeating event the same row is written for every date of the series (follow, hide and clear alike), so counters and pressed states agree across the run; 303 → `back` (local path only) else `/e/{slug}`; bad state → 400; unknown slug → 404 |
| GET | `/p/{slug}` | anyone | poster name, Upcoming then Past lists; not a poster → 404 |
| GET | `/k/{key}` | anyone | valid → new session for that account (also when another cookie is present: switches), `Set-Cookie: session=…; Path=/; HttpOnly; SameSite=Lax; Max-Age=31536000`, 303 `/mine`; unknown → 404 "This link doesn't work. It may have been replaced with a new one." |
| GET | `/account` | anyone | 200, two variants. No session: h1 "Account" + "This device has no list yet. Follow an event and your account starts here. No sign-up, no email." + "Opened a secret link before? Open it again on this device to continue there." With session: `<h2>Your secret link</h2>` `<a href="{link}">{link}</a>`, form `/account/key` "Get a new link", `<h2>This device</h2>` form `/forget` "Forget this device", `<h2>Delete account</h2>` "Deletes your account, your follows and your hidden events. It can't be undone." + form with `confirm` (posters: "Your curator page: …" and "removed by hand" instead) |
| POST | `/account/key` | session | rotate key, 303 `/account`; old link → 404, existing sessions untouched; no session → 303 `/account`; GET → 405 |
| POST | `/forget` | session | delete the session row, clear cookie, 303 `/`; account and data stay, link restores them; no session → 303 `/account`; GET → 405 |
| POST | `/account/delete` | session user | `confirm=1` required (else 422); poster → 403 "by hand"; 303 `/`; cascades sessions, event follows, key (link → 404); no session → 303 `/account` |
| GET/POST | `/new` | poster | event form incl. the repeat controls (`repeats`, seven `weekday`, `times`, `until`, always visible); non-poster incl. anon → 403; OK → 303 `/e/{slug}`; with `repeats=1` one event per date (every day from `date` to `until` whose weekday is ticked, at each of `times`, else at `time`; 2..200 dates; the first date's weekday need not be ticked) sharing a series id, each with its own slug (same-day second start gets `-2`), 303 → `/e/{slug}` of the earliest; invalid → 422 re-render with values and `role="alert"`, the repeat messages on `weekday` / `times` / `until` (see copy below); duplicate (same poster, normalised title, an Athens day already covered by an event or by a date of another series; the dates of one post never clash with each other) → 422 "already posted" |
| GET/POST | `/e/{slug}/edit` | owner | prefilled; no repeat controls, and `repeats`/`weekday`/`times`/`until` in the POST are ignored, so an edit changes one date only; slug never changes; other poster / user / anon → 403; unknown → 404 |
| POST | `/e/{slug}/delete` | owner | hard delete with cascade, 303 `/`; slug → 404 afterwards; others → 403 |
| GET | `/healthz` | anyone | `200 ok` text/plain |
| GET | `/manifest.webmanifest` | anyone | `application/manifest+json` with name, start_url, display, icons |
| GET | `/sw.js` | anyone | `text/javascript`, `Cache-Control: no-cache`, handles `'install'` and `'fetch'` |
| GET | `/static/*`, `/icon.svg`, `/icon-{180,192,512}.png`, `/offline` | anyone | embedded; `/static/*?v=` served `immutable` |
| any | `/login`, `/login/code`, `/logout` | | gone → 404 |
| * | anything else | | custom 404 page ("Page not found", link to `/`); HTML pages `Cache-Control: no-store` |

Nav is always `mine · account` (+ `new` for posters); no page links to a
login. Read pages never redirect and never set a cookie.

List rows (`/` both columns, `/upcoming`, `/mine` Upcoming,
`/p/{slug}` Upcoming) are `<li>` inside `<ul class="events">` (a date of a
repeating event: `<li data-series="{id}">`, the id shared by its siblings):
`<time>`, `<a href="/e/{slug}">`, a `<small class="meta">` of `<span>`s in
this order — tags, venue + price, "N following" (absent while nobody
follows), poster — plus a further
`<span class="series">One of N dates</span>` when the row is one of N ≥ 2
dates, and one toggle form
`<form method="post" action="/e/{slug}/follow" …>` with hidden
`name="back"` (the current path incl. query) and a single
`<button name="state" value="follow|clear" aria-pressed="false|true" data-on="Following" data-off="Follow">`
reading `Follow` or `Following` (the check mark is CSS). Rows never offer `hide`.
Past rows (`/mine` Past, `/p/{slug}` Past, `<ul class="past">`) carry no
form. Day headings are `<h3 class="day">{Today · |Tomorrow · }<b>Weekday</b>
2 January</h3>` everywhere. Helpers: `eventRows`, `rowsFor`/`rowFor`,
`slugOf`, `section` (client_test.go), `assertRowToggle`
(event_follow_test.go), `seriesSlugs`/`createSeries` (fixtures_test.go) and
`seriesOf`/`inputWith` (series_test.go).

Event form fields: `title`, `date`, `time`, `venue`, `price`, `tag` (repeated),
`link_label_1..3`, `link_url_1..3`, `description`; on `/new` only, the repeat
controls `repeats` (`1`), `weekday` (repeated, `mon`..`sun`), `times`
(optional, comma-separated `HH:MM`) and `until` (`dd/mm/yyyy`), which the
`eventForm` fixture emits only when set. `Date` and `Until` are held as
`YYYY-MM-DD` in the fixture and rendered as `dd/mm/yyyy` by `formDate` when
posted, so a fixture date still reads like the slug it produces. There is no
fourth link field, so `max_links` (3) is checked as "the form offers exactly
three slots"
rather than by posting a `link_url_4` the contract does not define. Slugs match
`^[a-z0-9-]+-\d{4}-\d{2}-\d{2}(-\d+)?$`; Greek titles are transliterated
("Ταξίδι στη Χώρα των Ήχων" → `taxidi-sti-chora-ton-ichon-…`).

Counter copy: "N following", past events "N followed"; at zero neither the
event page nor a row shows a counter at all. Hidden state copy: "Hidden from Upcoming." Series copy:
"One of N dates" (rows and event page), "Follow or hide applies to all N
dates." (event page). Repeat messages: `weekday` "Pick weekdays from the
list." / "Pick at least one weekday."; `times` "Pick valid start times, like
18:00, 21:00." / "Each start time must be different."; `until` "Pick an until
date." / "Pick a valid until date." / "Until must be on or after the date." /
"Until must be within the next three years." / "A repeating event needs at
least two dates." / "A repeating event can have at most 200 dates."; the
duplicate message names the day ("You already posted this event on 2 January
2027.") and is asserted only as "already posted". Copy assertions
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
find uncovered obligations. Home-page assertions cite `Home` (and its
guarantees as `Home.TenAtMost`, `Home.SideBySide`, the config as
`home_list_size`); `/upcoming` assertions cite
`UpcomingAll`, which replaced the former `Feed` surface.

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
| `client_test.go` | `client` (cookie jar, no redirects), `get`/`postForm`/`follow`/`cookie`/`setRawCookie`, `sessionSetCookie`, assertions incl. `assertCopy`, `assertSessionCookieFlags`, `assertListed`/`assertNotListed`, HTML helpers incl. `eventRows`/`rowsFor`/`rowFor`/`slugOf`/`section` |
| `fixtures_test.go` | `uid`/`uniqTitle`/`uniqSlug`, `asPoster`/`newUser`/`openLink`/`secretLink`/`keyPath`/`randomKey`, `eventForm`/`validEvent`/`createEvent`, `weekdayValue`/`validSeries`/`seriesSlugs`/`createSeries`, `tomorrow`/`yesterday`, `followerCount`, `setEventFollow` |
| `browse_test.go` | `/upcoming` list, tag filters on `/upcoming` and `/`, event and poster pages, anonymous event page with buttons, 403/200 route table |
| `home_test.go` | home page: Upcoming column of at most ten with the "All upcoming events →" link (also filtered), the eleventh-plus on `/upcoming` only, Mine column present/absent, `<body class="wide">` only on `/`, `/upcoming` h1/title/tag filter/404, `<h3 class="day">` headings with bold weekday |
| `secret_test.go` | lazy account creation, cookie flags, no-session read pages, account page with link and forms, open / unknown / switch link, rotate, forget, session-less POSTs, login routes gone, poster link login, `poster-link` CLI |
| `account_test.go` | account page poster variant, self-deletion (data, sessions, link), confirm, poster 403 |
| `events_test.go` | create/edit/delete, validation, duplicates, slugs, owner checks, `add-poster` CLI (link, no-op for existing slug, unique slugs/keys) |
| `event_follow_test.go` | follow / hide / clear (`FollowEvent`, `HideEvent`, `UnfollowEvent`), `follower_count`, `/mine` incl. the Hidden section, anon Follow starts an account, row toggles on every list (`assertRowToggle`), past rows without a toggle, pressed Following / Hidden buttons on the event page (`HiddenIsPrivate`) |
| `series_test.go` | repeating events (`CreateSeries`): form controls, expansion to rows with `data-series` and "One of N dates", several times per day, first date not ticked, duplicates against covered days, the validation table incl. the 200 cap, edit and delete per date, follow / hide / clear acting on the whole run |
| `pwa_test.go` | manifest, service worker, static assets, 404/405, no-JS guarantees (every mutation 303, forms well-formed, `/k/` is the one state-changing GET) |
