package web

import (
	"errors"
	"net/http"
	"slices"
	"strings"

	"github.com/jeffmvr/mangle-vpn/internal/config"
	"github.com/jeffmvr/mangle-vpn/internal/model"
	"github.com/jeffmvr/mangle-vpn/internal/openvpn"
	"github.com/jeffmvr/mangle-vpn/internal/store"
	"github.com/jeffmvr/mangle-vpn/internal/validate"
)

// groupNameLimit is the longest group name the database column holds.
const groupNameLimit = 32

// adminListGroups returns a page of groups.
func (s *Server) adminListGroups(w http.ResponseWriter, r *http.Request) {
	q := listQuery(r)

	result, err := s.app.Store.Groups.List(r.Context(), q)
	if s.handleStoreError(w, err, "failed to list groups") {
		return
	}
	counts, err := s.app.Store.Groups.Counts(r.Context())
	if s.handleStoreError(w, err, "failed to count group members") {
		return
	}
	writeList(s, w, r, q, result, func(group *model.Group) groupDTO {
		dto := newGroup(group)
		c := counts[group.ID]
		dto.MemberCount, dto.RuleCount = &c.Members, &c.Rules
		return dto
	})
}

// adminListAllGroups returns every group as a name and identity, for
// populating the group chooser.
func (s *Server) adminListAllGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := s.app.Store.Groups.All(r.Context())
	if s.handleStoreError(w, err, "failed to list groups") {
		return
	}

	named := make([]namedGroupDTO, 0, len(groups))
	for _, group := range groups {
		named = append(named, newNamedGroup(group))
	}
	s.writeJSON(w, http.StatusOK, named)
}

// adminGetGroup returns one group.
func (s *Server) adminGetGroup(w http.ResponseWriter, r *http.Request) {
	group, ok := s.loadGroup(w, r)
	if !ok {
		return
	}
	s.writeJSON(w, http.StatusOK, newGroup(group))
}

// groupRequest is the body of a group create or update.
type groupRequest struct {
	Description *string `json:"description"`
	IsEnabled   *bool   `json:"is_enabled"`
	MaxDevices  *int    `json:"max_devices"`
	MFAEnforced *bool   `json:"mfa_enforced"`
	Name        *string `json:"name"`

	Routes         *string `json:"routes"`
	Nameservers    *string `json:"nameservers"`
	DeviceIdleDays *int    `json:"device_idle_days"`
}

// deviceIdleDays are the device retirement periods on offer; 0 keeps
// devices however long they go unused.
var deviceIdleDays = []int{0, 30, 60, 90, 180, 365}

// apply copies the supplied fields onto a group and reports what was wrong.
func (b groupRequest) apply(group *model.Group) fieldErrors {
	errs := fieldErrors{}

	if b.Name != nil {
		name := strings.TrimSpace(*b.Name)
		switch {
		case name == "":
			errs.add("name", "This field is required.")
		case len(name) > groupNameLimit:
			errs.add("name", "Ensure this field has no more than 32 characters.")
		default:
			group.Name = name
		}
	}

	if b.MaxDevices != nil {
		if *b.MaxDevices < 1 {
			errs.add("max_devices", "A group must allow at least one device.")
		} else {
			group.MaxDevices = *b.MaxDevices
		}
	}

	// The routes and DNS servers are pushed into OpenVPN's configuration,
	// so each line is checked rather than passed through.
	if b.Routes != nil {
		lines := config.SplitLines(*b.Routes)
		for _, line := range lines {
			if !validate.IsCIDR(line) || openvpn.ExpandCIDR(line) == "" {
				errs.add("routes", "There are one or more invalid routes.")
				break
			}
		}
		group.Routes = strings.Join(lines, "\n")
	}
	if b.Nameservers != nil {
		lines := config.SplitLines(*b.Nameservers)
		for _, line := range lines {
			if !validate.IsIP(line) {
				errs.add("nameservers", "There are one or more invalid DNS servers.")
				break
			}
		}
		group.Nameservers = strings.Join(lines, "\n")
	}
	if b.DeviceIdleDays != nil {
		if !slices.Contains(deviceIdleDays, *b.DeviceIdleDays) {
			errs.add("device_idle_days", "Choose how long a device may go unused.")
		} else {
			group.DeviceIdleDays = *b.DeviceIdleDays
		}
	}

	assign(&group.Description, b.Description)
	assign(&group.IsEnabled, b.IsEnabled)
	assign(&group.MFAEnforced, b.MFAEnforced)
	return errs
}

// adminCreateGroup adds a group.
func (s *Server) adminCreateGroup(w http.ResponseWriter, r *http.Request) {
	var body groupRequest
	if !s.decodeJSON(w, r, &body) {
		return
	}

	// A new group starts enabled and allowing one device unless told
	// otherwise, which is what the creation form offers.
	group := &model.Group{IsEnabled: true, MaxDevices: 1, MFAEnforced: true}

	if errs := body.apply(group); !errs.empty() {
		s.invalid(w, errs)
		return
	}
	if group.Name == "" {
		s.invalidField(w, "name", "This field is required.")
		return
	}

	if err := s.app.SaveGroup(r.Context(), group); err != nil {
		if errors.Is(err, store.ErrConflict) {
			s.invalidField(w, "name", "A group with this name already exists.")
			return
		}
		s.serverError(w, "failed to create a group", err)
		return
	}

	s.audit(r, model.EventAdminGroupCreate, "Created group %s.", group.Name)
	s.writeJSON(w, http.StatusCreated, newGroup(group))
}

// adminUpdateGroup changes a group's settings and rebuilds its firewall
// chain to match.
func (s *Server) adminUpdateGroup(w http.ResponseWriter, r *http.Request) {
	group, ok := s.loadGroup(w, r)
	if !ok {
		return
	}

	var body groupRequest
	if !s.decodeJSON(w, r, &body) {
		return
	}

	before := *group
	if errs := body.apply(group); !errs.empty() {
		s.invalid(w, errs)
		return
	}

	// Disabling the group the administrator is in would sign them out with
	// no way back.
	if !group.IsEnabled && group.ID == currentUser(r).GroupID {
		s.invalidField(w, "is_enabled", "You cannot disable the group you are in.")
		return
	}

	if err := s.app.SaveGroup(r.Context(), group); err != nil {
		if errors.Is(err, store.ErrConflict) {
			s.invalidField(w, "name", "A group with this name already exists.")
			return
		}
		s.serverError(w, "failed to update a group", err)
		return
	}

	s.audit(r, model.EventAdminGroupUpdate, "Changed group %s: %s.", before.Name, describeGroupChange(&before, group))
	s.writeJSON(w, http.StatusOK, newGroup(group))
}

// describeGroupChange lists what an update changed about a group.
func describeGroupChange(before, after *model.Group) string {
	var changes changeList
	changes.add("name", before.Name, after.Name)
	changes.add("description", before.Description, after.Description)
	changes.add("devices allowed", before.MaxDevices, after.MaxDevices)
	changes.add("two-factor", onOff(before.MFAEnforced), onOff(after.MFAEnforced))
	changes.add("routes", strings.ReplaceAll(before.Routes, "\n", " "), strings.ReplaceAll(after.Routes, "\n", " "))
	changes.add("DNS servers", strings.ReplaceAll(before.Nameservers, "\n", " "), strings.ReplaceAll(after.Nameservers, "\n", " "))
	changes.add("unused devices removed after (days)", before.DeviceIdleDays, after.DeviceIdleDays)
	changes.note(!before.IsEnabled && after.IsEnabled, "enabled the group")
	changes.note(before.IsEnabled && !after.IsEnabled, "disabled the group")
	return changes.String()
}

// adminDeleteGroup removes a group.
//
// Deleting a group deletes its members too, since a user cannot exist
// outside one, so an administrator is stopped from deleting the group they
// are in and locking themselves out.
func (s *Server) adminDeleteGroup(w http.ResponseWriter, r *http.Request) {
	group, ok := s.loadGroup(w, r)
	if !ok {
		return
	}

	if group.ID == currentUser(r).GroupID {
		s.invalidField(w, "id",
			"You cannot delete the group your own account belongs to.")
		return
	}

	if err := s.app.DeleteGroup(r.Context(), group); err != nil {
		s.serverError(w, "failed to delete a group", err)
		return
	}

	s.audit(r, model.EventAdminGroupDelete, "Deleted group %s and its members.", group.Name)
	s.noContent(w)
}

// adminListGroupFirewall returns a group's firewall rules.
func (s *Server) adminListGroupFirewall(w http.ResponseWriter, r *http.Request) {
	group, ok := s.loadGroup(w, r)
	if !ok {
		return
	}

	rules, err := s.app.Store.FirewallRules.ByGroup(r.Context(), group.ID)
	if s.handleStoreError(w, err, "failed to list firewall rules") {
		return
	}

	out := make([]firewallRuleDTO, 0, len(rules))
	for _, rule := range rules {
		out = append(out, newFirewallRule(rule))
	}
	s.writeJSON(w, http.StatusOK, out)
}

// adminListGroupUsers returns a group's members.
func (s *Server) adminListGroupUsers(w http.ResponseWriter, r *http.Request) {
	group, ok := s.loadGroup(w, r)
	if !ok {
		return
	}

	users, err := s.app.Store.Users.ByGroup(r.Context(), group.ID)
	if s.handleStoreError(w, err, "failed to list group members") {
		return
	}

	out := make([]accountDTO, 0, len(users))
	for _, user := range users {
		out = append(out, newAccount(user))
	}
	s.writeJSON(w, http.StatusOK, out)
}

// loadGroup reads the group named in the request path.
func (s *Server) loadGroup(w http.ResponseWriter, r *http.Request) (*model.Group, bool) {
	id, ok := s.pathID(w, r, "id")
	if !ok {
		return nil, false
	}

	group, err := s.app.Store.Groups.Get(r.Context(), id)
	if s.handleStoreError(w, err, "failed to load a group") {
		return nil, false
	}
	return group, true
}
