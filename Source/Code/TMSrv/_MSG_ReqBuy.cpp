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

void Exec_MSG_ReqBuy(int conn, char *pMsg)
{
	MSG_ReqBuy *m = (MSG_ReqBuy*)pMsg;

	if (pMob[conn].MOB.CurrentScore.Hp == 0 || pUser[conn].Mode != USER_PLAY)
	{
		SendHpMode(conn);
		AddCrackError(conn, 10, 86);
		RemoveTrade(conn);
		return;
	}

	if (pUser[conn].TradeMode)
	{
		RemoveTrade(conn);
		return;
	}

	if (pUser[conn].Trade.OpponentID)
	{
		RemoveTrade(conn);
		return;
	}

	int targetID = m->TargetID;
	int itemPrice = m->Price;
	int itemTax = m->Tax;
	int itemPos = m->Pos;

	if (targetID <= 0 || targetID >= MAX_USER)
	{
		RemoveTrade(conn);
		return;
	}

	if (pUser[conn].Mode != USER_PLAY)
	{
		RemoveTrade(conn);
		return;
	}

	if (pMob[conn].TargetX >= pMob[targetID].TargetX - VIEWGRIDX && pMob[conn].TargetX <= pMob[targetID].TargetX + VIEWGRIDX&& pMob[conn].TargetY >= pMob[targetID].TargetY - VIEWGRIDY && pMob[conn].TargetY <= pMob[targetID].TargetY + VIEWGRIDY)
	{
		if (itemPos < 0 || itemPos >= MAX_AUTOTRADE)
			return;

		int StorageSlot = pUser[targetID].AutoTrade.CarryPos[itemPos];

		if (StorageSlot < 0 || StorageSlot >= MAX_CARGO)
			return;

		if (itemTax != pUser[targetID].AutoTrade.Tax)
			return;

		if (itemPrice != pUser[targetID].AutoTrade.Coin[itemPos])
			return;

		if (memcmp(&m->item, &pUser[targetID].AutoTrade.Item[itemPos], sizeof(STRUCT_ITEM)))
		{
			RemoveTrade(conn);
			return;
		}

		if (memcmp(&pUser[targetID].Cargo[StorageSlot], &pUser[targetID].AutoTrade.Item[itemPos], sizeof(STRUCT_ITEM)))
		{
			RemoveTrade(conn);
			return;
		}

						
		if (pMob[conn].MOB.Coin < itemPrice)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Not_Enough_Money]);
			return;
		}

		unsigned int xcoin = pUser[targetID].Coin + itemPrice;

		if (xcoin > 2000000000)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Cant_get_more_than_2G]);
			return;
		}

		int i = 0;

		for (i = 0; i < MAX_CARRY-4; i++)
		{
			if (pMob[conn].MOB.Carry[i].sIndex != 0)
				continue;

			break;
		}

		if (i == MAX_CARRY)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_You_Have_No_Space_To_Trade]);
			return;
		}

		int target_village = BASE_GetVillage(pMob[targetID].TargetX, pMob[targetID].TargetY);

		if (target_village < 0 || target_village >= 5)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_OnlyVillage]);
			return;
		}

		memcpy(&pMob[conn].MOB.Carry[i], &pUser[targetID].Cargo[StorageSlot], sizeof(STRUCT_ITEM));
		SendItem(conn, ITEM_PLACE_CARRY, i, &pMob[conn].MOB.Carry[i]);

		char tmplog[2048];
		BASE_GetItemCode(&m->item, tmplog);
								
		sprintf(temp, "autotrade_buy,target_name:%s price:%d item:%s", pUser[targetID].AccountName, itemPrice, tmplog);
		ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

		int imposto = 0;
		int price_end = itemPrice;
							
		if (itemPrice >= 100000)
		{
			imposto = (itemPrice / 100) *  itemTax;
			price_end = itemPrice - imposto;
		}

		pUser[targetID].AutoTrade.CarryPos[itemPos] = -1;

		memset(&pUser[targetID].AutoTrade.Item, 0, sizeof(STRUCT_ITEM));
						
		pUser[targetID].AutoTrade.Coin[itemPos] = 0;
							
		memset(&pUser[targetID].Cargo[StorageSlot], 0, sizeof(STRUCT_ITEM));
							
		SendItem(targetID, ITEM_PLACE_CARGO, StorageSlot, &pUser[targetID].Cargo[StorageSlot]);
							
		pMob[conn].MOB.Coin -= itemPrice;

		if (pUser[targetID].Coin < 2000000000)
			pUser[targetID].Coin += price_end;

		SendEtc(conn);
		SendCargoCoin(targetID);
		SaveUser(targetID, 1);

		if (target_village >= 0 && target_village < 5)
		{
			if (GuildImpostoID[target_village] >= MAX_USER && GuildImpostoID[target_village] < MAX_MOB && imposto > 0
				&& pMob[GuildImpostoID[target_village]].Mode != MOB_EMPTY && pMob[GuildImpostoID[target_village]].MOB.Guild == g_pGuildZone[target_village].ChargeGuild)
			{
				if (pMob[GuildImpostoID[target_village]].MOB.Exp < 200000000000)
					pMob[GuildImpostoID[target_village]].MOB.Exp += imposto;
			}
			if (GuildImpostoID[4] >= MAX_USER && GuildImpostoID[4] < MAX_MOB && imposto > 0 && pMob[GuildImpostoID[4]].Mode != MOB_EMPTY
				&& pMob[GuildImpostoID[4]].MOB.Guild == g_pGuildZone[4].ChargeGuild)
			{
				if (pMob[GuildImpostoID[4]].MOB.Exp < 200000000000)
					pMob[GuildImpostoID[4]].MOB.Exp += imposto;

			}
		}

		MSG_STANDARDPARM2 sm_is;
		memset(&sm_is, 0, sizeof(MSG_STANDARDPARM2));

		sm_is.Type = _MSG_ItemSold;
		sm_is.ID = ESCENE_FIELD;
		sm_is.Parm1 = targetID;
		sm_is.Parm2 = itemPos;
		sm_is.Size = sizeof(MSG_STANDARDPARM2);

		GridMulticast(pMob[targetID].TargetX, pMob[targetID].TargetY, (MSG_STANDARD*)&sm_is, 0);
		SendClientMessage(targetID, g_pMessageStringTable[_NN_ItemSold]);
	}

	else
	{
		Log("err,too far from autotrade - _MSG_ReqBuy", pUser[conn].AccountName, pUser[conn].IP);
		return;
	}
}