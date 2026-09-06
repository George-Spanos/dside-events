package e2e

import (
	"net/url"
	"testing"
)

// spec: AccountPage, Poster, SecretLink, PosterPage
func TestAccount_Poster_CuratorPageAndNoDelete(t *testing.T) {
	r := asPoster(t, poster1).get("/account")
	assertStatus(t, r, 200)
	assertContains(t, r, "Your curator page")
	assertContains(t, r, `href="/p/`+poster1.Slug+`"`)
	assertContains(t, r, "<h2>Your secret link</h2>")
	assertContains(t, r, `href="`+poster1.Link+`"`)
	assertForm(t, r, `action="/account/key"`)
	assertForm(t, r, `action="/forget"`)
	assertNoForm(t, r, `action="/account/delete"`)
	if !containsFold(r.Body, "by hand") {
		t.Errorf("poster account page does not say curator accounts are removed by hand")
	}
	assertNotContains(t, r, "@")
}

// spec: DeleteAccount, AccountPage, Event.follower_count, Session, SecretLink, EventFollow
func TestDeleteAccount_RemovesDataSessionAndLink(t *testing.T) {
	f := validEvent(t, tomorrow())
	slug := createEvent(t, asPoster(t, poster1), f)
	page := "/e/" + slug
	c := newUser(t)
	old := c.cookie("session")
	assertRedirect(t, setEventFollow(c, slug, "follow", page), page)
	link, _ := secretLink(t, c)
	other := openLink(t, shared, link) // the same account on a second device
	if n := followerCount(t, anon(t).get(page).Body); n != 1 {
		t.Fatalf("counter = %d, want 1", n)
	}

	r := c.postForm("/account/delete", url.Values{"confirm": {"1"}})
	assertRedirect(t, r, "/")
	if c.cookie("session") != nil {
		t.Errorf("session cookie still in jar after account deletion")
	}
	r = c.get("/account")
	assertStatus(t, r, 200)
	assertCopy(t, r, copyAccountNone)

	// The session rows are gone, not just the cookie: the old token and the
	// second device are anonymous now.
	replay := anon(t)
	replay.setRawCookie("session", old.Value, "/")
	r = replay.get("/account")
	assertStatus(t, r, 200)
	assertCopy(t, r, copyAccountNone)
	assertNotContains(t, replay.get("/mine"), f.Title)
	r = other.get("/mine")
	assertStatus(t, r, 200)
	assertNotContains(t, r, f.Title)
	assertCopy(t, other.get("/account"), copyAccountNone)

	// The secret link is dead.
	r = anon(t).get(keyPath(link))
	assertStatus(t, r, 404)
	assertCopy(t, r, copyLinkBroken)

	// Event follows cascaded: the public counter drops.
	if n := followerCount(t, anon(t).get(page).Body); n != 0 {
		t.Errorf("counter after account deletion = %d, want 0", n)
	}

	// Pressing Follow again starts from a clean, different account.
	assertRedirect(t, setEventFollow(c, slug, "follow", page), page)
	if l, _ := secretLink(t, c); l == link {
		t.Errorf("new account after deletion got the deleted link back")
	}
	assertNotContains(t, c.get(page), "Hidden from Upcoming")
	if n := followerCount(t, anon(t).get(page).Body); n != 1 {
		t.Errorf("counter = %d, want 1 (new account only)", n)
	}
}

// spec: DeleteAccount, AccountPage
func TestDeleteAccount_RequiresConfirm(t *testing.T) {
	c := newUser(t)
	link, _ := secretLink(t, c)
	assertStatus(t, c.postForm("/account/delete", nil), 422)
	assertStatus(t, c.postForm("/account/delete", url.Values{"confirm": {"0"}}), 422)
	assertStatus(t, c.postForm("/account/delete", url.Values{"confirm": {"yes"}}), 422)
	// Still there, still the same account.
	if c.cookie("session") == nil {
		t.Fatalf("a refused delete cleared the cookie")
	}
	if l, _ := secretLink(t, c); l != link {
		t.Errorf("a refused delete changed the account (link %q → %q)", link, l)
	}
	assertStatus(t, c.get("/account/delete"), 405)
	// The account page says what deleting does.
	assertCopy(t, c.get("/account"), copyAccountDelete)
}

// spec: DeleteAccount, AccountPage, Poster
func TestDeleteAccount_PosterForbidden(t *testing.T) {
	p := asPoster(t, poster2)
	r := p.postForm("/account/delete", url.Values{"confirm": {"1"}})
	assertStatus(t, r, 403)
	if !containsFold(r.Body, "by hand") {
		t.Errorf("403 body does not say curator accounts are removed by hand\nbody: %s", snippet(r.Body))
	}
	// Still a poster, still logged in, link still works.
	assertStatus(t, p.get("/new"), 200)
	assertStatus(t, anon(t).get("/p/"+poster2.Slug), 200)
	assertStatus(t, asPoster(t, poster2).get("/new"), 200)
}
