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
#pragma once

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
#include "GetFunc.h"
#include "SendFunc.h"
#include "CMob.h"
#include "CUser.h"
#include "CNPCGene.h"
#include "CReadFiles.h"
#include "CCastleZakum.h"
#include "CWarTower.h"

// Externs

extern HWND hWndMain;

extern int ServerGroup;

extern unsigned int CurrentTime;
extern int BrState;

extern unsigned short g_pGuildWar[65536];
extern unsigned short g_pGuildAlly[65536];
extern STRUCT_GUILDINFO GuildInfo[65536];

extern unsigned short pMobGrid  [MAX_GRIDY][MAX_GRIDX];
extern unsigned short pItemGrid [MAX_GRIDY][MAX_GRIDX];
extern char pHeightGrid[MAX_GRIDY][MAX_GRIDX];

extern int CurrentWeather;

extern int GuildImpostoID[MAX_GUILDZONE];

extern CNPCGenerator mNPCGen;

extern CUser pUser[MAX_USER];

void  ProcessClientMessage(int conn, char *pMsg, BOOL isServer);


///**************************************************************************************************
void Exec_MSG_Attack(int conn, char *pMsg);
void Exec_MSG_UseItem(int conn, char *pMsg);
void Exec_MSG_MessageWhisper(int conn, char *pMsg);
void Exec_MSG_AccountLogin(int conn, char *pMsg);
void Exec_MSG_CharacterLogin(int conn, char *pMsg);
void Exec_MSG_CharacterLogout(int conn, char *pMsg);
void Exec_MSG_DeleteCharacter(int conn, char *pMsg);
void Exec_MSG_CreateCharacter(int conn, char *pMsg);
void Exec_MSG_AccountSecure(int conn, char *pMsg);
void Exec_MSG_MessageChat(int conn, char *pMsg);
void Exec_MSG_Action(int conn, char *pMsg);
void Exec_MSG_Motion(int conn, char *pMsg);
void Exec_MSG_NoViewMob(int conn, char *pMsg);
void Exec_MSG_Restart(int conn, char *pMsg);
void Exec_MSG_Deprivate(int conn, char *pMsg);
void Exec_MSG_Challange(int conn, char *pMsg);
void Exec_MSG_ChallangeConfirm(int conn, char *pMsg);
void Exec_MSG_ReqTeleport(int conn, char *pMsg);
void Exec_MSG_REQShopList(int conn, char *pMsg);
void Exec_MSG_Deposit(int conn, char *pMsg);
void Exec_MSG_Withdraw(int conn, char *pMsg);
void Exec_MSG_RemoveParty(int conn, char *pMsg);
void Exec_MSG_SendReqParty(int conn, char *pMsg);
void Exec_MSG_AcceptParty(int conn, char *pMsg);
void Exec_MSG_ChangeCity(int conn, char *pMsg);
void Exec_MSG_PKMode(int conn, char *pMsg);
void Exec_MSG_ReqTradeList(int conn, char *pMsg);
void Exec_MSG_UpdateItem(int conn, char *pMsg);
void Exec_MSG_SetShortSkill(int conn, char *pMsg);
void Exec_MSG_DropItem(int conn, char *pMsg);
void Exec_MSG_GetItem(int conn, char *pMsg);
void Exec_MSG_QuitTrade(int conn, char *pMsg);
void Exec_MSG_ApplyBonus(int conn, char *pMsg);
void Exec_MSG_SendAutoTrade(int conn, char *pMsg);
void Exec_MSG_ReqBuy(int conn, char *pMsg);
void Exec_MSG_Buy(int conn, char *pMsg);
void Exec_MSG_Sell(int conn, char *pMsg);
void Exec_MSG_Trade(int conn, char *pMsg);
void Exec_MSG_CombineItem(int conn, char *pMsg);
void Exec_MSG_ReqRanking(int conn, char *pMsg);
void Exec_MSG_CombineItemEhre(int conn, char *pMsg);
void Exec_MSG_CombineItemTiny(int conn, char *pMsg);
void Exec_MSG_CombineItemAilyn(int conn, char *pMsg);
void Exec_MSG_CombineItemAgatha(int conn, char *pMsg);
void Exec_MSG_CombineItemOdin(int conn, char *pMsg);
void Exec_MSG_DeleteItem(int conn, char *pMsg);
void Exec_MSG_InviteGuild(int conn, char *pMsg);
void Exec_MSG_SplitItem(int conn, char *pMsg);
void Exec_MSG_CombineItemLindy(int conn, char *pMsg);
void Exec_MSG_CombineItemShany(int conn, char *pMsg);
void Exec_MSG_CombineItemAlquimia(int conn, char *pMsg);
void Exec_MSG_CombineItemExtracao(int conn, char *pMsg);
void Exec_MSG_GuildAlly(int conn, char *pMsg);
void Exec_MSG_War(int conn, char *pMsg);
void Exec_MSG_CapsuleInfo(int conn, char *pMsg);
void Exec_MSG_PutoutSeal(int conn, char *pMsg);
void Exec_MSG_TradingItem(int conn, char *pMsg);
void Exec_MSG_Quest(int conn, char *pMsg);
//**************************************************************************************************