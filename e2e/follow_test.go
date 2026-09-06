package e2e

import (
	"testing"
)

// spec: FollowTag, UnfollowTag, TagFollow, AccountPage, Home, UpcomingAll
func TestFollowTag_FollowUnfollow_Idempotent(t *testing.T) {
	c := newUser(t)
	on := []string{`action="/follow"`, `value="tag"`, `value="theater"`, `value="0"`}
	off := []string{`action="/follow"`, `value="tag"`, `value="theater"`, `value="1"`}

	r := c.get("/account")
	assertStatus(t, r, 200)
	assertForm(t, r, off...)
	assertNoForm(t, r, on...)

	assertRedirect(t, follow(c, "tag", "theater", "1", "/account"), "/account")
	r = c.get("/account")
	assertForm(t, r, on...)
	assertNoForm(t, r, off...)

	// Following twice is a no-op.
	assertRedirect(t, follow(c, "tag", "theater", "1", "/account"), "/account")
	r = c.get("/account")
	assertForm(t, r, on...)
	assertNoForm(t, r, off...)

	// The tag filter shows the toggle for the active tag, on the home page
	// and on the full list alike.
	r = c.get("/?tag=theater")
	assertStatus(t, r, 200)
	assertForm(t, r, on...)
	r = c.get("/upcoming?tag=theater")
	assertStatus(t, r, 200)
	assertForm(t, r, on...)

	assertRedirect(t, follow(c, "tag", "theater", "0", "/account"), "/account")
	r = c.get("/account")
	assertForm(t, r, off...)
	assertNoForm(t, r, on...)

	// Unfollowing twice is a no-op too.
	assertRedirect(t, follow(c, "tag", "theater", "0", "/account"), "/account")
	assertForm(t, c.get("/account"), off...)

	// back that is not a local path lands on /.
	assertRedirect(t, follow(c, "tag", "theater", "1", "https://evil.example/"), "/")
	assertRedirect(t, follow(c, "tag", "theater", "0", ""), "/")
}

// spec: FollowPoster, UnfollowPoster, PosterFollow, PosterPage, AccountPage
func TestFollowPoster_FollowUnfollow(t *testing.T) {
	c := newUser(t)
	page := "/p/" + poster2.Slug
	on := []string{`action="/follow"`, `value="poster"`, `value="` + poster2.Slug + `"`, `value="0"`}
	off := []string{`action="/follow"`, `value="poster"`, `value="` + poster2.Slug + `"`, `value="1"`}

	r := c.get(page)
	assertStatus(t, r, 200)
	assertForm(t, r, off...)
	assertNoForm(t, r, on...)
	assertNotContains(t, c.get("/account"), `href="`+page+`"`)

	assertRedirect(t, follow(c, "poster", poster2.Slug, "1", page), page)
	r = c.get(page)
	assertForm(t, r, on...)
	assertNoForm(t, r, off...)
	r = c.get("/account")
	assertContains(t, r, poster2.Name)
	assertContains(t, r, `href="`+page+`"`)
	assertForm(t, r, on...)

	assertRedirect(t, follow(c, "poster", poster2.Slug, "1", page), page)
	assertForm(t, c.get(page), on...)

	assertRedirect(t, follow(c, "poster", poster2.Slug, "0", page), page)
	r = c.get(page)
	assertForm(t, r, off...)
	assertNoForm(t, r, on...)
	assertNotContains(t, c.get("/account"), `href="`+page+`"`)
	assertRedirect(t, follow(c, "poster", poster2.Slug, "0", page), page)
}

// spec: FollowPoster, StartAccount, PosterPage, Visitor
func TestFollowPoster_AnonStartsAccountFromPosterPage(t *testing.T) {
	page := "/p/" + poster1.Slug
	v := anon(t)
	r := v.get(page)
	assertStatus(t, r, 200)
	// The follow button is offered to a visitor without a session too.
	assertForm(t, r, `action="/follow"`, `value="poster"`, `value="`+poster1.Slug+`"`, `value="1"`)

	r = follow(v, "poster", poster1.Slug, "1", page)
	assertRedirect(t, r, page)
	assertSessionCookieFlags(t, r)
	r = v.get(page)
	assertForm(t, r, `action="/follow"`, `value="poster"`, `value="`+poster1.Slug+`"`, `value="0"`)
	r = v.get("/account")
	assertContains(t, r, `href="`+page+`"`)
	assertContains(t, r, poster1.Name)
}

// spec: FollowTag, FollowPoster, PosterFollowTargetsPoster
func TestFollow_UnknownTargets404(t *testing.T) {
	c := newUser(t)
	assertStatus(t, follow(c, "tag", "opera", "1", "/account"), 404)
	assertStatus(t, follow(c, "tag", "", "1", "/account"), 404)
	assertStatus(t, follow(c, "poster", "nobody-here", "1", "/account"), 404)
	// An event slug is not a poster key either.
	slug := createEvent(t, asPoster(t, poster1), validEvent(t, tomorrow()))
	assertStatus(t, follow(c, "poster", slug, "1", "/account"), 404)
	if r := follow(c, "bogus", "concert", "1", "/account"); r.Status != 400 && r.Status != 404 {
		t.Errorf("kind=bogus: status %d, want 400 or 404", r.Status)
	}
	if r := follow(c, "tag", "concert", "maybe", "/account"); r.Status != 400 && r.Status != 404 {
		t.Errorf("on=maybe: status %d, want 400 or 404", r.Status)
	}
	// Nothing got followed along the way.
	r := c.get("/account")
	assertNoForm(t, r, `action="/follow"`, `value="tag"`, `value="concert"`, `value="0"`)
}

// spec: UpcomingAll, TagFollow, PosterFollow
func TestFollowing_UnionOfTagAndPoster(t *testing.T) {
	p1, p2 := asPoster(t, poster1), asPoster(t, poster2)
	byTag := validEvent(t, tomorrow())
	byTag.Title = uniqTitle(t, "Dance Night")
	byTag.Tags = []string{"theater"}
	byPoster := validEvent(t, tomorrow())
	byPoster.Title = uniqTitle(t, "Nikos Workshop")
	byPoster.Tags = []string{"exhibition"}
	neither := validEvent(t, tomorrow())
	neither.Title = uniqTitle(t, "Some Talk")
	neither.Tags = []string{"film"}
	pastDance := validEvent(t, yesterday())
	pastDance.Title = uniqTitle(t, "Old Dance")
	pastDance.Tags = []string{"theater"}
	createEvent(t, p1, byTag)
	createEvent(t, p2, byPoster)
	createEvent(t, p1, neither)
	createEvent(t, p1, pastDance)

	c := newUser(t)
	assertRedirect(t, follow(c, "tag", "theater", "1", "/following"), "/following")
	assertRedirect(t, follow(c, "poster", poster2.Slug, "1", "/following"), "/following")

	r := c.get("/following")
	assertStatus(t, r, 200)
	assertContains(t, r, byTag.Title)
	assertContains(t, r, byPoster.Title)
	assertNotContains(t, r, neither.Title)
	assertNotContains(t, r, pastDance.Title)

	// The full list is unfiltered.
	r = c.get("/upcoming")
	assertContains(t, r, byTag.Title)
	assertContains(t, r, byPoster.Title)
	assertContains(t, r, neither.Title)

	// Unfollowing the tag removes only the tag-matched event.
	assertRedirect(t, follow(c, "tag", "theater", "0", "/following"), "/following")
	r = c.get("/following")
	assertNotContains(t, r, byTag.Title)
	assertContains(t, r, byPoster.Title)
}

// spec: UpcomingAll
func TestFollowing_EmptyState(t *testing.T) {
	f := validEvent(t, tomorrow())
	createEvent(t, asPoster(t, poster1), f)
	c := newUser(t)
	r := c.get("/following")
	assertStatus(t, r, 200)
	assertNotContains(t, r, f.Title)
	assertCopy(t, r, copyFollowEmpty)
	assertContains(t, r, `href="/`)
}

// spec: UpcomingAll, Home, Visitor
func TestFollowing_Anon200Empty(t *testing.T) {
	f := validEvent(t, tomorrow())
	createEvent(t, asPoster(t, poster1), f)
	v := anon(t)
	r := v.get("/following")
	assertStatus(t, r, 200)
	assertNotContains(t, r, f.Title)
	assertCopy(t, r, copyFollowEmpty)
	assertNotContains(t, r, `href="/login`)
	if v.cookie("session") != nil {
		t.Errorf("GET /following set a session cookie")
	}
	// The filter link is offered on every list page for everyone.
	for _, path := range []string{"/", "/?tag=concert", "/upcoming", "/upcoming?tag=concert", "/following"} {
		assertContains(t, v.get(path), `href="/following"`)
	}
}
