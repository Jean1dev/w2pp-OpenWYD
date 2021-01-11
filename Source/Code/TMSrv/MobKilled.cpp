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
#include <stdio.h>
#include <fcntl.h>
#include <time.h>
#include <math.h>
#include <io.h>
#include <errno.h>

#include "..\Basedef.h"
#include "..\CPSock.h"
#include "..\ItemEffect.h"
#include "Language.h"

#include "CItem.h"
#include "Server.h"
#include "GetFunc.h"
#include "SendFunc.h"
#include "ProcessClientMessage.h"
#include "ProcessDBMessage.h"
#include "CCastleZakum.h"
#include "CWarTower.h"

void MobKilled(int target, int conn, int PosX, int PosY)
{
	if (conn <= 0 || conn >= MAX_MOB || target <= 0 || target >= MAX_MOB || pMob[target].Mode == USER_EMPTY)
		return;

	STRUCT_ITEM *FairySlot = &pMob[target].MOB.Equip[13];

	MSG_CreateMob sCreateMob;

	if (pMob[target].MOB.CurrentScore.Level >= 1000)
	{
		pMob[target].MOB.CurrentScore.Hp = pMob[target].MOB.CurrentScore.MaxHp;

		if (target < MAX_USER)
			SetReqHp(target);

		GetCreateMob(target, &sCreateMob);
		GridMulticast(pMob[target].TargetX, pMob[target].TargetY, (MSG_STANDARD*)&sCreateMob, 0);

		SendScore(conn);

		return;
	}

	int Face = pMob[conn].MOB.Equip[0].sIndex;

#pragma region Nyerds
	if (pMob[target].MOB.Equip[13].sIndex == 769 && RvRState == 0 && GTorreState == 0 && CastleState == 0)
	{
		int sanc = BASE_GetItemSanc(FairySlot);

		if (sanc > 0)
		{
			sanc--;

			if (FairySlot->stEffect[0].cEffect == 43)
				FairySlot->stEffect[0].cValue = sanc;

			else if (FairySlot->stEffect[1].cEffect == 43)
				FairySlot->stEffect[1].cValue = sanc;

			else if (FairySlot->stEffect[2].cEffect == 43)
				FairySlot->stEffect[2].cValue = sanc;
		}
		else
			BASE_ClearItem(FairySlot);

		if (target > 0 && target <= MAX_USER)
		{
			SendItem(target, ITEM_PLACE_EQUIP, 13, &pMob[target].MOB.Equip[13]);
			SendEmotion(target, 14, 2);
		}

		SendEquip(target, 0);

		pMob[target].MOB.CurrentScore.Hp = pMob[target].MOB.CurrentScore.MaxHp;
		
		if (target < MAX_USER)
			SetReqHp(target);

		GetCreateMob(target, &sCreateMob);
		GridMulticast(pMob[target].TargetX, pMob[target].TargetY, (MSG_STANDARD*)&sCreateMob, 0);

		SendScore(conn);

		return;
	}
	if ((pMob[target].MOB.Equip[13].sIndex == 769) && RvRState == 1 || (pMob[target].MOB.Equip[13].sIndex == 769) && GTorreState == 1 || (pMob[target].MOB.Equip[13].sIndex == 769) && CastleState == 1)
	{
		if (target < MAX_USER)
		SetReqHp(target);
		GetCreateMob(target, &sCreateMob);
		GridMulticast(pMob[target].TargetX, pMob[target].TargetY, (MSG_STANDARD*)&sCreateMob, 0);
		SendScore(conn);
		return;
	}
#pragma endregion
#pragma region Morte nas quests
	//Coveiro
	if (pMob[conn].TargetX > 2379 && pMob[conn].TargetY > 2076 && pMob[conn].TargetX < 2426 && pMob[conn].TargetY < 2133)
		pMob[conn].extra.CheckTimeKersef = 0;

	//Jardin
	if (pMob[conn].TargetX > 2228 && pMob[conn].TargetY > 1700 && pMob[conn].TargetX < 2257 && pMob[conn].TargetY < 1728)
		pMob[conn].extra.CheckTimeKersef = 0;

	//Kaizen
	if (pMob[conn].TargetX > 459 && pMob[conn].TargetY > 3887 && pMob[conn].TargetX < 497 && pMob[conn].TargetY < 3916)
		pMob[conn].extra.CheckTimeKersef = 0;

	//Hidra
	if (pMob[conn].TargetX > 658 && pMob[conn].TargetY > 3728 && pMob[conn].TargetX < 703 && pMob[conn].TargetY < 3762)
		pMob[conn].extra.CheckTimeKersef = 0;

	//Elfos
	if (pMob[conn].TargetX > 1312 && pMob[conn].TargetY > 4027 && pMob[conn].TargetX < 1348 && pMob[conn].TargetY < 4055)
		pMob[conn].extra.CheckTimeKersef = 0;
#pragma endregion
#pragma region Pergaminho da Ressurreição
	for (int i = 0; i < pMob[target].MaxCarry; i++)
	{
		if (pMob[target].MOB.Carry[i].sIndex == 3463)
		{
			int amount = BASE_GetItemAmount(&pMob[target].MOB.Carry[i]);

			if (amount > 1)
				BASE_SetItemAmount(&pMob[target].MOB.Carry[i], amount - 1);
			else
				BASE_ClearItem(&pMob[target].MOB.Carry[i]);

			SendItem(target, ITEM_PLACE_CARRY, i, &pMob[target].MOB.Carry[i]);

			pMob[target].MOB.CurrentScore.Hp = pMob[target].MOB.CurrentScore.MaxHp;
			pMob[target].MOB.CurrentScore.Mp = pMob[target].MOB.CurrentScore.MaxMp;

			pMob[target].GetCurrentScore(target);
			SetReqHp(target);
			SendScore(target);
			SendCreateMob(target, target, 1);
			return;
		}
	}
#pragma endregion
#pragma region Morte na Guerra de Noatum
	if (CastleState > 1 && pMob[target].TargetX == 1046 && pMob[target].TargetY == 1690 && target > 0 && target < MAX_USER)
	{
		DoTeleport(target, 1066, 1717);
		return;
	}
#pragma endregion
#pragma region Morte na Guerra RvR
	if (pMob[target].GenerateIndex == TORRE_RVR_BLUE || pMob[target].GenerateIndex == TORRE_RVR_RED)
	{
		if (conn == 29999)
		{
			DeleteMob(target, 1);
			return;
		}

		int connId = conn;
		if (connId > MAX_USER)
		{
			int summonerId = pMob[connId].Summoner;

			if (summonerId > 0 && summonerId < MAX_USER)
				connId = summonerId;
			else
				connId = 0;
		}

		if (connId <= MAX_USER && pUser[connId].Mode != USER_PLAY)
			connId = 0;

		// Ressuscita a torre
		pMob[target].MOB.CurrentScore.Hp = pMob[target].MOB.CurrentScore.MaxHp;

		MSG_CreateMob packet;
		GetCreateMob(target, &packet);

		GridMulticast(pMob[target].TargetX, pMob[target].TargetY, (MSG_STANDARD*)&packet, 0);

		int towerId = 0;

		if (pMob[target].GenerateIndex == TORRE_RVR_BLUE)
			towerId = 1;
		else
			towerId = 0;

		g_pRvrWar.Points[towerId] += 25;

		for (INT32 i = 1; i < MAX_USER; i++)
		{
			if (pUser[i].Mode != USER_PLAY)
				continue;

			if (pMob[i].TargetX >= AREA_RVR_MINX && pMob[i].TargetX <= AREA_RVR_MAXX && pMob[i].TargetY >= AREA_RVR_MINY && pMob[i].TargetY <= AREA_RVR_MAXY)
				SendClientMessage(i, "A torre do reino foi destruída");// se quiser pode fala qual da seguinte forma:  , (pMob[i].GenerateIndex == TORRE_RVR_BLUE) ? "Blue" : "Red", (towerId == 0) ? "Blue" : "Red");


			// RVR
			if (pMob[i].TargetX >= AREA_RVR_MINX && pMob[i].TargetX <= AREA_RVR_MAXX &&
				pMob[i].TargetY >= AREA_RVR_MINY && pMob[i].TargetY <= AREA_RVR_MAXY && g_pRvrWar.Status == 1)
			{
				int cape = pMob[i].MOB.Clan;

				INT32 posX = 0;
				INT32 posY = 0;

				// Adicionar 
				if (cape == 7)
				{
					posX = 1754 + rand() % 4;
					posY = 1971 + rand() % 20;
				}
				else if (cape == 8)
				{
					posX = 1711 + rand() % 4;
					posY = 1971 + rand() % 20;
				}

				if (posX != 0 && posY != 0)
					DoTeleport(i, posX, posY);
			}
		}

		for (int i = 1; i < MAX_USER; i++)
		{
			if (pMob[i].TargetX >= AREA_RVR_MINX && pMob[i].TargetX <= AREA_RVR_MAXX &&
				pMob[i].TargetY >= AREA_RVR_MINY && pMob[i].TargetY <= AREA_RVR_MAXY)
			{
				if (pUser[i].Mode != USER_PLAY)
					continue;

				// Atualiza o placar de kill
				SendCounterMob(i, g_pRvrWar.Points[0], g_pRvrWar.Points[1]);
			}
		}

		return;
	}

#pragma endregion
#pragma region Crias EXP
	if (conn >= MAX_USER && pMob[conn].MOB.Clan == 4 && Face >= 315 && Face <= 345 && target > MAX_USER && pMob[target].MOB.Clan != 4)
	{
		int summoner = pMob[conn].Summoner;

		if (summoner > 0 && summoner < MAX_USER && pUser[summoner].Mode && pMob[summoner].Mode)
		{
			STRUCT_ITEM *Mount = &pMob[summoner].MOB.Equip[14];

			if (pMob[summoner].MOB.Equip[14].sIndex >= 2330 && pMob[summoner].MOB.Equip[14].sIndex < 2390)
			{
				unsigned char XP = Mount->stEffect[1].cEffect;
				unsigned char Crescimento = Mount->stEffect[2].cValue;

				unsigned char exp = XP + 100;

				if (Mount->sIndex == 2330)
					exp = XP + 25;

				else if (Mount->sIndex == 2331)
					exp = XP + 35;

				else if (Mount->sIndex == 2332)
					exp = XP + 45;

				else if (Mount->sIndex == 2333)
					exp = XP + 55;

				else if (Mount->sIndex == 2334)
					exp = XP + 65;

				else if (Mount->sIndex == 2335)
					exp = XP + 75;

				if (XP < pMob[target].MOB.CurrentScore.Level && XP < 100)
				{
					Crescimento = Crescimento + 1;

					if (Crescimento < exp)
					{
						Mount->stEffect[2].cValue = Crescimento;
						SendItem(summoner, ITEM_PLACE_EQUIP, 14, &pMob[summoner].MOB.Equip[14]);
					}
					else
					{
						Crescimento = 1;
						XP = XP + 1;
						Mount->stEffect[2].cValue = 1;
						Mount->stEffect[1].cEffect = XP;
						SendClientMessage(summoner, g_pMessageStringTable[_NN_Mount_Level]);
						SendItem(summoner, ITEM_PLACE_EQUIP, 14, &pMob[summoner].MOB.Equip[14]);
						MountProcess(summoner, 0);
					}
				}
			}
		}
	}

#pragma endregion
#pragma region Invocações
	MSG_CNFMobKill sm;
	memset(&sm, 0, sizeof(MSG_CNFMobKill));

	sm.Type = _MSG_CNFMobKill;
	sm.Size = sizeof(MSG_CNFMobKill);
	sm.ID = ESCENE_FIELD;

	sm.KilledMob = target;
	sm.Killer = conn;

	pMob[target].MOB.CurrentScore.Hp = 0;

	int Leader = pMob[conn].Leader;

	if (Leader == 0)
		Leader = conn;

	if (conn >= MAX_USER && pMob[conn].MOB.Clan == 4)
	{
		int Summoner = pMob[conn].Summoner;

		if (Summoner <= 0 || Summoner >= MAX_USER || pUser[Summoner].Mode != USER_PLAY)
		{
			GridMulticast(pMob[target].TargetX, pMob[target].TargetY, (MSG_STANDARD*)&sm, 0);

			if (target > MAX_USER)
				DeleteMob(target, 1);

			return;
		}

		conn = Summoner;
	}
#pragma endregion
#pragma region PLACAR RVR
	if (target < MAX_USER && conn < MAX_USER)
	{
		if (pMob[target].TargetX >= AREA_RVR_MINX && pMob[target].TargetX <= AREA_RVR_MAXX &&
			pMob[target].TargetY >= AREA_RVR_MINY && pMob[target].TargetY <= AREA_RVR_MAXY && g_pRvrWar.Status == 1)
		{
			int capeKiller = pMob[conn].MOB.Clan;
			int capeDead = pMob[target].MOB.Clan;

			if ((capeKiller == 8 || capeKiller == 7) &&
				(capeDead == 8 || capeDead == 7))
			{
				if (capeDead != capeKiller)
				{
					int capeId = capeKiller - 7;

					g_pRvrWar.Points[capeId]++;

					for (INT32 i = 1; i < MAX_USER; i++)
					{
						if (pUser[i].Mode != USER_PLAY)
							continue;

						if (pMob[i].TargetX >= AREA_RVR_MINX && pMob[i].TargetX <= AREA_RVR_MAXX &&
							pMob[i].TargetY >= AREA_RVR_MINY && pMob[i].TargetY <= AREA_RVR_MINY)
						{
							char tmp[128];
							sprintf(tmp, "%s matou %s e pontuou para o reino %s", pMob[conn].MOB.MobName, pMob[target].MOB.MobName, (capeId == 0) ? "Blue" : "Red");
							SendClientMessage(i, tmp);

							// Atualiza o placar de kill
							SendCounterMob(i, g_pRvrWar.Points[0], g_pRvrWar.Points[1]);
						}
					}

				}
			}
		}
	}
#pragma endregion
#pragma region PvE
	if (target >= MAX_USER || pMob[target].MOB.BaseScore.Level > MAX_LEVEL + 5)
	{
		if (target >= MAX_USER || pMob[target].MOB.BaseScore.Level <= MAX_LEVEL)
		{
			if (conn < MAX_USER && pMob[target].MOB.Clan != 4)
			{
#pragma region Distribuição da EXP
				int MobExp = GetExpApply(pMob[conn].extra, (int)pMob[target].MOB.Exp, pMob[conn].MOB.CurrentScore.Level, pMob[target].MOB.CurrentScore.Level);
				int FinalExp = 0;

				// TODO: Change this UNK_s to something else.
				int UNK_1 = 30;
				int UNK_2 = 0;
				int UNK_3 = pMob[conn].extra.ClassMaster;

				int tx = pMob[conn].TargetX;
				int ty = pMob[conn].TargetY;

				int party = 0;


				if (UNK_3> 0 && UNK_3 <= MAX_PARTY)
				{
					int NumMob = g_EmptyMob + UNK_3;

					if (UNK_3 > 1)
						NumMob = NumMob + PARTYBONUS - 100;

					int eMob = MobExp;
					int isExp = NumMob * MobExp / 100;

					struct tm when;
					time_t now;
					time(&now);
					when = *localtime(&now);

					for (int i = 0; i < MAX_PARTY + 1; ++i)
					{
						party = 0;

						if (Leader && i < MAX_PARTY)
							party = pMob[Leader].PartyList[i];
						else
							party = Leader;

#pragma region Pesadelo Arcano
						if (party > 0 && party < MAX_USER && pMob[party].MOB.CurrentScore.Hp > 0 && (tx / 128) == 9 && (pMob[party].TargetX / 128) == 9
							&& (ty / 128) == 1 && (pMob[party].TargetY / 128) == 1)
						{
							isExp = GetExpApply(pMob[party].extra, (int)pMob[target].MOB.Exp, pMob[party].MOB.CurrentScore.Level, pMob[target].MOB.CurrentScore.Level);

							int myLevel = pMob[party].MOB.CurrentScore.Level;

							if (pMob[party].extra.ClassMaster != MORTAL && pMob[party].extra.ClassMaster != ARCH)
								myLevel += MAX_LEVEL + 1;

							int exp = (UNK_1 + myLevel) * isExp / (UNK_1 + myLevel);
							if (exp > 0 && exp <= 10000000)
							{
								if (pMob[party].extra.ClassMaster == MORTAL)
								{
									if (myLevel <= 200) // 150k
										exp /= 1;

									else if (myLevel <= 300) // 120k
										exp /= 0.84f;

									else if (myLevel <= 356) // 80k
										exp /= 1.05f;

									else if (myLevel <= 370) // 50k
										exp /= 1.63f;

									else if (myLevel <= 380) // 40k
										exp /= 1.95f;

									else if (myLevel <= 390) // 30k
										exp /= 2.55f;

									else if (myLevel <= 399) // 20k
										exp /= 3.70f;
								}

								if (pMob[party].extra.ClassMaster == ARCH)
								{
									if (myLevel <= 200) // Exp Média 90k
										exp /= 0.84f;

									else if (myLevel <= 300)// Exp Média 70k
										exp /= 0.72f;

									else if (myLevel <= 356)// Exp Média 30k
										exp /= 1.40f;

									else if (myLevel <= 360)// Exp Média 8k
										exp /= 4.75f;

									else if (myLevel <= 370)// Exp Média 6k
										exp /= 6.60f;

									else if (myLevel <= 380) // Exp Média 2.5k
										exp /= 15;

									else if (myLevel <= 390)  // Exp Média 1.8k
										exp /= 21;

									else if (myLevel <= 400) // Exp Média 1k
										exp /= 35;
								}

								if (pMob[party].extra.ClassMaster != MORTAL && pMob[party].extra.ClassMaster != ARCH)
								{
									if (myLevel < 120)
										exp /= 10;

									else if (myLevel < 150)
										exp /= 20;

									else if (myLevel < 170)
										exp /= 40;

									else if (myLevel < 180)
										exp /= 80;

									else if (myLevel < 190)
										exp /= 160;

									else
										exp /= 320;
								}

								exp = 6 * exp / 10;

								//if (exp > eMob)
								//	exp = eMob;

								if (pMob[conn].ExpBonus > 0 && pMob[conn].ExpBonus < 500)
									exp += exp * pMob[conn].ExpBonus / 100;

								if (NewbieEventServer && pMob[party].MOB.CurrentScore.Level < 100 && pMob[party].extra.ClassMaster != CELESTIAL && pMob[party].extra.ClassMaster != CELESTIALCS && pMob[party].extra.ClassMaster != SCELESTIAL)
									exp += exp / 4;

								if (DOUBLEMODE)
									exp *= 2;

								if (KefraLive == 0)
									exp /= 2;

								if (NewbieEventServer)
									exp += (exp * 15) / 100;
								else
									exp -= (exp * 15) / 100;

								//Log de Experiência diário
								if (when.tm_yday != pMob[party].extra.DayLog.YearDay)
									pMob[party].extra.DayLog.Exp = 0;

								pMob[party].extra.DayLog.YearDay = when.tm_yday;
								pMob[party].extra.DayLog.Exp += exp;

								if (pMob[party].extra.Hold > 0 && (unsigned int)exp >= pMob[party].extra.Hold)
								{
									int nhold = pMob[party].extra.Hold - exp;

									if (nhold < 0)
										nhold = 0;

									exp -= pMob[party].extra.Hold;
									pMob[party].extra.Hold = nhold;
								}

								else if (pMob[party].extra.Hold > 0 && (unsigned int)exp < pMob[party].extra.Hold)
								{
									pMob[party].extra.Hold -= exp;
									continue;
								}

								if (pMob[party].MOB.Exp + exp > g_pNextLevel[MAX_LEVEL + 1])
								{
									pMob[party].MOB.Exp = g_pNextLevel[MAX_LEVEL + 1];
								}
								else
								{
									pMob[party].MOB.Exp += exp;
								}

								//RankingUpdateExp
								STRUCT_RANKING rankInfo = STRUCT_RANKING(pMob[party].MOB.MobName, pMob[party].MOB.Exp, pMob[party].extra.ClassMaster, pMob[party].MOB.CurrentScore.Level, pMob[party].MOB.Class, pMob[party].MOB.Guild, pMob[party].MOB.Clan);

								MSG_UpdateExpRanking expRankingPacket = MSG_UpdateExpRanking(party, rankInfo);
								DBServerSocket.SendOneMessage((char*)&expRankingPacket, sizeof(expRankingPacket));
							}
						}
#pragma endregion
#pragma region Pesadelo Mistico
						else if (party > 0 && party < MAX_USER && pMob[party].MOB.CurrentScore.Hp > 0 && (tx / 128) == 8 && (pMob[party].TargetX / 128) == 8 && (ty / 128) == 2 && (pMob[party].TargetY / 128) == 2)
						{
							isExp = GetExpApply(pMob[party].extra, (int)pMob[target].MOB.Exp, pMob[party].MOB.CurrentScore.Level, pMob[target].MOB.CurrentScore.Level);

							int myLevel = pMob[party].MOB.CurrentScore.Level;

							if (pMob[party].extra.ClassMaster != MORTAL && pMob[party].extra.ClassMaster != ARCH)
								myLevel += MAX_LEVEL + 1;

							int exp = (UNK_1 + myLevel) * isExp / (UNK_1 + myLevel);
							if (exp > 0 && exp <= 10000000)
							{
								if (pMob[party].extra.ClassMaster == MORTAL)
								{
									if (myLevel <= 200) // 110k
										exp /= 1;

									else if (myLevel <= 300) // 70k
										exp /= 1.03f;

									else if (myLevel <= 356) // 40k
										exp /= 1.50f;

									else if (myLevel <= 370) // 27k
										exp /= 2.15f;

									else if (myLevel <= 380) // 20k
										exp /= 2.78f;

									else if (myLevel <= 390) // 15k
										exp /= 3.70f;

									else if (myLevel <= 399) // 10k
										exp /= 5.20f;
								}

								if (pMob[party].extra.ClassMaster == ARCH)
								{
									if (myLevel <= 200) // Exp Média 70k
										exp /= 0.84f;

									else if (myLevel <= 300)// Exp Média 40k
										exp /= 0.72f;

									else if (myLevel <= 356)// Exp Média 25k
										exp /= 1.40f;

									else if (myLevel <= 360)// Exp Média 5k
										exp /= 4.75f;

									else if (myLevel <= 370)// Exp Média 3k
										exp /= 6.60f;

									else if (myLevel <= 380) // Exp Média 1.5k
										exp /= 15;

									else if (myLevel <= 390)  // Exp Média 1k
										exp /= 21;

									else if (myLevel <= 400) // Exp Média 800
										exp /= 35;
								}

								if (pMob[party].extra.ClassMaster != MORTAL && pMob[party].extra.ClassMaster != ARCH)
								{
									if (myLevel < 120)
										exp /= 10;

									else if (myLevel < 150)
										exp /= 20;

									else if (myLevel < 170)
										exp /= 40;

									else if (myLevel < 180)
										exp /= 80;

									else if (myLevel < 190)
										exp /= 160;

									else
										exp /= 320;
								}

								exp = 6 * exp / 10;

								if (pMob[conn].ExpBonus > 0 && pMob[conn].ExpBonus < 500)
									exp += exp * pMob[conn].ExpBonus / 100;

								if (NewbieEventServer && pMob[party].MOB.CurrentScore.Level < 100 && pMob[party].extra.ClassMaster != CELESTIAL && pMob[party].extra.ClassMaster != CELESTIALCS && pMob[party].extra.ClassMaster != SCELESTIAL)
									exp += exp / 4;

								if (DOUBLEMODE)
									exp *= 2;

								if (KefraLive == 0)
									exp /= 2;

								if (NewbieEventServer)
									exp += (exp * 15) / 100;
								else
									exp -= (exp * 15) / 100;

								//Log de Experiência diário
								if (when.tm_yday != pMob[party].extra.DayLog.YearDay)
									pMob[party].extra.DayLog.Exp = 0;

								pMob[party].extra.DayLog.YearDay = when.tm_yday;
								pMob[party].extra.DayLog.Exp += exp;

								if (pMob[party].extra.Hold > 0 && (unsigned int)exp >= pMob[party].extra.Hold)
								{
									int nhold = pMob[party].extra.Hold - exp;

									if (nhold < 0)
										nhold = 0;

									exp -= pMob[party].extra.Hold;
									pMob[party].extra.Hold = nhold;
								}

								else if (pMob[party].extra.Hold > 0 && (unsigned int)exp < pMob[party].extra.Hold)
								{
									pMob[party].extra.Hold -= exp;
									continue;
								}

								if (pMob[party].MOB.Exp + exp > g_pNextLevel[MAX_LEVEL + 1])
								{
									pMob[party].MOB.Exp = g_pNextLevel[MAX_LEVEL + 1];
								}
								else
								{
									pMob[party].MOB.Exp += exp;
								}

								// RankingUpdateExp
								STRUCT_RANKING rankInfo = STRUCT_RANKING(pMob[party].MOB.MobName, pMob[party].MOB.Exp, pMob[party].extra.ClassMaster, pMob[party].MOB.CurrentScore.Level, pMob[party].MOB.Class, pMob[party].MOB.Guild, pMob[party].MOB.Clan);

								MSG_UpdateExpRanking expRankingPacket = MSG_UpdateExpRanking(party, rankInfo);
								DBServerSocket.SendOneMessage((char*)&expRankingPacket, sizeof(expRankingPacket));
							}
						}
#pragma endregion
#pragma region Pesadelo Normal
						else if (party > 0 && party < MAX_USER && pMob[party].MOB.CurrentScore.Hp > 0 && (tx / 128) == 10 && (pMob[party].TargetX / 128) == 10 && (ty / 128) == 2 && (pMob[party].TargetY / 128) == 2)
						{
							isExp = GetExpApply(pMob[party].extra, (int)pMob[target].MOB.Exp, pMob[party].MOB.CurrentScore.Level, pMob[target].MOB.CurrentScore.Level);

							int myLevel = pMob[party].MOB.CurrentScore.Level;

							if (pMob[party].extra.ClassMaster != MORTAL && pMob[party].extra.ClassMaster != ARCH)
								myLevel += MAX_LEVEL + 1;

							int exp = (UNK_1 + myLevel) * isExp / (UNK_1 + myLevel);
							if (exp > 0 && exp <= 10000000)
							{
								if (pMob[party].extra.ClassMaster != MORTAL && pMob[party].extra.ClassMaster != ARCH)
								{
									if (myLevel < 120)
										exp /= 10;

									else if (myLevel < 150)
										exp /= 20;

									else if (myLevel < 170)
										exp /= 40;

									else if (myLevel < 180)
										exp /= 80;

									else if (myLevel < 190)
										exp /= 160;

									else
										exp /= 320;
								}

								if (pMob[party].extra.ClassMaster != MORTAL && pMob[party].extra.ClassMaster != ARCH)
								{
									if (myLevel < 120)
										exp /= 10;

									else if (myLevel < 150)
										exp /= 20;

									else if (myLevel < 170)
										exp /= 40;

									else if (myLevel < 180)
										exp /= 80;

									else if (myLevel < 190)
										exp /= 160;

									else
										exp /= 320;
								}

								exp = 6 * exp / 10;

								if (pMob[conn].ExpBonus > 0 && pMob[conn].ExpBonus < 500)
									exp += exp * pMob[conn].ExpBonus / 100;

								if (NewbieEventServer && pMob[party].MOB.CurrentScore.Level < 100 && pMob[party].extra.ClassMaster != CELESTIAL && pMob[party].extra.ClassMaster != CELESTIALCS && pMob[party].extra.ClassMaster != SCELESTIAL)
									exp += exp / 4;

								if (DOUBLEMODE)
									exp *= 2;

								if (KefraLive == 0)
									exp /= 2;

								if (NewbieEventServer)
									exp += (exp * 15) / 100;
								else
									exp -= (exp * 15) / 100;

								// Log de Experiência diário
								if (when.tm_yday != pMob[party].extra.DayLog.YearDay)
									pMob[party].extra.DayLog.Exp = 0;

								pMob[party].extra.DayLog.YearDay = when.tm_yday;
								pMob[party].extra.DayLog.Exp += exp;

								if (pMob[party].extra.Hold > 0 && (unsigned int)exp >= pMob[party].extra.Hold)
								{
									int nhold = pMob[party].extra.Hold - exp;

									if (nhold < 0)
										nhold = 0;

									exp -= pMob[party].extra.Hold;
									pMob[party].extra.Hold = nhold;
								}

								else if (pMob[party].extra.Hold > 0 && (unsigned int)exp < pMob[party].extra.Hold)
								{
									pMob[party].extra.Hold -= exp;
									continue;
								}

								if (pMob[party].MOB.Exp + exp > g_pNextLevel[MAX_LEVEL + 1])
								{
									pMob[party].MOB.Exp = g_pNextLevel[MAX_LEVEL + 1];
								}
								else
								{
									pMob[party].MOB.Exp += exp;
								}
								// RankingUpdateExp
								STRUCT_RANKING rankInfo = STRUCT_RANKING(pMob[party].MOB.MobName, pMob[party].MOB.Exp, pMob[party].extra.ClassMaster, pMob[party].MOB.CurrentScore.Level, pMob[party].MOB.Class, pMob[party].MOB.Guild, pMob[party].MOB.Clan);

								MSG_UpdateExpRanking expRankingPacket = MSG_UpdateExpRanking(party, rankInfo);
								DBServerSocket.SendOneMessage((char*)&expRankingPacket, sizeof(expRankingPacket));
							}
						}
#pragma endregion
#pragma region Pergaminho da Água Arcano
						else if (party > 0 && party < MAX_USER && pMob[party].MOB.CurrentScore.Hp > 0 && (tx / 128) == 10 && (pMob[party].TargetX / 128) == 10	&& (ty / 128) == 27 && (pMob[party].TargetY / 128) == 27)
						{
							isExp = GetExpApply(pMob[party].extra, (int)pMob[target].MOB.Exp, pMob[party].MOB.CurrentScore.Level, pMob[target].MOB.CurrentScore.Level);

							int myLevel = pMob[party].MOB.CurrentScore.Level;

							if (pMob[party].extra.ClassMaster != MORTAL && pMob[party].extra.ClassMaster != ARCH)
							myLevel += MAX_LEVEL + 1;

							int exp = 450 * isExp / (UNK_1 + myLevel);

							if (exp > 0 && exp <= 10000000)
							{
								if (pMob[party].extra.ClassMaster == MORTAL)
								{
									if (myLevel <= 200) // 130k
										exp /= 1;

									else if (myLevel <= 300) // 90k
										exp /= 0.10f;

									else if (myLevel <= 356) // 50k
										exp /= 1;

									else if (myLevel <= 370) // 30k
										exp /= 1.55f;

									else if (myLevel <= 380) // 20k
										exp /= 2.20f;

									else if (myLevel <= 390) // 16k
										exp /= 2.60f;

									else if (myLevel <= 399) // 10k
										exp /= 3.85f;
								}

								if (pMob[party].extra.ClassMaster == ARCH)
								{
									if (myLevel <= 200) // Exp Média 70k
										exp /= 1;

									else if (myLevel <= 300)// Exp Média 50k
										exp /= 0.05f;

									else if (myLevel <= 356)// Exp Média 30k
										exp /= 0.85f;

									else if (myLevel <= 360)// Exp Média 6k
										exp /= 4.0f;

									else if (myLevel <= 370)// Exp Média 3k
										exp /= 6.60f;

									else if (myLevel <= 380) // Exp Média 2k
										exp /= 9;

									else if (myLevel <= 390)  // Exp Média 1.5k
										exp /= 15;

									else if (myLevel <= 400) // Exp Média 900
										exp /= 22;
								}

								if (pMob[party].extra.ClassMaster != MORTAL && pMob[party].extra.ClassMaster != ARCH)
								{
									if (myLevel < 120)
										exp /= 10;

									else if (myLevel < 150)
										exp /= 20;

									else if (myLevel < 170)
										exp /= 40;

									else if (myLevel < 180)
										exp /= 80;

									else if (myLevel < 190)
										exp /= 160;

									else
										exp /= 320;

								}

								exp = 6 * exp / 10;

								if (exp > eMob)
									exp = eMob;

								if (pMob[conn].ExpBonus > 0 && pMob[conn].ExpBonus < 500)
									exp += exp * (pMob[conn].ExpBonus + pMob[conn].g_pFairyContent[0]) / 100;

								if (NewbieEventServer && pMob[party].MOB.CurrentScore.Level < 100 && pMob[party].extra.ClassMaster != CELESTIAL && pMob[party].extra.ClassMaster != CELESTIALCS && pMob[party].extra.ClassMaster != SCELESTIAL)
									exp += exp / 4;

								if (DOUBLEMODE)
									exp *= 2;

								if (KefraLive == 0)
									exp /= 2;

								if (NewbieEventServer)
									exp += (exp * 15) / 100;
								else
									exp -= (exp * 15) / 100;

								//Log de Experiência diário
								if (when.tm_yday != pMob[party].extra.DayLog.YearDay)
									pMob[party].extra.DayLog.Exp = 0;

								pMob[party].extra.DayLog.YearDay = when.tm_yday;
								pMob[party].extra.DayLog.Exp += exp;

								if (pMob[party].extra.Hold > 0 && (unsigned int)exp >= pMob[party].extra.Hold)
								{
									int nhold = pMob[party].extra.Hold - exp;

									if (nhold < 0)
										nhold = 0;

									exp -= pMob[party].extra.Hold;
									pMob[party].extra.Hold = nhold;
								}

								else if (pMob[party].extra.Hold > 0 && (unsigned int)exp < pMob[party].extra.Hold)
								{
									pMob[party].extra.Hold -= exp;
									continue;
								}

								if (pMob[party].MOB.Exp + exp > g_pNextLevel[MAX_LEVEL + 1])
								{
									pMob[party].MOB.Exp = g_pNextLevel[MAX_LEVEL + 1];
								}
								else
								{
									pMob[party].MOB.Exp += exp;
								}

								//RankingUpdateExp
								STRUCT_RANKING rankInfo = STRUCT_RANKING(pMob[party].MOB.MobName, pMob[party].MOB.Exp, pMob[party].extra.ClassMaster, pMob[party].MOB.CurrentScore.Level, pMob[party].MOB.Class, pMob[party].MOB.Guild, pMob[party].MOB.Clan);

								MSG_UpdateExpRanking expRankingPacket = MSG_UpdateExpRanking(party, rankInfo);
								DBServerSocket.SendOneMessage((char*)&expRankingPacket, sizeof(expRankingPacket));
							}
						}
#pragma endregion
#pragma region Pergaminho da Água Mistico
						else if (party > 0 && party < MAX_USER && pMob[party].MOB.CurrentScore.Hp > 0 && (tx / 128) == 9 && (pMob[party].TargetX / 128) == 9 && (ty / 128) == 28 && (pMob[party].TargetY / 128) == 28)
						{
							isExp = GetExpApply(pMob[party].extra, (int)pMob[target].MOB.Exp, pMob[party].MOB.CurrentScore.Level, pMob[target].MOB.CurrentScore.Level);

							int myLevel = pMob[party].MOB.CurrentScore.Level;

							if (pMob[party].extra.ClassMaster != MORTAL && pMob[party].extra.ClassMaster != ARCH)
								myLevel += MAX_LEVEL + 1;

							int exp = 450 * isExp / (UNK_1 + myLevel);

							if (exp > 0 && exp <= 10000000)
							{
								if (pMob[party].extra.ClassMaster == MORTAL)
								{
									if (myLevel <= 200) // 100k
										exp /= 1;

									else if (myLevel <= 300) // 50k
										exp /= 1.08f;

									else if (myLevel <= 356) // 30k
										exp /= 1.30f;

									else if (myLevel <= 370) // 20k
										exp /= 1.80f;

									else if (myLevel <= 380) // 15k
										exp /= 2.20f;

									else if (myLevel <= 390) // 12k
										exp /= 2.60f;

									else if (myLevel <= 399) // 6k
										exp /= 4.70f;
								}

								if (pMob[party].extra.ClassMaster == ARCH)
								{
									if (myLevel <= 200) // Exp Média 50k
										exp /= 1;

									else if (myLevel <= 300)// Exp Média 30k
										exp /= 0.90f;

									else if (myLevel <= 356)// Exp Média 20k
										exp /= 0.95f;

									else if (myLevel <= 360)// Exp Média 4k
										exp /= 4.50f;

									else if (myLevel <= 370)// Exp Média 2k
										exp /= 6.60f;

									else if (myLevel <= 380) // Exp Média 1.4k
										exp /= 12;

									else if (myLevel <= 390)  // Exp Média 800
										exp /= 19;

									else if (myLevel <= 400) // Exp Média 500
										exp /= 32;
								}

								if (pMob[party].extra.ClassMaster != MORTAL && pMob[party].extra.ClassMaster != ARCH)
								{
									if (myLevel < 120)
										exp /= 10;

									else if (myLevel < 150)
										exp /= 20;

									else if (myLevel < 170)
										exp /= 40;

									else if (myLevel < 180)
										exp /= 80;

									else if (myLevel < 190)
										exp /= 160;

									else
										exp /= 320;
								}

								exp = 6 * exp / 10;

								if (exp > eMob)
									exp = eMob;

								if (pMob[conn].ExpBonus > 0 && pMob[conn].ExpBonus < 500)
									exp += exp * (pMob[conn].ExpBonus + pMob[conn].g_pFairyContent[0]) / 100;

								if (NewbieEventServer && pMob[party].MOB.CurrentScore.Level < 100 && pMob[party].extra.ClassMaster != CELESTIAL && pMob[party].extra.ClassMaster != CELESTIALCS && pMob[party].extra.ClassMaster != SCELESTIAL)
									exp += exp / 4;

								if (DOUBLEMODE)
									exp *= 2;

								if (KefraLive == 0)
									exp /= 2;

								if (NewbieEventServer)
									exp += (exp * 15) / 100;
								else
									exp -= (exp * 15) / 100;

								//Log de Experiência diário
								if (when.tm_yday != pMob[party].extra.DayLog.YearDay)
									pMob[party].extra.DayLog.Exp = 0;

								pMob[party].extra.DayLog.YearDay = when.tm_yday;
								pMob[party].extra.DayLog.Exp += exp;

								if (pMob[party].extra.Hold > 0 && (unsigned int)exp >= pMob[party].extra.Hold)
								{
									int nhold = pMob[party].extra.Hold - exp;

									if (nhold < 0)
										nhold = 0;

									exp -= pMob[party].extra.Hold;
									pMob[party].extra.Hold = nhold;
								}
	
								else if (pMob[party].extra.Hold > 0 && (unsigned int)exp < pMob[party].extra.Hold)
								{
									pMob[party].extra.Hold -= exp;
									continue;
								}

								if (pMob[party].MOB.Exp + exp > g_pNextLevel[MAX_LEVEL + 1])
								{
									pMob[party].MOB.Exp = g_pNextLevel[MAX_LEVEL + 1];
								}
								else
								{
									pMob[party].MOB.Exp += exp;
								}

								//RankingUpdateExp
								STRUCT_RANKING rankInfo = STRUCT_RANKING(pMob[party].MOB.MobName, pMob[party].MOB.Exp, pMob[party].extra.ClassMaster, pMob[party].MOB.CurrentScore.Level, pMob[party].MOB.Class, pMob[party].MOB.Guild, pMob[party].MOB.Clan);

								MSG_UpdateExpRanking expRankingPacket = MSG_UpdateExpRanking(party, rankInfo);
								DBServerSocket.SendOneMessage((char*)&expRankingPacket, sizeof(expRankingPacket));
							}
						}
#pragma endregion
#pragma region Pergaminho da Água Normal
						else if (party > 0 && party < MAX_USER && pMob[party].MOB.CurrentScore.Hp > 0 && (tx / 128) == 8 && (pMob[party].TargetX / 128) == 8 && (ty / 128) == 27 && (pMob[party].TargetY / 128) == 27)
						{
							isExp = GetExpApply(pMob[party].extra, (int)pMob[target].MOB.Exp, pMob[party].MOB.CurrentScore.Level, pMob[target].MOB.CurrentScore.Level);

							int myLevel = pMob[party].MOB.CurrentScore.Level;

							if (pMob[party].extra.ClassMaster != MORTAL && pMob[party].extra.ClassMaster != ARCH)
								myLevel += MAX_LEVEL + 1;

							int exp = 450 * isExp / (UNK_1 + myLevel);

							if (exp > 0 && exp <= 10000000)
							{
								if (pMob[party].extra.ClassMaster == MORTAL)
								{
									if (myLevel <= 200) // 70k
										exp /= 1.28;

									else if (myLevel <= 300) // 40k
										exp /= 1.03f;

									else if (myLevel <= 356) // 20k
										exp /= 1.45f;

									else if (myLevel <= 370) // 15k
										exp /= 1.80f;

									else if (myLevel <= 380) // 10k
										exp /= 2.45f;

									else if (myLevel <= 390) // 6k
										exp /= 3.70f;

									else if (myLevel <= 399) // 3k
										exp /= 6.80f;
								}

								if (pMob[party].extra.ClassMaster != MORTAL && pMob[party].extra.ClassMaster != ARCH)
								{
									if (myLevel < 120)
										exp /= 10;

									else if (myLevel < 150)
										exp /= 20;

									else if (myLevel < 170)
										exp /= 40;

									else if (myLevel < 180)
										exp /= 80;

									else if (myLevel < 190)
										exp /= 160;
									else
										exp /= 320;

								}

								exp = 6 * exp / 10;

								if (exp > eMob)
									exp = eMob;

								if (pMob[conn].ExpBonus > 0 && pMob[conn].ExpBonus < 500)
									exp += exp * (pMob[conn].ExpBonus + pMob[conn].g_pFairyContent[0]) / 100;

								if (NewbieEventServer && pMob[party].MOB.CurrentScore.Level < 100 && pMob[party].extra.ClassMaster != CELESTIAL && pMob[party].extra.ClassMaster != CELESTIALCS && pMob[party].extra.ClassMaster != SCELESTIAL)
									exp += exp / 4;

								if (DOUBLEMODE)
									exp *= 2;

								if (KefraLive == 0)
									exp /= 2;

								if (NewbieEventServer)
									exp += (exp * 15) / 100;
								else
									exp -= (exp * 15) / 100;

								//Log de Experiência diário
								if (when.tm_yday != pMob[party].extra.DayLog.YearDay)
									pMob[party].extra.DayLog.Exp = 0;

								pMob[party].extra.DayLog.YearDay = when.tm_yday;
								pMob[party].extra.DayLog.Exp += exp;

								if (pMob[party].extra.Hold > 0 && (unsigned int)exp >= pMob[party].extra.Hold)
								{
									int nhold = pMob[party].extra.Hold - exp;

									if (nhold < 0)
										nhold = 0;

									exp -= pMob[party].extra.Hold;
									pMob[party].extra.Hold = nhold;
								}

								else if (pMob[party].extra.Hold > 0 && (unsigned int)exp < pMob[party].extra.Hold)
								{
									pMob[party].extra.Hold -= exp;
									continue;
								}

								if (pMob[party].MOB.Exp + exp > g_pNextLevel[MAX_LEVEL + 1])
								{
									pMob[party].MOB.Exp = g_pNextLevel[MAX_LEVEL + 1];
								}
								else
								{
									pMob[party].MOB.Exp += exp;
								}

								//RankingUpdateExp
								STRUCT_RANKING rankInfo = STRUCT_RANKING(pMob[party].MOB.MobName, pMob[party].MOB.Exp, pMob[party].extra.ClassMaster, pMob[party].MOB.CurrentScore.Level, pMob[party].MOB.Class, pMob[party].MOB.Guild, pMob[party].MOB.Clan);

								MSG_UpdateExpRanking expRankingPacket = MSG_UpdateExpRanking(party, rankInfo);
								DBServerSocket.SendOneMessage((char*)&expRankingPacket, sizeof(expRankingPacket));
							}
						}
#pragma endregion
						else if (party > 0 && party < MAX_USER && pMob[party].MOB.CurrentScore.Hp > 0 && tx >= pMob[party].TargetX - HALFGRIDX	&& tx <= pMob[party].TargetX + HALFGRIDX && ty >= pMob[party].TargetY - HALFGRIDY && ty <= pMob[party].TargetY + HALFGRIDY)
						{

							isExp = GetExpApply(pMob[party].extra, (int)pMob[target].MOB.Exp, pMob[party].MOB.CurrentScore.Level, pMob[target].MOB.CurrentScore.Level);

							int myLevel = pMob[party].MOB.CurrentScore.Level;

							if (pMob[party].extra.ClassMaster != MORTAL && pMob[party].extra.ClassMaster != ARCH)
								myLevel += MAX_LEVEL + 1;

							int exp = 450 * isExp / (UNK_1 + myLevel);

							if (exp > 0 && exp <= 10000000)
							{
								if (pMob[party].extra.ClassMaster == MORTAL)
								{
									if (myLevel <= 200) //50k
										exp /= 1;

									else if (myLevel <= 300) //25k
										exp /= 1.07f;

									else if (myLevel <= 356) //15k
										exp /= 1.25f;

									else if (myLevel <= 370) //10k
										exp /= 1.70;

									else if (myLevel <= 380) //8k
										exp /= 2.10f;

									else if (myLevel <= 390) //6k
										exp /= 2.60;

									else if (myLevel <= 399) //3k
										exp /= 4;
								}

								if (pMob[party].extra.ClassMaster == ARCH)
								{
									if (myLevel <= 200) // Exp Média 25k
										exp /= 1;

									else if (myLevel <= 300)// Exp Média 15k
										exp /= 0.85f;

									else if (myLevel <= 356)// Exp Média 10k
										exp /= 0.90f;

									else if (myLevel <= 360)// Exp Média 2k
										exp /= 4.50f;

									else if (myLevel <= 370)// Exp Média 1k
										exp /= 5.90f;

									else if (myLevel <= 380) // Exp Média 700
										exp /= 11;

									else if (myLevel <= 390)  // Exp Média 400
										exp /= 17;

									else if (myLevel <= 400) // Exp Média 230
										exp /= 35;
								}

								if (pMob[party].extra.ClassMaster != MORTAL && pMob[party].extra.ClassMaster != ARCH)
								{
									if (myLevel < 120)
										exp /= 10;

									else if (myLevel < 150)
										exp /= 20;

									else if (myLevel < 170)
										exp /= 40;

									else if (myLevel < 180)
										exp /= 80;

									else if (myLevel < 190)
										exp /= 160;

									else
										exp /= 320;
								}

								exp = 6 * exp / 10;

								if (exp > eMob)
									exp = eMob;

								if (pMob[conn].ExpBonus > 0 && pMob[conn].ExpBonus < 500)
									exp += exp * (pMob[conn].ExpBonus + pMob[conn].g_pFairyContent[0]) / 100;

								if (g_pRvrWar.Bonus != 0)
								{
									if (g_pRvrWar.Bonus == pMob[party].MOB.Clan)
										exp += (exp * 5) / 100;
								}

								if (NewbieEventServer && pMob[party].MOB.CurrentScore.Level < 100 && pMob[party].extra.ClassMaster != CELESTIAL && pMob[party].extra.ClassMaster != CELESTIALCS && pMob[party].extra.ClassMaster != SCELESTIAL)
									exp += exp / 4;

								if (DOUBLEMODE)
									exp *= 2;

								if (KefraLive == 0)
									exp /= 2;

								if (NewbieEventServer)
									exp += (exp * 15) / 100;
								else
									exp -= (exp * 15) / 100;

								//Log de Experiência diário
								if (when.tm_yday != pMob[party].extra.DayLog.YearDay)
									pMob[party].extra.DayLog.Exp = 0;

								pMob[party].extra.DayLog.YearDay = when.tm_yday;
								pMob[party].extra.DayLog.Exp += exp;

								if (pMob[party].extra.Hold > 0 && (unsigned int)exp >= pMob[party].extra.Hold)
								{
									int nhold = pMob[party].extra.Hold - exp;

									if (nhold < 0)
										nhold = 0;

									exp -= pMob[party].extra.Hold;
									pMob[party].extra.Hold = nhold;
								}

								else if (pMob[party].extra.Hold > 0 && (unsigned int)exp < pMob[party].extra.Hold)
								{
									pMob[party].extra.Hold -= exp;
									continue;
								}

								if (pMob[party].MOB.Exp + exp > g_pNextLevel[MAX_LEVEL + 1])
								{
									pMob[party].MOB.Exp = g_pNextLevel[MAX_LEVEL + 1];
								}
								else
								{
									pMob[party].MOB.Exp += exp;
								}

								//RankingUpdateExp
								STRUCT_RANKING rankInfo = STRUCT_RANKING(pMob[party].MOB.MobName, pMob[party].MOB.Exp, pMob[party].extra.ClassMaster, pMob[party].MOB.CurrentScore.Level, pMob[party].MOB.Class, pMob[party].MOB.Guild, pMob[party].MOB.Clan);

								MSG_UpdateExpRanking expRankingPacket = MSG_UpdateExpRanking(party, rankInfo);
								DBServerSocket.SendOneMessage((char*)&expRankingPacket, sizeof(expRankingPacket));
							}
						}
					}
#pragma endregion
#pragma region Targets Monstros
					int GenerateIndex = pMob[target].GenerateIndex;
					int DieSay = rand() % 4;

					if (GenerateIndex >= 0 && mNPCGen.pList[GenerateIndex].DieAction[DieSay][0] && pMob[target].Leader == 0)
						SendChat(target, mNPCGen.pList[GenerateIndex].DieAction[DieSay]);

					GridMulticast(pMob[target].TargetX, pMob[target].TargetY, (MSG_STANDARD*)&sm, 0);

					if (pMob[conn].MOB.Clan && pMob[conn].MOB.Clan != 4 && pMob[conn].MOB.Clan != 7 && pMob[conn].MOB.Clan != 8)
						DeleteMob(target, 1);

					else
					{
						int AlvoX = PosX;
						if (!PosX)
							AlvoX = pMob[target].TargetX;

						int AlvoY = PosY;
						if (!PosY)
							AlvoY = pMob[target].TargetY;

#pragma region Kefra
						if (pMob[target].GenerateIndex == KEFRA_BOSS)
						{
							if (pMob[conn].MOB.Guild)
							{
								int Group = ServerGroup;
								int Server = pMob[conn].MOB.Guild / MAX_GUILD;
								int usGuild = pMob[conn].MOB.Guild;

								char guildname[256];

								BASE_GetGuildName(Group, usGuild, guildname);

								KefraLive = usGuild;

								MSG_GuildInfo sm_gi;
								memset(&sm_gi, 0, sizeof(MSG_GuildInfo));

								sm_gi.Type = _MSG_GuildInfo;
								sm_gi.Size = sizeof(MSG_GuildInfo);
								sm_gi.ID = conn;

								sm_gi.Guild = usGuild;
								GuildInfo[usGuild].Fame += 100;

								sm_gi.GuildInfo = GuildInfo[usGuild];

								DBServerSocket.SendOneMessage((char*)&sm_gi, sizeof(MSG_GuildInfo));

								sprintf(temp, g_pMessageStringTable[_SN_End_Khepra], KefraKiller);
								SendNotice(temp);

								DrawConfig(TRUE);
							}
							
							else
							{
								KefraLive = 1;
								sprintf(temp, g_pMessageStringTable[_SN_End_Khepra], "CdK");
								SendNotice(temp);

								DrawConfig(TRUE);
							}

							for (int xx = 2335; xx < 2395; xx++)
							{
								for (int yy = 3896; yy < 3955; yy++)
								{
									int tmob = pMobGrid[yy][xx];

									if (tmob < MAX_USER)
										continue;

									int itemrand = rand() % 60;

									STRUCT_ITEM *KefraItem = &pMob[target].MOB.Carry[itemrand];

									PutItem(conn, KefraItem);
								}
							}

							sprintf(temp, "etc,kefra killed");
							Log(temp, pMob[conn].MOB.MobName, 0);
						}
#pragma endregion
						if (pHeightGrid[AlvoY][AlvoX] >= -50 && pHeightGrid[AlvoY][AlvoX] <= 92)
						{
							int GenerateID = pMob[target].GenerateIndex;

#pragma region Portões de Noatum
							if (1 > 0)
							{
								int GenerateID = pMob[target].GenerateIndex;
								if (pMob[target].MOB.Equip[0].sIndex == 220 && CastleState)
								{
									if (pMob[target].TargetX < 0 || pMob[target].TargetX >= MAX_GRIDX || pMob[target].TargetY < 0 || pMob[target].TargetY >= MAX_GRIDY)
									{
										Log("err,no castle gate to open pos", "-system", 0);
										return;
									}

									int ItemID = pItemGrid[pMob[target].TargetY][pMob[target].TargetX];
									if (ItemID >= 0 && ItemID < MAX_ITEM && pItem[ItemID].Mode)
									{
										int heigth = 0;
										int isUpdate = UpdateItem(ItemID, 1, &heigth);
										if (isUpdate)
										{
											MSG_UpdateItem UpdateIt;
												
											UpdateIt.ID = ESCENE_FIELD;
											UpdateIt.Type = _MSG_UpdateItem;

											UpdateIt.ItemID = ItemID + 10000;

											UpdateIt.Size = sizeof(MSG_UpdateItem);

											//UpdateIt.Height = heigth;

											UpdateIt.State = pItem[ItemID].State;

											GridMulticast(pItem[ItemID].PosX, pItem[ItemID].PosY, (MSG_STANDARD*)&UpdateIt, 0);

											pItem[ItemID].Delay = 0;
										}
									}

									else
										Log("err,no castle gato to open", "-system", 0);
								}
								else
								{
#pragma endregion
#pragma region Agua M
						//AguaM
								if (GenerateID >= WATER_M_INITIAL && GenerateID <= WATER_M_INITIAL + 7)
								{
									int CurrentNumMob = mNPCGen.pList[GenerateID].CurrentNumMob;

									if (CurrentNumMob == 1)
									{
									int Sala = GenerateID - WATER_M_INITIAL;

										if (WaterClear1[1][Sala] > 15)
										WaterClear1[1][Sala] = 15;


										int partyleader = pMob[conn].Leader;

										if (partyleader <= 0)
										partyleader = conn;

										int slot_free = -1;

										STRUCT_ITEM Perga;
										memset(&Perga, 0, sizeof(STRUCT_ITEM));

										Perga.sIndex = 778 + Sala;

										if (partyleader > 0 && partyleader < MAX_USER)
										PutItem(partyleader, &Perga);

										SendClientSignalParm(partyleader, ESCENE_FIELD, _MSG_StartTime, WaterClear1[1][Sala] * 2);
	
										for (int i = 0; i < MAX_PARTY; i++)
										{
											int partymember = pMob[partyleader].PartyList[i];

											if (pUser[partymember].Mode != USER_PLAY)
											continue;

										SendClientSignalParm(partymember, ESCENE_FIELD, _MSG_StartTime, WaterClear1[1][Sala] * 2);
										}
									}
								}

								else if (GenerateID >= WATER_M_INITIAL + 8 && GenerateID <= WATER_M_INITIAL + 11)
								{
									int CurrentNumMob = mNPCGen.pList[GenerateID].CurrentNumMob;

									if (CurrentNumMob == 1)
									{
										int Sala = 9;

										if (WaterClear1[1][Sala] > 5)
											WaterClear1[1][Sala] = 5;


										int partyleader = pMob[conn].Leader;

										if (partyleader <= 0)
											partyleader = conn;

										SendClientSignalParm(partyleader, ESCENE_FIELD, _MSG_StartTime, WaterClear1[1][Sala] * 2);

										for (int i = 0; i < MAX_PARTY; i++)
										{
											int partymember = pMob[partyleader].PartyList[i];

											if (pUser[partymember].Mode != USER_PLAY)
												continue;

											SendClientSignalParm(partymember, ESCENE_FIELD, _MSG_StartTime, WaterClear1[1][Sala] * 2);
										}
									}
								}
#pragma endregion
#pragma region Agua N
								else if (GenerateID >= WATER_N_INITIAL && GenerateID <= WATER_N_INITIAL + 7)
								{
									int CurrentNumMob = mNPCGen.pList[GenerateID].CurrentNumMob;

									if (CurrentNumMob == 1)
									{
										int Sala = GenerateID - WATER_N_INITIAL;

										if (WaterClear1[0][Sala] > 15)
											WaterClear1[0][Sala] = 15;


										int partyleader = pMob[conn].Leader;

										if (partyleader <= 0)
											partyleader = conn;

										int slot_free = -1;

										STRUCT_ITEM Perga;
										memset(&Perga, 0, sizeof(STRUCT_ITEM));

										Perga.sIndex = 3174 + Sala;

										if (partyleader > 0 && partyleader < MAX_USER)
											PutItem(partyleader, &Perga);

										SendClientSignalParm(partyleader, ESCENE_FIELD, _MSG_StartTime, WaterClear1[0][Sala] * 2);

										for (int i = 0; i < MAX_PARTY; i++)
										{
											int partymember = pMob[partyleader].PartyList[i];
		
											if (pUser[partymember].Mode != USER_PLAY)
												continue;

											SendClientSignalParm(partymember, ESCENE_FIELD, _MSG_StartTime, WaterClear1[0][Sala] * 2);
										}
									}
								}

								else if (GenerateID >= WATER_N_INITIAL + 8 && GenerateID <= WATER_N_INITIAL + 11)
								{
									int CurrentNumMob = mNPCGen.pList[GenerateID].CurrentNumMob;

									if (CurrentNumMob == 1)
									{
										int Sala = 9;

										if (WaterClear1[0][Sala] > 5)
											WaterClear1[0][Sala] = 5;


										int partyleader = pMob[conn].Leader;

										if (partyleader <= 0)
											partyleader = conn;

										SendClientSignalParm(partyleader, ESCENE_FIELD, _MSG_StartTime, WaterClear1[0][Sala] * 2);

										for (int i = 0; i < MAX_PARTY; i++)
										{
											int partymember = pMob[partyleader].PartyList[i];

											if (pUser[partymember].Mode != USER_PLAY)
												continue;

											SendClientSignalParm(partymember, ESCENE_FIELD, _MSG_StartTime, WaterClear1[0][Sala] * 2);
										}
									}
								}
#pragma endregion
#pragma region Agua A
						else if (GenerateID >= WATER_A_INITIAL && GenerateID <= WATER_A_INITIAL + 7)
						{
							int CurrentNumMob = mNPCGen.pList[GenerateID].CurrentNumMob;

							if (CurrentNumMob == 1)
							{
								int Sala = GenerateID - WATER_A_INITIAL;

								if (WaterClear1[2][Sala] > 15)
									WaterClear1[2][Sala] = 15;


								int partyleader = pMob[conn].Leader;

								if (partyleader <= 0)
									partyleader = conn;

								int slot_free = -1;

								STRUCT_ITEM Perga;
								memset(&Perga, 0, sizeof(STRUCT_ITEM));

								Perga.sIndex = 3183 + Sala;

								if (partyleader > 0 && partyleader < MAX_USER)
									PutItem(partyleader, &Perga);

								SendClientSignalParm(partyleader, ESCENE_FIELD, _MSG_StartTime, WaterClear1[2][Sala] * 2);

								for (int i = 0; i < MAX_PARTY; i++)
								{
									int partymember = pMob[partyleader].PartyList[i];

									if (pUser[partymember].Mode != USER_PLAY)
										continue;

									SendClientSignalParm(partymember, ESCENE_FIELD, _MSG_StartTime, WaterClear1[2][Sala] * 2);
								}
							}
						}

						else if (GenerateID >= WATER_A_INITIAL + 8 && GenerateID <= WATER_A_INITIAL + 11)
						{
							int CurrentNumMob = mNPCGen.pList[GenerateID].CurrentNumMob;

							if (CurrentNumMob == 1)
							{
								int Sala = 9;

								if (WaterClear1[2][Sala] > 5)
									WaterClear1[2][Sala] = 5;


								int partyleader = pMob[conn].Leader;

								if (partyleader <= 0)
									partyleader = conn;

								SendClientSignalParm(partyleader, ESCENE_FIELD, _MSG_StartTime, WaterClear1[2][Sala] * 2);

								for (int i = 0; i < MAX_PARTY; i++)
								{
									int partymember = pMob[partyleader].PartyList[i];

									if (pUser[partymember].Mode != USER_PLAY)
										continue;

									SendClientSignalParm(partymember, ESCENE_FIELD, _MSG_StartTime, WaterClear1[2][Sala] * 2);
								}
							}
						}
#pragma endregion
#pragma region Castelo Zakum
						CCastleZakum::MobKilled(target, conn, PosX, PosY);
						// CEncampment::MobKilled(target, conn, PosX, PosY);

						if (GenerateID == ORC_GUERREIRO)
							DoRecall(conn);

						else if (GenerateID >= TORRE_NOATUM1 && GenerateID <= TORRE_NOATUM3)
							LiveTower[GenerateID - TORRE_NOATUM1] = 0;


						else if (GenerateID == REI_HARABARD)
						{
							Kingdom1Clear = 1;
							SendNoticeArea(g_pMessageStringTable[_NN_King1_Killed], 1676, 1556, 1776, 1636);
						}
						else if (GenerateID == REI_GLANTUAR)
						{
							Kingdom2Clear = 1;
							SendNoticeArea(g_pMessageStringTable[_NN_King2_Killed], 1676, 1816, 1776, 1892);
						}
#pragma endregion
#pragma region Boss R Lac R Ori Circulo divino
						else if (GenerateID == 0 || GenerateID == 1 || GenerateID == 2)
						{
							STRUCT_ITEM item;
							memset(&item, 0, sizeof(STRUCT_ITEM));

							int _rand = rand() % 14;

							if (_rand == 0)
								item.sIndex = 419;

							else if (_rand == 1)
								item.sIndex = 420;

							if (item.sIndex)
							{
								SetItemBonus(&item, 0, 0, 0);

								PutItem(conn, &item);
							}
						}

						else if (GenerateID == 5 || GenerateID == 6 || GenerateID == 7)
						{
							STRUCT_ITEM item;
							memset(&item, 0, sizeof(STRUCT_ITEM));

							int _rand = rand() % 14;

							if (_rand == 0)
								item.sIndex = 421 + (rand() % 7);

							else if (_rand == 1)
								item.sIndex = 419;

							if (item.sIndex)
							{
								SetItemBonus(&item, 0, 0, 0);

								if (pHeightGrid[AlvoY][AlvoX] > -40 && pHeightGrid[AlvoY][AlvoX] < 36)
									PutItem(conn, &item);
								//CreateItem(AlvoX, AlvoY, &item, rand() % 4, 1);
							}
						}
						//#pragma endregion
						else if (GenerateID == 3)
						{
							STRUCT_ITEM item;
							memset(&item, 0, sizeof(STRUCT_ITEM));

							int _rand = rand() % 7;

							if (_rand == 0)
								item.sIndex = 1106;

							else if (_rand == 1)
								item.sIndex = 1256;

							else if (_rand == 2)
								item.sIndex = 1418;

							else if (_rand == 3)
								item.sIndex = 1568;

							if (item.sIndex)
							{
								SetItemBonus(&item, 75, 1, 0);

								if (pHeightGrid[AlvoY][AlvoX] > -40 && pHeightGrid[AlvoY][AlvoX] < 36)
									PutItem(conn, &item);
								//CreateItem(AlvoX, AlvoY, &item, rand() % 4, 1);
							}
						}

#pragma endregion
#pragma region Pesas

						else if (GenerateIndex >= NIGHTMARE_M_INITIAL && GenerateIndex <= NIGHTMARE_M_END)
							GenerateMob(GenerateIndex, 0, 0);//Pesa M

						else if (GenerateIndex >= NIGHTMARE_N_INITIAL && GenerateIndex <= NIGHTMARE_N_END)
							GenerateMob(GenerateIndex, 0, 0);//Pesa N

						else if (GenerateIndex >= NIGHTMARE_A_INITIAL && GenerateIndex <= NIGHTMARE_A_END)
							GenerateMob(GenerateIndex, 0, 0);//Pesa A
#pragma endregion
#pragma region Carta de duelo N
								//Carta N
						if (GenerateID >= SECRET_ROOM_N_SALA1_MOB_1 && GenerateID <= SECRET_ROOM_N_SALA1_MOB_2)
						{
							int CurrentNumMob = mNPCGen.pList[SECRET_ROOM_N_SALA1_MOB_1].CurrentNumMob;
							CurrentNumMob += mNPCGen.pList[SECRET_ROOM_N_SALA1_MOB_2].CurrentNumMob;

							if (CurrentNumMob == 1)
							{
								CartaTime = 60;

								ClearAreaTeleport(778, 3652, 832, 3698, CartaPos[1][0], CartaPos[1][1]);

								CartaSala++;

								MSG_STANDARDPARM sm;

								sm.Type = _MSG_StartTime;
								sm.ID = ESCENE_FIELD;
								sm.Size = sizeof(MSG_STANDARDPARM);
								sm.Parm = CartaTime * 2;

								MapaMulticast(6, 28, (MSG_STANDARD*)&sm, 0);
							}
						}

						if (GenerateID >= SECRET_ROOM_N_SALA2_MOB_1 && GenerateID <= SECRET_ROOM_N_SALA2_MOB_2)
						{
							int CurrentNumMob = mNPCGen.pList[SECRET_ROOM_N_SALA2_MOB_1].CurrentNumMob;
							CurrentNumMob += mNPCGen.pList[SECRET_ROOM_N_SALA2_MOB_2].CurrentNumMob;

							if (CurrentNumMob == 1)
							{
								CartaTime = 60;

								ClearAreaTeleport(836, 3652, 890, 3698, CartaPos[2][0], CartaPos[2][1]);

								CartaSala++;

								MSG_STANDARDPARM sm;

								sm.Type = _MSG_StartTime;
								sm.ID = ESCENE_FIELD;
								sm.Size = sizeof(MSG_STANDARDPARM);
								sm.Parm = CartaTime * 2;

								MapaMulticast(6, 28, (MSG_STANDARD*)&sm, 0);
							}
						}

						if (GenerateID >= SECRET_ROOM_N_SALA3_MOB_1 && GenerateID <= SECRET_ROOM_N_SALA3_MOB_2)
						{
							int CurrentNumMob = mNPCGen.pList[SECRET_ROOM_N_SALA3_MOB_1].CurrentNumMob;
							CurrentNumMob += mNPCGen.pList[SECRET_ROOM_N_SALA3_MOB_2].CurrentNumMob;

							if (CurrentNumMob == 1)
							{
								CartaTime = 60;

								ClearAreaTeleport(834, 3595, 889, 3645, CartaPos[3][0], CartaPos[3][1]);

								CartaSala++;

								MSG_STANDARDPARM sm;

								sm.Type = _MSG_StartTime;
								sm.ID = ESCENE_FIELD;
								sm.Size = sizeof(MSG_STANDARDPARM);
								sm.Parm = CartaTime * 2;

								MapaMulticast(6, 28, (MSG_STANDARD*)&sm, 0);
							}
						}

						if (GenerateID >= SECRET_ROOM_N_SALA4_MOB_1 && GenerateID <= SECRET_ROOM_N_SALA4_MOB_4)
						{
							int CurrentNumMob = mNPCGen.pList[SECRET_ROOM_N_SALA4_MOB_1].CurrentNumMob;
							CurrentNumMob += mNPCGen.pList[SECRET_ROOM_N_SALA4_MOB_2].CurrentNumMob;
							CurrentNumMob += mNPCGen.pList[SECRET_ROOM_N_SALA4_MOB_3].CurrentNumMob;
							CurrentNumMob += mNPCGen.pList[SECRET_ROOM_N_SALA4_MOB_4].CurrentNumMob;

							if (CurrentNumMob == 1)
							{
								//ClearArea(776, 3595, 834, 3648);
								CartaTime = 3;
								CartaSala = 4;
							}
						}
#pragma endregion
#pragma region Carta de duelo M
						//Carta M
						if (GenerateID >= SECRET_ROOM_M_SALA1_MOB_1 && GenerateID <= SECRET_ROOM_M_SALA1_MOB_2)
						{
							int CurrentNumMob = mNPCGen.pList[SECRET_ROOM_M_SALA1_MOB_1].CurrentNumMob;
							CurrentNumMob += mNPCGen.pList[SECRET_ROOM_M_SALA1_MOB_2].CurrentNumMob;

							if (CurrentNumMob == 1)
							{
								CartaTime = 60;

								ClearAreaTeleport(778, 3652, 832, 3698, CartaPos[1][0], CartaPos[1][1]);

								CartaSala++;

								MSG_STANDARDPARM sm;

								sm.Type = _MSG_StartTime;
								sm.ID = ESCENE_FIELD;
								sm.Size = sizeof(MSG_STANDARDPARM);
								sm.Parm = CartaTime * 2;

								MapaMulticast(6, 28, (MSG_STANDARD*)&sm, 0);
							}
						}

						if (GenerateID >= SECRET_ROOM_M_SALA2_MOB_1 && GenerateID <= SECRET_ROOM_M_SALA2_MOB_2)
						{
							int CurrentNumMob = mNPCGen.pList[SECRET_ROOM_M_SALA2_MOB_1].CurrentNumMob;
							CurrentNumMob += mNPCGen.pList[SECRET_ROOM_M_SALA2_MOB_2].CurrentNumMob;

							if (CurrentNumMob == 1)
							{
								CartaTime = 60;

								ClearAreaTeleport(836, 3652, 890, 3698, CartaPos[2][0], CartaPos[2][1]);

								CartaSala++;

								MSG_STANDARDPARM sm;

								sm.Type = _MSG_StartTime;
								sm.ID = ESCENE_FIELD;
								sm.Size = sizeof(MSG_STANDARDPARM);
								sm.Parm = CartaTime * 2;

								MapaMulticast(6, 28, (MSG_STANDARD*)&sm, 0);
							}
						}

						if (GenerateID >= SECRET_ROOM_M_SALA3_MOB_1 && GenerateID <= SECRET_ROOM_M_SALA3_MOB_2)
						{
							int CurrentNumMob = mNPCGen.pList[SECRET_ROOM_M_SALA3_MOB_1].CurrentNumMob;
							CurrentNumMob += mNPCGen.pList[SECRET_ROOM_M_SALA3_MOB_2].CurrentNumMob;

							if (CurrentNumMob == 1)
							{
								CartaTime = 60;

								ClearAreaTeleport(834, 3595, 889, 3645, CartaPos[3][0], CartaPos[3][1]);

								CartaSala++;

								MSG_STANDARDPARM sm;

								sm.Type = _MSG_StartTime;
								sm.ID = ESCENE_FIELD;
								sm.Size = sizeof(MSG_STANDARDPARM);
								sm.Parm = CartaTime * 2;

								MapaMulticast(6, 28, (MSG_STANDARD*)&sm, 0);
							}
						}

						if (GenerateID >= SECRET_ROOM_M_SALA4_MOB_1 && GenerateID <= SECRET_ROOM_M_SALA4_MOB_4)
						{
							int CurrentNumMob = mNPCGen.pList[SECRET_ROOM_M_SALA4_MOB_1].CurrentNumMob;
							CurrentNumMob += mNPCGen.pList[SECRET_ROOM_M_SALA4_MOB_2].CurrentNumMob;
							CurrentNumMob += mNPCGen.pList[SECRET_ROOM_M_SALA4_MOB_3].CurrentNumMob;
							CurrentNumMob += mNPCGen.pList[SECRET_ROOM_M_SALA4_MOB_4].CurrentNumMob;

							if (CurrentNumMob == 1)
							{
								//ClearArea(776, 3595, 834, 3648);
								CartaTime = 3;
								CartaSala = 4;
							}
						}
#pragma endregion
#pragma region Carta de duelo A
						//Carta A
						if (GenerateID >= SECRET_ROOM_A_SALA1_MOB_1 && GenerateID <= SECRET_ROOM_A_SALA1_MOB_2)
						{
							int CurrentNumMob = mNPCGen.pList[SECRET_ROOM_A_SALA1_MOB_1].CurrentNumMob;
							CurrentNumMob += mNPCGen.pList[SECRET_ROOM_A_SALA1_MOB_2].CurrentNumMob;

							if (CurrentNumMob == 1)
							{
								CartaTime = 60;

								ClearAreaTeleport(778, 3652, 832, 3698, CartaPos[1][0], CartaPos[1][1]);

								CartaSala++;

								MSG_STANDARDPARM sm;

								sm.Type = _MSG_StartTime;
								sm.ID = ESCENE_FIELD;
								sm.Size = sizeof(MSG_STANDARDPARM);
								sm.Parm = CartaTime * 2;

								MapaMulticast(6, 28, (MSG_STANDARD*)&sm, 0);
							}
						}

						if (GenerateID >= SECRET_ROOM_A_SALA2_MOB_1 && GenerateID <= SECRET_ROOM_A_SALA2_MOB_2)
						{
							int CurrentNumMob = mNPCGen.pList[SECRET_ROOM_A_SALA2_MOB_1].CurrentNumMob;
							CurrentNumMob += mNPCGen.pList[SECRET_ROOM_A_SALA2_MOB_2].CurrentNumMob;

							if (CurrentNumMob == 1)
							{
								CartaTime = 60;

								ClearAreaTeleport(836, 3652, 890, 3698, CartaPos[2][0], CartaPos[2][1]);

								CartaSala++;

								MSG_STANDARDPARM sm;

								sm.Type = _MSG_StartTime;
								sm.ID = ESCENE_FIELD;
								sm.Size = sizeof(MSG_STANDARDPARM);
								sm.Parm = CartaTime * 2;

								MapaMulticast(6, 28, (MSG_STANDARD*)&sm, 0);
							}
						}

						if (GenerateID >= SECRET_ROOM_A_SALA3_MOB_1 && GenerateID <= SECRET_ROOM_A_SALA3_MOB_2)
						{
							int CurrentNumMob = mNPCGen.pList[SECRET_ROOM_A_SALA3_MOB_1].CurrentNumMob;
							CurrentNumMob += mNPCGen.pList[SECRET_ROOM_A_SALA3_MOB_2].CurrentNumMob;

							if (CurrentNumMob == 1)
							{
								CartaTime = 60;

								ClearAreaTeleport(834, 3595, 889, 3645, CartaPos[3][0], CartaPos[3][1]);

								CartaSala++;

								MSG_STANDARDPARM sm;

								sm.Type = _MSG_StartTime;
								sm.ID = ESCENE_FIELD;
								sm.Size = sizeof(MSG_STANDARDPARM);
								sm.Parm = CartaTime * 2;

								MapaMulticast(6, 28, (MSG_STANDARD*)&sm, 0);
							}
						}

						if (GenerateID >= SECRET_ROOM_A_SALA4_MOB_1 && GenerateID <= SECRET_ROOM_A_SALA4_MOB_4)
						{
							int CurrentNumMob = mNPCGen.pList[SECRET_ROOM_A_SALA4_MOB_1].CurrentNumMob;
							CurrentNumMob += mNPCGen.pList[SECRET_ROOM_A_SALA4_MOB_2].CurrentNumMob;
							CurrentNumMob += mNPCGen.pList[SECRET_ROOM_A_SALA4_MOB_3].CurrentNumMob;
							CurrentNumMob += mNPCGen.pList[SECRET_ROOM_A_SALA4_MOB_4].CurrentNumMob;

							if (CurrentNumMob == 1)
							{
								//ClearArea(776, 3595, 834, 3648);
								CartaTime = 3;
								CartaSala = 4;
							}
						}
#pragma endregion
#pragma region Pista de Runas
#pragma region Pista Nv0 Lich_Crunt
						if (GenerateID == RUNEQUEST_LV0_LICH1 || GenerateID == RUNEQUEST_LV0_LICH2)
						{
							int _rand = rand() % 100;

							if (_rand < 20)
							{
								for (int c = MAX_USER; c < MAX_MOB; c++)
								{
									if (pMob[c].Mode == USER_EMPTY)
										continue;

									if (pMob[c].GenerateIndex != RUNEQUEST_LV0_LICH1 && pMob[c].GenerateIndex != RUNEQUEST_LV0_LICH2)
										continue;

									DeleteMob(c, 3);
								}

								int partyleader = pMob[conn].Leader;

								if (partyleader <= 0)
									partyleader = conn;

								STRUCT_ITEM Runa;
								memset(&Runa, 0, sizeof(STRUCT_ITEM));

								Runa.sIndex = PistaRune[0][rand() % 4];

								if (partyleader > 0 && partyleader < MAX_USER)
									PutItem(partyleader, &Runa);

								for (int i = 0; i < MAX_PARTY; i++)
								{
									int partymember = pMob[partyleader].PartyList[i];

									if (pUser[partymember].Mode != USER_PLAY)
										continue;

									Runa.sIndex = PistaRune[0][rand() % 4];
									PutItem(partymember, &Runa);
								}

								STRUCT_ITEM NextPista;
								memset(&NextPista, 0, sizeof(STRUCT_ITEM));

								NextPista.sIndex = 5134;
								NextPista.stEffect[0].cEffect = 43;
								NextPista.stEffect[0].cValue = 1;

								PutItem(partyleader, &NextPista);

								sprintf(temp, "etc,questRune +0 complete leader:%s", pMob[partyleader].MOB.MobName);
								Log(temp, "-system", 0);
							}
							else
							{
								for (int c = MAX_USER; c < MAX_MOB; c++)
								{
									if (pMob[c].Mode == USER_EMPTY)
										continue;

									if (pMob[c].GenerateIndex != RUNEQUEST_LV0_LICH1 && pMob[c].GenerateIndex != RUNEQUEST_LV0_LICH2)
										continue;

									DeleteMob(c, 3);
								}

								GenerateMob(RUNEQUEST_LV0_LICH1, 0, 0);
								GenerateMob(RUNEQUEST_LV0_LICH1, 0, 0);

								GenerateMob(RUNEQUEST_LV0_LICH2, 0, 0);
								GenerateMob(RUNEQUEST_LV0_LICH2, 0, 0);
							}
						}
#pragma endregion
#pragma region Pista Nv1 Torre
						if (GenerateID == RUNEQUEST_LV1_TORRE1 || GenerateID == RUNEQUEST_LV1_TORRE2 || GenerateID == RUNEQUEST_LV1_TORRE3)
						{
							int party = GenerateID - RUNEQUEST_LV1_TORRE1;

							Pista[1].Party[party].MobCount = 0;
						}

						if (GenerateID >= RUNEQUEST_LV1_MOB_INITIAL && GenerateID <= RUNEQUEST_LV1_MOB_END)
						{
							int partyleader = pMob[conn].Leader;

							if (partyleader <= 0)
								partyleader = conn;

							if (mNPCGen.pList[RUNEQUEST_LV1_TORRE1].CurrentNumMob >= 1 && Pista[1].Party[0].LeaderID == partyleader)
								Pista[1].Party[0].MobCount++;

							else if (mNPCGen.pList[RUNEQUEST_LV1_TORRE2].CurrentNumMob >= 1 && Pista[1].Party[1].LeaderID == partyleader)
								Pista[1].Party[1].MobCount++;

							else if (mNPCGen.pList[RUNEQUEST_LV1_TORRE3].CurrentNumMob >= 1 && Pista[1].Party[2].LeaderID == partyleader)
								Pista[1].Party[2].MobCount++;
						}
#pragma endregion
#pragma region Pista Nv2 Amon
						if (GenerateID == RUNEQUEST_LV2_MOB_BOSS)
						{
							int partyleader = pMob[conn].Leader;

							if (partyleader <= 0)
								partyleader = conn;

							STRUCT_ITEM Runa;
							memset(&Runa, 0, sizeof(STRUCT_ITEM));

							Runa.sIndex = PistaRune[2][rand() % 5];

							if (partyleader > 0 && partyleader < MAX_USER)
								PutItem(partyleader, &Runa);

							for (int i = 0; i < MAX_PARTY; i++)
							{
								int partymember = pMob[partyleader].PartyList[i];

								if (pUser[partymember].Mode != USER_PLAY)
									continue;

								Runa.sIndex = PistaRune[2][rand() % 5];
								PutItem(partymember, &Runa);
							}

							STRUCT_ITEM NextPista;
							memset(&NextPista, 0, sizeof(STRUCT_ITEM));

							NextPista.sIndex = 5134;
							NextPista.stEffect[0].cEffect = 43;
							NextPista.stEffect[0].cValue = 3;

							PutItem(partyleader, &NextPista);

							sprintf(temp, "etc,questRune +2 complete leader:%s", pMob[partyleader].MOB.MobName);
							Log(temp, "-system", 0);
						}
#pragma endregion
#pragma region Pista Nv3 Sulrang
						if (GenerateID >= RUNEQUEST_LV3_MOB_SULRANG_INITIAL && GenerateID <= RUNEQUEST_LV3_MOB_SULRANG_END && Pista[3].Party[0].LeaderID != 0 && Pista[3].Party[0].MobCount == 0)
						{
							Pista[3].Party[0].MobCount = 1;

							GenerateMob(RUNEQUEST_LV3_MOB_BOSS_INITIAL + rand() % 7, 0, 0);
						}
#pragma endregion
#pragma region Pista Nv4 Labirinto
						if (GenerateID >= RUNEQUEST_LV4_MOB_INITIAL && GenerateID <= RUNEQUEST_LV4_MOB_END && mNPCGen.pList[GenerateID].CurrentNumMob == 1)
						{
							int leader = pMob[conn].Leader;

							int partyleader = pMob[conn].Leader;

							if (partyleader <= 0)
								partyleader = conn;

							if (partyleader == Pista[4].Party[0].LeaderID && Pista[4].Party[1].MobCount >= 1 && Pista[4].Party[2].MobCount >= 1)
							{
								Pista[4].Party[0].MobCount--;

								if (Pista[4].Party[0].MobCount == 0)
								{
									DoTeleport(partyleader, 3351, 1334);

									for (int i = 0; i < MAX_PARTY; i++)
									{
										int partyconn = pMob[partyleader].PartyList[i];

										if (partyconn > 0 && partyconn < MAX_USER && partyconn != partyleader && pUser[partyconn].Mode == USER_PLAY)
											DoTeleport(partyconn, 3352, 1334);
									}

									GenerateMob(RUNEQUEST_LV4_MOB_BOSS, 0, 0);
								}
							}
							else if (partyleader == Pista[4].Party[1].LeaderID && Pista[4].Party[0].MobCount >= 1 && Pista[4].Party[2].MobCount >= 1)
							{
								Pista[4].Party[1].MobCount--;

								if (Pista[4].Party[1].MobCount == 0)
								{
									DoTeleport(partyleader, 3351, 1334);

									for (int i = 0; i < MAX_PARTY; i++)
									{
										int partyconn = pMob[partyleader].PartyList[i];

										if (partyconn > 0 && partyconn < MAX_USER && partyconn != partyleader && pUser[partyconn].Mode == USER_PLAY)
											DoTeleport(partyconn, 3351, 1334);
									}

									GenerateMob(RUNEQUEST_LV4_MOB_BOSS, 0, 0);
								}
							}
							else if (partyleader == Pista[4].Party[2].LeaderID && Pista[4].Party[1].MobCount >= 1 && Pista[4].Party[0].MobCount >= 1)
							{
								Pista[4].Party[2].MobCount--;

								if (Pista[4].Party[2].MobCount == 0)
								{
									DoTeleport(partyleader, 3351, 1334);

									for (int i = 0; i < MAX_PARTY; i++)
									{
										int partyconn = pMob[partyleader].PartyList[i];

										if (partyconn > 0 && partyconn < MAX_USER && partyconn != partyleader && pUser[partyconn].Mode == USER_PLAY)
											DoTeleport(partyconn, 3351, 1334);
									}

									GenerateMob(RUNEQUEST_LV4_MOB_BOSS, 0, 0);
								}
							}
						}

						if (GenerateID == RUNEQUEST_LV4_MOB_BOSS)
						{
							int partyleader = pMob[conn].Leader;

							if (partyleader <= 0)
								partyleader = conn;

							STRUCT_ITEM Runa;
							memset(&Runa, 0, sizeof(STRUCT_ITEM));

							Runa.sIndex = PistaRune[4][rand() % 3];

							if (partyleader > 0 && partyleader < MAX_USER)
								PutItem(partyleader, &Runa);

							for (int i = 0; i < MAX_PARTY; i++)
							{
								int partymember = pMob[partyleader].PartyList[i];

								if (pUser[partymember].Mode != USER_PLAY)
									continue;

								Runa.sIndex = PistaRune[4][rand() % 3];
								PutItem(partymember, &Runa);
							}

							STRUCT_ITEM NextPista;
							memset(&NextPista, 0, sizeof(STRUCT_ITEM));

							NextPista.sIndex = 5134;
							NextPista.stEffect[0].cEffect = 43;
							NextPista.stEffect[0].cValue = 5;

							PutItem(partyleader, &NextPista);

							sprintf(temp, "etc,questRune +4 complete leader:%s", pMob[partyleader].MOB.MobName);
							Log(temp, "-system", 0);
						}
#pragma endregion
#pragma region Pista Nv5 Balrog
						if (GenerateID == RUNEQUEST_LV5_MOB_BOSS)
						{
							Pista[5].Party[0].MobCount = 1;

							int partyleader = pMob[conn].Leader;

							if (partyleader <= 0)
								partyleader = conn;

							STRUCT_ITEM Runa;
							memset(&Runa, 0, sizeof(STRUCT_ITEM));

							Runa.sIndex = PistaRune[5][rand() % 6];

							if (partyleader > 0 && partyleader < MAX_USER)
								PutItem(partyleader, &Runa);

							for (int i = 0; i < MAX_PARTY; i++)
							{
								int partymember = pMob[partyleader].PartyList[i];

								if (pUser[partymember].Mode != USER_PLAY)
									continue;

								Runa.sIndex = PistaRune[5][rand() % 6];
								PutItem(partymember, &Runa);
							}

							STRUCT_ITEM NextPista;
							memset(&NextPista, 0, sizeof(STRUCT_ITEM));

							NextPista.sIndex = 5134;
							NextPista.stEffect[0].cEffect = 43;
							NextPista.stEffect[0].cValue = 6;

							PutItem(partyleader, &NextPista);

							sprintf(temp, "etc,questRune +5 complete leader:%s", pMob[partyleader].MOB.MobName);
							Log(temp, "-system", 0);
						}
#pragma endregion
#pragma region Pista Nv6 Coelho
						if (GenerateID >= RUNEQUEST_LV6_MOB_INITIAL && GenerateID <= RUNEQUEST_LV6_MOB_END)
						{
							if (Pista[6].Party[0].MobCount == 1)
							{
								Pista[6].Party[0].MobCount--;

								GenerateMob(RUNEQUEST_LV6_MOB_BOSS, 0, 0);
							}

							else if (Pista[6].Party[0].MobCount > 0)
								Pista[6].Party[0].MobCount--;
						}

						if (GenerateID == RUNEQUEST_LV6_MOB_BOSS)
						{
							int partyleader = pMob[conn].Leader;

							if (partyleader <= 0)
								partyleader = conn;

							STRUCT_ITEM Runa;
							memset(&Runa, 0, sizeof(STRUCT_ITEM));

							Runa.sIndex = PistaRune[6][rand() % 8];

							if (partyleader > 0 && partyleader < MAX_USER)
								PutItem(partyleader, &Runa);

							for (int i = 0; i < MAX_PARTY; i++)
							{
								int partymember = pMob[partyleader].PartyList[i];

								if (pUser[partymember].Mode != USER_PLAY)
									continue;

								Runa.sIndex = PistaRune[6][rand() % 8];
								PutItem(partymember, &Runa);
							}

							sprintf(temp, "etc,questRune +6 complete leader:%s", pMob[partyleader].MOB.MobName);
							Log(temp, "-system", 0);
						}
#pragma endregion
#pragma endregion
#pragma region Coliseu (N)
						else if (GenerateID >= 4623 && GenerateID <= 4634)
						{
							STRUCT_ITEM item;
							memset(&item, 0, sizeof(STRUCT_ITEM));

							int _rand = rand() % 14;

							if (_rand == 0)
								item.sIndex = 419;

							else if (_rand == 1)
								item.sIndex = 420;

							else if (_rand == 2)
								item.sIndex = 4026;

							if (item.sIndex)
							{
								SetItemBonus(&item, 0, 0, 0);

								PutItem(conn, &item);
							}
						}
#pragma endregion
#pragma region Quest Diaria 
								if (pUser[conn].QuestAtiva != 0)
								{
									char msg[126];

									STRUCT_QUESTDIARIA packet;
									memset(&packet, 0, sizeof(STRUCT_QUESTDIARIA));
									packet.Type = 0xD01;
									packet.ID = conn;
									packet.Size = sizeof(STRUCT_QUESTDIARIA);

									if (pMob[target].MOB.Equip[0].sIndex == QuestDiaria[pUser[conn].QuestAtiva].IdMob1 && pUser[conn].CountMob1 < QuestDiaria[pUser[conn].QuestAtiva].QtdMob1)
									{
										pUser[conn].CountMob1++;

										packet.IndexQuest = pUser[conn].QuestAtiva;
										packet.Count1 = pUser[conn].CountMob1;
										packet.Count2 = pUser[conn].CountMob2;
										packet.Count3 = pUser[conn].CountMob3;

										packet.CountMax1 = QuestDiaria[packet.IndexQuest].QtdMob1;
										packet.CountMax2 = QuestDiaria[packet.IndexQuest].QtdMob2;
										packet.CountMax3 = QuestDiaria[packet.IndexQuest].QtdMob3;

										SendCounterMob(conn, pUser[conn].CountMob1, QuestDiaria[pUser[conn].QuestAtiva].QtdMob1);

										pUser[conn].cSock.SendOneMessage((char*)&packet, sizeof(STRUCT_QUESTDIARIA));
									}

									if (pMob[target].MOB.Equip[0].sIndex == QuestDiaria[pUser[conn].QuestAtiva].IdMob2 && pUser[conn].CountMob2 < QuestDiaria[pUser[conn].QuestAtiva].QtdMob2)
									{
										pUser[conn].CountMob2++;

										packet.IndexQuest = pUser[conn].QuestAtiva;
										packet.Count1 = pUser[conn].CountMob1;
										packet.Count2 = pUser[conn].CountMob2;
										packet.Count3 = pUser[conn].CountMob3;

										packet.CountMax1 = QuestDiaria[packet.IndexQuest].QtdMob1;
										packet.CountMax2 = QuestDiaria[packet.IndexQuest].QtdMob2;
										packet.CountMax3 = QuestDiaria[packet.IndexQuest].QtdMob3;

										SendCounterMob(conn, pUser[conn].CountMob2, QuestDiaria[pUser[conn].QuestAtiva].QtdMob2);

										pUser[conn].cSock.SendOneMessage((char*)&packet, sizeof(STRUCT_QUESTDIARIA));
									}


									if (pMob[target].MOB.Equip[0].sIndex == QuestDiaria[pUser[conn].QuestAtiva].IdMob3 && pUser[conn].CountMob3 < QuestDiaria[pUser[conn].QuestAtiva].QtdMob3)
									{
										pUser[conn].CountMob3++;

										packet.IndexQuest = pUser[conn].QuestAtiva;
										packet.Count1 = pUser[conn].CountMob1;
										packet.Count2 = pUser[conn].CountMob2;
										packet.Count3 = pUser[conn].CountMob3;

										packet.CountMax1 = QuestDiaria[packet.IndexQuest].QtdMob1;
										packet.CountMax2 = QuestDiaria[packet.IndexQuest].QtdMob2;
										packet.CountMax3 = QuestDiaria[packet.IndexQuest].QtdMob3;

										SendCounterMob(conn, pUser[conn].CountMob3, QuestDiaria[pUser[conn].QuestAtiva].QtdMob3);

										pUser[conn].cSock.SendOneMessage((char*)&packet, sizeof(STRUCT_QUESTDIARIA));
									}

									if (pUser[conn].CountMob1 == QuestDiaria[pUser[conn].QuestAtiva].QtdMob1)
									{
										if (pUser[conn].CountMob2 == QuestDiaria[pUser[conn].QuestAtiva].QtdMob2) // Adicionar o 3 mob aqui
										{
											if (pUser[conn].CountMob3 == QuestDiaria[pUser[conn].QuestAtiva].QtdMob3)
											{
												STRUCT_ITEM item;
												memset(&item, 0, sizeof(STRUCT_ITEM));

												pMob[conn].MOB.Exp += QuestDiaria[pUser[conn].QuestAtiva].ExpReward;
												pMob[conn].MOB.Coin += QuestDiaria[pUser[conn].QuestAtiva].GoldReward;

												sprintf(temp, "Adquiriu %d de exp", QuestDiaria[pUser[conn].QuestAtiva].GoldReward);
												SendMsgExp(conn, temp, TNColor::Default, false);

												sprintf(temp, "Adquiriu %d de gold", QuestDiaria[pUser[conn].QuestAtiva].GoldReward);
												SendMsgExp(conn, temp, TNColor::Yellow, false);

												item.sIndex = QuestDiaria[pUser[conn].QuestAtiva].Item->sIndex;
												item.stEffect[0].cEffect = QuestDiaria[pUser[conn].QuestAtiva].Item->stEffect[0].cEffect;
												item.stEffect[0].cValue = QuestDiaria[pUser[conn].QuestAtiva].Item->stEffect[0].cValue;
												item.stEffect[1].cEffect = QuestDiaria[pUser[conn].QuestAtiva].Item->stEffect[1].cEffect;
												item.stEffect[1].cValue = QuestDiaria[pUser[conn].QuestAtiva].Item->stEffect[1].cValue;
												item.stEffect[2].cEffect = QuestDiaria[pUser[conn].QuestAtiva].Item->stEffect[2].cEffect;
												item.stEffect[2].cValue = QuestDiaria[pUser[conn].QuestAtiva].Item->stEffect[2].cValue;

												PutItem(conn, &item);

												sprintf(msg, "!Você recebeu [%s (qnt. %d)] por concluir a missão diária.", g_pItemList[item.sIndex].Name, item.stEffect[0].cValue);
												SendClientMessage(conn, msg);

												pUser[conn].QuestAtiva = 0;

												packet.IndexQuest = pUser[conn].QuestAtiva;

												pUser[conn].cSock.SendOneMessage((char*)&packet, sizeof(STRUCT_QUESTDIARIA));
											}
										}
									}
								}
#pragma endregion		
#pragma region RvR
								if (GenerateID == TORRE_RVR_BLUE || GenerateID == TORRE_RVR_RED)
								{
									ClearArea(1020, 1916, 1286, 2178);

									for (int i = MAX_USER; i < MAX_MOB; i++)
									{
										if (pMob[i].GenerateIndex == TORRE_RVR_BLUE || pMob[i].GenerateIndex == TORRE_RVR_RED)
											DeleteMob(i, 1);
									}

									int winner = 0;

									if (pMob[conn].MOB.Clan)
									{
										if (pMob[conn].MOB.Clan == 8)
											sprintf(temp, g_pMessageStringTable[_SN_KINGDOMWAR_DROP_], g_pMessageStringTable[_NN_KINGDOM_RED]);
										winner = 8;

										if (pMob[conn].MOB.Clan == 7)
											sprintf(temp, g_pMessageStringTable[_SN_KINGDOMWAR_DROP_], g_pMessageStringTable[_NN_KINGDOM_BLUE]);
										winner = 7;

										SendNotice(temp);
										RvRBonus = pMob[conn].MOB.Clan;
										g_pRvrWar.Bonus = winner;
									}

									MSG_STANDARDPARM sm;
									memset(&sm, 0, sizeof(MSG_STANDARDPARM));

									sm.Type = 0x899;
									sm.ID = 0;
									sm.Parm = winner;

									DBServerSocket.SendOneMessage((char*)&sm, sizeof(MSG_STANDARDPARM));

									STRUCT_ITEM Item;

									if (winner != 0)
									{
										for (int i = 1; i < MAX_USER; i++)
										{
											if (pUser[i].Mode == USER_PLAY && pMob[i].MOB.Clan == winner)

											memset(&Item, 0, sizeof(STRUCT_ITEM));
											Item.sIndex = 416;
											Item.stEffect[0].cEffect = 61;
											Item.stEffect[0].cValue = 5;
											PutItem(pUser[i].Mode == USER_PLAY, &Item);

											pMob[i].GetCurrentScore(i);
											sprintf(temp, "etc,RvR clan winner:%d", pMob[conn].MOB.Clan);
											Log(temp, "-system", 0);
										}
									}
									g_pRvrWar.Status = 0;
								}
#pragma endregion
#pragma region Drop Gold
							int MobCoin = pMob[target].MOB.Coin;
							int UNKGOLD = 18;

							if (pMob[target].MOB.BaseScore.Level < 10)
								UNKGOLD = 2;
							else if (pMob[target].MOB.BaseScore.Level < 20)
								UNKGOLD = 4;
							else if (pMob[target].MOB.BaseScore.Level < 30)
								UNKGOLD = 6;
							else if (pMob[target].MOB.BaseScore.Level < 50)
								UNKGOLD = 9;

							UNKGOLD = rand() % (UNKGOLD+1);

							if (MobCoin && UNKGOLD == 0)
							{
								MobCoin = 4 * (rand() % (((MobCoin + 1) / 4)+1) + (MobCoin + 1) / 4 + MobCoin);

								if (MobCoin > 2000)
									MobCoin = 2000;

								if (MobCoin + pMob[conn].MOB.Coin > 2000000000)
									SendClientMessage(conn, g_pMessageStringTable[_NN_Cant_get_more_than_2G]);
								else
								{
									pMob[conn].MOB.Coin += MobCoin;
									SendEtc(conn);
								}
							}
						
#pragma endregion						
#pragma region Drop Evento em todos os mobs
							if (evOn && evStartIndex && evEndIndex && evItem && evRate && evCurrentIndex < evEndIndex && rand() % evRate == 0)
							{
								STRUCT_ITEM item;
								memset(&item, 0, sizeof(STRUCT_ITEM));

								item.sIndex = evItem;

								if (evIndex)
								{
									item.stEffect[0].cEffect = 62;
									item.stEffect[0].cValue = evCurrentIndex / 256;
									item.stEffect[1].cEffect = 63;
									item.stEffect[1].cValue = evCurrentIndex;
									item.stEffect[2].cEffect = 59;
									item.stEffect[2].cValue = rand();

									sprintf(temp, g_pMessageStringTable[_SSD_S_get_S_D], pMob[conn].MOB.MobName, g_pItemList[evItem].Name, evCurrentIndex);
								}
								else
									sprintf(temp, g_pMessageStringTable[_SSD_S_get_S], pMob[conn].MOB.MobName, g_pItemList[evItem].Name);

								if (evNotice)
									SendNotice(temp);

								evCurrentIndex++;

								SetItemBonus(&item, pMob[target].MOB.CurrentScore.Level, 0, 0);
								PutItem(conn, &item);
								DrawConfig(1);
							}
#pragma endregion
							int target_level = pMob[target].MOB.CurrentScore.Level;
#pragma region Drop comum
							for (int i = 0; i < MAX_CARRY; i++)
							{
								if (pMob[target].MOB.Carry[i].sIndex == 0)
									continue;

								int droprate = g_pDropRate[i];
								int dropbonus = g_pDropBonus[i] + pMob[conn].DropBonus;

								if (dropbonus != 100)
								{
									dropbonus = 10000 / (dropbonus + 1);
									droprate = dropbonus * droprate / 100;
								}

								int pos = i / 8;

								if (i < 60)
								{
									if (pos == 0 || pos == 1 || pos == 2)
									{
										if (target_level < 10)
											droprate = 4 * droprate / 100;

										else if (target_level < 20)
											droprate = 5 * droprate / 100;

										else if (target_level < 30)
											droprate = 6 * droprate / 100;

										else if (target_level < 40)
											droprate = 7 * droprate / 100;

										else if (target_level < 60)
											droprate = 8 * droprate / 100;

										else
											droprate = 99 * droprate / 100;
									}
								}

								else
								{
									if (target_level < 170)
										droprate = 90 * droprate / 100;

									else if (target_level < 200)
										droprate = 60 * droprate / 100;

									else if (target_level < 230)
										droprate = 50 * droprate / 100;

									else if (target_level < 255)
										droprate = 43 * droprate / 100;

									else if (target_level < 320)
										droprate = 38 * droprate / 100;

									else
										droprate = 50 * droprate / 100;
								}

								if (TESTSERVER)
									droprate /= 2;

								if (LOCALSERVER)
									droprate /= 100;

								if (pMob[conn].MOB.Rsv & 4)
								{
									int special2 = pMob[conn].MOB.CurrentScore.Special[2];
									special2 = special2 / 10 + 10;
									special2 = 100 - special2;
									droprate = special2 * droprate / 100;
								}

								if (i == 8 || i == 9 || i == 10)
									droprate = 4;

								if (i == 11)
									droprate = 1;

								if (droprate >= 32000)
									droprate = 32000;

								if (droprate <= 0)
									droprate = 0;
								else
									droprate = rand() % droprate;

								if (droprate == 0 || i == 11)
								{
									STRUCT_ITEM *item = &pMob[target].MOB.Carry[i];

									if (item->sIndex <= 390 || item->sIndex >= MAX_ITEMLIST)
										continue;

									if (item->sIndex == 454)
										continue;

									int reqlv = g_pItemList[item->sIndex].ReqLvl;

									if (reqlv < 140 || (droprate % 2) != 1)
									{
										int bonus = 0;

										if (conn > 0 && conn < MAX_USER)
											bonus = pMob[conn].DropBonus;

										SetItemBonus(item, pMob[target].MOB.CurrentScore.Level, 0, bonus);


										if(CCastleZakum::KeyDrop(target, conn, PosX, PosY, item) == TRUE)
											PutItem(conn, item);

										sprintf(temp, "MobName:%s dropou o item: %s:%d %d.%d.%d.%d.%d.%d do mob:%s", pMob[conn].MOB.MobName, g_pItemList[item->sIndex].Name, item->sIndex, item->stEffect[0].cEffect, item->stEffect[0].cValue, item->stEffect[1].cEffect, item->stEffect[1].cValue, item->stEffect[2].cEffect, item->stEffect[2].cValue, pMob[target].MOB.MobName);

										if (conn > 0 && conn < MAX_USER)
											Log(temp, pUser[conn].AccountName, pUser[conn].IP);

										MSG_STANDARDPARM updateItemDayLog;
										memset(&updateItemDayLog, 0, sizeof(MSG_STANDARDPARM));

										updateItemDayLog.Size = sizeof(MSG_STANDARDPARM);
										updateItemDayLog.Type = _MSG_DBItemDayLog;
										updateItemDayLog.ID = 0;
										updateItemDayLog.Parm = item->sIndex;

										DBServerSocket.SendOneMessage((char*)&updateItemDayLog, sizeof(MSG_STANDARDPARM));
										

										if (LOCALSERVER)
										{
											int item_index = item->sIndex;

											int reqlv = g_pItemList[item_index].ReqLvl;
											int itempos = g_pItemList[item_index].nPos;

											if (itempos & 0xFE && itempos != 128 && conn < MAX_USER)
											{
												sprintf(temp, "%-12s - %s(%d:%d) %d(%d:%d)",
													pMob[target].MOB.MobName, g_pItemList[item_index].Name,
													pMob[target].MOB.CurrentScore.Level, reqlv, g_dLevel, g_dLevel1,
													g_dLevel2);

												SendSay(conn, temp);
											}
										}

										if (i == 8 || i == 9 || i == 10)
											i = 11;
									}
								}
							}
#pragma endregion
#pragma region Quest Amuleto mistico
							if (conn < MAX_USER && (pMob[target].MOB.Equip[0].sIndex == 239 || pMob[target].MOB.Equip[0].sIndex == 241) && !(rand() % 20))
							{
								if (pMob[conn].extra.QuestInfo.Mortal.TerraMistica == 1)
								{
									SendClientMessage(conn, g_pMessageStringTable[_NN_Watching_Town_Success]);
									pMob[conn].extra.QuestInfo.Mortal.TerraMistica = 2;
								}
							}
#pragma endregion
#pragma region Drenagem de HP do mob com skill de HT
							/*							if (pMob[conn].MOB.Rsv & 2)
							{
							int mob_maxhp = pMob[target].MOB.CurrentScore.MaxHp;
							int myspecial = pMob[conn].MOB.CurrentScore.Special[3];

							mob_maxhp = myspecial + mob_maxhp / 30;
							int myhp = pMob[conn].MOB.CurrentScore.Hp;

							pMob[conn].MOB.CurrentScore.Hp += mob_maxhp;

							if (pMob[conn].MOB.CurrentScore.Hp > pMob[conn].MOB.CurrentScore.MaxHp)
							pMob[conn].MOB.CurrentScore.Hp = pMob[conn].MOB.CurrentScore.MaxHp;

							if (myhp != pMob[conn].MOB.CurrentScore.Hp)
							{
							if (conn > 0 && conn < MAX_USER)
							{
							SetReqHp(conn);
							SetReqMp(conn);
							}
							MSG_SetHpDam hpDam;

							hpDam.Type = _MSG_SetHpDam;
							hpDam.Size = sizeof(MSG_SetHpDam);
							hpDam.ID = conn;
							hpDam.Hp = pMob[conn].MOB.CurrentScore.Hp;
							hpDam.Dam = pMob[conn].MOB.CurrentScore.Hp - myhp;

							GridMulticast(pMob[conn].TargetX, pMob[conn].TargetY, (MSG_STANDARD*)&hpDam, 0);
							}
							}*/
#pragma endregion
							DeleteMob(target, 1);

#pragma region Pista +1 - Lugefer_Inf
							if (GenerateID >= RUNEQUEST_LV1_MOB_INITIAL && GenerateID <= RUNEQUEST_LV1_MOB_END)
								GenerateMob(GenerateID, 0, 0);
#pragma endregion
#pragma region Pista +2 - Amon_Inf
							if (GenerateID >= RUNEQUEST_LV2_MOB_INITIAL && GenerateID <= RUNEQUEST_LV2_MOB_END)
								GenerateMob(GenerateID, 0, 0);
#pragma endregion
#pragma region Pista +3 - Kalintz_Mago
							if (GenerateID >= RUNEQUEST_LV3_MOB_BOSS_INITIAL && GenerateID <= RUNEQUEST_LV3_MOB_BOSS_END && Pista[3].Party[0].LeaderID != 0 && Pista[3].Party[0].MobCount != 0)
							{
								GenerateMob(RUNEQUEST_LV3_MOB_BOSS_INITIAL + rand() % 7, 0, 0);

								int leader = pMob[conn].Leader;

								int partyleader = pMob[conn].Leader;

								if (partyleader <= 0)
									partyleader = conn;

								if (partyleader == Pista[3].Party[0].LeaderID)
									Pista[3].Party[0].MobCount++;

								else if (partyleader == Pista[3].Party[1].LeaderID)
									Pista[3].Party[1].MobCount++;

								else if (partyleader == Pista[3].Party[2].LeaderID)
									Pista[3].Party[2].MobCount++;
							}
#pragma endregion
#pragma region Kefra - Mago_Negro
							if (GenerateIndex >= KEFRA_MOB_INITIAL && GenerateIndex <= KEFRA_MOB_END && KefraLive == 0)
								GenerateMob(GenerateIndex, 0, 0);
#pragma endregion
						
							CWarTower::MobKilled(target, conn, PosX, PosY);
							}
		}
						}

						else
							DeleteMob(target, 1);
					}
				}
					else
					{
						GridMulticast(pMob[target].TargetX, pMob[target].TargetY, (MSG_STANDARD*)&sm, 0);
						DeleteMob(target, 1);
					}
				}

				else
				{
					GridMulticast(pMob[target].TargetX, pMob[target].TargetY, (MSG_STANDARD*)&sm, 0);
					DeleteMob(target, 1);
				}
			}
			else
			{
				pUser[target].ReqHp = pMob[target].MOB.CurrentScore.MaxHp;
				//SetReqHp(target);

				pMob[target].MOB.CurrentScore.Hp = pMob[target].MOB.CurrentScore.MaxHp;
				SendScore(target);
			}
		}
#pragma endregion

#pragma region PvP
	else
	{
		int tlevel = pMob[target].MOB.BaseScore.Level;

		if (tlevel < 0 || tlevel > MAX_LEVEL)
			return;

#pragma region Lose EXP
		unsigned int curexp = (unsigned int)(pMob[target].extra.ClassMaster == MORTAL || pMob[target].extra.ClassMaster == ARCH ? g_pNextLevel[tlevel] : g_pNextLevel_2[tlevel]);
		unsigned int nextexp = (unsigned int)(pMob[target].extra.ClassMaster == MORTAL || pMob[target].extra.ClassMaster == ARCH ? g_pNextLevel[tlevel + 1] : g_pNextLevel_2[tlevel + 1]);
		unsigned int alphaexp = (nextexp - curexp);
		unsigned int deltaexp = (nextexp - curexp) / 20;

		if (tlevel >= 30)
			deltaexp = alphaexp / 22;

		if (tlevel >= 40)
			deltaexp = alphaexp / 25;

		if (tlevel >= 50)
			deltaexp = alphaexp / 30;

		if (tlevel >= 60)
			deltaexp = alphaexp / 35;

		if (tlevel >= 70)
			deltaexp = alphaexp / 40;

		if (tlevel >= 80)
			deltaexp = alphaexp / 45;

		if (tlevel >= 90)
			deltaexp = alphaexp / 50;

		if (tlevel >= 100)
			deltaexp = alphaexp / 55;

		if (tlevel >= 150)
			deltaexp = alphaexp / 70;

		if (tlevel >= 200)
			deltaexp = alphaexp / 85;

		if (tlevel >= 250)
			deltaexp = alphaexp / 100;

		if (deltaexp < 0)
			deltaexp = 0;
		if (deltaexp > 150000)
			deltaexp = 150000;
#pragma endregion

		int killer_curkill = GetCurKill(conn);
		int killer_totkill = GetTotKill(conn);
		int killer_pkpoint = GetPKPoint(conn);
		int killed_pkpoint = GetPKPoint(target);
		int killed_guilty = GetGuilty(target);
		int killed_curkill = GetCurKill(target);
		int killed_clan = pMob[target].MOB.Clan;
		int killer_clan = pMob[conn].MOB.Clan;
		int SameClan = 0;
		int AtWar = 0;

		if (killed_pkpoint > 10 && killed_pkpoint <= 25)
			deltaexp *= 3;
		else
			deltaexp *= 5;

		int killed_x = pMob[target].TargetX / 128;
		int killed_y = pMob[target].TargetY / 128;

#pragma region No lose At war
		if (conn < MAX_USER)
		{
			if (TESTSERVER)
				deltaexp /= 3;

			else
				deltaexp /= 6;

			if (killed_clan == 7 && killer_clan == 8 || killed_clan == 8 && killer_clan == 7)
				SameClan = 1;

			int killed_guild = pMob[target].MOB.Guild;
			int killer_guild = pMob[conn].MOB.Guild;
			int max_guild = 65536;

			if (killed_guild > 0 && killed_guild < max_guild && killer_guild > 0 && killer_guild < max_guild
				&& g_pGuildWar[killed_guild] == killer_guild && g_pGuildWar[killer_guild] == killed_guild)
				AtWar = 1;

			if (CastleState && killed_x == 8 && killed_y == 13)
				AtWar = 1;

			if (g_pRvrWar.Status != 0)
				AtWar = 1;

			if (GTorreState != 0)
				AtWar = 1;
		}
#pragma endregion

		GridMulticast(pMob[target].TargetX, pMob[target].TargetY, (MSG_STANDARD*)&sm, 0);

		int arena = BASE_GetArena(pMob[target].TargetX, pMob[target].TargetY);
		int village = BASE_GetVillage(pMob[target].TargetX, pMob[target].TargetY);
		int ZoneUnk = 0;

		if (killed_x == 1 && killed_y == 31)

		if (arena != 5 || village != 5 || ZoneUnk)
		{
			SendClientMessage(target, g_pMessageStringTable[_NN_In_Arena_No_Exp_Loss]);

			if (arena == 5 && village != 5)
			{
				/*FILE *fpp = NULL;

				fpp = fopen("TesteDie.txt", "a+");

				fprintf(fpp, "%s %s %d %d %d %d\n", pMob[conn].MOB.MobName, pMob[target].MOB.MobName, conn, target, pMob[target].TargetX, pMob[target].TargetY);

				fclose(fpp);*/

				GetCreateMob(conn, &sCreateMob);
				GridMulticast(pMob[conn].TargetX, pMob[conn].TargetY, (MSG_STANDARD*)&sCreateMob, 0);

				SendEtc(conn);

				if (SameClan)
				SetCurKill(target, 0);

				else
				++killer_curkill;
				SetCurKill(conn, killer_curkill);

				++killer_totkill;
				SetTotKill(conn, killer_totkill);

				SetPKPoint(conn, killer_pkpoint);
			}

			if (arena >= 0 && arena < MAX_GUILDZONE)
			{
				STRUCT_ITEM item;
				memset(&item, 0, sizeof(STRUCT_ITEM));

				item.sIndex = 431;
				SetItemBonus(&item, 0, 0, 0);

				CreateItem(pMob[target].TargetX, pMob[target].TargetY, &item, rand() % 4, 1);

				GetCreateMob(conn, &sCreateMob);
				GridMulticast(pMob[conn].TargetX, pMob[conn].TargetY, (MSG_STANDARD*)&sCreateMob, 0);

				SendEtc(conn);

				if (SameClan)
					SetCurKill(target, 0);

				else
					++killer_curkill;
				SetCurKill(conn, killer_curkill);

				++killer_totkill;
				SetTotKill(conn, killer_totkill);

				SetPKPoint(conn, killer_pkpoint);
			}

			GetCreateMob(conn, &sCreateMob);
			GridMulticast(pMob[conn].TargetX, pMob[conn].TargetY, (MSG_STANDARD*)&sCreateMob, 0);

			SendEtc(conn);

			if (SameClan)
				SetCurKill(target, 0);

			else
				++killer_curkill;
			SetCurKill(conn, killer_curkill);

			++killer_totkill;
			SetTotKill(conn, killer_totkill);

			SetPKPoint(conn, killer_pkpoint);

			if (!killed_guilty)
			{
				int Lostpk = 3 * killed_pkpoint / -20;
				if (Lostpk < -9)
					Lostpk = -9;

				if (Lostpk > 0)
					Lostpk = 0;

				if (killed_guilty > 0)
					Lostpk = 0;

				killer_pkpoint += Lostpk;
				SetPKPoint(conn, killer_pkpoint);
				sprintf(temp, g_pMessageStringTable[_DD_PKPointMinus], killer_pkpoint - 75, Lostpk);
				SendClientMessage(conn, temp);
			}
		}

		else
		{
			if (TESTSERVER)
				deltaexp /= 4;

			if (SameClan)
				deltaexp += killed_curkill * deltaexp / 4;

			if (deltaexp > 30000)
				deltaexp = 30000;

			if (deltaexp < 0)
				deltaexp = 0;

			
			if ((tlevel >= 35 || pMob[target].extra.ClassMaster != MORTAL) && NewbieEventServer == 0)
			{
				if (conn >= MAX_USER)
				{
					if (DEADPOINT)
						pMob[target].extra.Hold += (unsigned int)(pMob[target].MOB.Exp > deltaexp ? deltaexp : pMob[target].MOB.Exp);
				}
				else
					pMob[target].extra.Hold += (unsigned int)(pMob[target].MOB.Exp > deltaexp ? deltaexp : pMob[target].MOB.Exp);

				if (DEADPOINT || conn < MAX_USER)
				{
					sprintf(temp, g_pMessageStringTable[_DN_lose_D_exp], deltaexp);
					SendClientMessage(target, temp);
					SendEtc(target);
				}
			}
			else if (pMob[target].MOB.CurrentScore.Level < FREEEXP && pMob[target].extra.ClassMaster == MORTAL && NewbieEventServer == 0)
				SendClientMessage(target, g_pMessageStringTable[_NN_Below_lv20_No_Exp_Loss]);

			if (SameClan)
			{
				SendEtc(conn);

				++killer_curkill;
				SetCurKill(conn, killer_curkill);

				++killer_totkill;
				SetTotKill(conn, killer_totkill);

				if (!AtWar && !killed_guilty && NewbieEventServer == 0)
				{
					int Lostpk = 3 * killed_pkpoint / -20;
					if (Lostpk < -9)
						Lostpk = -9;

					if (Lostpk > 0)
						Lostpk = 0;

					if (killed_guilty > 0)
						Lostpk = 0;

					killer_pkpoint += Lostpk;
					SetPKPoint(conn, killer_pkpoint);
					sprintf(temp, g_pMessageStringTable[_DD_PKPointMinus], killer_pkpoint - 75, Lostpk);
					SendClientMessage(conn, temp);
				}

				if (pMob[target].TargetX >= 1017 && pMob[target].TargetX <= 1290 && pMob[target].TargetY >= 1911 && pMob[target].TargetY <= 2183)
				{
					if (pMob[target].MOB.Clan == 7)
					{
						pMob[target].MOB.CurrentScore.Hp = 2;
						DoTeleport(target, 1061 + rand() % 2, 2129 + rand() % 2);
						g_pRvrWar.Points[0]++;
					}

					else if (pMob[target].MOB.Clan == 8)
					{
						pMob[target].MOB.CurrentScore.Hp = 2;
						DoTeleport(target, 1237 + rand() % 2, 1966 + rand() % 2);
						g_pRvrWar.Points[1]++;
					}
				}
			}
#pragma region PK Drop - CP
			else
			{
				if (!AtWar && NewbieEventServer == 0)
				{
					int LostPk = 3 * killed_pkpoint / -25;

					if (LostPk < -9)
						LostPk = -9;

					if (LostPk > 0)
						LostPk = 0;

					if (killed_guilty > 0)
						LostPk = 0;

					if (pMob[target].MOB.Equip[15].sIndex == 548 || pMob[target].MOB.Equip[15].sIndex == 549 || pMob[conn].MOB.Equip[15].sIndex == 548 || pMob[conn].MOB.Equip[15].sIndex == 549)
						LostPk *= 3;

					killer_pkpoint += LostPk;

					SetPKPoint(conn, killer_pkpoint);

					if (LostPk)
					{
						sprintf(temp, g_pMessageStringTable[_DD_PKPointMinus], killer_pkpoint - 75, -LostPk);
						SendClientMessage(conn, temp);
					}
#ifdef PKDrop
					if (killed_pkpoint <= 60)
					{
						int killed_loseitem = (75 - killed_pkpoint) / 10;
						int killed_numitem = 0;

						for (int i = 0; i < MAX_CARRY - 1; i++)
						{
							if (rand() % 5)
								continue;

							STRUCT_ITEM *ItemLose = &pMob[target].MOB.Carry[i];

							if (pMob[target].MOB.Carry[i].sIndex <= 0)
								continue;

							if (ItemLose->sIndex > MAX_ITEMLIST || ItemLose->sIndex == 508 || ItemLose->sIndex == 509 || ItemLose->sIndex == 522 || ItemLose->sIndex >= 526 && ItemLose->sIndex <= 531)
								continue;

							if (ItemLose->sIndex == 446)
								continue;

							int create = CreateItem(pMob[target].TargetX, pMob[target].TargetY, ItemLose, rand() % 4);

							if (create >= MAX_ITEM || create <= 0)
								continue;

							if (BASE_NeedLog(ItemLose, 0))
							{
								char logitem[256];

								BASE_GetItemCode(ItemLose, logitem);

								sprintf(temp, "cho %s", logitem);

								Log(temp, pUser[target].AccountName, pUser[target].IP);
							}

							memset(ItemLose, 0, sizeof(STRUCT_ITEM));

							MSG_CNFDropItem cnfDrop;

							cnfDrop.Type = _MSG_CNFDropItem;
							cnfDrop.Size = sizeof(MSG_CNFDropItem);

							cnfDrop.SourType = ITEM_PLACE_CARRY;
							cnfDrop.SourPos = i;

							cnfDrop.GridX = pMob[target].TargetX;
							cnfDrop.GridY = pMob[target].TargetY;

							pUser[target].cSock.SendOneMessage((char*)&cnfDrop, sizeof(MSG_CNFDropItem));

							killed_numitem++;

							if (rand() % 2 && killed_numitem < killed_loseitem)
								continue;
						}
					}
					if (killed_pkpoint <= 35)
					{
						int killed_loseitem = (killed_pkpoint + 10) / 10;

						if (killed_loseitem <= 0)
							killed_loseitem = 1;


						int drop = rand() % killed_loseitem;

						while (!drop)
						{
							int slot = rand() % 14 + 1;
							if (slot != 12)
							{
								STRUCT_ITEM *ItemLose = &pMob[target].MOB.Equip[slot];
								if (pMob[target].MOB.Equip[slot].sIndex > 0 && ItemLose->sIndex <= MAX_ITEMLIST && ItemLose->sIndex != 508 && ItemLose->sIndex != 509 && ItemLose->sIndex != 522 && (ItemLose->sIndex < 526 || ItemLose->sIndex > 531))
								{
									if (ItemLose->sIndex != 446)
									{
										int create = CreateItem(pMob[target].TargetX, pMob[target].TargetY, ItemLose, rand() % 4);
										if (create > 0 && create < 5000)
										{
											if (BASE_NeedLog(ItemLose, 0))
											{
												char itemname[256];

												BASE_GetItemCode(ItemLose, itemname);
												sprintf(temp, "cho %s", itemname);

												Log(temp, pUser[target].AccountName, pUser[target].IP);
											}
											SendItem(target, ITEM_PLACE_EQUIP, slot, &pMob[target].MOB.Equip[slot]);
											memset(ItemLose, 0, sizeof(STRUCT_ITEM));

											MSG_CNFDropItem cnfDrop;

											cnfDrop.Type = _MSG_CNFDropItem;
											cnfDrop.Size = sizeof(MSG_CNFDropItem);

											cnfDrop.SourType = ITEM_PLACE_CARRY;
											cnfDrop.SourPos = slot;

											cnfDrop.GridX = pMob[target].TargetX;
											cnfDrop.GridY = pMob[target].TargetY;

											pUser[target].cSock.SendOneMessage((char*)&cnfDrop, sizeof(MSG_CNFDropItem));
										}
									}
								}
								break;
							}
						}
					}
#endif
				}
			}
#pragma endregion

			GetCreateMob(conn, &sCreateMob);
			GridMulticast(pMob[conn].TargetX, pMob[conn].TargetY, (MSG_STANDARD*)&sCreateMob, 0);

			if (SameClan)
			SetCurKill(target, 0);

			if (killed_pkpoint < 75 && !AtWar && conn < MAX_USER)
			{
				killed_pkpoint++;
				SetPKPoint(target, killed_pkpoint);
				sprintf(temp, g_pMessageStringTable[_DD_PKPointPlus], killed_pkpoint - 75, 1);
				SendClientMessage(target, temp);
			}

			GetCreateMob(target, &sCreateMob);
			GridMulticast(pMob[target].TargetX, pMob[target].TargetY, (MSG_STANDARD*)&sCreateMob, 0);

			sprintf(temp, "etc PK killer:%s(%d,%d) killed:%s(%d,%d) kingdom:%d war:%d",
				pMob[conn].MOB.MobName, pMob[conn].MOB.CurrentScore.Level, killer_pkpoint, pMob[target].MOB.MobName,
				pMob[target].MOB.CurrentScore.Level, killed_pkpoint, SameClan, AtWar);

			Log(temp, "-system", 0);
		}

		if (pMob[target].MOB.Equip[13].sIndex == 753 || pMob[target].MOB.Equip[13].sIndex == 1726)//10X HP MOB  - 20X HP MOB
		{
			int sanc = BASE_GetItemSanc(&pMob[target].MOB.Equip[13]);
			if (sanc > 0)
			{
				sanc--;

				if (pMob[target].MOB.Equip[13].stEffect[0].cEffect == 43)
					pMob[target].MOB.Equip[13].stEffect[0].cValue = sanc;

				else if (pMob[target].MOB.Equip[13].stEffect[1].cEffect == 43)
					pMob[target].MOB.Equip[13].stEffect[1].cValue = sanc;

				else if (pMob[target].MOB.Equip[13].stEffect[2].cEffect == 43)
					pMob[target].MOB.Equip[13].stEffect[2].cValue = sanc;
			}
			else
				BASE_ClearItem(&pMob[target].MOB.Equip[13]);

			SendItem(target, ITEM_PLACE_EQUIP, 13, &pMob[target].MOB.Equip[13]);
		}
	}
#pragma endregion
}
