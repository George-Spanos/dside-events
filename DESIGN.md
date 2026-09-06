---
name: dside events
description: Upcoming events in Athens, posted by people we know — set like a printed programme.
colors:
  newsprint-warm-white: "#FCF8EF"
  printers-ink: "#2E2823"
  pencil-grey: "#6C6157"
  fold-line: "#E2D9C9"
  burnt-ochre: "#875800"
  alarm-red: "#B5261A"
  newsprint-warm-white-dark: "#272320"
  printers-ink-dark: "#F1EADF"
  pencil-grey-dark: "#B8AC9D"
  fold-line-dark: "#4B433B"
  lamp-amber: "#E9B653"
  alarm-red-dark: "#FF9585"
typography:
  display:
    fontFamily: "ui-rounded, Trebuchet MS, Ubuntu, Segoe UI, Roboto, Noto Sans, sans-serif"
    fontSize: "1.75rem"
    fontWeight: 700
    lineHeight: 1.2
    letterSpacing: "-0.02em"
  headline:
    fontFamily: "ui-rounded, Trebuchet MS, Ubuntu, Segoe UI, Roboto, Noto Sans, sans-serif"
    fontSize: "1.375rem"
    fontWeight: 700
    lineHeight: 1.5
    letterSpacing: "-0.03em"
  title:
    fontFamily: "ui-rounded, Trebuchet MS, Ubuntu, Segoe UI, Roboto, Noto Sans, sans-serif"
    fontSize: "1rem"
    fontWeight: 400
    lineHeight: 1.5
    letterSpacing: "normal"
  body:
    fontFamily: "ui-rounded, Trebuchet MS, Ubuntu, Segoe UI, Roboto, Noto Sans, sans-serif"
    fontSize: "1rem"
    fontWeight: 400
    lineHeight: 1.5
    letterSpacing: "normal"
  label:
    fontFamily: "ui-rounded, Trebuchet MS, Ubuntu, Segoe UI, Roboto, Noto Sans, sans-serif"
    fontSize: "0.8125rem"
    fontWeight: 700
    lineHeight: 1.5
    letterSpacing: "0.1em"
  meta:
    fontFamily: "ui-rounded, Trebuchet MS, Ubuntu, Segoe UI, Roboto, Noto Sans, sans-serif"
    fontSize: "0.9375rem"
    fontWeight: 400
    lineHeight: 1.5
    letterSpacing: "normal"
  time:
    fontFamily: "ui-rounded, Trebuchet MS, Ubuntu, Segoe UI, Roboto, Noto Sans, sans-serif"
    fontSize: "1rem"
    fontWeight: 700
    lineHeight: 1.5
    letterSpacing: "normal"
    fontFeature: "tabular-nums"
rounded:
  none: "0"
  sm: "4px"
spacing:
  xs: "0.25rem"
  sm: "0.5rem"
  md: "1rem"
  lg: "1.5rem"
  xl: "2rem"
  xxl: "2.5rem"
  xxxl: "3rem"
components:
  button:
    backgroundColor: "{colors.newsprint-warm-white}"
    textColor: "{colors.printers-ink}"
    typography: "{typography.body}"
    rounded: "{rounded.sm}"
    padding: "0.3rem 0.85rem"
    height: "2rem"
  button-pressed:
    backgroundColor: "{colors.burnt-ochre}"
    textColor: "{colors.newsprint-warm-white}"
    rounded: "{rounded.sm}"
    padding: "0.3rem 0.85rem"
    width: "8.5em"
  button-hover:
    backgroundColor: "{colors.newsprint-warm-white}"
    textColor: "{colors.burnt-ochre}"
    rounded: "{rounded.sm}"
    padding: "0.3rem 0.85rem"
  button-linklike:
    backgroundColor: "transparent"
    textColor: "{colors.burnt-ochre}"
    typography: "{typography.body}"
    rounded: "{rounded.none}"
    padding: "0"
    height: "auto"
  input:
    backgroundColor: "{colors.newsprint-warm-white}"
    textColor: "{colors.printers-ink}"
    typography: "{typography.body}"
    rounded: "{rounded.none}"
    padding: "0.375rem 0.5rem"
    width: "100%"
  input-invalid:
    backgroundColor: "{colors.newsprint-warm-white}"
    textColor: "{colors.printers-ink}"
    rounded: "{rounded.none}"
    padding: "0.375rem 0.5rem"
---

# Design System: dside events

## Overview

**Creative North Star: "The Printed Programme"**

A theatre season sheet. `UPCOMING` is set in small caps over a heavy rule; the days below it are mixed-case ink over a hairline. Times run bold and tabular down a six-character left margin, so the eye can drop straight through the column and find 21:00 without reading a word. The page is a document you read, not a surface you operate — and the whole thing is one 241-line stylesheet with no build step, no framework, and no breakpoints.

The mood is warm, plain and unhurried. Warm because the paper is `#FCF8EF` and the ink is `#2E2823`, never white-on-black-on-grey; plain because there is exactly one accent and one font stack; unhurried because nothing on the page competes for a decision. The system is old-internet in its restraint, but it is not nostalgia cosplay: it uses `color-mix()`, `:has()`, view transitions and `aria-pressed` where those do the job better, and degrades to a working HTML form everywhere they are absent.

The confirmed anti-reference is the event-app card grid. No poster thumbnails, no gradient overlays, no "Trending near you", no urgency badges, no follower counts dressed as social proof. An event is five lines of text and a toggle, and that is the finished form — not a placeholder waiting for art direction.

**Key Characteristics:**
- One accent colour, one font stack, one stylesheet, no build step
- Zero breakpoints — layout responds through intrinsic sizing, not media queries
- No imagery of any kind; the product forbids it and the design is built for its absence
- Rules and type weight carry all hierarchy; there is not a single shadow
- Full light and dark parity, system-following with an explicit override
- Every interactive control works as a plain HTML form before JavaScript touches it

## Colors

A warm, low-contrast paper palette carrying exactly one accent, which does all the work of links, pressed states and the app icon at once.

### Primary

- **Burnt Ochre** (`#875800`): The daylight pigment, and the system's entire colour budget. It is every link, the filled background of a pressed Follow toggle, the focus ring, and the app icon's field. It is deliberately dark enough to sit on warm white as body-weight underlined text without straining.
- **Lamp Amber** (`#E9B653`): The same role at night — a filament, not a highlight. It replaces Burnt Ochre wholesale in the dark theme; the two are never on screen together.

### Neutral

- **Newsprint Warm White** (`#FCF8EF`): The page. Also the fill of every input and the resting fill of every button, so controls sit *in* the paper rather than on top of it. Becomes `#272320` in dark — a warm near-black that reads as stock, never as void.
- **Printer's Ink** (`#2E2823`): All body text, headings, the bold weekday in a day heading, and the 1px border of a resting button. Becomes `#F1EADF` in dark.
- **Pencil Grey** (`#6C6157`): Everything secondary — the uppercase section labels, the four meta lines under an event title, hints, follower counts, the `.muted` class, and input borders. Becomes `#B8AC9D` in dark.
- **Fold Line** (`#E2D9C9`): Hairlines only — the rule under a day heading, the `<hr>` above the footer, the line under the header. Never a fill, never text. Becomes `#4B433B` in dark.

### Tertiary

- **Alarm Red** (`#B5261A`): Validation errors, error summaries, and the border of an `aria-invalid` field. Nothing else is permitted to use it. Becomes `#FF9585` in dark, lightened to hold contrast on the dark stock.

### Named Rules

**The One Pigment Rule.** There is a single accent and it never gains a sibling. If something needs to stand out and Burnt Ochre is already spoken for on that screen, the answer is type weight, a rule, or space — not a second hue. Tags, venues, prices and counts are all Pencil Grey precisely because nothing about them earns colour.

**The Two Lights Rule.** Light and dark are two printings of the same document, not a theme and its inversion. Every token has a counterpart, the accent swaps role-for-role, and no rule, spacing value or type size changes between them. If a change looks right in only one theme, it is wrong in both.

## Typography

**Display Font:** none — the system is single-stack
**Body Font:** `ui-rounded` (with `Trebuchet MS`, `Ubuntu`, `Segoe UI`, `Roboto`, `Noto Sans`, `sans-serif`)
**Label/Mono Font:** none — tabular figures come from `font-variant-numeric`, not a second family

**Character:** One rounded humanist sans, sized up to 106.25% at the root (17px) so body text is comfortable on a phone held at arm's length without any zoom. `ui-rounded` gives San Francisco Rounded on Apple platforms — friendly and slightly soft, never geometric or corporate. Nothing is downloaded; there is no web font and no FOIT to manage.

**Know what the stack actually resolves to.** The rounded character is Apple-only. On Android — a large share of an Athens audience — `ui-rounded` is unsupported and Trebuchet MS, Ubuntu and Segoe UI are all absent, so the stack lands on **Roboto**, which is neither rounded nor humanist. This is an accepted consequence of refusing a web font, not an oversight. Do not treat "rounded and soft" as a property the design can rely on: it is a bonus where it appears. Anything that must hold everywhere has to come from size, weight, colour or rule — never from the letterforms.

### Hierarchy

- **Display** (bold, 1.75rem, 1.2, `-0.02em`): The page `<h1>` — an event title, "My feed", "Upcoming", "Account". One per page. The negative tracking keeps a long Greek venue name from sprawling.
- **Headline** (bold, 1.375rem, `-0.03em`): The masthead only — `dside events` in the header, rendered as `<h1>` on `/` and as a linked `<p class="brand">` everywhere else.
- **Title** (regular, 1rem): Day headings. Set in Pencil Grey with the weekday and any Today/Tomorrow prefix wrapped in `<b>` at Printer's Ink, so the day reads as ink and the date as annotation.
- **Body** (regular, 1rem, 1.5): Event titles, descriptions, all prose. Long-form text is capped at 68ch (`--reading`) even though the page runs to 110ch.
- **Label** (bold, 0.8125rem, `0.1em`, uppercase): Section labels — `MY FEED`, `UPCOMING`, `PAST`, `HIDDEN`, and the account page's subsections. Pencil Grey over a 2px Printer's Ink rule.
- **Meta** (regular, 0.9375rem): The four-to-five stacked lines beneath an event title (tags · venue, price · curator · N following), plus hints and inline `<small>` help.
- **Time** (bold, 1rem, `tabular-nums`): The start time in the left column of every programme row. Bold and tabular so the times form a straight edge down the page.

### Named Rules

**The Two Headings Rule.** A section label and a day heading must never be mistaken for each other. They differ on four axes at once — case (upper vs. mixed), size (0.8125rem vs. 1rem), colour (Pencil Grey vs. Pencil Grey with ink weekday) and rule weight (2px vs. 1px). This is what lets a single column read as *sections containing days* rather than one flat run of headings. Changing any one of the four is a system-level decision, not a tweak.

**The Tabular Margin Rule.** Times are bold, tabular-numeric, and pinned to a fixed 6ch column. The alignment is the feature: it is how a visitor scans an evening without reading.

## Layout

Every page is one centred column with a hard ceiling of 110ch (`--measure`) and 1rem of side padding, with prose and forms narrowing further to 68ch (`--reading`). There is no sidebar, no sticky chrome and no overlay anywhere in the system.

The home page is the one exception to single-column: `.columns` is a `grid` of `repeat(auto-fit, minmax(min(24rem, 100%), 1fr))` with a `2.5rem 3rem` gap. My feed and Upcoming sit side by side when there is room and stack — My feed first — when there is not, and the switch happens through intrinsic sizing alone.

A programme row is a three-track grid: `6ch` for the time, `1fr` for the title and its stacked meta lines, `auto` for the Follow toggle. Rows carry 1rem of bottom margin; day groups open with 2rem above the heading, tightening to 0.75rem when a day heading directly follows a section label.

Vertical rhythm runs on a 0.25rem base: 0.25 / 0.5 / 0.75 / 1 / 1.5 / 2 / 2.5 / 3rem. Block elements share a uniform `0 0 1rem` bottom margin; the footer clears at 3rem.

`overflow-wrap: anywhere` is set on `<body>` — a defensive choice for pasted URLs and long unbroken Greek compounds in a layout with no horizontal escape.

### Named Rules

**The No-Breakpoint Rule.** This stylesheet contains no width media queries and must not gain one. The only `@media` blocks permitted are `prefers-color-scheme`, `prefers-reduced-motion` and `hover: hover`. Responsive behaviour comes from `minmax()`, `min()`, `ch` units and intrinsic wrapping. If a layout needs a breakpoint, the layout is wrong for this system.

**The One Width Rule.** Every page is 110ch wide; only the text inside narrows. A visitor moving between the feed, an event and the account page never sees the column jump.

## Elevation & Depth

There is no `box-shadow` anywhere in this system, and no elevation scale. Depth is expressed entirely through rule weight, type weight and fill: a 2px Printer's Ink rule under a section label sits "above" a 1px Fold Line hairline under a day, and a pressed toggle flooded with Burnt Ochre sits above an unpressed one drawn as a hairline box. Surfaces do not stack, because the page is a document and documents have one plane.

### Named Rules

**The Flat-By-Default Rule.** Surfaces are flat at rest. A shadow may appear only as a response to state — hover, focus, an overlay — never as decoration and never at rest. No such state exists in the system today; the first one to need it must be added here before it is added to the stylesheet.

## Shapes

Near-zero radius throughout. Inputs and textareas are explicitly squared (`border-radius: 0`) so they read as ruled fields on a form rather than as app chrome; buttons carry the system's only radius, a 4px softening (`{rounded.sm}`) that is just enough to separate a control from a field at a glance. Fieldsets have their native border and padding stripped entirely.

Borders come in exactly three weights and each has one job: 1px Fold Line for hairlines and dividers, 1px Printer's Ink for a resting button's edge, 2px Printer's Ink for the rule under a section label. The app icon is the single element with real curvature (a 12/64 rounded square), and it lives outside the document plane.

### Named Rules

**The Squared Field Rule.** Inputs are square-cornered and buttons are 4px. That two-value contrast is the entire form language — do not round a field to match a button, and do not add a third radius.

## Components

### Buttons

Controls are honest and mechanical. A button shows its state rather than advertising itself; the press is an acknowledgement, not a celebration.

- **Shape:** Gently softened corners (4px), 1px solid Printer's Ink border, minimum height 2rem for a comfortable touch target
- **Default:** Newsprint Warm White fill with Printer's Ink text and border — a hairline box that barely interrupts the column. Padding `0.3rem 0.85rem`
- **Pressed** (`aria-pressed="true"`): Burnt Ochre floods background *and* border, text flips to Newsprint Warm White. Font weight does not change
- **Hover** (pointer devices only): Border and text take Burnt Ochre; the fill does not move. A pressed button darkens to `color-mix(in srgb, var(--link) 82%, var(--fg))`
- **Active:** `scale(0.97)`, applied instantly on press and eased on release
- **Focus:** 2px Burnt Ochre outline at 2px offset — the same treatment on every focusable element in the system

### Toggles (signature component)

The Follow control is the system's one piece of real interaction design and it is built around a single principle: **one change, not two.**

- The button reserves `min-width: 8.5em` and centres its label, so switching between "Follow" and "Following" never resizes or reflows the row
- A `::before` check-mark slot exists in both states at zero width and zero opacity, expanding to `1.15em` when pressed. The mark appears *with* the colour fill, as one gesture, and the content is `"✓" / ""` so screen readers get the empty alternative rather than a spoken glyph — `aria-pressed` already carries the state
- Font weight is explicitly held constant (`font-weight: inherit`) across states. Colour changes; geometry does not
- Hide sits beneath Follow as a `.linklike` button — a real `<button>` styled as an underlined Burnt Ochre link, so a destructive-feeling action never looks like a primary control

### Inputs / Fields

- **Style:** Full-width, square-cornered, 1px Pencil Grey border, Newsprint Warm White fill, `0.375rem 0.5rem` padding, 1rem bottom margin. `font: inherit` so no input ever ships browser defaults
- **Label:** Always a real `<label>`, block-level, 0.25rem above its field
- **Hint:** `.hint` in Meta type, pulled up with `-0.75rem` top margin so it hugs the field it belongs to rather than floating between two
- **Error:** `aria-invalid="true"` switches the border to Alarm Red; the message sits directly beneath in Alarm Red at Meta size, again pulled up to stay attached
- **Textarea:** Minimum height 10rem
- **Checkbox:** 1.125rem square, inline inside its label, aligned at `-0.15em`

### Navigation

- **Header:** Flex row, baseline-aligned, wrapping; masthead left, account nav right, 1px Fold Line rule beneath. The masthead is an `<h1>` on `/` and a linked `.brand` paragraph everywhere else
- **Filters:** Plain text links separated by a middle dot, on a 1.75rem line-height so wrapped rows stay legible
- **Current state:** `[aria-current]` renders bold with the underline removed — the only place in the system where a link loses its underline
- **Mobile:** No treatment. The header wraps and the filters wrap; there is no menu, no drawer and no hamburger

### Named Rules

**The Working Form Rule.** Every interactive control in this system is a real HTML form that works with JavaScript disabled. `app.js` only intercepts, swaps the response in, and re-enables the native submit on any failure. A control that cannot be expressed as a form does not belong here.

## Do's and Don'ts

### Do:

- **Do** spend the colour budget on Burnt Ochre and nothing else. Links, pressed toggles and focus rings; everything else is Ink, Pencil Grey or Fold Line.
- **Do** keep the two heading kinds distinct on all four axes — case, size, colour and rule weight. It is what makes a column of sections-containing-days readable.
- **Do** hold geometry constant across interaction states. Reserve the space a state will need (the 8.5em toggle, the zero-width check-mark slot) so nothing reflows.
- **Do** set times bold and `tabular-nums` in the 6ch margin. The straight edge down the page is the scanning affordance.
- **Do** write every new control as a plain HTML form first, then enhance it.
- **Do** give any new colour, spacing step or type role a counterpart in both themes before shipping it.
- **Do** cap long-form text at 68ch even on a 110ch page.

### Don't:

- **Don't** add a width media query. Use `minmax()`, `min()`, `ch` and intrinsic wrapping.
- **Don't** add a shadow at rest. Depth is rule weight and fill.
- **Don't** introduce imagery, thumbnails, avatars or icon sets into the document. The product is text-only by design; the layout has no slot for a picture and must not grow one. (The app icon and PWA assets are the sole exception and live outside the page.)
- **Don't** add a second accent hue, a status-colour palette, or a tag-colour scheme. Tags are Burnt Ochre links because they are links, not because they are categories.
- **Don't** round an input to match a button, or add a third radius value.
- **Don't** animate anything on page load, or animate geometry to acknowledge a press. Motion is `--t: 150ms` / `--t-out: 120ms` on one curve (`cubic-bezier(.2, 0, 0, 1)`), colour-only for state, and fully disabled under `prefers-reduced-motion`.
- **Don't** attach hover treatments outside `@media (hover: hover)`. Touch devices must never inherit a sticky hover state.
- **Don't** use bold as decoration. Bold means one of exactly three things here: a page or section heading, a time, or a current nav item.
