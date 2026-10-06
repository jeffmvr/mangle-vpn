package web

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jeffmvr/mangle-vpn/internal/app"
	"github.com/jeffmvr/mangle-vpn/internal/model"
	"github.com/jeffmvr/mangle-vpn/internal/webui"
)

//
// The application
//

// showApp serves the single page frontend.
func (s *Server) showApp(w http.ResponseWriter, r *http.Request) {
	// Make sure the CSRF cookie is set before the frontend loads, since it
	// reads the cookie to authorise its own requests.
	s.csrfTokenFor(w, r)

	// The shell names the current bundles, so it must never be served
	// stale after an upgrade.
	w.Header().Set("Cache-Control", "no-cache")

	if !webui.Built() {
		http.Error(w, "The web interface was not built into this binary. "+
			"Build it with \"make build\" in mangle-go.", http.StatusServiceUnavailable)
		return
	}
	http.ServeFileFS(w, r, webui.FS(), "index.html")
}

//
// Installation
//

// setupTokenMatches reports whether code is the setup code the install
// command wrote. With no code on disk, setup cannot be completed.
func (s *Server) setupTokenMatches(code string) bool {
	expected, err := os.ReadFile(s.app.Paths.SetupToken)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			s.app.Log.Error("failed to read the setup code", "err", err)
		}
		return false
	}

	want := strings.TrimSpace(string(expected))
	return want != "" && subtle.ConstantTimeCompare([]byte(strings.TrimSpace(code)), []byte(want)) == 1
}

//
// Authentication
//

// showLogin renders the sign in page.
func (s *Server) showLogin(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, loginPage, s.newPageData(w, r))
}

// processLogin checks a username and password.
func (s *Server) processLogin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if !s.allowAttempt(w, r, "/login") {
		return
	}

	user, err := s.app.Authenticate(ctx, r.PostFormValue("username"), r.PostFormValue("password"))
	if err == nil && s.ssoOnly() && !user.IsAdmin {
		// Only administrators keep a password, so a broken provider cannot
		// lock everyone out. The same answer as a wrong password, so this
		// does not confirm the password was right.
		err = app.ErrBadCredentials
	}
	if err != nil {
		s.app.Log.Info("rejected a sign in", "remote", s.clientIP(r))
		s.SignOut(ctx)
		s.PutError(ctx, "Invalid username or password.")
		retry := "/login"
		if s.ssoOnly() {
			retry = "/login?password=1"
		}
		http.Redirect(w, r, retry, http.StatusFound)
		return
	}

	if user.PasswordExpired(time.Now()) {
		s.SignOut(ctx)
		s.PutError(ctx, "Your temporary password has expired. Ask an administrator to reset it.")
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	if err := s.SignIn(w, r, user); err != nil {
		s.app.Log.Error("failed to start a session", "err", err)
	}
	s.afterSignIn(r, user)

	http.Redirect(w, r, "/", http.StatusFound)
}

// afterSignIn marks a user who is exempt from two-factor authentication as
// already confirmed, so they are not sent to a page they have no code for.
func (s *Server) afterSignIn(r *http.Request, user *model.User) {
	ctx := r.Context()

	if !user.MFARequired() {
		if err := s.ConfirmMFA(ctx); err != nil {
			s.app.Log.Error("failed to confirm the session", "err", err)
		}
		s.app.SignInSucceeded(ctx, user)
		s.app.RecordWebSignIn(ctx, user, s.clientIP(r))
	}
}

// processLogout ends the session.
func (s *Server) processLogout(w http.ResponseWriter, r *http.Request) {
	if err := s.SignOut(r.Context()); err != nil {
		s.app.Log.Error("failed to end the session", "err", err)
	}
	http.Redirect(w, r, "/login", http.StatusFound)
}

//
// Password reset
//

// showPasswordReset renders the password change page.
func (s *Server) showPasswordReset(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, passwordPage, s.newPageData(w, r))
}

// processPasswordReset stores a user's chosen password.
func (s *Server) processPasswordReset(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := currentUser(r)

	if !s.allowAttempt(w, r, "/password") {
		return
	}

	form := s.readForm(r, "current_password", "password", "password_confirm")
	s.validatePasswordPair(&form, "password", "password_confirm")

	// Someone who has only borrowed a signed in browser must not be able to
	// change the password and lock its owner out. A forced change after a
	// reset is exempt: the user has only just proved the password.
	if !user.PasswordChange && !user.CheckPassword(form.Data["current_password"]) {
		form.Errors["current_password"] = "This is not your current password."
		s.app.RecordFailedSignIn(ctx, user)
	}

	password := form.Data["password"]
	form.redact("current_password", "password", "password_confirm")

	if !form.Valid() {
		s.putForm(r, form)
		http.Redirect(w, r, "/password", http.StatusFound)
		return
	}

	user.SetPassword(password)
	user.PasswordChange = false
	user.PasswordExpire = nil

	err := s.app.Store.Users.SetPassword(ctx, user)
	if err == nil {
		// Every other session that knew the old password is signed out;
		// this one is signed back in below.
		err = s.app.Store.Users.EndSessions(ctx, user)
	}
	if err != nil {
		s.app.Log.Error("failed to change a password", "user", user.Email, "err", err)
		form.Errors["password"] = "The password could not be changed."
		s.putForm(r, form)
		http.Redirect(w, r, "/password", http.StatusFound)
		return
	}

	s.app.RecordEvent(ctx, user, model.EventAccountPassword,
		fmt.Sprintf("Changed their password from %s.", s.clientIP(r)))

	// Choosing a password re-establishes the session, which keeps a user
	// who was sent here mid sign in from having to start again.
	if err := s.SignIn(w, r, user); err != nil {
		s.app.Log.Error("failed to renew the session", "err", err)
	}
	if err := s.ConfirmMFA(ctx); err != nil {
		s.app.Log.Error("failed to confirm the session", "err", err)
	}

	http.Redirect(w, r, "/", http.StatusFound)
}

//
// Two-factor authentication
//

// showMFAConfirm renders the page asking for a two-factor code.
func (s *Server) showMFAConfirm(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, mfaConfirmPage, s.newPageData(w, r))
}

// showMFASetup renders the enrolment page, with the QR code the user scans.
func (s *Server) showMFASetup(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if user.MFAEnabled {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	data := s.newPageData(w, r)
	data.MFAURL = user.ProvisioningURI(s.app.Config.Organization())

	s.render(w, r, mfaSetupPage, data)
}

// processMFA checks a submitted two-factor code and, when it is the user's
// first, completes their enrolment.
func (s *Server) processMFA(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := currentUser(r)

	if !s.allowAttempt(w, r, "/login") {
		return
	}

	ok, err := s.app.VerifyMFA(ctx, user, r.PostFormValue("code"))
	if err != nil {
		s.app.Log.Error("failed to check a two-factor code", "user", user.Email, "err", err)
	}

	if !ok {
		s.app.RecordEvent(ctx, user, model.EventWebError,
			"Incorrect two-factor authentication code")

		// A wrong code ends the session rather than allowing another guess
		// against a still-authenticated account.
		s.SignOut(ctx)
		s.PutError(ctx, "Invalid two-factor authentication code.")
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	if !user.MFAEnabled {
		user.MFAEnabled = true
		if err := s.app.Store.Users.SetMFAEnabled(ctx, user); err != nil {
			s.app.Log.Error("failed to complete two-factor enrolment",
				"user", user.Email, "err", err)
		}
	}

	s.app.SignInSucceeded(ctx, user)
	s.app.RecordWebSignIn(ctx, user, s.clientIP(r))

	if err := s.ConfirmMFA(ctx); err != nil {
		s.app.Log.Error("failed to confirm the session", "err", err)
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

//
// Form helpers
//

// readForm collects the named fields from a submission.
func (s *Server) readForm(r *http.Request, fields ...string) formState {
	form := newFormState()
	for _, field := range fields {
		form.Data[field] = strings.TrimSpace(r.PostFormValue(field))
	}
	return form
}

// validateRequired records an error for each named field left blank.
func validateRequired(form *formState, messages map[string]string) {
	for field, message := range messages {
		if form.Data[field] == "" {
			form.Errors[field] = message
		}
	}
}

// validatePasswordPair checks that a password meets the password rules in
// the settings and that its confirmation matches.
func (s *Server) validatePasswordPair(form *formState, field, confirmField string) {
	password, confirmation := form.Data[field], form.Data[confirmField]
	if password == "" {
		return
	}

	if problem := s.app.PasswordProblem(password); problem != "" {
		form.Errors[field] = "Not long enough or not varied enough. " + problem
	} else if password != confirmation {
		form.Errors[field] = "The passwords do not match."
	}
}
