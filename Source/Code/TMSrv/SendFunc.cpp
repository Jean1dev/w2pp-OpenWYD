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
#include <memory>

#include "..\Basedef.h"
#include "SendFunc.h"
#include "GetFunc.h"
#include "Server.h"
#include "Language.h"

void SendClientMessage(int conn, char *Message)
{
	if (conn <= 0 || conn >= MAX_USER)
		return;

	MSG_MessagePanel sm_mp;
	memset(&sm_mp, 0, sizeof(MSG_MessagePanel));

	sm_mp.Size = sizeof(MSG_MessagePanel);
	sm_mp.Type = _MSG_MessagePanel;
	sm_mp.ID = 0;

	memcpy(sm_mp.String, Message, MESSAGE_LENGTH);

	sm_mp.String[MESSAGE_LENGTH - 1] = 0;
	sm_mp.String[MESSAGE_LENGTH - 2] = 0;

	pUser[conn].cSock.AddMessage((char*)&sm_mp, sizeof(MSG_MessagePanel));
}

void SendRecycle(int conn)
{
	if (pUser[conn].Mode != USER_PLAY) return;

	MSG_Recycle m;
	memset(&m, 0, sizeof(MSG_Recycle));
	m.ID = conn;
	m.Type = 0x3E8;
	m.Size = sizeof(MSG_Recycle);

	for (int i = 0; i < _Max_Recycle_Bin_; i++)
	{
		m.Recycle[i] = pMob[conn].Recycle[i];
	}

	if (!pUser[conn].cSock.AddMessage((char*)&m, sizeof(MSG_Recycle)))
		CloseUser(conn);
}


void SendQuest(int conn)
{
	STRUCT_QUESTDIARIA packet;
	memset(&packet, 0, sizeof(STRUCT_QUESTDIARIA));

	packet.Type = 0xD08;
	packet.ID = conn;
	packet.Size = sizeof(STRUCT_QUESTDIARIA);

	packet.Count1 = pUser[conn].CountMob1;
	packet.Count2 = pUser[conn].CountMob2;
	packet.Count3 = pUser[conn].CountMob3;

	packet.CountMax1 = QuestDiaria[pUser[conn].QuestAtiva].QtdMob1;
	packet.CountMax2 = QuestDiaria[pUser[conn].QuestAtiva].QtdMob2;
	packet.CountMax3 = QuestDiaria[pUser[conn].QuestAtiva].QtdMob3;

	packet.QtdItem = QuestDiaria[pUser[conn].QuestAtiva].Item->stEffect[0].cValue;
	packet.ExpReward = QuestDiaria[pUser[conn].QuestAtiva].ExpReward;
	packet.GoldReward = QuestDiaria[pUser[conn].QuestAtiva].GoldReward;

	packet.IndexQuest = pUser[conn].QuestAtiva;

	pUser[conn].cSock.SendOneMessage((char*)&packet, sizeof(STRUCT_QUESTDIARIA));

}

void SendClientPac(int conn)
{
	STRUCT_CLIENTPAC packet;
	memset(&packet, 0, sizeof(STRUCT_CLIENTPAC));

	int value = 0;
	packet.Type = 0xD02;
	packet.ID = conn;
	packet.Size = sizeof(STRUCT_CLIENTPAC);

	if (pMob[conn].MOB.Clan == 7)
		packet.Points[0] = value++;

	if (pMob[conn].MOB.Clan == 8)
		packet.Points[1] = value++;


	pUser[conn].cSock.SendOneMessage((char*)&packet, sizeof(STRUCT_CLIENTPAC));

}

bool SendSaveBanned(int conn)
{
	if (conn < 0 || conn > MAX_USER)
		return false;

	std::ofstream outputFile(strFmt("./DataBase/Ban/%s.bin", pUser[conn].AccountName), std::ofstream::out | std::ofstream::binary);

	if (outputFile.is_open())
	{
		AccountBanned temp;
		std::memcpy(&temp, &BannedUser[conn], sizeof(AccountBanned));

		outputFile.write(reinterpret_cast<char*>(&BannedUser[conn]), sizeof(AccountBanned));
		outputFile.close();
		return true;
	}

	return false;


}



void SendNotice(char *Message)
{
	char Notice[512];

	sprintf(Notice, "not %s", Message);
	Log(Notice, "-system", NULL);

	if (Message[0] == '\'' && Message[1] == 'x')
		return;

	for (int i = 0; i < MAX_USER; i++)
	{
		if (pUser[i].Mode == USER_PLAY)
			SendClientMessage(i, Message);
	}
}

void SendNoticeChief(char *Message)
{
	char Notice[512];

	sprintf(Notice, "not %s", Message);

	Log(Notice, "-system", NULL);

	for (int i = 0; i < MAX_USER; i++)
	{
		if (pUser[i].Mode != USER_PLAY)
			continue;

		if (pMob[i].MOB.GuildLevel != 9)
			continue;

		int Guild = pMob[i].MOB.Guild;

		if (Guild <= 0)
			continue;

		int FoundCharged = 0;

		for (int j = 0; j < MAX_SERVER; j++)
		{
			for (int k = 0; k < MAX_GUILDZONE; k++)
			{
				if (ChargedGuildList[j][k] && ChargedGuildList[j][k] == Guild)
				{
					FoundCharged = 1;
					break;
				}
			}
		}

		SendClientMessage(i, Message);
	}
}

void SendSummonChief()
{
	Log("summon chief", "-system", NULL);

	for (int i = 0; i < MAX_USER; i++)
	{
		if (pUser[i].Mode != USER_PLAY)
			continue;

		if (pMob[i].MOB.GuildLevel != 9)
			continue;

		int Guild = pMob[i].MOB.Guild;

		if (Guild <= 0)
			continue;

		int FoundCharged = 0;

		int Server = 0;
		int GuildZone = 0;

		for (int j = 0; j < MAX_SERVER; j++)
		{
			for (int k = 0; k < MAX_GUILDZONE; k++)
			{
				if (ChargedGuildList[j][k] && ChargedGuildList[j][k] == Guild)
				{
					FoundCharged = 1;
					Server = j;
					GuildZone = k;

					break;
				}
			}
		}

		if (FoundCharged == 0)
			return;

		int tx = 7 * Server / 5 + 317;
		int ty = 4025 - 2 * Server % 5;

		if (Server / 5)
			tx = tx + GuildZone;
		else
			tx = tx - GuildZone;

		DoTeleport(i, tx, ty);
	}
}

void SendNoticeArea(char *Message, int x1, int y1, int x2, int y2)
{
	for (int i = 0; i < MAX_USER; i++)
	{
		if (pUser[i].Mode != USER_PLAY)
			continue;

		if (pMob[i].TargetX >= x1 && pMob[i].TargetX <= x2 && pMob[i].TargetY >= y1 && pMob[i].TargetY <= y2)
			SendClientMessage(i, Message);
	}
}

void SendGuildNotice(int Guild, char *Message)
{
	for (int i = 0; i < MAX_USER; i++)
	{
		if (pUser[i].Mode == USER_PLAY)
		{
			if (pMob[i].MOB.Guild == Guild)
				SendClientMessage(i, Message);
		}
	}
}

void SendClientMessageOk(int conn, char *Message, int Useless1, int Useless2) //Useless
{
	if (conn <= 0 || conn >= MAX_USER)
		return;

	MSG_MessageBoxOk sm_mbo;
	memset(&sm_mbo, 0, sizeof(MSG_MessageBoxOk));

	sm_mbo.Type = _MSG_MessageBoxOk;

	memcpy(sm_mbo.String, Message, MESSAGE_LENGTH);

	sm_mbo.Useless1 = Useless1;
	sm_mbo.Useless2 = Useless2;

	pUser[conn].cSock.AddMessage((char*)&sm_mbo, sizeof(MSG_MessageBoxOk));
}

void SendClientSignal(int conn, int id, unsigned short signal)
{
	MSG_STANDARD sm;
	memset(&sm, 0, sizeof(MSG_STANDARD));

	sm.Type = signal;
	sm.ID = id;

	pUser[conn].cSock.AddMessage((char*)&sm, sizeof(sm));
}

void SendClientSignalParm(int conn, int id, unsigned short signal, int parm)
{
	MSG_STANDARDPARM sm;
	memset(&sm, 0, sizeof(MSG_STANDARDPARM));

	sm.Type = signal;
	sm.ID = id;
	sm.Parm = parm;

	pUser[conn].cSock.AddMessage((char*)&sm, sizeof(sm));
}

void SendClientSignalParmCoord(int conn, int id, unsigned short signal, int parm, int x1, int y1, int x2, int y2)
{
	if (pMob[conn].TargetX >= x1 && pMob[conn].TargetX <= x2 && pMob[conn].TargetY >= y1 && pMob[conn].TargetY <= y2)
	{

		MSG_STANDARDPARM sm;
		memset(&sm, 0, sizeof(MSG_STANDARDPARM));

		sm.Type = signal;
		sm.ID = id;
		sm.Parm = parm;

		pUser[conn].cSock.AddMessage((char*)&sm, sizeof(sm));
	}
}

void SendClientSignalParm2(int conn, int id, unsigned short signal, int parm, int parm2)
{
	MSG_STANDARDPARM2 sm;
	memset(&sm, 0, sizeof(MSG_STANDARDPARM2));

	sm.Type = signal;
	sm.ID = id;
	sm.Parm1 = parm;
	sm.Parm2 = parm2;

	pUser[conn].cSock.AddMessage((char*)&sm, sizeof(sm));
}

void SendClientSignalParm3(int conn, int id, unsigned short signal, int parm, int parm2, int parm3)
{
	MSG_STANDARDPARM3 sm;
	memset(&sm, 0, sizeof(MSG_STANDARDPARM3));

	sm.Type = signal;
	sm.ID = id;
	sm.Parm1 = parm;
	sm.Parm2 = parm2;
	sm.Parm3 = parm3;

	pUser[conn].cSock.AddMessage((char*)&sm, sizeof(sm));
}

void SendClientSignalShortParm2(int conn, int id, unsigned short signal, int parm, int parm2)
{
	MSG_STANDARDSHORTPARM2 sm;
	memset(&sm, 0, sizeof(MSG_STANDARDSHORTPARM2));

	sm.Type = signal;
	sm.ID = id;
	sm.Parm1 = parm;
	sm.Parm2 = parm2;

	pUser[conn].cSock.AddMessage((char*)&sm, sizeof(sm));
}

void SyncMulticast(int conn, MSG_STANDARD *m, int bSend)
{
	for (int i = 0; i < MAX_USER; i++)
	{
		if (pUser[i].Mode == USER_PLAY && conn != i)
		{
			pUser[i].cSock.AddMessage((char*)m, m->Size);

			if (bSend)
				pUser[i].cSock.SendMessageA();
		}
	}
}

void SyncKingdomMulticast(int conn, int Kingdom, MSG_STANDARD *m, int bSend)
{
	for (int i = 0; i < MAX_USER; i++)
	{
		if (pUser[i].Mode == USER_PLAY && conn != i && pMob[i].MOB.Clan == Kingdom && pUser[i].KingChat == 0)
		{
			pUser[i].cSock.AddMessage((char*)m, m->Size);

			if (bSend)
				pUser[i].cSock.SendMessageA();
		}
	}
}

void SendCreateMob(int conn, int otherconn, int bSend)
{
	if (conn <= 0 || conn >= MAX_USER)
		return;

	if (pUser[conn].Mode != USER_PLAY)
		return;

	if (!pUser[conn].cSock.Sock)
		return;

	MSG_CreateMob sm;
	memset(&sm, 0, sizeof(MSG_CreateMob));
	MSG_CreateMobTrade sm2;
	memset(&sm2, 0, sizeof(MSG_CreateMobTrade));

	if (otherconn <= 0 || otherconn >= MAX_USER || pUser[otherconn].TradeMode != 1)
	{
		GetCreateMob(otherconn, &sm);

		if (pUser[conn].cSock.AddMessage((char*)&sm, sizeof(MSG_CreateMob)))
			pUser[conn].cSock.SendMessageA();

		return;
	}

	GetCreateMobTrade(otherconn, &sm2);

	if (pUser[conn].cSock.AddMessage((char*)&sm2, sizeof(MSG_CreateMobTrade)))
		pUser[conn].cSock.SendMessageA();
}

void SendCreateItem(int conn, int item, int bSend)
{
	MSG_CreateItem sm;
	memset(&sm, 0, sizeof(MSG_CreateItem));

	GetCreateItem(item, &sm);
	
	pUser[conn].cSock.AddMessage((char*)&sm, sizeof(MSG_CreateItem));

	if (bSend)
		pUser[conn].cSock.SendMessageA();
}

void SendChat(int conn, char *Message)
{
	MSG_MessageChat sm;
	memset(&sm, 0, sizeof(MSG_MessageChat));

	sm.Type = _MSG_MessageChat;
	sm.Size = sizeof(MSG_MessageChat);
	sm.ID = conn;
	sm.Size = sizeof(MSG_MessageChat);

	memcpy(sm.String, Message, MESSAGE_LENGTH);

	GridMulticast(pMob[conn].TargetX, pMob[conn].TargetY, (MSG_STANDARD*)&sm, conn);
}

void SendEnvEffect(int x1, int y1, int x2, int y2, int Effect, int EffectParm)
{
	MSG_EnvEffect sm;
	memset(&sm, 0, sizeof(MSG_EnvEffect));

	sm.Type = _MSG_EnvEffect;
	sm.Size = sizeof(MSG_EnvEffect);
	sm.ID = ESCENE_FIELD;
	sm.Size = sizeof(MSG_EnvEffect);

	sm.x1 = x1;
	sm.y1 = y1;
	sm.x2 = x2;
	sm.y2 = y2;

	sm.Effect = Effect;
	sm.EffectParm = EffectParm;

	/*
	for(int x = x1; x < x2; x++)
	{
		for(int y = y1; y < y2; y++)
		{
			if(x < 0 || x >= MAX_GRIDX || y < 0 || y >= MAX_GRIDY)
				continue;

			if(pMobGrid[y][x] == 0)
				continue;

			if(pMobGrid[y][x] >= MAX_USER)
				continue;

			pUser[pMobGrid[y][x]].cSock.AddMessage((char*)&sm, sizeof(MSG_EnvEffect));
		}
	}*/

	GridMulticast(x1 + ((x2 - x1) / 2), y1 + ((y2 - y1) / 2), (MSG_STANDARD*)&sm, 0);
}

void SendEnvEffectKingdom(int x1, int y1, int x2, int y2, int Effect, int EffectParm, int Clan)
{
	MSG_EnvEffect sm;
	memset(&sm, 0, sizeof(MSG_EnvEffect));

	sm.Type = _MSG_EnvEffect;
	sm.Size = sizeof(MSG_EnvEffect);
	sm.ID = ESCENE_FIELD;
	sm.Size = sizeof(MSG_EnvEffect);

	sm.x1 = x1;
	sm.y1 = y1;
	sm.x2 = x2;
	sm.y2 = y2;

	sm.Effect = Effect;
	sm.EffectParm = EffectParm;

	int HaveUser = 0;
	
	for(int x = x1; x < x2; x++)
	{
		for(int y = y1; y < y2; y++)
		{
			if(x < 0 || x >= MAX_GRIDX || y < 0 || y >= MAX_GRIDY)
				continue;

			if(pMobGrid[y][x] == 0)
				continue;

			if(pMobGrid[y][x] >= MAX_USER)
				continue;

			int tmob = pMobGrid[y][x];

			if(pMob[tmob].MOB.Clan == Clan)
				continue;

			HaveUser++;
		}
	}

	if(HaveUser)
		GridMulticast(x1 + ((x2 - x1) / 2), y1 + ((y2 - y1) / 2), (MSG_STANDARD*)&sm, 0);
}

void SendEnvEffectLeader(int x1, int y1, int x2, int y2, int Effect, int EffectParm)
{
	MSG_EnvEffect sm;
	memset(&sm, 0, sizeof(MSG_EnvEffect));

	sm.Type = _MSG_EnvEffect;
	sm.Size = sizeof(MSG_EnvEffect);
	sm.ID = ESCENE_FIELD;
	sm.Size = sizeof(MSG_EnvEffect);

	sm.x1 = x1;
	sm.y1 = y1;
	sm.x2 = x2;
	sm.y2 = y2;

	sm.Effect = Effect;
	sm.EffectParm = EffectParm;

	int HaveUser = 0;
	int bSend = 0;

	for(int x = x1; x < x2; x++)
	{
		for(int y = y1; y < y2; y++)
		{
			if(x < 0 || x >= MAX_GRIDX || y < 0 || y >= MAX_GRIDY)
				continue;

			if(pMobGrid[y][x] == 0)
				continue;

			if(pMobGrid[y][x] >= MAX_USER)
				continue;

			int tmob = pMobGrid[y][x];

			if(tmob == Pista[4].Party[0].LeaderID)
			{
				HaveUser++;
				continue;
			}

			if(tmob == Pista[4].Party[1].LeaderID)
			{
				HaveUser++;
				continue;
			}
			if(tmob == Pista[4].Party[1].LeaderID)
			{
				HaveUser++;
				continue;
			}

			bSend++;
		}
	}

	if(HaveUser == 0 && bSend)
		GridMulticast(x1 + ((x2 - x1) / 2), y1 + ((y2 - y1) / 2), (MSG_STANDARD*)&sm, 0);
}

void SendRemoveMob(int dest, int sour, int Type, int bSend)
{
	MSG_RemoveMob sm;
	memset(&sm, 0, sizeof(MSG_RemoveMob));

	sm.Type = _MSG_RemoveMob;
	sm.Size = sizeof(MSG_RemoveMob);
	sm.ID = sour;
	sm.RemoveType = Type;

	pUser[dest].cSock.AddMessage((char*)&sm, sizeof(MSG_RemoveMob));

	if (bSend)
		pUser[dest].cSock.SendMessageA();
}

void SendRemoveItem(int dest, int itemid, int bSend)
{
	MSG_DecayItem sm_deci;
	memset(&sm_deci, 0, sizeof(MSG_DecayItem));

	sm_deci.Type = _MSG_DecayItem;
	sm_deci.Size = sizeof(MSG_DecayItem);
	sm_deci.ID = ESCENE_FIELD;
	sm_deci.ItemID = 10000 + itemid;
	sm_deci.unk = 0;

	pUser[dest].cSock.AddMessage((char*)&sm_deci, sizeof(MSG_DecayItem));

	if (bSend)
		pUser[dest].cSock.SendMessageA();
}

void SendAutoTrade(int conn, int otherconn)
{
	if (conn <= 0 || conn >= MAX_USER)
		return;

	if (otherconn <= 0 || otherconn >= MAX_USER)
		return;

	if (pUser[otherconn].TradeMode == 0)
		return;

	if (pUser[otherconn].Mode != USER_PLAY)
		return;

	if (pUser[conn].Mode != USER_PLAY)
		return;

	MSG_SendAutoTrade sm;
	memset(&sm, 0, sizeof(MSG_SendAutoTrade));

	memcpy(&sm, &pUser[otherconn].AutoTrade, sizeof(MSG_SendAutoTrade));

	sm.ID = ESCENE_FIELD;

	sm.Index = otherconn;

	sm.Type = _MSG_SendAutoTrade;
	sm.Size = sizeof(MSG_SendAutoTrade);

	if (!pUser[conn].cSock.AddMessage((char*)&sm, sizeof(MSG_UpdateCarry)))
		CloseUser(conn);
}

void SendGridMob(int conn)
{
	int posX = pMob[conn].TargetX;
	int posY = pMob[conn].TargetY;

	if (conn <= 0 || conn >= MAX_USER)
		return;

	int SizeY = VIEWGRIDY;
	int SizeX = VIEWGRIDX;
	int StartX = posX - HALFGRIDX;
	int StartY = posY - HALFGRIDY;

	if ((posX - HALFGRIDX + VIEWGRIDX) >= MAX_GRIDX)
		SizeX -= (StartX + SizeX - MAX_GRIDX);

	if ((posY - HALFGRIDY + VIEWGRIDY) >= MAX_GRIDY)
		SizeY -= (StartY + SizeY - MAX_GRIDY);

	if (StartX < 0)
		StartX = 0;

	if (StartY < 0)
		StartY = 0;


	int sx1 = StartX;
	int sy1 = StartY;
	int sx2 = StartX + SizeX;
	int sy2 = StartY + SizeY;

	for (int y = sy1; y < sy2; y++)
	{
		for (int x = sx1; x < sx2; x++)
		{
			int tmob = pMobGrid[y][x];
			int titem = pItemGrid[y][x];

			if (tmob > 0 && tmob < MAX_MOB && tmob != conn)
			{
				if (pMob[tmob].Mode == MOB_EMPTY)
					pMobGrid[y][x] = 0;
				else
				{
					SendCreateMob(conn, tmob, 0);
					SendPKInfo(conn, tmob);
				}
			}

			if (titem > 0 && titem < MAX_ITEM)
			{
				if (pItem[titem].Mode)
					SendCreateItem(conn, titem, 0);
				else
					pItemGrid[y][x] = 0;
			}
		}
	}
}

void GridMulticast(int conn, int tx, int ty, MSG_STANDARD *msg)
{
	if (conn == 0 || pMob[conn].TargetX == 0)
	{
		Log("err,GridMulticast mobidx,pos", "-system", 0);

		return;
	}

	int tmob = pMobGrid[pMob[conn].TargetY][pMob[conn].TargetX];

	if (tmob != conn && tmob)
		GetEmptyMobGrid(conn, &pMob[conn].TargetX, &pMob[conn].TargetY);
 
	if (tmob != conn && tmob)
	{
		if (conn < MAX_USER)
			Log("PC do not have his own grid", "-system", 0);

		else
			Log("NPC do not have his own grid", "-system", 0);

	}
	pMobGrid[pMob[conn].TargetY][pMob[conn].TargetX] = 0;

	if (pMobGrid[ty][tx] != conn && pMobGrid[ty][tx] != 0)
		GetEmptyMobGrid(conn, &pMob[conn].TargetX, &pMob[conn].TargetY);

	if (pMobGrid[ty][tx] != conn && pMobGrid[ty][tx] != 0)
	{
		if (conn < MAX_USER)
			Log("PC step in other mob's grid", "-system", 0);

		else
			Log("NPC charge other mob's grid", "-system", 0);
	}

	pMobGrid[ty][tx] = conn;

	int SizeY = VIEWGRIDY;
	int SizeX = VIEWGRIDX;
	int StartX = pMob[conn].TargetX - HALFGRIDX;
	int StartY = pMob[conn].TargetY - HALFGRIDY;

	if (StartX + SizeX >= MAX_GRIDX)
		SizeX = SizeX - (StartX + SizeX - MAX_GRIDX);

	if (SizeY + StartY >= MAX_GRIDY)
		SizeY = SizeY - (StartY + SizeY - MAX_GRIDY);

	if (StartX < 0)
		StartX = 0;

	if (StartY < 0)
		StartY = 0;

	int sx1 = StartX;
	int sy1 = StartY;
	int sx2 = StartX + SizeX;
	int sy2 = StartY + SizeY;


	SizeY = VIEWGRIDY;
	SizeX = VIEWGRIDX;
	StartX = tx - HALFGRIDX;
	StartY = ty - HALFGRIDY;

	if (StartX + SizeX >= MAX_GRIDX)
		SizeX = SizeX - (StartX + SizeX - MAX_GRIDX);

	if (SizeY + StartY >= MAX_GRIDY)
		SizeY = SizeY - (StartY + SizeY - MAX_GRIDY);

	if (StartX < 0)
		StartX = 0;

	if (StartY < 0)
		StartY = 0;

	int tx1 = StartX;
	int ty1 = StartY;
	int tx2 = StartX + SizeX;
	int ty2 = StartY + SizeY;

	for (int y = sy1; y < sy2; y++)
	{
		for (int x = sx1; x < sx2; x++)
		{
			tmob = pMobGrid[y][x];

			if (tmob == 0 || tmob == conn)
				continue;

			if (msg != NULL && tmob < MAX_USER)
				pUser[tmob].cSock.AddMessage((char*)msg, msg->Size);

			if (x < tx1 || x >= tx2 || y < ty1 || y >= ty2 && tx)
			{
				if (tmob < MAX_USER)
					SendRemoveMob(tmob, conn, 0, 0);

				if (conn < MAX_USER)
					SendRemoveMob(conn, tmob, 0, 0);
			}
		}
	}


	for (int y = sy1; y < sy2; y++)
	{
		for (int x = sx1; x < sx2; x++)
		{
			int titem = pItemGrid[y][x];

			if (titem == 0)
				continue;

			if (x < tx1 || x >= tx2 || y < ty1 || y >= ty2 && tx)
			{
				if (titem > 0 && titem < MAX_ITEM && pItem[titem].Mode && pItem[titem].ITEM.sIndex && conn > 0 && conn < MAX_USER)
					SendRemoveItem(conn, titem, 0);
			}
		}
	}

	for (int y = ty1; y < ty2; y++)
	{
		for (int x = tx1; x < tx2; x++)
		{
			int titem = pItemGrid[y][x];
			tmob = pMobGrid[y][x];

			if (x < sx1 || x >= sx2 || y < sy1 || y >= sy2)
			{
				if (titem > 0 && titem < MAX_ITEM && pItem[titem].Mode && pItem[titem].ITEM.sIndex && conn < MAX_USER)
				{
					if (pItem[titem].Mode)
						SendCreateItem(conn, titem, 0);

					else
						pItemGrid[y][x] = 0;
				}

				if (tmob != conn && tmob)
				{
					if (pMob[tmob].Mode == MOB_EMPTY)
					{
						pMobGrid[y][x] = 0;

						Log("MOB GRID HAS EMPTY MOB", "-system", 0);
						continue;
					}

					if (tmob < MAX_USER)
					{
						if (pMob[conn].Mode == MOB_EMPTY)
							pMobGrid[y][x] = 0;

						else
						{
							SendCreateMob(tmob, conn, 0);
							SendPKInfo(tmob, conn);
						}
					}

					if (conn < MAX_USER)
					{
						if (pMob[tmob].Mode == MOB_EMPTY)
							pMobGrid[y][x] = 0;

						else
						{
							SendCreateMob(conn, tmob, 0);
							SendPKInfo(conn, tmob);
						}
					}

					if (msg != NULL && tmob > 0 && tmob < MAX_USER)
					{
						if (pUser[tmob].cSock.AddMessage((char*)msg, msg->Size) == 0)
						{
							pUser[tmob].AccountName[ACCOUNTNAME_LENGTH - 1] = 0;
							pUser[tmob].AccountName[ACCOUNTNAME_LENGTH - 2] = 0;

							sprintf(temp, "err,gridmulticast add %d-%d %s", tmob, pUser[tmob].Mode, pUser[tmob].AccountName);
							Log(temp, "-system", 0);
						}
					}
				}
			}
		}
	}

	MSG_Action *sm = (MSG_Action*)msg;

	pMob[conn].LastTime = sm->ClientTick;
	pMob[conn].LastSpeed = sm->Speed;

	pMob[conn].LastX = sm->PosX;
	pMob[conn].LastY = sm->PosY;

	pMob[conn].TargetX = tx;
	pMob[conn].TargetY = ty;
}

void SendEmotion(int conn, int Motion, int Parm)
{
	MSG_Motion sm;
	memset(&sm, 0, sizeof(MSG_Motion));

	sm.Type = _MSG_Motion;
	sm.Size = sizeof(MSG_Motion);

	sm.NotUsed = 0;

	sm.ID = conn;

	sm.Motion = Motion;
	sm.Parm = Parm;

	GridMulticast(pMob[conn].TargetX, pMob[conn].TargetY, (MSG_STANDARD*)&sm, 0);
}

void GridMulticast(int tx, int ty, MSG_STANDARD *msg, int skip)
{
	int SizeY = VIEWGRIDY;
	int SizeX = VIEWGRIDX;
	int StartX = tx - HALFGRIDX;
	int StartY = ty - HALFGRIDY;

	if (StartX + SizeX >= MAX_GRIDX)
		SizeX = SizeX - (StartX + SizeX - MAX_GRIDX);

	if (SizeY + StartY >= MAX_GRIDY)
		SizeY = SizeY - (StartY + SizeY - MAX_GRIDY);

	if (StartX < 0)
	{
		StartX = 0;

		SizeX = SizeX + StartX;
	}

	if (StartY < 0)
	{
		StartY = 0;

		SizeY = SizeY + StartY;
	}

	int sx1 = StartX;
	int sy1 = StartY;
	int sx2 = StartX + SizeX;
	int sy2 = StartY + SizeY;

	for (int y = sy1; y < sy2; y++)
	{
		for (int x = sx1; x < sx2; x++)
		{
			int tmob = pMobGrid[y][x];

			if (tmob <= 0 || tmob == skip)
				continue;

			if (pMob[tmob].Mode == MOB_EMPTY)
			{
				int Unk = 0;

				continue;
			}

			if (msg != NULL && tmob < MAX_USER)
			{
				if (msg->Type == _MSG_CreateMob)
				{
					int xx = ((MSG_CreateMob*)msg)->PosX;
					int yy = ((MSG_CreateMob*)msg)->PosY;

					if(xx >= 896 && yy >= 1405 && xx <= 1150 && yy <= 1538)
					{
						STRUCT_ITEM hcitem;

						memset(&hcitem, 0, sizeof(STRUCT_ITEM));

						hcitem.sIndex = 3505;

						((MSG_CreateMob*)msg)->Equip[1] = BASE_VisualItemCode(&hcitem, 1);

						((MSG_CreateMob*)msg)->AnctCode[1] = BASE_VisualAnctCode(&hcitem);


						hcitem.sIndex = 3999;

						((MSG_CreateMob*)msg)->Equip[15] = BASE_VisualItemCode(&hcitem, 15);

						((MSG_CreateMob*)msg)->AnctCode[15] = BASE_VisualAnctCode(&hcitem);
					}

				}

				if (msg->Type == _MSG_CNFMobKill)
				{
					((MSG_CNFMobKill*)msg)->Exp = pMob[tmob].MOB.Exp;
					((MSG_CNFMobKill*)msg)->Hold = pMob[tmob].extra.Hold;

					int Segment = pMob[tmob].CheckGetLevel();

					if (Segment >= 1 && Segment <= 4)
					{
						if (Segment == 4)
						{
							SetCircletSubGod(tmob);
							SendClientMessage(tmob, g_pMessageStringTable[_NN_Level_Up]);

							if(pMob[tmob].extra.ClassMaster == MORTAL)
								DoItemLevel(tmob);
						}
						if (Segment == 3)
							SendClientMessage(tmob, g_pMessageStringTable[_NN_3_Quarters_Bonus]);

						if (Segment == 2)
							SendClientMessage(tmob, g_pMessageStringTable[_NN_2_Quarters_Bonus]);

						if (Segment == 1)
							SendClientMessage(tmob, g_pMessageStringTable[_NN_1_Quarters_Bonus]);

						SendScore(tmob);
						SendEmotion(tmob, 14, 3);

						if (Segment == 4)
						{
							SendEtc(tmob);

							int PKPoint = GetPKPoint(tmob) + 5;
							SetPKPoint(tmob, PKPoint);

							MSG_CreateMob sm_lupc;
							memset(&sm_lupc, 0, sizeof(MSG_CreateMob));
							GetCreateMob(tmob, &sm_lupc);

							GridMulticast(pMob[tmob].TargetX, pMob[tmob].TargetY, (MSG_STANDARD*)&sm_lupc, 0);

							sprintf(temp, "lvl %s level up to %d", pMob[tmob].MOB.MobName, pMob[tmob].MOB.BaseScore.Level);
							Log(temp, pUser[tmob].AccountName, pUser[tmob].IP);
						}
					}
				}

				pUser[tmob].cSock.AddMessage((char*)msg, msg->Size);
			}
		}
	}
}

void PartyGridMulticast(int tx, int ty, MSG_STANDARD *msg, int skip, int Leaderconn)
{
	int SizeY = VIEWGRIDY;
	int SizeX = VIEWGRIDX;
	int StartX = tx - HALFGRIDX;
	int StartY = ty - HALFGRIDY;

	if (StartX + SizeX >= MAX_GRIDX)
		SizeX = SizeX - (StartX + SizeX - MAX_GRIDX);

	if (SizeY + StartY >= MAX_GRIDY)
		SizeY = SizeY - (StartY + SizeY - MAX_GRIDY);

	if (StartX < 0)
		StartX = 0;

	if (StartY < 0)
		StartY = 0;

	int sx1 = StartX;
	int sy1 = StartY;
	int sx2 = StartX + SizeX;
	int sy2 = StartY + SizeY;

	for (int y = StartY; y < sy2; y++)
	{
		for (int x = sx1; x < sx2; x++)
		{
			int tmob = pMobGrid[y][x];

			if (tmob <= 0 || tmob == Leaderconn)
				continue;

			if (!Leaderconn || pMob[tmob].Leader != Leaderconn)
			{
				if (pMob[tmob].Mode == MOB_EMPTY)
				{
					int Unk = 0;

					continue;
				}

				if (msg != NULL && tmob < MAX_USER && tmob != skip && !pUser[tmob].PartyChat)
				{
					if (pUser[tmob].Mode != USER_PLAY || pUser[tmob].cSock.Sock == 0)
						continue;

					pUser[tmob].cSock.AddMessage((char*)msg, msg->Size);
				}
			}
		}
	}

	if (Leaderconn > 0 && Leaderconn < MAX_USER && pUser[Leaderconn].Mode == USER_PLAY)
	{
		if (Leaderconn != skip)
			pUser[Leaderconn].cSock.AddMessage((char*)msg, msg->Size);

		for (int i = 0; i < MAX_PARTY; i++)
		{
			int partyconn = pMob[Leaderconn].PartyList[i];

			if (partyconn <= 0 || partyconn >= MAX_USER)
				continue;

			if (partyconn == skip)
				continue;

			if (pUser[partyconn].Mode != USER_PLAY || pUser[partyconn].PartyChat != 0)
				continue;


			if (pUser[partyconn].Mode != USER_PLAY || pUser[partyconn].cSock.Sock == 0)
				continue;

			if (!pUser[partyconn].cSock.AddMessage((char*)msg, msg->Size))
				CloseUser(partyconn);
		}
	}
}

void SendItem(int conn, int Type, int Slot, STRUCT_ITEM *item)
{
	if (conn <= 0 || conn >= MAX_USER)
		return;

	if (pUser[conn].Mode != USER_PLAY)
		return;

	if (pUser[conn].cSock.Sock == 0)
		return;

	MSG_SendItem sm_si;
	memset(&sm_si, 0, sizeof(MSG_SendItem));

	sm_si.Type = _MSG_SendItem;
	sm_si.Size = sizeof(MSG_SendItem);
	sm_si.ID = conn;

	sm_si.invType = Type;
	sm_si.Slot = Slot;

//	*(int*)&sm.item = *(int*)&item;
//	*(int*)((int)&sm.item + 4) = *(int*)((int)&item + 4);

	memcpy(&sm_si.item, item, sizeof(STRUCT_ITEM));

	pUser[conn].cSock.AddMessage((char*)&sm_si, sizeof(MSG_SendItem));
}

void SendEquip(int conn, int skip)
{
	MSG_UpdateEquip sm_ue;
	memset(&sm_ue, 0, sizeof(MSG_UpdateEquip));

	sm_ue.Type = _MSG_UpdateEquip;
	sm_ue.Size = sizeof(MSG_UpdateEquip);
	sm_ue.ID = conn;

	int SendMount = 0;

	for (int i = 0; i < MAX_EQUIP; i++)
	{
		STRUCT_ITEM *item = &pMob[conn].MOB.Equip[i];

		sm_ue.Equip[i] = BASE_VisualItemCode(item, i);

		sm_ue.AnctCode[i] = BASE_VisualAnctCode(item);

		if (i == 14 && sm_ue.Equip[14] >= 2360 && sm_ue.Equip[i] < 2390 && pMob[conn].MOB.Equip[i].stEffect[0].sValue <= 0)
		{
			sm_ue.Equip[i] = 0;

			SendMount = 1;

			continue;
		}

		if (i == 14 && sm_ue.Equip[14] >= 2360 && sm_ue.Equip[i] < 2390)
		{
			int MountLevel = pMob[conn].MOB.Equip[i].stEffect[1].cEffect; // level

			MountLevel /= 10;

			if (MountLevel > 13)
				MountLevel = 13;
			if (MountLevel < 0)
				MountLevel = 0;

			MountLevel = MountLevel * 4096;
			sm_ue.Equip[i] += MountLevel;

			continue;
		}
	}

	GridMulticast(pMob[conn].TargetX, pMob[conn].TargetY, (MSG_STANDARD*)&sm_ue, skip);

	if (SendMount != 0)
		SendItem(conn, ITEM_PLACE_EQUIP, 14, &pMob[conn].MOB.Equip[14]);
}

void SendScore(int conn)
{
	MSG_UpdateScore sm_vus;
	memset(&sm_vus, 0, sizeof(MSG_UpdateScore));

	sm_vus.Type = _MSG_UpdateScore;

	if (conn < MAX_USER)
	{
		sm_vus.CurrHp = pMob[conn].MOB.CurrentScore.Hp;
		sm_vus.CurrMp = pMob[conn].MOB.CurrentScore.Mp;
	}

	sm_vus.Size = sizeof(MSG_UpdateScore);
	sm_vus.ID = conn;

	memcpy(&sm_vus.Score, &pMob[conn].MOB.CurrentScore, sizeof(STRUCT_SCORE));

	sm_vus.Critical = pMob[conn].MOB.Critical;
	sm_vus.SaveMana = pMob[conn].MOB.SaveMana;
	sm_vus.Guild = pMob[conn].MOB.Guild;
	sm_vus.GuildLevel = pMob[conn].MOB.GuildLevel;

	GetAffect(sm_vus.Affect, pMob[conn].Affect);

	sm_vus.Resist[0] = pMob[conn].MOB.Resist[0];
	sm_vus.Resist[1] = pMob[conn].MOB.Resist[1];
	sm_vus.Resist[2] = pMob[conn].MOB.Resist[2];
	sm_vus.Resist[3] = pMob[conn].MOB.Resist[3];

	sm_vus.Special[0] = 0xCC;
	sm_vus.Special[1] = 0xCC;
	sm_vus.Special[2] = 0xCC;
	sm_vus.Special[3] = 0xCC;

	sm_vus.Magic = pMob[conn].MOB.Magic;

	if (pMob[conn].GuildDisable)
		sm_vus.Guild = 0;

	if (BrState != 0)
	{
		if (conn < MAX_USER)
		{
			int posX = pMob[conn].TargetX;
			int posY = pMob[conn].TargetY;

			if (posX >= 2604 && posY >= 1708 && posX <= 2648 && posY <= 1744)
			{
				sm_vus.Guild = 0;
				sm_vus.GuildLevel = 0;
			}
		}
	}
	GridMulticast(pMob[conn].TargetX, pMob[conn].TargetY, (MSG_STANDARD*)&sm_vus, 0);

	SendAffect(conn);
}

void SendEtc(int conn)
{
	if (conn <= 0 || conn >= MAX_USER)
		return;

	if (pUser[conn].Mode != USER_PLAY)
		return;

	if (pUser[conn].cSock.Sock == 0)
		return;

	MSG_UpdateEtc sm;

	memset(&sm, 0, sizeof(MSG_UpdateEtc));

	sm.Type = _MSG_UpdateEtc;
	sm.Size = sizeof(MSG_UpdateEtc);

	sm.ID = conn;

	sm.Exp = pMob[conn].MOB.Exp;

	sm.SpecialBonus = pMob[conn].MOB.SpecialBonus;

	sm.Learn = pMob[conn].MOB.LearnedSkill;

	sm.ScoreBonus = pMob[conn].MOB.ScoreBonus;
	sm.SkillBonus = pMob[conn].MOB.SkillBonus;

	sm.Coin = pMob[conn].MOB.Coin;

	sm.Hold = pMob[conn].extra.Hold;
	sm.Magic = pMob[conn].MOB.Magic;

	if (!pUser[conn].cSock.AddMessage((char*)&sm, sizeof(MSG_UpdateEtc)))
		CloseUser(conn);
}

void SendCargoCoin(int conn)
{
	if (conn <= 0 || conn >= MAX_USER)
		return;

	if (pUser[conn].Mode != USER_PLAY)
		return;

	if (pUser[conn].cSock.Sock == 0)
		return;

	MSG_STANDARDPARM sm_ucc;
	memset(&sm_ucc, 0, sizeof(MSG_STANDARDPARM));

	sm_ucc.Type = _MSG_UpdateCargoCoin;
	sm_ucc.Size = sizeof(MSG_STANDARDPARM);
	sm_ucc.Size = sizeof(MSG_STANDARDPARM);

	sm_ucc.ID = ESCENE_FIELD;

	sm_ucc.Parm = pUser[conn].Coin;

	if (!pUser[conn].cSock.AddMessage((char*)&sm_ucc, sizeof(MSG_STANDARDPARM)))
		CloseUser(conn);
}

void SendGuildList(int conn)
{
	int Guild = pMob[conn].MOB.Guild;

	if (Guild <= 0)
		return;

	char str[128];

	memset(str, 0, 128);

	int members = 0;
	int max_size = 70;

	for (int i = 0; i < MAX_USER; i++)
	{
		if (pUser[i].Mode != USER_PLAY)
			continue;

		if (Guild != pMob[i].MOB.Guild)
			continue;

		members++;

		int len = strlen(str);

		if (len >= max_size)
		{
			SendClientMessage(conn, str);

			memset(str, 0, len);
		}

		pMob[i].MOB.MobName[NAME_LENGTH - 1] = 0;
		pMob[i].MOB.MobName[NAME_LENGTH - 2] = 0;

		strcat(str, pMob[i].MOB.MobName);
		strcat(str, " ");
	}

	if (str[0])
		SendClientMessage(conn, str);

	if (members == 0)
		SendClientMessage(conn, g_pMessageStringTable[_NN_No_Guild_Members]);

	int max_guild = 65536;

	if (pMob[conn].MOB.Guild <= 0 || pMob[conn].MOB.Guild >= max_guild)
		return;

	int gwar = g_pGuildWar[Guild];
	char GuildName[256];

	BASE_GetGuildName(ServerGroup, gwar, GuildName);

	int FoundWar = -1;

	if (gwar > 0 && gwar < max_guild)
	{
		if (Guild == g_pGuildWar[gwar])
		{
			sprintf(str, g_pMessageStringTable[_SN_Your_are_at_war], GuildName);
			SendClientMessage(conn, str);

			FoundWar = g_pGuildWar[gwar];
		}
		else
		{
			sprintf(str, g_pMessageStringTable[_SN_War_to_S], GuildName);
			SendClientMessage(conn, str);
		}
	}

	for (int j = 1; j < max_guild; j++)
	{
		if (g_pGuildWar[j] == Guild || g_pGuildWar[j] == 0)
			continue;

		if (j == FoundWar)
			continue;

		BASE_GetGuildName(ServerGroup, j, GuildName);

		sprintf(str, g_pMessageStringTable[_SN_War_from_S], GuildName);
		SendClientMessage(conn, str);
	}

	int ally = g_pGuildAlly[Guild];

	BASE_GetGuildName(ServerGroup, ally, GuildName);

	if (ally > 0 && ally < max_guild)
	{
		sprintf(str, g_pMessageStringTable[_SN_Ally_to_S], GuildName);
		SendClientMessage(conn, str);
	}

	for (int j = 1; j < max_guild; j++)
	{
		if (g_pGuildAlly[j] == Guild || g_pGuildAlly[j] == 0)
			continue;

		BASE_GetGuildName(ServerGroup, j, GuildName);

		sprintf(str, g_pMessageStringTable[_SN_Ally_from_S], GuildName);
		SendClientMessage(conn, str);
	}
}

void SendShopList(int conn, int MobIndex, int ShopType)
{
	if (conn <= 0 || conn >= MAX_USER)
		return;

	if (pUser[conn].Mode != USER_PLAY)
		return;

	if (pUser[conn].cSock.Sock == 0)
		return;

	MSG_ShopList sm_sl;
	memset(&sm_sl, 0, sizeof(MSG_ShopList));

	sm_sl.Type = _MSG_ShopList;
	sm_sl.Size = sizeof(MSG_ShopList);

	sm_sl.ID = ESCENE_FIELD;

	sm_sl.ShopType = ShopType;

	for (int i = 0; i < MAX_SHOPLIST; i++)
	{
		int tx = i / 9;
		int ty = i % 9 + MAX_SHOPLIST * i / 9;

		int invpos = (i % 9) + ((i / 9) * MAX_SHOPLIST);


		*(int*)&sm_sl.List[i] = *(int*)&pMob[MobIndex].MOB.Carry[invpos].sIndex;
		*(int*)((int)&sm_sl.List[i] + 4) = *(int*)((int)&pMob[MobIndex].MOB.Carry[invpos] + 4);
	}

	int posX = pMob[MobIndex].TargetX;
	int posY = pMob[MobIndex].TargetY;

	int village = BASE_GetVillage(posX, posY);

	if (village >= 0 && village < MAX_GUILDZONE)
		sm_sl.Tax = g_pGuildZone[village].CityTax;
	else
		sm_sl.Tax = 0;

	if (!pUser[conn].cSock.AddMessage((char*)&sm_sl, sizeof(MSG_ShopList)))
		CloseUser(conn);
}

void SendWarInfo(int conn, int Clan)
{
	int Guild = 0;
	int max_guild = 65536;

	if (pMob[conn].MOB.Guild <= 0 || pMob[conn].MOB.Guild >= max_guild)
		pMob[conn].MOB.Guild = 0;

	if (g_pGuildWar[pMob[conn].MOB.Guild] <= 0 || g_pGuildWar[pMob[conn].MOB.Guild] >= max_guild)
		g_pGuildWar[pMob[conn].MOB.Guild] = 0;

	if (g_pGuildAlly[pMob[conn].MOB.Guild] <= 0 || g_pGuildAlly[pMob[conn].MOB.Guild] >= max_guild)
		g_pGuildAlly[pMob[conn].MOB.Guild] = 0;

	if (pMob[conn].MOB.Guild && g_pGuildWar[pMob[conn].MOB.Guild] && g_pGuildWar[g_pGuildWar[pMob[conn].MOB.Guild]] == pMob[conn].MOB.Guild)
		Guild = g_pGuildWar[pMob[conn].MOB.Guild];
	else
		Guild = 0;

	SendClientSignalParm3(conn, 30000, _MSG_SendWarInfo, Guild, Clan, g_pGuildAlly[pMob[conn].MOB.Guild]);
}

void SendReqParty(int conn, int Leaderconn, int PartyID)
{
	if (conn <= 0 || conn >= MAX_USER)
		return;

	if (pUser[conn].Mode != USER_PLAY)
		return;

	if (pUser[conn].cSock.Sock == 0)
		return;

	MSG_SendReqParty sm_srp;
	memset(&sm_srp, 0, sizeof(MSG_SendReqParty));

	sm_srp.ID = ESCENE_FIELD;
	sm_srp.Type = _MSG_SendReqParty;
	sm_srp.Size = sizeof(MSG_SendReqParty);

	sm_srp.Class = pMob[Leaderconn].MOB.Class;

	sm_srp.Level = pMob[Leaderconn].MOB.CurrentScore.Level;
	sm_srp.MaxHp = pMob[Leaderconn].MOB.CurrentScore.MaxHp;
	sm_srp.Hp = pMob[Leaderconn].MOB.CurrentScore.Hp;

	sm_srp.PartyPos = 0;

	sm_srp.PartyID = PartyID;

	strcpy(sm_srp.MobName, pMob[Leaderconn].MOB.MobName);

	if (!pUser[conn].cSock.AddMessage((char*)&sm_srp, sizeof(MSG_SendReqParty)))
		CloseUser(conn);
}

void SendAddParty(int Leaderconn, int conn, int PartyID)
{
	if (Leaderconn <= 0 || Leaderconn >= MAX_USER)
		return;

	if (pUser[Leaderconn].Mode != USER_PLAY)
		return;

	if (pUser[Leaderconn].cSock.Sock == 0)
		return;

	MSG_CNFAddParty sm_cap;
	memset(&sm_cap, 0, sizeof(MSG_CNFAddParty));

	sm_cap.ID = ESCENE_FIELD;
	sm_cap.Type = _MSG_CNFAddParty;
	sm_cap.Size = sizeof(MSG_CNFAddParty);

	if (PartyID == 0)
		sm_cap.Leaderconn = conn;
	else
		sm_cap.Leaderconn = 30000;

	sm_cap.Level = pMob[conn].MOB.CurrentScore.Level;
	sm_cap.MaxHp = pMob[conn].MOB.CurrentScore.MaxHp > 32000 ? ((pMob[conn].MOB.CurrentScore.MaxHp+1) / 100) : pMob[conn].MOB.CurrentScore.MaxHp;
	sm_cap.Hp = pMob[conn].MOB.CurrentScore.Hp > 32000 ? ((pMob[conn].MOB.CurrentScore.Hp+1) / 100) : pMob[conn].MOB.CurrentScore.Hp;

	sm_cap.PartyID = conn;//Check:
	sm_cap.Target = (short)52428;

	strcpy(sm_cap.MobName, pMob[conn].MOB.MobName);

	if (!pUser[Leaderconn].cSock.SendOneMessage((char*)&sm_cap, sizeof(MSG_CNFAddParty)))
		CloseUser(Leaderconn);
}

void SendRemoveParty(int conn, int connExit)
{
	if (conn <= 0 || conn >= MAX_USER)
		return;

	if (pUser[conn].Mode != USER_PLAY)
		return;

	if (pUser[conn].cSock.Sock == 0)
		return;


	MSG_RemoveParty sm_rp;
	memset(&sm_rp, 0, sizeof(MSG_RemoveParty));

	sm_rp.ID = ESCENE_FIELD;
	sm_rp.Type = _MSG_RemoveParty;
	sm_rp.Size = sizeof(MSG_RemoveParty);

	sm_rp.Leaderconn = connExit;
	sm_rp.unk = 0;

	if (!pUser[conn].cSock.AddMessage((char*)&sm_rp, sizeof(MSG_RemoveParty)))
		CloseUser(conn);
}

void SendCarry(int conn)
{
	if (conn <= 0 || conn >= MAX_USER)
		return;

	if (pUser[conn].Mode != USER_PLAY)
		return;

	if (pUser[conn].cSock.Sock == 0)
		return;

	MSG_UpdateCarry sm;

	memset(&sm, 0, sizeof(MSG_UpdateCarry));

	sm.ID = conn;
	sm.Type = _MSG_UpdateCarry;
	sm.Size = sizeof(MSG_UpdateCarry);

	memcpy(&sm.Carry, pMob[conn].MOB.Carry, sizeof(STRUCT_ITEM)*MAX_CARRY);

	sm.Coin = pMob[conn].MOB.Coin;

	if (!pUser[conn].cSock.AddMessage((char*)&sm, sizeof(MSG_UpdateCarry)))
		CloseUser(conn);
}

void SendWeather()
{
	MSG_UpdateWeather sm_uw;
	memset(&sm_uw, 0, sizeof(MSG_UpdateWeather));

	sm_uw.Type = _MSG_UpdateWeather;
	sm_uw.Size = sizeof(MSG_UpdateWeather);

	sm_uw.ID = ESCENE_FIELD;

	sm_uw.CurrentWeather = CurrentWeather;

	for (int i = 0; i < MAX_USER; i++)
	{
		if (pUser[i].Mode != USER_PLAY)
			continue;

		if (!pUser[i].cSock.Sock)
			continue;


//		if ((pMob[i].TargetX / 128) < 12 && (pMob[i].TargetY / 128) > 25)
		{
			if (!pUser[i].cSock.AddMessage((char*)&sm_uw, sizeof(MSG_UpdateWeather)))
				CloseUser(i);
		}
	}
}

void SendHpMp(int conn)
{
	if (conn <= 0 || conn >= MAX_USER)
		return;

	if (pUser[conn].Mode != USER_PLAY)
		return;

	if (pUser[conn].cSock.Sock == 0)
		return;

	MSG_SetHpMp sm_shm;
	memset(&sm_shm, 0, sizeof(MSG_SetHpMp));

	sm_shm.Type = _MSG_SetHpMp;
	sm_shm.Size = sizeof(MSG_SetHpMp);
	sm_shm.ID = conn;

	sm_shm.Hp = pMob[conn].MOB.CurrentScore.Hp;
	sm_shm.Mp = pMob[conn].MOB.CurrentScore.Mp;

	GridMulticast(pMob[conn].TargetX, pMob[conn].TargetY, (MSG_STANDARD*)&sm_shm, 0);
}

void SendSetHpMp(int conn)
{
	if (conn <= 0 || conn >= MAX_USER)
		return;

	if (pUser[conn].Mode != USER_PLAY)
		return;

	if (pUser[conn].cSock.Sock == 0)
		return;

	MSG_SetHpMp sm_shm;
	memset(&sm_shm, 0, sizeof(MSG_SetHpMp));

	sm_shm.Type = _MSG_SetHpMp;
	sm_shm.Size = sizeof(MSG_SetHpMp);
	sm_shm.ID = conn;

	sm_shm.Hp = pMob[conn].MOB.CurrentScore.Hp;
	sm_shm.Mp = pMob[conn].MOB.CurrentScore.Mp;

	SetReqHp(conn);
	SetReqMp(conn);

	sm_shm.ReqHp = pUser[conn].ReqHp;
	sm_shm.ReqMp = pUser[conn].ReqMp;


	if (!pUser[conn].cSock.AddMessage((char*)&sm_shm, sizeof(MSG_SetHpMp)))
		CloseUser(conn);
}

void SendHpMode(int conn)
{
	if (conn <= 0 || conn >= MAX_USER)
		return;

	if (pUser[conn].Mode != USER_PLAY)
		return;

	if (pUser[conn].cSock.Sock == 0)
		return;

	MSG_SetHpMode sm_shmd;
	memset(&sm_shmd, 0, sizeof(MSG_SetHpMode));

	sm_shmd.Type = _MSG_SetHpMode;
	sm_shmd.Size = sizeof(MSG_SetHpMode);
	sm_shmd.ID = conn;

	sm_shmd.Hp = pMob[conn].MOB.CurrentScore.Hp;
	sm_shmd.Mode = pUser[conn].Mode;

	if (!pUser[conn].cSock.AddMessage((char*)&sm_shmd, sizeof(MSG_SetHpMode)))
		CloseUser(conn);
}

void SendSay(int mob, char *Message)
{
	MSG_MessageChat sm_mc;
	memset(&sm_mc, 0, sizeof(MSG_MessageChat));

	sm_mc.Type = _MSG_MessageChat;
	sm_mc.Size = sizeof(MSG_MessageChat);
	sm_mc.ID = mob;

	memcpy(sm_mc.String, Message, MESSAGE_LENGTH);
	

	GridMulticast(pMob[mob].TargetX, pMob[mob].TargetY, (MSG_STANDARD*)&sm_mc, 0);
}

void MapaMulticast(int tx, int ty, MSG_STANDARD *m, int bSend)
{
	for (int i = 1; i < MAX_USER; i++)
	{
		if (pUser[i].Mode == USER_PLAY && (pMob[i].TargetX / 128) == tx && (pMob[i].TargetY / 128) == ty)
		{
			pUser[i].cSock.AddMessage((char*)m, m->Size);

			if (bSend)
				pUser[i].cSock.SendMessageA();
		}
	}
}

void SendMessageArea(int x1, int y1, int x2, int y2, MSG_STANDARD *m, int bSend)
{
	for (int i = 0; i < MAX_USER; i++)
	{
		if (pUser[i].Mode != USER_PLAY)
			continue;

		if (pMob[i].TargetX >= x1 && pMob[i].TargetY <= x2 && pMob[i].TargetY >= y1 && pMob[i].TargetY <= y2)
		{
			pUser[i].cSock.AddMessage((char*)m, m->Size);

			if (bSend)
				pUser[i].cSock.SendMessageA();
		}
	}
}

void SendSignalParmArea(int x1, int y1, int x2, int y2, int id, unsigned short signal, int parm)
{
	for(int x = x1; x < x2; x++)
	{
		for(int y = y1; y < y2; y++)
		{
			if(x < 0 || x >= MAX_GRIDX || y < 0 || y >= MAX_GRIDY)
				continue;

			int tmob = pMobGrid[y][x];

			if(tmob >= MAX_USER || tmob == 0)
				continue;

			if(pUser[tmob].Mode != USER_PLAY)
				continue;

			SendClientSignalParm(tmob, id, signal, parm); 
		}
	}
}

void SendShortSignalParm2Area(int x1, int y1, int x2, int y2, int id, unsigned short signal, int parm1, int parm2)
{
	for(int x = x1; x < x2; x++)
	{
		for(int y = y1; y < y2; y++)
		{
			if(x < 0 || x >= MAX_GRIDX || y < 0 || y >= MAX_GRIDY)
				continue;

			int tmob = pMobGrid[y][x];

			if(tmob >= MAX_USER || tmob == 0)
				continue;

			if(pUser[tmob].Mode != USER_PLAY)
				continue;

			SendClientSignalShortParm2(tmob, id, signal, parm1, parm2); 
		}
	}
}

void SendPKInfo(int conn, int target)
{
	if (conn <= 0 || conn >= MAX_USER)
		return;

	if (target <= 0 || target >= MAX_USER)
		return;

	MSG_STANDARDPARM sm;
	memset(&sm, 0, sizeof(MSG_STANDARDPARM));

	sm.Size = sizeof(MSG_STANDARDPARM);
	sm.Type = _MSG_PKInfo;
	sm.ID = target;

	if (NewbieEventServer == 0)
	{
		int guilty = GetGuilty(target);

		int state = 0;

		if (guilty || pUser[target].PKMode || g_pRvrWar.Status || CastleState || GTorreState)
			state = 1;

		sm.Parm = state;
	}
	else
		sm.Parm = 1;

	pUser[conn].cSock.AddMessage((char*)&sm, sizeof(MSG_STANDARDPARM));
}

void SendAffect(int conn)
{
	if (conn <= 0 || conn >= MAX_USER)
		return;

	MSG_SendAffect sm;

	memset(&sm, 0, sizeof(MSG_SendAffect));

	sm.Type = _MSG_SendAffect;
	sm.Size = sizeof(MSG_SendAffect);
	sm.ID = conn;

	for (int i = 0; i < MAX_AFFECT; i++)
	{
		if (pMob[conn].Affect[i].Type == 34 && pMob[conn].Affect[i].Time >= 32000000)
		{
			time_t now;

			time(&now);


			if ((pMob[conn].extra.DivineEnd - now) <= 3600)
			{
				pMob[conn].Affect[i].Time = 450;

				sm.Affect[i].Type = pMob[conn].Affect[i].Type;
				sm.Affect[i].Value = pMob[conn].Affect[i].Value;
				sm.Affect[i].Level = pMob[conn].Affect[i].Level;
				sm.Affect[i].Time = pMob[conn].Affect[i].Time;

				continue;
			}

			sm.Affect[i].Type = pMob[conn].Affect[i].Type;
			sm.Affect[i].Value = pMob[conn].Affect[i].Value;
			sm.Affect[i].Level = pMob[conn].Affect[i].Level;
			sm.Affect[i].Time = (int)(((pMob[conn].extra.DivineEnd - now) / 60 / 60 / 24 * AFFECT_1D) - 1);
		}
		else if (pMob[conn].Affect[i].Type >= 1)
		{
			sm.Affect[i].Type = pMob[conn].Affect[i].Type;
			sm.Affect[i].Value = pMob[conn].Affect[i].Value;
			sm.Affect[i].Level = pMob[conn].Affect[i].Level;
			sm.Affect[i].Time = pMob[conn].Affect[i].Time;
		}
	}
	pUser[conn].cSock.AddMessage((char*)&sm, sizeof(MSG_SendAffect));
}

bool FileExist(const char* Name)
{
	if (FILE* file = fopen(Name, "r"))
	{
		fclose(file);
		return true;
	}
	return false;
}

void SendCounterMob(int clientId, short value, short total)
{
	p3BB packet;
	memset(&packet, 0, sizeof p3BB);

	packet.Size = sizeof p3BB;
	packet.Type = 0x3BB;
	packet.ID = clientId;

	packet.Value = value;
	packet.Total = total;

	pUser[clientId].cSock.AddMessage((char*)&packet, sizeof p3BB);
}

char* strFmt(const char* str, ...)
{
	static char buffer[512] = { 0, };
	va_list va;
	va_start(va, str);
	vsprintf_s(buffer, str, va);
	va_end(va);
	return buffer;
}

int ReadBanMac(char* Mac)
{
	FILE* arq = NULL;
	fopen_s(&arq, "Ban/Mac/MacList.txt", "r");

	if (!arq)
	{
		MessageBoxA(0, "Arquivo não encontrado", "Maclist.txt", 0);
		ExitProcess(1);
	}
	char line[100];
	memset(&line, 0, 100);

	char mac[25];
	memset(&mac, 0, 25);

	while ((fscanf(arq, "%[^\n]", line)) != EOF)
	{
		fgetc(arq);
		sscanf(line, "%s", &mac);

		if (!strcmp(mac, Mac))
		{
			fclose(arq);
			return 1;
		}
	}

	fclose(arq);
	return -1;
}

void SendBanAccount(int conn, int type)
{
	if (conn <= NULL || conn >= MAX_USER)
		return;

	struct tm when;
	time_t now;
	time(&now);
	when = *localtime(&now);

	auto ban = &BannedUser[conn];
	memset(&BannedUser[conn], 0, sizeof(AccountBanned));

	switch (type)
	{
		/*Temporario 3 horas*/
	case Banned::Tempo3horas:
	{
		ban->mes = when.tm_mon;
		ban->dia = when.tm_mday;
		ban->hora = when.tm_hour + 3;
		ban->min = when.tm_min;
		ban->ano = when.tm_year;
		sprintf(temp, "!Conta [%s] bloqueada até as: [%02d:%02d]  de [%02d/%02d/%04d]", pUser[conn].AccountName, ban->hora, ban->min, ban->dia, ban->mes + 1, ban->ano + 1900);
		sprintf(temp, "Nome da conta: [%s] Data de Banimento: [%02d:%02d] de [%02d/%02d/%04d] Motivo: Grade B", pUser[conn].AccountName, ban->hora, ban->min, ban->dia, ban->mes + 1, ban->ano + 1900);
		MyLog(LogType::Banned, pMob[conn].MOB.MobName, temp, 0, pUser[conn].IP);
		break;
	}

	/* 3 dias*/
	case Banned::Tempo3dias:
	{
		ban->mes = when.tm_mon;
		ban->dia = when.tm_mday + 3;
		ban->hora = when.tm_hour;
		ban->min = when.tm_min;
		ban->ano = when.tm_year;
		sprintf(temp, "!Conta [%s] bloqueada até as: [%02d:%02d]  de [%02d/%02d/%04d]", pUser[conn].AccountName, ban->hora, ban->min, ban->dia, ban->mes + 1, ban->ano + 1900);
		sprintf(temp, "Nome da conta: [%s] Data de Banimento: [%02d:%02d] de [%02d/%02d/%04d] Motivo: Grade B", pUser[conn].AccountName, ban->hora, ban->min, ban->dia, ban->mes + 1, ban->ano + 1900);
		MyLog(LogType::Banned, pMob[conn].MOB.MobName, temp, 0, pUser[conn].IP);
		break;
	}

	/* 7 dias*/
	case Banned::Tempo7dias:
	{
		ban->mes = when.tm_mon;
		ban->dia = when.tm_mday + 7;
		ban->hora = when.tm_hour;
		ban->min = when.tm_min;
		ban->ano = when.tm_year;
		sprintf(temp, "!Conta [%s] bloqueada até as: [%02d:%02d]  de [%02d/%02d/%04d]", pUser[conn].AccountName, ban->hora, ban->min, ban->dia, ban->mes + 1, ban->ano + 1900);
		sprintf(temp, "Nome da conta: [%s] Data de Banimento: [%02d:%02d] de [%02d/%02d/%04d] Motivo: Grade B", pUser[conn].AccountName, ban->hora, ban->min, ban->dia, ban->mes + 1, ban->ano + 1900);
		MyLog(LogType::Banned, pMob[conn].MOB.MobName, temp, 0, pUser[conn].IP);
		break;
	}

	/* 15 dias*/
	case Banned::Tempo15dias:
	{
		ban->mes = when.tm_mon;
		ban->dia = when.tm_mday + 15;
		ban->hora = when.tm_hour;
		ban->min = when.tm_min;
		ban->ano = when.tm_year;
		sprintf(temp, "!Conta [%s] bloqueada até as: [%02d:%02d]  de [%02d/%02d/%04d]", pUser[conn].AccountName, ban->hora, ban->min, ban->dia, ban->mes + 1, ban->ano + 1900);
		sprintf(temp, "Nome da conta: [%s] Data de Banimento: [%02d:%02d] de [%02d/%02d/%04d] Motivo: Grade B", pUser[conn].AccountName, ban->hora, ban->min, ban->dia, ban->mes + 1, ban->ano + 1900);
		MyLog(LogType::Banned, pMob[conn].MOB.MobName, temp, 0, pUser[conn].IP);
		break;
	}

	/* 30 dias*/
	case Banned::Tempo30dias:
	{
		ban->mes = when.tm_mon;
		ban->dia = when.tm_mday + 30;
		ban->hora = when.tm_hour;
		ban->min = when.tm_min;
		ban->ano = when.tm_year;
		sprintf(temp, "!Conta [%s] bloqueada até as: [%02d:%02d]  de [%02d/%02d/%04d]", pUser[conn].AccountName, ban->hora, ban->min, ban->dia, ban->mes + 1, ban->ano + 1900);
		sprintf(temp, "Nome da conta: [%s] Data de Banimento: [%02d:%02d] de [%02d/%02d/%04d] Motivo: Grade B", pUser[conn].AccountName, ban->hora, ban->min, ban->dia, ban->mes + 1, ban->ano + 1900);
		MyLog(LogType::Banned, pMob[conn].MOB.MobName, temp, 0, pUser[conn].IP);
		break;
	}

	/* Permanente */
	case Banned::Permanente:
	{
		ban->Permanente = TRUE;
		sprintf(temp, "!Conta [%s] bloqueada até as: [%02d:%02d]  de [%02d/%02d/%04d]", pUser[conn].AccountName, ban->hora, ban->min, ban->dia, ban->mes + 1, ban->ano + 1900);
		sprintf(temp, "Nome da conta: [%s] Data de Banimento: [%02d:%02d] de [%02d/%02d/%04d] Motivo: Grade B", pUser[conn].AccountName, ban->hora, ban->min, ban->dia, ban->mes + 1, ban->ano + 1900);
		MyLog(LogType::Banned, pMob[conn].MOB.MobName, temp, 0, pUser[conn].IP);
		break;
	}

	/* Ban Mac */
	case Banned::Mac:
	{

		FILE* Arquivo;
		fopen_s(&Arquivo, "Ban/Mac/MacList.txt", "a+");

		char update[20];
		memset(update, 0, 20);

		sprintf(update, "%s \n", pUser[conn].Mac);

		fputs(update, Arquivo);
		fclose(Arquivo);
		break;
	}

	/* Ban Analise */
	case Banned::Analise:
	{

		ban->Analyze = TRUE;
		sprintf(temp, "!Conta [%s] bloqueada até as: [%02d:%02d]  de [%02d/%02d/%04d]", pUser[conn].AccountName, ban->hora, ban->min, ban->dia, ban->mes + 1, ban->ano + 1900);
		sprintf(temp, "Nome da conta: [%s] Data de Banimento: [%02d:%02d] de [%02d/%02d/%04d] Motivo: Grade B", pUser[conn].AccountName, ban->hora, ban->min, ban->dia, ban->mes + 1, ban->ano + 1900);
		MyLog(LogType::Banned, pMob[conn].MOB.MobName, temp, 0, pUser[conn].IP);
		break;
	}

	SendSaveBanned(conn);
	SendClientMessage(conn, temp);
	CloseUser(conn);
	}

}