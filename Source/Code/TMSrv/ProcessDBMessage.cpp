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


void ProcessDBMessage(char *Msg)
{
	MSG_STANDARD *std = (MSG_STANDARD *)Msg;

	if (!(std->Type & FLAG_DB2GAME) || (std->ID < 0) || (std->ID >= MAX_USER))
	{
		MSG_STANDARD *m = (MSG_STANDARD *)Msg;

		sprintf(temp, "err,packet Type:%d ID:%d Size:%d KeyWord:%d", m->Type, m->ID, m->Size, m->KeyWord);

		Log(temp, "-system", 0);

		return;
	}

	int conn = std->ID;
#pragma region DB - TM
	if (conn == 0)
	{
		switch (std->Type)
		{

		case _MSG_TransperCharacter:
		{

									   TransperCharacter = 1;
									   Log("TransperCharacter mode", "system", 0);

		} break;

		case _MSG_DBSetIndex:
		{
								MSG_STANDARDPARM3 *m = (MSG_STANDARDPARM3*)Msg;

								if (m->Parm1 != -1)
								{
									ServerGroup = m->Parm1;
									ServerIndex = m->Parm3;
								}

								Sapphire = m->Parm2;

		} break; // TODO: Check if it isn't a break (must compile and check against the original TM

		case _MSG_War:
		{
						 MSG_STANDARDPARM2 *m = (MSG_STANDARDPARM2*)Msg;

						 DoWar(m->Parm1, m->Parm2);

		} break;

		case _MSG_GuildAlly:
		{
							   MSG_STANDARDPARM2 *m = (MSG_STANDARDPARM2*)Msg;

							   DoAlly(m->Parm1, m->Parm2);

		} break;

		case _MSG_GuildInfo:
		{
							   MSG_GuildInfo *m = (MSG_GuildInfo*)Msg;

							   GuildInfo[m->Guild] = m->GuildInfo;

		} break;

		case _MSG_GuildReport:
		{
								 MSG_GuildReport *m = (MSG_GuildReport*)Msg;

								 memcpy(ChargedGuildList, m->ChargedGuildList, sizeof(ChargedGuildList));
		} break;

		case _MSG_NPNotice:
		{
							  MSG_NPNotice *m = (MSG_NPNotice*)Msg;

							  if (m->Parm1 == 1)
								  SendNotice(m->String);

		} break;

		case _MSG_MessageDBImple:
		{
									MSG_MessageDBImple *m = (MSG_MessageDBImple*)Msg;

									m->String[MESSAGE_LENGTH - 1] = 0;
									m->String[MESSAGE_LENGTH - 2] = 0;

									Log(m->String, "-system", 0);

									ProcessImple(0, m->Level, m->String);

		} break; // TODO: Check if it isn't a break (must compile and check against the original TM

		case _MSG_MagicTrumpet:
		{
								  SyncMulticast(0, (MSG_STANDARD*)Msg, 0);
		} break;

		case _MSG_DBNotice:
		{
							  MSG_DBNotice *m = (MSG_DBNotice*)Msg;

							  SendNotice(m->String);
		} break;
		
		case _MSG_DBCheckPrimaryAccount:
		{
			MSG_DBCheckPrimaryAccount *m = (MSG_DBCheckPrimaryAccount*)Msg;
		
			for (int i = 1; i < MAX_USER; i++)
			{
				if (pUser[i].Mode == USER_EMPTY || pUser[i].cSock.Sock == 0)
					continue;
		
				if ( memcmp(pUser[i].Mac, m->Mac, sizeof(m->Mac)))
					continue;
		
				/*if (strncmp(pUser[i].AccountName, m->AccountName, ACCOUNTNAME_LENGTH) == 0)
				{
					pUser[i].OnlyTrade = 0;
					continue;
				}
		
				pUser[i].OnlyTrade = 1;
				RemoveParty(i);
		
				if (pMob[i].Mode == USER_PLAY)
				{
					int Village = BASE_GetVillage(pMob[i].TargetX, pMob[i].TargetY);
					unsigned char mapAttribute = GetAttribute(pMob[i].TargetX, pMob[i].TargetY);
		
					if ((Village < 0 || Village >= 5) && (mapAttribute & 0x80) == 0)
					{
						SendClientMessage(i, g_pMessageStringTable[_NN_OnlyVillage]);
						DoRecall(i);
					}
				}*/
			}
		
		} break;
		
		}
	}
#pragma endregion
#pragma region DB - TM - CLIENT
	else
	{
		if ((conn > 0 && conn < MAX_USER) && pUser[conn].Mode == 0)
		{
			MSG_STANDARD sm;

			sm.Type = _MSG_DBNoNeedSave;
			sm.ID = conn;

			DBServerSocket.SendOneMessage((char*)&sm, sizeof(MSG_STANDARD));

			return;
		}

		if (conn <= 0 || conn > MAX_USER)
		{
			sprintf(temp, "Unknown DB Message - conn:%d - type:%d", conn, std->Type);
			Log(temp, "-system", 0);

			return;
		}

		switch (std->Type)
		{
#pragma region _MSG_ReqTransper
		case _MSG_ReqTransper:
		{
								 if (TransperCharacter == 0)
									 return;

								 MSG_ReqTransper *m = (MSG_ReqTransper*)Msg;

								 m->ID = ESCENE_FIELD + 1;

								 pUser[conn].cSock.SendOneMessage((char*)&m, sizeof(MSG_ReqTransper));

								 pUser[conn].Mode = USER_SELCHAR;

		} break;
#pragma endregion
#pragma region _MSG_DBSendItem
		case _MSG_DBSendItem:
		{
								MSG_DBSendItem *m = (MSG_DBSendItem*)Msg;

								if (pUser[conn].Mode < USER_SELCHAR)
								{
									char ItemCode[256];

									sprintf(temp, "tra,%s", pUser[conn].AccountName);
									BASE_GetItemCode(&m->Item, ItemCode);
									strcat(temp, ItemCode);
									Log(temp, "_fail_play_", pUser[conn].IP);

									break;
								}

								if (strcmp(pUser[conn].AccountName, m->Account) != 0)
								{
									char ItemCode[256];

									sprintf(temp, "tra,%s", pUser[conn].AccountName);
									BASE_GetItemCode(&m->Item, ItemCode);
									strcat(temp, ItemCode);
									Log(temp, "_fail_name_", pUser[conn].IP);

									break;
								}

								int i = 0;

								for (; i < MAX_CARGO - 2; i++)
								{
									int retsour = BASE_CanCargo(&m->Item, pUser[conn].Cargo, i % 9, i / 9);

									if (retsour != TRUE)
										continue;

									pUser[conn].Cargo[i] = m->Item;

									SendItem(conn, ITEM_PLACE_CARGO, i, &m->Item);

									char cMsg[128];

									sprintf(cMsg, "%s [%s]", g_pMessageStringTable[_NN_Item_Arrived], g_pItemList[m->Item.sIndex].Name);

									SendClientMessage(conn, cMsg);

									m->Result = 0;

									DBServerSocket.SendOneMessage((char*)m, m->Size);

									char ItemCode[256];

									sprintf(temp, "tra,%s %d", pUser[conn].AccountName, i);
									BASE_GetItemCode(&m->Item, ItemCode);
									strcat(temp, ItemCode);
									Log(temp, "_web1_", pUser[conn].IP);

									if (pUser[conn].Mode == USER_SELCHAR)
										SaveUser(conn, 0);

									return;
								}

								for (i = 127; i >= 0; i--)
								{
									if (pUser[conn].Cargo[i].sIndex != 0)
										continue;

									pUser[conn].Cargo[i] = m->Item;

									SendItem(conn, ITEM_PLACE_CARGO, i, &m->Item);

									char cMsg[128];

									sprintf(cMsg, "%s [%s]", g_pMessageStringTable[_NN_Item_Arrived], g_pItemList[m->Item.sIndex].Name);

									SendClientMessage(conn, cMsg);

									m->Result = 0;

									DBServerSocket.SendOneMessage((char*)m, m->Size);

									char ItemCode[256];

									sprintf(temp, "tra,%s %d", pUser[conn].AccountName, i);
									BASE_GetItemCode(&m->Item, ItemCode);
									strcat(temp, ItemCode);
									Log(temp, "_web2_", pUser[conn].IP);

									if (pUser[conn].Mode == USER_SELCHAR)
										SaveUser(conn, 0);

									return;
								}

								m->Result = 3;

								DBServerSocket.SendOneMessage((char*)m, m->Size);

								char ItemCode[256];

								SaveUser(conn, 1);

								sprintf(temp, "tra,%s", pUser[conn].AccountName);
								BASE_GetItemCode(&m->Item, ItemCode);
								strcat(temp, ItemCode);
								Log(temp, "_fail_empty_", pUser[conn].IP);

		} break;
#pragma endregion
#pragma region _MSG_NPNotice
		case _MSG_NPNotice:
		{
							  MSG_NPNotice *m = (MSG_NPNotice*)Msg;

							  if (m->Parm1 == 0)
							  {
								  if (pUser[conn].Mode == USER_PLAY)
									  SendClientMessage(conn, m->String);
							  }

		} break;
#pragma endregion
#pragma region _MSG_DBCNFAccountLogin
		case _MSG_DBCNFAccountLogin:
		{
									   MSG_DBCNFAccountLogin *m = (MSG_DBCNFAccountLogin*)Msg;

									   if (strcmp(m->AccountName, pUser[conn].AccountName) != 0)
									   {
										   SendClientMessage(conn, g_pMessageStringTable[_NN_Try_Reconnect]);

										   pUser[conn].cSock.SendMessageA();

										   CloseUser(conn);

										   return;
									   }

									   for (int x = 0; x < MAX_MAC; x++)
									   {
										   if (pMac[x].Mac[0] == 0 && pMac[x].Mac[1] == 0 && pMac[x].Mac[2] == 0 && pMac[x].Mac[3] == 0)
											   continue;

										   if (pMac[x].Mac[0] == pUser[conn].Mac[0] && pMac[x].Mac[1] == pUser[conn].Mac[1] && pMac[x].Mac[2] == pUser[conn].Mac[2] && pMac[x].Mac[3] == pUser[conn].Mac[3])
										   {
											   SendClientMessage(conn, g_pMessageStringTable[_NN_MAC_Block]);

											   pUser[conn].cSock.SendMessageA();

											   CloseUser(conn);
											   break;
										   }
									   }

									   if (m->Unknow_28 != 1)
									   {
										   for (int i = 0; i < MAX_CARGO - 2; i++)
										   {
											   if (i < 5)
											   {
												   m->Cargo[i].sIndex = 400;
											   }
											   else if (i >= 5 && i < 10)
											   {
												   m->Cargo[i].sIndex = 405;
											   }
										   }

										   m->Cargo[16].sIndex = 3318;

										   m->Cargo[17].sIndex = 3983;
										   //BASE_SetItemDate(&m->Cargo[17], 3);
									   }

									   pUser[conn].IsBlocked = m->IsBlocked;
									   strcpy(pUser[conn].BlockPass, m->BlockPass);

									   pUser[conn].IsNewBieRe = 1;


									   pUser[conn].IsBillConnect = 0;
									   pUser[conn].Whisper = 0;
									   pUser[conn].Guildchat = 0;
									   pUser[conn].PartyChat = 0;
									   pUser[conn].KingChat = 0;
									   pUser[conn].Admin = 0;
									   pUser[conn].Chatting = 0;
									   pUser[conn].Unk_2732 = 0;
									   pUser[conn].Unk_2728 = 0;
									  // pUser[conn].OnlyTrade = 1;

									   m->ID = ESCENE_FIELD + 2;
									   m->Type = _MSG_CNFAccountLogin;

									   pUser[conn].Mode = USER_SELCHAR;

									   for (int i = 0; i < MAX_CARGO; i++)
									   {
										   STRUCT_ITEM *tempsour = &m->Cargo[i];

										   if (tempsour->sIndex > 0 && tempsour->sIndex < MAX_ITEMLIST)
										   {
											   int nPos = g_pItemList[tempsour->sIndex].nPos;

											   if (nPos == 64 || nPos == 192)
											   {
												   if (tempsour->stEffect[0].cEffect == EF_DAMAGEADD || tempsour->stEffect[0].cEffect == EF_DAMAGE2)
													   tempsour->stEffect[0].cEffect = EF_DAMAGE;

												   if (tempsour->stEffect[1].cEffect == EF_DAMAGEADD || tempsour->stEffect[1].cEffect == EF_DAMAGE2)
													   tempsour->stEffect[1].cEffect = EF_DAMAGE;

												   if (tempsour->stEffect[2].cEffect == EF_DAMAGEADD || tempsour->stEffect[2].cEffect == EF_DAMAGE2)
													   tempsour->stEffect[2].cEffect = EF_DAMAGE;
											   }
										   }
									   }

									   if (evDelete != 0)
									   {
										   for (int i = 0; i < MAX_CARGO; i++)
										   {
											   if (m->Cargo[i].sIndex >= 470 && m->Cargo[i].sIndex <= 500)
												   m->Cargo[i].sIndex = 0;
										   }
									   }

									   pUser[conn].LastQuestDay = m->QuestDiaria.LastTimeQuest;
									   pUser[conn].QuestAtiva = m->QuestDiaria.IndexQuest;
									   pUser[conn].CountMob1 = m->QuestDiaria.MobCount1;
									   pUser[conn].CountMob2 = m->QuestDiaria.MobCount2;
									   pUser[conn].CountMob3 = m->QuestDiaria.MobCount3;

									   pUser[conn].cSock.SendOneMessage((char*)m, sizeof(MSG_DBCNFAccountLogin));

									   memcpy(pUser[conn].Cargo, m->Cargo, sizeof(STRUCT_ITEM)* MAX_CARGO);

									   pUser[conn].Coin = m->Coin;
									   pUser[conn].Unk_1816 = 0;
									   pUser[conn].SelChar = m->sel;

									   pUser[conn].Mode = USER_SELCHAR;

									   if (BILLING > 0 && IsFree(&m->sel) != 0)
									   {
										   if (CHARSELBILL == 0)
											   SendBilling(conn, m->AccountName, 8, 1);
										   else
											   SendBilling(conn, m->AccountName, 1, 1);

										   pUser[conn].Unk_2732 = SecCounter;
									   }

									   pUser[conn].Unk5[0] = 0;
									   pUser[conn].LastClientTick = 0;

									   sprintf(temp, "CNFAccountLogin Mac: %d.%d.%d.%d", pUser[conn].Mac[0], pUser[conn].Mac[1], pUser[conn].Mac[2], pUser[conn].Mac[3]);
									   Log(temp, pUser[conn].AccountName, pUser[conn].IP);

									   if (TransperCharacter != 0)
									   {
										   MSG_STANDARDPARM2 sm;

										   sm.Type = _MSG_TransperCharacter;
										   sm.ID = ESCENE_FIELD + 1;
										   sm.Parm1 = 0;
										   sm.Parm2 = 0;
										   sm.Size = sizeof(MSG_STANDARDPARM2);

										   pUser[conn].cSock.SendOneMessage((char*)&sm, sizeof(MSG_STANDARDPARM2));
									   }

		} break;
#pragma endregion
#pragma region _MSG_DBNewAccountFail
		case _MSG_DBNewAccountFail:
		{
									  MSG_STANDARD *m = (MSG_STANDARD*)Msg;

									  m->ID = ESCENE_FIELD + 3;

									  SendClientSignal(conn, 0, _MSG_NewAccountFail);

									  CloseUser(conn);
		} break;
#pragma endregion
#pragma region _MSG_DBAccountLoginFail_Account
		case _MSG_DBAccountLoginFail_Account:
		{
												MSG_STANDARD *m = (MSG_STANDARD*)Msg;

												int ID = m->ID;

												SendClientMessage(conn, g_pMessageStringTable[_NN_No_Account_With_That_Name]);
												pUser[conn].cSock.SendMessageA();

												CloseUser(conn);

		} break;
#pragma endregion
#pragma region _MSG_DBAccountLoginFail_Block
		case _MSG_DBAccountLoginFail_Block:
		{
											  MSG_STANDARDPARM *m = (MSG_STANDARDPARM*)Msg;

											  int ID = m->ID;

											  if (m->Parm == 0)
												  sprintf(temp, g_pMessageStringTable[_SN_Blocked_Account], "X");

											  else if (m->Parm == 1)
												  sprintf(temp, g_pMessageStringTable[_SN_Blocked_Account], "A");

											  else if (m->Parm == 2)
												  sprintf(temp, g_pMessageStringTable[_SN_Blocked_Account], "B");

											  else if (m->Parm == 3)
												  sprintf(temp, g_pMessageStringTable[_SN_Blocked_Account], "C");

											  SendClientMessage(conn, temp);
											  pUser[conn].cSock.SendMessageA();

											  CloseUser(conn);

		} break;
#pragma endregion
#pragma region _MSG_DBAccountLoginFail_Disable
		case _MSG_DBAccountLoginFail_Disable:
		{
												MSG_STANDARD *m = (MSG_STANDARD*)Msg;

												int ID = m->ID;

												SendClientMessage(conn, g_pMessageStringTable[_NN_Disabled_Account]);
												pUser[conn].cSock.SendMessageA();

												CloseUser(conn);

		} break;
#pragma endregion
#pragma region _MSG_AccountSecure
		case _MSG_AccountSecure:
		{
								   SendClientSignal(std->ID, ESCENE_FIELD, _MSG_AccountSecure);
		} break;
#pragma endregion
#pragma region _MSG_AccountSecureFail
		case _MSG_AccountSecureFail:
		{
									   SendClientSignal(std->ID, ESCENE_FIELD, _MSG_AccountSecureFail);
		} break;
#pragma endregion
#pragma region _MSG_DBOnlyOncePerDay
		case _MSG_DBOnlyOncePerDay:
		{
									  MSG_STANDARD *m = (MSG_STANDARD*)Msg;

									  int ID = m->ID;

									  SendClientMessage(conn, g_pMessageStringTable[_NN_Only_Once_Per_Day]);
									  pUser[conn].cSock.SendMessageA();

		} break;
#pragma endregion
#pragma region _MSG_DBAccountLoginFail_Pass
		case _MSG_DBAccountLoginFail_Pass:
		{
											 MSG_STANDARD *m = (MSG_STANDARD*)Msg;

											 int ID = m->ID;

											 SendClientMessage(conn, g_pMessageStringTable[_NN_Wrong_Password]);
											 pUser[conn].cSock.SendMessageA();

											 CloseUser(conn);

		} break;
#pragma endregion
#pragma region _MSG_DBCNFNewCharacter
		case _MSG_DBCNFNewCharacter:
		{
									   MSG_CNFNewCharacter *m = (MSG_CNFNewCharacter*)Msg;

									   m->Type = _MSG_CNFNewCharacter;
									   m->ID = ESCENE_FIELD + 1;

									   pUser[conn].cSock.SendOneMessage((char*)m, sizeof(MSG_CNFNewCharacter));

									   pUser[conn].Mode = USER_SELCHAR;



		} break;
#pragma endregion
#pragma region _MSG_DBCNFDeleteCharacter
		case _MSG_DBCNFDeleteCharacter:
		{
										  MSG_CNFDeleteCharacter *m = (MSG_CNFDeleteCharacter*)Msg;

										  m->Type = _MSG_CNFDeleteCharacter;
										  m->ID = ESCENE_FIELD + 1;

										  pUser[conn].cSock.SendOneMessage((char*)m, sizeof(MSG_CNFDeleteCharacter));

										  pUser[conn].Mode = USER_SELCHAR;

		} break;
#pragma endregion
#pragma region _MSG_DBDeleteCharacterFail
		case _MSG_DBDeleteCharacterFail:
		{
										   MSG_STANDARD *m = (MSG_STANDARD*)Msg;

										   m->ID = ESCENE_FIELD + 1;

										   SendClientSignal(conn, 0, _MSG_DeleteCharacterFail);

										   pUser[conn].Mode = USER_SELCHAR;

		} break;
#pragma endregion
#pragma region _MSG_DBNewCharacterFail
		case _MSG_DBNewCharacterFail:
		{
										MSG_STANDARD *m = (MSG_STANDARD*)Msg;

										m->ID = ESCENE_FIELD + 1;

										SendClientSignal(conn, 0, _MSG_NewCharacterFail);

										pUser[conn].Mode = USER_SELCHAR;

		} break;
#pragma endregion
#pragma region _MSG_DBCNFCharacterLogin
		case _MSG_DBCNFCharacterLogin:
		{
										 MSG_CNFCharacterLogin *m = (MSG_CNFCharacterLogin*)Msg;

										 MSG_CNFClientCharacterLogin sm;

										 memcpy(&sm, m, sizeof(MSG_CNFClientCharacterLogin));

										 if (conn == 0)
										 {
											 CrackLog(0, " CNFCharLogin");

											 CloseUser(conn);

											 break; // return; ?
										 }

										 int i;

										 for (i = 0; i < MAX_EQUIP; i++)
										 {
											 STRUCT_ITEM *tempsour = &sm.mob.Equip[i];

											 if (tempsour->sIndex > 0 && tempsour->sIndex < MAX_ITEMLIST)
											 {
												 int nPos = g_pItemList[tempsour->sIndex].nPos;

												 if (nPos == 64 || nPos == 192)
												 {
													 if (tempsour->stEffect[0].cEffect == EF_DAMAGE2 || tempsour->stEffect[0].cEffect == EF_DAMAGEADD)
														 tempsour->stEffect[0].cEffect = EF_DAMAGE;

													 if (tempsour->stEffect[1].cEffect == EF_DAMAGE2 || tempsour->stEffect[1].cEffect == EF_DAMAGEADD)
														 tempsour->stEffect[1].cEffect = EF_DAMAGE;

													 if (tempsour->stEffect[2].cEffect == EF_DAMAGE2 || tempsour->stEffect[2].cEffect == EF_DAMAGEADD)
														 tempsour->stEffect[2].cEffect = EF_DAMAGE;
												 }
											 }
										 }

										 for (i = 0; i < MAX_CARRY - 1; i++)
										 {
											 STRUCT_ITEM *tempsour = &sm.mob.Carry[i];

											 if (tempsour->sIndex > 0 && tempsour->sIndex < MAX_ITEMLIST)
											 {
												 int nPos = g_pItemList[tempsour->sIndex].nPos;

												 if (nPos == 64 || nPos == 192)
												 {
													 if (tempsour->stEffect[0].cEffect == EF_DAMAGE2 || tempsour->stEffect[0].cEffect == EF_DAMAGEADD)
														 tempsour->stEffect[0].cEffect = EF_DAMAGE;

													 if (tempsour->stEffect[1].cEffect == EF_DAMAGE2 || tempsour->stEffect[1].cEffect == EF_DAMAGEADD)
														 tempsour->stEffect[1].cEffect = EF_DAMAGE;

													 if (tempsour->stEffect[2].cEffect == EF_DAMAGE2 || tempsour->stEffect[2].cEffect == EF_DAMAGEADD)
														 tempsour->stEffect[2].cEffect = EF_DAMAGE;
												 }
											 }
										 }

										 if (evDelete)
										 {
											 for (int j = 0; j < MAX_CARRY - 1; j++)
											 {
												 if (sm.mob.Carry[j].sIndex >= 470 && sm.mob.Carry[j].sIndex <= 500)
													 sm.mob.Carry[j].sIndex = 0;
											 }
										 }
										 if (m->mobExtra.ClassMaster == MORTAL)
										 {
											 m->mob.Equip[0].stEffect[1].cEffect = 98;
											 m->mob.Equip[0].stEffect[1].cValue = 0;
											 m->mob.Equip[0].stEffect[2].cEffect = 106;
											 m->mob.Equip[0].stEffect[2].cValue = (unsigned char)m->mob.Equip[0].sIndex;
										 }
										 else if (m->mobExtra.ClassMaster == ARCH)
										 {
											 m->mob.Equip[0].stEffect[1].cEffect = 98;
											 m->mob.Equip[0].stEffect[1].cValue = 0;
											 m->mob.Equip[0].stEffect[2].cEffect = 106;
											 m->mob.Equip[0].stEffect[2].cValue = (unsigned char)m->mob.Equip[0].sIndex;
										 }
										 else if (m->mobExtra.ClassMaster == CELESTIAL || m->mobExtra.ClassMaster == SCELESTIAL || m->mobExtra.ClassMaster == CELESTIALCS)
										 {
											 m->mob.Equip[0].stEffect[1].cEffect = 98;
											 m->mob.Equip[0].stEffect[1].cValue = 3;
											 m->mob.Equip[0].stEffect[2].cEffect = 106;
											 m->mob.Equip[0].stEffect[2].cValue = (unsigned char)m->mob.Equip[0].sIndex;
										 }

										 pUser[conn].Donate = m->Donate;

										 m->mob.Equip[0].stEffect[2].cEffect = 28;
										 m->mob.Equip[0].stEffect[2].cValue = m->mobExtra.Citizen;

										 if (m->mob.CurrentScore.Hp <= 0)
											 m->mob.CurrentScore.Hp = 2;

										 pMob[conn].MOB = m->mob;

										 pUser[conn].Message = 0;
										 pUser[conn].UseItemTime = 0;
										 pUser[conn].PotionTime = 0;
										 pMob[conn].LastReqParty = 0;
										 pMob[conn].ProcessorCounter = 1;
										 pMob[conn].QuestFlag = 0;
										 pMob[conn].CheckDrop = 0;
										 pMob[conn].CheckItemDrop = 0;

										 memcpy(&pMob[conn].extra, &m->mobExtra, sizeof(STRUCT_MOBEXTRA));

										 if (m->mob.Guild)
											 pMob[conn].extra.Citizen = GuildInfo[m->mob.Guild].Citizen;

										 memcpy(pUser[conn].CharShortSkill, m->ShortSkill, 16);

										 memcpy(pMob[conn].Affect, m->affect, sizeof(m->affect));

										 pMob[conn].MaxCarry = 30;

										 if (pMob[conn].MOB.Carry[60].sIndex == 3467)
											 pMob[conn].MaxCarry += 15;

										 if (pMob[conn].MOB.Carry[61].sIndex == 3467)
											 pMob[conn].MaxCarry += 15;


										 strncpy(pMob[conn].Tab, "", sizeof(pMob[conn].Tab));
										 strncpy(pMob[conn].Snd, "", sizeof(pMob[conn].Snd));

										 memset(sm.Unk2, 0, sizeof(sm.Unk2));

										 pMob[conn].MOB.BaseScore.Merchant = 0;

										 if (pMob[conn].extra.ClassMaster == ARCH && pMob[conn].extra.QuestInfo.Arch.MortalSlot >= 0 && pMob[conn].extra.QuestInfo.Arch.MortalSlot < 3)
											 pMob[conn].extra.QuestInfo.Arch.MortalLevel = pUser[conn].SelChar.Score[pMob[conn].extra.QuestInfo.Arch.MortalSlot].Level - 299;
										
										 else
											 pMob[conn].extra.QuestInfo.Arch.MortalLevel = 99;

										 BASE_GetHpMp(&pMob[conn].MOB, &pMob[conn].extra);

										 pMob[conn].GetCurrentScore(conn);

										 GetGuild(conn);

										 BASE_GetBonusSkillPoint(&pMob[conn].MOB, &pMob[conn].extra);
										 BASE_GetBonusScorePoint(&pMob[conn].MOB, &pMob[conn].extra);

										 pMob[conn].Mode = MOB_USER;

										 sm.mob = pMob[conn].MOB;
										 sm.mob.SPX = sm.mob.SPX;
										 sm.mob.SPY = sm.mob.SPY;
										 sm.Type = _MSG_CNFCharacterLogin;
										
										 if(NewbieEventServer == 1)
											 sm.ID = ESCENE_FIELD + 1;
										 else
											sm.ID = ESCENE_FIELD;

										 sm.PosX = sm.mob.SPX;
										 sm.PosY = sm.mob.SPY;
										 sm.ClientID = conn;
										 sm.Weather = CurrentWeather;

										 memset(sm.Unk2, 0, sizeof(sm.Unk2));
										 memset(sm.Unk, 0, sizeof(sm.Unk));
										 memset(sm.unk, 0, sizeof(sm.unk));

										 strncpy(&sm.Unk2[448], pUser[conn].AccountName, 12);

										 pMob[conn].LastTime = CurrentTime;
										 pMob[conn].LastX = pMob[conn].TargetX = sm.mob.SPX;
										 pMob[conn].LastY = pMob[conn].TargetY = sm.mob.SPY;

										 if (pMob[conn].MOB.Carry[KILL_MARK].sIndex == 0)
										 {
											 memset(&pMob[conn].MOB.Carry[KILL_MARK], 0, sizeof(STRUCT_ITEM));

											 pMob[conn].MOB.Carry[KILL_MARK].sIndex = 547;
											 pMob[conn].MOB.Carry[KILL_MARK].stEffect[0].cEffect = EF_CURKILL;
											 pMob[conn].MOB.Carry[KILL_MARK].stEffect[1].cEffect = EF_LTOTKILL;
											 pMob[conn].MOB.Carry[KILL_MARK].stEffect[2].cEffect = EF_HTOTKILL;
										 }

										 pUser[conn].Unk_2708 = 0;
										 pUser[conn].LastChat[0] = 0;
										 pUser[conn].Slot = m->Slot;
										 pUser[conn].NumError = 0;
										 pUser[conn].LastMove = 0;
										 pUser[conn].LastAction = _MSG_Action;
										 pUser[conn].LastActionTick = SKIPCHECKTICK;
										 pUser[conn].LastAttack = 0;
										 pUser[conn].LastAttackTick = SKIPCHECKTICK;
										 pUser[conn].LastIllusionTick = SKIPCHECKTICK;
										 pUser[conn].RankingTarget = 0;
										 pUser[conn].RankingType = 0;
										 pUser[conn].CastleStatus = 0;


										 memset(pUser[conn].Unk9, -1, sizeof(pUser[conn].Unk9));
										 memset(&pUser[conn].Trade, 0, sizeof(pUser[conn].Trade));

										 int k;

										 for (k = 0; k < MAX_TRADE; k++)
											 pUser[conn].Trade.InvenPos[k] = -1;

										 for (k = 0; k < MAX_AUTOTRADE; k++)
											 pUser[conn].AutoTrade.CarryPos[k] = -1;

										 pUser[conn].TradeMode = 0;
										 pUser[conn].PKMode = 0;

										 int tx = sm.PosX;
										 int ty = sm.PosY;

										 int CityID = (pMob[conn].MOB.Merchant & 0xC0) >> 6;

										 tx = g_pGuildZone[CityID].CitySpawnX + rand() % 15;
										 ty = g_pGuildZone[CityID].CitySpawnY + rand() % 15;

										 int MobGuild = pMob[conn].MOB.Guild;
										 int MobCLS = pMob[conn].MOB.Class;

										 if (MobCLS < 0 || MobCLS > MAX_CLASS - 1)
										 {
											 Log("err,login Undefined class", pUser[conn].AccountName, pUser[conn].IP);

											 CloseUser(conn);

											 break; // TODO: compile and check if it's break or return
										 }

										 int n;

										 for (n = 0; n < MAX_GUILDZONE; n++)
										 {
											 if (MobGuild != 0 && MobGuild == g_pGuildZone[n].ChargeGuild)
											 {
												 tx = g_pGuildZone[n].GuildSpawnX;
												 ty = g_pGuildZone[n].GuildSpawnY;

												 break;
											 }
										 }

										 if (pMob[conn].MOB.BaseScore.Level < FREEEXP && pMob[conn].extra.ClassMaster == MORTAL)
										 {
											 tx = 2100 + rand() % 5 - 2;
											 ty = 2100 + rand() % 5 - 2;
										 }

										 int ret = GetEmptyMobGrid(conn, &tx, &ty);

										 if (ret == 0)
										 {
											 Log("Can't start can't get mobgrid", pUser[conn].AccountName, pUser[conn].IP);

											 CloseUser(conn);

											 break; // TODO: compile and check if it's break or return
										 }

										 sm.PosX = tx;
										 sm.PosY = ty;

										 pMob[conn].TargetX = tx;
										 pMob[conn].LastX = tx;
										 pMob[conn].TargetY = ty;
										 pMob[conn].LastY = ty;

										 pUser[conn].Mode = USER_PLAY;
										 pUser[conn].Unk_2628 = 0;
										 pUser[conn].Unk_2632 = 0;
										 pUser[conn].Trade.OpponentID = 0;

										 pMob[conn].GetCurrentScore(conn);

										 pMob[conn].GuildDisable = 0;

										 if (pMob[conn].MOB.Guild)
										 {
											 int Group = ServerGroup;
											 int Server = pMob[conn].MOB.Guild / MAX_GUILD;
											 int usGuild = pMob[conn].MOB.Guild & MAX_GUILD - 1;

											 if (pMob[conn].MOB.Clan != GuildInfo[usGuild].Clan)
											 {
												 int mantle = pMob[conn].MOB.Equip[15].sIndex;

												 if (GuildInfo[usGuild].Clan == 7)
												 {
													 if (m->mobExtra.ClassMaster == CELESTIAL || m->mobExtra.ClassMaster == SCELESTIAL || m->mobExtra.ClassMaster == CELESTIALCS)
														 pMob[conn].MOB.Equip[15].sIndex = 3197;

													 else if (mantle >= 543 && mantle <= 544)
														 pMob[conn].MOB.Equip[15].sIndex = 543;

													 else if (mantle >= 545 && mantle <= 546)
														 pMob[conn].MOB.Equip[15].sIndex = 545;

													 else if (mantle == 548)
														 pMob[conn].MOB.Equip[15].sIndex = 543;

													 else if (mantle == 549)
														 pMob[conn].MOB.Equip[15].sIndex = 545;

													 else if (mantle >= 3191 && mantle <= 3193)
														 pMob[conn].MOB.Equip[15].sIndex = 3191;

													 else if (mantle >= 3194 && mantle <= 3196)
														 pMob[conn].MOB.Equip[15].sIndex = 3194;

												 }
												 else if (GuildInfo[usGuild].Clan == 8)
												 {
													 if (m->mobExtra.ClassMaster == CELESTIAL || m->mobExtra.ClassMaster == SCELESTIAL || m->mobExtra.ClassMaster == CELESTIALCS)
														 pMob[conn].MOB.Equip[15].sIndex = 3198;

													 else if (mantle >= 543 && mantle <= 544)
														 pMob[conn].MOB.Equip[15].sIndex = 544;

													 else if (mantle >= 545 && mantle <= 546)
														 pMob[conn].MOB.Equip[15].sIndex = 546;

													 else if (mantle == 548)
														 pMob[conn].MOB.Equip[15].sIndex = 544;

													 else if (mantle == 549)
														 pMob[conn].MOB.Equip[15].sIndex = 546;

													 else if (mantle >= 3191 && mantle <= 3193)
														 pMob[conn].MOB.Equip[15].sIndex = 3192;

													 else if (mantle >= 3194 && mantle <= 3196)
														 pMob[conn].MOB.Equip[15].sIndex = 3195;
												 }

												 memcpy(&sm.mob.Equip[15], &pMob[conn].MOB.Equip[15], sizeof(STRUCT_ITEM));
											 }
										 }

										 char tt[512];

										 pUser[conn].cProgress = 0;
										 pUser[conn].ReqHp = pMob[conn].MOB.CurrentScore.Hp;
										 pUser[conn].ReqMp = pMob[conn].MOB.CurrentScore.Mp;
										 pUser[conn].Unk_2688 = 0;

										 pUser[conn].cSock.SendOneMessage((char*)&sm, sizeof(MSG_CNFClientCharacterLogin));

										 ///////////////////////////////////////////////
										 // Packet that will be sent to other players //
										 ///////////////////////////////////////////////

										 MSG_CreateMob sm2;

										 GetCreateMob(conn, &sm2);

										 sm2.CreateType = 2;

										 STRUCT_ITEM* Mount = &pMob[conn].MOB.Equip[14]; //
										 
										 pMobGrid[sm.PosY][sm.PosX] = conn;

										 GridMulticast(sm.PosX, sm.PosY, (MSG_STANDARD*)&sm2, 0);

										 SendPKInfo(conn, conn);
										 SendGridMob(conn);

										 if (pMob[n].MOB.CurrentScore.Level >= 999)
											 pUser[n].Admin = 1;

										 MountProcess(conn, Mount); // MountProcess(conn, 0);

										 pMob[conn].ClientIndex = conn;

										 MountProcess(conn, 0);

										 for (int q = 0; q < _Max_Recycle_Bin_; q++)
										 {
											 memset(&pMob[conn].Recycle[q].Item, 0, sizeof(STRUCT_ITEM));

											 pMob[conn].Recycle[q].Price = 0;
											 pMob[conn].Recycle[q].Ticks = 0;
										 }

										 SendQuest(conn);

										 if (pUser[conn].QuestAtiva != 0)
										 {
											 STRUCT_QUESTDIARIA packet;
											 memset(&packet, 0, sizeof(STRUCT_QUESTDIARIA));

											 packet.Type = 0xD00;
											 packet.ID = conn;
											 packet.Size = sizeof(STRUCT_QUESTDIARIA);

											 packet.Count1 = pUser[conn].CountMob1;
											 packet.Count2 = pUser[conn].CountMob2;
											 packet.Count3 = pUser[conn].CountMob3;

											 packet.CountMax1 = QuestDiaria[pUser[conn].QuestAtiva].QtdMob1;
											 packet.CountMax2 = QuestDiaria[pUser[conn].QuestAtiva].QtdMob2;
											 packet.CountMax3 = QuestDiaria[pUser[conn].QuestAtiva].QtdMob3;

											 packet.QtdItem = QuestDiaria[pUser[conn].QuestAtiva].Item->stEffect[0].cValue;
											 packet.ExpReward = QuestDiaria[pUser[conn].QuestAtiva].ExpReward;
											 packet.GoldReward = QuestDiaria[pUser[conn].QuestAtiva].GoldReward;

											 packet.IndexQuest = pUser[conn].QuestAtiva;

											 sprintf(temp, "Quest Diaria:");
											 SendMsgExp(conn, temp, TNColor::GreenYellow, false);

											 sprintf(temp, "Mate os monstros:");
											 SendMsgExp(conn, temp, TNColor::GoldenEscuro, false);

											 sprintf(temp, "%s(%d), %s(%d) e %s(%d).", g_pItemList[QuestDiaria[pUser[conn].QuestAtiva].IdMob1].Name, QuestDiaria[pUser[conn].QuestAtiva].QtdMob1, g_pItemList[QuestDiaria[pUser[conn].QuestAtiva].IdMob2].Name, QuestDiaria[pUser[conn].QuestAtiva].QtdMob2, g_pItemList[QuestDiaria[pUser[conn].QuestAtiva].IdMob3].Name, QuestDiaria[pUser[conn].QuestAtiva].QtdMob3);
											 SendMsgExp(conn, temp, TNColor::White, false);

											 sprintf(temp, "Receba:");
											 SendMsgExp(conn, temp, TNColor::GoldenEscuro, false);

											 sprintf(temp, "%d de EXP & %d de Gold.", QuestDiaria[pUser[conn].QuestAtiva].ExpReward, QuestDiaria[pUser[conn].QuestAtiva].GoldReward);
											 SendMsgExp(conn, temp, TNColor::White, false);

											 pUser[conn].cSock.SendOneMessage((char*)&packet, sizeof(STRUCT_QUESTDIARIA));
										 }


										 time_t mtime;
										 time(&mtime);

										 if (mtime > (pUser[conn].LastQuestDay + 86400)) // Se o tempo atual for menor que o prazo de 24hrs 
										 {
											 STRUCT_QUESTDIARIA packet;
											 memset(&packet, 0, sizeof(STRUCT_QUESTDIARIA));

											 packet.Type = 0xD00;
											 packet.ID = conn;
											 packet.Size = sizeof(STRUCT_QUESTDIARIA);

											 int rand_ = rand() % 5 + 1;

											 pUser[conn].LastQuestDay = mtime;

											 pUser[conn].QuestAtiva = QuestDiaria[rand_].IndexQuest;
											 packet.CountMax1 = QuestDiaria[rand_].QtdMob1;
											 packet.CountMax2 = QuestDiaria[rand_].QtdMob2;
											 packet.CountMax3 = QuestDiaria[rand_].QtdMob3;
											 packet.QtdItem = QuestDiaria[rand_].Item->stEffect[0].cValue;
											 packet.GoldReward = QuestDiaria[rand_].GoldReward;
											 packet.ExpReward = QuestDiaria[rand_].ExpReward;

											 packet.IndexQuest = pUser[conn].QuestAtiva;

											 pUser[conn].cSock.SendOneMessage((char*)&packet, sizeof(STRUCT_QUESTDIARIA));
										 }
										 SendWarInfo(conn, g_pGuildZone[4].Clan);

										 if (CastleState != 0)
											 SendClientSignalParm(conn, ESCENE_FIELD, _MSG_SendCastleState, CastleState);

										 ClearCrown(conn);

										 int curlvl = pMob[conn].MOB.BaseScore.Level;

										 int max_level = 0;

										 if (pMob[conn].extra.ClassMaster == MORTAL || pMob[conn].extra.ClassMaster == ARCH)
											 max_level = MAX_LEVEL;

										 else if (pMob[conn].extra.ClassMaster == CELESTIAL || pMob[conn].extra.ClassMaster == SCELESTIAL || pMob[conn].extra.ClassMaster == CELESTIALCS)
											 max_level = MAX_CLEVEL;


										 if (curlvl < max_level - 1)
										 {
											 long long exp = pMob[conn].MOB.Exp;
											 long long curexp = max_level == MAX_LEVEL ? g_pNextLevel[curlvl] : g_pNextLevel_2[curlvl];
											 long long nextexp = max_level == MAX_LEVEL ? g_pNextLevel[curlvl + 1] : g_pNextLevel_2[curlvl + 1];
											 long long deltaexp = (nextexp - curexp) / 4;
											 long long Segment1 = curexp + deltaexp;
											 long long Segment2 = curexp + (deltaexp * 2);
											 long long Segment3 = curexp + (deltaexp * 3);
											 long long Segment4 = nextexp;

											 if (exp > Segment3)
												 pMob[conn].Segment = 3;
											 else if (exp > Segment2)
												 pMob[conn].Segment = 2;
											 else if (exp > Segment1)
												 pMob[conn].Segment = 1;
											 else
												 pMob[conn].Segment = 0;

											 pUser[conn].Unk_2736 = 0;

											 if (pMob[conn].MOB.BaseScore.Level >= FREEEXP)
											 {
												 if (CHARSELBILL == 0)
													 SendBilling(conn, pUser[conn].AccountName, 1, 1);
											 }

										 }

										 if (KefraLive != 0)
										 {
											 BASE_GetGuildName(ServerGroup, KefraLive, KefraKiller);
											 sprintf(tt, g_pMessageStringTable[_SN_End_Khepra], KefraKiller);

											 SendClientMessage(conn, tt);
										 }

										 if (g_pRvrWar.Bonus != 0)
										 {
											 if (g_pRvrWar.Bonus == pMob[conn].MOB.Clan)
											 {
												 if (pMob[conn].MOB.Clan == 8)
													 sprintf(temp, g_pMessageStringTable[_SN_KINGDOMWAR_DROP_], g_pMessageStringTable[_NN_KINGDOM_RED]);

												 else if (pMob[conn].MOB.Clan == 7)
													 sprintf(temp, g_pMessageStringTable[_SN_KINGDOMWAR_DROP_], g_pMessageStringTable[_NN_KINGDOM_BLUE]);

												 SendClientMessage(conn, temp);

											 }
										 }

										 if (pMob[conn].MOB.Guild != 0)
										 {
											 if (g_quests.MsgGuild[pMob[conn].MOB.Guild].Aviso[0] != '\0' && g_quests.MsgGuild[pMob[conn].MOB.Guild].Aviso[0] != ' ')
											 {
												 sprintf(temp, "![Guild]> Aviso: %s", g_quests.MsgGuild[pMob[conn].MOB.Guild].Aviso);
												 SendClientMessage(conn, temp);
											 }
										 }

										 sprintf(tt, g_pMessageStringTable[_DN_TOWNSPEOPLE], Citizen);
										 SendClientMessage(conn, tt);

										 SendEtc(conn);
										 SendScore(conn);

										 SendUpdateWoteBattle();
										 CreateExportInspectTable(conn);

										 sprintf(tt, "sta,Login char:%s exp:%llu level:%d conn:%d money:%d, store:%d", pMob[conn].MOB.MobName, pMob[conn].MOB.Exp, pMob[conn].MOB.BaseScore.Level, conn, pMob[conn].MOB.Coin, pUser[conn].Donate);

										 Log(tt, pUser[conn].AccountName, pUser[conn].IP);
		} break;
#pragma endregion
#pragma region _MSG_DBCharacterLoginFail
		case _MSG_DBCharacterLoginFail:
		{
										  MSG_STANDARD *m = (MSG_STANDARD*)Msg;

										  m->ID = ESCENE_FIELD;

										  SendClientSignal(conn, 0, _MSG_CharacterLoginFail);

										  pUser[conn].Mode = USER_SELCHAR;

										  CrackLog(conn, " CharLoginFail");

		} break;
#pragma endregion
#pragma region _MSG_DBMessageBoxOk
		case _MSG_DBMessageBoxOk:
		{
									MSG_MessageBoxOk *m = (MSG_MessageBoxOk*)Msg;

									m->Type = _MSG_MessageBoxOk;
									m->ID = 0;

									pUser[conn].cSock.SendOneMessage((char*)m, sizeof(MSG_MessageBoxOk));

		} break;
#pragma endregion
#pragma region _MSG_DBAlreadyPlaying
		case _MSG_DBAlreadyPlaying:
		{
									  MSG_STANDARD *m = (MSG_STANDARD*)Msg;

									  SendClientSignal(m->ID, ESCENE_FIELD + 2, _MSG_AlreadyPlaying);

									  pUser[m->ID].cSock.SendMessageA();

									  CloseUser(conn);

		} break;
#pragma endregion
#pragma region _MSG_DBStillPlaying
		case _MSG_DBStillPlaying:
		{
									MSG_STANDARD *m = (MSG_STANDARD*)Msg;

									SendClientSignal(m->ID, ESCENE_FIELD + 2, _MSG_StillPlaying);

									pUser[m->ID].cSock.SendMessageA();

									CloseUser(conn);

		} break;

		case 0x3AE:
		{
			MSG_STANDARD sm2;
			memset(&sm2, 0, sizeof(MSG_STANDARD));

			sm2.Type = 0x123;
			sm2.ID = conn;
			sm2.Size = sizeof(MSG_STANDARD);

			pUser[conn].cSock.SendOneMessage((char*)&sm2, sizeof(MSG_STANDARD));
		}
#pragma endregion
#pragma region _MSG_DBSavingQuit
		case _MSG_DBSavingQuit:
		{
								  MSG_DBSavingQuit *m = (MSG_DBSavingQuit*)Msg;

								  if (m->ID <= 0 || m->ID >= MAX_USER)
								  {
									  Log("err,DBsavingquit - id range", "-system", 0);

									  break;
								  }

								  if (pUser[m->ID].Mode != USER_PLAY && pUser[m->ID].Mode != USER_SAVING4QUIT)
								  {
									  MSG_STANDARD sm;

									  sm.Type = _MSG_DBNoNeedSave;
									  sm.ID = conn;

									  DBServerSocket.SendOneMessage((char*)&sm, sizeof(MSG_STANDARD));
								  }

								  if (pUser[m->ID].Mode == USER_PLAY || pUser[m->ID].Mode == USER_SELCHAR)
								  {
									  if (m->Mode == 0)
										  SendClientMessage(m->ID, g_pMessageStringTable[_NN_Your_Account_From_Others]);
									  else if (m->Mode == 1)
										  SendClientMessage(m->ID, g_pMessageStringTable[_NN_Disabled_Account]);

									  pUser[conn].cSock.SendMessage();
								  }

								  CloseUser(m->ID);
		} break;
#pragma endregion
#pragma region _MSG_DBCNFAccountLogOut
		case _MSG_DBCNFAccountLogOut:
		{
										MSG_STANDARD *m = (MSG_STANDARD*)Msg;

										sprintf(temp, "etc,charlogout conn:%d name:%s", conn, pMob[conn].MOB.MobName);
										Log(temp, pUser[conn].AccountName, pUser[conn].IP);

										pMob[conn].Mode = MOB_EMPTY;
										pUser[conn].Mode = USER_ACCEPT;

										CloseUser(conn);
		} break;
#pragma endregion
#pragma region _MSG_DBCNFArchCharacterSucess
		case _MSG_DBCNFArchCharacterSucess:
		{
											  MSG_STANDARDPARM *m = (MSG_STANDARDPARM*)Msg;

											  SendClientMessage(conn, g_pMessageStringTable[_NN_My_King_Bless1]);

											  memset(&pMob[conn].MOB.Equip[10], 0, sizeof(STRUCT_ITEM));
											  SendItem(conn, ITEM_PLACE_EQUIP, 10, &pMob[conn].MOB.Equip[10]);

											  memset(&pMob[conn].MOB.Equip[11], 0, sizeof(STRUCT_ITEM));
											  SendItem(conn, ITEM_PLACE_EQUIP, 11, &pMob[conn].MOB.Equip[11]);

											  SaveUser(conn, 0);

											  CharLogOut(conn);

											  SendClientSignalParm(conn, ESCENE_FIELD, _MSG_SendArchEffect, m->Parm);

											  sprintf(temp, "etc,arch create name:%s slot:%d", pMob[conn].MOB.MobName, m->Parm);
											  Log(temp, pUser[conn].AccountName, pUser[conn].IP);
		} break;
#pragma endregion
#pragma region _MSG_DBCNFArchCharacterFail
		case _MSG_DBCNFArchCharacterFail:
		{
											pMob[conn].Mode = MOB_USER;
											SendClientMessage(conn, g_pMessageStringTable[_NN_NoEmptySlot]);
											break;
		} break;
#pragma endregion
#pragma region _MSG_DBSendDonate
		case _MSG_DBSendDonate:
		{
								  MSG_DBSendDonate *m = (MSG_DBSendDonate*)Msg;

								  if (pUser[conn].Mode < USER_SELCHAR)
								  {
									  // char ItemCode[256];

									  sprintf(temp, "tra,%s", pUser[conn].AccountName);
									  Log(temp, "_fail_play_", pUser[conn].IP);

									  break;
								  }

								  if (strcmp(pUser[conn].AccountName, m->Account) != 0)
								  {
									  // char ItemCode[256];

									  sprintf(temp, "tra,%s", pUser[conn].AccountName);
									  Log(temp, "_fail_name_", pUser[conn].IP);

									  break;
								  }
								  pUser[conn].Donate += m->Donate;

								  SendClientMessage(conn, g_pMessageStringTable[_NN_Cash_ChargeOk]);

								  m->Result = 0;

								  DBServerSocket.SendOneMessage((char*)m, m->Size);

								  sprintf(temp, "recv,%s %d donate", pUser[conn].AccountName, m->Donate);
								  Log(temp, "_web1_", pUser[conn].IP);

		} break;
#pragma endregion
#pragma region _MSG_CNFDBCapsuleInfo
		case _MSG_CNFDBCapsuleInfo:
		{
									  pUser[conn].cSock.SendOneMessage((char*)Msg, sizeof(MSG_CNFDBCapsuleInfo));
		} break;
#pragma endregion
#pragma region _MSG_DBCNFCapsuleCharacterFail
		case _MSG_DBCNFCapsuleCharacterFail:
		{
											   SendCarry(conn);
											   SendClientMessage(conn, g_pMessageStringTable[_NN_NoEmptySlot]);
											   break;
		} break;
#pragma endregion
#pragma region _MSG_DBCNFCapsuleCharacterFail2
		case _MSG_DBCNFCapsuleCharacterFail2:
		{
												SendCarry(conn);
												SendClientMessage(conn, g_pMessageStringTable[_NN_CANT_USE_ID]);
												break;
		} break;
#pragma endregion
#pragma region _MSG_DBCNFCapsuleSucess
		case _MSG_DBCNFCapsuleSucess:
		{
										memset(&pMob[conn].MOB, 0, sizeof(STRUCT_MOB));
										memset(&pMob[conn].extra, 0, sizeof(STRUCT_MOBEXTRA));
										SaveUser(conn, 1);
										break;
		} break;
#pragma endregion
#pragma region _MSG_GrindRankingData
		case _MSG_GrindRankingData:
		{
			/*
			Some change have happened in the ranking, notify the client.
			*/
			MSG_SendExpRanking *m = (MSG_SendExpRanking*)Msg;
			
			/* In the char sel Mode (0xC) the tmsrv still don't have the player data, so ignore the playerRank.Name check */
			if (pUser[conn].Mode == USER_PLAY && m->PlayerRank.Name[0] != '\0' && !strcmp(pMob[conn].MOB.MobName, m->PlayerRank.Name))
				pUser[conn].cSock.SendOneMessage((char*)m, sizeof(MSG_SendExpRanking));

		} break;
#pragma endregion
#pragma region _MSG_DBClientMessage
		case _MSG_DBClientMessage:
		{
			MSG_DBClientMessage *m = (MSG_DBClientMessage*)Msg;

			SendClientMessage(conn, m->String);
		} break;
#pragma endregion
#pragma region _MSG_DBServerSend1
		case _MSG_DBServerSend1:
		{
			MSG_STANDARDPARM *m = (MSG_STANDARDPARM*)Msg;

			m->ID = ESCENE_FIELD;

			pUser[conn].cSock.AddMessage((char*)m, sizeof(MSG_STANDARDPARM));
			pUser[conn].cSock.SendMessageA();
		} break;
#pragma endregion
#pragma region _MSG_DBCNFServerChange
		case _MSG_DBCNFServerChange:
		{
			MSG_DBCNFServerChange *m = (MSG_DBCNFServerChange*)Msg;

			pUser[conn].cSock.SendOneMessage((char*)m, sizeof(MSG_DBCNFServerChange));
			CloseUser(conn);
		} break;
#pragma endregion

		}
	}
#pragma endregion
}