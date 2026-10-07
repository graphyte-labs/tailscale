package main

import (
	"errors"
	"fmt"
	"net"
	"slices"
	"strconv"
	"strings"
)

// forward exposes listenPort on the container and connects each connection to
// target through tailscaled.
type forward struct {
	name       string // the env var it came from, for logs
	listenPort uint16
	host       string
	port       uint16
	target     string // host:port
}

const forwardPrefix = "TCP_FORWARD_"

// loadForwards reads TCP_FORWARD_<n> from environ and reports every problem at
// once, so a bad deploy shows all mistakes in one log line.
func loadForwards(environ []string) ([]forward, error) {
	env := map[string]string{}
	var names []string
	for _, kv := range environ {
		k, v, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(k, forwardPrefix) {
			env[k] = v
			names = append(names, k)
		}
	}
	slices.Sort(names)

	var forwards []forward
	var errs []error
	ports := map[uint16]string{}
	for _, name := range names {
		f, err := parseForward(name, env[name])
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if prev, dup := ports[f.listenPort]; dup {
			errs = append(errs, fmt.Errorf("%s: port %d is already used by %s", name, f.listenPort, prev))
			continue
		}
		ports[f.listenPort] = name
		forwards = append(forwards, f)
	}
	return forwards, errors.Join(errs...)
}

// parseForward parses "<listen port>:<host>:<port>". The host may be an IP
// address, a bracketed IPv6 address or a tailnet name.
func parseForward(name, value string) (forward, error) {
	bad := func(why string) (forward, error) {
		return forward{}, fmt.Errorf("%s=%q: %s (want <listen port>:<host>:<port>)", name, value, why)
	}
	listen, rest, ok := strings.Cut(value, ":")
	if !ok {
		return bad("missing target")
	}
	lp, err := parsePort(listen)
	if err != nil {
		return bad("listen port: " + err.Error())
	}
	host, port, err := net.SplitHostPort(rest)
	if err != nil {
		return bad("target: " + err.Error())
	}
	if host == "" {
		return bad("empty target host")
	}
	tp, err := parsePort(port)
	if err != nil {
		return bad("target port: " + err.Error())
	}
	return forward{name: name, listenPort: lp, host: host, port: tp, target: net.JoinHostPort(host, port)}, nil
}

func parsePort(s string) (uint16, error) {
	n, err := strconv.ParseUint(s, 10, 16)
	if err != nil || n == 0 {
		return 0, fmt.Errorf("%q is not a port between 1 and 65535", s)
	}
	return uint16(n), nil
}
