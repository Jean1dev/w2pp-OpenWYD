.PHONY: build test lint fmt vet run vuln tidy

build:
	go build ./...

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
	go run ./cmd/tmserver
