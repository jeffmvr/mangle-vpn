package web

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/jeffmvr/mangle-vpn/internal/model"
)

// The audit trail of what administrators change. Each change is recorded
// against the administrator who made it, with a sentence naming what was
// changed, so the audit log answers "who did this" as well as "who signed
// in".

// audit records a change made by the signed in user.
func (s *Server) audit(r *http.Request, name, format string, args ...any) {
	s.app.RecordEvent(r.Context(), currentUser(r), name, fmt.Sprintf(format, args...))
}

// changeList collects the differences between two versions of something,
// as phrases such as "name Sales → Marketing".
type changeList []string

// add notes a changed value, saying nothing when it is unchanged.
func (c *changeList) add(label string, before, after any) {
	b, a := fmt.Sprint(before), fmt.Sprint(after)
	if b == a {
		return
	}
	if b == "" {
		b = "(none)"
	}
	if a == "" {
		a = "(none)"
	}
	*c = append(*c, fmt.Sprintf("%s %s → %s", label, b, a))
}

// note adds a change described in words, when happened is true.
func (c *changeList) note(happened bool, phrase string) {
	if happened {
		*c = append(*c, phrase)
	}
}

// String joins the changes, or says there were none.
func (c changeList) String() string {
	if len(c) == 0 {
		return "nothing"
	}
	return strings.Join(c, ", ")
}

// onOff renders a switch.
func onOff(on bool) string {
	if on {
		return "on"
	}
	return "off"
}

// describeRule renders a firewall rule as a short sentence, such as
// "allow tcp 22,443 to 10.0.10.0/24".
func describeRule(rule *model.FirewallRule) string {
	parts := []string{"deny"}
	if rule.Action == model.ActionAccept {
		parts[0] = "allow"
	}

	protocol := rule.Protocol
	if protocol == "" || protocol == model.ProtocolAll {
		protocol = "all traffic"
	}
	parts = append(parts, protocol)
	if rule.Port != "" {
		parts = append(parts, "port "+rule.Port)
	}

	destination := rule.Destination
	if destination == "" {
		destination = "anywhere"
	}
	parts = append(parts, "to "+destination)

	if !rule.IsEnabled {
		parts = append(parts, "(turned off)")
	}
	return strings.Join(parts, " ")
}
