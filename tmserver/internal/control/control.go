// Package control is the tmServer's inbound control surface — the first one it
// has ever had. Before this the game server listened only on the client port,
// so nothing could ask it who was playing or tell it to do anything.
//
// It exists for the admin panel: see who is connected, kick an account, send a
// global notice.
//
// Two properties shape everything here.
//
// The world is a single-owner loop. Every operation crosses into it through
// World.GoDetached and waits for the loop to answer on a channel, so nothing in
// this package ever touches world state from its own goroutine. The callbacks
// queue is drained ahead of player input, which is why these are one-shot calls
// and why the panel must not poll them on a timer.
//
// The tmServer cannot authorize a moderator. It has no database and no way to
// read account.role, so it does not try: the listener refuses to start without a
// shared token, and whoever presents it is treated as the panel. The panel is
// where the role was checked and where the audit row was written.
package control

import (
	"context"
	"crypto/subtle"
	"errors"
	"log/slog"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	gamev1 "github.com/jeanluca/w2pp-openwyd/api/game/v1"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// loopTimeout bounds how long a call waits for the game loop to answer. The loop
// drains callbacks first, so it should be immediate; the bound exists so a
// wedged loop fails the call instead of holding the connection open forever.
const loopTimeout = 5 * time.Second

// ErrNoToken is returned by NewServer when no token was configured. It is an
// error rather than a warning on purpose: secure.ServerCreds silently falls back
// to insecure credentials when TLS is unconfigured, so "wire it like the other
// services" would otherwise ship an unauthenticated kick endpoint.
var ErrNoToken = errors.New("control: refusing to serve without a token")

// ErrNoTeleporter refuses to serve without the move unstuck depends on.
var ErrNoTeleporter = errors.New("control: refusing to serve without a teleporter")

// Server implements gamev1.GameControlServiceServer over a running world.
// Teleporter moves one session's character to a position, doing everything the
// game does when a player teleports: the jump frame for their own client, the
// re-sync of what is around them, the last-city mark.
//
// It is injected rather than imported because the move belongs to the gameplay
// layer and this package is a thin door onto the world. Writing the move here
// instead would leave the server with two teleports that drift apart.
//
// Called INSIDE the game loop.
type Teleporter func(w *world.World, s *world.Session, x, y int16)

// Overlays is which moderator-editing overlays the server booted with.
//
// They are boot flags that default to off, and the panel writes to the same
// database whether they are on or not — so without this the panel accepts an
// edit, says it saved, and the game never reads it. Reported rather than
// inferred: the flags live here, in this process environment.
type Overlays struct {
	ItemStats bool
	MobStats  bool
	NPCs      bool

	// XPConfigVersion reports the Mesa de XP version this process is paying by.
	//
	// It is not a flag like the three above, and that is exactly why it belongs
	// here: the Mesa has no flag to report, because an unedited table already IS
	// the legacy behaviour. So the only honest way for the panel to tell "saved
	// and live" from "saved but not applied" is to ask the running process which
	// version it is using. Zero means it is running the legacy tables — no
	// dbServer, or the boot read failed and no reload has succeeded since.
	//
	// A function and not an int64 because the Mesa reloads while the server runs
	// (handler.pollXPConfig): a value captured at wiring time would go stale on
	// the first reload and the panel would then report a restart that is not
	// needed. Nil is allowed and reads as zero, which is what a server wired
	// without a dispatcher gets.
	XPConfigVersion func() int64

	// MountConfigVersion is when the mount overlay (curves + absorption) last
	// changed, as unix seconds, as this process read it at boot. Same job as the
	// field above and for the same reason: the mount tables have no boot flag
	// either, because an unconfigured lineage already IS the legacy behaviour.
	MountConfigVersion int64

	// QuestContent is what Common/Settings/QuestsRate.txt gave this process,
	// captured before the panel's rows were applied over it.
	//
	// It is here rather than derived by the panel because the panel cannot
	// derive it: adminServer mounts no content tree, and the numbers compiled
	// into the binary are NOT the ones this content tree ships. Reporting what
	// the process actually loaded is also stronger than reading the file — a
	// file on disk can differ from the one the running server read.
	//
	// Nil means the server booted without a content tree and is running the
	// compiled defaults.
	QuestContent []QuestTierContent
}

// QuestTierContent is one quest trophy as the content file defines it. Arch is
// absent because these quests are Mortal's alone on this server.
type QuestTierContent struct {
	Tier      int32
	MortalExp int64
	Coin      int32
	MortalMin int32
	MortalMax int32
}

type Server struct {
	gamev1.UnimplementedGameControlServiceServer
	world     *world.World
	token     string
	log       *slog.Logger
	teleporta Teleporter
	overlays  Overlays
	blocos    BlockRunner // optional; see SetBlockRunner
}

// NewServer builds the control service. It fails when the token is empty or the
// teleporter is missing.
//
// Both are required rather than optional for the same reason: a service that
// starts without them looks healthy and is not. A missing token would serve an
// open endpoint that can kick every player off; a missing teleporter would
// accept unstuck calls and answer that it moved nobody.
func NewServer(w *world.World, token string, log *slog.Logger, tp Teleporter, ov Overlays) (*Server, error) {
	if strings.TrimSpace(token) == "" {
		return nil, ErrNoToken
	}
	if tp == nil {
		return nil, ErrNoTeleporter
	}
	return &Server{world: w, token: token, log: log, teleporta: tp, overlays: ov}, nil
}

// Overlays answers which moderator-editing overlays are active.
//
// It does NOT cross the game loop, unlike every other call here: three of these
// are boot flags fixed for the life of the process, and the two versions are
// published through atomics for exactly this reason. Making the panel wait
// behind player input to read them would be a cost with nothing bought.
// xpConfigVersion reads the live Mesa version, or zero when nothing published it.
func (s *Server) xpConfigVersion() int64 {
	if s.overlays.XPConfigVersion == nil {
		return 0
	}
	return s.overlays.XPConfigVersion()
}

func (s *Server) Overlays(_ context.Context, _ *gamev1.OverlaysRequest) (*gamev1.OverlaysResponse, error) {
	resp := &gamev1.OverlaysResponse{
		ItemStats:          s.overlays.ItemStats,
		MobStats:           s.overlays.MobStats,
		Npcs:               s.overlays.NPCs,
		XpConfigVersion:    s.xpConfigVersion(),
		MountConfigVersion: s.overlays.MountConfigVersion,
	}
	for _, q := range s.overlays.QuestContent {
		resp.QuestContent = append(resp.QuestContent, &gamev1.QuestContentRate{
			Tier: q.Tier, MortalExp: q.MortalExp, Coin: q.Coin,
			MortalMin: q.MortalMin, MortalMax: q.MortalMax,
		})
	}
	return resp, nil
}

// Interceptor authenticates every call against the shared token.
func (s *Server) Interceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing metadata")
		}
		vals := md.Get(gamev1.TokenHeader)
		// Constant time, and only after the length is known to match: comparing
		// with == would leak the token one byte at a time to a caller who can
		// measure, and this endpoint can kick every player off the server.
		if len(vals) != 1 || subtle.ConstantTimeCompare([]byte(vals[0]), []byte(s.token)) != 1 {
			return nil, status.Error(codes.Unauthenticated, "bad control token")
		}
		return handler(ctx, req)
	}
}

// noLoop runs fn inside the game loop and returns its value.
//
// GoDetached runs the outer function off the loop and applies the returned
// callback inside it, so the callback is where the world may be touched. The
// channel is buffered so the loop never blocks on a caller that has already
// given up.
func noLoop[T any](ctx context.Context, w *world.World, fn func(*world.World) T) (T, error) {
	ch := make(chan T, 1)
	w.GoDetached(func() func(*world.World) {
		return func(w *world.World) { ch <- fn(w) }
	})
	ctx, cancel := context.WithTimeout(ctx, loopTimeout)
	defer cancel()
	select {
	case v := <-ch:
		return v, nil
	case <-ctx.Done():
		var zero T
		return zero, status.Error(codes.DeadlineExceeded, "the game loop did not answer")
	}
}

// ListOnline reports the sessions the server holds.
func (s *Server) ListOnline(ctx context.Context, _ *gamev1.ListOnlineRequest) (*gamev1.ListOnlineResponse, error) {
	out, err := noLoop(ctx, s.world, func(w *world.World) *gamev1.ListOnlineResponse {
		resp := &gamev1.ListOnlineResponse{}
		w.ForEachSession(func(sess *world.Session, e *world.Entity) {
			p := &gamev1.OnlinePlayer{
				AccountName: sess.AccountName,
				Ip:          sess.IP,
				InPlay:      e != nil,
			}
			if e != nil {
				p.CharacterName = e.Name
				p.Level = e.Level
				p.MapX, p.MapY = int32(e.X), int32(e.Y)
				resp.InPlay++
			}
			resp.Connected++
			resp.Players = append(resp.Players, p)
		})
		return resp
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Kick ends every session of one account.
//
// By account name, not character: the panel works in accounts, and a player
// still on the character screen has no character name to match on. That is
// exactly how the in-game /gm ban misses people — it resolves through the
// entity name and only for sessions already in play.
func (s *Server) Kick(ctx context.Context, req *gamev1.KickRequest) (*gamev1.KickResponse, error) {
	nome := strings.TrimSpace(req.GetAccountName())
	if nome == "" {
		return nil, status.Error(codes.InvalidArgument, "account name is required")
	}
	n, err := noLoop(ctx, s.world, func(w *world.World) int32 {
		var alvos []*world.Session
		w.ForEachSession(func(sess *world.Session, _ *world.Entity) {
			if strings.EqualFold(sess.AccountName, nome) {
				alvos = append(alvos, sess)
			}
		})
		// Collected first, closed after: Close mutates the session table that
		// ForEachSession is walking.
		for _, sess := range alvos {
			w.Close(sess)
		}
		return int32(len(alvos))
	})
	if err != nil {
		return nil, err
	}
	if n > 0 {
		s.log.Info("control: account kicked", "account", nome, "sessions", n)
	}
	return &gamev1.KickResponse{Sessions: n}, nil
}

// Unstuck moves a character to the nearest city.
//
// The world has spots a player can enter and not leave: a gap in the collision
// map, a warp that lands inside scenery. Reconnecting does not help — the
// position is saved, so they come back exactly where they were stuck — and the
// only remedy before this was editing the database by hand, with the character
// offline.
//
// The nearest city rather than a fixed one: being rescued should not also cost
// the player the walk back. And it refuses a session that is not in the world,
// because a character still on the selection screen has no position to fix and
// moving "them" would mean writing to a character nobody has loaded.
func (s *Server) Unstuck(ctx context.Context, req *gamev1.UnstuckRequest) (*gamev1.UnstuckResponse, error) {
	nome := strings.TrimSpace(req.GetAccountName())
	if nome == "" {
		return nil, status.Error(codes.InvalidArgument, "account name is required")
	}
	out, err := noLoop(ctx, s.world, func(w *world.World) *gamev1.UnstuckResponse {
		resp := &gamev1.UnstuckResponse{}
		var alvo *world.Session
		var ent *world.Entity
		w.ForEachSession(func(sess *world.Session, e *world.Entity) {
			// e == nil is the character screen: no position, nothing to unstick.
			if e != nil && alvo == nil && strings.EqualFold(sess.AccountName, nome) {
				alvo, ent = sess, e
			}
		})
		if alvo == nil {
			return resp
		}
		destX, destY, _, cidade := world.NearestCitySpawn(ent.X, ent.Y)
		// An explicit destination overrides the rescue. Bounds-checked here and
		// not trusted from the wire: the panel is authenticated, but a typo in a
		// coordinate box should not put a character outside the grid.
		if x, y := req.GetToX(), req.GetToY(); x > 0 && y > 0 {
			dim := int32(w.GridDim())
			if x >= dim || y >= dim {
				return resp // Found stays false: nothing moved
			}
			destX, destY, cidade = int16(x), int16(y), ""
		}
		resp.Found = true
		resp.CharacterName = ent.Name
		resp.FromX, resp.FromY = int32(ent.X), int32(ent.Y)
		resp.ToX, resp.ToY = int32(destX), int32(destY)
		resp.City = cidade
		s.teleporta(w, alvo, destX, destY)
		return resp
	})
	if err != nil {
		return nil, err
	}
	if out.GetFound() {
		s.log.Info("control: player unstuck", "account", nome, "character", out.GetCharacterName(),
			"from_x", out.GetFromX(), "from_y", out.GetFromY(),
			"to_x", out.GetToX(), "to_y", out.GetToY(), "city", out.GetCity())
	}
	return out, nil
}

// DeliverNow empties an account's item mailbox without waiting for its login.
//
// delivery_queue is drained once, at account login, straight into the warehouse
// (World.ApplyDeliveries). That is right for the donate shop and wrong for
// support: the player is standing there, and the panel could only tell them to
// log out and back in.
//
// Nothing new is granted here — same mailbox, same placement the login path
// runs, only sooner. It is three steps because the middle one talks to the
// database and must not run inside the loop:
//
//  1. in the loop, find the session and take its account id;
//  2. off the loop, read the mailbox;
//  3. in the loop, place the items.
//
// The session is looked up AGAIN in step 3 rather than carried across. Between
// the read and the placement the player can disconnect, and a session pointer
// held across that gap would be written to after the world dropped it. Losing
// the race simply reports nothing delivered: the mailbox rows are untouched, so
// the next login drains them.
func (s *Server) DeliverNow(ctx context.Context, req *gamev1.DeliverNowRequest) (*gamev1.DeliverNowResponse, error) {
	nome := strings.TrimSpace(req.GetAccountName())
	if nome == "" {
		return nil, status.Error(codes.InvalidArgument, "account name is required")
	}

	type alvo struct {
		accountID  int64
		personagem string
		persist    world.Persistence
	}
	quem, err := noLoop(ctx, s.world, func(w *world.World) alvo {
		var a alvo
		w.ForEachSession(func(sess *world.Session, e *world.Entity) {
			if a.accountID != 0 || !strings.EqualFold(sess.AccountName, nome) {
				return
			}
			a.accountID = sess.AccountID
			if e != nil {
				a.personagem = e.Name
			}
		})
		// Read in the loop and carried out, rather than reached for off it: the
		// field is set once at construction, and taking it here keeps the rule
		// "world state is read in the loop" without an exception to remember.
		a.persist = w.Persistence()
		return a
	})
	if err != nil {
		return nil, err
	}
	if quem.accountID == 0 {
		return &gamev1.DeliverNowResponse{}, nil
	}

	pendentes, err := quem.persist.ListPendingDeliveries(ctx, quem.accountID)
	if err != nil {
		s.log.Warn("control: mailbox read failed", "account", nome, "err", err)
		return nil, status.Error(codes.Unavailable, "could not read the mailbox")
	}
	if len(pendentes) == 0 {
		return &gamev1.DeliverNowResponse{Found: true, CharacterName: quem.personagem}, nil
	}

	out, err := noLoop(ctx, s.world, func(w *world.World) *gamev1.DeliverNowResponse {
		resp := &gamev1.DeliverNowResponse{CharacterName: quem.personagem}
		w.ForEachSession(func(sess *world.Session, _ *world.Entity) {
			if resp.Found || sess.AccountID != quem.accountID {
				return
			}
			resp.Found = true
			// Lost keeps its wire name, but no item is lost any more: it counts
			// the grants that found no free slot and stay in the mailbox.
			entregues, semEspaco := w.ApplyDeliveries(sess, pendentes)
			resp.Delivered, resp.Lost = int32(entregues), int32(semEspaco)
		})
		return resp
	})
	if err != nil {
		return nil, err
	}
	s.log.Info("control: mailbox drained on request", "account", nome,
		"delivered", out.GetDelivered(), "held", out.GetLost())
	return out, nil
}

// maxBroadcast bounds the notice. The wire body is a fixed 96 bytes and the
// encoder truncates silently, so a longer message would arrive cut with no sign
// that it had been.
const maxBroadcast = protocol.MessageLength - 1

// Broadcast sends a notice to everyone in play.
func (s *Server) Broadcast(ctx context.Context, req *gamev1.BroadcastRequest) (*gamev1.BroadcastResponse, error) {
	msg := strings.TrimSpace(req.GetMessage())
	if msg == "" {
		return nil, status.Error(codes.InvalidArgument, "message is required")
	}
	if len(msg) > maxBroadcast {
		return nil, status.Errorf(codes.InvalidArgument,
			"message is %d bytes; the client carries %d", len(msg), maxBroadcast)
	}
	n, err := noLoop(ctx, s.world, func(w *world.World) int32 {
		// The same encoder and the same ID the three existing broadcasts use
		// (towerNotice, broadcastWorldEventNotice, noticeArea). A fourth spelling
		// of the same packet is how they drift apart.
		body := protocol.EncodeMessageChatBody(msg)
		var n int32
		w.ForEachPlaying(-1, func(sess *world.Session, _ *world.Entity) {
			w.SendTo(sess, protocol.Header{Type: protocol.MsgMessageChat, ID: 0}, body)
			n++
		})
		return n
	})
	if err != nil {
		return nil, err
	}
	s.log.Info("control: broadcast sent", "recipients", n)
	return &gamev1.BroadcastResponse{Recipients: n}, nil
}

// drainTimeout bounds the whole emptying. It is generous because the point is to
// finish the saves, not to be quick: a full server writing one row per player is
// still far inside this, and a database slow enough to exceed it is a database
// that would have lost the data on shutdown anyway.
const drainTimeout = 2 * time.Minute

// Drain empties the server and waits for every save to land.
//
// The order matters. The notice goes first, while there is still somebody to
// read it. The sessions end next, and each teardown queues that character's save
// and releases their cargo. Only then does this wait — off the loop, because the
// saves run off the loop and the loop must keep serving whatever is left.
//
// After this returns, a restart has nothing to lose: the shutdown path saves
// whoever is in play, and nobody is.
func (s *Server) Drain(ctx context.Context, req *gamev1.DrainRequest) (*gamev1.DrainResponse, error) {
	out := &gamev1.DrainResponse{}

	if msg := strings.TrimSpace(req.GetMessage()); msg != "" {
		if len(msg) > maxBroadcast {
			return nil, status.Errorf(codes.InvalidArgument,
				"message is %d bytes; the client carries %d", len(msg), maxBroadcast)
		}
		aviso, err := s.Broadcast(ctx, &gamev1.BroadcastRequest{Message: msg})
		if err != nil {
			return nil, err
		}
		out.Notified = aviso.GetRecipients()
	}

	n, err := noLoop(ctx, s.world, func(w *world.World) int32 {
		var alvos []*world.Session
		w.ForEachSession(func(sess *world.Session, _ *world.Entity) {
			alvos = append(alvos, sess)
		})
		// Collected first, closed after: Close mutates the table being walked.
		// Each Close saves the character and releases the account cargo.
		for _, sess := range alvos {
			w.Close(sess)
		}
		return int32(len(alvos))
	})
	if err != nil {
		return nil, err
	}
	out.Kicked = n

	// The saves are already running; this is where the caller's patience is
	// spent instead of the platform's kill timer.
	pronto := make(chan struct{})
	go func() { defer close(pronto); s.world.WaitSaves() }()

	espera, cancel := context.WithTimeout(ctx, drainTimeout)
	defer cancel()
	select {
	case <-pronto:
		s.log.Info("control: drained", "notified", out.Notified, "kicked", out.Kicked)
		return out, nil
	case <-espera.Done():
		// Saying so beats reporting success: the operator is about to restart,
		// and that decision changes if the writes have not landed.
		s.log.Error("control: drain timed out waiting for saves", "kicked", out.Kicked)
		return nil, status.Error(codes.DeadlineExceeded,
			"the sessions were ended but the saves have not finished; do not restart yet")
	}
}
