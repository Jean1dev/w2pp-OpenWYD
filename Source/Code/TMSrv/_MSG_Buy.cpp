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

void Exec_MSG_Buy(int conn, char *pMsg)
{
	MSG_Buy *m = (MSG_Buy*)pMsg;

	if (pMob[conn].MOB.CurrentScore.Hp == 0 || pUser[conn].Mode != USER_PLAY)
	{
		SendHpMode(conn);
		AddCrackError(conn, 10, 21);
		return;
	}

	if (pUser[conn].TradeMode)
	{
		RemoveTrade(pUser[conn].Trade.OpponentID);
		RemoveTrade(conn);
		return;
	}

	if (pUser[conn].Trade.OpponentID)
	{
		RemoveTrade(conn);
		return;
	}

	int TargetID = m->TargetID;
	int TargetInvenPos = m->TargetInvenPos;
	int MyInvenPos = m->MyInvenPos;

	if (TargetID < MAX_USER || TargetID >= MAX_MOB || TargetInvenPos < 0 || TargetInvenPos >= MAX_CARRY || MyInvenPos < 0 || MyInvenPos >= MAX_CARRY)
	{
		Log("err,MSG_BUY, target of carrypos outof range", pUser[conn].AccountName, pUser[conn].IP);
		return;
	}

	if (pMob[TargetID].MOB.Merchant != 1)
	{
		SendClientMessage(conn, g_pMessageStringTable[_NN_Only_Merchant_Sells]);
		return;
	}
	if (!GetInView(conn, TargetID))
	{
		SendClientSignal(conn, ESCENE_FIELD, _MSG_CloseShop);
		return;
	}

	int itemIndex = pMob[TargetID].MOB.Carry[TargetInvenPos].sIndex;

	STRUCT_ITEM *ItemMob = &pMob[TargetID].MOB.Carry[TargetInvenPos];

	if (itemIndex <= 0 || itemIndex >= MAX_ITEMLIST)
		return;

	int Donate = BASE_GetItemAbility(ItemMob, EF_DONATE);

	if(Donate)
	{
		if (Donate > pUser[conn].Donate)
		{
			sprintf(temp, "Você precisa de %d Cosmo Energia", Donate);
			SendClientMessage(conn, temp);
			return;
		}

		if (itemIndex == 0)
		{
			Log("err,buy request null item from shop donate - MSG_BUY", pUser[conn].AccountName, pUser[conn].IP);
			return;
		}

		if (pMob[conn].MOB.Carry[m->MyInvenPos].sIndex != 0)
		{
			SendItem(conn, ITEM_PLACE_CARRY, m->MyInvenPos, &pMob[conn].MOB.Carry[m->MyInvenPos]);
			return;
		}

		pUser[conn].Donate -= Donate;

		sprintf(temp, "%s comprou o item %s por %d e restou %d Cosmo Energia", pMob[conn].MOB.MobName, g_pItemList[itemIndex].Name, Donate, pUser[conn].Donate);
		ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

		goto DonateBuy;
	}

	int Price = g_pItemList[itemIndex].Price;
	int Price2 = Price;
	int Village = BASE_GetVillage(pMob[TargetID].TargetX, pMob[TargetID].TargetY);
	int CityTax = g_pGuildZone[Village].CityTax;

	if (Village < 0 || Village >= 5 || CityTax <= 0 || CityTax >= 30)
		goto LABEL_BUY1;

	if (Price < 100000)
		Price += Price * CityTax / 100;

	else
		Price += Price * CityTax / 100;

	if (Price < 0)
		return;

	int GuildTax = (Price - Price2) / 2;
LABEL_BUY1:

	int bPrice = Price;
	int Desconto = 0;
	int TargetVillage = BASE_GetVillage(pMob[TargetID].TargetX, pMob[TargetID].TargetY);
	int Guild = pMob[conn].MOB.Guild;
					 
	if (Price <= 1999999999 && TargetVillage >= 0 && TargetVillage < 5 && Guild > 0 && g_pGuildZone[TargetVillage].ChargeGuild == Guild)
	{
		Desconto = 30;
		Price = 7 * Price / 10;
	}

	if (Price <= 1999999999 && !Desconto && pMob[conn].MOB.Class == 3 && pMob[conn].MOB.LearnedSkill & 0x800)
	{
		int special = (pMob[conn].MOB.CurrentScore.Special[2] / 10) + 6;

		if ((pMob[conn].MOB.CurrentScore.Special[2] / 10) + 6 >= 26)
			special = 26;

		Desconto = special;
		special /= 2;

	}

	if (Price > pMob[conn].MOB.Coin)
	{
		SendClientMessage(conn, g_pMessageStringTable[_NN_Not_Enough_Money]);
		return;
	}

	if (itemIndex == 0)
	{
		Log("err,buy request null item from shop - MSG_BUY", pUser[conn].AccountName, pUser[conn].IP);
		return;
	}

	if (pMob[conn].MOB.Carry[m->MyInvenPos].sIndex != 0)
	{
		SendItem(conn, ITEM_PLACE_CARRY, m->MyInvenPos, &pMob[conn].MOB.Carry[m->MyInvenPos]);
		return;
	}

	if (itemIndex == 508 || itemIndex == 509 || itemIndex == 446 || itemIndex >= 526 && itemIndex <= 531)
	{
		if (ServerIndex == -1)
		{
			Log("err,Buy Guild item, but, Server index undefined", pUser[conn].AccountName, pUser[conn].IP);
			return;
		}

		if (!GuildCounter)
		{
			Log("err,Buy Guild item, but, Guild counter zero", pUser[conn].AccountName, pUser[conn].IP);
			return;
		}

		if (itemIndex == 508 || itemIndex == 446 || itemIndex >= 526 && itemIndex <= 531)
		{
			if (!pMob[conn].MOB.GuildLevel)
			{
				SendClientMessage(conn, g_pMessageStringTable[114]);
				return;
			}
			if (pMob[conn].MOB.GuildLevel < 2 && itemIndex >= 526 && itemIndex <= 531)
			{
				SendClientMessage(conn, g_pMessageStringTable[114]);
				return;
			}
			ItemMob->stEffect[0].cEffect = 56;
			ItemMob->stEffect[0].cValue = Guild;
			ItemMob->stEffect[1].cEffect = 57;
			ItemMob->stEffect[1].cValue = Guild;

			if (itemIndex == 508)
			{
				int sub = BASE_GetSubGuild(pMob[conn].MOB.Equip[12].sIndex);

				ItemMob->stEffect[2].cEffect = 85;
				ItemMob->stEffect[2].cValue = sub;

				if (sub >= 1 && sub <= 3)
					ItemMob->sIndex = sub + 531;
			}
		}

		if (itemIndex == 509)
		{
			ItemMob->stEffect[0].cEffect = 56;
			ItemMob->stEffect[0].cValue = (GuildCounter + (ServerIndex << 12)) >> 8;

			ItemMob->stEffect[1].cEffect = 57;
			ItemMob->stEffect[1].cValue = GuildCounter;

			ItemMob->stEffect[2].cEffect = 59;
			ItemMob->stEffect[2].cValue = rand();

			int GCount = GuildCounter + (ServerIndex << 12);

			sprintf(temp, "sys,guild medal value:%d count:%d", GCount, GuildCounter);
			Log(temp, pUser[conn].AccountName, pUser[conn].IP);

			GuildCounter++;

			if (GuildCounter >= 4096)
			{
				GuildCounter = 2048;
				Log("err,GuildCounter Restarted.", pUser[conn].AccountName, pUser[conn].IP);
			}
			CReadFiles::WriteGuild();
		}
	}


	if (Price >= 0)
	{
		int x = 0;

		for (x = 0; x < MAX_MOB_MERC; x++)
		{
			//Verifica se o vendedor é um mob mercador especial.
			if (pMob[TargetID].GenerateIndex == pMobMerc[x].GenerateIndex)
				break;
		}

		if (x != MAX_MOB_MERC)
		{//O Vendedor é um mob mercador especial, portanto ele verifica se ainda possui unidades daquele item 
		 //se tiver ele diminui do stock caso contrário ele avisa que não tem.
			if (pMobMerc[x].Stock[TargetInvenPos] == 0)
			{
				SendClientMessage(conn, g_pMessageStringTable[_NN_NOSTOCK]);
				return;
			}
			pMobMerc[x].Stock[TargetInvenPos]--;
		}

		if (TargetVillage >= 0 && TargetVillage < 5 && CityTax > 0 && CityTax < 30)
		{
			if (GuildImpostoID[TargetVillage] >= MAX_USER && GuildImpostoID[TargetVillage] < MAX_MOB && pMob[GuildImpostoID[TargetVillage]].Mode != USER_EMPTY && pMob[GuildImpostoID[TargetVillage]].MOB.Guild == g_pGuildZone[TargetVillage].ChargeGuild)
			{
				if (pMob[GuildImpostoID[TargetVillage]].MOB.Exp < 200000000000)
					pMob[GuildImpostoID[TargetVillage]].MOB.Exp += GuildTax / 2;


			}
			if (GuildImpostoID[4] >= MAX_USER && GuildImpostoID[4] < MAX_MOB && GuildTax > 0 && pMob[GuildImpostoID[4]].Mode != USER_EMPTY && pMob[GuildImpostoID[4]].MOB.Guild == g_pGuildZone[4].ChargeGuild)
			{
				if (pMob[GuildImpostoID[4]].MOB.Exp < 200000000000)
					pMob[GuildImpostoID[4]].MOB.Exp += GuildTax;

			}
		}
		pMob[conn].MOB.Coin -= Price;
						 
		m->Coin = pMob[conn].MOB.Coin;
						 
		m->ID = ESCENE_FIELD;
						 
		pUser[conn].cSock.AddMessage((char*)m, m->Size);

		SendEtc(conn);
DonateBuy:

		memcpy(&pMob[conn].MOB.Carry[MyInvenPos], ItemMob, sizeof(STRUCT_ITEM));
		//PutItem(conn, ItemMob);
		
		char tmplog[2048];
		BASE_GetItemCode(ItemMob, tmplog);

		sprintf(temp, "buy_npc,npc:%s price:%d item:%s", pMob[TargetID].MOB.MobName, Price, tmplog);
		ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);				 
		
		SendItem(conn, ITEM_PLACE_CARRY, m->MyInvenPos, &pMob[conn].MOB.Carry[m->MyInvenPos]);
						 
		if (FREEEXP == -2)
		{
			sprintf(temp, "%s base:%d tax:%d(%d%%) discount:%d(%d%%)", g_pItemList[itemIndex].Name, Price2, bPrice, CityTax, Price, Desconto);
			SendSay(TargetID, temp);
		}

		if (itemIndex == 787 || itemIndex == 481)
		{
			pMob[TargetID].PotionCount++;

			if (pMob[TargetID].PotionCount > POTIONCOUNT)
				DeleteMob(TargetID, 3);
		}
		if (pMob[TargetID].MOB.Equip[0].sIndex == 259 || pMob[TargetID].MOB.Equip[0].sIndex == 230)
			DeleteMob(TargetID, 3);
	}
	else
		Log("err,MSG_ReqBuy  <><><><><><> MINUS <><><><><><>", pUser[conn].AccountName, pUser[conn].IP);
}