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

void Exec_MSG_NoViewMob(int conn, char *pMsg)
{
	MSG_STANDARDPARM *m = (MSG_STANDARDPARM*)pMsg;

	int MobID = m->Parm;

	if (MobID <= 0 || MobID >= MAX_MOB)
	{
		Log("err,reqmobbyid MobId Out of range", pUser[conn].AccountName, pUser[conn].IP);
		return;
	}

	if (pMob[MobID].Mode == MOB_EMPTY)
	{
		SendRemoveMob(conn, MobID, 0, 0);
		return;
	}

	if (MobID >= MAX_USER || pUser[MobID].Mode == USER_PLAY)
	{
		if (GetInView(conn, MobID))
		{
			SendCreateMob(conn, MobID, 1);
			SendPKInfo(conn, MobID);
		}
		else
			SendRemoveMob(conn, MobID, 0, 0);
	}
	else
		SendRemoveMob(conn, MobID, 0, 0);
}