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

void Exec_MSG_ReqTeleport(int conn, char *pMsg)
{
	int posx = pMob[conn].TargetX;
	int posy = pMob[conn].TargetY;

	if ((posx / 4) == 491 && (posy / 4) == 443)
	{
		SendClientMessage(conn, g_pMessageStringTable[_NN_Only_By_Water_Scroll]);
		return;
	}

	int reqcoin = GetTeleportPosition(conn, &posx, &posy);

	int clan = pMob[conn].MOB.Clan;

	if (g_pGuildZone[4].Clan == clan)
		goto label_tel;
						
	if (reqcoin <= pMob[conn].MOB.Coin)
	{
		if (reqcoin > 0)
		{
			pMob[conn].MOB.Coin -= reqcoin;
			SendEtc(conn);

			int master = GuildImpostoID[4];
			if (GuildImpostoID[4] >= MAX_USER && master < MAX_MOB && pMob[master].Mode != MOB_EMPTY && pMob[master].MOB.Guild == g_pGuildZone[4].ChargeGuild)
			{
					if (pMob[master].MOB.Exp < 2000000000000)
						pMob[master].MOB.Exp += reqcoin / 2;
			}
		}

	label_tel:
		if (posx != pMob[conn].TargetX || posy != pMob[conn].TargetY)
		{
			DoTeleport(conn, posx, posy);
			
			sprintf(temp, "etc,reqteleport char:%s X:%d Y:%d price:%d", pMob[conn].MOB.MobName, pMob[conn].TargetX, pMob[conn].TargetY, reqcoin);
			Log(temp, pUser[conn].AccountName, pUser[conn].IP);
		}
	}
	else
		SendClientMessage(conn, g_pMessageStringTable[_NN_Not_Enough_Money]);
}