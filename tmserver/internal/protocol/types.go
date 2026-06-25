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
	MsgAccountLogin        Type = 0x020D // 525  MSG_AccountLogin
	MsgCharacterLogin      Type = 0x0213 // 531  MSG_CharacterLogin
	MsgCharacterLogout     Type = 0x0215 // 533
	MsgDeleteCharacter     Type = 0x0211 // 529
	MsgCreateCharacter     Type = 0x020F // 527
	MsgAccountSecure       Type = 0x0FDE // 4062 PIN token
	MsgMessageChat         Type = 0x0333 // 819
	MsgMessageWhisper      Type = 0x0334 // 820
	MsgAction              Type = 0x036C // 876  movement (also 0x0366/0x0368)
	MsgAction2             Type = 0x0366 // 870
	MsgAction3             Type = 0x0368 // 872
	MsgMotion              Type = 0x036A // 874
	MsgNoViewMob           Type = 0x0369 // 873  re-sync one entity's visibility
	MsgChangeCity          Type = 0x0291 // 657  set spawn city
	MsgReqTeleport         Type = 0x0290 // 656  paid teleport
	MsgAttack              Type = 0x0367 // 871  (also AttackOne 0x039D / AttackTwo 0x039E)
	MsgAttackOne           Type = 0x039D // 925
	MsgAttackTwo           Type = 0x039E // 926
	MsgDropItem            Type = 0x0272 // 626
	MsgGetItem             Type = 0x0270 // 624
	MsgUseItem             Type = 0x0373 // 883
	MsgTradingItem         Type = 0x0376 // 886
	MsgTrade               Type = 0x0383 // 899
	MsgQuitTrade           Type = 0x0384 // 900
	MsgCombineItem         Type = 0x03A6 // 934  refino base (Anct)
	MsgCombineItemEhre     Type = 0x02D3 // 723
	MsgCombineItemTiny     Type = 0x03C0 // 960
	MsgCombineItemShany    Type = 0x02C4 // 708
	MsgCombineItemAilyn    Type = 0x03B5 // 949
	MsgCombineItemAgatha   Type = 0x03BA // 954
	MsgCombineItemOdin     Type = 0x02D2 // 722
	MsgCombineItemLindy    Type = 0x02C3 // 707
	MsgCombineItemAlquimia Type = 0x02E1 // 737
	MsgCombineItemExtracao Type = 0x02D4 // 724  (MSG_STANDARDPARM2)
	MsgApplyBonus          Type = 0x0277 // 631  distribute attribute points
	MsgQuest               Type = 0x028B // 651  quest NPC (MSG_STANDARDPARM2)
	MsgReqRanking          Type = 0x039F // 927  duel / PvP ranking (MSG_STANDARDPARM2)
	MsgCapsuleInfo         Type = 0x02CD // 717  capsule/cash info (relay to DB)
	MsgPutoutSeal          Type = 0x03CC // 972  seal
	MsgRestart             Type = 0x0289 // 649
	MsgRemoveParty         Type = 0x037E // 894  leave/kick (MSG_STANDARDPARM)
	MsgSendReqParty        Type = 0x037F // 895  invite to party
	MsgAcceptParty         Type = 0x03AB // 939  accept invite
	MsgInviteGuild         Type = 0x03D5 // 981  invite to guild (MSG_STANDARDPARM2)
	MsgGuildAlly           Type = 0x0E12 // 3602 guild alliance
	MsgWar                 Type = 0x0E0E // 3598 declare guild war (MSG_STANDARDPARM2)
	MsgChallange           Type = 0x028E // 654  zone challenge / tax (MSG_STANDARDPARM)
	MsgChallangeConfirm    Type = 0x028F // 655  confirm challenge (MSG_STANDARDPARM2)
	MsgPing                Type = 0x03A0 // 928  keepalive — no-op on receive (§2)
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
	MsgCNFCharacterLogout Type = 0x0116 // 278
	MsgCharacterLoginFail Type = 0x0119 // 281
	MsgNewCharacterFail   Type = 0x011A // 282
	MsgAlreadyPlaying     Type = 0x011C // 284
	MsgCreateMob          Type = 0x0364 // 868  spawn in view
	MsgRemoveMob          Type = 0x0165 // 357  despawn
	MsgSendItem           Type = 0x0182 // 386  update one slot
	MsgUpdateEquip        Type = 0x006B // 107  S→C refresh visible equipment (_MSG_UpdateEquip)
	MsgREQShopList        Type = 0x027B // 635  C→S open NPC shop (Target=NPC)
	MsgShopList           Type = 0x017C // 380  S→C shop list (NPC Carry)
	MsgBuy                Type = 0x0379 // 889  C↔S buy from NPC
	MsgSell               Type = 0x037A // 890  C↔S sell to NPC
	MsgDeposit            Type = 0x0388 // 904  C↔S deposit gold into cargo (MSG_STANDARDPARM)
	MsgWithdraw           Type = 0x0387 // 903  C↔S withdraw gold from cargo (MSG_STANDARDPARM)
	MsgUpdateCargoCoin    Type = 0x0339 // 825  S→C account cargo gold (_MSG_UpdateCargoCoin)
	MsgUpdateEtc          Type = 0x0337 // 823  S→C update gold/exp/etc (Coin@40)
	MsgCNFGetItem         Type = 0x0171 // 369
	MsgCNFDropItem        Type = 0x0175 // 373
	MsgDecayItem          Type = 0x016F // 367  ground item gone
	MsgCombineComplete    Type = 0x03A7 // 935  combine result (parm 0/1/2)
	MsgUpdateScore        Type = 0x0336 // 822  attributes/score update
	MsgSetHpMp            Type = 0x0181 // 385
)
