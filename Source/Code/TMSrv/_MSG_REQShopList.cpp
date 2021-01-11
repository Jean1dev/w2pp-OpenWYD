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

void Exec_MSG_REQShopList(int conn, char *pMsg)
{
	MSG_REQShopList *m = (MSG_REQShopList*)pMsg;

	if (pMob[conn].MOB.CurrentScore.Hp == 0 || pUser[conn].Mode != USER_PLAY)
	{
		SendHpMode(conn);
		AddCrackError(conn, 10, 22);
		return;
	}

	int target = m->Target;

	if (target < MAX_USER || target >= MAX_MOB)
	{
		Log("err,MSG_ReqShopList target range", pUser[conn].AccountName, pUser[conn].IP);
		return;
	}

	if (pMob[target].MOB.Merchant == 0)
	{
		SendClientMessage(conn, g_pMessageStringTable[_NN_He_Is_Not_Merchant]);
		return;
	}

	int view = GetInView(conn, target);

	if (view == 0)
		return;

	if (pMob[target].MOB.Merchant == 1)
		SendShopList(conn, target, 1);

	else if ((pMob[target].MOB.Merchant) == 19)
		SendShopList(conn, target, 3);
}