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

// AccessLevel is a session's GM/moderation privilege tier, derived from the
// account.role string at login (issue #122). It replaces the legacy fragile
// "character Level >= 1000" backdoor with an explicit server-side authority.
// Ordered so a gate can compare with >= (a higher tier passes lower-tier gates).
type AccessLevel uint8

// Access tiers. Player is the zero value: an unknown/absent role is never a GM.
const (
	AccessPlayer    AccessLevel = iota // no GM privilege
	AccessModerator                    // in-game moderation (the first-batch commands)
	AccessAdmin                        // reserved for future destructive/server-wide ops
)

// ParseAccess maps an account.role string ('player'/'moderator'/'admin') to its
// AccessLevel. Any unrecognized value is AccessPlayer (fail closed).
func ParseAccess(role string) AccessLevel {
	switch role {
	case "admin":
		return AccessAdmin
	case "moderator":
		return AccessModerator
	default:
		return AccessPlayer
	}
}

// String renders the tier for audit logs.
func (a AccessLevel) String() string {
	switch a {
	case AccessAdmin:
		return "admin"
	case AccessModerator:
		return "moderator"
	default:
		return "player"
	}
}

// Session is a player's connection/session state (CUser subset,
// domain-model.md §2.1). It is owned by the loop goroutine; the conn/out/closeCh
// plumbing is shared with this session's reader and writer goroutines only.
type Session struct {
	Conn              int // index into pUser/pMob; also HEADER.ID on the wire
	AccountName       string
	AccountID         int64
	AccessLevel       AccessLevel // account.role tier; gates in-game GM commands (issue #122)
	Slot              int
	Mode              Mode
	IP                string
	CrackError        int        // anti-cheat violation count (CUser.NumError)
	Whisper           bool       // true blocks incoming whispers
	GuildDisable      bool       // hide guild tag (guildon/guildoff)
	TradeMode         int        // non-zero while in auto-trade (blocks attacks)
	Trade             TradeState // P2P direct-trade state (lote2-trade-autotrade.md)
	LastAttackTick    uint32     // ClientTick of the last accepted attack (cadence gate)
	PotionTick        uint32     // CUser.PotionTime: server clock of the last accepted potion
	LastAttack        int        // SkillIndex of the last attack
	LastIllusionTick  uint32     // ClientTick of the last Huntress Ilusao movement
	ReqHp             int32      // CUser.ReqHp: server-owned HP target for regen/potions
	ReqMp             int32      // CUser.ReqMp: server-owned MP target for regen/potions
	CriticalProgress  uint16     // CUser.cProgress used by BASE_GetDoubleCritical
	ShortSkill        [16]uint8  // client hotbar layout (CUser.CharShortSkill, _MSG_SetShortSkill)
	LoginSpawnX       int16      // last server-injected login spawn, for movement diagnostics
	LoginSpawnY       int16
	LoginTick         uint32
	LoggedFirstAction bool // first post-login _MSG_Action diagnostic was emitted

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
	ID   int
	Mode EntityMode
	Name string
	X    int16
	Y    int16
	// SaveX/SaveY are the Gema Estelar warp save-point (STRUCT_MOB.SPX/SPY,
	// _MSG_UseItem.cpp Vol 12/13) — distinct from X/Y, the player's live position.
	// 0/0 means no point has ever been saved.
	SaveX    int16
	SaveY    int16
	HP       int32
	MaxHP    int32
	MP       int32 // current mana (status display)
	MaxMP    int32
	Damage   int32 // CurrentScore.Damage (attacker output, combat §4.3)
	AC       int32 // CurrentScore.Ac (defender mitigation)
	Master   int   // weapon mastery (combat level)
	Critical uint8 // MOB.Critical: rand()%255 threshold for partial critical
	Parry    int   // CMob.Parry addend for GetParryRate
	Level    int32 // CurrentScore.Level (drop/exp curves)
	Exp      int64 // STRUCT_MOB.Exp: players accumulate it; for a mob it's the kill reward
	Coin     int32 // carried gold

	// Mob AI (mobai.go; only meaningful for monsters). EnemyList is the legacy
	// CMob.EnemyList[MAX_ENEMY=13]; Target is the currently selected entry
	// (0 = none). AtkTick is the mob's last-attack server time (cadence);
	// SpawnX/SpawnY is the position the mob (re)spawned at. Range is the mob's
	// attack reach — the max EF_RANGE over its template's equips
	// (BASE_GetMobAbility, Basedef.cpp:2415), cached at spawn; 0 means no ranged
	// gear (the AI falls back to melee adjacency).
	Target         int
	EnemyList      [protocol.MaxTarget]int
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
	Citizen     uint8    // MobExtra.Citizen; city allegiance and guild creation metadata
	ClassMaster uint8    // party tier (MobExtra.ClassMaster)
	Soul        uint8    // MobExtra.Soul; 0 means no modeled soul
	Fame        int32    // MobExtra.Fame; loaded from DB, updated by Selo do Guerreiro, and shown by /nick
	QuestFlag   uint8    // volatile quest-area pass (CMob.QuestFlag; e.g. Quest 256)
	// PKMode is the player-toggled Player-Killer consent flag (K key, _MSG_PKMode;
	// legacy pUser[conn].PKMode). It gates whether the player can land PvP combat
	// hits, but it does NOT by itself blink the nickname.
	// GuiltyUntil is the wall-clock deadline (Unix seconds) of the "chaotic" state
	// set when a PvP hit actually lands (refreshed on every hit); the nickname
	// blinks red while now < GuiltyUntil and reverts once it decays with no further
	// PvP (handler.pkGuiltyDuration). Neither is persisted (session-only).
	PKMode      bool
	GuiltyUntil int64

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
	LearnedSkill    int32
	SecLearnedSkill int32
	SkillBonus      uint16
	SpecialBonus    uint16
	BaseSpecial     [4]int16 // allocated mastery points (BaseScore.Special)
	SkillBar        [4]uint8 // MOB.SkillBar (persisted with the character)
	Magic           int16
	SaveMana        int16
	Resist          [4]int16

	// BaseScore: the equipment-free score (allocated attributes + level/class-derived
	// AC/Damage/MaxHP/MaxMP). CurrentScore (the live fields above + AC/Damage/MaxHP/
	// MaxMP) = BaseScore + equipment, recomputed by handler.refreshScore whenever gear
	// or attributes change. Derived once on login (current − equipment) and not
	// persisted (it is re-derived from the persisted CurrentScore each login).
	BaseStr, BaseInt, BaseDex, BaseCon int16
	BaseAC, BaseDamage                 int32
	BaseMaxHP, BaseMaxMP               int32
	// BaseMagic/BaseParry/BaseResist: the equipment-free Magic/Parry(evasion)/Resist,
	// the mount-bonus counterparts of the Base* fields above. Same policy — derived on
	// login (current − equipment) and re-added by refreshScore, never persisted.
	BaseMagic  int16
	BaseParry  int
	BaseResist [4]int16

	// HpAddPct/MpAddPct: EF_HPADD/EF_MPADD percent bonus from equipment (e.g. +10 =
	// +10%). Cached by refreshScore and applied at READ time (effective max HP/MP),
	// never baked into MaxHP/MaxMP — so the persisted score stays flat and the base
	// derivation by subtraction holds (captura-wyd-affect-divina.md §E).
	HpAddPct, MpAddPct int32

	// RunSpeedBonus is the summed EF_RUNSPEED from equipped gear (boots), cached by
	// refreshScore and applied at read time by handler.attackRunOf to the move-speed
	// (low) nibble of AttackRun.
	RunSpeedBonus int32

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
	Rsv               uint8
	AffDamage         int32
	AffAC             int32
	AffMaxHP          int32
	AffMaxMP          int32
	AffStr            int16
	AffInt            int16
	AffDex            int16
	AffCon            int16
	AffRunSpeed       int32
	AffAttackSpeed    int32
	AffCritical       int16
	AffSpecial        [4]int16
	AffResist         [4]int16
	AffForceDamage    int32 // ForceDamage, e.g. Ligacao Espectral
	AffForceMobDamage int32 // ForceMobDamage, e.g. Toxina da Serpente
	AffExpBonus       int32 // from affect type 39 (Baú de XP)
	// AffHpAbs is the on-hit lifesteal percent from the Jóia da Absorção (affect
	// type 8, bit 3): the attacker recovers AffHpAbs% of the damage dealt, 50%
	// proc, capped 350/hit (_MSG_Attack.cpp:1651). AffMagic is the +20% Magic
	// buff from the Jóia do Poder (affect 8, bit 5), read into effective Magic.
	AffHpAbs int32
	AffMagic int32
	// AffDamageMultiPct is the legacy DAMAGEMULTI percentage (100 = neutral): a
	// READ-time damage multiplier applied over Damage+AffDamage but before
	// WeaponDamage, exactly where Basedef.cpp:4654 multiplies CurrentScore.Damage.
	// Additive deltas can't express it — attributeDamageBonus lands on the flat
	// Damage after the affect pass, and the multiplier must cover it too.
	AffDamageMultiPct int32
	EquipExpBonus     int32 // from fairy slot + grade/gem gear (CMob.cpp:711-870)

	EquipVisual [16]uint16 // visual item codes for MSG_CreateMob/UpdateEquip
	EquipAnct   [16]uint8  // refine/ancient glow overlay bytes paired with EquipVisual

	// Party state (lote2-party-guilda-guerra.md). Leader is the leader's conn
	// (0 = solo); LastReqParty is who last invited this entity (anti-forge gate).
	Leader       int
	LastReqParty int

	// Summoner is the conn of the player that evoked this mob (GenerateSummon's
	// pMob.Summoner, Server.cpp:3244); 0 = not a summon. A summon's Leader is
	// its owner's party leader (or the owner), matching the legacy binding.
	Summoner  int
	PartyList [MaxParty]int

	Equip [MaxEquip]Item // equipped items
	Carry [MaxCarry]Item // inventory; for mobs this is also the loot table (§2.2)
}

// IsPlayer reports whether an entity index belongs to a player (domain-model.md
// §1: id < MaxUser ⇒ player).
func IsPlayer(id int) bool { return id >= 0 && id < MaxUser }
