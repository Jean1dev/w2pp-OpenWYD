// Package handler implements the per-message dispatch for the tmServer
// (handlers/*.md, game-rules.md). Every handler runs inside the world's
// game-loop goroutine, so it may mutate world state directly; blocking dbServer
// calls are issued off the loop via World.Go and their results re-enter the loop
// (domain-model.md §5).
//
// Batch 1 (login → character selection): AccountLogin, CreateCharacter,
// DeleteCharacter, CharacterLogin, CharacterLogout, Restart. These are mostly
// relays to the dbServer guarded by the session state machine (CUser.Mode,
// domain-model.md §3.1).
package handler

import (
	"fmt"
	"log/slog"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// Config tunes the dispatcher. Zero values get sensible defaults.
type Config struct {
	ClientVersion int32        // required client version (default AppVersion 7640)
	MaxFailLogin  int          // wrong-password lockout threshold (default 3)
	Log           *slog.Logger // default slog.Default()

	// CombineFamilies overrides the per-Type combine recipe/rate logic. When nil,
	// UNVERIFIED placeholders are installed (every recipe reports "no match").
	CombineFamilies map[protocol.Type]CombineFamily

	// BaseMobs are the per-class STRUCT_MOB templates (class index → raw 816 bytes,
	// content.LoadBaseMobs). Used to render a character on entering the world with
	// its class's starter equipment/stats. When nil, the snapshot is built from the
	// stored relational state (no equipment).
	BaseMobs map[int][]byte
}

type handlerFunc func(w *world.World, s *world.Session, h protocol.Header, payload []byte)

// Dispatcher routes decoded client frames to handlers. It is created once and
// installed as the world's Handler. Its mutable state (the per-account
// wrong-password counters) is only touched from the loop goroutine, so it needs
// no locks.
type Dispatcher struct {
	cfg             Config
	log             *slog.Logger
	routes          map[protocol.Type]handlerFunc
	fails           map[string]int // wrong-password count per account (CheckFailAccount)
	combineFamilies map[protocol.Type]CombineFamily
	baseMobs        map[int][]byte // per-class STRUCT_MOB templates
}

// New builds a Dispatcher with the batch-1 routes registered.
func New(cfg Config) *Dispatcher {
	if cfg.ClientVersion == 0 {
		cfg.ClientVersion = protocol.AppVersion
	}
	if cfg.MaxFailLogin <= 0 {
		cfg.MaxFailLogin = 3
	}
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}
	d := &Dispatcher{
		cfg:             cfg,
		log:             cfg.Log,
		routes:          make(map[protocol.Type]handlerFunc),
		fails:           make(map[string]int),
		combineFamilies: cfg.CombineFamilies,
		baseMobs:        cfg.BaseMobs,
	}
	if d.combineFamilies == nil {
		d.combineFamilies = make(map[protocol.Type]CombineFamily)
	}
	for _, ty := range combineItemTypes {
		if _, ok := d.combineFamilies[ty]; !ok {
			d.combineFamilies[ty] = defaultCombineFamily(fmt.Sprintf("combine-%#x", uint16(ty)))
		}
	}
	d.routes[protocol.MsgAccountLogin] = d.accountLogin
	d.routes[protocol.MsgCreateCharacter] = d.createCharacter
	d.routes[protocol.MsgDeleteCharacter] = d.deleteCharacter
	d.routes[protocol.MsgCharacterLogin] = d.characterLogin
	d.routes[protocol.MsgCharacterLogout] = d.characterLogout
	d.routes[protocol.MsgRestart] = d.restart
	// Batch 2 — movement & view.
	d.routes[protocol.MsgAction] = d.action
	d.routes[protocol.MsgAction2] = d.action
	d.routes[protocol.MsgAction3] = d.action
	d.routes[protocol.MsgMotion] = d.motion
	d.routes[protocol.MsgChangeCity] = d.changeCity
	d.routes[protocol.MsgReqTeleport] = d.reqTeleport
	d.routes[protocol.MsgNoViewMob] = d.noViewMob
	// Batch 3 — combat.
	d.routes[protocol.MsgAttack] = d.attack
	d.routes[protocol.MsgAttackOne] = d.attack
	d.routes[protocol.MsgAttackTwo] = d.attack
	// Batch 4 — items.
	d.routes[protocol.MsgDropItem] = d.dropItem
	d.routes[protocol.MsgGetItem] = d.getItem
	d.routes[protocol.MsgUseItem] = d.useItem
	// Batch 5 — P2P trade.
	d.routes[protocol.MsgTradingItem] = d.tradingItem
	d.routes[protocol.MsgTrade] = d.trade
	d.routes[protocol.MsgQuitTrade] = d.quitTrade
	// Batch 6 — combine/refine (one engine, all Item[]-based variants).
	for _, ty := range combineItemTypes {
		d.routes[ty] = d.combineItem
	}
	d.routes[protocol.MsgCombineItemExtracao] = d.combineExtracao
	// Batch 7 — party & guild.
	d.routes[protocol.MsgSendReqParty] = d.sendReqParty
	d.routes[protocol.MsgAcceptParty] = d.acceptParty
	d.routes[protocol.MsgRemoveParty] = d.removeParty
	d.routes[protocol.MsgInviteGuild] = d.inviteGuild
	d.routes[protocol.MsgGuildAlly] = d.guildAlly
	d.routes[protocol.MsgWar] = d.war
	d.routes[protocol.MsgChallange] = d.challange
	d.routes[protocol.MsgChallangeConfirm] = d.challangeConfirm
	// Batch 8 — chat, bonus, quest/cash (stubs).
	d.routes[protocol.MsgMessageChat] = d.messageChat
	d.routes[protocol.MsgMessageWhisper] = d.messageWhisper
	d.routes[protocol.MsgApplyBonus] = d.applyBonus
	d.routes[protocol.MsgAccountSecure] = d.accountSecure
	d.routes[protocol.MsgQuest] = d.quest
	d.routes[protocol.MsgReqRanking] = d.reqRanking
	d.routes[protocol.MsgCapsuleInfo] = d.capsuleInfo
	d.routes[protocol.MsgPutoutSeal] = d.putoutSeal
	return d
}

// Handle is the world.Handler. It runs in the loop goroutine.
func (d *Dispatcher) Handle(w *world.World, s *world.Session, h protocol.Header, payload []byte) {
	fn, ok := d.routes[h.Type]
	// DIAGNOSTIC: log every received Type (hex) so client packets can be mapped.
	d.log.Info("recv packet", "conn", s.Conn, "type", formatType(h.Type), "len", len(payload), "routed", ok)
	if !ok {
		return
	}
	fn(w, s, h, payload)
}

func formatType(t protocol.Type) string {
	const hexdigits = "0123456789abcdef"
	v := uint16(t)
	return "0x" + string([]byte{hexdigits[v>>12&0xf], hexdigits[v>>8&0xf], hexdigits[v>>4&0xf], hexdigits[v&0xf]})
}
