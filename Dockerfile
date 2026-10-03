# Official Tailscale image with output reformatted as JSON log lines.
# TS_* variables pass straight through to containerboot.
ARG TAILSCALE_VERSION
FROM tailscale/tailscale:${TAILSCALE_VERSION}

RUN apk add --no-cache jq

COPY entrypoint.sh /usr/local/bin/graphyte-entrypoint

ENTRYPOINT ["/usr/local/bin/graphyte-entrypoint"]
CMD []
