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

void Exec_MSG_SplitItem(int conn, char *pMsg)
{
	MSG_SplitItem *m = (MSG_SplitItem*)pMsg;

	if (m->Slot < 0 || m->Slot >= MAX_CARRY - 4)
		return;

	if (m->Num <= 0 || m->Num >= 120)
		return;

	if (pUser[conn].Mode != USER_PLAY)
	{
		SendHpMode(conn);
		return;
	}

	if (pUser[conn].Trade.OpponentID)
	{
		RemoveTrade(conn);
		return;
	}

	int slot = m->Slot;

	if(pMob[conn].MOB.Carry[slot].sIndex != 413 && pMob[conn].MOB.Carry[slot].sIndex != 412
	&& pMob[conn].MOB.Carry[slot].sIndex != 419 && pMob[conn].MOB.Carry[slot].sIndex != 420
	&& pMob[conn].MOB.Carry[slot].sIndex != 416 && pMob[conn].MOB.Carry[slot].sIndex != 414
	&& (pMob[conn].MOB.Carry[slot].sIndex < 2390 || pMob[conn].MOB.Carry[slot].sIndex > 2419))
	{
		Log("splititem, no validitem", pUser[conn].AccountName, pUser[conn].IP);
		return;
	}

	int amount = BASE_GetItemAmount(&pMob[conn].MOB.Carry[slot]);

	if(m->Num < 0 || m->Num > 120)
	{
		Log("splititem, invalid num", pUser[conn].AccountName, pUser[conn].IP);
		return;
	}

	int x = 0;
	int invfree = 0;

	for (x = 0; x < pMob[conn].MaxCarry; x++)
	{
		if(pMob[conn].MOB.Carry[x].sIndex == 0)
			invfree++;
	}

	if(invfree < 1)
		return;

	if(amount == 0 || amount == 1 || amount <= m->Num)
		return;
							 
	sprintf(temp, "splititem, itemindex:%d amount:%d num:%d", pMob[conn].MOB.Carry[slot].sIndex, amount, m->Num);
	ItemLog(temp, pMob[conn].MOB.MobName, pUser[conn].IP);

	if (amount > 1)
		BASE_SetItemAmount(&pMob[conn].MOB.Carry[slot], amount - m->Num);

	else
		memset(&pMob[conn].MOB.Carry[slot], 0, sizeof(STRUCT_ITEM));

	STRUCT_ITEM nItem;

	memset(&nItem, 0, sizeof(STRUCT_ITEM));

	nItem.sIndex = pMob[conn].MOB.Carry[slot].sIndex;
							 
	BASE_SetItemAmount(&nItem, m->Num);

	PutItem(conn, &nItem);
	SendItem(conn, ITEM_PLACE_CARRY, slot, &pMob[conn].MOB.Carry[slot]);
	return;
}