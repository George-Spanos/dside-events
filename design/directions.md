# dside events — three warm directions

Brief (founder): "I don't like the font and colours. We are an app about going out and having fun; pure black is not welcoming and doesn't express the context. Make it warmer. Simplicity of actions stays; we work on UI, colours, typography."

What stays in every direction (PROJECT.md is firm): plain lists, underlined links, filters as text links, 1px rules, no cards / shadows / animations / hero / images / icons, mobile-first, old-internet feel. One accent (`--link`) used only for links, focus rings and the pressed-button inversion. `.events li` grid, `.columns`, `.day`, `.meta`, `body.wide` untouched. No build step, no npm, no runtime font requests.

Contrast is WCAG 2.x relative luminance, computed by `contrast.py` in this folder. Targets: body text ≥ 7:1, muted ≥ 4.5:1, link ≥ 4.5:1, error ≥ 4.5:1. All six palettes pass; "warm" here never means low contrast.

Greek coverage: every family in the stacks below ships Greek (Palatino Linotype, Palatino [macOS/iOS], Book Antiqua, Georgia, Noto Serif, DejaVu Serif, Segoe UI, Lucida Grande, Lucida Sans Unicode, Roboto, Noto Sans, Trebuchet MS, Ubuntu, SF via `ui-rounded`). Verified locally with `fc-list :lang=el`: P052 (URW Palatino), Ubuntu, DejaVu, Noto, Liberation. Deliberately NOT used because they are Latin-only and would make Greek titles fall back to a different face mid-list: Iowan Old Style, Hoefler Text, Baskerville, Charter, Optima, Avenir Next, Cantarell (partial Greek).

---

## Direction 1 — Paper and ink

**Rationale.** The programme you pick up at the door of Cine Paris or Θέατρο Τέχνης is a folded sheet of warm uncoated paper, set in one old-style serif, dates in italics, with a red stamp for the price — that is the printed vernacular of Athenian going-out, not a design trend. It reads as *the listings*, which is exactly what the product is.

**Why this is not the "cream + high-contrast serif + terracotta" default.** The serif is a low-contrast old-style (Palatino-class), not a Didone/Playfair; the paper is a touch greyer and yellower than the AI-default #F4F1EA (uncoated stock, not "cream"); the accent is stamp red (#8E2F1A), not terracotta orange; and the whole page including body text is the one serif — the default look keeps a sans body. The italic section labels and italic dates carry the programme feel with zero decoration.

| token | light | ratio vs bg | dark | ratio vs bg |
|---|---|---|---|---|
| --bg | #F3ECDF | — | #1F1915 | — |
| --fg | #2B2119 | 13.40:1 | #EFE5D6 | 13.95:1 |
| --muted | #6A594B | 5.69:1 | #B6A794 | 7.40:1 |
| --link | #8E2F1A | 6.95:1 | #F0A283 | 8.44:1 |
| --error | #B8141F | 5.66:1 | #FF8F86 | 7.88:1 |
| --rule | #D8CDBB | 1.34:1 (hairline) | #3F352D | 1.46:1 |
| pressed button: bg text on link | | 6.95:1 | | 8.44:1 |

Fonts
- Display and body (same face): `"Palatino Linotype", Palatino, "Book Antiqua", P052, Georgia, "Noto Serif", "DejaVu Serif", serif` — Windows: Palatino Linotype (Greek yes) → Book Antiqua → Georgia; macOS/iOS: Palatino (Greek yes) → Georgia; Android: Noto Serif; Linux: P052/DejaVu Serif.
- Time (mono role): same face with `font-variant-numeric: lining-nums tabular-nums` — no monospace, a programme never mixes in a typewriter face.
- Optional self-hosted face if the founder wants the same look on every phone: **Alegreya** (SIL OFL 1.1, Greek + Latin), Regular + Bold, woff2 subset Latin+Greek ≈ 60–70 KB each (two files, ~130 KB). The preview does NOT load it; it renders the system stack.

CSS that differs from the current stylesheet (`css/d1-paper-and-ink.css`):

```css
:root {
  --bg: #F3ECDF; --fg: #2B2119; --muted: #6A594B; --link: #8E2F1A; --error: #B8141F; --rule: #D8CDBB;
  --serif: "Palatino Linotype", Palatino, "Book Antiqua", P052, Georgia, "Noto Serif", "DejaVu Serif", serif;
  font: 106.25%/1.55 var(--serif);
}
@media (prefers-color-scheme: dark) {
  :root { --bg: #1F1915; --fg: #EFE5D6; --muted: #B6A794; --link: #F0A283; --error: #FF8F86; --rule: #3F352D; }
}
h1 { font-size: 1.75rem; font-weight: normal; line-height: 1.2; }
header h1, header .brand { font-size: 1.375rem; font-weight: bold; letter-spacing: -.02em; }
h2 { font-size: 1.0625rem; font-weight: normal; font-style: italic; letter-spacing: .01em; }
.day { font-style: italic; }
.day b { font-style: normal; }
.events time { font-variant-numeric: lining-nums tabular-nums; }
a { text-decoration-thickness: 1px; text-underline-offset: .18em; text-decoration-color: color-mix(in srgb, var(--link) 55%, transparent); }
button { border-color: var(--muted); padding: .3rem .8rem; }
button[aria-pressed="true"] { background: var(--link); border-color: var(--link); color: var(--bg); }
footer small { font-style: italic; }
```

**In one sentence:** a folded cinema programme on warm paper — espresso serif, italic dates, stamp-red links, and a pressed "Interested" that looks like a red rubber stamp.

---

## Direction 2 — Evening out

**Rationale.** Going out in Athens starts on sun-warmed limestone and ends under a sky that is aubergine, not black, lit amber by sodium street lamps; the dark mode is literally that colour pair. A humanist sans keeps it a listings page, not a poster — the warmth is entirely in colour and measure.

**Why it avoids the AI defaults entirely.** No serif, no cream, no terracotta. The dark mode is neither near-black nor "dark + acid accent": it is a tinted aubergine with a warm amber link that stays at 9.3:1. The body face is Lucida Grande / Segoe UI — the humanist UI faces of the 2000s web, which is also where the founder's "old internet feel" comes from.

| token | light | ratio vs bg | dark | ratio vs bg |
|---|---|---|---|---|
| --bg | #EEE7DA | — | #1B1727 | — |
| --fg | #231B29 | 13.55:1 | #ECE3E7 | 13.94:1 |
| --muted | #655A6D | 5.28:1 | #B3A7BA | 7.64:1 |
| --link | #6E2A5F | 7.91:1 | #F0B24F | 9.33:1 |
| --error | #AE1A31 | 5.71:1 | #FF9494 | 8.27:1 |
| --rule | #D5CBBB | 1.30:1 (hairline) | #3A324B | 1.45:1 |
| pressed button: bg text on link | | 7.91:1 | | 9.33:1 |

Fonts
- Display and body: `"Lucida Grande", "Segoe UI", Roboto, "Noto Sans", "Lucida Sans Unicode", sans-serif` — macOS: Lucida Grande (Greek yes, still shipped); Windows: Segoe UI (Greek yes); Android: Roboto; Linux: Noto Sans. Lucida is wide, so the base stays 16px.
- Time: same face, `tabular-nums`.
- No self-hosted font needed. If a single face everywhere is wanted later: **Commissioner** (OFL, Greek yes, variable woff2 ≈ 190 KB) or **Fira Sans** (OFL, Greek yes, ≈ 80 KB per weight subset).

CSS that differs (`css/d2-evening-out.css`):

```css
:root {
  --bg: #EEE7DA; --fg: #231B29; --muted: #655A6D; --link: #6E2A5F; --error: #AE1A31; --rule: #D5CBBB;
  --sans: "Lucida Grande", "Segoe UI", Roboto, "Noto Sans", "Lucida Sans Unicode", sans-serif;
  font: 100%/1.55 var(--sans);
}
@media (prefers-color-scheme: dark) {
  :root { --bg: #1B1727; --fg: #ECE3E7; --muted: #B3A7BA; --link: #F0B24F; --error: #FF9494; --rule: #3A324B; }
}
h1 { font-size: 1.625rem; font-weight: bold; letter-spacing: -.015em; line-height: 1.2; }
header h1, header .brand { font-size: 1.25rem; letter-spacing: -.02em; }
h2 { letter-spacing: .01em; }
.day { color: var(--muted); }
.day b { color: var(--fg); }
.events time { font-variant-numeric: tabular-nums; }
.meta { line-height: 1.45; }
a { text-underline-offset: .18em; text-decoration-color: color-mix(in srgb, var(--link) 55%, transparent); }
button { border-color: var(--muted); padding: .3rem .8rem; }
button[aria-pressed="true"] { background: var(--link); border-color: var(--link); color: var(--bg); }
```

**In one sentence:** by day a sand page with plum links; after sunset an aubergine page with amber links — the same list, the colour of the city at 9 pm.

---

## Direction 3 — Kiosk

**Rationale.** The περίπτερο is the friendliest object on an Athenian street: warm white, a mustard awning, hand-written prices, everything at arm's reach. Bright, plain, unfussy — the direction for "having fun" that never tips into decoration.

**Why it avoids the defaults.** No serif anywhere; the light background is a brighter warm white (#FCF8EF), not cream; the ink is warm grey rather than black; the single accent is ochre-mustard, a colour the other two looks (and most AI output) never touch. Rounded humanist sans via `ui-rounded` (SF Rounded on Apple) and Trebuchet MS elsewhere — Trebuchet is a 1996 face with full Greek and a friendly quirk, the most "old internet" choice of the three.

| token | light | ratio vs bg | dark | ratio vs bg |
|---|---|---|---|---|
| --bg | #FCF8EF | — | #272320 | — |
| --fg | #2E2823 | 13.72:1 | #F1EADF | 13.04:1 |
| --muted | #6C6157 | 5.68:1 | #B8AC9D | 6.99:1 |
| --link | #875800 | 5.78:1 | #E9B653 | 8.37:1 |
| --error | #B5261A | 6.08:1 | #FF9585 | 7.34:1 |
| --rule | #E2D9C9 | 1.32:1 (hairline) | #4B433B | 1.61:1 |
| pressed button: bg text on link | | 5.78:1 | | 8.37:1 |

Fonts
- Display and body: `ui-rounded, "Trebuchet MS", Ubuntu, "Segoe UI", Roboto, "Noto Sans", sans-serif` — Apple: SF Rounded (Greek yes); Windows/macOS without ui-rounded: Trebuchet MS (Greek yes); Ubuntu desktop: Ubuntu; Android: Roboto.
- Time: same face, bold, `tabular-nums` (the price tag).
- Optional self-hosted face for one look everywhere: **Ubuntu** (Ubuntu Font Licence 1.0, Greek + Latin + Cyrillic), Regular + Bold, woff2 subset Latin+Greek ≈ 45–55 KB each (~100 KB total). Alternative with the same licence class: **Andika** (OFL, Greek yes, ≈ 90 KB per weight). The preview renders the system stack (Ubuntu only if the OS has it).

CSS that differs (`css/d3-kiosk.css`):

```css
:root {
  --bg: #FCF8EF; --fg: #2E2823; --muted: #6C6157; --link: #875800; --error: #B5261A; --rule: #E2D9C9;
  --sans: ui-rounded, "Trebuchet MS", Ubuntu, "Segoe UI", Roboto, "Noto Sans", sans-serif;
  font: 106.25%/1.5 var(--sans);
}
@media (prefers-color-scheme: dark) {
  :root { --bg: #272320; --fg: #F1EADF; --muted: #B8AC9D; --link: #E9B653; --error: #FF9585; --rule: #4B433B; }
}
h1 { font-size: 1.75rem; letter-spacing: -.02em; line-height: 1.2; }
header h1, header .brand { font-size: 1.375rem; letter-spacing: -.03em; }
h2 { font-size: .9375rem; text-transform: uppercase; letter-spacing: .08em; }
.events time { font-variant-numeric: tabular-nums; font-weight: bold; }
a { text-underline-offset: .18em; text-decoration-color: color-mix(in srgb, var(--link) 55%, transparent); }
button { border-radius: 4px; padding: .3rem .85rem; }
button[aria-pressed="true"] { background: var(--link); border-color: var(--link); color: var(--bg); }
```

**In one sentence:** a bright warm-white list with bold times, mustard links and a mustard "Interested" — a kiosk shelf, not a poster.

---

## Smallest typographic changes that add warmth regardless of palette

These five lines do most of the work even if the palette stays as it is; they are already folded into the three directions above.

1. `font-size: 106.25%` (17px) and `line-height: 1.55` — the current 16px/1.5 is the browser default and reads as "unstyled" rather than "simple". (Direction 2 keeps 16px because Lucida is wide.)
2. Wordmark `dside events`: keep lowercase, `letter-spacing: -.02em`, one step larger (1.375rem) than the nav. Type only, no logo.
3. Weekday in the display face and weight, date lighter (italic in D1, muted in D2) — the `.day` line becomes a heading you scan, not a string you read.
4. Underlines: `text-underline-offset: .18em` and `text-decoration-color: color-mix(in srgb, var(--link) 55%, transparent)` — links stay underlined (firm) but the underline stops fighting Greek descenders and accents. Safe fallback: browsers without `color-mix` keep the solid underline.
5. Buttons: border in `--muted` instead of `--fg`, pressed state inverted with the accent instead of pure fg/bg — the single warm spot on the page, tied to the one action that matters.

---

## Recommendation

**Direction 2 — Evening out**, with Direction 1's italic `.day` date line borrowed if the founder wants a touch more character.

Why: it answers the actual complaint most directly. The founder's objection was to *pure black* and to the *font*; Direction 2 removes black from both modes (sand / aubergine), changes the face to a humanist sans that still reads as a plain web page, and its dark mode is the only one of the three that is *about* going out — the amber-on-aubergine of an Athens night. It needs no self-hosted font, no serif (so it stays out of the AI cream-and-serif look), and keeps the current CSS almost byte-for-byte: the diff is 20 lines. Direction 1 is the most characterful and the biggest risk (an all-serif screen page divides people; Android renders it in Noto Serif, which is drier); Direction 3 is the friendliest but ochre on warm white sits closest to the 4.5:1 floor (5.78:1), leaving the least room for later tweaks.

Suggested next step if approved: apply `css/d2-evening-out.css` on top of `static/style.css`, update the two `theme-color` metas to `#EEE7DA` / `#1B1727`, and check the PWA icon against the sand background.

## Files

- `preview.html` — the real home and event HTML, three times, each in its own `.dir-N` scope with a light/dark toggle; renders from `file://`, no external requests.
- `css/d1-paper-and-ink.css`, `css/d2-evening-out.css`, `css/d3-kiosk.css` — the diffs in product form (`:root` + media query), ready to paste over `static/style.css`.
- `contrast.py` — the ratio script; `build.py` — assembles the preview from `raw/*.html` and the CSS above.
- `shots/*.png` — headless-Chromium renders of the preview (wide light/dark, mobile light, plus per-direction shots). Note this Linux box has no Palatino / Lucida Grande / Trebuchet, so the shots show the Linux fallbacks (P052, Noto Sans / Liberation Sans, Ubuntu); open `preview.html` on a Mac or Windows machine to see the intended first-choice faces.
