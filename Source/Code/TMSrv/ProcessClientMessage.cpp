/*
*   Copyright (C) {2015}  {Victor Klafke, Charles TheHouse}
*
*   This program is free software: you can redistribute it and/or modify
*   it under the terms of the GNU General Public License as published by
*   the Free Software Foundation, either version 3 of the License, or
*   (at your option) any later version.
*
*   This program is distributed in the hope that it will be useful,
*   but WITHOUT ANY WARRANTY; without even the implied warranty of
*   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
*   GNU General Public License for more details.
*
*   You should have received a copy of the GNU General Public License
*   along with this program.  If not, see [http://www.gnu.org/licenses/].
*
*   Contact at: victor.klafke@ecomp.ufsm.br
*/
#include <Windows.h>
#include <stdio.h>
#include <fcntl.h>
#include <time.h>
#include <math.h>
#include <io.h>
#include <errno.h>

#include "..\Basedef.h"
#include "..\CPSock.h"
#include "..\ItemEffect.h"
#include "Language.h"

#include "CItem.h"
#include "Server.h"
#include "ProcessClientMessage.h"
#include "GetFunc.h"
#include "SendFunc.h"

void  ProcessClientMessage(int conn, char *pMsg, BOOL isServer)
{
	MSG_STANDARD *std = (MSG_STANDARD *)pMsg;

	if ((std->ID < 0) || (std->ID >= MAX_USER)) 
	{	
		MSG_STANDARD *m = (MSG_STANDARD *)pMsg;

		sprintf(temp, "err,packet Type:%d ID:%d Size:%d KeyWord:%d", m->Type, m->ID, m->Size, m->KeyWord);

		Log(temp, "-system", 0);

		return;
	}

	if (ServerDown >= 120)
		return;

	if (conn > 0 && conn < MAX_USER)
		pUser[conn].LastReceiveTime = SecCounter;

	if (std->Type == _MSG_Ping)
		return;

	// Checa se o pacote foi enviado por algum jogador e possui o timestamp de controle interno.
	if (isServer == FALSE && std->ClientTick == SKIPCHECKTICK)
		return;

	switch(std->Type)
	{
	case _MSG_AccountLogin:
		Exec_MSG_AccountLogin(conn, pMsg);
		break;

	case _MSG_CharacterLogin:
		Exec_MSG_CharacterLogin(conn, pMsg);
		break;

	case _MSG_CharacterLogout:
		Exec_MSG_CharacterLogout(conn, pMsg);
		break;

	case _MSG_DeleteCharacter:
		Exec_MSG_DeleteCharacter(conn, pMsg);
		break;

	case _MSG_CreateCharacter:
		Exec_MSG_CreateCharacter(conn, pMsg);
		break;

	case _MSG_AccountSecure:
		Exec_MSG_AccountSecure(conn, pMsg);
		break;

	case _MSG_MessageChat:
		Exec_MSG_MessageChat(conn, pMsg);
		break;

	case _MSG_Action:
	case _MSG_Action2:
	case _MSG_Action3:
		Exec_MSG_Action(conn, pMsg);
		break;

	case _MSG_Motion:
		Exec_MSG_Motion(conn, pMsg);
		break;			

	case _MSG_UpdateScore:
	{
		Log("cra client send update score", pUser[conn].AccountName, pUser[conn].IP);
		AddCrackError(conn, 2, 91);
	} break;

	case _MSG_NoViewMob:
		Exec_MSG_NoViewMob(conn, pMsg);
		break;

	case _MSG_Restart:
		Exec_MSG_Restart(conn, pMsg);
		break;

	case _MSG_Deprivate:
		Exec_MSG_Deprivate(conn, pMsg);
		break;

	case _MSG_Challange:
		Exec_MSG_Challange(conn, pMsg);
		break;

	case _MSG_ChallangeConfirm:
		Exec_MSG_ChallangeConfirm(conn, pMsg);
		break;

	case _MSG_ReqTeleport:
		Exec_MSG_ReqTeleport(conn, pMsg);
		break;

	case _MSG_REQShopList:
		Exec_MSG_REQShopList(conn, pMsg);
		break;

	case _MSG_Deposit:
		Exec_MSG_Deposit(conn, pMsg);
		break;

	case _MSG_Withdraw:
		Exec_MSG_Withdraw(conn, pMsg);
		break;

	case _MSG_RemoveParty:
		Exec_MSG_RemoveParty(conn, pMsg);
		break;

	case _MSG_SendReqParty:
		Exec_MSG_SendReqParty(conn, pMsg);
		break;

	case _MSG_AcceptParty:
		Exec_MSG_AcceptParty(conn, pMsg);
		break;

	case _MSG_TradingItem:
		Exec_MSG_TradingItem(conn, pMsg);
		break;

	case _MSG_MessageWhisper:
	    Exec_MSG_MessageWhisper(conn, pMsg);
		break;

	case _MSG_ChangeCity:
		Exec_MSG_ChangeCity(conn, pMsg);
		break;

	case _MSG_PKMode:
		Exec_MSG_PKMode(conn, pMsg);
		break;

	case _MSG_ReqTradeList:
		Exec_MSG_ReqTradeList(conn, pMsg);
		break;

	case _MSG_UpdateItem:
		Exec_MSG_UpdateItem(conn, pMsg);
		break;

	case _MSG_Quest:
		Exec_MSG_Quest(conn, pMsg);
		break;

	case _MSG_SetShortSkill:
		Exec_MSG_SetShortSkill(conn, pMsg);
		break;

	case _MSG_Attack:
	case _MSG_AttackOne:
	case _MSG_AttackTwo:
		Exec_MSG_Attack(conn, pMsg);
		break;

	case _MSG_DropItem:
		Exec_MSG_DropItem(conn, pMsg);
		break;

	case _MSG_GetItem:
		Exec_MSG_GetItem(conn, pMsg);
		break;

	case _MSG_QuitTrade: 
		Exec_MSG_QuitTrade(conn, pMsg);
		break;

	case _MSG_UseItem:
		Exec_MSG_UseItem(conn, pMsg);
		break;

	case _MSG_ApplyBonus:
		Exec_MSG_ApplyBonus(conn, pMsg);
		break;

	case _MSG_SendAutoTrade:
		Exec_MSG_SendAutoTrade(conn, pMsg);
		break;

	case _MSG_ReqBuy:
		Exec_MSG_ReqBuy(conn, pMsg);
		break;

	case _MSG_Buy:
		Exec_MSG_Buy(conn, pMsg);
		break;

	case _MSG_Sell:
		Exec_MSG_Sell(conn, pMsg);
		break;

	case _MSG_Trade:
		Exec_MSG_Trade(conn, pMsg);
		break;

	case _MSG_CombineItem:
		Exec_MSG_CombineItem(conn, pMsg);
		break;

	case _MSG_ReqRanking:
		Exec_MSG_ReqRanking(conn, pMsg);
		break;

	case _MSG_CombineItemEhre:
		Exec_MSG_CombineItemEhre(conn, pMsg);
		break;

	case _MSG_CombineItemTiny:
		Exec_MSG_CombineItemTiny(conn, pMsg);
		break;

	case _MSG_CombineItemShany:
		Exec_MSG_CombineItemShany(conn, pMsg);
		break;

	case _MSG_CombineItemAilyn:
		Exec_MSG_CombineItemAilyn(conn, pMsg);
		break;

	case _MSG_CombineItemAgatha:
		Exec_MSG_CombineItemAgatha(conn, pMsg);
		break;

	case _MSG_CombineItemOdin:
	case _MSG_CombineItemOdin2:
		Exec_MSG_CombineItemOdin(conn, pMsg);
		break;

	case _MSG_DeleteItem:
		Exec_MSG_DeleteItem(conn, pMsg);
		break;

	case _MSG_InviteGuild:
		Exec_MSG_InviteGuild(conn, pMsg);
		break;

	case  _MSG_SplitItem:
		Exec_MSG_SplitItem(conn, pMsg);
		break;

	case _MSG_CombineItemLindy:
		Exec_MSG_CombineItemLindy(conn, pMsg);
		break;

	case _MSG_CombineItemAlquimia:
		Exec_MSG_CombineItemAlquimia(conn, pMsg);
		break;

	case _MSG_CombineItemExtracao:
		Exec_MSG_CombineItemExtracao(conn, pMsg);
		break;

	case _MSG_GuildAlly:
		Exec_MSG_GuildAlly(conn, pMsg);
		break;

	case _MSG_War:
		Exec_MSG_War(conn, pMsg);
		break;

	case _MSG_CapsuleInfo:
		Exec_MSG_CapsuleInfo(conn, pMsg);
		break;

	case _MSG_PutoutSeal:
		Exec_MSG_PutoutSeal(conn, pMsg);
		break;

	}
	return;
}