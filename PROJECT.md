# dside-events specification

Event discovery platform for Athens. Concerts, theater, films and more.

This file consolidates the concept, and restrictions on the MVP.

## 1. Product essence

**Problem statement**: Me and many artists dislike the promo and content they have to do on social media. Most artists do not want to promote and create content constantly.
Most individuals just want to discover events that might interest them.

**MVP restrictions**

- Two types of users, users and power users.
- Power users - Curators/artists - People we know, whitelisted, accounts by hand.
- No images no videos. All content is text with links to external content. ex musical content related to the event, link to a theater script brief etc.
- No spam. A single post per event.
- Events have tags. Tags are a specific defined list.
- Users - Attendees - ability to follow tags and/or curators/artists.
- Users - Attendees - email-only accounts with a single otp code for login.
- Simple UI - essentially our app is 
  - a list of upcoming events.
  - a list/calendar of events you're interested in.
  - an event details view.
  - an event create/edit view.
- No metrics.
- One post per event; editing yes, reposting no. Slug/URL immutable.
- "Not interested" stays internal is not visible to an event.
- "Intersted is clear and visible to an event.
- Posters are PUBLIC curators: their name is the trust signal, following a poster = following a
  curator. Event pages show who posted.
- No notifications. Users should open the app to see updates.
- App should be a PWA.
- Anon users can just browse, no login required but no follow features.

### Design direction (founder, firm)
- As simple and non-obstructive as possible. **Old internet feel.**
- Designed for mobile-first, responsive design. 
- The UI is essentially two lists: `/` (upcoming events) and `/mine` (events you marked you're
  going to). Click an event → its info page. THAT'S IT.
- Minimal CSS: system font stack, underlined text links, no cards/shadows/animations/hero.
  Filters are plain text links, not chip widgets. No images anywhere.

## 2. Stack 

**Go single static binary + SQLite + server-rendered HTML (html/template) + vanilla JS.**
No JS framework, no build step, no npm.

- use std lib whenever possible.
- keep code simple and adhere to google go principles.
- Deploy: docker image with a respective volume for sqlite.

## 3. Verification

Create an e2e suite that tests ALL features.

# Skills

The AI agents should use the following skills and always work with subagents: 
1. Allium Skills
2. Frontend Design
3. Superpowers
