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

void Exec_MSG_SendReqParty(int conn, char *pMsg)
{
	MSG_SendReqParty *m = (MSG_SendReqParty*)pMsg;

	int partyID = m->PartyID;
	int targetID = m->unk;

	m->MobName[NAME_LENGTH - 1] = 0;

	if (partyID <= 0 || partyID >= MAX_USER || partyID != conn)
	{
		Log("err,REQParty leader out of range", pUser[conn].AccountName, pUser[conn].IP);
		return;
	}
	if (pMob[partyID].Leader)
	{
		SendClientMessage(conn, g_pMessageStringTable[_NN_Dropped_Current_Party_First]);
		return;
	}
	if (targetID <= 0 || targetID >= MAX_USER)
	{
		Log("err,REQParty target out of range", pUser[conn].AccountName, pUser[conn].IP);
		return;
	}

	if (pUser[targetID].Mode != USER_PLAY)
	{
		SendClientMessage(conn, g_pMessageStringTable[_NN_Not_Connected]);
		return;
	}
	if (pMob[targetID].Leader)
	{
		SendClientMessage(conn, g_pMessageStringTable[_NN_Other_Partys_Member]);
		return;
	}

	/*if(pUser[conn].OnlyTrade)
	{
		SendClientMessage(conn, g_pMessageStringTable[_NN_ONLYTRADE]);
		return;
	}*/

	int index = 0;
	
	while (2)
	{
		if (index >= MAX_PARTY)
		{
			int lvl = pMob[targetID].extra.ClassMaster == ARCH || pMob[targetID].extra.ClassMaster == MORTAL ? pMob[targetID].MOB.BaseScore.Level : pMob[targetID].MOB.BaseScore.Level + MAX_CLEVEL;
			int leaderlv = pMob[conn].extra.ClassMaster == ARCH || pMob[conn].extra.ClassMaster == MORTAL ? pMob[conn].MOB.BaseScore.Level : pMob[conn].MOB.BaseScore.Level + MAX_CLEVEL;
			
			if (lvl >= leaderlv - PARTY_DIF && lvl < leaderlv + PARTY_DIF || lvl >= 1000 || leaderlv >= 1000 || pMob[targetID].extra.ClassMaster == pMob[conn].extra.ClassMaster) 
			{
				if (!BrState || conn >= MAX_USER || BRItem <= 0 || pMob[conn].TargetX < 2604
					|| pMob[conn].TargetY < 1708 || pMob[conn].TargetX > 2648 || pMob[conn].TargetY > 1744)
				{
					pMob[targetID].LastReqParty = conn;
					SendReqParty(targetID, partyID, partyID);
					break;
				}
			}
			else
			{
				SendClientMessage(conn, g_pMessageStringTable[_NN_Party_Level_Limit]);
				break;
			}
		}
		else
		{
			int mindex = pMob[targetID].PartyList[index];
			if (!mindex)
			{
				index++;
				continue;
			}
			SendClientMessage(conn, g_pMessageStringTable[_NN_Have_Own_Party_Already]);
			break;
		}
		break;
	}
}