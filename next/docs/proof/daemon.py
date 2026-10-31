"""Usage: python3 -I daemon.py <socket> <request>

Sends one request of the run daemon's protocol (a JSON line) on its unix
socket and prints each answer line as it comes, until the daemon closes the
connection: the driver of daemon.sh, before any `run` command exists."""

import socket
import sys

conn = socket.socket(socket.AF_UNIX)
conn.connect(sys.argv[1])
conn.sendall(sys.argv[2].encode() + b"\n")
conn.settimeout(60)
with conn.makefile("rb") as answers:
    for line in answers:
        sys.stdout.write(line.decode())
