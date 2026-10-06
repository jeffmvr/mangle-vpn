package main

import (
	"context"
	"fmt"
)

// runSecrets manages the encryption of secrets stored in the database.
//
// "decrypt" writes them all back as plaintext, which is what the Django
// release reads: run it, with the services stopped, before rolling back. The
// next start of this release encrypts them again.
func runSecrets(ctx context.Context, args []string) error {
	if len(args) == 0 || args[0] != "decrypt" {
		return fmt.Errorf("usage: mangle-vpn secrets decrypt [-root dir]")
	}

	a, err := openApp(ctx, "secrets", "", args[1:])
	if err != nil {
		return err
	}
	defer a.Close()

	if err := a.Store.UnsealAll(ctx); err != nil {
		return err
	}
	fmt.Println("Stored secrets are now plaintext. Start this release again and they are re-encrypted.")
	return nil
}
