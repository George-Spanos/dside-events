# Motion for dside events — proposal

Research basis: **Devouring Details** (Rauno Freiberg, https://devouringdetails.com). Every
other source below is secondary and is tied back to a Devouring Details principle.
Written 2026-09-06 against the *current* `static/style.css` (including the "Kiosk" block
appended today: ochre accent, accent-filled pressed buttons, 4px radius, tinted underlines,
`data-theme` light/dark override and the link-like `.linklike` theme switch in the header),
`static/app.js`, `templates/_list.html`, `_interest.html`, `_filters.html`, `home.html`,
`layout.html`, `event.html`, and the live pages at `http://127.0.0.1:8080/`, `/upcoming`,
`/e/dogville-2026-09-06`.

Nothing here is applied. Files: this proposal, `preview.html`, `preview-event.html`.

---

## 0. What was read, and what could not be

**Devouring Details, public pages (read in full):**

- https://devouringdetails.com — introduction, "Platform", "Structure", FAQ list.
- https://devouringdetails.com/prototypes/nextjs-dev-tools — the one free Prototype chapter
  (Focus rings, State animations, Reduced responsiveness, Debugging states, Notch anatomy,
  Scroll fading, Drag mechanics, Takeaways).
- https://devouringdetails.com/resources/behind-scenes — the free Resources chapter.

**Devouring Details, paywalled ($249):** all eight Principles chapters — Inferring intent,
Interaction metaphors, Ergonomic interactions, Simulating physics, Motion choreography,
Responsive interfaces, Contained gestures, Drawing inspiration — plus the other Prototypes
and Resources. Each `/principles/<slug>` URL returns the marketing shell (byte-identical
196 288-byte body), so only their titles are public. The free chapter cross-links to
*Ergonomic interactions* and *Responsive interfaces*, which tells us what those chapters
cover (hit-area ergonomics; cancelling animation that cannot keep up with input).

**Rauno Freiberg's public work used to fill the paywalled chapters** (the platform itself
names the first essay as its origin: "The name was inspired by my design essays like
Invisible Details of Interaction Design"):

- https://rauno.me/craft/interaction-design — *Invisible Details of Interaction Design*
  (July 2023). Sections: Metaphors, Kinetic Physics, Swipe Gestures, Responsive Gestures,
  Spatial Consistency, Fluid Morphing, Frequency & Novelty, Fidgetability, Scroll
  Landmarks, Touch Content Visibility, Implicit Input, Fitts's Law, Scrolling.
- https://rauno.me/craft/novelty — *Novelty* (February 2026).
- https://interfaces.rauno.me — *Web Interface Guidelines* (the "Motion", "Touch",
  "Typography", "Design" sections).
- https://rauno.me/craft — index.

---

## 1. Basis: Devouring Details — principles and what each means for a text-only events list

Each principle: quote or close paraphrase, source URL, then the consequence for a page
that is two lists of underlined text with one toggle button per row.

### 1.1 Sometimes the best motion is none
> "Have you ever noticed that some animation sequences can be improved with just a touch of
> delay? Or that some interactions just feel better without any motion at all?"
> — https://devouringdetails.com (introduction)

> "Some of these may be subjective, but most apply to all websites. [...] This is a matter
> of taste but some interactions just feel better with no motion. For example, the native
> macOS right click menu only animates out, not in, due to the frequent usage of it."
> — https://interfaces.rauno.me (footnote 2)

*For us:* the default answer for any element is "no animation". Every animation below has
to argue its way in. This is the same instinct as PROJECT.md's original "no animations";
the founder is loosening it, not reversing it.

### 1.2 Frequency and novelty: animate the rare, not the routine
> "It's not so obvious when not to animate something. [...] if we for a moment consider the
> interaction frequency being hundreds of times a day, it does start to feel more like
> cognitive burden after seeing the same animation for the hundredth time."
> "I removed motion from core interactions and suddenly felt like I was moving much faster."
> — https://rauno.me/craft/interaction-design (Frequency & Novelty)

> "Actions that are frequent and low in novelty should avoid extraneous animations:
> Opening a right click menu; Deleting or adding items from a list; Hovering trivial
> buttons." — https://interfaces.rauno.me (Motion)

> "make 90% of the experience familiar, and 10% novel." "Now, this animation can only be
> experienced right after logging in. When you reload the page, the page shows up instantly
> without motion." "Paco's website [...] There is a beautiful staggered animation when first
> loading the page, but navigating to another page and then back to the index page does not
> trigger this animation." "Make most things familiar, do something unexpected."
> — https://rauno.me/craft/novelty

*For us:* loading a list (every visit, many times) is routine → no entrance stagger, no
fade-in of rows. Pressing *Interested* is a considered, occasional decision → it may carry a
small acknowledgement. Hovering a link is routine → at most an instant change with a soft
release (the macOS-menu shape: in instantly, out gently).

### 1.3 Motion explains a change of state; never play it on load
> "Motion is a great way to draw attention to an element that offers new, useful
> information if the element was previously on the screen in another state."
> "Humans are sensitive to subtle changes in peripheral vision so it's important to
> communicate only useful, not cosmetic changes unrelated to their primary intent."
> "Often state transitions accidentally play out when the page loads, but this makes the
> application feel poorly built." "State transitions are best reserved for responding to
> input or changes in the system."
> — https://devouringdetails.com/prototypes/nextjs-dev-tools (State animations)

*For us:* the toggle inversion and the updated "N interested" count are state changes in
response to input → animate them. First render of any page → nothing may animate. Anything
using `@starting-style` or an entrance keyframe must be scoped so it cannot fire on load.

### 1.4 Responsive interfaces: acknowledge immediately, and skip what cannot keep up
> "Truly fluid gestures are immediately responsive." "It feels a lot better by feeling the
> scale delta applying immediately, and then performing an animation past a given
> threshold." — https://rauno.me/craft/interaction-design (Responsive Gestures)

> "Sometimes updates happen with higher frequency than an animation can run. [...] we can
> measure how much time has passed since each update, and cancel animations" — with
> `const animationDurationMs = 150`. Takeaway: "Skip animations for high-frequency updates."
> — https://devouringdetails.com/prototypes/nextjs-dev-tools (Reduced responsiveness)

> "Toggles should immediately take effect, not require confirmation." "Optimistically
> update data locally and roll back on server error with feedback."
> — https://interfaces.rauno.me (Interactivity, Design)

*For us:* the press must register on the same frame (instant `:active`), the state flip is
optimistic (app.js already does this), and the acknowledgement is in the 150 ms class —
Rauno's own working number.

### 1.5 Interruptibility and robustness
> "Great interactions are modeled after properties from the real world, like
> interruptability. [...] imagine if it were an animation that you had to wait for!"
> — https://rauno.me/craft/interaction-design (Metaphors)

> "Interactions should feel robust, interruptible, and in the worst case—tolerate spamming
> without breaking." — https://devouringdetails.com/prototypes/nextjs-dev-tools (Debugging states)

*For us:* only CSS transitions (which retarget mid-flight for free), never JS timers or
`transitionend` for correctness. app.js's `data-busy` guard already tolerates spamming;
motion must not add state to it.

### 1.6 Spatial consistency: motion tells you where a thing came from
> "Through motion this helps establish a relationship between the audio player and its
> source." "By moving in from the right, not left, it also signifies that the app is now
> first on the stack." — https://rauno.me/craft/interaction-design (Spatial Consistency)

*For us:* the one place this applies is row title → event page `<h1>`: the title you
pressed is the title you land on. A shared-element view transition is the web's version.
Everything else (header, filters) should *stay still* rather than move.

### 1.7 Proportion: motion scaled to the trigger
> "Animation duration should not be more than 200ms for interactions to feel immediate."
> "Animation values should be proportional to the trigger size: Don't animate dialog scale
> in from 0 → 1, fade opacity and scale from ~0.8; Don't scale buttons on press from 1 → 0.8,
> but ~0.96, ~0.9, or so." — https://interfaces.rauno.me (Motion)

*For us:* ≤200 ms for anything you trigger; a button press is a ~3 % scale at most;
a page change may take the same 200 ms because the whole viewport is the trigger.

### 1.8 Feedback belongs next to its trigger; the "blink of assurance"
> "Display feedback relative to its trigger: Show a temporary inline checkmark on a
> successful copy, not a notification." — https://interfaces.rauno.me (Design)

> "the selected item briefly blinks the accent color (pink) to provide assurance that the
> element was successfully selected. I can only assume that the menu fading out makes this
> feel more graceful and intentional than abruptly disappearing after the blink"
> — https://rauno.me/craft/interaction-design (Frequency & Novelty)

*For us:* the ✓ and the accent fill *inside the button* are the feedback. No toasts, no
banners. The count next to the button may briefly fade in when its value changes.

### 1.9 Input modality: pointer and keyboard are mechanical, touch is visceral
> "There is an inherent disconnect between input from peripheral devices and what happens
> on the screen. Pressing a key feels less visceral, and more mechanical than touching the
> screen." — https://rauno.me/craft/interaction-design (Frequency & Novelty)

> "Hover states should not be visible on touch press, use @media (hover: hover)."
> "Font weight should not change on hover or selected state to prevent layout shift."
> — https://interfaces.rauno.me (Touch, Typography)

*For us:* keyboard focus rings do not animate. Hover styling only under `(hover: hover)`.
The current `font-weight: bold` on pressed buttons is a layout-shifting state change — the
one thing Rauno explicitly forbids for a selected state.

### 1.10 Details that don't announce themselves
> "there are hundreds of design decisions made by someone obsessesing over the tiniest
> margins so that when they work, no one has to think about." — https://rauno.me/craft/interaction-design (Metaphors)

> "Switching themes should not trigger transitions and animations on elements."
> "Looping animations should pause when not visible on the screen."
> — https://interfaces.rauno.me (Motion)

*For us:* nothing may look like "an animation". Nothing loops. Success is the founder
saying "it feels nicer" without being able to point at what moved.

### 1.11 Reduced motion — the gap in the public corpus
Devouring Details' public pages and Rauno's essays never mention `prefers-reduced-motion`.
The closest is the platform's own preference switch: "all videos on the platform will auto
play by default. Depending on the context and person, this behavior can be distracting or
convenient. The platform offers a preference to disable or enable this behavior"
(https://devouringdetails.com/resources/behind-scenes, Platform ergonomics). The principle —
motion is optional and the person decides — is there; the mechanism comes from the
secondary sources (section 3). For this site: one `@media (prefers-reduced-motion: reduce)`
block turns everything off, including page transitions.

### 1.12 Lists, toggles, page transitions, typography-only interfaces — what DD says
- Lists: "Deleting or adding items from a list" is named as a case that should *not* animate
  (interfaces.rauno.me); bmrks.com list add/remove motion was removed as "sluggish".
- Toggles: "immediately take effect" (interfaces.rauno.me).
- Page transitions: the Novelty essay accepts a bold route transition only "for a microsite
  [...] as a beautiful contrast against the navigation of the otherwise static sibling
  pages" — "I don't want to navigate all of the pages on a given website this way." So a
  page transition on an everyday site must be near-invisible: a short crossfade, one moving
  element at most.
- Typography-only: no direct statement. Derived: the only things that can move are colour,
  opacity and the geometry of one snapshot; text should never scale in place (it blurs).

---

## 2. Evaluation of dside events against each principle

| Principle | Today | Verdict |
|---|---|---|
| 1.1 None by default | `style.css` has zero `transition`/`animation`. | Embodies. Keep as the baseline; add only what is argued below. |
| 1.2 Frequency/novelty | Nothing animates, so nothing routine animates. | Embodies. Risk is *adding* a row stagger or hover flourish — do not. |
| 1.3 State change, not load | app.js flips `aria-pressed` on submit; the pressed style snaps from outline to accent fill with no transition. The ✓ prefix is *not* set by JS; it arrives only when the server's HTML replaces the form (`form.replaceWith(fresh)`) — a second visual change 50–400 ms later, and the button's width jumps when "✓ " is inserted. | Opportunity. One-step acknowledgement: fill and ✓ together, geometry stable, 150 ms. |
| 1.4 Immediate, optimistic | Optimistic flip, `data-busy` guard, native fallback on error (`form.dataset.native = '1'; btn.click()`). | Embodies. Rauno's "optimistically update, roll back on error" is already exactly the code. Only the `:active` press has no visible response at all today. |
| 1.5 Interruptible | No animation, so trivially. `data-busy` blocks a second submit until the swap. | Embodies. New motion must be CSS-only so it stays this way. |
| 1.6 Spatial consistency | Full page reload between `/` and `/e/{slug}`: white flash, no link between the pressed title and the heading. Header re-renders (`<h1>` on `/`, `<p class="brand">` elsewhere). | Opportunity. Cross-document view transitions: title morphs, header and filters stay still. |
| 1.7 Proportion | n/a | Set the scale now: 120/150/200 ms, press scale .97. |
| 1.8 Feedback by the trigger | ✓ inside the button, `.count` beside it on the event page, "Hidden from your feed." inline. No toasts. | Embodies. The count change is the one thing worth a 150 ms fade because it is *new information* (1.3). |
| 1.9 Modality | `:focus-visible` outline (good, keep static). No `:hover` rule at all. `button[aria-pressed="true"] { font-weight: bold }` → text widens on press. | Violates (bold on pressed). Recommend dropping bold: the accent fill and ✓ already carry the state twice. Hover: add only under `(hover: hover)`. |
| 1.10 Unannounced | Kiosk block adds `color-mix` tinted underlines and 4px radii — quiet. The theme switch is a plain POST → 303 → same page with `data-theme` set: a full navigation, so no element-level transitions can fire on it (Rauno: "Switching themes should not trigger transitions"). | Embodies. Keep motion at the same volume. With §6 in place the theme switch becomes one 200 ms root crossfade from light to dark — the whole page, once, not per element. Judged pleasant; off under reduced motion. |
| 1.11 Reduced motion | No media query (nothing to reduce). | Must add with the first transition. |
| 1.12 Lists/pages | Rows are static; navigation is a hard reload. | Rows stay static. Navigation gets the near-invisible crossfade. |

Other observations from the real markup (curl):
- The same event can appear in **both** columns of `/` (Mine and Upcoming). Any per-slug
  `view-transition-name` must therefore be emitted for one column only, or the browser
  skips the whole transition (MDN: "If two rendered elements have the same
  view-transition-name at the same time, the ViewTransition.ready Promise will reject and
  the transition will be skipped").
- The row title `<a>` is inline. A wrapped two-line title is a fragmented box; give it
  `display: inline-block` so its snapshot is a single box.
- `sw.js` serves navigations network-only (never caches HTML) and `/static/*` cache-first.
  Both are same-origin and do not interfere with cross-document view transitions.
- Pages are `Cache-Control: no-store`; static is `immutable` with `?v=` — adding CSS is a
  normal version bump.

---

## 3. Secondary sources, each measured against Devouring Details

| Source | Says | vs DD | Side taken for dside |
|---|---|---|---|
| **Chrome Developers — Cross-document view transitions** https://developer.chrome.com/docs/web-platform/view-transitions/cross-document | Opt-in is `@view-transition { navigation: auto }` on *both* pages; same-origin only; not for reload/URL-bar navigations; skipped with `TimeoutError` if navigation takes "more than four seconds in Chrome's case"; elements with a unique `view-transition-name` on both pages are snapshotted and morphed; "for best effect [...] you need fast loading pages". Support: Chrome/Edge 126, Safari 18.2, Firefox ✗. | **Refines 1.6** (spatial consistency) with a zero-JS mechanism, and **agrees with 1.5**: the browser skips rather than blocks when it cannot keep up. | Take it. It is the only way a server-rendered site gets continuity, and it degrades to today's behaviour. |
| **CSS View Transitions Level 1 (W3C), UA stylesheet** https://drafts.csswg.org/css-view-transitions-1/#ua-styles | `::view-transition-group(*) { animation-duration: 0.25s }`, default cross-fade keyframes, timing inherited down the pseudo-tree. | **Contradicts 1.7 mildly**: 250 ms > Rauno's 200 ms ceiling. | Override to 200 ms with our curve (one rule on `::view-transition-group(*)`). |
| **MDN — `view-transition-name`, `@view-transition`** https://developer.mozilla.org/en-US/docs/Web/CSS/view-transition-name , https://developer.mozilla.org/en-US/docs/Web/CSS/@view-transition | Duplicate names → transition skipped. `match-element` is auto-unique but internal, so useless across documents. `navigation: auto \| none`. BCD: `view-transition-name` Chrome 111 / Firefox 144 / Safari 18; `@view-transition` Chrome 126 / Safari 18.2 / Firefox none. | **Refines 1.6**: constrains the morph to one name per element per page. | Emit the per-slug name only in the page's primary list (see §6). |
| **MDN — `@starting-style`, `transition-behavior: allow-discrete`** https://developer.mozilla.org/en-US/docs/Web/CSS/@starting-style , https://developer.mozilla.org/en-US/docs/Web/CSS/transition-behavior | "CSS transitions are by default not triggered on an element's initial style update"; `@starting-style` enables entry transitions; `allow-discrete` lets `display` transition. BCD: `@starting-style` Chrome 117 / Firefox 129 / Safari 17.5; `transition-behavior` Chrome 117 / Firefox 129 / Safari 17.4. | **Contradicts 1.3 in practice**: a bare `@starting-style` fires on every element's first render, i.e. on page load — the exact "accidentally plays on load" DD warns about. | Not used. A plain `@keyframes` on `form[data-swapped]` (one attribute set by app.js) fires only on the swapped form, never on load, and works in every browser. |
| **Apple HIG — Motion** https://developer.apple.com/design/human-interface-guidelines/motion | "Add motion purposefully [...] Don't add motion for the sake of adding motion." "Make motion optional." "Aim for brevity and precision in feedback animations. When animated feedback is brief and precise, it tends to feel lightweight and unobtrusive." "In apps, generally avoid adding motion to UI interactions that occur frequently." "Let people cancel motion. As much as possible, don't make people wait for an animation to complete." | **Agrees with 1.1, 1.2, 1.5, 1.11** almost sentence for sentence. | Take it; it is DD's argument from the platform vendor. |
| **Material Design — Duration & easing** (M1 spec, still the canonical numbers) https://m1.material.io/motion/duration-easing.html ; M3 tokens https://m3.material.io/styles/motion/easing-and-duration/tokens-specs | Mobile ~300 ms ("Elements entering the screen occur over 225ms. Elements leaving the screen occur over 195ms. Transitions that exceed 400ms may feel too slow"); "Desktop animations should be faster [...] 150ms to 200ms"; "Objects leaving the screen may have shorter durations"; standard curve `cubic-bezier(0.4, 0, 0.2, 1)`; M3 standard `cubic-bezier(0.2, 0, 0, 1)`, `duration.short4` = 200 ms (M3 page is JS-rendered; these two values verified via search, the rest of the M3 table not re-verified). | **Refines 1.7**: gives the exit-shorter-than-entry rule and a named curve; **contradicts 1.7** on mobile (300 ms). | Take the desktop numbers everywhere (150–200 ms) — the founder wants "discreet", and the mobile allowance is for large moving surfaces we don't have. Take one M3 curve as the single easing family. |
| **Nielsen Norman Group — The Role of Animation and Motion in UX** (Page Laubheimer, 2020) https://www.nngroup.com/articles/animation-purpose-ux/ | Purposes: feedback, state change, spatial relationships, orientation. Costs: the visual system is "sensitive and prone to be distracted by any type of motion (meaningful or not)"; gratuitous animation "distract[s] and annoy[s]". Animation should be "subtle, unobtrusive, and brief". | **Agrees with 1.3 and 1.6**; gives the research vocabulary for DD's "only useful, not cosmetic changes". | Use its four purposes as the admission test in §4. |
| **Josh Comeau — An Interactive Guide to CSS Transitions** https://www.joshwcomeau.com/animation/css-transitions/ | ease-out for entering, ease-in for exiting, "linear is rarely the best choice"; only `transform` and `opacity` are cheap; hover: "make the enter animation quick and snappy, while the exit animation can be a bit more relaxed"; `@media (prefers-reduced-motion: reduce) { transition: none }`. | **Agrees with 1.2's macOS observation** (instant in, soft out) and gives it a CSS shape; **refines 1.11** with the media query. | Take the asymmetric hover and the media query. Use one curve rather than two: our motions are so short that entry/exit curve differences are imperceptible, and one token is simpler. |
| **ui-ux-pro-max skill data** `/home/gspanos/.claude/skills/ui-ux-pro-max/data/motion.csv`, `data/ux-guidelines.csv`, `references/quick-reference.md`, `references/pro-rules.md` | Hover micro-interaction 150–200 ms, "Keep displacement under 2px so it reads as feedback not motion"; page transition subtle 200–300 ms, "cap exit duration at ~250ms"; "Animate 1-2 key elements per view maximum"; `cancellable-state-transitions`: never depend on `transitionend` for correctness; "Use color, opacity, or elevation transitions for press states without changing layout bounds"; tap feedback within 80–150 ms; `exit-faster-than-enter`; `spring-physics` ("Prefer spring/physics-based curves"); shared-element route transitions 500–800 ms with `expo.inOut`. | **Agrees with 1.4, 1.5, 1.7, 1.9**. **Contradicts 1.2/1.7** in two places: springs, and 500–800 ms hero morphs. | Take the agreements. Reject springs (they overshoot; a typography-only page has nothing to bounce) and the long morph (DD's Novelty essay: everyday navigation must not be the special moment). |

Where the secondary sources disagree with each other (M3 300 ms mobile vs Rauno's 200 ms
ceiling; skill's springs vs Comeau's curves), Devouring Details decides.

---

## 4. Motion principles for dside events

1. **Motion only to explain a state change you caused, or to connect two views you moved
   between.** Nothing on load, nothing on scroll, nothing that loops. (1.1, 1.2, 1.3; NN/g's
   four purposes as the test.)
2. **Three durations, one curve.** 120 ms release, 150 ms acknowledgement, 200 ms page;
   `cubic-bezier(.2, 0, 0, 1)` for all. Nothing longer than 200 ms, ever. (1.7, Material desktop.)
3. **In instantly, out softly.** The press and the hover register on the same frame; only
   the return eases. (1.4, macOS menu in 1.2, Comeau.)
4. **Colour, opacity, and one travelling snapshot. Never geometry in place.** No width or
   weight changes, no lifts, no scale beyond 3 %. (1.9, 1.10.)
5. **`prefers-reduced-motion: reduce` turns all of it off**, page transitions included; the
   final state appears immediately. (1.11, Apple "Make motion optional".)

---

## 5. Interactions

Each item: what happens, timing, the principle.

**5.1 Interested / Not interested / Follow — the press**
`button:active { transform: scale(.97); transition-duration: 0s }` then the release eases
back over 120 ms. Instant down = 1.4; ~3 % = 1.7 ("~0.96"). Under reduced motion the scale is
removed entirely.

**5.2 The state flip (inversion)**
`background-color`, `border-color`, `color` transition 150 ms from outline to accent fill.
This runs the moment app.js sets `aria-pressed` (optimistic), so it is the acknowledgement
(1.4, 1.8). 150 ms is Rauno's own `animationDurationMs` (1.4). It also runs on the no-JS
path — see 5.6.

**5.3 The ✓**
Today the ✓ is text from the server and arrives with the swapped form, after the fill.
Proposal: render it from CSS, `button[aria-pressed="true"]::before { content: "✓" / "" }`,
in a **reserved 1.15 em slot** that exists in both states, fading 0→1 over the same 150 ms.
Result: one change, not two; the button never changes width (1.3, 1.9). Cost: remove the
three `{{if …}}✓ {{end}}` fragments from `_list.html` and `_interest.html`, and the Follow
buttons gain a ✓ when pressed (which the existing comment in style.css — "pressed = fg/bg
swapped, bold, label gets a check mark" — already promises). The `/ ""` alternative text
keeps screen readers on `aria-pressed`. Alternative if the founder dislikes the leading slot:
drop the slot and accept the width jump, or set the text optimistically in app.js (one line;
but the server's Follow buttons carry no ✓, so it would vanish on swap — not recommended).

**5.4 Bold on pressed — remove**
`font-weight: bold` on `[aria-pressed="true"]` widens the label and shifts the row's grid
(1.9: "Font weight should not change on hover or selected state to prevent layout shift").
The accent fill and ✓ are already two indicators. One-line change; a design call for the
founder.

**5.5 Links — hover and focus**
Under `@media (hover: hover)` only (1.9): on hover the underline goes from the Kiosk 55 %
tint to full `--link` **instantly**; on leave it fades back over 150 ms — the macOS-menu shape
(1.2) and Comeau's asymmetry. No offset or thickness change (thickness animation is uneven
across engines and reads as movement). Focus rings: unchanged, no transition — keyboard is
mechanical (1.9).

**5.6 The JS-swapped form**
Do **not** fade the whole swapped form: it is pixel-identical to the optimistic state, so a
fade would read as a flicker — the "poorly built" tell (1.3). Fade only what is *new
information*: `.count` ("2 interested") and the `<small>Hidden from your feed.</small>`,
150 ms, via a keyframe scoped to `form[data-swapped]`. app.js sets that attribute on the
fresh form before `replaceWith` (one line), so nothing can fire on page load (1.3). On list
rows the count lives outside the form and does not update — nothing to animate there.

**5.7 No-JS path**
Before the redirect: only 5.1 (the press) is visible; the form posts and the browser
navigates. After the 303 to `back`: with §6 in place the returned page arrives through the
root crossfade (a same-origin, user-initiated `push`/`replace`), so the toggled button fades
from outline to fill along with the page — no extra work. Without view-transition support
the page simply reloads, as today.

---

## 6. Browsing

**6.1 Cross-document view transitions** for `/` ↔ `/e/{slug}` ↔ `/upcoming` ↔ `/mine` ↔
`/account` ↔ `/following` ↔ `/p/{poster}` — one `@view-transition { navigation: auto }` in
the shared stylesheet opts every page in (both sides must opt in; they do, it's one file).
Default is a root crossfade; we set it to 200 ms with our curve (§3: spec default 250 ms).
Principle: 1.6 (continuity) kept at the volume of 1.2 (everyday navigation must stay
unremarkable — Novelty essay).

**Static elements:** `header { view-transition-name: header }` and
`.filters { view-transition-name: filters }`. Named elements are snapshotted separately, so
the header sits still while the content crossfades beneath it (1.6: "stay still rather than
move"). `/` renders the brand as `<h1>` and other pages as `<p class="brand">` in the same
place — a 200 ms crossfade of two identical-looking headers is invisible, which is the point.
The filters nav exists on `/`, `/upcoming`, `/following` only; elsewhere it fades out on its
own.

**Shared-element morph, row title → event `<h1>`:** emit
`style="view-transition-name: e-{{.Slug}}"` on the row title `<a>` and on `event.html`'s
`<h1>`. Slugs are ASCII (`dogville-2026-09-06`); the `e-` prefix keeps the ident valid if a
slug ever starts with a digit. The browser then moves and scales the title's snapshot from
row to heading over 200 ms while the rest crossfades. Back navigation reverses it.

Constraints and fallbacks:
- **One element per name per page.** `/` can show the same event in *Mine* and *Upcoming*.
  Emit the name for the page's primary list only: a boolean on the day-list data
  (`{{if .Morph}} style="…"{{end}}` in `_list.html`; set true for `Upcoming` on `/`, for the
  single list on `/upcoming`, `/mine`, `/following`, `/p/*`). Never on `pastlist` or the
  Hidden list. If a duplicate ever slips through, the browser skips that navigation's
  transition — the page still loads.
- **Fragmentation.** `.events li > div > a:first-child { display: inline-block }` so a
  wrapped title is one box; visually unchanged.
- **Text scaling.** The morph scales a bitmap of ~17 px text to ~30 px; at 200 ms this is a
  brief soft blur, acceptable. If it bothers the founder, drop the per-slug names and keep
  the root crossfade + static header — still 1.6, one line less.
- **Speed.** Chrome skips the transition if the navigation exceeds 4 s; pages are tiny and
  the SW is network-only for HTML, so this only bites on a dead connection, where it should.
- **Not a transition:** reload, URL bar, bookmarks (by spec) — correct, those are not
  "moving between views".
- **Theme switch.** `/theme` answers with a 303 back to the same path, so pressing
  *dark*/*light* is a same-origin, user-initiated navigation and gets the 200 ms root
  crossfade: the whole page fades from light to dark, once. This is the opposite of the
  per-element theme flicker Rauno warns about (interfaces.rauno.me, Motion), and it is the
  one place where a document-wide fade is exactly the right unit. Off under reduced motion
  like everything else.

**6.2 Page-load stagger of rows — no.**
Every principle says no: rows load on every visit (1.2, "hundreds of times"); a stagger is
motion on load (1.3, "feel poorly built"); the Novelty essay's only sanctioned stagger is
first-load-only and never on return; and it would run *inside* the crossfade, doubling the
motion on a page whose whole point is scanning times and titles. A text list is fastest
when it is simply there.

---

## 7. Exact CSS, JS, support, and the "not doing" list

### 7.1 CSS to append to `static/style.css` (49 lines)

```css
/* Motion. Basis: Devouring Details (Rauno Freiberg). Three durations, one curve; motion only
   for a state change you caused or a view you moved to; everything off under reduced motion. */
:root { --ease: cubic-bezier(.2, 0, 0, 1); --t-out: 120ms; --t: 150ms; --t-page: 200ms; }

/* Responsive interfaces: the inversion is the acknowledgement. Colour only, no geometry.
   .linklike (the theme switch) reads as a link and behaves like one below. */
button:not(.linklike) {
  transition: background-color var(--t) var(--ease), border-color var(--t) var(--ease),
              color var(--t) var(--ease), transform var(--t-out) var(--ease);
}
/* Proportional press (WIG: ~0.96, never 0.8): instant down, eased release. */
button:not(.linklike):active { transform: scale(.97); transition-duration: 0s; }

/* One change, not two: the ✓ appears with the fill, in a slot that exists in both states,
   so the button never changes width. Templates must stop emitting "✓ ". */
button[aria-pressed]::before {
  content: ""; display: inline-block; width: 1.15em; opacity: 0;
  transition: opacity var(--t) var(--ease);
}
button[aria-pressed="true"]::before { content: "✓" / ""; opacity: 1; }
button[aria-pressed="true"] { font-weight: inherit; } /* no layout shift on selected state */

/* Frequency & novelty: hover is routine, so it shows instantly (like a macOS menu)
   and only fades out. Pointer devices only; touch never sees it. */
@media (hover: hover) {
  a, .linklike { transition: text-decoration-color var(--t) var(--ease); }
  a:hover, .linklike:hover { text-decoration-color: var(--link); transition-duration: 0s; }
}

/* Draw attention to new information after the optimistic swap — only inside the form
   app.js just replaced (data-swapped), never on page load. */
@keyframes fade-in { from { opacity: 0; } }
form[data-swapped] .count, form[data-swapped] small { animation: fade-in var(--t) var(--ease); }

/* Spatial consistency between documents: root crossfade at 200 ms (spec default 250),
   header and filters stay still, the pressed title travels to the event heading
   (per-slug view-transition-name is set inline by the templates). */
@view-transition { navigation: auto; }
::view-transition-group(*) { animation-duration: 200ms; animation-timing-function: cubic-bezier(.2, 0, 0, 1); }
header { view-transition-name: header; }
.filters { view-transition-name: filters; }
.events li > div > a:first-child { display: inline-block; } /* one box, one snapshot */

/* Make motion optional: final state immediately, page transitions included. */
@media (prefers-reduced-motion: reduce) {
  *, ::before, ::after { transition: none !important; animation: none !important; }
  button:active { transform: none; }
  ::view-transition-group(*), ::view-transition-old(*), ::view-transition-new(*) { animation: none !important; }
}
```

Notes on the block:
- `--t-out`, `--t`, `--t-page` are the whole vocabulary; `::view-transition-group(*)` gets
  literal values because custom-property inheritance into the view-transition pseudo-tree
  is not something to depend on.
- The explicit theme switch (`form.theme`) is a navigation, so no element transitions run;
  the page crossfades as a whole (§6.1). The `color` transition on buttons *does* run when the
  OS itself flips light/dark while a page is open (150 ms, colours only). Rauno's guideline
  says themes should not trigger transitions; CSS cannot distinguish that cause without JS,
  and a 150 ms colour fade on a few buttons is judged acceptable. If not, drop `color` and
  `border-color` from the list and keep `background-color`.
- `.linklike` (the theme switch button) is excluded from the button rules and included in the
  link hover rule: it looks like a link, so it must move like one — i.e. not at all.

### 7.2 JS — one line in `static/app.js`

In the `.then(function (html) {…})` block:

```js
if (fresh) { fresh.dataset.swapped = '1'; form.replaceWith(fresh); } else delete form.dataset.busy;
```

(currently `if (fresh) form.replaceWith(fresh); else delete form.dataset.busy;`). Nothing
else; the inversion already animates from the existing `aria-pressed` flip.

### 7.3 Templates — small edits (not motion, but required by 5.3 and 6.1)

- `_list.html`: title link becomes
  `<a href="/e/{{.Slug}}"{{if .Morph}} style="view-transition-name: e-{{.Slug}}"{{end}}>{{.Title}}</a>`;
  remove `{{if .Marked}}✓ {{end}}` from the button label.
- `_interest.html`: remove the two `{{if eq .State …}}✓ {{end}}` fragments.
- `event.html`: `<h1 style="view-transition-name: e-{{.Event.Slug}}">{{.Event.Title}}</h1>`.
- Go: a `Morph bool` on the day-list view data, true for the primary list of each page.
  (Alternative with zero Go: name titles on every page's *first* `.events` list only via
  the template's range index — but `/` interleaves Mine before Upcoming, so the flag is cleaner.)

### 7.4 Browser support and graceful fallback

| Feature | Chrome/Edge | Firefox | Safari | If unsupported |
|---|---|---|---|---|
| `transition` on colour/transform/opacity | all | all | all | — |
| `@media (prefers-reduced-motion)` | 74 | 63 | 10.1 | Motion plays (only very old browsers) |
| `@media (hover: hover)` | 38 | 64 | 9 | No hover fade at all |
| `text-decoration-color` transition | yes | yes | yes | Instant colour change |
| `content: "✓" / ""` alternative text | 77 | 128 | 17.4 | Older Firefox: whole declaration invalid → no ✓ rendered; button still inverts and `aria-pressed` still speaks. Acceptable, or omit the `/ ""` and let readers hear "check mark". |
| `@keyframes` on inserted element | all | all | all | — |
| `@view-transition { navigation: auto }` | 126 | ✗ (not shipped, 2026-09) | 18.2 | Hard navigation exactly as today |
| `view-transition-name` (cross-doc) | 126 | ✗ | 18.2 | Ignored |
| `::view-transition-*` timing overrides | 126 | ✗ | 18.2 | Ignored |
| `color-mix()` (already used by Kiosk) | 111 | 113 | 16.2 | Untinted underline (existing behaviour) |
| Not used: `@starting-style` | 117 | 129 | 17.5 | (reason in §3) |
| Not used: `transition-behavior: allow-discrete` | 117 | 129 | 17.4 | (nothing here transitions `display`) |

Versions from MDN browser-compat-data (2026-09-06). Every row degrades to the current
site; nothing is required for function.

### 7.5 Not doing

- Parallax, scroll-linked or scroll-reveal effects (1.2, 1.3; NN/g distraction).
- Skeletons and spinners (the optimistic flip *is* the loading state; DD 1.4).
- Row stagger or fade-in on load (1.2, 1.3, Novelty essay).
- Whole-form fade on swap (identical pixels; would flicker — 1.3).
- Bouncy/spring/elastic easings (1.7; nothing here has mass).
- Hover lifts, shadows, scale or colour changes on link text (1.9, 1.10; Kiosk has no shadows).
- Animated focus rings (1.9).
- Animating `text-underline-offset`/`thickness`, `font-weight`, `width`, `height` (1.9, 1.10).
- Anything longer than 200 ms; anything over 400 ms is excluded twice over (1.7, Material).
- Any per-page "special" route transition (Novelty essay: contrast, not routine).
- Toasts, banners, count-up numbers (1.8).
- Theme-switch transitions beyond the accepted 150 ms colour fade on buttons.

---

## Preview

- `preview.html` — self-contained, works from `file://`. Real row markup from `/` (Kiosk
  stylesheet inlined) plus the event-page `.interest` form. Press *Interested*: the
  optimistic flip animates, then a simulated server round-trip (350 ms) swaps in a fresh
  form with `data-swapped` and a changed count so the fade-in of new information is visible.
  A checkbox simulates `prefers-reduced-motion: reduce` by mirroring the media-query rules
  under `html.reduce` (the real rule is present too, so an OS setting works as well).
- `preview-event.html` — the event page, linked both ways with `preview.html`. Both carry
  `@view-transition { navigation: auto }` and matching `view-transition-name`s
  (`header`, `filters`, `e-dogville-2026-09-06`). **Cross-document transitions need a real
  same-origin URL** (file:// origins are opaque), so serve the folder:

  ```
  cd /tmp/claude-1000/-home-gspanos-dside-events/0a2344fc-fd43-4342-a6e4-681fc8bf690a/scratchpad/motion
  python3 -m http.server 8090
  # open http://localhost:8090/preview.html in Chrome 126+ / Safari 18.2+, click "Dogville"
  ```
  Firefox shows the same pages with a plain navigation — that is the fallback, on purpose.
