.PHONY: build binaries test lint fmt vet run run-local vuln tidy proto certs certs-clean

build:
	go build ./...

# Build each service binary into bin/.
binaries:
	go build -o bin/tmserver ./tmserver/cmd/tmserver
	go build -o bin/dbserver ./dbserver/cmd/dbserver
	go build -o bin/binserver ./binserver/cmd/binserver
	go build -o bin/webserver ./webserver/cmd/webserver

test:
	go test -race -cover ./...

vet:
	go vet ./...

# Requires golangci-lint (see README / run inside the golang:1.26 container).
lint:
	golangci-lint run

fmt:
	gofmt -w .
	goimports -w . 2>/dev/null || true

# Requires govulncheck (go install golang.org/x/vuln/cmd/govulncheck@latest).
vuln:
	govulncheck ./...

tidy:
	go mod tidy

run:
	go run ./tmserver/cmd/tmserver

# Bring up the full stack locally, seed a test account, and print the address to
# point a real client at. Override account with W2PP_LOCAL_ACCOUNT/_PASSWORD.
run-local:
	./scripts/run-local.sh

# Generate gRPC code from api/ (requires protoc + protoc-gen-go / protoc-gen-go-grpc
# on PATH; install with `go install google.golang.org/protobuf/cmd/protoc-gen-go@latest`
# and `.../grpc/cmd/protoc-gen-go-grpc@latest`).
proto:
	protoc --go_out=. --go_opt=module=github.com/jeanluca/w2pp-openwyd \
	       --go-grpc_out=. --go-grpc_opt=module=github.com/jeanluca/w2pp-openwyd \
	       api/db/v1/db.proto api/bin/v1/bin.proto api/web/v1/web.proto

# Generate dev mTLS certs into ./certs (gitignored). Apply with the mTLS overlay:
#   make certs && docker compose -f docker-compose.yaml -f docker-compose.mtls.yaml up --build
certs:
	./scripts/gen-certs.sh

certs-clean:
	rm -rf certs
