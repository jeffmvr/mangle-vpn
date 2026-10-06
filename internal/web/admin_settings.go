package web

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/netip"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jeffmvr/mangle-vpn/internal/app"
	"github.com/jeffmvr/mangle-vpn/internal/host"

	"github.com/jeffmvr/mangle-vpn/internal/config"
	"github.com/jeffmvr/mangle-vpn/internal/mailer"
	"github.com/jeffmvr/mangle-vpn/internal/model"
	"github.com/jeffmvr/mangle-vpn/internal/openvpn"
	"github.com/jeffmvr/mangle-vpn/internal/provision"
	"github.com/jeffmvr/mangle-vpn/internal/validate"
)

// Settings are grouped into the sections the settings page has tabs for.
const (
	sectionApp      = "app"
	sectionAuth     = "auth"
	sectionMail     = "mail"
	sectionVPN      = "vpn"
	sectionAlerts   = "alerts"
	sectionSecurity = "security"
	sectionBackups  = "backups"
)

// sectionFields lists the settings each section owns. A request may only
// read or write the fields of the section it names.
var sectionFields = map[string][]string{
	sectionApp: {
		config.AppHostname, config.AppHTTPPort, config.AppHTTPSPort,
		config.AppOrganization, config.AppSSLCertificate, config.AppSSLPrivateKey,
		config.AppEventRetentionDays, config.AppLetsEncrypt, config.AppACMEEmail,
		config.AppSignInNotice, config.AppLogo, config.AppSupportContact,
	},
	sectionAuth: {
		config.OAuth2Provider, config.OAuth2ClientID, config.OAuth2ClientSecret,
		config.OAuth2AllowedDomain, config.OAuth2Issuer, config.OAuth2Name,
		config.OAuth2Only, config.OAuth2AutoGroup,
	},
	sectionMail: {
		config.SMTPHost, config.SMTPPort, config.SMTPUsername,
		config.SMTPPassword, config.SMTPReplyAddress,
	},
	sectionVPN: {
		config.VPNDomain, config.VPNHostname, config.VPNInterface,
		config.VPNNameservers, config.VPNNATInterface, config.VPNPort,
		config.VPNProtocol, config.VPNRoutes, config.VPNRedirectGW, config.VPNSubnet,
		config.VPNSessionHours, config.VPNIdleMinutes, config.VPNDeviceToDevice, config.VPNLogLevel,
		config.VPNRequirePassword, config.VPNPortShare, config.VPNMSSFix,
		config.VPNSingleSession, config.PKIDeviceDays,
	},
	sectionAlerts: {
		config.AlertEmails, config.AlertWebhook,
		config.AlertVPNDown, config.AlertCertificates, config.AlertLockouts,
		config.AlertNewDevice, config.AlertAdminSignIn, config.AlertNewAddress,
		config.AlertDailyDigest,
	},
	sectionSecurity: {
		config.AuthLockoutAttempts, config.AuthLockoutMinutes,
		config.AuthSessionIdleMinutes, config.AuthSessionHours,
		config.AuthPasswordMinLength, config.AuthPasswordComplexity,
		config.AuthAdminNetworks,
	},
	sectionBackups: {
		config.BackupKeep, config.BackupS3Endpoint, config.BackupS3Region,
		config.BackupS3Bucket, config.BackupS3Prefix, config.BackupS3AccessKey,
		config.BackupS3SecretKey,
	},
}

// sectionNames are what the audit log calls each section.
var sectionNames = map[string]string{
	sectionApp:      "general",
	sectionAuth:     "single sign-on",
	sectionMail:     "email",
	sectionVPN:      "OpenVPN",
	sectionAlerts:   "alert",
	sectionSecurity: "security",
	sectionBackups:  "backup",
}

// changedSettings names the submitted settings whose values differ from
// those stored, for the audit log. The values themselves are left out:
// some are secrets, and the names are what someone reviewing the log needs.
func (s *Server) changedSettings(submitted map[string]string) []string {
	var changed []string
	for _, field := range slices.Sorted(maps.Keys(submitted)) {
		value := submitted[field]
		// The TLS pair is only ever sent to replace what is on disk.
		isTLS := field == config.AppSSLCertificate || field == config.AppSSLPrivateKey
		if (isTLS && value != "") || (!isTLS && value != s.app.Config.Get(field)) {
			changed = append(changed, settingLabel(field))
		}
	}
	return changed
}

// settingLabels name settings the way the settings page does.
var settingLabels = map[string]string{
	config.AppHostname:            "hostname",
	config.AppHTTPPort:            "HTTP port",
	config.AppHTTPSPort:           "HTTPS port",
	config.AppOrganization:        "organization name",
	config.AppSSLCertificate:      "TLS certificate",
	config.AppSSLPrivateKey:       "TLS private key",
	config.AppEventRetentionDays:  "audit log retention",
	config.AppLetsEncrypt:         "Let's Encrypt",
	config.AppACMEEmail:           "Let's Encrypt contact",
	config.OAuth2Provider:         "provider",
	config.OAuth2ClientID:         "client ID",
	config.OAuth2ClientSecret:     "client secret",
	config.OAuth2AllowedDomain:    "allowed domain",
	config.OAuth2Issuer:           "issuer",
	config.OAuth2Name:             "provider name",
	config.SMTPHost:               "SMTP server",
	config.SMTPPort:               "SMTP port",
	config.SMTPUsername:           "SMTP username",
	config.SMTPPassword:           "SMTP password",
	config.SMTPReplyAddress:       "reply-to address",
	config.VPNDomain:              "search domain",
	config.VPNHostname:            "hostname",
	config.VPNInterface:           "listen interface",
	config.VPNNameservers:         "DNS servers",
	config.VPNNATInterface:        "NAT interface",
	config.VPNPort:                "port",
	config.VPNProtocol:            "protocol",
	config.VPNRoutes:              "routes",
	config.VPNRedirectGW:          "send all traffic",
	config.VPNSubnet:              "client subnet",
	config.VPNSessionHours:        "code interval",
	config.VPNIdleMinutes:         "idle timeout",
	config.VPNDeviceToDevice:      "device to device",
	config.VPNLogLevel:            "log detail",
	config.VPNRequirePassword:     "account password",
	config.AlertEmails:            "alert addresses",
	config.AlertWebhook:           "webhook",
	config.AlertVPNDown:           "OpenVPN stopping",
	config.AlertCertificates:      "expiring certificates",
	config.AlertLockouts:          "lockouts",
	config.AlertNewDevice:         "new devices",
	config.AlertAdminSignIn:       "administrator sign-ins",
	config.AlertNewAddress:        "sign-ins from new addresses",
	config.AlertDailyDigest:       "daily summary",
	config.AppSignInNotice:        "sign-in notice",
	config.AppLogo:                "logo",
	config.AppSupportContact:      "help contact",
	config.OAuth2Only:             "single sign-on only",
	config.OAuth2AutoGroup:        "accounts on first sign-in",
	config.VPNPortShare:           "share port 443",
	config.VPNMSSFix:              "packet size",
	config.VPNSingleSession:       "one connection per person",
	config.PKIDeviceDays:          "device certificate lifetime",
	config.AuthLockoutAttempts:    "lockout attempts",
	config.AuthLockoutMinutes:     "lockout length",
	config.AuthSessionIdleMinutes: "session idle time",
	config.AuthSessionHours:       "session length",
	config.AuthPasswordMinLength:  "minimum password length",
	config.AuthPasswordComplexity: "mixed characters",
	config.AuthAdminNetworks:      "administration networks",
	config.BackupKeep:             "backups kept",
	config.BackupS3Endpoint:       "S3 endpoint",
	config.BackupS3Region:         "S3 region",
	config.BackupS3Bucket:         "S3 bucket",
	config.BackupS3Prefix:         "S3 prefix",
	config.BackupS3AccessKey:      "S3 access key",
	config.BackupS3SecretKey:      "S3 secret key",
}

// settingLabel returns the name a setting is shown under.
func settingLabel(field string) string {
	if label, ok := settingLabels[field]; ok {
		return label
	}
	return field
}

// secretSettings are never sent back to the browser. The settings page shows
// whether each one is set, through "<name>_set", and a blank value submitted
// for one keeps what is stored.
var secretSettings = []string{config.SMTPPassword, config.OAuth2ClientSecret, config.AlertWebhook,
	config.BackupS3SecretKey}

// adminGetSettings returns the settings in one section.
func (s *Server) adminGetSettings(w http.ResponseWriter, r *http.Request) {
	fields, ok := sectionFields[r.PathValue("section")]
	if !ok {
		s.notFound(w)
		return
	}

	settings := make(map[string]any, len(fields)+1)
	for _, field := range fields {
		// The TLS material is kept on disk rather than in the database, so
		// there is nothing to show; the fields are offered empty for
		// pasting a replacement into.
		switch {
		case slices.Contains(secretSettings, field):
			settings[field] = ""
			settings[field+"_set"] = s.app.Config.Get(field) != ""
		case field == config.AppSSLCertificate, field == config.AppSSLPrivateKey:
			settings[field] = ""
		default:
			settings[field] = s.app.Config.Get(field)
		}
	}

	// The address single sign-on returns to, which has to be registered
	// with the provider exactly.
	if r.PathValue("section") == sectionAuth {
		settings["oauth2_redirect_uri"] = s.app.Config.URL("oauth")
	}

	// The interface chooser needs to know what the machine has.
	interfaces := map[string]string{}
	for _, name := range host.InterfaceNames("lo", "tun") {
		interfaces[name] = host.InterfaceIP(name)
	}
	settings["interfaces"] = interfaces

	s.writeJSON(w, http.StatusOK, settings)
}

// adminUpdateSettings validates and stores the settings in one section.
func (s *Server) adminUpdateSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	section := r.PathValue("section")

	fields, ok := sectionFields[section]
	if !ok {
		s.notFound(w)
		return
	}

	submitted, ok := s.readSettings(w, r, fields)
	if !ok {
		return
	}

	// The page never had the stored secret, so a blank one means "keep it".
	for _, field := range secretSettings {
		if value, sent := submitted[field]; sent && value == "" {
			submitted[field] = s.app.Config.Get(field)
		}
	}

	errs := fieldErrors{}
	switch section {
	case sectionApp:
		validateAppSettings(submitted, errs)
	case sectionAuth:
		validateAuthSettings(submitted, errs)
		if errs.empty() && submitted[config.OAuth2Provider] == ProviderOIDC {
			// A typo in the issuer would otherwise only show when someone
			// tries to sign in.
			if _, err := discoverOIDC(ctx, submitted[config.OAuth2Issuer]); err != nil {
				s.app.Log.Info("OpenID Connect discovery failed", "err", err)
				errs.add(config.OAuth2Issuer, "The provider's discovery document could not be read from this address.")
			}
		}
	case sectionMail:
		validateMailSettings(submitted, errs)
	case sectionVPN:
		validateVPNSettings(submitted, errs)
	case sectionAlerts:
		validateAlertSettings(submitted, errs)
	case sectionSecurity:
		validateSecuritySettings(submitted, errs)
		// Saving a list the administrator is outside of would end their
		// own access with this very request.
		if errs.empty() && !s.networksInclude(submitted[config.AuthAdminNetworks], s.clientIP(r)) {
			errs.add(config.AuthAdminNetworks, fmt.Sprintf(
				"Your own address, %s, isn't in these networks, so saving would lock you out.", s.clientIP(r)))
		}
	case sectionBackups:
		validateBackupSettings(submitted, errs)
	}
	s.validateAcrossSections(section, submitted, errs)
	if !errs.empty() {
		s.invalid(w, errs)
		return
	}

	changed := s.changedSettings(submitted)
	if err := s.saveSettings(ctx, section, submitted); err != nil {
		s.serverError(w, "failed to save settings", err)
		return
	}

	if len(changed) > 0 {
		s.audit(r, model.EventAdminSettings, "Changed %s settings: %s.",
			sectionNames[section], strings.Join(changed, ", "))
	}
	s.noContent(w)
}

// readSettings pulls the fields belonging to a section out of the request
// body, ignoring anything else the frontend sends back.
//
// Every setting is stored as text, but the frontend may hand back a number
// or a boolean where it read one, so values are normalised here.
func (s *Server) readSettings(w http.ResponseWriter, r *http.Request, fields []string) (map[string]string, bool) {
	var body map[string]any
	if !s.decodeJSON(w, r, &body) {
		return nil, false
	}

	submitted := make(map[string]string, len(fields))
	for _, field := range fields {
		if value, ok := body[field]; ok {
			submitted[field] = settingToString(value)
		}
	}
	return submitted, true
}

// settingToString renders a submitted value as the text that is stored.
func settingToString(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	case bool:
		if v {
			return "True"
		}
		return "False"
	case float64:
		// JSON numbers arrive as floats; settings are only ever whole.
		return fmt.Sprintf("%d", int64(v))
	default:
		encoded, _ := json.Marshal(v)
		return string(encoded)
	}
}

// saveSettings stores a validated section and applies whatever has to
// happen outside the database for it to take effect.
func (s *Server) saveSettings(ctx context.Context, section string, submitted map[string]string) error {
	// The TLS material never goes into the database; it is written to the
	// files the server reads its certificate from.
	certificate, privateKey := submitted[config.AppSSLCertificate], submitted[config.AppSSLPrivateKey]
	delete(submitted, config.AppSSLCertificate)
	delete(submitted, config.AppSSLPrivateKey)

	before := s.listenerSettings()

	for name, value := range submitted {
		if err := s.app.Config.Set(ctx, name, value); err != nil {
			return err
		}
	}

	switch section {
	case sectionApp:
		// The running server picks the new certificate up on the next
		// connection, so there is nothing to reload.
		if certificate != "" && privateKey != "" {
			if err := provision.WriteTLSMaterial(s.app.Paths, certificate, privateKey); err != nil {
				return err
			}
		}

		// Ports and the certificate source are set up when the service
		// starts, so it has to come back for a change to take effect. The
		// restart drops this connection, so it happens once the response
		// has been sent.
		if after := s.listenerSettings(); after != before {
			s.app.Log.Info("listener settings changed, restarting", "settings", after)
			go s.restartWebService(context.WithoutCancel(ctx))
		}

	case sectionVPN:
		// The running server keeps its old configuration until it is
		// restarted, so flag that for the administrator.
		return s.app.Config.SetBool(ctx, config.VPNRestartPending, true)
	}
	return nil
}

// listenerSettings collects the settings the web service reads when it
// starts, to tell whether saving changed any of them.
func (s *Server) listenerSettings() string {
	cfg := s.app.Config
	values := []string{
		strconv.Itoa(cfg.Int(config.AppHTTPPort, 80)),
		strconv.Itoa(cfg.Int(config.AppHTTPSPort, 443)),
		strconv.FormatBool(cfg.Bool(config.AppLetsEncrypt, false)),
		cfg.Get(config.AuthSessionIdleMinutes),
		cfg.Get(config.AuthSessionHours),
	}
	// The hostname and contact only matter to Let's Encrypt.
	if cfg.Bool(config.AppLetsEncrypt, false) {
		values = append(values, cfg.Get(config.AppHostname), cfg.Get(config.AppACMEEmail))
	}
	return strings.Join(values, " ")
}

//
// Per-section validation
//

// validateAppSettings checks the web application settings.
func validateAppSettings(submitted map[string]string, errs fieldErrors) {
	requireHostname(submitted, errs, config.AppHostname)
	requirePort(submitted, errs, config.AppHTTPPort)
	requirePort(submitted, errs, config.AppHTTPSPort)

	requireChoice(submitted, errs, config.AppEventRetentionDays, eventRetentionDays,
		"Choose how long to keep the audit log.")

	requireChoice(submitted, errs, config.AppLetsEncrypt, []string{"True", "False"}, "Must be on or off.")
	if email := submitted[config.AppACMEEmail]; email != "" && !validate.IsEmail(email) {
		errs.add(config.AppACMEEmail, "A valid email address is required.")
	}
	// Let's Encrypt issues certificates for names, so the hostname has to be
	// one: a DNS name that points at this server.
	if submitted[config.AppLetsEncrypt] == "True" {
		if hostname, ok := submitted[config.AppHostname]; ok && !validate.IsDomain(strings.ToLower(hostname)) {
			errs.add(config.AppHostname, "Let's Encrypt needs a DNS name that points at this server, not an IP address.")
		}
	}

	if organization, ok := submitted[config.AppOrganization]; ok && organization == "" {
		errs.add(config.AppOrganization, "This field is required.")
	}

	if notice := submitted[config.AppSignInNotice]; len(notice) > 1000 {
		errs.add(config.AppSignInNotice, "Keep the notice under 1,000 characters.")
	}
	if contact := submitted[config.AppSupportContact]; contact != "" && !validate.IsEmail(contact) && !isHTTPSURL(contact) {
		errs.add(config.AppSupportContact, "Enter an email address or a URL starting with https://.")
	}
	if logo := submitted[config.AppLogo]; logo != "" {
		if _, _, err := decodeLogo(logo); err != nil {
			errs.add(config.AppLogo, err.Error())
		}
	}

	// A certificate is only usable with the matching key, so neither may be
	// supplied on its own.
	certificate := submitted[config.AppSSLCertificate]
	privateKey := submitted[config.AppSSLPrivateKey]
	switch {
	case certificate != "" && privateKey == "":
		errs.add(config.AppSSLPrivateKey, "A private key is required with a certificate.")
	case privateKey != "" && certificate == "":
		errs.add(config.AppSSLCertificate, "A certificate is required with a private key.")
	case certificate != "":
		// Saved as it is, a bad pair would only fail at the next restart,
		// leaving the web interface down until it is fixed over SSH.
		if _, err := tls.X509KeyPair([]byte(certificate), []byte(privateKey)); err != nil {
			errs.add(config.AppSSLCertificate,
				"The certificate and private key are not a valid PEM pair that belong together.")
		}
	}
}

// validateAuthSettings checks the single sign-on settings.
func validateAuthSettings(submitted map[string]string, errs fieldErrors) {
	provider, ok := submitted[config.OAuth2Provider]
	if !ok {
		return
	}

	if provider == "" || provider == config.NoOAuth2Provider {
		// Switching single sign-on off clears the credentials rather than
		// leaving them behind in the database.
		submitted[config.OAuth2Provider] = config.NoOAuth2Provider
		submitted[config.OAuth2ClientID] = ""
		submitted[config.OAuth2ClientSecret] = ""
		submitted[config.OAuth2AllowedDomain] = ""
		submitted[config.OAuth2Issuer] = ""
		submitted[config.OAuth2Name] = ""
		submitted[config.OAuth2Only] = "False"
		submitted[config.OAuth2AutoGroup] = ""
		return
	}

	requireChoice(submitted, errs, config.OAuth2Only, []string{"True", "False"}, "Must be on or off.")
	// Creating accounts for whoever signs in is only safe when "whoever"
	// is limited to the organization's own domain.
	if submitted[config.OAuth2AutoGroup] != "" && submitted[config.OAuth2AllowedDomain] == "" {
		errs.add(config.OAuth2AutoGroup, "Limit sign-in to your domain first, or anyone with an account at the provider would get one here.")
	}

	if domain := submitted[config.OAuth2AllowedDomain]; domain != "" {
		submitted[config.OAuth2AllowedDomain] = strings.ToLower(domain)
		if !validate.IsDomain(strings.ToLower(domain)) {
			errs.add(config.OAuth2AllowedDomain, "A valid domain, such as example.com, is required.")
		}
	}

	switch provider {
	case ProviderGoogle:
		// Google's endpoints are built in.
		submitted[config.OAuth2Issuer] = ""
		submitted[config.OAuth2Name] = ""
	case ProviderOIDC:
		// Kept exactly as typed: the provider's discovery document has to
		// name the same issuer, trailing slash and all.
		if !validIssuer(submitted[config.OAuth2Issuer]) {
			errs.add(config.OAuth2Issuer, "An https:// issuer URL is required, such as https://example.okta.com.")
		}
		switch name := submitted[config.OAuth2Name]; {
		case name == "":
			errs.add(config.OAuth2Name, "Name the provider, as the sign in button should show it.")
		case len(name) > 40:
			errs.add(config.OAuth2Name, "Keep the name under 40 characters.")
		}
	default:
		errs.add(config.OAuth2Provider, "Unknown single sign-on provider.")
	}
	for _, field := range []string{config.OAuth2ClientID, config.OAuth2ClientSecret} {
		if submitted[field] == "" {
			errs.add(field, "This field is required.")
		}
	}
}

// validateMailSettings checks the outgoing mail settings.
func validateMailSettings(submitted map[string]string, errs fieldErrors) {
	// Mail is optional, so an entirely blank form switches it off rather
	// than failing.
	if submitted[config.SMTPHost] == "" && submitted[config.SMTPUsername] == "" {
		return
	}

	requireHostname(submitted, errs, config.SMTPHost)
	requirePort(submitted, errs, config.SMTPPort)

	if reply, ok := submitted[config.SMTPReplyAddress]; ok && reply != "" && !validate.IsEmail(reply) {
		errs.add(config.SMTPReplyAddress, "A valid e-mail address is required.")
	}
}

// validateVPNSettings checks the OpenVPN server settings.
func validateVPNSettings(submitted map[string]string, errs fieldErrors) {
	requireHostname(submitted, errs, config.VPNHostname)
	requirePort(submitted, errs, config.VPNPort)

	available := host.InterfaceNames()
	for _, field := range []string{config.VPNInterface, config.VPNNATInterface} {
		if name, ok := submitted[field]; ok && !slices.Contains(available, name) {
			errs.add(field, "The interface does not exist.")
		}
	}

	if protocol, ok := submitted[config.VPNProtocol]; ok {
		if protocol != model.ProtocolTCP && protocol != model.ProtocolUDP {
			errs.add(config.VPNProtocol, "Protocol must be TCP or UDP.")
		}
	}

	// The subnet is handed to OpenVPN as an address and mask pair, so it
	// has to be a network address with no host bits set.
	// It is also divided into a pool and a range for fixed addresses,
	// which needs room for both.
	if subnet, ok := submitted[config.VPNSubnet]; ok {
		if _, err := openvpn.PlanSubnet(subnet); err != nil {
			errs.add(config.VPNSubnet, "Subnet must be an IPv4 network address between /8 and /29, such as 172.25.0.0/16.")
		}
	}

	for _, line := range config.SplitLines(submitted[config.VPNNameservers]) {
		if !validate.IsIP(line) {
			errs.add(config.VPNNameservers, "There are one or more invalid DNS servers.")
			break
		}
	}
	for _, line := range config.SplitLines(submitted[config.VPNRoutes]) {
		if !validate.IsCIDR(line) || openvpn.ExpandCIDR(line) == "" {
			errs.add(config.VPNRoutes, "There are one or more invalid routes.")
			break
		}
	}

	if domain, ok := submitted[config.VPNDomain]; ok && domain != "" && !validate.IsDomain(domain) {
		errs.add(config.VPNDomain, "A valid DNS domain is required.")
	}

	requireChoice(submitted, errs, config.VPNSessionHours, vpnSessionHours,
		"Choose how often to ask for a new code.")
	requireChoice(submitted, errs, config.VPNIdleMinutes, vpnIdleMinutes,
		"Choose an idle timeout.")
	requireChoice(submitted, errs, config.VPNLogLevel, vpnLogLevels,
		"Choose a log level.")
	for _, field := range []string{config.VPNRedirectGW, config.VPNDeviceToDevice, config.VPNRequirePassword,
		config.VPNPortShare, config.VPNSingleSession} {
		requireChoice(submitted, errs, field, []string{"True", "False"}, "Must be on or off.")
	}
	requireChoice(submitted, errs, config.VPNMSSFix, vpnMSSFixes, "Choose a packet size.")
	requireChoice(submitted, errs, config.PKIDeviceDays, deviceDays, "Choose how long device certificates last.")

	// Sharing the port means OpenVPN takes TCP 443 and hands the web server
	// everything else, so the web server has to listen somewhere else.
	if submitted[config.VPNPortShare] == "True" {
		if submitted[config.VPNProtocol] != model.ProtocolTCP || submitted[config.VPNPort] != "443" {
			errs.add(config.VPNPortShare, "Set the protocol to TCP and the port to 443 to share it with the web server.")
		}
	}
}

// validateAlertSettings checks where alerts go and what they are sent for.
func validateAlertSettings(submitted map[string]string, errs fieldErrors) {
	if emails, ok := submitted[config.AlertEmails]; ok {
		addresses := strings.Fields(strings.NewReplacer(",", " ", ";", " ").Replace(emails))
		for _, address := range addresses {
			if !validate.IsEmail(address) {
				errs.add(config.AlertEmails, fmt.Sprintf("%q is not an e-mail address.", address))
				break
			}
		}
		submitted[config.AlertEmails] = strings.Join(addresses, "\n")
	}

	if hook := submitted[config.AlertWebhook]; hook != "" {
		u, err := url.Parse(hook)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
			errs.add(config.AlertWebhook, "A webhook URL starting with https:// is required.")
		}
	}

	for _, field := range []string{config.AlertVPNDown, config.AlertCertificates, config.AlertLockouts,
		config.AlertNewDevice, config.AlertAdminSignIn, config.AlertNewAddress, config.AlertDailyDigest} {
		requireChoice(submitted, errs, field, []string{"True", "False"}, "Must be on or off.")
	}
}

// The choices the security settings offer.
var (
	lockoutAttempts    = []string{"5", "10", "20"}
	lockoutMinutes     = []string{"5", "15", "30", "60"}
	sessionIdleMinutes = []string{"15", "30", "60", "240"}
	sessionHours       = []string{"8", "12", "24", "72"}
	passwordLengths    = []string{"8", "10", "12", "14", "16"}
)

// validateAcrossSections checks the combinations of settings that span
// sections, against what the other section has stored.
func (s *Server) validateAcrossSections(section string, submitted map[string]string, errs fieldErrors) {
	cfg := s.app.Config
	value := func(name string) string {
		if v, ok := submitted[name]; ok {
			return v
		}
		return cfg.Get(name)
	}

	// Port sharing hands the web server what reaches 443 on OpenVPN, so the
	// web server must listen elsewhere.
	if value(config.VPNPortShare) == "True" && value(config.AppHTTPSPort) == "443" {
		switch section {
		case sectionVPN:
			errs.add(config.VPNPortShare, "Move the web server's HTTPS port off 443 first, in General, such as to 8443.")
		case sectionApp:
			errs.add(config.AppHTTPSPort, "OpenVPN is sharing port 443 with the web server, so the web server needs another port.")
		}
	}

	// Someone who only ever signs in through the provider has no password
	// to give when connecting.
	if value(config.OAuth2Only) == "True" && value(config.VPNRequirePassword) == "True" &&
		value(config.OAuth2Provider) != config.NoOAuth2Provider {
		switch section {
		case sectionAuth:
			errs.add(config.OAuth2Only, "Turn off \"Ask for the account password too\" in OpenVPN first: people who only use single sign-on have no password to give.")
		case sectionVPN:
			errs.add(config.VPNRequirePassword, "Single sign-on is the only way to sign in, so people have no password to give when connecting.")
		}
	}
}

// validateSecuritySettings checks the sign in and session policy.
func validateSecuritySettings(submitted map[string]string, errs fieldErrors) {
	requireChoice(submitted, errs, config.AuthLockoutAttempts, lockoutAttempts, "Choose how many attempts lock an account.")
	requireChoice(submitted, errs, config.AuthLockoutMinutes, lockoutMinutes, "Choose how long a lockout lasts.")
	requireChoice(submitted, errs, config.AuthSessionIdleMinutes, sessionIdleMinutes, "Choose an idle time.")
	requireChoice(submitted, errs, config.AuthSessionHours, sessionHours, "Choose a session length.")
	requireChoice(submitted, errs, config.AuthPasswordMinLength, passwordLengths, "Choose a minimum length.")
	requireChoice(submitted, errs, config.AuthPasswordComplexity, []string{"True", "False"}, "Must be on or off.")

	if networks, ok := submitted[config.AuthAdminNetworks]; ok {
		lines := config.SplitLines(networks)
		for _, line := range lines {
			if _, err := app.ParseNetwork(line); err != nil {
				errs.add(config.AuthAdminNetworks, fmt.Sprintf("%q is not an address or a network such as 10.0.0.0/8.", line))
				break
			}
		}
		submitted[config.AuthAdminNetworks] = strings.Join(lines, "\n")
	}
}

// networksInclude reports whether address is in one of the networks, given
// one per line; no networks at all include everything.
func (s *Server) networksInclude(networks, address string) bool {
	lines := config.SplitLines(networks)
	if len(lines) == 0 {
		return true
	}
	addr, err := netip.ParseAddr(address)
	if err != nil {
		return false
	}
	for _, line := range lines {
		if prefix, err := app.ParseNetwork(line); err == nil && prefix.Contains(addr.Unmap()) {
			return true
		}
	}
	return false
}

// backupKeeps are the numbers of nightly backups that may be kept.
var backupKeeps = []string{"3", "7", "14", "30"}

// validateBackupSettings checks how many backups are kept and where the
// off-machine copy goes.
func validateBackupSettings(submitted map[string]string, errs fieldErrors) {
	requireChoice(submitted, errs, config.BackupKeep, backupKeeps, "Choose how many backups to keep.")

	endpoint := submitted[config.BackupS3Endpoint]
	if endpoint == "" {
		// No bucket: the copy off the machine is turned off, and nothing
		// about it is kept.
		for _, field := range []string{config.BackupS3Region, config.BackupS3Bucket, config.BackupS3Prefix,
			config.BackupS3AccessKey, config.BackupS3SecretKey} {
			if _, sent := submitted[field]; sent {
				submitted[field] = ""
			}
		}
		return
	}

	if u, err := url.Parse(endpoint); err != nil || u.Scheme != "https" || u.Host == "" || u.Path != "" && u.Path != "/" {
		errs.add(config.BackupS3Endpoint, "An https:// address with no path is required, such as https://s3.us-east-1.amazonaws.com.")
	}
	if bucket := submitted[config.BackupS3Bucket]; bucket == "" {
		errs.add(config.BackupS3Bucket, "This field is required.")
	}
	for _, field := range []string{config.BackupS3AccessKey, config.BackupS3SecretKey} {
		if submitted[field] == "" {
			errs.add(field, "This field is required.")
		}
	}
	submitted[config.BackupS3Prefix] = strings.Trim(submitted[config.BackupS3Prefix], "/")
}

// adminSendTestAlert sends an alert through every configured channel.
func (s *Server) adminSendTestAlert(w http.ResponseWriter, r *http.Request) {
	sent, err := s.app.SendTestAlert(r.Context())
	switch {
	case err != nil:
		s.serverError(w, "failed to queue a test alert", err)
	case !sent:
		s.invalidField(w, config.AlertEmails, "Add an e-mail address or a webhook first, and save.")
	default:
		s.noContent(w)
	}
}

// eventRetentionDays are the audit log retentions on offer; 0 keeps every
// event.
var eventRetentionDays = []string{"30", "90", "180", "365", "0"}

// The choices the settings page offers. Values are written into the OpenVPN
// configuration, so only these are accepted.
var (
	vpnMSSFixes     = []string{"0", "1400", "1360", "1300", "1200"}
	deviceDays      = []string{"0", "90", "180", "365", "730"}
	vpnSessionHours = []string{"0", "8", "12", "24", "168"}
	vpnIdleMinutes  = []string{"0", "30", "60", "240", "480"}
	vpnLogLevels    = []string{"1", "3", "4"}
)

// requireChoice records an error when a submitted field holds anything but
// one of the allowed values.
func requireChoice(submitted map[string]string, errs fieldErrors, field string, allowed []string, message string) {
	if value, ok := submitted[field]; ok && !slices.Contains(allowed, value) {
		errs.add(field, message)
	}
}

// requireHostname records an error unless the field holds a DNS name or an
// IP address.
func requireHostname(submitted map[string]string, errs fieldErrors, field string) {
	value, ok := submitted[field]
	if !ok {
		return
	}
	if !validate.IsHostname(strings.ToLower(value)) {
		errs.add(field, "Hostname must be a valid DNS hostname or IPv4 address.")
		return
	}
	submitted[field] = strings.ToLower(value)
}

// requirePort records an error unless the field holds a usable port number.
func requirePort(submitted map[string]string, errs fieldErrors, field string) {
	if value, ok := submitted[field]; ok && !validate.IsPort(value) {
		errs.add(field, "Port must be a valid TCP or UDP port.")
	}
}

//
// Test mail and updates
//

// testEmailRequest is the body of a test e-mail request.
type testEmailRequest struct {
	Email string `json:"email"`
}

// restartWebService brings the web application back on its new ports,
// after a moment for the response to reach the browser.
func (s *Server) restartWebService(ctx context.Context) {
	time.Sleep(restartDelay)
	if s.restart != nil {
		s.restart()
		return
	}
	host.Run(ctx, "systemctl", "restart", "mangle-web")
}

// restartDelay is how long to wait before a restart that was asked for by
// the request still in flight.
const restartDelay = 2 * time.Second

// adminSendTestEmail sends a message to the given address so that an
// administrator can confirm the mail settings work.
func (s *Server) adminSendTestEmail(w http.ResponseWriter, r *http.Request) {
	var body testEmailRequest
	if !s.decodeJSON(w, r, &body) {
		return
	}

	if !validate.IsEmail(body.Email) {
		s.invalidField(w, "email", "A valid e-mail address is required.")
		return
	}
	if !s.app.MailConfigured() {
		s.invalidField(w, "email", "Outgoing mail is not configured.")
		return
	}

	test := mailer.Content{HTML: "<p>It works!</p>", Text: "It works!"}
	if err := s.app.QueueEmail(r.Context(), body.Email, "Mangle VPN Test E-mail", test); err != nil {
		s.serverError(w, "failed to queue a test e-mail", err)
		return
	}
	s.noContent(w)
}
