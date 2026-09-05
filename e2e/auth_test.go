package e2e

import (
	"net/url"
	"strings"
	"testing"
	"time"
)

// wrongCode returns a six-digit code different from real.
func wrongCode(real string) string {
	if real == "000000" {
		return "111111"
	}
	return "000000"
}

// sessionSetCookie returns the Set-Cookie header for the session cookie, or "".
func sessionSetCookie(r resp) string {
	for _, sc := range r.Header.Values("Set-Cookie") {
		if strings.HasPrefix(sc, "session=") {
			return sc
		}
	}
	return ""
}

// spec: RequestCode, Login
func TestLogin_RequestCode_WritesOtpLine(t *testing.T) {
	email := uniqEmail(t, "alice")
	c := anon(t)

	r := c.get("/login")
	assertStatus(t, r, 200)
	assertContains(t, r, `name="email"`)
	assertContains(t, r, `name="next"`)

	// The code step without an otp_email cookie goes back to the email step.
	assertRedirectPath(t, anon(t).get("/login/code"), "/login")

	r = requestCode(c, email, "")
	assertRedirectPath(t, r, "/login/code")
	if c.cookie("otp_email") == nil {
		t.Fatalf("no otp_email cookie after POST /login")
	}
	code := readOTP(t, shared, email)
	if len(code) != 6 || strings.Trim(code, "0123456789") != "" {
		t.Errorf("OTP code = %q, want six digits", code)
	}

	r = c.follow(r)
	assertStatus(t, r, 200)
	assertContains(t, r, email)
	assertContains(t, r, `name="code"`)
	assertContains(t, r, "one-time-code")
}

// spec: RequestCode
func TestLogin_RequestCode_InvalidEmail422(t *testing.T) {
	c := anon(t)
	for _, bad := range []string{"notanemail", "", "   ", "a@b", "@example.test", "x@"} {
		r := requestCode(c, bad, "")
		if r.Status != 422 {
			t.Errorf("email %q: status %d, want 422", bad, r.Status)
			continue
		}
		assertContains(t, r, "Enter a valid email address.")
		if c.cookie("otp_email") != nil {
			t.Errorf("email %q: otp_email cookie set on 422", bad)
		}
	}
	if n := otpLineCount(shared, "notanemail"); n != 0 {
		t.Errorf("OTP file has %d lines for an invalid email", n)
	}
}

// spec: VerifyCodeWrong
func TestLogin_Verify_WrongCode422_NoCookie(t *testing.T) {
	email := uniqEmail(t, "alice")
	c := anon(t)
	assertRedirectPath(t, requestCode(c, email, ""), "/login/code")
	code := readOTP(t, shared, email)

	r := verifyCode(c, wrongCode(code), "")
	assertStatus(t, r, 422)
	assertContains(t, r, "didn't match")
	if c.cookie("session") != nil {
		t.Fatalf("session cookie set after a wrong code")
	}
	if sessionSetCookie(r) != "" {
		t.Fatalf("Set-Cookie session on 422: %q", sessionSetCookie(r))
	}
	// The visitor is still anonymous.
	assertLoginRedirect(t, c.get("/account"), "/account")

	// The right code still works after one wrong attempt.
	r = verifyCode(c, code, "")
	assertRedirect(t, r, "/")
}

// spec: VerifyCodeNewAccount, Login
func TestLogin_Verify_Success_SetsCookie(t *testing.T) {
	email := uniqEmail(t, "alice")
	c := anon(t)
	assertRedirectPath(t, requestCode(c, email, ""), "/login/code")
	code := readOTP(t, shared, email)

	r := verifyCode(c, code, "")
	assertRedirect(t, r, "/")
	sc := sessionSetCookie(r)
	if sc == "" {
		t.Fatalf("no Set-Cookie for session; headers: %v", r.Header)
	}
	for _, want := range []string{"HttpOnly", "SameSite=Lax", "Path=/"} {
		if !strings.Contains(sc, want) {
			t.Errorf("Set-Cookie %q lacks %s", sc, want)
		}
	}
	if c.cookie("session") == nil {
		t.Fatalf("jar has no session cookie")
	}
	if c.cookie("otp_email") != nil {
		t.Errorf("otp_email cookie still present after login")
	}

	r = c.get("/account")
	assertStatus(t, r, 200)
	assertContains(t, r, email)
}

// spec: VerifyCodeNewAccount, OtpCode
func TestLogin_Verify_CodeSingleUse(t *testing.T) {
	email := uniqEmail(t, "alice")
	a := anon(t)
	assertRedirectPath(t, requestCode(a, email, ""), "/login/code")
	code := readOTP(t, shared, email)

	// A second browser holding the same otp_email cookie.
	b := anon(t)
	otp := a.cookie("otp_email")
	if otp == nil {
		t.Fatalf("no otp_email cookie")
	}
	b.setRawCookie("otp_email", otp.Value, "/login")

	assertRedirect(t, verifyCode(a, code, ""), "/")

	r := verifyCode(b, code, "")
	assertStatus(t, r, 422)
	if b.cookie("session") != nil {
		t.Fatalf("a used code produced a session")
	}
}

// spec: VerifyCodeWrong, OtpCode
func TestLogin_Verify_AttemptsLimitExhaustsCode(t *testing.T) {
	s := startServer(t, serverOpts{env: map[string]string{"OTP_MAX_ATTEMPTS": "3"}})
	email := uniqEmail(t, "alice")
	c := newClient(t, s)
	assertRedirectPath(t, requestCode(c, email, ""), "/login/code")
	code := readOTP(t, s, email)
	bad := wrongCode(code)

	for i := 1; i <= 3; i++ {
		r := verifyCode(c, bad, "")
		if r.Status != 422 {
			t.Fatalf("wrong attempt %d: status %d, want 422", i, r.Status)
		}
	}
	// The code is exhausted: even the right one is refused.
	r := verifyCode(c, code, "")
	assertStatus(t, r, 422)
	assertContains(t, r, "Too many attempts")
	if c.cookie("session") != nil {
		t.Fatalf("exhausted code produced a session")
	}

	// A new request issues a fresh code that works.
	assertRedirectPath(t, requestCode(c, email, ""), "/login/code")
	code2 := waitOTP(t, s, email, 2)
	assertRedirect(t, verifyCode(c, code2, ""), "/")
}

// spec: CodeExpires, VerifyCodeWrong
func TestLogin_Verify_CodeExpires(t *testing.T) {
	s := startServer(t, serverOpts{env: map[string]string{"OTP_TTL": "1s"}})
	email := uniqEmail(t, "alice")
	c := newClient(t, s)
	assertRedirectPath(t, requestCode(c, email, ""), "/login/code")
	code := readOTP(t, s, email)

	time.Sleep(1500 * time.Millisecond)
	r := verifyCode(c, code, "")
	assertStatus(t, r, 422)
	assertContains(t, r, "expired")
	if c.cookie("session") != nil {
		t.Fatalf("expired code produced a session")
	}
}

// spec: RequestCode, OnePendingCodePerEmail
func TestLogin_NewRequestSupersedesPrevious(t *testing.T) {
	email := uniqEmail(t, "alice")
	c := anon(t)
	assertRedirectPath(t, requestCode(c, email, ""), "/login/code")
	code1 := waitOTP(t, shared, email, 1)
	assertRedirectPath(t, requestCode(c, email, ""), "/login/code")
	code2 := waitOTP(t, shared, email, 2)
	if code1 == code2 {
		t.Skip("both random codes are identical; nothing to distinguish")
	}

	r := verifyCode(c, code1, "")
	assertStatus(t, r, 422)
	if c.cookie("session") != nil {
		t.Fatalf("superseded code produced a session")
	}
	assertRedirect(t, verifyCode(c, code2, ""), "/")
}

// spec: RequestCode, UniqueAccountEmail, AccountPage
func TestLogin_EmailNormalised(t *testing.T) {
	lower := uniqEmail(t, "alice")
	local, domain, _ := strings.Cut(lower, "@")
	fancy := "  " + strings.ToUpper(local[:1]) + local[1:] + "@" + strings.ToUpper(domain) + "  "

	a := loginAs(t, fancy)
	r := a.get("/account")
	assertStatus(t, r, 200)
	assertContains(t, r, lower)
	assertRedirect(t, follow(a, "tag", "film", "1", "/account"), "/account")

	b := loginAs(t, lower)
	r = b.get("/account")
	assertStatus(t, r, 200)
	assertContains(t, r, lower)
	assertForm(t, r, `action="/follow"`, `value="tag"`, `value="film"`, `value="0"`)
}

// spec: VerifyCodeExistingAccount, Logout, AccountPage
func TestLogin_ExistingAccountKeepsData(t *testing.T) {
	email := uniqEmail(t, "alice")
	a := loginAs(t, email)
	assertRedirect(t, follow(a, "tag", "dance", "1", "/account"), "/account")
	assertRedirect(t, a.postForm("/logout", nil), "/")

	b := loginAs(t, email)
	r := b.get("/account")
	assertStatus(t, r, 200)
	assertContains(t, r, email)
	assertForm(t, r, `action="/follow"`, `value="tag"`, `value="dance"`, `value="0"`)
}

// spec: VerifyCodeExistingAccount, Poster, EventComposer, AccountPage
func TestLogin_PosterSameFlow(t *testing.T) {
	p := loginAs(t, poster1.Email)
	assertStatus(t, p.get("/new"), 200)
	r := p.get("/account")
	assertStatus(t, r, 200)
	assertContains(t, r, poster1.Email)
	assertContains(t, r, `href="/p/`+poster1.Slug+`"`)
}

// spec: Login, VerifyCodeNewAccount
func TestLogin_NextParam_SafeOnly(t *testing.T) {
	// A local path is honoured end to end.
	email := uniqEmail(t, "alice")
	c := anon(t)
	r := requestCode(c, email, "/mine")
	assertRedirectPath(t, r, "/login/code")
	if u, _ := url.Parse(r.Location); u.Query().Get("next") != "/mine" {
		t.Errorf("Location %q does not carry next=/mine", r.Location)
	}
	r = c.follow(r)
	assertStatus(t, r, 200)
	assertContains(t, r, `value="/mine"`)
	assertRedirect(t, verifyCode(c, readOTP(t, shared, email), "/mine"), "/mine")

	// Anything that is not a local path lands on the feed.
	for _, evil := range []string{"https://evil.example/", "//evil.example/x", "evil.example"} {
		email := uniqEmail(t, "eve")
		c := anon(t)
		assertRedirectPath(t, requestCode(c, email, evil), "/login/code")
		r := verifyCode(c, readOTP(t, shared, email), evil)
		if r.Status != 303 || r.Location != "/" {
			t.Errorf("next=%q: got %d → %q, want 303 → /", evil, r.Status, r.Location)
		}
	}

	// GET /login keeps next in a hidden field.
	r = anon(t).get("/login?next=/mine")
	assertStatus(t, r, 200)
	assertContains(t, r, `name="next"`)
	assertContains(t, r, `value="/mine"`)
}

// spec: Login
func TestLogin_AlreadyLoggedInRedirects(t *testing.T) {
	c := loginAs(t, uniqEmail(t, "alice"))
	assertRedirect(t, c.get("/login"), "/")
}

// spec: RequestCode
func TestLogin_RateLimited(t *testing.T) {
	email := uniqEmail(t, "alice")
	c := anon(t)
	for i := 1; i <= 3; i++ {
		r := requestCode(c, email, "")
		if r.Status != 303 {
			t.Fatalf("request %d: status %d, want 303", i, r.Status)
		}
	}
	waitOTP(t, shared, email, 3)
	r := requestCode(c, email, "")
	assertStatus(t, r, 422)
	if n := otpLineCount(shared, email); n != 3 {
		t.Errorf("OTP lines = %d after the rate limit hit, want 3", n)
	}
	// The last issued code still logs in.
	assertRedirect(t, verifyCode(c, readOTP(t, shared, email), ""), "/")
}

// spec: Logout, Session, AccountPage
func TestLogout_RevokesSessionServerSide(t *testing.T) {
	c := loginAs(t, uniqEmail(t, "alice"))
	old := c.cookie("session")
	if old == nil {
		t.Fatalf("no session cookie")
	}
	assertStatus(t, c.get("/logout"), 405)

	r := c.postForm("/logout", nil)
	assertRedirect(t, r, "/")
	if c.cookie("session") != nil {
		t.Errorf("session cookie still in jar after logout")
	}
	assertLoginRedirect(t, c.get("/account"), "/account")

	// Replaying the old token must not work: the row is gone.
	replay := anon(t)
	replay.setRawCookie("session", old.Value, "/")
	assertLoginRedirect(t, replay.get("/account"), "/account")
	assertLoginRedirect(t, replay.get("/mine"), "/mine")
}
