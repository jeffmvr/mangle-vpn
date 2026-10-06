package app

import (
	"context"

	"github.com/jeffmvr/mangle-vpn/internal/model"
)

// SaveGroup stores a group and brings its firewall chain into line with it.
//
// Disabling a group is how an administrator cuts off a whole set of users at
// once, so it must take their live connections down too.
func (a *App) SaveGroup(ctx context.Context, group *model.Group) error {
	if err := a.Store.Groups.Save(ctx, group); err != nil {
		return err
	}

	if !group.IsEnabled {
		if err := a.DisconnectGroupClients(ctx, group.ID); err != nil {
			return err
		}
		a.DeleteGroupChain(ctx, group)
		return nil
	}
	return a.RebuildGroupChain(ctx, group)
}

// DeleteGroup removes a group along with its members and their devices.
//
// Deleting a group deletes the users in it. There is nowhere else to put
// them: a user must belong to a group, and the application offers no way to
// choose a destination.
func (a *App) DeleteGroup(ctx context.Context, group *model.Group) error {
	users, err := a.Store.Users.ByGroup(ctx, group.ID)
	if err != nil {
		return err
	}
	for _, user := range users {
		if err := a.DeleteUser(ctx, user); err != nil {
			return err
		}
	}

	a.DeleteGroupChain(ctx, group)
	return a.Store.Groups.Delete(ctx, group.ID)
}

// RebuildAllGroupChains recreates the chain of every enabled group. It runs
// when the VPN server starts, after the base chains have been created.
func (a *App) RebuildAllGroupChains(ctx context.Context) error {
	groups, err := a.Store.Groups.Enabled(ctx)
	if err != nil {
		return err
	}
	for _, group := range groups {
		if err := a.RebuildGroupChain(ctx, group); err != nil {
			return err
		}
	}
	return nil
}

// DeleteAllGroupChains removes the chain of every group. It runs when the
// VPN server stops.
func (a *App) DeleteAllGroupChains(ctx context.Context) error {
	groups, err := a.Store.Groups.All(ctx)
	if err != nil {
		return err
	}
	for _, group := range groups {
		a.DeleteGroupChain(ctx, group)
	}
	return nil
}
