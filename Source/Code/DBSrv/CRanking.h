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

#pragma once

#include "..\Basedef.h"
#include <stdio.h>

enum RankPos
{
	FIRST = MAX_RANK_INDEX - 1,
	LAST = 0,
	OUT_OF_RANK = -1
};

struct GrindRankingConnId
{
	int TMSrvId;
	int PlayerId;

	GrindRankingConnId()
	{
		TMSrvId = -1;
		PlayerId = -1;
	}

	GrindRankingConnId(int tmsrvId, int playerId)
	{
		TMSrvId = tmsrvId;
		PlayerId = playerId;
	}
};

class GrindRanking
{
private:
	STRUCT_RANKING* m_GrindRank[MAX_RANK_INDEX];
	GrindRankingConnId m_GrindRankConnId[MAX_RANK_INDEX];

public:
	GrindRanking();

public:
	int getElementIndex(char* name);
	void updateElement(int rankId, STRUCT_RANKING*);
	STRUCT_RANKING* const getElement(int index);
	void setElement(int index, STRUCT_RANKING* target, bool deleteOlder = true);
	GrindRankingConnId getElementConnId(int rankId);
	void setElementConnId(int rankId, GrindRankingConnId connId);
};

class RankingSystem
{
public:
	GrindRanking grindRanking;
public:
	RankingSystem();

public:
	void loadRanking();

public:
	int readAccountsInDir(char *dir);
	int getRankingIndexFromName(char* charName);
	int tryInsertInRanking(int rankId, char* name, long long value, int classvalue, int Level, int cls, GrindRankingConnId* connId, STRUCT_RANKING* olderLastRank, GrindRankingConnId* olderLastConnId, int g, int clan);
	void sendUpdateIndividualRank(int rankId, int tmsrvId, int playerId, STRUCT_RANKING* playerRank);
	void riseRankingElement(int rankId);
	int increaseRankingElementValue(int rankId, int newValue, int classvalue);
	void sendUpdateRangeRank(int initId, int endId);
};

extern RankingSystem rankingSystem;