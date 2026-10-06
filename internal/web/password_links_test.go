package web

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/jeffmvr/mangle-vpn/internal/config"
)

// linkCode pulls the code out of a password link.
func linkCode(t *testing.T, link string) string {
	t.Helper()
	u, err := url.Parse(link)
	if err != nil || u.Query().Get("code") == "" {
		t.Fatalf("not a password link: %q", link)
	}
	return u.Query().Get("code")
}

// page fetches a page and returns its status and body.
func (b *browser) page(path string) (int, string) {
	b.ts.t.Helper()
	resp, err := b.client.Get(b.ts.http.URL + path)
	if err != nil {
		b.ts.t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body)
}

// setPassword submits the choose-a-password form for a link code, with a
// name, which an invitation asks for and other links ignore.
func (b *browser) setPassword(code, password string) *http.Response {
	return b.post("/password/set/process", url.Values{
		"code": {code}, "name": {"New Person"}, "password": {password}, "password_confirm": {password},
	}, true)
}

func TestInvitationLinkSetsThePasswordOnce(t *testing.T) {
	ts := newTestServer(t)
	ts.install()
	ts.addUser("admin@example.com", false)

	admin := ts.browser()
	admin.login("admin@example.com", "Password1")
	group, _ := ts.app.Store.Groups.ByName(t.Context(), "NoMFA")

	status, body := admin.do(http.MethodPost, "/api/admin/users",
		`{"email": "new@example.com", "group_id": "`+group.ID.String()+`", "notify": false}`)
	var invited []struct{ Email, Link string }
	json.Unmarshal(body, &invited)
	if status != http.StatusCreated || len(invited) != 1 || invited[0].Email != "new@example.com" {
		t.Fatalf("invite = %d: %s", status, body)
	}
	code := linkCode(t, invited[0].Link)

	// Until a password is chosen, there is none to sign in with.
	user, _ := ts.app.Store.Users.ByEmail(t.Context(), "new@example.com")
	if user.CheckPassword("") || !strings.HasPrefix(user.Password, "!") {
		t.Fatalf("a new account has a usable password: %q", user.Password)
	}

	b := ts.browser()
	if status, page := b.page("/password/set?code=" + code); status != http.StatusOK || !strings.Contains(page, "new@example.com") {
		t.Fatalf("link page = %d, without the address", status)
	}
	if _, page := b.page("/password/set?code=" + code); !strings.Contains(page, `name="name"`) {
		t.Error("accepting an invitation does not ask for a name")
	}

	// An invitation is not accepted without a name.
	resp := b.post("/password/set/process", url.Values{
		"code": {code}, "name": {"  "}, "password": {"Chosen123"}, "password_confirm": {"Chosen123"},
	}, true)
	if loc := resp.Header.Get("Location"); !strings.HasPrefix(loc, "/password/set?code=") {
		t.Fatalf("accepting without a name went to %q, want back to the form", loc)
	}
	if _, page := b.page("/password/set?code=" + code); !strings.Contains(page, "Your full name is required.") {
		t.Error("the form does not say the name is missing")
	}

	if loc := b.setPassword(code, "Chosen123").Header.Get("Location"); loc != "/" {
		t.Fatalf("choosing a password went to %q, want /", loc)
	}
	if user, _ := ts.app.Store.Users.ByEmail(t.Context(), "new@example.com"); user.Name != "New Person" {
		t.Errorf("name = %q, want the one given", user.Name)
	}
	if got := b.get("/api/profile").StatusCode; got != http.StatusOK {
		t.Errorf("profile after choosing a password = %d, want 200", got)
	}

	// The link is spent.
	other := ts.browser()
	if loc := other.setPassword(code, "Another123").Header.Get("Location"); loc != "/password/forgot" {
		t.Errorf("reusing the link went to %q, want /password/forgot", loc)
	}
	if _, page := other.page("/password/set?code=" + code); !strings.Contains(page, invalidLinkMessage) {
		t.Error("a spent link's page does not say so")
	}

	// And the password chosen through it is the one that works.
	fresh := ts.browser()
	fresh.login("new@example.com", "Chosen123")
	if got := fresh.get("/api/profile").StatusCode; got != http.StatusOK {
		t.Errorf("signing in with the chosen password = %d, want 200", got)
	}
}

func TestAdminResetRetiresTheOldPassword(t *testing.T) {
	ts := newTestServer(t)
	ts.install()
	ts.addUser("admin@example.com", false)
	member := ts.addUser("member@example.com", false)

	admin := ts.browser()
	admin.login("admin@example.com", "Password1")
	status, body := admin.do(http.MethodDelete, "/api/admin/users/"+member.ID.String()+"/password", "")
	var reset struct{ Link string }
	json.Unmarshal(body, &reset)
	if status != http.StatusOK {
		t.Fatalf("reset = %d: %s", status, body)
	}

	b := ts.browser()
	b.login("member@example.com", "Password1")
	if got := b.get("/api/profile").StatusCode; got != http.StatusForbidden {
		t.Errorf("the old password after a reset = %d, want 403", got)
	}

	b.setPassword(linkCode(t, reset.Link), "Chosen123")
	if got := b.get("/api/profile").StatusCode; got != http.StatusOK {
		t.Errorf("profile after choosing a password = %d, want 200", got)
	}
}

func TestForgotPasswordSaysTheSameForEveryAddress(t *testing.T) {
	ts := newTestServer(t)
	ts.install()
	ts.addUser("person@example.com", false)
	for name, value := range map[string]string{
		config.SMTPHost: "smtp.example.com", config.SMTPPort: "587",
		config.SMTPUsername: "mailer@example.com", config.SMTPPassword: "secret",
	} {
		ts.app.Config.Set(t.Context(), name, value)
	}

	if _, page := ts.browser().page("/login"); !strings.Contains(page, "/password/forgot") {
		t.Error("the sign in page does not offer a forgotten password link")
	}

	for _, email := range []string{"person@example.com", "nobody@example.com"} {
		resp := ts.browser().post("/password/forgot/process", url.Values{"email": {email}}, true)
		if loc := resp.Header.Get("Location"); loc != "/password/forgot?sent=1" {
			t.Errorf("%s went to %q, want the same page as any other address", email, loc)
		}
	}

	// Only the real account got a link, and asking did not touch its password.
	person, _ := ts.app.Store.Users.ByEmail(t.Context(), "person@example.com")
	var links int
	ts.app.Store.DB().QueryRow(`SELECT COUNT(*) FROM "password_links" WHERE user_id = ?`, person.ID).Scan(&links)
	if links != 1 {
		t.Errorf("person has %d links, want 1", links)
	}
	b := ts.browser()
	b.login("person@example.com", "Password1")
	if got := b.get("/api/profile").StatusCode; got != http.StatusOK {
		t.Errorf("the password after asking for a link = %d, want it to still work", got)
	}
}

func TestExpiredLinkIsRefused(t *testing.T) {
	ts := newTestServer(t)
	ts.install()
	user := ts.addUser("person@example.com", false)

	link, err := ts.app.IssuePasswordLink(t.Context(), user, "forgot")
	if err != nil {
		t.Fatal(err)
	}
	ts.app.Store.DB().Exec(`UPDATE "password_links" SET expires_at = '2000-01-01 00:00:00'`)

	b := ts.browser()
	if loc := b.setPassword(linkCode(t, link), "Chosen123").Header.Get("Location"); loc != "/password/forgot" {
		t.Errorf("an expired link went to %q, want /password/forgot", loc)
	}
}
