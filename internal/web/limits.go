package web

import (
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Sign in attempts are limited two ways. Here, each address gets a small
// budget of attempts that refills slowly, which stops a single host from
// guessing quickly against many accounts. The other limit, locking an
// account after too many failures in a row, is in internal/app, where the
// OpenVPN hooks share it.
const (
	// attemptBurst and attemptEvery shape the per-address budget: ten
	// attempts straight away, then one more every six seconds.
	attemptBurst = 10
	attemptEvery = 6 * time.Second

	// limiterIdle is how long an address's budget is kept after its last
	// attempt. A budget left this long has refilled completely, so
	// forgetting it changes nothing.
	limiterIdle = attemptBurst * attemptEvery
)

// attemptLimiter tracks the sign in budget of each client address.
type attemptLimiter struct {
	mu        sync.Mutex
	clients   map[string]*addressBudget
	lastSweep time.Time
}

// addressBudget is one address's budget and when it was last drawn on.
type addressBudget struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func newAttemptLimiter() *attemptLimiter {
	return &attemptLimiter{clients: map[string]*addressBudget{}}
}

// Allow spends one attempt from the address's budget, reporting false when
// it has none left.
func (l *attemptLimiter) Allow(address string) bool {
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	l.sweep(now)

	budget, ok := l.clients[address]
	if !ok {
		budget = &addressBudget{limiter: rate.NewLimiter(rate.Every(attemptEvery), attemptBurst)}
		l.clients[address] = budget
	}
	budget.lastSeen = now
	return budget.limiter.AllowN(now, 1)
}

// sweep forgets addresses that have been idle long enough for their budget
// to be full again, at most once a minute, so the map cannot grow without
// bound.
func (l *attemptLimiter) sweep(now time.Time) {
	if now.Sub(l.lastSweep) < time.Minute {
		return
	}
	l.lastSweep = now

	for address, budget := range l.clients {
		if now.Sub(budget.lastSeen) > limiterIdle {
			delete(l.clients, address)
		}
	}
}

// allowAttempt spends one sign in attempt for the request's address,
// answering the request itself and reporting false when the budget is spent.
func (s *Server) allowAttempt(w http.ResponseWriter, r *http.Request, retry string) bool {
	if s.attempts.Allow(s.clientIP(r)) {
		return true
	}

	s.app.Log.Warn("refused a sign in attempt over the address limit",
		"path", r.URL.Path, "remote", s.clientIP(r))
	s.PutError(r.Context(), "Too many attempts. Wait a minute and try again.")
	http.Redirect(w, r, retry, http.StatusFound)
	return false
}
