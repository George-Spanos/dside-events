---
name: find-events
description: Find concerts, theatre plays and films in Athens and publish them on dside events. Use when asked to fill the app, add events for a month or a category, or refresh the listings. Runs seed/fetch.py for candidates, curates them, writes a seed TSV in the house style, validates with seed/check.py and publishes with make prod-seed.
---

# find-events

Fill the site with real, upcoming events in Athens: concerts (`concert`),
theatre (`theater`) and films (`film`). Everything goes through the seed
pipeline, so nothing bypasses the form's validation or the one-post-per-event
rule:

    seed/fetch.py  →  seed/drafts/*.tsv (raw)  →  you curate  →  seed/<name>.tsv
                   →  seed/check.py  →  make prod-seed FILE=seed/<name>.tsv

## 1. Decide the window and what is already up

- `git pull --ff-only` first: production runs `origin/main`, and the form
  fields (date format, for one) must match what `seed/seed.sh` sends.

- Window: from tomorrow to about six weeks out unless told otherwise. Events
  further out are fine when tickets are on sale (autumn seasons of Megaron,
  Onassis Stegi, Gazarte, Half Note open early).
- What is live: `curl -s https://events.dside.studio/upcoming | grep -o 'href="/e/[^"]*"[^<]*'`
  and the existing `seed/*.tsv`. Never repost a title that is there.

## 2. Fetch candidates

    make fetch CATEGORY=theater FROM=2026-09-10 TO=2026-10-31
    make fetch CATEGORY=music   FROM=2026-09-10 TO=2026-10-31 ARGS="--min-popularity 10"
    make fetch CATEGORY=cinema  FROM=2026-09-10 TO=2026-09-30 ARGS="--venue Κήπος"

writes `seed/drafts/<category>-<from>-<to>.tsv` (gitignored). For the whole
picture use `python3 seed/fetch.py <category> --from .. --to .. --json`: every
date, venue, price and sold-out flag per event. The source is more.com
(region Αττική); one request per second, cached for six hours under
`$TMPDIR/dside-fetch-cache`. A full category takes 5–10 minutes: run it in
the background and do the research meanwhile.

Read the `#` line above each draft row: more.com popularity, number of
dates, how many are sold out, and the promoter's own title.

## 3. Curate

Pick what an adult in Athens who likes music, theatre and cinema would want
to know about. Aim for 10–20 per category per month, spread over the weeks.

Take:
- concerts of any genre with a real artist or programme (jazz, classical,
  Greek song, rock, electronic, world);
- theatre: drama, comedy, ancient drama, dance-theatre, new writing, festival
  productions, one-off performances and premieres;
- films at open-air and independent cinemas (Cine Kipos, Zefyros, Riviera,
  Thision, Cine Paris, Aigli, Danaos, Asty, Trianon, Astor), festival
  screenings, restored classics, Greek releases.

Skip:
- children's shows, school events, stand-up, tribute nights, magic shows,
  corporate or tourist shows, multiplex blockbuster screenings;
- tours listed as "Πολλαπλοί χώροι" unless there is a date in Athens or
  Piraeus; venues outside the city (Lavrio, Marathon, Koropi count as Αττική
  on more.com but not as Athens);
- anything already on the site or in a `seed/*.tsv`;
- anything whose date, venue or start time you could not confirm on a page
  you actually opened.

## 4. Research and write, one subagent per batch

For each pick open the more.com page and one more source: the venue's own
page, the company's site, Athinorama (`athinorama.gr/music/gig/…`,
`/cinema/movie/…`), elculture, CultureNow, the National Theatre, Megaron.
Take cast, director, programme, duration, running dates and reduced prices
from there. Fan the picks out to subagents in batches of 5–8 and have each
return finished TSV rows; then merge, check and publish yourself.

### House style (look at `seed/jazz-athens-2026-09.tsv` and `seed/theater-athens-2026-09.tsv`)

- **Title**: what the poster says, cleaned. Greek titles keep the Greek with
  the English in parentheses: `Ο Γλάρος (The Seagull)`. Concerts: `Artist`
  or `Artist: programme`. No ALL CAPS, no "2ος χρόνος", no promoter fluff.
  ≤ 120 characters.
- **Date, time**: `YYYY-MM-DD`, `HH:MM`, Athens time, from the ticket page.
  Doors vs start: use the start; mention doors in the description.
- **Venue**: `Place, neighbourhood`, in Latin script, ≤ 120 characters:
  `Gazarte Main Stage, Gazi`, `Theatro Vrachon Melina Mercouri, Vyronas`,
  `Cine Kipos (Σινέ Κήπος), Moschato`. Greek in parentheses when the Latin
  name is not the one on the door.
- **Price**: `15€`, `from 15€`, `12€ – 30€`, `free`, `not announced`.
  ≤ 60 characters; the reductions go in the description.
- **Tags**: one of `concert`, `theater`, `film`, `exhibition`; comma-separated
  if two apply (a concert film is `film,concert`).
- **Description**: English, 2–4 sentences, plain and factual, no marketing
  adjectives. Who (director, company, artists, key cast), what (the piece,
  one line of what it is about or what will be played), then the practical
  line: duration, run dates, language and subtitles, reduced prices, doors,
  "start time to be confirmed by the venue" when the source does not print
  one. ≤ 4000 characters; no line breaks.
- **Links**: up to three. Label `Event page (Source)`, `Tickets (more.com)`,
  `Programme (Venue)`; URLs full `https://`, opened by you, not guessed.
  Prefer the venue or company page first and the ticket page second.
- **Repeats**: a weekly pattern with one start time is one row with `until`
  and `weekdays` (`fri,sat`). A different time on Sundays is a second row.
  Irregular dates are one row per date. The draft already collapses what it
  can.
- **Doubt**: put a `#` comment line above the row saying what is unconfirmed
  and where you looked, exactly as the existing files do.

Write the rows to `seed/<category>-athens-<YYYY-MM>.tsv` with the header
line from the existing files. Tab-separated, one event per line, empty
fields left empty. Never edit a file that has already been published; add a
new one.

## 5. Check, publish, commit

    make seed-check FILE=seed/theater-athens-2026-10.tsv   # rules, links, duplicates on prod
    make run                                               # optional: local server
    make seed FILE=seed/theater-athens-2026-10.tsv         # optional: publish locally, look at it
    make prod-seed FILE=seed/theater-athens-2026-10.tsv    # production, as george-spanos

`make seed-check` must end with `0 errors`; read every warning. Rows the
server skips as duplicates are reported, not failed. `make prod-seed` without
`FILE=` publishes every `seed/*.tsv`, which is safe (existing events are
skipped) but slow. Publishing to production is public and visible to every
visitor: unless the user asked for the events to go up in this session,
stop after `make seed-check` and show them the file first.

Commit the seed file afterwards; `seed/drafts/` is not committed.
