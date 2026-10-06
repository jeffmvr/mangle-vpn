package web

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/jeffmvr/mangle-vpn/internal/model"
	"github.com/jeffmvr/mangle-vpn/internal/store"
)

// do sends an API request with the CSRF token and returns the status and
// the body.
func (b *browser) do(method, path, body string) (int, []byte) {
	b.ts.t.Helper()
	req, _ := http.NewRequest(method, b.ts.http.URL+path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(csrfHeader, b.csrf())
	resp, err := b.client.Do(req)
	if err != nil {
		b.ts.t.Fatal(err)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, out
}

// events returns the audit log entries of one kind, newest first.
func (ts *testServer) events(kind string) []*model.Event {
	ts.t.Helper()
	page, err := ts.app.Store.Events.List(ts.t.Context(), store.Query{}, store.EventFilter{Kind: kind})
	if err != nil {
		ts.t.Fatal(err)
	}
	return page.Items
}

func TestAdminChangesAreAudited(t *testing.T) {
	ts := newTestServer(t)
	ts.install()
	admin := ts.addUser("admin@example.com", false)
	member := ts.addUser("member@example.com", false)

	b := ts.browser()
	b.login("admin@example.com", "Password1")

	if status, body := b.do(http.MethodPut, "/api/admin/users/"+member.ID.String(),
		`{"name": "Grace Hopper", "is_enabled": false}`); status != http.StatusOK {
		t.Fatalf("update = %d: %s", status, body)
	}
	if status, body := b.do(http.MethodPost, "/api/admin/groups",
		`{"name": "Engineering", "max_devices": 3}`); status != http.StatusCreated {
		t.Fatalf("create group = %d: %s", status, body)
	}
	if status, body := b.do(http.MethodPut, "/api/admin/settings/vpn",
		`{"vpn_port": "1195", "vpn_protocol": "udp"}`); status != http.StatusNoContent {
		t.Fatalf("settings = %d: %s", status, body)
	}

	events := ts.events("admin")
	if len(events) != 3 {
		t.Fatalf("got %d admin events, want 3: %v", len(events), events)
	}
	want := []string{
		"Changed OpenVPN settings: port.",
		"Created group Engineering.",
		"Changed member@example.com: name (none) → Grace Hopper, disabled the account.",
	}
	for i, event := range events {
		if event.UserID != admin.ID {
			t.Errorf("%s was recorded against %s, want the administrator", event.Name, event.UserID)
		}
		if event.Detail != want[i] {
			t.Errorf("detail = %q, want %q", event.Detail, want[i])
		}
	}

	// The filter is offered over the API too, and refuses unknown kinds.
	status, body := b.do(http.MethodGet, "/api/admin/events?kind=admin&size=10", "")
	var page struct{ Count int }
	json.Unmarshal(body, &page)
	if status != http.StatusOK || page.Count != 3 {
		t.Errorf("GET ?kind=admin = %d with %d events, want 3", status, page.Count)
	}
	if status, _ := b.do(http.MethodGet, "/api/admin/events?kind=nope", ""); status != http.StatusBadRequest {
		t.Errorf("GET ?kind=nope = %d, want 400", status)
	}
}

func TestDescribeRule(t *testing.T) {
	for _, tc := range []struct {
		rule model.FirewallRule
		want string
	}{
		{model.FirewallRule{Action: model.ActionAccept, Protocol: "tcp", Port: "22,443", Destination: "10.0.10.0/24", IsEnabled: true},
			"allow tcp port 22,443 to 10.0.10.0/24"},
		{model.FirewallRule{Action: model.ActionDrop, Protocol: "all", Destination: "10.0.99.0/24", IsEnabled: false},
			"deny all traffic to 10.0.99.0/24 (turned off)"},
		{model.FirewallRule{Action: model.ActionDrop, IsEnabled: true}, "deny all traffic to anywhere"},
	} {
		if got := describeRule(&tc.rule); got != tc.want {
			t.Errorf("describeRule = %q, want %q", got, tc.want)
		}
	}
}
