package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/jeffmvr/mangle-vpn/internal/config"
)

// runHealth checks that the web application answers its health check, for
// a container's HEALTHCHECK. It exits non-zero when it does not.
func runHealth(ctx context.Context, args []string) error {
	var common commonFlags
	fs := flag.NewFlagSet("health", flag.ContinueOnError)
	common.bind(fs, "")
	url := fs.String("url", "", "the health check to ask (default: this machine's configured HTTPS port)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *url == "" {
		a, err := common.open(ctx)
		if err != nil {
			return err
		}
		port := a.Config.Int(config.AppHTTPSPort, 443)
		a.Close()
		*url = "https://127.0.0.1:" + strconv.Itoa(port) + "/healthz"
	}

	// The server's own certificate is often self-signed, and it is this
	// machine being asked, so the certificate is not checked.
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // asking ourselves
		},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, *url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check answered %s", resp.Status)
	}
	return nil
}
