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

void Exec_MSG_MessageChat(int conn, char *pMsg)
{
	MSG_MessageChat *m = (MSG_MessageChat*)pMsg;

	m->String[MESSAGE_LENGTH - 1] = 0;
	m->String[MESSAGE_LENGTH - 2] = 0;

	m->ID = conn;

	if (pUser[conn].Mode != USER_PLAY)
		return;

	char szCmd[256];
	char szString[256];

	sscanf(m->String, "%s %s", szCmd, szString);

	if (strcmp(szCmd, "guildon") == 0)
	{
		if (pMob[conn].GuildDisable == 1)
		{
			pMob[conn].GuildDisable = 0;

			SendScore(conn);

			SendClientSignalParm(conn, ESCENE_FIELD, _MSG_GuildDisable, 0);
		}
		return;
	}

	else if (strcmp(szCmd, "guildoff") == 0)
	{
		if (pMob[conn].GuildDisable == 0)
		{
			pMob[conn].GuildDisable = 1;

			SendScore(conn);

			SendClientSignalParm(conn, ESCENE_FIELD, _MSG_GuildDisable, 1);
		}
		return;
	}
	else if (strcmp(szCmd, "guildtax") == 0)
	{
		if (pMob[conn].MOB.GuildLevel != 9)
			return;

		int Guild = pMob[conn].MOB.Guild;

		for (int i = 0; i < MAX_GUILDZONE; i++)
		{
			if (g_pGuildZone[i].ChargeGuild != Guild)
				continue;

			int tax = atoi(szString);

			if (tax < 0 || tax > 30 || szString[0] != 48 && !tax)
			{
				SendClientMessage(conn, g_pMessageStringTable[_NN_Guild_Tax_0_to_30]);
				break;
			}

			if (TaxChanged[i] == 1)
			{
				SendClientMessage(conn, g_pMessageStringTable[_NN_Only_Once_Per_Day]);
				break;
			}

			g_pGuildZone[i].CityTax = tax;
			TaxChanged[i] = 1;

			sprintf(temp, g_pMessageStringTable[169], tax, 0);
			SendClientMessage(conn, temp);

			CReadFiles::WriteGuild();

			sprintf(temp, "sys,%s", m->String);
			Log(temp, pUser[conn].AccountName, pUser[conn].IP);

			break;
		}
		return;
	}

	else if (strcmp(szCmd, "guild") == 0)
	{
		SendGuildList(conn);

		return;
	}

	else if (strcmp(szCmd, "whisper") == 0)
	{
		pUser[conn].Whisper = pUser[conn].Whisper == 0;

		if (pUser[conn].Whisper)
			SendClientMessage(conn, "Whisper : Off");
		else
			SendClientMessage(conn, "Whisper : On");

		return;
	}

	else if (strcmp(szCmd, "partychat") == 0)
	{
		pUser[conn].PartyChat = pUser[conn].PartyChat == 0;

		if (pUser[conn].PartyChat)
			SendClientMessage(conn, "Party Chatting : Off");
		else
			SendClientMessage(conn, "Party Chatting : On");

		return;
	}

	else if (strcmp(szCmd, "kingdomchat") == 0)
	{
		pUser[conn].KingChat = pUser[conn].KingChat == 0;

		if (pUser[conn].KingChat)
			SendClientMessage(conn, "Kingdom Chatting : Off");
		else
			SendClientMessage(conn, "Kingdom Chatting : On");

		return;
	}

	else if (strcmp(szCmd, "guildchat") == 0)
	{
		pUser[conn].Guildchat = pUser[conn].Guildchat == 0;

		if (pUser[conn].Guildchat)
			SendClientMessage(conn, "Guild Chatting : Off");
		else
			SendClientMessage(conn, "Guild Chatting : On");

		return;
	}

	else if (strcmp(szCmd, "chatting") == 0)
	{
		pUser[conn].Chatting = pUser[conn].Chatting == 0;

		if (pUser[conn].Chatting)
			SendClientMessage(conn, "Chatting : Off");
		else
			SendClientMessage(conn, "Chatting : On");

		return;
	}

	if (BrState && conn < MAX_USER && BRItem > 0)
	{
		if (pMob[conn].TargetX >= 2604 && pMob[conn].TargetY >= 1708 && pMob[conn].TargetX <= 2648 && pMob[conn].TargetY <= 1744 || pMob[conn].TargetX >= 896 && pMob[conn].TargetY >= 1405 && pMob[conn].TargetX <= 1150 && pMob[conn].TargetY <= 1538)
			strncpy(m->String, "??????", 6);

	}

	m->String[MESSAGE_LENGTH - 1] = 0;
	m->String[MESSAGE_LENGTH - 2] = 0;

	if(pUser[conn].MuteChat == 1)
	{
		SendClientMessage(conn, g_pMessageStringTable[_NN_No_Speak]);
		return;
	}

	if (pUser[conn].Mode == USER_PLAY)
	{
		int leader = pMob[conn].Leader;
		if (!leader)
			leader = conn;

		GridMulticast(pMob[conn].TargetX, pMob[conn].TargetY, (MSG_STANDARD*)pMsg, conn);
	}
	else
	{
		SendClientMessage(conn, "DEBUG:Client send chatting message with wrong status");
		Log("err,send chatting message with wrong status", pUser[conn].AccountName, pUser[conn].IP);
	}

	sprintf(temp, "chat, %s : %s", pMob[conn].MOB.MobName, m->String);
	ChatLog(temp, pUser[conn].AccountName, pUser[conn].IP);
}