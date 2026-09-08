package webv1

// TokenHeader carries the key that authenticates a caller to the web-api.
//
// It lives beside the generated code for the same reason the control API's does
// (api/game/v1/header.go): both sides need it and neither may import the other —
// Go's internal rule keeps the adminServer out of webserver/internal — and a
// second copy of the string would be free to drift, the failure being an
// endpoint that rejects every call with no hint as to why.
//
// Lower-case because gRPC normalises metadata keys that way; a capitalised
// constant would silently never match.
const TokenHeader = "x-w2pp-web-token"
