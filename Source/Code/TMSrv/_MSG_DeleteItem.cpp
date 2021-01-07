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

void Exec_MSG_DeleteItem(int conn, char *pMsg)
{
	MSG_DeleteItem *m = (MSG_DeleteItem*)pMsg;

	if (m->Slot < 0 || m->Slot >= MAX_CARRY - 4)
		return;

	if (m->sIndex <= 0 || m->sIndex >= MAX_ITEMLIST)
		return;

	if (pUser[conn].Mode != USER_PLAY)
	{
		SendHpMode(conn);
		return;
	}

	if (pUser[conn].Trade.OpponentID)
	{
		RemoveTrade(pUser[conn].Trade.OpponentID);
		RemoveTrade(conn);
		return;
	}
	char itemtmp[2048];
	BASE_GetItemCode(&pMob[conn].MOB.Carry[m->Slot], itemtmp);

	sprintf(temp, "deleteitem, %s", itemtmp);
	ItemLog(temp, pMob[conn].MOB.MobName, pUser[conn].IP);

	memset(&pMob[conn].MOB.Carry[m->Slot], 0, sizeof(STRUCT_ITEM));
}