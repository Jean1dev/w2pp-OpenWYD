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

void Exec_MSG_ReqRanking(int conn, char *pMsg)
{
	MSG_STANDARDPARM2 *m = (MSG_STANDARDPARM2*)pMsg;

	int tDuel = m->Parm1;
	int DuelParm = m->Parm2;

	if (DuelParm < 0 || DuelParm > 4 || tDuel <= 0 || tDuel >= MAX_USER)
		return;

	if (pUser[tDuel].Whisper)
	{
		SendClientMessage(conn, g_pMessageStringTable[_NN_Deny_Whisper]);
		return;
	}

	if (DuelParm != 4)
	{
		if (DuelParm != 1 && DuelParm != 2 || pMob[conn].MOB.GuildLevel && pMob[tDuel].MOB.GuildLevel)
		{
			pUser[conn].RankingTarget = tDuel;
			pUser[conn].RankingType = DuelParm;
									
			m->Parm1 = conn;
			m->ID = tDuel;
									
			pUser[tDuel].cSock.AddMessage((char*)m, sizeof(MSG_STANDARDPARM2));
			return;
		}
	}

	else
	{
		if (pUser[tDuel].Mode != USER_PLAY)
			return;

		if (pUser[tDuel].RankingTarget != conn)
			return;

		if (RankingProgress)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Battle_In_Progress]);
			SendClientMessage(tDuel, g_pMessageStringTable[_NN_Battle_In_Progress]);
			return;
		}

		if (pUser[tDuel].RankingType >= 0 && pUser[tDuel].RankingType <= 3)
		{
			SendEtc(conn);
			SendEtc(tDuel);
			DoRanking(pUser[tDuel].RankingType, conn, tDuel);

			sprintf(temp, "etc,reqranking %s vs %s type:%d", pMob[conn].MOB.MobName, pMob[tDuel].MOB.MobName, DuelParm);
			Log(temp, pUser[conn].AccountName, pUser[conn].IP);
			return;
		}
	}
}