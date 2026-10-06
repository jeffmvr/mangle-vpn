// Package mailer renders and delivers the application's e-mail.
package mailer

import (
	"context"
	"embed"
	"errors"
	"fmt"
	htmltemplate "html/template"
	"strings"
	texttemplate "text/template"

	"github.com/jeffmvr/mangle-vpn/internal/config"
	"github.com/jeffmvr/mangle-vpn/internal/validate"
	"github.com/wneessen/go-mail"
)

//go:embed templates
var templateFS embed.FS

// Every e-mail has an HTML body and a plain text one, from templates of the
// same name: Welcome.html and Welcome.txt. The plain text is what a client
// that does not render HTML shows, so it is written for reading, not
// derived from the markup.
var (
	htmlTemplates = htmltemplate.Must(htmltemplate.ParseFS(templateFS, "templates/*.html"))
	textTemplates = texttemplate.Must(texttemplate.ParseFS(templateFS, "templates/*.txt"))
)

// Template names, without their extensions.
const (
	WelcomeTemplate  = "Welcome"
	PasswordTemplate = "Password"
	ForgotTemplate   = "Forgot"
)

// ErrNotConfigured is returned when the SMTP settings are incomplete.
var ErrNotConfigured = errors.New("mailer: SMTP is not configured")

// Message is one e-mail to deliver. It is also the payload of the job that
// delivers it, so its JSON form must stay readable by the task worker:
// Body has always held the HTML, and a message queued before Text existed
// is sent as HTML alone.
type Message struct {
	Recipient string `json:"recipient"`
	Subject   string `json:"subject"`
	Body      string `json:"body"`
	Text      string `json:"text,omitempty"`
	Sender    string `json:"sender"`
}

// Content is a rendered e-mail body, in both forms.
type Content struct {
	HTML string
	Text string
}

// Account holds the credentials a message is sent through.
type Account struct {
	Host     string
	Port     int
	Username string
	Password string
	UseTLS   bool
}

// AccountFromConfig reads the SMTP account out of the application settings.
func AccountFromConfig(cfg *config.Config) Account {
	return Account{
		Host:     cfg.Get(config.SMTPHost),
		Port:     cfg.Int(config.SMTPPort, 587),
		Username: cfg.Get(config.SMTPUsername),
		Password: cfg.Get(config.SMTPPassword),
		UseTLS:   cfg.Bool(config.SMTPTLS, true),
	}
}

// IsConfigured reports whether the account has everything needed to send.
func (a Account) IsConfigured() bool {
	return a.Host != "" && a.Port != 0 && a.Username != "" && a.Password != ""
}

// SenderFromConfig returns the address outgoing mail is sent from. The reply
// address is preferred, but it is a free text setting, so the authenticating
// username stands in when it does not hold an address.
func SenderFromConfig(cfg *config.Config) string {
	if reply := cfg.Get(config.SMTPReplyAddress); validate.IsEmail(reply) {
		return reply
	}
	return cfg.Get(config.SMTPUsername)
}

// Send delivers a message through the account.
func Send(ctx context.Context, account Account, msg Message) error {
	if !account.IsConfigured() {
		return ErrNotConfigured
	}

	m := mail.NewMsg()
	if err := m.From(msg.Sender); err != nil {
		return fmt.Errorf("mailer: sender %q: %w", msg.Sender, err)
	}
	if err := m.To(msg.Recipient); err != nil {
		return fmt.Errorf("mailer: recipient %q: %w", msg.Recipient, err)
	}

	m.Subject(msg.Subject)

	// The plain text part goes first and the HTML second, as alternatives:
	// clients show the last one they can render.
	if msg.Text != "" {
		m.SetBodyString(mail.TypeTextPlain, msg.Text)
		m.AddAlternativeString(mail.TypeTextHTML, msg.Body)
	} else {
		m.SetBodyString(mail.TypeTextHTML, msg.Body)
	}

	options := []mail.Option{
		mail.WithPort(account.Port),
		mail.WithSMTPAuth(mail.SMTPAuthAutoDiscover),
		mail.WithUsername(account.Username),
		mail.WithPassword(account.Password),
	}
	if account.UseTLS {
		options = append(options, mail.WithTLSPolicy(mail.TLSMandatory))
	} else {
		options = append(options, mail.WithTLSPolicy(mail.NoTLS))
	}

	client, err := mail.NewClient(account.Host, options...)
	if err != nil {
		return fmt.Errorf("mailer: connect to %s: %w", account.Host, err)
	}
	defer client.Close()

	if err := client.DialAndSendWithContext(ctx, m); err != nil {
		return fmt.Errorf("mailer: send to %s: %w", msg.Recipient, err)
	}
	return nil
}

// Render returns both bodies of the named e-mail.
func Render(name string, data any) (Content, error) {
	var html, text strings.Builder
	if err := htmlTemplates.ExecuteTemplate(&html, name+".html", data); err != nil {
		return Content{}, fmt.Errorf("mailer: render %s.html: %w", name, err)
	}
	if err := textTemplates.ExecuteTemplate(&text, name+".txt", data); err != nil {
		return Content{}, fmt.Errorf("mailer: render %s.txt: %w", name, err)
	}
	return Content{HTML: html.String(), Text: text.String()}, nil
}

// LinkData fills in the templates that send someone a link to choose a
// password: the welcome, the administrator's reset, and the forgotten
// password.
type LinkData struct {
	Email        string
	Organization string

	// Link is the single-use address of the page to choose a password on,
	// and Lasts how long it works for, such as "3 days".
	Link  string
	Lasts string

	// URL is the application's own address.
	URL string
}
