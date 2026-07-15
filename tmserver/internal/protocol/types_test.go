package protocol

import "testing"

// Legacy message-type direction flags (Basedef.h:1212-1221). These bits are
// part of the value the client matches on, not metadata about it: every wire
// type is a base number OR'd with its direction flags.
const (
	flagGame2Client = 0x0100
	flagClient2Game = 0x0200
	flagDB2Game     = 0x0400
	flagGame2DB     = 0x0800
)

// TestMessageTypeValues pins every opcode to the legacy constant it ports by
// re-deriving "base | flags" straight from Basedef.h.
//
// The handler and codec tests assert against these same constants, so they
// agree with a mistyped opcode by construction and cannot catch one. Only
// re-deriving the number independently does. Dropping the flag bits yields a
// type the client silently discards, with no server-side symptom: issue #101
// sent _MSG_UpdateEquip as 107 instead of 875, so the BM transform's body mesh
// never rendered while _MSG_UpdateScore (correctly typed) still delivered the
// transform's stats.
func TestMessageTypeValues(t *testing.T) {
	tests := []struct {
		legacy string // the ported constant and its Basedef.h line
		got    Type
		want   int // base | flags, as spelled in the legacy header
	}{
		{"_MSG_AccountLogin (Basedef.h:1543)", MsgAccountLogin, 13 | flagClient2Game},
		{"_MSG_CharacterLogin (Basedef.h:1673)", MsgCharacterLogin, 19 | flagClient2Game},
		{"_MSG_CharacterLogout (Basedef.h:1725)", MsgCharacterLogout, 21 | flagClient2Game},
		{"_MSG_DeleteCharacter (Basedef.h:1607)", MsgDeleteCharacter, 17 | flagClient2Game},
		{"_MSG_CreateCharacter (Basedef.h:1597)", MsgCreateCharacter, 15 | flagClient2Game},
		{"_MSG_AccountSecure (Basedef.h:1586)", MsgAccountSecure, 222 | flagDB2Game | flagGame2DB | flagClient2Game | flagGame2Client},
		{"_MSG_MessageChat (Basedef.h:1788)", MsgMessageChat, 51 | flagGame2Client | flagClient2Game},
		{"_MSG_MessageWhisper (Basedef.h:1796)", MsgMessageWhisper, 52 | flagGame2Client | flagClient2Game},
		{"_MSG_Action (Basedef.h:2067)", MsgAction, 108 | flagGame2Client | flagClient2Game},
		{"_MSG_Action2 (Basedef.h:2068)", MsgAction2, 102 | flagGame2Client | flagClient2Game},
		{"_MSG_Action3 (Basedef.h:2069)", MsgAction3, 104 | flagGame2Client | flagClient2Game},
		{"_MSG_Motion (Basedef.h:2084)", MsgMotion, 106 | flagGame2Client | flagClient2Game},
		{"_MSG_NoViewMob (Basedef.h:1944)", MsgNoViewMob, 105 | flagGame2Client | flagClient2Game},
		{"_MSG_ChangeCity (Basedef.h:2181)", MsgChangeCity, 145 | flagClient2Game},
		{"_MSG_ReqTeleport (Basedef.h:2180)", MsgReqTeleport, 144 | flagClient2Game},
		{"_MSG_Attack (Basedef.h:2399)", MsgAttack, 103 | flagGame2Client | flagClient2Game},
		{"_MSG_AttackOne (Basedef.h:2450)", MsgAttackOne, 157 | flagGame2Client | flagClient2Game},
		{"_MSG_AttackTwo (Basedef.h:2487)", MsgAttackTwo, 158 | flagGame2Client | flagClient2Game},
		{"_MSG_DropItem (Basedef.h:2002)", MsgDropItem, 114 | flagClient2Game},
		{"_MSG_GetItem (Basedef.h:1982)", MsgGetItem, 112 | flagClient2Game},
		{"_MSG_UseItem (Basedef.h:2195)", MsgUseItem, 115 | flagGame2Client | flagClient2Game},
		{"_MSG_SendAffect (Basedef.h:2574)", MsgSendAffect, 185 | flagGame2Client | flagClient2Game},
		{"_MSG_TradingItem (Basedef.h:2102)", MsgTradingItem, 118 | flagClient2Game | flagGame2Client},
		{"_MSG_Trade (Basedef.h:2434)", MsgTrade, 131 | flagGame2Client | flagClient2Game},
		{"_MSG_QuitTrade (Basedef.h:2447)", MsgQuitTrade, 132 | flagGame2Client | flagClient2Game},
		{"_MSG_CombineItem (Basedef.h:2328)", MsgCombineItem, 166 | flagGame2Client | flagClient2Game},
		{"_MSG_CombineItemEhre (Basedef.h:2339)", MsgCombineItemEhre, 211 | flagClient2Game},
		{"_MSG_CombineItemTiny (Basedef.h:2329)", MsgCombineItemTiny, 192 | flagGame2Client | flagClient2Game},
		{"_MSG_CombineItemShany (Basedef.h:2331)", MsgCombineItemShany, 196 | flagClient2Game},
		{"_MSG_CombineItemAilyn (Basedef.h:2341)", MsgCombineItemAilyn, 181 | flagGame2Client | flagClient2Game},
		{"_MSG_CombineItemAgatha (Basedef.h:2342)", MsgCombineItemAgatha, 186 | flagGame2Client | flagClient2Game},
		{"_MSG_CombineItemOdin (Basedef.h:2333)", MsgCombineItemOdin, 210 | flagClient2Game},
		{"_MSG_CombineItemLindy (Basedef.h:2330)", MsgCombineItemLindy, 195 | flagClient2Game},
		{"_MSG_CombineItemAlquimia (Basedef.h:2335)", MsgCombineItemAlquimia, 225 | flagClient2Game},
		{"_MSG_CombineItemExtracao (Basedef.h:2334)", MsgCombineItemExtracao, 212 | flagClient2Game},
		{"_MSG_ApplyBonus (Basedef.h:2143)", MsgApplyBonus, 119 | flagClient2Game},
		{"_MSG_SetShortSkill (Basedef.h:2115)", MsgSetShortSkill, 120 | flagClient2Game | flagGame2Client},
		{"_MSG_Quest (Basedef.h:2175)", MsgQuest, 139 | flagClient2Game},
		{"_MSG_MasterGriff (Basedef.h:1804)", MsgMasterGriff, 217 | flagGame2DB | flagClient2Game},
		{"_MSG_ReqRanking (Basedef.h:2523)", MsgReqRanking, 159 | flagGame2Client | flagClient2Game},
		{"_MSG_CapsuleInfo (Basedef.h:2332)", MsgCapsuleInfo, 205 | flagClient2Game},
		{"_MSG_PutoutSeal (Basedef.h:2356)", MsgPutoutSeal, 204 | flagGame2Client | flagClient2Game},
		{"_MSG_Restart (Basedef.h:2161)", MsgRestart, 137 | flagClient2Game},
		{"_MSG_RemoveParty (Basedef.h:2283)", MsgRemoveParty, 126 | flagGame2Client | flagClient2Game},
		{"_MSG_SendReqParty (Basedef.h:2263)", MsgSendReqParty, 127 | flagGame2Client | flagClient2Game},
		{"_MSG_AcceptParty (Basedef.h:2547)", MsgAcceptParty, 171 | flagGame2Client | flagClient2Game},
		{"_MSG_InviteGuild (Basedef.h:2354)", MsgInviteGuild, 213 | flagGame2Client | flagClient2Game},
		{"_MSG_GuildAlly (Basedef.h:1654)", MsgGuildAlly, 18 | flagClient2Game | flagDB2Game | flagGame2DB},
		{"_MSG_War (Basedef.h:1618)", MsgWar, 14 | flagClient2Game | flagDB2Game | flagGame2DB},
		{"_MSG_Challange (Basedef.h:2178)", MsgChallange, 142 | flagClient2Game},
		{"_MSG_ChallangeConfirm (Basedef.h:2179)", MsgChallangeConfirm, 143 | flagClient2Game},
		{"_MSG_Ping (Basedef.h:2524)", MsgPing, 160 | flagGame2Client | flagClient2Game},
		{"_MSG_MessagePanel (Basedef.h:1527)", MsgMessagePanel, 1 | flagGame2Client},
		{"_MSG_MessageBoxOk (Basedef.h:1534)", MsgMessageBoxOk, 2 | flagGame2Client},
		{"_MSG_CNFAccountLogin (Basedef.h:1544)", MsgCNFAccountLogin, 10 | flagGame2Client},
		{"_MSG_CNFNewCharacter (Basedef.h:1638)", MsgCNFNewCharacter, 16 | flagGame2Client},
		{"_MSG_CNFDeleteCharacter (Basedef.h:1646)", MsgCNFDeleteCharacter, 18 | flagGame2Client},
		{"_MSG_CNFCharacterLogin (Basedef.h:1682)", MsgCNFCharacterLogin, 20 | flagGame2Client},
		{"_MSG_CNFCharacterLogout (Basedef.h:1727)", MsgCNFCharacterLogout, 22 | flagGame2Client},
		{"_MSG_CharacterLoginFail (Basedef.h:1730)", MsgCharacterLoginFail, 25 | flagGame2Client},
		{"_MSG_NewCharacterFail (Basedef.h:1731)", MsgNewCharacterFail, 26 | flagGame2Client},
		{"_MSG_AlreadyPlaying (Basedef.h:1733)", MsgAlreadyPlaying, 28 | flagGame2Client},
		{"_MSG_CreateMob (Basedef.h:1917)", MsgCreateMob, 100 | flagGame2Client | flagClient2Game},
		{"_MSG_RemoveMob (Basedef.h:1946)", MsgRemoveMob, 101 | flagGame2Client},
		{"_MSG_PKInfo (Basedef.h:1954)", MsgPKInfo, 102 | flagGame2Client},
		{"_MSG_PKMode (Basedef.h:2164)", MsgPKMode, 153 | flagGame2Client | flagClient2Game},
		{"_MSG_SendItem (Basedef.h:2036)", MsgSendItem, 130 | flagGame2Client},
		{"_MSG_CNFAddParty (Basedef.h:2246)", MsgCNFAddParty, 125 | flagGame2Client | flagClient2Game},
		{"_MSG_UpdateEquip (Basedef.h:2094)", MsgUpdateEquip, 107 | flagGame2Client | flagClient2Game},
		{"_MSG_REQShopList (Basedef.h:2153)", MsgREQShopList, 123 | flagClient2Game},
		{"_MSG_ShopList (Basedef.h:2014)", MsgShopList, 124 | flagGame2Client},
		{"_MSG_Buy (Basedef.h:2123)", MsgBuy, 121 | flagGame2Client | flagClient2Game},
		{"_MSG_Sell (Basedef.h:2134)", MsgSell, 122 | flagGame2Client | flagClient2Game},
		{"_MSG_Deposit (Basedef.h:2163)", MsgDeposit, 136 | flagGame2Client | flagClient2Game},
		{"_MSG_Withdraw (Basedef.h:2162)", MsgWithdraw, 135 | flagGame2Client | flagClient2Game},
		{"_MSG_UpdateCargoCoin (Basedef.h:1887)", MsgUpdateCargoCoin, 57 | flagGame2Client | flagClient2Game},
		{"_MSG_UpdateEtc (Basedef.h:1852)", MsgUpdateEtc, 55 | flagGame2Client | flagClient2Game},
		{"_MSG_CNFGetItem (Basedef.h:1993)", MsgCNFGetItem, 113 | flagGame2Client},
		{"_MSG_CNFDropItem (Basedef.h:2235)", MsgCNFDropItem, 117 | flagGame2Client},
		{"_MSG_DecayItem (Basedef.h:1973)", MsgDecayItem, 111 | flagGame2Client},
		{"_MSG_CombineComplete (Basedef.h:2353)", MsgCombineComplete, 167 | flagGame2Client | flagClient2Game},
		{"_MSG_UpdateScore (Basedef.h:1824)", MsgUpdateScore, 54 | flagGame2Client | flagClient2Game},
		{"_MSG_SetHpDam (Basedef.h:2048)", MsgSetHpDam, 138 | flagGame2Client},
		{"_MSG_SetHpMp (Basedef.h:2023)", MsgSetHpMp, 129 | flagGame2Client},
		{"_MSG_SendArchEffect (Basedef.h:2572)", MsgSendArchEffect, 180 | flagGame2Client | flagClient2Game},
	}
	for _, tt := range tests {
		if tt.got != Type(tt.want) {
			t.Errorf("%s: type = %#04x (%d), want %#04x (%d)",
				tt.legacy, uint16(tt.got), int(tt.got), tt.want, tt.want)
		}
	}
}
