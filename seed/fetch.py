#!/usr/bin/env python3
"""fetch.py — pull candidate events from more.com into a draft seed TSV.

more.com lists nearly every ticketed concert, play and screening in Greece
and marks each one up with schema.org microdata, so the listing page gives
the title, first and last date, venue and region, and the event page gives
every date and time, every venue, the price list and a short blurb. This
script turns that into the TSV that seed/seed.sh publishes:

    seed/fetch.py theater --from 2026-09-10 --to 2026-10-31 > seed/drafts/theater.tsv
    seed/fetch.py music   --from 2026-09-10 --to 2026-10-31 --min-popularity 20
    seed/fetch.py cinema  --from 2026-09-10 --to 2026-09-30 --venue Κήπος
    seed/fetch.py theater --from ... --to ... --json      # everything, for curation

The output is a DRAFT: descriptions are the Greek blurb from more.com,
titles are as the promoter typed them, and every Attica venue of a film is
its own line. Someone (or the find-events skill) still has to pick what
belongs on the site, rewrite the description in the house style and check
the links. seed/check.py validates the result before seed.sh publishes it.

Standard library only; be polite to more.com (one request per second,
cached on disk). Each `#` line above an event carries the source data that
helps curation: popularity, how many dates, how many are sold out.
"""

import argparse
import datetime as dt
import html
import json
import os
import re
import sys
import time
import urllib.error
import urllib.request

BASE = "https://www.more.com"
LISTING = {"music": "/gr-el/tickets/music/", "theater": "/gr-el/tickets/theater/", "cinema": "/gr-el/tickets/cinema/"}
TAG = {"music": "concert", "theater": "theater", "cinema": "film"}
# A browser UA: more.com sits behind a bot gate (queue-it) that stalls
# anything else, and answers a browser with a 302 whose body already carries
# the page, so redirects are not followed.
UA = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0 Safari/537.36"
WEEKDAYS = ["mon", "tue", "wed", "thu", "fri", "sat", "sun"]  # date.weekday(): Monday is 0
COLUMNS = ["title", "date", "time", "venue", "price", "tags", "description",
           "label1", "url1", "label2", "url2", "label3", "url3", "until", "weekdays"]


def log(msg):
    print(msg, file=sys.stderr, flush=True)


class _NoRedirect(urllib.request.HTTPRedirectHandler):
    """Hand a 3xx back as a response instead of following it."""

    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None

    def http_error_302(self, req, fp, code, msg, headers):
        return fp

    http_error_301 = http_error_303 = http_error_307 = http_error_308 = http_error_302


class Fetcher:
    """GET with a disk cache and a pause between live requests."""

    def __init__(self, cache_dir, pause=1.0):
        self.cache_dir = cache_dir
        self.pause = pause
        self.last = 0.0
        self.opener = urllib.request.build_opener(_NoRedirect, urllib.request.HTTPCookieProcessor())
        os.makedirs(cache_dir, exist_ok=True)

    def get(self, url):
        key = re.sub(r"[^A-Za-z0-9._-]+", "_", url)[-180:]
        path = os.path.join(self.cache_dir, key)
        if os.path.exists(path) and time.time() - os.path.getmtime(path) < 6 * 3600:
            with open(path, encoding="utf-8") as f:
                return f.read()
        wait = self.pause - (time.time() - self.last)
        if wait > 0:
            time.sleep(wait)
        req = urllib.request.Request(url, headers={"User-Agent": UA, "Accept-Language": "el,en"})
        for attempt in range(3):
            try:
                with self.opener.open(req, timeout=45) as r:
                    body = r.read().decode("utf-8", "replace")
                if 300 <= r.status < 400 and "<article" not in body and "bookingPanel" not in body:
                    raise urllib.error.URLError(f"redirected to {r.headers.get('Location')} without content")
                break
            except (urllib.error.URLError, TimeoutError) as e:
                if attempt == 2:
                    raise
                log(f"retry {url}: {e}")
                time.sleep(3)
        self.last = time.time()
        with open(path, "w", encoding="utf-8") as f:
            f.write(body)
        return body


def parse_listing(page):
    """One dict per <article> of a more.com category page."""
    out = []
    for art in re.findall(r"<article id=\"Element\"(.*?)</article>", page, re.S):
        def meta(prop):
            m = re.search(r'itemprop="%s" content="([^"]*)"' % prop, art)
            return html.unescape(m.group(1)) if m else ""

        name = re.search(r'itemprop="name">([^<]*)<', art)
        venue = re.search(r'id="PlayVenue" itemprop="name">([^<]*)<', art)
        code = re.search(r'data-code="([^"]*)"', art)
        pop = re.search(r'data-popularity="(\d+)"', art)
        out.append({
            "url": BASE + meta("url"),
            "name": html.unescape(name.group(1)).strip() if name else "",
            "start": meta("startDate")[:10],
            "end": meta("endDate")[:10],
            "venue": html.unescape(venue.group(1)).strip() if venue else "",
            "locality": meta("addressLocality"),
            "region": meta("addressRegion"),
            "blurb": meta("description"),
            "code": code.group(1) if code else "",
            "popularity": int(pop.group(1)) if pop else 0,
        })
    # The same event appears once per date it is featured on; keep one.
    seen, uniq = set(), []
    for a in out:
        if a["url"] not in seen:
            seen.add(a["url"])
            uniq.append(a)
    return uniq


def parse_event_page(page):
    """The booking data of an event page: dates, venues, prices, plus the
    region of each date from the Buy buttons and the schema.org name."""
    i = page.find("bookingPanel.data = ")
    if i < 0:
        return None
    data, _ = json.JSONDecoder().raw_decode(page, i + len("bookingPanel.data = "))
    areas = {}
    for m in re.finditer(r'evId="(\d+)"[^>]*venueArea="([^"]*)"', page):
        areas[int(m.group(1))] = html.unescape(m.group(2))
    name = ""
    for block in re.findall(r'<script type="application/ld\+json">(.*?)</script>', page, re.S):
        try:
            ld = json.loads(block)
        except ValueError:
            continue
        if isinstance(ld, dict) and ld.get("@type") == "Event":
            name = html.unescape(ld.get("name", "")).strip()
    og = re.search(r'<meta property="og:description" content="([^"]*)"', page)
    return {
        "data": data,
        "areas": areas,
        "name": name,
        "blurb": html.unescape(og.group(1)).strip() if og else "",
    }


def fmt_price(p):
    s = ("%.2f" % p).rstrip("0").rstrip(".")
    return s.replace(".", ",") + "€"


def price_text(prices):
    """`15€` or `15€ – 30€` from the regular prices when they are labelled,
    else from every price in the list."""
    regular = [p for name, p in prices if re.search(r"κανονικ|regular|general", name, re.I)]
    use = regular or [p for _, p in prices]
    use = [p for p in use if p > 0]
    if not use:
        return ""
    lo, hi = min(use), max(use)
    return fmt_price(lo) if lo == hi else f"{fmt_price(lo)} – {fmt_price(hi)}"


def build_event(listing, page, args, tag):
    """Collapse an event page into the candidate record the TSV/JSON is
    written from: per venue, the dates in the window and their times."""
    ev = parse_event_page(page)
    if ev is None:
        return None
    data = ev["data"]
    venues = {v["id"]: v for v in data.get("venues", [])}
    pricelists = {}
    for pl in data.get("pricelists", []):
        pricelists[pl["id"]] = [(d.get("discount-name", ""), float(d.get("price") or 0)) for d in pl.get("discounts", [])]
    lo, hi = args.date_from, args.date_to
    shows = []
    for e in data.get("events", []):
        start = e.get("event-date", "")
        day, clock = start[:10], start[11:16]
        if not day or day < lo or day > hi:
            continue
        area = ev["areas"].get(e.get("eventId"))
        if args.region and area and area != args.region:
            continue
        v = venues.get(e.get("venueId"), {})
        vname = v.get("venue-name", listing["venue"])
        if args.venue and args.venue.lower() not in vname.lower():
            continue
        shows.append({
            "day": day, "time": clock, "venue": vname, "city": v.get("venue-city", ""),
            "address": v.get("venue-address", ""), "sold_out": bool(e.get("isSoldout")),
            "prices": pricelists.get(e.get("pricelistId"), []),
        })
    if not shows:
        return None
    shows.sort(key=lambda s: (s["venue"], s["day"], s["time"]))
    prices = [p for s in shows for p in s["prices"]]
    return {
        "title": ev["name"] or listing["name"],
        "listing_name": listing["name"],
        "tag": tag,
        "url": listing["url"],
        "code": listing["code"],
        "popularity": listing["popularity"],
        "description": ev["blurb"] or listing["blurb"],
        "price": price_text(prices),
        "prices": sorted({(n, p) for n, p in prices}, key=lambda x: x[1]),
        "shows": shows,
    }


def series(dates):
    """If dates (sorted, unique) are exactly every ticked weekday from the
    first to the last, return (until, weekdays); else None."""
    if len(dates) < 2:
        return None
    ticked = sorted({d.weekday() for d in dates})
    d, want = dates[0], set(dates)
    covered = set()
    while d <= dates[-1]:
        if d.weekday() in ticked:
            covered.add(d)
        d += dt.timedelta(days=1)
    if covered != want:
        return None
    return dates[-1].isoformat(), ",".join(WEEKDAYS[w] for w in ticked)


def clean(s):
    return re.sub(r"\s+", " ", s or "").strip()


def tsv_lines(cand):
    """The TSV lines of one candidate: per venue, one line per time slot,
    collapsed to a repeating line when the dates form a weekly pattern."""
    by_venue = {}
    for s in cand["shows"]:
        by_venue.setdefault((s["venue"], s["city"]), []).append(s)
    n = len(cand["shows"])
    sold = sum(1 for s in cand["shows"] if s["sold_out"])
    yield "# more.com %s  popularity=%d  dates=%d  sold_out=%d  venues=%d  listed_as=%s" % (
        cand["code"], cand["popularity"], n, sold, len(by_venue), clean(cand["listing_name"]))
    for (vname, city), shows in sorted(by_venue.items()):
        venue = clean(vname) if not city or city.lower() in vname.lower() else f"{clean(vname)}, {clean(city)}"
        by_time = {}
        for s in shows:
            by_time.setdefault(s["time"], set()).add(dt.date.fromisoformat(s["day"]))
        for clock, days in sorted(by_time.items()):
            days = sorted(days)
            run = series(days)
            rows = [(days[0], run)] if run else [(d, None) for d in days]
            for day, run in rows:
                until, wd = run if run else ("", "")
                yield "\t".join([
                    clean(cand["title"]), day.isoformat(), clock, venue, cand["price"], cand["tag"],
                    clean(cand["description"]), "Tickets (more.com)", cand["url"], "", "", "", "", until, wd,
                ])


def main():
    ap = argparse.ArgumentParser(description="draft a seed TSV from more.com")
    ap.add_argument("category", choices=sorted(LISTING))
    ap.add_argument("--from", dest="date_from", required=True, help="first date, YYYY-MM-DD")
    ap.add_argument("--to", dest="date_to", required=True, help="last date, YYYY-MM-DD")
    ap.add_argument("--region", default="Αττική", help="schema.org addressRegion to keep (default Αττική; '' for all)")
    ap.add_argument("--venue", default="", help="keep only venues whose name contains this")
    ap.add_argument("--match", default="", help="keep only titles matching this regex (case-insensitive)")
    ap.add_argument("--min-popularity", type=int, default=0, help="skip events below this more.com popularity")
    ap.add_argument("--max", type=int, default=0, help="stop after this many events (0 = all)")
    ap.add_argument("--json", action="store_true", help="write the full candidate records as JSON instead of TSV")
    ap.add_argument("--cache", default=os.path.join(os.environ.get("TMPDIR", "/tmp"), "dside-fetch-cache"))
    ap.add_argument("--pause", type=float, default=1.0, help="seconds between live requests")
    args = ap.parse_args()
    for d in (args.date_from, args.date_to):
        dt.date.fromisoformat(d)

    f = Fetcher(args.cache, args.pause)
    listing = parse_listing(f.get(BASE + LISTING[args.category]))
    picked = []
    for a in listing:
        if args.region and a["region"] != args.region:
            continue
        if a["start"] > args.date_to or (a["end"] or a["start"]) < args.date_from:
            continue
        if a["popularity"] < args.min_popularity:
            continue
        if args.match and not re.search(args.match, a["name"], re.I):
            continue
        if args.venue and args.venue.lower() not in a["venue"].lower() and "πολλαπλ" not in a["venue"].lower():
            continue
        picked.append(a)
    log(f"{len(listing)} events listed, {len(picked)} in {args.region or 'any region'} between {args.date_from} and {args.date_to}")

    cands = []
    for i, a in enumerate(picked, 1):
        if args.max and len(cands) >= args.max:
            break
        log(f"[{i}/{len(picked)}] {a['name']}")
        try:
            page = f.get(a["url"])
        except Exception as e:  # keep going; one bad page should not lose the run
            log(f"  failed: {e}")
            continue
        c = build_event(a, page, args, TAG[args.category])
        if c:
            cands.append(c)
    cands.sort(key=lambda c: (min(s["day"] for s in c["shows"]), -c["popularity"]))

    out = sys.stdout
    if args.json:
        json.dump(cands, out, ensure_ascii=False, indent=1)
        out.write("\n")
    else:
        out.write("# " + "\t".join(COLUMNS) + "\n")
        out.write(f"# draft from more.com/{args.category} {args.date_from}..{args.date_to} region={args.region or 'any'} "
                  f"fetched {dt.date.today().isoformat()} — curate, rewrite descriptions, then seed/check.py\n")
        for c in cands:
            for line in tsv_lines(c):
                out.write(line + "\n")
    log(f"{len(cands)} candidates written")


if __name__ == "__main__":
    main()
