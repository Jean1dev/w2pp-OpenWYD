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
#ifndef __CMOB__
#define __CMOB__

#include "..\Basedef.h"

#define MAX_ENEMY 13

#define MOB_EMPTY 0  // There's no mob on the slot
#define MOB_USERDOCK    1
#define MOB_USER		2
#define MOB_IDLE        3
#define MOB_PEACE		4
#define MOB_COMBAT      5
#define MOB_RETURN		6
#define MOB_FLEE		7
#define MOB_ROAM		8
#define MOB_WAITDB		9

class CMob
{
public:
	STRUCT_MOB    MOB;//
	STRUCT_RECYCLE Recycle[_Max_Recycle_Bin_];
	STRUCT_AFFECT Affect[MAX_AFFECT];
	int DetectDeadOnCastle;
	int EffectCast;
	int Mode;
	int Leader;
    int Formation;
	int RouteType;
	int LastX;
	int LastY;
	int LastTime;
	int LastSpeed;
	int TargetX;
	int TargetY;
	int NextX;
	int NextY;
	int NextAction;
	char Route[MAX_ROUTE];
	int WaitSec;
	char Unk5[12];
	int Segment;
	int SegmentX;
	int SegmentY;
	int SegmentListX[5];
	int SegmentListY[5];
	int SegmentWait[5];
	int SegmentRange[5];
	int SegmentDirection;
	int SegmentProgress;
	int GenerateIndex;
	short CurrentTarget;
	short EnemyList[MAX_ENEMY];
	short PartyList[MAX_PARTY];
	short Unk7;
	int WeaponDamage;
	int Summoner;
	int PotionCount;
	int Parry;
	int GuildDisable;
	int DropBonus;
	int ExpBonus;
	int ForceDamage;
	int ReflectDamage;
	int ClientIndex;
	int CurrentTradeSecTimer;


	bool CheckSpellState[MAX_FORCE_TABLE];

	int g_pFairyContent[MAX_FAIRY_BONUS];
	int g_pFairyAmounts[MAX_FAIRY_BONUS];
	int CheckItemDrop;
	int CheckDrop;
	int CheckQuest;


	//********************
	STRUCT_MOBEXTRA extra;

	char Tab[36];
	char Snd[96];

	int ForceMobDamage;
	int Accuracy;
	int HpAbs;

	int PvPDamage;
	int ReflectPvP; 

	int MaxCarry;
	int QuestFlag;

	int LastReqParty;
	unsigned int ProcessorCounter;

	short DivineBlessing; // 2292 - 2293
	int RateRegen; // 2294 - 2297
public:

	CMob();
	~CMob();

	void ProcessorSecTimer();
	int StandingByProcessor(void);
	int BattleProcessor();
	void AddEnemyList(short TargetId);
	void RemoveEnemyList(short TargetId);
	void SelectTargetFromEnemyList(void);
	int SetSegment(void);                // 1:ShipMove   0:DeleteObject
	void GetCurrentScore(int idx);
	void UpdateScore();
	void GetTargetPosDistance(int tz);
	void GetRandomPos();
	void GetTargetPos(int tz);
	int  CheckGetLevel();
	void Clear();
	void GetNextPos(int battle);
	int  GetEnemyFromView(void);

	void DetectFairyBuffer();
};

extern CMob pMob[MAX_MOB];
#endif