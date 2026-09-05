package e2e

import (
	"net/url"
	"strings"
	"testing"
)

const (
	copyLinkBroken   = "This link doesn't work. It may have been replaced with a new one."
	copyMineEmpty    = "Nothing here yet. Press Interested on an event and it shows up here."
	copyAccountNone  = "This device has no list yet. Press Interested on an event, or follow a tag or a curator, and your account starts here. No sign-up, no email."
	copyAccountAgain = "Opened a secret link before? Open it again on this device to continue there."
	copyFollowEmpty  = "You're not following anything yet. Pick a tag above and press Follow, or follow a curator from an event page."
)

// spec: StartAccount, Visitor, Account, Session, session_duration, MarkInterested, EventDetail, MyEvents
func TestStartAccount_InterestFromAnonCreatesAccount(t *testing.T) {
	f := validEvent(t, tomorrow())
	slug := createEvent(t, asPoster(t, poster1), f)
	page := "/e/" + slug

	v := anon(t)
	if v.cookie("session") != nil {
		t.Fatalf("fresh client already has a session cookie")
	}
	// Reading the event page does not start anything.
	r := v.get(page)
	assertStatus(t, r, 200)
	if v.cookie("session") != nil || sessionSetCookie(r) != "" {
		t.Fatalf("GET %s set a session cookie", page)
	}

	// The first Interested starts the account and performs the action.
	r = setInterest(v, slug, "interested", page)
	assertRedirect(t, r, page)
	assertSessionCookieFlags(t, r)
	if v.cookie("session") == nil {
		t.Fatalf("no session cookie in jar after the first interest POST")
	}

	r = v.get("/mine")
	assertStatus(t, r, 200)
	assertContains(t, r, f.Title)
	assertNoCopy(t, r, copyMineEmpty)
	if n := interestedCount(t, anon(t).get(page).Body); n != 1 {
		t.Errorf("public counter = %d, want 1", n)
	}
	// The next POST reuses the account: the cookie is not replaced.
	old := v.cookie("session").Value
	r = setInterest(v, slug, "interested", page)
	assertRedirect(t, r, page)
	if sessionSetCookie(r) != "" {
		t.Errorf("a POST with a valid session re-issued the cookie: %q", sessionSetCookie(r))
	}
	if v.cookie("session").Value != old {
		t.Errorf("session cookie changed between two POSTs of the same visitor")
	}
	if n := interestedCount(t, anon(t).get(page).Body); n != 1 {
		t.Errorf("counter after the second mark = %d, want 1 (same account)", n)
	}
}

// spec: StartAccount, Visitor, Account, Session, FollowTag, AccountPage, Feed
func TestStartAccount_FollowFromAnonCreatesAccount(t *testing.T) {
	f := validEvent(t, tomorrow())
	f.Tags = []string{"workshop"}
	createEvent(t, asPoster(t, poster1), f)

	v := anon(t)
	r := follow(v, "tag", "workshop", "1", "/following")
	assertRedirect(t, r, "/following")
	assertSessionCookieFlags(t, r)
	if v.cookie("session") == nil {
		t.Fatalf("no session cookie in jar after the first follow POST")
	}

	r = v.follow(r)
	assertStatus(t, r, 200)
	assertContains(t, r, f.Title)
	r = v.get("/account")
	assertStatus(t, r, 200)
	assertForm(t, r, `action="/follow"`, `value="tag"`, `value="workshop"`, `value="0"`)
	assertNoCopy(t, r, copyAccountNone)

	// A follow that fails (unknown target) from a fresh visitor gives 404 and
	// must not leave a half-made account behind in the cookie.
	w := anon(t)
	r = follow(w, "tag", "opera", "1", "/")
	assertStatus(t, r, 404)
	if w.cookie("session") != nil {
		t.Errorf("a rejected follow POST left a session cookie")
	}
}

// spec: StartAccount, Session, Account
func TestStartAccount_DifferentVisitorsGetDifferentAccounts(t *testing.T) {
	slug := createEvent(t, asPoster(t, poster1), validEvent(t, tomorrow()))
	page := "/e/" + slug
	a, b := anon(t), anon(t)
	assertRedirect(t, setInterest(a, slug, "interested", page), page)
	assertRedirect(t, setInterest(b, slug, "interested", page), page)
	if a.cookie("session").Value == b.cookie("session").Value {
		t.Fatalf("two visitors share one session token")
	}
	if n := interestedCount(t, anon(t).get(page).Body); n != 2 {
		t.Errorf("counter = %d, want 2 (two accounts)", n)
	}
	_, ka := secretLink(t, a)
	_, kb := secretLink(t, b)
	if ka == kb {
		t.Errorf("two accounts share the secret key %s", ka)
	}
}

// spec: MyEvents, Feed, AccountPage, Visitor
func TestReadPages_NoSession_200WithEmptyCopy_NeverRedirect(t *testing.T) {
	v := anon(t)

	r := v.get("/mine")
	assertStatus(t, r, 200)
	assertContains(t, r, "Mine")
	assertCopy(t, r, copyMineEmpty)
	assertNotContains(t, r, `href="/e/`)

	r = v.get("/following")
	assertStatus(t, r, 200)
	assertCopy(t, r, copyFollowEmpty)

	r = v.get("/account")
	assertStatus(t, r, 200)
	assertContains(t, r, "Account")
	assertCopy(t, r, copyAccountNone)
	assertCopy(t, r, copyAccountAgain)
	assertNotContains(t, r, "/k/")
	assertNoForm(t, r, `action="/account/key"`)
	assertNoForm(t, r, `action="/forget"`)
	assertNoForm(t, r, `action="/account/delete"`)

	for _, path := range []string{"/", "/mine", "/following", "/account"} {
		r := v.get(path)
		assertStatus(t, r, 200)
		if sessionSetCookie(r) != "" || v.cookie("session") != nil {
			t.Fatalf("GET %s set a session cookie for a visitor without one", path)
		}
		// Nav is always mine · account.
		assertContains(t, r, `href="/mine"`)
		assertContains(t, r, `href="/account"`)
		assertNotContains(t, r, `href="/login`)
	}
}

// spec: AccountPage, Account, SecretLink, Account.key, key_length
func TestAccount_WithSession_ShowsSecretLinkAndForms(t *testing.T) {
	u := newUser(t)
	link, key := secretLink(t, u)
	if len(key) != 43 {
		t.Errorf("key %q has length %d, want 43 (32 bytes base64url)", key, len(key))
	}
	if strings.Trim(key, "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_") != "" {
		t.Errorf("key %q is not base64url", key)
	}
	if !strings.HasSuffix(link, "/k/"+key) {
		t.Errorf("link %q does not end in /k/%s", link, key)
	}

	r := u.get("/account")
	assertStatus(t, r, 200)
	assertContains(t, r, "<h2>Your secret link</h2>")
	assertContains(t, r, `<a href="`+link+`">`+link+`</a>`)
	assertCopy(t, r, "Open it on another device to see the same list there. Anyone with this link is you, so keep it to yourself.")
	assertForm(t, r, `action="/account/key"`, "Get a new link")
	assertCopy(t, r, "The old link stops working.")
	assertContains(t, r, "<h2>Following</h2>")
	assertContains(t, r, "<h2>This device</h2>")
	assertForm(t, r, `action="/forget"`, "Forget this device")
	assertCopy(t, r, "Your list stays; the secret link brings it back.")
	assertContains(t, r, "<h2>Delete account</h2>")
	assertForm(t, r, `action="/account/delete"`, `name="confirm"`)
	assertNoCopy(t, r, copyAccountNone)
	// No email anywhere.
	assertNotContains(t, r, `type="email"`)
	assertNotContains(t, r, "@")

	// The link is stable across reads until rotated.
	link2, _ := secretLink(t, u)
	if link2 != link {
		t.Errorf("secret link changed between two reads: %q → %q", link, link2)
	}
}

// spec: OpenSecretLink, SecretLink, Session, Account, MyEvents, Visitor
func TestOpenSecretLink_SameListOnAnotherDevice(t *testing.T) {
	f := validEvent(t, tomorrow())
	slug := createEvent(t, asPoster(t, poster1), f)
	page := "/e/" + slug

	phone := newUser(t)
	assertRedirect(t, setInterest(phone, slug, "interested", page), page)
	assertRedirect(t, follow(phone, "tag", "talk", "1", "/account"), "/account")
	link, _ := secretLink(t, phone)

	laptop := openLink(t, shared, link)
	if laptop.cookie("session").Value == phone.cookie("session").Value {
		t.Errorf("the link re-used the phone's session token instead of creating a new session")
	}
	r := laptop.get("/mine")
	assertStatus(t, r, 200)
	assertContains(t, r, f.Title)
	r = laptop.get("/account")
	assertForm(t, r, `action="/follow"`, `value="tag"`, `value="talk"`, `value="0"`)
	link2, _ := secretLink(t, laptop)
	if link2 != link {
		t.Errorf("laptop sees link %q, phone %q; want the same account", link2, link)
	}
	// Same account, not a copy: the counter still says one.
	if n := interestedCount(t, anon(t).get(page).Body); n != 1 {
		t.Errorf("counter = %d after opening the link, want 1", n)
	}
	// Both devices stay live and see each other's changes.
	g := validEvent(t, tomorrow())
	slug2 := createEvent(t, asPoster(t, poster1), g)
	assertRedirect(t, setInterest(laptop, slug2, "interested", "/mine"), "/mine")
	assertContains(t, phone.get("/mine"), g.Title)
	assertContains(t, phone.get("/mine"), f.Title)

	// The link can be opened as often as needed.
	tablet := openLink(t, shared, link)
	assertContains(t, tablet.get("/mine"), f.Title)
}

// spec: OpenSecretLink, SecretLink, SecretKeysUnique
func TestOpenSecretLink_Unknown404(t *testing.T) {
	v := anon(t)
	for _, key := range []string{randomKey(), "short", strings.Repeat("A", 44), strings.Repeat("A", 43) + "/x", ""} {
		r := v.get("/k/" + key)
		if r.Status != 404 {
			t.Errorf("GET /k/%s: status %d, want 404", key, r.Status)
			continue
		}
		if len(key) == 43 {
			assertCopy(t, r, copyLinkBroken)
		}
		if v.cookie("session") != nil || sessionSetCookie(r) != "" {
			t.Errorf("GET /k/%s set a session cookie", key)
		}
	}
	// A real key with one character changed is not a key.
	u := newUser(t)
	_, key := secretLink(t, u)
	flipped := []byte(key)
	if flipped[0] == 'A' {
		flipped[0] = 'B'
	} else {
		flipped[0] = 'A'
	}
	r := anon(t).get("/k/" + string(flipped))
	assertStatus(t, r, 404)
	assertCopy(t, r, copyLinkBroken)
	// Wrong method is not a login either.
	if r := anon(t).postForm("/k/"+key, nil); r.Status == 303 {
		t.Errorf("POST /k/{key} logged in (303); only GET should")
	}
}

// spec: OpenSecretLink, SecretLink, Session, MyEvents
func TestOpenSecretLink_SwitchesAccount(t *testing.T) {
	fa := validEvent(t, tomorrow())
	fa.Title = uniqTitle(t, "Alpha")
	fb := validEvent(t, tomorrow())
	fb.Title = uniqTitle(t, "Beta")
	p := asPoster(t, poster1)
	slugA, slugB := createEvent(t, p, fa), createEvent(t, p, fb)

	a := newUser(t)
	assertRedirect(t, setInterest(a, slugA, "interested", "/mine"), "/mine")
	linkA, _ := secretLink(t, a)

	b := newUser(t)
	assertRedirect(t, setInterest(b, slugB, "interested", "/mine"), "/mine")
	linkB, _ := secretLink(t, b)
	oldB := b.cookie("session").Value

	// b's browser opens a's link: it is now a.
	r := b.get(keyPath(linkA))
	assertRedirect(t, r, "/mine")
	if b.cookie("session").Value == oldB {
		t.Fatalf("cookie unchanged after opening another account's link")
	}
	r = b.get("/mine")
	assertStatus(t, r, 200)
	assertContains(t, r, fa.Title)
	assertNotContains(t, r, fb.Title)
	if l, _ := secretLink(t, b); l != linkA {
		t.Errorf("after the switch /account shows %q, want a's link %q", l, linkA)
	}
	// Nothing about b was lost: its own link still opens b's list.
	back := openLink(t, shared, linkB)
	r = back.get("/mine")
	assertContains(t, r, fb.Title)
	assertNotContains(t, r, fa.Title)
	// a's original device is unaffected.
	assertContains(t, a.get("/mine"), fa.Title)
}

// spec: RotateSecretLink, SecretLink, AccountPage, Session, SecretKeysUnique, Account.key
func TestRotateSecretLink_OldLinkDies_SessionsSurvive(t *testing.T) {
	f := validEvent(t, tomorrow())
	slug := createEvent(t, asPoster(t, poster1), f)
	u := newUser(t)
	assertRedirect(t, setInterest(u, slug, "interested", "/mine"), "/mine")
	oldLink, oldKey := secretLink(t, u)
	other := openLink(t, shared, oldLink) // a second device on the same account

	r := u.postForm("/account/key", nil)
	assertRedirect(t, r, "/account")
	newLink, newKey := secretLink(t, u)
	if newKey == oldKey || newLink == oldLink {
		t.Fatalf("key did not change on rotate: %s", newKey)
	}
	if len(newKey) != 43 {
		t.Errorf("new key %q has length %d, want 43", newKey, len(newKey))
	}

	// The old link is dead; the new one works.
	r = anon(t).get(keyPath(oldLink))
	assertStatus(t, r, 404)
	assertCopy(t, r, copyLinkBroken)
	fresh := openLink(t, shared, newLink)
	assertContains(t, fresh.get("/mine"), f.Title)

	// Existing sessions on both devices are untouched.
	assertContains(t, u.get("/mine"), f.Title)
	assertContains(t, other.get("/mine"), f.Title)
	if l, _ := secretLink(t, other); l != newLink {
		t.Errorf("second device shows link %q, want the rotated %q", l, newLink)
	}
	// Rotating twice yields yet another key.
	assertRedirect(t, u.postForm("/account/key", nil), "/account")
	if _, k := secretLink(t, u); k == newKey || k == oldKey {
		t.Errorf("second rotate re-used a key")
	}
	assertStatus(t, anon(t).get(keyPath(newLink)), 404)
	// GET is not a rotate.
	assertStatus(t, u.get("/account/key"), 405)
}

// spec: ForgetDevice, Session, AccountPage, OpenSecretLink, MyEvents
func TestForgetDevice_ClearsCookie_KeepsAccount(t *testing.T) {
	f := validEvent(t, tomorrow())
	slug := createEvent(t, asPoster(t, poster1), f)
	page := "/e/" + slug
	u := newUser(t)
	assertRedirect(t, setInterest(u, slug, "interested", page), page)
	assertRedirect(t, follow(u, "tag", "film", "1", "/account"), "/account")
	link, _ := secretLink(t, u)
	old := u.cookie("session")

	r := u.postForm("/forget", nil)
	assertRedirect(t, r, "/")
	if sc := sessionSetCookie(r); sc == "" {
		t.Errorf("POST /forget did not send a clearing Set-Cookie for session")
	}
	if u.cookie("session") != nil {
		t.Errorf("session cookie still in jar after /forget")
	}
	// This device is anonymous again: no redirect, the no-session copy.
	r = u.get("/account")
	assertStatus(t, r, 200)
	assertCopy(t, r, copyAccountNone)
	assertNotContains(t, r, "/k/")
	r = u.get("/mine")
	assertStatus(t, r, 200)
	assertNotContains(t, r, f.Title)
	assertCopy(t, r, copyMineEmpty)

	// The session row is gone, not just the cookie.
	replay := anon(t)
	replay.setRawCookie("session", old.Value, "/")
	r = replay.get("/account")
	assertStatus(t, r, 200)
	assertCopy(t, r, copyAccountNone)
	assertNotContains(t, replay.get("/mine"), f.Title)

	// The account and its data stay: the counter holds and the link restores it.
	if n := interestedCount(t, anon(t).get(page).Body); n != 1 {
		t.Errorf("counter after /forget = %d, want 1 (account kept)", n)
	}
	again := openLink(t, shared, link)
	r = again.get("/mine")
	assertContains(t, r, f.Title)
	assertForm(t, again.get("/account"), `action="/follow"`, `value="tag"`, `value="film"`, `value="0"`)
	// A forgotten device that presses Interested starts a brand-new account.
	assertRedirect(t, setInterest(u, slug, "interested", page), page)
	if l, _ := secretLink(t, u); l == link {
		t.Errorf("a forgotten device got the old account back without the link")
	}
	if n := interestedCount(t, anon(t).get(page).Body); n != 2 {
		t.Errorf("counter = %d, want 2 (old account + new one)", n)
	}
	assertStatus(t, again.get("/forget"), 405)
}

// spec: ForgetDevice, RotateSecretLink, DeleteAccount, AccountPage, Visitor
func TestSessionPosts_WithoutSession_RedirectToAccount(t *testing.T) {
	v := anon(t)
	for _, path := range []string{"/forget", "/account/key", "/account/delete"} {
		form := url.Values{}
		if path == "/account/delete" {
			form.Set("confirm", "1")
		}
		r := v.postForm(path, form)
		if r.Status != 303 || r.Location != "/account" {
			t.Errorf("anon POST %s: got %d → %q, want 303 → /account\nbody: %s", path, r.Status, r.Location, snippet(r.Body))
		}
		if v.cookie("session") != nil || sessionSetCookie(r) != "" {
			t.Errorf("anon POST %s started an account", path)
		}
	}
	r := v.follow(resp{Status: 303, Location: "/account"})
	assertStatus(t, r, 200)
	assertCopy(t, r, copyAccountNone)
}

// spec: Session, AccountPage, MyEvents, Feed, EventComposer, PwaAssets
func TestLoginRoutes_Gone404(t *testing.T) {
	v := anon(t)
	for _, path := range []string{"/login", "/login/code", "/logout", "/login?next=/mine"} {
		r := v.get(path)
		if r.Status != 404 {
			t.Errorf("GET %s: status %d, want 404", path, r.Status)
		}
		r = v.postForm(path, url.Values{"email": {"a@example.test"}, "code": {"123456"}})
		if r.Status != 404 {
			t.Errorf("POST %s: status %d, want 404", path, r.Status)
		}
		if v.cookie("session") != nil {
			t.Fatalf("%s produced a session cookie", path)
		}
	}
	// No page links to a login any more, logged in or not.
	slug := createEvent(t, asPoster(t, poster1), validEvent(t, tomorrow()))
	u := newUser(t)
	for name, r := range map[string]resp{
		"/ anon":         v.get("/"),
		"/e/{slug} anon": v.get("/e/" + slug),
		"/mine anon":     v.get("/mine"),
		"/account anon":  v.get("/account"),
		"/account user":  u.get("/account"),
		"/new poster":    asPoster(t, poster1).get("/new"),
	} {
		assertStatus(t, r, 200)
		if hasLinkToPath(r.Body, "/login") || hasForm(r.Body, `action="/logout"`) || hasForm(r.Body, `action="/login"`) {
			t.Errorf("%s still offers login/logout\nbody: %s", name, snippet(r.Body))
		}
		if containsFold(r.Body, "log in") || containsFold(r.Body, "log out") {
			t.Errorf("%s still mentions logging in/out\nbody: %s", name, snippet(r.Body))
		}
	}
}

// spec: AddPoster, OpenSecretLink, Poster, EventComposer, MyEvents, PosterPage, Event.PostedByPoster
func TestPosterLink_LogsInAsPoster(t *testing.T) {
	p := asPoster(t, poster1) // asserts 303 /mine + cookie
	r := p.get("/new")
	assertStatus(t, r, 200)
	assertForm(t, r, `action="/new"`, `name="title"`)
	r = p.get("/mine")
	assertStatus(t, r, 200)
	assertContains(t, r, `href="/new"`)
	r = p.get("/account")
	assertStatus(t, r, 200)
	assertContains(t, r, `href="/p/`+poster1.Slug+`"`)
	if l, _ := secretLink(t, p); keyPath(l) != keyPath(poster1.Link) {
		t.Errorf("poster's /account shows link %q, want the CLI link %q", l, poster1.Link)
	}
	f := validEvent(t, tomorrow())
	slug := createEvent(t, p, f)
	r = anon(t).get("/e/" + slug)
	assertStatus(t, r, 200)
	assertContains(t, r, poster1.Name)
	assertContains(t, r, `href="/p/`+poster1.Slug+`"`)

	// Each opening is a fresh session; the earlier one stays valid.
	q := asPoster(t, poster1)
	if q.cookie("session").Value == p.cookie("session").Value {
		t.Errorf("two openings of the poster link share a session token")
	}
	assertStatus(t, p.get("/new"), 200)
	assertStatus(t, q.get("/new"), 200)
}

// spec: RotatePosterLink, AddPoster, OpenSecretLink, SecretKeysUnique, Poster, Session
func TestPosterLink_CLI_RotatesKey(t *testing.T) {
	s := startServer(t)
	p := addPoster(t, s, "Eleni R.", uniqSlug("eleni"))
	first := openLink(t, s, p.Link)
	assertStatus(t, first.get("/new"), 200)

	link2, key2 := posterLink(t, s, p.Slug)
	if key2 == p.Key || link2 == p.Link {
		t.Fatalf("poster-link did not rotate the key: %s", key2)
	}
	if !strings.HasPrefix(link2, s.url+"/k/") {
		t.Errorf("poster-link printed %q, want prefix %s/k/", link2, s.url)
	}
	// Old link dead, new link logs in, the earlier session survives.
	r := newClient(t, s).get(keyPath(p.Link))
	assertStatus(t, r, 404)
	assertCopy(t, r, copyLinkBroken)
	second := openLink(t, s, link2)
	assertStatus(t, second.get("/new"), 200)
	assertStatus(t, first.get("/new"), 200)
	if l, _ := secretLink(t, first); keyPath(l) != keyPath(link2) {
		t.Errorf("/account shows %q after poster-link, want %q", l, link2)
	}
	// The poster can also rotate from the account page; the CLI link dies.
	assertRedirect(t, first.postForm("/account/key", nil), "/account")
	assertStatus(t, newClient(t, s).get(keyPath(link2)), 404)
	link3, _ := secretLink(t, first)
	assertStatus(t, openLink(t, s, link3).get("/new"), 200)

	// Unknown slug fails without printing a link.
	if out, err := runCLI(s, "poster-link", "-slug", "nobody-here-"+uid()); err == nil {
		t.Errorf("poster-link for an unknown slug succeeded:\n%s", out)
	} else if linkLine.MatchString(out) {
		t.Errorf("poster-link for an unknown slug printed a link:\n%s", out)
	}
}
