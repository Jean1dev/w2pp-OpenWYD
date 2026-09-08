// Package jogo is the panel's link to the RUNNING game server.
//
// It is the sibling of package gamedata and the distinction matters: gamedata
// talks to the webServer about cold config — item prices, NPCs, mob stats —
// which the game re-reads later. This package talks to the tmServer about the
// world as it is right now, and every call has an immediate effect on people
// who are playing.
//
// The tmServer cannot check who is asking: it has no database and no way to read
// account.role. It authenticates this client by shared token and trusts that the
// panel checked the role and wrote the audit row, which the panel does.
package jogo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	gamev1 "github.com/jeanluca/w2pp-openwyd/api/game/v1"
)

// callTimeout bounds a call. The control API answers from inside the game loop,
// which drains its queue first, so this is about a server that is gone rather
// than one that is busy.
const callTimeout = 8 * time.Second

// Refusals the panel can tell apart and explain.
var (
	// ErrRecusado means the tmServer rejected the token — the two sides are
	// configured with different secrets.
	ErrRecusado = errors.New("jogo: the game server refused our token")
	// ErrForaDoAr means the tmServer could not be reached at all.
	ErrForaDoAr = errors.New("jogo: the game server is not answering")
	// ErrInvalido means the request itself was rejected.
	ErrInvalido = errors.New("jogo: the game server rejected the request")
)

// Player is one session the game server is holding.
type Player struct {
	Conta      string
	Personagem string // empty while still on the character screen
	Nivel      int32
	X, Y       int32
	Jogando    bool
	IP         string
}

// Estado is what the server answers about who is connected.
type Estado struct {
	Players    []Player
	Jogando    int32 // sessions with a character in the world
	Conectados int32 // every session, including the character screen
}

// Client calls the tmServer control API.
type Client struct {
	api   gamev1.GameControlServiceClient
	token string
}

// New wraps a connection. The token must match the tmServer's.
func New(conn grpc.ClientConnInterface, token string) *Client {
	return &Client{api: gamev1.NewGameControlServiceClient(conn), token: token}
}

// ctx attaches the token and the deadline.
func (c *Client) ctx(parent context.Context) (context.Context, context.CancelFunc) {
	md := metadata.Pairs(gamev1.TokenHeader, c.token)
	ctx, cancel := context.WithTimeout(metadata.NewOutgoingContext(parent, md), callTimeout)
	return ctx, cancel
}

// traduz turns a gRPC status into something the panel can put on a page.
//
// The three cases lead to different actions: a refused token is a
// misconfiguration somebody has to fix, an unreachable server is something to
// wait out, and an invalid argument is the operator's own input.
func traduz(err error, oque string) error {
	switch status.Code(err) {
	case codes.OK:
		return nil
	case codes.Unauthenticated:
		return fmt.Errorf("%s: %w", oque, ErrRecusado)
	case codes.InvalidArgument:
		return fmt.Errorf("%s: %w: %s", oque, ErrInvalido, status.Convert(err).Message())
	case codes.Unavailable, codes.DeadlineExceeded:
		return fmt.Errorf("%s: %w", oque, ErrForaDoAr)
	default:
		return fmt.Errorf("jogo: %s: %w", oque, err)
	}
}

// Estado lists the sessions the server is holding.
func (c *Client) Estado(parent context.Context) (Estado, error) {
	ctx, cancel := c.ctx(parent)
	defer cancel()
	resp, err := c.api.ListOnline(ctx, &gamev1.ListOnlineRequest{})
	if err != nil {
		return Estado{}, traduz(err, "ler quem está online")
	}
	out := Estado{Jogando: resp.GetInPlay(), Conectados: resp.GetConnected()}
	for _, p := range resp.GetPlayers() {
		out.Players = append(out.Players, Player{
			Conta: p.GetAccountName(), Personagem: p.GetCharacterName(),
			Nivel: p.GetLevel(), X: p.GetMapX(), Y: p.GetMapY(),
			Jogando: p.GetInPlay(), IP: p.GetIp(),
		})
	}
	return out, nil
}

// Derrubar ends every session of one account and reports how many there were.
// Zero is an answer, not a failure: the account was simply not connected.
func (c *Client) Derrubar(parent context.Context, conta string) (int32, error) {
	ctx, cancel := c.ctx(parent)
	defer cancel()
	resp, err := c.api.Kick(ctx, &gamev1.KickRequest{AccountName: conta})
	if err != nil {
		return 0, traduz(err, "derrubar a conta")
	}
	return resp.GetSessions(), nil
}

// Desatolo is what an unstuck did, for the confirmation the panel shows and the
// audit line it writes.
type Desatolo struct {
	// Achou is false when the account was not in the world. Not an error: the
	// player may have logged off between the report and the click.
	Achou      bool
	Personagem string
	DeX, DeY   int32
	ParaX      int32
	ParaY      int32
	Cidade     string
}

// Desatolar moves a character out of where it is: to the nearest city when
// paraX/paraY are zero, or to that exact tile otherwise.
//
// The explicit destination is the same operation with a different target, and it
// is what lets staff reach a place the game has no entry for — an area whose
// quest is not implemented has, by definition, no other way in.
func (c *Client) Desatolar(parent context.Context, conta string, paraX, paraY int32) (Desatolo, error) {
	ctx, cancel := c.ctx(parent)
	defer cancel()
	resp, err := c.api.Unstuck(ctx, &gamev1.UnstuckRequest{AccountName: conta, ToX: paraX, ToY: paraY})
	if err != nil {
		return Desatolo{}, traduz(err, "desatolar o personagem")
	}
	return Desatolo{
		Achou: resp.GetFound(), Personagem: resp.GetCharacterName(),
		DeX: resp.GetFromX(), DeY: resp.GetFromY(),
		ParaX: resp.GetToX(), ParaY: resp.GetToY(), Cidade: resp.GetCity(),
	}, nil
}

// Entrega is what a deliver-now did.
type Entrega struct {
	// Conectado is false when the account is not on the server. Not a failure:
	// the mailbox keeps the items and the next login drains them.
	Conectado bool
	Entregues int32
	// Perdidos counts items the warehouse had no room for. The login drain loses
	// them the same way — the panel has to be able to say so instead of
	// reporting a delivery that did not happen.
	Perdidos   int32
	Personagem string
}

// EntregarAgora drains an account mailbox without waiting for its next login.
func (c *Client) EntregarAgora(parent context.Context, conta string) (Entrega, error) {
	ctx, cancel := c.ctx(parent)
	defer cancel()
	resp, err := c.api.DeliverNow(ctx, &gamev1.DeliverNowRequest{AccountName: conta})
	if err != nil {
		return Entrega{}, traduz(err, "entregar agora")
	}
	return Entrega{
		Conectado: resp.GetFound(), Entregues: resp.GetDelivered(),
		Perdidos: resp.GetLost(), Personagem: resp.GetCharacterName(),
	}, nil
}

// Overlays is which moderator-editing overlays the game booted with.
//
// The panel writes to the same database whether they are on or not, so without
// asking it would accept an edit, say it saved, and leave it inert forever.
type Overlays struct {
	AtributosDeItem    bool
	AtributosDeMonstro bool
	NPCs               bool

	// VersaoMesaXP is the Mesa de XP version the game process loaded at boot.
	// Unlike the three flags above there is nothing to switch on: an unedited
	// table already is the legacy behaviour, so the only way to know whether a
	// saved table is live is to ask which version the process read. Zero means
	// it booted without a Mesa and is running the legacy tables.
	VersaoMesaXP int64

	// VersaoMontarias é quando o overlay de montarias (curvas + absorção) mudou
	// pela última vez, em segundos unix, como o processo leu no boot. Mesmo papel
	// do campo acima e pelo mesmo motivo: uma linhagem sem configuração já É o
	// comportamento do legado, então não há bandeira para ligar — só dá para
	// saber perguntando que número o processo leu.
	VersaoMontarias int64

	// QuestsDoConteudo é o que o arquivo de conteúdo paga por troféu de quest,
	// como o processo do jogo o leu — antes de as linhas do painel entrarem por
	// cima. Vazio quer dizer que o servidor subiu sem árvore de conteúdo, e aí
	// está rodando os padrões compilados.
	//
	// O painel mostrava, sob o rótulo "no arquivo", as constantes do próprio
	// binário dele. Nesta árvore de conteúdo elas são muito menores, então uma
	// edição que parecia aumentar a recompensa estava cortando.
	QuestsDoConteudo []QuestDoConteudo
}

// QuestDoConteudo é um troféu de quest como o arquivo de conteúdo o define.
// Arch não aparece porque estas quests são só de Mortal neste servidor.
type QuestDoConteudo struct {
	Tier      int32
	MortalExp int64
	Coin      int32
	MortalMin int32
	MortalMax int32
}

// Ajustes asks which overlays are active.
func (c *Client) Ajustes(parent context.Context) (Overlays, error) {
	ctx, cancel := c.ctx(parent)
	defer cancel()
	resp, err := c.api.Overlays(ctx, &gamev1.OverlaysRequest{})
	if err != nil {
		return Overlays{}, traduz(err, "perguntar o que o servidor está lendo")
	}
	ov := Overlays{
		AtributosDeItem:    resp.GetItemStats(),
		AtributosDeMonstro: resp.GetMobStats(),
		NPCs:               resp.GetNpcs(),
		VersaoMesaXP:       resp.GetXpConfigVersion(),
		VersaoMontarias:    resp.GetMountConfigVersion(),
	}
	for _, q := range resp.GetQuestContent() {
		ov.QuestsDoConteudo = append(ov.QuestsDoConteudo, QuestDoConteudo{
			Tier: q.GetTier(), MortalExp: q.GetMortalExp(), Coin: q.GetCoin(),
			MortalMin: q.GetMortalMin(), MortalMax: q.GetMortalMax(),
		})
	}
	return ov, nil
}

// Avisar sends a notice to everyone in play and reports how many got it.
func (c *Client) Avisar(parent context.Context, msg string) (int32, error) {
	ctx, cancel := c.ctx(parent)
	defer cancel()
	resp, err := c.api.Broadcast(ctx, &gamev1.BroadcastRequest{Message: msg})
	if err != nil {
		return 0, traduz(err, "enviar o aviso")
	}
	return resp.GetRecipients(), nil
}

// drainTimeout is separate from callTimeout and much longer: emptying the server
// waits for every character save to land, which is the whole point of it, and
// eight seconds would abandon exactly the work being waited on.
const drainTimeout = 3 * time.Minute

// Drenagem is what emptying the server accomplished.
type Drenagem struct {
	Avisados   int32
	Derrubados int32
}

// Drenar empties the server and waits for every save to finish.
//
// An error here means the saves did NOT finish. The caller must not restart on
// it: the sessions are already gone, so a restart would drop whatever had not
// been written — which is the exact loss this call exists to avoid.
func (c *Client) Drenar(parent context.Context, aviso string) (Drenagem, error) {
	md := metadata.Pairs(gamev1.TokenHeader, c.token)
	ctx, cancel := context.WithTimeout(metadata.NewOutgoingContext(parent, md), drainTimeout)
	defer cancel()

	resp, err := c.api.Drain(ctx, &gamev1.DrainRequest{Message: aviso})
	if err != nil {
		return Drenagem{}, traduz(err, "esvaziar o servidor")
	}
	return Drenagem{Avisados: resp.GetNotified(), Derrubados: resp.GetKicked()}, nil
}
