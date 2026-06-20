package protocol

// Type is the 2-byte HEADER.Type field: a message base OR'd with direction
// flags (protocol-spec.md §2, Basedef.h:1212-1221). The dispatcher compares the
// full wire value (base + flags) directly, so a constant with multiple flags
// has a single wire value that matches in any context.
type Type uint16

// Direction flag bits (protocol-spec.md §2, Basedef.h:1212-1221).
const (
	FlagGame2Client Type = 0x0100 // TMSrv → client
	FlagClient2Game Type = 0x0200 // client → TMSrv
	FlagDB2Game     Type = 0x0400 // DBSrv → TMSrv
	FlagGame2DB     Type = 0x0800 // TMSrv → DBSrv
	FlagDB2NP       Type = 0x1000 // DBSrv → NPServer
	FlagNP2DB       Type = 0x2000 // NPServer → DBSrv
	FlagNew         Type = 0x4000 // "new" message family (ranking grind/exp)

	// flagMask covers all direction bits; the remaining low bits are the base.
	flagMask Type = 0xFF00
)

// Base returns the message base (the wire value with direction flags stripped).
func (t Type) Base() Type { return t &^ flagMask }

// HasFlag reports whether t carries the given direction flag.
func (t Type) HasFlag(f Type) bool { return t&f == f }

// Client → TMSrv message types the server must handle (protocol-spec.md §3.1).
// These are the only ones with a case in the original dispatcher
// (ProcessClientMessage.cpp); the full 198-Type dump lives in protocol-spec.md
// Appendix B and is not all named here yet.
const (
	MsgAccountLogin    Type = 0x020D // 525  MSG_AccountLogin
	MsgCharacterLogin  Type = 0x0213 // 531  MSG_CharacterLogin
	MsgCharacterLogout Type = 0x0215 // 533
	MsgDeleteCharacter Type = 0x0211 // 529
	MsgCreateCharacter Type = 0x020F // 527
	MsgAccountSecure   Type = 0x0FDE // 4062 PIN token
	MsgMessageChat     Type = 0x0333 // 819
	MsgMessageWhisper  Type = 0x0334 // 820
	MsgAction          Type = 0x036C // 876  movement (also 0x0366/0x0368)
	MsgAction2         Type = 0x0366 // 870
	MsgAction3         Type = 0x0368 // 872
	MsgMotion          Type = 0x036A // 874
	MsgAttack          Type = 0x0367 // 871  (also AttackOne 0x039D / AttackTwo 0x039E)
	MsgAttackOne       Type = 0x039D // 925
	MsgAttackTwo       Type = 0x039E // 926
	MsgDropItem        Type = 0x0272 // 626
	MsgGetItem         Type = 0x0270 // 624
	MsgUseItem         Type = 0x0373 // 883
	MsgTradingItem     Type = 0x0376 // 886
	MsgTrade           Type = 0x0383 // 899
	MsgQuitTrade       Type = 0x0384 // 900
	MsgCombineItem     Type = 0x03A6 // 934  refino base
	MsgApplyBonus      Type = 0x0277 // 631
	MsgRestart         Type = 0x0289 // 649
	MsgPing            Type = 0x03A0 // 928  keepalive — no-op on receive (§2)
)

// TMSrv → client message types the server must produce (protocol-spec.md §3.2,
// actionable subset).
const (
	MsgMessagePanel       Type = 0x0101 // 257
	MsgMessageBoxOk       Type = 0x0102 // 258
	MsgCNFAccountLogin    Type = 0x010A // 266  → character selection
	MsgCNFNewCharacter    Type = 0x0110 // 272
	MsgCNFDeleteCharacter Type = 0x0112 // 274
	MsgCNFCharacterLogin  Type = 0x0114 // 276  enter world (char snapshot)
	MsgCharacterLoginFail Type = 0x0119 // 281
	MsgAlreadyPlaying     Type = 0x011C // 284
	MsgCreateMob          Type = 0x0364 // 868  spawn in view
	MsgRemoveMob          Type = 0x0165 // 357  despawn
	MsgSendItem           Type = 0x0182 // 386  update one slot
	MsgCNFGetItem         Type = 0x0171 // 369
	MsgCNFDropItem        Type = 0x0175 // 373
	MsgSetHpMp            Type = 0x0181 // 385
)
