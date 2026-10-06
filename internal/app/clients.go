package app

import (
	"context"
	"fmt"

	"github.com/jeffmvr/mangle-vpn/internal/jobs"
	"github.com/jeffmvr/mangle-vpn/internal/model"
	"github.com/jeffmvr/mangle-vpn/internal/paths"
	"github.com/jeffmvr/mangle-vpn/internal/uuid"
)

// ConnectClient records a newly connected VPN client and opens its path
// through the firewall.
func (a *App) ConnectClient(ctx context.Context, client *model.Client) error {
	if err := a.Store.Clients.Create(ctx, client); err != nil {
		return err
	}

	// Reload so the client carries its device, user, and group, which the
	// firewall rule is derived from.
	stored, err := a.Store.Clients.Get(ctx, client.ID)
	if err != nil {
		return err
	}

	a.AddClientRule(ctx, stored)
	return nil
}

// DisconnectClient withdraws a client's firewall rule, forgets it, and asks
// OpenVPN to drop the connection.
//
// The kill is queued rather than performed here because this runs from the
// web request that disabled the account as well as from OpenVPN's own
// disconnect hook, and in the latter case the connection is already gone.
func (a *App) DisconnectClient(ctx context.Context, client *model.Client) error {
	a.RemoveClientRule(ctx, client)

	if err := a.Store.Clients.Delete(ctx, client.ID); err != nil {
		return err
	}

	if err := a.Jobs.Enqueue(ctx, jobs.KindKillClient,
		jobs.KillClientPayload{Address: client.RemoteIP}); err != nil {
		return fmt.Errorf("app: queue disconnect for %s: %w", client.CommonName, err)
	}
	return nil
}

// DisconnectClients disconnects every client in the list.
func (a *App) DisconnectClients(ctx context.Context, clients []*model.Client) error {
	for _, client := range clients {
		if err := a.DisconnectClient(ctx, client); err != nil {
			return err
		}
	}
	return nil
}

// DisconnectUserClients disconnects every client belonging to a user.
func (a *App) DisconnectUserClients(ctx context.Context, userID uuid.UUID) error {
	clients, err := a.Store.Clients.ByUser(ctx, userID)
	if err != nil {
		return err
	}
	return a.DisconnectClients(ctx, clients)
}

// DisconnectGroupClients disconnects every client whose user is in a group.
func (a *App) DisconnectGroupClients(ctx context.Context, groupID uuid.UUID) error {
	clients, err := a.Store.Clients.ByGroup(ctx, groupID)
	if err != nil {
		return err
	}
	return a.DisconnectClients(ctx, clients)
}

// ManagementSocket returns the path of OpenVPN's management socket.
func (a *App) ManagementSocket() string { return paths.VPNManagementSocket }
