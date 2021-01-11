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
#include <windows.h>

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
#include "CCastleZakum.h"

void SaveAll()
{
	for(int i = 0; i < MAX_USER; i++)
	{
		if(pUser[i].Mode == USER_PLAY )
		{
			CharLogOut(i);
			Log("saveall", pUser[i].AccountName, pUser[i].IP);
		}
	}

}

void ProcessImple(int conn, int level, char * str)
{
	char cmd[128];
	char sval1[128];
	char sval2[128];
	char sval3[128];
	char sval4[128];
	char sval5[128];
	char sval6[128];
	char sval7[128];
	char sval8[128];

	unsigned int  ival1 = 0;
	unsigned int  ival2 = 0;
	long long  dval2 = 0;
	unsigned int  ival3 = 0;
	unsigned int  ival4 = 0;
	unsigned int  ival5 = 0;
	unsigned int  ival6 = 0;
	unsigned int  ival7 = 0;
	unsigned int  ival8 = 0;

	sscanf(str + 1, "%s %s %s %s %s %s %s %s %s", cmd, sval1, sval2, sval3, sval4, sval5, sval6, sval7, sval8);

	sscanf(sval1, "%d", &ival1);
	sscanf(sval2, "%llu", &dval2);
	sscanf(sval2, "%d", &ival2);
	sscanf(sval3, "%d", &ival3);
	sscanf(sval4, "%d", &ival4);
	sscanf(sval5, "%d", &ival5);
	sscanf(sval6, "%d", &ival6);
	sscanf(sval7, "%d", &ival7);
	sscanf(sval8, "%d", &ival8);

	char logtemp[256];
	sprintf(logtemp, "adm %s", str);

	if (conn != 0)
		Log(logtemp, pUser[conn].AccountName, pUser[conn].IP);

	else
		Log(logtemp, "system", 0);

	if (level >= 8)
	{
		if (conn <= 0 && conn >= MAX_USER)
			return;

		if (!strcmp(cmd, "set"))
		{
			if (!strcmp(sval1, "sanc"))
			{
				if (pMob[conn].MOB.Equip[0].sIndex != 0) { pMob[conn].MOB.Equip[0].stEffect[0].cEffect = EF_SANC; pMob[conn].MOB.Equip[0].stEffect[0].cValue = ival2; }
				if (pMob[conn].MOB.Equip[1].sIndex != 0) { pMob[conn].MOB.Equip[1].stEffect[0].cEffect = EF_SANC; pMob[conn].MOB.Equip[1].stEffect[0].cValue = ival2; }
				if (pMob[conn].MOB.Equip[2].sIndex != 0) { pMob[conn].MOB.Equip[2].stEffect[0].cEffect = EF_SANC; pMob[conn].MOB.Equip[2].stEffect[0].cValue = ival2; }
				if (pMob[conn].MOB.Equip[3].sIndex != 0) { pMob[conn].MOB.Equip[3].stEffect[0].cEffect = EF_SANC; pMob[conn].MOB.Equip[3].stEffect[0].cValue = ival2; }
				if (pMob[conn].MOB.Equip[4].sIndex != 0) { pMob[conn].MOB.Equip[4].stEffect[0].cEffect = EF_SANC; pMob[conn].MOB.Equip[4].stEffect[0].cValue = ival2; }
				if (pMob[conn].MOB.Equip[5].sIndex != 0) { pMob[conn].MOB.Equip[5].stEffect[0].cEffect = EF_SANC; pMob[conn].MOB.Equip[5].stEffect[0].cValue = ival2; }
				if (pMob[conn].MOB.Equip[6].sIndex != 0) { pMob[conn].MOB.Equip[6].stEffect[0].cEffect = EF_SANC; pMob[conn].MOB.Equip[6].stEffect[0].cValue = ival2; }
				if (pMob[conn].MOB.Equip[7].sIndex != 0) { pMob[conn].MOB.Equip[7].stEffect[0].cEffect = EF_SANC; pMob[conn].MOB.Equip[7].stEffect[0].cValue = ival2; }
				SendClientMessage(conn, "set sanc");
			}
			else if (!strcmp(sval1, "buildhtml"))
			{
				BuildList();
				SendClientMessage(conn, "html built");
			}
			else if (!strcmp(sval1, "clearquest"))
			{
				pMob[conn].MOB.Quest = 0;
				SendClientMessage(conn, "set clearquest");
			}
			else if (!strcmp(sval1, "clearskill"))
			{
				pMob[conn].MOB.LearnedSkill = 0;
				SendClientMessage(conn, "set clearskill");
			}
			else if (!strcmp(sval1, "level"))
			{
				if (ival2>2010)
					ival2 = 2010;

				pMob[conn].MOB.BaseScore.Level = ival2;
				SendClientMessage(conn, "set level");
			}
			else if (!strcmp(sval1, "hp"))
			{
				pMob[conn].MOB.BaseScore.MaxHp = pMob[conn].MOB.BaseScore.Hp = ival2;
				pMob[conn].MOB.CurrentScore.MaxHp = pMob[conn].MOB.CurrentScore.Hp = ival2;
				SendClientMessage(conn, "set hp");
			}
			else if (!strcmp(sval1, "mp"))
			{
				pMob[conn].MOB.BaseScore.MaxMp = pMob[conn].MOB.BaseScore.Mp = ival2;
				pMob[conn].MOB.CurrentScore.MaxMp = pMob[conn].MOB.CurrentScore.Mp = ival2;
				SendClientMessage(conn, "set mp");
			}
			else if (!strcmp(sval1, "ac"))
			{
				pMob[conn].MOB.BaseScore.Ac = ival2;
				SendClientMessage(conn, "set ac");
			}
			else if (!strcmp(sval1, "dam"))
			{
				pMob[conn].MOB.BaseScore.Damage = ival2;
				SendClientMessage(conn, "set dam");
			}
			else if (!strcmp(sval1, "exp"))
			{
				pMob[conn].MOB.Exp = dval2;
				SendClientMessage(conn, "set exp");
				SendEtc(conn);
				pMob[conn].CheckGetLevel();
			}
			else if (!strcmp(sval1, "str"))
			{
				pMob[conn].MOB.BaseScore.Str = ival2;
				SendClientMessage(conn, "set str");
			}
			else if (!strcmp(sval1, "int"))
			{
				pMob[conn].MOB.BaseScore.Int = ival2;
				SendClientMessage(conn, "set int");
			}
			else if (!strcmp(sval1, "dex"))
			{
				pMob[conn].MOB.BaseScore.Dex = ival2;
				SendClientMessage(conn, "set dex");
			}
			else if (!strcmp(sval1, "con"))
			{
				pMob[conn].MOB.BaseScore.Con = ival2;
				SendClientMessage(conn, "set con");
			}
			else if (!strcmp(sval1, "coin"))
			{
				pMob[conn].MOB.Coin = ival2;
				SendClientMessage(conn, "set coin");
			}
			else if (!strcmp(sval1, "merchant"))
			{
				pMob[conn].MOB.Merchant = ival2;
				SendClientMessage(conn, "set merchant");
			}
			else if (!strcmp(sval1, "skillbonus"))
			{
				pMob[conn].MOB.SkillBonus = ival2;
				SendClientMessage(conn, "set skillbonus");
			}
			else if (!strcmp(sval1, "special0"))
			{
				pMob[conn].MOB.BaseScore.Special[0] = ival2;
				SendClientMessage(conn, "set special 0");
			}
			else if (!strcmp(sval1, "special1"))
			{
				pMob[conn].MOB.BaseScore.Special[1] = ival2;
				SendClientMessage(conn, "set special 1");
			}
			else if (!strcmp(sval1, "special2"))
			{
				pMob[conn].MOB.BaseScore.Special[2] = ival2;
				SendClientMessage(conn, "set special 2");
			}
			else if (!strcmp(sval1, "special3"))
			{
				pMob[conn].MOB.BaseScore.Special[3] = ival2;
				SendClientMessage(conn, "set special 3");
			}
			else if (!strcmp(sval1, "scorebonus"))
			{
				pMob[conn].MOB.ScoreBonus = ival2;
				SendClientMessage(conn, "set scorebonus");
			}
			else if (!strcmp(sval1, "attackrun"))
			{
				pMob[conn].MOB.BaseScore.AttackRun = ival2;
				SendClientMessage(conn, "set AttackRun");
			}
			else if (!strcmp(sval1, "critical"))
			{
				pMob[conn].MOB.Critical = ival2;
				SendClientMessage(conn, "set Critical");
			}
			else if(!strcmp(sval1, "learned"))
			{
				int skillpos = ival2 % 24;
				int learn = 1 << skillpos;

				sprintf(temp, "%d", learn);
				SendClientMessage(conn, temp);
			}

			else if (!strcmp(sval1, "special"))
			{
				pMob[conn].MOB.SpecialBonus = ival2;
				SendClientMessage(conn, "set Special");
			}
			else if (!strcmp(sval1, "classmaster"))
			{
				pMob[conn].extra.ClassMaster = ival2;
				SendClientMessage(conn, "set classmaster");
			}

			else if(!strcmp(sval1, "reloadfile"))
			{
				BASE_InitializeBaseDef();
				BASE_ReadGuardFile();
				BASE_ReadQuestDiaria();
				BASE_InitializeMessage("Language.txt");
				ReadLevelItemConfig();
				CReadFiles::ReadSancRate();
				CReadFiles::ReadQuestsRate();
				CReadFiles::ReadCompRate();
				//CEncampment::ReadCamp();
				CCastleZakum::ReadCastleQuest();

				SendClientMessage(conn, "SET RELOAD FILE");
			}

			else if (!strcmp(sval1, "gate"))
			{
				STRUCT_ITEM item;

				memset(&item, 0, sizeof(STRUCT_ITEM));

				item.sIndex = ival2;

				int gateid = CreateItem(ival3, ival4, &item, ival5, 1);

				if (gateid >= MAX_ITEM || gateid <= 0)
					return;

				MSG_CreateItem sm_cig;
				memset(&sm_cig, 0, sizeof(MSG_CreateItem));

				sm_cig.Type = _MSG_CreateItem;
				sm_cig.Size = sizeof(MSG_CreateItem);
				sm_cig.ID = ESCENE_FIELD;

				sm_cig.ItemID = ival2 + 10000;

				memcpy(&sm_cig.Item, &pItem[gateid].ITEM, sizeof(STRUCT_ITEM));

				sm_cig.GridX = ival3;
				sm_cig.GridY = ival4;

				sm_cig.Rotate = ival5;
				sm_cig.State = pItem[gateid].State;

				GridMulticast(ival3, ival4, (MSG_STANDARD*)&sm_cig, 0);

				SendClientMessage(conn, "CREATE GATE");
			}

			else if (!strcmp(sval1, "destroygate"))
			{
				MSG_CreateItem sm_cig;
				memset(&sm_cig, 0, sizeof(MSG_CreateItem));

				GetCreateItem(ival2, &sm_cig);

				GridMulticast(pItem[ival2].PosX, pItem[ival2].PosY, (MSG_STANDARD*)&sm_cig, 0);

				memset(&pItem[ival2], 0, sizeof(CItem));
				SendClientMessage(conn, "DESTROY GATE");
			}

			else if (!strcmp(sval1, "cgate"))
			{
				pItem[ival2].Delay = 0;
				UpdateItem(ival2, STATE_LOCKED, &pItem[ival2].Height);

				MSG_CreateItem sm_cig;
				memset(&sm_cig, 0, sizeof(MSG_CreateItem));

				GetCreateItem(ival2, &sm_cig);

				GridMulticast(pItem[ival2].PosX, pItem[ival2].PosY, (MSG_STANDARD*)&sm_cig, 0);

				SendClientMessage(conn, "CLOSED GATE");
			}

			else if (!strcmp(sval1, "closearmia"))
			{
				int i = 0;

				for(i = 0; i < MAX_ITEM; i++)
				{
					if(pItem[i].ITEM.sIndex == 4143)
						break;
				}
				if(i == MAX_ITEM)
					return;

				UpdateItem(i, STATE_LOCKED, &pItem[i].Height);

				MSG_CreateItem sm_cig;
				memset(&sm_cig, 0, sizeof(MSG_CreateItem));

				GetCreateItem(i, &sm_cig);

				GridMulticast(pItem[i].PosX, pItem[i].PosY, (MSG_STANDARD*)&sm_cig, 0);

				SendClientMessage(conn, "CLOSED GATE");
			}

			else if (!strcmp(sval1, "openarmia"))
			{
				int i = 0;

				for(i = 0; i < MAX_ITEM; i++)
				{
					if(pItem[i].ITEM.sIndex == 4143)
						break;
				}
				if(i == MAX_ITEM)
					return;

				UpdateItem(i, STATE_OPEN, &pItem[i].Height);

				MSG_CreateItem sm_cig;
				memset(&sm_cig, 0, sizeof(MSG_CreateItem));

				GetCreateItem(i, &sm_cig);

				GridMulticast(pItem[i].PosX, pItem[i].PosY, (MSG_STANDARD*)&sm_cig, 0);

				SendClientMessage(conn, "OPEN GATE");
			}

			else if (!strcmp(sval1, "ogate"))
			{
				pItem[ival2].Delay = 1;
				UpdateItem(ival2, STATE_OPEN, &pItem[ival2].Height);
				
				MSG_UpdateItem sm_upi;
				memset(&sm_upi, 0, sizeof(MSG_UpdateItem));

				sm_upi.ID = ESCENE_FIELD;
				sm_upi.Type = _MSG_UpdateItem;

				sm_upi.ItemID = ival2 + 10000;

				sm_upi.Size = sizeof(MSG_UpdateItem);

				sm_upi.State = pItem[ival2].State;

				GridMulticast(pItem[ival2].PosX, pItem[ival2].PosY, (MSG_STANDARD*)&sm_upi, 0);

				SendClientMessage(conn, "OPEN GATE");
			}

			else if (!strcmp(sval1, "ngate"))
			{
				pItem[ival2].Delay = 0;
				UpdateItem(ival2, STATE_CLOSED, &pItem[ival2].Height);

				MSG_CreateItem sm_cig;
				memset(&sm_cig, 0, sizeof(MSG_CreateItem));

				GetCreateItem(ival2, &sm_cig);

				GridMulticast(pItem[ival2].PosX, pItem[ival2].PosY, (MSG_STANDARD*)&sm_cig, 0);

				SendClientMessage(conn, "NOTHING GATE");
			}

			else if (!strcmp(sval1, "mgate"))
			{
				pItem[45].Delay = 0;
				UpdateItem(45, STATE_CLOSED, &pItem[45].Height);

				pItem[45].PosX = ival2;
				pItem[45].PosY = ival3;

				MSG_CreateItem sm_cig;
				memset(&sm_cig, 0, sizeof(MSG_CreateItem));

				GetCreateItem(45, &sm_cig);

				GridMulticast(pItem[45].PosX, pItem[45].PosY, (MSG_STANDARD*)&sm_cig, 0);

				SendClientMessage(conn, "MOVE GATE");
			}

			else if(!strcmp(sval1, "clearcarry"))
			{
				for(int i = 0; i < MAX_CARRY-4; i++)
				{
					BASE_ClearItem(&pMob[conn].MOB.Carry[i]);
					SendItem(conn, ITEM_PLACE_CARRY, i, &pMob[conn].MOB.Carry[i]);
				}

				SendClientMessage(conn, "SET CLEAR CARRY");
			}

			else if (!strcmp(sval1, "item"))
			{
				STRUCT_ITEM Item;
				memset(&Item, 0, sizeof(STRUCT_ITEM));
				
				Item.sIndex = ival2;
				
				if (ival2 >= 2330 && ival2<2390)
				{
					Item.stEffect[0].sValue = 2000;
					Item.stEffect[1].cEffect = 120;
					Item.stEffect[1].cValue = 60;
				}

				Item.stEffect[0].cEffect = ival3;
				Item.stEffect[0].cValue = ival4;
				Item.stEffect[1].cEffect = ival5;
				Item.stEffect[1].cValue = ival6;
				Item.stEffect[2].cEffect = ival7;
				Item.stEffect[2].cValue = ival8;

				PutItem(conn, &Item);

				SendClientMessage(conn, "SET ITEM CARRY");

				sprintf(temp, "%s - Set Item Carry [%d %d:%d:%d:%d:%d:%d]", pMob[conn].MOB.MobName, Item.sIndex, Item.stEffect[0].cEffect, Item.stEffect[0].cValue, Item.stEffect[1].cEffect, Item.stEffect[1].cValue, Item.stEffect[2].cEffect, Item.stEffect[2].cValue);
				Log("-system", temp, pUser[conn].IP);
			}

			else if(!strcmp(sval1, "noatum"))
			{
				CastleServer = 1;
				CastleState = 1;

				SendClientMessage(conn, "SET NOATUM WAR");
			}

	
			if (ival1 >= 1 && ival1 <= MAX_EQUIP)
			{
				memset(&(pMob[conn].MOB.Equip[ival1 - 1]), 0, sizeof(STRUCT_ITEM));
				pMob[conn].MOB.Equip[ival1 - 1].sIndex = ival2;
				if (ival2 >= 2330 && ival2<2390)
				{
					pMob[conn].MOB.Equip[ival1 - 1].stEffect[0].sValue = 2000;
					pMob[conn].MOB.Equip[ival1 - 1].stEffect[1].cEffect = 120;
					pMob[conn].MOB.Equip[ival1 - 1].stEffect[1].cValue = 60;
				}

				pMob[conn].MOB.Equip[ival1 - 1].stEffect[0].cEffect = ival3;
				pMob[conn].MOB.Equip[ival1 - 1].stEffect[0].cValue = ival4;
				pMob[conn].MOB.Equip[ival1 - 1].stEffect[1].cEffect = ival5;
				pMob[conn].MOB.Equip[ival1 - 1].stEffect[1].cValue = ival6;
				pMob[conn].MOB.Equip[ival1 - 1].stEffect[2].cEffect = ival7;
				pMob[conn].MOB.Equip[ival1 - 1].stEffect[2].cValue = ival8;

				sprintf(temp, "%s - Set Item Equip [%d %d:%d:%d:%d:%d:%d]", pMob[conn].MOB.MobName, pMob[conn].MOB.Equip[ival1 - 1].sIndex, pMob[conn].MOB.Equip[ival1 - 1].stEffect[0].cEffect, pMob[conn].MOB.Equip[ival1 - 1].stEffect[0].cValue, pMob[conn].MOB.Equip[ival1 - 1].stEffect[1].cEffect, pMob[conn].MOB.Equip[ival1 - 1].stEffect[1].cValue, pMob[conn].MOB.Equip[ival1 - 1].stEffect[2].cEffect, pMob[conn].MOB.Equip[ival1 - 1].stEffect[2].cValue);
				Log("-system", temp, pUser[conn].IP);
			}
			pMob[conn].GetCurrentScore(conn);
			SendEquip(conn, conn);
			SendScore(conn);
			SendEtc(conn);

			for (int j = 0; j < MAX_EQUIP; j++)
				SendItem(conn, 0, j, &pMob[conn].MOB.Equip[j]);
		}
		else if (!strcmp(cmd, "save") && conn != 0)
		{
			strcpy(temp, "./npc/");

			pMob[conn].GetCurrentScore(conn);

			strcat(temp, sval1);

			int handle = _open(temp, _O_CREAT | _O_RDWR | _O_BINARY, _S_IREAD | _S_IWRITE);
			if (handle == -1)
			{
				SendClientMessage(conn, "fail - save file");
				return;
			}
			_write(handle, (void*)&pMob[conn].MOB, sizeof(pMob[conn].MOB));
			_close(handle);
			sprintf(temp, "npc: %s saved", sval1);
			SendClientMessage(conn, temp);
		}
		else if (!strcmp(cmd, "read") && conn != 0)
		{
			strcpy(temp, "./npc/");

			strcat(temp, sval1);

			int handle = _open(temp, _O_RDONLY | _O_BINARY, NULL);
			if (handle == -1)
			{
				SendClientMessage(conn, "fail - open file");
				return;
			}

			_read(handle, &pMob[conn].MOB, sizeof(pMob[conn].MOB));
			_close(handle);

			sprintf(temp, "npc: %s read.", sval1);
			SendClientMessage(conn, temp);

			for (int j = 0; j < MAX_EQUIP; j++)
				SendItem(conn, 0, j, &pMob[conn].MOB.Equip[j]);
			
			strcpy(pMob[conn].MOB.MobName, sval1);

			pMob[conn].MOB.BaseScore = pMob[conn].MOB.CurrentScore;
			pMob[conn].GetCurrentScore(conn);
			SendScore(conn);
			SendEtc(conn);
		}
		else if (!strcmp(cmd, "name") && conn != 0)
		{
			strcpy(pMob[conn].MOB.MobName, sval1);

			MSG_CreateMob sm_cmn;
			memset(&sm_cmn, 0, sizeof(MSG_CreateMob));
			GetCreateMob(conn, &sm_cmn);

			GridMulticast(pMob[conn].TargetX, pMob[conn].TargetY, (MSG_STANDARD*)&sm_cmn, 0);

			SendClientMessage(conn, "set name");
		}
	}
	if (level >= 3)
	{
		if (!strcmp(cmd, "timer"))
		{
			if (ival1 < 5000)
			{
				SendClientMessage(conn, "SET TIMER can't be less than 5 sec");
				return;
			}
			SetTimer(hWndMain, TIMER_MIN, ival1, NULL);
			SendClientMessage(conn, "SET TIMER");
		}
		else if (!strcmp(cmd, "saveall"))
		{
			SaveAll();
			SendClientMessage(conn, "SAVEALL");
		}
		else if (!strcmp(cmd, "decay"))
		{
			//DecayValue = ival1;
			SendClientMessage(conn, "set decay");
		}
		else if (!strcmp(cmd, "set"))
		{
			if (!strcmp(sval1, "billmode"))
			{
				if (ival2 == 2 || ival2 == 3)
				{
					if (BillServerSocket.Sock == NULL)
					{
						SendClientMessage(conn, "not connected to billing server.+billconnect first");
						return;
					}
				}
				BILLING = ival2;
				SendClientMessage(conn, "set billing");
				DrawConfig(1);
			}
			else if (!strcmp(sval1, "billfree"))
			{
				FREEEXP = ival2;
				SendClientMessage(conn, "SET FREE LEVEL");
				DrawConfig(1);
			}
			else if (!strcmp(sval1, "charselbill"))
			{
				CHARSELBILL = ival2;
				SendClientMessage(conn, "SET CHARSELL BILL");
				DrawConfig(1);
			}
			else if (!strcmp(sval1, "potioncount"))
			{
				POTIONCOUNT = ival2;
				SendClientMessage(conn, "SET POTIONCOUNT BILL");
				DrawConfig(1);
			}
			else if (!strcmp(sval1, "potiondelay"))
			{
				PotionDelay = ival2;
				SendClientMessage(conn, "SET POTIONDELAY");
				DrawConfig(1);
			}
			else if (!strcmp(sval1, "partybonus"))
			{
				PARTYBONUS = ival2;
				SendClientMessage(conn, "SET PARTYBONUS");
				DrawConfig(1);
			}
			else if (!strcmp(sval1, "guildboard"))
			{
				GUILDBOARD = ival2;
				SendClientMessage(conn, "SET PARTYBONUS");
				DrawConfig(1);
			}

		}
	}

	if (level >= 2)
	{
		if (!strcmp(cmd, "reboot"))
		{
			ServerDown = 1;
		}
		else if (!strcmp(cmd, "delayreboot"))
		{
			ServerDown = -1 * ServerIndex * (ival1 * 2);

			if (ServerDown > 1 || ServerDown<-1000)
				ServerDown = 1;
		}
		else if (!strcmp(cmd, "generate"))
		{
			if (ival1 <= 0 || ival1 > (unsigned int)mNPCGen.NumList)
			{
				SendClientMessage(conn, "generation index wrong");
				return;
			}
			if (conn <= 0 || conn >= MAX_USER)
				return;

			GenerateMob(ival1, pMob[conn].TargetX, pMob[conn].TargetY);
		}
		else if (!strcmp(cmd, "create"))
		{
			int ret = CreateMob(sval1, pMob[conn].TargetX, pMob[conn].TargetY, "boss", 0);

			if (ret == 1)
				SendClientMessage(conn, "Create mob success");
			if (ret == 0)
				SendClientMessage(conn, "No monster file in boss directory");
			if (ret == -1)
				SendClientMessage(conn, "No empty mob");
			if (ret == -2)
				SendClientMessage(conn, "No empty mob grid");
		}

		else if (!strcmp(cmd, "gift"))
		{
			STRUCT_ITEM Item;
			memset(&Item, 0, sizeof(STRUCT_ITEM));
				
			int i = 0;

			for(i = 0; i < NAME_LENGTH; i++)
			{
				if(sval1[i] == '_')
					sval1[i] = 32;
			}

			for(i = 1; i < MAX_MOB; i++)
			{
				if(strcmp(pMob[i].MOB.MobName, sval1) == 0)
					break;
			}

			if(i == MAX_MOB)
			{
				SendClientMessage(conn, "MOB not found");
				return;
			}

			Item.sIndex = ival2;
				
			if (ival2 >= 2330 && ival2<2390)
			{
				Item.stEffect[0].sValue = 2000;
				Item.stEffect[1].cEffect = 120;
				Item.stEffect[1].cValue = 60;
			}

			Item.stEffect[0].cEffect = ival3;
			Item.stEffect[0].cValue = ival4;
			Item.stEffect[1].cEffect = ival5;
			Item.stEffect[1].cValue = ival6;
			Item.stEffect[2].cEffect = ival7;
			Item.stEffect[2].cValue = ival8;

			if(i >= MAX_USER)
			{
				int pos = 0;

				for (pos = 0; pos < MAX_SHOPLIST; pos++)
				{
					int invpos = (pos % 9) + ((pos / 9) * MAX_SHOPLIST);

					if (pMob[i].MOB.Carry[invpos].sIndex == 0)
					{
						pos = invpos;
						break;
					}
				}

				if (pos >= 0 && pos < MAX_CARRY)
				{
					memcpy(&pMob[i].MOB.Carry[pos], &Item, sizeof(STRUCT_ITEM));

					SendItem(i, ITEM_PLACE_CARRY, pos, &pMob[i].MOB.Carry[pos]);
				}
				else
				{
					SendClientMessage(conn, g_pMessageStringTable[_NN_You_Have_No_Space_To_Trade]);
					return;
				}
			}

			else
				PutItem(i, &Item);

			sprintf(temp, "%s - Set NPCGift [%d %d:%d:%d:%d:%d:%d]", pMob[conn].MOB.MobName, Item.sIndex, Item.stEffect[0].cEffect, Item.stEffect[0].cValue, Item.stEffect[1].cEffect, Item.stEffect[1].cValue, Item.stEffect[2].cEffect, Item.stEffect[2].cValue);
			Log("-system", temp, pUser[conn].IP);

			SendClientMessage(conn, "SET NPCGIFT");
		}

		else if (!strcmp(cmd, "cp"))
		{
			SetPKPoint(conn, ival1);

			MSG_CreateMob sm_cp;
			memset(&sm_cp, 0, sizeof(MSG_CreateMob));
			GetCreateMob(conn, &sm_cp);

			GridMulticast(pMob[conn].TargetX, pMob[conn].TargetY, (MSG_STANDARD*)&sm_cp, 0);

			SendClientMessage(conn, "CP");
		}

		else if (!strcmp(cmd, "frag"))
		{
			SetTotKill(conn, ival1);

			MSG_CreateMob sm_cp;
			memset(&sm_cp, 0, sizeof(MSG_CreateMob));
			GetCreateMob(conn, &sm_cp);

			GridMulticast(pMob[conn].TargetX, pMob[conn].TargetY, (MSG_STANDARD*)&sm_cp, 0);

			SendClientMessage(conn, "FRAG");
		}



		else if (!strcmp(cmd, "log"))
		{
			StartLog();
			StartChatLog();
			StartItemLog();

			SendClientMessage(conn, "RESTART LOG");
		}
		else if (!strcmp(cmd, "guildday"))
		{
			if (ival1 >= 0 && ival1 <= 6)
			{
				GuildDay = ival1;
				SendClientMessage(conn, "SET GUILD BATTLE DAY");
			}
		}
		else if (!strcmp(cmd, "guildhour"))
		{
			if (ival1 >= 0 && ival1 <= 23)
			{
				GuildHour = ival1;
				SendClientMessage(conn, "SET GUILD BATTLE HOUR");
			}
		}
		else if (!strcmp(cmd, "newbiehour"))
		{
			if (ival1 >= 0 && ival1 <= 23)
			{
				NewbieHour = ival1;
				SendClientMessage(conn, "SET NEWBIE BATTLE HOUR");
			}
		}
		else if (!strcmp(cmd, "weekmode"))
		{
			if (ival1 >= 0 && ival1 <= 5)
			{
				ForceWeekMode = ival1;
				WeekMode = ival1 - 1;

				if (WeekMode == -1)
					WeekMode = 5;
				SendClientMessage(conn, "FORCE GUILD MODE");
				return;
			}
			ForceWeekMode = -1;
			SendClientMessage(conn, "WEEKLY GUILD MODE");
		}
		else if (!strcmp(cmd, "reloadnpc"))
		{
			mNPCGen.ReadNPCGenerator();
			SendClientMessage(conn, "RELOADNPC");
		}
		else if (!strcmp(cmd, "reloadguild"))
		{
			CReadFiles::ReadGuild();
			SendClientMessage(conn, "RELOADGUILD");
		}
		else if (!strcmp(cmd, "readguildname"))
		{
			BASE_InitializeGuildName();
			SendClientMessage(conn, "READ GUILD NAME");
		}
		else if (!strcmp(cmd, "trtarget"))
		{
			if (ival1<1 || ival1>8)
			{
				SendClientMessage(conn, "err: +treasure (1-8) (1-5) (item)");
				return;
			}
			if (ival2 < 1 || ival2 > 5)
			{
				SendClientMessage(conn, "err: +treasure (1-8) (1-5) (item)");
				return;
			}
			int idx = ival1 - 1;
			int tgt = ival2 - 1;
			int iv1 = 0;
			int iv2 = 0;
			int iv3 = 0;
			int iv4 = 0;
			int iv5 = 0;

			sscanf(str + 1, "%s %s %s %d %d %d %d %d", cmd, sval1, sval2, &iv1, &iv2, &iv3, &iv4, &iv5);

			g_pTreasure[idx].Target[tgt].sIndex = iv1;
			g_pTreasure[idx].Target[tgt].stEffect[0].cEffect = iv2;
			g_pTreasure[idx].Target[tgt].stEffect[0].cValue = iv3;
			g_pTreasure[idx].Target[tgt].stEffect[1].cEffect = iv4;
			g_pTreasure[idx].Target[tgt].stEffect[1].cValue = iv5;
			g_pTreasure[idx].Target[tgt].stEffect[2].cEffect = 0;
			g_pTreasure[idx].Target[tgt].stEffect[2].cValue = 0;

			DrawConfig(1);
			SendClientMessage(conn, "+troktarget success");
		}
		else if (!strcmp(cmd, "trsource"))
		{
			if (ival1 < 1 || ival1 > 8)
			{
				SendClientMessage(conn, "err: +source (1-8) (item)");
				return;
			}
			int idx = ival1 - 1;
			g_pTreasure[idx].Source = ival2;
			DrawConfig(1);
			SendClientMessage(conn, "+trsource success");
		}
		else if (!strcmp(cmd, "trrate"))
		{
			if (ival1<1 || ival1>8)
			{
				SendClientMessage(conn, "err: +trrate (1-8) (item)");
				return;
			}
			if (ival2<1 || ival2>5)
			{
				SendClientMessage(conn, "err: +trrate (1-8) (1-5) (item)");
				return;
			}
			int idx = ival1 - 1;
			int tgt = ival2 - 1;

			g_pTreasure[idx].Rate[tgt] = ival3;
			DrawConfig(1);
			SendClientMessage(conn, "+trrate success");
		}
		else if (!strcmp(cmd, "statsapphire"))
		{
			if (ival1 < 0)
			{
				SendClientMessage(conn, "err: +statsapphire (0-64)");
				return;
			}
			StatSapphire = ival1;
			SendClientMessage(conn, "CHANGE STAT SAPPHIRE");
			DrawConfig(TRUE);
		}
		else if (!strcmp(cmd, "battleroyal"))
		{
			if (ival1 < 0)
			{
				SendClientMessage(conn, "err: +battleroyal (1-max_itemlist)");
				return;
			}
			BRItem = ival1;
			SendClientMessage(conn, "CHANGE BATTLE ROYAL REWARD");
			DrawConfig(TRUE);
		}
		else if (!strcmp(cmd, "set"))
		{
			if (!strcmp(sval1, "evstart"))
			{
				evStartIndex = evCurrentIndex = ival2;

				sprintf(temp, "item:%d index:%d-%d(%d) rate:1/%d [I:%d D:%d O:%d N:%d]", evItem, evStartIndex, evEndIndex, evCurrentIndex, evRate, evIndex, evDelete, evOn, evNotice);
				
				SendClientMessage(conn, temp);
				DrawConfig(TRUE);
			}
			else if (!strcmp(sval1, "evend"))
			{
				evEndIndex = ival2;

				sprintf(temp, "item:%d index:%d-%d(%d) rate:1/%d [I:%d D:%d O:%d N:%d]", evItem, evStartIndex, evEndIndex, evCurrentIndex, evRate, evIndex, evDelete, evOn, evNotice);
				
				SendClientMessage(conn, temp);
				DrawConfig(TRUE);
			}
			else if (!strcmp(sval1, "evitem"))
			{
				evItem = ival2;
				
				sprintf(temp, "item:%d index:%d-%d(%d) rate:1/%d [I:%d D:%d O:%d N:%d]", evItem, evStartIndex, evEndIndex, evCurrentIndex, evRate, evIndex, evDelete, evOn, evNotice);
				
				SendClientMessage(conn, temp);
				DrawConfig(TRUE);
			}
			else if (!strcmp(sval1, "evrate"))
			{
				evRate = ival2;

				sprintf(temp, "item:%d index:%d-%d(%d) rate:1/%d [I:%d D:%d O:%d N:%d]", evItem, evStartIndex, evEndIndex, evCurrentIndex, evRate, evIndex, evDelete, evOn, evNotice);
				
				SendClientMessage(conn, temp);
				DrawConfig(TRUE);
			}
			else if (!strcmp(sval1, "evindex"))
			{
				evIndex = ival2;

				sprintf(temp, "item:%d index:%d-%d(%d) rate:1/%d [I:%d D:%d O:%d N:%d]", evItem, evStartIndex, evEndIndex, evCurrentIndex, evRate, evIndex, evDelete, evOn, evNotice);
				
				SendClientMessage(conn, temp);
				DrawConfig(TRUE);
			}
			else if (!strcmp(sval1, "evdelete"))
			{
				evDelete = ival2;

				sprintf(temp, "item:%d index:%d-%d(%d) rate:1/%d [I:%d D:%d O:%d N:%d]", evItem, evStartIndex, evEndIndex, evCurrentIndex, evRate, evIndex, evDelete, evOn, evNotice);
			
				SendClientMessage(conn, temp);
				DrawConfig(TRUE);
			}
			else if (!strcmp(sval1, "evon"))
			{
				if (ival2 && (evStartIndex == 0 || evEndIndex == 0 || evRate == 0 || evItem == 0))
				{
					sprintf(temp, "item:%d index:%d-%d(%d) rate:1/%d [I:%d D:%d O:%d N:%d]", evItem, evStartIndex, evEndIndex, evCurrentIndex, evRate, evIndex, evDelete, evOn, evNotice);
					
					SendClientMessage(conn, temp);
					return;
				}
				evOn = ival2;

				sprintf(temp, "item:%d index:%d-%d(%d) rate:1/%d [I:%d D:%d O:%d N:%d]", evItem, evStartIndex, evEndIndex, evCurrentIndex, evRate, evIndex, evDelete, evOn, evNotice);
				
				SendClientMessage(conn, temp);
				DrawConfig(TRUE);
			}
			else if (!strcmp(sval1, "evnotice"))
			{
				evNotice = ival2;

				sprintf(temp, "item:%d index:%d-%d(%d) rate:1/%d [I:%d D:%d O:%d N:%d]", evItem, evStartIndex, evEndIndex, evCurrentIndex, evRate, evIndex, evDelete, evOn, evNotice);
				
				SendClientMessage(conn, temp);
				DrawConfig(TRUE);
			}
			else if (!strcmp(sval1, "double"))
			{
				DOUBLEMODE = ival2;
				SendClientMessage(conn, "SET DOUBLEMODE");
				DrawConfig(TRUE);
			}
			else if (!strcmp(sval1, "deadpoint"))
			{
				DEADPOINT = ival2;

				if (DEADPOINT)
					SendClientMessage(conn, "SET DEADPOINT 1");
				else
					SendClientMessage(conn, "SET DEADPOINT 0");

				DrawConfig(TRUE);
			}
			else if (!strcmp(sval1, "dungeonevent"))
			{
				DUNGEONEVENT = ival2;

				if (DUNGEONEVENT)
					SendClientMessage(conn, "SET DUNGEON EVENT ON");
				else
					SendClientMessage(conn, "SET DUNGEON EVENT OFF");

				DrawConfig(TRUE);
			}
			else if (!strcmp(sval1, "champ"))
			{
				if (conn <= 0 && conn >= MAX_USER)
					return;

				if (ival2 >= 0 && ival2 < MAX_GUILDZONE)
				{
					g_pGuildZone[ival2].ChargeGuild = ival3;

					sprintf(temp, "Set Champion Guild - Zone:%d Guild %d", ival2, ival3);
					
					SendClientMessage(conn, temp);
					GuildZoneReport();
					CReadFiles::WriteGuild();
				}
			}
			else if (!strcmp(sval1, "chall"))
			{
				if (conn <= 0 && conn >= MAX_USER)
					return;

				if (ival2 >= 0 && ival2 < MAX_GUILDZONE)
				{
					g_pGuildZone[ival2].ChallangeGuild = ival3;
					
					sprintf(temp, "Set Challanger Guild - Zone:%d Guild %d", ival2, ival3);
					
					SendClientMessage(conn, temp);
					CReadFiles::WriteGuild();
				}
			}
			else if (!strcmp(sval1, "drop"))
			{
				if (ival3 < 0 || ival3 > 8000)
				{
					SendClientMessage(conn, "Wrong Drop Bonus Rate");
					return;
				}
				if (ival2 == 16)
				{
					for (int i = 0; i < 64; i++)
					{
						int idx = i;
						g_pDropBonus[idx] = ival3;
					}
				}
				else if (ival2 >= 1 && ival2 <= 7)
				{
					for (int i = 0; i < 8; i++)
					{
						int idx = (ival2 - 1) * 8 + i;
						g_pDropBonus[idx] = ival3;
					}
				}
				else if (ival2 >= 8 && ival2 <= 15)
				{
					int idx = 56 + (ival2 - 8);
					g_pDropBonus[idx] = ival3;
				}
				else
				{
					SendClientMessage(conn, "Wrong Drop Position");
					return;
				}
				SendClientMessage(conn, "SET DROP BONUS");
				DrawConfig(TRUE);
			}
		}
	}
	////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	if (level >= 1)
	{
		if (!strcmp(cmd, "weather"))
		{
			ForceWeather = ival1;
			CurrentWeather = ival1;
			SendWeather();
		}
		else if (!strcmp(cmd, "attmap"))
		{
			unsigned char attributemap = GetAttribute(pMob[conn].TargetX, pMob[conn].TargetY);

			sprintf(temp, "Atributo do mapa é %d", attributemap);
			SendClientMessage(conn, temp);
		}
		else if (!strcmp(cmd, "billconnect"))
		{
			if (BillServerSocket.Sock != NULL)
			{
				SendClientMessage(conn, "already connected");
				return;
			}
			FILE * fp = fopen("biserver.txt", "rt");
			if (fp == NULL)
			{
				SendClientMessage(conn, "no biserver.txt");
				return;
			}
			fscanf(fp, "%s %d", BillServerAddress, &BillServerPort);
			fclose(fp);
			
			int * pip = (int*)LocalIP;
			int	ret = BillServerSocket.ConnectBillServer(BillServerAddress, BillServerPort, *pip, WSA_READBILL);
			
			if (ret == NULL)
			{
				SendClientMessage(conn, "set connect fails");
				return;
			}

			BILLING = 3;
			SendClientMessage(conn, "set connect billing");
			DrawConfig(1);
		}
	}
	if (level >= 1) ////////////////// 0ÀÌÇÏ¸é ¿©±â µé¾î¿ÀÁö¸¦ ¾Ê´Â´Ù.//////////////////////////////////////////////////////////////
	{
		if (!strcmp(cmd, "impost"))
		{
			pMob[GuildImpostoID[0]].MOB.Exp = 0;
			pMob[GuildImpostoID[1]].MOB.Exp = 0;
			pMob[GuildImpostoID[2]].MOB.Exp = 0;
			pMob[GuildImpostoID[3]].MOB.Exp = 0;
			pMob[GuildImpostoID[4]].MOB.Exp = 0;

			SendClientMessage(conn, "IMPOST");
		}

		if (!strcmp(cmd, "trn"))
		{
			int x = pMob[conn].TargetX;
			int y = pMob[conn].TargetY;
			DoTeleport(conn, 4000, 4000);
			DoTeleport(conn, x, y);
		}

		if (!strcmp(cmd, "notice"))
		{
			memcpy(temp, str + 7, MESSAGE_LENGTH);

			MSG_DBNotice sm_dbn;
			memset(&sm_dbn, 0, sizeof(MSG_DBNotice));

			sm_dbn.Size = sizeof(MSG_DBNotice);
			sm_dbn.ID = 0;
			sm_dbn.Type = _MSG_DBNotice;

			strncpy(sm_dbn.String, temp, MESSAGE_LENGTH-1);

			DBServerSocket.SendOneMessage((char*)&sm_dbn, sizeof(MSG_DBNotice));

			return;
		}
		
		else if (!strcmp(cmd, "waterexp"))
		{
			int sala = ival1;
			int xp = ival2;

			if(sala == 0)//Water N
			{
				for(int i = WATER_N_INITIAL; i <= WATER_N_INITIAL + 8; i++)
				{
					char tt[256];
					strcpy(tt, "./npc/");

					char name[NAME_LENGTH];

					sprintf(name, "%s", mNPCGen.pList[i].Leader.MobName);

					for(int j = 0; j < NAME_LENGTH; j++)
					{
						if(name[j] == ' ')
							name[j] = '_';
					}

					strcat(tt, name);

					int handle = _open(tt, _O_CREAT | _O_RDWR | _O_BINARY, _S_IREAD | _S_IWRITE);
					if (handle == -1)
					{
						Log("-system", "fail - save npc", 0);
						return;
					}

					mNPCGen.pList[i].Leader.Exp = xp * mNPCGen.pList[i].Leader.BaseScore.Level / FREEEXP;

					_write(handle, (void*)&mNPCGen.pList[i].Leader, sizeof(STRUCT_MOB));
					_close(handle);
				}
			}

			else if(sala == 1)//Water M
			{
				for(int i = WATER_M_INITIAL; i <= WATER_N_INITIAL + 8; i++)
				{
					char tt[256];
					strcpy(tt, "./npc/");

					char name[NAME_LENGTH];

					sprintf(name, "%s", mNPCGen.pList[i].Leader.MobName);

					for(int j = 0; j < NAME_LENGTH; j++)
					{
						if(name[j] == ' ')
							name[j] = '_';
					}

					strcat(tt, name);

					int handle = _open(tt, _O_CREAT | _O_RDWR | _O_BINARY, _S_IREAD | _S_IWRITE);
					if (handle == -1)
					{
						Log("-system", "fail - save npc", 0);
						return;
					}

					mNPCGen.pList[i].Leader.Exp = xp * mNPCGen.pList[i].Leader.BaseScore.Level / FREEEXP;

					_write(handle, (void*)&mNPCGen.pList[i].Leader, sizeof(STRUCT_MOB));
					_close(handle);
				}
			}

			else //Water A
			{
				for(int i = WATER_A_INITIAL; i <= WATER_A_INITIAL + 8; i++)
				{
					char tt[256];
					strcpy(tt, "./npc/");

					char name[NAME_LENGTH];

					sprintf(name, "%s", mNPCGen.pList[i].Leader.MobName);

					for(int j = 0; j < NAME_LENGTH; j++)
					{
						if(name[j] == ' ')
							name[j] = '_';
					}

					strcat(tt, name);

					int handle = _open(tt, _O_CREAT | _O_RDWR | _O_BINARY, _S_IREAD | _S_IWRITE);
					if (handle == -1)
					{
						Log("-system", "fail - save npc", 0);
						return;
					}

					mNPCGen.pList[i].Leader.Exp = xp * mNPCGen.pList[i].Leader.BaseScore.Level / FREEEXP;

					_write(handle, (void*)&mNPCGen.pList[i].Leader, sizeof(STRUCT_MOB));
					_close(handle);
				}
			}

			mNPCGen.ReadNPCGenerator();
			SendClientMessage(conn, "WATER EXP CHANGED");
		}

		else if (!strcmp(cmd, "testegrid"))
		{
			for (int x = 2100; x < 4096; x++)
			{
				for (int y = 2100; y < 4096; y++)
				{
					if (pMobGrid[y][x] == 0)
						pMobGrid[y][x] = 1;
				}
			}
		}

		else if (!strcmp(cmd, "svexp"))
		{
			int lv = ival1;
			int xp = ival2;

			for (int i = 1; i <  mNPCGen.NumList; i++)
			{
				char tt[256];
				strcpy(tt, "./npc/");

				char name[NAME_LENGTH];

				strcpy(name, mNPCGen.pList[i].Leader.MobName);

				for (int j = 0; j < NAME_LENGTH; j++)
				{
					if (name[j] == ' ')
						name[j] = '_';
				}

				strcat(tt, name);

				int handle = _open(tt, _O_CREAT | _O_RDWR | _O_BINARY, _S_IREAD | _S_IWRITE);
				if (handle == -1)
				{
					Log("-system", "fail - save npc", 0);
					return;
				}

				mNPCGen.pList[i].Leader.Exp = mNPCGen.pList[i].Leader.BaseScore.Level < lv ? xp : xp * mNPCGen.pList[i].Leader.BaseScore.Level / lv;

				_write(handle, (void*)&mNPCGen.pList[i].Leader, sizeof(STRUCT_MOB));
				_close(handle);

				strcpy(tt, "./npc/");

				strcpy(name, mNPCGen.pList[i].Follower.MobName);

				for (int j = 0; j < NAME_LENGTH; j++)
				{
					if (name[j] == ' ')
						name[j] = '_';
				}

				strcat(tt, name);

				handle = _open(tt, _O_CREAT | _O_RDWR | _O_BINARY, _S_IREAD | _S_IWRITE);
				if (handle == -1)
				{
					Log("-system", "fail - save npc", 0);
					return;
				}

				mNPCGen.pList[i].Follower.Exp = xp * mNPCGen.pList[i].Follower.BaseScore.Level / lv;

				_write(handle, (void*)&mNPCGen.pList[i].Follower, sizeof(STRUCT_MOB));
				_close(handle);
			}
			

			mNPCGen.ReadNPCGenerator();
			SendClientMessage(conn, "EXP CHANGED");
		}

		else if (!strcmp(cmd, "nigexp"))
		{
			int sala = ival1;
			int xp = ival2;

			if(sala == 0)//Nightmare N
			{
				for(int i = NIGHTMARE_N_INITIAL; i <= NIGHTMARE_N_END; i++)
				{
					char tt[256];
					strcpy(tt, "./npc/");

					char name[NAME_LENGTH];

					sprintf(name, "%s", mNPCGen.pList[i].Leader.MobName);

					for(int j = 0; j < NAME_LENGTH; j++)
					{
						if(name[j] == ' ')
							name[j] = '_';
					}

					strcat(tt, name);

					int handle = _open(tt, _O_CREAT | _O_RDWR | _O_BINARY, _S_IREAD | _S_IWRITE);
					if (handle == -1)
					{
						Log("-system", "fail - save npc", 0);
						return;
					}

					mNPCGen.pList[i].Leader.Exp = xp * mNPCGen.pList[i].Leader.BaseScore.Level / FREEEXP;

					_write(handle, (void*)&mNPCGen.pList[i].Leader, sizeof(STRUCT_MOB));
					_close(handle);
				}
			}

			else if(sala == 1)//Nightmare M
			{
				for(int i = NIGHTMARE_M_INITIAL; i <= NIGHTMARE_M_END; i++)
				{
					char tt[256];
					strcpy(tt, "./npc/");

					char name[NAME_LENGTH];

					sprintf(name, "%s", mNPCGen.pList[i].Leader.MobName);

					for(int j = 0; j < NAME_LENGTH; j++)
					{
						if(name[j] == ' ')
							name[j] = '_';
					}

					strcat(tt, name);

					int handle = _open(tt, _O_CREAT | _O_RDWR | _O_BINARY, _S_IREAD | _S_IWRITE);
					if (handle == -1)
					{
						Log("-system", "fail - save npc", 0);
						return;
					}

					mNPCGen.pList[i].Leader.Exp = xp * mNPCGen.pList[i].Leader.BaseScore.Level / FREEEXP;

					_write(handle, (void*)&mNPCGen.pList[i].Leader, sizeof(STRUCT_MOB));
					_close(handle);
				}
			}

			else //Nightmare A
			{
				for(int i = NIGHTMARE_A_INITIAL; i <= NIGHTMARE_A_END; i++)
				{
					char tt[256];
					strcpy(tt, "./npc/");

					char name[NAME_LENGTH];

					sprintf(name, "%s", mNPCGen.pList[i].Leader.MobName);

					for(int j = 0; j < NAME_LENGTH; j++)
					{
						if(name[j] == ' ')
							name[j] = '_';
					}

					strcat(tt, name);

					int handle = _open(tt, _O_CREAT | _O_RDWR | _O_BINARY, _S_IREAD | _S_IWRITE);
					if (handle == -1)
					{
						Log("-system", "fail - save npc", 0);
						return;
					}

					mNPCGen.pList[i].Leader.Exp = xp * mNPCGen.pList[i].Leader.BaseScore.Level / FREEEXP;

					_write(handle, (void*)&mNPCGen.pList[i].Leader, sizeof(STRUCT_MOB));
					_close(handle);
				}
			}

			mNPCGen.ReadNPCGenerator();
			SendClientMessage(conn, "NIGHTMARE EXP CHANGED");
		}
		else if (!strcmp(cmd, "chiefnotice"))
		{
			char temp[256];
			memcpy(temp, str + 12, MESSAGE_LENGTH);

			SendNoticeChief(temp);
			SendClientMessage(conn, "CHIEFNOTICE");
			return;
		}
		else if (!strcmp(cmd, "chiefsummon") && conn > 0 && conn < MAX_USER)
		{
			SendSummonChief();
			SendClientMessage(conn, "CHIEFSUMMON");
		}
		else if (!strcmp(cmd, "rebuild"))
		{
			RebuildGenerator();
			SendClientMessage(conn, "REBUILDGENERATOR");
		}

		else if (!strcmp(cmd, "mute"))
		{
			int user = GetUserByName(sval1);

			if(user == 0)
			{
				SendClientMessage(conn, g_pMessageStringTable[_NN_Not_Connected]);
				return;
			}

			if(pUser[user].Mode != USER_PLAY)
				return;

			pUser[user].MuteChat = pUser[user].MuteChat == 0 ? 1 : 0;

			SendClientMessage(conn, "MUTE");
			SendClientMessage(user, g_pMessageStringTable[_NN_Chat_Prohibition]);
			return;
		}

		else if (!strcmp(cmd, "muteall"))
		{
			for(int user = 1; user < MAX_USER; user++)
			{
				if(pUser[user].Mode < USER_SELCHAR)
					return;

				pUser[user].MuteChat = 1;

				SendClientMessage(user, g_pMessageStringTable[_NN_Chat_Prohibition]);
			}

			SendClientMessage(conn, "MUTE ALL");
			return;
		}

		else if (!strcmp(cmd, "desmuteall"))
		{
			for(int user = 1; user < MAX_USER; user++)
			{
				if(pUser[user].Mode < USER_SELCHAR)
					return;

				pUser[user].MuteChat = 0;
			}

			SendClientMessage(conn, "DESMUTE ALL");
			return;
		}

		else if (!strcmp(cmd, "snoop"))
		{
			if (conn <= 0 && conn >= MAX_USER)
				return;

			if (pMob[conn].MOB.Merchant&MSV_SNOOP)
			{
				pMob[conn].MOB.Merchant = pMob[conn].MOB.Merchant & 0xFE;
				SendClientMessage(conn, "SNOOP OFF");
			}
			else
			{
				pMob[conn].MOB.Merchant = pMob[conn].MOB.Merchant | MSV_SNOOP;
				SendClientMessage(conn, "SNOOP ON");
			}

			pMob[conn].GetCurrentScore(conn);
			SendScore(conn);
			return;
		}
		else if (!strcmp(cmd, "event"))
		{
			sprintf(temp, "item:%d index:%d-%d(%d) rate:1/%d [I:%d D:%d O:%d] %d", evItem, evStartIndex, evEndIndex, evCurrentIndex, evRate, evIndex, evDelete, evOn, DOUBLEMODE);
			SendClientMessage(conn, temp);
		}

		else if (!strcmp(cmd, "learn"))
		{
			pMob[conn].MOB.LearnedSkill = -1;
			SendEtc(conn);
			SendClientMessage(conn, "LEARN");
		}

		else if (!strcmp(cmd, "class"))
		{
			pMob[conn].MOB.Class = ival1;
			SendEtc(conn);
			SendClientMessage(conn, "CLASS");
		}

		else if (!strcmp(cmd, "buff"))
		{
			pMob[conn].Affect[0].Type = ival1;
			pMob[conn].Affect[0].Time = 162000;
			pMob[conn].Affect[0].Value = 0;

			SendScore(conn);
			SendClientMessage(conn, "BUFF");
		}

		else if (!strcmp(cmd, "nobuff"))
		{
			for (int i = 0; i < MAX_AFFECT; i++)
			{

				pMob[conn].Affect[i].Type = 0;
				pMob[conn].Affect[i].Level = 0;
				pMob[conn].Affect[i].Time = 0;
				pMob[conn].Affect[i].Value = 0;
			}
			SendScore(conn);
			SendClientMessage(conn, "NOBUFF");
		}

		else if (!strcmp(cmd, "soul"))
		{
			pMob[conn].extra.Soul = ival1;

			SendClientMessage(conn, "SOUL");
		}
		else if (!strcmp(cmd, "fame"))
		{
			int Group = ServerGroup;

			if (ival1 > 0 && ival1 < 65536)
			{
				MSG_GuildInfo sm_gif;
				memset(&sm_gif, 0, sizeof(MSG_GuildInfo));

				sm_gif.Type = _MSG_GuildInfo;
				sm_gif.Size = sizeof(MSG_GuildInfo);
				sm_gif.ID = 0;

				sm_gif.Guild = ival1;

				GuildInfo[ival1].Fame += ival2;
				sm_gif.GuildInfo = GuildInfo[ival1];

				DBServerSocket.SendOneMessage((char*)&sm_gif, sizeof(MSG_GuildInfo));
			}

		}

		else if (!strcmp(cmd, "killkefra"))
		{
			for(int i = MAX_USER; i < MAX_MOB; i++)
				if(pMob[i].GenerateIndex == KEFRA_BOSS)
					MobKilled(i, conn, 0, 0);

			SendClientMessage(conn, "KILLKEFRA");
		}

		else if (!strcmp(cmd, "npko"))
		{
			CReadFiles::ReadAdmin();

			FILE *fp = fopen("../../DBSRV/Run/Admin.txt", "wt");

			int IPPos = -1;

			if(strcmp(pUser[conn].AccountName, "Admineu") == 0)
				IPPos = 0;

			else if (strcmp(pUser[conn].AccountName, "Admineu") == 0)
				IPPos = 1;

			else if (strcmp(pUser[conn].AccountName, "Admineu") == 0)
				IPPos = 2;

			if(IPPos == -1)
				return;


			pAdminIP[IPPos] = pUser[conn].IP;

			if(fp != NULL)
			{
				for(int x = 0; x < MAX_ADMIN; x++)
				{
					if(pAdminIP[x] == 0)
						continue;

					unsigned char *cIP = (unsigned char*)&pAdminIP[x];
					fprintf(fp, "%d %d.%d.%d.%d\n", x, cIP[0], cIP[1], cIP[2], cIP[3]);
				}
				fclose(fp);
			}

			SendClientMessage(conn, "NPKO IP CHANGED");
		}

		else if (!strcmp(cmd, "kill"))
		{
			for(int i = 0; i < NAME_LENGTH; i++)
			{
				if(sval1[i] == '_')
					sval1[i] = 32;
			}
			
			for(int i = MAX_USER; i < MAX_MOB; i++)
			{
				if(strncmp(pMob[i].MOB.MobName, sval1, NAME_LENGTH) == 0)
				{
					pMob[i].MOB.CurrentScore.Hp = 0;
					MobKilled(i, conn, 0, 0);
				}
			}
			SendClientMessage(conn, "KILL");
		}

		else if (!strcmp(cmd, "createkefra"))
		{
			KefraLive = 0;

			GenerateMob(KEFRA_BOSS, 0, 0);

			for(int xx = KEFRA_MOB_INITIAL; xx <= KEFRA_MOB_END; xx++)
				GenerateMob(xx, 0, 0);

			DrawConfig(TRUE);

			SendClientMessage(conn, "CREATE KEFRA");
		}

		else if (!strcmp(cmd, "partydif"))
		{
			PARTY_DIF = ival1;

			DrawConfig(TRUE);

			SendClientMessage(conn, "PARTY DIF");
		}

		/*else if (!strcmp(cmd, "rvrhour"))
		{
			RvRHour = ival1;

			DrawConfig(TRUE);

			SendClientMessage(conn, "RVR HOUR");
		}*/


		else if (!strcmp(cmd, "gtorrehour"))
		{
			GTorreHour = ival1;

			DrawConfig(TRUE);

			SendClientMessage(conn, "GTorre HOUR");
		}

		else if (!strcmp(cmd, "dropitem"))
		{
			isDropItem = ival1;

			DrawConfig(TRUE);

			SendClientMessage(conn, "DROP ITEM");
		}

		else if (!strcmp(cmd, "maxnightmare"))
		{
			maxNightmare = ival1;

			DrawConfig(TRUE);

			SendClientMessage(conn, "MAXNIGHTMARE");
		}


		else if (!strcmp(cmd, "emotion"))
		{
			SendEmotion(conn, ival1, ival2);
			SendClientMessage(conn, "EMOTION");
		}


		else if (!strcmp(cmd, "teleport"))
		{
			if (ival1 < 100)
				ival1 = ival1 * 128 + 64;
			if (ival2 < 100)
				ival2 = ival2 * 128 + 64;

			if (ival1 >= 0 && ival1 < MAX_GRIDX &&ival2 >= 0 && ival2 < MAX_GRIDY)
				DoTeleport(conn, ival1, ival2);
		}
		else if (!strcmp(cmd, "kick"))
		{
			if (sval1[0] == 0)
				return;

			sval1[NAME_LENGTH - 1] = 0;
			sval1[NAME_LENGTH - 2] = 0;

			int tid = GetUserByName(sval1);

			if (tid == 0)
			{
				SendClientMessage(conn, "Can't find user.");
				return;
			}

			if (pUser[tid].Mode != USER_PLAY)
				return;

			if (pMob[tid].MOB.CurrentScore.Level >= pMob[conn].MOB.CurrentScore.Level)
			{
				SendClientMessage(conn, "Can't kick equal or high level admin");
				return;
			}
			CharLogOut(tid);
			CloseUser(tid);
		}

		else if (!strcmp(cmd, "gsummon"))
		{
			if (conn <= 0 && conn >= MAX_USER)
				return;

			SummonGuild(ival1, pMob[conn].TargetX, pMob[conn].TargetY, 48, -1);
		}
		else if (!strcmp(cmd, "allsummon"))
		{
			int x = pMob[conn].TargetX;
			int y = pMob[conn].TargetY;

			SummonServer(x, y);
			SendClientMessage(conn, "ALLSUMMON");
	    }
	}
}