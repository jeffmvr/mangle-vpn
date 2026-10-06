package web

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/jeffmvr/mangle-vpn/internal/app"
	"github.com/jeffmvr/mangle-vpn/internal/model"
	"github.com/jeffmvr/mangle-vpn/internal/uuid"
)

// This file implements the part of OpenVPN Access Server's REST API that
// OpenVPN Connect's "Import Profile > URL" uses: the user types the server's
// address, their email and their password into the app, and the app fetches
// a profile for a new device.
//
// The wire format follows the OpenVPN web authentication RFC
// (github.com/OpenVPN/openvpn-rfc, webauth.md) and what the open source
// clients that implement it actually accept:
//
//   - GET /rest/GetUserlogin, with HTTP Basic credentials. Some clients send
//     them straight away; others wait for a 401 naming the realm below.
//   - Where two-factor is required, a 401 whose XML body carries a CRV1
//     challenge. The flags must be exactly "R,E" and the prompt must hold
//     no colons, or Android's client does not recognise it. The answer comes
//     back as the password "CRV1::<state>::<code>".
//   - The profile, with a 200.
//
// Each successful import creates a new device for the user, subject to their
// group's limit, exactly as the New device button on the website does.

// restRealm is the Basic authentication realm clients expect. The Python
// client only sends credentials for this exact realm.
const restRealm = `Basic realm="OpenVPN Access Server"`

// restChallengeWindow is how long a two-factor challenge may be answered.
const restChallengeWindow = 3 * time.Minute

// restChallengePrompt is what the app shows when asking for the code. It must
// not contain a colon.
const restChallengePrompt = "Enter the six-digit code from your authenticator app"

// restConnectDevice is the name given to devices added through the app, with
// a number appended when the user already has one.
const restConnectDevice = "OpenVPN Connect"

// restError is the XML error body the clients parse.
type restError struct {
	XMLName  xml.Name `xml:"Error"`
	Type     string   `xml:"Type"`
	Synopsis string   `xml:"Synopsis"`
	Message  string   `xml:"Message"`
}

// writeRESTError sends an error in the form the clients read. Every 401
// names the realm, so clients that wait to be asked send their credentials.
func writeRESTError(w http.ResponseWriter, status int, kind, message string) {
	body, _ := xml.Marshal(restError{Type: kind, Synopsis: "REST method failed", Message: message})

	if status == http.StatusUnauthorized {
		w.Header().Set("WWW-Authenticate", restRealm)
	}
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	w.Write(body)
}

// restDenied refuses a request the user cannot fix from the app.
func restDenied(w http.ResponseWriter, message string) {
	writeRESTError(w, http.StatusForbidden, "Access denied", message)
}

// restAuthFailed refuses credentials without saying which part was wrong.
func restAuthFailed(w http.ResponseWriter) {
	writeRESTError(w, http.StatusUnauthorized, "Authorization Required",
		"AUTH_FAILED: The email address, password or code is incorrect.")
}

// restChallenges holds the two-factor challenges waiting for an answer. They
// live in memory: the web application is a single process, and a challenge
// lost to a restart only means the user is asked to sign in again.
type restChallenges struct {
	mu      sync.Mutex
	pending map[string]restChallenge
}

// restChallenge is one challenge: who it was issued to and until when.
type restChallenge struct {
	userID  uuid.UUID
	expires time.Time
}

func newRESTChallenges() *restChallenges {
	return &restChallenges{pending: map[string]restChallenge{}}
}

// issue records a challenge for the user and returns its state ID.
func (c *restChallenges) issue(userID uuid.UUID) string {
	now := time.Now()
	state := rand.Text()

	c.mu.Lock()
	defer c.mu.Unlock()

	for id, pending := range c.pending {
		if now.After(pending.expires) {
			delete(c.pending, id)
		}
	}
	c.pending[state] = restChallenge{userID: userID, expires: now.Add(restChallengeWindow)}
	return state
}

// take spends a challenge, returning whom it was issued to. A challenge can
// be answered once, right or wrong.
func (c *restChallenges) take(state string) (uuid.UUID, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	pending, ok := c.pending[state]
	delete(c.pending, state)
	if !ok || time.Now().After(pending.expires) {
		return uuid.Nil, false
	}
	return pending.userID, true
}

// restGetAutologin refuses: a Mangle profile always asks for the user's
// code, so there is no profile that connects on its own.
func (s *Server) restGetAutologin(w http.ResponseWriter, r *http.Request) {
	restDenied(w, "NEED_AUTOLOGIN: This server does not issue auto-login profiles. "+
		"Turn off auto-login and import again.")
}

// restGetUserlogin signs a user in with the credentials the app sends, asks
// for their two-factor code where it is required, and returns a profile for
// a new device.
func (s *Server) restGetUserlogin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	username, password, ok := r.BasicAuth()
	if !ok {
		// Asking is what makes the clients that wait for a 401 send their
		// credentials, so this does not count as an attempt.
		writeRESTError(w, http.StatusUnauthorized, "Authorization Required",
			"AUTH_FAILED: Sign in with your email address and password.")
		return
	}

	if !s.attempts.Allow(s.clientIP(r)) {
		restDenied(w, "Too many attempts. Wait a minute and try again.")
		return
	}

	var user *model.User
	if state, code, isAnswer := parseCRV1Answer(password); isAnswer {
		user = s.restAnswerChallenge(w, r, username, state, code)
	} else {
		user = s.restSignIn(w, r, username, password)
	}
	if user == nil {
		return
	}

	device, err := s.restCreateDevice(r, user)
	switch {
	case errors.Is(err, app.ErrDeviceLimitReached):
		restDenied(w, "You already have as many devices as your group allows. "+
			"Revoke one on the website, then import again.")
		return
	case err != nil:
		s.app.Log.Error("failed to create a device for an app import", "user", user.Email, "err", err)
		writeRESTError(w, http.StatusInternalServerError, "Internal Server Error",
			"The profile could not be created.")
		return
	}

	conf, err := s.deviceProfile(ctx, device, "")
	if err != nil {
		s.app.Log.Error("failed to build a profile for an app import", "user", user.Email, "err", err)
		// Leaving the device would use up a slot for a profile nobody has.
		if err := s.app.DeleteDevice(ctx, device); err != nil {
			s.app.Log.Error("failed to remove the device of a failed import", "err", err)
		}
		writeRESTError(w, http.StatusInternalServerError, "Internal Server Error",
			"The profile could not be created.")
		return
	}

	s.app.RecordEvent(ctx, user, model.EventDeviceImport,
		fmt.Sprintf("Device %s added from OpenVPN Connect at %s.", device.Name, s.clientIP(r)))
	s.writeProfile(w, device, conf)
}

// restSignIn checks an email and password, and either returns the user, or
// answers the request itself, with a refusal or a two-factor challenge, and
// returns nil.
func (s *Server) restSignIn(w http.ResponseWriter, r *http.Request, username, password string) *model.User {
	ctx := r.Context()

	user, err := s.app.Authenticate(ctx, username, password)
	if err != nil || s.ssoOnly() && !user.IsAdmin {
		restAuthFailed(w)
		return nil
	}
	if !s.restMayImport(w, user) {
		return nil
	}

	if !user.MFARequired() {
		s.app.SignInSucceeded(ctx, user)
		return user
	}

	state := s.challenges.issue(user.ID)
	login := base64.StdEncoding.EncodeToString([]byte(user.Email))
	writeRESTError(w, http.StatusUnauthorized, "Authorization Required",
		fmt.Sprintf("CRV1:R,E:%s:%s:%s", state, login, restChallengePrompt))
	return nil
}

// restAnswerChallenge checks the answer to a two-factor challenge, returning
// the user or answering the request itself and returning nil.
func (s *Server) restAnswerChallenge(w http.ResponseWriter, r *http.Request, username, state, code string) *model.User {
	ctx := r.Context()

	userID, ok := s.challenges.take(state)
	if !ok {
		restAuthFailed(w)
		return nil
	}

	user, err := s.app.Store.Users.Get(ctx, userID)
	if err != nil || !strings.EqualFold(user.Email, strings.TrimSpace(username)) {
		restAuthFailed(w)
		return nil
	}

	// The account may have changed while the challenge was waiting. Being
	// locked or disabled since is checked by VerifyMFA.
	if !s.restMayImport(w, user) {
		return nil
	}

	ok, err = s.app.VerifyMFA(ctx, user, code)
	if err != nil {
		s.app.Log.Error("failed to check a two-factor code for an app import", "user", user.Email, "err", err)
	}
	if !ok {
		s.app.RecordEvent(ctx, user, model.EventWebError,
			"Incorrect two-factor authentication code while importing into OpenVPN Connect")
		restAuthFailed(w)
		return nil
	}

	s.app.SignInSucceeded(ctx, user)
	return user
}

// restMayImport refuses, with a reason the app can show, an account that has
// something to sort out on the website before it may have a profile.
func (s *Server) restMayImport(w http.ResponseWriter, user *model.User) bool {
	switch {
	case user.PasswordExpired(time.Now()):
		restDenied(w, "Your temporary password has expired. Ask an administrator to reset it.")
	case user.PasswordChange:
		restDenied(w, "Sign in on the website and choose a new password first.")
	case user.MFARequired() && !user.MFAEnabled:
		restDenied(w, "Sign in on the website and set up two-factor authentication first.")
	default:
		return true
	}
	return false
}

// restCreateDevice adds a device named after the app, numbering it when the
// user already has one of that name.
func (s *Server) restCreateDevice(r *http.Request, user *model.User) (*model.Device, error) {
	for n := 1; n <= 99; n++ {
		name := restConnectDevice
		if n > 1 {
			name = fmt.Sprintf("%s %d", restConnectDevice, n)
		}

		device, err := s.app.CreateDevice(r.Context(), user, name, "")
		if errors.Is(err, app.ErrDuplicateDeviceName) {
			continue
		}
		return device, err
	}
	return nil, app.ErrDeviceLimitReached
}

// parseCRV1Answer reads the password a client sends back after a challenge,
// "CRV1::<state>::<response>".
func parseCRV1Answer(password string) (state, response string, ok bool) {
	rest, found := strings.CutPrefix(password, "CRV1::")
	if !found {
		return "", "", false
	}
	state, response, found = strings.Cut(rest, "::")
	if !found || state == "" {
		return "", "", false
	}
	return state, response, true
}
