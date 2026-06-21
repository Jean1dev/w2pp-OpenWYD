.PHONY: build test lint fmt vet run vuln tidy proto

build:
	go build ./...

# Build each service binary into bin/.
binaries:
	go build -o bin/tmserver ./tmserver/cmd/tmserver
	go build -o bin/dbserver ./dbserver/cmd/dbserver
	go build -o bin/binserver ./binserver/cmd/binserver

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

# Generate gRPC code from api/ (requires protoc + protoc-gen-go / protoc-gen-go-grpc).
proto:
	protoc --go_out=. --go_opt=module=github.com/jeanluca/w2pp-openwyd \
	       --go-grpc_out=. --go-grpc_opt=module=github.com/jeanluca/w2pp-openwyd \
	       api/db/v1/db.proto
