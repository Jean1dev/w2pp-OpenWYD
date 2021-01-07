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
#include "ProcessClientMessage.h"

void Exec_MSG_CombineItemLindy(int conn, char *pMsg)
{
	MSG_CombineItem *m = (MSG_CombineItem*)pMsg;

	for (int i = 0; i < MAX_COMBINE; i++)
	{
		if (m->Item[i].sIndex == 0)
			continue;

		int invPos = m->InvenPos[i];

		if (invPos < 0 || invPos >= pMob[conn].MaxCarry)
		{
			RemoveTrade(conn);
			return;
		}

		if (memcmp(&pMob[conn].MOB.Carry[invPos], &m->Item[i], sizeof(STRUCT_ITEM)))
		{
			ItemLog("err,msg_CombineLindy - item remove or changed.", pUser[conn].AccountName, pUser[conn].IP);
			SendClientSignalParm(conn, ESCENE_FIELD, _MSG_CombineComplete, 0);
			return;
		}
	}

	if(pMob[conn].extra.ClassMaster != ARCH)
		return;

	if(pMob[conn].MOB.CurrentScore.Level != 369 && pMob[conn].MOB.CurrentScore.Level != 354)
		return;

	if(pMob[conn].extra.QuestInfo.Arch.Level355 == 1 && pMob[conn].MOB.CurrentScore.Level == 354)
		return;

	if(pMob[conn].extra.QuestInfo.Arch.Level370 == 0 && pMob[conn].MOB.CurrentScore.Level == 369 && pMob[conn].extra.Fame <= 0)
		return;

	if(pMob[conn].extra.QuestInfo.Arch.Level370 == 1 && pMob[conn].MOB.CurrentScore.Level == 369)
		return;

	ItemLog("*** Item combine lindy. ***", pUser[conn].AccountName, pUser[conn].IP);

	for (int i = 0; i < MAX_COMBINE; i++)
	{
		if (m->Item[i].sIndex)
		{
			char itemlog[2048];
									 
			BASE_GetItemCode(&m->Item[i], itemlog);
									 
			strcat(temp, itemlog);
		}
	}
	ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);
	ItemLog("*** ------------------- ***", pUser[conn].AccountName, pUser[conn].IP);


	if(m->Item[0].sIndex == 413 && BASE_GetItemAmount(&m->Item[0]) == 10 && m->Item[1].sIndex == 413 && BASE_GetItemAmount(&m->Item[1]) == 10
	&& m->Item[2].sIndex == 4127 && m->Item[3].sIndex == 413 && m->Item[4].sIndex == 413 && m->Item[5].sIndex == 413 && m->Item[6].sIndex == 413)
	{

		for (int i = 0; i < MAX_COMBINE; i++)
		{
			if (m->Item[i].sIndex == 0)
				continue;

			memset(&pMob[conn].MOB.Carry[m->InvenPos[i]], 0, sizeof(STRUCT_ITEM));
			SendItem(conn, ITEM_PLACE_CARRY, m->InvenPos[i], &pMob[conn].MOB.Carry[m->InvenPos[i]]);
		}

		if(pMob[conn].MOB.CurrentScore.Level == 354)
		{
			pMob[conn].extra.QuestInfo.Arch.Level355 = 1;

			memset(&pMob[conn].MOB.Equip[15], 0, sizeof(STRUCT_ITEM));

			if(pMob[conn].MOB.Clan == 7)
				pMob[conn].MOB.Equip[15].sIndex = 3191;

			else if(pMob[conn].MOB.Clan == 8)
				pMob[conn].MOB.Equip[15].sIndex = 3192;

			else
				pMob[conn].MOB.Equip[15].sIndex = 3193;

			SendItem(conn, ITEM_PLACE_EQUIP, 15, &pMob[conn].MOB.Equip[15]);
		}
								 
		if(pMob[conn].MOB.CurrentScore.Level == 369)
		{
			pMob[conn].extra.QuestInfo.Arch.Level370 = 1;
			pMob[conn].extra.Fame -= 1;
		}
		sprintf(temp, "lindy,unlock lv%d", pMob[conn].MOB.CurrentScore.Level);
		Log(temp, pUser[conn].AccountName, pUser[conn].IP);

		SendClientSignalParm(conn, ESCENE_FIELD, _MSG_CombineComplete, 1);

		SendEmotion(conn, 14, 3);
		SendClientMessage(conn, g_pMessageStringTable[_NN_Processing_Complete]);
		return;
	}
}