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

	NoticeNoEmptySlot // _NN_NoEmptySlot

	NoticeNoKey // _NN_No_Key: a locked gate needs a key the player doesn't hold

	// Duel (_MSG_ReqRanking, issue #118). _SS_S_S_Draw/_SS_S_WinBy_S carry the two
	// player names in the original (Server.cpp:8940-8970); the placeholder wire
	// format here can't interpolate names until captured, so the client-side text
	// is UNVERIFIED — only the code + recipient identify the outcome.
	NoticeDuelBattleInProgress // _NN_Battle_In_Progress
	NoticeDuelWin              // you won the duel
	NoticeDuelLose             // you lost the duel
	NoticeDuelDraw             // the duel ended in a draw (timeout)

	// NoticeCantUseHere (_NN_Cant_Use_That_Here) is sent for consumables whose
	// real _MSG_UseItem.cpp behavior depends on data absent from this repo, and
	// for Gema Estelar / Portal Scroll zone gates (issues #135 and #140).
	NoticeCantUseHere

	// Gema Estelar / Portal Scroll (_MSG_UseItem.cpp Vol 12/13, issue #140).
	NoticeSetWarp // _NN_Set_Warp (186): warp save-point recorded

	NoticeReinoCapeRequired // /reino (issue #127): cape must be empty or Capa Branca do Monstro (#550)

	// Personal shop / autotrade (issue #115, _MSG_SendAutoTrade.cpp / _MSG_ReqBuy.cpp).
	NoticeOnlyVillage    // _NN_OnlyVillage: a shop can only open inside a village
	NoticeNotEnoughMoney // _NN_Not_Enough_Money: buyer can't afford the item
	NoticeCantGetMore2G  // _NN_Cant_get_more_than_2G: seller would exceed the 2G cap
	NoticeNoSpaceToTrade // _NN_You_Have_No_Space_To_Trade: buyer inventory full
	NoticeItemSold       // _NN_ItemSold: an item left the seller's shop
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
