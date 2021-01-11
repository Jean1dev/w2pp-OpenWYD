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
#ifndef __CNPCGENE__
#define __CNPCGENE__

#include "..\Basedef.h"

#define MOB_FREE 0
#define MOB_USE 1

#define MAX_SEGMENT 5

struct NPCGENLIST // 2988 total
{
	int  Mode;//0
	int  MinuteGenerate;  //4
	int  MaxNumMob;     //8
	int  CurrentNumMob;    //12
	int  MinGroup;     //16 //uNK3 COMEÇA AQUI
	int  MaxGroup;    //20
	int  SegmentListX[MAX_SEGMENT]; //20 - 40
	int  SegmentListY[MAX_SEGMENT]; //40 - 60
	int  SegmentRange[MAX_SEGMENT]; //60 - 80
	int  SegmentWait[MAX_SEGMENT]; // 80 - 100
	char SegmentAction[MAX_SEGMENT][80]; //100 - 500
	char FightAction[4][80]; //500 - 820
	char FleeAction[4][80];
	char DieAction[4][80];
	int  Formation;
	int  RouteType;
	int	 Offset;

	STRUCT_MOB   Leader;                    // ¸¿ÀÇ ¼¼ºÎ ³»¿ë.
	STRUCT_MOB   Follower;                  // ¶Ê¸¶´Ï ¼¼ºÎ³»¿ë
};

struct MAPREGION
{
	int minX;
	int minY;
	int maxX;
	int maxY;

	char RegionName[128];
};

class CNPCSummon
{	
public:
	STRUCT_MOB Mob[MAX_SUMMONLIST];

public:

	CNPCSummon(); 
	~CNPCSummon();
	void Initialize();	
};

class CNPCGenerator 
{	
public:

	NPCGENLIST pList [MAX_NPCGENERATOR];
	int NumOld		 [MAX_NPCGENERATOR];
	int NumList;

	MAPREGION pRegion[MAX_NPCGENERATOR];
	int NumRegion;

public:

	CNPCGenerator();  
	~CNPCGenerator();

	int  ReadNPCGenerator();
	int  ParseString(int i, char *str);
	void ReadRegion();
	void DropList();
	void LevelList();
};

void SetAct(char *dest, char *act);


#endif