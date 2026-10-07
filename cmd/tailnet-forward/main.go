// tailnet-forward runs next to containerboot and talks to tailscaled over its
// local socket.
//
// It forwards TCP_FORWARD_<n>=<port>:<host>:<port> from the container's own
// ports to tailnet destinations, including hosts behind subnet routes. Railway
// gives containers no TUN device, so other services cannot route into the
// tailnet themselves. Subnet routes need accept-routes on the node.
//
// It also re-advertises the Tailscale Services in TS_SERVE_CONFIG (see
// advertise.go), a workaround for tailscale/tailscale#21455.
//
// It never changes any other node setting, so nothing here can affect the
// Tailscale node: every failure is logged, and at worst forwarding stops.
package main

import (
	"cmp"
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"tailscale.com/client/local"
	"tailscale.com/ipn"
)

func main() {
	// Same shape as the entrypoint's jq wrapper, which Railway reads as
	// structured logs: lowercase "level", "message", the rest as attributes.
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			switch a.Key {
			case slog.TimeKey:
				return slog.Attr{} // Railway timestamps every line
			case slog.MessageKey:
				a.Key = "message"
			case slog.LevelKey:
				a.Value = slog.StringValue(strings.ToLower(a.Value.String()))
			}
			return a
		},
	})))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	// Same default as containerboot.
	lc := &local.Client{Socket: cmp.Or(os.Getenv("TS_SOCKET"), "/tmp/tailscaled.sock"), UseSocketOnly: true}

	// A bad forward must not stop the Services workaround, so the forward
	// config is only checked after that has started.
	forwards, err := loadForwards(os.Environ())

	if !waitRunning(ctx, lc) {
		return
	}
	if path := os.Getenv("TS_SERVE_CONFIG"); path != "" {
		go advertiseServices(ctx, lc, path)
	}

	if err != nil {
		slog.Error("invalid forward configuration, not forwarding", "error", err)
	} else {
		runForwards(ctx, lc, forwards)
	}
	// Stay alive until shutdown, so the advertise workaround keeps running and
	// this process does not linger as an unreaped zombie under containerboot.
	<-ctx.Done()
}

// waitRunning blocks until tailscaled's state is Running. It returns false
// only when ctx ends; it never gives up on its own.
func waitRunning(ctx context.Context, lc *local.Client) bool {
	for {
		// The socket appears only once containerboot has started tailscaled.
		w, err := lc.WatchIPNBus(ctx, ipn.NotifyInitialState)
		if err == nil {
			for {
				n, err := w.Next()
				if err != nil {
					break
				}
				if n.State != nil && *n.State == ipn.Running {
					w.Close()
					slog.Info("tailscaled is running")
					return true
				}
			}
			w.Close()
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(time.Second):
		}
	}
}
