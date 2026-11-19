#!/usr/bin/env node
// check-snfcc.js — watch the Stavros Niarchos Foundation Cultural Center
// (SNFCC / ΚΠΙΣΝ) for new events, alongside check-venues.py.
//
// snfcc.org is a WordPress site behind an Akamai bot gate: plain HTTP
// clients get 403 (TLS fingerprinting), so this uses puppeteer-core with
// the Chrome for Testing binary that omp keeps under ~/.omp/puppeteer.
// The WP REST API (`/wp-json/wp/v2/event`) lists events ordered by
// modification date, and every event page carries a schema.org `Event`
// JSON-LD block with the real startDate — the REST record itself has no
// date fields. So: take events modified since the last look, open each
// page, read the JSON-LD, keep what is still upcoming, and diff the titles
// against production plus the committed seed/*.tsv files. Exit 1 means
// there is something new to curate.
//
// Runs from the dside-venue-watch systemd user timer (same schedule as
// check-venues.py, staggered so both don't open the browser at once).

import fs from "node:fs";
import path from "node:path";
import puppeteer from "puppeteer-core";

const REPO = path.join(import.meta.dir, "..");
const REPORT = path.join(REPO, "seed", "snfcc-watch-report.txt");
const STATE = path.join(import.meta.dir, ".snfcc-watch-state"); // last run timestamp
const PROD_URL = "https://events.dside.studio";
const SNFCC = "https://www.snfcc.org";
// Chrome for Testing installed by omp's browser prelude; keep in step with it.
const CHROME = [
  path.join(process.env.HOME, ".omp/puppeteer/chrome"),
  ...fs.readdirSync(path.join(process.env.HOME, ".omp/puppeteer/chrome")),
  "chrome-linux64/chrome",
].join("/");

const UA = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0 Safari/537.36";

const lines = [];
const out = (...m) => { lines.push(...m); };

const norm = (s) => s.replace(/\s+/g, " ").trim().toLowerCase();

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

// Akamai intermittently serves an HTML error page instead of the answer.
// Retry a few times with backoff before giving up on the whole run.
async function resilient(page, fn, what) {
  for (let attempt = 1; attempt <= 4; attempt++) {
    try {
      return await fn();
    } catch (e) {
      if (attempt === 4) throw new Error(`${what}: ${e.message || e}`);
      await sleep(5000 * attempt);
    }
  }
}

async function main() {
  const known = new Set();

  // What is already on the site: prod + committed seeds.
  const res = await fetch(`${PROD_URL}/upcoming`);
  const prod = await res.text();
  for (const m of prod.matchAll(/href="\/e\/[^"]*"[^>]*>([^<]+)</g)) known.add(norm(m[1]));
  for (const f of fs.readdirSync(path.join(REPO, "seed"))) {
    if (!f.endsWith(".tsv")) continue;
    for (const line of fs.readFileSync(path.join(REPO, "seed", f), "utf8").split("\n")) {
      if (line && !line.startsWith("#") && !line.startsWith("title\t")) known.add(norm(line.split("\t")[0]));
    }
  }
  out(`snfcc watch ${new Date().toISOString().slice(0, 16)} UTC`,
      `${known.size} titles known (prod + seed/*.tsv)`);

  const since = fs.existsSync(STATE)
    ? new Date(fs.readFileSync(STATE, "utf8").trim())
    : new Date(Date.now() - 14 * 864e5);
  out(`events modified since ${since.toISOString().slice(0, 10)}`);

  const browser = await puppeteer.launch({
    executablePath: CHROME,
    headless: true,
    args: ["--no-sandbox", "--disable-dev-shm-usage"],
  });
  const page = await browser.newPage();
  await page.setUserAgent(UA);
  try {
    // Land once; after that everything is same-origin fetches, cheap and JSON.
    await resilient(page,
      () => page.goto(`${SNFCC}/en`, { waitUntil: "domcontentloaded", timeout: 90000 }),
      "landing on snfcc.org");

    // Modified-descending REST pages until we pass the watermark.
    let recent = [];
    for (let paged = 1; paged <= 5; paged++) {
      const recs = await resilient(page, () => page.evaluate(async (p) => {
        const r = await fetch(`/wp-json/wp/v2/event?per_page=100&orderby=modified&order=desc&_fields=id,link,title,modified&paged=${p}`);
        const j = await r.json();
        if (!Array.isArray(j)) throw new Error("REST returned " + JSON.stringify(j).slice(0, 80));
        return j;
      }, paged), `REST page ${paged}`);
      recent.push(...recs);
      if (recs.length < 100 || recs.every((r) => new Date(r.modified) <= since)) break;
    }
    recent = recent.filter((r) => new Date(r.modified) > since);
    // Each event exists once per language translation with the same link.
    const unique = [...new Map(recent.map((r) => [r.link, r])).values()];
    out(`${unique.length} event(s) touched since then on snfcc.org (${recent.length} records)`);

    const rows = [];
    const seenRow = new Set();
    for (const ev of unique) {
      // Each event page's JSON-LD carries the real startDate/endDate.
      const events = await resilient(page, () => page.evaluate(async (url) => {
        const html = await fetch(url).then((r) => r.text());
        const found = [];
        for (const m of html.matchAll(/<script type="application\/ld\+json"[^>]*>([\s\S]*?)<\/script>/g)) {
          try {
            const j = JSON.parse(m[1]);
            for (const n of j["@graph"] || [j]) {
              if (n["@type"] === "Event" || (Array.isArray(n["@type"]) && n["@type"].includes("Event")))
                found.push({ name: n.name, start: n.startDate, end: n.endDate, offers: n.offers?.url || n.url });
            }
          } catch {}
        }
        return found;
      }, ev.link), `JSON-LD for ${ev.link}`);
      for (const e of events) {
        if (!e.start || new Date(e.start.replace(" ", "T")) < Date.now()) continue;
        // One JSON-LD block per language on the page; same (name, start) repeats.
        const key = `${norm(e.name || "")}|${e.start}`;
        if (seenRow.has(key)) continue;
        seenRow.add(key);
        rows.push({ ...e, page: ev.link, isNew: !known.has(norm(e.name || "")) });
      }
    }

    const fresh = rows.filter((r) => r.isNew);
    out(`\n== Stavros Niarchos Foundation Cultural Center (SNFCC): ${rows.length} upcoming, ${fresh.length} not yet on the site`);
    for (const r of rows) {
      out(`${r.isNew ? "  NEW " : "  seen"} ${r.start} – ${r.end || "?"} — ${r.name}`);
      if (r.isNew) out(`        ${r.page}`);
    }
    out(`\ntotal new: ${fresh.length}`);
    fs.writeFileSync(STATE, new Date().toISOString());
    return fresh.length;
  } finally {
    await browser.close();
  }
}

main()
  .then((fresh) => {
    fs.writeFileSync(REPORT, lines.join("\n") + "\n");
    console.log(lines.join("\n"));
    process.exit(fresh ? 1 : 0);
  })
  .catch((e) => {
    lines.push(`SNFCC WATCH FAILED: ${e.message || e}`);
    fs.writeFileSync(REPORT, lines.join("\n") + "\n");
    console.error(lines.join("\n"));
    process.exit(2);
  });
