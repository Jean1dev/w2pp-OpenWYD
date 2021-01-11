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

void Exec_MSG_UpdateItem(int conn, char *pMsg)
{
	MSG_UpdateItem *m = (MSG_UpdateItem*)pMsg;

	if (pMob[conn].MOB.CurrentScore.Hp == 0 || pUser[conn].Mode != USER_PLAY)
	{
		SendHpMode(conn);
		AddCrackError(conn, 1, 16);
		return;
	}
	if(m->State < 0 || m->State > 5)
	{
		AddCrackError(conn, 50, 50);
		return;
	}

	if(m->ItemID < 10000 || m->ItemID >= 10000 + MAX_ITEM)
	{
		AddCrackError(conn, 50, 50);
		return;
	}

	int gateid = m->ItemID - 10000;
	int gatestate = m->State;

	if(gateid < 0 || gateid >= MAX_ITEM)
	{
		AddCrackError(conn, 50, 50);
		return;
	}

	if (CCastleZakum::OpenCastleGate(conn, gateid, m) == TRUE)
		return;
		
	int state = pItem[gateid].State;
	int gatestate2 = m->State;

	int gatekey = BASE_GetItemAbility(&pItem[gateid].ITEM, EF_KEYID);

	if (state != 3 && gatestate2 != 3)
		goto NoNeedKey;

	if (gatekey == 0)
		goto NoNeedKey;

	int key = 0;
	int Quest = -1;
	
	int i = 0;
	for (i = 0; i < MAX_CARRY; i++)
	{
		key = BASE_GetItemAbility(&pMob[conn].MOB.Carry[i], EF_KEYID);

		if (key != gatekey)
			continue;

		Quest = BASE_GetItemAbility(&pMob[conn].MOB.Carry[i], EF_QUEST);

		break;
	}

	if (gatekey != key)
	{
		if (pItem[gateid].ITEM.sIndex != 773)
			SendClientMessage(conn, g_pMessageStringTable[_NN_No_Key]);

		return;
	}
	
	memset(&pMob[conn].MOB.Carry[i], 0, sizeof(STRUCT_ITEM));
	SendItem(conn, ITEM_PLACE_CARRY, i, &pMob[conn].MOB.Carry[i]);
NoNeedKey:

	/*if (CEncampment::OpenTreasureBox(conn, gateid) == TRUE)
		return;*/

	int heigth = 0;

	int isUpdate = UpdateItem(gateid, STATE_OPEN, &heigth);

	if (isUpdate)
		GridMulticast(pItem[gateid].PosX, pItem[gateid].PosY, (MSG_STANDARD*)m, 0);

	sprintf(temp, "etc,opengate gateid:%d X:%d Y:%d", gateid, pItem[gateid].PosX, pItem[gateid].PosY);
	Log(temp, pUser[conn].AccountName, pUser[conn].IP);
	return;
}