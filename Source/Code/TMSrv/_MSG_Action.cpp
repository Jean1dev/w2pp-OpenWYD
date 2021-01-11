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

void Exec_MSG_Action(int conn, char *pMsg)
{
	MSG_Action *m = (MSG_Action*)pMsg;

	if (pUser[conn].Mode != USER_PLAY)
	{
		SendHpMode(conn);

		return;
	}

	if (pMob[conn].MOB.CurrentScore.Hp == 0)
	{
		SendHpMode(conn);

		AddCrackError(conn, 5, 3);
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

	/*if((pMob[conn].TargetX/128) == 9 && (pMob[conn].TargetY/128) == 1 || (pMob[conn].TargetX/128) == 8 && (pMob[conn].TargetY/128) == 2 || (pMob[conn].TargetX/128) == 10 && (pMob[conn].TargetY/128) == 2)
	{
		if((pMob[conn].extra.ClassMaster == CELESTIAL || pMob[conn].extra.ClassMaster == CELESTIALCS || pMob[conn].extra.ClassMaster == SCELESTIAL) && pMob[conn].MOB.CurrentScore.Level >= 180)
		{
			DoRecall(conn);

			return;
		}
	}*/


	int movetime = m->ClientTick;
	int checktime = 0;

	if (CurrentTime <= 0x1D4C0)
		checktime = 0;
	else
		checktime = CurrentTime - 120000;

	//Skill Ilusão
	if (m->Type == _MSG_Action3)
	{
		if (pMob[conn].MOB.Class != 3 || (pMob[conn].MOB.LearnedSkill & 2) == 0)
		{
			if (pMob[conn].MOB.CurrentScore.Level < MAX_LEVEL)
				if (AddCrackError(conn, 10, 28) == TRUE)
					return;
		}

		int mana = g_pSpell[73].ManaSpent;

		if (pMob[conn].MOB.CurrentScore.Mp < mana)
		{
			SendSetHpMp(conn);
			return;
		}

		pMob[conn].MOB.CurrentScore.Mp -= mana;
		pUser[conn].ReqMp -= mana;

		if (pUser[conn].LastActionTick != 235543242 && movetime < pUser[conn].LastIllusionTick + 900)
		{
			sprintf(temp, "err,illusion %d %d 900ms limit", movetime, pUser[conn].LastIllusionTick);

			Log(temp, pUser[conn].AccountName, pUser[conn].IP);
			AddCrackError(conn, 1, 105);
			return;
		}
		if ((unsigned int)movetime > CurrentTime + 15000 || movetime < checktime)
		{
			sprintf(temp, "err,illusion %d %d 15000ms limit", movetime, CurrentTime);

			Log(temp, pUser[conn].AccountName, pUser[conn].IP);
			Log("err,illution faster than 15 sec - MSG_ACTION", pUser[conn].AccountName, pUser[conn].IP);

			AddCrackError(conn, 1, 104);
			return;
		}
		pUser[conn].LastIllusionTick = movetime;
	}

	else if (m->Type == _MSG_Action)
	{
		if (pUser[conn].LastActionTick != 235543242 && movetime < pUser[conn].LastIllusionTick + 900)
		{
			sprintf(temp, "err,illusion %d %d 900ms limit", movetime, pUser[conn].LastIllusionTick);

			Log(temp, pUser[conn].AccountName, pUser[conn].IP);
			AddCrackError(conn, 1, 103);
			return;
		}
		if ((unsigned int)movetime > CurrentTime + 15000 || movetime < checktime)
		{
			sprintf(temp, "err,illusion %d %d 15000ms limit", movetime, CurrentTime);

			Log(temp, pUser[conn].AccountName, pUser[conn].IP);
			Log("err,illution faster than 15 sec - MSG_ACTION", pUser[conn].AccountName, pUser[conn].IP);

			AddCrackError(conn, 1, 102);
			return;
		}
		pUser[conn].LastIllusionTick = movetime;
	}

	else if (m->Type == _MSG_Action2)
	{
		if (pUser[conn].LastMove == _MSG_Action2)
		{
			Log("err,can't send MSG_Stop continuously", pUser[conn].AccountName, pUser[conn].IP);
			AddCrackError(conn, 1, 101);

			return;
		}
	}

	if (m->Speed > (pMob[conn].MOB.CurrentScore.AttackRun & 0xF))
	{
		AddCrackError(conn, 5, 4);

		sprintf(temp, "etc,diffrent movement. %d-%d", m->Speed, pMob[conn].MOB.CurrentScore.AttackRun);
		Log(temp, pUser[conn].AccountName, pUser[conn].IP);

		m->Speed = pMob[conn].MOB.CurrentScore.AttackRun & 0xF;
	}

	int posx = pMob[conn].TargetX;
	int posy = pMob[conn].TargetY;
		
	if (/*m->Effect != 1 && m->Effect != 2 && */(m->TargetX < posx - VIEWGRIDX || m->TargetX  > posx + VIEWGRIDX || m->TargetY < posy - VIEWGRIDY || m->TargetY > posy + VIEWGRIDY))
	{
		if (m->TargetX < posx - VIEWGRIDX * 2 || m->TargetX > posx + VIEWGRIDX * 2 || m->TargetY < posy - VIEWGRIDY * 2 || m->TargetY > posy + VIEWGRIDY * 2)
		{
			MSG_Action sm_action;
			memset(&sm_action, 0, sizeof(MSG_Action));

			GetAction(conn, posx, posy, (MSG_Action*)&sm_action);
			sm_action.Type = _MSG_Action3;
			sm_action.Speed = 6;

			pUser[conn].cSock.SendOneMessage((char*)&sm_action, sizeof(MSG_Action));
		}
		AddCrackError(conn, 1, 5);
		return;
	}

	if (m->TargetX >= 3378 && m->TargetX <= 3398 && m->TargetY >= 1427 && m->TargetY <= 1448)
	{
		int partyleader = pMob[conn].Leader;

		if (partyleader <= 0)
			partyleader = conn;

		if(partyleader != conn || conn != Pista[2].Party[0].LeaderID && conn != Pista[2].Party[1].LeaderID && conn != Pista[2].Party[2].LeaderID)
		{
			int rd = rand()%3;
			DoTeleport(conn, PistaPos[2][rd][0], PistaPos[2][rd][1]);
			return;
		}
	}
		
	if (m->TargetX <= 0 || m->TargetX >= 4096 || m->TargetY <= 0 || m->TargetY >= 4096)
	{
		Log("err,action - viewgrid", pMob[conn].MOB.MobName, pUser[conn].IP);
		return;
	}

	/*if(pUser[conn].OnlyTrade)
	{
		int Village = BASE_GetVillage(m->TargetX, m->TargetY);
		unsigned char mapAttribute = GetAttribute(m->TargetX, m->TargetY) & 0x80;

		if ((Village < 0 || Village >= 5) && mapAttribute == 0)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_OnlyVillage]);
			DoRecall(conn);
			return;
		}
	}
	*/
	if (m->TargetX != pMob[conn].TargetX || m->TargetY != pMob[conn].TargetY)
	{
		unsigned char mapAttribute = GetAttribute(m->TargetX, m->TargetY);

		if (mapAttribute & 0x80 && (pMob[conn].MOB.CurrentScore.Level >= 35 && pMob[conn].MOB.CurrentScore.Level <= 999 || pMob[conn].extra.ClassMaster != MORTAL)) //verifica o level
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Newbie_zone]);

			DoTeleport(conn, 2091 + rand() % 3, 2101 + rand() % 3); // teleporta pra armia
			return;
		}

		if (mapAttribute & 0x20)
		{
			if (pMob[conn].MOB.CurrentScore.Level <= MAX_LEVEL)
			{
				int Zone = BASE_GetGuild(m->TargetX, m->TargetY);

				if (Zone >= 0 && Zone < MAX_GUILDZONE && pMob[conn].MOB.Guild != g_pGuildZone[Zone].ChargeGuild)
				{
					SendClientMessage(conn, g_pMessageStringTable[_NN_Only_Guild_Members]);
					DoRecall(conn);
					return;
				}
			}
		}
		
		CCastleZakum::CheckMove(conn, m->TargetX, m->TargetY);

		pUser[conn].LastMove = m->Type;

		pMob[conn].LastSpeed = 0;

		strncpy(pMob[conn].Route, m->Route, MAX_ROUTE);

		if (pMobGrid[m->TargetY][m->TargetX] && pMobGrid[m->TargetY][m->TargetX] != conn)
		{
			int destx = m->TargetX;
			int desty = m->TargetY;

			int len = strlen(m->Route);

			int i = 0;

			GetEmptyMobGrid(conn, &destx, &desty);

			for (i = len; i >= 0; i--)
			{
				if (pMobGrid[desty][destx] && pMobGrid[desty][destx] != conn)
					continue;

				BASE_GetRoute(m->PosX, m->PosY, &destx, &desty, m->Route, i, (char*)pHeightGrid);
			}

			if (i == -1)
			{
				m->TargetX = destx;
				m->TargetY = desty;

				m->Effect = 0;

				pUser[conn].cSock.SendOneMessage((char*)m, sizeof(m));
				return;
			}
			m->TargetX = destx;
			m->TargetY = desty;

			pUser[conn].cSock.AddMessage((char*)m, sizeof(m));
		}

		memcpy(pMob[conn].Route, m->Route, MAX_ROUTE);

		GridMulticast(conn, m->TargetX, m->TargetY, (MSG_STANDARD*)pMsg);
			
		if (m->Type == _MSG_Action3)
		{
			pUser[conn].cSock.SendOneMessage((char*)m, sizeof(MSG_Action));
			SendSetHpMp(conn);
		}


		int len = strlen(pMob[conn].Route);
		unsigned char Unk = 0;

		if (len > 0 && len < MAX_ROUTE-3)
		{
			Unk = pMob[conn].Route[len] + 3;
			Unk = Unk - 48;

			if (Unk < 1 || Unk > 9)
				Unk = 0;
			else
				Unk = 16 * Unk;
		}

		pMob[conn].MOB.CurrentScore.Merchant = pMob[conn].MOB.CurrentScore.Merchant | Unk & 0xF0;
		pMob[conn].MOB.BaseScore.Merchant = pMob[conn].MOB.CurrentScore.Merchant;

		if (BrState && BRItem > 0)
		{
			int tx = pMob[conn].TargetX;
			int ty = pMob[conn].TargetY;

			int lvl = pMob[conn].MOB.CurrentScore.Level;

			if (tx >= 2604 && ty >= 1708 && tx <= 2648 && ty <= 1744)
			{
				if (!BrGrid && lvl >= 100 && lvl < 1000)
					DoRecall(conn);

				if (BrGrid == 1 && lvl >= 200 && lvl < 1000)
					DoRecall(conn);
			}
		}

		if (Colo150Limit)
		{
			int lvl = pMob[conn].MOB.CurrentScore.Level;
			if (lvl >= 150)
			{
				int tx = pMob[conn].TargetX;
				int ty = pMob[conn].TargetY;

				if (tx >= 2604 && ty >= 1708 && tx <= 2648 && ty <= 1744)
					DoRecall(conn);

			}
		}
	}
}