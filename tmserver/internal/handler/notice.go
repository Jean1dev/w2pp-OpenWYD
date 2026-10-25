package handler

import (
	"encoding/binary"
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

	// Duel codes (issue #118). Obsolete since issue #424: the duel now sends the
	// legacy Language.txt texts (battleInProgress, duelWinBy, duelDraw) and has no
	// city restriction, so nothing sends these. Kept so the iota codes below them
	// — which the client and the handler tests match on — do not shift.
	NoticeDuelBattleInProgress
	NoticeDuelWin
	NoticeDuelLose
	NoticeDuelDraw
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
)

// notify sends a client notification.
//
// UNVERIFIED: the exact wire format of the _NN_*/_DN_* notifications is not yet
// captured. As a placeholder we send MSG_MessageBoxOk carrying the 4-byte notice
// code; the real format (notification id / Language.txt string) is pinned once a
// capture exists (parity-tests.md §5). Handler tests assert the Type + code, not
// the final byte layout.
func (d *Dispatcher) notify(w *world.World, s *world.Session, n Notice) {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], uint32(n))
	w.Send(s, protocol.MsgMessageBoxOk, b[:])
}

// sendClientMessage mirrors the legacy SendClientMessage (SendFunc.cpp:27-43): a
// MSG_MessagePanel with HEADER.ID 0. The 7662 client only handles a message
// panel whose ID is 0 (TMScene.cpp:1371); w.Send stamps the session's own conn,
// which the client drops.
func (d *Dispatcher) sendClientMessage(w *world.World, s *world.Session, text string) {
	w.SendTo(s, protocol.Header{Type: protocol.MsgMessagePanel}, protocol.EncodeMessagePanelBody(text))
}

// Legacy refusal panels (Release/TMsrv/run/Language.txt), in the table's
// Windows-1252 bytes like the rest of the legacy string table.
const (
	cantDepositThatMuch  = "Esta quantia n\xe3o pode ser armazenada."      // _NN_Cant_Deposit_That_Much (44)
	cantWithdrawThatMuch = "Esta quantia n\xe3o pode ser retirada."        // _NN_Cant_Withdraw_That_Much (45)
	notEnoughMoney       = "N\xe3o possui gold suficiente."                // _NN_Not_Enough_Money (113)
	onlyMerchantSells    = "Pode comprar itens apenas do mercador."        // _NN_Only_Merchant_Sells (112)
	onlyMerchantBuyYours = "Poss\xedvel vender somente para comerciantes." // _NN_Only_Merchant_Buy_Yours (116)

	cantWithArmor = "No momento voc\xea n\xe3o pode equipar arma e armadura." // _NN_Cant_with_armor (205)

	newbieZone = "Somente n\xedvel 35 ou inferior pode entrar no campo de treinamento." // _NN_Newbie_zone (46)
)

// PvP and duel texts (Language.txt, Windows-1252), sent through
// sendClientMessage like the legacy SendClientMessage/SendNoticeArea calls.
const (
	inArenaNoExpLoss = "Na arena n\xe3o h\xe1 perda de EXP."                       // _NN_In_Arena_No_Exp_Loss (143)
	denyWhisper      = "N\xe3o \xe9 poss\xedvel sussurrar para este jogador."      // _NN_Deny_Whisper (165)
	battleInProgress = "H\xe1 uma partida em progresso."                           // _NN_Battle_In_Progress (193)
	battleStarted    = "Batalha entre %s e %s foi iniciada."                       // _NN_Battle_Started (194)
	duelWinBy        = "%s \xe9 o vencedor, %s \xe9 o perdedor desta partida."     // _SS_S_WinBy_S (196)
	duelDraw         = "Este jogo com %s e %s finalizou em empate."                // _SS_S_S_Draw (197)
	pkPointMinus     = "Seu ponto caos \xe9 %d (Menos_%d)."                        // _DD_PKPointMinus (208)
	cantKillUser     = "Voc\xea n\xe3o pode mais derrotar jogadores. (Atual : %d)" // _DN_CantKillUser (209)
	cantTradePKMode  = "A troca n\xe3o fica dispon\xedvel com o modo PK ativado."  // _NN_Cant_trade_pkmode (331)
)

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
