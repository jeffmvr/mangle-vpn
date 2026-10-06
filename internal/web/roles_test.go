package web

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/jeffmvr/mangle-vpn/internal/model"
)

// addMember stores an active user with the given role who is not an
// administrator.
func (ts *testServer) addMember(email, role string) *model.User {
	ts.t.Helper()
	user := ts.addUser(email, false)
	user.IsAdmin, user.Role = false, role
	if err := ts.app.Store.Users.Save(ts.t.Context(), user); err != nil {
		ts.t.Fatal(err)
	}
	return user
}

func TestHelpDeskHelpsMembersButChangesNothingElse(t *testing.T) {
	ts := newTestServer(t)
	ts.install()
	admin := ts.addUser("admin@example.com", false)
	ts.addMember("desk@example.com", model.RoleHelpDesk)
	member := ts.addMember("member@example.com", "")

	// The member has locked themselves out.
	until := time.Now().Add(time.Hour)
	ts.app.Store.DB().Exec(`UPDATE "users" SET locked_until = ? WHERE id = ?`,
		until.UTC().Format("2006-01-02 15:04:05"), member.ID)

	desk := ts.browser()
	desk.login("desk@example.com", "Password1")

	for _, tc := range []struct {
		method, path, body string
		want               int
	}{
		// Looking is allowed.
		{http.MethodGet, "/api/admin/users?size=10", "", http.StatusOK},
		{http.MethodGet, "/api/admin/clients?size=10", "", http.StatusOK},
		{http.MethodGet, "/api/admin/events?size=10", "", http.StatusOK},
		{http.MethodGet, "/api/admin/openvpn", "", http.StatusOK},

		// So is helping a member back in.
		{http.MethodDelete, "/api/admin/users/" + member.ID.String() + "/lockout", "", http.StatusNoContent},
		{http.MethodPut, "/api/admin/users/" + member.ID.String() + "/mfa", "", http.StatusNoContent},
		{http.MethodDelete, "/api/admin/users/" + member.ID.String() + "/password", "", http.StatusOK},

		// But not an administrator's account.
		{http.MethodDelete, "/api/admin/users/" + admin.ID.String() + "/password", "", http.StatusForbidden},
		{http.MethodPut, "/api/admin/users/" + admin.ID.String() + "/mfa", "", http.StatusForbidden},

		// And nothing that administers.
		{http.MethodPut, "/api/admin/users/" + member.ID.String(), `{"role": "admin"}`, http.StatusForbidden},
		{http.MethodPost, "/api/admin/users", `{"email": "x@example.com"}`, http.StatusForbidden},
		{http.MethodDelete, "/api/admin/users/" + member.ID.String(), "", http.StatusForbidden},
		{http.MethodPost, "/api/admin/groups", `{"name": "X"}`, http.StatusForbidden},
		{http.MethodGet, "/api/admin/settings/vpn", "", http.StatusForbidden},
		{http.MethodGet, "/api/admin/logs/app", "", http.StatusForbidden},
		{http.MethodPost, "/api/admin/openvpn/restart", "", http.StatusForbidden},
	} {
		if got, body := desk.do(tc.method, tc.path, tc.body); got != tc.want {
			t.Errorf("%s %s = %d, want %d: %s", tc.method, tc.path, got, tc.want, body)
		}
	}

	if after, _ := ts.app.Store.Users.Get(t.Context(), member.ID); after.IsLocked(time.Now()) {
		t.Error("the member is still locked out")
	}

	// A member of no staff role reaches none of it.
	plain := ts.browser()
	ts.addMember("plain@example.com", "")
	plain.login("plain@example.com", "Password1")
	if got, _ := plain.do(http.MethodGet, "/api/admin/users", ""); got != http.StatusForbidden {
		t.Errorf("a member listing users = %d, want 403", got)
	}
}

func TestAdministratorSetsRoles(t *testing.T) {
	ts := newTestServer(t)
	ts.install()
	admin := ts.addUser("admin@example.com", false)
	member := ts.addMember("member@example.com", "")

	b := ts.browser()
	b.login("admin@example.com", "Password1")

	status, body := b.do(http.MethodPut, "/api/admin/users/"+member.ID.String(), `{"role": "helpdesk"}`)
	var got struct {
		Role    string `json:"role"`
		IsAdmin bool   `json:"is_admin"`
	}
	json.Unmarshal(body, &got)
	if status != http.StatusOK || got.Role != "helpdesk" || got.IsAdmin {
		t.Fatalf("make help desk = %d %+v", status, got)
	}

	b.do(http.MethodPut, "/api/admin/users/"+member.ID.String(), `{"role": "admin"}`)
	if after, _ := ts.app.Store.Users.Get(t.Context(), member.ID); !after.IsAdmin || after.Role != "" {
		t.Errorf("after making them an administrator: admin %v, role %q", after.IsAdmin, after.Role)
	}

	if status, _ := b.do(http.MethodPut, "/api/admin/users/"+member.ID.String(), `{"role": "owner"}`); status != http.StatusBadRequest {
		t.Errorf("unknown role = %d, want 400", status)
	}
	if status, _ := b.do(http.MethodPut, "/api/admin/users/"+admin.ID.String(), `{"role": "helpdesk"}`); status != http.StatusBadRequest {
		t.Errorf("demoting yourself = %d, want 400", status)
	}

	events := ts.events("admin")
	if len(events) == 0 || events[0].Detail != "Changed member@example.com: role help desk → administrator." {
		t.Errorf("latest admin event = %v", events)
	}
}
