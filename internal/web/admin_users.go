package web

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/jeffmvr/mangle-vpn/internal/app"
	"github.com/jeffmvr/mangle-vpn/internal/model"
	"github.com/jeffmvr/mangle-vpn/internal/store"
	"github.com/jeffmvr/mangle-vpn/internal/uuid"
	"github.com/jeffmvr/mangle-vpn/internal/validate"
)

// adminListUsers returns a page of users, optionally narrowed to
// administrators or not (?admin=true|false) and to enabled accounts or not
// (?enabled=true|false).
func (s *Server) adminListUsers(w http.ResponseWriter, r *http.Request) {
	q := listQuery(r)

	errs := fieldErrors{}
	filter := store.UserFilter{
		Admin:   boolParam(r, "admin", errs),
		Enabled: boolParam(r, "enabled", errs),
	}
	if !errs.empty() {
		s.invalid(w, errs)
		return
	}

	result, err := s.app.Store.Users.List(r.Context(), q, filter)
	if s.handleStoreError(w, err, "failed to list users") {
		return
	}
	writeList(s, w, r, q, result, newUser)
}

// adminGetUser returns one user.
func (s *Server) adminGetUser(w http.ResponseWriter, r *http.Request) {
	user, ok := s.loadUser(w, r)
	if !ok {
		return
	}
	s.writeJSON(w, http.StatusOK, newUser(user))
}

// inviteRequest is the body of a user invitation.
type inviteRequest struct {
	// Email holds one or more whitespace separated addresses, so that a
	// whole team can be invited in one go.
	Email   string    `json:"email"`
	GroupID uuidField `json:"group_id"`
	Notify  bool      `json:"notify"`
}

// adminInviteUsers creates an account for each submitted address and
// returns the credentials of the ones that were new.
func (s *Server) adminInviteUsers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var body inviteRequest
	if !s.decodeJSON(w, r, &body) {
		return
	}

	// Anything that is not an address is passed over rather than failing
	// the whole batch, which is what makes pasting a list of names work.
	var emails []string
	for _, candidate := range strings.Fields(body.Email) {
		if validate.IsEmail(candidate) {
			emails = append(emails, strings.ToLower(candidate))
		}
	}

	errs := fieldErrors{}
	if len(emails) == 0 {
		errs.add("email", "At least one valid e-mail address is required.")
	}
	groupID := uuid.Nil
	body.GroupID.resolve(errs, "group_id", &groupID, func(id uuid.UUID) bool {
		return s.app.Store.Groups.Exists(ctx, id)
	})
	if !body.GroupID.Present {
		errs.add("group_id", "This field is required.")
	}
	if !errs.empty() {
		s.invalid(w, errs)
		return
	}

	// Inviting yourself into a disabled group would lock you out.
	if group, err := s.app.Store.Groups.Get(ctx, groupID); err == nil && !group.IsEnabled &&
		slices.Contains(emails, strings.ToLower(currentUser(r).Email)) {
		s.invalidField(w, "email", "You cannot move yourself into a disabled group.")
		return
	}

	invited, err := s.app.InviteUsers(ctx, emails, groupID, body.Notify)
	if err != nil {
		s.serverError(w, "failed to invite users", err)
		return
	}

	groupName := ""
	if group, err := s.app.Store.Groups.Get(ctx, groupID); err == nil {
		groupName = group.Name
	}
	s.audit(r, model.EventAdminUserInvite, "Invited %s to %s.", strings.Join(emails, ", "), groupName)

	if invited == nil {
		invited = []app.Invitation{}
	}
	s.writeJSON(w, http.StatusCreated, invited)
}

// updateUserRequest is the body of a user update. Pointers distinguish a
// field that was left out from one that was set to its zero value.
type updateUserRequest struct {
	Email       *string           `json:"email"`
	GroupID     uuidField         `json:"group_id"`
	IsAdmin     *bool             `json:"is_admin"`
	Role        *string           `json:"role"`
	IsEnabled   *bool             `json:"is_enabled"`
	MFAEnabled  *bool             `json:"mfa_enabled"`
	MFAEnforced optionalBoolField `json:"mfa_enforced"`
	Name        *string           `json:"name"`
}

// adminUpdateUser changes a user's details, group, or access.
func (s *Server) adminUpdateUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user, ok := s.loadUser(w, r)
	if !ok {
		return
	}

	var body updateUserRequest
	if !s.decodeJSON(w, r, &body) {
		return
	}

	// A copy to describe the change against, once it is made.
	before := *user

	errs := fieldErrors{}

	if body.Email != nil {
		email := strings.ToLower(strings.TrimSpace(*body.Email))
		switch {
		case !validate.IsEmail(email):
			errs.add("email", "A valid e-mail address is required.")
		default:
			existing, err := s.app.Store.Users.ByEmail(ctx, email)
			if err == nil && existing.ID != user.ID {
				errs.add("email", "A user with this e-mail address already exists.")
			}
			user.Email = email
		}
	}

	body.GroupID.resolve(errs, "group_id", &user.GroupID, func(id uuid.UUID) bool {
		return s.app.Store.Groups.Exists(ctx, id)
	})

	// The role is the newer way of saying whether someone is an
	// administrator, and takes over from is_admin when both are sent.
	if body.Role != nil {
		switch *body.Role {
		case roleAdmin:
			body.IsAdmin, user.Role = new(true), ""
		case model.RoleHelpDesk:
			body.IsAdmin, user.Role = new(false), model.RoleHelpDesk
		case roleMember:
			body.IsAdmin, user.Role = new(false), ""
		default:
			errs.add("role", "Choose administrator, help desk or member.")
		}
	}

	if !errs.empty() {
		s.invalid(w, errs)
		return
	}

	// An administrator must not be able to lock themselves out, which would
	// leave the application with no way back in.
	if user.ID == currentUser(r).ID {
		if body.IsAdmin != nil && !*body.IsAdmin {
			s.invalidField(w, "is_admin", "You cannot remove your own administrator access.")
			return
		}
		if body.IsEnabled != nil && !*body.IsEnabled {
			s.invalidField(w, "is_enabled", "You cannot disable your own account.")
			return
		}
		if group, err := s.app.Store.Groups.Get(ctx, user.GroupID); err == nil && !group.IsEnabled {
			s.invalidField(w, "group_id", "You cannot move yourself into a disabled group.")
			return
		}
	}

	assign(&user.IsAdmin, body.IsAdmin)
	assign(&user.IsEnabled, body.IsEnabled)
	assign(&user.Name, body.Name)
	if body.MFAEnforced.Present {
		user.MFAEnforced = body.MFAEnforced.Value
	}

	// Turning two-factor off for an account also retires its secret, or
	// the old one would be shown again to whoever enrols next.
	if body.MFAEnabled != nil && !*body.MFAEnabled && user.MFAEnabled {
		user.ResetMFA()
	}

	if err := s.app.SaveUser(ctx, user); err != nil {
		if errors.Is(err, store.ErrConflict) {
			s.invalidField(w, "email", "A user with this e-mail address already exists.")
			return
		}
		s.serverError(w, "failed to update a user", err)
		return
	}

	s.audit(r, model.EventAdminUserUpdate, "Changed %s: %s.", before.Email, describeUserChange(&before, user))
	s.writeJSON(w, http.StatusOK, newUser(user))
}

// describeUserChange lists what an update changed about a user.
func describeUserChange(before, after *model.User) string {
	var changes changeList
	changes.add("email", before.Email, after.Email)
	changes.add("name", before.Name, after.Name)
	if before.Group != nil && after.Group != nil {
		changes.add("group", before.Group.Name, after.Group.Name)
	}
	changes.add("role", roleNames[roleOf(before)], roleNames[roleOf(after)])
	changes.note(!before.IsEnabled && after.IsEnabled, "enabled the account")
	changes.note(before.IsEnabled && !after.IsEnabled, "disabled the account")
	changes.add("two-factor", mfaSetting(before.MFAEnforced), mfaSetting(after.MFAEnforced))
	changes.note(before.MFAEnabled && !after.MFAEnabled, "turned two-factor off")
	return changes.String()
}

// mfaSetting renders a user's own two-factor setting.
func mfaSetting(enforced *bool) string {
	switch {
	case enforced == nil:
		return "inherited from group"
	case *enforced:
		return "required"
	default:
		return "not required"
	}
}

// adminDeleteUser removes a user along with their devices and connections.
func (s *Server) adminDeleteUser(w http.ResponseWriter, r *http.Request) {
	user, ok := s.loadUser(w, r)
	if !ok {
		return
	}

	if user.ID == currentUser(r).ID {
		s.invalidField(w, "id", "You cannot delete your own account.")
		return
	}

	if err := s.app.DeleteUser(r.Context(), user); err != nil {
		s.serverError(w, "failed to delete a user", err)
		return
	}
	s.audit(r, model.EventAdminUserDelete, "Deleted %s.", user.Email)
	s.noContent(w)
}

// adminUnlockUser ends a lockout after too many failed sign ins, so the
// user can try again straight away.
func (s *Server) adminUnlockUser(w http.ResponseWriter, r *http.Request) {
	user, ok := s.loadUserToHelp(w, r)
	if !ok {
		return
	}

	if err := s.app.Store.Users.ClearFailedLogins(r.Context(), user); err != nil {
		s.serverError(w, "failed to unlock a user", err)
		return
	}

	s.audit(r, model.EventAdminUserUnlock, "Unlocked %s.", user.Email)
	s.noContent(w)
}

// adminResetUserMFA issues a user a new two-factor secret, so that they
// enrol again on their next sign in. It is how a lost authenticator is
// recovered from.
func (s *Server) adminResetUserMFA(w http.ResponseWriter, r *http.Request) {
	user, ok := s.loadUserToHelp(w, r)
	if !ok {
		return
	}

	user.ResetMFA()

	err := s.app.SaveUser(r.Context(), user)
	if err == nil {
		// A session already past two-factor would otherwise carry on.
		err = s.app.Store.Users.EndSessions(r.Context(), user)
	}
	if err != nil {
		s.serverError(w, "failed to reset two-factor authentication", err)
		return
	}
	s.audit(r, model.EventAdminUserMFA, "Reset two-factor authentication for %s.", user.Email)
	s.noContent(w)
}

// adminResetUserPassword retires a user's password and e-mails them a link
// to choose a new one, returning the link so an administrator can pass it
// on directly when mail is not set up.
func (s *Server) adminResetUserPassword(w http.ResponseWriter, r *http.Request) {
	user, ok := s.loadUserToHelp(w, r)
	if !ok {
		return
	}

	link, err := s.app.ResetUserPassword(r.Context(), user)
	if err != nil {
		s.serverError(w, "failed to reset a password", err)
		return
	}

	s.audit(r, model.EventAdminUserPassword, "Reset the password of %s.", user.Email)
	s.writeJSON(w, http.StatusOK, map[string]any{"link": link, "emailed": s.app.MailConfigured()})
}

// adminListUserDevices returns a user's devices.
func (s *Server) adminListUserDevices(w http.ResponseWriter, r *http.Request) {
	user, ok := s.loadUser(w, r)
	if !ok {
		return
	}

	devices, err := s.app.Store.Devices.ByUser(r.Context(), user.ID)
	if s.handleStoreError(w, err, "failed to list a user's devices") {
		return
	}
	out := newDevices(devices)
	s.addDeviceStatus(r, user.ID, out)
	s.writeJSON(w, http.StatusOK, out)
}

// adminListUserSessions returns a page of a user's finished connections,
// the latest first.
func (s *Server) adminListUserSessions(w http.ResponseWriter, r *http.Request) {
	user, ok := s.loadUser(w, r)
	if !ok {
		return
	}

	q := listQuery(r)
	if !q.Paginated() {
		q.Size, q.Page = 10, 1
	}
	result, err := s.app.Store.VPNSessions.ByUser(r.Context(), user.ID, q)
	if s.handleStoreError(w, err, "failed to list a user's connections") {
		return
	}
	writeList(s, w, r, q, result, newSession)
}

// adminDeleteDevice removes any user's device.
func (s *Server) adminDeleteDevice(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, ok := s.pathID(w, r, "id")
	if !ok {
		return
	}

	device, err := s.app.Store.Devices.Get(ctx, id)
	if s.handleStoreError(w, err, "failed to load a device") {
		return
	}
	if !s.mayHelp(w, r, device.User) {
		return
	}

	if err := s.app.DeleteDevice(ctx, device); err != nil {
		s.serverError(w, "failed to delete a device", err)
		return
	}
	s.audit(r, model.EventAdminDeviceDelete, "Removed %s's device %s.", device.User.Email, device.Name)
	s.noContent(w)
}

// updateDeviceRequest is the body of a device update.
type updateDeviceRequest struct {
	// StaticIP is the fixed address to give the device: an address, "auto"
	// for the first free one, or "" for none.
	StaticIP *string `json:"static_ip"`
}

// adminUpdateDevice changes a device's fixed VPN address.
func (s *Server) adminUpdateDevice(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, ok := s.pathID(w, r, "id")
	if !ok {
		return
	}
	device, err := s.app.Store.Devices.Get(ctx, id)
	if s.handleStoreError(w, err, "failed to load a device") {
		return
	}

	var body updateDeviceRequest
	if !s.decodeJSON(w, r, &body) {
		return
	}
	if body.StaticIP == nil {
		s.writeJSON(w, http.StatusOK, newDevice(device))
		return
	}

	before := device.StaticIP
	err = s.app.AssignStaticIP(ctx, device, strings.TrimSpace(*body.StaticIP))
	switch {
	case errors.Is(err, app.ErrAddressOutOfRange):
		plan, _ := s.app.AddressPlan()
		s.invalidField(w, "static_ip", fmt.Sprintf("Choose an address from %s to %s.", plan.FixedStart, plan.FixedEnd))
		return
	case errors.Is(err, app.ErrAddressTaken):
		s.invalidField(w, "static_ip", "Another device already has this address.")
		return
	case errors.Is(err, app.ErrNoFreeAddress):
		s.invalidField(w, "static_ip", "Every fixed address is taken.")
		return
	case err != nil:
		s.serverError(w, "failed to set a fixed address", err)
		return
	}

	var changes changeList
	changes.add("fixed address", before, device.StaticIP)
	s.audit(r, model.EventAdminDeviceUpdate, "Changed %s's device %s: %s.", device.User.Email, device.Name, changes)
	s.writeJSON(w, http.StatusOK, newDevice(device))
}

// loadUser reads the user named in the request path.
func (s *Server) loadUser(w http.ResponseWriter, r *http.Request) (*model.User, bool) {
	id, ok := s.pathID(w, r, "id")
	if !ok {
		return nil, false
	}

	user, err := s.app.Store.Users.Get(r.Context(), id)
	if s.handleStoreError(w, err, "failed to load a user") {
		return nil, false
	}
	return user, true
}

// loadUserToHelp reads the user named in the request path, for one of the
// changes the help desk may make, refusing when the signed in user may not
// make it to them.
func (s *Server) loadUserToHelp(w http.ResponseWriter, r *http.Request) (*model.User, bool) {
	user, ok := s.loadUser(w, r)
	if !ok || !s.mayHelp(w, r, user) {
		return nil, false
	}
	return user, true
}

// mayHelp reports whether the signed in user may reset, unlock or remove
// devices of the given one, refusing the request when not. The help desk
// helps members only: reaching another member of staff's account would be
// a way of borrowing their access.
func (s *Server) mayHelp(w http.ResponseWriter, r *http.Request, user *model.User) bool {
	if currentUser(r).IsAdmin || user == nil || !user.IsStaff() {
		return true
	}
	s.forbidden(w, r, "Only an administrator can do this to another member of staff.")
	return false
}

// The roles the API accepts and reports.
const (
	roleAdmin  = "admin"
	roleMember = "member"
)

// roleNames are what the audit log calls each role.
var roleNames = map[string]string{
	roleAdmin:          "administrator",
	model.RoleHelpDesk: "help desk",
	roleMember:         "member",
}

// roleOf names a user's role.
func roleOf(user *model.User) string {
	switch {
	case user.IsAdmin:
		return roleAdmin
	case user.Role == model.RoleHelpDesk:
		return model.RoleHelpDesk
	default:
		return roleMember
	}
}

// assign copies value into target when value was supplied.
func assign[T any](target *T, value *T) {
	if value != nil {
		*target = *value
	}
}
