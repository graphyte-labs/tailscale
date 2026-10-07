# tailscale

[`tailscale/tailscale`](https://hub.docker.com/r/tailscale/tailscale) with JSON logs, rebuilt for every stable Tailscale release. Each tag is built from the upstream tag of the same name.

```
ghcr.io/graphyte-labs/tailscale:<version>   # e.g. v1.102.5 or 1.102.5
ghcr.io/graphyte-labs/tailscale:latest
```

All output goes to stdout as one `{"level","message"}` object per line, so platforms such as Railway show real severities. The level is guessed from keywords, since tailscaled emits none. `TS_*` variables work as upstream.

Runs `containerboot` by default; pass a command to run something else instead.

## TCP forwards

Railway containers have no TUN device, so Tailscale runs in userspace mode and other services, such as a database client, cannot route into the tailnet. `TCP_FORWARD_<n>` exposes tailnet destinations on the container's own ports instead:

```
TCP_FORWARD_1=5432:192.0.2.10:5432
TCP_FORWARD_2=6379:cache.example.ts.net:6379
```

Each `<listen port>:<host>:<port>` listens on IPv4 and IPv6 (Railway's private network is IPv6) and connects every connection through tailscaled. Other services connect to `<service>.railway.internal:<listen port>`. Works for any TCP protocol, including ones that cannot use a proxy.

- **Subnet routes need accept-routes** on the node (`tailscale set --accept-routes`, stored in `TS_STATE_DIR`) and a tailnet grant from the node's tag to the target.
- **Set `TS_DEBUG_MTU=1250` on Railway.** Railway's MTU is below what Tailscale assumes, so full-size packets are dropped silently.
- **Failures never affect the Tailscale node.** A target that cannot be reached fails only that connection; a port that cannot be opened is retried every 10 s; a bad mapping is logged and skipped.
- At most 64 connections per forward. Each closed connection is logged with bytes and duration.

## Tailscale Services after restart

containerboot does not re-advertise the Services in `TS_SERVE_CONFIG` after a restart ([tailscale/tailscale#21455](https://github.com/tailscale/tailscale/issues/21455)). The image does it once tailscaled is running. To be removed once the upstream fix is released.
