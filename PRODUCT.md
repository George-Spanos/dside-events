# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

**Primary: an Athens local, on a phone, in two modes of one habit.** The quick check — a free evening, no plan, "what is on tonight or this week" — and the occasional pass further out, marking something they don't want to miss. The same person, the same visit, no navigation step between the two: that is why My feed and Upcoming sit side by side as equal columns on the home page rather than one being a tab or a section of the other.

They browse without an account and never asked for one. The first press of Follow starts an anonymous account in a cookie; there is no sign-up, no email, and no personal data at any point.

**Secondary: curators (posters)** — artists and people the founder knows personally, whitelisted by hand through the CLI, which prints their secret login link. They post events. They are public: their name appears on every event they post and is the trust signal for it.

## Product Purpose

Event discovery for Athens — concerts, theatre, film, exhibitions — where posting an event costs a curator one form and one minute, with no audience to perform for.

The founding problem belongs to the artist, not the attendee: artists and curators dislike the constant promotion and content production that social platforms demand of them. Most people, meanwhile, just want to find events that might interest them. This product separates those two needs so neither has to carry the other's cost.

**Success is that the curators we know keep posting here** instead of on Instagram. Attendee behaviour is secondary and deliberately unmeasured. If posting ever starts to feel like content creation, the product has failed regardless of any other signal.

## Positioning

An event listing where the trust signal is a named human being rather than an algorithm, a rating, or a promotion budget. Every event carries the name of the curator who posted it, that curator is someone the operators personally know, and admission to post is by hand.

A neighbouring product could copy the tag list and the follow button. It could not truthfully copy "posted by people we know" — that claim is only true while the curator list is short and vouched-for, and it is the whole product.

The corollaries are structural, not stylistic: one post per event and no reposting, so the list cannot be gamed by frequency; no metrics of any kind, so nothing can be optimised against; no notifications, so the product never reaches out to demand attention. People open it when they want it.

## Operating Context

Consumed almost entirely on a phone, in Greek daily life, in short sessions. Installable as a PWA; usable offline only to the extent of an offline page, since the whole value is current information.

All times are Athens local (Europe/Athens) and displayed as such regardless of where the reader is. "Upcoming" means on or after midnight of the current Athens day.

Curators are onboarded out of band: the operator runs `add-poster`, which prints a secret link, and sends that link to the curator by whatever private channel they already use. Opening it logs them in and lands on `/mine`. There is no login page, no password, and no recovery flow other than the operator issuing a new link.

Attendee accounts travel the same way. The account page shows a secret link (`/k/<key>`); opening it on a second device continues the same list there. Lose both the cookie and the link and the list is gone. This is understood and accepted, not a defect to be designed around.

## Capabilities and Constraints

- **Text only.** No images, no video, anywhere in the product. An event is a title, a date and time, a venue, an optional price, one to four tags, up to three outbound links, and a plain-text description. This is a permanent product constraint, not a stage.
- **Fixed tag list:** `concert`, `theater`, `film`, `exhibition`. Not user-extensible. An unknown tag in a URL is a 404, not an empty list.
- **One post per event.** Editing is allowed; reposting is not. The slug and therefore the URL are immutable after publishing.
- **The home page shows the next 10 upcoming events** beside the visitor's followed events. Ten is a founder decision and deliberately not a setting. There is no pagination anywhere — the full list is one link away.
- **Follow is public** (an event shows how many follow it); **hide is private** (nobody else can tell, including the curator).
- **Following, hiding and clearing act on a whole repeating series**, not one date. A repeating post is many ordinary events sharing a series id.
- **No metrics, no analytics, no notifications, no email.** People open the app to see what changed.
- **Anonymous accounts only** for attendees. Curator accounts are created by hand and removed by hand; nothing promotes a user into a curator.
- **Every form works with JavaScript disabled.** Success is a 303, validation failure a 422 carrying the submitted values, wrong role a 403. JavaScript only intercepts and swaps in the response, falling back to native submission on any error.
- **Curator scale is open.** Vouched-for and hand-added, but the product must not assume the current two. Dozens is plausible. Whether curators ever need to be discoverable as a browsable list is **explicitly undecided** — do not build for it, do not design against it.
- **Language: English interface, permanently.** Event content is whatever language the curator typed, usually Greek. There is no translation layer, no language switcher, and no per-event language field. Mixed Greek and English in a single list is the normal, expected state, and layouts must hold up under it. The document declares `lang="en"` and keeps it; see the accepted 3.1.2 exception under Accessibility & Inclusion.

## Brand Commitments

- **Name:** dside events (short name `dside`). Lowercase in the masthead.
- **Voice:** plain, warm, and unhurried; second person, no exclamation, no marketing register. The footer line is the register to match: *"Events in Athens, posted by people we know. Times are Athens local."* Empty states state the fact and the next step, and nothing more — *"Nothing here yet. Follow an event and it shows up here."*
- **Terminology is binding.** Attendees **follow** and **hide** events (renamed from "interested"/"not interested" by the founder on 2026-09-06). Posters are called **curators** in prose. A visitor's list is **My feed** (renamed from "Mine" by the founder on 2026-09-06; "feed" now names that column alone, never the Upcoming list). Do not reintroduce retired vocabulary.
- **Design direction, stated firm by the founder:** as simple and non-obstructive as possible; old-internet feel; mobile-first and responsive. The UI is two lists on one screen and an info page behind each item. Recorded here as a durable commitment; its visual expression lives in `DESIGN.md`.
- **App icon:** the `calendar-event` glyph from Tabler Icons (MIT, © Paweł Kuna) in white on the site's ochre tile. Attribution is carried in `README.md` and must survive any redesign of the icon's colour or container.

## Evidence on Hand

- **Specification:** `PROJECT.md` is the founder's product spec and the stated source of truth for intent. `spec/dside-events.allium` is a 1093-line formal specification of the same product — entities, 18 rules, 5 invariants, config, and 10 declared surfaces with their exposed data, offered actions, and `@guarantee` constraints. Its Open Questions section carries dated founder rulings. `.allium-loop/dside-events.weed.json` is the current spec-versus-code divergence report (verdict: clean).
- **Design system:** `DESIGN.md` and `.impeccable/design.json`, derived from the shipped stylesheet and templates.
- **Tests:** `e2e/` is a black-box HTTP suite of 84 tests, each carrying a `// spec:` header citing the obligation it covers; 33 of 34 spec obligations are cited.
- **Real curators:** two, named in the Makefile's `CURATORS` variable. `curators.txt` is gitignored and currently holds placeholders — production has not yet been deployed from this machine.
- **Real event data:** three seed files in `seed/` (jazz, theatre, open-air cinema, September 2026), published through `make prod-seed`.
- **Absences future work must not fabricate:** there are no usage numbers, no attendee counts, no testimonials, no press, no case studies, and no pricing — the product has no metrics by design and is not yet live. Do not invent any of these, and do not present follower counts as evidence of traction.

## Product Principles

1. **The curator's minute is the budget.** Posting an event must never grow into content production. Any addition to the compose form is measured against the minute it costs the person who least wants to spend it.
2. **Nothing is measured, so nothing can be optimised against.** The absence of metrics is a product feature. A follower count is a fact displayed on a page, never a score, never social proof, and never a ranking input.
3. **The list is the product.** Two columns and an info page. Features that add a surface, a step, or a decision are the wrong shape by default; the burden of proof is on the addition.
4. **Never reach out.** No notifications, no email, no re-engagement. The visitor comes when they want to and finds the current state waiting.
5. **Works before it is enhanced.** Every capability exists as a plain HTML form first. JavaScript, view transitions and the service worker are improvements on a product that is already complete without them.

## Accessibility & Inclusion

**WCAG 2.2 AA is the floor, not an aspiration.** Contrast, focus visibility, target size and keyboard operability have a standard to fail against rather than a taste to argue with.

The mechanisms already in place and expected to be preserved: state carried in semantics (`aria-pressed` on toggles, `aria-current` on navigation and filters, `aria-invalid` with `aria-describedby` on fields), a visible 2px focus ring on every focusable element, full `prefers-reduced-motion` support that disables transitions and view transitions alike, hover treatments confined to `@media (hover: hover)` so touch devices never inherit a stuck state, and a root font size of 106.25% so body text is readable on a phone without zooming.

**One accepted exception, ruled by the founder on 2026-09-06.** The document declares `lang="en"` and keeps it. Curator-authored content is usually Greek and is not marked up as such, because there is no per-event language field and there will not be one. This is a known WCAG 3.1.2 (Language of Parts, AA) failure: a screen reader will read Greek titles with English phonetics. It is accepted rather than engineered around, and it is not an oversight to be "fixed" by a later pass — reopen it only if the founder says so.

Note that no formal audit has been performed. AA is the stated requirement; conformance must not be claimed anywhere until someone verifies it, and any claim that is made has to carry the 3.1.2 exception above.
