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

void Exec_MSG_CombineItem(int conn, char *pMsg)
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
			ItemLog("err,msg_Combine - item remove or changed.", pUser[conn].AccountName, pUser[conn].IP);
			SendClientSignalParm(conn, ESCENE_FIELD, _MSG_CombineComplete, 0);
			return;
		}
	}

	int combine = GetMatchCombine(m->Item);

	if (combine == 0)
	{
		SendClientMessage(conn, g_pMessageStringTable[_NN_Wrong_Combination]);
		SendClientSignalParm(conn, ESCENE_FIELD, _MSG_CombineComplete, 0);
		return;
	}

	for (int i = 0; i < MAX_COMBINE; i++)
	{
		if (m->Item[i].sIndex == 0)
			continue;

		memset(&pMob[conn].MOB.Carry[m->InvenPos[i]], 0, sizeof(STRUCT_ITEM));
		SendItem(conn, ITEM_PLACE_CARRY, m->InvenPos[i], &pMob[conn].MOB.Carry[m->InvenPos[i]]);
	}

	ItemLog("*** Item combine anct. ***", pUser[conn].AccountName, pUser[conn].IP);

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

	int _rand = rand() % 115;
	if (_rand >= 100)
		_rand -= 15;

	if (_rand <= combine || LOCALSERVER)
	{
		int itemindex = m->Item[0].sIndex;
		int extra = g_pItemList[itemindex].Extra;

		if (extra > 0 && extra < MAX_ITEMLIST)
		{
			int ipos = m->InvenPos[0];
			int joia = m->Item[1].sIndex - 2441;
									 
			if (joia >= 0 && joia <= 3)
			{
				memcpy(&pMob[conn].MOB.Carry[ipos].sIndex, &m->Item[0], sizeof(STRUCT_ITEM));

				pMob[conn].MOB.Carry[ipos].sIndex = joia + extra;

				BASE_SetItemSanc(&pMob[conn].MOB.Carry[ipos], 7, 0);

				char tt[256];

				sprintf(tt, g_pMessageStringTable[_SS_CombineSucceed], pMob[conn].MOB.MobName, g_pItemList[extra].Name);
				sprintf(temp, "%s(%d/%d)", tt, _rand, combine);
										 
				SendClientMessage(conn, temp);
										 
				SendClientSignalParm(conn, ESCENE_FIELD, _MSG_CombineComplete, 1);
										 
				sprintf(temp, "%s ", pUser[conn].AccountName);
				BASE_GetItemCode(&pMob[conn].MOB.Carry[ipos], tt);
				strcat(temp, tt);
										 
				ItemLog(temp, "**combine anct sucess ***", pUser[conn].IP);
				SendItem(conn, ITEM_PLACE_CARRY, ipos, &pMob[conn].MOB.Carry[ipos]);

				return;
			}
			else
			{
				sprintf(temp, "err,msg_combine - stone wrong %d", joia);
				ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);
				return;
			}
		}
		else
		{
			sprintf(temp, "err,msg_combine - extra item index wrong %d", itemindex);
			ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);
			return;
		}
	}
	else
	{
		sprintf(temp, "%s(%d/%d)", g_pMessageStringTable[269], _rand, combine);
		SendClientMessage(conn, temp);
								 
		SendClientSignalParm(conn, ESCENE_FIELD, _MSG_CombineComplete, 2);
		ItemLog("*** Combine anct fail ***", pUser[conn].AccountName, pUser[conn].IP);

		return;
	}
}