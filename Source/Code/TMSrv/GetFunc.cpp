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
#include <Windows.h>

#include "..\Basedef.h"
#include "..\ItemEffect.h"
#include "Language.h"

#include "Server.h"
#include "GetFunc.h"
#include "CMob.h"
#include "SendFunc.h"

extern STRUCT_ITEMLIST g_pItemList[MAX_ITEMLIST];

int GetRecycleBin(int conn, STRUCT_ITEM *item, int price)
{
	if (conn <= 0 || conn >= MAX_USER) return 0;

	if (pUser[conn].Mode != USER_PLAY) return 0;

	int i = -1;

	for (int k = 0; k < _Max_Recycle_Bin_; k++)
	{
		auto My = pMob[conn].Recycle[k];

		if (My.Item.sIndex <= 0 || My.Item.sIndex >= MAX_ITEMLIST)
		{
			i = k;

			break;
		}
	}

	if (i == -1)
	{
		for (int k = 0; k < _Max_Recycle_Bin_ - 1; k++)
		{
			pMob[conn].Recycle[k] = pMob[conn].Recycle[k + 1];
		}

		i = 9;

		pMob[conn].Recycle[i].Price = price;
		memcpy(&pMob[conn].Recycle[i].Item, item, sizeof(STRUCT_ITEM));
		pMob[conn].Recycle[i].Ticks = rand() % 100000;

		return 1;
	}

	pMob[conn].Recycle[i].Price = price;
	memcpy(&pMob[conn].Recycle[i].Item, item, sizeof(STRUCT_ITEM));
	pMob[conn].Recycle[i].Ticks = rand() % 100000;

	return 1;
}


int GetMatchCombine(STRUCT_ITEM *item)
{
	int target = item[0].sIndex;

	for(int i = 0; i < MAX_COMBINE; i++)
	{
		if(item[i].sIndex == 747)
			return 0;
	}

	if (target <= 0 || target >= MAX_ITEMLIST)
		return 0;

	int	stone = item[1].sIndex;

	if (stone <= 0 || stone >= MAX_ITEMLIST)
		return 0;

	if (g_pItemList[target].nUnique < 41 || g_pItemList[target].nUnique > 49)
		return 0;

	if (g_pItemList[target].Extra <= 0)
		return 0;

	if (BASE_GetItemAbility(&item[0], EF_MOBTYPE) == 3)
		return 0;

	int rate = 1;
	for (int j = 2; j < MAX_COMBINE; j++)
	{
		int idx = item[j].sIndex;

		if (idx <= 0 || idx >= MAX_ITEMLIST)
			continue;

		int eq = BASE_GetItemAbility(&item[j], EF_POS);

		if (eq == 0)
			return 0;

		int il1 = BASE_GetItemAbility(&item[0], EF_ITEMLEVEL);
		int il2 = (BASE_GetItemAbility(&item[j], EF_ITEMLEVEL));

		if(il1 > il2)
			return 0;

		int	sa = BASE_GetItemSanc(&item[j]);

		if (sa == 7)
		{
			rate = rate + g_pAnctChance[0];

			continue;
		}
		if (sa == 8)
		{
			rate = rate + g_pAnctChance[1];

			continue;
		}
		if (sa == 9)
		{
			rate = rate + g_pAnctChance[2];

			continue;
		}

		return 0;
	}

	return rate;
}

int GetAttackMiss(int conn, int target)
{
	int Min = pMob[target].MOB.CurrentScore.Dex;
	int Max = pMob[conn].MOB.CurrentScore.Dex;

	int Res = 1;
	int Rsv = 75;

	int EvE = pMob[conn].MOB.Rsv;

	if (EvE & 0x20)		Max += 100;

	if (EvE & 0x40)		Max += 100;

	if (EvE & 0x80)		Max += 50;

	if (EvE & 0x200)	Max += 50;

	if (Min > Max)
	{
		Res = (Min - Max) / 100;

		Res *= 3; //3

		Res = Res > Rsv ? Rsv : Res;
	}

	if (Max > Min)
	{
		Res = (Max - Min) / 100;

		if (Res > 10)	Res = 10;

	}

	return Res;
}

int GetMatchCombineEhre(STRUCT_ITEM *Item)
{
	int rate = 0;

	for(int i = 0; i < MAX_COMBINE; i++)
	{
		if(Item[i].sIndex == 747)
			return 0;
	}

	if (Item[0].sIndex < 0 || Item[0].sIndex >= MAX_ITEMLIST)
		return 0;

	if (Item[1].sIndex < 0 || Item[1].sIndex >= MAX_ITEMLIST)
		return 0;

	if (Item[2].sIndex < 0 || Item[2].sIndex >= MAX_ITEMLIST)
		return 0;

	if (Item[3].sIndex < 0 || Item[3].sIndex >= MAX_ITEMLIST)
		return 0;

	if (Item[4].sIndex < 0 || Item[4].sIndex >= MAX_ITEMLIST)
		return 0;

	if (Item[5].sIndex < 0 || Item[5].sIndex >= MAX_ITEMLIST)
		return 0;

	if (Item[6].sIndex < 0 || Item[6].sIndex >= MAX_ITEMLIST)
		return 0;

	if (Item[7].sIndex < 0 || Item[7].sIndex >= MAX_ITEMLIST)
		return 0;
	
	if (Item[0].sIndex == 697 && Item[1].sIndex == 697 && BASE_GetItemSanc(&Item[2]) >= 9 && Item[2].sIndex != 3338)
		rate = 1;//Pacote Oriharucon

	else if (Item[0].sIndex >= 5110 && Item[0].sIndex <= 5133 && Item[1].sIndex >= 5110 && Item[1].sIndex <= 5133 && Item[2].sIndex == 413 && BASE_GetItemAmount(&Item[2]) >= 10)
		rate = 2;//Pedra Misteriosa

	else if (Item[0].sIndex >= 661 && Item[0].sIndex <= 663 && Item[1].sIndex >= 661 && Item[1].sIndex <= 663 && Item[2].sIndex == 633 && BASE_GetItemSanc(&Item[2]) >= 9)
		rate = 3;//Pedra espiritual

	else if (Item[0].sIndex >= 661 && Item[0].sIndex <= 663 && Item[1].sIndex >= 661 && Item[1].sIndex <= 663 && Item[2].sIndex == 3464 && BASE_GetItemSanc(&Item[2]) >= 9)
		rate = 4;//Pedra Amunrá

	else if (Item[0].sIndex == 697 && Item[1].sIndex == 697 && Item[2].sIndex == 3338 && BASE_GetItemSanc(&Item[2]) <= 8)
		rate = 5;//Refinação abençoada Purificada

	else if (Item[0].sIndex >= 2360 && Item[0].sIndex <= 2389 && Item[1].sIndex >= 4190 && Item[1].sIndex <= 4199)
		rate = 6;//Traje montaria

	else if (Item[0].sIndex >= 2360 && Item[0].sIndex <= 2389 && Item[1].sIndex == 4899)
		rate = 7;//Restaurar Traje montaria

	else if (Item[0].sIndex >= 2441 && Item[0].sIndex <= 2444 && Item[1].sIndex >= 2441 && Item[1].sIndex <= 2444 && Item[2].sIndex >= 2441 && Item[2].sIndex <= 2444)
		rate = 8;//Soul

	return rate;
}

int GetMatchCombineTiny(STRUCT_ITEM *Item)
{
	for(int i = 0; i < MAX_COMBINE; i++)
	{
		if(Item[i].sIndex == 747)
			return 0;
	}

	if (Item[0].sIndex <= 0 || Item[0].sIndex >= MAX_ITEMLIST)
		return 0;

	if (Item[1].sIndex <= 0 || Item[1].sIndex >= MAX_ITEMLIST)
		return 0;

	if (Item[2].sIndex <= 0 || Item[2].sIndex >= MAX_ITEMLIST)
		return 0;

	if (BASE_GetItemAbility(&Item[0], EF_MOBTYPE) != ARCH)
		return 0;

	if (g_pItemList[Item[0].sIndex].Grade < 5 || g_pItemList[Item[0].sIndex].Grade > 8)
		return 0;

	if (BASE_GetItemAbility(&Item[1], EF_ITEMTYPE) != 4 || BASE_GetItemAbility(&Item[1], EF_ITEMTYPE) != 5)
		return 0;

	if (g_pItemList[Item[1].sIndex].Grade < 5 || g_pItemList[Item[1].sIndex].Grade > 8)
		return 0;

	if (BASE_GetItemAbility(&Item[2], EF_ITEMTYPE) != 4 || BASE_GetItemAbility(&Item[2], EF_ITEMTYPE) != 5)
		return 0;

	if (g_pItemList[Item[0].sIndex].nPos != g_pItemList[Item[1].sIndex].nPos)
		return 0;

	if(g_pItemList[Item[0].sIndex].nPos != 64 && g_pItemList[Item[0].sIndex].nPos != 192)
		return 0;

	if (BASE_GetItemSanc(&Item[0]) < 9)
		return 0;

	if (BASE_GetItemSanc(&Item[1]) < 9)
		return 0;

	if (BASE_GetItemSanc(&Item[2]) < 9)
		return 0;

	return g_pTinyBase + (BASE_GetItemAbility(&Item[1], EF_ITEMLEVEL) * 5);
}

int GetMatchCombineShany(STRUCT_ITEM *Item)
{
	for (int i = 0; i < MAX_COMBINE; i++)
	{
		if (Item[i].sIndex == 747)
			return 0;
	}

	if (Item[0].sIndex <= 0 || Item[0].sIndex >= MAX_ITEMLIST)
		return 0;

	if (Item[1].sIndex <= 0 || Item[1].sIndex >= MAX_ITEMLIST)
		return 0;

	if (Item[2].sIndex <= 0 || Item[2].sIndex >= MAX_ITEMLIST)
		return 0;

	if (Item[3].sIndex <= 0 || Item[3].sIndex >= MAX_ITEMLIST)
		return 0;

	if (Item[4].sIndex <= 0 || Item[4].sIndex >= MAX_ITEMLIST)
		return 0;

	if (Item[5].sIndex <= 0 || Item[5].sIndex >= MAX_ITEMLIST)
		return 0;

	if (Item[6].sIndex <= 0 || Item[6].sIndex >= MAX_ITEMLIST)
		return 0;

	if (Item[0].sIndex != 540 && Item[0].sIndex != 541 || BASE_GetItemSanc(&Item[0]) < 9)
		return 0;

	if (Item[1].sIndex != 540 && Item[1].sIndex != 541 || BASE_GetItemSanc(&Item[1]) < 9)
		return 0;

	if (Item[2].sIndex != 540 && Item[2].sIndex != 541 && Item[2].sIndex != 633)
		return 0;

	if (Item[3].sIndex != 413 || Item[4].sIndex != 413 || Item[5].sIndex != 413 || Item[6].sIndex != 413)
		return 0;

	return 1;
}

int GetMatchCombineAilyn(STRUCT_ITEM *Item)
{
	for(int i = 0; i < MAX_COMBINE; i++)
	{
		if(Item[i].sIndex == 747)
			return 0;
	}

	if (Item[0].sIndex <= 0 || Item[0].sIndex >= MAX_ITEMLIST)
		return 0;

	if (Item[1].sIndex <= 0 || Item[1].sIndex >= MAX_ITEMLIST)
		return 0;

	if (Item[2].sIndex <= 0 || Item[2].sIndex >= MAX_ITEMLIST)
		return 0;

	if (Item[3].sIndex <= 0 || Item[3].sIndex >= MAX_ITEMLIST)
		return 0;

	if (Item[4].sIndex <= 0 || Item[4].sIndex >= MAX_ITEMLIST)
		return 0;

	if (Item[5].sIndex <= 0 || Item[5].sIndex >= MAX_ITEMLIST)
		return 0;

	if (Item[6].sIndex <= 0 || Item[6].sIndex >= MAX_ITEMLIST)
		return 0;

	if (Item[0].sIndex != Item[1].sIndex)
		return 0;

	if (g_pItemList[Item[0].sIndex].Grade != g_pItemList[Item[1].sIndex].Grade)
		return 0;

	if (Item[2].sIndex != 1774)
		return 0;

	int rate = 1;

	if((g_pItemList[Item[0].sIndex].nPos != 2 && g_pItemList[Item[0].sIndex].nPos != 4 && g_pItemList[Item[0].sIndex].nPos != 8 && g_pItemList[Item[0].sIndex].nPos != 16 && g_pItemList[Item[0].sIndex].nPos != 32 && g_pItemList[Item[0].sIndex].nPos != 64 && g_pItemList[Item[0].sIndex].nPos != 192 && g_pItemList[Item[0].sIndex].nPos != 128))
		return 0;

	if (g_pItemList[Item[0].sIndex].Grade == 5)
	{
		for (int i = 3; i < 7; i++)
		{
			if (Item[i].sIndex == 0)
				continue;

			if (Item[i].sIndex != 2441)
				return 0;

			rate += g_pAilynBase;
		}
	}
	
	else if (g_pItemList[Item[0].sIndex].Grade == 6)
	{
		for (int i = 3; i < 7; i++)
		{
			if (Item[i].sIndex == 0)
				continue;

			if (Item[i].sIndex != 2442)
				return 0;

			rate += g_pAilynBase;
		}
	}

	else if (g_pItemList[Item[0].sIndex].Grade == 7)
	{
		for (int i = 3; i < 7; i++)
		{
			if (Item[i].sIndex == 0)
				continue;

			if (Item[i].sIndex != 2443)
				return 0;

			rate += g_pAilynBase;
		}
	}

	else if (g_pItemList[Item[0].sIndex].Grade == 8)
	{
		for (int i = 3; i < 7; i++)
		{
			if (Item[i].sIndex == 0)
				continue;

			if (Item[i].sIndex != 2444)
				return 0;

			rate += g_pAilynBase;
		}
	}

	else
	{
		for (int i = 3; i < 7; i++)
		{
			if (Item[i].sIndex == 0)
				continue;

			if (Item[3].sIndex != Item[i].sIndex)
				return 0;

			rate += g_pAilynBase;
		}
	}

	return rate;
}

int GetMatchCombineAgatha(STRUCT_ITEM *Item)
{
	for(int i = 0; i < MAX_COMBINE; i++)
	{
		if(Item[i].sIndex == 747)
			return 0;
	}

	if (Item[0].sIndex <= 0 || Item[0].sIndex >= MAX_ITEMLIST)
		return 0;

	if (Item[1].sIndex <= 0 || Item[1].sIndex >= MAX_ITEMLIST)
		return 0;

	if (Item[2].sIndex <= 0 || Item[2].sIndex >= MAX_ITEMLIST)
		return 0;

	if (Item[3].sIndex <= 0 || Item[3].sIndex >= MAX_ITEMLIST)
		return 0;

	if (Item[4].sIndex <= 0 || Item[4].sIndex >= MAX_ITEMLIST)
		return 0;

	if (Item[5].sIndex <= 0 || Item[5].sIndex >= MAX_ITEMLIST)
		return 0;

	if (BASE_GetItemAbility(&Item[0], EF_MOBTYPE) != ARCH)
		return 0;

	if (BASE_GetItemAbility(&Item[1], EF_ITEMTYPE) != MORTAL && BASE_GetItemAbility(&Item[1], EF_ITEMTYPE) != 0)
		return 0;

	if (BASE_GetItemAbility(&Item[1], EF_ITEMLEVEL) < 4)
		return 0;

	if (g_pItemList[Item[0].sIndex].nPos != g_pItemList[Item[1].sIndex].nPos)
		return 0;

	if (BASE_GetItemSanc(&Item[0]) < 9)
		return 0;

	if (BASE_GetItemSanc(&Item[1]) < 9)
		return 0;

	if (Item[2].sIndex != 3140 || Item[3].sIndex != 3140 || Item[4].sIndex != 3140 || Item[5].sIndex != 3140)
		return 0;

	return g_pAgathaBase + (g_pItemList[Item[1].sIndex].Grade * 5) + (BASE_GetItemAbility(&Item[1], EF_ITEMLEVEL) == 5 ? 30 : 1);
}

int GetMatchCombineDedekinto(STRUCT_ITEM* Item)
{
	int rate = 0;

	for (int i = 0; i < MAX_COMBINE; i++)
	{
		if (Item[i].sIndex == 747)
			return 0;
	}

	if (Item[0].sIndex < 0 || Item[0].sIndex >= MAX_ITEMLIST)
		return 0;

	if (Item[1].sIndex < 0 || Item[1].sIndex >= MAX_ITEMLIST)
		return 0;

	if (Item[2].sIndex < 0 || Item[2].sIndex >= MAX_ITEMLIST)
		return 0;

	if (Item[3].sIndex < 0 || Item[3].sIndex >= MAX_ITEMLIST)
		return 0;

	if (Item[4].sIndex < 0 || Item[4].sIndex >= MAX_ITEMLIST)
		return 0;

	if (Item[5].sIndex < 0 || Item[5].sIndex >= MAX_ITEMLIST)
		return 0;

	if (Item[6].sIndex < 0 || Item[6].sIndex >= MAX_ITEMLIST)
		return 0;

	if (Item[7].sIndex < 0 || Item[7].sIndex >= MAX_ITEMLIST)
		return 0;

	if (Item[0].sIndex == 673 && Item[1].sIndex == 674 && Item[2].sIndex == 675 && Item[3].sIndex == 676 && Item[4].sIndex == 677 && Item[5].sIndex == 770)
		rate = 1; //Pedra Amunra

	else if (Item[0].sIndex == 677 && Item[1].sIndex == 677 && Item[2].sIndex == 678 && Item[3].sIndex == 678)
		rate = 2; //Proteção Divina

	return rate;
}

int GetMatchCombineOdin(STRUCT_ITEM *Item)
{
	for(int i = 0; i < MAX_COMBINE; i++)
	{
		if(Item[i].sIndex == 747)
			return 0;
	}

	if (Item[0].sIndex < 0 || Item[0].sIndex >= MAX_ITEMLIST)
		return 0;

	if (Item[1].sIndex < 0 || Item[1].sIndex >= MAX_ITEMLIST)
		return 0;

	if (Item[2].sIndex < 0 || Item[2].sIndex >= MAX_ITEMLIST)
		return 0;

	if (Item[3].sIndex < 0 || Item[3].sIndex >= MAX_ITEMLIST)
		return 0;

	if (Item[4].sIndex < 0 || Item[4].sIndex >= MAX_ITEMLIST)
		return 0;

	if (Item[5].sIndex < 0 || Item[5].sIndex >= MAX_ITEMLIST)
		return 0;

	if (Item[6].sIndex < 0 || Item[6].sIndex >= MAX_ITEMLIST)
		return 0;

	if (Item[7].sIndex < 0 || Item[7].sIndex >= MAX_ITEMLIST)
		return 0;
	
	else if (Item[0].sIndex == g_pItemList[Item[1].sIndex].Extra && BASE_GetItemSanc(&Item[0]) >= 9 && BASE_GetItemSanc(&Item[1]) == REF_15
		&& (Item[2].sIndex == 542 || Item[2].sIndex == 772) && Item[3].sIndex == 5334 && Item[4].sIndex == 5335 && Item[5].sIndex == 5336 && Item[6].sIndex == 5337)
		return 1;//Item celestial

	else if ((Item[0].sIndex == 413 && BASE_GetItemAmount(&Item[0]) >= 10 && Item[1].sIndex == 413 && BASE_GetItemAmount(&Item[1]) >= 10 || Item[0].sIndex == 4043 && Item[1].sIndex == 4043)
		&& BASE_GetItemSanc(&Item[2]) > REF_10 && BASE_GetItemSanc(&Item[2]) <= REF_15 && (Item[3].sIndex >= 5334 && Item[3].sIndex <= 5337 || Item[3].sIndex == 3338) && (Item[4].sIndex >= 5334 && Item[4].sIndex <= 5337 || Item[4].sIndex == 3338)
		&& (Item[5].sIndex >= 5334 && Item[5].sIndex <= 5337 || Item[5].sIndex == 3338) && (Item[6].sIndex >= 5334 && Item[6].sIndex <= 5337 || Item[6].sIndex == 3338) && (g_pItemList[Item[2].sIndex].nPos == 2 || g_pItemList[Item[2].sIndex].nPos == 4 || g_pItemList[Item[2].sIndex].nPos == 8 || g_pItemList[Item[2].sIndex].nPos == 16 || g_pItemList[Item[2].sIndex].nPos == 32 || g_pItemList[Item[2].sIndex].nPos == 64 || g_pItemList[Item[2].sIndex].nPos == 192 || g_pItemList[Item[2].sIndex].nPos == 128))
		return 2;//Ref+12+

	else if (Item[0].sIndex == 413 && Item[1].sIndex == 413 && Item[2].sIndex == 413 && Item[3].sIndex == 413 && Item[4].sIndex == 413 && Item[5].sIndex == 413 && Item[6].sIndex == 413)
		return 3;//Pista de runas

	else if (Item[0].sIndex == 4127 && Item[1].sIndex == 4127 && Item[2].sIndex == 5135 && Item[3].sIndex == 5113 && Item[4].sIndex == 5129 && Item[5].sIndex == 5112 && Item[6].sIndex == 5110)
		return 4;//Level40 Celestial

	else if (Item[0].sIndex == 5125 && Item[1].sIndex == 5115 && Item[2].sIndex == 5111 && Item[3].sIndex == 5112 && Item[4].sIndex == 5120 && Item[5].sIndex == 5128 && Item[6].sIndex == 5119)
		return 5;//Pedra da fúria

	else if (Item[0].sIndex == 5126 && Item[1].sIndex == 5127 && Item[2].sIndex == 5121 && Item[3].sIndex == 5114 && Item[4].sIndex == 5125 && Item[5].sIndex == 5111 && Item[6].sIndex == 5118)
		return 6;//Secreta da Agua

	else if (Item[0].sIndex == 5131 && Item[1].sIndex == 5113 && Item[2].sIndex == 5115 && Item[3].sIndex == 5116 && Item[4].sIndex == 5125 && Item[5].sIndex == 5112 && Item[6].sIndex == 5114)
		return 7;//Secreta da Terra

	else if (Item[0].sIndex == 5110 && Item[1].sIndex == 5124 && Item[2].sIndex == 5117 && Item[3].sIndex == 5129 && Item[4].sIndex == 5114 && Item[5].sIndex == 5125 && Item[6].sIndex == 5128)
		return 8;//Secreta do Sol

	else if (Item[0].sIndex == 5122 && Item[1].sIndex == 5119 && Item[2].sIndex == 5132 && Item[3].sIndex == 5120 && Item[4].sIndex == 5130 && Item[5].sIndex == 5133 && Item[6].sIndex == 5123)
		return 9;//Secreta do Vento

	else if (Item[0].sIndex == 421 && Item[1].sIndex == 422 && Item[2].sIndex == 423 && Item[3].sIndex == 424 && Item[4].sIndex == 425 && Item[5].sIndex == 426 && Item[6].sIndex == 427)
		return 10;//Semente de cristal
	
	else if (Item[0].sIndex == 4127 && Item[1].sIndex == 4127 && Item[2].sIndex == 5135 && Item[3].sIndex == 413 && Item[4].sIndex == 413 && Item[5].sIndex == 413 && Item[6].sIndex == 413)
		return 11;//Capa

	return 0;
}

int GetMatchCombineAlquimia(STRUCT_ITEM *Item)
{
	for(int i = 0; i < MAX_COMBINE; i++)
	{
		if(Item[i].sIndex == 747)
			return 0;
	}
	if (Item[0].sIndex < 0 || Item[0].sIndex >= MAX_ITEMLIST)
		return -1;

	if (Item[1].sIndex < 0 || Item[1].sIndex >= MAX_ITEMLIST)
		return -1;

	if (Item[2].sIndex < 0 || Item[2].sIndex >= MAX_ITEMLIST)
		return -1;

	if (Item[3].sIndex < 0 || Item[3].sIndex >= MAX_ITEMLIST)
		return -1;

	if (Item[4].sIndex < 0 || Item[4].sIndex >= MAX_ITEMLIST)
		return -1;

	if (Item[5].sIndex < 0 || Item[5].sIndex >= MAX_ITEMLIST)
		return -1;

	if (Item[6].sIndex < 0 || Item[6].sIndex >= MAX_ITEMLIST)
		return -1;

	if (Item[7].sIndex < 0 || Item[7].sIndex >= MAX_ITEMLIST)
		return -1;
	
	if(Item[0].sIndex == 413 && Item[1].sIndex == 2441 && Item[2].sIndex == 2442)
		return 0;//Sagacidade

	if(Item[0].sIndex == 413 && Item[1].sIndex == 2443 && Item[2].sIndex == 2442)
		return 1;//Resistencia

	if(Item[0].sIndex == 4127 && Item[1].sIndex == 4127 && Item[2].sIndex == 4127)
		return 2;//Revelação

	if(Item[0].sIndex == 4127 && Item[1].sIndex == 4127 && Item[2].sIndex == 697)
		return 3;//Recuperação

	if(Item[0].sIndex == 412 && Item[1].sIndex == 2441 && Item[2].sIndex == 2444)
		return 4;//Absorção

	if(Item[0].sIndex == 412 && Item[1].sIndex == 2444 && Item[2].sIndex == 2443)
		return 5;//Proteção

	if(Item[0].sIndex == 612 && BASE_GetItemSanc(&Item[0]) >= 9 && Item[1].sIndex == 2441 && Item[2].sIndex == 2442)
		return 6;//Poder

	if(Item[0].sIndex == 612 && Item[1].sIndex == 613 && Item[2].sIndex == 614 && Item[3].sIndex == 615)
		return 7;//Armazenagem

	if(Item[0].sIndex == 614 && BASE_GetItemSanc(&Item[0]) >= 9 && Item[1].sIndex == 2443 && Item[2].sIndex == 2444)
		return 8;//Precisão

	if(Item[0].sIndex == 615 && BASE_GetItemSanc(&Item[0]) >= 9 && Item[1].sIndex == 697 && Item[2].sIndex == 697 && Item[3].sIndex == 697)
		return 9;//Magia

	return -1;
}

int GetParryRate(STRUCT_MOB *mob, int add, int attackerdex, int attackrsv)
{
	if (add > 100)
		add = 100;

	if (add < 0)
		add = 0;

	int	targetdex = mob->CurrentScore.Dex;

	if (targetdex > 1000)
		targetdex = 1000;

	int parryrate1 = mob->CurrentScore.Dex - 1000;

	if (parryrate1 < 0)
		parryrate1 = 0;

	if (parryrate1 >= 2000)
		parryrate1 = 2000;

	int parryrate2 = mob->CurrentScore.Dex - 3000;

	if (parryrate2 < 0)
		parryrate2 = 0;

	int parryrate = targetdex / 2 + add + parryrate1 / 4 + parryrate2 / 8 - attackerdex;

	int rsv = attackrsv;

	if (rsv & 0x20)
		parryrate += 100;

	if (rsv & 0x80)
		parryrate += 50;

	if (rsv & 0x200)
		parryrate += 50;

	if (parryrate >= 650)
		parryrate = 650;

	if (parryrate < 1)
		parryrate = 1;

	return parryrate;
}

int GetEmptyAffect(int mob, int type)
{
	for (int i = 0; i < MAX_AFFECT; i++)
	{
		if (pMob[mob].Affect[i].Type == type)
			return i;
	}

	for (int i = 0; i < MAX_AFFECT; i++)
	{
		if (pMob[mob].Affect[i].Type == 0)
			return i;
	}

	return -1;
}

int GetHide(int mob)
{
	if (mob <= 0 || mob >= MAX_USER)
		return FALSE;

	for(int i = 0; i < MAX_AFFECT; i++)
	{
		if (pMob[mob].Affect[i].Type == 28)
			return TRUE;
	}
	return FALSE;
}

int GetInView(int a, int b)
{
	if (pMob[a].TargetX < pMob[b].TargetX - VIEWGRIDX || pMob[a].TargetX > pMob[b].TargetX + VIEWGRIDX ||
		pMob[a].TargetY < pMob[b].TargetY - VIEWGRIDY || pMob[a].TargetY > pMob[b].TargetY + VIEWGRIDY)
		return FALSE;

	return TRUE;
}

int GetInHalf(int a, int b)
{
	if (pMob[a].TargetX < pMob[b].TargetX - HALFGRIDX || pMob[a].TargetX > pMob[b].TargetX + HALFGRIDX ||
		pMob[a].TargetY < pMob[b].TargetY - HALFGRIDY || pMob[a].TargetY > pMob[b].TargetY + HALFGRIDY)
		return FALSE;

	return TRUE;
}

int GetTeleportPosition(int conn, int *x, int *y)
{
	int xv = (*x) & 0xFFFC;
	int yv = (*y) & 0xFFFC;
	int Charge = 0;

	if (xv == 2116 && yv == 2100) // Armia to Noatum	
	{
		*x = 1044 + rand() % 3;
		*y = 1724 + rand() % 3;

		Charge = 700;
	}
	else if (xv == 2480 && yv == 1716) // Azran to Noatum
	{
		*x = 1044 + rand() % 3;
		*y = 1716 + rand() % 3;

		Charge = 700;
	}
	else if (xv == 2456 && yv == 2016) // Erion to Noatum
	{
		*x = 1044 + rand() % 3;
		*y = 1708 + rand() % 3;

		Charge = 700;
	}
	else if (xv == 1044 && yv == 1724) // Noatum to Armia
	{
		*x = 2116 + rand() % 3;
		*y = 2100 + rand() % 3;
	}
	else if (xv == 1044 && yv == 1716) // Noatum to Azran
	{
		*x = 2480 + rand() % 3;
		*y = 1716 + rand() % 3;
	}
	else if (xv == 1044 && yv == 1708) // Noatum to Erion
	{
		*x = 2456 + rand() % 3;
		*y = 2016 + rand() % 3;
	}
	else if (xv == 1048 && yv == 1764) // Noatum guild area to Noatum siege war area
	{
		*x = 1100 + rand() % 3;
		*y = 1712 + rand() % 3;
	}
	else if (xv == 1052 && yv == 1708) // Noatum to Karden
	{
		*x = 3650 + rand() % 3;
		*y = 3110 + rand() % 3;
	}
	else if (xv == 2140 && yv == 2068) // Armia to Armia Field
	{
		*x = 2588 + rand() % 3;
		*y = 2096 + rand() % 3;
	}
	else if (xv == 2468 && yv == 1716) // Azran to Azran Field, near cape quest
	{
		*x = 2248 + rand() % 3;
		*y = 1556 + rand() % 3;
	}
	else if (xv == 2364 && yv == 2284) // Armia Field to Dungeon 1st Floor
	{
		*x = 144 + rand() % 3;
		*y = 3788 + rand() % 3;
	}
	else if (xv == 144 && yv == 3788) // Dungeon 1st floor to Armia Field
	{
		*x = 2364 + rand() % 3;
		*y = 2284 + rand() % 3;
	}
	else if (xv == 2668 && yv == 2156) // Armia Field to Dungeon 1st Floor (other locations)
	{
		*x = 148 + rand() % 2;
		*y = 3774 + rand() % 2;
	}
	else if (xv == 144 && yv == 3772) // Dungeon 1st floor to Armia Field (other locations)
	{
		*x = 2668 + rand() % 3;
		*y = 2156 + rand() % 3;
	}
	else if (xv == 148 && yv == 3780) // Dungeon 1st floor to Dungeon 2nd floor
	{
		*x = 1004 + rand() % 3;
		*y = 4028 + rand() % 3;
	}
	else if (xv == 144 && yv == 3780) // Dungeon 1st floor to Dungeon 2nd floor
	{
		*x = 1004 + rand() % 3;
		*y = 4028 + rand() % 3;
	}
	else if (xv == 1004 && yv == 4028) // Dungeon 2nd floor to Dungeon 1st Floor
	{
		*x = 148 + rand() % 3;
		*y = 3780 + rand() % 3;
	}
	else if (xv == 408 && yv == 4072) // Dungeon 1st floor to Dungeon 2nd floor
	{
		*x = 1004 + rand() % 3;
		*y = 4064 + rand() % 3;
	}
	else if (xv == 1004 && yv == 4064) // Dungeon 2nd floor to Dungeon 1st floor
	{
		*x = 408 + rand() % 3;
		*y = 4072 + rand() % 3;
	}
	else if (xv == 744 && yv == 3820) // Dungeon 1st floor to Dungeon 3rd floor
	{
		*x = 1004 + rand() % 3;
		*y = 3992 + rand() % 3;
	}
	else if (xv == 1004 && yv == 3992) // Dungeon 3rd floor to Dugeon 1st floor
	{
		*x = 744 + rand() % 3;
		*y = 3820 + rand() % 3;
	}
	else if (xv == 680 && yv == 4076) // Dungeon 2nd floor to Dungeon 3rd floor
	{
		*x = 916 + rand() % 3;
		*y = 3820 + rand() % 3;
	}
	else if (xv == 916 && yv == 3820) // Dungeon 3rd floor to Dungeon 2nd floor
	{
		*x = 680 + rand() % 3;
		*y = 4076 + rand() % 3;
	}
	else if (xv == 876 && yv == 3872) // Dungeon 2nd floor to Dungeon 3rd floor
	{
		*x = 932 + rand() % 3;
		*y = 3820 + rand() % 3;
	}
	else if (xv == 932 && yv == 3820) // Dungeon 3rd floor to Dungeon 2nd floor
	{
		*x = 876 + rand() % 3;
		*y = 3872 + rand() % 3;
	}
	else if (xv == 188 && yv == 188) // War area to Azran
	{
		*x = 2548 + rand() % 3;
		*y = 1740 + rand() % 3;
	}
	else if (xv == 2548 && yv == 1740) // Azran to vale
	{
		if (pMob[conn].MOB.Equip[13].sIndex == 3916)
		{
			*x = 2281 + rand() % 3;
			*y = 3688 + rand() % 3;
		}
	}
	else if (xv == 1824 && yv == 1772) // Azran Field to Underworld
	{
		*x = 1172 + rand() % 3;
		*y = 4080 + rand() % 3;
	}
	else if (xv == 1172 && yv == 4080) // Underworld to Azran Field
	{
		*x = 1824 + rand() % 3;
		*y = 1772 + rand() % 3;
	}
	else if (xv == 1516 && yv == 3996) // Underworld 1st floor to Underworld 2nd floor
	{
		*x = 1304 + rand() % 3;
		*y = 3816 + rand() % 3;
	}
	else if (xv == 1304 && yv == 3816) // Underworld 2nd floor to Underworld 1st floor
	{
		*x = 1516 + rand() % 3;
		*y = 3996 + rand() % 3;
	}
	else if (xv == 2452 && yv == 1716) // Azran 2nd teleport to Azran field
	{
		*x = 1969 + rand() % 3;
		*y = 1711 + rand() % 3;
	}
	else if (xv == 2452 && yv == 1988) // Erion teleport to Azran field
	{
		*x = 1989 + rand() % 3;
		*y = 1755 + rand() % 3;
	}

	else if (xv == 3648 && yv == 3108) // Karden teleport to Noatum
	{
		*x = 1054 + rand() % 3;
		*y = 1710 + rand() % 3;
	}

	else if (xv == 1056 && yv == 1724) // Noatum to RvR - Deserto
	{
		if (g_pRvrWar.Status == 1)
		{
			if (pMob[conn].MOB.Clan == 7)//BLUE
			{
				*x = 1061 + rand() % 3;
				*y = 2129 + rand() % 3;
			}
			else if (pMob[conn].MOB.Clan == 8)//RED
			{
				*x = 1237 + rand() % 3;
				*y = 1966 + rand() % 3;
			}

			SendCounterMob(conn, g_pRvrWar.Points[0], g_pRvrWar.Points[1]);
		}

		else
		{
			*x = 1164 + rand() % 3;
			*y = 1720 + rand() % 3;
		}
	}

	else if(xv == 2364 && yv == 3892 && pMob[conn].extra.KefraTicket > 0) // Kefra para Hall
	{
		pMob[conn].extra.KefraTicket--;

		*x = 2364 + rand() % 3;
		*y = 3906 + rand() % 3;


		sprintf(temp, g_pMessageStringTable[_DN_CHANGE_COUNT], pMob[conn].extra.KefraTicket);
		SendClientMessage(conn, temp);
	}

	//Deserto - Kefra
	else if(xv == 2364 && yv == 3924 && KefraLive != 0)
	{
		*x = 3250 + rand() % 3;
		*y = 1703 + rand() % 3;
	}

	//Kefra - Deserto
	else if(xv == 1312 && yv == 1900)
	{
		*x = 2366 + rand() % 2;
		*y = 4073 + rand() % 2;
	}

	else if(xv == 2364 && yv == 4072)
	{
		*x = 1314 + rand() % 2;
		*y = 1900 + rand() % 2;
	}
	return Charge;
}

int	GetExpApply(STRUCT_MOBEXTRA extra, int exp, int attacker, int target)
{
	if (extra.ClassMaster == ARCH && exp > 0)
	{
		if (attacker >= 354 && extra.QuestInfo.Arch.Level355 == 0)
			return 0;

		if (attacker >= 369 && extra.QuestInfo.Arch.Level370 == 0)
			return 0;

		exp = (int)(exp * 0.50);
	}
	else if (extra.ClassMaster == CELESTIAL && exp > 0)
	{
		if (attacker >= 39 && extra.QuestInfo.Celestial.Lv40 == 0)
			return 0;

		if (attacker >= 89 && extra.QuestInfo.Celestial.Lv90 == 0)
			return 0;
	}

	if ((extra.ClassMaster == CELESTIAL || extra.ClassMaster == SCELESTIAL || extra.ClassMaster == CELESTIALCS) && exp > 0)
		attacker = MAX_LEVEL;

	if (target > MAX_LEVEL+1 || attacker < 0 || target < 0)
		return exp;

	attacker++;
	target++;

	int	multiexp = (target * 100) / attacker;

	if (multiexp < 80 && attacker >= 50)
		multiexp = multiexp * 2 - 100;
	else if (multiexp > 200)
		multiexp = 200;

	if (multiexp < 0)
		multiexp = 0;

	exp = (exp * multiexp + 1) / 100;

	return exp;
}

int GetCreateMob(int mob, MSG_CreateMob *sm)
{
	memset(sm, 0, sizeof(MSG_CreateMob));

	sm->Type = _MSG_CreateMob;

	strncpy(sm->MobName, pMob[mob].MOB.MobName, NAME_LENGTH);

	if (mob < MAX_USER)
	{
		int ck = GetCurKill(mob);
		int tk = GetTotKill(mob);

		unsigned char cv = ck;
		unsigned char cv1 = tk % 256;
		unsigned char cv2 = tk / 256;

		sm->MobName[13] = cv;
		sm->MobName[14] = cv1;
		sm->MobName[15] = cv2;

		int chaos = GetPKPoint(mob);
		int gv = GetGuilty(mob);

		if (gv > 0)
			chaos = 0;

		cv = chaos;
		sm->MobName[12] = chaos;
	}

	sm->PosX = pMob[mob].TargetX;
	sm->PosY = pMob[mob].TargetY;
	sm->Size = sizeof(MSG_CreateMob);
	sm->ID = ESCENE_FIELD;
	sm->MobID = mob;
	sm->ClientTick = CurrentTime;

	sm->Score = pMob[mob].MOB.CurrentScore;

	sm->Guild = pMob[mob].MOB.Guild;
	sm->GuildMemberType = pMob[mob].MOB.GuildLevel;

	if (pMob[mob].GuildDisable == 1)
	{
		sm->Guild = 0;
		sm->GuildMemberType = 0;
	}
	if (mob >= MAX_USER)
	{
		if (pMob[mob].MOB.Clan == 4)
			sm->Score.Ac = 0;

		else
			sm->Score.Ac = 1;
	}

	sm->CreateType = 0;

	if (pMob[mob].MOB.GuildLevel == 9)
		sm->CreateType = sm->CreateType | 0x80;

	else if (pMob[mob].MOB.GuildLevel >= 6)
		sm->CreateType = sm->CreateType | 0x40;


	sm->Hold = pMob[mob].extra.Hold;

	strncpy(sm->Tab, pMob[mob].Tab, 26);

	int selfdead = 0;

	int xx = pMob[mob].TargetX;
	int yy = pMob[mob].TargetY;

	for (int i = 0; i < MAX_EQUIP; i++)
	{
		STRUCT_ITEM *item = &pMob[mob].MOB.Equip[i];

		sm->Equip[i] = BASE_VisualItemCode(item, i);

		sm->AnctCode[i] = BASE_VisualAnctCode(item);
		
		if (i == 14 && sm->Equip[i] >= 2360 && sm->Equip[i] < 2390 && pMob[mob].MOB.Equip[i].stEffect[0].sValue <= 0)
		{
			sm->Equip[i] = 0;

			selfdead = 1;

			continue;
		}

		if (i == 14 && sm->Equip[i] >= 2360 && sm->Equip[i] < 2390)
		{
			int sanc = pMob[mob].MOB.Equip[i].stEffect[1].cEffect;
			sanc = sanc / 10;

			if (sanc > 13)
				sanc = 13;

			if (sanc < 0)
				sanc = 0;

			sanc = sanc << 12;

			sm->Equip[i] = sm->Equip[i] + sanc;

			continue;
		}
	}

	GetAffect(sm->Affect, pMob[mob].Affect);

	if (BrState != 0 && xx >= 2604 && yy >= 1708 && xx <= 2648 && yy <= 1744)
	{
		memset(sm->MobName, 0, NAME_LENGTH); // MOBNAME_LEGNTH = 16
		strncpy(sm->MobName, "??????", 6);

		sm->Equip[15] = 0;
		sm->Guild = 0;
	}

	return selfdead;
}

int GetCreateMobTrade(int mob, MSG_CreateMobTrade *sm)
{
	sm->Type = _MSG_CreateMobTrade;
	
	strncpy(sm->MobName, pMob[mob].MOB.MobName, NAME_LENGTH);
	
	if(mob >= MAX_USER)
	{
		Log("err,getcreatemob request by non player %d", pMob[mob].MOB.MobName, 0);
		return 0;
	}

	if	(mob < MAX_USER)
	{	
		int ck = GetCurKill(mob);
		int tk = GetTotKill(mob);

		unsigned char cv = ck;
		unsigned char cv1 = tk%256;
		unsigned char cv2 = tk/256;

		sm->MobName[13] = cv;
		sm->MobName[14] = cv1;
		sm->MobName[15] = cv2;

		int chaos = GetPKPoint(mob);
		
		cv = chaos;
		sm->MobName[12] = cv;
	}

	sm->PosX = pMob[mob].TargetX;
	sm->PosY = pMob[mob].TargetY;
	sm->Size = sizeof(MSG_CreateMobTrade);
	sm->ID   = ESCENE_FIELD;
	sm->MobID = mob;
	sm->ClientTick = CurrentTime;

	memcpy(&sm->Score, &pMob[mob].MOB.CurrentScore, sizeof(STRUCT_SCORE));

	sm->Guild = pMob[mob].MOB.Guild;

	if	(pMob[mob].GuildDisable == 1)
		sm->Guild = 0;

	if	(mob >= MAX_USER)
		sm->Score.Ac = pMob[mob].MOB.Clan != 4;

	sm->CreateType = 0;

	if (pMob[mob].MOB.GuildLevel == 9)
		sm->CreateType = sm->CreateType | 0x80;

	if (pMob[mob].MOB.GuildLevel != 0)
		sm->CreateType = sm->CreateType | 0x40;

	int selfdead=0;

	for (int i = 0; i < MAX_EQUIP; i++)
	{
		STRUCT_ITEM *item = &pMob[mob].MOB.Equip[i];

		sm->Equip[i] = BASE_VisualItemCode(item, i);

		sm->AnctCode[i] = BASE_VisualAnctCode(item);

		if (i == 14 && sm->Equip[i] >= 2360 && sm->Equip[i] < 2390 && pMob[mob].MOB.Equip[i].stEffect[0].sValue <= 0)
		{
			sm->Equip[i] = 0;

			selfdead = 1;

			continue;
		}

		if (i == 14 && sm->Equip[i] >= 2360 && sm->Equip[i] < 2390)
		{
			int sanc = pMob[mob].MOB.Equip[i].stEffect[1].cEffect;
			sanc = sanc / 10;

			if (sanc > 13)
				sanc = 13;

			if (sanc < 0)
				sanc = 0;

			sanc = sanc << 12;

			sm->Equip[i] = sm->Equip[i] + sanc;

			continue;
		}
	}

	strncpy(sm->Tab, pMob[mob].Tab, 26);
	strncpy(sm->Desc, pUser[mob].AutoTrade.Title, MAX_AUTOTRADETITLE-1);

	GetAffect(sm->Affect,pMob[mob].Affect);

	return selfdead;
}

void GetAffect(unsigned short *out, STRUCT_AFFECT *affect)
{
	for (int i = 0; i < MAX_AFFECT; i++)
	{
		int type = affect[i].Type;
		int value = affect[i].Time;

		//if(type == 0)
		//	continue;

		value = value;

		if (value > 2550000)
			value = 2550000;

		unsigned short tout = (type << 8) + (value & 0xFF);

		out[i] = tout;
	}
}

void GetCreateItem(int idx, MSG_CreateItem *sm)
{
	sm->ID = ESCENE_FIELD;
	sm->Size = sizeof(MSG_CreateItem);
	sm->Type = _MSG_CreateItem;


	sm->GridX = pItem[idx].PosX;
	sm->GridY = pItem[idx].PosY;

	sm->ItemID = idx + 10000;

	sm->Rotate = pItem[idx].Rotate;

	sm->Item = pItem[idx].ITEM;

	sm->State = pItem[idx].State;

	sm->Height = -204;

	if(pItem[idx].ITEM.sIndex == 3145)
	{
		int zone = BASE_GetVillage(pItem[idx].PosX, pItem[idx].PosY);
		sm->Item.sIndex = 3145 + g_pGuildZone[zone].Victory;

		sm->State = STATE_CLOSED;

		sm->Item.stEffect[0].cEffect = 56;
		sm->Item.stEffect[1].cEffect = 57;

		sm->Item.stEffect[0].cValue = (g_pGuildZone[zone].ChargeGuild + (ServerIndex << 12)) >> 8;
		sm->Item.stEffect[1].cValue = g_pGuildZone[zone].ChargeGuild;
	}
	
	if (pItem[idx].ITEM.sIndex == 5700)
		return;
		
	if (sm->State == STATE_OPEN)
	{
		sm->Height = pItem[idx].Height;
		sm->Create = 0;
	}
}

void GetAction(int mob, int targetx, int targety, MSG_Action *sm)
{
	sm->ID = mob;
	sm->ClientTick = CurrentTime;
	sm->Size = sizeof(MSG_Action);
	sm->Type = _MSG_Action;

	sm->TargetX = targetx;
	sm->TargetY = targety;
	sm->Route[0] = 0;


	sm->Speed = 2;
	sm->Effect = 1;
	sm->PosX = pMob[mob].TargetX;
	sm->PosY = pMob[mob].TargetY;
}

unsigned char GetAttribute(int x, int y)
{
	if (x < 0 || x > MAX_GRIDX || y < 0 || y > MAX_GRIDY)
		return 0;

	int xx = x / 4;

	xx = xx & 0x3FF;

	int yy = y / 4;

	yy = yy & 0x3FF;

	unsigned char ret = g_pAttribute[yy][xx];

	return ret;
}

void GetAttack(int mob, int target, MSG_AttackOne *sm)
{
	memset(sm, 0, sizeof(MSG_AttackOne));

	sm->ID = ESCENE_FIELD;
	sm->AttackerID = mob;
	sm->ClientTick = CurrentTime;
	sm->PosX = pMob[mob].TargetX;
	sm->PosY = pMob[mob].TargetY;
	sm->TargetX = pMob[target].TargetX;
	sm->TargetY = pMob[target].TargetY;
	sm->Size = sizeof(MSG_AttackOne);
	sm->Type = _MSG_AttackOne;

	sm->DoubleCritical = 0;
	sm->CurrentMp = -1;
	sm->ReqMp = -1;
	
	sm->Dam[0].TargetID = 0;
	sm->Dam[0].Damage = 0;

	sm->Dam[0].TargetID = target;

	int Resist = -1;

	sm->SkillParm = 0;
	sm->SkillIndex = -1;
	sm->Motion = rand() % 3 + 4;

	int special = 0;
	int special2 = 0;

	if (mob >= MAX_USER)
	{
		int dis = BASE_GetDistance(pMob[mob].TargetX, pMob[mob].TargetY, pMob[target].TargetX, pMob[target].TargetY);
		if (dis >= 3)
		{
			special = /*pMob[mob].MOB.BaseScore.Special[2]*/pMob[mob].MOB.SkillBar[2];
			special2 = /*pMob[mob].MOB.BaseScore.Special[3] */ pMob[mob].MOB.SkillBar[3];
		}
		else
		{
			special = /*pMob[mob].MOB.BaseScore.Special[0]*/pMob[mob].MOB.SkillBar[0];
			special2 = /*pMob[mob].MOB.BaseScore.Special[1]*/pMob[mob].MOB.SkillBar[1];
		}

		sm->Motion = 0;

		if (special == 255)
			special = -1;

		switch (special)
		{
		case 121:
			sm->SkillIndex = 103;
			sm->SkillParm = 5;
			break;
		case 122:
			sm->SkillIndex = 105;
			sm->SkillParm = 1;
			break;
		case 123:
			sm->SkillIndex = 101;
			sm->SkillParm = 1;
			break;
		case 124:
			sm->SkillIndex = 101;
			sm->SkillParm = 2;
			break;
		case 125:
			sm->SkillIndex = 40;
			sm->SkillParm = 1;
			break;
		case 126:
			sm->SkillIndex = 40;
			sm->SkillParm = 2;
			break;
		case 127:
			sm->SkillIndex = 40;
			sm->SkillParm = 3;
			break;
		case 128:
			sm->SkillIndex = 33;
			sm->SkillParm = -4;
			sm->Motion = -4;
			break;
		default:
			sm->SkillIndex = special;
			sm->SkillParm = 1;
			break;
		}
		if (special2)
		{
			sm->Motion = 4;
			int _rand = rand() % 4;

			switch (special2)
			{
			case 3:
				if (_rand <= 1)
					sm->Motion++;
				break;
			case 6:
				if (_rand > 1)
					sm->Motion  += 2;
				else
					sm->Motion++;
				break;
			case 7:
				if (_rand == 1)
					sm->Motion++;
				else
					sm->Motion  += 2;
				break;
			case 15:
				switch (_rand)
				{
				case 1:
					sm->Motion++;
					break;
				case 2:
					sm->Motion += 2;
					break;
				case 3:
					sm->Motion += 3;
					break;
				}
				break;
			case 24:
				if (_rand > 1)
					sm->Motion += 4;
				else
					sm->Motion += 3;
				break;
			case 27:
				switch (_rand)
				{
				case 1:
					sm->Motion++;
					break;
				case 2:
					sm->Motion += 3;
					break;
				case 3:
					sm->Motion += 4;
					break;
				}
				break;
			case 23:
				switch (_rand)
				{
				case 1:
					sm->Motion++;
					break;
				case 2:
					sm->Motion += 2;
					break;
				case 3:
					sm->Motion += 4;
					break;
				}
				break;

			default:
				sm->SkillParm = 1;
				break;
			}
		}
		int rand_ = rand() % 100;
		if (pMob[mob].MOB.SkillBar[3] != 255 && rand_ >= 25 && rand_ <= 64)
		{
			int skillb3 = pMob[mob].MOB.SkillBar[3];
			
			int insttype = g_pSpell[skillb3].InstanceType;
			
			int leader = pMob[mob].Leader;
			if (leader <= 0)
				leader = mob;
			
			int hp = pMob[mob].MOB.CurrentScore.Hp;
			hp *= 10;
			hp /= pMob[mob].MOB.CurrentScore.MaxHp+1;
			
			int lhp = pMob[leader].MOB.CurrentScore.Hp;
			lhp *= 10;
			lhp /= pMob[leader].MOB.CurrentScore.MaxHp+1;
			
			if (insttype == 6)
			{
				if (hp <= 8 || lhp <= 8)
				{
					int _mob = 0;

					sm->SkillIndex = insttype;

					_mob = mob;
					if (hp > lhp)
						_mob = leader;


					sm->Dam[0].TargetID = _mob;
					sm->Dam[0].Damage = pMob[_mob].MOB.CurrentScore.MaxHp / 10;
					
					return;
				}
			}
		}
		if (pMob[mob].MOB.SkillBar[0] == 255 || rand_ < 0 || rand_ > 49)
		{
			if (pMob[mob].MOB.SkillBar[1] == 255 || rand_ < 50 || rand_ > 84)
			{
				if (pMob[mob].MOB.SkillBar[2] != 255 && rand_ >= 85 && rand_ <= 99)
				{
					Resist = g_pSpell[pMob[mob].MOB.SkillBar[2]].InstanceType - 2;
					sm->SkillIndex = pMob[mob].MOB.SkillBar[2];
				}
			}
			else
			{
				Resist = g_pSpell[pMob[mob].MOB.SkillBar[1]].InstanceType - 2;
				sm->SkillIndex = pMob[mob].MOB.SkillBar[1];
			}
		}
		else
		{
			Resist = g_pSpell[pMob[mob].MOB.SkillBar[0]].InstanceType - 2;
			sm->SkillIndex = pMob[mob].MOB.SkillBar[0];
		}
	}

	int fisdam = pMob[mob].MOB.CurrentScore.Damage;
	fisdam = BASE_GetDamage(fisdam, pMob[target].MOB.CurrentScore.Ac, 0);
	
	if (Resist >= 0 && Resist <= 3)
		fisdam = (200 - pMob[target].MOB.Resist[Resist]) * fisdam / 100;
	
	if (fisdam > 0 && target < MAX_USER)
	{
		if (pMob[target].ReflectDamage > 0)
		{
			fisdam -= pMob[target].ReflectDamage;
			if (fisdam <= 0)
				fisdam = 1;
		}
	}

	if (fisdam > 0)
	{
		if ((mob < MAX_USER || pMob[mob].MOB.Clan == 4) && (target < MAX_USER || pMob[target].MOB.Clan == 4))
		{
			fisdam = 3 * fisdam / 10;

			if (GetAttribute(pMob[target].TargetX, pMob[target].TargetY) & 1 || !(GetAttribute(pMob[target].TargetX, pMob[target].TargetY) & 0x40))
				fisdam = 0;
			if (GetAttribute(pMob[mob].TargetX, pMob[mob].TargetY) & 1 || !(GetAttribute(pMob[mob].TargetX, pMob[mob].TargetY) & 0x40))
				fisdam = 0;
		}

		int mob_leader = pMob[mob].Leader;
		if (!mob_leader)
			mob_leader = mob;

		int target_leader = pMob[target].Leader;
		if (!target_leader)
			target_leader = target;

		int mob_guild = pMob[mob].MOB.Guild;
		int target_guild = pMob[target].MOB.Guild;

		if (!mob_guild && !target_guild)
			mob_guild = -1;

		if (target < MAX_USER)
		{
			if (pMob[target].GuildDisable)
				target_guild = -2;
		}
		if (mob_leader == target_leader || mob_guild == target_guild)
		{
			pMob[mob].CurrentTarget = 0;
			fisdam = 0;
		}
	}

	sm->CurrentHp = -1;
	sm->Progress = 52428;
	sm->Dam[0].Damage = fisdam;
}

void GetAttackArea(int mob, MSG_Attack *sm)
{
	sm->ID = ESCENE_FIELD;
	sm->AttackerID = mob;
	sm->ClientTick = CurrentTime;
	sm->PosX = pMob[mob].TargetX;
	sm->PosY = pMob[mob].TargetY;
	sm->Size = sizeof(MSG_Attack);
	sm->Type = _MSG_Attack;

	sm->DoubleCritical = 0;
	sm->CurrentMp = -1;
	sm->ReqMp = -1;
	

	for(int i = 0; i < MAX_TARGET; i++)
	{
		sm->Dam[i].TargetID = 0;
		sm->Dam[i].Damage = 0;

		sm->Dam[i].TargetID = pMob[mob].EnemyList[i];

		if(sm->Dam[i].TargetID < 0 || sm->Dam[i].TargetID >= MAX_MOB)
			sm->Dam[i].TargetID = 0;
	}
	int Resist = -1;

	sm->SkillParm = 0;
	sm->SkillIndex = -1;
	sm->Motion = rand() % 3 + 4;

	for(int i = 0; i < MAX_TARGET; i++)
	{
		if(sm->Dam[i].TargetID == 0)
			continue;

		sm->TargetX = pMob[sm->Dam[i].TargetID].TargetX;
		sm->TargetY = pMob[sm->Dam[i].TargetID].TargetY;

		int special = 0;
		int special2 = 0;

		if (mob >= MAX_USER)
		{
			int dis = BASE_GetDistance(pMob[mob].TargetX, pMob[mob].TargetY, pMob[sm->Dam[i].TargetID].TargetX, pMob[sm->Dam[i].TargetID].TargetY);
			if (dis >= 3)
			{
				special = /*pMob[mob].MOB.BaseScore.Special[2]*/pMob[mob].MOB.SkillBar[2];
				special2 = /*pMob[mob].MOB.BaseScore.Special[3] */ pMob[mob].MOB.SkillBar[3];
			}
			else
			{
				special = /*pMob[mob].MOB.BaseScore.Special[0]*/pMob[mob].MOB.SkillBar[0];
				special2 = /*pMob[mob].MOB.BaseScore.Special[1]*/pMob[mob].MOB.SkillBar[1];
			}

			sm->Motion = 0;

			if (special == 255)
				special = -1;

			switch (special)
			{
			case 121:
				sm->SkillIndex = 103;
				sm->SkillParm = 5;
				break;
			case 122:
				sm->SkillIndex = 105;
				sm->SkillParm = 1;
				break;
			case 123:
				sm->SkillIndex = 101;
				sm->SkillParm = 1;
				break;
			case 124:
				sm->SkillIndex = 101;
				sm->SkillParm = 2;
				break;
			case 125:
				sm->SkillIndex = 40;
				sm->SkillParm = 1;
				break;
			case 126:
				sm->SkillIndex = 40;
				sm->SkillParm = 2;
				break;
			case 127:
				sm->SkillIndex = 40;
				sm->SkillParm = 3;
				break;
			case 128:
				sm->SkillIndex = 33;
				sm->SkillParm = -4;
				sm->Motion = -4;
				break;
			default:
				sm->SkillIndex = special;
				sm->SkillParm = 1;
				break;
			}
			if (special2)
			{
				sm->Motion = 4;
				int _rand = rand() % 4;

				switch (special2)
				{
				case 3:
					if (_rand <= 1)
						sm->Motion++;
					break;
				case 6:
					if (_rand > 1)
						sm->Motion  += 2;
					else
						sm->Motion++;
					break;
				case 7:
					if (_rand == 1)
						sm->Motion++;
					else
						sm->Motion  += 2;
					break;
				case 15:
					switch (_rand)
					{
					case 1:
						sm->Motion++;
						break;
					case 2:
						sm->Motion += 2;
						break;
					case 3:
						sm->Motion += 3;
						break;
					}
					break;
				case 24:
					if (_rand > 1)
						sm->Motion += 4;
					else
						sm->Motion += 3;
					break;
				case 27:
					switch (_rand)
					{
					case 1:
						sm->Motion++;
						break;
					case 2:
						sm->Motion += 3;
						break;
					case 3:
						sm->Motion += 4;
						break;
					}
					break;
				case 23:
					switch (_rand)
					{
					case 1:
						sm->Motion++;
						break;
					case 2:
						sm->Motion += 2;
						break;
					case 3:
						sm->Motion += 4;
						break;
					}
					break;

				default:
					sm->SkillParm = 1;
					break;
				}
			}
			int rand_ = rand() % 100;
			if (pMob[mob].MOB.SkillBar[3] != 255 && rand_ >= 25 && rand_ <= 64)
			{
				int skillb3 = pMob[mob].MOB.SkillBar[3];
			
				int insttype = g_pSpell[skillb3].InstanceType;
			
				int leader = pMob[mob].Leader;
				if (leader <= 0)
					leader = mob;
			
				int hp = pMob[mob].MOB.CurrentScore.Hp;
				hp *= 10;
				hp /= pMob[mob].MOB.CurrentScore.MaxHp+1;
			
				int lhp = pMob[leader].MOB.CurrentScore.Hp;
				lhp *= 10;
				lhp /= pMob[leader].MOB.CurrentScore.MaxHp+1;
			
				if (insttype == 6)
				{
					if (hp <= 8 || lhp <= 8)
					{
						int _mob = 0;

						sm->SkillIndex = insttype;

						_mob = mob;
						if (hp > lhp)
							_mob = leader;


						sm->Dam[0].TargetID = _mob;
						sm->Dam[0].Damage = pMob[_mob].MOB.CurrentScore.MaxHp / 10;
					
						return;
					}
				}
			}
			if (pMob[mob].MOB.SkillBar[0] == 255 || rand_ < 0 || rand_ > 49)
			{
				if (pMob[mob].MOB.SkillBar[1] == 255 || rand_ < 50 || rand_ > 84)
				{
					if (pMob[mob].MOB.SkillBar[2] != 255 && rand_ >= 85 && rand_ <= 99)
					{
						Resist = g_pSpell[pMob[mob].MOB.SkillBar[2]].InstanceType - 2;
						sm->SkillIndex = pMob[mob].MOB.SkillBar[2];
					}
				}
				else
				{
					Resist = g_pSpell[pMob[mob].MOB.SkillBar[1]].InstanceType - 2;
					sm->SkillIndex = pMob[mob].MOB.SkillBar[1];
				}
			}
			else
			{
				Resist = g_pSpell[pMob[mob].MOB.SkillBar[0]].InstanceType - 2;
				sm->SkillIndex = pMob[mob].MOB.SkillBar[0];
			}
		}

		int fisdam = pMob[mob].MOB.CurrentScore.Damage;
		fisdam = BASE_GetDamage(fisdam, pMob[sm->Dam[i].TargetID].MOB.CurrentScore.Ac, 0);
	
		if (Resist >= 0 && Resist <= 3)
			fisdam = (200 - pMob[sm->Dam[i].TargetID].MOB.Resist[Resist]) * fisdam / 100;
	
		if (fisdam > 0 && sm->Dam[i].TargetID < MAX_USER)
		{
			if (pMob[sm->Dam[i].TargetID].ReflectDamage > 0)
			{
				fisdam -= pMob[sm->Dam[i].TargetID].ReflectDamage;
				if (fisdam < 0)
					fisdam = 0;
			}
		}

		if (fisdam > 0)
		{
			if ((mob < MAX_USER || pMob[mob].MOB.Clan == 4) && (sm->Dam[i].TargetID < MAX_USER || pMob[sm->Dam[i].TargetID].MOB.Clan == 4))
			{
				fisdam = 3 * fisdam / 10;

				if (GetAttribute(pMob[sm->Dam[i].TargetID].TargetX, pMob[sm->Dam[i].TargetID].TargetY) & 1 || !(GetAttribute(pMob[sm->Dam[i].TargetID].TargetX, pMob[sm->Dam[i].TargetID].TargetY) & 0x40))
					fisdam = 0;
				if (GetAttribute(pMob[mob].TargetX, pMob[mob].TargetY) & 1 || !(GetAttribute(pMob[mob].TargetX, pMob[mob].TargetY) & 0x40))
					fisdam = 0;
			}

			int mob_leader = pMob[mob].Leader;
			if (!mob_leader)
				mob_leader = mob;

			int target_leader = pMob[sm->Dam[i].TargetID].Leader;
			if (!target_leader)
				target_leader = sm->Dam[i].TargetID;

			int mob_guild = pMob[mob].MOB.Guild;
			int target_guild = pMob[sm->Dam[i].TargetID].MOB.Guild;

			if (!mob_guild && !target_guild)
				mob_guild = -1;

			if (sm->Dam[i].TargetID < MAX_USER)
			{
				if (pMob[sm->Dam[i].TargetID].GuildDisable)
					target_guild = -2;
			}
			if (mob_leader == target_leader || mob_guild == target_guild)
			{
				pMob[mob].CurrentTarget = 0;
				fisdam = 0;
			}
		}

		sm->Dam[i].Damage = fisdam;
	}
}

void GetGuild(int conn)
{
	STRUCT_ITEM *item = &pMob[conn].MOB.Equip[12];

	int guild = pMob[conn].MOB.Guild;
}

int GetGuild(STRUCT_ITEM *item)
{
	int guild = BASE_GetGuild(item);

	return guild;
}

BOOL GetEmptyItemGrid(int *gridx, int *gridy)
{
	if ((pItemGrid[*gridy][*gridx] == 0) && (pHeightGrid[*gridy][*gridx] != 127))
		return TRUE;

	for (int y = (*gridy) - 1; y <= (*gridy) + 1; y++)
	{
		for (int x = (*gridx) - 1; x <= (*gridx) + 1; x++)
		{
			if (x < 0 || y < 0 || x >= MAX_GRIDX || y >= MAX_GRIDY)
				continue;

			if (pItemGrid[y][x] == 0 && pHeightGrid[y][x] != 127)
			{
				*gridy = y;
				*gridx = x;

				return TRUE;
			}
		}
	}

	return FALSE;
}

BOOL GetEmptyMobGrid(int mob, int *tx, int *ty)
{
	if (*tx < 0 || *ty < 0 || *tx >= MAX_GRIDX || *ty >= MAX_GRIDY)
	{
		Log("GetEmptyMobGridOut of range", "-system", 0);

		return FALSE;
	}

	if (pMobGrid[*ty][*tx] == mob)
		return TRUE;

	if (pMobGrid[*ty][*tx] == 0 && pHeightGrid[*ty][*tx] != 127)
		return TRUE;

	int y = 0;
	int x = 0;

	for (y = (*ty) - 1; y <= (*ty) + 1; y++)
	{
		for (x = (*tx) - 1; x <= (*tx) + 1; x++)
		{
			if (x < 0 || y < 0 || x >= MAX_GRIDX || y >= MAX_GRIDY)
				continue;

			if (pMobGrid[y][x] != 0)
				continue;

			if (pHeightGrid[*ty][*tx] == 127)
				continue;

			*ty = y;
			*tx = x;

			return TRUE;
		}
	}

	for (y = (*ty) - 2; y <= (*ty) + 2; y++)
	{
		for (x = (*tx) - 2; x <= (*tx) + 2; x++)
		{
			if (x < 0 || y < 0 || x >= MAX_GRIDX || y >= MAX_GRIDY)
				continue;

			if (pMobGrid[y][x] != 0)
				continue;

			if (pHeightGrid[*ty][*tx] == 127)
				continue;

			*ty = y;
			*tx = x;

			return TRUE;
		}
	}

	for (y = (*ty) - 3; y <= (*ty) + 3; y++)
	{
		for (int x = (*tx) - 3; x <= (*tx) + 3; x++)
		{
			if (x < 0 || y < 0 || x >= MAX_GRIDX || y >= MAX_GRIDY)
				continue;

			if (pMobGrid[y][x] != 0)
				continue;

			if (pHeightGrid[*ty][*tx] == 127)
				continue;

			*ty = y;
			*tx = x;

			return TRUE;
		}
	}

	for (y = (*ty) - 4; y <= (*ty) + 4; y++)
	{
		for (int x = (*tx) - 4; x <= (*tx) + 4; x++)
		{
			if (x < 0 || y < 0 || x >= MAX_GRIDX || y >= MAX_GRIDY)
				continue;

			if (pMobGrid[y][x] != 0)
				continue;

			if (pHeightGrid[*ty][*tx] == 127)
				continue;

			*ty = y;
			*tx = x;

			return TRUE;
		}
	}

	
	return FALSE;
}

BOOL GetEmptyMobGridGreat(int mob, int *tx, int *ty)
{
	if (*tx < 0 || *ty < 0 || *tx >= MAX_GRIDX || *ty >= MAX_GRIDY)
	{
		Log("GetEmptyMobGridOut of range", "-system", 0);

		return FALSE;
	}

	if (pMobGrid[*ty][*tx] == mob)
		return TRUE;

	if (pMobGrid[*ty][*tx] == 0 && pHeightGrid[*ty][*tx] != 127)
		return TRUE;

	int y = 0;
	int x = 0;

	for(int k = 0; k < 30; k++)
	{
		for (y = (*ty) - k; y <= (*ty) + k; y++)
		{
			for (x = (*tx) - k; x <= (*tx) + k; x++)
			{
				if (x < 0 || y < 0 || x >= MAX_GRIDX || y >= MAX_GRIDY)
					continue;

				if (pMobGrid[y][x] != 0)
					continue;

				if (pHeightGrid[*ty][*tx] == 127)
					continue;

				*ty = y;
				*tx = x;

				return TRUE;
			}
		}
	}
	return FALSE;
}

int GetCurKill(int conn)
{
	if (conn <= 0 || conn >= MAX_USER)
		return 0;

	unsigned char cv = pMob[conn].MOB.Carry[KILL_MARK].stEffect[0].cValue;

	int ret = cv;

	return ret;
}

int GetTotKill(int conn)
{
	if (conn <= 0 || conn >= MAX_USER)
		return 0;

	unsigned char cv1 = pMob[conn].MOB.Carry[KILL_MARK].stEffect[1].cValue;
	unsigned char cv2 = pMob[conn].MOB.Carry[KILL_MARK].stEffect[2].cValue;

	int ret = cv1 + cv2 * 256;

	return ret;
}

void SetCurKill(int conn, int value)
{
	if (conn <= 0 || conn >= MAX_USER)
		return;

	if (value < 0)
		value = 0;

	if (value > 200)
		value = 200;

	unsigned char cv = value;

	pMob[conn].MOB.Carry[KILL_MARK].stEffect[0].cValue = cv;
}

void SetTotKill(int conn, int value)
{
	if (conn <= 0 || conn >= MAX_USER)
		return;

	if (value < 0)
		value = 0;

	if (value > 32767)
		value = 32767;

	unsigned char cv1 = value % 256;
	unsigned char cv2 = value / 256;

	pMob[conn].MOB.Carry[KILL_MARK].stEffect[1].cValue = cv1;
	pMob[conn].MOB.Carry[KILL_MARK].stEffect[2].cValue = cv2;
}

int	GetPKPoint(int conn)
{
	unsigned char cv = pMob[conn].MOB.Carry[KILL_MARK].stEffect[0].cEffect;

	int ret = cv;

	return ret;
}

int GetGuilty(int conn)
{
	unsigned char cv = pMob[conn].MOB.Carry[KILL_MARK].stEffect[1].cEffect;

	int ret = cv;

	if (ret > 50)
	{
		pMob[conn].MOB.Carry[KILL_MARK].stEffect[1].cEffect = 0;
		ret = 0;
	}

	return ret;
}

void SetGuilty(int conn, int value)
{
	if (conn <= 0 || conn >= MAX_USER)
		return;

	if (value < 0)
		value = 0;

	if (value > 50)
		value = 50;

	unsigned char cv = value;

	pMob[conn].MOB.Carry[KILL_MARK].stEffect[1].cEffect = cv;
}

void SetPKPoint(int conn, int value)
{
	if (conn <= 0 || conn >= MAX_USER)
		return;

	if (value < 1)
		value = 1;

	if (value > 150)
		value = 150;

	unsigned char cv = value;

	pMob[conn].MOB.Carry[KILL_MARK].stEffect[0].cEffect = cv;
}

