package web

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"html/template"
	"net/http"
	"net/url"

	"github.com/jeffmvr/mangle-vpn/internal/config"
	"github.com/jeffmvr/mangle-vpn/internal/model"
	"github.com/jeffmvr/mangle-vpn/internal/validate"
)

//go:embed templates/*.html
var templateFS embed.FS

// pageTemplates holds the server rendered pages.
var pageTemplates = template.Must(template.ParseFS(templateFS, "templates/*.html"))

// Page template names.
const (
	loginPage       = "Login.html"
	forgotPage      = "Forgot.html"
	setPasswordPage = "SetPassword.html"
	mfaConfirmPage  = "MfaConfirm.html"
	mfaSetupPage    = "MfaSetup.html"
	passwordPage    = "Password.html"
)

// formState carries a rejected submission back to the page it came from, so
// the user sees what they typed alongside what was wrong with it.
type formState struct {
	Data   map[string]string `json:"data"`
	Errors map[string]string `json:"errors"`
}

// newFormState returns an empty form state with usable maps.
func newFormState() formState {
	return formState{Data: map[string]string{}, Errors: map[string]string{}}
}

// Valid reports whether the submission passed validation.
func (f formState) Valid() bool { return len(f.Errors) == 0 }

// redact drops the named values so that they are never written to the
// session or rendered back into the page. Any error recorded against them
// is kept, so the user is still told what was wrong.
func (f formState) redact(fields ...string) {
	for _, field := range fields {
		delete(f.Data, field)
	}
}

// pageData is the data every server rendered page is given.
type pageData struct {
	CSRFToken      string
	Error          string
	Form           formState
	MFAURL         string
	OAuth2Provider string
	OAuth2Name     string
	Organization   string
	SetupToken     string
	User           *model.User

	// MailConfigured offers the forgotten password page, which only works
	// when the application can send e-mail.
	MailConfigured bool

	// LinkCode and LinkEmail are the code of the password link being used
	// and the address of the account it is for.
	LinkCode  string
	LinkEmail string

	// LinkInvite says the link accepts an invitation, which also asks for
	// the person's name.
	LinkInvite bool

	// Sent tells the forgotten password page that a link has been asked
	// for.
	Sent bool

	// PasswordRule describes the password rules, under a new password.
	PasswordRule string

	// LogoURL is the organization's logo, or "" for the plain mark, and
	// SignInNotice a notice to show on the sign in page.
	LogoURL      string
	SignInNotice string

	// SupportURL is a mailto: or https link to whoever helps people sign
	// in, or "" for none.
	SupportURL template.URL

	// Setup is the setup wizard's state, on its pages, and SetupDone what
	// it did, on the page after.
	Setup     *setupView
	SetupDone *setupSummary

	// PasswordSignIn offers the email and password form; it is false when
	// single sign-on is the only way in, unless an administrator asked for
	// it.
	PasswordSignIn bool
}

// newPageData collects the values shared by every page, including any
// message or rejected form left behind by the previous request.
func (s *Server) newPageData(w http.ResponseWriter, r *http.Request) pageData {
	return pageData{
		CSRFToken:      s.csrfTokenFor(w, r),
		Error:          s.takeError(r.Context()),
		Form:           s.takeForm(r.Context()),
		MailConfigured: s.app.MailConfigured(),
		PasswordRule:   s.app.PasswordRule(),
		LogoURL:        s.logoURL(),
		SignInNotice:   s.app.Config.Get(config.AppSignInNotice),
		SupportURL:     supportURL(s.app.Config.Get(config.AppSupportContact)),
		PasswordSignIn: !s.ssoOnly() || r.URL.Query().Get("password") != "",
		OAuth2Provider: s.app.Config.Get(config.OAuth2Provider),
		OAuth2Name:     s.app.Config.GetOr(config.OAuth2Name, "single sign-on"),
		Organization:   s.app.Config.Organization(),
		User:           currentUser(r),
	}
}

// render writes a page to the response.
//
// The page is rendered into memory first so that a template failure becomes
// an error page rather than a half-written one.
func (s *Server) render(w http.ResponseWriter, r *http.Request, name string, data pageData) {
	var buf bytes.Buffer
	if err := pageTemplates.ExecuteTemplate(&buf, name, data); err != nil {
		s.app.Log.Error("failed to render page", "page", name, "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Write(buf.Bytes())
}

// putForm stores a rejected submission for the page to redisplay.
func (s *Server) putForm(r *http.Request, form formState) {
	encoded, err := json.Marshal(form)
	if err != nil {
		s.app.Log.Error("failed to store form state", "err", err)
		return
	}
	s.sessions.Put(r.Context(), sessionForm, string(encoded))
}

// takeForm removes and returns the stored submission, if any.
func (s *Server) takeForm(ctx context.Context) formState {
	form := newFormState()

	stored := s.sessions.PopString(ctx, sessionForm)
	if stored == "" {
		return form
	}
	if err := json.Unmarshal([]byte(stored), &form); err != nil {
		s.app.Log.Error("failed to read stored form state", "err", err)
		return newFormState()
	}
	if form.Data == nil {
		form.Data = map[string]string{}
	}
	if form.Errors == nil {
		form.Errors = map[string]string{}
	}
	return form
}

// supportURL turns the support contact setting into a link: an email address
// becomes a mailto: link, and anything but an https URL is dropped.
func supportURL(contact string) template.URL {
	switch {
	case validate.IsEmail(contact):
		return template.URL("mailto:" + url.PathEscape(contact))
	case isHTTPSURL(contact):
		return template.URL(contact)
	}
	return ""
}

// isHTTPSURL reports whether s is an absolute https URL with a host.
func isHTTPSURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && u.Scheme == "https" && u.Host != ""
}
