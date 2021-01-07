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
#include "GetFunc.h"
#include "SendFunc.h"
#include "ProcessClientMessage.h"
#include "ProcessDBMessage.h"
#include "CReadFiles.h"

#include "CCastleZakum.h"

STRUCT_CASTLEQUEST CastleQuest[MAX_CASTLE_QUEST];

const char* const CCastleZakum::CASTLE_QUEST_PATH = "../../Common/Settings/CastleQuest.txt";

int CastleQuestPosition[MAX_CASTLE_POS][4] =
{
#pragma region Hidden
	{ 2221, 1270, 2259, 1246 }, //Primeiro bloco

	//Lado Esquerdo da quest
	{ 2217, 1212, 2234, 1235 },
	{ 2186, 1212, 2205, 1232 },
	{ 2192, 1171, 2212, 1193 },
	{ 2221, 1171, 2238, 1198 },

	//Lado Direito da quest
	{ 2245, 1212, 2261, 1234 },
	{ 2273, 1210, 2291, 1232 },
	{ 2268, 1171, 2287, 1193 },
	{ 2240, 1171, 2258, 1198 },
#pragma endregion
};

int CastleQuestClear = 0;

int CastleQuestTime = -1;
int CastleQuestParty[MAX_PARTY + 1] = { 0, 0, 0, 0, 0, 0, };
int CastleQuestLevel = -1;
char CastleLeader[NAME_LENGTH];

BOOL CCastleZakum::OpenCastleGate(int conn, int gateid, MSG_UpdateItem *m)
{
	int state = pItem[gateid].State;
	int gatestate2 = m->State;

	int gatekey = BASE_GetItemAbility(&pItem[gateid].ITEM, EF_KEYID);

	if (gatekey < 10 || gatekey > 14)
		return FALSE;

	int key = 0;
	int Quest = -1;

	int i = 0;
	for (i = 0; i < MAX_CARRY; i++)
	{
		key = BASE_GetItemAbility(&pMob[conn].MOB.Carry[i], EF_KEYID);
		Quest = BASE_GetItemAbility(&pMob[conn].MOB.Carry[i], EF_QUEST);

		if (key != gatekey || Quest != CastleQuestLevel && gatekey != 10)
			continue;

		break;
	}

	if (key != gatekey || Quest != CastleQuestLevel && gatekey != 10)
	{
		if (pItem[gateid].ITEM.sIndex != 773)
			SendClientMessage(conn, g_pMessageStringTable[_NN_No_Key]);

		return TRUE;
	}

#pragma region Portão de entrada
	if (gatekey == 10)
	{//Portão principal do castelo zakum onde define qual level será a quest.
		if (Quest < 0 || Quest >= MAX_CASTLE_QUEST)
			return TRUE;

		if (CastleQuestTime != -1)
		{
			int count = 0;

			for (int i = 0; i < MAX_PARTY; i++)
			{
				if (CastleQuestParty[i] == 0 || CastleQuestParty[i] >= MAX_USER)
					continue;

				count++;
			}

			sprintf(temp, g_pMessageStringTable[_SD_Zakum_Quest_by_S_N], CastleLeader, count);
			SendClientMessage(conn, temp);

			return TRUE;
		}

		//Deleta todos os mobs do mapa que sobraram em alguma outra quest.
		for (int x = 2180; x <= 2296; x++)
		{
			for (int y = 1160; y <= 1269; y++)
			{
				int tmob = pMobGrid[y][x];

				if (tmob < MAX_USER)
					continue;

				int generate = pMob[tmob].GenerateIndex;

				mNPCGen.pList[generate].MinuteGenerate = -1;
				DeleteMob(tmob, 3);
			}
		}

		//Cria todos os mobs para essa quest
		for (int x = CastleQuest[Quest].MOB_INITIAL; x <= CastleQuest[Quest].MOB_END; x++)
		{
			if (x != CastleQuest[Quest].BOSS[0] && x != CastleQuest[Quest].BOSS[1])
				mNPCGen.pList[x].MinuteGenerate = 1;

			GenerateMob(x, 0, 0);
			GenerateMob(x, 0, 0);
		}
		CastleQuestLevel = Quest;
		CastleQuestTime = CastleQuest[Quest].QuestTime-1;

		int Leader = pMob[conn].Leader;

		if (Leader == 0)
			Leader = conn;

		strncpy(CastleLeader, pMob[Leader].MOB.MobName, NAME_LENGTH);

		SendClientSignalParm(Leader, ESCENE_FIELD, _MSG_StartTime, CastleQuest[Quest].QuestTime);

		for (int i = 0; i < MAX_PARTY; i++)
		{
			int partyconn = pMob[Leader].PartyList[i];

			if (partyconn > 0 && partyconn < MAX_USER && partyconn != Leader && pUser[partyconn].Mode == USER_PLAY)
				SendClientSignalParm(partyconn, ESCENE_FIELD, _MSG_StartTime, CastleQuest[Quest].QuestTime);
		}

		for (int x = 0; x < MAX_PARTY + 1; x++)
		{
			if (x == MAX_PARTY)
			{
				CastleQuestParty[MAX_PARTY] = Leader;

				continue;
			}
			CastleQuestParty[x] = pMob[Leader].PartyList[x];
		}
	}
#pragma endregion
#pragma region Portões de dentro
	else if (gatekey >= 11 && gatekey <= 14)
	{
		if (Quest != CastleQuestLevel)
			return TRUE;
	}
#pragma endregion

	memset(&pMob[conn].MOB.Carry[i], 0, sizeof(STRUCT_ITEM));
	SendItem(conn, ITEM_PLACE_CARRY, i, &pMob[conn].MOB.Carry[i]);

	int heigth = 0;

	int isUpdate = UpdateItem(gateid, STATE_OPEN, &heigth);

	if (isUpdate)
		GridMulticast(pItem[gateid].PosX, pItem[gateid].PosY, (MSG_STANDARD*)m, 0);

	sprintf(temp, "etc,opengate gateid:%d X:%d Y:%d", gateid, pItem[gateid].PosX, pItem[gateid].PosY);
	Log(temp, pUser[conn].AccountName, pUser[conn].IP);
	return TRUE;
}

void CCastleZakum::MobKilled(int target, int conn, int PosX, int PosY)
{
	int GenerateID = pMob[target].GenerateIndex;

	for (int k = 0; k < MAX_CASTLE_QUEST; k++)
	{
		if (GenerateID == CastleQuest[k].BOSS[0] || GenerateID == CastleQuest[k].BOSS[1])
		{
			sprintf(temp, g_pMessageStringTable[_SN_CastleQuest_Killed], pMob[conn].MOB.MobName);
			SendNoticeArea(temp, 2176, 1160, 2300, 1276);
			CastleQuestClear = 1;

			//Entrega os itens, exp e gold para todos após matarem o boss.

			int partyleader = pMob[conn].Leader;

			if (partyleader <= 0)
				partyleader = conn;

			for (int j = 0; j < MAX_CARRY; j++)
			{
				if (CastleQuest[k].Prize[j].sIndex)
					PutItem(partyleader, &CastleQuest[k].Prize[j]);
			}
			if (CastleQuest[k].ExpPrize[pMob[partyleader].extra.ClassMaster])
			{
				pMob[partyleader].MOB.Exp += CastleQuest[k].ExpPrize[pMob[partyleader].extra.ClassMaster];

				sprintf(temp, g_pMessageStringTable[_NS_GETEXP], CastleQuest[k].ExpPrize[pMob[partyleader].extra.ClassMaster]);
				SendClientMessage(partyleader, temp);
			}

			if (CastleQuest[k].CoinPrize)
			{
				unsigned int Coin = pMob[partyleader].MOB.Coin + CastleQuest[k].CoinPrize;

				if (Coin <= 2000000000)
				{
					sprintf(temp, g_pMessageStringTable[_DN_receive_D_gold], CastleQuest[k].CoinPrize);
					SendClientMessage(partyleader, temp);

					pMob[partyleader].MOB.Coin = Coin;
					SendEtc(partyleader);
				}
				else
					SendClientMessage(partyleader, g_pMessageStringTable[_NN_Cant_get_more_than_2G]);
			}

			for (int i = 0; i < MAX_PARTY && CastleQuest[k].PartyPrize; i++)
			{
				int partymember = pMob[partyleader].PartyList[i];

				if (pUser[partymember].Mode != USER_PLAY)
					continue;

				for (int j = 0; j < MAX_CARRY; j++)
				{
					if (CastleQuest[k].Prize[j].sIndex)
						PutItem(partymember, &CastleQuest[k].Prize[j]);
				}

				if (CastleQuest[k].ExpPrize[pMob[partymember].extra.ClassMaster])
				{
					pMob[partymember].MOB.Exp += CastleQuest[k].ExpPrize[pMob[partymember].extra.ClassMaster];

					sprintf(temp, g_pMessageStringTable[_NS_GETEXP], CastleQuest[k].ExpPrize[pMob[partymember].extra.ClassMaster]);
					SendClientMessage(partymember, temp);
				}

				if (CastleQuest[k].CoinPrize)
				{
					unsigned int Coin = pMob[partymember].MOB.Coin + CastleQuest[k].CoinPrize;

					if (Coin <= 2000000000)
					{
						sprintf(temp, g_pMessageStringTable[_DN_receive_D_gold], CastleQuest[k].CoinPrize);
						SendClientMessage(partymember, temp);

						pMob[partymember].MOB.Coin = Coin;
						SendEtc(partymember);
					}
					else
						SendClientMessage(partymember, g_pMessageStringTable[_NN_Cant_get_more_than_2G]);
				}
			}
		}
	}
}

BOOL CCastleZakum::KeyDrop(int target, int conn, int PosX, int PosY, STRUCT_ITEM *Key)
{
	int ItemKey = BASE_GetItemAbility(Key, EF_KEYID);

	if (ItemKey >= 11 && ItemKey <= 14)
	{
		Key->stEffect[0].cEffect = EF_QUEST;
		Key->stEffect[0].cValue = CastleQuestLevel;

		PutItem(conn, Key);
		return FALSE;
	}
	return TRUE;
}

void CCastleZakum::ProcessSecTimer()
{
	if (SecCounter % 2 == 0)
	{
		if (CastleQuestTime == 0)
		{
			CastleQuestTime = -1;
			ClearArea(2180, 1160, 2296, 1269);

			//Deleta todos os mobs do mapa que sobraram em alguma outra quest.
			for (int x = 2180; x <= 2296; x++)
			{
				for (int y = 1160; y <= 1269; y++)
				{
					int tmob = pMobGrid[y][x];

					if (tmob < MAX_USER)
						continue;

					int generate = pMob[tmob].GenerateIndex;

					mNPCGen.pList[generate].MinuteGenerate = -1;
					DeleteMob(tmob, 3);
				}
			}
		}
		if (CastleQuestTime > 0)
			CastleQuestTime--;
	}
}

void CCastleZakum::ProcessMinTimer()
{
	if (CastleQuestClear == 1)
	{
		CastleQuestClear = 2;
		SendNoticeArea(g_pMessageStringTable[_NN_CastleQuest_Initialize], 2176, 1160, 2300, 1276);
	}
	else if (CastleQuestClear == 2)
	{
		CastleQuestClear = 0;

		ClearArea(2180, 1160, 2296, 1269);

		//Deleta todos os mobs do mapa que sobraram em alguma outra quest.
		for (int x = 2180; x <= 2296; x++)
		{
			for (int y = 1160; y <= 1269; y++)
			{
				int tmob = pMobGrid[y][x];

				if (tmob < MAX_USER)
					continue;

				int generate = pMob[tmob].GenerateIndex;

				mNPCGen.pList[generate].MinuteGenerate = -1;
				DeleteMob(tmob, 3);
			}
		}
	}
}

void CCastleZakum::CheckMove(int conn, int targetx, int targety)
{
	int i = MAX_PARTY + 1;

	for (i = 0; i < MAX_PARTY + 1; i++)
	{
		if (CastleQuestParty[i] == conn)
			break;
	}
	for (int k = 0; k < MAX_CASTLE_POS; k++)
	{
		if (i == MAX_PARTY + 1 && targetx >= CastleQuestPosition[k][0] && targety >= CastleQuestPosition[k][1] && targetx <= CastleQuestPosition[k][2] && targety <= CastleQuestPosition[k][3])
		{
			DoTeleport(conn, pMob[conn].TargetX, pMob[conn].TargetY);
			return;
		}
	}
}

void CCastleZakum::ReadCastleQuest()
{
	FILE *fs = fopen(CASTLE_QUEST_PATH, "rt");

	if (fs == NULL)
	{
		MessageBoxA(hWndMain, "CastleQuest.txt não foi encontrado.", CASTLE_QUEST_PATH, MB_OK);

		return;
	}

	int Num = -1;

	char tp[1024];

	memset(tp, 0, 1023);

	while (1)
	{
		char *getret = fgets(tp, 1024, fs);

		if (getret == NULL)
			break;


		if (tp[0] == 47) //'\ '
			continue;

		else if (tp[0] == '#')
		{
			if (Num >= MAX_CASTLE_QUEST)
				break;

			Num++;

			memset(&CastleQuest[Num], 0, sizeof(STRUCT_CASTLEQUEST));
		}

		else if (tp[0] != 13)
		{
			int pars = ParseCastleString(Num, tp);

			if (pars == 0)
				continue;
		}
	}
	fclose(fs);
}

int  CCastleZakum::ParseCastleString(int Num, char *str)
{
	char str1[128];
	str1[0] = 0;

	char str2[128];
	str2[0] = 0;

	char str3[128];
	str3[0] = 0;

	int  value = 0;
	int secondvalue = 0;

	sscanf(str, "%s %s %s", str1, str2, str3);

	if (str[0] == 47)
		return FALSE;

	if (str[0] == 0)
		return FALSE;

	STRUCT_ITEM Item;
	memset(&Item, 0, sizeof(STRUCT_ITEM));

	char item_str1[256];
	char item_str2[256];
	char item_str3[256];
	char item_str4[256];
	char item_str5[256];
	char item_str6[256];
	char item_str7[256];

	int item_value[7] = { 0, 0, 0, 0, 0, 0, 0 };

	sscanf(str, "%s %s %s %s %s %s %s %s", str1, item_str1, item_str2, item_str3, item_str4, item_str5, item_str6, item_str7);
	item_value[0] = atoi(item_str1);
	item_value[1] = atoi(item_str2);
	item_value[2] = atoi(item_str3);
	item_value[3] = atoi(item_str4);
	item_value[4] = atoi(item_str5);
	item_value[5] = atoi(item_str6);
	item_value[6] = atoi(item_str7);

	value = atoi(str2);
	secondvalue = atoi(str3);

	if (strlen(str2) > 80)
		return FALSE;

	_strupr(str1);
	_strupr(str2);

	if (!strcmp(str1, "MOB_INITIAL:"))
		CastleQuest[Num].MOB_INITIAL = value;

	else if (!strcmp(str1, "MOB_END:"))
		CastleQuest[Num].MOB_END = value;

	else if (!strcmp(str1, "BOSS1:"))
		CastleQuest[Num].BOSS[0] = value;

	else if (!strcmp(str1, "BOSS2:"))
		CastleQuest[Num].BOSS[1] = value;

	else if (!strcmp(str1, "COINPRIZE:"))
		CastleQuest[Num].CoinPrize = value;

	else if (!strcmp(str1, "EXPPRIZE_ARCH:"))
		CastleQuest[Num].ExpPrize[1] = value;

	else if (!strcmp(str1, "EXPPRIZE_MORTAL:"))
		CastleQuest[Num].ExpPrize[2] = value;

	else if (!strcmp(str1, "EXPPRIZE_CELESTIAL:"))
	{
		CastleQuest[Num].ExpPrize[3] = value;
		CastleQuest[Num].ExpPrize[4] = value;
	}

	else if (!strcmp(str1, "EXPPRIZE_SUBCELESTIAL:"))
		CastleQuest[Num].ExpPrize[2] = value;

	else if (!strcmp(str1, "PARTYPRIZE:"))
		CastleQuest[Num].PartyPrize = strcmp(str2, "ON") == 0 ? 1 : 0;

	else if (!strcmp(str1, "QUESTTIME:"))
		CastleQuest[Num].QuestTime = value;

	else
	{
		for (int i = 0; i < MAX_CARRY; i++)
		{
			sprintf(temp, "PRIZE_%d:", i);

			if (!strcmp(str1, temp))
			{
				CastleQuest[Num].Prize[i].sIndex = item_value[0];
				CastleQuest[Num].Prize[i].stEffect[0].cEffect = item_value[1];
				CastleQuest[Num].Prize[i].stEffect[0].cValue = item_value[2];
				CastleQuest[Num].Prize[i].stEffect[1].cEffect = item_value[3];
				CastleQuest[Num].Prize[i].stEffect[1].cValue = item_value[4];
				CastleQuest[Num].Prize[i].stEffect[2].cEffect = item_value[5];
				CastleQuest[Num].Prize[i].stEffect[2].cValue = item_value[6];

				return TRUE;
			}
		}
	}
	return TRUE;
}