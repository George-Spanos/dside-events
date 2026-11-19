#!/usr/bin/env python3
"""check-venues.py — weekly watch for new events at followed venues.

Fetches the more.com listings for the venues dside events follows (currently
Theatre of the NO, Half Note Jazz Club and Spiti Art Bar) and compares every
title with what is already live on https://events.dside.studio plus the
committed seed/*.tsv files. Writes a plain-text report; non-zero exit means
there is something new to curate.

    run-parts style, called by the dside-venue-watch systemd user timer.
"""

import datetime as dt
import json
import pathlib
import subprocess
import sys
import urllib.request

REPO = pathlib.Path.home() / "dside-events"
REPORT = REPO / "seed" / "venue-watch-report.txt"
PROD_URL = "https://events.dside.studio"

# more.com venue-name filters (substring match, what seed/fetch.py --venue takes).
VENUES = ["Theatre of NO", "Half Note", "Spiti Art Bar"]
UA = ("Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 "
      "(KHTML, like Gecko) Chrome/128.0 Safari/537.36")


def known_titles():
    """Titles live on production plus titles in every committed seed file."""
    titles = set()
    html = urllib.request.urlopen(
        urllib.request.Request(f"{PROD_URL}/upcoming", headers={"User-Agent": UA}),
        timeout=60).read().decode()
    for href in html.split('href="/e/')[1:]:
        title = href.split(">", 1)[1].split("<", 1)[0].strip()
        if title:
            titles.add(title)
    for tsv in sorted((REPO / "seed").glob("*.tsv")):
        for line in tsv.read_text().splitlines():
            if line and not line.startswith("#") and not line.startswith("title\t"):
                titles.add(line.split("\t", 1)[0].strip())
    return titles


def norm(s):
    # more.com prints ALL CAPS for some venues; compare case-insensitively
    # and ignore the punctuation/accents that vary between sources.
    return " ".join(s.casefold().split())


def fetch_venue(venue):
    """Run the repo's own fetcher filtered to one venue; returns draft rows."""
    out = subprocess.run(
        ["python3", "seed/fetch.py", "music", "--json",
         "--from", dt.date.today().isoformat(),
         "--to", (dt.date.today() + dt.timedelta(days=70)).isoformat(),
         "--venue", venue],
        cwd=REPO, capture_output=True, text=True, timeout=1200)
    if out.returncode != 0:
        raise RuntimeError(f"fetch.py --venue {venue} failed:\n{out.stderr[-2000:]}")
    rows = json.loads(out.stdout)
    return rows


def main():
    titles = known_titles()
    known = {norm(t) for t in titles}
    lines = [f"venue watch {dt.datetime.now(dt.UTC):%Y-%m-%d %H:%M} UTC",
             f"{len(known)} titles known (prod + seed/*.tsv)"]
    new = 0
    for venue in VENUES:
        try:
            rows = fetch_venue(venue)
        except Exception as e:  # report, don't die: other venues still checked
            lines.append(f"\n== {venue}: FETCH FAILED: {e}")
            continue
        fresh = [r for r in rows if norm(r.get("title", "")) not in known]
        lines.append(f"\n== {venue}: {len(rows)} listing(s), {len(fresh)} not yet on the site")
        for r in fresh:
            new += 1
            shows = r.get("shows") or []
            when = ", ".join(f"{s.get('day', '?')} {s.get('time', '')}".strip()
                             for s in shows[:6]) or "?"
            lines.append(f"  NEW  {when} — {r.get('title', '?')} ({r.get('venue', venue)})")
            if r.get("url"):
                lines.append(f"       {r['url']}")
    lines.append(f"\ntotal new: {new}")
    REPORT.write_text("\n".join(lines) + "\n")
    print("\n".join(lines))
    sys.exit(1 if new else 0)


if __name__ == "__main__":
    main()
