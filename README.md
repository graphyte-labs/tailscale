# tailscale

[`tailscale/tailscale`](https://hub.docker.com/r/tailscale/tailscale) with JSON logs, rebuilt for every stable Tailscale release. Each tag is built from the upstream tag of the same name.

```
ghcr.io/graphyte-labs/tailscale:<version>   # e.g. v1.102.5
ghcr.io/graphyte-labs/tailscale:latest
```

All output goes to stdout as one `{"level","message"}` object per line, so platforms such as Railway show real severities. The level is guessed from keywords, since tailscaled emits none. `TS_*` variables work as upstream.

Runs `containerboot` by default; pass a command to run something else instead.
