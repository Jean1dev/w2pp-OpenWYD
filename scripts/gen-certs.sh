#!/usr/bin/env bash
# Generate a development mTLS trust set for the internal gRPC links (Fase 7):
#
#   ca.crt / ca.key          local CA that signs both peers
#   server.crt / server.key  shared by dbServer + binServer (serverAuth)
#   client.crt / client.key  presented by tmServer (clientAuth)
#
# The server cert carries SANs for the docker-compose service names so the
# tmServer client can verify each peer by its dial authority (dbserver:7514,
# binserver:3000, webserver:7600) without setting W2PP_TLS_SERVER_NAME.
# localhost/127.0.0.1 are
# included for running the binaries directly on the host.
#
# These are throwaway dev certs — NEVER ship them. `certs/` is gitignored.
#
# Usage:
#   ./scripts/gen-certs.sh            # writes ./certs (skips if present)
#   FORCE=1 ./scripts/gen-certs.sh    # regenerate, overwriting existing certs
#   CERT_DIR=/tmp/c DAYS=30 ./scripts/gen-certs.sh
set -euo pipefail

CERT_DIR=${CERT_DIR:-certs}
DAYS=${DAYS:-825}        # < 825d keeps the leaf certs within common TLS limits
FORCE=${FORCE:-0}
SERVER_SAN=${SERVER_SAN:-DNS:dbserver,DNS:binserver,DNS:webserver,DNS:localhost,IP:127.0.0.1}

if [[ -f "$CERT_DIR/server.crt" && "$FORCE" != "1" ]]; then
	echo "certs already present in $CERT_DIR (set FORCE=1 to regenerate)"
	exit 0
fi

mkdir -p "$CERT_DIR"
cd "$CERT_DIR"

newkey() { openssl genpkey -algorithm EC -pkeyopt ec_paramgen_curve:P-256 -out "$1"; }

echo "==> CA"
newkey ca.key
openssl req -x509 -new -key ca.key -days "$DAYS" -out ca.crt \
	-subj "/CN=w2pp-dev-ca" \
	-addext "basicConstraints=critical,CA:TRUE,pathlen:0" \
	-addext "keyUsage=critical,keyCertSign,cRLSign"

echo "==> server (dbServer + binServer)"
newkey server.key
openssl req -new -key server.key -out server.csr -subj "/CN=w2pp-internal-server"
openssl x509 -req -in server.csr -CA ca.crt -CAkey ca.key -CAcreateserial \
	-days "$DAYS" -out server.crt \
	-extfile <(printf 'basicConstraints=critical,CA:FALSE\nkeyUsage=critical,digitalSignature,keyEncipherment\nextendedKeyUsage=serverAuth\nsubjectAltName=%s\n' "$SERVER_SAN")

echo "==> client (tmServer)"
newkey client.key
openssl req -new -key client.key -out client.csr -subj "/CN=w2pp-tmserver"
openssl x509 -req -in client.csr -CA ca.crt -CAkey ca.key -CAcreateserial \
	-days "$DAYS" -out client.crt \
	-extfile <(printf 'basicConstraints=critical,CA:FALSE\nkeyUsage=critical,digitalSignature\nextendedKeyUsage=clientAuth\nsubjectAltName=DNS:tmserver\n')

rm -f server.csr client.csr ca.srl
# World-readable on purpose: the keys are bind-mounted into the distroless
# `nonroot` (UID 65532) containers, which won't share the host UID that wrote
# them. These are throwaway dev certs (gitignored) — never reuse this for prod.
chmod 644 ./*.key

echo "==> done. mTLS material in $CERT_DIR/:"
ls -1 .
