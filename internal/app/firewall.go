package app

import (
	"context"

	"github.com/jeffmvr/mangle-vpn/internal/iptables"
	"github.com/jeffmvr/mangle-vpn/internal/model"
	"github.com/jeffmvr/mangle-vpn/internal/provision"
	"github.com/jeffmvr/mangle-vpn/internal/uuid"
)

// RebuildGroupChain rewrites a group's iptables chain from its stored rules.
//
// The chain is rebuilt from scratch rather than amended, so that it always
// reflects exactly what is in the database. Every chain ends in a DROP, so a
// group permits only what it has been given a rule for.
func (a *App) RebuildGroupChain(ctx context.Context, group *model.Group) error {
	chain := group.Chain()

	iptables.CreateChain(ctx, iptables.Filter, chain)
	iptables.Flush(ctx, iptables.Filter, chain)

	rules, err := a.Store.FirewallRules.ByGroup(ctx, group.ID)
	if err != nil {
		return err
	}

	for _, rule := range rules {
		if rule.IsEnabled {
			iptables.AppendUniqueRule(ctx, iptables.Filter, chain, rule.Args()...)
		}
	}

	iptables.AppendUniqueRule(ctx, iptables.Filter, chain, "-j", model.ActionDrop)
	return nil
}

// DeleteGroupChain removes a group's iptables chain.
func (a *App) DeleteGroupChain(ctx context.Context, group *model.Group) {
	iptables.DeleteChain(ctx, iptables.Filter, group.Chain())
}

// AddClientRule points a connected client's traffic at its group's chain.
func (a *App) AddClientRule(ctx context.Context, client *model.Client) {
	group := client.Group()
	if group == nil {
		a.Log.Error("cannot add a firewall rule for a client with no group",
			"client", client.CommonName)
		return
	}
	iptables.AppendUniqueRule(ctx, iptables.Filter, provision.ClientsChain,
		"-s", client.VirtualIP, "-j", group.Chain())
}

// RemoveClientRule withdraws a connected client's dispatch rule.
func (a *App) RemoveClientRule(ctx context.Context, client *model.Client) {
	group := client.Group()
	if group == nil {
		return
	}
	iptables.DeleteRule(ctx, iptables.Filter, provision.ClientsChain,
		"-s", client.VirtualIP, "-j", group.Chain())
}

// SaveFirewallRule stores a rule and rebuilds the chain it belongs to. A rule
// moved to another group also has the old group's chain rebuilt, or it would
// stay in force there.
func (a *App) SaveFirewallRule(ctx context.Context, rule *model.FirewallRule) error {
	previousGroupID := uuid.Nil
	if !rule.ID.IsZero() {
		if existing, err := a.Store.FirewallRules.Get(ctx, rule.ID); err == nil {
			previousGroupID = existing.GroupID
		}
	}

	if err := a.Store.FirewallRules.Save(ctx, rule); err != nil {
		return err
	}
	if !previousGroupID.IsZero() && previousGroupID != rule.GroupID {
		if err := a.rebuildChainForGroup(ctx, previousGroupID); err != nil {
			return err
		}
	}
	return a.rebuildChainForGroup(ctx, rule.GroupID)
}

// DeleteFirewallRule removes a rule and rebuilds the chain it belonged to.
func (a *App) DeleteFirewallRule(ctx context.Context, rule *model.FirewallRule) error {
	if err := a.Store.FirewallRules.Delete(ctx, rule.ID); err != nil {
		return err
	}
	return a.rebuildChainForGroup(ctx, rule.GroupID)
}

// rebuildChainForGroup rebuilds the chain of the group with the given ID.
func (a *App) rebuildChainForGroup(ctx context.Context, groupID uuid.UUID) error {
	group, err := a.Store.Groups.Get(ctx, groupID)
	if err != nil {
		return err
	}
	return a.RebuildGroupChain(ctx, group)
}
