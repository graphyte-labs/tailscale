package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"strconv"
	"sync"
	"syscall"
	"time"

	"tailscale.com/client/local"
)

const (
	dialTimeout = 10 * time.Second
	maxConns    = 64 // per forward
	retryDelay  = 10 * time.Second
)

// runForwards serves every forward until ctx ends. Each forward is independent:
// one that cannot listen retries on its own and does not affect the others.
func runForwards(ctx context.Context, lc *local.Client, forwards []forward) {
	var wg sync.WaitGroup
	for _, f := range forwards {
		wg.Go(func() { keepServing(ctx, lc, f) })
	}
	wg.Wait()
}

// keepServing listens on f's port and serves it, starting over after
// retryDelay if the listener cannot be opened or fails.
func keepServing(ctx context.Context, lc *local.Client, f forward) {
	for {
		// [::] accepts IPv4 too; Railway's private network is IPv6.
		ln, err := net.Listen("tcp", "[::]:"+strconv.Itoa(int(f.listenPort)))
		if err != nil {
			slog.Error("cannot listen, retrying", "forward", f.name, "port", f.listenPort, "error", err)
		} else {
			slog.Info("forwarding", "forward", f.name, "port", f.listenPort, "target", f.target)
			stop := context.AfterFunc(ctx, func() { ln.Close() })
			err = serve(ctx, lc, ln, f)
			stop()
			ln.Close()
			if err != nil {
				slog.Error("listener failed, retrying", "forward", f.name, "error", err)
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(retryDelay):
		}
	}
}

func serve(ctx context.Context, lc *local.Client, ln net.Listener, f forward) error {
	slots := make(chan struct{}, maxConns)
	for {
		c, err := ln.Accept()
		if errors.Is(err, net.ErrClosed) && ctx.Err() != nil {
			return nil
		}
		if err != nil {
			return err
		}
		select {
		case slots <- struct{}{}:
		default:
			slog.Warn("too many connections, refusing", "forward", f.name, "client", c.RemoteAddr().String())
			c.Close()
			continue
		}
		go func() {
			defer func() { <-slots }()
			handle(ctx, lc, c, f)
		}()
	}
}

// handle forwards one connection. A target that cannot be reached only fails
// this connection; the next one tries again.
func handle(ctx context.Context, lc *local.Client, src net.Conn, f forward) {
	defer src.Close()
	log := slog.With("forward", f.name, "client", src.RemoteAddr().String(), "target", f.target)

	dctx, cancel := context.WithTimeout(ctx, dialTimeout)
	dst, err := lc.DialTCP(dctx, f.host, f.port)
	cancel()
	if err != nil {
		log.Error("dial failed", "error", err)
		return
	}
	defer dst.Close()

	start := time.Now()
	up, down := pipe(ctx, src, dst)
	log.Info("connection closed", "bytes_up", up, "bytes_down", down, "duration", time.Since(start).Round(time.Millisecond).String())
}

// pipe copies in both directions until both are done. When one side stops
// sending, only that direction is shut where the connection supports it, so
// the other can still finish (TCP half-close). The tailnet side from DialTCP
// cannot half-close, so a client that half-closes first ends the connection.
// An error in either direction, or ctx ending, closes both.
func pipe(ctx context.Context, a, b net.Conn) (aToB, bToA int64) {
	var once sync.Once
	closeBoth := func() { once.Do(func() { a.Close(); b.Close() }) }
	defer context.AfterFunc(ctx, closeBoth)()
	defer closeBoth()

	done := make(chan struct{})
	go func() {
		defer close(done)
		bToA = copyHalf(a, b, closeBoth)
	}()
	aToB = copyHalf(b, a, closeBoth)
	<-done
	return aToB, bToA
}

// copyHalf copies src to dst. At a clean end of src it shuts only dst's write
// side; on an error it closes both connections.
func copyHalf(dst, src net.Conn, closeBoth func()) int64 {
	n, err := io.Copy(dst, src)
	if err != nil {
		if !errors.Is(err, net.ErrClosed) && !errors.Is(err, syscall.ECONNRESET) && !errors.Is(err, syscall.EPIPE) {
			slog.Warn("copy error", "error", err)
		}
		closeBoth()
		return n
	}
	if cw, ok := dst.(interface{ CloseWrite() error }); ok {
		cw.CloseWrite()
	} else {
		closeBoth()
	}
	return n
}
