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
# rates/catalogs/BaseMob templates at boot and webserver can scan merchant
# templates + ItemList.csv for the moderator UI. Without it the char-login
# handler falls back to a template-less CNFCharacterLogin that the real client
# rejects (crash on entering the world). dbserver/binserver ignore it.
# The heavy legacy artifacts are stripped by .dockerignore.
COPY --from=build /src/Release /Release
# The storage-manager assigns opaque object names, so the validated published
# manifest is the immutable URL map consumed by webserver. Pinning its digest
# makes the production build fail instead of silently accepting changed data.
ADD --chown=65532:65532 --chmod=0444 --checksum=sha256:e6e10d3bac61792d9bae048715ff9bf5f85168c1fe18ec43c5dc900e4d57a4da https://jeanluca-teste.s3.amazonaws.com/602dce1d-bdf1-4ce6-a329-7e1fa630bec6file /published-item-icons-manifest.json
ENV W2PP_ITEM_ICONS_MANIFEST=/published-item-icons-manifest.json
ENTRYPOINT ["/app"]
