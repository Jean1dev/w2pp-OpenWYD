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

#include "../ItemEffect.h"

#include "CMob.h"
#include "Server.h"
#include "GetFunc.h"
#include "SendFunc.h"


CMob::CMob()
{
	Mode			 = MOB_EMPTY;
	LastX			 = 0xFFFFFFFF;
	LastY			 = 0xFFFFFFFF;
	Leader			 = 0;
	WaitSec			 = 0;
	PotionCount		 = 0;
	Summoner		 = 0;
	GuildDisable	 = 0;
	LastReqParty	 = 0;
	ProcessorCounter = 0;

	memset(&Tab, 0, sizeof(Tab));
	memset(&Snd, 0, sizeof(Snd));
	memset(&extra, 0, sizeof(STRUCT_MOBEXTRA));
}

CMob::~CMob()
{

}

void CMob::Clear()
{
	Mode = MOB_EMPTY;
	LastX = 0xFFFFFFFF;
	LastY = 0xFFFFFFFF;
	TargetX = 0;
	TargetY = 0;
	Leader = 0;
	WaitSec = 0;
	PotionCount = 0;
	Summoner = 0;
	GuildDisable = 0;
	LastReqParty = 0;
	ProcessorCounter = 0;
	ClientIndex = 0;

	memset(&Recycle, 0, sizeof(STRUCT_RECYCLE));
	memset(&MOB, 0, sizeof(STRUCT_MOB));
	memset(&Tab, 0, sizeof(Tab));
	memset(&Snd, 0, sizeof(Snd));
	memset(&extra, 0, sizeof(STRUCT_MOBEXTRA));
}

void CMob::ProcessorSecTimer()
{
	//ProcessorCounter++;
}

int  CMob::StandingByProcessor(void)
{
	int rt = FALSE;

	if (RouteType == 5 || Affect[0].Type == 24)
	{
		int Face = MOB.Equip[0].sIndex;

		if (Leader == 0 && (Affect[0].Type == 24 || Face >= 315 && Face <= 345))
		{
				rt |= 0x100;
				return rt;
		}

		if (Leader <= 0 || Leader >= MAX_USER)
		{
			rt |= 1;
			return rt;
		}

		if (Affect[0].Type == 24 || Face >= 315 && Face <= 345)
		{
			int summoner = Summoner;

			if (summoner <= 0 || summoner >= MAX_USER)
			{
				rt |= 1;
				return rt;
			}

			int _leader = 0;
			if (Leader == summoner)
				_leader = 1;

			for (int i = 0; i < MAX_PARTY; ++i)
			{
				if (pMob[Leader].PartyList[i] == summoner)
					_leader = 1;
			}

			if (_leader == 0)
			{
				rt |= 1;
				return rt;
			}

			if (pUser[summoner].Mode != USER_PLAY)
			{
				rt |= 1;
				return rt;
			}

			int Distance = BASE_GetDistance(TargetX, TargetY, pMob[summoner].TargetX, pMob[summoner].TargetY);
			if (Distance >= 13)
			{
				NextX = pMob[summoner].TargetX;
				NextY = pMob[summoner].TargetY;
				rt = rt | 0x02;
				return rt;
			}

			if (Distance <= 4 || Distance >= 13)
				return rt;

			SegmentX = pMob[summoner].TargetX;
			SegmentY = pMob[summoner].TargetY;

			GetTargetPos(summoner);
			rt = rt | 0x01;
			return rt;
		}

		rt = rt |= 1;
		return rt;
	}
	
	else
	{
		if (Leader == 0)
		{
			int enemy = GetEnemyFromView();

			if (enemy && TargetX <= SegmentX + HALFGRIDX && TargetX >= SegmentX - HALFGRIDX && TargetY >= SegmentY - HALFGRIDY && TargetY <= SegmentY + HALFGRIDY)
				return enemy | 0x10000000;
			
		}
		
		if (RouteType == 6 && TargetX == SegmentX && TargetY == SegmentY)
			return 0;

		if (SegmentX == TargetX && SegmentY == TargetY)
		{
			if(SegmentProgress == 4 && RouteType == 3)
			{
				if(WaitSec <= 0)
					return 0x10000;
				else
					WaitSec -= 6;

				return 0;
			}
			if (WaitSec <= 0 || RouteType == 6)
			{
				int Seg = SegmentWait[SegmentProgress];
				if (Seg > 0)
				{
					WaitSec = Seg;
					return 0;
				}
			}
			else
			{
				WaitSec -= 6;
				if (WaitSec > 0)
				{
					if (RouteType && TargetX != SegmentListX[0] || TargetY != SegmentListY[0])
					{
						if (MOB.BaseScore.AttackRun & 0xF)
							return rt | 0x10;

						return 0;
					}
					else
						return 0;
				}
				WaitSec = 0;
			}
			int SetSeg = SetSegment();
			
			if (SetSeg == 1)
				return 0;

			if (SetSeg == 2)
				return rt | 1;

			if (SetSeg == 0x10)
				return rt | 1;
		}

		GetNextPos(0);

		if (NextX != TargetX || NextY != TargetY)
			return rt | 1;

		else
		{
			SetSegment();
			return rt;
		}
	}
	return rt;
}

int  CMob::BattleProcessor()
{
	SelectTargetFromEnemyList();

	if (CurrentTarget == 0)
	{
		Mode = MOB_PEACE;

		return 0;
	}

	if (RouteType == 5)
	{
		if (Leader <= 0 || Leader >= MAX_USER)
			return 32;

		int SummonerId = Summoner;

		if (SummonerId <= 0 || SummonerId >= MAX_USER)
			return 256;

		int IsSummon = 0;

		if (Leader == SummonerId)
			IsSummon = 1;

		for (int i = 0; i < MAX_PARTY; i++)
		{
			if (pMob[Leader].PartyList[i] == SummonerId)
				IsSummon = 1;
		}

		if (IsSummon == 0)
			return 256;

		int dis = BASE_GetDistance(TargetX, TargetY, pMob[SummonerId].TargetX, pMob[SummonerId].TargetY);

		if (dis >= 20)
		{
			NextX = pMob[SummonerId].TargetX;
			NextY = pMob[SummonerId].TargetY;
			return 2;
		}

	}
	
	int BaseInt = MOB.BaseScore.Int;
	
	if (BaseInt < rand() % 100)
	{
		return 0x010000;
	}
	
	int BaseDex = MOB.BaseScore.Dex;
	int tx = pMob[CurrentTarget].TargetX;
	int ty = pMob[CurrentTarget].TargetY;
	int Range = GenerateIndex == KEFRA_BOSS ? 25 : BASE_GetMobAbility(&MOB, EF_RANGE);
	int dis = BASE_GetDistance(TargetX, TargetY, tx, ty);

	if (RouteType != 5 && (TargetX > SegmentX + HALFGRIDX || TargetX < SegmentX - HALFGRIDX || TargetY > SegmentY + HALFGRIDY || TargetY < SegmentY - HALFGRIDY))
	{
		CurrentTarget = 0;

		for (int i = 0; i < MAX_ENEMY; i++)
			EnemyList[i] = 0;

		Mode = MOB_PEACE;

		GetNextPos(1);

		if (NextX == TargetX && NextY == TargetY)
			return 0;
		else
			return 16;
	}
	if (dis <= Range)
	{
		int Rand = rand() % 100;

		if (Range >= 4 && dis <= 4 && Rand > BaseDex)
			return 256;

		int bx = tx;
		int by = ty;

		//BASE_GetHitPosition(TargetX, TargetY, &tx, &ty, pHeightGrid);

		if (tx == bx && ty == by)
			return 4096;
		else
			return 256;
		
	}

	return 1;
}

void CMob::AddEnemyList(short TargetId)
{
	if (TargetId <= 0)
		return;

	if (TargetId <= MAX_USER)
	if (pMob[TargetId].MOB.Rsv & RSV_HIDE)
		return;

	if (TargetId <= MAX_USER)
	if (pMob[TargetId].MOB.Merchant & 1)
		return;

	int i = 0;

	for (i = 0; i < MAX_ENEMY; i++)
	if (EnemyList[i] == TargetId)
		return;

	for (i = 0; i < MAX_ENEMY; i++)
	if (EnemyList[i] == 0)
		break;

	if (i == MAX_ENEMY)
		return;

	EnemyList[i] = TargetId;

	if(GenerateIndex == KEFRA_BOSS)
	{
		for (i = 0; i < MAX_USER; i++)
		{
			if (pUser[i].Mode != USER_PLAY)
				continue;

			if (pMob[i].Mode == USER_EMPTY)
				continue;

			if (pMob[i].TargetX < (TargetX-30) || pMob[i].TargetX > (TargetX+30) || pMob[i].TargetY < (TargetY-30) || pMob[i].TargetY > (TargetY+30))
				continue;

			int c = 0;

			for (c = 0; c < MAX_ENEMY; c++)
			if (EnemyList[c] == TargetId)
				return;

			for (c = 0; c < MAX_ENEMY; c++)
			if (EnemyList[c] == 0)
				break;

			if (c == MAX_ENEMY)
				continue;

			EnemyList[c] = TargetId;
		}
	}
}

void CMob::RemoveEnemyList(short TargetId)
{
	if (TargetId <= 0)
		return;

	for (int i = 0; i < MAX_ENEMY; i++)
	{
		if (EnemyList[i] == TargetId)
		{
			EnemyList[i] = 0;

			return;
		}
	}
}

void CMob::SelectTargetFromEnemyList(void)
{
	CurrentTarget = 0;

	int Enemy[MAX_ENEMY] = { 0, 0, 0, 0 };

	for (int i = 0; i < MAX_ENEMY; i++)
		Enemy[i] = MAX_USER;

	int dis = 6;

	if (MOB.Clan == 4 || MOB.Clan == 7 || MOB.Clan == 8)
		dis = 12;

	if ((TargetX / 128) < 12 && (TargetY / 128) > 25)
		dis = 8;

	
	if (GenerateIndex == KEFRA_BOSS)
		dis = HALFGRIDX;

	for (int i = 0; i < MAX_ENEMY; i++)
	{
		int myenemy = EnemyList[i];

		if (myenemy <= 0 || myenemy >= MAX_MOB)
			continue;

		if (pMob[myenemy].Mode == MOB_EMPTY)
		{
			EnemyList[i] = 0;
			continue;
		}

		if (pMob[myenemy].MOB.CurrentScore.Hp <= 0)
		{
			EnemyList[i] = 0;
			continue;
		}

		if (myenemy >= MAX_USER)
			goto LABEL_51;

		if (pMob[myenemy].MOB.Rsv & 0x10)
		{
			EnemyList[i] = 0;
			continue;
		}
		if (pMob[myenemy].MOB.BaseScore.Level > MAX_LEVEL && ((pMob[myenemy].MOB.Merchant & 1) == 0))
		{
			EnemyList[i] = 0;
			continue;
		}

	LABEL_51:

		if (pMob[myenemy].TargetX >= TargetX - dis && pMob[myenemy].TargetX <= TargetX + dis &&
			pMob[myenemy].TargetY >= TargetY - dis && pMob[myenemy].TargetY <= TargetY + dis)
		{
			Enemy[i] = BASE_GetDistance(TargetX, TargetY, pMob[myenemy].TargetX, pMob[myenemy].TargetY);

			if (myenemy > MAX_USER)
				Enemy[i] += 2;
		}
		else
			EnemyList[i] = 0;
		
	}

	int NoTarget = MAX_USER;
	int nextenemy = 0;
	for (int i = 0; i < MAX_ENEMY; i++)
	{
		if (EnemyList[i])
		{
			if (NoTarget >= Enemy[i])
			{
				nextenemy = EnemyList[i];
				NoTarget = Enemy[i];
			}
		}
	}
	if (NoTarget == MAX_USER)
		CurrentTarget = 0;

	else
		CurrentTarget = nextenemy;
}

int CMob::SetSegment()
{
	if (RouteType == 6)
	{
		SegmentProgress = 0;
		SegmentDirection = 0;
		SegmentX = SegmentListX[SegmentProgress];
		SegmentY = SegmentListY[SegmentProgress];

		WaitSec = 0;
		return 0;
	}

	if (RouteType <= 0 && RouteType > 4)
	{
		Log("Wrong SetSegment", MOB.MobName, 0);
		return 0;
	}

	int iterator;
	while (1)
	{
		if (SegmentDirection == 0)
			SegmentProgress = SegmentProgress + 1;
		else
			SegmentProgress = SegmentProgress - 1;
		
		if (SegmentProgress == -1)
		{
			if (RouteType == 0)
			{
				SegmentProgress = 0;
				SegmentDirection = 0;
				iterator = 2;
				Log("SetSegment SegmentProgress -1 but route type 0", MOB.MobName, 0);
				break;
			}

			if (RouteType == 1)
			{
				SegmentProgress = 0;
				SegmentDirection = 0;
			}
			else if (RouteType == 2)
			{
				SegmentProgress = 0;
				SegmentDirection = 0;
			}
			else if (RouteType == 3)
			{
				iterator = 2;
				break;
			}

			else if (RouteType == 4)
			{
				Log("SetSegment SegmentProgress -1 but route type 4", MOB.MobName, 0);
				break;
			}
			continue;
		}

		if (SegmentProgress == 5)
		{
			if (RouteType == 0)
			{
				SegmentProgress = 4;
				Mode = 4;
				MOB.BaseScore.Merchant = MOB.Merchant;
				int len = strlen(Route);
				int rt = 0;

				if (len > 0)
				{
					rt = Route[len];

					rt = (rt & 0xFF) - 48;

					if ((rt & 0xFF) >= 1 && (rt & 0xFF) <= 9)
					{
						rt = (rt & 0xFF) << 4;

						MOB.Merchant = MOB.Merchant | (rt & 0xFF);
						
					}
				}

				GetCurrentScore(MAX_USER);

				iterator = 1;
				break;
			}
			
			if (RouteType == 1)
			{
				iterator = 2;
				break;
			}

			if (RouteType == 2 || RouteType == 3)
			{
				SegmentProgress = 4;
				SegmentDirection = 1;
				continue;
			}

			if (RouteType == 4)
			{
				SegmentProgress = -1;
				continue;
			}
			continue;
		}

		if (SegmentListX[SegmentProgress] == 0)
			continue;

		iterator = 0;
		break;
	}

	SegmentX = SegmentListX[SegmentProgress];
	SegmentY = SegmentListY[SegmentProgress];

	WaitSec = 0;
	return iterator;
}

void CMob::UpdateScore()
{
	if (ClientIndex <= 0) return;

	GetCurrentScore(ClientIndex);

	SendScore(ClientIndex);

	SendEtc(ClientIndex);
}

void CMob::GetCurrentScore(int idx)
{
	if (idx < MAX_USER)
		pMob[idx].MOB.Clan = 0;

	if (idx < MAX_USER && MOB.BaseScore.Level < 2000)
	{
		MOB.Resist[0] = BASE_GetMobAbility(&MOB, EF_RESIST1) >= 100 ? 100 : BASE_GetMobAbility(&MOB, EF_RESIST1);
		MOB.Resist[1] = BASE_GetMobAbility(&MOB, EF_RESIST2) >= 100 ? 100 : BASE_GetMobAbility(&MOB, EF_RESIST2);
		MOB.Resist[2] = BASE_GetMobAbility(&MOB, EF_RESIST3) >= 100 ? 100 : BASE_GetMobAbility(&MOB, EF_RESIST3);
		MOB.Resist[3] = BASE_GetMobAbility(&MOB, EF_RESIST4) >= 100 ? 100 : BASE_GetMobAbility(&MOB, EF_RESIST4);

		if(MOB.Resist[0] >= 100)
			MOB.Resist[0] = 100;

		if(MOB.Resist[1] >= 100)
			MOB.Resist[1] = 100;

		if(MOB.Resist[2] >= 100)
			MOB.Resist[2] = 100;

		if(MOB.Resist[3] >= 100)
			MOB.Resist[3] = 100;

		MOB.Equip[0].stEffect[0].cEffect = 0;
		MOB.Equip[0].stEffect[0].cValue = 0;

		MOB.Rsv = 0;



	}
	else if (idx >= MAX_USER)
	{
		int geneidx = GenerateIndex;

		if (geneidx > 0 && geneidx < MAX_NPCGENERATOR)
		{
			MOB.Resist[0] = mNPCGen.pList[geneidx].Leader.Resist[0];
			MOB.Resist[1] = mNPCGen.pList[geneidx].Leader.Resist[1];
			MOB.Resist[2] = mNPCGen.pList[geneidx].Leader.Resist[2];
			MOB.Resist[3] = mNPCGen.pList[geneidx].Leader.Resist[3];

			if(MOB.Resist[0] > 100)
				MOB.Resist[0] = 50;

			if(MOB.Resist[1] > 100)
				MOB.Resist[1] = 50;

			if(MOB.Resist[2] > 100)
				MOB.Resist[2] = 50;

			if(MOB.Resist[3] > 100)
				MOB.Resist[3] = 50;
		}

		MOB.Rsv = 0;
	}

	Parry = BASE_GetMobAbility(&MOB, EF_PARRY);

	if (idx < MAX_USER)
	{
		pUser[idx].Range = BASE_GetMobAbility(&MOB, EF_RANGE);
		pUser[idx].Range = 23;
	}

	ExpBonus = 0;
	DropBonus = 0;
	ForceDamage = 0;
	ReflectDamage = 0;
	ForceMobDamage = 0;
	Accuracy = 0;
	HpAbs = 0;
	

	BASE_GetCurrentScore(MOB, Affect, &extra, &ExpBonus, &ForceMobDamage, idx >= MAX_USER ? 1 : 0, &Accuracy, &HpAbs, &ForceDamage);

	//Fada Verde 3D
	if(MOB.Equip[13].sIndex == 3900)
		ExpBonus += 16;

	//Fada Azul 3D
	if(MOB.Equip[13].sIndex == 3901)
		DropBonus += 32;

	//Fada Vermelha
	if(MOB.Equip[13].sIndex == 3902 || MOB.Equip[13].sIndex == 3905 || MOB.Equip[13].sIndex == 3908)
	{
		ExpBonus += 32;
		DropBonus += 16;
	}

	//Fada Verde
	if(MOB.Equip[13].sIndex == 3903 || MOB.Equip[13].sIndex == 3906 || MOB.Equip[13].sIndex == 3911 || MOB.Equip[13].sIndex == 3912 || MOB.Equip[13].sIndex == 3913)
		ExpBonus += 16;

	//Fada Verde Azul 
	if(MOB.Equip[13].sIndex == 3904 || MOB.Equip[13].sIndex == 3907)
		ExpBonus += 32;

	//Concentração
	if ((MOB.LearnedSkill & (1 << 28)) != 0)
		Accuracy += 50;

	if(MOB.Resist[0] < 0 && idx < MAX_USER)
		MOB.Resist[0] = 0;

	if(MOB.Resist[1] < 0 && idx < MAX_USER)
		MOB.Resist[1] = 0;

	if(MOB.Resist[2] < 0 && idx < MAX_USER)
		MOB.Resist[2] = 0;

	if(MOB.Resist[3] < 0 && idx < MAX_USER)
		MOB.Resist[3] = 0;

	if (MOB.CurrentScore.Hp > MOB.CurrentScore.MaxHp)
		MOB.CurrentScore.Hp = MOB.CurrentScore.MaxHp;

	if (MOB.CurrentScore.Mp > MOB.CurrentScore.MaxMp)
		MOB.CurrentScore.Mp = MOB.CurrentScore.MaxMp;

	int w1 = BASE_GetItemAbility(&MOB.Equip[6], EF_DAMAGE);
	int w2 = BASE_GetItemAbility(&MOB.Equip[7], EF_DAMAGE);

	int fw1 = (w1 / 2);
	int fw2 = (w2 / 2);

	//Pericia do caçador
	if(MOB.LearnedSkill & (1 << 10) && MOB.Class == 3)
	{
		fw1 = w1;
		fw2 = w2;
	}

	//Mestre das Armas
	if(MOB.LearnedSkill & (1 << 9) && MOB.Class == 0)
	{
		fw1 = w1;
		fw2 = w2;
	}

	if(MOB.Class == 2)
	{
		if(MOB.LearnedSkill & (1 << 17))
			ReflectDamage += ((MOB.CurrentScore.Special[3] + 1) / 6);

		//Escudo do tormento
		if(MOB.LearnedSkill & (1 << 19) && g_pItemList[MOB.Equip[7].sIndex].nPos == 128)
			MOB.CurrentScore.Ac += (BASE_GetItemAbility(&MOB.Equip[7], EF_AC) + 1) / 7;
	}

	if (w1 >= w2)
		WeaponDamage = w1 + fw2;
	else
		WeaponDamage = w2 + fw1;



	int idx1 = MOB.Equip[6].sIndex;
	int nPos = g_pItemList[idx1].nPos;

	if (idx1 >= 0 || idx1 < MAX_ITEMLIST)
	{
		if (nPos == 64 || nPos == 192)
		{
			int isanc = BASE_GetItemSanc(&MOB.Equip[6]);

			if (isanc >= 9)
				WeaponDamage += 40;
		}
	}

	int idx2 = MOB.Equip[7].sIndex;
	int nPos2 = g_pItemList[idx2].nPos;

	if (idx2 >= 0 || idx2 < MAX_ITEMLIST)
	{
		if (nPos2 == 64 || nPos2 == 192)
		{
			int isanc = BASE_GetItemSanc(&MOB.Equip[7]);

			if (isanc >= 9)
				WeaponDamage += 40;
		}
	}

	for (int i = 0; i < 16; i++)
	{
		int ItemId = MOB.Equip[i].sIndex;
		int itemGem = BASE_GetItemGem(&MOB.Equip[i]);
		int itemSanc = BASE_GetItemSanc(&MOB.Equip[i]);


		if (ItemId <= 0 || ItemId > MAX_ITEMLIST)
			continue;

		if (g_pItemList[ItemId].Grade == 5)
			DropBonus += 8;

		if (g_pItemList[ItemId].Grade == 6)
			ForceDamage += i == 20;

		if (g_pItemList[ItemId].Grade == 7)
			ExpBonus += 2;

		if (g_pItemList[ItemId].Grade == 8)
			ReflectDamage += 20;

		int isanc = 0;

		if(itemSanc == REF_10)
			isanc = 1;

		else if(itemSanc == REF_11)
			isanc = 2;

		else if(itemSanc == REF_12)
			isanc = 3;

		else if(itemSanc == REF_13)
			isanc = 4;

		else if(itemSanc == REF_14)
			isanc = 5;

		else if(itemSanc == REF_15)
			isanc = 6;

		if(itemGem == 0)
			DropBonus += 8;

		if(itemGem == 1)
			ForceDamage += (g_pItemList[ItemId].Grade == 6 ? 80 : 40) * isanc;

		if(itemGem == 2)
			ExpBonus += 2;

		if(itemGem == 3)
			ReflectDamage += (g_pItemList[ItemId].Grade == 8 ? 80 : 40) * isanc;
	}
	
	PvPDamage = 0;
	ReflectPvP = 0;

	int AtaquePvP = BASE_GetMobAbility(&MOB, EF_HWORDGUILD);

	AtaquePvP = (AtaquePvP + 1) / 10;

	PvPDamage = AtaquePvP;

	int DefesaPvP = BASE_GetMobAbility(&MOB, EF_LWORDGUILD);

	DefesaPvP = (DefesaPvP + 1) / 10;

	ReflectPvP = DefesaPvP;
}

void CMob::GetTargetPosDistance(int tz)
{
	if ((MOB.BaseScore.AttackRun & 0xF) == 0)
	{
		NextX = TargetX;
		NextY = TargetY;

		return;
	}

	int speed = BASE_GetSpeed(&MOB.CurrentScore);
	int distance = (speed * SECBATTLE) / 4; // SECBATTLE(?) = 8

	if (distance >= MAX_ROUTE)
		distance = MAX_ROUTE - 1;

	LastX = TargetX;
	LastY = TargetY;
	NextX = LastX;
	NextY = LastY;

	if (pMob[tz].TargetX > LastX)
		NextX = NextX - (1 + rand() % 2);

	else if (pMob[tz].TargetX < LastX)
		NextX = NextX + (1 + rand() % 2);

	if (pMob[tz].TargetY > LastY)
		NextY = NextY - (1 + rand() % 2);
	else if (pMob[tz].TargetY < LastY)
		NextY = NextY + (1 + rand() % 2);

	GetEmptyMobGrid(0, &NextX, &NextY);


	int dis = 0;

	for (dis = distance; dis >= 0; dis--)
	{
		BASE_GetRoute(LastX, LastY, &NextX, &NextY, Route, dis, (char*)pHeightGrid);

		if (pMobGrid[NextY][NextX] == 0)
			break;

		if (dis != distance)
		{
			GetEmptyMobGrid(0, &NextX, &NextY);

			BASE_GetRoute(LastX, LastY, &NextX, &NextY, Route, dis, (char*)pHeightGrid);

			if (pMobGrid[NextY][NextX] == 0)
				break;
		}
	}

	if (dis == -1 || Route[0] == 0)
	{
		NextX = TargetX;
		NextY = TargetY;

		Route[0] = 0;
	}
}

void CMob::GetRandomPos()
{
	if ((MOB.BaseScore.AttackRun & 15) == 0)
	{
		NextX = TargetX;
		NextY = TargetY;

		return;
	}

	int speed = BASE_GetSpeed(&MOB.CurrentScore);
	int distance = speed * SECBATTLE / 4;

	if (distance >= MAX_ROUTE)
		distance = MAX_ROUTE - 1;

	LastX = TargetX;
	LastY = TargetY;
	NextX = LastX + rand() % 5 - 3;
	NextY = LastY + rand() % 5 - 3;

	GetEmptyMobGrid(0, &NextX, &NextY);

	int i = 0;

	for (i = distance; i >= 0; i--)
	{
		BASE_GetRoute(LastX, LastY, &NextX, &NextY, Route, i, (char*)pHeightGrid);

		if (pMobGrid[NextY][NextX] == 0)
			break;

		if (i != distance)
		{
			GetEmptyMobGrid(0, &NextX, &NextY);

			BASE_GetRoute(LastX, LastY, &NextX, &NextY, Route, i, (char*)pHeightGrid);

			if (pMobGrid[NextY][NextX] == 0)
				break;
		}
	}

	if (i == -1 || Route[0] == 0)
	{
		NextX = TargetX + rand() % 5 - 3;
		NextY = TargetY + rand() % 5 - 3;

		Route[0] = 0;
	}
}

void CMob::GetTargetPos(int tz)
{
	if(MOB.Equip[0].sIndex == 219 || MOB.Equip[0].sIndex == 220)
	{
		NextX = TargetX;
		NextY = TargetY;
		return;
	}

	if ((MOB.BaseScore.AttackRun & 15) == 0)
	{

		NextX = pMob[tz].TargetX-1;
		NextY = pMob[tz].TargetY-1;

		return;
	}

	int speed = BASE_GetSpeed(&MOB.CurrentScore) + 2;
	int distance = speed * SECBATTLE / 4;

	if (distance >= MAX_ROUTE)
		distance = MAX_ROUTE - 1;

	LastX = TargetX;
	LastY = TargetY;
	NextX = pMob[tz].TargetX - 1;
	NextY = pMob[tz].TargetY - 1;

	GetEmptyMobGrid(0, &NextX, &NextY);


	int i = 0;

	for (i = distance; i >= 0; i--)
	{
		BASE_GetRoute(LastX, LastY, &NextX, &NextY, Route, i, (char*)pHeightGrid);

		if (pMobGrid[NextY][NextX] == 0)
			break;

		if (i != distance)
		{
			int t = GetEmptyMobGrid(0, &NextX, &NextY);

			BASE_GetRoute(LastX, LastY, &NextX, &NextY, Route, i, (char*)pHeightGrid);

			if (pMobGrid[NextY][NextX] == 0)
				break;
		}
	}

	if (i == -1 || Route[0] == 0)
	{
		NextX = TargetX;
		NextY = TargetY;

		Route[0] = 0;
	}
}

int  CMob::CheckGetLevel()
{
	int ret = 0;
	int cls = MOB.Class;
	int CurSeg = 0;

	if (MOB.BaseScore.Level >= MAX_LEVEL)
		return 0;

	int cur = MOB.BaseScore.Level;

	int max_level = 0;

	if (extra.ClassMaster == MORTAL || extra.ClassMaster == ARCH)
		max_level = MAX_LEVEL;
	
	else if (extra.ClassMaster == CELESTIAL || extra.ClassMaster == SCELESTIAL || extra.ClassMaster == CELESTIALCS)
		max_level = MAX_CLEVEL;

	if (cur >=  max_level)
		return 0;

	long long exp = MOB.Exp;
	long long curexp = max_level == MAX_LEVEL ? g_pNextLevel[cur] : g_pNextLevel_2[cur];
	long long nextexp = max_level == MAX_LEVEL ? g_pNextLevel[cur + 1] : g_pNextLevel_2[cur + 1];
	long long deltaexp = (nextexp - curexp) / 4;
	long long Segment1 = curexp + deltaexp;
	long long Segment2 = curexp + (deltaexp * 2);
	long long Segment3 = curexp + (deltaexp * 3);
	long long Segment4 = nextexp;

	if (exp > Segment3)
		CurSeg = 3;
	else if (exp > Segment2)
		CurSeg = 2;
	else if (exp > Segment1)
		CurSeg = 1;

	if(extra.ClassMaster == CELESTIAL && (cur == 39 && extra.QuestInfo.Celestial.Lv40 == 0 || cur == 89 && extra.QuestInfo.Celestial.Lv90 == 0))
		return 0;

	if(extra.ClassMaster == ARCH && (cur == 354 && extra.QuestInfo.Arch.Level355 == 0 || cur == 369 && extra.QuestInfo.Arch.Level370 == 0))
		return 0;

	if (exp >= nextexp)
	{
		MOB.BaseScore.Level++;
		MOB.BaseScore.MaxHp = MOB.BaseScore.MaxHp + g_pIncrementHp[cls];
		MOB.BaseScore.MaxMp = MOB.BaseScore.MaxMp + g_pIncrementMp[cls];
		MOB.CurrentScore.Hp = MOB.CurrentScore.MaxHp;
		MOB.CurrentScore.Mp = MOB.CurrentScore.MaxMp;

		if (extra.ClassMaster == MORTAL)
		{
			if (MOB.BaseScore.Level >= 200)
				MOB.SkillBonus += 4;
			else
				MOB.SkillBonus += 3;

			MOB.SpecialBonus += 2;

			BASE_GetBonusScorePoint(&MOB, &extra);


			MOB.BaseScore.Ac++;
		}
		else if (extra.ClassMaster == ARCH)
		{
			if (MOB.BaseScore.Level >= 200)
				MOB.SkillBonus += 4;
			else
				MOB.SkillBonus += 3;

			MOB.SpecialBonus += 2;

			BASE_GetBonusScorePoint(&MOB, &extra);
			MOB.BaseScore.Ac++;
		}
		else if (extra.ClassMaster == CELESTIAL || extra.ClassMaster == SCELESTIAL || extra.ClassMaster == CELESTIALCS)
		{
			BASE_GetBonusScorePoint(&MOB, &extra);
			MOB.BaseScore.Ac++;
		}

		GetCurrentScore(0);

		ret = 4;
		Segment = 0;
	}
	else if (exp >= Segment3 && Segment < 3)
	{
		ret = 3;
		Segment = 3;
	}
	else if (exp >= Segment2 && Segment < 2)
	{
		ret = 2;
		Segment = 2;
	}
	else if (exp >= Segment1 && Segment < 1)
	{
		ret = 1;
		Segment = 1;
	}
	else
		return ret;

	if (ret >= 1 && ret <= 3)
	{
		MOB.CurrentScore.Hp = MOB.CurrentScore.MaxHp;
		MOB.CurrentScore.Mp = MOB.CurrentScore.MaxMp;

		GetCurrentScore(0);

		return ret;
	}

	return ret;
}

void CMob::GetNextPos(int battle)
{
	if(MOB.Equip[0].sIndex == 219 || MOB.Equip[0].sIndex == 220 || GenerateIndex == KEFRA_BOSS)
	{
		NextX = TargetX;
		NextY = TargetY;
		return;
	}

	if ((MOB.BaseScore.AttackRun & 0xF) == 0)
	{
		NextX = TargetX;
		NextY = TargetY;
		return;
	}

	int speed = BASE_GetSpeed(&MOB.CurrentScore);
	int distance = speed * SECBATTLE / 4;

	if (distance >= MAX_ROUTE)
		distance = MAX_ROUTE - 1;

	if (battle)
	{
		int str = MOB.BaseScore.Str + 1;
		int res = str;
		
		if (res > distance)
			res = distance;
	}
	else
	{
		if (distance > MOB.BaseScore.Str)
			distance = MOB.BaseScore.Str;
	}

	LastX = TargetX;
	LastY = TargetY;
	NextX = SegmentX;
	NextY = SegmentY;

	GetEmptyMobGrid(0, &NextX, &NextY);
	
	if(NextX == -1)
		return;

	int i = 0;

	for (i = distance; i >= 0; i--)
	{
		BASE_GetRoute(LastX, LastY, &NextX, &NextY, Route, i, (char*)pHeightGrid);

		if (pMobGrid[NextY][NextX] == 0)
			break;

		if (i != distance)
		{
			GetEmptyMobGrid(0, &NextX, &NextY);

			BASE_GetRoute(LastX, LastY, &NextX, &NextY, Route, i, (char*)pHeightGrid);

			if (pMobGrid[NextY][NextX] == 0)
				break;
		}
	}

	if (i == -1 || Route[0] == 0)
	{
		NextX = TargetX;
		NextY = TargetY;

		Route[0] = 0;
	}
}

void CMob::DetectFairyBuffer()
{
	if (MOB.Equip[13].sIndex == 3913)
	{
		// Bônus de experiência nos montros
		g_pFairyContent[0] = 30;

		// Bônus no drop de itens nos montros
		g_pFairyContent[1] = 20;

		// Aumento de chance na composição de Oriharucun e Lactolerium
		g_pFairyContent[2] = 10;

		// Libera o acesso para juntar itens após drop
		g_pFairyContent[3] = 1;

		/* Matriz que contém os indícios dos itens que vão ser agrupados */
		g_pFairyAmounts[0] = 412;
		g_pFairyAmounts[1] = 413;
		g_pFairyAmounts[2] = 419;
		g_pFairyAmounts[3] = 420;
		g_pFairyAmounts[4] = 4019;
		g_pFairyAmounts[5] = 4020;
		g_pFairyAmounts[6] = 4024;
		g_pFairyAmounts[7] = 4025;
		g_pFairyAmounts[8] = 4026;
		g_pFairyAmounts[9] = 4027;
		g_pFairyAmounts[10] = 4028;
		g_pFairyAmounts[11] = 4117;
		g_pFairyAmounts[12] = 4118;
		g_pFairyAmounts[13] = 4119;
		g_pFairyAmounts[14] = 4120;
		g_pFairyAmounts[15] = 4121;

	}
	else
	{
		for (int i = 0; i < MAX_FAIRY_BONUS; i++)
		{
			g_pFairyContent[i] = 0;
		}
	}
}

int CMob::GetEnemyFromView(void)
{
	int SizeX = 9;
	int SizeY = 9;

	int StartX = TargetX - 4;
	int StartY = TargetY - 4;

	if (MOB.Clan == 7 || MOB.Clan == 8)
	{
		SizeX = HALFGRIDX;
		SizeY = HALFGRIDY;
		StartX = TargetX - 6;
		StartY = TargetY - 6;
	}

	int sx1 = StartX;
	int sy1 = StartY;
	int sx2 = StartX + SizeX;
	int sy2 = StartY + SizeY;

	for (int y = sy1; y < sy2; y++)
	{
		for (int x = sx1; x < sx2; x++)
		{
			if(x < 0 || x >= MAX_GRIDX || y < 0 || y >= MAX_GRIDY)
				continue;

			int tmob = pMobGrid[y][x];

			if (x == TargetX && y == TargetY)
				continue;

			if(pMob[tmob].MOB.CurrentScore.Hp > 0 && pMob[tmob].Mode)
			{
				if ((tmob < MAX_USER && pMob[tmob].MOB.Rsv & 0x10))
					continue;

				if (MOB.Clan < 0 || MOB.Clan >= 9 || pMob[tmob].MOB.Clan < 0 || pMob[tmob].MOB.Clan >= 9)
				{
					Log("err,clan out or range", MOB.MobName, 0);
					return 0;
				}
				if (g_pClanTable[MOB.Clan][pMob[tmob].MOB.Clan] == 0)
					return tmob;
			}
		}
	}

	return 0;
}
