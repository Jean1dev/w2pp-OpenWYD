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

void Exec_MSG_ReqTradeList(int conn, char *pMsg)
{
	MSG_STANDARDPARM *m = (MSG_STANDARDPARM*)pMsg;

	int autoID = m->Parm;

	if (pMob[conn].MOB.CurrentScore.Hp == 0 || pUser[conn].Mode != USER_PLAY)
	{
		SendHpMode(conn);
		AddCrackError(conn, 10, 87);
		return;
	}

	if (autoID <= 0 || autoID >= MAX_USER)
		return;

	if (pUser[autoID].TradeMode == 0)
		return;

	if (pMob[conn].TargetX >= pMob[autoID].TargetX - VIEWGRIDX && pMob[conn].TargetX <= pMob[autoID].TargetX + VIEWGRIDX
		&& pMob[conn].TargetY >= pMob[autoID].TargetY - VIEWGRIDY && pMob[conn].TargetY <= pMob[autoID].TargetY + VIEWGRIDY)
		SendAutoTrade(conn, autoID);

	else
		Log("err,too far from autotrade - _MSG_ReqTradeList", pUser[conn].AccountName, pUser[conn].IP);
}