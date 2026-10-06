// Package jobs runs the application's deferred work: sending e-mail,
// disconnecting VPN clients, and regenerating the certificate revocation
// list.
//
// Work is queued in SQLite rather than in a message broker, so the web, task,
// and VPN hook processes share one durable queue with nothing extra to
// install. This replaces the Redis backed queue of the Django release.
package jobs

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jeffmvr/mangle-vpn/internal/store"
)

// The kinds of work the queue carries.
const (
	KindSendEmail  = "send_email"
	KindKillClient = "kill_client"
	KindCreateCRL  = "create_crl"
	KindWebhook    = "send_webhook"
)

// WebhookPayload is the payload of a send_webhook job.
type WebhookPayload struct {
	URL  string `json:"url"`
	Text string `json:"text"`
}

// Queue accepts work to be run later.
type Queue struct {
	jobs *store.JobStore
}

// NewQueue returns a queue backed by the given job store.
func NewQueue(jobs *store.JobStore) *Queue {
	return &Queue{jobs: jobs}
}

// Enqueue adds a job of the given kind, encoding payload as JSON.
func (q *Queue) Enqueue(ctx context.Context, kind string, payload any) error {
	encoded := ""
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("jobs: encode %s payload: %w", kind, err)
		}
		encoded = string(data)
	}

	if err := q.jobs.Enqueue(ctx, kind, encoded); err != nil {
		return fmt.Errorf("jobs: enqueue %s: %w", kind, err)
	}
	return nil
}

// Decode reads a job payload into v.
func Decode(payload string, v any) error {
	if payload == "" {
		return nil
	}
	if err := json.Unmarshal([]byte(payload), v); err != nil {
		return fmt.Errorf("jobs: decode payload: %w", err)
	}
	return nil
}

// KillClientPayload is the payload of a [KindKillClient] job.
type KillClientPayload struct {
	Address string `json:"address"`
}
