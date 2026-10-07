#!/bin/sh
# Runs the given command (default: containerboot) with its output as one
# {"level","message"} JSON object per line on stdout. With containerboot it
# also starts tailnet-forward, which logs JSON itself.
set -eu
[ $# -gt 0 ] || set -- /usr/local/bin/containerboot

# jq reads from a FIFO so the command can still exec as PID 1 and get SIGTERM.
fifo="$(mktemp -u)"
mkfifo "$fifo"
jq -c -R --unbuffered '{
  level: (if test("error|failed|fatal|panic"; "i") then "error" elif test("warn"; "i") then "warn" else "info" end),
  message: sub("^(boot: )?[0-9/]{10} [0-9:.]+ "; "")
}' <"$fifo" &

# tailnet-forward runs alongside and never affects the Tailscale node: if it
# fails, it logs the error and only forwarding stops.
if [ "$1" = /usr/local/bin/containerboot ]; then
  /usr/local/bin/tailnet-forward &
fi

exec "$@" >"$fifo" 2>&1
