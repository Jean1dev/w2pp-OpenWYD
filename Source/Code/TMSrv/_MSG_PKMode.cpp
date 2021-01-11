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

void Exec_MSG_PKMode(int conn, char *pMsg)
{
	MSG_STANDARDPARM *m = (MSG_STANDARDPARM*)pMsg;

	pUser[conn].PKMode = m->Parm != 0 ? 1 : 0;

	if (pUser[conn].Trade.OpponentID)
	{
		if (pUser[pUser[conn].Trade.OpponentID].Mode == USER_PLAY && pUser[pUser[conn].Trade.OpponentID].Trade.OpponentID == conn)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Cant_trade_pkmode]);
			SendClientMessage(pUser[conn].Trade.OpponentID, g_pMessageStringTable[_NN_Cant_trade_pkmode]);

			RemoveTrade(pUser[conn].Trade.OpponentID);
			RemoveTrade(conn);
		}
	}

	MSG_STANDARDPARM sm;
	memset(&sm, 0, sizeof(MSG_STANDARDPARM));

	sm.Size = sizeof(MSG_STANDARDPARM);
	sm.Type = _MSG_PKInfo;
	sm.ID = conn;

	if (NewbieEventServer == 0)
	{
		int guilty = GetGuilty(conn);

		int state = 0;

		if (guilty || pUser[conn].PKMode || g_pRvrWar.Status && pMob[conn].TargetX >= 1023 && pMob[conn].TargetY >= 1919 && pMob[conn].TargetX <= 1280 && pMob[conn].TargetY <= 2179 || CastleState && pMob[conn].TargetX >= 1024 && pMob[conn].TargetY >= 1664 && pMob[conn].TargetX <= 1153 && pMob[conn].TargetY <= 1793 || GTorreState && pMob[conn].TargetX >= 2430 && pMob[conn].TargetY >= 1825 && pMob[conn].TargetX <= 2560 && pMob[conn].TargetY <= 1925)
			state = 1;

		sm.Parm = state;
	}
	else
		sm.Parm = 1;


	GridMulticast(pMob[conn].TargetX, pMob[conn].TargetY, (MSG_STANDARD*)&sm, 0);

	sprintf(temp, "etc,pkmode %d", pUser[conn].PKMode);
	Log(temp, pUser[conn].AccountName, pUser[conn].IP);
}