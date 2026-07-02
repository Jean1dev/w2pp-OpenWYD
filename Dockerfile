# syntax=docker/dockerfile:1
# Production multi-stage build (guidelines §4.8). Parameterized by service:
#   docker build --build-arg SVC=tmserver -t w2pp-tmserver .
# SVC is one of {tmserver, dbserver, binserver, webserver}; each has
# cmd/<SVC>/main.go.
FROM golang:1.26-alpine AS build
WORKDIR /src
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG SVC
#RUN test -n "$SVC" || (echo "SVC build-arg is required" && false)
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/app ./${SVC}/cmd/${SVC}

# Distroless static: minimal, includes CA certs, runs as nonroot.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/app /app
# Bake the game content tree so tmServer (W2PP_CONTENT=/Release) can load
# rates/catalogs/BaseMob templates at boot. Without it the char-login handler
# falls back to a template-less CNFCharacterLogin that the real client rejects
# (crash on entering the world). Only tmServer reads it; other services ignore it.
# The heavy legacy artifacts are stripped by .dockerignore.
COPY --from=build /src/Release /Release
ENTRYPOINT ["/app"]
