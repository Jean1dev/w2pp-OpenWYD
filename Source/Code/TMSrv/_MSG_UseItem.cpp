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

int HuntingScrolls[6][10][2] =
{
	//Armia
	{ {2370, 2106}, {2508, 2101}, {2526, 2109}, {2529, 1882}, {2126, 1600}, {2005, 1617}, {2241, 1474}, {1858, 1721}, {2250, 1316}, {1989, 1755} },
	
	//Dungeon
	{ {290, 3799}, {724, 3781}, {481, 4062}, {876, 4058}, {855, 3922}, {808, 3876}, {959, 3813}, {926, 3750}, {1096, 3730}, {1132, 3800} },
	
	//SubMundo
	{ {1242 , 4035}, {1264 , 4017}, {1333 , 3994}, {1358 , 4041}, {1462 , 4033}, {1326 , 3788}, {1493 , 3777}, {1437 , 3741}, {1389 , 3740}, {1422 , 3810} },
	
	//Kult
	{ {1376 , 1722}, {1426 , 1686}, {1381 , 1861}, {1326 , 1896}, {1510 , 1723}, {1543 , 1726}, {1580 , 1758}, {1182 , 1714}, {1634 , 1727}, {1237 , 1764} }, 
	
	//Kefra
	{ {2367 , 4024}, {2236 , 4044}, {2236 , 3993}, {2209 , 3989}, {2453 , 4067}, {2485 , 4043}, {2537 , 3897}, {2489 , 3919}, {2269 , 3910}, {2202 , 3866} },
	
	//Nipple
	{ {3664 , 3024}, {3582 , 3007}, {3514 , 3008}, {3819 , 2977}, {3517 , 2889}, {3745 , 2977}, {3639 , 2877}, {3650 , 2727}, {3660 , 2773}, {3746 , 2879} }
};

void Exec_MSG_UseItem(int conn, char *pMsg)
{
	MSG_UseItem *m = (MSG_UseItem*)pMsg;

	if (pUser[conn].Mode != USER_PLAY)
	{
		SendHpMode(conn);
		return;
	}

	if (pUser[conn].Trade.OpponentID)
	{
		RemoveTrade(pUser[conn].Trade.OpponentID);
		RemoveTrade(conn);
		return;
	}

	if (pUser[conn].TradeMode)
	{
		RemoveTrade(conn);
		return;
	}

	if (pMob[conn].MOB.CurrentScore.Hp == 0)
	{
		SendHpMode(conn);
		AddCrackError(conn, 1, 15);

		if (m->SourType == ITEM_PLACE_CARRY)
			SendItem(conn, m->SourType, m->SourPos, &pMob[conn].MOB.Carry[m->SourPos]);

		else if (m->SourType == ITEM_PLACE_CARGO)
			SendItem(conn, m->SourType, m->SourPos, &pUser[conn].Cargo[m->SourPos]);

		else
			SendItem(conn, m->SourType, m->SourPos, &pMob[conn].MOB.Equip[m->SourPos]);

		return;
	}

	if (m->GridX >= MAX_GRIDX || m->GridY >= MAX_GRIDY)
	{
		CrackLog(conn, " Drop grid, out of range");
		return;
	}
	STRUCT_ITEM *item = GetItemPointer(&pMob[conn].MOB, pUser[conn].Cargo, m->SourType, m->SourPos);

	if (item == NULL)
	{
		sprintf(temp, "err,use item fail source %d %d", m->SourType, m->SourPos);
		ItemLog(temp, "-system", 0);
		return;
	}				 
	int Vol = BASE_GetItemAbility(item, EF_VOLATILE);
	int amount = BASE_GetItemAmount(item);

	STRUCT_ITEM Item;

#pragma region POT HP / MP
	if (Vol == 1)
	{
		int Hp = BASE_GetItemAbility(item, EF_HP);

		if(pUser[conn].PotionTime != 0)
		{
			int isTime = GetTickCount() - pUser[conn].PotionTime;			

			if(isTime < PotionDelay)
			{
				SendItem(conn, m->SourType, m->SourPos, item);
				return;
			}
		}
		pUser[conn].PotionTime = GetTickCount();

		if (Hp && pMob[conn].MOB.CurrentScore.Hp > 0)
		{
			int ReqHp = pUser[conn].ReqHp;
			ReqHp += Hp;

			if (ReqHp > pMob[conn].MOB.CurrentScore.MaxHp)
				ReqHp = pMob[conn].MOB.CurrentScore.MaxHp;

			pUser[conn].ReqHp = ReqHp;
		}
		int Mp = BASE_GetItemAbility(item, EF_MP);

		if (Mp)
		{
			int ReqMp = pUser[conn].ReqMp;
			ReqMp += Mp;

			if (ReqMp > pMob[conn].MOB.CurrentScore.MaxMp)
				ReqMp = pMob[conn].MOB.CurrentScore.MaxMp;

			pUser[conn].ReqMp = ReqMp;
		}
		pUser[conn].Unk_2688 = 1;
		SendSetHpMp(conn);

		if (amount > 1)
			BASE_SetItemAmount(item, amount - 1);

		else
			memset(item, 0, sizeof(STRUCT_ITEM));

		return;
	}

#pragma endregion
#pragma region PO / PL
	if (Vol == 4 || Vol == 5) // PO - PL
	{
		STRUCT_ITEM *dest = GetItemPointer(&pMob[conn].MOB, pUser[conn].Cargo, m->DestType, m->DestPos);

		if (dest == NULL)
		{
			sprintf(temp, "err,use item fail dest %d %d", m->DestType, m->DestPos);
			ItemLog(temp, "-system", 0);
			return;
		}

		int Egg = 0;

		if (dest->sIndex >= 2300 && dest->sIndex < 2330)
			Egg = 1;

		if (m->DestType == ITEM_PLACE_CARRY && m->DestPos < pMob[conn].MaxCarry)
		{
			if (dest == NULL)
			{
				SendClientMessage(conn, g_pMessageStringTable[_NN_Only_To_Equips]);
				ItemLog("use item fail - dest", pUser[conn].AccountName, pUser[conn].IP);
				SendItem(conn, m->SourType, m->SourPos, item);
				return;
			}

			int dstVolatile = BASE_GetItemAbility(dest, EF_VOLATILE);

			if (dstVolatile)
			{
				SendClientMessage(conn, g_pMessageStringTable[_NN_Only_To_Equips]);
				SendItem(conn, m->SourType, m->SourPos, item);
				return;
			}

			int isSanc = BASE_GetItemAbility(dest, EF_NOSANC);

			if (isSanc)
			{
				SendClientMessage(conn, g_pMessageStringTable[_NN_Cant_Refine_More]);
				SendItem(conn, m->SourType, m->SourPos, item);
				return;
			}

			int sanc = BASE_GetItemSanc(dest);
			int itemtype = 0;

			if(dest->sIndex >= 1234 && dest->sIndex <= 1237 || dest->sIndex >= 1369 && dest->sIndex <= 1372 || dest->sIndex >= 1519 && dest->sIndex <= 1522 || dest->sIndex >= 1669 && dest->sIndex <= 1672 || dest->sIndex >= 1901 && dest->sIndex <= 1910 || dest->sIndex == 1714)
				itemtype = 5;

			if (sanc >= 6 && Vol == 4)
			{
				SendClientMessage(conn, g_pMessageStringTable[_NN_Cant_Refine_More]);
				SendItem(conn, m->SourType, m->SourPos, item);
				return;
			}
/*
			if(pUser[conn].UseItemTime != 0)
			{
				int isTime = GetTickCount() - pUser[conn].UseItemTime;			

				if(isTime < 1000)
				{
					SendItem(conn, m->SourType, m->SourPos, item);
					SendClientMessage(conn, "Você deve esperar 1 segundo.");
					return;
				}
			}
			pUser[conn].UseItemTime = GetTickCount();
*/

#pragma region Refinar item Selado no inventário
			if (itemtype == 5)
			{
				if (sanc >= 9)
				{
					SendClientMessage(conn, g_pMessageStringTable[_NN_Cant_Refine_More]);
					SendItem(conn, m->SourType, m->SourPos, item);
					return;
				}

				if (sanc >= 6 && Vol == 4)
				{
					SendClientMessage(conn, g_pMessageStringTable[_NN_Cant_Refine_More]);
					SendItem(conn, m->SourType, m->SourPos, item);
					return;
				}

				if (sanc == 0)
				{
					if (dest->stEffect[0].cEffect && dest->stEffect[0].cEffect != 43 && (dest->stEffect[0].cEffect && dest->stEffect[0].cEffect < 116 || dest->stEffect[0].cEffect && dest->stEffect[0].cEffect > 125))
					{
						if (dest->stEffect[1].cEffect && dest->stEffect[1].cEffect != 43 && (dest->stEffect[1].cEffect && dest->stEffect[1].cEffect < 116 || dest->stEffect[1].cEffect && dest->stEffect[1].cEffect > 125))
						{
							if (dest->stEffect[2].cEffect && dest->stEffect[2].cEffect != 43 && (dest->stEffect[2].cEffect && dest->stEffect[2].cEffect < 116 || dest->stEffect[2].cEffect && dest->stEffect[2].cEffect > 125))
							{
								SendClientMessage(conn, g_pMessageStringTable[_NN_Cant_Refine_More]);
								SendItem(conn, m->SourType, m->SourPos, item);
								return;
							}
							dest->stEffect[2].cEffect = 43;
							dest->stEffect[2].cValue = 0;
						}
						else
						{
							dest->stEffect[1].cEffect = 43;
							dest->stEffect[1].cValue = 0;
						}
					}
					else
					{
						dest->stEffect[0].cEffect = 43;
						dest->stEffect[0].cValue = 0;
					}
				}

				int _rd = rand() % 115;

				if (_rd > 100)
					_rd -= 15;

				int ref = sanc;

				int _chance = g_pCelestialRate[ref];

				if (_rd <= _chance)
				{
					sprintf(temp, "%s", g_pMessageStringTable[_NN_Refine_Success]);
					SendClientMessage(conn, temp);

					BASE_SetItemSanc(dest, ref+1, 0);

					pMob[conn].GetCurrentScore(conn);

					SendScore(conn);
					sprintf(temp, "useitem,refine success %d+%d - (%d,%d,%d)", dest->sIndex, sanc, item->stEffect[0].cEffect, item->stEffect[1].cEffect, item->stEffect[2].cEffect);
					ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);


					if (amount > 1)
						BASE_SetItemAmount(item, amount - 1);

					else
						memset(item, 0, sizeof(STRUCT_ITEM));

									 
					SendItem(conn, m->DestType, m->DestPos, dest);
					SendEmotion(conn, 14, 3);
				}

				else
				{
					sprintf(temp, "%s", g_pMessageStringTable[_NN_Fail_To_Refine]);

					SendClientMessage(conn, temp);
					sprintf(temp, "useitem,refine fail %d+%d - (%d,%d,%d)", dest->sIndex, sanc, item->stEffect[0].cEffect, item->stEffect[1].cEffect, item->stEffect[2].cEffect);
					ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

					if (amount > 1)
						BASE_SetItemAmount(item, amount - 1);

					else
						memset(item, 0, sizeof(STRUCT_ITEM));

									 
					BASE_ClearItem(dest);
					SendItem(conn, m->DestType, m->DestPos, dest);

					int sFace = pMob[conn].MOB.Equip[0].sIndex / 10;

					if (sFace)
						SendEmotion(conn, 15, 0);

					else
						SendEmotion(conn, 20, 0);
				}

				return;
			}
			SendClientMessage(conn, g_pMessageStringTable[_NN_Only_To_Equips]);
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
#pragma endregion
		}

		if (m->DestType == ITEM_PLACE_EQUIP && m->DestPos == 0 || m->DestType || m->DestPos > 11 && m->DestPos < 15 && dest->sIndex != 753 && dest->sIndex != 769 && dest->sIndex != 1726 && !Egg)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Only_To_Equips]);

			ItemLog("err,use item fail - dest", pUser[conn].AccountName, pUser[conn].IP);

			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		if (dest == NULL)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Only_To_Equips]);
			ItemLog("use item fail - dest", pUser[conn].AccountName, pUser[conn].IP);
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		int dstVolatile = BASE_GetItemAbility(dest, EF_VOLATILE);

		if (dstVolatile)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Only_To_Equips]);
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		int isSanc = BASE_GetItemAbility(dest, EF_NOSANC);

		if (isSanc)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Cant_Refine_More]);
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		int sanc = BASE_GetItemSanc(dest);
		int itemtype = BASE_GetItemAbility(dest, EF_MOBTYPE);

		if (sanc >= 6 && Vol == 4)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Cant_Refine_More]);
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

#pragma region Refinar item celestial / HC
		if (itemtype == 3)
		{
			if (sanc >= REF_15)
			{
				SendClientMessage(conn, g_pMessageStringTable[_NN_Cant_Refine_More]);
				SendItem(conn, m->SourType, m->SourPos, item);
				return;
			}

			if (sanc >= 6 && Vol == 4)
			{
				SendClientMessage(conn, g_pMessageStringTable[_NN_Cant_Refine_More]);
				SendItem(conn, m->SourType, m->SourPos, item);
				return;
			}

			if (sanc == 0)
			{
				if (dest->stEffect[0].cEffect && dest->stEffect[0].cEffect != 43 && (dest->stEffect[0].cEffect && dest->stEffect[0].cEffect < 116 || dest->stEffect[0].cEffect && dest->stEffect[0].cEffect > 125))
				{
					if (dest->stEffect[1].cEffect && dest->stEffect[1].cEffect != 43 && (dest->stEffect[1].cEffect && dest->stEffect[1].cEffect < 116 || dest->stEffect[1].cEffect && dest->stEffect[1].cEffect > 125))
					{
						if (dest->stEffect[2].cEffect && dest->stEffect[2].cEffect != 43 && (dest->stEffect[2].cEffect && dest->stEffect[2].cEffect < 116 || dest->stEffect[2].cEffect && dest->stEffect[2].cEffect > 125))
						{
							SendClientMessage(conn, g_pMessageStringTable[_NN_Cant_Refine_More]);
							SendItem(conn, m->SourType, m->SourPos, item);
							return;
						}
						dest->stEffect[2].cEffect = 43;
						dest->stEffect[2].cValue = 0;
					}
					else
					{
						dest->stEffect[1].cEffect = 43;
						dest->stEffect[1].cValue = 0;
					}
				}
				else
				{
					dest->stEffect[0].cEffect = 43;
					dest->stEffect[0].cValue = 0;
				}
			}

			int _rd = rand() % 115;

			if (_rd > 100)
				_rd -= 15;

			int ref = sanc;

			if (ref == REF_10)
				ref = 10;

			else if (ref == REF_11)
				ref = 11;

			else if (ref == REF_12)
				ref = 12;

			else if (ref == REF_13)
				ref = 13;

			else if (ref == REF_14)
				ref = 14;

			else if (ref == REF_15)
				ref = 15;

			int _chance = g_pCelestialRate[ref];

			if (_rd <= _chance)
			{
				sprintf(temp, "%s", g_pMessageStringTable[_NN_Refine_Success]);
				SendClientMessage(conn, temp);

									 
				BASE_SetItemSanc(dest, ref+1, 0);

				pMob[conn].GetCurrentScore(conn);

				SendScore(conn);
				sprintf(temp, "useitem,refine success %d+%d - (%d,%d,%d)", dest->sIndex, sanc, item->stEffect[0].cEffect, item->stEffect[1].cEffect, item->stEffect[2].cEffect);
				ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);


				//SendNotice(g_pMessageStringTable[_SS_Combin_12Succ]);

				if (amount > 1)
					BASE_SetItemAmount(item, amount - 1);

				else
					memset(item, 0, sizeof(STRUCT_ITEM));

									 
				SendItem(conn, m->DestType, m->DestPos, dest);
				SendEmotion(conn, 14, 3);
			}

			else
			{
				sprintf(temp, "%s", g_pMessageStringTable[_NN_Fail_To_Refine]);

				SendClientMessage(conn, temp);
				sprintf(temp, "useitem,refine fail %d+%d - (%d,%d,%d)", dest->sIndex, sanc, item->stEffect[0].cEffect, item->stEffect[1].cEffect, item->stEffect[2].cEffect);
				ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

				if (amount > 1)
					BASE_SetItemAmount(item, amount - 1);

				else
					memset(item, 0, sizeof(STRUCT_ITEM));

									 
				BASE_SetItemSanc(dest, 0, 0);
				SendItem(conn, m->DestType, m->DestPos, dest);

				int sFace = pMob[conn].MOB.Equip[0].sIndex / 10;

				if (sFace)
					SendEmotion(conn, 15, 0);

				else
					SendEmotion(conn, 20, 0);
			}

			return;
		}
#pragma endregion
#pragma region Pedras Arch
		if(dest->sIndex >= 1752 && dest->sIndex <= 1759)
		{
			int _rd = rand() % 115;

			if (_rd > 100)
				_rd -= 15;

			int NextPedra = 1744;
			int RateSucess = 0;

			if(dest->sIndex == 1752)//Orc Tropper
			{
				if(_rd < 56)
					NextPedra = 1744;//Inteligencia
									 
				else if(_rd < 80)
					NextPedra = 1745;//Sabedoria

				else if(_rd < 90)
					NextPedra = 1746;//Misericórdia

				else if(_rd < 93)
					NextPedra = 1747;//Abismo

				RateSucess = 93;
			}

			else if(dest->sIndex == 1753)//Esqueleto
			{
				if(_rd < 21)
					NextPedra = 1744;//Inteligencia
									 
				else if(_rd < 76)
					NextPedra = 1745;//Sabedoria

				else if(_rd < 86)
					NextPedra = 1746;//Misericórdia

				else if(_rd < 90)
					NextPedra = 1747;//Abismo

				RateSucess = 90;
			}

			else if(dest->sIndex == 1754)//Dragão Lich
			{
				if(_rd < 3)
					NextPedra = 1744;//Inteligencia
									 
				else if(_rd < 21)
					NextPedra = 1745;//Sabedoria

				else if(_rd < 76)
					NextPedra = 1746;//Misericórdia

				else if(_rd < 85)
					NextPedra = 1747;//Abismo

				RateSucess = 85;
			}

			else if(dest->sIndex == 1755)//DemonLord
			{
				if(_rd < 3)
					NextPedra = 1744;//Inteligencia
									 
				else if(_rd < 10)
					NextPedra = 1745;//Sabedoria

				else if(_rd < 25)
					NextPedra = 1746;//Misericórdia

				else if(_rd < 80)
					NextPedra = 1747;//Abismo

				RateSucess = 80;
			}

			else if(dest->sIndex == 1756)//Manticora
			{
				if(_rd < 50)
					NextPedra = 1748;//Beleza
									 
				else if(_rd < 62)
					NextPedra = 1749;//Vitória

				else if(_rd < 68)
					NextPedra = 1750;//Originalidade

				else if(_rd < 70)
					NextPedra = 1751;//Reino

				RateSucess = 70;
			}

			else if(dest->sIndex == 1757)//Gargula de fogo
			{
				if(_rd < 9)
					NextPedra = 1748;//Beleza
									 
				else if(_rd < 59)
					NextPedra = 1749;//Vitória

				else if(_rd < 63)
					NextPedra = 1750;//Originalidade

				else if(_rd < 65)
					NextPedra = 1751;//Reino

				RateSucess = 65;
			}

			else if(dest->sIndex == 1758)//Lugefer
			{
				if(_rd < 2)
					NextPedra = 1748;//Beleza
									 
				else if(_rd < 8)
					NextPedra = 1749;//Vitória

				else if(_rd < 58)
					NextPedra = 1750;//Originalidade

				else if(_rd < 62)
					NextPedra = 1751;//Reino

				RateSucess = 62;
			}

			else if(dest->sIndex == 1759)//DemonLord
			{
				if(_rd < 2)
					NextPedra = 1748;//Beleza
									 
				else if(_rd < 5)
					NextPedra = 1749;//Vitória

				else if(_rd < 10)
					NextPedra = 1750;//Originalidade

				else if(_rd < 60)
					NextPedra = 1751;//Reino

				RateSucess = 60;
			}

			if (_rd <= RateSucess)
			{
				sprintf(temp, "%s", g_pMessageStringTable[_NN_Refine_Success]);
				SendClientMessage(conn, temp);

				dest->sIndex = NextPedra; 

				pMob[conn].GetCurrentScore(conn);

				SendScore(conn);
				sprintf(temp, "useitem,refine success %d+%d - (%d,%d,%d)", dest->sIndex, sanc, item->stEffect[0].cEffect, item->stEffect[1].cEffect, item->stEffect[2].cEffect);
				ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

				if (amount > 1)
					BASE_SetItemAmount(item, amount - 1);

				else
					memset(item, 0, sizeof(STRUCT_ITEM));

									 
				SendItem(conn, m->DestType, m->DestPos, dest);
				SendEmotion(conn, 14, 3);
			}

			else
			{
				sprintf(temp, "%s", g_pMessageStringTable[_NN_Fail_To_Refine]);

				SendClientMessage(conn, temp);
				sprintf(temp, "useitem,refine fail %d+%d - (%d,%d,%d)", dest->sIndex, sanc, item->stEffect[0].cEffect, item->stEffect[1].cEffect, item->stEffect[2].cEffect);
				ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

				if (amount > 1)
					BASE_SetItemAmount(item, amount - 1);

				else
					memset(item, 0, sizeof(STRUCT_ITEM));

									 
				BASE_SetItemSanc(dest, 0, 0);
				SendItem(conn, m->DestType, m->DestPos, dest);

				int sFace = pMob[conn].MOB.Equip[0].sIndex / 10;

				if (sFace)
					SendEmotion(conn, 15, 0);

				else
					SendEmotion(conn, 20, 0);
			}

			return;
		}
#pragma endregion

#pragma region Refinar brincos +10+
		if(sanc >= 9 && sanc <= REF_14 && Vol == 5 && m->DestPos == 8 && g_pItemList[dest->sIndex].nPos == 256)
		{
			int RateSucess = 15;
			int _rand = rand() % 100;

			int ref = sanc;

			if (ref == 9)
				ref = 9;

			else if (ref == REF_10)
				ref = 10;

			else if (ref == REF_11)
				ref = 11;

			else if (ref == REF_12)
				ref = 12;

			else if (ref == REF_13)
				ref = 13;

			else if (ref == REF_14)
				ref = 14;

			else if (ref == REF_15)
				ref = 15;

			int _chance = 15;

			if (_rand <= _chance)
			{
				sprintf(temp, "%s", g_pMessageStringTable[_NN_Refine_Success]);
				SendClientMessage(conn, temp);

									 
				BASE_SetItemSanc(dest, ref+1, 0);

				pMob[conn].GetCurrentScore(conn);

				SendScore(conn);
				sprintf(temp, "useitem,refine success %d+%d - (%d,%d,%d)", dest->sIndex, sanc, dest->stEffect[0].cEffect, dest->stEffect[1].cEffect, dest->stEffect[2].cEffect);
				ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

				if (amount > 1)
					BASE_SetItemAmount(item, amount - 1);

				else
					memset(item, 0, sizeof(STRUCT_ITEM));

									 
				SendItem(conn, m->DestType, m->DestPos, dest);
				SendEmotion(conn, 14, 3);
			}

			else
			{
				sprintf(temp, "%s", g_pMessageStringTable[_NN_Fail_To_Refine]);

				SendClientMessage(conn, temp);
				sprintf(temp, "useitem,refine fail %d+%d - (%d,%d,%d)", dest->sIndex, sanc, dest->stEffect[0].cEffect, dest->stEffect[1].cEffect, dest->stEffect[2].cEffect);
				ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

				if (amount > 1)
					BASE_SetItemAmount(item, amount - 1);

				else
					memset(item, 0, sizeof(STRUCT_ITEM));

									 
				BASE_ClearItem(dest);
				SendItem(conn, m->DestType, m->DestPos, dest);

				int sFace = pMob[conn].MOB.Equip[0].sIndex / 10;

				if (sFace)
					SendEmotion(conn, 15, 0);

				else
					SendEmotion(conn, 20, 0);
			}
			return;
		}
#pragma endregion

		if (sanc == 9 || sanc >= REF_11 || sanc < 0 || sanc >= 9 && dest->sIndex == 769)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Cant_Refine_More]);
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		int sancsucess = BASE_GetItemSancSuccess(dest);

		if (sanc < 0)
			sanc = 0;

		if (sanc == 0)
		{
			if (dest->stEffect[0].cEffect && dest->stEffect[0].cEffect != 43 && (dest->stEffect[0].cEffect && dest->stEffect[0].cEffect < 116 || dest->stEffect[0].cEffect && dest->stEffect[0].cEffect > 125))
			{
				if (dest->stEffect[1].cEffect && dest->stEffect[1].cEffect != 43 && (dest->stEffect[1].cEffect && dest->stEffect[1].cEffect < 116 || dest->stEffect[1].cEffect && dest->stEffect[1].cEffect > 125))
				{
					if (dest->stEffect[2].cEffect && dest->stEffect[2].cEffect != 43 && (dest->stEffect[2].cEffect && dest->stEffect[2].cEffect < 116 || dest->stEffect[2].cEffect && dest->stEffect[2].cEffect > 125))
					{
						SendClientMessage(conn, g_pMessageStringTable[_NN_Cant_Refine_More]);
						SendItem(conn, m->SourType, m->SourPos, item);
						return;
					}
					dest->stEffect[2].cEffect = 43;
					dest->stEffect[2].cValue = 0;
				}
				else
				{
					dest->stEffect[1].cEffect = 43;
					dest->stEffect[1].cValue = 0;
				}
			}
			else
			{
				dest->stEffect[0].cEffect = 43;
				dest->stEffect[0].cValue = 0;
			}
		}

		if (dest->sIndex >= 2300 && dest->sIndex < 2330 && pMob[conn].MOB.BaseScore.Level < 999 && BASE_GetBonusItemAbility(dest, EF_INCUDELAY) > 0)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Incu_Wait_More]);
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		int OriLacto = Vol - 4;
		int RateSucess = item->sIndex != 4141 ? BASE_GetSuccessRate(dest, Vol - 4) : BASE_GetSuccessRate(dest, 2);
		int _rand = rand() % 100;

		int ItemGrade = BASE_GetItemAbility(dest, EF_ITEMLEVEL);

		if (_rand <= RateSucess && RateSucess)
		{
			if(sanc == REF_10)
				BASE_SetItemSanc(dest, 11, BASE_GetItemGem(dest));
								 
			else
			{
				if (ItemGrade >= 1 && ItemGrade <= 5)
				{
					ItemGrade--;
					sanc += g_pSancGrade[OriLacto][ItemGrade];


					if (sanc >= 6 && Vol == 4)
						sanc = 6;

					else if (sanc > 9)
						sanc = 9;
				}
				else
					sanc++;


				sancsucess = 0;

				BASE_SetItemSanc(dest, sanc, 0);
			}
			pMob[conn].GetCurrentScore(conn);

			SendScore(conn);
			sprintf(temp, "useitem,refine success %d+%d - (%d,%d,%d)", dest->sIndex, sanc, dest->stEffect[0].cEffect, dest->stEffect[1].cEffect, dest->stEffect[2].cEffect);
			ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

			SendClientMessage(conn, g_pMessageStringTable[_NN_Refine_Success]);

			if (dest->sIndex >= 2300 && dest->sIndex < 2330)
			{
				dest->stEffect[2].cEffect = 84;

				int incurand = rand() & 0x80000003;

				if (incurand < 0)
					incurand = ((incurand - 1) | 0xFC) + 1;

				dest->stEffect[2].cValue = incurand + 6;

				int incubate = BASE_GetBonusItemAbility(dest, EF_INCUBATE);

				if (sanc >= incubate)
				{
					dest->sIndex += 30;

					dest->stEffect[0].sValue = 20000;
					dest->stEffect[1].cEffect = 1;
					dest->stEffect[1].cValue = rand() % 20 + 10;
					dest->stEffect[2].cEffect = 30;
					dest->stEffect[2].cValue = 1;

					SendClientMessage(conn, g_pMessageStringTable[_NN_INCUBATED]);
					MountProcess(conn, 0);

					SendItem(conn, m->DestType, m->DestPos, dest);
				}
			}
			SendItem(conn, m->DestType, m->DestPos, dest);
			SendEmotion(conn, 14, 3);

			if (amount > 1)
				BASE_SetItemAmount(item, amount - 1);

			else
				memset(item, 0, sizeof(STRUCT_ITEM));
		}

		else
		{
			sprintf(temp, "%s", g_pMessageStringTable[_NN_Fail_To_Refine]);

			SendClientMessage(conn, temp);
			sprintf(temp, "useitem,refine fail %d+%d - (%d,%d,%d)", dest->sIndex, sanc, dest->stEffect[0].cEffect, dest->stEffect[1].cEffect, dest->stEffect[2].cEffect);
			ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

			if (amount > 1)
				BASE_SetItemAmount(item, amount - 1);

			else
				memset(item, 0, sizeof(STRUCT_ITEM));

			if (rand() % 4 <= 2)
			{
				if (OriLacto == 1)
					++sancsucess;

				else if (sanc <= 5 && !OriLacto)
					++sancsucess;
			}
			if (dest->sIndex >= 2300 && dest->sIndex < 2330)
			{
				dest->stEffect[2].cEffect = 84;

				int incu = rand() & 0x80000003;
				if (incu < 0)
					incu = ((incu - 1) | 0xFC) + 1;

				dest->stEffect[2].cValue = incu;
			}

			if(sanc != REF_10)
				BASE_SetItemSanc(dest, sanc, sancsucess);

			SendItem(conn, m->DestType, m->DestPos, dest);

			int sFace = pMob[conn].MOB.Equip[0].sIndex / 10;

			if (sFace)
			SendEmotion(conn, 15, 0);

			else
				SendEmotion(conn, 20, 0);
		}

		return;
	}

#pragma endregion
#pragma region Pilula Orc
	if (Vol == 6) // Pilula Orc
	{
		if(pMob[conn].extra.QuestInfo.Mortal.PilulaOrc)
			return;

		pMob[conn].MOB.SkillBonus += 9;
		pMob[conn].extra.QuestInfo.Mortal.PilulaOrc = 1;
		SendEmotion(conn, 14, 3);
		SendEtc(conn);

		if (amount > 1)
			BASE_SetItemAmount(item, amount - 1);
		else
			memset(item, 0, sizeof(STRUCT_ITEM));

		sprintf(temp, "useitem,pilula_orc quest complete.");
		ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);
		return;
	}
#pragma endregion
#pragma region Poeira de Fada
	if (Vol == 7)//Poeira de Fada
	{
		int Level = pMob[conn].MOB.BaseScore.Level;
		int maxlevel = pMob[conn].extra.ClassMaster == MORTAL || pMob[conn].extra.ClassMaster == ARCH ? MAX_LEVEL : MAX_CLEVEL;

		if (Level >= maxlevel || Level < 0 || rand() % 2)
		{
		//	int sFace = pMob[conn].MOB.Equip[0].sIndex / 10;

	//		if (sFace)
	//			SendEmotion(conn, 15, 0);
	//		else
	//			SendEmotion(conn, 20, 0);

	//		SendClientMessage(conn, g_pMessageStringTable[_NN_Fail_To_Level_Up]);
			pMob[conn].MOB.Exp = pMob[conn].extra.ClassMaster == MORTAL || pMob[conn].extra.ClassMaster == ARCH ? g_pNextLevel[Level + 1] : g_pNextLevel_2[Level + 1];

			if (pMob[conn].CheckGetLevel())
			{
				SendClientMessage(conn, "+ + + + SUBIU DE NÍVEL + + + + ");

				SendEmotion(conn, 14, 3);

				SendScore(conn);
				SendEtc(conn);

				if (pMob[conn].extra.ClassMaster == MORTAL)
					DoItemLevel(conn);
			}
		}
		else
		{
			pMob[conn].MOB.Exp = pMob[conn].extra.ClassMaster == MORTAL || pMob[conn].extra.ClassMaster == ARCH ? g_pNextLevel[Level+1] : g_pNextLevel_2[Level+1];

			if (pMob[conn].CheckGetLevel())
			{
				SendClientMessage(conn, "+ + + + SUBIU DE NÍVEL + + + + ");

				SendEmotion(conn, 14, 3);

				SendScore(conn);
				SendEtc(conn);

				if(pMob[conn].extra.ClassMaster == MORTAL)
					DoItemLevel(conn);
			}
		}

		if (amount > 1)
			BASE_SetItemAmount(item, amount - 1);

		else
			memset(item, 0, sizeof(STRUCT_ITEM));

		sprintf(temp, "useitem,poeira_de_fada classmaster:%d level:%d", pMob[conn].extra.ClassMaster, pMob[conn].MOB.BaseScore.Level);
		ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);
		return;
	}
	#pragma endregion
#pragma region Olho Crescente
	if (Vol == 8)
	{
		if(pMob[conn].extra.ClassMaster != MORTAL)
			return;

		pMob[conn].MOB.Exp += 2500;

		sprintf(temp, g_pMessageStringTable[_NS_GETEXP], 2500);
		SendClientMessage(conn, temp);

		SendEmotion(conn, 14, 3);

		if (pMob[conn].CheckGetLevel() == 4)
		{
			SetCircletSubGod(conn);
			SendClientMessage(conn, g_pMessageStringTable[_NN_Level_Up]);
			SendScore(conn);

			if(pMob[conn].extra.ClassMaster == MORTAL)
				DoItemLevel(conn);
		}
		SendEtc(conn);

		if (amount > 1)
			BASE_SetItemAmount(item, amount-1);
							 
		else
			memset(item, 0, sizeof(STRUCT_ITEM));
		
		sprintf(temp, "useitem,olho_crescente level:%d exp recebida:2500", pMob[conn].MOB.BaseScore.Level);
		ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);
		return;
	}
#pragma endregion
#pragma region Adamantita/beril etc LE
	if (Vol == 9) // Adamantita / Beril etc
	{
		STRUCT_ITEM *dest = GetItemPointer(&pMob[conn].MOB, pUser[conn].Cargo, m->DestType, m->DestPos);

		if (dest == NULL)
		{
			sprintf(temp, "err,use item fail dest %d %d", m->DestType, m->DestPos);
			ItemLog(temp, "-system", 0);
			return;
		}

		int sourIndex = item->sIndex;
		int destIndex = dest->sIndex;
		int Type = sourIndex - 575;

		if (sourIndex - 575 >= 0 && Type < 4)
		{
			int UniqueType = -1;
			int nUnique = g_pItemList[destIndex].nUnique;

			if (nUnique == 5 || nUnique == 14 || nUnique == 24 || nUnique == 34)
				UniqueType = 0;

			if (nUnique == 6 || nUnique == 15 || nUnique == 25 || nUnique == 35)
				UniqueType = 1;

			if (nUnique == 7 || nUnique == 16 || nUnique == 26 || nUnique == 36)
				UniqueType = 2;

			if (nUnique == 8 || nUnique == 17 || nUnique == 27 || nUnique == 37)
				UniqueType = 3;

			if (nUnique == 10 || nUnique == 20 || nUnique == 30 || nUnique == 40)
				UniqueType = 3;

			if (nUnique == -1)
			{
				SendItem(conn, m->SourType, m->SourPos, item);
				SendClientMessage(conn, g_pMessageStringTable[Type + _NN_Only_Level1_Armor]);
			}
			else
			{
				if (UniqueType == Type)
				{
					int Grade = g_pItemList[destIndex].Grade;

					if (Grade > 0 && Grade < 4)
					{
						int _rand = rand() % 100;
						int chance = 50;
						if (_rand <= 50)
						{
							int Extra = g_pItemList[destIndex].Extra;

							dest->sIndex = Extra;

							SendItem(conn, m->DestType, m->DestPos, dest);

							if (amount > 1)
								BASE_SetItemAmount(item, amount - 1);

							else
								memset(item, 0, sizeof(STRUCT_ITEM));

							SendScore(conn);
							SendEmotion(conn, 14, 0);

							sprintf(temp, "useitem,regendary success %d - (%d,%d,%d)", dest->sIndex,
								item->stEffect[0].cEffect, item->stEffect[1].cEffect, item->stEffect[2].cEffect);

							ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);
							sprintf(temp, "%s(%d/%d)", g_pMessageStringTable[_NN_Refine_Success], _rand, chance);

							SendClientMessage(conn, temp);
						}
						else
						{
							if (pMob[conn].MOB.Equip[0].sIndex / 10)
								SendEmotion(conn, 15, 0);

							else
								SendEmotion(conn, 20, 0);

							sprintf(temp, "useitem,regendary fail %d - (%d,%d,%d)",
								dest->sIndex, item->stEffect[0].cEffect, item->stEffect[1].cEffect, item->stEffect[2].cEffect);

							ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

							sprintf(temp, "%s(%d/%d)", g_pMessageStringTable[_NN_Fail_To_Refine], _rand, chance);
							SendClientMessage(conn, temp);

							if (amount > 1)
								BASE_SetItemAmount(item, amount - 1);

							else
								memset(item, 0, sizeof(STRUCT_ITEM));
						}
					}
					else
					{
						SendItem(conn, m->SourType, m->SourPos, item);
						SendClientMessage(conn, g_pMessageStringTable[_NN_Cant_Refine_More]);
					}
				}
				else
				{
					SendItem(conn, m->SourType, m->SourPos, item);
					SendClientMessage(conn, g_pMessageStringTable[Type + _NN_Only_Level1_Armor]);
				}
			}
		}
		else
			SendItem(conn, m->SourType, m->SourPos, item);

		return;
	}
	#pragma endregion
#pragma region Poções Kappa Combatente Mental Sephira Antídoto
	if (Vol == 10 || Vol == 55 || Vol == 200 || Vol == 201 || Vol == 56 || Vol == 52 || Vol == 53 || Vol == 57 || Vol == 202)
	{
		int value = 0;

		int tempo = 80;

		//Kappa
		if(item->sIndex == 787)
			value = 1;

		//Combatente
		else if(item->sIndex == 1764)
			value = 2;

		//Mental
		else if(item->sIndex == 1765)
			value = 3;

		//Mental 60m
		else if(item->sIndex == 3312)
		{
			tempo = AFFECT_1H;
			value = 3;
		}
		//Combatente 60m
		else if(item->sIndex == 3311)
		{
			tempo = AFFECT_1H;
			value = 2;
		}

		//Kappa 30m
		else if(item->sIndex == 3310)
		{
			tempo = AFFECT_1H/2;
			value = 1;
		}

		//Mental 20h
		else if(item->sIndex == 3321)
		{
			tempo = AFFECT_1H*20;
			value = 3;
		}

		//Combatente 20h
		else if(item->sIndex == 3320)
		{
			tempo = AFFECT_1H*20;
			value = 2;
		}

		//Kappa 20h
		else if (item->sIndex == 3319)
		{
			tempo = AFFECT_1H * 20;
			value = 1;
		}

		//Sephira 7d
		else if (item->sIndex == 3361)
		{
			tempo = AFFECT_1H * 168;
			value = 4;
		}

		//Sephira 15d
		else if (item->sIndex == 3362)
		{
			tempo = AFFECT_1H * 360;
			value = 4;
		}

		//Sephira 30d
		else if (item->sIndex == 3363)
		{
			tempo = AFFECT_1H * 720;
			value = 4;
		}

		//Antídoto
		else if (item->sIndex == 416)
		{
			tempo = AFFECT_1H / 2;
			value = 5;
		}

		int EmptyAffect = -1;

		for(int i = 0; i < MAX_AFFECT; i++)
		{
			if(pMob[conn].Affect[i].Type == 4 && pMob[conn].Affect[i].Value == value)
			{
				EmptyAffect = i;
				break;
			}
		}

		for(int i = 0; i < MAX_AFFECT && EmptyAffect == -1; i++)
		{
			if(pMob[conn].Affect[i].Type == 0)
			{
				EmptyAffect = i;
				break;
			}
		}

		if(EmptyAffect != -1)
		{
			pMob[conn].Affect[EmptyAffect].Type = 4;
			pMob[conn].Affect[EmptyAffect].Level = 0;
			pMob[conn].Affect[EmptyAffect].Value = value;
			pMob[conn].Affect[EmptyAffect].Time = tempo;

			pMob[conn].GetCurrentScore(conn);
			SendScore(conn);
		}
							
		if (amount > 1)
			BASE_SetItemAmount(item, amount - 1);

		else
			memset(item, 0, sizeof(STRUCT_ITEM));

		sprintf(temp, "useitem,poção_kappa affectslot:%d value:%d time:%d", EmptyAffect, value, tempo);
		ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);
		return;
	}
#pragma endregion
#pragma region Retorno
	if (Vol == 11)
	{
		DoRecall(conn);

		if (amount > 1)
			BASE_SetItemAmount(item, amount - 1);

		else
			memset(item, 0, sizeof(STRUCT_ITEM));

		sprintf(temp, "useitem,recall");
		ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);
		return;
	}
#pragma endregion
#pragma region Gema Estelar
	if (Vol == 12)
	{
		unsigned char map_att = GetAttribute(pMob[conn].TargetX, pMob[conn].TargetY);

		if((pMob[conn].TargetX/128) == 9 && (pMob[conn].TargetY/128) == 1)
			goto CanSave;

		if((pMob[conn].TargetX/128) == 8 && (pMob[conn].TargetY/128) == 2)
			goto CanSave;

		if (map_att & 4 && pMob[conn].MOB.CurrentScore.Level < 1000)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Cant_Use_That_Here]);
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

							 
		int Arena = BASE_GetArena(pMob[conn].TargetX, pMob[conn].TargetY);
		int Village = BASE_GetVillage(pMob[conn].TargetX, pMob[conn].TargetY);

		if (Arena < MAX_GUILDZONE || Village < MAX_GUILDZONE)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Cant_Use_That_Here]);
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}
	CanSave:
		int sFace = pMob[conn].MOB.Equip[0].sIndex / 10;
							
		if (sFace == 0)
			SendEmotion(conn, 23, 0);

		else if (sFace == 1)
			SendEmotion(conn, 15, 0);

		else if (sFace == 2)
			SendEmotion(conn, 15, 0);

		else if (sFace == 3)
			SendEmotion(conn, 15, 0);
							 

		pMob[conn].MOB.SPX = pMob[conn].TargetX;
		pMob[conn].MOB.SPY = pMob[conn].TargetY;

		sprintf(temp, "useitem,gema salva %d %d", pMob[conn].TargetX, pMob[conn].TargetY);
		ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

		if (amount > 1)
			BASE_SetItemAmount(item, amount - 1);

		else
			memset(item, 0, sizeof(STRUCT_ITEM));
							 
		SendClientMessage(conn, g_pMessageStringTable[_NN_Set_Warp]);

		return;
	}
#pragma endregion
#pragma region Portal
	if (Vol == 13)
	{
		if((pMob[conn].MOB.SPX/128) == 9 && (pMob[conn].MOB.SPY/128) == 1 || (pMob[conn].MOB.SPX/128) == 8 && (pMob[conn].MOB.SPY/128) == 2)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Cant_Use_That_Here]);
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		unsigned char map_att = GetAttribute(pMob[conn].TargetX, pMob[conn].TargetY);

		if (map_att & 4 && pMob[conn].MOB.CurrentScore.Level < 1000)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Cant_Use_That_Here]);
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		DoTeleport(conn, pMob[conn].MOB.SPX, pMob[conn].MOB.SPY);

		sprintf(temp, "useitem,teleport portal scrool %d %d", pMob[conn].TargetX, pMob[conn].TargetY);
		ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

		if (amount > 1)
			BASE_SetItemAmount(item, amount - 1);

		else
			memset(item, 0, sizeof(STRUCT_ITEM));

		return;
	}
#pragma endregion
#pragma region DBRecord
	if (Vol == 14)
	{
		MSG_MessageDBRecord sm_dbrec;
		memset(&sm_dbrec, 0, sizeof(MSG_MessageDBRecord));

		sm_dbrec.Type = _MSG_MessageDBRecord;
		sm_dbrec.ID = 0;
		sm_dbrec.Size = sizeof(MSG_MessageDBRecord);

		sprintf(sm_dbrec.Record, "TCK %d %s %d %d %d %d", ServerIndex, pUser[conn].AccountName, item->stEffect[0].cEffect, item->stEffect[1].cEffect, item->stEffect[2].cEffect, item->sIndex);

		DBServerSocket.SendOneMessage((char*)&sm_dbrec, sizeof(MSG_MessageDBRecord));

		if (amount > 1)
			BASE_SetItemAmount(item, amount - 1);

		else
			memset(item, 0, sizeof(STRUCT_ITEM));

		return;
	}
#pragma endregion
#pragma region Rações
	if (Vol == 15)
	{
		if (m->DestType || m->DestPos != 14)
		{
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		STRUCT_ITEM *dest = &pMob[conn].MOB.Equip[14];

		if (dest->sIndex < 2330 || dest->sIndex >= 2390)
		{
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		int mount = (dest->sIndex - 2330) % 30;

		if (mount >= 6 && mount <= 15 || mount == 27)
			mount = 6;

		if (mount == 19)
			mount = 7;

		if (mount == 20)
			mount = 8;

		if (mount == 21 || mount == 22 || mount == 23 || mount == 28)
			mount = 9;

		if (mount == 24 || mount == 25 || mount == 26)
			mount = 10;

		if (mount == 29)
			mount = 19;

		int racid = item->sIndex >= 3367 ? item->sIndex - 3367 : item->sIndex - 2420;

		int racao = racid % 30;

		if (mount == racao)
		{
			if (dest->stEffect[0].sValue > 0)
			{
				if ((pMob[conn].MOB.Equip[14].stEffect[0].sValue + 5000) > 30000)
					pMob[conn].MOB.Equip[14].stEffect[0].sValue = 30000;
				else
					pMob[conn].MOB.Equip[14].stEffect[0].sValue += 5000;

				int _racao = dest->stEffect[2].cEffect + 2;

				if (_racao > 100)
					_racao = 100;

				dest->stEffect[2].cEffect = _racao;

				if (dest->sIndex >= 2330 && dest->sIndex < 2360)
					MountProcess(conn, 0);

				if (dest->sIndex >= 2360 && dest->sIndex < 2390)
					ProcessAdultMount(conn, 0);

				if (amount > 1)
					BASE_SetItemAmount(item, amount - 1);

				else
					memset(item, 0, sizeof(STRUCT_ITEM));
									 
				SendClientSignalParm(conn, ESCENE_FIELD, _MSG_SoundEffect, 270);
				SendItem(conn, m->DestType, m->DestPos, dest);
			}
			else
				SendItem(conn, m->SourType, m->SourPos, item);
		}
		else
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Mount_Not_Match]);
			SendItem(conn, m->SourType, m->SourPos, item);
		}

		sprintf(temp, "useitem,ração mount:%d", mount);
		ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);
		return;
	}
#pragma endregion
#pragma region Âmago
	if (Vol == 16)
	{
		if (m->DestType || m->DestPos != 14)
		{
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		STRUCT_ITEM *dest = &pMob[conn].MOB.Equip[14];

		if (dest->sIndex < 2330 || dest->sIndex >= 2390 || dest->stEffect[0].sValue <= 0)
		{
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		int mountIndex = (dest->sIndex - 2330) % 30;
		int amgIndex = (item->sIndex - 2390) % 30;

		if(mountIndex == 28)//Sleipnir
			mountIndex = 21;

		if(mountIndex == 27)//Svadilfire
			mountIndex = 10;

		if (mountIndex != amgIndex)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Mount_Not_Match]);
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		dest->stEffect[0].sValue = 20000;
		dest->stEffect[2].cEffect = 100;
		int level = dest->stEffect[1].cEffect;

		if (level >= 120 && dest->sIndex >= 2360 && dest->sIndex < 2390)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Cant_Upgrade_More]);
			SendItem(conn, m->SourType, m->SourPos, item);
			ProcessAdultMount(conn, 0);
			return;
		}

		if (dest->sIndex >= 2360 && dest->sIndex < 2390)
		{
			int rate = BASE_GetGrowthRate(dest);
			int _rand = rand() % 101;
								
			if (_rand > rate)
			{
				if (pMob[conn].MOB.Equip[0].sIndex / 10)
					SendEmotion(conn, 15, 0);
				else
					SendEmotion(conn, 20, 0);
									 
				sprintf(temp, "%s", g_pMessageStringTable[_NN_Fail_To_Refine]);
				SendClientMessage(conn, temp);
									 
				sprintf(temp, "useitem,mount refine fail %d+%d (%d,%d,%d)", dest->sIndex, dest->stEffect[1].cEffect,
					item->stEffect[0].cEffect, item->stEffect[1].cEffect, item->stEffect[2].cEffect);
									 
				ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);
									 
				if (amount > 1)
					BASE_SetItemAmount(item, amount-1);
									 
				else
					memset(item, 0, 8);

				if (!(rand() % 5) && dest->stEffect[1].cEffect > 0)
					dest->stEffect[1].cEffect--;

				ProcessAdultMount(conn, 0);
				return;
			}
			sprintf(temp, "useitem,mount refine success %d+%d (%d,%d,%d)", dest->sIndex, dest->stEffect[1].cValue,
				item->stEffect[0].cEffect, item->stEffect[1].cEffect, item->stEffect[2].cEffect);

			ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);
		}
		int levelUP = 1;

		if (LOCALSERVER)
			levelUP = 10;

		level = levelUP + level;
		dest->stEffect[1].cEffect = level;
		dest->stEffect[2].cValue = 1;

		if (level >= 25 && dest->sIndex == 2330
			|| level >= 50 && dest->sIndex == 2331
			|| level >= 100 && dest->sIndex >= 2332 && dest->sIndex < 2360)
		{
			dest->sIndex += 30;
								
			dest->stEffect[1].cValue = rand() % 20 + dest->stEffect[1].cEffect;
			dest->stEffect[1].cEffect = 0;
			dest->stEffect[2].cValue = 0;
								 
			SendClientMessage(conn, g_pMessageStringTable[_NN_Mount_Growth]);
			MountProcess(conn, 0);
		}

		SendEmotion(conn, 14, 3);

		if (dest->sIndex >= 2330 && dest->sIndex < 2360)
			MountProcess(conn, 0);

		if (dest->sIndex >= 2360 && dest->sIndex < 2390)
			ProcessAdultMount(conn, 0);

		SendItem(conn, m->DestType, m->DestPos, dest);

		if (amount > 1)
			BASE_SetItemAmount(item, amount - 1);

		else
			memset(item, 0, sizeof(STRUCT_ITEM));

		return;
	}
#pragma endregion
#pragma region GUILDBOARD
	if (Vol == 17)
	{
		int Guild = BASE_GetGuild(item);

		if (Guild > 0)
		{
			MSG_STANDARDPARM2 sm_gbo;
			memset(&sm_gbo, 0, sizeof(MSG_STANDARDPARM2));

			sm_gbo.Type = _MSG_GuildBoard;
			sm_gbo.ID = ESCENE_FIELD;
			sm_gbo.Parm1 = Guild;
			sm_gbo.Parm2 = GUILDBOARD;
			sm_gbo.Size = sizeof(MSG_STANDARDPARM2);

			pUser[conn].cSock.AddMessage((char*)&sm_gbo, sizeof(MSG_STANDARDPARM2));
		}

		return;
	}
#pragma endregion
#pragma region Fogos de Artificio
	if (Vol == 19)
	{
		SendEmotion(conn, 100, rand()%5);

		if (amount > 1)
			BASE_SetItemAmount(item, amount - 1);

		else
			memset(item, 0, sizeof(STRUCT_ITEM));

		sprintf(temp, "useitem,fogos_de_artificio");
		ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);
		return;
	}
#pragma endregion
#pragma region Pergaminho da agua M
	if (Vol >= 21 && Vol <= 30)
	{
		int TargetX = pMob[conn].TargetX;
		int TargetY = pMob[conn].TargetY;

		int inside = 0;

		for (int i = 0; i < 10; i++)
		{
			if (TargetX >= WaterScrollPosition[1][i][0] - 12 && TargetY >= WaterScrollPosition[1][i][1] - 12
				&& TargetX <= WaterScrollPosition[1][i][0] + 12 && TargetY <= WaterScrollPosition[1][i][1] + 12 && i >= 9)
			{
				inside = 1;
				break;
			}

			if (TargetX >= WaterScrollPosition[1][i][0] - 8 && TargetY >= WaterScrollPosition[1][i][1] - 8
				&& TargetX <= WaterScrollPosition[1][i][0] + 8 && TargetY <= WaterScrollPosition[1][i][1] + 8)
			{
				inside = 1;
				break;
			}
		}

		if (((TargetX / 4) != 491 || (TargetY / 4) != 443 || Vol == 29) && inside == 0)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Cant_Use_That_Here]);
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		if (pMob[conn].Leader != -1 && pMob[conn].Leader)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Party_Leader_Only]);
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		char UserName[128];

		int Sala = Vol - 21;
		int UserArea = Sala <= 7 ? GetUserInArea(WaterScrollPosition[1][Vol - 21][0] - 8, WaterScrollPosition[1][Vol - 21][1] - 8, WaterScrollPosition[1][Vol - 21][0] + 8, WaterScrollPosition[1][Vol - 21][1] + 8, UserName) : GetUserInArea(WaterScrollPosition[1][Vol - 21][0] - 12, WaterScrollPosition[1][Vol - 21][1] - 12, WaterScrollPosition[1][Vol - 21][0] + 12, WaterScrollPosition[1][Vol - 21][1] + 12, UserName);
							 
		if (UserArea >= 1)
		{
			sprintf(temp, g_pMessageStringTable[_NN_Someone_is_on_quest], UserName, UserArea - 1);
								
			SendClientMessage(conn, temp);
			SendItem(conn, m->SourType, m->SourPos, item);
								 
			return;
		}

		WaterClear1[1][Vol - 21] = Sala <= 7 ? 30 : 15;

		DoTeleport(conn, WaterScrollPosition[1][Vol - 21][0], WaterScrollPosition[1][Vol - 21][1]);
		SendClientSignalParm(conn, ESCENE_FIELD, _MSG_StartTime, WaterClear1[1][Vol - 21]*2);

		for (int i = 0; i < MAX_PARTY; i++)
		{
			int partyconn = pMob[conn].PartyList[i];

			if (partyconn > 0 && partyconn < MAX_USER && partyconn != conn && pUser[partyconn].Mode == USER_PLAY)
			{
				DoTeleport(partyconn, WaterScrollPosition[1][Vol - 21][0], WaterScrollPosition[1][Vol - 21][1]);
				SendClientSignalParm(partyconn, ESCENE_FIELD, _MSG_StartTime, WaterClear1[1][Vol - 21]*2);
			}
		}

		if (Sala <= 7)
		{
			GenerateMob(Sala + WATER_M_INITIAL, 0, 0);
			GenerateMob(Sala + WATER_M_INITIAL, 0, 0);
			RebuildGenerator();
		}
		else if (Sala == 9)
		{
			int _rand = rand() % 10;
								 
			if (_rand < 4)
				GenerateMob(WATER_M_INITIAL + 8, 0, 0);
								 
			else if (_rand < 5)
				GenerateMob(WATER_M_INITIAL + 9, 0, 0);

			else if (_rand < 6)
				GenerateMob(WATER_M_INITIAL + 10, 0, 0);
								 
			else
				GenerateMob(WATER_M_INITIAL + 11, 0, 0);
		}

		sprintf(temp, "useitem,water scroll M %d", Sala);
		ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

		if (amount > 1)
			BASE_SetItemAmount(item, amount - 1);

		else
			memset(item, 0, sizeof(STRUCT_ITEM));

		return;
	}
#pragma endregion
#pragma region Livros Sephira
	if (Vol >= 31 && Vol <= 38)
	{
		int ItemID = item->sIndex;

		if (ItemID <= 0 || ItemID > MAX_ITEMLIST)
		{
			ItemLog("err, wrong skill book index", pUser[conn].AccountName, pUser[conn].IP);
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		int SkillLearn = 1 << (Vol-7);

		if ((pMob[conn].MOB.LearnedSkill & SkillLearn) != 0)
		{
			SendItem(conn, m->SourType, m->SourPos, item);
			SendClientMessage(conn, g_pMessageStringTable[_NN_Already_Learned_It]);
			return;
		}

		pMob[conn].MOB.LearnedSkill = pMob[conn].MOB.LearnedSkill | SkillLearn;
							 
		sprintf(temp, g_pMessageStringTable[_SN_Learn_Sephera], g_pItemList[ItemID].Name);
							 
		SendClientMessage(conn, temp);
		SendEtc(conn);
							 
		SetAffect(conn, 44, 20, 20);

		if (amount > 1)
			BASE_SetItemAmount(item, amount - 1);

		else
			memset(item, 0, sizeof(STRUCT_ITEM));

		sprintf(temp, "useitem,skillbook index:%d", ItemID);
		ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);
		return;
	}
#pragma endregion
#pragma region Contratos do reino
	if (Vol >= 41 && Vol < 50)
	{
		int Evock = Vol - 41;

		if (pMob[conn].MOB.Clan != 7 && pMob[conn].MOB.Clan != 8)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Only_same_kingdom]);
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}
							 
		if (Evock < 5 && pMob[conn].MOB.Clan == 8)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Only_same_kingdom]);
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}
							 
		if (Evock >= 5 && pMob[conn].MOB.Clan == 7)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Only_same_kingdom]);
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

							 
		if (CastleState == 0)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Only_when_castle_war]);
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

							 
		GenerateSummon(conn, Evock + 28, 0, 1);
							 
		if (amount > 1)
			BASE_SetItemAmount(item, amount - 1);

		else
			memset(item, 0, sizeof(STRUCT_ITEM));

		sprintf(temp, "useitem,contrato_de_reino evock:%d", Evock + 28);
		ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);
		return;
	}
#pragma endregion
#pragma region Pergaminho da agua N
	if (Vol >= 131 && Vol <= 140)
	{
		int TargetX = pMob[conn].TargetX;
		int TargetY = pMob[conn].TargetY;

		int inside = 0;

		for (int i = 0; i < 10; i++)
		{
			if (TargetX >= WaterScrollPosition[0][i][0] - 12 && TargetY >= WaterScrollPosition[0][i][1] - 12
				&& TargetX <= WaterScrollPosition[0][i][0] + 12 && TargetY <= WaterScrollPosition[0][i][1] + 12 && i >= 9)
			{
				inside = 1;
				break;
			}

			if (TargetX >= WaterScrollPosition[0][i][0] - 8 && TargetY >= WaterScrollPosition[0][i][1] - 8
				&& TargetX <= WaterScrollPosition[0][i][0] + 8 && TargetY <= WaterScrollPosition[0][i][1] + 8)
			{
				inside = 1;
				break;
			}
		}

		if (((TargetX / 4) != 491 || (TargetY / 4) != 443 || Vol == 139) && inside == 0)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Cant_Use_That_Here]);
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		if (pMob[conn].Leader != -1 && pMob[conn].Leader)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Party_Leader_Only]);
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		char UserName[128];

		int Sala = Vol - 131;
		int UserArea = Sala <= 7 ? GetUserInArea(WaterScrollPosition[0][Vol - 131][0] - 8, WaterScrollPosition[0][Vol - 131][1] - 8, WaterScrollPosition[0][Vol - 131][0] + 8, WaterScrollPosition[0][Vol - 131][1] + 8, UserName) : GetUserInArea(WaterScrollPosition[0][Vol - 131][0] - 12, WaterScrollPosition[0][Vol - 131][1] - 12, WaterScrollPosition[0][Vol - 131][0] + 12, WaterScrollPosition[0][Vol - 131][1] + 12, UserName);

		if (UserArea >= 1)
		{
			sprintf(temp, g_pMessageStringTable[_NN_Someone_is_on_quest], UserName, UserArea - 1);

			SendClientMessage(conn, temp);
			SendItem(conn, m->SourType, m->SourPos, item);

			return;
		}

		WaterClear1[0][Vol - 131] = Sala <= 7 ? 30 : 15;

		DoTeleport(conn, WaterScrollPosition[0][Vol - 131][0], WaterScrollPosition[0][Vol - 131][1]);
		SendClientSignalParm(conn, ESCENE_FIELD, _MSG_StartTime, WaterClear1[0][Vol - 131] * 2);

		for (int i = 0; i < MAX_PARTY; i++)
		{
			int partyconn = pMob[conn].PartyList[i];

			if (partyconn > 0 && partyconn < MAX_USER && partyconn != conn && pUser[partyconn].Mode == USER_PLAY)
			{
				DoTeleport(partyconn, WaterScrollPosition[0][Vol - 131][0], WaterScrollPosition[0][Vol - 131][1]);
				SendClientSignalParm(partyconn, ESCENE_FIELD, _MSG_StartTime, WaterClear1[0][Vol - 131] * 2);
			}
		}

		if (Sala <= 7)
		{
			GenerateMob(Sala + WATER_N_INITIAL, 0, 0);
			GenerateMob(Sala + WATER_N_INITIAL, 0, 0);
			//RebuildGenerator();
		}
		else if (Sala == 9)
		{
			int _rand = rand() % 10;

			if (_rand < 4)
				GenerateMob(WATER_N_INITIAL + 8, 0, 0);

			else if (_rand < 5)
				GenerateMob(WATER_N_INITIAL + 9, 0, 0);

			else if (_rand < 6)
				GenerateMob(WATER_N_INITIAL + 10, 0, 0);

			else
				GenerateMob(WATER_N_INITIAL + 11, 0, 0);
		}

		sprintf(temp, "useitem,water scroll N %d", Sala);
		ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

		if (amount > 1)
			BASE_SetItemAmount(item, amount - 1);

		else
			memset(item, 0, sizeof(STRUCT_ITEM));

		return;
	}
#pragma endregion
#pragma region Pergaminho da agua A
	if (Vol >= 161 && Vol <= 170)
	{
		int TargetX = pMob[conn].TargetX;
		int TargetY = pMob[conn].TargetY;

		int inside = 0;

		for (int i = 0; i < 10; i++)
		{
			if (TargetX >= WaterScrollPosition[2][i][0] - 12 && TargetY >= WaterScrollPosition[2][i][1] - 12
				&& TargetX <= WaterScrollPosition[2][i][0] + 12 && TargetY <= WaterScrollPosition[2][i][1] + 12 && i >= 9)
			{
				inside = 1;
				break;
			}

			if (TargetX >= WaterScrollPosition[2][i][0] - 8 && TargetY >= WaterScrollPosition[2][i][1] - 8
				&& TargetX <= WaterScrollPosition[2][i][0] + 8 && TargetY <= WaterScrollPosition[2][i][1] + 8)
			{
				inside = 1;
				break;
			}
		}

		if (((TargetX / 4) != 491 || (TargetY / 4) != 443 || Vol == 169) && inside == 0)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Cant_Use_That_Here]);
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		if (pMob[conn].Leader != -1 && pMob[conn].Leader)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Party_Leader_Only]);
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		char UserName[128];

		int Sala = Vol - 161;
		int UserArea = Sala <= 7 ? GetUserInArea(WaterScrollPosition[2][Vol - 161][0] - 8, WaterScrollPosition[2][Vol - 161][1] - 8, WaterScrollPosition[2][Vol - 161][0] + 8, WaterScrollPosition[2][Vol - 161][1] + 8, UserName) : GetUserInArea(WaterScrollPosition[2][Vol - 161][0] - 12, WaterScrollPosition[2][Vol - 161][1] - 12, WaterScrollPosition[2][Vol - 161][0] + 12, WaterScrollPosition[2][Vol - 161][1] + 12, UserName);

		if (UserArea >= 1)
		{
			sprintf(temp, g_pMessageStringTable[_NN_Someone_is_on_quest], UserName, UserArea - 1);

			SendClientMessage(conn, temp);
			SendItem(conn, m->SourType, m->SourPos, item);

			return;
		}

		WaterClear1[2][Vol - 161] = Sala <= 7 ? 30 : 15;

		DoTeleport(conn, WaterScrollPosition[2][Vol - 161][0], WaterScrollPosition[2][Vol - 161][1]);
		SendClientSignalParm(conn, ESCENE_FIELD, _MSG_StartTime, WaterClear1[2][Vol - 161] * 2);

		for (int i = 0; i < MAX_PARTY; i++)
		{
			int partyconn = pMob[conn].PartyList[i];

			if (partyconn > 0 && partyconn < MAX_USER && partyconn != conn && pUser[partyconn].Mode == USER_PLAY)
			{
				DoTeleport(partyconn, WaterScrollPosition[2][Vol - 161][0], WaterScrollPosition[2][Vol - 161][1]);
				SendClientSignalParm(partyconn, ESCENE_FIELD, _MSG_StartTime, WaterClear1[2][Vol - 161] * 2);
			}
		}

		if (Sala <= 7)
		{
			GenerateMob(Sala + WATER_A_INITIAL, 0, 0);
			GenerateMob(Sala + WATER_A_INITIAL, 0, 0);
			//RebuildGenerator();
		}
		else if (Sala == 9)
		{
			int _rand = rand() % 10;

			if (_rand < 4)
				GenerateMob(WATER_A_INITIAL + 8, 0, 0);

			else if (_rand < 5)
				GenerateMob(WATER_A_INITIAL + 9, 0, 0);

			else if (_rand < 6)
				GenerateMob(WATER_A_INITIAL + 10, 0, 0);

			else
				GenerateMob(WATER_A_INITIAL + 11, 0, 0);
		}
		sprintf(temp, "useitem,water scroll A %d", Sala);
		ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

		if (amount > 1)
			BASE_SetItemAmount(item, amount - 1);

		else
			memset(item, 0, sizeof(STRUCT_ITEM));

		return;
	}
#pragma endregion
#pragma region Divina 7 15 30 Dias
	if (Vol >= 64 && Vol <= 66)
	{
		int sAffect = GetEmptyAffect(conn, 34);

		if (sAffect == -1 || pMob[conn].Affect[sAffect].Type == 34)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_CantEatMore]);
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		time(&pMob[conn].extra.DivineEnd);

		if (Vol == 64)//7DD
			pMob[conn].extra.DivineEnd += 60 * 60 * 24 * 8;

		else  if (Vol == 65)//15D
			pMob[conn].extra.DivineEnd += 60 * 60 * 24 * 16;

		else if (Vol == 66)//30D
			pMob[conn].extra.DivineEnd += 60 * 60 * 24 * 31;


		pMob[conn].Affect[sAffect].Type = 34;
		pMob[conn].Affect[sAffect].Level = 1;
		pMob[conn].Affect[sAffect].Value = 0;
		pMob[conn].Affect[sAffect].Time = 2000000000;

		BASE_GetHpMp(&pMob[conn].MOB, &pMob[conn].extra);
		pMob[conn].GetCurrentScore(conn);

		SendScore(conn);

		sprintf(temp, "useitem,item divine type: %d", Vol-64);
		ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

		if (amount > 1)
			BASE_SetItemAmount(item, amount - 1);

		else
			memset(item, 0, sizeof(STRUCT_ITEM));

		return;
	}
#pragma endregion
#pragma region Vigor
	if (Vol == 58)
	{
		int sAffect = GetEmptyAffect(conn, 35);

		if (sAffect == -1)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_CantEatMore]);
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}
		if (item->sIndex == 3313) // Vigor 1 hora
		{
			pMob[conn].Affect[sAffect].Type = 35;
			pMob[conn].Affect[sAffect].Level = 1;
			pMob[conn].Affect[sAffect].Value = 0;
			pMob[conn].Affect[sAffect].Time = AFFECT_1H;

			BASE_GetHpMp(&pMob[conn].MOB, &pMob[conn].extra);
			pMob[conn].GetCurrentScore(conn);

			SendScore(conn);

			if (amount > 1)
				BASE_SetItemAmount(item, amount - 1);

			else
				memset(item, 0, sizeof(STRUCT_ITEM));

			return;
		}
		else if (item->sIndex == 3364) // 7 Dias
		{
			pMob[conn].Affect[sAffect].Type = 35;
			pMob[conn].Affect[sAffect].Level = 1;
			pMob[conn].Affect[sAffect].Value = 0;
			pMob[conn].Affect[sAffect].Time = AFFECT_1D * 7;

			BASE_GetHpMp(&pMob[conn].MOB, &pMob[conn].extra);
			pMob[conn].GetCurrentScore(conn);

			SendScore(conn);


			if (amount > 1)
				BASE_SetItemAmount(item, amount - 1);

			else
				memset(item, 0, sizeof(STRUCT_ITEM));

			return;
		}
		else if (item->sIndex == 3365) // 15 Dias
		{
			pMob[conn].Affect[sAffect].Type = 35;
			pMob[conn].Affect[sAffect].Level = 1;
			pMob[conn].Affect[sAffect].Value = 0;
			pMob[conn].Affect[sAffect].Time = AFFECT_1D * 15;

			BASE_GetHpMp(&pMob[conn].MOB, &pMob[conn].extra);
			pMob[conn].GetCurrentScore(conn);

			SendScore(conn);


			if (amount > 1)
				BASE_SetItemAmount(item, amount - 1);

			else
				memset(item, 0, sizeof(STRUCT_ITEM));

			return;
		}
		else if (item->sIndex == 3366) // 30 Dias
		{
			pMob[conn].Affect[sAffect].Type = 35;
			pMob[conn].Affect[sAffect].Level = 1;
			pMob[conn].Affect[sAffect].Value = 0;
			pMob[conn].Affect[sAffect].Time = AFFECT_1D * 30;

			BASE_GetHpMp(&pMob[conn].MOB, &pMob[conn].extra);
			pMob[conn].GetCurrentScore(conn);

			SendScore(conn);


			if (amount > 1)
				BASE_SetItemAmount(item, amount - 1);

			else
				memset(item, 0, sizeof(STRUCT_ITEM));

			return;
		}

		sprintf(temp, "useitem,item vigor");
		ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);
	}
#pragma endregion
#pragma region Baú de XP
	if (Vol == 198)
	{
		int sAffect = GetEmptyAffect(conn, 39);

		if (sAffect == -1)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_CantEatMore]);
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		pMob[conn].Affect[sAffect].Type = 39;
		pMob[conn].Affect[sAffect].Level = 0;
		pMob[conn].Affect[sAffect].Value = 0;
		pMob[conn].Affect[sAffect].Time += AFFECT_1H * 2;

		if (pMob[conn].Affect[sAffect].Time >= 324000)
			pMob[conn].Affect[sAffect].Time = 324000;

		pMob[conn].GetCurrentScore(conn);
		SendScore(conn);

		sprintf(temp, "useitem,item bauexp time %d", pMob[conn].Affect[sAffect].Time);
		ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

		if (amount > 1)
			BASE_SetItemAmount(item, amount - 1);

		else
			memset(item, 0, sizeof(STRUCT_ITEM));

		return;
	}
#pragma endregion
#pragma region Frango Assado
	if (Vol == 63 || Vol == 64)
	{
		int sAffect = GetEmptyAffect(conn, 30);

		if (sAffect == -1 || (pMob[conn].Affect[sAffect].Value != 2000 && pMob[conn].Affect[sAffect].Time == 30))
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_CantEatMore]);
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		pMob[conn].Affect[sAffect].Type = 30;
		pMob[conn].Affect[sAffect].Level = 2000;
		pMob[conn].Affect[sAffect].Value = 0;
		pMob[conn].Affect[sAffect].Time = AFFECT_1H * 4;

		if (pMob[conn].Affect[sAffect].Time >= 324000)
			pMob[conn].Affect[sAffect].Time = 324000;

		pMob[conn].GetCurrentScore(conn);
		SendScore(conn);

		sprintf(temp, "useitem,item frango time %d", pMob[conn].Affect[sAffect].Time);
		ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

		if (amount > 1)
			BASE_SetItemAmount(item, amount - 1);

		else
			memset(item, 0, sizeof(STRUCT_ITEM));

		return;
	}
#pragma endregion
#pragma region Itens de quests
	if (Vol == 191)
	{
		/*if (pMob[conn].extra.ClassMaster != MORTAL && pMob[conn].extra.ClassMaster != ARCH)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Level_Limit2]);
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}*/
		int thisQuest = item->sIndex - 4117;

		int minlevel = pMob[conn].extra.ClassMaster == MORTAL ? CReadFiles::QuestLevel[thisQuest][0] : CReadFiles::QuestLevel[thisQuest][2];
		int maxlevel = pMob[conn].extra.ClassMaster == MORTAL ? CReadFiles::QuestLevel[thisQuest][1] : CReadFiles::QuestLevel[thisQuest][3];

		if (pMob[conn].MOB.CurrentScore.Level < minlevel || pMob[conn].MOB.CurrentScore.Level >= maxlevel)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Level_limit]);
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		struct tm when;
		time_t now;
		time(&now);
		when = *localtime(&now);

		int QuestExp = pMob[conn].extra.ClassMaster == MORTAL ? CReadFiles::QuestExp[thisQuest][0] : CReadFiles::QuestExp[thisQuest][1];
		int QuestExpParty = QuestExp / 10;

		pMob[conn].MOB.Coin += CReadFiles::QuestCoin[thisQuest];
		pMob[conn].MOB.Exp += QuestExp;

#pragma region Log de Experiência diário
		if (when.tm_yday != pMob[conn].extra.DayLog.YearDay)
			pMob[conn].extra.DayLog.Exp = 0;

		pMob[conn].extra.DayLog.YearDay = when.tm_yday;
		pMob[conn].extra.DayLog.Exp += QuestExp;
#pragma endregion

		if(pMob[conn].MOB.Coin > 2000000000)
			pMob[conn].MOB.Coin = 2000000000;

		sprintf(temp, g_pMessageStringTable[_NS_GETEXP], pMob[conn].extra.ClassMaster == MORTAL ? CReadFiles::QuestExp[thisQuest][0] : CReadFiles::QuestExp[thisQuest][1]);
		SendClientMessage(conn, temp);

		SendEmotion(conn, 14, 3);

		if (pMob[conn].CheckGetLevel() == 4)
		{
			SetCircletSubGod(conn);
			SendClientMessage(conn, g_pMessageStringTable[_NN_Level_Up]);
			SendScore(conn);

			if(pMob[conn].extra.ClassMaster == MORTAL)
				DoItemLevel(conn);
		}

		SendEtc(conn);

		int partyleader = pMob[conn].Leader;

		if (partyleader <= 0)
			partyleader = conn;

		if (partyleader >= MAX_USER)
			partyleader = conn;

		//if(partyleader != conn && (pMob[partyleader].extra.ClassMaster == MORTAL || pMob[partyleader].extra.ClassMaster == ARCH)
		//	&& pMob[partyleader].MOB.CurrentScore.Level < 350 && pUser[partyleader].OnlyTrade == 0)
		{
			pMob[partyleader].MOB.Exp += QuestExpParty;

			sprintf(temp, g_pMessageStringTable[_NS_GETEXP], QuestExpParty);
			SendClientMessage(partyleader, temp);

#pragma region Log de Experiência diário
			if (when.tm_yday != pMob[partyleader].extra.DayLog.YearDay)
				pMob[partyleader].extra.DayLog.Exp = 0;

			pMob[partyleader].extra.DayLog.YearDay = when.tm_yday;
			pMob[partyleader].extra.DayLog.Exp += QuestExpParty;
#pragma endregion

			SendEmotion(partyleader, 14, 3);

			if (pMob[partyleader].CheckGetLevel() == 4)
			{
				SetCircletSubGod(partyleader);
				SendClientMessage(partyleader, g_pMessageStringTable[_NN_Level_Up]);
				SendScore(partyleader);

				if(pMob[partyleader].extra.ClassMaster == MORTAL)
				DoItemLevel(partyleader);
			}
			SendEtc(partyleader);
		}

		for (int i = 0; i < MAX_PARTY; i++)
		{
			int partymember = pMob[partyleader].PartyList[i];

			if (partymember <= 0 || partymember >= MAX_USER)
				continue;

			if (pUser[partymember].Mode != USER_PLAY || partymember == conn)
				continue;

			//if(partymember != conn && (pMob[partymember].extra.ClassMaster == MORTAL || pMob[partymember].extra.ClassMaster == ARCH)
			//	&& pMob[partymember].MOB.CurrentScore.Level < 350 && pUser[partymember].OnlyTrade == 0)
			{
				pMob[partymember].MOB.Exp += QuestExpParty;

#pragma region Log de Experiência diário
				if (when.tm_yday != pMob[partymember].extra.DayLog.YearDay)
					pMob[partymember].extra.DayLog.Exp = 0;

				pMob[partymember].extra.DayLog.YearDay = when.tm_yday;
				pMob[partymember].extra.DayLog.Exp += QuestExpParty;
#pragma endregion

				sprintf(temp, g_pMessageStringTable[_NS_GETEXP], QuestExpParty);
				SendClientMessage(partymember, temp);

				SendEmotion(partymember, 14, 3);

				if (pMob[partymember].CheckGetLevel() == 4)
				{
					SetCircletSubGod(partymember);
					SendClientMessage(partymember, g_pMessageStringTable[_NN_Level_Up]);
					SendScore(partymember);

					if(pMob[partymember].extra.ClassMaster == MORTAL)
						DoItemLevel(partymember);
				}

				SendEtc(partymember);
			}
		}
		sprintf(temp, "useitem,%s level:%d classmaster:%d", g_pItemList[item->sIndex].Name, pMob[conn].MOB.BaseScore.Level, pMob[conn].extra.ClassMaster);
		ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

		if (amount > 1)
			BASE_SetItemAmount(item, amount - 1);

		else
			memset(item, 0, sizeof(STRUCT_ITEM));
		return;
	}
#pragma endregion
#pragma region Molar do Gargula
	if (Vol == 194)
	{
		if (pMob[conn].extra.ClassMaster != MORTAL)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Level_Limit2]);
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		if (pMob[conn].MOB.CurrentScore.Level < 199 || pMob[conn].MOB.CurrentScore.Level >= 254)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Level_limit]);
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		if (pMob[conn].extra.QuestInfo.Mortal.MolarGargula)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Youve_Done_It_Already]);
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		for (int i = 1; i < 6; i++)
		{
			if (pMob[conn].MOB.Equip[i].sIndex == 0)
				continue;

			int sanc = BASE_GetItemSanc(&pMob[conn].MOB.Equip[i]);

			if (sanc < 6)
			{
				BASE_SetItemSanc(&pMob[conn].MOB.Equip[i], 6, 0);
				SendItem(conn, ITEM_PLACE_EQUIP, i, &pMob[conn].MOB.Equip[i]);
			}
		}
		pMob[conn].extra.QuestInfo.Mortal.MolarGargula = 1;
		pMob[conn].GetCurrentScore(conn);

		sprintf(temp, g_pMessageStringTable[_DN_Play_Quest], "Molar do Gargula");
		SendClientMessage(conn, temp);

		sprintf(temp, "useitem,itemquest molar %d", pMob[conn].extra.QuestInfo.Mortal.MolarGargula);
		ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

		if (amount > 1)
			BASE_SetItemAmount(item, amount - 1);

		else
			memset(item, 0, sizeof(STRUCT_ITEM));

		return;
	}
#pragma endregion
#pragma region Pesadelo N
	if (Vol == 173)
	{
		int TargetX = pMob[conn].TargetX;
		int TargetY = pMob[conn].TargetY;

		if ((TargetX / 128) != 19 || (TargetY / 128) != 15)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Cant_Use_That_Here]);
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		if (pMob[conn].Leader != -1 && pMob[conn].Leader)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Party_Leader_Only]);
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		if (pMob[conn].extra.ClassMaster == ARCH || pMob[conn].extra.ClassMaster == CELESTIAL || pMob[conn].extra.ClassMaster == CELESTIALCS || pMob[conn].extra.ClassMaster == SCELESTIAL)
		{
			SendItem(conn, m->SourType, m->SourPos, item);
			SendClientMessage(conn, "Entrada permitida somente à Mortais");
			return;
		}

		struct tm when;
		time_t now;
		time(&now);
		when = *localtime(&now);

		if (when.tm_min >= 4 && when.tm_min <= 19 || when.tm_min >= 24 && when.tm_min <= 39
			|| when.tm_min >= 44 && when.tm_min <= 59)
		{
			SendItem(conn, m->SourType, m->SourPos, item);
			SendClientMessage(conn, "Horário não permitido.");
			return;
		}
		
		int isParty = item->sIndex == 3324 ? 1 : 0;

		if (isParty == 0)
			goto lbl_n_noparty;

		if (PartyPesa[0] >= maxNightmare)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Night_Limited]);
			SendItem(conn, m->SourType, m->SourPos, item);

			return;
		}
		PartyPesa[0]++;

lbl_n_noparty:
		int NigthTime = 240;

		if (when.tm_min >= 20 && when.tm_min <= 23)
			NigthTime -= ((when.tm_min-20) * 60) + when.tm_sec;

		else if (when.tm_min >= 40 && when.tm_min <= 43)
			NigthTime -= ((when.tm_min-40) * 60) + when.tm_sec;

		else if (when.tm_min >= 0 && when.tm_min <= 3)
			NigthTime -= (when.tm_min * 60) + when.tm_sec;


		DoTeleport(conn, PesaNPosStandard[0][0], PesaNPosStandard[0][1]);
		SendClientSignalParm(conn, ESCENE_FIELD, _MSG_StartTime, NigthTime);

		for (int i = 0; i < MAX_PARTY && isParty == 1; i++)
		{
			int partyconn = pMob[conn].PartyList[i];

			if(pMob[partyconn].extra.ClassMaster == ARCH || pMob[partyconn].extra.ClassMaster == CELESTIAL || pMob[partyconn].extra.ClassMaster == CELESTIALCS || pMob[partyconn].extra.ClassMaster == SCELESTIAL)
				continue;

			if (partyconn > 0 && partyconn < MAX_USER && partyconn != conn && pUser[partyconn].Mode == USER_PLAY)
			{
				DoTeleport(partyconn, PesaNPosStandard[i][0], PesaNPosStandard[i][1]);
				SendClientSignalParm(partyconn, ESCENE_FIELD, _MSG_StartTime, NigthTime);
			}
		}

		sprintf(temp, "useitem,nightmare N ticket used party:%d", isParty);
		ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

		if (amount > 1)
			BASE_SetItemAmount(item, amount - 1);

		else
			memset(item, 0, sizeof(STRUCT_ITEM));

		return;
	}
#pragma endregion
#pragma region Pesadelo M
	if (Vol == 174)
	{
		int TargetX = pMob[conn].TargetX;
		int TargetY = pMob[conn].TargetY;

		if ((TargetX / 128) != 16 || (TargetY / 128) != 16)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Cant_Use_That_Here]);
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		if (pMob[conn].Leader != -1 && pMob[conn].Leader)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Party_Leader_Only]);
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		if (pMob[conn].extra.ClassMaster == MORTAL || pMob[conn].extra.ClassMaster == CELESTIAL || pMob[conn].extra.ClassMaster == CELESTIALCS || pMob[conn].extra.ClassMaster == SCELESTIAL)
		{
			SendItem(conn, m->SourType, m->SourPos, item);
			SendClientMessage(conn, "Entrada permitida somente à Archs");

			return;
		}

		struct tm when;
		time_t now;
		time(&now);
		when = *localtime(&now);

		if (when.tm_min >= 0 && when.tm_min <= 4 || when.tm_min >= 9 && when.tm_min <= 24
			|| when.tm_min >= 29 && when.tm_min <= 44 || when.tm_min >= 49 && when.tm_min <= 59)
		{
			SendItem(conn, m->SourType, m->SourPos, item);
			SendClientMessage(conn, "Horário não permitido.");
			return;
		}

		int isParty = item->sIndex == 3325 ? 1 : 0;

		if (isParty == 0)
			goto lbl_m_noparty;

		if (PartyPesa[1] >= maxNightmare)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Night_Limited]);
			SendItem(conn, m->SourType, m->SourPos, item);

			return;
		}
		PartyPesa[1]++;

lbl_m_noparty:
		int NigthTime = 240;

		if (when.tm_min >= 5 && when.tm_min <= 8)
			NigthTime -= ((when.tm_min - 5) * 60) + when.tm_sec;

		else if (when.tm_min >= 25 && when.tm_min <= 28)
			NigthTime -= ((when.tm_min - 25) * 60) + when.tm_sec;

		else if (when.tm_min >= 45 && when.tm_min <= 48)
			NigthTime -= ((when.tm_min - 45) * 60) + when.tm_sec;

		if((pMob[conn].MOB.SPX/128) == 8 && (pMob[conn].MOB.SPY/128) == 2)
			DoTeleport(conn, pMob[conn].MOB.SPX, pMob[conn].MOB.SPY);
		else
			DoTeleport(conn, PesaMPosStandard[0][0], PesaMPosStandard[0][1]);
							 
		SendClientSignalParm(conn, ESCENE_FIELD, _MSG_StartTime, NigthTime);

		for (int i = 0; i < MAX_PARTY && isParty == 1; i++)
		{
			int partyconn = pMob[conn].PartyList[i];

			if(pMob[partyconn].extra.ClassMaster == MORTAL || pMob[partyconn].extra.ClassMaster == CELESTIAL || pMob[partyconn].extra.ClassMaster == CELESTIALCS || pMob[partyconn].extra.ClassMaster == SCELESTIAL)
				continue;

			if (partyconn > 0 && partyconn < MAX_USER && partyconn != conn && pUser[partyconn].Mode == USER_PLAY)
			{
				if((pMob[partyconn].MOB.SPX/128) == 8 && (pMob[partyconn].MOB.SPY/128) == 2)
					DoTeleport(partyconn, pMob[partyconn].MOB.SPX, pMob[partyconn].MOB.SPY);
				else
					DoTeleport(partyconn, PesaMPosStandard[i][0], PesaMPosStandard[i][1]);

				SendClientSignalParm(partyconn, ESCENE_FIELD, _MSG_StartTime, NigthTime);
			}
		}

		sprintf(temp, "useitem,nightmare M ticket used party:%d", isParty);
		ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

		if (amount > 1)
			BASE_SetItemAmount(item, amount - 1);

		else
			memset(item, 0, sizeof(STRUCT_ITEM));

		return;
	}
#pragma endregion
#pragma region Pesadelo A
	if (Vol == 175)
	{
		int TargetX = pMob[conn].TargetX;
		int TargetY = pMob[conn].TargetY;


		if ((TargetX / 128) != 19 || (TargetY / 128) != 13)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Cant_Use_That_Here]);
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		if (pMob[conn].Leader != -1 && pMob[conn].Leader)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Party_Leader_Only]);
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		if (pMob[conn].extra.ClassMaster == MORTAL || pMob[conn].extra.ClassMaster == ARCH)
		{
			SendItem(conn, m->SourType, m->SourPos, item);
			SendClientMessage(conn, "Entrada permitida somente à Celestiais");
			return;
		}

		struct tm when;
		time_t now;
		time(&now);
		when = *localtime(&now);

		if (when.tm_min >= 0 && when.tm_min <= 9 || when.tm_min >= 14 && when.tm_min <= 29
			|| when.tm_min >= 34 && when.tm_min <= 49 || when.tm_min >= 54 && when.tm_min <= 60)
		{
			SendItem(conn, m->SourType, m->SourPos, item);
			SendClientMessage(conn, "Horário não permitido.");
			return;
		}

		if (pMob[conn].extra.ClassMaster == MORTAL || pMob[conn].extra.ClassMaster == ARCH || (pMob[conn].extra.ClassMaster == CELESTIAL || pMob[conn].extra.ClassMaster == SCELESTIAL || pMob[conn].extra.ClassMaster == CELESTIALCS) && pMob[conn].extra.NT <= 0)
		{
			SendItem(conn, m->SourType, m->SourPos, item);
			SendClientMessage(conn, "Entrada não permitida. Cheque sua quantidade de entradas com o comando /nt");
			return;
		}

		if (PartyPesa[2] >= maxNightmare)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Night_Limited]);
			SendItem(conn, m->SourType, m->SourPos, item);

			return;
		}

		PartyPesa[2]++;

		int isParty = item->sIndex == 3326 ? 1 : 0;

		if (isParty == 0)
			goto lbl_a_noparty;

lbl_a_noparty:
		int NigthTime = 240;

		if (when.tm_min >= 10 && when.tm_min <= 13)
			NigthTime -= ((when.tm_min - 10) * 60) + when.tm_sec;

		else if (when.tm_min >= 30 && when.tm_min <= 33)
			NigthTime -= ((when.tm_min - 30) * 60) + when.tm_sec;

		else if (when.tm_min >= 50 && when.tm_min <= 53)
			NigthTime -= ((when.tm_min - 50) * 60) + when.tm_sec;


		if ((pMob[conn].extra.ClassMaster == CELESTIAL || pMob[conn].extra.ClassMaster == SCELESTIAL || pMob[conn].extra.ClassMaster == CELESTIALCS))
			pMob[conn].extra.NT--;

		if((pMob[conn].MOB.SPX/128) == 9 && (pMob[conn].MOB.SPY/128) == 1)
			DoTeleport(conn, pMob[conn].MOB.SPX+rand()%1, pMob[conn].MOB.SPY+rand()%1);

		else
			DoTeleport(conn, PesaAPosStandard[0][0]+rand()%1, PesaAPosStandard[0][1]+rand()%1);

		SendClientSignalParm(conn, ESCENE_FIELD, _MSG_StartTime, NigthTime);

		for (int i = 0; i < MAX_PARTY && isParty == 1; i++)
		{
			int partyconn = pMob[conn].PartyList[i];

			if(pMob[partyconn].extra.ClassMaster == MORTAL || pMob[partyconn].extra.ClassMaster == ARCH) //  || (pMob[partyconn].extra.ClassMaster == CELESTIAL || pMob[partyconn].extra.ClassMaster == CELESTIALCS || pMob[partyconn].extra.ClassMaster == SCELESTIAL) && pMob[partyconn].MOB.CurrentScore.Level >= 180
				continue;

			if (partyconn > 0 && partyconn < MAX_USER && partyconn != conn && pUser[partyconn].Mode == USER_PLAY)
			{
				if ((pMob[partyconn].extra.ClassMaster == CELESTIAL || pMob[partyconn].extra.ClassMaster == SCELESTIAL || pMob[partyconn].extra.ClassMaster == CELESTIALCS))
				{
					if (pMob[partyconn].extra.NT <= 0)
						continue;

					pMob[partyconn].extra.NT--;
				}

				if((pMob[partyconn].MOB.SPX/128) == 9 && (pMob[partyconn].MOB.SPY/128) == 1)
					DoTeleport(partyconn, pMob[partyconn].MOB.SPX+rand()%1, pMob[partyconn].MOB.SPY+rand()%1);
				else
				DoTeleport(partyconn, PesaAPosStandard[i][0]+rand()%1, PesaAPosStandard[i][1]+rand()%1);

				SendClientSignalParm(partyconn, ESCENE_FIELD, _MSG_StartTime, NigthTime);
			}
		}
		sprintf(temp, "useitem,nightmare A ticket used party:%d", isParty);
		ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

		if (amount > 1)
			BASE_SetItemAmount(item, amount - 1);

		else
			memset(item, 0, sizeof(STRUCT_ITEM));

		return;
	}
#pragma endregion
#pragma region Carta de Duelo N.M.A
	if (Vol == 20)
	{
		int TargetX = pMob[conn].TargetX;
		int TargetY = pMob[conn].TargetY;


		if ((TargetX / 4) != 261 || (TargetY / 4) != 422)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Only_at_tor_alter]);
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		if (pMob[conn].Leader != -1 && pMob[conn].Leader)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Party_Leader_Only]);
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		char UserName[128];

		int UserArea = GetUserInArea(774, 3593, 892, 3702, UserName);

		if (UserArea >= 1)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Someone_is_on_quest]);
			SendItem(conn, m->SourType, m->SourPos, item);

			return;
		}

		CartaTime = 60;
		CartaSala = 1;

		DoTeleport(conn, CartaPos[0][0], CartaPos[0][1]);
		SendClientSignalParm(conn, ESCENE_FIELD, _MSG_StartTime, CartaTime * 2);

		for (int i = 0; i < MAX_PARTY; i++)
		{
			int partyconn = pMob[conn].PartyList[i];

			if (partyconn > 0 && partyconn < MAX_USER && partyconn != conn && pUser[partyconn].Mode == USER_PLAY)
			{
				DoTeleport(partyconn, CartaPos[0][0], CartaPos[0][1]);
				SendClientSignalParm(partyconn, ESCENE_FIELD, _MSG_StartTime, CartaTime * 2);
			}
		}

		for (int xx = 767; xx < 897; xx++)
		{
			for (int yy = 3582; yy < 3712; yy++)
			{
				int tmob = pMobGrid[yy][xx];

				if (tmob < MAX_USER)
					continue;

				DeleteMob(tmob, 3);
			}
		}

		int cartaindex = item->sIndex - 3171;

		if (cartaindex == 1)
		{//Carta N
			for (int i = SECRET_ROOM_N_INITIAL; i <= SECRET_ROOM_N_END; i++)
			{
				GenerateMob(i, 0, 0);
				GenerateMob(i, 0, 0);
			}

			GenerateMob(SECRET_ROOM_N_BOSS_INITIAL + rand() % 1, 0, 0);
		}

		else if (cartaindex == 0)
		{//Carta M
			for (int i = SECRET_ROOM_M_INITIAL; i <= SECRET_ROOM_M_END; i++)
			{
				GenerateMob(i, 0, 0);
				GenerateMob(i, 0, 0);
			}

			GenerateMob(SECRET_ROOM_M_BOSS_INITIAL + rand() % 1, 0, 0);
		}
		else
		{//Carta A
			for (int i = SECRET_ROOM_A_INITIAL; i <= SECRET_ROOM_A_END; i++)
			{
				GenerateMob(i, 0, 0);
				GenerateMob(i, 0, 0);
			}

			GenerateMob(SECRET_ROOM_A_BOSS_INITIAL + rand() % 1, 0, 0);
		}
		if (amount > 1)
			BASE_SetItemAmount(item, amount - 1);

		else
			memset(item, 0, sizeof(STRUCT_ITEM));

		sprintf(temp, "useitem,secretroom ticket used itemindex:%d", item->sIndex);
		ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

		return;
	}
#pragma endregion
#pragma region Pedidos de caça
	if (Vol == 195)
	{
		if (item->sIndex < 3432 || item->sIndex > 3437)
			return;

		if (m->WarpID <= 0 || m->WarpID > 10)
			return;

		DoTeleport(conn, HuntingScrolls[item->sIndex - 3432][m->WarpID-1][0], HuntingScrolls[item->sIndex - 3432][m->WarpID-1][1]);

		if (amount > 1)
			BASE_SetItemAmount(item, amount - 1);

		else
			memset(item, 0, sizeof(STRUCT_ITEM));

		sprintf(temp, "useitem,huntingscrolls itemindex:%d warpid:%d tx:%d ty:%d", item->sIndex, m->WarpID, HuntingScrolls[item->sIndex - 3432][m->WarpID-1][0], HuntingScrolls[item->sIndex - 3432][m->WarpID-1][1]);
		ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);
		return;
	}
#pragma endregion
#pragma region Pedra Ideal
	if (Vol == 211)
	{
		if (pMob[conn].extra.ClassMaster != ARCH && pMob[conn].extra.ClassMaster != CELESTIAL)
		{
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		if (pMob[conn].extra.ClassMaster == ARCH && pMob[conn].MOB.CurrentScore.Level < 355)
		{
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		if (pMob[conn].extra.ClassMaster == ARCH && (pMob[conn].MOB.Equip[1].sIndex != 0 || pMob[conn].extra.QuestInfo.Arch.MortalLevel < 99))
		{
			if(pMob[conn].MOB.Equip[1].sIndex)
				SendClientMessage(conn, g_pMessageStringTable[_NN_Cant_with_armor]);

			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		if (pMob[conn].extra.ClassMaster == CELESTIAL && (pMob[conn].MOB.Equip[11].sIndex < 1760 || pMob[conn].MOB.Equip[11].sIndex > 1763))
		{
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		if (pMob[conn].extra.ClassMaster == CELESTIAL && pMob[conn].MOB.CurrentScore.Level < 120)
		{
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		if (pMob[conn].extra.ClassMaster == CELESTIAL)
		{
			pMob[conn].extra.ClassMaster = CELESTIALCS;

			pMob[conn].extra.SaveCelestial[1].Class = pMob[conn].MOB.Equip[11].sIndex - 1760;

			pMob[conn].extra.SaveCelestial[1].BaseScore = pMob[conn].MOB.BaseScore;

			pMob[conn].extra.SaveCelestial[1].LearnedSkill = pMob[conn].MOB.LearnedSkill;

			pMob[conn].extra.SaveCelestial[1].BaseScore.Level = 0;

			pMob[conn].extra.SaveCelestial[1].BaseScore.Str = BaseSIDCHM[pMob[conn].MOB.Class][0];
			pMob[conn].extra.SaveCelestial[1].BaseScore.Int = BaseSIDCHM[pMob[conn].MOB.Class][1];
			pMob[conn].extra.SaveCelestial[1].BaseScore.Dex = BaseSIDCHM[pMob[conn].MOB.Class][2];
			pMob[conn].extra.SaveCelestial[1].BaseScore.Con = BaseSIDCHM[pMob[conn].MOB.Class][3];

			pMob[conn].extra.SaveCelestial[1].BaseScore.Special[0] = 0;
			pMob[conn].extra.SaveCelestial[1].BaseScore.Special[1] = 0;
			pMob[conn].extra.SaveCelestial[1].BaseScore.Special[2] = 0;
			pMob[conn].extra.SaveCelestial[1].BaseScore.Special[3] = 0;

			pMob[conn].extra.SaveCelestial[1].BaseScore.Ac = 230;
			pMob[conn].extra.SaveCelestial[1].BaseScore.Damage = 0;
			pMob[conn].extra.SaveCelestial[1].BaseScore.Hp = BaseSIDCHM[pMob[conn].MOB.Equip[11].sIndex - 1760][4];
			pMob[conn].extra.SaveCelestial[1].BaseScore.MaxHp = BaseSIDCHM[pMob[conn].MOB.Equip[11].sIndex - 1760][4];
			pMob[conn].extra.SaveCelestial[1].BaseScore.Mp = BaseSIDCHM[pMob[conn].MOB.Equip[11].sIndex - 1760][5];
			pMob[conn].extra.SaveCelestial[1].BaseScore.MaxMp = BaseSIDCHM[pMob[conn].MOB.Equip[11].sIndex - 1760][5];

			pMob[conn].extra.SaveCelestial[1].SpecialBonus = 855;

			pMob[conn].extra.SaveCelestial[1].LearnedSkill &= 0xFF000000;

			pMob[conn].extra.SaveCelestial[1].Exp = 0;

			pMob[conn].extra.SaveCelestial[1].Soul = pMob[conn].extra.Soul;

			memset(&pMob[conn].MOB.Equip[11], 0, sizeof(STRUCT_ITEM));

			SendItem(conn, ITEM_PLACE_EQUIP, 11, &pMob[conn].MOB.Equip[11]);

			if (amount > 1)
				BASE_SetItemAmount(item, amount - 1);

			else
				memset(item, 0, sizeof(STRUCT_ITEM));

			sprintf(temp, "useitem,ideal stone used to make god2");
			ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

			SendClientMessage(conn, g_pMessageStringTable[_NN_My_King_Bless1]);
			return;
		}

		if (pMob[conn].extra.ClassMaster == ARCH)
		{
			int ptlevel = 0;

			if (pMob[conn].MOB.CurrentScore.Level < 370)
				ptlevel = 1;
								 
			else if (pMob[conn].MOB.CurrentScore.Level < 380)
				ptlevel = 2;

			else if (pMob[conn].MOB.CurrentScore.Level < 398)
				ptlevel = 3;

			else if (pMob[conn].MOB.CurrentScore.Level == 398)
				ptlevel = 4;
			else
				ptlevel = 5;

			memset(&pMob[conn].MOB.Equip[1], 0, sizeof(STRUCT_ITEM));

			if (pMob[conn].MOB.CurrentScore.Level < 380)
				pMob[conn].MOB.Equip[1].sIndex = 3500;

			else if (pMob[conn].MOB.CurrentScore.Level < 399)
				pMob[conn].MOB.Equip[1].sIndex = 3501;

			else if (pMob[conn].MOB.CurrentScore.Level < 400)
				pMob[conn].MOB.Equip[1].sIndex = 3502;

			pMob[conn].extra.QuestInfo.Celestial.ArchLevel = ptlevel;
			pMob[conn].extra.ClassMaster = CELESTIAL;

			pMob[conn].MOB.BaseScore.Level = 0;

			pMob[conn].MOB.BaseScore.Str = BaseSIDCHM[pMob[conn].MOB.Class][0];
			pMob[conn].MOB.BaseScore.Int = BaseSIDCHM[pMob[conn].MOB.Class][1];
			pMob[conn].MOB.BaseScore.Dex = BaseSIDCHM[pMob[conn].MOB.Class][2];
			pMob[conn].MOB.BaseScore.Con = BaseSIDCHM[pMob[conn].MOB.Class][3];

			pMob[conn].MOB.BaseScore.Special[0] = 0;
			pMob[conn].MOB.BaseScore.Special[1] = 0;
			pMob[conn].MOB.BaseScore.Special[2] = 0;
			pMob[conn].MOB.BaseScore.Special[3] = 0;

			pMob[conn].MOB.BaseScore.Ac = 230;
			pMob[conn].MOB.BaseScore.Damage = 0;
			pMob[conn].MOB.BaseScore.Hp = BaseSIDCHM[pMob[conn].MOB.Class][4];
			pMob[conn].MOB.BaseScore.MaxHp = BaseSIDCHM[pMob[conn].MOB.Class][4];
			pMob[conn].MOB.BaseScore.Mp = BaseSIDCHM[pMob[conn].MOB.Class][5];
			pMob[conn].MOB.BaseScore.MaxMp = BaseSIDCHM[pMob[conn].MOB.Class][5];

			pMob[conn].MOB.SpecialBonus = 855;

			pMob[conn].MOB.LearnedSkill = 1073741824;

			pMob[conn].MOB.Exp = 0;

			memset(&pMob[conn].MOB.Equip[15], 0, sizeof(STRUCT_ITEM));

			if (pMob[conn].MOB.Clan == 7)
				pMob[conn].MOB.Equip[15].sIndex = 3197;

			else if (pMob[conn].MOB.Clan == 8)
				pMob[conn].MOB.Equip[15].sIndex = 3198;

			else
				pMob[conn].MOB.Equip[15].sIndex = 3199;

			pMob[conn].MOB.Equip[0].stEffect[1].cEffect = 98;
			pMob[conn].MOB.Equip[0].stEffect[1].cValue = 3;
			pMob[conn].MOB.Equip[0].stEffect[2].cEffect = 106;
			pMob[conn].MOB.Equip[0].stEffect[2].cValue = (unsigned char)pMob[conn].MOB.Equip[0].sIndex;

			SendClientMessage(conn, g_pMessageStringTable[_NN_My_King_Bless1]);
		}

		if (amount > 1)
			BASE_SetItemAmount(item, amount - 1);

		else
			memset(item, 0, sizeof(STRUCT_ITEM));

		sprintf(temp, "useitem,ideal stone used to make god %d %d", pMob[conn].extra.QuestInfo.Celestial.ArchLevel, pMob[conn].extra.QuestInfo.Arch.Cristal);
		ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

		CharLogOut(conn);

		SendClientSignalParm(conn, ESCENE_FIELD, _MSG_SendArchEffect, pUser[conn].Slot);

		return;
	}
#pragma endregion
#pragma region Pedra Misteriosa
	if (item->sIndex == 4148)
	{
		if (pMob[conn].extra.ClassMaster != CELESTIALCS && pMob[conn].extra.ClassMaster != SCELESTIAL)
		{
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		int Village = BASE_GetVillage(pMob[conn].TargetX, pMob[conn].TargetY);

		if (Village < 0 || Village >= 5)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_OnlyVillage]);
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		int cl = pMob[conn].extra.ClassMaster == CELESTIALCS ? 0 : 1;
		int ncl = pMob[conn].extra.ClassMaster == CELESTIALCS ? 1 : 0;

		if (pMob[conn].extra.ClassMaster == CELESTIALCS)
		{
			pMob[conn].extra.ClassMaster = SCELESTIAL;
			pMob[conn].extra.QuestInfo.Celestial.CelestialLevel = pMob[conn].MOB.BaseScore.Level;
		}
		else
		{
			pMob[conn].extra.ClassMaster = CELESTIALCS;
			pMob[conn].extra.QuestInfo.Celestial.SubCelestialLevel = pMob[conn].MOB.BaseScore.Level;
		}
		pMob[conn].extra.SaveCelestial[cl].BaseScore = pMob[conn].MOB.BaseScore;
		pMob[conn].extra.SaveCelestial[cl].Class = pMob[conn].MOB.Class;
		pMob[conn].extra.SaveCelestial[cl].Exp = pMob[conn].MOB.Exp;
		pMob[conn].extra.SaveCelestial[cl].LearnedSkill = pMob[conn].MOB.LearnedSkill;
		pMob[conn].extra.SaveCelestial[cl].ScoreBonus = pMob[conn].MOB.ScoreBonus;
		pMob[conn].extra.SaveCelestial[cl].SkillBonus = pMob[conn].MOB.SkillBonus;
		pMob[conn].extra.SaveCelestial[cl].SpecialBonus = pMob[conn].MOB.SpecialBonus;
		pMob[conn].extra.SaveCelestial[cl].SPX = pMob[conn].MOB.SPX;
		pMob[conn].extra.SaveCelestial[cl].SPY = pMob[conn].MOB.SPY;

		pMob[conn].extra.SaveCelestial[cl].Soul = pMob[conn].extra.Soul;

		memcpy(pMob[conn].extra.SaveCelestial[cl].SkillBar1, pMob[conn].MOB.SkillBar, 4);
		memcpy(pMob[conn].extra.SaveCelestial[cl].SkillBar2, pUser[conn].CharShortSkill, 16);

		pMob[conn].MOB.BaseScore = pMob[conn].extra.SaveCelestial[ncl].BaseScore;
		pMob[conn].MOB.Class = pMob[conn].extra.SaveCelestial[ncl].Class;
		pMob[conn].MOB.Exp = pMob[conn].extra.SaveCelestial[ncl].Exp;
		pMob[conn].MOB.LearnedSkill = pMob[conn].extra.SaveCelestial[ncl].LearnedSkill;
		pMob[conn].MOB.ScoreBonus = pMob[conn].extra.SaveCelestial[ncl].ScoreBonus;
		pMob[conn].MOB.SkillBonus =  pMob[conn].extra.SaveCelestial[ncl].SkillBonus;
		pMob[conn].MOB.SpecialBonus = pMob[conn].extra.SaveCelestial[ncl].SpecialBonus;
		pMob[conn].MOB.SPX = pMob[conn].extra.SaveCelestial[ncl].SPX;
		pMob[conn].MOB.SPY = pMob[conn].extra.SaveCelestial[ncl].SPY;
		pMob[conn].extra.Soul = pMob[conn].extra.SaveCelestial[ncl].Soul;

		memcpy(pMob[conn].MOB.SkillBar, pMob[conn].extra.SaveCelestial[ncl].SkillBar1, 4);
		memcpy(pUser[conn].CharShortSkill, pMob[conn].extra.SaveCelestial[ncl].SkillBar2, 16);

		for (int i = 0; i < MAX_AFFECT; i++)
		{
			if (pMob[conn].Affect[i].Type == 4 || pMob[conn].Affect[i].Type == 8 || pMob[conn].Affect[i].Type == 29 || pMob[conn].Affect[i].Type == 30 || pMob[conn].Affect[i].Type == 34 || pMob[conn].Affect[i].Type == 35 || pMob[conn].Affect[i].Type == 39)
				continue;

			memset(&pMob[conn].Affect[i], 0, sizeof(STRUCT_AFFECT));
		}

		pMob[conn].MOB.Equip[0].sIndex = pMob[conn].extra.MortalFace + 5 + pMob[conn].MOB.Class;
		SendItem(conn, ITEM_PLACE_EQUIP, 0, &pMob[conn].MOB.Equip[0]);

		MSG_SetShortSkill sm_sss;
		memset(&sm_sss, 0, sizeof(MSG_SetShortSkill));

		sm_sss.Size = sizeof(MSG_SetShortSkill);
		sm_sss.ID = ESCENE_FIELD;
		sm_sss.Type = _MSG_SetShortSkill;
							 
		memcpy(sm_sss.Skill1, pMob[conn].extra.SaveCelestial[ncl].SkillBar1, 4);
		memcpy(sm_sss.Skill2, pMob[conn].extra.SaveCelestial[ncl].SkillBar2, 16);

		pUser[conn].cSock.AddMessage((char*)&sm_sss, sizeof(MSG_SetShortSkill));

		BASE_GetBonusSkillPoint(&pMob[conn].MOB, &pMob[conn].extra);
		BASE_GetBonusScorePoint(&pMob[conn].MOB, &pMob[conn].extra);

		BASE_GetHpMp(&pMob[conn].MOB, &pMob[conn].extra);

		pMob[conn].GetCurrentScore(conn);

		GetGuild(conn);

		SendScore(conn);
		SendEtc(conn);
							 
		sprintf(temp, "useitem,mistery stone change god2god");
		ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

		if (amount > 1)
			BASE_SetItemAmount(item, amount - 1);

		else
			memset(item, 0, sizeof(STRUCT_ITEM));

		return;
	}
#pragma endregion
#pragma region Escritura do Pesadelo
	if (Vol == 212 && item->sIndex == 5137)
	{
		time_t mtime;

		time(&mtime);

		/*if (mtime < (pMob[conn].extra.LastNT + 72000))
		{
			int rtime = (int)((72000 - (mtime - pMob[conn].extra.LastNT)) / 60 / 60);
			int rmin = (int)((72000 - (mtime - pMob[conn].extra.LastNT)) / 60 - (rtime*60));

			sprintf(temp, g_pMessageStringTable[_DD_NightmareTime], rtime, rmin);
			SendClientMessage(conn, temp);
			
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}*/

		pMob[conn].extra.LastNT = mtime;
		pMob[conn].extra.NT += 13;

		sprintf(temp, "useitem,nightmare entrance ticket %d", pMob[conn].extra.NT);
		ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

		if (amount > 1)
			BASE_SetItemAmount(item, amount - 1);

		else
			memset(item, 0, sizeof(STRUCT_ITEM));

		return;
	}
#pragma endregion
#pragma region Selo do Guerreiro
	if (item->sIndex == 4146)
	{
		pUser[conn].UseItemTime = GetTickCount();

		if (pMob[conn].extra.Fame >= 2000000000)
			pMob[conn].extra.Fame = 2000000000;

		pMob[conn].extra.Fame += 10;

		SendEmotion(conn, 14, 3);

		if(pMob[conn].extra.ClassMaster == MORTAL && pMob[conn].MOB.CurrentScore.Level >= 354 && pMob[conn].MOB.Equip[15].sIndex != 3191 && pMob[conn].MOB.Equip[15].sIndex != 3192 && pMob[conn].MOB.Equip[15].sIndex != 3193 && pMob[conn].MOB.Equip[15].sIndex != 3194 && pMob[conn].MOB.Equip[15].sIndex != 3195 && pMob[conn].MOB.Equip[15].sIndex != 3196)
		{
			memset(&pMob[conn].MOB.Equip[15], 0, sizeof(STRUCT_ITEM));

			if(pMob[conn].MOB.Clan == 7)
				pMob[conn].MOB.Equip[15].sIndex = 3191;

			else if(pMob[conn].MOB.Clan == 8)
				pMob[conn].MOB.Equip[15].sIndex = 3192;

			else
				pMob[conn].MOB.Equip[15].sIndex = 3193;

			SendItem(conn, ITEM_PLACE_EQUIP, 15, &pMob[conn].MOB.Equip[15]);
		}

		sprintf(temp, "useitem,sealofwarrior %d", pMob[conn].extra.Fame);
		ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

		if (amount > 1)
			BASE_SetItemAmount(item, amount - 1);

		else
			memset(item, 0, sizeof(STRUCT_ITEM));

		return;
	}
#pragma endregion
#pragma region Cristal - Elime - Sylphed - Thelion - Noas
	if (Vol == 187)
	{
		/*if (pMob[conn].extra.ClassMaster != ARCH) // if (pMob[conn].extra.ClassMaster != ARCH)
		{
			SendItem(conn, m->SourType, m->SourPos, item);
			SendClientMessage(conn, "Quest permitida somente à Archs.");
			return;
		}

		if (pMob[conn].MOB.CurrentScore.Level < 355)
		{
			sprintf(temp, g_pMessageStringTable[_DN_Level_Limit], 356);
			SendClientMessage(conn, temp);
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}*/

		int quest = item->sIndex - 4106 + 1;

		if (pMob[conn].extra.QuestInfo.Arch.Cristal == quest)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Youve_Done_It_Already]);
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		if (pMob[conn].extra.QuestInfo.Arch.Cristal >= quest)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Youve_Done_It_Already]);
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		if (pMob[conn].extra.QuestInfo.Arch.Cristal != quest - 1)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Need_BeforeQuest]);
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		pMob[conn].extra.QuestInfo.Arch.Cristal = quest;

		if (quest == 1)
			pMob[conn].MOB.BaseScore.MaxMp += 80;

		else if (quest == 2)
			pMob[conn].MOB.BaseScore.Ac += 30;

		else if (quest == 3)
			pMob[conn].MOB.BaseScore.MaxHp += 80;

		else
		{
			pMob[conn].MOB.BaseScore.MaxHp += 60;
			pMob[conn].MOB.BaseScore.MaxMp += 60;
			pMob[conn].MOB.BaseScore.Ac += 20;
		}

		pMob[conn].MOB.Exp = pMob[conn].MOB.Exp - 100000000;		
		if (pMob[conn].MOB.Exp >= 40000000000)
			pMob[conn].MOB.Exp = 40000000000;
		

		/*int newlevel = 0;

		for(int i = 0; i < MAX_LEVEL; i++)
		{
			if(pMob[conn].MOB.Exp >= g_pNextLevel[i] && pMob[conn].MOB.Exp <= g_pNextLevel[i+1])
				newlevel = i;

			if (newlevel >= MAX_LEVEL)
				newlevel = MAX_LEVEL;
		}

		pMob[conn].MOB.BaseScore.Level = newlevel;*/

		int check = pMob[conn].CheckGetLevel();

		if(check == 4)
			SetCircletSubGod(conn);

		BASE_GetBonusScorePoint(&pMob[conn].MOB, &pMob[conn].extra);

		pMob[conn].GetCurrentScore(conn);
		SendEtc(conn);
		SendScore(conn);

		SendEmotion(conn, 14, 3);

		sprintf(temp, g_pMessageStringTable[_DN_Play_Quest], g_pItemList[item->sIndex].Name);
		SendClientMessage(conn, temp);

		sprintf(temp, "useitem,archcristal complete etapa:%d", quest);
		ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

		if (amount > 1)
			BASE_SetItemAmount(item, amount - 1);
		else
			memset(item, 0, sizeof(STRUCT_ITEM));

		return;
	}
#pragma endregion
#pragma region Pedra da Fúria
	if (item->sIndex == 3020)
	{
		if ((pMob[conn].extra.ClassMaster == CELESTIAL || pMob[conn].extra.ClassMaster == CELESTIALCS || pMob[conn].extra.ClassMaster == SCELESTIAL) && pMob[conn].MOB.CurrentScore.Level >= 199)
		{
			if(pMob[conn].extra.Fame < 500)
			{
				SendItem(conn, m->SourType, m->SourPos, item);
				return;
			}

			if(pMob[conn].extra.QuestInfo.Circle == 0)
			{
				int sAgua = 0;
				int sTerra = 0;
				int sSol = 0;
				int sVento = 0;

				for (int i = 0; i < pMob[conn].MaxCarry; i++)
				{
					if (pMob[conn].MOB.Carry[i].sIndex == 5334)
						sAgua = 1;

					if (pMob[conn].MOB.Carry[i].sIndex == 5335)
						sTerra = 1;

					if (pMob[conn].MOB.Carry[i].sIndex == 5336)
						sSol = 1;

					if (pMob[conn].MOB.Carry[i].sIndex == 5337)
						sVento = 1;
				}

				if (sAgua == 1 && sTerra == 1 && sSol == 1 && sVento == 1)
				{
					for (int i = 0; i < pMob[conn].MaxCarry; i++)
					{
						if (pMob[conn].MOB.Carry[i].sIndex == 5334 && sAgua == 1)
						{
							memset(&pMob[conn].MOB.Carry[i], 0, sizeof(STRUCT_ITEM));
							SendItem(conn, ITEM_PLACE_CARRY, i, &pMob[conn].MOB.Carry[i]);
							sAgua = 0;
						}

						if (pMob[conn].MOB.Carry[i].sIndex == 5335 && sTerra == 1)
						{
							memset(&pMob[conn].MOB.Carry[i], 0, sizeof(STRUCT_ITEM));
							SendItem(conn, ITEM_PLACE_CARRY, i, &pMob[conn].MOB.Carry[i]);
							sTerra = 0;
						}

						if (pMob[conn].MOB.Carry[i].sIndex == 5336 && sSol == 1)
						{
							memset(&pMob[conn].MOB.Carry[i], 0, sizeof(STRUCT_ITEM));
							SendItem(conn, ITEM_PLACE_CARRY, i, &pMob[conn].MOB.Carry[i]);
							sSol = 0;
						}

						if (pMob[conn].MOB.Carry[i].sIndex == 5337 && sVento == 1)
						{
							memset(&pMob[conn].MOB.Carry[i], 0, sizeof(STRUCT_ITEM));
							SendItem(conn, ITEM_PLACE_CARRY, i, &pMob[conn].MOB.Carry[i]);
							sVento = 0;
						}
					}

					pMob[conn].extra.Fame -= 500;

					int _rand = rand()%115;

					if(_rand > 100)
						_rand -= 15;

					if(_rand < 100)
					{
						if(!pMob[conn].MOB.Equip[1].sIndex)
							memset(&pMob[conn].MOB.Equip[1], 0, sizeof(STRUCT_ITEM));

						pMob[conn].MOB.Equip[1].sIndex = 3507;

						SendItem(conn, ITEM_PLACE_EQUIP, 1, &pMob[conn].MOB.Equip[1]);

						sprintf(temp, g_pMessageStringTable[_DN_Play_Quest], "Cythera Arcana");
						SendClientMessage(conn, temp);
												 
						SendEmotion(conn, 14, 3);
						pMob[conn].extra.QuestInfo.Circle = 1;

						sprintf(temp, "useitem,furystone quest arcan complete.");
						ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);
					}

					else 
					{
						SendClientMessage(conn, g_pMessageStringTable[_NN_FAILURE]);
						sprintf(temp, "useitem,furystone quest arcan fail.");
						ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);
					}
					if (amount > 1)
						BASE_SetItemAmount(item, amount - 1);

					else
						memset(item, 0, sizeof(STRUCT_ITEM));

					return;
				}
				SendItem(conn, m->SourType, m->SourPos, item);
				return;
			}

			else if(pMob[conn].extra.QuestInfo.Circle && pMob[conn].extra.QuestInfo.Celestial.Reset < 3)
			{
				pMob[conn].extra.QuestInfo.Celestial.Reset++;
				pMob[conn].extra.Fame -= 500;

				pMob[conn].MOB.Exp -= 20000000;

				int newlevel = pMob[conn].MOB.CurrentScore.Level;

				for(int i = 0; i < MAX_CLEVEL; i++)
				{
					if(pMob[conn].MOB.Exp >= g_pNextLevel_2[i] && pMob[conn].MOB.Exp <= g_pNextLevel_2[i+1])
						newlevel = i;
										 
				}

				pMob[conn].MOB.BaseScore.Level = newlevel;
				int check = pMob[conn].CheckGetLevel();

				if(check == 4)
					SetCircletSubGod(conn);

				BASE_GetBonusScorePoint(&pMob[conn].MOB, &pMob[conn].extra);

				pMob[conn].GetCurrentScore(conn);
				SendEtc(conn);
				SendScore(conn);

				SendEmotion(conn, 14, 3);

				sprintf(temp, g_pMessageStringTable[_DN_Play_Quest], "Reset Celestial");
				SendClientMessage(conn, temp);

				sprintf(temp, "useitem,furystone quest reset %d", pMob[conn].extra.QuestInfo.Celestial.Reset);
				ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

				if (amount > 1)
				BASE_SetItemAmount(item, amount - 1);

				else
				memset(item, 0, sizeof(STRUCT_ITEM));

				return;
			}
		}
		if (pMob[conn].extra.ClassMaster != CELESTIAL)
		{
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		if(pMob[conn].MOB.CurrentScore.Level < 89)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Level_Limit2]);
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		if(pMob[conn].extra.QuestInfo.Celestial.Lv90 == 1)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Youve_Done_It_Already]);
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		if(pMob[conn].extra.Fame < 500)
		{
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		int _rand = rand()%115;

		if(_rand > 100)
			_rand -= 15;

		pMob[conn].extra.Fame -= 500;

		if(_rand < 100)
		{
			pMob[conn].extra.QuestInfo.Celestial.Lv90 = 1;

			SendEmotion(conn, 14, 3);
								 
			sprintf(temp, g_pMessageStringTable[_DN_Play_Quest], "Lv90");
			SendClientMessage(conn, temp);

			if (amount > 1)
				BASE_SetItemAmount(item, amount - 1);

			else
				memset(item, 0, sizeof(STRUCT_ITEM));

			STRUCT_ITEM Prize;
			memset(&Prize, 0, sizeof(STRUCT_ITEM));

			Prize.sIndex = 3502;

			PutItem(conn, &Prize);

			sprintf(temp, "useitem,furystone quest90 sucess");
			ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);
		}
		else
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_FAILURE]);
						
			if (amount > 1)
				BASE_SetItemAmount(item, amount - 1);

			else
				memset(item, 0, sizeof(STRUCT_ITEM));

			sprintf(temp, "useitem,furystone quest90 fail");
			ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);
		}

		return;
	}
#pragma endregion
#pragma region Remédio da Coragem
	if (item->sIndex == 4046 || item->sIndex == 646)
	{
		int sAffect = GetEmptyAffect(conn, 30);

		if (sAffect == -1 || pMob[conn].Affect[sAffect].Type == 30 && pMob[conn].Affect[sAffect].Value != 500)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_CantEatMore]);
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		pMob[conn].Affect[sAffect].Type = 30;
		pMob[conn].Affect[sAffect].Value = 1;
		pMob[conn].Affect[sAffect].Level = 500;
		pMob[conn].Affect[sAffect].Time = 100;

		if (pMob[conn].Affect[sAffect].Time >= 324000)
			pMob[conn].Affect[sAffect].Time = 324000;

		pMob[conn].GetCurrentScore(conn);
		SendScore(conn);

		if (amount > 1)
			BASE_SetItemAmount(item, amount - 1);

		else
			memset(item, 0, sizeof(STRUCT_ITEM));

		sprintf(temp, "useitem,remedio_da_coragem dam:500");
		ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);
		return;
	}
#pragma endregion
#pragma region Elixir da Coragem
	if (Vol == 230 && (item->sIndex == 647 || item->sIndex == 3378))
	{
		int sAffect = GetEmptyAffect(conn, 30);

		if (sAffect == -1 || pMob[conn].Affect[sAffect].Type == 30 && pMob[conn].Affect[sAffect].Value != 2000)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_CantEatMore]);
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		pMob[conn].Affect[sAffect].Type = 30;
		pMob[conn].Affect[sAffect].Value = 1;
		pMob[conn].Affect[sAffect].Level = 2000;
		pMob[conn].Affect[sAffect].Time = item->sIndex != 3378 ? AFFECT_1H : AFFECT_1D*30;

		if (pMob[conn].Affect[sAffect].Time >= 324000)
			pMob[conn].Affect[sAffect].Time = 324000;

		pMob[conn].GetCurrentScore(conn);
		SendScore(conn);

		if (amount > 1)
			BASE_SetItemAmount(item, amount - 1);

		else
			memset(item, 0, sizeof(STRUCT_ITEM));

		sprintf(temp, "useitem,elixir_da_coragem dam:2000");
		ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);
		return;
	}
#pragma endregion
#pragma region Feijões mágicos - Removedor
	if(Vol == 186)
	{
		STRUCT_ITEM *dest = GetItemPointer(&pMob[conn].MOB, pUser[conn].Cargo, m->DestType, m->DestPos);

		if (dest == NULL)
		{
			sprintf(temp, "err,use item fail dest %d %d", m->DestType, m->DestPos);
			ItemLog(temp, "-system", 0);
			return;
		}

		if (m->DestType || m->DestPos >= 8 && m->DestPos < 16 || m->DestType == ITEM_PLACE_EQUIP && m->DestPos == 0)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Only_To_Equips]);

			ItemLog("err,use item fail - dest", pUser[conn].AccountName, pUser[conn].IP);

			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		int sanc = BASE_GetItemSanc(dest);

		if(sanc < 1)
		{
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		int color = item->sIndex - 3407;


		//Removedor
		if(color == 10)
		{
			if ((dest->stEffect[0].cEffect && dest->stEffect[0].cEffect < 116 || dest->stEffect[0].cEffect && dest->stEffect[0].cEffect > 125))
			{
				if ((dest->stEffect[1].cEffect && dest->stEffect[1].cEffect < 116 || dest->stEffect[1].cEffect && dest->stEffect[1].cEffect > 125))
				{
					if ((dest->stEffect[2].cEffect && dest->stEffect[2].cEffect < 116 || dest->stEffect[2].cEffect && dest->stEffect[2].cEffect > 125))
					{
						SendClientMessage(conn, g_pMessageStringTable[_NN_Cant_Refine_More]);
						SendItem(conn, m->SourType, m->SourPos, item);
						return;
					}
					dest->stEffect[2].cEffect = 43;
				}
				else
					dest->stEffect[1].cEffect = 43;
			}
			else
				dest->stEffect[0].cEffect = 43;
		}
		else
		{
			if (dest->stEffect[0].cEffect && dest->stEffect[0].cEffect != 43 && (dest->stEffect[0].cEffect && dest->stEffect[0].cEffect < 116 || dest->stEffect[0].cEffect && dest->stEffect[0].cEffect > 125))
			{
				if (dest->stEffect[1].cEffect && dest->stEffect[1].cEffect != 43 && (dest->stEffect[1].cEffect && dest->stEffect[1].cEffect < 116 || dest->stEffect[1].cEffect && dest->stEffect[1].cEffect > 125))
				{
					if (dest->stEffect[2].cEffect && dest->stEffect[2].cEffect != 43 && (dest->stEffect[2].cEffect && dest->stEffect[2].cEffect < 116 || dest->stEffect[2].cEffect && dest->stEffect[2].cEffect > 125))
					{
						SendClientMessage(conn, g_pMessageStringTable[_NN_Cant_Refine_More]);
						SendItem(conn, m->SourType, m->SourPos, item);
						return;
					}
					dest->stEffect[2].cEffect = 116 + color;
				}
				else
					dest->stEffect[1].cEffect = 116 + color;
			}
			else
				dest->stEffect[0].cEffect = 116 + color;
		}

		sprintf(temp, "%s", g_pMessageStringTable[_NN_Refine_Success]);
		SendClientMessage(conn, temp);

		pMob[conn].GetCurrentScore(conn);

		SendScore(conn);
		sprintf(temp, "useitem,paint success %d+%d - (%d,%d,%d)", dest->sIndex, sanc, item->stEffect[0].cEffect, item->stEffect[1].cEffect, item->stEffect[2].cEffect);
		ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

		if (amount > 1)
			BASE_SetItemAmount(item, amount - 1);

		else
			memset(item, 0, sizeof(STRUCT_ITEM));

		SendEquip(conn, 0);
		SendItem(conn, m->DestType, m->DestPos, dest);
		SendEmotion(conn, 14, 3);

		return;
	}
#pragma endregion
#pragma region Pergaminho do Perdão
	if (Vol == 203)
	{
		SetPKPoint(conn, 150);

		MSG_CreateMob sm_pp;
		memset(&sm_pp, 0, sizeof(MSG_CreateMob));
		GetCreateMob(conn, &sm_pp);

		GridMulticast(pMob[conn].TargetX, pMob[conn].TargetY, (MSG_STANDARD*)&sm_pp, 0);

		SendEmotion(conn, 14, 3);

		sprintf(temp, "useitem,perdao new cp %d", 75);
		ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

		if (amount > 1)
			BASE_SetItemAmount(item, amount - 1);

		else
			memset(item, 0, sizeof(STRUCT_ITEM));

		return;
	}
#pragma endregion
#pragma region Gema de diamante
	if(Vol == 180)
	{
		STRUCT_ITEM *dest = GetItemPointer(&pMob[conn].MOB, pUser[conn].Cargo, m->DestType, m->DestPos);

		if (dest == NULL)
		{
			sprintf(temp, "err,use item fail dest %d %d", m->DestType, m->DestPos);
			ItemLog(temp, "-system", 0);
			return;
		}

		if (m->DestType || m->DestPos >= 8 && m->DestPos < 16 || m->DestType == ITEM_PLACE_EQUIP && m->DestPos == 0)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Only_To_Equips]);

			ItemLog("err,use item fail - dest", pUser[conn].AccountName, pUser[conn].IP);

			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		int sanc = BASE_GetItemSanc(dest);
		int ItemId = dest->sIndex;

		if(sanc < REF_10 && g_pItemList[ItemId].Grade < 5)
		{
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		if(g_pItemList[ItemId].Grade == 6)
			ItemId -= 1;

		else if(g_pItemList[ItemId].Grade == 7)
			ItemId -= 2;

		else if(g_pItemList[ItemId].Grade == 8)
			ItemId -= 3;

		dest->sIndex = ItemId;

		if(sanc == REF_10)
			BASE_SetItemSanc(dest, 10, 0);

		else if(sanc == REF_11)
			BASE_SetItemSanc(dest, 11, 0);

		else if(sanc == REF_12)
			BASE_SetItemSanc(dest, 12, 0);

		else if(sanc == REF_13)
			BASE_SetItemSanc(dest, 13, 0);

		else if(sanc == REF_14)
			BASE_SetItemSanc(dest, 14, 0);

		else if(sanc == REF_15)
			BASE_SetItemSanc(dest, 15, 0);

		pMob[conn].GetCurrentScore(conn);

		SendScore(conn);
		sprintf(temp, "useitem,change jewel success %d+%d - (%d,%d,%d)", dest->sIndex, sanc, item->stEffect[0].cEffect, item->stEffect[1].cEffect, item->stEffect[2].cEffect);
		ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

		if (amount > 1)
			BASE_SetItemAmount(item, amount - 1);

		else
			memset(item, 0, sizeof(STRUCT_ITEM));

		SendItem(conn, m->DestType, m->DestPos, dest);
		SendEmotion(conn, 14, 3);

		return;
	}
#pragma endregion
#pragma region Gema de esmeralda
	if(Vol == 181)
	{
		STRUCT_ITEM *dest = GetItemPointer(&pMob[conn].MOB, pUser[conn].Cargo, m->DestType, m->DestPos);

		if (dest == NULL)
		{
			sprintf(temp, "err,use item fail dest %d %d", m->DestType, m->DestPos);
			ItemLog(temp, "-system", 0);
			return;
		}

		if (m->DestType || m->DestPos >= 8 && m->DestPos < 16 || m->DestType == ITEM_PLACE_EQUIP && m->DestPos == 0)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Only_To_Equips]);

			ItemLog("err,use item fail - dest", pUser[conn].AccountName, pUser[conn].IP);

			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		int sanc = BASE_GetItemSanc(dest);
		int ItemId = dest->sIndex;

		if(sanc < REF_10 && g_pItemList[ItemId].Grade < 5)
		{
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		if(g_pItemList[ItemId].Grade == 5)
			ItemId += 1;

		else if(g_pItemList[ItemId].Grade == 7)
			ItemId -= 1;

		else if(g_pItemList[ItemId].Grade == 8)
			ItemId -= 2;

		dest->sIndex = ItemId;

		if(sanc == REF_10)
			BASE_SetItemSanc(dest, 10, 1);

		else if(sanc == REF_11)
			BASE_SetItemSanc(dest, 11, 1);

		else if(sanc == REF_12)
			BASE_SetItemSanc(dest, 12, 1);

		else if(sanc == REF_13)
			BASE_SetItemSanc(dest, 13, 1);

		else if(sanc == REF_14)
			BASE_SetItemSanc(dest, 14, 1);

		else if(sanc == REF_15)
			BASE_SetItemSanc(dest, 15, 1);

		pMob[conn].GetCurrentScore(conn);

		SendScore(conn);
		sprintf(temp, "useitem,change jewel success %d+%d - (%d,%d,%d)", dest->sIndex, sanc, item->stEffect[0].cEffect, item->stEffect[1].cEffect, item->stEffect[2].cEffect);
		ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

		if (amount > 1)
			BASE_SetItemAmount(item, amount - 1);

		else
			memset(item, 0, sizeof(STRUCT_ITEM));

		SendItem(conn, m->DestType, m->DestPos, dest);
		SendEmotion(conn, 14, 3);

		return;
	}
#pragma endregion
#pragma region Gema de Coral
	if(Vol == 182)
	{
		STRUCT_ITEM *dest = GetItemPointer(&pMob[conn].MOB, pUser[conn].Cargo, m->DestType, m->DestPos);

		if (dest == NULL)
		{
			sprintf(temp, "err,use item fail dest %d %d", m->DestType, m->DestPos);
			ItemLog(temp, "-system", 0);
			return;
		}

		if (m->DestType || m->DestPos >= 8 && m->DestPos < 16 || m->DestType == ITEM_PLACE_EQUIP && m->DestPos == 0)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Only_To_Equips]);

			ItemLog("err,use item fail - dest", pUser[conn].AccountName, pUser[conn].IP);

			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		int sanc = BASE_GetItemSanc(dest);
		int ItemId = dest->sIndex;

		if(sanc < REF_10 && g_pItemList[ItemId].Grade < 5)
		{
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		if(g_pItemList[ItemId].Grade == 5)
			ItemId += 2;

		else if(g_pItemList[ItemId].Grade == 6)
			ItemId += 1;

		else if(g_pItemList[ItemId].Grade == 8)
			ItemId -= 1;

		dest->sIndex = ItemId;

		if(sanc == REF_10)
			BASE_SetItemSanc(dest, 10, 2);

		else if(sanc == REF_11)
			BASE_SetItemSanc(dest, 11, 2);

		else if(sanc == REF_12)
			BASE_SetItemSanc(dest, 12, 2);

		else if(sanc == REF_13)
			BASE_SetItemSanc(dest, 13, 2);

		else if(sanc == REF_14)
			BASE_SetItemSanc(dest, 14, 2);

		else if(sanc == REF_15)
			BASE_SetItemSanc(dest, 15, 2);

		pMob[conn].GetCurrentScore(conn);

		SendScore(conn);
		sprintf(temp, "useitem,change jewel success %d+%d - (%d,%d,%d)", dest->sIndex, sanc, item->stEffect[0].cEffect, item->stEffect[1].cEffect, item->stEffect[2].cEffect);
		ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

		if (amount > 1)
			BASE_SetItemAmount(item, amount - 1);

		else
			memset(item, 0, sizeof(STRUCT_ITEM));

		SendItem(conn, m->DestType, m->DestPos, dest);
		SendEmotion(conn, 14, 3);

		return;
	}
#pragma endregion
#pragma region Gema de Garnet
	if(Vol == 183)
	{
		STRUCT_ITEM *dest = GetItemPointer(&pMob[conn].MOB, pUser[conn].Cargo, m->DestType, m->DestPos);

		if (dest == NULL)
		{
			sprintf(temp, "err,use item fail dest %d %d", m->DestType, m->DestPos);
			ItemLog(temp, "-system", 0);
			return;
		}

		if (m->DestType || m->DestPos >= 8 && m->DestPos < 16 || m->DestType == ITEM_PLACE_EQUIP && m->DestPos == 0)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Only_To_Equips]);

			ItemLog("err,use item fail - dest", pUser[conn].AccountName, pUser[conn].IP);

			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		int sanc = BASE_GetItemSanc(dest);
		int ItemId = dest->sIndex;

		if(sanc < REF_10 && g_pItemList[ItemId].Grade < 5)
		{
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		if(g_pItemList[ItemId].Grade == 5)
			ItemId += 3;

		else if(g_pItemList[ItemId].Grade == 6)
			ItemId += 2;

		else if(g_pItemList[ItemId].Grade == 7)
			ItemId += 1;

		dest->sIndex = ItemId;

		if(sanc == REF_10)
			BASE_SetItemSanc(dest, 10, 3);

		else if(sanc == REF_11)
			BASE_SetItemSanc(dest, 11, 3);

		else if(sanc == REF_12)
			BASE_SetItemSanc(dest, 12, 3);

		else if(sanc == REF_13)
			BASE_SetItemSanc(dest, 13, 3);

		else if(sanc == REF_14)
			BASE_SetItemSanc(dest, 14, 3);

		else if(sanc == REF_15)
			BASE_SetItemSanc(dest, 15, 3);

		pMob[conn].GetCurrentScore(conn);

		SendScore(conn);
		sprintf(temp, "useitem,change jewel success %d+%d - (%d,%d,%d)", dest->sIndex, sanc, item->stEffect[0].cEffect, item->stEffect[1].cEffect, item->stEffect[2].cEffect);
		ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

		if (amount > 1)
			BASE_SetItemAmount(item, amount - 1);

		else
			memset(item, 0, sizeof(STRUCT_ITEM));

		SendItem(conn, m->DestType, m->DestPos, dest);
		SendEmotion(conn, 14, 3);

		return;
	}
#pragma endregion
#pragma region Entrada do Território
	if(Vol == 188)
	{
		int territorio = item->sIndex - 4111;

		if(territorio <= 1 && pMob[conn].extra.ClassMaster != ARCH && pMob[conn].extra.ClassMaster != MORTAL)
		{
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}
		else if(territorio >= 2 && pMob[conn].extra.ClassMaster != CELESTIAL && pMob[conn].extra.ClassMaster != SCELESTIAL && pMob[conn].extra.ClassMaster != CELESTIALCS)
		{
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		if(territorio == 0)
			DoTeleport(conn, (3639 + rand()%5 - 3), (3639 + rand()%5 - 3));

		else if(territorio == 1)
			DoTeleport(conn, (3782 + rand()%5 - 3), (3527 + rand()%5 - 3));

		else
			DoTeleport(conn, (3911 + rand()%5 - 3), (3655 + rand()%5 - 3));

		sprintf(temp, "useitem,territorio ticket used %d", territorio);
		ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

		if (amount > 1)
			BASE_SetItemAmount(item, amount - 1);

		else
			memset(item, 0, sizeof(STRUCT_ITEM));

		return;
	}
#pragma endregion
#pragma region Barra de Prata
	if(Vol == 185)
	{
		int gold = 0;

		if(item->sIndex == 4010)
			gold = 100000000;

		else if(item->sIndex == 4011)
			gold = 1000000000;

		else if(item->sIndex == 4026)
			gold = 1000000;

		else if(item->sIndex == 4027)
			gold = 5000000;

		else if(item->sIndex == 4028)
			gold = 10000000;

		else if(item->sIndex == 4029)
			gold = 50000000;

		unsigned int fGold = pMob[conn].MOB.Coin + gold;

		if(fGold > 2000000000)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Cant_get_more_than_2G]);
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}
							
		pMob[conn].MOB.Coin = fGold;
		SendEtc(conn);
		SendEmotion(conn, 14, 3);

		sprintf(temp, "useitem,silver coin used V:%d fV:%d", gold, fGold);
		ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

		if (amount > 1)
			BASE_SetItemAmount(item, amount - 1);

		else
			memset(item, 0, sizeof(STRUCT_ITEM));

		return;
	}
#pragma endregion
#pragma region Jóias PvP
	if (Vol == 242)
	{
		int sAffect = GetEmptyAffect(conn, 8);

		int joia = 0;

		if(item->sIndex == 3200)
			joia = 0;

		else if(item->sIndex == 3201)
			joia = 1;

		else if(item->sIndex == 3202)
			joia = 2;

		else if(item->sIndex == 3204)
			joia = 3;

		else if(item->sIndex == 3205)
			joia = 4;

		else if(item->sIndex == 3206)
			joia = 5;

		else if(item->sIndex == 3208)
			joia = 6;

		else if(item->sIndex == 3209)
			joia = 7;

		if (sAffect == -1)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_CantEatMore]);
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		if(pMob[conn].Affect[sAffect].Type != 8)
		{
			pMob[conn].Affect[sAffect].Type = 8;
			pMob[conn].Affect[sAffect].Level = 1 << joia;
			pMob[conn].Affect[sAffect].Value = 0;
		}

		else if((pMob[conn].Affect[sAffect].Level & (1 << joia)) == 0)
			pMob[conn].Affect[sAffect].Level |= 1 << joia;

		pMob[conn].Affect[sAffect].Time = AFFECT_1H;
		pMob[conn].GetCurrentScore(conn);
		SendScore(conn);

		if (amount > 1)
			BASE_SetItemAmount(item, amount - 1);

		else
			memset(item, 0, sizeof(STRUCT_ITEM));

		sprintf(temp, "useitem,joia_pvp joia:%d", item->sIndex);
		ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);
		return;
	}
#pragma endregion
#pragma region Armazenagem - Recuperação
	if(Vol == 243)
	{
		if(item->sIndex == 3203)
		{
			int flagSkillOff = FALSE;

			for (int k = 0; k < MAX_AFFECT; k++)
			{
				int skillAffectType = pMob[conn].Affect[k].Type;
				if (skillAffectType == 1 || skillAffectType == 3 || skillAffectType == 5 || skillAffectType == 7 || skillAffectType == 10 || skillAffectType == 12 || skillAffectType == 20 || skillAffectType == 32)
				{
					pMob[conn].Affect[k].Type = 0;
					flagSkillOff = TRUE;
				}
			}

			if (flagSkillOff == TRUE)
			{
				pMob[conn].GetCurrentScore(conn);
				SendScore(conn);
			}
		}
		if (amount > 1)
			BASE_SetItemAmount(item, amount - 1);

		else
			memset(item, 0, sizeof(STRUCT_ITEM));

		sprintf(temp, "useitem,joia_pvp joia:%d", item->sIndex);
		ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);
		return;
	}
#pragma endregion
#pragma region Barra Mytril (Dano)
	if(Vol == 235)
	{
		STRUCT_ITEM *dest = GetItemPointer(&pMob[conn].MOB, pUser[conn].Cargo, m->DestType, m->DestPos);

		if (dest == NULL)
		{
			sprintf(temp, "err,use item fail dest %d %d", m->DestType, m->DestPos);
			ItemLog(temp, "-system", 0);
			return;
		}

		if (m->DestType || m->DestPos >= 8 && m->DestPos < 16 || m->DestType == ITEM_PLACE_EQUIP && m->DestPos == 0 || m->DestType == ITEM_PLACE_EQUIP && m->DestPos == 1)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Only_To_Equips]);

			ItemLog("err,use item fail - dest", pUser[conn].AccountName, pUser[conn].IP);

			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		int sanc = BASE_GetItemSanc(dest);
		int ItemId = dest->sIndex;

		if(sanc < REF_10 || BASE_GetItemAbility(dest, EF_MOBTYPE) != 1)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_10]);
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		int dam = BASE_GetItemAbilityNosanc(dest, EF_DAMAGE);

		int max_add = m->DestPos != 6 && m->DestPos != 7 ? 42 : 81;
		int min_add = m->DestPos != 6 && m->DestPos != 7 ? 12 : 45;

		if(dam < min_add)
		{
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		if(dam >= max_add)
		{
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		int RateSucess = 18;
		int _rand = rand() % 115;
		if(_rand > 100)
			_rand -= 15;

		if (_rand <= RateSucess && RateSucess)
		{
			for(int i = 0; i < 3; i++)
			{
				if(dest->stEffect[i].cEffect == EF_DAMAGE || dest->stEffect[i].cEffect == EF_DAMAGE2)
				{
					dest->stEffect[i].cEffect = EF_DAMAGE;
					dest->stEffect[i].cValue += m->DestPos != 6 && m->DestPos != 7 ? 6 : 9;

					if(dest->stEffect[i].cValue > (m->DestPos != 6 && m->DestPos != 7 ? 42 : 81))
						dest->stEffect[i].cValue = (m->DestPos != 6 && m->DestPos != 7 ? 42 : 81);

					break;
				}
			}

			pMob[conn].GetCurrentScore(conn);

			SendScore(conn);
			sprintf(temp, "useitem,upgrade success %d+%d - (%d,%d,%d)", dest->sIndex, sanc, item->stEffect[0].cEffect, item->stEffect[1].cEffect, item->stEffect[2].cEffect);
			ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

			if (amount > 1)
				BASE_SetItemAmount(item, amount - 1);

			else
				memset(item, 0, sizeof(STRUCT_ITEM));

			SendItem(conn, m->DestType, m->DestPos, dest);
			SendEmotion(conn, 14, 3);
		}

		else
		{
			if((rand()%7) == 0)
			{
				for(int i = 0; i < 3; i++)
				{
					if(dest->stEffect[i].cEffect == EF_DAMAGE || dest->stEffect[i].cEffect == EF_DAMAGE2)
					{
						dest->stEffect[i].cEffect = EF_DAMAGE;
											
						if(dest->stEffect[i].cValue >= (m->DestPos != 6 && m->DestPos != 7 ? 6 : 9))
							dest->stEffect[i].cValue -= m->DestPos != 6 && m->DestPos != 7 ? 6 : 9;

						break;
					}
				}
				sprintf(temp, "useitem,upgrade fail-- %d+%d - (%d,%d,%d)", dest->sIndex, sanc, item->stEffect[0].cEffect, item->stEffect[1].cEffect, item->stEffect[2].cEffect);

				SendItem(conn, m->DestType, m->DestPos, dest);
				SendEmotion(conn, 14, 3);
			}
			else
			{
				sprintf(temp, "useitem,upgrade fail %d+%d - (%d,%d,%d)", dest->sIndex, sanc, item->stEffect[0].cEffect, item->stEffect[1].cEffect, item->stEffect[2].cEffect);
									
				int sFace = pMob[conn].MOB.Equip[0].sIndex / 10;

				if (sFace)
					SendEmotion(conn, 15, 0);

				else
					SendEmotion(conn, 20, 0);
			}
			ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

			if (amount > 1)
				BASE_SetItemAmount(item, amount - 1);

			else
				memset(item, 0, sizeof(STRUCT_ITEM));
		}

		return;
	}
#pragma endregion
#pragma region Barra Mytril (Magia)
	if(Vol == 236)
	{
		STRUCT_ITEM *dest = GetItemPointer(&pMob[conn].MOB, pUser[conn].Cargo, m->DestType, m->DestPos);

		if (dest == NULL)
		{
			sprintf(temp, "err,use item fail dest %d %d", m->DestType, m->DestPos);
			ItemLog(temp, "-system", 0);
			return;
		}

		if (m->DestType || m->DestPos >= 8 && m->DestPos < 16 || m->DestType == ITEM_PLACE_EQUIP && m->DestPos == 0 || m->DestType == ITEM_PLACE_EQUIP && m->DestPos == 1)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Only_To_Equips]);

			ItemLog("err,use item fail - dest", pUser[conn].AccountName, pUser[conn].IP);

			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		int sanc = BASE_GetItemSanc(dest);
		int ItemId = dest->sIndex;

		if(sanc < REF_10 || BASE_GetItemAbility(dest, EF_MOBTYPE) != 1)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_10]);
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		int dam = BASE_GetItemAbilityNosanc(dest, EF_MAGIC);

		int max_add = m->DestPos != 6 && m->DestPos != 7 ? 14 : 36;
		int min_add = m->DestPos != 6 && m->DestPos != 7 ? 6 : 24;

		if(dam < min_add)
		{
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		if(dam >= max_add)
		{
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		for (int a = 0; a < 3; a++)
		{
			if (dest->stEffect[a].cEffect == EF_MAGIC || dest->stEffect[a].cEffect == EF_MAGICADD)
			{
				dest->stEffect[a].cEffect = EF_MAGIC;
				if (dest->stEffect[a].cValue < (m->DestPos != 6 && m->DestPos != 7 ? 6 : 24))
				{
					SendClientMessage(conn, "Não é possível aprimorar este item.");
					SendItem(conn, m->SourType, m->SourPos, item);
					return;
				}
			}
		}


		for (int b = 0; b < 3; b++)
		{
			if (dest->stEffect[b].cEffect == EF_MAGIC || dest->stEffect[b].cEffect == EF_MAGICADD)
			{
				dest->stEffect[b].cEffect = EF_MAGIC;
				if (dest->stEffect[b].cValue == (m->DestPos != 6 && m->DestPos != 7 ? 14 : 36))
				{
					SendClientMessage(conn, "Não é possível aprimorar este item.");
					SendItem(conn, m->SourType, m->SourPos, item);
					return;
				}
			}
		}

		int RateSucess = 18;
		int _rand = rand() % 115;
		if(_rand > 100)
			_rand -= 15;

		if (_rand <= RateSucess && RateSucess)
		{
			for(int i = 0; i < 3; i++)
			{
				if(dest->stEffect[i].cEffect == EF_MAGIC || dest->stEffect[i].cEffect == EF_MAGICADD)
				{
					dest->stEffect[i].cEffect = EF_MAGIC;
					dest->stEffect[i].cValue += m->DestPos != 6 && m->DestPos != 7 ? 2 : 4;

					if(dest->stEffect[i].cValue > (m->DestPos != 6 && m->DestPos != 7 ? 14 : 36))
						dest->stEffect[i].cValue = (m->DestPos != 6 && m->DestPos != 7 ? 14 : 36);

					break;
				}
			}

			pMob[conn].GetCurrentScore(conn);

			SendScore(conn);
			sprintf(temp, "useitem,upgrade success %d+%d - (%d,%d,%d)", dest->sIndex, sanc, item->stEffect[0].cEffect, item->stEffect[1].cEffect, item->stEffect[2].cEffect);
			ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

			if (amount > 1)
				BASE_SetItemAmount(item, amount - 1);

			else
				memset(item, 0, sizeof(STRUCT_ITEM));

			SendItem(conn, m->DestType, m->DestPos, dest);
			SendEmotion(conn, 14, 3);
		}

		else
		{
			if((rand()%7) == 0)
			{
				for(int i = 0; i < 3; i++)
				{
					if(dest->stEffect[i].cEffect == EF_MAGIC || dest->stEffect[i].cEffect == EF_MAGICADD)
					{
						dest->stEffect[i].cEffect = EF_MAGIC;

						if(dest->stEffect[i].cValue > (m->DestPos != 6 && m->DestPos != 7 ? 2 : 4))
							dest->stEffect[i].cValue -= m->DestPos != 6 && m->DestPos != 7 ? 2 : 4;

						break;
					}
				}
				sprintf(temp, "useitem,upgrade fail-- %d+%d - (%d,%d,%d)", dest->sIndex, sanc, item->stEffect[0].cEffect, item->stEffect[1].cEffect, item->stEffect[2].cEffect);

				SendItem(conn, m->DestType, m->DestPos, dest);
				SendEmotion(conn, 14, 3);
			}
			else
			{
				sprintf(temp, "useitem,upgrade fail %d+%d - (%d,%d,%d)", dest->sIndex, sanc, item->stEffect[0].cEffect, item->stEffect[1].cEffect, item->stEffect[2].cEffect);
									
				int sFace = pMob[conn].MOB.Equip[0].sIndex / 10;

				if (sFace)
				SendEmotion(conn, 15, 0);

				else
					SendEmotion(conn, 20, 0);
			}
			ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

			if (amount > 1)
				BASE_SetItemAmount(item, amount - 1);

			else
				memset(item, 0, sizeof(STRUCT_ITEM));
		}

		return;
	}
#pragma endregion
#pragma region Barra Mytril (Defesa)
	if(Vol == 237)
	{
		STRUCT_ITEM *dest = GetItemPointer(&pMob[conn].MOB, pUser[conn].Cargo, m->DestType, m->DestPos);

		if (dest == NULL)
		{
			sprintf(temp, "err,use item fail dest %d %d", m->DestType, m->DestPos);
			ItemLog(temp, "-system", 0);
			return;
		}

		if (m->DestType || m->DestPos >= 6 && m->DestPos < 16 || m->DestType == ITEM_PLACE_EQUIP && m->DestPos == 0 || m->DestType == ITEM_PLACE_EQUIP && m->DestPos == 1)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Only_To_Equips]);

			ItemLog("err,use item fail - dest", pUser[conn].AccountName, pUser[conn].IP);

			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		if(dest->sIndex >= 3500 && dest->sIndex <= 3507)
		{
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		int sanc = BASE_GetItemSanc(dest);
		int ItemId = dest->sIndex;

		if(sanc < REF_10 || BASE_GetItemAbility(dest, EF_MOBTYPE) != 1)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_10]);
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		int dam = 0;
		int effect = 0;

		for(int i = 0; i < 3; i++)
		{
			if(dest->stEffect[i].cEffect == EF_AC || dest->stEffect[i].cEffect == EF_ACADD2)
			{
				dam = dest->stEffect[i].cValue;
				effect = dest->stEffect[i].cEffect;

				break;
			}
		}

		int max_add = (effect == EF_AC ? 40 : 70);
		int min_add = (effect == EF_AC ? 15 : 15);


		if(dam < min_add)
		{
			SendItem(conn, m->SourType, m->SourPos, item);
			SendClientMessage(conn, "Não é possível aprimorar este item.");
			return;
		}

		if(dam >= max_add)
		{
			SendItem(conn, m->SourType, m->SourPos, item);
			SendClientMessage(conn, "Não é possível aprimorar este item.");
			return;
		}


		int RateSucess = 50;
		int _rand = rand() % 115;
		if(_rand > 100)
			_rand -= 15;

		if (_rand <= RateSucess && RateSucess)
		{
			for(int i = 0; i < 3; i++)
			{
				if(dest->stEffect[i].cEffect == EF_AC || dest->stEffect[i].cEffect == EF_ACADD2)
				{
					dest->stEffect[i].cValue += (dest->stEffect[i].cEffect == EF_AC ? 5 : 10);

					if(dest->stEffect[i].cValue > (dest->stEffect[i].cEffect == EF_AC ? 40 : 70))
						dest->stEffect[i].cValue = (dest->stEffect[i].cEffect == EF_AC ? 40 : 70);

					break;
				}
			}

			pMob[conn].GetCurrentScore(conn);

			SendScore(conn);
			sprintf(temp, "useitem,upgrade success %d+%d - (%d,%d,%d)", dest->sIndex, sanc, item->stEffect[0].cEffect, item->stEffect[1].cEffect, item->stEffect[2].cEffect);
			ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

			if (amount > 1)
				BASE_SetItemAmount(item, amount - 1);

			else
				memset(item, 0, sizeof(STRUCT_ITEM));

			SendItem(conn, m->DestType, m->DestPos, dest);
			SendEmotion(conn, 14, 3);
		}

		else
		{
			if((rand()%7) == 0)
			{
				for(int i = 0; i < 3; i++)
				{
					if(dest->stEffect[i].cEffect == EF_AC || dest->stEffect[i].cEffect == EF_ACADD2)
					{
						if(dest->stEffect[i].cEffect == EF_AC && dest->stEffect[i].cValue > 5 || dest->stEffect[i].cEffect == EF_ACADD2 && dest->stEffect[i].cValue > 10)
							dest->stEffect[i].cValue -= (dest->stEffect[i].cEffect == EF_AC ? 5 : 10);

						break;
					}
				}
				sprintf(temp, "useitem,upgrade fail-- %d+%d - (%d,%d,%d)", dest->sIndex, sanc, item->stEffect[0].cEffect, item->stEffect[1].cEffect, item->stEffect[2].cEffect);

				SendItem(conn, m->DestType, m->DestPos, dest);
				SendEmotion(conn, 14, 3);
			}
			else
			{
				sprintf(temp, "useitem,upgrade fail %d+%d - (%d,%d,%d)", dest->sIndex, sanc, item->stEffect[0].cEffect, item->stEffect[1].cEffect, item->stEffect[2].cEffect);
									
				int sFace = pMob[conn].MOB.Equip[0].sIndex / 10;

				if (sFace)
				SendEmotion(conn, 15, 0);

				else
					SendEmotion(conn, 20, 0);
			}
			ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

			if (amount > 1)
				BASE_SetItemAmount(item, amount - 1);

			else
				memset(item, 0, sizeof(STRUCT_ITEM));
		}

		return;
	}
#pragma endregion
#pragma region Barra Mytril (Crítico)
	if(Vol == 238)
	{
		STRUCT_ITEM *dest = GetItemPointer(&pMob[conn].MOB, pUser[conn].Cargo, m->DestType, m->DestPos);

		if (dest == NULL)
		{
			sprintf(temp, "err,use item fail dest %d %d", m->DestType, m->DestPos);
			ItemLog(temp, "-system", 0);
			return;
		}

		if (m->DestType || m->DestPos >= 8 && m->DestPos < 16 || m->DestType == ITEM_PLACE_EQUIP && m->DestPos == 0 || m->DestType == ITEM_PLACE_EQUIP && m->DestPos == 1)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Only_To_Equips]);

			ItemLog("err,use item fail - dest", pUser[conn].AccountName, pUser[conn].IP);

			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		int sanc = BASE_GetItemSanc(dest);
		int ItemId = dest->sIndex;

		if(sanc < REF_10 || BASE_GetItemAbility(dest, EF_MOBTYPE) != 1)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_10]);
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		int dam = BASE_GetItemAbilityNosanc(dest, EF_CRITICAL2);

		int max_add = m->DestPos != 6 && m->DestPos != 7 ? 90 : 0;
		int min_add = m->DestPos != 6 && m->DestPos != 7 ? 50 : 0;

		if(dam < min_add)
		{
			SendItem(conn, m->SourType, m->SourPos, item);
			SendClientMessage(conn, "Não é possível aprimorar este item.");
			return;
		}

		if(dam >= max_add)
		{
			SendItem(conn, m->SourType, m->SourPos, item);
			SendClientMessage(conn, "Não é possível aprimorar este item.");
			return;
		}


		int RateSucess = 18;
		
		int _rand = rand() % 115;
		
		if(_rand > 100)
			_rand -= 15;

		if (_rand <= RateSucess && RateSucess)
		{
			for(int i = 0; i < 3; i++)
			{
				if(dest->stEffect[i].cEffect == EF_CRITICAL || dest->stEffect[i].cEffect == EF_CRITICAL2)
				{
					dest->stEffect[i].cEffect = EF_CRITICAL2;
					dest->stEffect[i].cValue += m->DestPos != 6 && m->DestPos != 7 ? 10 : 1;

					if(dest->stEffect[i].cValue > (m->DestPos != 6 && m->DestPos != 7 && m->DestPos != 4 ? 90 : 0))
						dest->stEffect[i].cValue = (m->DestPos != 6 && m->DestPos != 7 && m->DestPos != 4 ? 90 : 0);

					break;
				}
			}

			pMob[conn].GetCurrentScore(conn);

			SendScore(conn);
			sprintf(temp, "useitem,upgrade success %d+%d - (%d,%d,%d)", dest->sIndex, sanc, item->stEffect[0].cEffect, item->stEffect[1].cEffect, item->stEffect[2].cEffect);
			ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

			if (amount > 1)
				BASE_SetItemAmount(item, amount - 1);

			else
				memset(item, 0, sizeof(STRUCT_ITEM));

			SendItem(conn, m->DestType, m->DestPos, dest);
			SendEmotion(conn, 14, 3);
		}

		else
		{
			if((rand()%7) == 0)
			{
				for(int i = 0; i < 3; i++)
				{
					if(dest->stEffect[i].cEffect == EF_CRITICAL || dest->stEffect[i].cEffect == EF_CRITICAL2)
					{
						dest->stEffect[i].cEffect = EF_CRITICAL2;
						if(dest->stEffect[i].cValue > (m->DestPos != 6 && m->DestPos != 7 ? 10 : 10))
							dest->stEffect[i].cValue -= m->DestPos != 6 && m->DestPos != 7 ? 10 : 10;
						break;
					}
				}
				sprintf(temp, "useitem,upgrade fail-- %d+%d - (%d,%d,%d)", dest->sIndex, sanc, item->stEffect[0].cEffect, item->stEffect[1].cEffect, item->stEffect[2].cEffect);

				SendItem(conn, m->DestType, m->DestPos, dest);
				SendEmotion(conn, 14, 3);
			}
			else
			{
				sprintf(temp, "useitem,upgrade fail %d+%d - (%d,%d,%d)", dest->sIndex, sanc, item->stEffect[0].cEffect, item->stEffect[1].cEffect, item->stEffect[2].cEffect);
									
				int sFace = pMob[conn].MOB.Equip[0].sIndex / 10;

				if (sFace)
					SendEmotion(conn, 15, 0);

				else
					SendEmotion(conn, 20, 0);
			}
			ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

			if (amount > 1)
				BASE_SetItemAmount(item, amount - 1);

			else
				memset(item, 0, sizeof(STRUCT_ITEM));
		}

		return;
	}
#pragma endregion
#pragma region Classe
	if(Vol == 190)
	{
		STRUCT_ITEM *dest = GetItemPointer(&pMob[conn].MOB, pUser[conn].Cargo, m->DestType, m->DestPos);

		if (dest == NULL)
		{
			sprintf(temp, "err,use item fail dest %d %d", m->DestType, m->DestPos);
			ItemLog(temp, "-system", 0);
			return;
		}

		if (!m->DestType || m->DestPos >= 60)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Only_To_Equips]);

			ItemLog("err,use item fail - dest", pUser[conn].AccountName, pUser[conn].IP);

			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		int sanc = BASE_GetItemSanc(dest);
		int ItemId = dest->sIndex;

		if(sanc > 9 || BASE_GetItemAbility(dest, EF_MOBTYPE) != 0 && BASE_GetItemAbility(dest, EF_MOBTYPE) != 2)
		{
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		int replation = item->sIndex >= 4016 && item->sIndex <= 4020 ? item->sIndex - 4015 : item->sIndex - 4020;

		if(BASE_GetItemAbility(dest, EF_ITEMLEVEL) != replation)
		{
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		SetItemBonus2(dest);
		SendItem(conn, m->DestType, m->DestPos, dest);
		SendEmotion(conn, 14, 3);

		sprintf(temp, "useitem,reset effect %d+%d - (%d,%d,%d)", dest->sIndex, sanc, dest->stEffect[0].cEffect, dest->stEffect[1].cEffect, dest->stEffect[2].cEffect);	
		ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

		if (amount > 1)
			BASE_SetItemAmount(item, amount - 1);

		else
			memset(item, 0, sizeof(STRUCT_ITEM));

		return;
	}
#pragma endregion
#pragma region Catalizadores
	if(Vol == 94)
	{
		STRUCT_ITEM *dest = GetItemPointer(&pMob[conn].MOB, pUser[conn].Cargo, m->DestType, m->DestPos);

		if (dest == NULL)
		{
			sprintf(temp, "err,use item fail dest %d %d", m->DestType, m->DestPos);
			ItemLog(temp, "-system", 0);
			return;
		}

		if (m->DestType || m->DestPos != 14)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Mount_Not_Match]);

			ItemLog("err,use item fail - dest", pUser[conn].AccountName, pUser[conn].IP);

			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		int mount = (dest->sIndex - 2333);
		int cat = item->sIndex - 3344;

		if(mount >= 0 && mount <= 2)
			mount = 0;

		if(mount >= 3 && mount <= 6 || mount >= 8 && mount <= 11)
			mount = 1;

		if(mount == 7 || mount == 12 || mount == 24)
			mount = 2;

		if(mount >= 13 && mount <= 15)
			mount = 3;

		if(mount >= 18 && mount <= 20 || mount == 25)
			mount = 4;

		if(mount >= 21 && mount <= 23)
			mount = 5;

		if(mount >= 16 && mount <= 17)
			mount = 6;

		if(mount != cat)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Mount_Not_Match]);

			ItemLog("err,use item fail - dest", pUser[conn].AccountName, pUser[conn].IP);

			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		dest->sIndex += 30;
								
		dest->stEffect[1].cValue = rand() % 20 + dest->stEffect[1].cEffect;
		dest->stEffect[1].cEffect = 0;
		dest->stEffect[2].cValue = 0;
								 
		SendClientMessage(conn, g_pMessageStringTable[_NN_Mount_Growth]);
		MountProcess(conn, 0);

		sprintf(temp, "useitem,catalizador %d - (%d,%d,%d)", dest->sIndex, item->stEffect[1].cEffect, item->stEffect[1].cValue, item->stEffect[2].cValue);
									
		int sFace = pMob[conn].MOB.Equip[0].sIndex / 10;

		if (sFace)
			SendEmotion(conn, 15, 0);
		else
			SendEmotion(conn, 20, 0);
								 
		SendItem(conn, m->DestType, m->DestPos, dest);

		ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

		if (amount > 1)
			BASE_SetItemAmount(item, amount - 1);

		else
			memset(item, 0, sizeof(STRUCT_ITEM));
						
		return;
	}
#pragma endregion		
#pragma region Restaurador
	if(Vol == 93)
	{
		STRUCT_ITEM *dest = GetItemPointer(&pMob[conn].MOB, pUser[conn].Cargo, m->DestType, m->DestPos);

		if (dest == NULL)
		{
			sprintf(temp, "err,use item fail dest %d %d", m->DestType, m->DestPos);
			ItemLog(temp, "-system", 0);
			return;
		}

		if (m->DestType || m->DestPos != 14)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Mount_Not_Match]);

			ItemLog("err,use item fail - dest", pUser[conn].AccountName, pUser[conn].IP);

			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		int mount = (dest->sIndex - 2363);
		int cat = item->sIndex - 3351;

		if(mount >= 0 && mount <= 2)
			mount = 0;

		if(mount >= 3 && mount <= 6 || mount >= 8 && mount <= 11)
			mount = 1;

		if(mount == 7 || mount == 12 || mount == 24)
			mount = 2;

		if(mount >= 13 && mount <= 15)
			mount = 3;

		if(mount >= 18 && mount <= 20 || mount == 25)
			mount = 4;

		if(mount >= 21 && mount <= 23)
			mount = 5;

		if(mount >= 16 && mount <= 17)
			mount = 6;

		if(mount != cat)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Mount_Not_Match]);

			ItemLog("err,use item fail - dest", pUser[conn].AccountName, pUser[conn].IP);

			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}
		if (dest->stEffect[1].cValue >= 50 || dest->stEffect[1].cValue <= 5 || dest->stEffect[0].sValue <= 0)
		{
			ItemLog("err,use item fail - dest", pUser[conn].AccountName, pUser[conn].IP);

			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}
		dest->stEffect[1].cValue += rand() % 2 + 1;

		sprintf(temp, "useitem,restaurador %d - (%d,%d,%d)", dest->sIndex, item->stEffect[1].cEffect, item->stEffect[1].cValue, item->stEffect[2].cValue);
									
		int sFace = pMob[conn].MOB.Equip[0].sIndex / 10;

		if (sFace)
			SendEmotion(conn, 15, 0);
		else
			SendEmotion(conn, 20, 0);
								 
		SendItem(conn, m->DestType, m->DestPos, dest);

		ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

		if (amount > 1)
			BASE_SetItemAmount(item, amount - 1);

		else
			memset(item, 0, sizeof(STRUCT_ITEM));
						
		return;
	}
#pragma endregion
#pragma region Extração Elmo
	if (item->sIndex == 3021)
	{
		int extra = BASE_GetItemAbility(item, EF_ITEMLEVEL);

		if(pMob[conn].MOB.Equip[1].sIndex == 0)
		{
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		if(extra <= 5)
		{//Item abaixo de E
			int elmoextra = BASE_GetItemAbility(&pMob[conn].MOB.Equip[1], EF_ITEMLEVEL);
			int elmomobtype = BASE_GetItemAbility(&pMob[conn].MOB.Equip[1], EF_MOBTYPE);

			if(elmoextra != 0 && elmoextra != 5 || elmoextra != extra || elmomobtype != 2 && elmomobtype != 0)
			{
				SendItem(conn, m->SourType, m->SourPos, item);
				return;
			}

			int sanc = BASE_GetItemSanc(&pMob[conn].MOB.Equip[1]);

			if(sanc != REF_11)
			{
				sprintf(temp, g_pMessageStringTable[_DN_SANCREQ], 11);
				SendClientMessage(conn, temp);
				SendItem(conn, m->SourType, m->SourPos, item);
				return;
			}

			pMob[conn].MOB.Equip[1].stEffect[1].cEffect = item->stEffect[1].cEffect;
			pMob[conn].MOB.Equip[1].stEffect[1].cValue = item->stEffect[1].cValue;

			pMob[conn].MOB.Equip[1].stEffect[2].cEffect = item->stEffect[2].cEffect;
			pMob[conn].MOB.Equip[1].stEffect[2].cValue = item->stEffect[2].cValue;

			SendItem(conn, ITEM_PLACE_EQUIP, 1, &pMob[conn].MOB.Equip[1]);

			sprintf(temp, "useitem,extracao %d-%d:%d:%d:%d:%d:%d", item->sIndex, item->stEffect[0].cEffect, item->stEffect[0].cValue, item->stEffect[1].cEffect, item->stEffect[1].cValue, item->stEffect[2].cEffect, item->stEffect[2].cValue);
			ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

			if (amount > 1)
				BASE_SetItemAmount(item, amount - 1);

			else
				memset(item, 0, sizeof(STRUCT_ITEM));

			SendEmotion(conn, 14, 3);
			return;
		}
		//Extração de cythera

		else 
		{
			int elmo = pMob[conn].MOB.Equip[1].sIndex;

			if(elmo < 3500 || elmo > 3507)
			{
			//SendClientMessage(conn, g_pMessageStringTable[_NN_ExtraWrongItemLevel]);
				SendItem(conn, m->SourType, m->SourPos, item);
				return;
			}

			int sanc = BASE_GetItemSanc(&pMob[conn].MOB.Equip[1]);

			if(sanc != REF_11)
			{
				sprintf(temp, g_pMessageStringTable[_DN_SANCREQ], 11);
				SendClientMessage(conn, temp);
				SendItem(conn, m->SourType, m->SourPos, item);
				return;
			}

			pMob[conn].MOB.Equip[1].stEffect[1].cEffect = item->stEffect[1].cEffect;
			pMob[conn].MOB.Equip[1].stEffect[1].cValue = item->stEffect[1].cValue;

			pMob[conn].MOB.Equip[1].stEffect[2].cEffect = item->stEffect[2].cEffect;
			pMob[conn].MOB.Equip[1].stEffect[2].cValue = item->stEffect[2].cValue;

			SendItem(conn, ITEM_PLACE_EQUIP, 1, &pMob[conn].MOB.Equip[1]);

			sprintf(temp, "useitem,extracao %d-%d:%d:%d:%d:%d:%d", item->sIndex, item->stEffect[0].cEffect, item->stEffect[0].cValue, item->stEffect[1].cEffect, item->stEffect[1].cValue, item->stEffect[2].cEffect, item->stEffect[2].cValue);
			ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

			if (amount > 1)
				BASE_SetItemAmount(item, amount - 1);

			else
				memset(item, 0, sizeof(STRUCT_ITEM));

			SendEmotion(conn, 14, 3);
			return;
		}

		return;
	}
#pragma endregion
#pragma region Extração Peito
	if (item->sIndex == 3022)
	{
		int extra = BASE_GetItemAbility(item, EF_ITEMLEVEL);

		if(pMob[conn].MOB.Equip[2].sIndex == 0)
		{
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		if(extra <= 5)
		{//Item abaixo de E
			int destextra = BASE_GetItemAbility(&pMob[conn].MOB.Equip[2], EF_ITEMLEVEL);
			int desttype = BASE_GetItemAbility(&pMob[conn].MOB.Equip[2], EF_MOBTYPE);

			if(destextra != 0 && destextra != 5 || destextra != extra || desttype != 2 && desttype != 0)
			{
				SendItem(conn, m->SourType, m->SourPos, item);
				return;
			}

			int sanc = BASE_GetItemSanc(&pMob[conn].MOB.Equip[2]);

			if(sanc != REF_11)
			{
				sprintf(temp, g_pMessageStringTable[_DN_SANCREQ], 11);
				SendClientMessage(conn, temp);
				SendItem(conn, m->SourType, m->SourPos, item);
				return;
			}

			pMob[conn].MOB.Equip[2].stEffect[1].cEffect = item->stEffect[1].cEffect;
			pMob[conn].MOB.Equip[2].stEffect[1].cValue = item->stEffect[1].cValue;

			pMob[conn].MOB.Equip[2].stEffect[2].cEffect = item->stEffect[2].cEffect;
			pMob[conn].MOB.Equip[2].stEffect[2].cValue = item->stEffect[2].cValue;

			SendItem(conn, ITEM_PLACE_EQUIP, 2, &pMob[conn].MOB.Equip[2]);
								 
			sprintf(temp, "useitem,extracao %d-%d:%d:%d:%d:%d:%d", item->sIndex, item->stEffect[0].cEffect, item->stEffect[0].cValue, item->stEffect[1].cEffect, item->stEffect[1].cValue, item->stEffect[2].cEffect, item->stEffect[2].cValue);
			ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);
								 
			if (amount > 1)
				BASE_SetItemAmount(item, amount - 1);

			else
				memset(item, 0, sizeof(STRUCT_ITEM));

			SendEmotion(conn, 14, 3);
			return;
		}
		//Extração de item arch

		else 
		{
			int desttype = BASE_GetItemAbility(&pMob[conn].MOB.Equip[2], EF_MOBTYPE);

			if(desttype != 1)
			{
				SendItem(conn, m->SourType, m->SourPos, item);
				return;
			}

			int sanc = BASE_GetItemSanc(&pMob[conn].MOB.Equip[2]);

			if(sanc != REF_11)
			{
				sprintf(temp, g_pMessageStringTable[_DN_SANCREQ], 11);
				SendClientMessage(conn, temp);
				SendItem(conn, m->SourType, m->SourPos, item);
				return;
			}

			pMob[conn].MOB.Equip[2].stEffect[1].cEffect = item->stEffect[1].cEffect;
			pMob[conn].MOB.Equip[2].stEffect[1].cValue = item->stEffect[1].cValue;

			pMob[conn].MOB.Equip[2].stEffect[2].cEffect = item->stEffect[2].cEffect;
			pMob[conn].MOB.Equip[2].stEffect[2].cValue = item->stEffect[2].cValue;

			SendItem(conn, ITEM_PLACE_EQUIP, 2, &pMob[conn].MOB.Equip[2]);

			sprintf(temp, "useitem,extracao %d-%d:%d:%d:%d:%d:%d", item->sIndex, item->stEffect[0].cEffect, item->stEffect[0].cValue, item->stEffect[1].cEffect, item->stEffect[1].cValue, item->stEffect[2].cEffect, item->stEffect[2].cValue);
			ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

			if (amount > 1)
				BASE_SetItemAmount(item, amount - 1);

			else
				memset(item, 0, sizeof(STRUCT_ITEM));

			SendEmotion(conn, 14, 3);
			return;
		}

		return;
	}
#pragma endregion
#pragma region Extração Calça
	if (item->sIndex == 3023)
	{
		int extra = BASE_GetItemAbility(item, EF_ITEMLEVEL);

		if(pMob[conn].MOB.Equip[3].sIndex == 0)
		{
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		if(extra <= 5)
		{//Item abaixo de E
			int destextra = BASE_GetItemAbility(&pMob[conn].MOB.Equip[3], EF_ITEMLEVEL);
			int desttype = BASE_GetItemAbility(&pMob[conn].MOB.Equip[3], EF_MOBTYPE);

			if(destextra != 0 && destextra != 5 || destextra != extra || desttype != 2 && desttype != 0)
			{
				SendItem(conn, m->SourType, m->SourPos, item);
				return;
			}

			int sanc = BASE_GetItemSanc(&pMob[conn].MOB.Equip[3]);

			if(sanc != REF_11)
			{
				sprintf(temp, g_pMessageStringTable[_DN_SANCREQ], 11);
				SendClientMessage(conn, temp);
				SendItem(conn, m->SourType, m->SourPos, item);
				return;
			}

			pMob[conn].MOB.Equip[3].stEffect[1].cEffect = item->stEffect[1].cEffect;
			pMob[conn].MOB.Equip[3].stEffect[1].cValue = item->stEffect[1].cValue;

			pMob[conn].MOB.Equip[3].stEffect[2].cEffect = item->stEffect[2].cEffect;
			pMob[conn].MOB.Equip[3].stEffect[2].cValue = item->stEffect[2].cValue;

			SendItem(conn, ITEM_PLACE_EQUIP, 3, &pMob[conn].MOB.Equip[3]);

			sprintf(temp, "useitem,extracao %d-%d:%d:%d:%d:%d:%d", item->sIndex, item->stEffect[0].cEffect, item->stEffect[0].cValue, item->stEffect[1].cEffect, item->stEffect[1].cValue, item->stEffect[2].cEffect, item->stEffect[2].cValue);
			ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

			if (amount > 1)
				BASE_SetItemAmount(item, amount - 1);

			else
				memset(item, 0, sizeof(STRUCT_ITEM));

			SendEmotion(conn, 14, 3);
			return;
		}
		//Extração de item arch

		else 
		{
			int desttype = BASE_GetItemAbility(&pMob[conn].MOB.Equip[3], EF_MOBTYPE);

			if(desttype != 1)
			{
				SendItem(conn, m->SourType, m->SourPos, item);
				return;
			}

			int sanc = BASE_GetItemSanc(&pMob[conn].MOB.Equip[3]);

			if(sanc != REF_11)
			{
				sprintf(temp, g_pMessageStringTable[_DN_SANCREQ], 11);
				SendClientMessage(conn, temp);
				SendItem(conn, m->SourType, m->SourPos, item);
				return;
			}

			pMob[conn].MOB.Equip[3].stEffect[1].cEffect = item->stEffect[1].cEffect;
			pMob[conn].MOB.Equip[3].stEffect[1].cValue = item->stEffect[1].cValue;

			pMob[conn].MOB.Equip[3].stEffect[2].cEffect = item->stEffect[2].cEffect;
			pMob[conn].MOB.Equip[3].stEffect[2].cValue = item->stEffect[2].cValue;

			SendItem(conn, ITEM_PLACE_EQUIP, 3, &pMob[conn].MOB.Equip[3]);

			sprintf(temp, "useitem,extracao %d-%d:%d:%d:%d:%d:%d", item->sIndex, item->stEffect[0].cEffect, item->stEffect[0].cValue, item->stEffect[1].cEffect, item->stEffect[1].cValue, item->stEffect[2].cEffect, item->stEffect[2].cValue);
			ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

			if (amount > 1)
				BASE_SetItemAmount(item, amount - 1);

			else
				memset(item, 0, sizeof(STRUCT_ITEM));

			SendEmotion(conn, 14, 3);
			return;
		}

		return;
	}
#pragma endregion
#pragma region Extração Luva
	if (item->sIndex == 3024)
	{
		int extra = BASE_GetItemAbility(item, EF_ITEMLEVEL);

		if(pMob[conn].MOB.Equip[4].sIndex == 0)
		{
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		if(extra <= 5)
		{//Item abaixo de E
			int destextra = BASE_GetItemAbility(&pMob[conn].MOB.Equip[4], EF_ITEMLEVEL);
			int desttype = BASE_GetItemAbility(&pMob[conn].MOB.Equip[4], EF_MOBTYPE);

			if(destextra != 0 && destextra != 5 || destextra != extra || desttype != 2 && desttype != 0)
			{
				SendItem(conn, m->SourType, m->SourPos, item);
				return;
			}

			int sanc = BASE_GetItemSanc(&pMob[conn].MOB.Equip[4]);

			if(sanc != REF_11)
			{
				sprintf(temp, g_pMessageStringTable[_DN_SANCREQ], 11);
				SendClientMessage(conn, temp);
				SendItem(conn, m->SourType, m->SourPos, item);
				return;
			}

			pMob[conn].MOB.Equip[4].stEffect[1].cEffect = item->stEffect[1].cEffect;
			pMob[conn].MOB.Equip[4].stEffect[1].cValue = item->stEffect[1].cValue;

			pMob[conn].MOB.Equip[4].stEffect[2].cEffect = item->stEffect[2].cEffect;
			pMob[conn].MOB.Equip[4].stEffect[2].cValue = item->stEffect[2].cValue;

			SendItem(conn, ITEM_PLACE_EQUIP, 4, &pMob[conn].MOB.Equip[4]);

			sprintf(temp, "useitem,extracao %d-%d:%d:%d:%d:%d:%d", item->sIndex, item->stEffect[0].cEffect, item->stEffect[0].cValue, item->stEffect[1].cEffect, item->stEffect[1].cValue, item->stEffect[2].cEffect, item->stEffect[2].cValue);
			ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

			if (amount > 1)
				BASE_SetItemAmount(item, amount - 1);

			else
				memset(item, 0, sizeof(STRUCT_ITEM));

			SendEmotion(conn, 14, 3);
			return;
		}
		//Extração de item arch

		else 
		{
			int desttype = BASE_GetItemAbility(&pMob[conn].MOB.Equip[4], EF_MOBTYPE);

			if(desttype != 1)
			{
				SendItem(conn, m->SourType, m->SourPos, item);
				return;
			}

			int sanc = BASE_GetItemSanc(&pMob[conn].MOB.Equip[4]);

			if(sanc != REF_11)
			{
				sprintf(temp, g_pMessageStringTable[_DN_SANCREQ], 11);
				SendClientMessage(conn, temp);
				SendItem(conn, m->SourType, m->SourPos, item);
				return;
			}

			pMob[conn].MOB.Equip[4].stEffect[1].cEffect = item->stEffect[1].cEffect;
			pMob[conn].MOB.Equip[4].stEffect[1].cValue = item->stEffect[1].cValue;

			pMob[conn].MOB.Equip[4].stEffect[2].cEffect = item->stEffect[2].cEffect;
			pMob[conn].MOB.Equip[4].stEffect[2].cValue = item->stEffect[2].cValue;

			SendItem(conn, ITEM_PLACE_EQUIP, 4, &pMob[conn].MOB.Equip[4]);

			sprintf(temp, "useitem,extracao %d-%d:%d:%d:%d:%d:%d", item->sIndex, item->stEffect[0].cEffect, item->stEffect[0].cValue, item->stEffect[1].cEffect, item->stEffect[1].cValue, item->stEffect[2].cEffect, item->stEffect[2].cValue);
			ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

			if (amount > 1)
				BASE_SetItemAmount(item, amount - 1);

			else
				memset(item, 0, sizeof(STRUCT_ITEM));

			SendEmotion(conn, 14, 3);
			return;
		}

		return;
	}
#pragma endregion
#pragma region Extração Bota
	if (item->sIndex == 3025)
	{
		int extra = BASE_GetItemAbility(item, EF_ITEMLEVEL);

		if(pMob[conn].MOB.Equip[5].sIndex == 0)
		{
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		if(extra <= 5)
		{//Item abaixo de E
			int destextra = BASE_GetItemAbility(&pMob[conn].MOB.Equip[5], EF_ITEMLEVEL);
			int desttype = BASE_GetItemAbility(&pMob[conn].MOB.Equip[5], EF_MOBTYPE);

			if(destextra != 0 && destextra != 5 || destextra != extra || desttype != 2 && desttype != 0)
			{
				SendItem(conn, m->SourType, m->SourPos, item);
				return;
			}

			int sanc = BASE_GetItemSanc(&pMob[conn].MOB.Equip[5]);

			if(sanc != REF_11)
			{
				sprintf(temp, g_pMessageStringTable[_DN_SANCREQ], 11);
				SendClientMessage(conn, temp);
				SendItem(conn, m->SourType, m->SourPos, item);
				return;
			}

			pMob[conn].MOB.Equip[5].stEffect[1].cEffect = item->stEffect[1].cEffect;
			pMob[conn].MOB.Equip[5].stEffect[1].cValue = item->stEffect[1].cValue;

			pMob[conn].MOB.Equip[5].stEffect[2].cEffect = item->stEffect[2].cEffect;
			pMob[conn].MOB.Equip[5].stEffect[2].cValue = item->stEffect[2].cValue;

			if(pMob[conn].MOB.Equip[5].stEffect[1].cEffect == EF_DAMAGE && BASE_GetStaticItemAbility(&pMob[conn].MOB.Equip[5], EF_DAMAGE) != 0)
				pMob[conn].MOB.Equip[5].stEffect[1].cValue -= BASE_GetStaticItemAbility(&pMob[conn].MOB.Equip[5], EF_DAMAGE);

			if(pMob[conn].MOB.Equip[5].stEffect[2].cEffect == EF_DAMAGE && BASE_GetStaticItemAbility(&pMob[conn].MOB.Equip[5], EF_DAMAGE) != 0)
				pMob[conn].MOB.Equip[5].stEffect[2].cValue -= BASE_GetStaticItemAbility(&pMob[conn].MOB.Equip[5], EF_DAMAGE);

			SendItem(conn, ITEM_PLACE_EQUIP, 5, &pMob[conn].MOB.Equip[5]);

			sprintf(temp, "useitem,extracao %d-%d:%d:%d:%d:%d:%d", item->sIndex, item->stEffect[0].cEffect, item->stEffect[0].cValue, item->stEffect[1].cEffect, item->stEffect[1].cValue, item->stEffect[2].cEffect, item->stEffect[2].cValue);
			ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

			if (amount > 1)
				BASE_SetItemAmount(item, amount - 1);

			else
				memset(item, 0, sizeof(STRUCT_ITEM));

			SendEmotion(conn, 14, 3);
			return;
		}
		//Extração de item arch

		else 
		{
			int desttype = BASE_GetItemAbility(&pMob[conn].MOB.Equip[5], EF_MOBTYPE);

			if(desttype != 1)
			{
				SendItem(conn, m->SourType, m->SourPos, item);
				return;
			}

			int sanc = BASE_GetItemSanc(&pMob[conn].MOB.Equip[5]);

			if(sanc != REF_11)
			{
				sprintf(temp, g_pMessageStringTable[_DN_SANCREQ], 11);
				SendClientMessage(conn, temp);
				SendItem(conn, m->SourType, m->SourPos, item);
				return;
			}

			pMob[conn].MOB.Equip[5].stEffect[1].cEffect = item->stEffect[1].cEffect;
			pMob[conn].MOB.Equip[5].stEffect[1].cValue = item->stEffect[1].cValue;

			pMob[conn].MOB.Equip[5].stEffect[2].cEffect = item->stEffect[2].cEffect;
			pMob[conn].MOB.Equip[5].stEffect[2].cValue = item->stEffect[2].cValue;

			if(pMob[conn].MOB.Equip[5].stEffect[1].cEffect == EF_DAMAGE && BASE_GetStaticItemAbility(&pMob[conn].MOB.Equip[5], EF_DAMAGE) != 0)
				pMob[conn].MOB.Equip[5].stEffect[1].cValue -= BASE_GetStaticItemAbility(&pMob[conn].MOB.Equip[5], EF_DAMAGE);

			if(pMob[conn].MOB.Equip[5].stEffect[2].cEffect == EF_DAMAGE && BASE_GetStaticItemAbility(&pMob[conn].MOB.Equip[5], EF_DAMAGE) != 0)
				pMob[conn].MOB.Equip[5].stEffect[2].cValue -= BASE_GetStaticItemAbility(&pMob[conn].MOB.Equip[5], EF_DAMAGE);

			SendItem(conn, ITEM_PLACE_EQUIP, 5, &pMob[conn].MOB.Equip[5]);

			sprintf(temp, "useitem,extracao %d-%d:%d:%d:%d:%d:%d", item->sIndex, item->stEffect[0].cEffect, item->stEffect[0].cValue, item->stEffect[1].cEffect, item->stEffect[1].cValue, item->stEffect[2].cEffect, item->stEffect[2].cValue);
			ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

			if (amount > 1)
				BASE_SetItemAmount(item, amount - 1);

			else
				memset(item, 0, sizeof(STRUCT_ITEM));

			SendEmotion(conn, 14, 3);
			return;
		}

		return;
	}
#pragma endregion
#pragma region Extração Arma
	if (item->sIndex == 3026)
	{
		int extra = BASE_GetItemAbility(item, EF_ITEMLEVEL);

		if(pMob[conn].MOB.Equip[6].sIndex == 0)
		{
			SendItem(conn, m->SourType, m->SourPos, item);
			return;
		}

		if(extra <= 5)
		{//Item abaixo de E
			int destextra = BASE_GetItemAbility(&pMob[conn].MOB.Equip[6], EF_ITEMLEVEL);
			int desttype = BASE_GetItemAbility(&pMob[conn].MOB.Equip[6], EF_MOBTYPE);

			if(destextra != 0 && destextra != 5 || destextra != extra || desttype != 2 && desttype != 0)
			{
				SendItem(conn, m->SourType, m->SourPos, item);
				return;
			}

			int sanc = BASE_GetItemSanc(&pMob[conn].MOB.Equip[6]);

			if(sanc != REF_11)
			{
				sprintf(temp, g_pMessageStringTable[_DN_SANCREQ], 11);
				SendClientMessage(conn, temp);
				SendItem(conn, m->SourType, m->SourPos, item);
				return;
			}

			pMob[conn].MOB.Equip[6].stEffect[1].cEffect = item->stEffect[1].cEffect;
			pMob[conn].MOB.Equip[6].stEffect[1].cValue = item->stEffect[1].cValue;

			pMob[conn].MOB.Equip[6].stEffect[2].cEffect = item->stEffect[2].cEffect;
			pMob[conn].MOB.Equip[6].stEffect[2].cValue = item->stEffect[2].cValue;

			SendItem(conn, ITEM_PLACE_EQUIP, 6, &pMob[conn].MOB.Equip[6]);

			sprintf(temp, "useitem,extracao %d-%d:%d:%d:%d:%d:%d", item->sIndex, item->stEffect[0].cEffect, item->stEffect[0].cValue, item->stEffect[1].cEffect, item->stEffect[1].cValue, item->stEffect[2].cEffect, item->stEffect[2].cValue);
			ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

			if (amount > 1)
				BASE_SetItemAmount(item, amount - 1);

			else
				memset(item, 0, sizeof(STRUCT_ITEM));

			SendEmotion(conn, 14, 3);
			return;
		}
		//Extração de item arch

		else 
		{
			int desttype = BASE_GetItemAbility(&pMob[conn].MOB.Equip[6], EF_MOBTYPE);

			if(desttype != 1)
			{
				SendItem(conn, m->SourType, m->SourPos, item);
				return;
			}

			int sanc = BASE_GetItemSanc(&pMob[conn].MOB.Equip[6]);

			if(sanc != REF_11)
			{
				sprintf(temp, g_pMessageStringTable[_DN_SANCREQ], 11);
				SendClientMessage(conn, temp);
				SendItem(conn, m->SourType, m->SourPos, item);
				return;
			}

			pMob[conn].MOB.Equip[6].stEffect[1].cEffect = item->stEffect[1].cEffect;
			pMob[conn].MOB.Equip[6].stEffect[1].cValue = item->stEffect[1].cValue;

			pMob[conn].MOB.Equip[6].stEffect[2].cEffect = item->stEffect[2].cEffect;
			pMob[conn].MOB.Equip[6].stEffect[2].cValue = item->stEffect[2].cValue;

			SendItem(conn, ITEM_PLACE_EQUIP, 6, &pMob[conn].MOB.Equip[6]);

			sprintf(temp, "useitem,extracao %d-%d:%d:%d:%d:%d:%d", item->sIndex, item->stEffect[0].cEffect, item->stEffect[0].cValue, item->stEffect[1].cEffect, item->stEffect[1].cValue, item->stEffect[2].cEffect, item->stEffect[2].cValue);
			ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

			if (amount > 1)
				BASE_SetItemAmount(item, amount - 1);

			else
				memset(item, 0, sizeof(STRUCT_ITEM));

			SendEmotion(conn, 14, 3);
			return;
		}

		return;
	}
#pragma endregion
#pragma region Selo das Almas
	if(Vol == 206)
	{
		if(item->stEffect[0].cEffect == 0)
		{//Selo Vazio
			if(pMob[conn].extra.ClassMaster != SCELESTIAL)
			{
				SendClientMessage(conn, g_pMessageStringTable[_NN_GOD2CHARONLY_]);
				SendItem(conn, m->SourType, m->SourPos, item);
				return;
			}

			if (amount > 1)
				BASE_SetItemAmount(item, amount - 1);

			else
				memset(item, 0, sizeof(STRUCT_ITEM));

			CharLogOut(conn);

			MSG_STANDARDPARM sm_sa;
			memset(&sm_sa, 0, sizeof(MSG_STANDARDPARM));

			sm_sa.Size = sizeof(MSG_STANDARDPARM);
			sm_sa.Type = _MSG_DBPutInCapsule;
			sm_sa.ID = conn;

			sm_sa.Parm = pUser[conn].Slot;

			DBServerSocket.SendOneMessage((char*)&sm_sa, sizeof(MSG_STANDARDPARM));
		}

		return;
	}
#pragma endregion
#pragma region Bolsa do Andarilho
	if (item->sIndex == 3467)
	{
		if (pMob[conn].MOB.Carry[60].sIndex == 3467 && pMob[conn].MOB.Carry[61].sIndex == 3467)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_MAX_BAG]);
			return;
		}

		if (pMob[conn].MOB.Carry[60].sIndex != 3467)
		{
			pMob[conn].MOB.Carry[60].sIndex = 3467;
			BASE_SetItemDate(&pMob[conn].MOB.Carry[60], 30);
			SendCarry(conn);
		}
		else
		{
			pMob[conn].MOB.Carry[61].sIndex = 3467;
			BASE_SetItemDate(&pMob[conn].MOB.Carry[61], 30);
			SendCarry(conn);
		}

		SendScore(conn);
		SendEtc(conn);

		pMob[conn].MaxCarry = 30;

		if (pMob[conn].MOB.Carry[60].sIndex == 3467)
			pMob[conn].MaxCarry += 15;

		if (pMob[conn].MOB.Carry[61].sIndex == 3467)
			pMob[conn].MaxCarry += 15;

		if (amount > 1)
			BASE_SetItemAmount(item, amount - 1);

		else
			memset(item, 0, sizeof(STRUCT_ITEM));

		SendCarry(conn);

		return;
	}
#pragma endregion
#pragma region Donate
	if (Vol == 184)
	{
		int donate = BASE_GetItemAbility(item, EF_DONATE);

		pUser[conn].Donate += donate;

		SendClientMessage(conn, g_pMessageStringTable[_NN_Cash_ChargeOk]);

		if (amount > 1)
			BASE_SetItemAmount(item, amount - 1);

		else
			memset(item, 0, sizeof(STRUCT_ITEM));

		SendEmotion(conn, 14, 3);
		return;
	}
#pragma endregion
#pragma region Baú de Runas
	if (Vol == 210 && item->sIndex == 3219)
	{
		{
			int _rd = rand() % 100;
			if (_rd < 3) // Sol 1 Ansuz
			{
				Item.sIndex = 5110;
			}
			else if (_rd < 6) // Sol 2 Elhaz
			{
				Item.sIndex = 5124;
			}
			else if (_rd < 9) // Sol 3 Gebo
			{
				Item.sIndex = 5117;
			}
			else if (_rd < 12) // Sol 4 Mannaz
			{
				Item.sIndex = 5129;
			}
			else if (_rd < 15) // Sol 5 Raidho
			{
				Item.sIndex = 5114;
			}
			else if (_rd < 18) // Sol 6 Swoilo
			{
				Item.sIndex = 5125;
			}
			else if (_rd < 21) // Sol 7 Tiwaz
			{
				Item.sIndex = 5128;
			}

			else if (_rd < 24) // Terra 1 Dagaz
			{
				Item.sIndex = 5131;
			}
			else if (_rd < 27) // Terra 2 Fehu
			{
				Item.sIndex = 5113;
			}
			else if (_rd < 30) // Terra 3 Kenaz
			{
				Item.sIndex = 5115;
			}
			else if (_rd < 33) // Terra 4 Naudhiz
			{
				Item.sIndex = 5116;
			}
			else if (_rd < 36) // Terra 5 Swoilo
			{
				Item.sIndex = 5125;
			}
			else if (_rd < 39) // Terra 6 Thurisaz
			{
				Item.sIndex = 5112;
			}
			else if (_rd < 42) // Terra 7 Raidho
			{
				Item.sIndex = 5114;
			}

			else if (_rd < 45) // Água 1 Berkano
			{
				Item.sIndex = 5126;
			}
			else if (_rd < 48) // Água 2 Ehwaz
			{
				Item.sIndex = 5127;
			}
			else if (_rd < 51) // Água 3 Jara
			{
				Item.sIndex = 5121;
			}
			else if (_rd < 54) // Água 4 Raidho
			{
				Item.sIndex = 5114;
			}
			else if (_rd < 57) // Água 5 Swoilo
			{
				Item.sIndex = 5125;
			}
			else if (_rd < 60) // Água 6 Uraz
			{
				Item.sIndex = 5111;
			}
			else if (_rd < 63) // Água 7 Wunjo
			{
				Item.sIndex = 5118;
			}

			else if (_rd < 66) // Vento 1 Eihwaz
			{
				Item.sIndex = 5122;
			}
			else if (_rd < 69) // Vento 2 Hagalaz
			{
				Item.sIndex = 5119;
			}
			else if (_rd < 72) // Vento 3 Ing
			{
				Item.sIndex = 5132;
			}
			else if (_rd < 78) // Vento 4 Isa
			{
				Item.sIndex = 5120;
			}
			else if (_rd < 81) // Vento 5 Laguz
			{
				Item.sIndex = 5130;
			}
			else if (_rd < 84) // Vento 6 Othel
			{
				Item.sIndex = 5133;
			}
			else if (_rd < 87) // Vento 7 Perthro
			{
				Item.sIndex = 5123;
			}

			else if (_rd < 100)// Tenta de novo x3
			{
				Item.sIndex = 3219;
				Item.stEffect[0].cEffect = 61;
				Item.stEffect[0].cValue = 3;
			}
		}
		{
			if (amount > 1)
				BASE_SetItemAmount(item, amount - 1);

			else
				memset(item, 0, sizeof(STRUCT_ITEM));
		}

			SendCarry(conn);

			PutItem(conn, &Item);
			sprintf(temp, g_pMessageStringTable[_SN_Item_Arrived], g_pItemList[Item.sIndex].Name);
			SendClientMessage(conn, temp);

	}
#pragma endregion
#pragma region Coração doce
	if (Vol == 205)
	{
		{
			int sAffect = GetEmptyAffect(conn, 2); // Velocidade

			if (sAffect == -1 && pMob[conn].Affect[sAffect].Type == 2)
			{
				SendClientMessage(conn, g_pMessageStringTable[_NN_CantEatMore]);
				SendItem(conn, m->SourType, m->SourPos, item);
				return;
			}

			pMob[conn].Affect[sAffect].Type = 2; // Velocidade
			pMob[conn].Affect[sAffect].Value = 2; // Velocidade
			pMob[conn].Affect[sAffect].Time = AFFECT_1H / 5;

			if (pMob[conn].Affect[sAffect].Time >= 324000)
				pMob[conn].Affect[sAffect].Time = 324000;
		}

		{
			int sAffect = GetEmptyAffect(conn, 11); // Defesa

			if (sAffect == -1 && pMob[conn].Affect[sAffect].Type == 11)
			{
				SendClientMessage(conn, g_pMessageStringTable[_NN_CantEatMore]);
				SendItem(conn, m->SourType, m->SourPos, item);
				return;
			}

			pMob[conn].Affect[sAffect].Type = 11; // Defesa
			pMob[conn].Affect[sAffect].Time = AFFECT_1H / 5;

			if (pMob[conn].Affect[sAffect].Time >= 324000)
				pMob[conn].Affect[sAffect].Time = 324000;
		}

		pMob[conn].GetCurrentScore(conn);
		SendScore(conn);

		if (amount > 1)
			BASE_SetItemAmount(item, amount - 1);

		else
			memset(item, 0, sizeof(STRUCT_ITEM));

		sprintf(temp, "useitem,coração_doce");
		ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);
		return;
	}
#pragma endregion
#pragma region Chocolate do amor
	if (Vol == 204)
	{
		{
			int sAffect = GetEmptyAffect(conn, 9); // Dano

			if (sAffect == -1 && pMob[conn].Affect[sAffect].Type == 9)
			{
				SendClientMessage(conn, g_pMessageStringTable[_NN_CantEatMore]);
				SendItem(conn, m->SourType, m->SourPos, item);
				return;
			}

			pMob[conn].Affect[sAffect].Type = 9; // Dano
			pMob[conn].Affect[sAffect].Time = AFFECT_1H / 5;

			if (pMob[conn].Affect[sAffect].Time >= 324000)
				pMob[conn].Affect[sAffect].Time = 324000;
		}

		{
			int sAffect = GetEmptyAffect(conn, 15); // Skill

			if (sAffect == -1 && pMob[conn].Affect[sAffect].Type == 11)
			{
				SendClientMessage(conn, g_pMessageStringTable[_NN_CantEatMore]);
				SendItem(conn, m->SourType, m->SourPos, item);
				return;
			}

			pMob[conn].Affect[sAffect].Type = 15; // Skill
			pMob[conn].Affect[sAffect].Value = 55; // Skill
			pMob[conn].Affect[sAffect].Time = AFFECT_1H / 5;

			if (pMob[conn].Affect[sAffect].Time >= 324000)
				pMob[conn].Affect[sAffect].Time = 324000;
		}

		pMob[conn].GetCurrentScore(conn);
		SendScore(conn);

		if (amount > 1)
			BASE_SetItemAmount(item, amount - 1);

		else
			memset(item, 0, sizeof(STRUCT_ITEM));

		sprintf(temp, "useitem,coração_doce");
		ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);
		return;
	}
#pragma endregion
#pragma region Pergaminho do Vale Congelado
	if (Vol == 212 && item->sIndex == 4149)
	{
		DoTeleport(conn, 3895 + rand() % 3, 2878 + rand() % 3);
		ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

		if (amount > 1)
			BASE_SetItemAmount(item, amount - 1);

		else
			memset(item, 0, sizeof(STRUCT_ITEM));

		SendClientMessage(conn, "Bem vindo ao Vale Congelado");
		return;
	}
#pragma endregion
#pragma region Pergaminho do Vale do Deserto
	if (Vol == 212 && item->sIndex == 3221)
	{
		DoTeleport(conn, 1090 + rand() % 3, 1766 + rand() % 3);
		ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

		if (amount > 1)
			BASE_SetItemAmount(item, amount - 1);

		else
			memset(item, 0, sizeof(STRUCT_ITEM));

		SendClientMessage(conn, "Bem vindo ao Vale do Deserto");
		return;
	}
#pragma endregion
#pragma region Pergaminho da Fama da Guild
	if (Vol == 212 && item->sIndex == 3220)
	{
		if (pMob[conn].MOB.Guild == 0 || pMob[conn].MOB.GuildLevel != 9)
		{
			SendClientMessage(conn, "Uso permitido somente ao líder da Guild");
			return;
		}

		if (pMob[conn].MOB.GuildLevel == 9)
		{
			int Server = pMob[conn].MOB.Guild / MAX_GUILD;
			int Group = ServerGroup;
			int usGuild = pMob[conn].MOB.Guild;

			char guildname[256];

			BASE_GetGuildName(Group, usGuild, guildname);

			MSG_GuildInfo sm;

			sm.Type = _MSG_GuildInfo;
			sm.Size = sizeof(MSG_GuildInfo);
			sm.ID = 0;

			sm.Guild = usGuild;
			GuildInfo[usGuild].Fame += 100;

			sm.GuildInfo = GuildInfo[usGuild];

			DBServerSocket.SendOneMessage((char*)&sm, sizeof(MSG_GuildInfo));
			sprintf(temp, "Foi adicionado 100 de fama para sua Guild. ID [%d] Fama Atual [%d]", usGuild, GuildInfo[usGuild].Fame);
			SendClientMessage(conn, temp);
		}

		if (amount > 1)
			BASE_SetItemAmount(item, amount - 1);

		else
			memset(item, 0, sizeof(STRUCT_ITEM));

		return;
	}
#pragma endregion
#pragma region Cupons
	if (Vol == 210 && item->sIndex == 3210) // Cupom da Imortalidade
	{
		{
			int _rd = rand() % 100;

			if (_rd < 50) // Alma da Fênix
			{
				Item.sIndex = 1740;
				Item.stEffect[0].cEffect = 0;
				Item.stEffect[0].cValue = 0;
				Item.stEffect[1].cEffect = 0;
				Item.stEffect[1].cValue = 0;
				Item.stEffect[2].cEffect = 0;
				Item.stEffect[2].cValue = 0;
			}
			else if (_rd < 100) // Alma do Unicórnio
			{
				Item.sIndex = 1741;
				Item.stEffect[0].cEffect = 0;
				Item.stEffect[0].cValue = 0;
				Item.stEffect[1].cEffect = 0;
				Item.stEffect[1].cValue = 0;
				Item.stEffect[2].cEffect = 0;
				Item.stEffect[2].cValue = 0;
			}
		}
		{
			if (amount > 1)
				BASE_SetItemAmount(item, amount - 1);

			else
				memset(item, 0, sizeof(STRUCT_ITEM));
		}
			SendCarry(conn);

			PutItem(conn, &Item);
			sprintf(temp, g_pMessageStringTable[_SN_Item_Arrived], g_pItemList[Item.sIndex].Name);
			SendClientMessage(conn, temp);
	}

	if (Vol == 210 && item->sIndex == 3211) // Cupom da Sephirot
	{
		{
			int _rd = rand() % 100;
			if (_rd < 25) // Sephirot TK
			{
				Item.sIndex = 1760;
				Item.stEffect[0].cEffect = 0;
				Item.stEffect[0].cValue = 0;
				Item.stEffect[1].cEffect = 0;
				Item.stEffect[1].cValue = 0;
				Item.stEffect[2].cEffect = 0;
				Item.stEffect[2].cValue = 0;
			}
			else if (_rd < 50) // Sephirot FM
			{
				Item.sIndex = 1761;
				Item.stEffect[0].cEffect = 0;
				Item.stEffect[0].cValue = 0;
				Item.stEffect[1].cEffect = 0;
				Item.stEffect[1].cValue = 0;
				Item.stEffect[2].cEffect = 0;
				Item.stEffect[2].cValue = 0;
			}
			else if (_rd < 75) // Sephirot BM
			{
				Item.sIndex = 1762;
				Item.stEffect[0].cEffect = 0;
				Item.stEffect[0].cValue = 0;
				Item.stEffect[1].cEffect = 0;
				Item.stEffect[1].cValue = 0;
				Item.stEffect[2].cEffect = 0;
				Item.stEffect[2].cValue = 0;
			}
			else if (_rd < 100)// Sephirot HT
			{
				Item.sIndex = 1763;
				Item.stEffect[0].cEffect = 0;
				Item.stEffect[0].cValue = 0;
				Item.stEffect[1].cEffect = 0;
				Item.stEffect[1].cValue = 0;
				Item.stEffect[2].cEffect = 0;
				Item.stEffect[2].cValue = 0;
			}
		}
		{
		if (amount > 1)
			BASE_SetItemAmount(item, amount - 1);

		else
			memset(item, 0, sizeof(STRUCT_ITEM));
		}
			SendCarry(conn);

			PutItem(conn, &Item);
			sprintf(temp, g_pMessageStringTable[_SN_Item_Arrived], g_pItemList[Item.sIndex].Name);
			SendClientMessage(conn, temp);
	}

	if (Vol == 210 && item->sIndex == 3212) // Pedras Secretas
	{
		{
			int _rd = rand() % 100;
			if (_rd < 25) // Pedra Secreta da Água
			{
				Item.sIndex = 5334;
				Item.stEffect[0].cEffect = 0;
				Item.stEffect[0].cValue = 0;
				Item.stEffect[1].cEffect = 0;
				Item.stEffect[1].cValue = 0;
				Item.stEffect[2].cEffect = 0;
				Item.stEffect[2].cValue = 0;
			}
			else if (_rd < 50) // Pedra Secreta da Terra
			{
				Item.sIndex = 5335;
				Item.stEffect[0].cEffect = 0;
				Item.stEffect[0].cValue = 0;
				Item.stEffect[1].cEffect = 0;
				Item.stEffect[1].cValue = 0;
				Item.stEffect[2].cEffect = 0;
				Item.stEffect[2].cValue = 0;
			}
			else if (_rd < 75) // Pedra Secreta do Sol
			{
				Item.sIndex = 5336;
				Item.stEffect[0].cEffect = 0;
				Item.stEffect[0].cValue = 0;
				Item.stEffect[1].cEffect = 0;
				Item.stEffect[1].cValue = 0;
				Item.stEffect[2].cEffect = 0;
				Item.stEffect[2].cValue = 0;
			}
			else if (_rd < 100) // Pedra Secreta do Vento
			{
				Item.sIndex = 5337;
				Item.stEffect[0].cEffect = 0;
				Item.stEffect[0].cValue = 0;
				Item.stEffect[1].cEffect = 0;
				Item.stEffect[1].cValue = 0;
				Item.stEffect[2].cEffect = 0;
				Item.stEffect[2].cValue = 0;
			}
		}
		{
			if (amount > 1)
				BASE_SetItemAmount(item, amount - 1);

			else
				memset(item, 0, sizeof(STRUCT_ITEM));
		}
			SendCarry(conn);

			PutItem(conn, &Item);
			sprintf(temp, g_pMessageStringTable[_SN_Item_Arrived], g_pItemList[Item.sIndex].Name);
			SendClientMessage(conn, temp);
	}

	if (Vol == 210 && item->sIndex == 3213) // Cupom do Peito
	{
		{
			int _rd = rand() % 100;
			if (_rd < 25) // Flamejante
			{
				Item.sIndex = 1221;
				Item.stEffect[0].cEffect = 0;
				Item.stEffect[0].cValue = 0;
				Item.stEffect[1].cEffect = 0;
				Item.stEffect[1].cValue = 0;
				Item.stEffect[2].cEffect = 0;
				Item.stEffect[2].cValue = 0;
			}
			else if (_rd < 50) // Guardiã
			{
				Item.sIndex = 1356;
				Item.stEffect[0].cEffect = 0;
				Item.stEffect[0].cValue = 0;
				Item.stEffect[1].cEffect = 0;
				Item.stEffect[1].cValue = 0;
				Item.stEffect[2].cEffect = 0;
				Item.stEffect[2].cValue = 0;
			}
			else if (_rd < 75) // Destruição
			{
				Item.sIndex = 1506;
				Item.stEffect[0].cEffect = 0;
				Item.stEffect[0].cValue = 0;
				Item.stEffect[1].cEffect = 0;
				Item.stEffect[1].cValue = 0;
				Item.stEffect[2].cEffect = 0;
				Item.stEffect[2].cValue = 0;
			}
			else if (_rd < 100) // Rake
			{
				Item.sIndex = 1656;
				Item.stEffect[0].cEffect = 0;
				Item.stEffect[0].cValue = 0;
				Item.stEffect[1].cEffect = 0;
				Item.stEffect[1].cValue = 0;
				Item.stEffect[2].cEffect = 0;
				Item.stEffect[2].cValue = 0;
			}
		}
		{
			if (amount > 1)
				BASE_SetItemAmount(item, amount - 1);

			else
				memset(item, 0, sizeof(STRUCT_ITEM));
		}
			SendCarry(conn);

			PutItem(conn, &Item);
			sprintf(temp, g_pMessageStringTable[_SN_Item_Arrived], g_pItemList[Item.sIndex].Name);
			SendClientMessage(conn, temp);
	}

	if (Vol == 210 && item->sIndex == 3214) // Cupom da Calça
	{
		{
			int _rd = rand() % 100;
			if (_rd < 25) // Flamejante
			{
				Item.sIndex = 1222;
				Item.stEffect[0].cEffect = 0;
				Item.stEffect[0].cValue = 0;
				Item.stEffect[1].cEffect = 0;
				Item.stEffect[1].cValue = 0;
				Item.stEffect[2].cEffect = 0;
				Item.stEffect[2].cValue = 0;
			}
			else if (_rd < 50) // Guardiã
			{
				Item.sIndex = 1357;
				Item.stEffect[0].cEffect = 0;
				Item.stEffect[0].cValue = 0;
				Item.stEffect[1].cEffect = 0;
				Item.stEffect[1].cValue = 0;
				Item.stEffect[2].cEffect = 0;
				Item.stEffect[2].cValue = 0;
			}
			else if (_rd < 75) // Destruição
			{
				Item.sIndex = 1507;
				Item.stEffect[0].cEffect = 0;
				Item.stEffect[0].cValue = 0;
				Item.stEffect[1].cEffect = 0;
				Item.stEffect[1].cValue = 0;
				Item.stEffect[2].cEffect = 0;
				Item.stEffect[2].cValue = 0;
			}
			else if (_rd < 100) // Rake
			{
				Item.sIndex = 1657;
				Item.stEffect[0].cEffect = 0;
				Item.stEffect[0].cValue = 0;
				Item.stEffect[1].cEffect = 0;
				Item.stEffect[1].cValue = 0;
				Item.stEffect[2].cEffect = 0;
				Item.stEffect[2].cValue = 0;
			}
		}
		{
			if (amount > 1)
				BASE_SetItemAmount(item, amount - 1);

			else
				memset(item, 0, sizeof(STRUCT_ITEM));
		}
			SendCarry(conn);

			PutItem(conn, &Item);
			sprintf(temp, g_pMessageStringTable[_SN_Item_Arrived], g_pItemList[Item.sIndex].Name);
			SendClientMessage(conn, temp);
	}

	if (Vol == 210 && item->sIndex == 3215) // Cupom da Luva
	{
		{
			int _rd = rand() % 100;
			if (_rd < 25) // Flamejante
			{
				Item.sIndex = 1223;
				Item.stEffect[0].cEffect = 0;
				Item.stEffect[0].cValue = 0;
				Item.stEffect[1].cEffect = 0;
				Item.stEffect[1].cValue = 0;
				Item.stEffect[2].cEffect = 0;
				Item.stEffect[2].cValue = 0;
			}
			else if (_rd < 50) // Guardiã
			{
				Item.sIndex = 1358;
				Item.stEffect[0].cEffect = 0;
				Item.stEffect[0].cValue = 0;
				Item.stEffect[1].cEffect = 0;
				Item.stEffect[1].cValue = 0;
				Item.stEffect[2].cEffect = 0;
				Item.stEffect[2].cValue = 0;
			}
			else if (_rd < 75) // Destruição
			{
				Item.sIndex = 1508;
				Item.stEffect[0].cEffect = 0;
				Item.stEffect[0].cValue = 0;
				Item.stEffect[1].cEffect = 0;
				Item.stEffect[1].cValue = 0;
				Item.stEffect[2].cEffect = 0;
				Item.stEffect[2].cValue = 0;
			}
			else if (_rd < 100) // Rake
			{
				Item.sIndex = 1658;
				Item.stEffect[0].cEffect = 0;
				Item.stEffect[0].cValue = 0;
				Item.stEffect[1].cEffect = 0;
				Item.stEffect[1].cValue = 0;
				Item.stEffect[2].cEffect = 0;
				Item.stEffect[2].cValue = 0;
			}
		}
		{
			if (amount > 1)
				BASE_SetItemAmount(item, amount - 1);

			else
				memset(item, 0, sizeof(STRUCT_ITEM));
		}
			SendCarry(conn);

			PutItem(conn, &Item);
			sprintf(temp, g_pMessageStringTable[_SN_Item_Arrived], g_pItemList[Item.sIndex].Name);
			SendClientMessage(conn, temp);
	}

	if (Vol == 210 && item->sIndex == 3216) // Cupom da Bota
	{
		{
			int _rd = rand() % 100;
			if (_rd < 25) // Flamejante
			{
				Item.sIndex = 1224;
				Item.stEffect[0].cEffect = 0;
				Item.stEffect[0].cValue = 0;
				Item.stEffect[1].cEffect = 0;
				Item.stEffect[1].cValue = 0;
				Item.stEffect[2].cEffect = 0;
				Item.stEffect[2].cValue = 0;
			}
			else if (_rd < 50) // Guardiã
			{
				Item.sIndex = 1359;
				Item.stEffect[0].cEffect = 0;
				Item.stEffect[0].cValue = 0;
				Item.stEffect[1].cEffect = 0;
				Item.stEffect[1].cValue = 0;
				Item.stEffect[2].cEffect = 0;
				Item.stEffect[2].cValue = 0;
			}
			else if (_rd < 75) // Destruição
			{
				Item.sIndex = 1509;
				Item.stEffect[0].cEffect = 0;
				Item.stEffect[0].cValue = 0;
				Item.stEffect[1].cEffect = 0;
				Item.stEffect[1].cValue = 0;
				Item.stEffect[2].cEffect = 0;
				Item.stEffect[2].cValue = 0;
			}
			else if (_rd < 100) // Rake
			{
				Item.sIndex = 1659;
				Item.stEffect[0].cEffect = 0;
				Item.stEffect[0].cValue = 0;
				Item.stEffect[1].cEffect = 0;
				Item.stEffect[1].cValue = 0;
				Item.stEffect[2].cEffect = 0;
				Item.stEffect[2].cValue = 0;
			}
		}
		{
			if (amount > 1)
				BASE_SetItemAmount(item, amount - 1);

			else
				memset(item, 0, sizeof(STRUCT_ITEM));
		}
			SendCarry(conn);

			PutItem(conn, &Item);
			sprintf(temp, g_pMessageStringTable[_SN_Item_Arrived], g_pItemList[Item.sIndex].Name);
			SendClientMessage(conn, temp);
	}

	if (Vol == 210 && item->sIndex == 3217) // Cupom da Montaria
	{
		{
			int _rd = rand() % 100;
			if (_rd < 50) // Slepnir
			{
				Item.sIndex = 2388;
				Item.stEffect[0].cEffect = 28;
				Item.stEffect[0].cValue = 104;
				Item.stEffect[1].cEffect = 1;
				Item.stEffect[1].cValue = 20;
				Item.stEffect[2].cEffect = 100;
				Item.stEffect[2].cValue = 1;
			}
			else if (_rd < 100) // Svaldfire
			{
				Item.sIndex = 2387;
				Item.stEffect[0].cEffect = 28;
				Item.stEffect[0].cValue = 104;
				Item.stEffect[1].cEffect = 1;
				Item.stEffect[1].cValue = 20;
				Item.stEffect[2].cEffect = 100;
				Item.stEffect[2].cValue = 1;
			}
		}
		{
			if (amount > 1)
				BASE_SetItemAmount(item, amount - 1);

			else
				memset(item, 0, sizeof(STRUCT_ITEM));
		}
			SendCarry(conn);

			PutItem(conn, &Item);
			sprintf(temp, g_pMessageStringTable[_SN_Item_Arrived], g_pItemList[Item.sIndex].Name);
			SendClientMessage(conn, temp);
	}

	if (Vol == 210 && item->sIndex == 3218) // Cupom do Âmago
	{
		{
			int _rd = rand() % 100;

			if (_rd < 15) // Âmago de Unicórnio 15%
			{
				Item.sIndex = 2400;
				Item.stEffect[0].cEffect = 61;
				Item.stEffect[0].cValue = 20;
			}
			else if (_rd < 30) // Âmago de Andaluz N 15%
			{
				Item.sIndex = 2411;
				Item.stEffect[0].cEffect = 61;
				Item.stEffect[0].cValue = 20;
			}
			else if (_rd < 65) // Âmago de Andaluz N 30%
			{
				Item.sIndex = 2400;
				Item.stEffect[0].cEffect = 61;
				Item.stEffect[0].cValue = 10;
			}
			else if (_rd < 100) // Âmago de Unicórnio 30%
			{
				Item.sIndex = 2411;
				Item.stEffect[0].cEffect = 61;
				Item.stEffect[0].cValue = 10;
			}
		}
		{
			if (amount > 1)
				BASE_SetItemAmount(item, amount - 1);

			else
				memset(item, 0, sizeof(STRUCT_ITEM));
		}
			SendCarry(conn);

			PutItem(conn, &Item);
			sprintf(temp, g_pMessageStringTable[_SN_Item_Arrived], g_pItemList[Item.sIndex].Name);
			SendClientMessage(conn, temp);
	}
#pragma endregion
}