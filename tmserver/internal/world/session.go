package world

import (
	"net"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
)

// outFrame is a logical S→C message queued to a session's writer goroutine,
// which encodes it (CPSock) just before writing.
type outFrame struct {
	header  protocol.Header
	payload []byte
}

// Session is a player's connection/session state (CUser subset,
// domain-model.md §2.1). It is owned by the loop goroutine; the conn/out/closeCh
// plumbing is shared with this session's reader and writer goroutines only.
type Session struct {
	Conn           int // index into pUser/pMob; also HEADER.ID on the wire
	AccountName    string
	AccountID      int64
	Slot           int
	Mode           Mode
	IP             string
	CrackError     int        // anti-cheat violation count (CUser.NumError)
	Whisper        bool       // true blocks incoming whispers
	GuildDisable   bool       // hide guild tag (guildon/guildoff)
	TradeMode      int        // non-zero while in auto-trade (blocks attacks)
	Trade          TradeState // P2P direct-trade state (lote2-trade-autotrade.md)
	LastAttackTick uint32     // ClientTick of the last accepted attack (cadence gate)
	LastAttack     int        // SkillIndex of the last attack
	ShortSkill     [16]uint8  // client hotbar layout (CUser.CharShortSkill, _MSG_SetShortSkill)

	seen map[int]struct{} // entity ids already create-mob'd to this client (view set)

	conn    net.Conn
	out     chan outFrame
	closeCh chan struct{}
	closed  bool
}

// close signals the session's writer to flush any queued S→C frames and then
// close the socket (which in turn unblocks the reader). The writer owns the
// socket close so that messages queued just before a close (e.g. an error
// notice) are still delivered. Idempotent; loop-only.
func (s *Session) close() {
	if s.closed {
		return
	}
	s.closed = true
	close(s.closeCh)
}

// TradeState is a player's direct (P2P) trade with another player
// (lote2-trade-autotrade.md). Active is set when the trade window opens; Slots
// and Money are the finalized offer recorded at confirmation.
type TradeState struct {
	Active     bool
	OpponentID int
	Confirmed  bool
	Money      int32
	Slots      []int // offered carry slots
}

// Entity is a world entity (CMob subset, domain-model.md §2.2). Players
// (ID < MaxUser) and mobs (ID >= MaxUser) share this type and the same index
// space. Phase 3 carries only the minimum; full STRUCT_MOB state arrives with
// the handlers (Phase 4).
type Entity struct {
	ID     int
	Mode   EntityMode
	Name   string
	X      int16
	Y      int16
	HP     int32
	MaxHP  int32
	MP     int32 // current mana (status display)
	MaxMP  int32
	Damage int32 // CurrentScore.Damage (attacker output, combat §4.3)
	AC     int32 // CurrentScore.Ac (defender mitigation)
	Master int   // weapon mastery (combat level)
	Level  int32 // CurrentScore.Level (drop/exp curves)
	Exp    int64 // STRUCT_MOB.Exp: players accumulate it; for a mob it's the kill reward
	Coin   int32 // carried gold

	// Mob AI (mobai.go; only meaningful for monsters). Target is the current
	// combat target's conn (0 = none); AtkTick is the mob's last-attack server
	// time (cadence); SpawnX/SpawnY is the position the mob (re)spawned at.
	// Range is the mob's attack reach — the max EF_RANGE over its template's
	// equips (BASE_GetMobAbility, Basedef.cpp:2415), cached at spawn; 0 means
	// no ranged gear (the AI falls back to melee adjacency).
	Target         int
	AtkTick        uint32
	SpawnX, SpawnY int16
	Range          int16

	// Mob roaming (CMob.h:47-69, StandingByProcessor/SetSegment). SegX/SegY are
	// this INSTANCE's waypoints (already randomized ±SegmentRange at spawn,
	// GenerateMob Server.cpp:3536-3546; 0 = unused slot, skipped by the walker).
	// SegmentX/SegmentY is the current waypoint — also the aggro/leash anchor
	// (BattleProcessor leashes on SegmentX±HALFGRIDX, CMob.cpp:292) — always set,
	// even for mobs without a route (= spawn point). WaitTicks counts down the
	// pause at a waypoint (legacy WaitSec; our tick≈1s so ticks≈seconds,
	// cadence UNVERIFIED). GenIndex is the NPCGener block that spawned this mob
	// (-1 = none), reserved for the per-generator respawn accounting (M5).
	RouteType          uint8
	SegListX, SegListY [5]int16
	SegWait            [5]int16
	SegProgress        int8
	SegDir             uint8 // 0 = forward, 1 = backward (RouteType 2/3 ping-pong)
	WaitTicks          int16
	SegmentX, SegmentY int16
	GenIndex           int16
	// Template is the raw STRUCT_MOB bytes this mob was spawned from (boot template,
	// shared by reference — no copy). Retained so the mob can be re-spawned at its
	// SpawnX/SpawnY after it dies (world/respawn.go). nil for players.
	Template []byte
	Merchant uint8 // bit-packed: spawn city in bits 6-7 (lote2-movimento.md ChangeCity)
	Grade    uint8 // NPC sub-type for Merchant==100 quest NPCs (EF_GRADE0 of Equip[0])

	Class       uint8    // character class (0=TK 1=FM 2=BM 3=HT); drives the visual model
	AttackRun   uint8    // CurrentScore.AttackRun speed byte — mobs: template value (set at spawn); players: derived live (handler attackRunOf)
	Route       [24]byte // last walk route from _MSG_Action (pMob.Route, MAX_ROUTE=24)
	LastCity    int16    // last city visited (0..3); login spawn = its default area
	Clan        uint8    // clan/race
	Guild       uint16   // guild id (0 = none)
	GuildLevel  uint8    // 0 = member … 9 = leader
	ClassMaster uint8    // party tier (MobExtra.ClassMaster)

	Str        int16 // CurrentScore attributes (base + equipment, kept live by refreshScore)
	Int        int16
	Dex        int16
	Con        int16
	Special    [4]int16 // CurrentScore.Special[4] = BaseSpecial + equipment/affects
	ScoreBonus uint16   // free attribute points

	// Skill state (skills front). SkillBonus is derived (level*3 − Σ learned
	// costs, BASE_GetBonusSkillPoint) at login and level-up; SpecialBonus is
	// incremental (+2/level) and persisted. Magic scales caster skill damage;
	// SaveMana discounts mana costs (source of both on players UNVERIFIED —
	// zero until captured). Resist[4] feeds SkillResistScale (mobs: template).
	LearnedSkill int32
	SkillBonus   uint16
	SpecialBonus uint16
	BaseSpecial  [4]int16 // allocated mastery points (BaseScore.Special)
	SkillBar     [4]uint8 // MOB.SkillBar (persisted with the character)
	Magic        int16
	SaveMana     int16
	Resist       [4]int16

	// BaseScore: the equipment-free score (allocated attributes + level/class-derived
	// AC/Damage/MaxHP/MaxMP). CurrentScore (the live fields above + AC/Damage/MaxHP/
	// MaxMP) = BaseScore + equipment, recomputed by handler.refreshScore whenever gear
	// or attributes change. Derived once on login (current − equipment) and not
	// persisted (it is re-derived from the persisted CurrentScore each login).
	BaseStr, BaseInt, BaseDex, BaseCon int16
	BaseAC, BaseDamage                 int32
	BaseMaxHP, BaseMaxMP               int32

	// HpAddPct/MpAddPct: EF_HPADD/EF_MPADD percent bonus from equipment (e.g. +10 =
	// +10%). Cached by refreshScore and applied at READ time (effective max HP/MP),
	// never baked into MaxHP/MaxMP — so the persisted score stays flat and the base
	// derivation by subtraction holds (captura-wyd-affect-divina.md §E).
	HpAddPct, MpAddPct int32

	// Affect holds the active buffs/debuffs (STRUCT_AFFECT[32]). DivineEnd is the
	// wall-clock (Unix seconds) deadline of the Divine buff — the source of truth for
	// its expiry; the slot's Affect.Time is only the client icon timer.
	Affect    [MaxAffect]Affect
	DivineEnd int64

	// Rsv is the MOB.Rsv state-flag byte (RSV_HASTE/BLOCK/…), recomputed from
	// the active affects by refreshScore. The affect score contributions (Aff*)
	// are cached the same way and applied at READ time (effective getters), so
	// the persisted flat score never bakes a buff in (no double-count on
	// re-login — same policy as HpAddPct/Divine).
	Rsv           uint8
	AffDamage     int32
	AffAC         int32
	AffMaxHP      int32
	AffMaxMP      int32
	AffCon        int16
	AffSpecial    [4]int16
	AffResist     [4]int16
	AffExpBonus   int32 // from affect type 39 (Baú de XP)
	EquipExpBonus int32 // from fairy slot + grade/gem gear (CMob.cpp:711-870)

	EquipVisual [16]uint16 // visual item codes for MSG_CreateMob (gear shown to others)

	// Party state (lote2-party-guilda-guerra.md). Leader is the leader's conn
	// (0 = solo); LastReqParty is who last invited this entity (anti-forge gate).
	Leader       int
	LastReqParty int
	PartyList    [MaxParty]int

	Equip [MaxEquip]Item // equipped items
	Carry [MaxCarry]Item // inventory; for mobs this is also the loot table (§2.2)
}

// IsPlayer reports whether an entity index belongs to a player (domain-model.md
// §1: id < MaxUser ⇒ player).
func IsPlayer(id int) bool { return id >= 0 && id < MaxUser }
