package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/jeffmvr/mangle-vpn/internal/store"
	"github.com/jeffmvr/mangle-vpn/internal/uuid"
)

// maxRequestBody caps how much JSON a request may carry. Nothing the API
// accepts is large, apart from pasted TLS material.
const maxRequestBody = 1 << 20

// Reasons an API request is refused, as the frontend's interceptor sees
// them. They are carried over from the previous deployment, whose frontend
// is still in use.
const (
	reasonNotSignedIn    = "NotLoggedIn"
	reasonMFANotEnabled  = "MfaNotEnabled"
	reasonMFAUnconfirmed = "MfaNotConfirmed"

	// reasonPasswordChange is new with the Go server: the frontend sends
	// the user to the password page.
	reasonPasswordChange = "PasswordChangeRequired"
)

// writeJSON sends a value as a JSON response.
func (s *Server) writeJSON(w http.ResponseWriter, status int, value any) {
	body, err := json.Marshal(value)
	if err != nil {
		s.app.Log.Error("failed to encode a response", "err", err)
		http.Error(w, `{"detail":"Internal Server Error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	w.Write(body)
}

// noContent sends an empty success response.
func (s *Server) noContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// fieldErrors maps a field name to the problems found with it. It is the
// shape the frontend reads validation failures from.
type fieldErrors map[string][]string

// add records a problem with a field.
func (e fieldErrors) add(field, message string) {
	e[field] = append(e[field], message)
}

// empty reports whether anything was recorded.
func (e fieldErrors) empty() bool { return len(e) == 0 }

// invalid sends a validation failure.
func (s *Server) invalid(w http.ResponseWriter, errs fieldErrors) {
	s.writeJSON(w, http.StatusBadRequest, errs)
}

// invalidField sends a validation failure about a single field.
func (s *Server) invalidField(w http.ResponseWriter, field, message string) {
	s.invalid(w, fieldErrors{field: {message}})
}

// detail is the single message body used for failures that are not about a
// particular field.
type detail struct {
	Detail string `json:"detail"`
}

// forbidden refuses a request.
func (s *Server) forbidden(w http.ResponseWriter, r *http.Request, reason string) {
	if isAPIRequest(r) {
		s.writeJSON(w, http.StatusForbidden, detail{reason})
		return
	}
	http.Error(w, reason, http.StatusForbidden)
}

// notFound reports that a resource does not exist.
func (s *Server) notFound(w http.ResponseWriter) {
	s.writeJSON(w, http.StatusNotFound, detail{"Not found."})
}

// serverError reports an unexpected failure, logging the cause rather than
// returning it.
func (s *Server) serverError(w http.ResponseWriter, message string, err error) {
	s.app.Log.Error(message, "err", err)
	s.writeJSON(w, http.StatusInternalServerError, detail{"A server error occurred."})
}

// handleStoreError turns a store failure into the right response, reporting
// whether it did so.
func (s *Server) handleStoreError(w http.ResponseWriter, err error, message string) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, store.ErrNotFound):
		s.notFound(w)
	case errors.Is(err, store.ErrConflict):
		s.invalidField(w, "name", "This value is already in use.")
	default:
		s.serverError(w, message, err)
	}
	return true
}

// decodeJSON reads a JSON request body into v.
func (s *Server) decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	decoder := json.NewDecoder(io.LimitReader(r.Body, maxRequestBody))

	if err := decoder.Decode(v); err != nil {
		s.writeJSON(w, http.StatusBadRequest, detail{"The request body could not be read."})
		return false
	}
	return true
}

// pathID reads a UUID from a path parameter.
func (s *Server) pathID(w http.ResponseWriter, r *http.Request, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue(name))
	if err != nil {
		s.notFound(w)
		return uuid.Nil, false
	}
	return id, true
}

// boolParam reads an optional true or false from the query string. It is
// nil when the parameter is absent, and records a message against the
// parameter when it is present but not a boolean.
func boolParam(r *http.Request, name string, errs fieldErrors) *bool {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return nil
	}

	value, err := strconv.ParseBool(raw)
	if err != nil {
		errs.add(name, "Must be true or false.")
		return nil
	}
	return &value
}

//
// Pagination
//

// page is one page of results, in the envelope the frontend expects.
type page[T any] struct {
	Count    int     `json:"count"`
	Next     *string `json:"next"`
	Previous *string `json:"previous"`
	Results  []T     `json:"results"`
}

// listQuery reads the search and paging parameters from a request.
//
// Paging only happens when a size is asked for; without one the whole list
// is returned as a bare array, which is what the frontend expects from the
// endpoints it does not page.
func listQuery(r *http.Request) store.Query {
	params := r.URL.Query()

	q := store.Query{Search: params.Get("search")}
	if size, err := strconv.Atoi(params.Get("size")); err == nil && size > 0 {
		q.Size = size
		q.Page = 1
		if number, err := strconv.Atoi(params.Get("page")); err == nil && number > 0 {
			q.Page = number
		}
	}
	return q
}

// writeList sends a listing, paged or bare depending on what was asked for.
func writeList[T, U any](s *Server, w http.ResponseWriter, r *http.Request,
	q store.Query, result store.Page[T], convert func(T) U) {

	items := make([]U, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, convert(item))
	}

	if !q.Paginated() {
		s.writeJSON(w, http.StatusOK, items)
		return
	}

	s.writeJSON(w, http.StatusOK, page[U]{
		Count:    result.Total,
		Next:     s.pageURL(r, q, result.Total, q.Page+1),
		Previous: s.pageURL(r, q, result.Total, q.Page-1),
		Results:  items,
	})
}

// pageURL returns the path and query of a page, or nil when that page is out
// of range.
func (s *Server) pageURL(r *http.Request, q store.Query, total, number int) *string {
	lastPage := (total + q.Size - 1) / q.Size
	if number < 1 || number > lastPage {
		return nil
	}

	target := *r.URL
	params := target.Query()
	params.Set("page", strconv.Itoa(number))
	target.RawQuery = params.Encode()

	// Relative, unlike the Django release's absolute links: building them
	// from the request's Host header would put whatever a client sent there
	// into the response.
	relative := target.RequestURI()
	return &relative
}

// absoluteURL returns u as an absolute URL for the current request.
func (s *Server) absoluteURL(r *http.Request, u *url.URL) string {
	scheme := "http"
	switch {
	case r.TLS != nil:
		scheme = "https"
	case s.trustProxy && r.Header.Get("X-Forwarded-Proto") != "":
		scheme = r.Header.Get("X-Forwarded-Proto")
	}
	return fmt.Sprintf("%s://%s%s", scheme, r.Host, u.RequestURI())
}

//
// API authentication
//

// isAPIRequest reports whether a request is for the JSON API.
func isAPIRequest(r *http.Request) bool {
	return len(r.URL.Path) >= 4 && r.URL.Path[:4] == "/api"
}
