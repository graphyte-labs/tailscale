#!/bin/sh
# Runs the given command (default: containerboot) with its output as one
# {"level","message"} JSON object per line on stdout.
set -eu
[ $# -gt 0 ] || set -- /usr/local/bin/containerboot

# jq reads from a FIFO so the command can still exec as PID 1 and get SIGTERM.
fifo="$(mktemp -u)"
mkfifo "$fifo"
jq -c -R --unbuffered '{
  level: (if test("error|failed|fatal|panic"; "i") then "error" elif test("warn"; "i") then "warn" else "info" end),
  message: sub("^(boot: )?[0-9/]{10} [0-9:.]+ "; "")
}' <"$fifo" &
exec "$@" >"$fifo" 2>&1
