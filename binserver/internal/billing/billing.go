// Package billing is the binServer's billing policy, DESIGNED FROM SCRATCH
// (migration-plan.md §4, Fase 6): the legacy _AUTH_GAME 196-byte link is
// UNVERIFIED and is NOT reproduced. Instead the new binServer exposes a clean
// gRPC contract (api/bin/v1) with its own policy. The tmServer character-login
// billing gate calls Check; this service is the source of truth.
package billing

import (
	"sync"
	"time"
)

// Status is an account's billing standing.
type Status int

// Billing statuses.
const (
	StatusUnknown Status = iota // no record (policy decides allow/deny)
	StatusActive                // paying / free-to-play active
	StatusBlocked               // administratively blocked
	StatusExpired               // subscription lapsed
)

// String renders a status for logs/wire.
func (s Status) String() string {
	switch s {
	case StatusActive:
		return "active"
	case StatusBlocked:
		return "blocked"
	case StatusExpired:
		return "expired"
	default:
		return "unknown"
	}
}

// Account is a billing record. A zero ExpiresAt means "never expires".
type Account struct {
	Name      string
	Status    Status
	ExpiresAt time.Time
}

// Decision is the result of a billing check.
type Decision struct {
	Allowed bool
	Status  Status
}

// Service holds billing records. The in-memory map is a stand-in for persistent
// storage; the policy and API are stable regardless of backing store. Safe for
// concurrent use (a gRPC server handles requests on many goroutines).
type Service struct {
	mu           sync.RWMutex
	accounts     map[string]Account
	allowUnknown bool // free-to-play default for accounts without a record
}

// New creates a Service. allowUnknown sets the policy for accounts with no
// record (true = free-to-play: allow; false = require a record).
func New(allowUnknown bool) *Service {
	return &Service{accounts: make(map[string]Account), allowUnknown: allowUnknown}
}

// Set inserts or replaces an account record.
func (s *Service) Set(acc Account) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.accounts[acc.Name] = acc
}

// Check decides whether an account may log in at time now.
func (s *Service) Check(name string, now time.Time) Decision {
	s.mu.RLock()
	acc, ok := s.accounts[name]
	s.mu.RUnlock()

	if !ok {
		return Decision{Allowed: s.allowUnknown, Status: StatusUnknown}
	}
	if acc.Status == StatusBlocked {
		return Decision{Allowed: false, Status: StatusBlocked}
	}
	if !acc.ExpiresAt.IsZero() && now.After(acc.ExpiresAt) {
		return Decision{Allowed: false, Status: StatusExpired}
	}
	return Decision{Allowed: acc.Status == StatusActive, Status: acc.Status}
}
