# Dev image — canonical Go 1.26 target (guidelines §4.3, §4.8).
# Production builds will use a multi-stage scratch/distroless image (Phase 7).
FROM golang:1.26-alpine
WORKDIR /app
RUN apk add --no-cache git make
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
CMD ["sleep", "infinity"]
