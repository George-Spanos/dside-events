#!/usr/bin/env python3
"""check.py — validate a seed TSV before seed.sh publishes it.

    seed/check.py seed/theater-athens-2026-10.tsv
    seed/check.py seed/x.tsv --urls                       # also GET every link
    seed/check.py seed/x.tsv --against https://events.dside.studio   # warn on titles already up

Applies the same rules as the /new form (lengths, tags, dates, links, the
repeat fields) so a bad line fails here, in one place, instead of as a 422
half-way through publishing. Errors exit 1; warnings only print. With
--against it downloads the instance's upcoming list and warns about titles
that are already on the site (same title, same day, which the server would
skip as a duplicate) so a re-fetched draft does not repost what is there.

Standard library only.
"""

import argparse
import datetime as dt
import html
import re
import sys
import urllib.error
import urllib.request
from urllib.parse import urlparse

TAGS = {"concert", "theater", "film", "exhibition"}
WEEKDAYS = ["mon", "tue", "wed", "thu", "fri", "sat", "sun"]
COLUMNS = ["title", "date", "time", "venue", "price", "tags", "description",
           "label1", "url1", "label2", "url2", "label3", "url3", "until", "weekdays"]
MAX_SERIES = 200
UA = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0 Safari/537.36"


def normalize(title):
    return " ".join(title.split()).lower()


def parse_date(s):
    try:
        return dt.date.fromisoformat(s)
    except ValueError:
        return None


def expand(first, until, ticked):
    """Every date from first to until whose weekday is ticked, like the
    server's expandSeries; stops once it is clearly too long."""
    out, d = [], first
    while d <= until and len(out) <= MAX_SERIES:
        if WEEKDAYS[d.weekday()] in ticked:
            out.append(d)
        d += dt.timedelta(days=1)
    return out


def check_row(n, row, today):
    """Errors and the days the row occupies (for the duplicate check)."""
    errs, warns = [], []
    f = dict(zip(COLUMNS, row + [""] * (len(COLUMNS) - len(row))))
    if len(row) > len(COLUMNS):
        errs.append(f"{len(row)} columns; at most {len(COLUMNS)} (a stray tab?)")
    for k, v in f.items():
        if "\n" in v or "\r" in v:
            errs.append(f"{k}: contains a line break")

    if not f["title"]:
        errs.append("title: required")
    elif len(f["title"]) > 120:
        errs.append("title: over 120 characters")
    if not f["venue"]:
        errs.append("venue: required")
    elif len(f["venue"]) > 120:
        errs.append("venue: over 120 characters")
    if len(f["price"]) > 60:
        errs.append("price: over 60 characters")
    if not f["price"]:
        warns.append("price: empty (the page will show none)")
    if len(f["description"]) > 4000:
        errs.append("description: over 4000 characters")
    if not f["description"]:
        warns.append("description: empty")
    elif re.search(r"[Ͱ-Ͽ]{4,}", f["description"]) and not re.search(r"\b(the|and|with|by|of)\b", f["description"]):
        warns.append("description: looks like the untranslated Greek blurb from the source")

    tags = [t.strip() for t in f["tags"].split(",") if t.strip()]
    if not tags:
        errs.append("tags: pick at least one")
    for t in tags:
        if t not in TAGS:
            errs.append(f"tags: {t!r} is not one of {sorted(TAGS)}")
    if len(set(tags)) > 4:
        errs.append("tags: at most 4")

    date = parse_date(f["date"])
    if not date:
        errs.append(f"date: {f['date']!r} is not YYYY-MM-DD")
    elif not (today.replace(year=today.year - 1) <= date <= today.replace(year=today.year + 3)):
        errs.append("date: must be within the past year or the next three years")
    elif date < today:
        warns.append("date: already in the past")
    if not re.fullmatch(r"([01]\d|2[0-3]):[0-5]\d", f["time"]):
        errs.append(f"time: {f['time']!r} is not HH:MM")

    urls = []
    for i in (1, 2, 3):
        label, url = f[f"label{i}"], f[f"url{i}"]
        if not label and not url:
            continue
        u = urlparse(url)
        if not url or u.scheme not in ("http", "https") or not u.netloc or len(url) > 500:
            errs.append(f"url{i}: needs a full address starting with https://")
            continue
        if len(label) > 80:
            errs.append(f"label{i}: over 80 characters")
        if not label:
            warns.append(f"label{i}: empty, the page will show the host name")
        urls.append(url)

    days = [date] if date else []
    if f["until"] or f["weekdays"]:
        until = parse_date(f["until"])
        ticked = [w for w in f["weekdays"].split(",") if w] or WEEKDAYS
        bad = [w for w in ticked if w not in WEEKDAYS]
        if bad:
            errs.append(f"weekdays: {bad} not in {WEEKDAYS}")
        if not until:
            errs.append(f"until: {f['until']!r} is not YYYY-MM-DD")
        elif date and until < date:
            errs.append("until: before date")
        elif date and until > today.replace(year=today.year + 3):
            errs.append("until: more than three years ahead")
        elif date and not bad:
            days = expand(date, until, ticked)
            if WEEKDAYS[date.weekday()] not in ticked:
                warns.append(f"weekdays: the first date is a {WEEKDAYS[date.weekday()]}, which is not ticked, so it is not a show")
            if len(days) < 2:
                errs.append("until/weekdays: a repeating event needs at least two dates")
            elif len(days) > MAX_SERIES:
                errs.append(f"until/weekdays: more than {MAX_SERIES} dates")
    return f, errs, warns, days, urls


def fetch(url, timeout=25):
    req = urllib.request.Request(url, headers={"User-Agent": UA, "Accept-Language": "el,en"})
    try:
        with urllib.request.urlopen(req, timeout=timeout) as r:
            return r.status, r.read().decode("utf-8", "replace")
    except urllib.error.HTTPError as e:
        return e.code, ""
    except Exception as e:  # DNS, TLS, timeout: report, do not crash
        return 0, str(e)


def live_titles(base):
    """Titles on the instance's upcoming list (every event, all dates)."""
    status, page = fetch(base.rstrip("/") + "/upcoming")
    if status != 200:
        print(f"warning: could not read {base}/upcoming (HTTP {status})", file=sys.stderr)
        return set()
    titles = re.findall(r'href="/e/[^"]+"[^>]*>([^<]+)<', page)
    return {normalize(html.unescape(t)) for t in titles}


def main():
    ap = argparse.ArgumentParser(description="validate a seed TSV")
    ap.add_argument("file")
    ap.add_argument("--urls", action="store_true", help="GET every link and report anything but 2xx/3xx")
    ap.add_argument("--against", metavar="BASE_URL", help="warn about titles already on this instance")
    args = ap.parse_args()

    today = dt.date.today()
    errors = warnings = rows = 0
    seen_days = {}
    all_urls = {}
    with open(args.file, encoding="utf-8", newline="") as fh:
        for n, raw in enumerate(fh, 1):
            line = raw.rstrip("\n").rstrip("\r")
            if not line.strip() or line.startswith("#"):
                continue
            rows += 1
            f, errs, warns, days, urls = check_row(n, line.split("\t"), today)
            key = normalize(f["title"])
            for d in days:
                prev = seen_days.get((key, d))
                if prev:
                    warns.append(f"same title and day as line {prev} — the server will skip one as a duplicate")
                    break
            for d in days:
                seen_days.setdefault((key, d), n)
            for u in urls:
                all_urls.setdefault(u, n)
            for e in errs:
                print(f"{args.file}:{n}: error: {e}  [{f['title'][:60]}]")
            for w in warns:
                print(f"{args.file}:{n}: warning: {w}  [{f['title'][:60]}]")
            errors += len(errs)
            warnings += len(warns)

    if args.against:
        live = live_titles(args.against)
        for (key, d), n in sorted(seen_days.items(), key=lambda x: x[1]):
            if key in live:
                print(f"{args.file}:{n}: warning: a title like this is already on {args.against}: {key!r}")
                warnings += 1
                live.discard(key)  # once per title

    if args.urls:
        for u, n in all_urls.items():
            status, body = fetch(u)
            if 200 <= status < 400:
                continue
            kind = "warning" if status in (403, 429, 999) else "error"
            print(f"{args.file}:{n}: {kind}: link {u} answered {status or body}")
            if kind == "error":
                errors += 1
            else:
                warnings += 1

    print(f"{args.file}: {rows} events, {errors} errors, {warnings} warnings")
    sys.exit(1 if errors else 0)


if __name__ == "__main__":
    main()
