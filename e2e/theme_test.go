package e2e

import (
	"net/url"
	"testing"
)

// spec: ChooseTheme, Visitor
func TestTheme_SwitchIsOnEveryPage_AndWorksWithoutScript(t *testing.T) {
	v := anon(t)
	for _, path := range []string{"/", "/upcoming", "/account", "/mine"} {
		r := v.get(path)
		assertStatus(t, r, 200)
		// One toggle: without a stored choice it offers dark.
		assertForm(t, r, `action="/theme"`, `name="theme"`, `value="dark"`)
		assertNotContains(t, r, `value="light"`)
		assertNotContains(t, r, `data-theme=`)
	}

	// Choosing dark sets a cookie and marks the page root; the choice is bold.
	r := v.postForm("/theme", url.Values{"theme": {"dark"}, "back": {"/upcoming"}})
	assertRedirect(t, r, "/upcoming")
	if c := v.cookie("theme"); c == nil || c.Value != "dark" {
		t.Fatalf("theme cookie = %v, want dark", c)
	}
	r = v.get("/")
	assertContains(t, r, `<html lang="en" data-theme="dark">`)
	// Now the toggle offers the way back.
	assertContains(t, r, `value="light" class="linklike" aria-label="Switch to light theme">light</button>`)
	assertNotContains(t, r, `value="dark" class="linklike"`)

	// Light replaces dark.
	assertRedirect(t, v.postForm("/theme", url.Values{"theme": {"light"}, "back": {"/"}}), "/")
	r = v.get("/")
	assertContains(t, r, `data-theme="light"`)

	// Auto clears the choice again.
	assertRedirect(t, v.postForm("/theme", url.Values{"theme": {"auto"}}), "/")
	r = v.get("/")
	assertNotContains(t, r, `data-theme=`)
	if c := v.cookie("theme"); c != nil && c.Value != "" {
		t.Fatalf("theme cookie still %q after auto", c.Value)
	}

	// Anything else is refused.
	assertStatus(t, v.postForm("/theme", url.Values{"theme": {"sepia"}}), 400)

	// No account was created by any of this.
	if v.cookie("session") != nil {
		t.Fatal("choosing a theme must not start an account")
	}
}
