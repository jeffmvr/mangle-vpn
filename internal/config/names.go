package config

// The names of every application setting. Settings live in the database as
// text and are cached in memory; naming them here keeps the spelling honest
// across the handlers, templates, and command line.
const (
	AppInstalled    = "app_installed"
	AppHostname     = "app_hostname"
	AppHTTPPort     = "app_http_port"
	AppHTTPSPort    = "app_https_port"
	AppOrganization = "app_organization"

	// AppEventRetentionDays is how long the audit log keeps an event; 0
	// keeps every event for good.
	AppEventRetentionDays = "app_event_retention_days"

	// AppLetsEncrypt turns on certificates from Let's Encrypt for the web
	// server, and AppACMEEmail is where Let's Encrypt sends expiry notices.
	AppLetsEncrypt = "app_letsencrypt"
	AppACMEEmail   = "app_acme_email"

	// AppSSLCertificate and AppSSLPrivateKey are accepted by the settings
	// API but written to disk rather than stored here.
	AppSSLCertificate = "app_ssl_crt"
	AppSSLPrivateKey  = "app_ssl_key"

	// AppSignInNotice is shown on the sign in page, such as an "authorized
	// use only" warning; AppLogo is an image, as a data URL, shown in place
	// of the shield beside the organization's name.
	AppSignInNotice = "app_signin_notice"
	AppLogo         = "app_logo"

	// AppSupportContact is where people who cannot sign in go for help: an
	// email address or an https URL, linked from the sign in page.
	AppSupportContact = "app_support_contact"

	// Sign in and session policy. Lockouts follow AuthLockoutAttempts wrong
	// passwords or codes in a row and last AuthLockoutMinutes; a web
	// session ends after AuthSessionIdleMinutes without a request or
	// AuthSessionHours in all; passwords are at least
	// AuthPasswordMinLength long, with a lowercase and an uppercase letter
	// and a digit when AuthPasswordComplexity is on. AuthAdminNetworks, one
	// CIDR network per line, limits the administration API to those
	// addresses.
	AuthLockoutAttempts    = "auth_lockout_attempts"
	AuthLockoutMinutes     = "auth_lockout_minutes"
	AuthSessionIdleMinutes = "auth_session_idle_minutes"
	AuthSessionHours       = "auth_session_hours"
	AuthPasswordMinLength  = "auth_password_min_length"
	AuthPasswordComplexity = "auth_password_complexity"
	AuthAdminNetworks      = "auth_admin_networks"

	CACertificate = "ca_crt"
	CAPrivateKey  = "ca_key"

	OAuth2Provider     = "oauth2_provider"
	OAuth2ClientID     = "oauth2_client_id"
	OAuth2ClientSecret = "oauth2_client_secret"

	// OAuth2AllowedDomain, when set, limits single sign-on to accounts of
	// that domain: a Google Workspace domain, or the domain of the address
	// an OpenID Connect provider vouches for.
	OAuth2AllowedDomain = "oauth2_allowed_domain"

	// OAuth2Issuer is the issuer URL of an OpenID Connect provider, whose
	// endpoints are discovered from it, and OAuth2Name is what the sign in
	// button calls the provider.
	OAuth2Issuer = "oauth2_issuer"
	OAuth2Name   = "oauth2_name"

	// OAuth2Only turns off sign in with a password for everyone but
	// administrators. OAuth2AutoGroup, a group's ID, creates an account in
	// that group for anyone from the allowed domain who signs in through
	// the provider without one.
	OAuth2Only      = "oauth2_only"
	OAuth2AutoGroup = "oauth2_auto_group"

	PKIKeySize = "pki_key_size"

	// PKIDeviceDays is how long a device certificate lasts, in days; 0 lets
	// it last as long as the certificate authority.
	PKIDeviceDays = "pki_device_days"

	SMTPHost         = "smtp_host"
	SMTPPort         = "smtp_port"
	SMTPUsername     = "smtp_username"
	SMTPPassword     = "smtp_password"
	SMTPReplyAddress = "smtp_reply_address"
	SMTPTLS          = "smtp_tls"

	VPNCertificate     = "vpn_crt"
	VPNPrivateKey      = "vpn_key"
	VPNDeviceToDevice  = "vpn_device_to_device"
	VPNDomain          = "vpn_domain"
	VPNIdleMinutes     = "vpn_idle_minutes"
	VPNLogLevel        = "vpn_log_level"
	VPNSessionHours    = "vpn_session_hours"
	VPNFirewallRules   = "vpn_firewall_rules"
	VPNHostname        = "vpn_hostname"
	VPNInterface       = "vpn_interface"
	VPNNATInterface    = "vpn_nat_interface"
	VPNNameservers     = "vpn_nameservers"
	VPNPort            = "vpn_port"
	VPNProtocol        = "vpn_protocol"
	VPNRedirectGW      = "vpn_redirect_gateway"
	VPNRequirePassword = "vpn_require_password"
	VPNRestartPending  = "vpn_restart_pending"
	VPNRoutes          = "vpn_routes"
	VPNSubnet          = "vpn_subnet"
	VPNTLSAuthKey      = "vpn_tls_auth_key"

	// VPNPortShare has OpenVPN, on TCP 443, hand anything that is not a VPN
	// connection to the web server. VPNMSSFix lowers the largest packet
	// sent through the tunnel, 0 leaving OpenVPN's default. VPNSingleSession
	// lets each person be connected from one device at a time.
	VPNPortShare     = "vpn_port_share"
	VPNMSSFix        = "vpn_mssfix"
	VPNSingleSession = "vpn_single_session"

	// Backups: how many nightly backups are kept, and where off the
	// machine a copy goes, an S3-compatible bucket. BackupLast and
	// BackupLastError record the outcome of the latest backup.
	BackupKeep        = "backup_keep"
	BackupS3Endpoint  = "backup_s3_endpoint"
	BackupS3Region    = "backup_s3_region"
	BackupS3Bucket    = "backup_s3_bucket"
	BackupS3Prefix    = "backup_s3_prefix"
	BackupS3AccessKey = "backup_s3_access_key"
	BackupS3SecretKey = "backup_s3_secret_key"
	BackupLast        = "backup_last"
	BackupLastError   = "backup_last_error"

	WebFirewallRules = "web_firewall_rules"

	// Alerts tell administrators when something needs them: AlertEmails
	// lists the addresses, one per line, and AlertWebhook is a URL that
	// receives a Slack-style JSON message. The switches choose what is
	// alerted on.
	AlertEmails       = "alert_emails"
	AlertWebhook      = "alert_webhook_url"
	AlertVPNDown      = "alert_vpn_down"
	AlertCertificates = "alert_certificates"
	AlertLockouts     = "alert_lockouts"
	AlertNewDevice    = "alert_new_device"
	AlertAdminSignIn  = "alert_admin_signin"
	AlertNewAddress   = "alert_new_address"
	AlertDailyDigest  = "alert_daily_digest"

	// Alert state, kept between runs of the task worker: whether OpenVPN
	// was last seen running, and the day the certificates were last
	// warned about. VPNStoppedByAdmin marks a stop someone asked for,
	// which is not alerted on.
	AlertVPNState        = "alert_vpn_state"
	AlertCertificatesDay = "alert_certificates_day"
	AlertDigestDay       = "alert_digest_day"
	VPNStoppedByAdmin    = "vpn_stopped_by_admin"
)

// NoOAuth2Provider is the value of [OAuth2Provider] when OAuth2 sign-in is
// switched off.
const NoOAuth2Provider = "none"

// SecretSettings are the settings stored encrypted: private keys and
// passwords.
var SecretSettings = []string{
	CAPrivateKey,
	VPNPrivateKey,
	VPNTLSAuthKey,
	SMTPPassword,
	OAuth2ClientSecret,
	// Slack and the like put the credential in the webhook's URL.
	AlertWebhook,
	BackupS3SecretKey,
}
