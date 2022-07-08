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

void Exec_MSG_MessageWhisper(int conn, char *pMsg)
{
	MSG_MessageWhisper *m = (MSG_MessageWhisper*)pMsg;

	int Clan = pMob[conn].MOB.Clan;

	m->MobName[NAME_LENGTH - 1] = 0;
	m->String[MESSAGEWHISPER_LENGTH - 1] = 0;

	if (pUser[conn].Mode != USER_PLAY)
		return;

#pragma region /cp
	if (strcmp(m->MobName, "cp") == 0)
	{
		sprintf(temp, g_pMessageStringTable[_DN_Show_Chao], GetPKPoint(conn) - 75);
		SendClientMessage(conn, temp);
		return;
	}
#pragma endregion
#pragma region /buffs
	if (strcmp(m->MobName, "buffs") == 0)
	{
		for (int i = 0; i < MAX_AFFECT; i++)
		{
			pMob[conn].GetCurrentScore(conn);
			memset(&pMob[conn].Affect[i], 0, sizeof(STRUCT_AFFECT));
			SendClientMessage(conn, g_pMessageStringTable[_NN_BUFFS]);
			sprintf(temp, "comando, limpar buffs");
			Log(temp, pUser[conn].AccountName, pUser[conn].IP);
			SendScore(conn);
		}
			return;	
	}
#pragma endregion
#pragma region /getout - Fim cidadão
	else if (strcmp(m->MobName, "getout") == 0)
	{
		pMob[conn].extra.Citizen = 0;
		SendClientMessage(conn, g_pMessageStringTable[_DN_NO_TOWNSPEOPLE]);
			
		sprintf(temp, "etc,getout %d ", pMob[conn].extra.Citizen);
		Log(temp, pUser[conn].AccountName, pUser[conn].IP);
		return;
	}
#pragma endregion
#pragma region /srv - Troca de Server
	else if (strcmp(m->MobName, "srv") == 0)
	{
		int srv = 0;

		sscanf(m->String, "%d", &srv);
			
		if (srv <= 0 || srv >= MAX_SERVER)
			return;

		if(srv == (ServerIndex + 1))
			return;

		if(srv > NumServerInGroup)
			return;

		MSG_DBServerChange sm_cs;
		memset(&sm_cs, 0, sizeof(MSG_DBServerChange));

		sm_cs.Size = sizeof(MSG_DBServerChange);
		sm_cs.Type = _MSG_DBServerChange;
		sm_cs.ID = conn;
		sm_cs.NewServerID = srv;
		sm_cs.Slot = pUser[conn].Slot;

		DBServerSocket.SendOneMessage((char*)&sm_cs, sizeof(MSG_DBServerChange));
		return;
	}
#pragma endregion	
#pragma region /index
	else if (strcmp(m->MobName, "index") == 0)
	{
		if (pMob[conn].MOB.Guild == 0 || pMob[conn].MOB.GuildLevel != 9)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Only_Guild_Master_can]);
			return;
		}

		int Group = ServerGroup;
		int Server = pMob[conn].MOB.Guild / MAX_GUILD;
		int usGuild = pMob[conn].MOB.Guild;

		sprintf(temp, "Guild ID [%d] Fama Atual [%d]", usGuild, GuildInfo[usGuild].Fame);
		SendClientMessage(conn, temp);
		return;
	}
#pragma endregion
#pragma region /spk - Chat para todos os servers
	else if (strcmp(m->MobName, "spk") == 0 || strcmp(m->MobName, "gritar") == 0)
	{
		int i = 0;

		if(pUser[conn].MuteChat == 1)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_No_Speak]);
			return;
		}

		for (i = 0; i < pMob[conn].MaxCarry; i++)
		{
			if (pMob[conn].MOB.Carry[i].sIndex != 3330)
				continue;

			int amount = BASE_GetItemAmount(&pMob[conn].MOB.Carry[i]);

			if(amount > 1)
				BASE_SetItemAmount(&pMob[conn].MOB.Carry[i], amount - 1);
			else
				BASE_ClearItem(&pMob[conn].MOB.Carry[i]);

			SendItem(conn, ITEM_PLACE_CARRY, i, &pMob[conn].MOB.Carry[i]);
			break;
		}

		if (i == pMob[conn].MaxCarry)
			return;
			
		MSG_MagicTrumpet sm_mt;
		memset(&sm_mt, 0, sizeof(MSG_STANDARDPARM));

		sm_mt.Size = sizeof(MSG_MagicTrumpet);
		sm_mt.ID = 0;
		sm_mt.Type = _MSG_MagicTrumpet;

		sprintf(temp, "[%s]> %s", pMob[conn].MOB.MobName, m->String);

		strncpy(sm_mt.String, temp, 96);

		sm_mt.String[94] = 1;

		DBServerSocket.SendOneMessage((char*)&sm_mt, sizeof(MSG_MagicTrumpet));

		sprintf(temp, "chat_spk,%s %s", pMob[conn].MOB.MobName, sm_mt.String);
		ChatLog(temp, pUser[conn].AccountName, pUser[conn].IP);
		return;
	}
#pragma endregion
#pragma region /qst - QuestInfo
	else if (strcmp(m->MobName, "qst") == 0)
	{
		if(pMob[conn].extra.QuestInfo.Arch.Cristal == 0)
			sprintf(temp, g_pMessageStringTable[_NN_NoHave_Quest]);

		else if(pMob[conn].extra.QuestInfo.Arch.Cristal == 1)
			sprintf(temp, g_pMessageStringTable[_DN_Play_Quest], "Contrato de Elime");

		else if(pMob[conn].extra.QuestInfo.Arch.Cristal == 2)
			sprintf(temp, g_pMessageStringTable[_DN_Play_Quest], "Contrato de Sylphed");

		else if(pMob[conn].extra.QuestInfo.Arch.Cristal == 3)
			sprintf(temp, g_pMessageStringTable[_DN_Play_Quest], "Contrato de Thelion");

		else if(pMob[conn].extra.QuestInfo.Arch.Cristal == 4)
			sprintf(temp, g_pMessageStringTable[_DN_Play_Quest], "Contrato de Noas");

		SendClientMessage(conn, temp);
		return;
	}
#pragma endregion
#pragma region /criar
	else if (strcmp(m->MobName, "create") == 0)
	{
		if (m->String[0] == 0)
			return;

		if (pMob[conn].MOB.Coin < 100000000)
		{
			sprintf(temp, g_pMessageStringTable[_DN_D_Cost], 100000000);
			SendClientMessage(conn, temp);
			return;
		}

		if (pMob[conn].MOB.Guild != 0)
			return;	
		if (pMob[conn].MOB.Clan != 7 && pMob[conn].MOB.Clan != 8)
		{			
			SendClientMessage(conn, g_pMessageStringTable[_NN_GUILDCREATECLAN]); //Re-Ativado Max[Master]
			return;
		}
		if (GuildCounter >= 4096)
			return;

		if (pMob[conn].extra.Citizen == 0)
		{
			SendClientMessage(conn, g_pMessageStringTable[_DN_NO_TOWNSPEOPLE]);
			return;
		}
		//Adicionado By Faelz'
		time_t rawtime;
		tm *timeinfo;

		time(&rawtime);
		timeinfo = localtime(&rawtime);
		if (timeinfo->tm_wday == 0)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_GUILDCREATEWEEK]);
			return;
		}
		//Fim
		if (ServerIndex == -1)
		{
			Log("err,Buy Guild item, but, Server index undefined", pUser[conn].AccountName, pUser[conn].IP);
			return;
		}

		if (GuildCounter == 0)
		{
			Log("err,Buy Guild item, but, Guild counter zero", pUser[conn].AccountName, pUser[conn].IP);
			return;
		}

		pMob[conn].MOB.Guild = ServerIndex * MAX_GUILD + GuildCounter;
		pMob[conn].MOB.GuildLevel = 9;

		pMob[conn].MOB.Coin -= 100000000;
		SendEtc(conn);

		char szName[GUILDNAME_LENGTH];

		memset(szName, 0, GUILDNAME_LENGTH);

		strncpy(szName, m->String, GUILDNAME_LENGTH);

		FILE *fp = fopen("../../Common/Guilds.txt", "a+");

		if (fp == NULL)
		{
			Log("err,Buy Guild item, but, no open file undefined", pUser[conn].AccountName, pUser[conn].IP);
			return;
		}

		fprintf(fp, "\n%d %d %d %s   ", ServerGroup, ServerIndex, GuildCounter, szName);

		fclose(fp);

		int Group = ServerGroup;
		int Server = pMob[conn].MOB.Guild / MAX_GUILD;
		int usGuild = pMob[conn].MOB.Guild;

		MSG_GuildInfo sm2;
		memset(&sm2, 0, sizeof(MSG_GuildInfo));

		sm2.Type = _MSG_GuildInfo;
		sm2.Size = sizeof(MSG_GuildInfo);
		sm2.ID = conn;

		sm2.Guild = usGuild;

		GuildInfo[usGuild].Clan = pMob[conn].MOB.Clan;
		GuildInfo[usGuild].Fame = 0;
		GuildInfo[usGuild].Citizen = pMob[conn].extra.Citizen;


		sm2.GuildInfo = GuildInfo[usGuild];

		DBServerSocket.SendOneMessage((char*)&sm2, sizeof(MSG_GuildInfo));

		int GCount = GuildCounter + (ServerIndex << 12);

		sprintf(temp, "sys,guild medal value:%d count:%d", GCount, GuildCounter);
		Log(temp, pUser[conn].AccountName, pUser[conn].IP);

		GuildCounter++;

		CReadFiles::WriteGuild();

		BASE_InitializeGuildName();

		sprintf(temp, g_pMessageStringTable[_SN_CREATEGUILD], szName);
		SendClientMessage(conn, temp);
		return;
	}
#pragma endregion
#pragma region /subcreate
	else if (strcmp(m->MobName, "subcreate") == 0)
	{
		if(pMob[conn].MOB.Guild == 0)
			return;

		char SubMobName[256];
		char SubName[256];

		sscanf(m->String, "%s %s", SubMobName, SubName);

		char sub[16];

		strncpy(sub, SubMobName, 16);

		int user = GetUserByName(sub);

		if(user == conn)
			return;

		if(user == 0)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Not_Connected]);
			return;
		}

		if(pMob[user].MOB.Guild != pMob[conn].MOB.Guild)
			return;

		if(pMob[conn].MOB.GuildLevel != 9)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Only_Guild_Master_can]);
			return;
		}

		if(pMob[user].MOB.GuildLevel != 0)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Haven_Guild_Medal]);
			return;
		}

		int Guild = pMob[conn].MOB.Guild;
			
		if(GuildInfo[Guild].Sub1 != 0 && GuildInfo[Guild].Sub2 != 0 && GuildInfo[Guild].Sub3 != 0)
			return;
			

		if(pMob[conn].MOB.Coin < 100000000)
		{
			sprintf(temp, g_pMessageStringTable[_DN_D_Cost], 100000000);
			SendClientMessage(conn, temp);
			return;
		}

		pMob[conn].MOB.Coin -= 100000000;

		if(GuildInfo[Guild].Sub1 == 0)
		{
			pMob[user].MOB.GuildLevel = 6;
			GuildInfo[Guild].Sub1 = 1;
		}
		else if(GuildInfo[Guild].Sub2 == 0)
		{
			pMob[user].MOB.GuildLevel = 7;
			GuildInfo[Guild].Sub2 = 1;
		}
		else if(GuildInfo[Guild].Sub3 == 0)
		{
			pMob[user].MOB.GuildLevel = 8;
			GuildInfo[Guild].Sub3 = 1;
		}

		sprintf(temp, "etc,subcreate %s %d ", SubName, Guild);
		Log(temp, pUser[conn].AccountName, pUser[conn].IP);

		MSG_GuildInfo sm_gi;
		memset(&sm_gi, 0, sizeof(MSG_GuildInfo));

		sm_gi.Type = _MSG_GuildInfo;
		sm_gi.Size = sizeof(MSG_GuildInfo);
		sm_gi.ID = conn;

		sm_gi.Guild = Guild;
		sm_gi.GuildInfo = GuildInfo[Guild];

		DBServerSocket.SendOneMessage((char*)&sm_gi, sizeof(MSG_GuildInfo));

		MSG_CreateMob sm2;
		memset(&sm2, 0, sizeof(MSG_CreateMob));
		GetCreateMob(conn, &sm2);

		GridMulticast(pMob[conn].TargetX, pMob[conn].TargetY, (MSG_STANDARD*)&sm2, 0);
		return;
	}
#pragma endregion
#pragma region /sair /expulsar /abandonar
	else if (strcmp(m->MobName, "sair") == 0 || strcmp(m->MobName, "expulsar") == 0 || strcmp(m->MobName, "abandonar") == 0)
	{
		if(pMob[conn].MOB.Guild == 0)
			return;
			
		sprintf(temp, "etc,abandonar %d %d", pMob[conn].MOB.Guild, pMob[conn].MOB.GuildLevel);
		Log(temp, pUser[conn].AccountName, pUser[conn].IP);

		int gGuild = pMob[conn].MOB.Guild;

		if(pMob[conn].MOB.GuildLevel >= 6 && pMob[conn].MOB.GuildLevel <= 8)
		{
			if(GuildInfo[gGuild].Sub1 != 0)
				GuildInfo[gGuild].Sub1 = 0;
			else if(GuildInfo[gGuild].Sub2 != 0)
				GuildInfo[gGuild].Sub2 = 0;
		
			else if(GuildInfo[gGuild].Sub3 != 0)
				GuildInfo[gGuild].Sub3 = 0;
		

			sprintf(temp, "etc,subdelete %s %d ", pMob[conn].MOB.MobName, gGuild);
			Log(temp, pUser[conn].AccountName, pUser[conn].IP);

			MSG_GuildInfo sm_gi;
			memset(&sm_gi, 0, sizeof(MSG_GuildInfo));

			sm_gi.Type = _MSG_GuildInfo;
			sm_gi.Size = sizeof(MSG_GuildInfo);
			sm_gi.ID = conn;

			sm_gi.Guild = gGuild;
			sm_gi.GuildInfo = GuildInfo[gGuild];

			DBServerSocket.SendOneMessage((char*)&sm_gi, sizeof(MSG_GuildInfo));
		}

		pMob[conn].MOB.Guild = 0;
		pMob[conn].MOB.GuildLevel = 0;

		MSG_CreateMob sm_eg;
		memset(&sm_eg, 0, sizeof(MSG_CreateMob));
		GetCreateMob(conn, &sm_eg);

		GridMulticast(pMob[conn].TargetX, pMob[conn].TargetY, (MSG_STANDARD*)&sm_eg, 0);
		return;
	}
#pragma endregion
#pragma region /transferir
	else if (strcmp(m->MobName, "handover") == 0)
	{
	if (pMob[conn].MOB.Guild == 0)
		return;

	if (pMob[conn].MOB.GuildLevel != 9)
		return;

	m->String[NAME_LENGTH - 1] = 0;
	m->String[NAME_LENGTH - 2] = 0;

	int target = GetUserByName(m->String);

	if (target == 0)
	{
		SendClientMessage(conn, g_pMessageStringTable[_NN_Not_Connected]);
		return;
	}

	if (pUser[target].Mode != USER_PLAY)
	{
		SendClientMessage(conn, g_pMessageStringTable[_NN_Not_Connected]);
		return;
	}

	if (pMob[target].MOB.Guild != pMob[conn].MOB.Guild)
		return;

	char guildname[256];
	BASE_GetGuildName(ServerGroup, pMob[conn].MOB.Guild, guildname);
	sprintf(temp, "etc,handover guild:%d guildname:%s  Lider:%s -> NovoLider:%s ", pMob[conn].MOB.Guild, guildname, pMob[conn].MOB.MobName, pMob[target].MOB.MobName);
	Log(temp, pUser[conn].AccountName, pUser[conn].IP);

	pMob[target].MOB.GuildLevel = 9;
	pMob[conn].MOB.GuildLevel = 0;

	ClearCrown(conn);

	MSG_CreateMob sm_eg;
	memset(&sm_eg, 0, sizeof(MSG_CreateMob));
	GetCreateMob(conn, &sm_eg);

	GridMulticast(pMob[conn].TargetX, pMob[conn].TargetY, (MSG_STANDARD*)&sm_eg, 0);

	memset(&sm_eg, 0, sizeof(MSG_CreateMob));
	GetCreateMob(target, &sm_eg);

	GridMulticast(pMob[target].TargetX, pMob[target].TargetY, (MSG_STANDARD*)&sm_eg, 0);

	MSG_GuildInfo sm_gi;
	memset(&sm_gi, 0, sizeof(MSG_GuildInfo));

	sm_gi.Type = _MSG_GuildInfo;
	sm_gi.Size = sizeof(MSG_GuildInfo);
	sm_gi.ID = conn;

	sm_gi.Guild = pMob[conn].MOB.Guild;

	GuildInfo[pMob[conn].MOB.Guild].Fame = 0;

	sm_gi.GuildInfo = GuildInfo[pMob[conn].MOB.Guild];

	DBServerSocket.SendOneMessage((char*)&sm_gi, sizeof(MSG_GuildInfo));

	return;
	}
#pragma endregion
#pragma region /nt - NT
	else if (strcmp(m->MobName, "nt") == 0)
	{
		sprintf(temp, g_pMessageStringTable[_DN_CHANGE_COUNT], pMob[conn].extra.NT);

		SendClientMessage(conn, temp);
		return;
	}
#pragma endregion
#pragma region /nig - Pesadelo TIME
	else if (strcmp(m->MobName, "nig") == 0)
	{
		char tmptime[256];

		time_t mytime;

		time(&mytime);

		tm *timeinfo = localtime(&mytime);

		strftime(tmptime, 256, "!!%H%M%S", timeinfo);

		SendClientMessage(conn, tmptime);
		return;
	}
#pragma endregion
#pragma region /tab
	if (strcmp(m->MobName, "tab") == 0)
	{
		if (pMob[conn].MOB.CurrentScore.Level < 69 && pMob[conn].extra.ClassMaster == MORTAL)
		{
			sprintf(temp, g_pMessageStringTable[_DN_Level_Limit], 70);
			SendClientMessage(conn, temp);
			return;
		}

		strncpy(pMob[conn].Tab, m->String, 26);

		if(pUser[conn].TradeMode == 0)
		{
			MSG_CreateMob sm_tb;
			memset(&sm_tb, 0, sizeof(MSG_CreateMob));
			GetCreateMob(conn, &sm_tb);
			GridMulticast(pMob[conn].TargetX, pMob[conn].TargetY, (MSG_STANDARD*)&sm_tb, 0);
		}
		else
		{
			MSG_CreateMobTrade sm_tb;
			memset(&sm_tb, 0, sizeof(MSG_CreateMobTrade));
			GetCreateMobTrade(conn, &sm_tb);
			GridMulticast(pMob[conn].TargetX, pMob[conn].TargetY, (MSG_STANDARD*)&sm_tb, 0);
		}

		MSG_STANDARDPARM sm;
		memset(&sm, 0, sizeof(MSG_STANDARDPARM));

		sm.Size = sizeof(MSG_STANDARDPARM);
		sm.Type = _MSG_PKInfo;
		sm.ID = conn;

		if (NewbieEventServer == 0)
		{
			int guilty = GetGuilty(conn);

			int state = 0;

			if (guilty || pUser[conn].PKMode || g_pRvrWar.Status && pMob[conn].TargetX >= 1023 && pMob[conn].TargetY >= 1919 && pMob[conn].TargetX <= 1280 && pMob[conn].TargetY <= 2179 || CastleState && pMob[conn].TargetX >= 1024 && pMob[conn].TargetY >= 1664 && pMob[conn].TargetX <= 1153 && pMob[conn].TargetY <= 1793 || GTorreState && pMob[conn].TargetX >= 2430 && pMob[conn].TargetY >= 1825 && pMob[conn].TargetX <= 2560 && pMob[conn].TargetY <= 1925)
				state = 1;

			sm.Parm = state;
		}
		else
			sm.Parm = 1;

		GridMulticast(pMob[conn].TargetX, pMob[conn].TargetY, (MSG_STANDARD*)&sm, 0);
		return;
	}
#pragma endregion
#pragma region /snd
	if (strcmp(m->MobName, "snd") == 0)
	{
		strncpy(pMob[conn].Snd, m->String, 96);

		sprintf(temp, "%s %s", g_pMessageStringTable[_NN_SND_MESSAGE], pMob[conn].Snd);

		SendClientMessage(conn, temp);

		return;
	}
#pragma endregion
#pragma region /day - Skill
	if (strcmp(m->MobName, "day") == 0)
	{
		SendClientMessage(conn, "!#11  2");

		return;
	}
#pragma endregion
#pragma region /online
	if (strcmp(m->MobName, "online") == 0)
	{
			short num = 0;
			for (int i = 0; i < MAX_USER; i++)
			{
				if (pUser[i].Mode == USER_PLAY)
				{
					num++;
				}
			}
			char cMsg[128];
			sprintf(cMsg, "Somos %d jogadore(s) online.", num);
			SendClientMessage(conn, cMsg);
			return;
	}
#pragma endregion
#pragma region /destravar40
	if (strcmp(m->MobName, "destravar40") == 0)
	{
		
		SendClientMessage(conn, g_pMessageStringTable[_NN_Processing_Complete]);

		SendClientSignalParm(conn, ESCENE_FIELD, _MSG_CombineComplete, 1);

		sprintf(temp, "destrave,mobname:%s lv:40", pMob[conn].MOB.MobName);

		ItemLog(temp, "*** Combine odin sucess ***", pUser[conn].IP);

		pMob[conn].extra.QuestInfo.Celestial.Lv40 = 1;

		return;
	}
#pragma endregion
#pragma region /destravar90
	if (strcmp(m->MobName, "destravar90") == 0)
	{

		SendClientMessage(conn, g_pMessageStringTable[_NN_Processing_Complete]);

		SendClientSignalParm(conn, ESCENE_FIELD, _MSG_CombineComplete, 1);

		SendEmotion(conn, 14, 3);

		sprintf(temp, g_pMessageStringTable[_DN_Play_Quest], "Lv90");

		SendClientMessage(conn, temp);

		ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

		pMob[conn].extra.QuestInfo.Celestial.Lv90 = 1;

		STRUCT_ITEM Prize;
		memset(&Prize, 0, sizeof(STRUCT_ITEM));

		Prize.sIndex = 3502;

		PutItem(conn, &Prize);

		sprintf(temp, "useitem,furystone quest90 sucess");
		ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

		return;
	}
#pragma endregion
#pragma region /arcana
	if (strcmp(m->MobName, "arcana") == 0)
	{
		if (!pMob[conn].MOB.Equip[1].sIndex)
			memset(&pMob[conn].MOB.Equip[1], 0, sizeof(STRUCT_ITEM));


			pMob[conn].MOB.Equip[1].sIndex = 3507;

			SendItem(conn, ITEM_PLACE_EQUIP, 1, &pMob[conn].MOB.Equip[1]);

			sprintf(temp, g_pMessageStringTable[_DN_Play_Quest], "Cythera Arcana");

			SendClientMessage(conn, g_pMessageStringTable[_NN_Processing_Complete]);

			SendClientSignalParm(conn, ESCENE_FIELD, _MSG_CombineComplete, 1);

			SendClientMessage(conn, temp);

			SendEmotion(conn, 14, 3);
			pMob[conn].extra.QuestInfo.Circle = 1;
	
			sprintf(temp, "useitem,furystone quest arcan complete.");
			ItemLog(temp, pUser[conn].AccountName, pUser[conn].IP);

		return;
	}
#pragma endregion
#pragma region /red
	if (strcmp(m->MobName, "red") == 0)
	{
		DoTeleport(conn, 1744 + rand() % 3, 1880 + rand() % 3);
		 
		return;
	}
#pragma endregion
#pragma region /blue
	if (strcmp(m->MobName, "blue") == 0)
	{
		DoTeleport(conn, 1745 + rand() % 3, 1573 + rand() % 3);

		return;
	}
#pragma endregion
#pragma region /arch
	if (strcmp(m->MobName, "arch") == 0)
	{
		DoTeleport(conn, 1706 + rand() % 3, 1723 + rand() % 3);

		return;
	}
#pragma endregion
#pragma region /selados
	if (strcmp(m->MobName, "selados") == 0)
	{
		DoTeleport(conn, 1843 + rand() % 3, 3652 + rand() % 3);
		return;
	}
#pragma endregion
#pragma region /amagos
	if (strcmp(m->MobName, "amagos") == 0)
	{
		DoTeleport(conn, 3910 + rand() % 3, 2878 + rand() % 3);
		return;
	}
#pragma endregion
#pragma region _NN_TP_ARMIA - /armia
	if (strcmp(m->MobName, "armia") == 0 && RvRState == 0 && GTorreState == 0 && CastleState == 0)
	{
		DoTeleport(conn, 2100 + rand() % 3, 2100 + rand() % 3);
		SendClientMessage(conn, g_pMessageStringTable[_NN_TP_ARMIA]);

		sprintf(temp, "comando, teleporte armia");
		Log(temp, pUser[conn].AccountName, pUser[conn].IP);
		return;
	}

	else
	{
		SendClientMessage(conn, g_pMessageStringTable[_NN_TP_DENY]);
		return;
	}
#pragma endregion
#pragma region _NN_TP_AZRAN - /azran
	if (strcmp(m->MobName, "azran") == 0 && RvRState == 0 && GTorreState == 0 && CastleState == 0)
	{
		DoTeleport(conn, 2500 + rand() % 3, 1716 + rand() % 3);
		SendClientMessage(conn, g_pMessageStringTable[_NN_TP_AZRAN]);

		sprintf(temp, "etc,azran command");
		Log(temp, pUser[conn].AccountName, pUser[conn].IP);
		return;
	}

	else
	{
		SendClientMessage(conn, g_pMessageStringTable[_NN_TP_DENY]);
		return;
	}
#pragma endregion
#pragma region _NN_TP_TORRE - /torre
	if (strcmp(m->MobName, "torre") == 0 && RvRState == 0 && GTorreState == 0 && CastleState == 0)
	{
		DoTeleport(conn, 2506 + rand() % 3, 1878 + rand() % 3);
		SendClientMessage(conn, g_pMessageStringTable[_NN_TP_TORRE]);

		sprintf(temp, "etc,torre command");
		Log(temp, pUser[conn].AccountName, pUser[conn].IP);
		return;
	}

	else
	{
		SendClientMessage(conn, g_pMessageStringTable[_NN_TP_DENY]);
		return;
	}
#pragma endregion
#pragma region _NN_GUILD - /guild
	if (strcmp(m->MobName, "guild") == 0)
	{
		SendGuildList(conn);

		return;
	}
#pragma endregion
#pragma region _NN_TP_ERION - /erion
	if (strcmp(m->MobName, "erion") == 0 && RvRState == 0 && GTorreState == 0 && CastleState == 0)
	{
		DoTeleport(conn, 2461 + rand() % 3, 2003 + rand() % 3);
		SendClientMessage(conn, g_pMessageStringTable[_NN_TP_ERION]);

		sprintf(temp, "etc,erion command");
		Log(temp, pUser[conn].AccountName, pUser[conn].IP);
		return;
	}

	else
	{
		SendClientMessage(conn, g_pMessageStringTable[_NN_TP_DENY]);
		return;
	}
#pragma endregion
#pragma region _NN_TP_GELO - /gelo
	if (strcmp(m->MobName, "gelo") == 0 && RvRState == 0 && GTorreState == 0 && CastleState == 0)
	{
		DoTeleport(conn, 3650 + rand() % 3, 3130 + rand() % 3);
		SendClientMessage(conn, g_pMessageStringTable[_NN_TP_GELO]);

		sprintf(temp, "etc,gelo command");
		Log(temp, pUser[conn].AccountName, pUser[conn].IP);
		return;
	}

	else
	{
		SendClientMessage(conn, g_pMessageStringTable[_NN_TP_DENY]);
		return;
	}
#pragma endregion
#pragma region _NN_TP_KEFRA - /kefra
	if (strcmp(m->MobName, "kefra") == 0 && RvRState == 0 && GTorreState == 0 && CastleState == 0)
	{
		if (KefraLive == 0) {
			DoTeleport(conn, 2365 + rand() % 3, 3884 + rand() % 3);
			SendClientMessage(conn, "O kefra ainda esta vivo, precisa ser derrotado");
			return;
		}

		DoTeleport(conn, 3250 + rand() % 3, 1695 + rand() % 3);
		SendClientMessage(conn, g_pMessageStringTable[_NN_TP_KEFRA]);

		sprintf(temp, "etc,kefra command");
		Log(temp, pUser[conn].AccountName, pUser[conn].IP);
		return;
	}

	else
	{
		SendClientMessage(conn, g_pMessageStringTable[_NN_TP_DENY]);
		return;
	}
#pragma endregion
#pragma region _NN_TP_AGUA - /agua
	if (strcmp(m->MobName, "agua") == 0)
	{
		DoTeleport(conn, 1966 + rand() % 3, 1770 + rand() % 3);
		SendClientMessage(conn, "Bem vindo à Zona Elemental da Água");

		sprintf(temp, "etc,agua command");
		Log(temp, pUser[conn].AccountName, pUser[conn].IP);
		return;
	}
#pragma endregion
#pragma region _NN_Kingdom - /kingdom
	if (strcmp(m->MobName, g_pMessageStringTable[_NN_Kingdom]) == 0 || strcmp(m->MobName, "kingdom") == 0)
	{
		if (Clan == 7)
			DoTeleport(conn, 1690, 1618);
		else if (Clan == 8)
			DoTeleport(conn, 1690, 1842);
		else
			DoTeleport(conn, 1702, 1726);

		sprintf(temp, "etc,kingdom command %d ", Clan);
		Log(temp, pUser[conn].AccountName, pUser[conn].IP);
		return;
	}
#pragma endregion
#pragma region _NN_King  - /king
	else if (strcmp(m->MobName, g_pMessageStringTable[_NN_King]) == 0 || strcmp(m->MobName, "king") == 0)
	{
		if (Clan == 7)
			DoTeleport(conn, 1748, 1574);

		else if (Clan == 8)
			DoTeleport(conn, 1748, 1880);

		sprintf(temp, "etc,king command %d ", Clan);
		Log(temp, pUser[conn].AccountName, pUser[conn].IP);
		return;
	}
#pragma endregion
#pragma region _NN_Summon_Guild - /summonguild
	else if (strcmp(m->MobName, g_pMessageStringTable[_NN_Summon_Guild]) == 0 || strcmp(m->MobName, "summonguild") == 0)
	{
		int usGuild = pMob[conn].MOB.Guild;
		int usGuildLv = pMob[conn].MOB.GuildLevel;

		int xx = pMob[conn].TargetX;
		int yy = pMob[conn].TargetY;

		int village = BASE_GetVillage(xx, yy);

		unsigned char mapatt = GetAttribute(xx, yy);

		if (usGuildLv <= 0)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Only_Guild_Master_Can]);
			return;
		}

		int sub = BASE_GetSubGuild(pMob[conn].MOB.Equip[12].sIndex);

		if (village < 0 || village >= 5)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Cant_Use_That_Here]);
			return;
		}

		if (mapatt & 4 || mapatt & 0x40)
			SendClientMessage(conn, g_pMessageStringTable[_NN_Cant_Use_That_Here]);
		else
			SummonGuild2(usGuild, xx, yy, 350, sub);

		sprintf(temp, "etc,summonguild guild:%d X:%d Y:%d", usGuild, xx, yy);
		Log(temp, pUser[conn].AccountName, pUser[conn].IP);
		return;
	}
#pragma endregion
#pragma region _NN_Summon - /summon
	else if (strcmp(m->MobName, g_pMessageStringTable[_NN_Summon]) == 0 || strcmp(m->MobName, "summon") == 0)
	{
		int admin = 0;

		if (pMob[conn].MOB.CurrentScore.Level >= 1000)
			admin = 1;

		if (pMob[conn].MOB.CurrentScore.Hp <= 0)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Cant_Use_Killed]);
			return;
		}

		if (m->String[0] == 0)
			return;

		m->String[NAME_LENGTH - 1] = 0;
		m->String[NAME_LENGTH - 2] = 0;

		int target = GetUserByName(m->String);

		if (target == 0)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Not_Connected]);
			return;
		}

		if (target == conn)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Cant_Use_To_Yourself]);
			return;
		}

		if (pUser[target].Mode != USER_PLAY)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Not_Connected]);
			return;
		}

		int myguild = pMob[conn].MOB.Guild;
		int targetguild = pMob[target].MOB.Guild;

		int myleader = pMob[conn].Leader;

		if (myleader <= 0)
			myleader = conn;

		int targetleader = pMob[target].Leader;

		if (targetleader <= 0)
			targetleader = target;

		if (pMob[target].MOB.CurrentScore.Hp <= 0 && admin == 0)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Cant_Use_To_Killed]);
			return;
		}

		if ((myguild == 0 && myleader == 0 || myleader != targetleader || myguild != targetguild && myleader == 0) && admin == 0)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Only_To_Party_Or_Guild]);
			return;
		}

		if (pMob[conn].MOB.Class != 1 && admin == 0)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Didnt_Learn_Summon_Skill]);
			return;
		}

		int learn = pMob[conn].MOB.LearnedSkill;

		if ((learn & 0x40000) == 0 && admin == 0)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Didnt_Learn_Summon_Skill]);
			return;
		}

		int special = pMob[conn].MOB.CurrentScore.Special[3];

		unsigned char map_att = GetAttribute(pMob[conn].TargetX, pMob[conn].TargetY);

		if ((map_att & 4) && admin == 0)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Cant_Use_That_Here]);
			return;
		}

		int dif = pMob[conn].MOB.CurrentScore.Level + special + 30;

		if (pMob[target].MOB.CurrentScore.Level > dif && admin == 0)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Too_High_Level]);
			return;
		}

		/*if (pUser[target].OnlyTrade)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_ONLYTRADE]);
			return;
		}*/

		if (pMob[target].TargetX & 0xFF00 || pMob[target].TargetY & 0xFF00)
		{
			DoSummon(target, pMob[conn].TargetX, pMob[conn].TargetY);

			sprintf(temp, g_pMessageStringTable[_SN_Summoned_By_S], pMob[conn].MOB.MobName);
			SendClientMessage(target, temp);

			sprintf(temp, "etc,summon %s X:%d Y:%d", pMob[target].MOB.MobName, pMob[target].TargetX, pMob[target].TargetY);
			Log(temp, pUser[conn].AccountName, pUser[conn].IP);
			return;
		}
		else
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Cant_Use_That_Here]);
			return;
		}
		return;
	}
#pragma endregion
#pragma region /time
	else if (strcmp(m->MobName, "time") == 0)
	{
		char tmptime[256];

		time_t mytime;

		time(&mytime);

		tm *timeinfo = localtime(&mytime);

		strftime(tmptime, 256, "%d-%m-%Y %H:%M:%S", timeinfo);
									
		SendClientMessage(conn, tmptime);
		return;
	}
#pragma endregion
#pragma region _CM_GM - /gm
	else if (strcmp(m->MobName, "gm") == 0 || strcmp(m->MobName, "GM") == 0)
	{
		int level = pMob[conn].MOB.CurrentScore.Level - 1000;

		ProcessImple(conn, level, m->String);
		return;
	}
#pragma endregion
#pragma region _NN_Relocate - /relo
	else if (strcmp(m->MobName, g_pMessageStringTable[_NN_Relocate]) == 0 || strcmp(m->MobName, "relo") == 0)
	{
		int Class = pMob[conn].MOB.Class;
		int admin = 0;

		if (pMob[conn].MOB.CurrentScore.Level >= 1000)
			admin = 1;

		if (pMob[conn].MOB.CurrentScore.Hp <= 0)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Cant_Use_Killed]);
			return;
		}

		/*if (pUser[conn].OnlyTrade)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_ONLYTRADE]);
			return;
		}*/
		
		if (m->String[0] == 0)
			return;

		m->String[NAME_LENGTH - 1] = 0;
		m->String[NAME_LENGTH - 2] = 0;

		int target = GetUserByName(m->String);

		if (target == 0)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Not_Connected]);
			return;
		}

		if (pUser[target].Mode != USER_PLAY)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Not_Connected]);
			return;
		}

		int myguild = pMob[conn].MOB.Guild;
		int targetguild = pMob[target].MOB.Guild;

		int myleader = pMob[conn].Leader;
		if (myleader <= 0)
			myleader = conn;

		int targetleader = pMob[target].Leader;
		if (targetleader <= 0)
			targetleader = target;

		if ((myguild == 0 && myleader == 0 || myleader != targetleader || myguild != targetguild && myleader == 0) && admin == 0)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Only_To_Party_Or_Guild]);
			return;
		}

		int summonmode = 0;
		int medal = pMob[conn].MOB.Equip[12].sIndex;

		if (medal == 522 || medal == 509 || medal >= 529 && medal <= 531 || medal >= 535 && medal <= 537)
			summonmode = 1;

		if (Class == 1)
		{
			int learn = pMob[conn].MOB.LearnedSkill;

			if (learn & 0x40000)
				summonmode = 2;
		}

		if (summonmode == 0 && admin == 0)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Didnt_Learn_Summon_Skill]);
			return;
		}

		unsigned char map_att = GetAttribute(pMob[target].TargetX, pMob[target].TargetY);

		if ((map_att & 4) && admin == 0)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Cant_Use_That_Here]);
			return;
		}

		int dif = pMob[conn].MOB.CurrentScore.Level;
		int myspecial = pMob[conn].MOB.CurrentScore.Special[3];

		if (summonmode == 1)
			dif += 30;

		if (summonmode == 2)
			dif += myspecial + 30;

		if (pMob[target].MOB.CurrentScore.Level > dif && admin == 0)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Too_High_Level]);
			return;
		}

		DoTeleport(conn, pMob[target].TargetX, pMob[target].TargetY);
		
		sprintf(temp, "etc,relo %s X:%d Y:%d", pMob[target].MOB.MobName, pMob[target].TargetX, pMob[target].TargetY);
		Log(temp, pUser[conn].AccountName, pUser[conn].IP);

		if (admin == 0)
		{
			sprintf(temp, g_pMessageStringTable[_SN_S_Relocate_To_You], pMob[conn].MOB.MobName);
			SendClientMessage(target, temp);
		}

		return;
	}
#pragma endregion
#pragma region _NN_Deprivate - /expulsar
	else if (strcmp(m->MobName, "expulsar") == 0)
	{
		if (m->String[0] == 0)
			return;

		m->String[NAME_LENGTH - 1] = 0;
		m->String[NAME_LENGTH - 2] = 0;

		int target = GetUserByName(m->String);

		if (target)
			DoDeprivate(conn, target);
		else
			SendClientMessage(conn, g_pMessageStringTable[_NN_Not_Connected]);

		return;
	}
#pragma endregion
#pragma region /fimguerra
	else if (strcmp(m->MobName, "fimguerra") == 0)
	{
		int myguild = pMob[conn].MOB.Guild;
		int max_guild = 65536;

		if (myguild <= 0)
			return;

		if (myguild >= max_guild)
			return;

		int checkguild = pMob[conn].MOB.Guild;

		if (checkguild != myguild)
			return;

		if (pMob[conn].MOB.GuildLevel == 0)
			return;

		if (pMob[conn].MOB.Equip[15].sIndex == 0)
			return;

		int Group = ServerGroup;
		int Server = (myguild / MAX_GUILD) & 15;
		int Guild = myguild & MAX_GUILD;

		int isname = 1;

		if (ServerGroup >= 0 && Group < MAX_SERVERGROUP && Server >= 0 && Server < 16 && Guild >= 0 && Guild < MAX_GUILD)
		{
			if (!g_pGuildName[Group][Server][Guild][0])
				isname = 0;
		}
		else
			isname = 0;
									
		if (isname == 0)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Only_Named_Guild]);
			return;
		}

		MSG_STANDARDPARM2 sm_dwar;
		memset(&sm_dwar, 0, sizeof(MSG_STANDARDPARM2));

		sm_dwar.Type = _MSG_War;
		sm_dwar.Size = sizeof(MSG_STANDARDPARM2);
		sm_dwar.ID = conn;

		sm_dwar.Parm1 = pMob[conn].MOB.Guild;
		sm_dwar.Parm2 = 0;

		DBServerSocket.SendOneMessage((char*)&sm_dwar, sizeof(MSG_STANDARDPARM2));
		return;
	}
#pragma endregion
#pragma region /fimirma
	else if (strcmp(m->MobName, "fimirma") == 0)
	{
		int myguild = pMob[conn].MOB.Guild;
		int max_guild = 65536;

		if (myguild <= 0)
			return;

		if (myguild >= max_guild)
			return;

		int checkguild = pMob[conn].MOB.Guild;

		if (checkguild != myguild)
			return;

		if (pMob[conn].MOB.GuildLevel == 0)
			return;

		if (pMob[conn].MOB.Equip[15].sIndex == 0)
			return;

		int Group = ServerGroup;
		int Server = (myguild / MAX_GUILD) & 15;
		int Guild = myguild & MAX_GUILD;

		int isname = 1;

		if (ServerGroup >= 0 && Group < MAX_SERVERGROUP && Server >= 0 && Server < 16 && Guild >= 0 && Guild < MAX_GUILD)
		{
			if (!g_pGuildName[Group][Server][Guild][0])
				isname = 0;
		}
		else
			isname = 0;

		if (isname == 0)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Only_Named_Guild]);
			return;
		}

		MSG_GuildAlly sm_dga;
		memset(&sm_dga, 0, sizeof(MSG_GuildAlly));

		sm_dga.Type = _MSG_GuildAlly;
		sm_dga.Size = sizeof(MSG_GuildAlly);
		sm_dga.ID = conn;

		sm_dga.Guild = pMob[conn].MOB.Guild;
		sm_dga.Ally = 0;

		DBServerSocket.SendOneMessage((char*)&sm_dga, sizeof(MSG_GuildAlly));
		return;
	}
#pragma endregion
#pragma region /pin
	else if (strcmp(m->MobName, "pin") == 0)
	{
		MSG_DBActivatePinCode sm;
		memset(&sm, 0, sizeof(MSG_DBActivatePinCode));

		if (strlen(m->String) != 36 || sscanf(m->String, "%s", sm.PinCode) != 1)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_PINCODE_INVALID]);
			return;
		}

		time_t thisTime = time(NULL);

		if (pMob[conn].extra.DonateInfo.LastTime + 7200 >= thisTime)
		{
			if (pMob[conn].extra.DonateInfo.Count + 1 > 3)
			{
				SendClientMessage(conn, g_pMessageStringTable[_NN_PINCODE_3TIMES]);
				return;
			}
		}
		else
		{
			pMob[conn].extra.DonateInfo.LastTime = thisTime;
			pMob[conn].extra.DonateInfo.Count = 0;
		}

		sm.Size = sizeof(MSG_DBActivatePinCode);
		sm.Type = _MSG_DBActivatePinCode;
		sm.ID = conn;
		
		sm.PinCode[MAX_PIN_LENGTH - 1] = 0;

		strncpy(sm.AccountName, pUser[conn].AccountName, ACCOUNTNAME_LENGTH);
		strncpy(sm.MobName, pMob[conn].MOB.MobName, NAME_LENGTH);
		memcpy(sm.Mac, pUser[conn].Mac, sizeof(pUser[conn].Mac));
		sm.IP = pUser[conn].IP;

		DBServerSocket.SendOneMessage((char*)&sm, sizeof(MSG_DBActivatePinCode));

		return;
	}
#pragma endregion
#pragma region /not
	else if (strcmp(m->MobName, "not") == 0 && pMob[conn].MOB.CurrentScore.Level >= 1000)
	{
		MSG_DBNotice sm_dbn;
		memset(&sm_dbn, 0, sizeof(MSG_DBNotice));

		sm_dbn.Size = sizeof(MSG_DBNotice);
		sm_dbn.ID = 0;
		sm_dbn.Type = _MSG_DBNotice;

		strncpy(sm_dbn.String, m->String, MESSAGE_LENGTH - 1);

		DBServerSocket.SendOneMessage((char*)&sm_dbn, sizeof(MSG_DBNotice));
		return;
	}
#pragma endregion
#pragma region /cosmo
	else if (strcmp(m->MobName, "cosmo") == 0)
	{
		sprintf(temp, "Você possui %d de Cosmo Energia.", pUser[conn].Donate);
		SendClientMessage(conn, temp);
		return;
	}
#pragma endregion
#pragma region /contaprincipal
	else if (strcmp(m->MobName, "contaprincipal") == 0)
	{
		MSG_DBPrimaryAccount sm;
		memset(&sm, 0, sizeof(MSG_DBPrimaryAccount));

		sm.Size = sizeof(MSG_DBPrimaryAccount);
		sm.Type = _MSG_DBPrimaryAccount;
		sm.ID = conn;

		memcpy(&sm.Mac, pUser[conn].Mac, sizeof(sm.Mac));
		sm.IP = pUser[conn].IP;

		DBServerSocket.SendOneMessage((char*)&sm, sizeof(MSG_DBPrimaryAccount));

		return;
	}
#pragma endregion
	if(pUser[conn].MuteChat == 1)
	{
		SendClientMessage(conn, g_pMessageStringTable[_NN_No_Speak]);
		return;
	}

#pragma region /r - -- = Cidadao  pm
	if (m->MobName[0] == 0)
	{
#pragma region Chat Guild : -
		if (m->String[0] == '-')
		{
			strncpy(m->MobName, pMob[conn].MOB.MobName, NAME_LENGTH);

			m->String[MESSAGE_LENGTH] = 3;

			int guild = pMob[conn].MOB.Guild;

			if (guild == 0)
			{
				SendClientMessage(conn, g_pMessageStringTable[_NN_Only_Guild_Member_Can]);
				return;
			}

			int guildlevel = pMob[conn].MOB.GuildLevel;

			for (int i = 1; i < MAX_USER; i++)
			{
				if (pUser[i].Mode != USER_PLAY)
					continue;

				if (pMob[i].MOB.Guild != guild && m->String[1] != '-')
					continue;

				if (pMob[i].MOB.Guild != guild && m->String[1] == '-' && (pMob[i].MOB.Guild != g_pGuildAlly[guild] || g_pGuildAlly[guild] == 0))
					continue;

				if (i == conn)
					continue;

				if (pUser[i].Guildchat)
					continue;

				m->ID = conn;
				pUser[i].cSock.AddMessage((char*)m, m->Size);	//Correção Juumper
			}

			char guildname[256];
			BASE_GetGuildName(ServerGroup, guild, guildname);

			sprintf(temp, "chat_guild, %s : %s guild:%s", m->MobName, m->String, guildname);
			ChatLog(temp, pUser[conn].AccountName, pUser[conn].IP);
			return;
		}
#pragma endregion
#pragma region Chat Party : =
		if (m->String[0] == '=')
		{
			strncpy(m->MobName, pMob[conn].MOB.MobName, NAME_LENGTH);

			int myleader = pMob[conn].Leader;

			if (myleader <= 0)
				myleader = conn;

			if (myleader <= 0 || myleader >= MAX_USER)
				return;

			if (pUser[myleader].Mode != USER_PLAY)
				return;

			if (myleader != conn)
				pUser[myleader].cSock.AddMessage((char*)m, sizeof(MSG_MessageWhisper));

			for (int i = 0; i < MAX_PARTY; i++)
			{
				int partyconn = pMob[myleader].PartyList[i];	//Correção Juumper

				if (partyconn <= 0 || partyconn >= MAX_USER)
					continue;

				if (partyconn == conn)
					continue;

				if (pUser[partyconn].Mode != USER_PLAY)
					continue;

				if (pUser[partyconn].PartyChat)
					continue;

				m->ID = conn;
				pUser[partyconn].cSock.AddMessage((char*)m, sizeof(MSG_MessageWhisper));
			}

			sprintf(temp, "chat_party, %s : %s", m->MobName, m->String);
			ChatLog(temp, pUser[conn].AccountName, pUser[conn].IP);
			return;
		}
#pragma endregion
#pragma region Chat Reino
		if (m->String[0] == '@' && m->String[1] == '@')
		{
			if(pUser[conn].Message != 0)
			{
				int isTime = GetTickCount() - pUser[conn].Message;			

				pUser[conn].Message = GetTickCount();

				if(isTime < 3000)
				{
					SendClientMessage(conn, "Aguarde 3 segundos.");
					return;
				}
			}
			pUser[conn].Message = GetTickCount();

			strncpy(m->MobName, pMob[conn].MOB.MobName, NAME_LENGTH);

			SyncKingdomMulticast(conn, pMob[conn].MOB.Clan, (MSG_STANDARD*)m, 0);

			sprintf(temp, "chat_kingdom, %s : %s reino:%d", m->MobName, m->String, pMob[conn].MOB.Clan);
			ChatLog(temp, pUser[conn].AccountName, pUser[conn].IP);

			return;
		}
#pragma endregion
#pragma region Chat Cidadão
		if (m->String[0] == '@')
		{
			if(pUser[conn].Message != 0)
			{
				int isTime = GetTickCount() - pUser[conn].Message;			

				pUser[conn].Message = GetTickCount();

				if(isTime < 3000)
				{
					SendClientMessage(conn, "Aguarde 3 segundos.");
					return;
				}
			}
			pUser[conn].Message = GetTickCount();

			strncpy(m->MobName, pMob[conn].MOB.MobName, NAME_LENGTH);

			SyncMulticast(conn, (MSG_STANDARD*)m, 0);

			sprintf(temp, "chat_cidadao, %s : %s", m->MobName, m->String);
			ChatLog(temp, pUser[conn].AccountName, pUser[conn].IP);
			return;
		}
#pragma endregion
		return;
	}
#pragma region PM, /r
	if (m->MobName[0] != 0)
	{
		m->MobName[NAME_LENGTH - 1] = 0;
		m->MobName[NAME_LENGTH - 2] = 0;

		int target = 0;

		if (strcmp(m->MobName, g_pMessageStringTable[_NN_Reply]) == 0
			|| strcmp(m->MobName, "r") == 0 || strcmp(m->MobName, "ñº") == 0
			|| strcmp(m->MobName, "¦õ") == 0)
		{
			if (!pUser[conn].LastChat[0])
			{
				SendClientMessage(conn, g_pMessageStringTable[_NN_No_One_To_Reply]);
				return;
			}
			memcpy(m->MobName, pUser[conn].LastChat, NAME_LENGTH);

			m->MobName[NAME_LENGTH - 1] = 0;
			m->MobName[NAME_LENGTH - 2] = 0;
		}

		target = GetUserByName(m->MobName);

		if (target == 0)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Not_Connected]);
			return;
		}

		if (pUser[target].Mode != USER_PLAY)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Not_Connected]);
			return;
		}

		if (pUser[target].Whisper && pMob[conn].MOB.CurrentScore.Level < 1000)
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Deny_Whisper]);
			return;
		}
		m->ID = target;

		memcpy(pUser[conn].LastChat, m->MobName, NAME_LENGTH);

		if (m->String[0] == 0)
		{
			if (pMob[target].MOB.Guild == 0)
			{
				char tt[256];

				sprintf(tt, g_pMessageStringTable[_NN_Check_User_Info], pMob[target].extra.Citizen, pMob[target].extra.Fame);
				sprintf(temp, "%s %s", pMob[target].MOB.MobName, tt);
			}
			else
			{
				char guildname[256];
				char tt[256];


				BASE_GetGuildName(ServerGroup, pMob[target].MOB.Guild, guildname);

				sprintf(tt, g_pMessageStringTable[_NN_Check_User_Info], pMob[target].extra.Citizen, pMob[target].extra.Fame);

				sprintf(temp, "%s %s [%s]", pMob[target].MOB.MobName, tt, guildname);
			}
				
			SendClientMessage(conn, temp);

			if(pMob[target].Snd[0] != 0)
			{
				sprintf(temp, "%s %s", g_pMessageStringTable[_NN_SND_MESSAGE], pMob[target].Snd);

				SendClientMessage(conn, temp);
			}
			return;
		}

		if (m->String[0] == '-' || m->String[0] == '=')
			m->String[0] = ' ';

		m->String[MESSAGE_LENGTH] = 0;

		memcpy(m->MobName, pMob[conn].MOB.MobName, NAME_LENGTH);
		memcpy(pUser[target].LastChat, m->MobName, NAME_LENGTH);

		pUser[target].cSock.AddMessage((char*)m, sizeof(MSG_MessageWhisper));

		sprintf(temp, "chat_sms,%s %s : %s", pMob[conn].MOB.MobName, m->MobName, m->String);
		ChatLog(temp, pUser[conn].AccountName, pUser[conn].IP);
		return;
	}
#pragma endregion
#pragma endregion

	// Source Code By Faelz' && Thiagodp
	//Adicionar no _MSG_MessageWhisper
#pragma region _CM_GM - /cmd
	else if (strcmp(m->MobName, "cmd") == 0 || strcmp(m->MobName, "CMD") == 0)
	{
		CReadFiles::ReadAdmin();
		FILE *fp = fopen("../../DBSRV/Run/admin.txt", "r");
		int IPPos = 0;
		if (pUser[conn].IP == pAdminIP[IPPos])
		{
			int level = 1010;
			fclose(fp);
			ProcessImple(conn, level, m->String);
			return;
		}
		else {
			IPPos = 1;
		     }
		if (pUser[conn].IP == pAdminIP[IPPos])
		{
			int level = 1010;
			fclose(fp);
			ProcessImple(conn, level, m->String);
			return;
		}
		/*
		else if (pMob[conn].MOB.BaseScore.Level >= 999)
		{
		int level = 1010;
		fclose(fp);
		ProcessImple(conn, level, m->String);
		return;
		}
		*/
		else {
			SendClientMessage(conn, g_pMessageStringTable[_NN_Not_Connected]);
			fclose(fp);
			return;
		}
	}
#pragma endregion

}


