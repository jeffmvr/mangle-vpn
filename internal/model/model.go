// Package model holds the application's domain entities and the behaviour
// that belongs to them alone. Persistence lives in the store package and
// anything with a side effect lives in the app package.
package model

import (
	"time"

	"github.com/jeffmvr/mangle-vpn/internal/uuid"
)

// Base carries the identity and timestamps every entity shares.
type Base struct {
	ID        uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time
}
