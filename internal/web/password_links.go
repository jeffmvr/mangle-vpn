package web

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/jeffmvr/mangle-vpn/internal/app"
	"github.com/jeffmvr/mangle-vpn/internal/validate"
)

// The pages behind the password links e-mailed with an invitation, an
// administrator's reset, or a forgotten password, and the page that asks
// for the last of those.

// invalidLinkMessage is shown for a link that cannot be used.
const invalidLinkMessage = "This link has expired or has already been used."

// showSetPassword renders the page for choosing a password through a link.
func (s *Server) showSetPassword(w http.ResponseWriter, r *http.Request) {
	data := s.newPageData(w, r)
	data.LinkCode = r.URL.Query().Get("code")

	user, purpose, err := s.app.PasswordLinkUser(r.Context(), data.LinkCode)
	switch {
	case errors.Is(err, app.ErrLinkInvalid):
		data.LinkCode = ""
		data.Error = invalidLinkMessage
	case err != nil:
		s.app.Log.Error("failed to check a password link", "err", err)
		data.LinkCode = ""
		data.Error = "The link could not be checked. Try again in a moment."
	default:
		data.LinkEmail = user.Email
		data.LinkInvite = purpose == app.LinkInvite
		// An administrator may have given the name when inviting.
		if _, typed := data.Form.Data["name"]; !typed {
			data.Form.Data["name"] = user.Name
		}
	}

	// The code is in the address; keep it out of the Referer of anything
	// the page links to.
	w.Header().Set("Referrer-Policy", "no-referrer")
	s.render(w, r, setPasswordPage, data)
}

// processSetPassword sets the password a link was sent for and signs its
// owner in.
func (s *Server) processSetPassword(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	code := r.PostFormValue("code")
	retry := "/password/set?code=" + url.QueryEscape(code)

	if !s.allowAttempt(w, r, retry) {
		return
	}

	form := s.readForm(r, "name", "password", "password_confirm")

	// Accepting an invitation sets up the account, so it asks for the
	// person's name too.
	_, purpose, _ := s.app.PasswordLinkUser(ctx, code)
	name := ""
	if purpose == app.LinkInvite {
		name = form.Data["name"]
		validateFullName(&form, "name")
	}

	if form.Data["password"] == "" {
		form.Errors["password"] = "A password is required."
	}
	s.validatePasswordPair(&form, "password", "password_confirm")
	password := form.Data["password"]
	form.redact("password", "password_confirm")

	if !form.Valid() {
		s.putForm(r, form)
		http.Redirect(w, r, retry, http.StatusFound)
		return
	}

	user, err := s.app.RedeemPasswordLink(ctx, code, password, name)
	if err != nil {
		if !errors.Is(err, app.ErrLinkInvalid) {
			s.app.Log.Error("failed to set a password through a link", "err", err)
		}
		s.PutError(ctx, invalidLinkMessage+" Ask for a new one.")
		http.Redirect(w, r, "/password/forgot", http.StatusFound)
		return
	}

	// Choosing the password is a sign in with it; two-factor, where the
	// account needs it, still follows.
	if err := s.SignIn(w, r, user); err != nil {
		s.app.Log.Error("failed to start a session", "err", err)
	}
	s.afterSignIn(r, user)
	http.Redirect(w, r, "/", http.StatusFound)
}

// showForgotPassword renders the page that asks for a password link.
func (s *Server) showForgotPassword(w http.ResponseWriter, r *http.Request) {
	data := s.newPageData(w, r)
	data.Sent = r.URL.Query().Get("sent") != ""
	s.render(w, r, forgotPage, data)
}

// processForgotPassword e-mails a password link to the address given, if
// it has an account. The answer is the same either way.
func (s *Server) processForgotPassword(w http.ResponseWriter, r *http.Request) {
	if !s.allowAttempt(w, r, "/password/forgot") {
		return
	}

	email := strings.ToLower(strings.TrimSpace(r.PostFormValue("email")))
	if !validate.IsEmail(email) {
		form := newFormState()
		form.Data["email"] = email
		form.Errors["email"] = "Enter the e-mail address you sign in with."
		s.putForm(r, form)
		http.Redirect(w, r, "/password/forgot", http.StatusFound)
		return
	}

	// With single sign-on the only way in, only administrators keep a
	// password; anyone else is quietly passed over, as an unknown address
	// is.
	if err := s.app.ForgotPassword(r.Context(), email, s.ssoOnly()); err != nil {
		s.app.Log.Error("failed to send a password link", "err", err)
	}
	http.Redirect(w, r, "/password/forgot?sent=1", http.StatusFound)
}

// fullNameLimit is the longest full name accepted.
const fullNameLimit = 150

// validateFullName records an error unless the field holds a name.
func validateFullName(form *formState, field string) {
	name := strings.Join(strings.Fields(form.Data[field]), " ")
	form.Data[field] = name
	switch {
	case name == "":
		form.Errors[field] = "Your full name is required."
	case len(name) > fullNameLimit:
		form.Errors[field] = fmt.Sprintf("Keep it under %d characters.", fullNameLimit)
	}
}
