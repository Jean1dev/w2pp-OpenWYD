#!/usr/bin/env bash
# Run the full WYD stack locally and make it ready for a real client on the LAN.
#
# Brings up the 4 services (PostgreSQL + dbServer + binServer + tmServer) via
# docker compose, seeds one test account so you can actually log in, mounts the
# Release/ content tree into tmServer, and prints the IP:port to point the client
# (WYD.exe) at — generate a serverlist.bin for that address with the
# "serverlist editor.exe" and copy it into the client folder.
#
# Usage:
#   ./scripts/run-local.sh                     # account "test" / pass "test123"
#   W2PP_LOCAL_ACCOUNT=jl W2PP_LOCAL_PASSWORD=secret ./scripts/run-local.sh
#   W2PP_TM_PORT=8281 ./scripts/run-local.sh
#
# Re-runnable: the account seed is idempotent and compose reuses running
# containers. Stop everything with: docker compose down
set -euo pipefail

cd "$(dirname "$0")/.."

ACCOUNT=${W2PP_LOCAL_ACCOUNT:-test}
PASSWORD=${W2PP_LOCAL_PASSWORD:-test123}
TM_PORT=${W2PP_TM_PORT:-8281}

# Pick the compose command (v2 plugin preferred, v1 fallback).
if docker compose version >/dev/null 2>&1; then
	COMPOSE=(docker compose)
elif command -v docker-compose >/dev/null 2>&1; then
	COMPOSE=(docker-compose)
else
	echo "error: docker compose is not installed" >&2
	exit 1
fi

# Best-effort LAN IP of this host (the address the Windows client dials).
lan_ip() {
	if command -v ip >/dev/null 2>&1; then
		ip -4 route get 1.1.1.1 2>/dev/null | awk '{for(i=1;i<=NF;i++) if($i=="src"){print $(i+1); exit}}'
	else
		hostname -I 2>/dev/null | awk '{print $1}'
	fi
}
HOST_IP=$(lan_ip)
[ -n "$HOST_IP" ] || HOST_IP="<this-machine-LAN-IP>"

echo "==> Building and starting the stack (db, dbserver, binserver, tmserver)…"
"${COMPOSE[@]}" up -d --build

echo "==> Seeding test account '${ACCOUNT}' (idempotent)…"
# One-off container on the compose network; waits for db (depends_on healthy) and
# reads W2PP_DB_DSN from the dbserver service env. argon2id hash, never plaintext.
"${COMPOSE[@]}" run --rm dbserver seed-account -name "$ACCOUNT" -pass "$PASSWORD"

cat <<EOF

────────────────────────────────────────────────────────────────────────
 Stack is up. Point the client (other PC) here:

     SERVER:  ${HOST_IP}:${TM_PORT}
     ACCOUNT: ${ACCOUNT}
     PASS:    ${PASSWORD}

 Next steps on the Windows machine:
   1. Run "serverlist editor.exe", set the server to ${HOST_IP}:${TM_PORT},
      generate serverlist.bin and copy it into the client folder.
   2. Make sure this host's firewall allows TCP ${TM_PORT}
      (e.g.  sudo ufw allow ${TM_PORT}/tcp ).
   3. Launch WYD.exe and log in with the account above. It has no characters —
      create one in the client.

 Logs:   ${COMPOSE[*]} logs -f tmserver
 Stop:   ${COMPOSE[*]} down
────────────────────────────────────────────────────────────────────────
EOF
