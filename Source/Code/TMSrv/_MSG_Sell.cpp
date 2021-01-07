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

void Exec_MSG_Sell(int conn, char *pMsg)
{
	MSG_Sell *m = (MSG_Sell*)pMsg;

	if (pMob[conn].MOB.CurrentScore.Hp == 0 || pUser[conn].Mode != USER_PLAY)
	{
		SendHpMode(conn);
		AddCrackError(conn, 10, 23);
		return;
	}

	if (pUser[conn].Trade.OpponentID)
	{
		RemoveTrade(pUser[conn].Trade.OpponentID);
		RemoveTrade(conn);
		return;
	}

	if (pUser[conn].TradeMode && m->MyType == ITEM_PLACE_CARGO)
	{
		SendClientMessage(conn, g_pMessageStringTable[_NN_CantWhenAutoTrade]);
		return;
	}

	int sTargetID = m->TargetID;
	int sMyType = m->MyType;
	int sMyPos = m->MyPos;
	int isEquip = 0;

	if (sMyType)
	{
		if (sMyType == 1)
		{
			if (sMyPos < 0 || sMyPos >= MAX_CARRY-4)
			{
				Log("err,(Wrong dest position)Sell-Carry", pUser[conn].AccountName, pUser[conn].IP);
				return;
			}
		}
		else
		{
			if (sMyType != 2)
			{
				Log("err,sell mytype", pUser[conn].AccountName, pUser[conn].IP);
				return;
			}
			if (sMyPos < 0 || sMyPos >= MAX_CARGO)
			{
				Log("err,(Wrong dest position)Sell-Cargo", pUser[conn].AccountName, pUser[conn].IP);
				return;
			}
		}
	}
	else
	{
		if (sMyPos <= 0 || sMyPos >= MAX_EQUIP)
		{
			Log("err,(Wrong dest position)Sell-Equip", pUser[conn].AccountName, pUser[conn].IP);
			return;
		}
		if (sMyPos < 8)
			isEquip = 1;
	}

	STRUCT_ITEM *Item = GetItemPointer(&pMob[conn].MOB, pUser[conn].Cargo, m->MyType, m->MyPos);

	if (Item == NULL)
	{
		Log("err,sell target NULL", pUser[conn].AccountName, pUser[conn].IP);
		return;
	}

	if (Item->sIndex == 3193 || Item->sIndex == 3194 || Item->sIndex == 747)
		return;
					  

	if (pMob[conn].MOB.Class == 3 && pMob[conn].MOB.LearnedSkill & 0x1000)
		goto LABEL_S1;

	if (sTargetID < MAX_USER || sTargetID >= MAX_MOB)
	{
		Log("err,sell target of carrypos outof range", pUser[conn].AccountName, pUser[conn].IP);
		return;
	}
	if (pMob[sTargetID].MOB.Merchant != 1)
	{
		SendClientMessage(conn, g_pMessageStringTable[_NN_Only_Merchant_Buy_Yours]);
		return;
	}
	if (!GetInView(conn, sTargetID))
	{
		SendClientSignal(conn, ESCENE_FIELD, _MSG_CloseShop);
		return;
	}
					  
LABEL_S1:

	char SellName[256];

	strncpy(SellName, pMob[sTargetID].MOB.MobName, NAME_LENGTH);

	if (Item->sIndex <= 0 || Item->sIndex >= MAX_ITEMLIST)
	{
		sprintf(temp, "err,request sell %d %d %d %d", Item->sIndex, sTargetID, sMyType, sMyPos);
						  
		if (sMyType != 1 || sMyPos < 0 || sMyPos >= MAX_CARRY)
			Log(temp, pUser[conn].AccountName, pUser[conn].IP);
		else
			SendItem(conn, ITEM_PLACE_CARRY, sMyPos, &pMob[conn].MOB.Carry[sMyPos]);

		return;
	}

	int Vol = BASE_GetItemAbility(Item, EF_VOLATILE);
	
	if (Vol == 1)
	{
		SendClientMessage(conn, g_pMessageStringTable[_NN_Cant_Sell_Potion]);
		return;
	}

	int sPrice = g_pItemList[Item->sIndex].Price;
	int sIndex = Item->sIndex;
					 
	sPrice /= 4;
					  
	if (sPrice <= 10000)
	{
		if (sPrice > 5000)
			sPrice = 2 * sPrice / 3;
	}
	else
		sPrice /= 2;

	if (sPrice > 1000 && sIndex != 693 && sIndex != 694 && sIndex != 695 && pMob[conn].MOB.Class == 3 && pMob[conn].MOB.LearnedSkill & 0x200)
	{
		int special = pMob[conn].MOB.CurrentScore.Special[2];
		special /= 10;

		int discount = special + 6;
		if (special + 6 >= 26)
			discount = 26;

		if (discount >= 5 && discount <= 26)
		{
			discount /= 2;
			sPrice = (discount + 50) * sPrice / 50;
		}
	}

	if (sPrice < 0)
	{
		Log("err,MSG_ReqSell  <><><><><><> MINUS <><><><><><>", pUser[conn].AccountName, pUser[conn].IP);
		return;
	}


	int sVillage = BASE_GetVillage(pMob[sTargetID].TargetX, pMob[sTargetID].TargetY);
	if (sVillage < 0 || sVillage > 5)
		goto LABEL_S2;


	int CityTax = g_pGuildZone[sVillage].CityTax;

	if (CityTax >= 0 && CityTax <= 30)
	{
		int fPrice = sPrice;
						  
		if (sPrice < 100000)
			sPrice = sPrice * (100 - CityTax) / 100;
						  
		else
			sPrice = (100 - CityTax) * sPrice / 100;
						  
		if (sPrice >= 0)
		{
			int tcoin = (fPrice - sPrice) / 2;
			tcoin /= 2;

			if (GuildImpostoID[sVillage] >= MAX_USER && GuildImpostoID[sVillage] < MAX_MOB  && tcoin > 0  && pMob[GuildImpostoID[sVillage]].Mode != MOB_EMPTY)
			{
				if (pMob[GuildImpostoID[sVillage]].MOB.Exp < 200000000000)
					pMob[GuildImpostoID[sVillage]].MOB.Exp += tcoin;
								  

			}

			if (GuildImpostoID[4] >= MAX_USER && GuildImpostoID[4] < MAX_MOB && tcoin > 0  && pMob[GuildImpostoID[4]].Mode != MOB_EMPTY)
			{
				if (pMob[GuildImpostoID[4]].MOB.Exp < 20000000000000)
					pMob[GuildImpostoID[4]].MOB.Exp += tcoin;
			}

		LABEL_S2:

			if ((pMob[conn].MOB.Coin + sPrice) < 2000000000)
				pMob[conn].MOB.Coin += sPrice;
			else
				pMob[conn].MOB.Coin = 2000000000;

			char tmplog[2048];
			BASE_GetItemCode(Item, tmplog);

			sprintf(temp, "sel_npc,npc:%s price:%d item:%s", SellName, sPrice, tmplog);
			Log(temp, pUser[conn].AccountName, pUser[conn].IP);

			BASE_ClearItem(Item);
							  
			pUser[conn].cSock.AddMessage((char*)m, sizeof(MSG_Sell));

			if (isEquip)
				SendEquip(conn, conn);

			SendEtc(conn);
		}
	}
}