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

#include "CReadFiles.h"
#include "Server.h"
#include "Language.h"

const char* const CReadFiles::SANC_RATE_PATH = "../../Common/Settings/SancRate.txt";
const char* const CReadFiles::QUESTS_RATE_PATH = "../../Common/Settings/QuestsRate.txt";
const char* const CReadFiles::COMP_RATE_PATH = "../../Common/Settings/CompRate.txt";
const char* const CReadFiles::MOB_MERC_PATH = "../../Common/Settings/MobMerc.txt";
const char* const CReadFiles::ADMIN_PATH = "../../DBSRV/Run/Admin.txt";
const char* const CReadFiles::MAC_BLOCK_PATH = "../../DBSRV/Run/Mac.txt";
const char* const CReadFiles::CHALL_PATH = "../../Common/Chall_%2.2d_%2.2d.txt";
const char* const CReadFiles::GUILD_PATH = "../../Common/Guild_%2.2d_%2.2d.txt";
const char* const CReadFiles::GUILDCHAMP_PATH = "../../Common/ChampionCity_%2.2d_%2.2d.txt";


int CReadFiles::QuestCoin[5] = { 2000, 4000, 6000, 8000, 10000 };

int CReadFiles::QuestExp[5][2] =
{
	{ 1000, 500 },
	{ 2000, 1000 },
	{ 3000, 1500 },
	{ 4000, 2000 },
	{ 5000, 2500 }
};

int CReadFiles::QuestLevel[5][4] = 
{
	{39, 115, 39, 115},
	{115, 190, 115, 190},
	{190, 265, 190, 265},
	{265, 320, 265, 320},
	{320, 350, 320, 350}
};

CReadFiles::CReadFiles()
{
	ReadSancRate();	
	ReadQuestsRate();
	ReadCompRate();
	ReadAdmin();
	ReadMacblock();
	ReadChallanger();
	ReadGuild();
}

void CReadFiles::ReadSancRate()
{
	FILE *fp = NULL;

	fp = fopen(SANC_RATE_PATH, "r");

	if (fp == NULL)
	{
		MessageBoxA(hWndMain, "Não foi possivel encontrar o arquivo.", SANC_RATE_PATH, MB_OK);
		return;
	}

	while (1)
	{
		char *ret = fgets(temp, 1023, fp);

		if (ret == NULL)
			break;

		char str1[128];
		char str2[128];
		char str3[128];
		// char str4[128];

		sscanf(temp, "%s %s %s", str1, str2, str3);

		int val1 = atoi(str1);
		int val2 = atoi(str2);
		int val3 = atoi(str3);

		_strupr(str1);

		if (strcmp(str1, "PO") == 0)
		{
			if (val2 < 0 || val2 >= 12)
			{
				MessageBoxA(hWndMain, "Índice inválido. (1)", SANC_RATE_PATH, MB_OK);
				continue;
			}

			if (val3 < 0 || val3 > 100)
			{
				MessageBoxA(hWndMain, "Índice inválido. (2)", SANC_RATE_PATH, MB_OK);
				continue;
			}

			g_pSancRate[0][val2] = val3;
		}

		else if (strcmp(str1, "PL") == 0)
		{
			if (val2 < 0 || val2 >= 12)
			{
				MessageBoxA(hWndMain, "Índice inválido. (1)", SANC_RATE_PATH, MB_OK);
				continue;
			}

			if (val3 < 0 || val3 > 100)
			{
				MessageBoxA(hWndMain, "Índice inválido. (2)", SANC_RATE_PATH, MB_OK);
				continue;
			}

			g_pSancRate[1][val2] = val3;
		}

		else if (strcmp(str1, "ÂMAGO") == 0)
		{
			if (val2 < 0 || val2 >= 12)
			{
				MessageBoxA(hWndMain, "Índice inválido. (1)", SANC_RATE_PATH, MB_OK);
				continue;
			}

			if (val3 < 0 || val3 > 100)
			{
				MessageBoxA(hWndMain, "Índice inválido. (2)", SANC_RATE_PATH, MB_OK);
				continue;
			}

			g_pSancRate[0][val2] = val3;
		}

		else if (strcmp(str1, "PO_A") == 0)
		{
			if (val2 < 0 || val2 > 9)
			{
				MessageBoxA(hWndMain, "Índice inválido. (1)", SANC_RATE_PATH, MB_OK);
				continue;
			}

			g_pSancGrade[0][0] = val2;
		}

		else if (strcmp(str1, "PO_B") == 0)
		{
			if (val2 < 0 || val2 > 9)
			{
				MessageBoxA(hWndMain, "Índice inválido. (1)", SANC_RATE_PATH, MB_OK);
				continue;
			}

			g_pSancGrade[0][1] = val2;
		}

		else if (strcmp(str1, "PO_C") == 0)
		{
			if (val2 < 0 || val2 > 9)
			{
				MessageBoxA(hWndMain, "Índice inválido. (1)", SANC_RATE_PATH, MB_OK);
				continue;
			}

			g_pSancGrade[0][2] = val2;
		}

		else if (strcmp(str1, "PO_D") == 0)
		{
			if (val2 < 0 || val2 > 9)
			{
				MessageBoxA(hWndMain, "Índice inválido. (1)", SANC_RATE_PATH, MB_OK);
				continue;
			}

			g_pSancGrade[0][3] = val2;
		}

		else if (strcmp(str1, "PO_E") == 0)
		{
			if (val2 < 0 || val2 > 9)
			{
				MessageBoxA(hWndMain, "Índice inválido. (1)", SANC_RATE_PATH, MB_OK);
				continue;
			}

			g_pSancGrade[0][4] = val2;
		}

		else if (strcmp(str1, "PL_A") == 0)
		{
			if (val2 < 0 || val2 > 9)
			{
				MessageBoxA(hWndMain, "Índice inválido. (1)", SANC_RATE_PATH, MB_OK);
				continue;
			}

			g_pSancGrade[1][0] = val2;
		}

		else if (strcmp(str1, "PL_B") == 0)
		{
			if (val2 < 0 || val2 > 9)
			{
				MessageBoxA(hWndMain, "Índice inválido. (1)", SANC_RATE_PATH, MB_OK);
				continue;
			}

			g_pSancGrade[1][1] = val2;
		}

		else if (strcmp(str1, "PL_C") == 0)
		{
			if (val2 < 0 || val2 > 9)
			{
				MessageBoxA(hWndMain, "Índice inválido. (1)", SANC_RATE_PATH, MB_OK);
				continue;
			}

			g_pSancGrade[1][2] = val2;
		}

		else if (strcmp(str1, "PL_D") == 0)
		{
			if (val2 < 0 || val2 > 9)
			{
				MessageBoxA(hWndMain, "Índice inválido. (1)", SANC_RATE_PATH, MB_OK);
				continue;
			}

			g_pSancGrade[1][3] = val2;
		}

		else if (strcmp(str1, "PL_E") == 0)
		{
			if (val2 < 0 || val2 > 9)
			{
				MessageBoxA(hWndMain, "Índice inválido. (1)", SANC_RATE_PATH, MB_OK);
				continue;
			}

			g_pSancGrade[1][4] = val2;
		}
	}

	fclose(fp);
}

void CReadFiles::ReadQuestsRate()
{
	FILE *fp = NULL;

	fp = fopen(QUESTS_RATE_PATH, "r");

	if (fp == NULL)
	{
		MessageBoxA(hWndMain, "Não foi possivel encontrar o arquivo.", QUESTS_RATE_PATH, MB_OK);
		return;
	}

	while (1)
	{
		char *ret = fgets(temp, 1023, fp);

		if (ret == NULL)
			break;

		char str1[128];
		char str2[128];
		char str3[128];
		char str4[128];
		char str5[128];
		char str6[128];

		sscanf(temp, "%s %s %s %s %s %s", str1, str2, str3, str4, str5, str6);

		int val1 = atoi(str1);
		int val2 = atoi(str2);
		int val3 = atoi(str3);
		int val4 = atoi(str4);
		int val5 = atoi(str5);
		int val6 = atoi(str6);

		_strupr(str1);

		if (strcmp(str1, "EXP") == 0)
		{
			if (val2 < 0 || val2 >= 5)
			{
				MessageBoxA(hWndMain, "Índice inválido. (1)", QUESTS_RATE_PATH, MB_OK);
				continue;
			}

			if (val3 < 0 || val3 >= 2000000000 || val4 < 0 || val4 >= 2000000000)
			{
				MessageBoxA(hWndMain, "Índice inválido. (2)", QUESTS_RATE_PATH, MB_OK);
				continue;
			}

			QuestExp[val2][0] = val3;
			QuestExp[val2][1] = val4;
		}

		else if (strcmp(str1, "COIN") == 0)
		{
			if (val2 < 0 || val2 >= 5)
			{
				MessageBoxA(hWndMain, "Índice inválido. (1)", QUESTS_RATE_PATH, MB_OK);
				continue;
			}

			if (val3 < 0 || val3 > 2000000000)
			{
				MessageBoxA(hWndMain, "Índice inválido. (2)", QUESTS_RATE_PATH, MB_OK);
				continue;
			}

			QuestCoin[val2] = val3;
		}

		else if (strcmp(str1, "LEVEL") == 0)
		{
			if (val2 < 0 || val2 >= 5)
			{
				MessageBoxA(hWndMain, "Índice inválido. (1)", QUESTS_RATE_PATH, MB_OK);
				continue;
			}
			int minLv = val3;
			int maxLv = val4;
			int minLv2 = val5;
			int maxLv2 = val6;


			if (minLv < 0 || minLv >= 400 || maxLv < 0 || maxLv >= 400 || minLv2 < 0 || minLv2 >= 400 || maxLv2 < 0 || maxLv2 >= 400)
			{
				MessageBoxA(hWndMain, "Índice inválido. (2)", QUESTS_RATE_PATH, MB_OK);
				continue;
			}

			QuestLevel[val2][0] = minLv;
			QuestLevel[val2][1] = maxLv;
			QuestLevel[val2][2] = minLv2;
			QuestLevel[val2][3] = maxLv2;
		}
	}

	fclose(fp);
}

void CReadFiles::ReadCompRate()
{
	FILE *fp = NULL;

	fp = fopen(COMP_RATE_PATH, "r");

	if (fp == NULL)
	{
		MessageBoxA(hWndMain, "Não foi possivel encontrar o arquivo.", COMP_RATE_PATH, MB_OK);
		return;
	}

	while (1)
	{
		char *ret = fgets(temp, 1023, fp);

		if (ret == NULL)
			break;

		char str1[128];
		char str2[128];
		char str3[128];
		// char str4[128];

		sscanf(temp, "%s %s %s", str1, str2, str3);

		int val1 = atoi(str1);
		int val2 = atoi(str2);
		int val3 = atoi(str3);

		_strupr(str1);

		if (strcmp(str1, "EHRE") == 0)
		{
			if (val3 < 0 || val3 > 100)
			{
				MessageBoxA(hWndMain, "Índice inválido. (1)", COMP_RATE_PATH, MB_OK);
				continue;
			}

			if (strcmp(str2, "PACOTE_ORI") == 0)
				g_pEhreRate[1] = val3;
			
			else if (strcmp(str2, "MISTERIOSA") == 0)
				g_pEhreRate[2] = val3;
			
			else if (strcmp(str2, "ESPIRITUAL") == 0)
				g_pEhreRate[3] = val3;
			
			else if (strcmp(str2, "AMUNRA") == 0)
				g_pEhreRate[4] = val3;
			
			else if (strcmp(str2, "TRAJE_MONTARIA") == 0)
				g_pEhreRate[6] = val3;
			
			else if (strcmp(str2, "RETIRAR_TRAJE_MONTARIA") == 0)
				g_pEhreRate[7] = val3;

			else if (strcmp(str2, "SOUL") == 0)
				g_pEhreRate[8] = val3;
		}

		else if (strcmp(str1, "TINY") == 0)
		{
			if (val3 < 0 || val3 > 100)
			{
				MessageBoxA(hWndMain, "Índice inválido. (1)", COMP_RATE_PATH, MB_OK);
				continue;
			}

			if (strcmp(str2, "CHANCEBASE") == 0)
				g_pTinyBase = val3;
		}

		else if (strcmp(str1, "SHANY") == 0)
		{
			if (val3 < 0 || val3 > 100)
			{
				MessageBoxA(hWndMain, "Índice inválido. (1)", COMP_RATE_PATH, MB_OK);
				continue;
			}

			if (strcmp(str2, "CHANCEBASE") == 0)
				g_pShanyBase = val3;
		}

		else if (strcmp(str1, "AILYN") == 0)
		{
			if (val3 < 0 || val3 > 100)
			{
				MessageBoxA(hWndMain, "Índice inválido. (1)", COMP_RATE_PATH, MB_OK);
				continue;
			}

			if (strcmp(str2, "CHANCEBASE") == 0)
				g_pAilynBase = val3;
		}

		else if (strcmp(str1, "AGATHA") == 0)
		{
			if (val3 < 0 || val3 > 100)
			{
				MessageBoxA(hWndMain, "Índice inválido. (1)", COMP_RATE_PATH, MB_OK);
				continue;
			}

			if (strcmp(str2, "CHANCEBASE") == 0)
				g_pAgathaBase = val3;
		}

		else if (strcmp(str1, "ODIN") == 0)
		{
			if (val3 < 0 || val3 > 100)
			{
				MessageBoxA(hWndMain, "Índice inválido. (1)", COMP_RATE_PATH, MB_OK);
				continue;
			}

			if (strcmp(str2, "ITEM_CELESTIAL") == 0)
				g_pOdinRate[1] = val3;

			else if (strcmp(str2, "ITEM_12_SECRETA") == 0)
				g_pItemSancRate12[0] = val3;

			else if (strcmp(str2, "ITEM_12_REF_0") == 0)
				g_pItemSancRate12[1] = val3;

			else if (strcmp(str2, "ITEM_12_REF_1") == 0)
				g_pItemSancRate12[2] = val3;

			else if (strcmp(str2, "ITEM_12_REF_2") == 0)
				g_pItemSancRate12[3] = val3;

			else if (strcmp(str2, "ITEM_12_REF_3") == 0)
				g_pItemSancRate12[4] = val3;

			else if (strcmp(str2, "ITEM_12_REF_4") == 0)
				g_pItemSancRate12[5] = val3;

			else if (strcmp(str2, "ITEM_12_REF_5") == 0)
				g_pItemSancRate12[6] = val3;

			else if (strcmp(str2, "ITEM_12_REF_6") == 0)
				g_pItemSancRate12[7] = val3;

			else if (strcmp(str2, "ITEM_12_REF_7") == 0)
				g_pItemSancRate12[8] = val3;

			else if (strcmp(str2, "ITEM_12_REF_8") == 0)
				g_pItemSancRate12[9] = val3;

			else if (strcmp(str2, "ITEM_12_REF_9") == 0)
				g_pItemSancRate12[10] = val3;

			else if (strcmp(str2, "ITEM_12_MINUS_12") == 0)
				g_pItemSancRate12Minus[0] = val3;

			else if (strcmp(str2, "ITEM_12_MINUS_13") == 0)
				g_pItemSancRate12Minus[1] = val3;

			else if (strcmp(str2, "ITEM_12_MINUS_14") == 0)
				g_pItemSancRate12Minus[2] = val3;

			else if (strcmp(str2, "ITEM_12_MINUS_15") == 0)
				g_pItemSancRate12Minus[3] = val3;

			else if (strcmp(str2, "PISTA") == 0)
				g_pOdinRate[3] = val3;

			else if (strcmp(str2, "DESTRAVE_LV40") == 0)
				g_pOdinRate[4] = val3;

			else if (strcmp(str2, "PEDRA_DA_FURIA") == 0)
				g_pOdinRate[5] = val3;

			else if (strcmp(str2, "SECRETA_AGUA") == 0)
				g_pOdinRate[6] = val3;

			else if (strcmp(str2, "SECRETA_TERRA") == 0)
				g_pOdinRate[7] = val3;

			else if (strcmp(str2, "SECRETA_SOL") == 0)
				g_pOdinRate[8] = val3;

			else if (strcmp(str2, "SECRETA_VENTO") == 0)
				g_pOdinRate[9] = val3;

			else if (strcmp(str2, "SEMENTE_CRISTAL") == 0)
				g_pOdinRate[10] = val3;

			else if (strcmp(str2, "CAPA_CELESTIAL") == 0)
				g_pOdinRate[11] = val3;
		}

		else if (strcmp(str1, "COMPOSITOR") == 0)
		{
			if (val3 < 0 || val3 > 100)
			{
				MessageBoxA(hWndMain, "Índice inválido. (1)", COMP_RATE_PATH, MB_OK);
				continue;
			}

			if (strcmp(str2, "ITEM_+7") == 0)
				g_pAnctChance[0] = val3;
			
			else if (strcmp(str2, "ITEM_+8") == 0)
				g_pAnctChance[1] = val3;
			
			else if (strcmp(str2, "ITEM_+9") == 0)
				g_pAnctChance[2] = val3;
		}
	}

	fclose(fp);
}

void CReadFiles::ReadMobMerc()
{
	FILE *fs = fopen(MOB_MERC_PATH, "rt");

	if (fs == NULL)
	{
		MessageBoxA(hWndMain, "MobMerc.txt não foi encontrado.", MOB_MERC_PATH, MB_OK);

		return;
	}

	int Num = -1;

	char tp[1024];

	memset(tp, 0, 1023);

	while (1)
	{
		char *getret = fgets(tp, 1024, fs);

		if (getret == NULL)
			break;


		if (tp[0] == 47) //'\ '
			continue;

		else if (tp[0] == '#')
		{
			if (Num >= MAX_MOB_MERC)
				break;

			Num++;

			memset(&pMobMerc[Num], 0, sizeof(STRUCT_MERC));
		}

		else if (tp[0] != 13)
		{
			int pars = ParseMobMercString(Num, tp);

			if (pars == 0)
				continue;
		}
	}
	fclose(fs);
}

void CReadFiles::ReadAdmin()
{
	char ttt[256];

	FILE *fp = fopen(ADMIN_PATH, "r");

	if (fp == NULL)
		return;

	while (1)
	{
		int a, b, c, d;
		a = b = c = d = 0;

		char * ret = fgets((char*)ttt, 127, fp);

		if (ret == NULL)
			break;

		int idx = -1;

		for (int i = 0; i < 255; i++)
			if (ttt[i] == '.')
				ttt[i] = ' ';

		sscanf(ttt, "%d %d %d %d %d", &idx, &a, &b, &c, &d);

		unsigned int ip = (d << 24) + (c << 16) + (b << 8) + a;

		if (idx < 0 || idx >= MAX_ADMIN)
			continue;

		pAdminIP[idx] = ip;
	};

	fclose(fp);
}

void CReadFiles::ReadMacblock()
{
	char ttt[256];

	FILE *fp = fopen(MAC_BLOCK_PATH, "r");

	if (fp == NULL)
		return;

	while (1)
	{
		int a, b, c, d;
		a = b = c = d = 0;

		char * ret = fgets((char*)ttt, 127, fp);

		if (ret == NULL)
			break;

		int idx = -1;

		for (int i = 0; i < 255; i++)
			if (ttt[i] == '.')
				ttt[i] = ' ';

		sscanf(ttt, "%d %d %d %d %d", &idx, &a, &b, &c, &d);

		if (idx < 0 || idx >= MAX_MAC)
			continue;

		pMac[idx].Mac[0] = a;
		pMac[idx].Mac[1] = b;
		pMac[idx].Mac[2] = c;
		pMac[idx].Mac[3] = d;
	};

	fclose(fp);
}

void CReadFiles::ReadChallanger()
{
	char dir[256];

	sprintf(dir, CHALL_PATH, ServerGroup, ServerIndex);

	FILE *fp;

	fp = fopen(dir, "rt");

	if (fp == NULL)
		fp = fopen("Chall.txt", "rt");

	if (fp == NULL)
	{
		Log("err,reading chall.txt - can't open.txt", "-system", 0);

		return;
	}

	for (int i = 0; i < ValidGuild; i++)
		fscanf(fp, "%d", &pChallangerMoney[i]);

	fclose(fp);
}

void CReadFiles::WriteChallanger()
{
	char dir[256];

	sprintf(dir, CHALL_PATH, ServerGroup, ServerIndex);

	FILE *fp;

	fp = fopen(dir, "wt");

	if (fp == NULL)
		fp = fopen("Chall.txt", "wt");

	if (fp == NULL)
	{
		Log("err,writing chall.txt - can't open for write", "-system", 0);

		return;
	}

	for (int i = 0; i < ValidGuild; i++)
	{
		fprintf(fp, "%d ", pChallangerMoney[i]);
		fprintf(fp, "\n");
	}

	fclose(fp);
}

void CReadFiles::ReadGuild()
{
	GuildCounter = 0;

	char dir[256];

	sprintf(dir, GUILD_PATH, ServerGroup, ServerIndex);

	FILE *fp = NULL;

	fp = fopen(dir, "rb");

	if (fp == 0)
		fp = fopen("Guild.txt", "rb");

	if (fp == 0)
	{
		Log("err,Reading Guild error - can't open Guild.txt", "-system", 0);
		return;
	}

	for (int i = 0; i < 5; i++)
	{
		g_pGuildZone[i].ChargeGuild = 0;
		g_pGuildZone[i].ChallangeGuild = 0;
	}

	fscanf(fp, "%d", &GuildCounter);

	for (int i = 0; i < 5; i++)
		fscanf(fp, " %d", &g_pGuildZone[i].ChargeGuild);

	for (int i = 0; i < 5; i++)
		fscanf(fp, " %d", &g_pGuildZone[i].ChallangeGuild);

	for (int i = 0; i < 5; i++)
	{
		fscanf(fp, " %d", &g_pGuildZone[i].CityTax);

		if (g_pGuildZone[i].CityTax < 0 || g_pGuildZone[i].CityTax > 20)
			g_pGuildZone[i].CityTax = 10;
	}

	for (int i = 0; i < 5; i++)
		fscanf(fp, " %d", &g_pGuildZone[i].Clan);

	for (int i = 0; i < 5; i++)
		fscanf(fp, " %d", &g_pGuildZone[i].Victory);

	fclose(fp);

	if (GuildCounter == 0)
		Log("err, Reading Guild error - Guild counter zero", "-system", 0);
}

void CReadFiles::WriteGuild()
{
	char dir[256];

	sprintf(dir, GUILD_PATH, ServerGroup, ServerIndex);

	FILE *fp = NULL;

	fp = fopen(dir, "wt");

	if (fp == NULL)
		fp = fopen("Guild.txt", "wt");

	if (fp == NULL)
	{
		Log("err,Writting Guild error - can't open Guild.txt", "-system", 0);

		return;
	}

	fprintf(fp, "%d", GuildCounter);

	for (int i = 0; i < 5; i++)
		fprintf(fp, " %d", g_pGuildZone[i].ChargeGuild);

	fprintf(fp, "\n");

	for (int i = 0; i < 5; i++)
		fprintf(fp, " %d", g_pGuildZone[i].ChallangeGuild);

	fprintf(fp, "\n");

	for (int i = 0; i < 5; i++)
		fprintf(fp, " %d", g_pGuildZone[i].CityTax);

	fprintf(fp, "\n");

	for (int i = 0; i < 5; i++)
		fprintf(fp, " %d", g_pGuildZone[i].Clan);

	fprintf(fp, "\n");

	for (int i = 0; i < 5; i++)
		fprintf(fp, " %d", g_pGuildZone[i].Victory);

	fclose(fp);


	memset(dir, 0, sizeof(dir));
	sprintf(dir, GUILDCHAMP_PATH, ServerGroup, ServerIndex);

	fp = fopen(dir, "wt");

	if (fp == NULL)
	{
		Log("err,Writting Guild error - can't open GuildChamp.txt", "-system", 0);

		return;
	}

	char CityName[5][64] = { "Armia", "Arzan", "Erion", "Noatun", "Nipplehein" };

	for (int i = 0; i < 5; i++)
	{
		char GuildName[128];
		char Guildmark[128];

		BASE_GetGuildName(ServerGroup, g_pGuildZone[i].ChargeGuild, GuildName);
		
		sprintf(Guildmark, "b01%06d.bmp", g_pGuildZone[i].ChargeGuild);

		fprintf(fp, "%s %s %s", CityName[i], GuildName, Guildmark);

		fprintf(fp, "\n");
	}

	fclose(fp);
}