---
target: /
total_score: 24
max_score: 40
na_heuristics: 
p0_count: 0
p1_count: 3
target_identity: "file:/home/gspanos/dside-events/templates/home.html"
target_fingerprint: "sha256:970929ff85b6876e3aa83f2a0ddb7e0e7eaf8b824895fa7e4ac21b4c9d701bd2"
target_path: /home/gspanos/dside-events/templates/home.html
timestamp: 2026-09-06T15-18-41Z
slug: templates-home-html
---
**Method: dual-agent** (A: design review, isolated · B: detector + browser evidence, isolated). Neither saw the other's output; A finished before detector findings entered synthesis.

# Design Critique: `/` (home) — dside events

**Surface mode: Operate.** Judged on scanability, consistency and the real usage scene, not expression.

## Design Health Score

| # | Heuristic | Score | Key Issue |
|---|-----------|-------|-----------|
| 1 | Visibility of System Status | 2 | Follow's feedback is excellent; Hide removes up to 3 rows with no message, and the first press of either silently creates an account and sets a cookie. |
| 2 | Match System / Real World | 4 | `Today · Monday 7 September`, 24-hour clock, venue, price, curator. Only wobble: `0 following` reads as a fragment. |
| 3 | User Control and Freedom | 2 | The one destructive action has no undo, and its recovery page has no link from here when Mine is empty. |
| 4 | Consistency and Standards | 3 | `.hide` is the only underlined non-accent text, contradicting DESIGN.md; `header nav` and `.filters` share one language for two jobs. |
| 5 | Error Prevention | 1 | The only hazardous control: no confirmation, no undo, ~34x24px hit area, ~6px below a 145px primary. |
| 6 | Recognition Rather Than Recall | 3 | Visitor must deduce what Follow does; `One of 3 dates` never says Follow acts on all three. |
| 7 | Flexibility and Efficiency | 2 | Follow press destroys focus — the fast repeat path is worse with JS than without. No skip link. |
| 8 | Aesthetic and Minimalist Design | 3 | Held back by row bulk: ~230px per phone row, ~2,300px for ten events. |
| 9 | Error Recovery | 2 | Optimistic "Following" then a generic error page; nothing says which action failed. |
| 10 | Help and Documentation | 2 | Nothing explains Follow, Hide, the account or the series rule; the sentence that does sits at `account.html:5`. |
| **Total** | | **24/40** | **Acceptable — top of band** |

Not a mediocre interface with mediocre problems: an unusually well-made document with two sharp holes and one implementation defect dragging five heuristics down.

## Design Specificity Verdict

**Highly specific. The composition is not interchangeable — the chrome is.**

Authored for this product: the 6ch bold tabular time margin (`style.css:106,108,184`); the two-heading system separating section label from day heading on four axes at once (`style.css:172-183`); `.columns` with `section.mine` first in DOM order as the product thesis in source order; `overflow-wrap: anywhere` for Greek compounds.

Generic: the header is the default web header; the theme toggle as the bare word `dark`; the filter row looks like a list that will grow when it is permanently closed at four.

**Caveat:** DESIGN.md claims "rounded humanist" character via `ui-rounded`, which resolves to SF Rounded on Apple only. On Android the stack falls to Roboto — neither rounded nor humanist. Documentation fix, not a font change.

**Deterministic scan: clean. 0 primary findings, exit 0.** 13 advisories, all one rule (`design-system-color`), all matched value `rgb(0, 0, 0)` — 3 in scope. All false positives, one root cause: template fragments have no resolvable stylesheet (only `layout.html` carries the link, root-absolute), so the detector computes text nodes at UA default black. Proven by control: a reconstructed full page with resolvable CSS returns `[]`. `layout.html` is the one file with a stylesheet link and the one file with zero findings.

Detector independently verified: exactly three `@media` blocks, no width queries; zero `box-shadow`; zero images in templates; two inline `style=`, both per-slug `view-transition-name`.

Detector caught one thing A missed: a `flat-type-hierarchy` warning on a stripped fragment (h2 13px / h3 16px / body 17px, 1.23:1 vs 1.25 target). False positive twice over — vanishes with the masthead h1 present (1.37:1), and the flat step is h2 deliberately below body per the Two Headings Rule. Worth knowing before a `typeset` pass.

**Visual overlays: none.** No browser automation exposed. No live server started, no injection, no overlay exists, nothing screenshotted. All visual claims derived from complete source with contrast and widths computed arithmetically, independently verified.

## Overall Impression

The design system is more finished than the interaction design. The stylesheet has a point of view and defends it. The three moments a person actually touches the page — first Follow press, mis-tapped Hide, any keyboard press — are the three least designed moments on it.

Biggest opportunity: render the Mine column always, with one line of copy. It fixes five problems at once (first-visit teaching, the reassurance gap, the layout jump, the tag-filter disappearance, the wide-row proximity break). Obstacle: the literal wording of `@guarantee SideBySide`. Founder call.

## What's Working

1. **The tabular time margin is a real scanning affordance.** Tabular figures make every time identical in width, so the eye tracks a straight edge and reaches `21:00` without parsing a word. 6ch is measured, not guessed.
2. **The two-heading system solves a problem the layout creates**, on four axes simultaneously, so the distinction survives losing any single channel.
3. **Colour passes AA everywhere, both themes, one pigment.** Light: ink 13.72:1, muted 5.68:1, ochre 5.78:1, error 6.08:1. Dark: ink 13.04:1, muted 6.99:1, amber 8.37:1, error 7.34:1. No exceptions. Independently verified.

## Priority Issues

### [P1] Hide is an undo-less destructive control at ~24px, and its recovery page is unreachable from here
Borders P0. `_list.html:21`; hit area ~34x24px, ~6px below a 145px primary. No confirmation, no message, no undo; a repeating event loses every date. `href="/mine"` appears in exactly ONE place in the product — `home.html:8`, gated on `{{if .Mine.Days}}`. Hiding creates no follows, so the link never renders. Header nav has only `account · new · dark`. A visitor who mis-taps Hide before following anything has no path back from anywhere.
Under WCAG 2.2 §2.5.8 against a stated AA floor; the mistake also silently creates an account.
**Fix:** (1) `min-height: 1.75rem; padding: .25rem .5rem; margin-top: .5rem`; (2) drop the `color: var(--muted)` override at `style.css:110` — currently the only underlined non-accent text, reading as disabled while behaving as destructive; (3) render `All of mine →` whenever `.Account` is non-nil, or add `mine` to the nav. `SideBySide` governs the column, not the link.
**Command:** `/impeccable harden`

### [P1] The programme row has no authored width behaviour at either end
`.events li { grid-template-columns: 6ch 1fr auto }` with no `gap`, plus `min-width: 8.5em`. Phone (360px viewport, 328px content): time ~51px, toggle ~145px, leaving ~132px for title + five meta lines ≈ 15 chars/line; rows ~230px tall with a ~145x166px void; ten events ~2,300px scroll; `overflow-wrap: anywhere` breaks Greek mid-syllable. Anonymous visitor: Mine omitted, `auto-fit` collapses the track and gutter, Upcoming takes ~900px, Follow sits ~390px right of its text with zero column gap — proximity breaks in the default first impression. Neither state is a founder constraint.
**Fix:** one wrapping flex cell, zero breakpoints, reusing the `style.css:137` idiom:
`.events li { grid-template-columns: 6ch 1fr; column-gap: 1rem; }`
`.events li > .rowbody { display: flex; flex-wrap: wrap; gap: .5rem 1rem; align-items: start; }`
`.events li > .rowbody > div { flex: 1 1 16rem; max-width: var(--reading); }`
`.events li > .rowbody > form { flex: 0 0 auto; }`
Toggle wraps below its own line on a phone; sits after a 68ch-capped title block when wide. Independently: add `column-gap: 1rem` regardless.
**Command:** `/impeccable layout`

### [P1] Every Follow press destroys the pressed button, losing focus to `<body>`
`app.js:34-35` `columns.replaceWith(freshColumns)` discards the subtree including the pressed button. `activeElement` becomes `<body>`: following row 7 keyboard-only means re-tabbing from the top. Nothing announced — no `aria-live`, and the optimistic `aria-pressed` is thrown away with its element. The check-mark transition is cancelled mid-flight, so on the only surface taking this path "one change, not two" is never seen. Scroll anchor destroyed while Mine is inserted above.
The no-JS path is LESS broken for a keyboard user, inverting the progressive-enhancement premise.
**Fix:** after `replaceWith`, `freshColumns.querySelector('form[action="' + key + '"] [aria-pressed]')` then `.focus({ preventScroll: true })`. The refocused `aria-pressed` supplies the spoken confirmation for free.
**Command:** `/impeccable harden`

### [P2] The anonymous visit never says what Follow does — and the sentence that says it is already written
Ten `Follow` and ten `Hide` buttons, no statement that Follow is local, no sign-up, no email, no notification, or that the first press starts an account. "Mine" never appears. The reassurance exists verbatim at `account.html:5`. The allium `provides:` block declares `StartAccount` while `exposes:` declares nothing about it — the surface provides an action it does not disclose; closing that is spec-completing.
**Fix:** `{{if not .Account}}<p class="hint">Follow an event and it starts a list on this device. No sign-up, no email.</p>{{end}}` — existing class, one line, disappears once done.
**Command:** `/impeccable clarify`

### [P2] Filtering by tag silently deletes the Mine column, and the tagged empty state's only exit leads back to itself
(1) The tag correctly narrows Mine, but `home.html:5` gates the section on `{{if .Mine.Days}}`, so a visitor with four follows who taps `film` watches their personal column disappear with no explanation. (2) `handlers_feed.go:191` sets `AllHref = /upcoming?tag=`, while `home.html:13` labels it `All upcoming events →`. When filtered, "All" is false; in the tagged-empty case the only exit promises "All upcoming events" and lands on the same empty list. Also `No upcoming events tagged film.` drops the next step the untagged copy carries.
Both (1) and (2) touch strings the allium guidance names literally — route to the founder as decisions. Needing no ruling: give the tagged empty copy its next step.
**Command:** `/impeccable clarify`

## Persona Red Flags

**Casey (Distracted Mobile).** Hide ~6px below Follow at ~24px, destroys the row with no undo. 15-char lines, 230px rows, 145x166px void. Loses scroll position in BOTH paths. Strong: no images, no web fonts, ~7KB, `no-store` — instant on 3G. Fix within constraint: `Back` as `/#e-<slug>` plus an `id` per `<li>`.

**Sam (Accessibility-Dependent).** Focus destroyed on every press; no skip link (8+ tab stops to row 1); no state change announced. `aria-current="page"` sits on a non-focusable `<strong>`, not the `<a>` (`_filters.html:3`, `layout.html:19`) — screen-reader users never hear "current page" while sighted users get it from CSS. The theme control's entire accessible name is the word `dark`, naming the target state, no `aria-pressed`, announced inside a landmark labelled "Account". Strong: real elements with text labels, correct heading order, `:focus-visible` everywhere, all contrast passing, reduced-motion killing transitions and view transitions.

**Riley (Stress Tester).** Tag filter deletes a column. Tagged-empty dead end. Follow on one date changes three rows with nothing stating it. A followed event renders in both columns at once (spec-mandated, but unsignalled). Dead markup: `data-series` emitted but `app.js:35` returns before the series loop reads it.

**Eleni (Athens local, two modes — project-specific).** Mode (a) at 19:40: today first, bold tabular times, answer in ~2 seconds before reading a word — close to perfect. But today's group includes events that already started, so the front of the fixed ten can be spent on the past. Bound decision (`is_upcoming` from Athens midnight), not a defect — deserves a ruling since mode (a) is primary. Mode (b): the action she came for is unexplained, and tapping `theater` deletes half the page.

**Nikos (curator who won't self-promote — project-specific; his retention IS the success metric).** `_list.html:13-14` renders his name and immediately beneath it `{{.Followers}} following` — `0 following` on every row pre-launch. Adjacency creates the reading: his name, then a zero, ten rows deep in public. The exact shape of what he left Instagram to escape. Principle 2 is honoured in the system and undermined by the composition. The five-line row is bound; the ORDER is not — the guidance fixes the count, never the sequence. Reorder to `tags · venue/price · N following · curator`: five lines kept, adjacency broken, the name terminates the row where a programme puts attribution. One-line template change. Strong: no image slot, no avatar, no engagement affordance, nowhere a poster can be better than another — structurally unperformable-on, which is why he'd stay.

## Minor Observations

1. `header nav, .filters` styled identically (`style.css:82`) for two unrelated jobs. Filters are bound to plain links, so differentiate spatially: `margin-bottom: 1.5rem`.
2. DESIGN.md says a pressed button "darkens" on hover; `color-mix(… 82%, var(--fg))` darkens in light and brightens in dark. Fix the sentence, not the CSS.
3. `content: "✓" / ""` alt-text syntax — invalid on older Firefox, degrades acceptably. Confirm against the real browser floor.
4. README is stale: it says Hiding is only on the event page, but `_list.html:21` puts it on every row, matching the guidance.
5. Two columns fit with ~34px to spare (901px vs ~867px). Any change to `--measure`, root size or gap moves the breakpoint that officially doesn't exist.
6. Focusing a pressed Follow draws a `--link` ring outside a `--link` fill. Consider `outline-color: var(--fg)` for `[aria-pressed="true"]:focus-visible`.
7. `data-series` is dead on this surface.
8. The masthead isn't a link on `/` — correct, it's the h1. Recorded so nobody "fixes" it.

## Questions to Consider

1. Is the empty Mine column a blank space or a promise? One change fixes five problems. Is `SideBySide` protecting a principle or a phrasing?
2. Is "upcoming" the right word at 19:40? What if today's passed rows were simply `--muted` — no new surface, no spec change?
3. Mine and Upcoming overlap by construction — should the page say so?
4. `0 following` glued beneath the curator's name — a fact, or a fact in the wrong place? The row is fixed at five lines; the order is not.
5. The Follow button spends 145px of 328px to guarantee 0px reflow. Priced correctly? What's the narrowest geometry that still holds "one change, not two"?
6. What's the printed-programme rule for a single column? Should the title block obey `--reading: 68ch`, which the system already defines?
7. Where does undo live in a product that never notifies? Is the fix a link, or does Hide deserve to ask before it acts?
