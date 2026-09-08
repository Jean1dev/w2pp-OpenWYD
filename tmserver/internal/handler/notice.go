package handler

import (
	"encoding/binary"
	"regexp"
	"strings"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// Notice identifies a client-facing notification (the legacy _NN_*/_DN_* codes).
type Notice uint32

// Notification codes used by batch 1 (handlers/*.md). The numeric values are
// local identifiers, NOT the original notification numbers.
const (
	NoticeVersionMismatch     Notice = iota // _NN_Version_Not_Match_Rerun
	NoticeLoginNow                          // "Login now, wait a moment."
	Notice3WrongPass                        // _NN_3_Tims_Wrong_Pass
	NoticeBadPass                           // _MSG_DBAccountLoginFail_Pass
	NoticeNoAccount                         // _MSG_DBAccountLoginFail_Account
	NoticeBlocked                           // _MSG_DBAccountLoginFail_Block
	NoticeSelectCharacter                   // _NN_SelectCharacter
	NoticeDeletingWait                      // "Deleting Character. wait a moment."
	NoticeDBError                           // DB call failed
	NoticeCantDropHere                      // _NN_Cant_Drop_Here
	NoticeCantAutoTrade                     // _NN_CantWhenAutoTrade
	NoticeNotConnected                      // whisper target offline
	NoticeDenyWhisper                       // whisper target blocked whispers
	NoticeBillingDenied                     // binServer denied entry (expired/blocked)
	NoticeCargoFull                         // deposit/withdraw would exceed the 2G ceiling
	NoticeReqNotMet                         // equip requirement (level/attributes) not met
	NoticeCantEatMore                       // _NN_CantEatMore (a buff of this kind is already active)
	NoticeOtherClassSkill                   // _NN_Cant_Learn_Other_Class_Skill
	NoticeNotEnoughSkillPoint               // _NN_Not_Enough_Skill_Point
	NoticeOnlyOneEighthSkill                // _NN_Only_OneSkillLearn (8th skill is exclusive)
	NoticeLearnPrereq                       // _NN_Befor_LearnSkill (learn the tree's 7 first)
	NoticeAlreadyLearned                    // _NN_Already_Learned_It
	NoticeNotEnoughCoin                     // _DN_D_Cost (8th skill costs 50M gold)
	NoticeMaxPoint                          // _NN_Maximum_Point_Now / _200_Now (mastery cap)

	// Dust refine (_MSG_UseItem.cpp dust path; Language.h ids in the comments).
	NoticeOnlyToEquips   // _NN_Only_To_Equips (74): a dust only refines equipment
	NoticeCantRefineMore // _NN_Cant_Refine_More (75): at the cap, or EF_NOSANC
	NoticeFailToRefine   // _NN_Fail_To_Refine (76)
	NoticeRefineSuccess  // _NN_Refine_Success (176)
	NoticeIncubated      // _NN_INCUBATED (253): the egg hatched into its mount
	NoticeIncuWaitMore   // _NN_Incu_Wait_More (261): the egg's incubation timer
	NoticeMountLevel     // _NN_Mount_Level (263): a cria gained one mount level

	NoticeNoEmptySlot // _NN_NoEmptySlot
	NoticeMaxBag      // _NN_MAX_BAG: both Bolsa do Andarilho slots are already active

	NoticeNoKey // _NN_No_Key: a locked gate needs a key the player doesn't hold

	// Duel (_MSG_ReqRanking, issue #118). _SS_S_S_Draw/_SS_S_WinBy_S carry the two
	// player names in the original (Server.cpp:8940-8970); the placeholder wire
	// format here can't interpolate names until captured, so the client-side text
	// is UNVERIFIED — only the code + recipient identify the outcome.
	NoticeDuelBattleInProgress // _NN_Battle_In_Progress
	NoticeDuelWin              // you won the duel
	NoticeDuelLose             // you lost the duel
	NoticeDuelDraw             // the duel ended in a draw (timeout)

	// NoticeDuelInCity: neither this code nor the underlying rule is
	// confirmed against a legacy capture — the issue #118 scope explicitly
	// leaves "duelo só fora de cidade?" as an open question. Blocking duel
	// request/accept while either side stands in a safe city is a best-effort
	// guess pending real verification.
	NoticeDuelInCity

	// NoticeCantUseHere (_NN_Cant_Use_That_Here) is sent for consumables whose
	// real _MSG_UseItem.cpp behavior depends on data absent from this repo, and
	// for Gema Estelar / Portal Scroll zone gates (issues #135 and #140).
	NoticeCantUseHere

	// Gema Estelar / Portal Scroll (_MSG_UseItem.cpp Vol 12/13, issue #140).
	NoticeSetWarp // _NN_Set_Warp (186): warp save-point recorded

	// NoticeReinoCapeRequired was the /reino cape gate (issue #127). Obsolete since
	// issue #208: /reino now routes every cape to a destination instead of refusing
	// kingdom capes, so nothing sends this code. Kept so the iota codes below it —
	// which the client and the handler tests match on — do not shift.
	NoticeReinoCapeRequired

	// Personal shop / autotrade (issue #115, _MSG_SendAutoTrade.cpp / _MSG_ReqBuy.cpp).
	NoticeOnlyVillage    // _NN_OnlyVillage: a shop can only open inside a village
	NoticeNotEnoughMoney // _NN_Not_Enough_Money: buyer can't afford the item
	NoticeCantGetMore2G  // _NN_Cant_get_more_than_2G: seller would exceed the 2G cap
	NoticeNoSpaceToTrade // _NN_You_Have_No_Space_To_Trade: buyer inventory full
	NoticeItemSold       // _NN_ItemSold: an item left the seller's shop

	// NoticeAlreadyDone (_NN_Youve_Done_It_Already) — a one-shot quest hand-in
	// (e.g. CAPAVERDE_TRADE) was already completed.
	NoticeAlreadyDone

	// Pergaminho da Água gates (_MSG_UseItem.cpp:1755/1762/1771).
	//
	// NoticeSomeoneOnQuest is _NN_Someone_is_on_quest (Language.txt:295,
	// "Outros jogadores já estão realizando esta quest."). The legacy passes the
	// occupant's name and a count to sprintf, but the string carries no format
	// verbs, so both are discarded and every player sees the generic text — we
	// reproduce that and log the name instead.
	NoticeSomeoneOnQuest
	// NoticePartyLeaderOnly is _NN_Party_Leader_Only (Language.txt:229, "Uso
	// restrito ao líder do grupo.").
	NoticePartyLeaderOnly

	// NoticeWaterClassNotAllowed refuses a water scroll whose chain is scoped to
	// another progression tier. It has NO legacy counterpart — the original lets
	// any class open any room — so there is no _NN_ id to match; it exists for
	// the server rule documented on waterClassAllowed.
	NoticeWaterClassNotAllowed

	// Quarter-of-a-level progress (_NN_1/2/3_Quarters_Bonus, Language.txt:54-56
	// "1/4 BONUS" … "3/4 BONUS"). CheckGetLevel splits every level into four and
	// the attack handler announces each crossing (_MSG_Attack.cpp:1772-1781),
	// which is the feedback a player gets between one level and the next.
	Notice1QuarterBonus
	Notice2QuartersBonus
	Notice3QuartersBonus

	// Feijão Mágico / Removedor de tintura (item.go useMagicBean).
	//
	// DELIBERATE DIVERGENCE: these have NO legacy counterpart. The original
	// reuses the REFINE strings on the paint path — _NN_Refine_Success when a
	// colour lands and _NN_Cant_Refine_More when it cannot
	// (_MSG_UseItem.cpp:3767-3861) — so a player who painted a helmet was told
	// "Obteve sucesso na refinação". That is the legacy being sloppy, not a port
	// bug; these four say what actually happened, and painting and REMOVING are
	// kept apart because telling someone their item was "pintado" while they
	// stripped the colour off it is the same mistake with a new coat.
	NoticePaintSuccess
	NoticePaintRemoved
	NoticeCantPaint
	NoticeNotPainted

	// Pesadelo entry refusals (_MSG_UseItem.cpp:2548/2644/2748, pesadelo.go).
	//
	// NoticePesadeloLimited is the one with a real id: _NN_Night_Limited, the
	// maxNightmare run cap. The other three are literal strings the legacy passes
	// straight to SendClientMessage instead of going through the string table
	// ("Entrada permitida somente a Mortais/Archs/Celestiais", "Horário não
	// permitido.", "Entrada não permitida. Cheque sua quantidade de entradas com
	// o comando /nt"), so they have no _NN_ id to match. They are split into
	// three codes rather than folded into one because a player refused at the
	// door needs to know WHICH gate said no — wrong tier, wrong time, or out of
	// entries are three different fixes.
	NoticePesadeloClassNotAllowed
	// NoticePesadeloLevelTooHigh: the class may enter this tier, but the
	// character has outgrown it (a Celestial past 40 in Místico, past 150 in
	// Arcano). Separate from ClassNotAllowed because the fix is different — the
	// player moves up the ladder rather than being in the wrong dungeon. No
	// legacy id: the level caps are a server rule (pesadelo.go).
	NoticePesadeloLevelTooHigh
	NoticePesadeloClosed
	NoticePesadeloNoEntries
	NoticePesadeloLimited

	// NoticeLevelLimit is _NN_Level_limit (Language.txt:298), what the legacy
	// answers when a level-banded item is used outside its band — the Quest 256
	// trophies above all (_MSG_UseItem.cpp Vol 191). It is NOT NoticeReqNotMet:
	// that one stays silent on purpose, because the equip path it was named for is
	// silent in the legacy too, and reusing it here made a level-gated item look
	// like a dead click.
	//
	// APPENDED AT THE END on purpose: the codes above are iota-ordered and other
	// branches are editing this block, so inserting anywhere else would silently
	// renumber their notices.
	NoticeLevelLimit

	// Mount growth, the Âmago path (_MSG_UseItem.cpp:1591/1602/1664). Appended at
	// the end for the same reason NoticeLevelLimit is.
	NoticeMountNotMatch
	NoticeCantUpgradeMore
	NoticeMountGrowth

	// Party invite refusals (_MSG_SendReqParty.cpp / _MSG_AcceptParty.cpp).
	// Every one of these is a SendClientMessage in the legacy and was a bare
	// `return` here, which is why inviting somebody looked like a dead button:
	// the click reached the server, the server decided no, and nothing on the
	// screen said so. Appended at the end for the reason NoticeLevelLimit gives.
	NoticePartyDropCurrentFirst
	NoticePartyNotConnected
	NoticePartyOtherMember
	NoticePartyHasOwn
	NoticePartyLevelLimit

	// NoticeCantMoveItem is _NN_Cant_MoveItem (Language.txt:379), the refusal for an
	// EF_NOTRADE item offered in a trade or put in a personal shop
	// (_MSG_Trade.cpp:182, _MSG_SendAutoTrade.cpp:87). Appended at the end for the
	// reason NoticeLevelLimit gives.
	NoticeCantMoveItem

	// NoticeOnlyByWaterScroll is _NN_Only_By_Water_Scroll (Language.txt:228): the
	// Elemental Zone refuses a teleport tile, it is entered with the scroll
	// (_MSG_ReqTeleport.cpp:27). Appended at the end for the reason NoticeLevelLimit
	// gives.
	NoticeOnlyByWaterScroll
)

// noticeKey maps a Notice to its key in the shipped client string table
// (Release/TMsrv/run/Language.txt; the _NN_* names are Source/Code/TMSrv/Language.h).
// This map is what makes a notice visible at all: notify resolves the key against
// the loaded table and sends the line through MSG_MessagePanel, which is what the
// legacy SendClientMessage does. The numeric ids in the comments are the file's,
// kept for grepping against Language.h — the lookup is by name, because a wrong
// name yields nothing (and fails the loader test) while a wrong number yields a
// plausible but wrong sentence.
//
// A Notice missing from this map is deliberate, and there are three reasons for
// it: the rule is this port's own and has no legacy string (the Pesadelo level
// cap, the water-scroll class scope); the legacy refuses in silence and parity
// means staying silent (equip requirements — nothing in Source/Code/TMSrv sends
// a message on that path); or the shipped line takes sprintf arguments that this
// wire format cannot carry, which noticeLine also rejects at runtime.
var noticeKey = map[Notice]string{
	// Login / character select.
	NoticeVersionMismatch: "_NN_Version_Not_Match_Rerun",   // 36
	NoticeLoginNow:        "_NN_WAITFORLOGIN",              // 470
	Notice3WrongPass:      "_NN_3_Tims_Wrong_Pass",         // 37
	NoticeBadPass:         "_NN_Wrong_Password",            // 133
	NoticeNoAccount:       "_NN_No_Account_With_That_Name", // 131
	NoticeBlocked:         "_NN_ISNOTALLOWEDACCOUNT",       // 472
	NoticeSelectCharacter: "_NN_SelectCharacter",           // 34
	Notice1QuarterBonus:   "_NN_1_Quarters_Bonus",          // 56 "1/4 BONUS"
	Notice2QuartersBonus:  "_NN_2_Quarters_Bonus",          // 55 "2/4 BONUS"
	Notice3QuartersBonus:  "_NN_3_Quarters_Bonus",          // 54 "3/4 BONUS"
	NoticeDeletingWait:    "_NN_WAITFORDELCHAR",            // 469

	// Items / inventory.
	NoticeCantDropHere:  "_NN_Cant_Drop_Here",        // 72
	NoticeCantAutoTrade: "_NN_CantWhenAutoTrade",     // 170
	NoticeCargoFull:     "_NN_Cant_get_more_than_2G", // 273
	NoticeCantEatMore:   "_NN_CantEatMore",           // 345
	NoticeNoEmptySlot:   "_NN_NoEmptySlot",           // 307
	NoticeMaxBag:        "_NN_MAX_BAG",               // 531
	NoticeNoKey:         "_NN_No_Key",                // 79
	NoticeCantUseHere:   "_NN_Cant_Use_That_Here",    // 96
	NoticeSetWarp:       "_NN_Set_Warp",              // 186
	NoticeAlreadyDone:   "_NN_Youve_Done_It_Already", // 71

	// Chat / whisper.
	NoticeNotConnected: "_NN_Not_Connected", // 91
	NoticeDenyWhisper:  "_NN_Deny_Whisper",  // 165

	// Skills.
	NoticeOtherClassSkill:     "_NN_Cant_Learn_Other_Class_Skill", // 107
	NoticeNotEnoughSkillPoint: "_NN_Not_Enough_Skill_Point",       // 108
	NoticeOnlyOneEighthSkill:  "_NN_Only_OneSkillLearn",           // 402
	NoticeLearnPrereq:         "_NN_Befor_LearnSkill",             // 403
	NoticeAlreadyLearned:      "_NN_Already_Learned_It",           // 109
	NoticeMaxPoint:            "_NN_Maximum_Point_Now",            // 105

	// Dust refine and the mount eggs.
	NoticeOnlyToEquips:   "_NN_Only_To_Equips",   // 74
	NoticeCantRefineMore: "_NN_Cant_Refine_More", // 75
	NoticeFailToRefine:   "_NN_Fail_To_Refine",   // 76
	NoticeRefineSuccess:  "_NN_Refine_Success",   // 176
	NoticeIncubated:      "_NN_INCUBATED",        // 253
	NoticeIncuWaitMore:   "_NN_Incu_Wait_More",   // 261
	NoticeMountLevel:     "_NN_Mount_Level",      // 263

	// Duel. Only the "a match is already running" line is usable: the win/lose/
	// draw strings (_SS_S_WinBy_S, _SS_S_S_Draw) interpolate both player names,
	// which the placeholder wire format cannot carry.
	NoticeDuelBattleInProgress: "_NN_Battle_In_Progress", // 193

	// Personal shop / autotrade.
	NoticeOnlyVillage:    "_NN_OnlyVillage",                // 172
	NoticeNotEnoughMoney: "_NN_Not_Enough_Money",           // 113
	NoticeCantGetMore2G:  "_NN_Cant_get_more_than_2G",      // 273
	NoticeNoSpaceToTrade: "_NN_You_Have_No_Space_To_Trade", // 86
	NoticeItemSold:       "_NN_ItemSold",                   // 171

	// Pergaminho da Água.
	NoticeSomeoneOnQuest:  "_NN_Someone_is_on_quest", // 295
	NoticePartyLeaderOnly: "_NN_Party_Leader_Only",   // 229

	// Grupo. The legacy answers every one of these with SendClientMessage, so a
	// refused invite tells the player why instead of doing nothing.
	NoticePartyDropCurrentFirst: "_NN_Dropped_Current_Party_First", // 121
	NoticePartyNotConnected:     "_NN_Not_Connected",               // 91
	NoticePartyOtherMember:      "_NN_Other_Partys_Member",         // 119
	NoticePartyHasOwn:           "_NN_Have_Own_Party_Already",      // 120
	NoticePartyLevelLimit:       "_NN_Party_Level_Limit",           // 215

	// Pesadelo. Only these two are in the string table; the other refusals are
	// literals the legacy passes straight to SendClientMessage (noticeText).
	NoticePesadeloClosed:  "_NN_CANT_USE_NIGHTMARE", // 418
	NoticePesadeloLimited: "_NN_Night_Limited",      // 349

	NoticeLevelLimit: "_NN_Level_limit", // 298

	NoticeMountNotMatch:   "_NN_Mount_Not_Match",   // 256
	NoticeCantUpgradeMore: "_NN_Cant_Upgrade_More", // 254
	NoticeMountGrowth:     "_NN_Mount_Growth",      // 255

	NoticeCantMoveItem:      "_NN_Cant_MoveItem",        // 379
	NoticeOnlyByWaterScroll: "_NN_Only_By_Water_Scroll", // 228
}

// noticeText is the compiled fallback for notices with no Language.txt line: the
// literals the legacy passes straight to SendClientMessage, and the few rules
// this port added. It also keeps those notices working when the content tree is
// not mounted.
//
// Text is written with its real accents; protocol.ClientText re-encodes to
// CP1252 on the way out.
var noticeText = map[Notice]string{
	NoticeOnlyToEquips:   "Possível somente com armas e armaduras equipadas.", // 74
	NoticeCantRefineMore: "Este item não pode ser mais refinado.",             // 75
	NoticeFailToRefine:   "Refinação falhou.",                                 // 76
	NoticeRefineSuccess:  "Obteve sucesso na refinação.",                      // 176

	// Paint. No Language.txt id: see the DELIBERATE DIVERGENCE note above.
	NoticePaintSuccess: "Item pintado com sucesso.",
	NoticePaintRemoved: "Pintura removida com sucesso.",
	NoticeCantPaint:    "Este item não pode receber mais pintura.",
	NoticeNotPainted:   "Este item não está pintado.",

	// Pesadelo literals (_MSG_UseItem.cpp:2548/2644/2748), copied verbatim from
	// the legacy calls: these never went through the string table.
	NoticePesadeloNoEntries: "Entrada não permitida. Cheque sua quantidade de entradas com o comando /nt",

	// These two were sent as local literals beside the notice before the string
	// table existed. The literal is gone (it would now arrive twice), so they keep
	// a fallback here: without it, a server booted with no -content would go quiet
	// on two messages that used to work regardless of the content mount.
	NoticeNotConnected: "O jogador não está conectado.", // 91
	NoticeAlreadyDone:  "Você já completou esta Quest.", // 71

	NoticeLevelLimit: "Nível Insuficiente. Isto não pode ser utilizado.", // 298

	// Without this fallback a server booted with no -content would refuse the trade
	// and say nothing — which is the very failure this notice exists to end.
	NoticeCantMoveItem: "Este item não pode ser movimentado.", // 379
}

// formatVerb matches a printf conversion, so a shipped line that interpolates
// arguments is never sent raw — a player told "Você precisa de %d Gold" is worse
// off than one told nothing. Width/flags/length modifiers are included so "%2d"
// and "%llu" are caught; "%%" and a literal "50%." are not conversions and pass.
var formatVerb = regexp.MustCompile(`%[-+ #0]*[0-9]*(\.[0-9]+)?(hh|h|ll|l|I64)?[diouxXeEfFgGaAcspn]`)

// noticeLine returns the text a notice should carry, and whether there is any.
// The string table wins over the compiled fallback: it is the text this client
// actually shipped with, including any edit the operator made to it.
func (d *Dispatcher) noticeLine(n Notice) (string, bool) {
	if key, ok := noticeKey[n]; ok {
		if text, ok := d.lang.Text(key); ok && !formatVerb.MatchString(text) {
			return text, true
		}
	}
	text, ok := noticeText[n]
	return text, ok
}

// notify sends a client notification: the box frame, then the line the player
// actually reads.
//
// UNVERIFIED: the exact wire format of the _NN_*/_DN_* notifications is not yet
// captured. As a placeholder we send MSG_MessageBoxOk carrying the 4-byte notice
// code; the real format (notification id / Language.txt string) is pinned once a
// capture exists (parity-tests.md §5). Handler tests assert the Type + code, not
// the final byte layout.
//
// That placeholder renders as NOTHING on the client, which is why a refused
// action used to look exactly like a dead handler — the box carries a code this
// client was never taught to draw. So the text goes out beside it through
// MSG_MessagePanel, the legacy SendClientMessage, which the client does render.
// A notice with no text (noticeLine) keeps the old silent behaviour.
func (d *Dispatcher) notify(w *world.World, s *world.Session, n Notice) {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], uint32(n))
	w.Send(s, protocol.MsgMessageBoxOk, b[:])
	if text, ok := d.noticeLine(n); ok {
		sendClientMessage(w, s, text)
	}
}

// cstr trims a fixed-size NUL/space-padded C char array to a Go string.
func cstr(b []byte) string {
	if i := indexByte(b, 0); i >= 0 {
		b = b[:i]
	}
	return strings.TrimRight(string(b), " ")
}

func indexByte(b []byte, c byte) int {
	for i := range b {
		if b[i] == c {
			return i
		}
	}
	return -1
}
