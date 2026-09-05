package e2e

import (
	"net/url"
	"testing"
)

// spec: AccountPage, FollowTag, FollowPoster
func TestAccount_ShowsEmailRoleFollows(t *testing.T) {
	email := uniqEmail(t, "alice")
	c := loginAs(t, email)

	r := c.get("/account")
	assertStatus(t, r, 200)
	assertContains(t, r, email)
	if !containsFold(r.Body, "user") {
		t.Errorf("account page does not name the role")
	}
	assertForm(t, r, `action="/logout"`)
	assertForm(t, r, `action="/account/delete"`, `name="confirm"`)
	// Every tag can be followed from here.
	for _, tag := range []string{"concert", "theater", "film", "exhibition", "talk", "party", "dance", "workshop", "other"} {
		assertForm(t, r, `action="/follow"`, `value="tag"`, `value="`+tag+`"`, `value="1"`)
	}
	assertNotContains(t, r, `href="/p/`+poster2.Slug+`"`)

	assertRedirect(t, follow(c, "tag", "film", "1", "/account"), "/account")
	assertRedirect(t, follow(c, "poster", poster2.Slug, "1", "/account"), "/account")
	r = c.get("/account")
	assertStatus(t, r, 200)
	assertForm(t, r, `action="/follow"`, `value="tag"`, `value="film"`, `value="0"`)
	assertNoForm(t, r, `action="/follow"`, `value="tag"`, `value="film"`, `value="1"`)
	assertContains(t, r, poster2.Name)
	assertContains(t, r, `href="/p/`+poster2.Slug+`"`)
	assertForm(t, r, `action="/follow"`, `value="poster"`, `value="`+poster2.Slug+`"`, `value="0"`)

	// A poster's account page: role, link to own curator page, no delete form.
	r = asPoster(t, poster1).get("/account")
	assertStatus(t, r, 200)
	assertContains(t, r, poster1.Email)
	assertContains(t, r, `href="/p/`+poster1.Slug+`"`)
	assertNoForm(t, r, `action="/account/delete"`)
	if !containsFold(r.Body, "by hand") {
		t.Errorf("poster account page does not say curator accounts are removed by hand")
	}
}

// spec: DeleteAccount, AccountPage, Event.interested_count
func TestDeleteAccount_RemovesDataAndSession(t *testing.T) {
	f := validEvent(t, tomorrow())
	slug := createEvent(t, asPoster(t, poster1), f)
	page := "/e/" + slug
	email := uniqEmail(t, "alice")
	c := loginAs(t, email)
	old := c.cookie("session")
	assertRedirect(t, setInterest(c, slug, "interested", page), page)
	assertRedirect(t, follow(c, "tag", "party", "1", "/account"), "/account")
	assertRedirect(t, follow(c, "poster", poster1.Slug, "1", "/account"), "/account")
	if n := interestedCount(t, anon(t).get(page).Body); n != 1 {
		t.Fatalf("counter = %d, want 1", n)
	}

	r := c.postForm("/account/delete", url.Values{"confirm": {"1"}})
	assertRedirect(t, r, "/")
	if c.cookie("session") != nil {
		t.Errorf("session cookie still in jar after account deletion")
	}
	assertLoginRedirect(t, c.get("/account"), "/account")

	// The session row is gone, not just the cookie.
	replay := anon(t)
	replay.setRawCookie("session", old.Value, "/")
	assertLoginRedirect(t, replay.get("/account"), "/account")

	// Interests cascaded: the public counter drops.
	if n := interestedCount(t, anon(t).get(page).Body); n != 0 {
		t.Errorf("counter after account deletion = %d, want 0", n)
	}

	// Logging in again starts from a clean account.
	again := loginAs(t, email)
	r = again.get("/account")
	assertStatus(t, r, 200)
	assertContains(t, r, email)
	assertNoForm(t, r, `action="/follow"`, `value="tag"`, `value="party"`, `value="0"`)
	assertNotContains(t, r, `href="/p/`+poster1.Slug+`"`)
	assertNotContains(t, again.get("/mine"), f.Title)
	assertNotContains(t, again.get(page), "Hidden from your feed")
}

// spec: DeleteAccount
func TestDeleteAccount_RequiresConfirm(t *testing.T) {
	email := uniqEmail(t, "alice")
	c := loginAs(t, email)
	assertStatus(t, c.postForm("/account/delete", nil), 422)
	assertStatus(t, c.postForm("/account/delete", url.Values{"confirm": {"0"}}), 422)
	assertStatus(t, c.postForm("/account/delete", url.Values{"confirm": {"yes"}}), 422)
	// Still logged in, still there.
	r := c.get("/account")
	assertStatus(t, r, 200)
	assertContains(t, r, email)
	assertStatus(t, c.get("/account/delete"), 405)
}

// spec: DeleteAccount, AccountPage
func TestDeleteAccount_PosterForbidden(t *testing.T) {
	p := asPoster(t, poster2)
	r := p.postForm("/account/delete", url.Values{"confirm": {"1"}})
	assertStatus(t, r, 403)
	if !containsFold(r.Body, "by hand") {
		t.Errorf("403 body does not say curator accounts are removed by hand\nbody: %s", snippet(r.Body))
	}
	// Still a poster, still logged in.
	assertStatus(t, p.get("/new"), 200)
	assertStatus(t, anon(t).get("/p/"+poster2.Slug), 200)
}
