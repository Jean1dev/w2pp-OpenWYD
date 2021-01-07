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
#include <string.h>
#include <string>
#include <vector>
#include <list>
#include <map>

#include "CNPCGene.h"
#include "Server.h"

using namespace std;

CNPCGenerator::CNPCGenerator()
{
	NumList = 0;
	NumRegion = 0;

	memset(pList, 0, sizeof(NPCGENLIST)* MAX_NPCGENERATOR);
	memset(pRegion, 0, sizeof(MAPREGION)* MAX_NPCGENERATOR);
}

CNPCGenerator::~CNPCGenerator()
{
}

int CNPCGenerator::ReadNPCGenerator()
{
	FILE *fp = fopen("NPCGener.new.txt", "wt");
	FILE *fs = fopen("NPCGener.txt", "rt");

	if (fs == NULL)
	{
		MessageBoxA(hWndMain, "NPCGener.txt¦í ¥", "+-¦Ô+¡ ¢Ããð", MB_OK);

		return 0;
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
		{
			char tmp1[128];
			char tmp2[128];

			sscanf(tp, "%s %s", &tmp1, &tmp2);

			if (tmp1 && tmp2 && tmp2[0] != '*')
				fprintf(fp, "\n\n%s", &tp);
		}

		else if (tp[0] == '#')
		{
			Num++;

			for (int i = 0; i < 5; i++)
			{
				pList[Num].SegmentListX[i] = 0;
				pList[Num].SegmentListY[i] = 0;
			}
			fprintf(fp, "// ********************************************\n");

			if (tp[1] == '*')
				fprintf(fp, "#*\t[%4d]\n", Num);

			else
				fprintf(fp, "#\t[%4d]\n", Num);
		}

		else if (tp[0] != 13)
		{
			int pars = ParseString(Num, tp);

			if (pars == 0)
				continue;

			char tmp1[128];
			char tmp2[128];

			sscanf(tp, "%s %s", &tmp1, &tmp2);

			int len = strlen(tmp1);

			if (len > 8 && tmp1[0] != 'Í')
				fprintf(fp, "\t%s\t%s\n", &tmp1, &tmp2);

			else if(tmp1[0] != 'Í')
				fprintf(fp, "\t%s\t\t%s\n", &tmp1, &tmp2);
		}
	}

	NumList = Num + 1;

	fclose(fs);
	fclose(fp);

	return 1;
}

int CNPCGenerator::ParseString(int i, char *str)
{
	char str1[128];
	str1[0] = 0;

	char str2[128];
	str2[0] = 0;

	char str3[128];
	str3[0] = 0;

	int  value = 0;
	int secondvalue = 0;

	sscanf(str, "%s %s %s", str1, str2, str3);

	if (str[0] == 47)
		return FALSE;

	if (str[0] == 0) 
		return FALSE;

	value = atoi(str2);
	secondvalue = atoi(str3);

	if (strlen(str2) > 80)
		return FALSE;


	if (!strcmp(str1, "MinuteGenerate:"))
		pList[i].MinuteGenerate = value;

	else if (!strcmp(str1, "MaxNumMob:"))
		pList[i].MaxNumMob = value;

	else if (!strcmp(str1, "MinGroup:"))
		pList[i].MinGroup = value;

	else if (!strcmp(str1, "MaxGroup:"))
		pList[i].MaxGroup = value;

	else if (!strcmp(str1, "Leader:"))
	{
		if (!strcmp(str2, "0"))
			return FALSE;

		strncpy(pList[i].Leader.MobName, str2, NAME_LENGTH);

		BOOL ret = ReadMob(&pList[i].Leader, "npc");

		if (ret == FALSE)
			MessageBox(hWndMain, str2, "Can't read NPC file in NPCGener.txt", NULL);

		else
			pList[i].Mode = MOB_USE;
	}

	else if (!strcmp(str1, "Follower:"))
	{
		if (!strcmp(str2, "0"))
			return FALSE;

		strncpy(pList[i].Follower.MobName, str2, NAME_LENGTH);

		BOOL ret = ReadMob(&pList[i].Follower, "npc");

		if (ret == FALSE)
			MessageBox(hWndMain, str2, "Can't read NPC file in NPCGener.txt", NULL);

		else
			pList[i].Mode = MOB_USE;
	}
	else if (!strcmp(str1, "RouteType:"))
		pList[i].RouteType = value;

	else if (!strcmp(str1, "Formation:"))
		pList[i].Formation = value;

	else if (!strcmp(str1, "StartWait:"))
		pList[i].SegmentWait[0] = value;

	else if (!strcmp(str1, "StartRange:"))
		pList[i].SegmentRange[0] = value;

	else if (!strcmp(str1, "StartX:"))
	{
		if(value <= 0 || value >= MAX_GRIDX)
			MessageBoxA(NULL, str1, pList[i].Leader.MobName, MB_OK);

		pList[i].SegmentListX[0] = value;
	}
	else if (!strcmp(str1, "StartY:"))
	{
		if(value <= 0 || value >= MAX_GRIDX)
			MessageBoxA(NULL, str1, pList[i].Leader.MobName, MB_OK);
		pList[i].SegmentListY[0] = value;
	}
	else if (!strcmp(str1, "DestRange:"))
		pList[i].SegmentRange[4] = value;

	else if (!strcmp(str1, "DestX:"))
		pList[i].SegmentListX[4] = value;

	else if (!strcmp(str1, "DestY:"))
		pList[i].SegmentListY[4] = value;

	else if (!strcmp(str1, "DestWait:"))
		pList[i].SegmentWait[4] = value;

	else if (!strcmp(str1, "Segment1Range:"))
		pList[i].SegmentRange[1] = value;

	else if (!strcmp(str1, "Segment1X:"))
		pList[i].SegmentListX[1] = value;

	else if (!strcmp(str1, "Segment1Y:"))
		pList[i].SegmentListY[1] = value;

	else if (!strcmp(str1, "Segment1Wait:"))
		pList[i].SegmentWait[1] = value;

	else if (!strcmp(str1, "Segment2Range:"))
		pList[i].SegmentRange[2] = value;

	else if (!strcmp(str1, "Segment2X:"))
		pList[i].SegmentListX[2] = value;

	else if (!strcmp(str1, "Segment2Y:"))
		pList[i].SegmentListY[2] = value;

	else if (!strcmp(str1, "Segment2Wait:"))
		pList[i].SegmentWait[2] = value;

	else if (!strcmp(str1, "Segment3Range:"))
		pList[i].SegmentRange[3] = value;

	else if (!strcmp(str1, "Segment3X:"))
		pList[i].SegmentListX[3] = value;

	else if (!strcmp(str1, "Segment3Y:"))
		pList[i].SegmentListY[3] = value;

	else if (!strcmp(str1, "Segment3Wait:"))
		pList[i].SegmentWait[3] = value;

	else if (!strcmp(str1, "StartAction:"))
		SetAct(pList[i].SegmentAction[0], str2);

	else if (!strcmp(str1, "Segment1Action:"))
		SetAct(pList[i].SegmentAction[1], str2);

	else if (!strcmp(str1, "Segment2Action:"))
		SetAct(pList[i].SegmentAction[2], str2);

	else if (!strcmp(str1, "Segment3Action:"))
		SetAct(pList[i].SegmentAction[3], str2);

	else
		return FALSE;

	return TRUE;
}

void CNPCGenerator::ReadRegion()
{
	FILE *fp = NULL;

	fp = fopen("Regions.txt", "rb");

	if (fp == NULL)
	{
		MessageBoxA(hWndMain, "Não foi possivel carregar regions.txt", "WARNING!", MB_OK);
		return;
	}

	while (1)
	{
		char *ret = fgets(temp, 1023, fp);

		if (ret == NULL)
			break;

		sscanf(temp, "%d, %d, %d, %d = %s", &pRegion[NumRegion].minX, &pRegion[NumRegion].minY, &pRegion[NumRegion].maxX, &pRegion[NumRegion].maxY, pRegion[NumRegion].RegionName);

		NumRegion++;
	}
	fclose(fp);
}

void CNPCGenerator::DropList()
{
	list<char*> MobList;

	for (int i = 0; i < NumList; i++)
	{
		int Leader = 1;
		int Follower = 1;

		for (list<char*>::iterator l = MobList.begin(); l != MobList.end(); ++l)
		{
			if (strncmp(*l, pList[i].Leader.MobName, strlen(pList[i].Leader.MobName)) == 0)
				Leader = 0;

			if (strncmp(*l, pList[i].Follower.MobName, strlen(pList[i].Leader.MobName)) == 0)
				Follower = 0;
		}

		if (Leader == 0 && Follower == 0)
			continue;

		char filename[256];

		int j = 0;

		for (j = 0; j < NumRegion; j++)
		{
			if (pList[i].SegmentListX[0] >= pRegion[j].minX && pList[i].SegmentListY[0] >= pRegion[j].minY &&
				pList[i].SegmentListX[0] <= pRegion[j].maxX && pList[i].SegmentListY[0] <= pRegion[j].maxY)
			{
				strcpy(filename, pRegion[j].RegionName);
				break;
			}
		}

		if (j == NumRegion)
			strcpy(filename, "Zona_Desconhecida");

		sprintf(temp, "./Drop/%s.txt", filename);
		FILE *fp = fopen(temp, "a+");

		if (Leader)
		{
			STRUCT_MOB *MOB = &pList[i].Leader;

			fprintf(fp, "MobName: %s\r\n\n", MOB->MobName);

			int count = 0;

			for (int k = 0; k < MAX_CARRY; k++)
			{
				if (MOB->Carry[k].sIndex == 0 || MOB->Carry[k].sIndex == 419 || MOB->Carry[k].sIndex == 420 || MOB->Carry[k].sIndex == 412 || MOB->Carry[k].sIndex == 413)
					continue;

				int price = g_pItemList[MOB->Carry[k].sIndex].Price;
				
				//if (price < 100000)
				//	continue;

				int chance = 0;

				if (k >= 0 && k <= 7)
					chance = 1000;

				else if (k >= 8 && k <= 11)
					chance = 4;

				else if (k >= 12 && k <= 23)
					chance = 1000;

				else if (k >= 24 && k <= 55)
					chance = 1000;

				else if (k == 56)
					chance = 1;

				else if (k == 57)
					chance = 100;

				else if (k == 58)
					chance = 500;

				else if (k >= 60)
					chance = 2500;

				else if (k == 61 || k == 62)
					chance = 5000;

				else
					chance = 10000;

				fprintf(fp, "%s(%d)(1/%d), ", g_pItemList[MOB->Carry[k].sIndex].Name, MOB->Carry[k].sIndex, chance);

				if (++count % 5 == 0)
					fputs("\r\n", fp);
			}
			fputs("\r\n", fp);
			for (int k = 0; k < MAX_CARRY; k++)
			{
				if (MOB->Carry[k].sIndex == 0 || MOB->Carry[k].sIndex != 419 && MOB->Carry[k].sIndex != 420 && MOB->Carry[k].sIndex != 412 && MOB->Carry[k].sIndex != 413)
					continue;

				int price = g_pItemList[MOB->Carry[k].sIndex].Price;

				//if (price < 100000)
				//	continue;

				int chance = 0;

				if (k >= 0 && k <= 7)
					chance = 1000;

				else if (k >= 8 && k <= 11)
					chance = 4;

				else if (k >= 12 && k <= 23)
					chance = 1000;

				else if (k >= 24 && k <= 55)
					chance = 1000;

				else if (k == 56)
					chance = 1;

				else if (k == 57)
					chance = 100;

				else if (k == 58)
					chance = 500;

				else if (k >= 60)
					chance = 2500;

				else if (k == 61 || k == 62)
					chance = 5000;

				else
					chance = 10000;

				fprintf(fp, "%s(%d)(1/%d), ", g_pItemList[MOB->Carry[k].sIndex].Name, MOB->Carry[k].sIndex, chance);

				if (++count % 5 == 0)
					fputs("\r\n", fp);
			}

			if (count % 5)
				fputs("\r\n", fp);

			fputs("\r\n", fp);

			MobList.push_back(MOB->MobName);
		}

		for (list<char*>::iterator l = MobList.begin(); l != MobList.end(); ++l)
		{
			if (strncmp(*l, pList[i].Follower.MobName, strlen(pList[i].Leader.MobName)) == 0)
				Follower = 0;
		}

		if (Follower)
		{
			STRUCT_MOB *MOB = &pList[i].Follower;

			fprintf(fp, "MobName: %s\r\n\n", MOB->MobName);

			int count = 0;

			for (int k = 0; k < MAX_CARRY; k++)
			{
				if (MOB->Carry[k].sIndex == 0 || MOB->Carry[k].sIndex == 412 || MOB->Carry[k].sIndex == 413 || MOB->Carry[k].sIndex == 419 || MOB->Carry[k].sIndex == 420)
					continue;

				int price = g_pItemList[MOB->Carry[k].sIndex].Price;

				//if (price < 100000)
				//	continue;


				int chance = 0;

				if (k >= 0 && k <= 7)
					chance = 1000;

				else if (k >= 8 && k <= 11)
					chance = 4;

				else if (k >= 12 && k <= 23)
					chance = 1000;

				else if (k >= 24 && k <= 55)
					chance = 1000;

				else if (k == 56)
					chance = 1;

				else if (k == 57)
					chance = 100;

				else if (k == 58)
					chance = 500;

				else if (k >= 60)
					chance = 2500;

				else if (k == 61 || k == 62)
					chance = 5000;

				else
					chance = 10000;

				fprintf(fp, "%s(%d)(1/%d), ", g_pItemList[MOB->Carry[k].sIndex].Name, MOB->Carry[k].sIndex, chance);

				if (++count % 5 == 0)
					fputs("\r\n", fp);
			}
			fputs("\r\n", fp);
			for (int k = 0; k < MAX_CARRY; k++)
			{
				if (MOB->Carry[k].sIndex == 0 || MOB->Carry[k].sIndex != 419 && MOB->Carry[k].sIndex != 420 && MOB->Carry[k].sIndex != 412 && MOB->Carry[k].sIndex != 413)
					continue;

				int price = g_pItemList[MOB->Carry[k].sIndex].Price;

				//if (price < 100000)
				//	continue;


				int chance = 0;

				if (k >= 0 && k <= 7)
					chance = 1000;

				else if (k >= 8 && k <= 11)
					chance = 4;

				else if (k >= 12 && k <= 23)
					chance = 1000;

				else if (k >= 24 && k <= 55)
					chance = 1000;

				else if (k == 56)
					chance = 1;

				else if (k == 57)
					chance = 100;

				else if (k == 58)
					chance = 500;

				else if (k >= 60)
					chance = 2500;

				else if (k == 61 || k == 62)
					chance = 5000;

				else
					chance = 10000;

				fprintf(fp, "%s(%d)(1/%d), ", g_pItemList[MOB->Carry[k].sIndex].Name, MOB->Carry[k].sIndex, chance);

				if (++count % 5 == 0)
					fputs("\r\n", fp);
			}

			if (count % 5)
				fputs("\r\n", fp);

			fputs("\r\n", fp);

			MobList.push_back(MOB->MobName);
		}

		fclose(fp);
	}
}

void CNPCGenerator::LevelList()
{
	list<char*> MobList;

	for (int i = 0; i < NumList; i++)
	{
		int Leader = 1;
		int Follower = 1;

		for (list<char*>::iterator l = MobList.begin(); l != MobList.end(); ++l)
		{
			if (strncmp(*l, pList[i].Leader.MobName, strlen(pList[i].Leader.MobName)) == 0)
				Leader = 0;

			if (strncmp(*l, pList[i].Follower.MobName, strlen(pList[i].Leader.MobName)) == 0)
				Follower = 0;
		}

		if (Leader == 0 && Follower == 0)
			continue;

		char filename[256];

		int j = 0;

		for (j = 0; j < NumRegion; j++)
		{
			if (pList[i].SegmentListX[0] >= pRegion[j].minX && pList[i].SegmentListY[0] >= pRegion[j].minY &&
				pList[i].SegmentListX[0] <= pRegion[j].maxX && pList[i].SegmentListY[0] <= pRegion[j].maxY)
			{
				strcpy(filename, pRegion[j].RegionName);
				break;
			}
		}

		if (j == NumRegion)
			strcpy(filename, "Zona_Desconhecida");

		sprintf(temp, "./LevelList/%s.txt", filename);
		FILE *fp = fopen(temp, "a+");

		if (Leader)
		{
			STRUCT_MOB *MOB = &pList[i].Leader;

			fprintf(fp, "MobName: %s Level:%d\r\n\n", MOB->MobName, MOB->BaseScore.Level);

			fputs("\r\n", fp);

			MobList.push_back(MOB->MobName);
		}

		for (list<char*>::iterator l = MobList.begin(); l != MobList.end(); ++l)
		{
			if (strncmp(*l, pList[i].Follower.MobName, strlen(pList[i].Leader.MobName)) == 0)
				Follower = 0;
		}

		if (Follower)
		{
			STRUCT_MOB *MOB = &pList[i].Follower;

			fprintf(fp, "MobName: %s Level:%d\r\n\n", MOB->MobName, MOB->BaseScore.Level);

			fputs("\r\n", fp);

			MobList.push_back(MOB->MobName);
		}

		fclose(fp);
	}
}


CNPCSummon::CNPCSummon()
{
}

CNPCSummon::~CNPCSummon()
{
}

void CNPCSummon::Initialize()
{
	int MobFound = 0;

	BASE_InitModuleDir();

	strcpy((char*)Mob[0].MobName, "Condor");

	MobFound = ReadMob(&Mob[0], "BaseSummon");

	if (!MobFound)
		MessageBoxA(hWndMain, Mob[0].MobName, "Can't read NPC 0", MB_OK);

	strcpy((char*)Mob[1].MobName, "Javali");

	MobFound = ReadMob(&Mob[1], "BaseSummon");

	if (!MobFound)
		MessageBoxA(hWndMain, Mob[1].MobName, "Can't read NPC 1", MB_OK);

	strcpy((char*)Mob[2].MobName, "Lobo");

	MobFound = ReadMob(&Mob[2], "BaseSummon");

	if (!MobFound)
		MessageBoxA(hWndMain, Mob[2].MobName, "Can't read NPC 2", MB_OK);

	strcpy((char*)Mob[3].MobName, "Urso");

	MobFound = ReadMob(&Mob[3], "BaseSummon");

	if (!MobFound)
		MessageBoxA(hWndMain, Mob[3].MobName, "Can't read NPC 3", MB_OK);

	strcpy((char*)Mob[4].MobName, "Tigre");

	MobFound = ReadMob(&Mob[4], "BaseSummon");

	if (!MobFound)
		MessageBoxA(hWndMain, Mob[4].MobName, "Can't read NPC 4", MB_OK);

	strcpy((char*)Mob[5].MobName, "Gorila");

	MobFound = ReadMob(&Mob[5], "BaseSummon");

	if (!MobFound)
		MessageBoxA(hWndMain, Mob[5].MobName, "Can't read NPC 5", MB_OK);

	strcpy((char*)Mob[6].MobName, "Dragao_Negro");

	MobFound = ReadMob(&Mob[6], "BaseSummon");

	if (!MobFound)
		MessageBoxA(hWndMain, Mob[6].MobName, "Can't read NPC 6", MB_OK);

	strcpy((char*)Mob[7].MobName, "Succubus");

	MobFound = ReadMob(&Mob[7], "BaseSummon");

	if (!MobFound)
		MessageBoxA(hWndMain, Mob[7].MobName, "Can't read NPC 7", MB_OK);

	strcpy((char*)Mob[8].MobName, "Porco");

	MobFound = ReadMob(&Mob[8], "BaseSummon");

	if (!MobFound)
		MessageBoxA(hWndMain, Mob[8].MobName, "Can't read NPC 8", MB_OK);

	strcpy((char*)Mob[9].MobName, "Javali_");

	MobFound = ReadMob(&Mob[9], "BaseSummon");

	if (!MobFound)
		MessageBoxA(hWndMain, Mob[9].MobName, "Can't read NPC 9", MB_OK);

	strcpy((char*)Mob[10].MobName, "Porco");

	MobFound = ReadMob(&Mob[10], "BaseSummon");

	if (!MobFound)
		MessageBoxA(hWndMain, Mob[10].MobName, "Can't read NPC 10", MB_OK);

	strcpy((char*)Mob[11].MobName, "Javali");

	MobFound = ReadMob(&Mob[11], "BaseSummon");

	if (!MobFound)
		MessageBoxA(hWndMain, Mob[11].MobName, "Can't read NPC 11", MB_OK);

	strcpy((char*)Mob[12].MobName, "Lobo");

	MobFound = ReadMob(&Mob[12], "BaseSummon");

	if (!MobFound)
		MessageBoxA(hWndMain, Mob[12].MobName, "Can't read NPC 12", MB_OK);

	strcpy((char*)Mob[13].MobName, "Dragao_menor");

	MobFound = ReadMob(&Mob[13], "BaseSummon");

	if (!MobFound)
		MessageBoxA(hWndMain, Mob[13].MobName, "Can't read NPC 13", MB_OK);

	strcpy((char*)Mob[14].MobName, "Urso");

	MobFound = ReadMob(&Mob[14], "BaseSummon");

	if (!MobFound)
		MessageBoxA(hWndMain, Mob[14].MobName, "Can't read NPC 14", MB_OK);

	strcpy((char*)Mob[15].MobName, "Dente_de_Sabre");

	MobFound = ReadMob(&Mob[15], "BaseSummon");

	if (!MobFound)
		MessageBoxA(hWndMain, Mob[15].MobName, "Can't read NPC 15", MB_OK);

	strcpy((char*)Mob[16].MobName, "Sem_Sela_N");

	MobFound = ReadMob(&Mob[16], "BaseSummon");

	if (!MobFound)
		MessageBoxA(hWndMain, Mob[16].MobName, "Can't read NPC 16", MB_OK);

	strcpy((char*)Mob[17].MobName, "Fantasma_N");

	MobFound = ReadMob(&Mob[17], "BaseSummon");

	if (!MobFound)
		MessageBoxA(hWndMain, Mob[17].MobName, "Can't read NPC 17", MB_OK);

	strcpy((char*)Mob[18].MobName, "Leve_N");

	MobFound = ReadMob(&Mob[18], "BaseSummon");

	if (!MobFound)
		MessageBoxA(hWndMain, Mob[18].MobName, "Can't read NPC 18", MB_OK);

	strcpy((char*)Mob[19].MobName, "Equip_N");

	MobFound = ReadMob(&Mob[19], "BaseSummon");

	if (!MobFound)
		MessageBoxA(hWndMain, Mob[19].MobName, "Can't read NPC 19", MB_OK);

	strcpy((char*)Mob[20].MobName, "Andaluz_N");

	MobFound = ReadMob(&Mob[20], "BaseSummon");

	if (!MobFound)
		MessageBoxA(hWndMain, Mob[20].MobName, "Can't read NPC 20", MB_OK);

	strcpy((char*)Mob[21].MobName, "Sem_Sela_B");

	MobFound = ReadMob(&Mob[21], "BaseSummon");

	if (!MobFound)
		MessageBoxA(hWndMain, Mob[21].MobName, "Can't read NPC 21", MB_OK);

	strcpy((char*)Mob[22].MobName, "Fantasma_B");

	MobFound = ReadMob(&Mob[22], "BaseSummon");

	if (!MobFound)
		MessageBoxA(hWndMain, Mob[22].MobName, "Can't read NPC 22", MB_OK);

	strcpy((char*)Mob[23].MobName, "Leve_B");

	MobFound = ReadMob(&Mob[23], "BaseSummon");

	if (!MobFound)
		MessageBoxA(hWndMain, Mob[23].MobName, "Can't read NPC 23", MB_OK);

	strcpy((char*)Mob[24].MobName, "Equip_B");

	MobFound = ReadMob(&Mob[24], "BaseSummon");

	if (!MobFound)
		MessageBoxA(hWndMain, Mob[24].MobName, "Can't read NPC 24", MB_OK);

	strcpy((char*)Mob[25].MobName, "Andaluz_B");

	MobFound = ReadMob(&Mob[25], "BaseSummon");

	if (!MobFound)
		MessageBoxA(hWndMain, Mob[25].MobName, "Can't read NPC 25", MB_OK);

	strcpy((char*)Mob[26].MobName, "Fenrir");

	MobFound = ReadMob(&Mob[26], "BaseSummon");

	if (!MobFound)
		MessageBoxA(hWndMain, Mob[26].MobName, "Can't read NPC 26", MB_OK);

	strcpy((char*)Mob[27].MobName, "Dragao");

	MobFound = ReadMob(&Mob[27], "BaseSummon");

	if (!MobFound)
		MessageBoxA(hWndMain, Mob[27].MobName, "Can't read NPC 27", MB_OK);

	strcpy((char*)Mob[28].MobName, "FenrirSombra");

	MobFound = ReadMob(&Mob[28], "BaseSummon");

	if (!MobFound)
		MessageBoxA(hWndMain, Mob[28].MobName, "Can't read NPC 28", MB_OK);

	strcpy((char*)Mob[29].MobName, "Tigre_de_Fogo");

	MobFound = ReadMob(&Mob[29], "BaseSummon");

	if (!MobFound)
		MessageBoxA(hWndMain, Mob[29].MobName, "Can't read NPC 29", MB_OK);

	strcpy((char*)Mob[30].MobName, "Dragao_Vermelho");

	MobFound = ReadMob(&Mob[30], "BaseSummon");

	if (!MobFound)
		MessageBoxA(hWndMain, Mob[30].MobName, "Can't read NPC 30", MB_OK);

	strcpy((char*)Mob[31].MobName, "Unicornio");

	MobFound = ReadMob(&Mob[31], "BaseSummon");

	if (!MobFound)
		MessageBoxA(hWndMain, Mob[31].MobName, "Can't read NPC 31", MB_OK);

	strcpy((char*)Mob[32].MobName, "Pegasus");

	MobFound = ReadMob(&Mob[32], "BaseSummon");

	if (!MobFound)
		MessageBoxA(hWndMain, Mob[32].MobName, "Can't read NPC 32", MB_OK);

	strcpy((char*)Mob[33].MobName, "Unisus");

	MobFound = ReadMob(&Mob[33], "BaseSummon");

	if (!MobFound)
		MessageBoxA(hWndMain, Mob[33].MobName, "Can't read NPC 33", MB_OK);

	strcpy((char*)Mob[34].MobName, "Grifo");

	MobFound = ReadMob(&Mob[34], "BaseSummon");

	if (!MobFound)
		MessageBoxA(hWndMain, Mob[34].MobName, "Can't read NPC 34", MB_OK);

	strcpy((char*)Mob[35].MobName, "Hipogrifo");

	MobFound = ReadMob(&Mob[35], "BaseSummon");

	if (!MobFound)
		MessageBoxA(hWndMain, Mob[35].MobName, "Can't read NPC 35", MB_OK);

	strcpy((char*)Mob[36].MobName, "Grifo_Sangrento");

	MobFound = ReadMob(&Mob[36], "BaseSummon");

	if (!MobFound)
		MessageBoxA(hWndMain, Mob[36].MobName, "Can't read NPC 36", MB_OK);

	strcpy((char*)Mob[37].MobName, "Svadilfire");

	MobFound = ReadMob(&Mob[37], "BaseSummon");

	if (!MobFound)
		MessageBoxA(hWndMain, Mob[37].MobName, "Can't read NPC 37", MB_OK);

	strcpy((char*)Mob[38].MobName, "Sleipnir");

	MobFound = ReadMob(&Mob[38], "BaseSummon");

	if (!MobFound)
		MessageBoxA(hWndMain, Mob[38].MobName, "Can't read NPC 38", MB_OK);

	strcpy((char*)Mob[39].MobName, "Pantera_Negra");

	MobFound = ReadMob(&Mob[39], "BaseSummon");

	if (!MobFound)
		MessageBoxA(hWndMain, Mob[39].MobName, "Can't read NPC 39", MB_OK);
}

void SetAct(char *dest, char *act)
{
	if (strlen(act) >= 79)
	{
		MessageBox(hWndMain, act, "NPC Generater's Action can't be larger than 80.", NULL);

		return;
	}

	strcpy(dest, act);
}