package router_test

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain/service/authsvc"
	authmocks "github.com/ceffo/critical/api/internal/domain/service/authsvc/mocks"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/rest/handler/webhdl"
	mw "github.com/ceffo/critical/api/internal/rest/middleware"
	"github.com/ceffo/critical/api/internal/rest/router"
)

// anon sends a request without a session cookie, with extra headers.
func anon(h http.Handler, method, path string, headers ...string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(""))
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// gated is the full router (every handler on mocks that fail any call) with a
// web build and the signedIn auth service.
func gated(t *testing.T) http.Handler {
	t.Helper()
	web, err := webhdl.New(webBuild(t))
	require.NoError(t, err)
	return webRouter(t, web)
}

const unauthenticated = `{"code":"unauthenticated","error":"authentication required"}`

// ADR 0029 step 6.2: without a session a data route and the catalogue answer
// 401 (the service mocks would fail on any call).
func TestGate_Step6_2_NoSessionIs401(t *testing.T) {
	h, _ := newRouter(t)
	for _, p := range []string{"/api/character", "/api/spells?q=armor", "/api/spells/names", "/api/nope"} {
		rec := anon(h, "GET", p)
		assert.Equal(t, http.StatusUnauthorized, rec.Code, p)
		assert.JSONEq(t, unauthenticated, rec.Body.String(), p)
	}
	rec := anon(h, "POST", "/api/roll/attack", "Content-Type", "application/json")
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// Step 6.2: a path that only looks public is judged once cleaned: dot
// segments out of /assets/ (plain or with an escaped slash) never reach an
// /api handler without a session (the service mocks fail on any call) nor
// return its data.
func TestGate_Step6_2_DotSegmentsOutOfAssetsAreGated(t *testing.T) {
	web := gated(t)
	for _, target := range []string{"/assets/../api/character", "/assets/..%2fapi/character", "/assets/%2e%2e/api/character"} {
		// As the server's request parser leaves it: dot segments kept, the
		// escaped slash in RawPath.
		u, err := url.Parse(target)
		require.NoError(t, err)
		req := httptest.NewRequest("GET", "/", nil)
		req.URL, req.RequestURI = u, target
		rec := httptest.NewRecorder()
		web.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code, target)
		assert.JSONEq(t, unauthenticated, rec.Body.String(), target)
	}
}

// Step 6.2: a page opened without a session goes to the login page.
func TestGate_Step6_2_PageRedirectsToLogin(t *testing.T) {
	web := gated(t)
	for _, p := range []string{"/", "/spells/42", "/index.html"} {
		rec := anon(web, "GET", p, "Accept", "text/html,application/xhtml+xml")
		assert.Equal(t, http.StatusFound, rec.Code, p)
		assert.Equal(t, mw.LoginPath, rec.Header().Get("Location"), p)
	}
}

// Step 6.1: the login page (and what is under it), /api/auth/* and
// /api/health are served without a session; health carries no data.
func TestGate_Step6_1_PublicPathsReachable(t *testing.T) {
	web := gated(t)
	login := anon(web, "GET", "/login", "Accept", "text/html")
	require.Equal(t, http.StatusOK, login.Code)
	assert.Contains(t, login.Body.String(), "<title>critical</title>")
	assert.Equal(t, http.StatusOK, anon(web, "GET", "/login/whatever").Code)
	assert.Equal(t, http.StatusOK, anon(web, "GET", "/login?error=not_invited", "Accept", "text/html").Code)

	// The static files the login page loads: the bundle and the favicon.
	asset := anon(web, "GET", "/assets/index-a1b2.js")
	require.Equal(t, http.StatusOK, asset.Code)
	assert.Equal(t, "console.log(1)", asset.Body.String())
	assert.Equal(t, http.StatusOK, anon(web, "GET", "/favicon.svg").Code)

	health := anon(web, "GET", "/api/health")
	require.Equal(t, http.StatusOK, health.Code)
	assert.JSONEq(t, `{"status":"ok","commit":"abc123","version":"1.2.3"}`, health.Body.String(),
		"the server's build only: nothing about users or characters")

	// /api/auth/me is reached: its own answer, a 401 with no session.
	me := anon(web, "GET", "/api/auth/me")
	assert.Equal(t, http.StatusUnauthorized, me.Code)

	// The dev login is reached without a session.
	a := authmocks.NewMockSVC(t)
	a.EXPECT().DevLoginEnabled().Return(true).Once()
	a.EXPECT().DevLogin(mock.Anything, "").Return(&authsvc.NewSession{
		Token: "tok", Expires: time.Now().Add(time.Hour), User: authsvc.User{ID: uuid.Nil, Email: "dev@localhost", Role: authsvc.RoleAdmin, Root: true},
	}, nil).Once()
	h := authRouter(t, a, router.Options{})
	dev := anon(h, "POST", "/api/auth/dev")
	require.Equal(t, http.StatusOK, dev.Code)
	assert.JSONEq(t, `{"id":"00000000-0000-0000-0000-000000000000","email":"dev@localhost","role":"admin","root":true}`, dev.Body.String())
	c := sessionCookie(t, dev)
	assert.Equal(t, "tok", c.Value)
}

func authRouter(t *testing.T, a authsvc.SVC, opts router.Options) http.Handler {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return router.New(log, &composition.BackingServices{CharacterID: 1, Auth: a}, opts)
}

func sessionCookie(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == mw.SessionCookie {
			return c
		}
	}
	t.Fatalf("no %s cookie in %v", mw.SessionCookie, rec.Header().Values("Set-Cookie"))
	return nil
}

// Step 4.2: the session cookie is HttpOnly, SameSite=Lax, path /, host-only,
// and Secure when the app is on HTTPS.
func TestSessionCookie_Step4_2_Attributes(t *testing.T) {
	for _, secure := range []bool{false, true} {
		a := authmocks.NewMockSVC(t)
		a.EXPECT().DevLoginEnabled().Return(true).Once()
		a.EXPECT().DevLogin(mock.Anything, "e2e@example.com").Return(&authsvc.NewSession{
			Token: "tok", Expires: time.Now().Add(authsvc.SessionLifetime), User: authsvc.User{Role: authsvc.RoleAdmin},
		}, nil).Once()
		h := authRouter(t, a, router.Options{SecureCookies: secure})
		req := httptest.NewRequest("POST", "/api/auth/dev", strings.NewReader(`{"email":"e2e@example.com"}`))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
		c := sessionCookie(t, rec)
		assert.True(t, c.HttpOnly)
		assert.Equal(t, http.SameSiteLaxMode, c.SameSite)
		assert.Equal(t, "/", c.Path)
		assert.Empty(t, c.Domain)
		assert.Equal(t, secure, c.Secure)
		assert.Greater(t, c.MaxAge, 29*24*3600)
	}
}

// Step 8.2: with OAuth settings there is no dev login route.
func TestDevLogin_Step8_2_AbsentWithOAuth(t *testing.T) {
	a := authmocks.NewMockSVC(t)
	a.EXPECT().DevLoginEnabled().Return(false).Once()
	h := authRouter(t, a, router.Options{})
	rec := anon(h, "POST", "/api/auth/dev")
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// Step 4.4: an unknown or expired session is refused and its cookie
// cleared.
func TestGate_Step4_4_DeadSessionRefusedAndCookieCleared(t *testing.T) {
	h, _ := newRouter(t)
	req := httptest.NewRequest("GET", "/api/character", nil)
	req.AddCookie(&http.Cookie{Name: mw.SessionCookie, Value: "expired"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	c := sessionCookie(t, rec)
	assert.Negative(t, c.MaxAge)
}

// Step 4.4: a page opened with a session the server no longer knows goes to
// the login page saying it has ended, and /api/auth/me answers
// session_expired for it (unauthenticated without a cookie), so the login
// page can tell the two apart.
func TestGate_Step4_4_DeadSessionSaysItEnded(t *testing.T) {
	web := gated(t)
	for _, p := range []string{"/", "/spells/42"} {
		req := httptest.NewRequest("GET", p, nil)
		req.Header.Set("Accept", "text/html")
		req.AddCookie(&http.Cookie{Name: mw.SessionCookie, Value: "expired"})
		rec := httptest.NewRecorder()
		web.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusFound, rec.Code, p)
		assert.Equal(t, "/login?error=session_expired", rec.Header().Get("Location"), p)
	}

	req := httptest.NewRequest("GET", "/api/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: mw.SessionCookie, Value: "expired"})
	rec := httptest.NewRecorder()
	web.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.JSONEq(t, `{"code":"session_expired","error":"session expired: authentication required"}`, rec.Body.String())
	assert.Negative(t, sessionCookie(t, rec).MaxAge)

	me := anon(web, "GET", "/api/auth/me")
	assert.Equal(t, http.StatusUnauthorized, me.Code)
	assert.JSONEq(t, unauthenticated, me.Body.String())

	// A data route keeps the gate's own code with a dead session.
	req = httptest.NewRequest("GET", "/api/character", nil)
	req.AddCookie(&http.Cookie{Name: mw.SessionCookie, Value: "expired"})
	rec = httptest.NewRecorder()
	web.ServeHTTP(rec, req)
	assert.JSONEq(t, unauthenticated, rec.Body.String())
}

// Step 8: the login page offers the server's login methods, public: the
// dev login without OAuth settings, Google with them.
func TestLoginOptions_Step8_OneMethodPerMode(t *testing.T) {
	for _, tc := range []struct {
		dev  bool
		want string
	}{
		{dev: true, want: `{"google":false,"devLogin":true}`},
		{dev: false, want: `{"google":true,"devLogin":false}`},
	} {
		a := authmocks.NewMockSVC(t)
		a.EXPECT().DevLoginEnabled().Return(tc.dev).Times(2)
		h := authRouter(t, a, router.Options{})
		rec := anon(h, "GET", "/api/auth/options")
		require.Equal(t, http.StatusOK, rec.Code)
		assert.JSONEq(t, tc.want, rec.Body.String())
	}
}

// Step 4.3: a request that extends the session sets the cookie again with
// the new expiry.
func TestGate_Step4_3_ExtendedSessionRefreshesTheCookie(t *testing.T) {
	expires := time.Now().Add(authsvc.SessionLifetime)
	a := authmocks.NewMockSVC(t)
	a.EXPECT().DevLoginEnabled().Return(true).Once()
	a.EXPECT().Authenticate(mock.Anything, "tok").Return(&authsvc.Principal{
		User: authsvc.User{Email: "ann@example.com", Role: authsvc.RolePlayer}, Expires: expires, Extended: true,
	}, nil).Once()
	h := authRouter(t, a, router.Options{})
	req := httptest.NewRequest("GET", "/api/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: mw.SessionCookie, Value: "tok"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"id":"00000000-0000-0000-0000-000000000000","email":"ann@example.com","role":"player","root":false}`, rec.Body.String())
	c := sessionCookie(t, rec)
	assert.Equal(t, "tok", c.Value)
	assert.Greater(t, c.MaxAge, 29*24*3600)
}

// Step 4.5: a state-changing request whose Origin (or Referer) names another
// host is refused before anything runs; the app's own host passes, and so
// does a request with neither header (not a browser's).
func TestGate_Step4_5_CrossOriginMutationRefused(t *testing.T) {
	h, m := newRouter(t)
	refused := []string{"Origin", "https://evil.example", "Referer", "https://evil.example/page"}
	for i := 0; i < len(refused); i += 2 {
		req := httptest.NewRequest("POST", "/api/round/advance", nil)
		req.Header.Set(refused[i], refused[i+1])
		req.AddCookie(&http.Cookie{Name: mw.SessionCookie, Value: testSession})
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code, refused[i])
		assert.JSONEq(t, `{"code":"cross_origin","error":"cross-origin request refused"}`, rec.Body.String())
	}

	m.buffs.On("AdvanceRound", anyArgs(2)...).Return(nil, nil, 0, buffsvc.Initiative{}, errors.New("reached")).Twice()
	for _, origin := range []string{"http://example.com", ""} {
		req := httptest.NewRequest("POST", "/api/round/advance", nil) // Host: example.com
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		req.AddCookie(&http.Cookie{Name: mw.SessionCookie, Value: testSession})
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		assert.NotEqual(t, http.StatusForbidden, rec.Code, "origin %q", origin)
	}
}

// Step 4.5 with a public URL: the app's host is the configured one.
func TestGate_Step4_5_ConfiguredHost(t *testing.T) {
	a := authmocks.NewMockSVC(t)
	a.EXPECT().DevLoginEnabled().Return(false).Once()
	a.EXPECT().Logout(mock.Anything, "tok").Return(nil).Once()
	h := authRouter(t, a, router.Options{Host: "critical-hit.fly.dev"})
	send := func(origin string) int {
		req := httptest.NewRequest("POST", "/api/auth/logout", nil)
		req.Header.Set("Origin", origin)
		req.AddCookie(&http.Cookie{Name: mw.SessionCookie, Value: "tok"})
		a.EXPECT().Authenticate(mock.Anything, "tok").Return(&authsvc.Principal{}, nil).Maybe()
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}
	assert.Equal(t, http.StatusForbidden, send("http://example.com"), "the request's own Host is not the app's")
	assert.Equal(t, http.StatusOK, send("https://critical-hit.fly.dev"))
}

// Step 6.2: a router without an auth service authenticates nobody: it fails
// closed.
func TestGate_Step6_2_NoAuthServiceFailsClosed(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := router.New(log, &composition.BackingServices{CharacterID: 1}, router.Options{})
	assert.Equal(t, http.StatusUnauthorized, do(h, "GET", "/api/character", "").Code)
	assert.Equal(t, http.StatusOK, do(h, "GET", "/api/health", "").Code)
}

// Step 3.1: the login start keeps the state and the PKCE verifier in a
// short-lived HttpOnly cookie on the login's paths, and sends the browser to
// the provider.
func TestGoogleStart_Step3_1_KeepsStateInACookie(t *testing.T) {
	a := authmocks.NewMockSVC(t)
	a.EXPECT().DevLoginEnabled().Return(false).Once()
	a.EXPECT().StartLogin(mock.Anything).Return(&authsvc.LoginStart{URL: "https://accounts.example/auth?state=st", State: "st", Verifier: "ver"}, nil).Once()
	h := authRouter(t, a, router.Options{SecureCookies: true})
	rec := anon(h, "GET", "/api/auth/google/start")
	require.Equal(t, http.StatusFound, rec.Code)
	assert.Equal(t, "https://accounts.example/auth?state=st", rec.Header().Get("Location"))
	cookies := rec.Result().Cookies()
	require.Len(t, cookies, 1)
	c := cookies[0]
	assert.Equal(t, "critical_login", c.Name)
	assert.Equal(t, "st.ver", c.Value)
	assert.Equal(t, "/api/auth/google", c.Path)
	assert.Equal(t, 600, c.MaxAge)
	assert.True(t, c.HttpOnly)
	assert.True(t, c.Secure)
}

// Steps 3.1 and 3.7: the callback hands the cookie's state and verifier with
// the query to the service, drops the login cookie, and on success sets the
// session and goes to the app; a refusal goes to the login page with its
// stable code and no session.
func TestGoogleCallback_Step3_7_SessionOrRefusalCode(t *testing.T) {
	in := authsvc.FinishLogin{Code: "c", State: "st", ExpectedState: "st", Verifier: "ver"}
	cases := []struct {
		name     string
		err      error
		location string
	}{
		{"login", nil, "/"},
		{"not invited", authsvc.ErrNotInvited, "/login?error=not_invited"},
		{"disabled", authsvc.ErrUserDisabled, "/login?error=user_disabled"},
		{"bad state", authsvc.ErrStateMismatch, "/login?error=login_state_mismatch"},
		{"unverified", authsvc.ErrEmailUnverified, "/login?error=email_unverified"},
		{"failed", authsvc.ErrLoginFailed, "/login?error=login_failed"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := authmocks.NewMockSVC(t)
			a.EXPECT().DevLoginEnabled().Return(false).Once()
			var s *authsvc.NewSession
			if c.err == nil {
				s = &authsvc.NewSession{Token: "tok", Expires: time.Now().Add(time.Hour)}
			}
			a.EXPECT().FinishLogin(mock.Anything, in).Return(s, c.err).Once()
			h := authRouter(t, a, router.Options{})
			req := httptest.NewRequest("GET", "/api/auth/google/callback?code=c&state=st", nil)
			req.AddCookie(&http.Cookie{Name: "critical_login", Value: "st.ver"})
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			require.Equal(t, http.StatusFound, rec.Code)
			assert.Equal(t, c.location, rec.Header().Get("Location"))
			var gotSession, cleared bool
			for _, ck := range rec.Result().Cookies() {
				gotSession = gotSession || (ck.Name == mw.SessionCookie && ck.Value == "tok")
				cleared = cleared || (ck.Name == "critical_login" && ck.MaxAge < 0)
			}
			assert.Equal(t, c.err == nil, gotSession)
			assert.True(t, cleared, "the login cookie is dropped")
		})
	}
}

// Step 4.4: logout deletes the session and the cookie.
func TestLogout_Step4_4_DeletesSessionAndCookie(t *testing.T) {
	a := authmocks.NewMockSVC(t)
	a.EXPECT().DevLoginEnabled().Return(false).Once()
	a.EXPECT().Authenticate(mock.Anything, "tok").Return(&authsvc.Principal{}, nil).Once()
	a.EXPECT().Logout(mock.Anything, "tok").Return(nil).Once()
	h := authRouter(t, a, router.Options{})
	req := httptest.NewRequest("POST", "/api/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: mw.SessionCookie, Value: "tok"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Negative(t, sessionCookie(t, rec).MaxAge)
}

// The auth cookies are named per app (mw.SessionCookieName): browsers send a
// cookie to every port of a host, so `just dev` and a hosted app tunnelled to
// another localhost port must not read, clear or overwrite each other's
// session, and one app's token never reaches the other's server.
func TestSessionCookie_NamedPerApp(t *testing.T) {
	const own = "critical_session_localhost_8091"
	assert.Equal(t, own, mw.SessionCookieName("localhost:8091"))
	assert.Equal(t, "critical_session_critical-hit.fly.dev", mw.SessionCookieName("critical-hit.fly.dev"))

	a := authmocks.NewMockSVC(t)
	a.EXPECT().DevLoginEnabled().Return(true).Once()
	a.EXPECT().Authenticate(mock.Anything, "mine").Return(&authsvc.Principal{
		User: authsvc.User{Email: "ann@example.com", Role: authsvc.RolePlayer},
	}, nil).Once()
	a.EXPECT().DevLogin(mock.Anything, "").Return(&authsvc.NewSession{Token: "new", Expires: time.Now().Add(time.Hour)}, nil).Once()
	a.EXPECT().StartLogin(mock.Anything).Return(&authsvc.LoginStart{URL: "https://accounts.example/auth", State: "st", Verifier: "ver"}, nil).Once()
	h := authRouter(t, a, router.Options{CookieApp: "localhost:8091"})

	// Another app's cookie (the dev stack on 5173, the base name) is not this
	// app's: never looked up (the mock refuses any other token), never cleared.
	req := httptest.NewRequest("GET", "/api/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: "critical_session_dev_8080", Value: "theirs"})
	req.AddCookie(&http.Cookie{Name: mw.SessionCookie, Value: "old"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Empty(t, rec.Header().Values("Set-Cookie"), "another app's cookie is left alone")

	req = httptest.NewRequest("GET", "/api/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: "critical_session_dev_8080", Value: "theirs"})
	req.AddCookie(&http.Cookie{Name: own, Value: "mine"})
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	rec = anon(h, "POST", "/api/auth/dev")
	require.Equal(t, http.StatusOK, rec.Code)
	cookies := rec.Result().Cookies()
	require.Len(t, cookies, 1)
	assert.Equal(t, own, cookies[0].Name)
	assert.Equal(t, "new", cookies[0].Value)

	rec = anon(h, "GET", "/api/auth/google/start")
	cookies = rec.Result().Cookies()
	require.Len(t, cookies, 1)
	assert.Equal(t, "critical_login_localhost_8091", cookies[0].Name)
}
