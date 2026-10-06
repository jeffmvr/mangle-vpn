package store

import (
	"context"
	"time"

	"github.com/jeffmvr/mangle-vpn/internal/model"
	"github.com/jeffmvr/mangle-vpn/internal/uuid"
)

// firewallRuleColumns lists the rule columns in the order firewallRuleScan
// expects.
const firewallRuleColumns = `f.id, f.created_at, f.updated_at, f.action, f.destination,
	f.group_id, f.is_enabled, f.port, f.protocol`

// newFirewallRuleScan allocates a scanner for firewallRuleColumns.
func newFirewallRuleScan() entityScan[*model.FirewallRule] { return &firewallRuleScan{} }

// FirewallRuleStore reads and writes per-group firewall rules.
type FirewallRuleStore struct{ s *Store }

// Get returns the rule with the given ID.
func (fs *FirewallRuleStore) Get(ctx context.Context, id uuid.UUID) (*model.FirewallRule, error) {
	return queryOne(fs.s.db.QueryRowContext(ctx,
		`SELECT `+firewallRuleColumns+` FROM "firewall_rules" f WHERE f.id = ?`, id),
		&firewallRuleScan{})
}

// ByGroup returns every rule belonging to the group. Rules are ordered by
// action descending so that DROP rules are written into the chain before
// ACCEPT rules, which is the order iptables evaluates them in.
func (fs *FirewallRuleStore) ByGroup(ctx context.Context, groupID uuid.UUID) ([]*model.FirewallRule, error) {
	rows, err := fs.s.db.QueryContext(ctx,
		`SELECT `+firewallRuleColumns+` FROM "firewall_rules" f
		 WHERE f.group_id = ? ORDER BY f.action DESC, f.created_at`, groupID)
	return queryAll(rows, err, newFirewallRuleScan)
}

// Save inserts or updates the rule.
func (fs *FirewallRuleStore) Save(ctx context.Context, r *model.FirewallRule) error {
	now := time.Now().UTC()
	r.UpdatedAt = now

	if r.ID.IsZero() {
		r.ID, r.CreatedAt = uuid.New(), now

		_, err := fs.s.db.ExecContext(ctx,
			`INSERT INTO "firewall_rules" (id, created_at, updated_at, action,
				destination, group_id, is_enabled, port, protocol)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			r.ID, formatTime(r.CreatedAt), formatTime(r.UpdatedAt), r.Action,
			r.Destination, r.GroupID, r.IsEnabled, r.Port, r.Protocol)
		return translate(err)
	}

	_, err := fs.s.db.ExecContext(ctx,
		`UPDATE "firewall_rules" SET updated_at = ?, action = ?, destination = ?,
			group_id = ?, is_enabled = ?, port = ?, protocol = ? WHERE id = ?`,
		formatTime(r.UpdatedAt), r.Action, r.Destination, r.GroupID, r.IsEnabled,
		r.Port, r.Protocol, r.ID)
	return translate(err)
}

// Delete removes the rule.
func (fs *FirewallRuleStore) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := fs.s.db.ExecContext(ctx, `DELETE FROM "firewall_rules" WHERE id = ?`, id)
	return translate(err)
}
