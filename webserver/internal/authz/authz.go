// Package authz decides who may call what on the web-api.
//
// The web-api carries sixteen services, and most of them administer the game:
// create and delete NPCs, set item prices and item stats, move donate balances,
// read revenue. Until this package existed it had one gate — mutual TLS — and
// that gate answers the wrong question. It says "is the caller one of our
// services", not "which one, and may it do this". With a single caller (the
// staff panel) the difference did not matter. It stops being free the moment a
// player-facing site becomes a second caller, because then the site's credential
// opens the game's administration too.
//
// Worse, the TLS gate degrades silently: secure.ServerCreds falls back to an
// insecure listener when no certificate is configured, so an unconfigured
// deployment serves every administrative call to anyone who can reach the port.
// The only thing standing in the way is the hosting's private network.
//
// The rule here is the one tmserver/internal/control already applies to the
// control API, quoted from its own doc: a service that starts without its
// credentials looks healthy and is not.
package authz

import (
	"context"
	"crypto/subtle"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	webv1 "github.com/jeanluca/w2pp-openwyd/api/web/v1"
)

// TokenHeader is the metadata key carrying the caller's key. Re-exported from
// the proto package, where it has to live so the panel can name it too.
const TokenHeader = webv1.TokenHeader

// servicosDoJogador are the services a player-facing site legitimately calls:
// sign-up and login, its own characters, rankings, the item catalog it renders,
// and the three that move a player's own donate balance.
//
// This is an ALLOWLIST, and that is the point. Everything not named here needs
// the panel's key, so a service somebody adds next year is closed from birth and
// has to be opened deliberately. The failure of forgetting is a caller getting a
// loud Unauthenticated, never a door left open quietly.
//
// Names are the proto service names, without the package: the full method looks
// like /web.v1.NpcAdminService/UpsertNpc.
var servicosDoJogador = map[string]bool{
	"AccountWebService":   true,
	"RankingWebService":   true,
	"CharacterWebService": true,
	"ItemCatalogService":  true,
	"DonateShopService":   true,
	"DailyRewardService":  true,
	"DonateTopupService":  true,
}

// DoJogador reports whether a proto service name is one a player-facing caller
// may reach. Exported for the guard test that keeps this list and the .proto
// from drifting apart.
func DoJogador(servico string) bool { return servicosDoJogador[servico] }

// Chaves are the keys the web-api accepts, one per caller.
//
// Two callers and not one shared secret, because the whole point is that the
// site's key must not open the administration. A key that opens everything is
// not a key, it is the absence of one.
type Chaves struct {
	// Painel opens every service. Held by the staff panel.
	Painel string
	// Site opens only servicosDoJogador. Held by the player-facing site, which
	// does not exist yet — an empty value simply matches nobody.
	Site string
}

// Configurada reports whether any key was set. A web-api with none is the
// pre-existing behaviour: it serves everything to whoever reaches the port.
func (c Chaves) Configurada() bool {
	return strings.TrimSpace(c.Painel) != "" || strings.TrimSpace(c.Site) != ""
}

// Interceptor authenticates every call against the keys.
//
// With NO key configured it lets every call through. That is the migration step
// and nothing else: this has to be deployable to a running server before the
// keys exist on either side, so the panel does not break in the window between
// the two deploys. Once the keys are set, the follow-up change makes an
// unconfigured web-api refuse to start at all, which is the end state — the
// pass-through above is precisely the behaviour being removed.
func Interceptor(c Chaves) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		if !c.Configurada() {
			return handler(ctx, req)
		}
		token, ok := tokenDe(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing web-api key")
		}
		switch {
		case confere(token, c.Painel):
			return handler(ctx, req)
		case confere(token, c.Site):
			if !DoJogador(servicoDe(info.FullMethod)) {
				// Named in the message on purpose: this is the line that says
				// the split is working, and the person reading the log is
				// usually somebody wondering why the site cannot do something.
				return nil, status.Errorf(codes.PermissionDenied,
					"the site key does not open %s", servicoDe(info.FullMethod))
			}
			return handler(ctx, req)
		default:
			return nil, status.Error(codes.Unauthenticated, "bad web-api key")
		}
	}
}

// StreamInterceptor is the same rule for streaming calls. None exist today, and
// one added later must not arrive unguarded.
func StreamInterceptor(c Chaves) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		if !c.Configurada() {
			return handler(srv, ss)
		}
		token, ok := tokenDe(ss.Context())
		if !ok {
			return status.Error(codes.Unauthenticated, "missing web-api key")
		}
		switch {
		case confere(token, c.Painel):
			return handler(srv, ss)
		case confere(token, c.Site):
			if !DoJogador(servicoDe(info.FullMethod)) {
				return status.Errorf(codes.PermissionDenied,
					"the site key does not open %s", servicoDe(info.FullMethod))
			}
			return handler(srv, ss)
		default:
			return status.Error(codes.Unauthenticated, "bad web-api key")
		}
	}
}

func tokenDe(ctx context.Context) (string, bool) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", false
	}
	vals := md.Get(TokenHeader)
	if len(vals) != 1 {
		return "", false
	}
	return vals[0], true
}

// confere compares in constant time, and never matches an unset key.
//
// The empty check is load-bearing: Site is empty until the player site exists,
// and without it a caller sending an empty key would be granted the site's
// access. Constant time because the alternative leaks the key one byte at a
// time to anybody who can measure, and this endpoint edits the whole game.
func confere(recebido, esperado string) bool {
	if strings.TrimSpace(esperado) == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(recebido), []byte(esperado)) == 1
}

// servicoDe pulls the service name out of a full method
// (/web.v1.NpcAdminService/UpsertNpc → NpcAdminService). An unparseable one
// returns the empty string, which is in no allowlist — closed by default, here
// too.
func servicoDe(fullMethod string) string {
	partes := strings.Split(strings.TrimPrefix(fullMethod, "/"), "/")
	if len(partes) != 2 {
		return ""
	}
	if i := strings.LastIndex(partes[0], "."); i >= 0 {
		return partes[0][i+1:]
	}
	return partes[0]
}
