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

void Exec_MSG_Challange(int conn, char *pMsg)
{
	MSG_STANDARDPARM *m = (MSG_STANDARDPARM*)pMsg;

	int target = m->Parm;

	if (target <= 0 || target >= MAX_MOB)
		return;

	int zone = pMob[target].MOB.BaseScore.Level;

	if (zone < 0 || zone >= ValidGuild)
		return;

	if (zone != 5)
	{
		char ChargeName[256];
		char ChallName[256];

		int charge = g_pGuildZone[zone].ChargeGuild;
		int chall = g_pGuildZone[zone].ChallangeGuild;

		BASE_GetGuildName(ServerGroup, charge, ChargeName);
		BASE_GetGuildName(ServerGroup, chall, ChallName);

		if (WeekMode && WeekMode != 1 && WeekMode != 2 && WeekMode != 3)
		{
			if (WeekMode == 4)
			{
				if(pMob[conn].MOB.Guild && pMob[conn].MOB.Guild == g_pGuildZone[zone].ChargeGuild && pMob[conn].MOB.GuildLevel == 9)
				{
					long long Coin = (pMob[GuildImpostoID[zone]].MOB.Exp+1) / 1000000000;

					if(pMob[GuildImpostoID[zone]].MOB.Exp <= 0)
					{
						sprintf(temp, g_pMessageStringTable[_I64D_TOWN_TAX], pMob[target].MOB.Exp);
						SendSay(target, temp);
						return;
					}


					if(Coin == 0)
					{
						long long bGold = pMob[GuildImpostoID[zone]].MOB.Exp;
						long long fGold = pMob[GuildImpostoID[zone]].MOB.Exp + pMob[conn].MOB.Coin;

						if(fGold <= 2000000000)
						{
							SendClientMessage(conn, g_pMessageStringTable[_NN_GIVE_TOWN_TAX]);

							pMob[GuildImpostoID[zone]].MOB.Exp = 0;

							pMob[conn].MOB.Coin = (int)fGold;

							SendEtc(conn);

							sprintf(temp, "etc,imposto recolhido(1) zone:%d coin:%llu", zone, bGold);
							Log(temp, pUser[conn].AccountName, pUser[conn].IP);
							return;
						}
						else
						{
							SendClientMessage(conn, g_pMessageStringTable[_NN_Cant_get_more_than_2G]);
							return;
						}
					}

					else
					{
						STRUCT_ITEM Item;
						memset(&Item, 0, sizeof(STRUCT_ITEM));

						Item.sIndex = 4011;

						int i = 0;

						for(i = (int)Coin; i > 0; i--)
						{
							if(PutItem(conn, &Item) == 0)
								return;

							pMob[GuildImpostoID[zone]].MOB.Exp -= 1000000000;

							sprintf(temp, "etc,imposto recolhido(2) zone:%d coin:%d", zone, 1);
							Log(temp, pUser[conn].AccountName, pUser[conn].IP);
						}

						SendClientMessage(conn, g_pMessageStringTable[_NN_GIVE_TOWN_TAX]);
						return;
					}
				}
				sprintf(temp, g_pMessageStringTable[_I64D_TOWN_TAX], pMob[target].MOB.Exp);
				SendSay(target, temp);
			}
			else if (WeekMode == 5)
				SendClientSignal(conn, 0, _MSG_ReqChallange);
		}

		else
		{
			if (g_pGuildZone[zone].ChallangeGuild)
			{
				sprintf(temp, g_pMessageStringTable[_DS_S_Challanged], pChallangerMoney[zone], ChallName);
				SendSay(target, temp);
				sprintf(temp, g_pMessageStringTable[_SS_Champion_And_Challanger], ChargeName, ChallName);
				SendSay(target, temp);
			}
			else
			{
				sprintf(temp, g_pMessageStringTable[_SN_No_Challanger], ChargeName);
				SendSay(target, temp);
			}
		}
	}
}