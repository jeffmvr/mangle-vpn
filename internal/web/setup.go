package web

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"maps"
	"net"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/jeffmvr/mangle-vpn/internal/config"
	"github.com/jeffmvr/mangle-vpn/internal/model"
	"github.com/jeffmvr/mangle-vpn/internal/openvpn"
	"github.com/jeffmvr/mangle-vpn/internal/pki"
	"github.com/jeffmvr/mangle-vpn/internal/store"
	"github.com/jeffmvr/mangle-vpn/internal/validate"
)

// First run setup is a short wizard: the organization, then the VPN, then
// the first administrator. The answers are kept in the session as they are
// given, and nothing is created until the last step is submitted, so a
// setup abandoned halfway leaves nothing behind. Passwords are never kept:
// the administrator step is the last, and creates everything at once.

// Session keys for the wizard's answers so far and for the summary shown
// once it has finished.
const (
	sessionSetup     = "setup"
	sessionSetupDone = "setup_done"
)

// setupStep is one page of the wizard.
type setupStep struct {
	Slug     string
	Label    string
	template string
}

var setupSteps = []setupStep{
	{"organization", "Organization", "InstallOrganization.html"},
	{"vpn", "VPN", "InstallVPN.html"},
	{"admin", "Administrator", "InstallAdmin.html"},
}

// Choices on the VPN step.
const (
	accessNetworks = "networks"
	accessAll      = "all"
)

// setupKeyTypes are the certificate authority's key choices, with what each
// comes to.
var setupKeyTypes = map[string]pki.Options{
	"ecdsa":   {KeyType: pki.KeyECDSA},
	"rsa2048": {KeyType: pki.KeyRSA, KeySize: 2048},
	"rsa4096": {KeyType: pki.KeyRSA, KeySize: 4096},
}

var setupKeyLabels = map[string]string{
	"ecdsa":   "ECDSA P-256",
	"rsa2048": "RSA 2048",
	"rsa4096": "RSA 4096",
}

// setupView is what a wizard page is given beyond the usual page data.
type setupView struct {
	Steps   []setupStep
	Current int

	// NeedsCode asks for the setup code, which the link printed by
	// "mangle-vpn install" otherwise carries.
	NeedsCode bool

	// HasAuthority says a certificate authority already exists, as after
	// an import from the Django release, so there is no key to choose.
	HasAuthority bool
}

// setupItem is one step as the progress indicator shows it.
type setupItem struct {
	Number int
	Slug   string
	Label  string
	State  string // done, current or upcoming
}

// Items returns the steps for the progress indicator.
func (v setupView) Items() []setupItem {
	items := make([]setupItem, len(v.Steps))
	for i, step := range v.Steps {
		state := "upcoming"
		switch {
		case i < v.Current:
			state = "done"
		case i == v.Current:
			state = "current"
		}
		items[i] = setupItem{Number: i + 1, Slug: step.Slug, Label: step.Label, State: state}
	}
	return items
}

// setupSummary is what the finished page reports.
type setupSummary struct {
	AuthorityName  string    `json:"authority_name"`
	AuthorityKey   string    `json:"authority_key"`
	AuthorityUntil time.Time `json:"authority_until"`
	KeptAuthority  bool      `json:"kept_authority"`
	VPNAddress     string    `json:"vpn_address"`
	VPNRunning     bool      `json:"vpn_running"`
	FullTunnel     bool      `json:"full_tunnel"`
	Routes         []string  `json:"routes"`
	MFARequired    bool      `json:"mfa_required"`
}

// setupFinished reports whether setup has finished, sending the visitor on
// when it has.
func (s *Server) setupFinished(w http.ResponseWriter, r *http.Request) bool {
	if s.app.Config.Bool(config.AppInstalled, false) {
		http.Redirect(w, r, "/", http.StatusFound)
		return true
	}
	return false
}

// showSetupStart sends the visitor to the first step, keeping the setup
// code the link carries.
func (s *Server) showSetupStart(w http.ResponseWriter, r *http.Request) {
	if s.setupFinished(w, r) {
		return
	}
	target := "/install/organization"
	if token := r.URL.Query().Get("token"); token != "" {
		target += "?token=" + url.QueryEscape(token)
	}
	http.Redirect(w, r, target, http.StatusFound)
}

// showSetupStep renders one step of the wizard.
func (s *Server) showSetupStep(w http.ResponseWriter, r *http.Request) {
	if s.setupFinished(w, r) {
		return
	}
	ctx := r.Context()

	index := setupStepIndex(r.PathValue("step"))
	if index < 0 {
		s.notFound(w)
		return
	}

	state := s.setupState(ctx)

	// Following the link "mangle-vpn install" printed proves the visitor
	// can read the server's files, so the code need not be typed.
	if token := r.URL.Query().Get("token"); token != "" && s.setupTokenMatches(token) {
		state["setup_token"] = token
		s.putSetupState(ctx, state)
	}

	// The steps are taken in order.
	if first := firstUnfinishedStep(state); index > first {
		http.Redirect(w, r, "/install/"+setupSteps[first].Slug, http.StatusFound)
		return
	}

	data := s.newPageData(w, r)
	// A page shown afresh starts from what was answered before, or from
	// sensible guesses.
	if len(data.Form.Data) == 0 {
		data.Form.Data = s.setupDefaults(r, state)
	}
	data.Setup = &setupView{
		Steps:        setupSteps,
		Current:      index,
		NeedsCode:    !s.setupTokenMatches(state["setup_token"]),
		HasAuthority: s.app.Config.Get(config.CACertificate) != "",
	}
	s.render(w, r, setupSteps[index].template, data)
}

// processSetupStep checks a step's answers and moves on to the next, or
// finishes setup after the last.
func (s *Server) processSetupStep(w http.ResponseWriter, r *http.Request) {
	if s.setupFinished(w, r) {
		return
	}
	ctx := r.Context()

	index := setupStepIndex(r.PathValue("step"))
	if index < 0 {
		s.notFound(w)
		return
	}
	state := s.setupState(ctx)
	if first := firstUnfinishedStep(state); index > first {
		http.Redirect(w, r, "/install/"+setupSteps[first].Slug, http.StatusFound)
		return
	}

	here := "/install/" + setupSteps[index].Slug
	var form formState
	switch setupSteps[index].Slug {
	case "organization":
		form = s.checkSetupOrganization(r, state)
	case "vpn":
		form = s.checkSetupVPN(r)
	case "admin":
		s.finishSetup(w, r, state)
		return
	}

	if !form.Valid() {
		s.putForm(r, form)
		http.Redirect(w, r, here, http.StatusFound)
		return
	}

	maps.Copy(state, form.Data)
	state["done_"+setupSteps[index].Slug] = "1"
	s.putSetupState(ctx, state)
	http.Redirect(w, r, "/install/"+setupSteps[index+1].Slug, http.StatusFound)
}

// checkSetupOrganization checks the first step: the setup code, unless the
// link carried it, the organization's name and the web address.
func (s *Server) checkSetupOrganization(r *http.Request, state map[string]string) formState {
	form := s.readForm(r, "setup_token", config.AppOrganization, config.AppHostname)
	if form.Data["setup_token"] == "" {
		form.Data["setup_token"] = state["setup_token"]
	}

	// Until setup finishes, anyone who can reach the server could otherwise
	// make themselves its administrator. The code proves the visitor can
	// read the server's own files.
	if !s.setupTokenMatches(form.Data["setup_token"]) {
		form.Errors["setup_token"] = "This is not the setup code \"mangle-vpn install\" printed on the server."
	}

	validateRequired(&form, map[string]string{
		config.AppOrganization: "An organization name is required.",
		config.AppHostname:     "A web address is required.",
	})
	form.Data[config.AppHostname] = strings.ToLower(form.Data[config.AppHostname])
	if hostname := form.Data[config.AppHostname]; hostname != "" && !validate.IsHostname(hostname) {
		form.Errors[config.AppHostname] = "Enter a DNS name or an IPv4 address, without https:// or a path."
	}
	return form
}

// checkSetupVPN checks the second step, with the same rules as the
// OpenVPN settings.
func (s *Server) checkSetupVPN(r *http.Request) formState {
	form := s.readForm(r, config.VPNHostname, config.VPNProtocol, config.VPNPort,
		"vpn_access", config.VPNRoutes, config.VPNNameservers, config.VPNSubnet, "pki_key")
	form.Data[config.VPNHostname] = strings.ToLower(form.Data[config.VPNHostname])
	form.Data[config.VPNRoutes] = strings.Join(config.SplitLines(form.Data[config.VPNRoutes]), "\n")
	form.Data[config.VPNNameservers] = strings.Join(config.SplitLines(form.Data[config.VPNNameservers]), "\n")

	validateRequired(&form, map[string]string{
		config.VPNHostname: "A public address is required.",
		config.VPNPort:     "A port is required.",
		config.VPNSubnet:   "A subnet is required.",
	})

	errs := fieldErrors{}
	validateVPNSettings(map[string]string{
		config.VPNHostname:    form.Data[config.VPNHostname],
		config.VPNProtocol:    form.Data[config.VPNProtocol],
		config.VPNPort:        form.Data[config.VPNPort],
		config.VPNRoutes:      form.Data[config.VPNRoutes],
		config.VPNNameservers: form.Data[config.VPNNameservers],
		config.VPNSubnet:      form.Data[config.VPNSubnet],
	}, errs)
	for field, messages := range errs {
		if _, taken := form.Errors[field]; !taken {
			form.Errors[field] = messages[0]
		}
	}

	switch form.Data["vpn_access"] {
	case accessAll:
	case accessNetworks:
		if form.Data[config.VPNRoutes] == "" {
			form.Errors[config.VPNRoutes] = "Add at least one network, or send all traffic through the VPN."
		}
	default:
		form.Errors["vpn_access"] = "Choose what devices can reach."
	}

	if s.app.Config.Get(config.CACertificate) == "" {
		if _, ok := setupKeyTypes[form.Data["pki_key"]]; !ok {
			form.Errors["pki_key"] = "Choose a key type."
		}
	}
	return form
}

// finishSetup checks the administrator step and, with every answer in
// hand, sets the server up.
func (s *Server) finishSetup(w http.ResponseWriter, r *http.Request, state map[string]string) {
	ctx := r.Context()

	// Setup may only ever run once; afterwards this would be a way to mint
	// an administrator. Holding the lock until the installed flag is set
	// means a second submission waits and then sees it.
	s.installing.Lock()
	defer s.installing.Unlock()
	if s.setupFinished(w, r) {
		return
	}

	form := s.readForm(r, "admin_name", "admin_email", "admin_password", "admin_password_confirm")
	password := form.Data["admin_password"]
	form.Data["admin_email"] = strings.ToLower(form.Data["admin_email"])
	validateFullName(&form, "admin_name")
	validateRequired(&form, map[string]string{
		"admin_email":    "An email address is required.",
		"admin_password": "A password is required.",
	})
	if email := form.Data["admin_email"]; email != "" && !validate.IsEmail(email) {
		form.Errors["admin_email"] = "A valid email address is required."
	}
	s.validatePasswordPair(&form, "admin_password", "admin_password_confirm")

	// Drop the passwords before anything is stored: a rejected form is
	// written to the session, which is kept in the database.
	form.redact("admin_password", "admin_password_confirm")

	if !form.Valid() {
		s.putForm(r, form)
		http.Redirect(w, r, "/install/admin", http.StatusFound)
		return
	}

	// The code is checked again: it may have been reissued since the first
	// step.
	if !s.setupTokenMatches(state["setup_token"]) {
		delete(state, "done_organization")
		s.putSetupState(ctx, state)
		s.PutError(ctx, "The setup code has changed. Enter the one \"mangle-vpn install\" printed most recently.")
		http.Redirect(w, r, "/install/organization", http.StatusFound)
		return
	}

	admin, summary, err := s.install(ctx, state, form.Data["admin_name"], form.Data["admin_email"], password)
	if err == nil {
		// Only now is the application open for use.
		err = s.app.Config.SetBool(ctx, config.AppInstalled, true)
	}
	if err != nil {
		s.app.Log.Error("setup failed", "err", err)
		s.PutError(ctx, "Setup could not be completed. The server's log has the details.")
		s.putForm(r, form)
		http.Redirect(w, r, "/install/admin", http.StatusFound)
		return
	}

	// The code has done its job; leaving it would only be something else
	// to protect.
	if err := os.Remove(s.app.Paths.SetupToken); err != nil && !errors.Is(err, fs.ErrNotExist) {
		s.app.Log.Error("failed to remove the setup code", "err", err)
	}

	s.app.Log.Info("setup completed", "admin", admin.Email)
	s.app.RecordEvent(ctx, admin, model.EventAdminSettings, "Completed setup.")

	summary.VPNRunning = s.startVPNAfterSetup(ctx)

	// Signing in starts a fresh session, so the summary goes in after.
	if err := s.SignIn(w, r, admin); err != nil {
		s.app.Log.Error("failed to sign in the new administrator", "err", err)
	}
	if encoded, err := json.Marshal(summary); err == nil {
		s.sessions.Put(ctx, sessionSetupDone, string(encoded))
	}
	http.Redirect(w, r, "/install/done", http.StatusFound)
}

// install applies the wizard's answers: the settings, the certificate
// authority and the OpenVPN server's keys, the default group, and the first
// administrator.
func (s *Server) install(ctx context.Context, state map[string]string, name, email, password string) (*model.User, setupSummary, error) {
	var summary setupSummary
	cfg := s.app.Config

	fullTunnel := state["vpn_access"] == accessAll
	routes := state[config.VPNRoutes]
	if fullTunnel {
		routes = ""
	}
	settings := map[string]string{
		config.AppOrganization: state[config.AppOrganization],
		config.AppHostname:     state[config.AppHostname],
		config.VPNHostname:     state[config.VPNHostname],
		config.VPNProtocol:     state[config.VPNProtocol],
		config.VPNPort:         state[config.VPNPort],
		config.VPNSubnet:       state[config.VPNSubnet],
		config.VPNRoutes:       routes,
		config.VPNNameservers:  state[config.VPNNameservers],
		config.VPNRedirectGW:   boolSetting(fullTunnel),
	}
	for name, value := range settings {
		if err := cfg.Set(ctx, name, value); err != nil {
			return nil, summary, err
		}
	}

	// An authority imported from an earlier release is kept: replacing it
	// would strand every device it has issued.
	if cfg.Get(config.CACertificate) == "" {
		opts := setupKeyTypes[state["pki_key"]]
		opts.Name = state[config.AppOrganization] + " VPN CA"
		if err := s.app.CreateAuthorityWith(ctx, opts); err != nil {
			return nil, summary, err
		}
		summary.AuthorityKey = setupKeyLabels[state["pki_key"]]
	} else {
		summary.KeptAuthority = true
	}
	if err := s.app.WriteCRL(ctx); err != nil {
		return nil, summary, err
	}
	if err := s.app.CreateVPNKeys(ctx); err != nil {
		return nil, summary, err
	}
	if ca, err := s.app.Authority(); err == nil {
		summary.AuthorityName = ca.Certificate.Subject.CommonName
		summary.AuthorityUntil = ca.Certificate.NotAfter
	}

	group, err := s.app.Store.Groups.ByName(ctx, "Default")
	if errors.Is(err, store.ErrNotFound) {
		group = &model.Group{
			Name:        "Default",
			Description: "the default group that contains all users.",
			IsEnabled:   true,
			MaxDevices:  1,
			MFAEnforced: true,
		}
		err = s.app.SaveGroup(ctx, group)
	}
	if err != nil {
		return nil, summary, err
	}

	admin := &model.User{
		Name:      name,
		Email:     email,
		GroupID:   group.ID,
		Group:     group,
		IsAdmin:   true,
		IsEnabled: true,
	}
	admin.SetPassword(password)
	if err := s.app.SaveUser(ctx, admin); err != nil {
		return nil, summary, err
	}

	summary.VPNAddress = state[config.VPNHostname] + ", " + strings.ToUpper(state[config.VPNProtocol]) + " port " + state[config.VPNPort]
	summary.FullTunnel = fullTunnel
	summary.Routes = config.SplitLines(routes)
	summary.MFARequired = admin.MFARequired()
	return admin, summary, nil
}

// startVPNAfterSetup starts OpenVPN with the new settings, so the server is
// ready for devices once setup is done. It reports whether it is running.
func (s *Server) startVPNAfterSetup(ctx context.Context) bool {
	s.markVPNStoppedByAdmin(ctx, false)
	if openvpn.IsRunning(ctx) {
		openvpn.Restart(ctx)
	} else {
		openvpn.Start(ctx)
	}
	time.Sleep(settleDelay)
	running := openvpn.IsRunning(ctx)
	if !running {
		s.app.Log.Error("OpenVPN did not start after setup")
	}
	return running
}

// showSetupDone reports what setup did and what to do next.
func (s *Server) showSetupDone(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	stored := s.sessions.GetString(ctx, sessionSetupDone)
	var summary setupSummary
	if !s.app.Config.Bool(config.AppInstalled, false) || currentUser(r) == nil ||
		stored == "" || json.Unmarshal([]byte(stored), &summary) != nil {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	data := s.newPageData(w, r)
	data.SetupDone = &summary
	s.render(w, r, "InstallDone.html", data)
}

// setupDefaults is what a step starts with: the answers given so far, and
// guesses for the rest.
func (s *Server) setupDefaults(r *http.Request, state map[string]string) map[string]string {
	data := maps.Clone(state)
	delete(data, "setup_token")

	// The address this page was reached on is the likeliest web address,
	// and the VPN is most often on the same machine.
	if data[config.AppHostname] == "" {
		if host, _, err := net.SplitHostPort(r.Host); err == nil {
			data[config.AppHostname] = host
		} else {
			data[config.AppHostname] = r.Host
		}
	}
	defaults := map[string]string{
		config.VPNHostname:    data[config.AppHostname],
		config.VPNProtocol:    s.app.Config.GetOr(config.VPNProtocol, "udp"),
		config.VPNPort:        s.app.Config.GetOr(config.VPNPort, "1194"),
		config.VPNSubnet:      s.app.Config.GetOr(config.VPNSubnet, "172.25.0.0/16"),
		config.VPNRoutes:      s.app.Config.Get(config.VPNRoutes),
		config.VPNNameservers: s.app.Config.Get(config.VPNNameservers),
		"vpn_access":          accessNetworks,
		"pki_key":             "ecdsa",
	}
	for name, value := range defaults {
		if data[name] == "" {
			data[name] = value
		}
	}
	return data
}

// boolSetting writes a boolean the way the settings store them.
func boolSetting(value bool) string {
	if value {
		return "True"
	}
	return "False"
}

// setupStepIndex returns the position of the named step, or -1.
func setupStepIndex(slug string) int {
	return slices.IndexFunc(setupSteps, func(step setupStep) bool { return step.Slug == slug })
}

// firstUnfinishedStep returns the position of the first step not yet
// answered. The last step is never recorded as answered: it finishes setup.
func firstUnfinishedStep(state map[string]string) int {
	for i, step := range setupSteps[:len(setupSteps)-1] {
		if state["done_"+step.Slug] == "" {
			return i
		}
	}
	return len(setupSteps) - 1
}

// setupState returns the wizard's answers so far.
func (s *Server) setupState(ctx context.Context) map[string]string {
	state := map[string]string{}
	if stored := s.sessions.GetString(ctx, sessionSetup); stored != "" {
		if err := json.Unmarshal([]byte(stored), &state); err != nil {
			s.app.Log.Error("failed to read the setup answers", "err", err)
		}
	}
	return state
}

// putSetupState stores the wizard's answers so far.
func (s *Server) putSetupState(ctx context.Context, state map[string]string) {
	encoded, err := json.Marshal(state)
	if err != nil {
		s.app.Log.Error("failed to store the setup answers", "err", err)
		return
	}
	s.sessions.Put(ctx, sessionSetup, string(encoded))
}
