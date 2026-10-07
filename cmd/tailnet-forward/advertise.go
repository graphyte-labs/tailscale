package main

// Workaround for tailscale/tailscale#21455: containerboot clears
// AdvertiseServices on shutdown and does not restore it on boot, so Tailscale
// Services stay offline after every restart. Delete this file once the fix
// (tailscale/tailscale#21462) is in a release we build from.

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"slices"
	"time"

	"tailscale.com/client/local"
	"tailscale.com/ipn"
)

const advertiseRetry = 10 * time.Second

// advertiseServices keeps trying until every Service named in the serve config
// is advertised, or ctx ends. The file may appear or change after tailscaled
// starts, so a missing or empty file is retried, not treated as final.
func advertiseServices(ctx context.Context, lc *local.Client, path string) {
	for {
		done, err := advertiseOnce(ctx, lc, path)
		if err != nil {
			slog.Error("could not advertise services, retrying", "path", path, "error", err)
		}
		if done {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(advertiseRetry):
		}
	}
}

// advertiseOnce adds every Service in the serve config to the node's advertised
// Services, keeping any already advertised. It does the same as
// `tailscale serve advertise <svc>` for each one. It reports done once the file
// names at least one Service and all of them are advertised.
func advertiseOnce(ctx context.Context, lc *local.Client, path string) (done bool, err error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) || (err == nil && len(raw) == 0) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var sc ipn.ServeConfig
	if err := json.Unmarshal(raw, &sc); err != nil {
		return false, err
	}
	if len(sc.Services) == 0 {
		return true, nil
	}

	prefs, err := lc.GetPrefs(ctx)
	if err != nil {
		return false, err
	}
	want := slices.Clone(prefs.AdvertiseServices)
	var added []string
	for name := range sc.Services {
		if !slices.Contains(want, name.String()) {
			want = append(want, name.String())
			added = append(added, name.String())
		}
	}
	if len(added) == 0 {
		return true, nil
	}
	slices.Sort(want)
	if _, err := lc.EditPrefs(ctx, &ipn.MaskedPrefs{
		AdvertiseServicesSet: true,
		Prefs:                ipn.Prefs{AdvertiseServices: want},
	}); err != nil {
		return false, err
	}
	slog.Info("advertised services", "services", added)
	return true, nil
}
