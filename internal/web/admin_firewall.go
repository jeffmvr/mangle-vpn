package web

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/jeffmvr/mangle-vpn/internal/model"
	"github.com/jeffmvr/mangle-vpn/internal/uuid"
	"github.com/jeffmvr/mangle-vpn/internal/validate"
)

// firewallRuleRequest is the body of a firewall rule create or update.
type firewallRuleRequest struct {
	Action      *string   `json:"action"`
	Destination *string   `json:"destination"`
	GroupID     uuidField `json:"group_id"`
	IsEnabled   *bool     `json:"is_enabled"`
	Port        *string   `json:"port"`
	Protocol    *string   `json:"protocol"`
}

// adminGetFirewallRule returns one firewall rule.
func (s *Server) adminGetFirewallRule(w http.ResponseWriter, r *http.Request) {
	rule, ok := s.loadFirewallRule(w, r)
	if !ok {
		return
	}
	s.writeJSON(w, http.StatusOK, newFirewallRule(rule))
}

// adminCreateFirewallRule adds a rule to a group's chain.
func (s *Server) adminCreateFirewallRule(w http.ResponseWriter, r *http.Request) {
	var body firewallRuleRequest
	if !s.decodeJSON(w, r, &body) {
		return
	}

	rule := &model.FirewallRule{IsEnabled: true}
	if !s.applyFirewallRule(w, r, rule, body) {
		return
	}

	if err := s.app.SaveFirewallRule(r.Context(), rule); err != nil {
		s.serverError(w, "failed to create a firewall rule", err)
		return
	}

	s.audit(r, model.EventAdminRuleCreate, "Added a firewall rule to %s: %s.", s.groupName(r, rule.GroupID), describeRule(rule))
	s.writeJSON(w, http.StatusCreated, newFirewallRule(rule))
}

// adminUpdateFirewallRule changes a rule and rebuilds the chain.
func (s *Server) adminUpdateFirewallRule(w http.ResponseWriter, r *http.Request) {
	rule, ok := s.loadFirewallRule(w, r)
	if !ok {
		return
	}

	var body firewallRuleRequest
	if !s.decodeJSON(w, r, &body) {
		return
	}

	before := describeRule(rule)
	if !s.applyFirewallRule(w, r, rule, body) {
		return
	}

	if err := s.app.SaveFirewallRule(r.Context(), rule); err != nil {
		s.serverError(w, "failed to update a firewall rule", err)
		return
	}

	s.audit(r, model.EventAdminRuleUpdate, "Changed a firewall rule in %s from %s to %s.",
		s.groupName(r, rule.GroupID), before, describeRule(rule))
	s.writeJSON(w, http.StatusOK, newFirewallRule(rule))
}

// adminDeleteFirewallRule removes a rule and rebuilds the chain.
func (s *Server) adminDeleteFirewallRule(w http.ResponseWriter, r *http.Request) {
	rule, ok := s.loadFirewallRule(w, r)
	if !ok {
		return
	}

	if err := s.app.DeleteFirewallRule(r.Context(), rule); err != nil {
		s.serverError(w, "failed to delete a firewall rule", err)
		return
	}

	s.audit(r, model.EventAdminRuleDelete, "Removed a firewall rule from %s: %s.", s.groupName(r, rule.GroupID), describeRule(rule))
	s.noContent(w)
}

// applyFirewallRule validates a submission and copies it onto a rule,
// reporting whether it was accepted.
//
// These values are written straight into iptables arguments, so each one is
// checked against what iptables will accept rather than being passed
// through.
func (s *Server) applyFirewallRule(w http.ResponseWriter, r *http.Request,
	rule *model.FirewallRule, body firewallRuleRequest) bool {

	errs := fieldErrors{}

	if body.Action != nil {
		action := strings.ToUpper(strings.TrimSpace(*body.Action))
		if action != model.ActionAccept && action != model.ActionDrop {
			errs.add("action", "Must be ACCEPT or DROP.")
		} else {
			rule.Action = action
		}
	}
	if rule.Action == "" {
		errs.add("action", "This field is required.")
	}

	if body.Destination != nil {
		destination := strings.TrimSpace(*body.Destination)
		if destination != "" && !validate.IsCIDR(destination) {
			errs.add("destination", "Must be an IPv4 address or CIDR address.")
		} else {
			rule.Destination = destination
		}
	}

	if body.Protocol != nil {
		protocol := strings.ToLower(strings.TrimSpace(*body.Protocol))
		switch protocol {
		case model.ProtocolAll, model.ProtocolTCP, model.ProtocolUDP, "":
			rule.Protocol = protocol
		default:
			errs.add("protocol", "Protocol must be either All, TCP, or UDP.")
		}
	}

	if body.Port != nil {
		port := strings.ReplaceAll(strings.TrimSpace(*body.Port), " ", "")
		if message := validatePortSpec(port); message != "" {
			errs.add("port", message)
		} else {
			rule.Port = port
		}
	}

	// iptables can only match a port once a protocol narrows the rule to
	// one that has ports at all.
	if rule.Port != "" && (rule.Protocol == "" || rule.Protocol == model.ProtocolAll) {
		errs.add("protocol", "Protocol must be TCP or UDP when using ports.")
	}

	body.GroupID.resolve(errs, "group_id", &rule.GroupID, func(id uuid.UUID) bool {
		return s.app.Store.Groups.Exists(r.Context(), id)
	})
	if rule.GroupID.IsZero() && !body.GroupID.Present {
		errs.add("group_id", "This field is required.")
	}

	assign(&rule.IsEnabled, body.IsEnabled)

	if !errs.empty() {
		s.invalid(w, errs)
		return false
	}
	return true
}

// validatePortSpec checks an iptables port specification, which may be a
// single port, a low:high range, or a comma separated mix of both. It
// returns an empty string when the specification is usable.
func validatePortSpec(spec string) string {
	if spec == "" {
		return ""
	}

	for part := range strings.SplitSeq(spec, ",") {
		low, high, isRange := strings.Cut(part, ":")

		if !isRange {
			if !isPortNumber(part) {
				return "There are one or more invalid ports."
			}
			continue
		}

		from, to := parsePort(low), parsePort(high)
		if from == 0 || to == 0 || from >= to {
			return "There are one or more invalid port ranges."
		}
	}
	return ""
}

// isPortNumber reports whether s is a port number.
func isPortNumber(s string) bool { return parsePort(s) != 0 }

// parsePort returns the port number in s, or zero when it is not one.
func parsePort(s string) int {
	port, err := strconv.Atoi(s)
	if err != nil || port < 1 || port > 65535 {
		return 0
	}
	return port
}

// loadFirewallRule reads the rule named in the request path.
func (s *Server) loadFirewallRule(w http.ResponseWriter, r *http.Request) (*model.FirewallRule, bool) {
	id, ok := s.pathID(w, r, "id")
	if !ok {
		return nil, false
	}

	rule, err := s.app.Store.FirewallRules.Get(r.Context(), id)
	if s.handleStoreError(w, err, "failed to load a firewall rule") {
		return nil, false
	}
	return rule, true
}

// groupName returns the name of a group, for the audit log, or its ID when
// it cannot be read.
func (s *Server) groupName(r *http.Request, id uuid.UUID) string {
	group, err := s.app.Store.Groups.Get(r.Context(), id)
	if err != nil {
		return id.String()
	}
	return group.Name
}
