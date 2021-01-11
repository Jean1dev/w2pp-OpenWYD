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


#include <windows.h>
#include <windowsx.h>
#include <stdlib.h>
#include <stdio.h> 
#include <fcntl.h>
#include <io.h>
#include <sys/types.h>
#include <sys/stat.h>
#include <sys/timeb.h>
#include <string.h>        
#include <time.h>
#include <mbstring.h>

#include "Basedef.h"
#include "ItemEffect.h"

#pragma region Defines

bool ReceivedItem;

// Items que pode ser ganhado aleatoriamente por 1 hora de online
int g_pRewardBonus[MAX_ITEM_REWARD_PER_HOUR] = { 3210, 3211, 3212 };

int BaseSIDCHM[4][6] = // Define the base Strength, Intelligence, Dexterity, Constance, HP and MP of the 4 classes
{
	{8, 4, 7, 6, 80, 45}, // TK
	{5, 8, 5, 5, 60, 65}, //FM
	{6, 6, 9, 5, 70, 55}, // BM
	{8, 9, 13, 6, 75, 60} // HT
};

STRUCT_RVRWAR g_pRvrWar;

STRUCT_GUILDZONE g_pGuildZone[MAX_GUILDZONE] = 
{
	{0, 0, 2088, 2148, 2086, 2093, 2052, 2052, 2171, 2163, 197, 213, 238, 230, 205, 220, 228, 220, 5, 0}, // Armia
	{0, 0, 2531, 1700, 2494, 1707, 2432, 1672, 2675, 1767, 197, 149, 238, 166, 205, 157, 228, 157, 5, 0}, // Azran
	{0, 0, 2460, 1976, 2453, 2000, 2448, 1966, 2476, 2024, 141, 213, 182, 230, 146, 220, 173, 220, 5, 0}, // Erion
	{0, 0, 3614, 3124, 3652, 3122, 3605, 3090, 3690, 3260, 141, 149, 182, 166, 146, 157, 173, 157, 5, 0}, // Nippleheim
	{0, 0, 1066, 1760, 1050, 1706, 1036, 1700, 1072, 1760, 4000, 4000, 4010, 4010, 4005, 4005, 4005, 4005, 5, 0} // Noatum
};

int g_pIncrementHp[4] = {3, 1, 1, 2}; // TK, FM, BM, HT. The ammount of HP that they get every level up
int g_pIncrementMp[4] = {1, 3, 2, 1}; // TK, FM, BM, HT. The ammount of MP that they get every level up

int CurrentClientGuild = -1;

int g_pSancRate[3][12] =
{
	100, 100, 100, 100, 100, 100, 100, 00, 00, 00, 00, 00, // PO
  //+0   +1   +2   +3   +4   +5   +6   +7  +8  +9
	100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 00, 00, // PL
  //+0   +1   +2   +3   +4   +5   +6   +7   +8   +9
	100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 80, 80  // Amagos
};

int g_pSancGrade[2][5] =
{
	1, 1, 1, 1, 1,
	1, 1, 1, 1, 1
};


int g_pEhreRate[10] =
{
	0, 100, 100, 40, 10, 10, 100, 100, 100, 100
};

int g_pDedekintoRate[3] =
{
	0, 80, 80
};


int g_pOdinRate[12] =
{
	100, 35, 100, 40, 100, 100, 100, 100, 100, 100, 100, 100 //0, 35, 100, 40, 100, 100, 100, 100, 100, 100, 100, 100
};  //N  ?    +   R    L    L
    //A  ?    1   U    4    9
    //D  ?    2   N    0    0
    //A  ?        A
    //            S
int g_pCelestialRate[15] =
{
	100, 100, 100, 100, 100, 100, 100, 100, 70, 50, 40, 40, 40, 10, 5
};

int g_pTinyBase = 15;
int g_pShanyBase = 35;
int g_pAilynBase = 100; //10
int g_pAgathaBase = 15;

int g_pAnctChance[3] = { 2, 4, 10 };

int g_pItemSancRate12[11] =
{
	1, 2, 3, 4, 5, 6, 7, 8, 9, 12, 15
};

int g_pItemSancRate12Minus[4] = { 2, 3, 4, 5 };

int g_pSuccessRate[11] = {5, 5, 5, 5, 4, 4, 3, 3, 2, 1, 0};

char g_pItemGrid[8][4][2] = 
{
#pragma region ItemGrid
	{
		{01, 00},
		{00, 00},
		{00, 00},
		{00, 00}
	},
	{  
		{01, 00},
		{01, 00},
		{00, 00},
		{00, 00}

	},
	{
		{01, 00},
		{01, 00},
		{01, 00},
		{00, 00}
	},
	{   
		{01, 00},
		{01, 00},
		{01, 00},
		{01, 00}
	},
	{   
		{01, 01},
		{00, 00},
		{00, 00},
		{00, 00}
	},
	{  
		{01, 01},
		{01, 01},
		{00, 00},
		{00, 00}
	},
	{  
		{01, 01},
		{01, 01},
		{01, 01},
		{00, 00}
	},
	{  
		{01, 01},
		{01, 01},
		{01, 01},
		{01, 01}
	}

#pragma endregion
};

int	g_pDistanceTable[7][7] =
{	
	0, 1, 2, 3, 4, 5, 6,
	1, 1, 2, 3, 4, 5, 6,
	2, 2, 3, 4, 4, 5, 6,
	3, 3, 4, 4, 5, 5, 6,
	4, 4, 4, 5, 5, 5, 6,
	5, 5, 5, 5, 5, 6, 6,
	6, 6, 6, 6, 6, 6, 6,
};

 int g_pGroundMask[MAX_GROUNDMASK][4][6][6];

 char g_pFormation[5][12][2] =
 {
	 1, 1, -1, 1, 1, -1, -1, -1,
	 1, 0, -1, 0, 0, 1, 0, -1, 2, 0, -2, 0,
	 0, 2, 0, -2, 1, 0, -1, 0, 2, 0, -2, 0,
	 3, 0, -3, 0, 4, 0, -4, 0, 5, 0, -5, 0,
	 0, 6, 0, -6, 1, 1, -1, 1, 1, -1, -1, -1,
	 1, 0, -1, 0, 0, 1, 0, -1, 2, 0, -2, 0,
	 0, 2, 0, -2, 1, 0, -1, 0, 2, 0, -2, 0,
	 3, 0, -3, 0, 4, 0, -4, 0, 5, 0, -5, 0,
	 0, 6, 0, -6, 2, 0, 0, 2, 1, 1, 0, 1,
	 1, 0, -1, 3, 3, -1, -1, 2, 2, -1, -1, 1,
	 1, -1, 1, 2
 };
 int g_pClanTable[9][9] =
 {
	{  1,  0,  1,  1,  1,  0,  1,  1,  1, },
	{  0,  1,  1,  0,  1,  1,  1,  0,  0, },
	{  1,  1,  1,  0,  1,  1,  1,  1,  1, },
	{  1,  0,  0,  1,  1,  0,  1,  1,  1, },
	{  1,  1,  1,  1,  1,  0,  1,  1,  1, },
	{  0,  1,  1,  0,  0,  1,  0,  0,  0, },
	{  1,  1,  1,  1,  1,  0,  1,  1,  1, },
	{  1,  0,  1,  1,  1,  0,  1,  1,  0, },
	{  1,  0,  1,  1,  1,  0,  1,  0,  1, }


 };

 int g_pDropBonus[64] = 
 {
	 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 
	 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 
	 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 
	 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100
 };

 int g_pDropRate[64] =
 {
	 900, 900, 900, 900, 900, 900, 900, 900, 4, 4, 4,  4, 900, 900, 900, 900, 20000, 20000, 20000, 20000,
	 20000, 20000, 20000, 20000, 2000, 2000, 2000, 2000, 2000, 2000, 2000, 2000, 2000, 2000, 2000, 2000, 
	 2000, 2000, 2000, 2000, 2000, 2000, 2000, 2000, 2000, 2000, 2000, 2000, 3000, 3000, 3000, 3000, 3000,
	 3000, 3000, 3000, 1, 35, 500, 2500, 5000, 5000, 10000, 20000
 };

int g_pMountBonus[30][5] = 
{
	// AttackAdd, MagicAdd, Evasion, Resistence, Speed

	{750, 110, 80, 32, 6},//{10, 1, 0, 0, 4},//Porco
	{750, 110, 80, 32, 6},//{10, 1, 0, 0, 4},//Javali
	{750, 110, 80, 32, 6},//{50, 1, 0, 0, 5},//Lobo
	{750, 110, 80, 32, 6},//{80, 15, 0, 0, 5},//Dragão menor
	{750, 110, 80, 32, 6},//{100, 20, 0, 0, 4},//Urso
	{750, 110, 80, 32, 6},//{150, 25, 0, 0, 5},//Dente de sabre
	{750, 110, 80, 32, 6},//{250, 50, 40, 0, 6},//Cavalo s/sela N
	{750, 110, 80, 32, 6},//{300, 60, 50, 0, 6},//Fantasma N
	{750, 110, 80, 32, 6},//{350, 65, 60, 0, 6},//Leve N
	{750, 110, 80, 32, 6},//{400, 70, 70, 0, 6},//Equip N
	{750, 110, 80, 32, 6},//{500, 85, 80, 0, 6},//Andaluz N
	{750, 110, 80, 32, 6},//{250, 50, 0, 16, 6},//Cavalo s/sela B
	{750, 110, 80, 32, 6},//{300, 60, 0, 20, 6},//Fantasma B
	{750, 110, 80, 32, 6},//{350, 65, 0, 24, 6},//Leve B
	{750, 110, 80, 32, 6},//{400, 70, 0, 28, 6},//Equip B
	{750, 110, 80, 32, 6},//{500, 85, 0, 32, 6},//Andaluz B
	{750, 110, 80, 32, 6},//{550, 90, 0, 0, 6},//Fenrir
	{750, 110, 80, 32, 6},//{600, 90, 0, 0, 6},//Dragao
	{750, 110, 80, 32, 6},//{550, 90, 0, 20, 6},//Fenrir das sombras
	{750, 110, 80, 32, 6},//{650, 100, 60, 28, 6},//Tigre de fogo
	{750, 110, 80, 32, 6},//{750, 110, 80, 32, 6},//Dragão vermelho
	{750, 110, 80, 32, 6},//{570, 90, 20, 16, 6},//Unicórnio
	{750, 110, 80, 32, 6},//{570, 90, 30, 8, 6},//Pegasus
	{750, 110, 80, 32, 6},//{570, 90, 40, 12, 6},//Unisus
	{750, 110, 80, 32, 6},//{590, 80, 30, 20, 6},//Grifo
	{750, 110, 80, 32, 6},//{615, 90, 40, 16, 6},//HipoGrifo
	{750, 110, 80, 32, 6},//{615, 90, 50, 16, 6},//Sangrento
	{750, 110, 80, 32, 6},//{620, 35, 30, 28, 6},//Svaldfire
	{750, 110, 80, 32, 6},//{250, 95, 0, 28, 6},//Sleipnir
	{750, 110, 80, 32, 6},//{150, 10, 20, 0, 6}//Pantera negra
};

int g_pMountTempBonus[20][5] =
{
	// AttackAdd, MagicAdd, Evasion, Resistence, Speed

	{750, 110, 80, 32, 6},//{ 35, 7, 0, 0, 6 },//Shire 3D
	{750, 110, 80, 32, 6},//{ 350, 55, 10, 28, 6 },//Thoroughbred 3D
	{750, 110, 80, 32, 6},//{ 450, 55, 0, 0, 6 },//Klazedale 3D
	{750, 110, 80, 32, 6},//{ 35, 7, 0, 0, 6 },//Shire 15D
	{750, 110, 80, 32, 6},//{ 450, 72, 10, 28, 6 },//Thoroughbred 15D
	{750, 110, 80, 32, 6},//{ 450, 72, 0, 0, 6 },//Klazedale 15D
	{750, 110, 80, 32, 6},//{ 120, 45, 0, 0, 6 },//Shire 30D
	{750, 110, 80, 32, 6},//{ 450, 72, 10, 28, 6 },//Thoroughbred 30D
	{750, 110, 80, 32, 6},//{ 450, 72, 0, 0, 6 },//Klazedale 30D
	{750, 110, 80, 32, 6},//{ 325, 35, 16, 28, 6 },//Gulfaxi 30D
	{750, 110, 80, 32, 6},//{ 350, 45, 10, 4, 6 },//Tigre de Fogo
	{750, 110, 80, 32, 6},//{ 250, 25, 0, 31, 6 },//Dragão Vermelho
	{750, 110, 80, 32, 6},//{ 80, 15, 0, 31, 6 },//Dragão Menor
	{750, 110, 80, 32, 6},//{ 950, 145, 60, 20, 6 },//Dragão Akelo
	{750, 110, 80, 32, 6},//{ 950, 145, 60, 20, 6 },//Dragão Hekalo
};
unsigned char g_pGuildSub[8] = {0xA5, 0xB0, 0xA5, 0xB1, 0xA5, 0xB2, 0x00, 0x00};

//char g_pGuildIndex[54] = {"????????????????"};
unsigned char g_pGuildIndex[36] = 
{
	0xA5, 0xB0, 0xA5, 0xB1, 0xA5, 0xB2, 0xA5, 0xB3, 0xA5, 0xB4, 0xA5, 0xB5, 0xA5, 0xB6, 0xA5, 0xB7, 
	0xA5, 0xB8, 0xA5, 0xB9, 0xA3, 0xC1, 0xA3, 0xC2, 0xA3, 0xC3, 0xA3, 0xC4, 0xA3, 0xC5, 0xA3, 0xC6, 
	0x00, 0x00, 0x00, 0x00
};
//char g_pGuildMark[168] = {"??????????????×?????¿???????†‡????????®?????????????????????????"};
unsigned char g_pGuildMark[132] = 
{
	0xA5, 0xB0, 0xA5, 0xB1, 0xA5, 0xB2, 0xA5, 0xB3, 0xA5, 0xB4, 0xA5, 0xB5, 0xA5, 0xB6, 0xA5, 0xB7, 
	0xA5, 0xB8, 0xA5, 0xB9, 0xA1, 0xE2, 0xA1, 0xE0, 0xA1, 0xDD, 0xA1, 0xD9, 0xA1, 0xBF, 0xA2, 0xB2, 
	0xA1, 0xDE, 0xA6, 0xB6, 0xA1, 0xCC, 0xA5, 0xC8, 0xA2, 0xAF, 0xA1, 0xDB, 0xA2, 0xB7, 0xA2, 0xBF, 
	0xA2, 0xC1, 0xA2, 0xC2, 0xA2, 0xC3, 0xA2, 0xC4, 0xA2, 0xD3, 0xA2, 0xD4, 0xA1, 0xC4, 0xA1, 0xD4, 
	0xA1, 0xF3, 0xA2, 0xB3, 0xA8, 0xCD, 0xA3, 0xC1, 0xA5, 0xC6, 0xA5, 0xCE, 0xA2, 0xE7, 0xA2, 0xB9, 
	0xA5, 0xE1, 0xA5, 0xE2, 0xA5, 0xE3, 0xA5, 0xE4, 0xA5, 0xE5, 0xA5, 0xE6, 0xA5, 0xE7, 0xA5, 0xE8, 
	0xA5, 0xE9, 0xA5, 0xEA, 0xA5, 0xEB, 0xA5, 0xEC, 0xA5, 0xED, 0xA5, 0xEE, 0xA5, 0xEF, 0xA5, 0xF0, 
	0xA5, 0xF1, 0xA5, 0xF2, 0xA5, 0xF3, 0xA5, 0xF4, 0xA5, 0xF5, 0xA5, 0xF6, 0xA5, 0xD7, 0xA5, 0xF8, 
	0x00, 0x00, 0x00, 0x00
};

char g_pAffectTable[32][24] = 
{
	"",
	"Slow",
	"Haste",
	"Drag Reduction",
	"Power-up",
	"Reduced avoidance",
	"Increase in avoidance",
	"Attack speed reduction",
	"SPEED",
	"Demijieop",
	"Reduction in attacks",
	"Magic Shield",
	"Reduction in defense",
	"Assault",
	"Gross",
	"Increase in skill",
	"Polymorph",
	"Tikrayipeu",
	"Magic Stamina",
	"Immunity",
	"Poison",
	"Melee attack",
	"Lightning",
	"Elemental",
	"Self-destruct",
	"Protectors",
	"Avoid falling",
	"Frost",
	"Hyde",
	"Drain",
	"Vision",
	"Coin Armor"
};

int g_pBonusValue5[30][4] = 
{
	{2, 30, 74, 18},
	{2, 30, 74, 15},
	{2, 30, 74, 12},

	{2, 24, 74, 18},
	{2, 24, 74, 15},
	{2, 24, 74, 12},

	{2, 18, 74, 18},
	{2, 18, 74, 15},
	{2, 18, 74, 12},

	{2, 12, 74, 18},
	{2, 12, 74, 15},
	{2, 12, 74, 12},

	{2, 6, 74, 18},
	{2, 6, 74, 15},
	{2, 6, 74, 12},

	{2, 30, 60, 10},
	{2, 30, 60, 8},
	{2, 30, 60, 6},

	{2, 24, 60, 10},
	{2, 24, 60, 8},
	{2, 24, 60, 6},

	{2, 18, 60, 10},
	{2, 18, 60, 8},
	{2, 18, 60, 6},

	{2, 12, 60, 10},
	{2, 12, 60, 8},
	{2, 12, 60, 6},

	{2, 6, 60, 10},
	{2, 6, 60, 8},
	{2, 6, 60, 6}
};

int g_pBonusValue4[30][4] = 
{
	{2, 30, 72, 30},
	{2, 30, 72, 25},
	{2, 30, 72, 20},
	{2, 30, 72, 15},
	{2, 30, 72, 10},

	{2, 24, 72, 30},
	{2, 24, 72, 25},
	{2, 24, 72, 20},
	{2, 24, 72, 15},
	{2, 24, 72, 10},

	{2, 18, 72, 30},
	{2, 18, 72, 25},
	{2, 18, 72, 20},
	{2, 18, 72, 15},
	{2, 18, 72, 10},

	{60, 10, 72, 30},
	{60, 10, 72, 25},
	{60, 10, 72, 20},
	{60, 10, 72, 15},

	{60, 8, 72, 30},
	{60, 8, 72, 25},
	{60, 8, 72, 20},
	{60, 8, 72, 15},

	{60, 6, 72, 30},
	{60, 6, 72, 25},
	{60, 6, 72, 20},
	{60, 6, 72, 15}
};

int g_pBonusValue3[25][4] = 
{
	{4, 60, 26, 18},
	{4, 60, 26, 15},
	{4, 60, 26, 12},

	{4, 50, 26, 18},
	{4, 50, 26, 15},
	{4, 50, 26, 12},

	{4, 40, 26, 18},
	{4, 40, 26, 15},
	{4, 40, 26, 12},

	{4, 30, 26, 18},
	{4, 30, 26, 15},
	{4, 30, 26, 12},

	{4, 60, 60, 12},
	{4, 60, 60, 10},
	{4, 60, 60, 8},
	{4, 60, 60, 6},

	{4, 50, 60, 12},
	{4, 50, 60, 10},
	{4, 50, 60, 8},
	{4, 50, 60, 6},

	{4, 40, 60, 12},
	{4, 40, 60, 10},
	{4, 40, 60, 8},
	{4, 40, 60, 6},
	{4, 40, 60, 4}
};

int g_pBonusValue2[48][4] = 
{
	{2, 30, 3, 30},
	{2, 30, 3, 25},
	{2, 30, 3, 20},
	{2, 30, 3, 10},

	{2, 24, 3, 30},
	{2, 24, 3, 25},
	{2, 24, 3, 20},
	{2, 24, 3, 15},

	{2, 18, 3, 30},
	{2, 18, 3, 25},
	{2, 18, 3, 20},
	{2, 18, 3, 15},

	{2, 30, 71, 50},
	{2, 30, 71, 60},
	{2, 30, 71, 70},

	{2, 24, 71, 50},
	{2, 24, 71, 60},
	{2, 24, 71, 70},

	{2, 18, 71, 50},
	{2, 18, 71, 60},
	{2, 18, 71, 70},

	{60, 10, 3, 30},
	{60, 10, 3, 25},
	{60, 10, 3, 20},
	{60, 10, 3, 15},
	{60, 10, 3, 10},

	{60, 8, 3, 30},
	{60, 8, 3, 25},
	{60, 8, 3, 20},
	{60, 8, 3, 15},
	{60, 8, 3, 10},

	{60, 6, 3, 30},
	{60, 6, 3, 25},
	{60, 6, 3, 20},
	{60, 6, 3, 15},
	{60, 6, 3, 10},

	{60, 10, 71, 50},
	{60, 10, 71, 60},
	{60, 10, 71, 70},

	{60, 8, 71, 50},
	{60, 8, 71, 60},
	{60, 8, 71, 70},

	{60, 6, 71, 50},
	{60, 6, 71, 60},
	{60, 6, 71, 70},

	{60, 4, 71, 50},
	{60, 4, 71, 60},
	{60, 4, 71, 70}
};


int g_pBonusValue[10][2][2] =
{
	{
		{10 , 30},
		{2 , 5}
	},

	{
		{3 , 10},
		{2 , 8}
	},

	{
		{2 , 8},
		{2 , 8}
	},

	{
		{2 , 6},
		{2 , 6}
	},

	{
		{2 , 6},
		{2 , 6}
	},

	{
		{20 , 45},
		{4 , 8}
	},

	{
		{7 , 20},
		{5 , 14}
	},

	{
		{5 , 14},
		{5 , 14}
	},

	{
		{4 , 12},
		{4 , 12}
	},

	{
		{4 , 12},
		{4 , 12}
	}
};

int g_pBonusType[10] = {10, 30, 2, 5, 3, 10, 2, 8, 2, 8 };

int DungeonPos[30][2] = 
{
#pragma region DungeonPos
	{244, 3822}, 
	{380, 3810},
	{352, 3780},
	{349, 3748},
	{456, 3736},
	{562, 3802},
	{592, 3770}, 
	{612, 3795},
	{748, 3732}, 
	{740, 3776}, 
	{424, 3892}, 
	{478, 3928},
	{432, 3944}, 
	{402, 3876}, 
	{492, 4076}, 
	{448, 4078},
	{996, 3988}, 
	{922, 4062}, 
	{802, 4000}, 
	{774, 4048},
	{710, 3988}, 
	{660, 4076}, 
	{788, 3860}, 
	{876, 3892},
	{964, 3821}, 
	{956, 3732}, 
	{1032, 3820}, 
	{1104, 3737},
	{1124, 3804}, 
	{1132, 3756}
#pragma endregion
};

int DungeonItem[10] = {	421, 422, 423, 419, 420, 685, 691, 696, 696, 696 };

long long g_pNextLevel[MAX_LEVEL+202] = 
{
		0, 500 ,1124 ,1826 ,2610 ,3480 ,4440 ,5494 ,6646 ,7900 ,
		9260 ,10893 ,12817 ,15050 ,17610 ,20515 ,23783 ,27432 ,31480 ,
		35945 ,40845 ,46251 ,52187 ,58677 ,65745 ,73415 ,81711 ,90657 ,
		100277 ,110595 ,121635 ,133647 ,146671 ,160747 ,175915 ,192215 ,209687 ,
		228371 ,248307 ,269535 ,292095 ,316151 ,341751 ,368943 ,397775 ,428295 ,
		460551 ,494591 ,530463 ,568215 ,607895 ,649715 ,693731 ,739999 ,788575 ,
		839515 ,892875 ,948711 ,1007079 ,1068035 ,1131635 ,1198670 ,1269230 ,1343405 ,
		1421285 ,1502960 ,1588520 ,1678055 ,1771655 ,1869410 ,1971410 ,2078255 ,2190055 ,
		2306920 ,2428960 ,2556285 ,2689005 ,2827230 ,2971070 ,3120635 ,3276035 ,3438521 ,
		3608249 ,3785375 ,3970055 ,4162445 ,4362701 ,4570979 ,4787435 ,5012225 ,5245505 ,
		5488163 ,5740379 ,6002333 ,6274205 ,6556175 ,6848423 ,7151129 ,7464473 ,7788635 ,
		8123795 ,8460174 ,8797774 ,9136597 ,9476645 ,9817920 ,10160424 ,10504159 ,10849127 ,
		11195330 ,11542770 ,11892311 ,12243959 ,12597720 ,12953600 ,13311605 ,13671741 ,14034014 ,
		14398430 ,14764995 ,15133715 ,15508850 ,15890450 ,16278565 ,16673245 ,17074540 ,17482500 ,
		17897175 ,18318615 ,18746870 ,19181990 ,19625811 ,20078403 ,20539836 ,21010180 ,21489505 ,
		21977881 ,22475378 ,22982066 ,23498015 ,24023295 ,24559110 ,25105558 ,25662737 ,26230745 ,
		26809680 ,27399640 ,28000723 ,28613027 ,29236650 ,29871690 ,30517485 ,31174125 ,31841700 ,
		32520300 ,33210015 ,33910935 ,34623150 ,35346750 ,36081825 ,36828465 ,37587867 ,38360139 ,
		39145389 ,39943725 ,40755255 ,41580087 ,42418329 ,43270089 ,44135475 ,45014595 ,45904870 ,
		46806370 ,47719165 ,48643325 ,49578920 ,50526020 ,51484695 ,52455015 ,53437050 ,54430870 ,
		55439542 ,56463162 ,57501826 ,58555630 ,59624670 ,60709042 ,61808842 ,62924166 ,64055110 ,
		65201770 ,66366010 ,67547930 ,68747630 ,69965210 ,71200770 ,72454410 ,73726230 ,75016330 ,
		76324810 ,77651770 ,78985354 ,80325578 ,81672458 ,83026010 ,84386250 ,85753194 ,87126858 ,
		88507258 ,89894410 ,91288330 ,92693002 ,94108458 ,95534730 ,96971850 ,98419850 ,99878762 ,
		101348618 ,102829450 ,104321290 ,105824170 ,107352234 ,108905674 ,110484682 ,112089450 ,113720170 ,
		115377034 ,117060234 ,118769962 ,120506410 ,122269770 ,124065890 ,125895058 ,127757562 ,129653690 ,
		131583730 ,133547970 ,135546698 ,137580202 ,139648770 ,141752690 ,143928178 ,146176386 ,148498466 ,
		150895570 ,153368850 ,155919458 ,158548546 ,161257266 ,164046770 ,166918210 ,169956978 ,173167682 ,
		176554930 ,180123330 ,205345890 ,209100050 ,212902550 ,216753470 ,220652890 ,224600890 ,228597550 ,
		232642950 ,236737170 ,240880290 ,245072390 ,249313550 ,253603850 ,257943370 ,262332190 ,266770390 ,
		271258050 ,275795250 ,280382070 ,285018590 ,289904810 ,295042730 ,300434350 ,306081670 ,311986690 ,
		318151410 ,324577830 ,331267950 ,338223770 ,345447290 ,354039310 ,364049830 ,375528850 ,388526370 ,
		403092390 ,419276910 ,437129930 ,456701450 ,476272970 ,495844490 ,515416010 ,534987530 ,554559050 ,
		574130570 ,593702090 ,613273610 ,632845130 ,652416650 ,671988170 ,691559690 ,711131210 ,730702730 ,
		750274250 ,769845770 ,789417290 ,808988810 ,828560330 ,848131850 ,867703370 ,887274890 ,906846410 ,
		926417930 ,945989450 ,965560970 ,985132490 ,1004704010 ,1024275530 ,1043847050 ,1063418570 ,1082990090 ,
		1102561610 ,1122133130 ,1141704650 ,1161276170 ,1180847690 ,1200419210 ,1222705731 ,1244995262 ,1267288477 ,
		1289622601 ,1311966887 ,1334333102 ,1356724650 ,1379151914 ,1401651370 ,1424151231 ,1448674779 ,1473220997 ,
		1497782544 ,1522364697 ,1546957043 ,1571581919 ,1596243411 ,1620925875 ,1645647464 ,1670373305 ,1710373305 ,
		1770373305 ,1870373305 ,2000000000 ,2039000000 ,2078000000 ,2117000000 ,2156000000 ,2195000000 ,2234000000 ,
		2273000000 ,2312000000 ,2351000000 ,2390000000 ,2429000000 ,2468000000 ,2507000000 ,2546000000 ,2585000000 ,
		2624000000 ,2663000000 ,2702000000 ,2741000000 ,2780000000 ,2819000000 ,2858000000 ,2897000000 ,2936000000 ,
		3000000000 ,3043000000 ,3086000000 ,3129000000 ,3172000000 ,3215000000 ,3258000000 ,3301000000 ,3344000000 ,
		3387000000 ,3430000000 ,3473000000 ,3516000000 ,3559000000 ,3602000000 ,3645000000 ,3688000000 ,3731000000 ,
		3774000000 ,3817000000 ,4000000000 ,4100000000 ,4120000000, 4140000000, 4160000000,	4180000000, 4200000000, 
		4220000000, 4240000000, 4260000000, 4280000000, 4300000000, 4320000000, 4340000000,	4360000000, 4380000000,
		4400000000, 4420000000, 4440000000, 4460000000, 4480000000, 4500000000, 4520000000,	4540000000, 4560000000,
		4580000000, 4600000000, 4620000000, 4640000000, 4660000000, 4680000000, 4700000000,	4720000000, 4740000000,
		4760000000, 4780000000, 4800000000, 4820000000, 4840000000, 4860000000, 4880000000,	4900000000, 4920000000,
		4940000000, 4960000000, 4980000000, 5000000000, 5020000000, 5040000000, 5060000000,	5080000000, 5100000000,
		5120000000, 5140000000, 5160000000, 5180000000, 5200000000, 5220000000, 5240000000,	5260000000, 5280000000,
		5300000000, 5320000000, 5340000000, 5360000000, 5380000000, 5400000000, 5420000000,	5440000000, 5460000000,
		5480000000, 5500000000, 5520000000, 5540000000, 5560000000, 5580000000, 5600000000,	5620000000, 5640000000,
		5660000000, 5680000000, 5700000000, 5720000000, 5740000000, 5760000000, 5780000000,	5800000000, 5820000000,
		5840000000, 5860000000, 5880000000, 5900000000, 5920000000, 5940000000, 5960000000,	5980000000, 6000000000,
		6020000000, 6040000000, 6060000000, 6080000000, 6100000000, 6120000000, 6140000000,	6160000000, 6180000000,
		6200000000, 6220000000, 6240000000, 6260000000, 6280000000, 6300000000, 6320000000,	6340000000, 6360000000,
		6380000000, 6400000000, 6420000000, 6440000000, 6460000000, 6480000000, 6500000000,	6520000000, 6540000000,
		6560000000, 6580000000, 6600000000, 6620000000, 6640000000, 6660000000, 6680000000,	6700000000, 6720000000,
		6740000000, 6760000000, 6780000000, 6800000000, 6820000000, 6840000000, 6860000000,	6880000000, 6900000000,
		6920000000, 6940000000, 6960000000, 6980000000, 7000000000, 7020000000, 7040000000,	7060000000, 7080000000,
		7100000000, 7120000000, 7140000000, 7160000000, 7180000000, 7200000000, 7220000000,	7240000000, 7260000000,
		7280000000, 7300000000, 7320000000, 7340000000, 7360000000, 7380000000, 7400000000,	7420000000, 7440000000,
		7460000000, 7480000000, 7500000000, 7520000000, 7540000000, 7560000000, 7580000000,	7600000000, 7620000000,
		7640000000, 7660000000, 7680000000, 7700000000, 7720000000, 7740000000, 7760000000,	7780000000, 7800000000,
		7820000000, 7840000000, 7860000000, 7880000000, 7900000000, 7920000000, 7940000000,	7960000000, 7980000000,
		8000000000, 8200000000
};

long long g_pNextLevel_2[MAX_CLEVEL + 202] = {
	0, 20000000, 40000000, 60000000, 80000000, 100000000, 120000000, 140000000, 160000000, 180000000, 
	200000000, 220000000, 240000000, 260000000, 280000000, 300000000, 320000000, 340000000, 360000000, 
	380000000, 400000000, 420000000, 440000000, 460000000, 480000000, 500000000, 520000000, 540000000, 
	560000000, 580000000, 600000000, 620000000, 640000000, 660000000, 680000000, 700000000, 720000000, 
	740000000, 760000000, 780000000, 800000000, 820000000, 840000000, 860000000, 880000000, 900000000, 
	920000000, 940000000, 960000000, 980000000, 1000000000, 1020000000, 1040000000, 1060000000, 1080000000, 
	1100000000, 1120000000, 1140000000, 1160000000, 1180000000, 1200000000, 1220000000, 1240000000, 1260000000, 
	1280000000, 1300000000, 1320000000, 1340000000, 1360000000, 1380000000, 1400000000, 1420000000, 1440000000, 
	1460000000, 1480000000, 1500000000, 1520000000, 1540000000, 1560000000, 1580000000, 1600000000, 1620000000, 
	1640000000, 1660000000, 1680000000, 1700000000, 1720000000, 1740000000, 1760000000, 1780000000, 1800000000, 
	1820000000, 1840000000, 1860000000, 1880000000, 1900000000, 1920000000, 1940000000, 1960000000, 1980000000, 
	2000000000, 2020000000, 2040000000, 2060000000, 2080000000, 2100000000, 2120000000, 2140000000, 2160000000, 
	2180000000, 2200000000, 2220000000, 2240000000, 2260000000, 2280000000, 2300000000, 2320000000, 2340000000, 
	2360000000, 2380000000, 2400000000, 2420000000, 2440000000, 2460000000, 2480000000, 2500000000, 2520000000, 
	2540000000, 2560000000, 2580000000, 2600000000, 2620000000, 2640000000, 2660000000, 2680000000, 2700000000, 
	2720000000, 2740000000, 2760000000, 2780000000, 2800000000, 2820000000, 2840000000, 2860000000, 2880000000, 
	2900000000, 2920000000, 2940000000, 2960000000, 2980000000, 3000000000, 3020000000, 3040000000, 3060000000, 
	3080000000, 3100000000, 3120000000, 3140000000, 3160000000, 3180000000, 3200000000, 3220000000, 3240000000, 
	3260000000, 3280000000, 3300000000, 3320000000, 3340000000, 3360000000, 3380000000, 3400000000, 3420000000, 
	3440000000, 3460000000, 3480000000, 3500000000, 3520000000, 3540000000, 3560000000, 3580000000, 3600000000, 
	3620000000, 3640000000, 3660000000, 3680000000, 3700000000, 3720000000, 3740000000, 3760000000, 3780000000, 
	3800000000, 3820000000, 3840000000, 3860000000, 3880000000, 3900000000, 3920000000, 3940000000, 3960000000, 
	4000000000, 4020000000, 4040000000, 4060000000, 4080000000, 4100000000, 4120000000, 4140000000, 4160000000, 
	4180000000, 4200000000, 4220000000, 4240000000, 4260000000, 4280000000, 4300000000, 4320000000, 4340000000, 
	4360000000, 4380000000, 4400000000, 4420000000, 4440000000, 4460000000, 4480000000, 4500000000, 4520000000, 
	4540000000, 4560000000, 4580000000, 4600000000, 4620000000, 4640000000, 4660000000, 4680000000, 4700000000, 
	4720000000, 4740000000, 4760000000, 4780000000, 4800000000, 4820000000, 4840000000, 4860000000, 4880000000, 
	4900000000, 4920000000, 4940000000, 4960000000, 4980000000, 5000000000, 5020000000, 5040000000, 5060000000, 
	5080000000, 5100000000, 5120000000, 5140000000, 5160000000, 5180000000, 5200000000, 5220000000, 5240000000, 
	5260000000, 5280000000, 5300000000, 5320000000, 5340000000, 5360000000, 5380000000, 5400000000, 5420000000, 
	5440000000, 5460000000, 5480000000, 5500000000, 5520000000, 5540000000, 5560000000, 5580000000, 5600000000, 
	5620000000, 5640000000, 5660000000, 5680000000, 5700000000, 5720000000, 5740000000, 5760000000, 5780000000, 
	5800000000, 5820000000, 5840000000, 5860000000, 5880000000, 5900000000, 5920000000, 5940000000, 5960000000, 
	5980000000, 6000000000, 6020000000, 6040000000, 6060000000, 6080000000, 6100000000, 6120000000, 6140000000, 
	6160000000, 6180000000, 6200000000, 6220000000, 6240000000, 6260000000, 6280000000, 6300000000, 6320000000, 
	6340000000, 6360000000, 6380000000, 6400000000, 6420000000, 6440000000, 6460000000, 6480000000, 6500000000, 
	6520000000, 6540000000, 6560000000, 6580000000, 6600000000, 6620000000, 6640000000, 6660000000, 6680000000, 
	6700000000, 6720000000, 6740000000, 6760000000, 6780000000, 6800000000, 6820000000, 6840000000, 6860000000, 
	6880000000, 6900000000, 6920000000, 6940000000, 6960000000, 6980000000, 7000000000, 7020000000, 7040000000, 
	7060000000, 7080000000, 7100000000, 7120000000, 7140000000, 7160000000, 7180000000, 7200000000, 7220000000, 
	7240000000, 7260000000, 7280000000, 7300000000, 7320000000, 7340000000, 7360000000, 7380000000, 7400000000, 
	7420000000, 7440000000, 7460000000, 7480000000, 7500000000, 7520000000, 7540000000, 7560000000, 7580000000, 
	7600000000, 7620000000, 7640000000, 7660000000, 7680000000, 7700000000, 7720000000, 7740000000, 7760000000, 
	7780000000, 7800000000, 7820000000, 7840000000, 7860000000, 7880000000, 7900000000, 7920000000, 7940000000, 
	7960000000, 7980000000, 8000000000, 8200000000
};


STRUCT_BEASTBONUS pSummonBonus[MAX_SUMMONLIST] = 
{
	{ 80, 300, 50,  75, 100, 400, 0, 0, 0, 0, 0, 7, 0, 0, 100, 100,   5},
	{ 80, 250, 50, 150, 125, 400, 0, 0, 0, 0, 0, 7, 0, 0, 100, 100,  20},
	{ 80, 400, 50, 125, 125, 400, 0, 0, 0, 0, 0, 6, 0, 0, 100, 100,  50},
	{ 80, 350, 50, 200, 150, 400, 0, 0, 0, 0, 0, 6, 0, 0, 100, 100,  70},
	{ 80, 500, 50, 175, 150, 400, 0, 0, 0, 0, 0, 6, 0, 0, 100, 100,  90},
	{ 80, 450, 50, 250, 175, 400, 0, 0, 0, 0, 0, 6, 0, 0, 100, 100, 110},
	{100, 500, 50, 250, 174, 400, 0, 0, 0, 0, 0, 6, 0, 0, 100, 100, 140},
	{130, 250, 60, 200, 180, 250, 0, 0, 0, 0, 0, 6, 0, 0, 100, 100, 160},
	{0,}
};


STRUCT_BEASTBONUS pTransBonus[5] = 
{
	//MinDam, MaxDam, MinAc, MaxAc, MinHp, MaxHp, RunSpeed, UNK, AttackSpeed
	{110, 130,  95, 105,  95, 105, 1, 0, 20, 0, 100, 100, 100, 100, 100, 100,  15},
	{ 80, 100, 100, 110, 110, 140, 0, 0,  0, 0, 100, 100, 100, 100, 100, 100,  60},
	{100, 120, 105, 115, 100, 120, 1, 0, 20, 0, 100, 100, 100, 100, 100, 100, 115},
	{ 90, 110, 110, 125, 105, 110, 0, 0, 20, 0, 100, 100, 100, 100, 100, 100, 155 },
	{ 105, 120, 110, 120, 105, 115, 3, 0, 20, 0, 100, 100, 100, 100, 100, 100, 155 }
};

//char KorFirst[36] = {0,};//"¤¡¤¢¤¤¤§¤¨¤©¤±¤²¤³¤µ¤¶¤·¤¸¤¹¤º¤»¤¼¤½¤¾";
unsigned char KorFirst[36] = 
{
	0xA4, 0xA1, 0xA4, 0xA2, 0xA4, 0xA4, 0xA4, 0xA7, 0xA4, 0xA8, 0xA4, 0xA9, 0xA4, 0xB1, 0xA4, 0xB2, 
	0xA4, 0xB3, 0xA4, 0xB5, 0xA4, 0xB6, 0xA4, 0xB7, 0xA4, 0xB8, 0xA4, 0xB9, 0xA4, 0xBA, 0xA4, 0xBB, 
	0xA4, 0xBC, 0xA4, 0xBD
};
int KorIndex[19] = {171, 291, 432, 560, 646, 773, 902, 1031, 1104, 1237, 1353, 1561, 1696, 1770, 1890, 1997, 2103, 2208, 2350};

int  ChargedGuildList[MAX_SERVER][MAX_GUILDZONE] = {0,};
char g_pGuildName[10][16][MAX_GUILD][GUILDNAME_LENGTH];
char g_pClientGuildName[16][MAX_GUILD][4][GUILDNAME_LENGTH];
char EffectNameTable[MAX_EFFECTINDEX][24];
char g_pMessageStringTable[MAX_STRING][128];

STRUCT_SPELL g_pSpell[MAX_SKILLINDEX];
STRUCT_INITITEM g_pInitItem[MAX_INITITEM];

char g_pEnglish[MAX_ENGLISH][3][NAME_LENGTH];

int g_pHitRate[1024] = {0,};

unsigned char g_pAttribute[1024][1024];

STRUCT_ITEMLIST g_pItemList[MAX_ITEMLIST];

char g_pServerList[MAX_SERVERGROUP][MAX_SERVERNUMBER][64];

int g_dwInitItem = 0;

int g_HeightWidth = 4096;
int g_HeightHeight = 4096;
int g_HeightPosX = 0;
int g_HeightPosY = 0;

#pragma endregion

int BASE_NeedLog(struct STRUCT_ITEM *item, int money)
{
	int idx = item->sIndex;

	if(idx < 0 || idx >= MAX_ITEMLIST)
		return FALSE;

	if(money >= 0)
		return TRUE;

	int sanc = BASE_GetItemSanc(item);

	if(sanc >= 3)
		return TRUE;

	int npos = g_pItemList[idx].nPos;

	int magic = BASE_GetBonusItemAbility(item, EF_MAGIC);
	int dam = BASE_GetBonusItemAbility(item, EF_DAMAGE);

	int guild = BASE_GetGuild(item);

	if(npos == 64 || npos == 192)
	{
		 if(magic >= 12 || dam >= 27)
			return TRUE;
	}
	else  if(magic >= 8 || dam >= 12)
		return TRUE;

	if(guild > 0 || idx == 419 || idx == 420 || idx == 412 || idx == 413 || idx == 753 || idx == 657 || (idx >= 470 && idx < 500) || idx == 522)
		return TRUE;

	if((idx >= 551 || idx <= 562) || item->stEffect[0].cEffect != 0 || item->stEffect[0].cEffect != 59|| item->stEffect[1].cEffect != 0 || item->stEffect[1].cEffect != 59)
		return TRUE;

	int price = g_pItemList[idx].Price;

	if(price >= 100)
		return TRUE;

	return FALSE;
}

int BASE_GetBonusSkillPoint(STRUCT_MOB *mob, STRUCT_MOBEXTRA *extra)
{
	int spellperlevel = mob->CurrentScore.Level * 3;

	if (extra->ClassMaster == ARCH)
		spellperlevel += 112;

	if (extra->ClassMaster != ARCH && extra->ClassMaster != MORTAL)
		spellperlevel += 1500;

	int mod = mob->CurrentScore.Level - 199;

	if(mod > 0)
		spellperlevel += mod;

	int quest = extra->QuestInfo.Mortal.PilulaOrc;

	if(quest != 0)
		spellperlevel += 9;

	int cls = mob->Class;

	if(cls < 0 || cls >= MAX_CLASS)
		return FALSE;

	int spelluse = 0;
	int skillbit = 1;

	for(int j = 0; j < MAX_SKILL; j++)
	{
		if((mob->LearnedSkill & skillbit) != 0)
			spelluse += g_pSpell[((cls * MAX_SKILL) + j)].SkillPoint;

		skillbit *= 2;
	}

	int rest = spellperlevel - spelluse;

	if(rest != 0)
	{
		mob->SkillBonus = rest;

		return FALSE;
	}
	
	return TRUE;
}

int BASE_GetBonusScorePoint(STRUCT_MOB *mob, STRUCT_MOBEXTRA *extra)
{
	int cls = mob->Class;

	if(cls < 0 || cls >= MAX_CLASS)
		return FALSE;

	int usestr = mob->BaseScore.Str - BaseSIDCHM[cls][0];
	int useint = mob->BaseScore.Int - BaseSIDCHM[cls][1];
	int usedex = mob->BaseScore.Dex - BaseSIDCHM[cls][2];
	int usecon = mob->BaseScore.Con - BaseSIDCHM[cls][3];

	if (extra->ClassMaster == MORTAL)
	{
		int totaluse = usestr + useint + usedex + usecon;

		int lvl = mob->BaseScore.Level;

		int leveluse = lvl * 5;

		if (lvl >= 254)
			leveluse += (lvl - 254) * 5;

		if (lvl >= 299)
			leveluse += (lvl - 299) * 10;

		if (lvl >= 354)
			leveluse += (lvl - 354) * -8;

		if(totaluse > leveluse)
		{
			int dif = totaluse - leveluse;

			if(mob->BaseScore.Str >= dif)
				mob->BaseScore.Str -= dif;

			else if(mob->BaseScore.Int >= dif)
				mob->BaseScore.Int -= dif;

			else if(mob->BaseScore.Dex >= dif)
				mob->BaseScore.Dex -= dif;

			else if(mob->BaseScore.Con >= dif)
				mob->BaseScore.Con -= dif;

			totaluse = mob->BaseScore.Str + mob->BaseScore.Int + mob->BaseScore.Dex + mob->BaseScore.Con - BaseSIDCHM[cls][0] - BaseSIDCHM[cls][1] - BaseSIDCHM[cls][2] - BaseSIDCHM[cls][3];
		}
		mob->ScoreBonus = leveluse - totaluse;
	}
	else if (extra->ClassMaster == ARCH)
	{
		int totaluse = usestr + useint + usedex + usecon;

		int lvl = mob->BaseScore.Level;

		int leveluse = lvl * 6;

		leveluse += extra->QuestInfo.Arch.MortalLevel * 6;

		if (lvl >= 354)
			leveluse += (lvl - 354) * 6;

		if(totaluse > leveluse)
		{
			int dif = totaluse - leveluse;

			if(mob->BaseScore.Str >= dif)
				mob->BaseScore.Str -= dif;

			else if(mob->BaseScore.Int >= dif)
				mob->BaseScore.Int -= dif;

			else if(mob->BaseScore.Dex >= dif)
				mob->BaseScore.Dex -= dif;

			else if(mob->BaseScore.Con >= dif)
				mob->BaseScore.Con -= dif;

			totaluse = mob->BaseScore.Str + mob->BaseScore.Int + mob->BaseScore.Dex + mob->BaseScore.Con - BaseSIDCHM[cls][0] - BaseSIDCHM[cls][1] - BaseSIDCHM[cls][2] - BaseSIDCHM[cls][3];
		}
		mob->ScoreBonus = leveluse - totaluse;
	}
	else if (extra->ClassMaster == CELESTIAL)
	{
		int totaluse = usestr + useint + usedex + usecon;

		int lvl = mob->BaseScore.Level;

		int leveluse = lvl * 6;

		leveluse += (extra->QuestInfo.Arch.Cristal * 100);
		leveluse += (extra->QuestInfo.Celestial.Reset * 200);

		leveluse += 1001;

		if (extra->QuestInfo.Celestial.ArchLevel == 1)
			leveluse += 100;

		else if (extra->QuestInfo.Celestial.ArchLevel == 2)
			leveluse += 300;

		else if (extra->QuestInfo.Celestial.ArchLevel == 3)
			leveluse += 600;

		else if (extra->QuestInfo.Celestial.ArchLevel == 4)
			leveluse += 900;

		else if (extra->QuestInfo.Celestial.ArchLevel == 5)
			leveluse += 1200;


		if (lvl >= 120)
			leveluse += (lvl - 119) * 6;

		if (lvl >= 150)
			leveluse += (lvl - 149) * 2;

		if (lvl >= 170)
			leveluse += (lvl - 169) * 2;

		if (lvl >= 180)
			leveluse += (lvl - 179) * 2;

		if (lvl >= 190)
			leveluse += (lvl - 189) * 2;

		if(totaluse > leveluse)
		{
			int dif = totaluse - leveluse;

			if(mob->BaseScore.Str >= dif)
				mob->BaseScore.Str -= dif;

			else if(mob->BaseScore.Int >= dif)
				mob->BaseScore.Int -= dif;

			else if(mob->BaseScore.Dex >= dif)
				mob->BaseScore.Dex -= dif;

			else if(mob->BaseScore.Con >= dif)
				mob->BaseScore.Con -= dif;

			totaluse = mob->BaseScore.Str + mob->BaseScore.Int + mob->BaseScore.Dex + mob->BaseScore.Con - BaseSIDCHM[cls][0] - BaseSIDCHM[cls][1] - BaseSIDCHM[cls][2] - BaseSIDCHM[cls][3];
		}
		mob->ScoreBonus = leveluse - totaluse;
	}
	else if (extra->ClassMaster == CELESTIALCS)
	{
		int totaluse = usestr + useint + usedex + usecon;

		int lvl = mob->BaseScore.Level;

		int leveluse = lvl * 6;

		leveluse += (extra->QuestInfo.Arch.Cristal * 100);
		leveluse += (extra->QuestInfo.Celestial.Reset * 200);

		leveluse += 1001;

		if (extra->QuestInfo.Celestial.ArchLevel == 1)
			leveluse += 100;

		else if (extra->QuestInfo.Celestial.ArchLevel == 2)
			leveluse += 300;

		else if (extra->QuestInfo.Celestial.ArchLevel == 3)
			leveluse += 600;

		else if (extra->QuestInfo.Celestial.ArchLevel == 4)
			leveluse += 900;

		else if (extra->QuestInfo.Celestial.ArchLevel == 5)
			leveluse += 1200;


		if (lvl >= 120)
			leveluse += (lvl - 119) * 6;

		if (lvl >= 150)
			leveluse += (lvl - 149) * 2;

		if (lvl >= 170)
			leveluse += (lvl - 169) * 2;

		if (lvl >= 180)
			leveluse += (lvl - 179) * 2;

		if (lvl >= 190)
			leveluse += (lvl - 189) * 2;

		lvl = extra->QuestInfo.Celestial.SubCelestialLevel;

		leveluse += (lvl * 6)/2;

		if (lvl >= 120)
			leveluse += ((lvl - 119) * 6);

		if (lvl >= 150)
			leveluse += ((lvl - 149) * 2);

		if (lvl >= 170)
			leveluse += ((lvl - 169) * 2);

		if (lvl >= 180)
			leveluse += ((lvl - 179) * 2);

		if (lvl >= 190)
			leveluse += ((lvl - 189) * 2);

		if(totaluse > leveluse)
		{
			int dif = totaluse - leveluse;

			if(mob->BaseScore.Str >= dif)
				mob->BaseScore.Str -= dif;

			else if(mob->BaseScore.Int >= dif)
				mob->BaseScore.Int -= dif;

			else if(mob->BaseScore.Dex >= dif)
				mob->BaseScore.Dex -= dif;

			else if(mob->BaseScore.Con >= dif)
				mob->BaseScore.Con -= dif;

			totaluse = mob->BaseScore.Str + mob->BaseScore.Int + mob->BaseScore.Dex + mob->BaseScore.Con - BaseSIDCHM[cls][0] - BaseSIDCHM[cls][1] - BaseSIDCHM[cls][2] - BaseSIDCHM[cls][3];
		}
		mob->ScoreBonus = leveluse - totaluse;
	}

	else if (extra->ClassMaster == SCELESTIAL)
	{
		int totaluse = usestr + useint + usedex + usecon;

		int lvl = mob->BaseScore.Level;

		int leveluse = lvl * 6;

		leveluse += (extra->QuestInfo.Arch.Cristal * 100);
		leveluse += (extra->QuestInfo.Celestial.Reset * 200);

		leveluse += 1001;

		if (extra->QuestInfo.Celestial.ArchLevel == 1)
			leveluse += 100;

		else if (extra->QuestInfo.Celestial.ArchLevel == 2)
			leveluse += 300;

		else if (extra->QuestInfo.Celestial.ArchLevel == 3)
			leveluse += 600;

		else if (extra->QuestInfo.Celestial.ArchLevel == 4)
			leveluse += 900;

		else if (extra->QuestInfo.Celestial.ArchLevel == 5)
			leveluse += 1200;


		if (lvl >= 120)
			leveluse += (lvl - 119) * 6;

		if (lvl >= 150)
			leveluse += (lvl - 149) * 2;

		if (lvl >= 170)
			leveluse += (lvl - 169) * 2;

		if (lvl >= 180)
			leveluse += (lvl - 179) * 2;

		if (lvl >= 190)
			leveluse += (lvl - 189) * 2;

		lvl = extra->QuestInfo.Celestial.CelestialLevel;
		leveluse += (lvl * 6)/2;

		if (lvl >= 120)
			leveluse += ((lvl - 119) * 6);

		if (lvl >= 150)
			leveluse += ((lvl - 149) * 2);

		if (lvl >= 170)
			leveluse += ((lvl - 169) * 2);

		if (lvl >= 180)
			leveluse += ((lvl - 179) * 2);

		if (lvl >= 190)
			leveluse += ((lvl - 189) * 2);


		if(totaluse > leveluse)
		{
			int dif = totaluse - leveluse;

			if(mob->BaseScore.Str >= dif)
				mob->BaseScore.Str -= dif;

			else if(mob->BaseScore.Int >= dif)
				mob->BaseScore.Int -= dif;

			else if(mob->BaseScore.Dex >= dif)
				mob->BaseScore.Dex -= dif;

			else if(mob->BaseScore.Con >= dif)
				mob->BaseScore.Con -= dif;

			totaluse = mob->BaseScore.Str + mob->BaseScore.Int + mob->BaseScore.Dex + mob->BaseScore.Con - BaseSIDCHM[cls][0] - BaseSIDCHM[cls][1] - BaseSIDCHM[cls][2] - BaseSIDCHM[cls][3];
		}
		mob->ScoreBonus = leveluse - totaluse;
	}
	return TRUE;
}

int BASE_GetHpMp(STRUCT_MOB *mob, STRUCT_MOBEXTRA *extra)
{
	int cls = mob->Class;

	if(cls < 0 || cls >= MAX_CLASS)
		return FALSE;


	int basehp = BaseSIDCHM[cls][4];	
	int stathp = (mob->BaseScore.Con - BaseSIDCHM[cls][3]) * 2;
	int levelhp = ((extra->ClassMaster == ARCH || extra->ClassMaster == MORTAL) ? mob->CurrentScore.Level : mob->CurrentScore.Level + MAX_LEVEL) * g_pIncrementHp[cls];

	int t_hp = basehp + stathp + levelhp;

	if (t_hp >= MAX_HP)
		t_hp = MAX_HP;

	mob->BaseScore.MaxHp = t_hp;
	mob->CurrentScore.MaxHp = t_hp;

	int basemp = BaseSIDCHM[cls][5];
	int statmp = (mob->BaseScore.Int - BaseSIDCHM[cls][1]) * 2;
	int levelmp = ((extra->ClassMaster == ARCH || extra->ClassMaster == MORTAL) ? mob->CurrentScore.Level : mob->CurrentScore.Level + MAX_LEVEL) * g_pIncrementMp[cls];

	int t_mp = basemp + statmp + levelmp;

	if (t_mp >= MAX_MP)
		t_mp = MAX_MP;


	mob->BaseScore.MaxMp = t_mp;
	mob->CurrentScore.MaxMp = t_mp;

	return TRUE;
}

int BASE_GetSpeed(STRUCT_SCORE *score)
{
	int Run = score->AttackRun;

	Run &= 15;

	if(Run < 1)
		Run = 1;

	if(Run > 6)
		Run = 6;

	return Run;
}

int BASE_GetDamage(int dam, int ac, int combat)
{
	long long tdam = dam - ac / 2;
	combat /= 2;

	if(combat > 7)
		combat = 7;

	int delta = 12 - combat;

	int rnd = rand() % delta + combat + 99;

	tdam = (long long)rnd * tdam / 100;

	if(tdam < -50)
		tdam = 0;
	else if(tdam >= -50 && tdam < 0)
	{
		tdam += 50;
		tdam /= 7;
	}
	else if(tdam >= 0 && tdam <= 50)
	{
		tdam = 5 * tdam / 4;
		tdam += 7;
	}

	if(tdam <= 0)
		tdam = 1;

	return (int)tdam;
}

void BASE_GetLanguage(char *str, int idx)
{
  if(idx >= 0 && idx < MAX_STRING)
	  sprintf(str, g_pMessageStringTable[idx]);
  else
	  strcpy(str, "[UNHANDLED]");
}

void BASE_GetLanguage(char *str, int idx, char *parm1)
{
	if(idx >= 0 && idx < MAX_STRING)
		sprintf(str, g_pMessageStringTable[idx], parm1);
	else
		strcpy(str, "[UNHANDLED]");
}

void BASE_GetLanguage(char *str, int idx, char *parm1, char *parm2)
{
	if (idx >= 0 && idx < MAX_STRING)
		sprintf(str, g_pMessageStringTable[idx], parm1, parm2);
	else
		strcpy(str, "[UNHANDLED]");
}

void BASE_GetLanguage(char *str, int idx, int parm1)
{
	if (idx >= 0 && idx < MAX_STRING)
		sprintf(str, g_pMessageStringTable[idx], parm1);
	else
		strcpy(str, "[UNHANDLED]");
}

void BASE_GetLanguage(char *str, int idx, int parm1, int parm2)
{
	if(idx >= 0 && idx < MAX_STRING)
		sprintf(str, g_pMessageStringTable[idx], parm1, parm2);
	else
		strcpy(str, "[UNHANDLED]");
}

void BASE_GetLanguage(char *str, int idx, char * parm1, int parm2)
{
	if(idx >= 0 && idx < MAX_STRING)
		sprintf(str, g_pMessageStringTable[idx], parm1, parm2);
	else
		strcpy(str, "[UNHANDLED]");
}

void BASE_GetLanguage(char *str, int idx, int parm1, char *parm2)
{
	if(idx >= 0 && idx < MAX_STRING)
		sprintf(str, g_pMessageStringTable[idx], parm1, parm2);
	else
		strcpy(str, "[UNHANDLED]");
}

void BASE_GetGuildName(int ServerGroup, unsigned short usGuild, char *szGuildName)
{
	int Group = ServerGroup;
	int Server = (usGuild / MAX_GUILD); //& 15;
	int Guild = usGuild - (Server * MAX_GUILD);

	if(szGuildName == NULL)
		return;

	if((Group < 0 || Group >= MAX_SERVERGROUP) || (Server < 0 || Server >= 16))
	{
		szGuildName[0] = 0;

		return;
	}

	if(Guild >= 0 && Guild < MAX_GUILD &&  g_pGuildName[Group][Server][Guild][0] != 0)
	{
		memcpy(szGuildName, g_pGuildName[Group][Server][Guild], GUILDNAME_LENGTH);

		BASE_UnderBarToSpace(szGuildName);

		return;
	}
	else
	{
		char szGuildNum[3];
		memset(szGuildNum, 0, 3);

		int nGuildIndex = Server * 2;
		memcpy(szGuildNum, &g_pGuildIndex[nGuildIndex], 2);
		sprintf(szGuildName, "%s-", szGuildNum);

		nGuildIndex = 2 * ((usGuild / 64) & 63);
		memcpy(szGuildNum, &g_pGuildMark[nGuildIndex], 2);
		strcat(szGuildName, szGuildNum);

		nGuildIndex = 2 * (usGuild & 63);
		memcpy(szGuildNum, &g_pGuildMark[nGuildIndex], 2);
		strcat(szGuildName, szGuildNum);
	}
}

void BASE_GetClientGuildName(int ServerGroup, unsigned short usGuild, char *szGuildName, int Sub)
{
	char szGuildNum[4];
	memset(szGuildNum, 0, 4);

	int Group = ServerGroup;
	int Server = usGuild / MAX_GUILD;
	int Guild = usGuild & MAX_GUILD-1;

	if(CurrentClientGuild != Group)
		BASE_InitializeClientGuildName(Group);

	if(szGuildName == NULL)
		return;

	if(Group < 0 || Group >= MAX_SERVERGROUP || Server < 0 || Server >= 16)
	{
		szGuildName[0] = 0;

		return;
	}

	if(Guild >= 0 && Guild < MAX_GUILD &&  g_pGuildName[Server][Guild][0][0] != 0)
	{
		memcpy(szGuildName, g_pGuildName[Server][Guild], GUILDNAME_LENGTH);

		BASE_UnderBarToSpace(szGuildName);

		if(Sub >= 1 && Sub <= 3)
		{
			if(g_pClientGuildName[Server][Guild][Sub][0] != 0)
			{
				strcat(szGuildName, "-");
				strcat(szGuildName, g_pClientGuildName[Server][Guild][Sub]);
			}
			else
			{
				strcat(szGuildName, "-");
				memcpy(szGuildNum, (void*)(((int)&g_pGuildSub - 2) + 2 * Sub), 2);
				strcat(&szGuildName[0], szGuildNum);
			}
		}

		return;
	}

	int nGuildIndex = (usGuild / MAX_GUILD) * 2;

	if(nGuildIndex < 0 || nGuildIndex > 20)
	{
		sprintf(szGuildName, "");

		return;
	}

	memcpy(szGuildNum, &g_pGuildIndex[nGuildIndex], 2);
	sprintf(szGuildName, "%s-", szGuildNum);

	nGuildIndex = 2 * ((usGuild / 64) & 63);

	if(nGuildIndex < 0 || nGuildIndex > 128)
	{
		 sprintf(szGuildName, "");

		 return;
	}

	memcpy(&szGuildNum, &g_pGuildMark[nGuildIndex], 2);
	strcat(szGuildName, szGuildNum);

	nGuildIndex = 2 * (usGuild & 0x3F);

	if(nGuildIndex < 0 || nGuildIndex > 128)
	{
		sprintf(szGuildName, "");

		return;
	}

	memcpy(&szGuildNum, &g_pGuildMark[nGuildIndex], 2);
	strcat(szGuildName, szGuildNum);

	if(Sub >= 1 && Sub <= 3)
	{
		memcpy(&szGuildNum, (void*)(((int)&g_pGuildSub - 2) + 2 * Sub), 2);
		strcat(szGuildName, szGuildNum);
	}
}

int BASE_GetSkillDamage(int dam, int ac, int combat)
{
	int tdam = dam - ((ac/ 2));

	if(combat > 15)
		combat = 15;

	int delta = 21 - combat;
	int rnd = rand() % delta + combat + 90;

	tdam = (tdam * rnd) / 100;

	if(tdam < -50)
		tdam = 0;

	else if(tdam > -50 && tdam < 0)
	{
		tdam += 50;
		tdam /= 10;
	}
	else if(tdam >= 0 && tdam <= 45)
	{
		tdam = 5 * tdam / 4;
		tdam += 5;
	}

	if(tdam <= 0)
		tdam = 1;


	return tdam;
}

void BASE_GetItemCode(STRUCT_ITEM * item, char * str)
{
	int iidx = item->sIndex;
	unsigned char c0 = item->stEffect[0].cEffect;
	unsigned char v0 = item->stEffect[0].cValue;
	unsigned char c1 = item->stEffect[1].cEffect;
	unsigned char v1 = item->stEffect[1].cValue;
	unsigned char c2 = item->stEffect[2].cEffect;
	unsigned char v2 = item->stEffect[2].cValue;

	if (iidx <= 0 || iidx >= MAX_ITEMLIST)
	{
		str[0] = 0;
		return;
	}
	sprintf(str, " %s : %d.%d.%d.%d.%d.%d.%d", g_pItemList[item->sIndex].Name, item->sIndex, c0, v0, c1, v1, c2, v2);	
}

int BASE_GetItemAbilityNosanc(STRUCT_ITEM *item, unsigned char Type)
{
	int value = 0;
	int idx = item->sIndex;

	if(idx <= 0 || idx > MAX_ITEMLIST)
		return value;

	int nUnique = g_pItemList[idx].nUnique;
	int nPos = g_pItemList[idx].nPos;

	if((Type == EF_DAMAGEADD || Type == EF_MAGICADD) && (nUnique < 41 || nUnique > 50))
		return FALSE;
	if(Type == EF_CRITICAL && (item->stEffect[1].cEffect == EF_CRITICAL2 || item->stEffect[2].cEffect == EF_CRITICAL2))
		Type = EF_CRITICAL2;
	
	if(Type == EF_DAMAGE && nPos == 32 && (item->stEffect[1].cEffect == EF_DAMAGE2 || item->stEffect[2].cEffect == EF_DAMAGE2))
		Type = EF_DAMAGE2;

	if (Type == EF_MPADD && (item->stEffect[1].cEffect == EF_MPADD2 || item->stEffect[2].cEffect == EF_MPADD2))
		Type = EF_MPADD2;
	
	if (Type == EF_HPADD && (item->stEffect[1].cEffect == EF_HPADD2 || item->stEffect[2].cEffect == EF_HPADD2))
		Type = EF_HPADD2;
	
	if(Type == EF_LEVEL && idx >= 2330 && idx < 2360)
		value = item->stEffect[1].cEffect - 1;

	else if(Type == EF_LEVEL)
		value += g_pItemList[idx].ReqLvl;

	if(Type == EF_REQ_STR)
		value += g_pItemList[idx].ReqStr;

	if(Type == EF_REQ_INT)
		value += g_pItemList[idx].ReqInt;

	if(Type == EF_REQ_DEX)
		value += g_pItemList[idx].ReqDex;

	if(Type == EF_REQ_CON)
		value += g_pItemList[idx].ReqCon;

	if(Type == EF_POS)
		value += g_pItemList[idx].nPos;

	if(Type != EF_INCUBATE)
	{
		for(int i = 0; i < MAX_STATICEFFECT; i++)
		{
			if(g_pItemList[idx].stEffect[i].sEffect != Type)
                continue;

			int tvalue = g_pItemList[idx].stEffect[i].sValue;

            if(Type == EF_ATTSPEED && tvalue == 1)
                tvalue = 10;

            value += tvalue;
		}
	}
	
	if(idx >= 2330 && idx < 2390)
	{
		if(Type == EF_MOUNTHP)
			return item->stEffect[0].sValue;

		else if(Type == EF_MOUNTSANC)
			return item->stEffect[1].cEffect;

		else if(Type == EF_MOUNTLIFE)
			return item->stEffect[1].cValue;

		else if(Type == EF_MOUNTFEED)
			return item->stEffect[2].cEffect;

		else if(Type == EF_MOUNTKILL)
			return item->stEffect[2].cValue;

		if(idx < 2362 || idx >= 2390 || item->stEffect[0].sValue <= 0)
			return value;

		int lv = item->stEffect[1].cEffect;
		int cd = item->sIndex - 2360;

		if(Type == EF_DAMAGE)
			return (lv + 20) * g_pMountBonus[cd][0] / 100; //Retorna o DN da montaria no level atual

		else if(Type == EF_MAGIC)
			return (lv + 15) * g_pMountBonus[cd][1] / 100;

		else if(Type == EF_PARRY)
			return g_pMountBonus[cd][2];

		else if(Type == EF_RESIST1 || Type == EF_RESIST2 || Type == EF_RESIST3 || Type == EF_RESIST4)
			return g_pMountBonus[cd][3];
		else
			return value;
	}

	if(idx >= 3980 && idx <= 3994)
	{
		int cd = idx - 3980;

		if (Type == EF_DAMAGE)
			return g_pMountTempBonus[cd][0]; //Retorna o DN da montaria

		else if (Type == EF_MAGIC)
			return g_pMountTempBonus[cd][1];

		else if (Type == EF_PARRY)
			return g_pMountTempBonus[cd][2];

		else if (Type == EF_RESIST1 || Type == EF_RESIST2 || Type == EF_RESIST3 || Type == EF_RESIST4)
			return g_pMountTempBonus[cd][3];
		else
			return value;
	}

	for(int i = 0; i < 3; i++)
	{
		if(item->stEffect[i].cEffect != Type)
            continue;

		int tvalue = item->stEffect[i].cValue;

		if(Type == EF_ATTSPEED && tvalue == 1)
            tvalue = 10;

		value += tvalue;
	}

	if(Type == EF_RESIST1 || Type == EF_RESIST2 || Type == EF_RESIST3 || Type == EF_RESIST4)
	{
		for(int i = 0; i < MAX_STATICEFFECT; i++)
		{
			if(g_pItemList[idx].stEffect[i].sEffect == EF_RESISTALL)
				value += g_pItemList[idx].stEffect[i].sValue;
		}

		for(int i = 0; i < 3; i++)
		{
			if(item->stEffect[i].cEffect == EF_RESISTALL)
				value += item->stEffect[i].cValue;
		}
	}

	return value;
}

int BASE_GetItemAbility(STRUCT_ITEM *item, unsigned char Type)
{
	int value = 0;

	int idx = item->sIndex;

	if(idx < 0 || idx >= MAX_ITEMLIST)
		return value;

	int nUnique = g_pItemList[idx].nUnique;
	int nPos	= g_pItemList[idx].nPos;

	if(Type == EF_DAMAGEADD || Type == EF_MAGICADD)
	{
		if(nUnique < 41 || nUnique > 50)
			return value;
	}

	if(Type == EF_CRITICAL)
	{
		if(item->stEffect[1].cEffect == EF_CRITICAL2 || item->stEffect[2].cEffect == EF_CRITICAL2)
			Type = EF_CRITICAL2;
	}

	if(Type == EF_DAMAGE && nPos == 32)
	{
		 if(item->stEffect[1].cEffect == EF_DAMAGE2 || item->stEffect[2].cEffect == EF_DAMAGE2)
            Type = EF_DAMAGE2;
	}

	if(Type == EF_ACADD)
	{
        if(item->stEffect[1].cEffect == EF_ACADD2 || item->stEffect[2].cEffect == EF_ACADD2)
            Type = EF_ACADD2;
	}

	if(Type == EF_LEVEL && idx >= 2330 && idx < 2360)
		value = (item->stEffect[1].cEffect - 1);

    else if(Type == EF_LEVEL)
        value += g_pItemList[idx].ReqLvl;

	if(Type == EF_REQ_STR)
		value += g_pItemList[idx].ReqStr;

    if(Type == EF_REQ_INT)
        value += g_pItemList[idx].ReqInt;

    if(Type == EF_REQ_DEX)
        value += g_pItemList[idx].ReqDex;

    if(Type == EF_REQ_CON)
        value += g_pItemList[idx].ReqCon;

	if(Type == EF_POS)
		value += g_pItemList[idx].nPos;

	if(Type != EF_INCUBATE)
	{
		for(int i = 0; i < MAX_STATICEFFECT; i++)
		{
			if(g_pItemList[idx].stEffect[i].sEffect != Type)
                continue;

			int tvalue = g_pItemList[idx].stEffect[i].sValue;

            if(Type == EF_ATTSPEED && tvalue == 1)
                tvalue = 10;

            value += tvalue;
		}
	}

	if(idx >= 2330 && idx < 2390)
	{
		if(Type == EF_MOUNTHP)
			return item->stEffect[0].sValue;

		else if(Type == EF_MOUNTSANC)
            return item->stEffect[1].cEffect;

		else if(Type == EF_MOUNTLIFE)
			return item->stEffect[1].cValue;

        else if(Type == EF_MOUNTFEED)
            return item->stEffect[2].cEffect;

		else if(Type == EF_MOUNTKILL)
            return item->stEffect[2].cValue;

		if(idx < 2362 || idx >= 2390 || item->stEffect[0].sValue <= 0)
			return value;

		int lv = item->stEffect[1].cEffect;
		int cd = item->sIndex - 2360;

		if(Type == EF_DAMAGE)
			return (lv + 20) * g_pMountBonus[cd][0] / 100; //Retorna o DN da montaria no level atual

		else if(Type == EF_MAGIC)
			return (lv + 15) * g_pMountBonus[cd][1] / 100;

		else if(Type == EF_PARRY)
			return g_pMountBonus[cd][2];

		else if(Type == EF_RESIST1 || Type == EF_RESIST2 || Type == EF_RESIST3 || Type == EF_RESIST4)
				return g_pMountBonus[cd][3];
		else
			return value;
	}

	if (idx >= 3980 && idx <= 3994)
	{
		int cd = idx - 3980;

		if (Type == EF_DAMAGE)
			return g_pMountTempBonus[cd][0]; //Retorna o DN da montaria

		else if (Type == EF_MAGIC)
			return g_pMountTempBonus[cd][1];

		else if (Type == EF_PARRY)
			return g_pMountTempBonus[cd][2];

		else if (Type == EF_RESIST1 || Type == EF_RESIST2 || Type == EF_RESIST3 || Type == EF_RESIST4)
			return g_pMountTempBonus[cd][3];
		else
			return value;
	}

	for(int i = 0; i < 3; i++)
	{
		if(item->stEffect[i].cEffect != Type)
            continue;

		int tvalue = item->stEffect[i].cValue;

		if(Type == EF_ATTSPEED && tvalue == 1)
            tvalue = 10;

		value += tvalue;
	}

	if(Type == EF_RESIST1 || Type == EF_RESIST2 || Type == EF_RESIST3 || Type == EF_RESIST4)
    {
        for(int i = 0; i < MAX_STATICEFFECT; i++)
        {
			if(g_pItemList[idx].stEffect[i].sEffect != EF_RESISTALL)
                continue;

            value += g_pItemList[idx].stEffect[i].sValue;
        }

        for(int i = 0; i < 3; i++)
        {
			if(item->stEffect[i].cEffect != EF_RESISTALL)
                continue;

			value += item->stEffect[i].cValue;
        }
    }

	int sanc = BASE_GetItemSanc(item);

	if(sanc == 9 && (nPos & 0xF00) != 0)
		sanc = 10;

	if (Type != EF_GRID && Type != EF_CLASS && Type != EF_POS && Type != EF_WTYPE && Type != EF_RANGE && Type != EF_LEVEL && Type != EF_REQ_STR && Type != EF_REQ_INT && Type != EF_REQ_DEX && Type != EF_REQ_CON && Type != EF_VOLATILE && Type != EF_INCUBATE && Type != EF_INCUDELAY && Type != EF_MOBTYPE && Type != EF_ITEMTYPE && Type != EF_ITEMLEVEL && Type != EF_NOTRADE && Type != EF_NOSANC && Type != EF_DONATE)
	{
		value *= sanc + 10;
		value /= 10;
	}

	if(Type == EF_RUNSPEED)
	{
		if(value >= 3)
			value = 2;

		if(value > 0 && sanc == 9)
			value++;
	}
	/* 
	if(Type == EF_HWORDGUILD || Type == EF_LWORDGUILD)
	{
		unsigned char v = value;

		value = v;
	}
	*/
	if(Type == EF_GRID)
	{
		if(value < 0 || value > 7)
			value = 0;
	}

	return value;
}

int BASE_GetStaticItemAbility(STRUCT_ITEM *item, unsigned char Type)
{
	int value = 0;
	int idx = item->sIndex;

	if(idx <= 0 || idx > MAX_ITEMLIST)
		return value;

	int nPos = g_pItemList[idx].nPos;

	if(Type == EF_LEVEL && idx >= 2330 && idx < 2360)
		value = item->stEffect[1].cEffect - 1;
	else if(Type == EF_LEVEL)
		value += g_pItemList[idx].ReqLvl;

	if(Type == EF_REQ_STR)
		value += g_pItemList[idx].ReqStr;

	if(Type == EF_REQ_INT)
		value += g_pItemList[idx].ReqInt;

	if(Type == EF_REQ_DEX)
		value += g_pItemList[idx].ReqDex;

	if(Type == EF_REQ_CON)
		value += g_pItemList[idx].ReqCon;

	if(Type == EF_POS)
		value += g_pItemList[idx].nPos;

	if(Type != EF_INCUBATE)
	{
		for(int i = 0; i < MAX_STATICEFFECT; i++)
		{
			if(g_pItemList[idx].stEffect[i].sEffect != Type)
                continue;

			int tvalue = g_pItemList[idx].stEffect[i].sValue;

            if(Type == EF_ATTSPEED && tvalue == 1)
                tvalue = 10;

            value += tvalue;
		}
	}

	if(Type == EF_RESIST1 || Type == EF_RESIST2 || Type == EF_RESIST3 || Type == EF_RESIST4)
	{
		for(int i = 0; i < MAX_STATICEFFECT; i++)
		{
			if(g_pItemList[idx].stEffect[i].sEffect == EF_RESISTALL)
				value += g_pItemList[idx].stEffect[i].sValue;
		}

		for(int i = 0; i < 3; i++)
		{
			if(item->stEffect[i].cEffect == EF_RESISTALL)
				value += item->stEffect[i].cValue;
		}
	}

	if(idx >= 2330 && idx < 2390)
	{
		if(Type == EF_MOUNTHP)
			return item->stEffect[0].sValue;

		else if(Type == EF_MOUNTSANC)
			return item->stEffect[1].cEffect;

		else if(Type == EF_MOUNTLIFE)
			return item->stEffect[1].cValue;

		else if(Type == EF_MOUNTFEED)
			return item->stEffect[2].cEffect;

		else if(Type == EF_MOUNTKILL)
			return item->stEffect[2].cValue;

		if(idx < 2362 || idx >= 2390 || item->stEffect[0].sValue <= 0)
			return value;

		int lv = item->stEffect[1].cEffect;
		int cd = item->sIndex - 2360;

		if(Type == EF_DAMAGE)
			return (lv + 20) * g_pMountBonus[cd][0] / 100; //Retorna o DN da montaria no level atual

		else if(Type == EF_MAGIC)
			return (lv + 15) * g_pMountBonus[cd][1] / 100;

		else if(Type == EF_PARRY)
			return g_pMountBonus[cd][2];

		else if(Type == EF_RESIST1 || Type == EF_RESIST2 || Type == EF_RESIST3 || Type == EF_RESIST4)
			return g_pMountBonus[cd][3];
		else
			return value;
	}

	if (idx >= 3980 && idx <= 3994)
	{
		int cd = idx - 3980;

		if (Type == EF_DAMAGE)
			return g_pMountTempBonus[cd][0]; //Retorna o DN da montaria

		else if (Type == EF_MAGIC)
			return g_pMountTempBonus[cd][1];

		else if (Type == EF_PARRY)
			return g_pMountTempBonus[cd][2];

		else if (Type == EF_RESIST1 || Type == EF_RESIST2 || Type == EF_RESIST3 || Type == EF_RESIST4)
			return g_pMountTempBonus[cd][3];
		else
			return value;
	}

	int sanc = BASE_GetItemSanc(item);

	if(sanc == 9 && (nPos & 0xF00) != 0)
		sanc = 10;

	if(Type != EF_GRID && Type != EF_CLASS && Type != EF_POS && Type != EF_WTYPE && Type != EF_RANGE && Type != EF_LEVEL && 
		Type != EF_REQ_STR && Type != EF_REQ_INT && Type != EF_REQ_DEX && Type != EF_REQ_CON && Type != EF_INCUBATE && Type != EF_INCUDELAY)
	{
		value *= sanc + 10;
		value /= 10;
	}

	if(Type == EF_RUNSPEED)
	{
		if(value >= 3)
			value = 2;

		if(value > 0 && sanc == 9)
			value++;
	}

	if(Type == EF_HWORDGUILD || Type == EF_LWORDGUILD)
	{
		unsigned char v = value;

		value = v;
	}

	return value;
}

int BASE_GetBonusItemAbility(STRUCT_ITEM *item, unsigned char Type)
{
	if(item->sIndex >= 2330 && item->sIndex < 2390)
		return FALSE;

	int value = 0;

	int idx = item->sIndex;

	if(idx <= 0 || idx > MAX_ITEMLIST)
		return value;

	int nPos = g_pItemList[idx].nPos;

	for(int i = 0; i < 3; i++)
	{
		if(item->stEffect[i].cEffect != Type)
			continue;

		int tvalue = item->stEffect[i].cValue;

		if(Type == EF_ATTSPEED && tvalue == 1)
			tvalue = 10;

		value += tvalue;
	}

	if(Type == EF_RESIST1 || Type == EF_RESIST2 || Type == EF_RESIST3 || Type == EF_RESIST4)
	{
		for(int i = 0; i < MAX_STATICEFFECT; i++)
		{
			if(g_pItemList[idx].stEffect[i].sEffect == EF_RESISTALL)
				value += g_pItemList[idx].stEffect[i].sValue;
		}

		for(int i = 0; i < 3; i++)
		{
			if(item->stEffect[i].cEffect == EF_RESISTALL)
				value += item->stEffect[i].cValue;
		}
	}

	int sanc = BASE_GetItemSanc(item);

	if(sanc == 9 && (nPos & 0xF00) != 0)
		sanc = 10;

	if(sanc == 0)
		return value;

	if(Type != EF_GRID && Type != EF_CLASS && Type != EF_POS && Type != EF_WTYPE && Type != EF_RANGE && Type != EF_LEVEL && Type != EF_REQ_STR && Type != EF_REQ_INT && Type != EF_REQ_DEX && Type != EF_REQ_CON && Type != EF_VOLATILE && Type != EF_INCUBATE && Type != EF_INCUDELAY)
	{
		value *= sanc + 10;
		value /= 10;
	}

	return value;
}

int BASE_GetBonusItemAbilityNosanc(STRUCT_ITEM *item, unsigned char Type)
{
	if(item->sIndex >= 2330 && item->sIndex < 2390)
		return FALSE;

	int value = 0;

	int idx = item->sIndex;

	if(idx <= 0 || idx > MAX_ITEMLIST)
		return value;

	for(int i = 0; i < 3; i++)
	{
		if(item->stEffect[i].cEffect != Type)
			continue;

		int tvalue = item->stEffect[i].cValue;

		if(Type == EF_ATTSPEED && tvalue == 1)
			tvalue = 10;

		value += tvalue;
	}

	if(Type == EF_RESIST1 || Type == EF_RESIST2 || Type == EF_RESIST3 || Type == EF_RESIST4)
	{
		for(int i = 0; i < MAX_STATICEFFECT; i++)
		{
			if(g_pItemList[idx].stEffect[i].sEffect == EF_RESISTALL)
				value += g_pItemList[idx].stEffect[i].sValue;
		}

		for(int i = 0; i < 3; i++)
		{
			if(item->stEffect[i].cEffect == EF_RESISTALL)
				value += item->stEffect[i].cValue;
		}
	}

	return value;
}

int BASE_GetItemSanc(STRUCT_ITEM *item)
{
	if(item->sIndex >= 2330 && item->sIndex < 2390)
		return FALSE;

	unsigned char sanc = 0;

	if (item->stEffect[0].cEffect >= 116 && item->stEffect[0].cEffect <= 125)
		sanc = item->stEffect[0].cValue;

	else if (item->stEffect[1].cEffect >= 116 && item->stEffect[1].cEffect <= 125)
		sanc = item->stEffect[1].cValue;
	
	else if (item->stEffect[2].cEffect >= 116 && item->stEffect[2].cEffect <= 125)
		sanc = item->stEffect[2].cValue;

	else if(item->stEffect[0].cEffect == EF_SANC || item->stEffect[1].cEffect == EF_SANC || item->stEffect[2].cEffect == EF_SANC)
	{
		if(item->stEffect[0].cEffect == EF_SANC)
			sanc = item->stEffect[0].cValue;
		else if(item->stEffect[1].cEffect == EF_SANC)
			sanc = item->stEffect[1].cValue;
		else
			sanc = item->stEffect[2].cValue;
	}

	int isanc = sanc;

	if (isanc == 9)
		isanc = 9;
	else if (isanc >= 230 && isanc <= 233)
		isanc = REF_10;
	else if (isanc >= 234 && isanc <= 237)
		isanc = REF_11;
	else if (isanc >= 238 && isanc <= 241)
		isanc = REF_12;
	else if (isanc >= 242 && isanc <= 245)
		isanc = REF_13;
	else if (isanc >= 246 && isanc <= 249)
		isanc = REF_14;
	else if (isanc >= 250 && isanc <= 253)
		isanc = REF_15;
	else
		isanc = isanc % 10;

	return isanc;
}

int BASE_GetItemGem(STRUCT_ITEM *item)
{
	if (item->sIndex >= 2330 && item->sIndex < 2390)
		return FALSE;

	unsigned char sanc = 0;

	if (item->stEffect[0].cEffect >= 116 && item->stEffect[0].cEffect <= 125)
		sanc = item->stEffect[0].cValue;

	else if (item->stEffect[1].cEffect >= 116 && item->stEffect[1].cEffect <= 125)
		sanc = item->stEffect[1].cValue;

	else if (item->stEffect[2].cEffect >= 116 && item->stEffect[2].cEffect <= 125)
		sanc = item->stEffect[2].cValue;

	else if (item->stEffect[0].cEffect == EF_SANC || item->stEffect[1].cEffect == EF_SANC || item->stEffect[2].cEffect == EF_SANC)
	{
		if (item->stEffect[0].cEffect == EF_SANC)
			sanc = item->stEffect[0].cValue;
		else if (item->stEffect[1].cEffect == EF_SANC)
			sanc = item->stEffect[1].cValue;
		else
			sanc = item->stEffect[2].cValue;
	}

	int isanc = sanc;

	if(isanc < 230)
		return -1;

	return (isanc-230) % 4;
}

int BASE_GetItemSancSuccess(STRUCT_ITEM *item)
{
	 if(item->sIndex >= 2330 && item->sIndex < 2390)
		 return FALSE;

	 unsigned char sanc = 0;

	 if (item->stEffect[0].cEffect == EF_SANC || item->stEffect[1].cEffect == EF_SANC || item->stEffect[2].cEffect == EF_SANC || item->stEffect[0].cEffect >= 116 && item->stEffect[0].cEffect <= 125 || item->stEffect[1].cEffect >= 116 && item->stEffect[1].cEffect <= 125 || item->stEffect[2].cEffect >= 116 && item->stEffect[2].cEffect <= 125)
	 {
		 if (item->stEffect[0].cEffect == EF_SANC || item->stEffect[0].cEffect >= 116 && item->stEffect[0].cEffect <= 125)
			 sanc = item->stEffect[0].cValue;
		 
		 else if (item->stEffect[1].cEffect == EF_SANC || item->stEffect[1].cEffect >= 116 && item->stEffect[1].cEffect <= 125)
			 sanc = item->stEffect[1].cValue;
		 
		 else
			 sanc = item->stEffect[2].cValue;		 
	 }

	 int isanc = sanc;
	 isanc /= 10;

	 return isanc;
}

int BASE_GetSuccessRate(STRUCT_ITEM *item, int OriLacto)
{
	if(item->sIndex >= 2330 && item->sIndex < 2390)
		return FALSE;

	if(OriLacto != 0 && OriLacto != 1 && OriLacto != 2)
		return FALSE;
	
	int sanc = BASE_GetItemSanc(item);

	if(sanc == REF_10 && OriLacto != 2)
		return 15;

	if(sanc == REF_10 && OriLacto == 2)
		return 100;

	sanc %= 10;
	int succ = BASE_GetItemSancSuccess(item);
	int rate = g_pSancRate[OriLacto][sanc + 1];

	rate += succ * g_pSuccessRate[sanc + 1];

	return OriLacto == 2 ? 100 : rate;
}

int BASE_GetGrowthRate(STRUCT_ITEM *item)
{
	if(item->sIndex < 2360 || item->sIndex >= 2390)
		return FALSE;

	int sanc = item->stEffect[1].cEffect;
	sanc /= 10;
	int rate = g_pSancRate[2][sanc];

	return rate;
}

int BASE_SetItemSanc(STRUCT_ITEM *dest, int sanc, int success)
{
	if(sanc < 0)
		sanc = 0;
	if(sanc > 15)
		sanc = 0;
	if(success < 0)
		success = 0;
	if(success > 20)
		success = 20;

	int	sc = sanc + (10 * success);

	if (sanc == 10)
		sc = 230 + success;

	else if (sanc == 11)
		sc = 234 + success;

	else if (sanc == 12)
		sc = 238 + success;

	else if (sanc == 13)
		sc = 242 + success;

	else if (sanc == 14)
		sc = 246 + success;

	else if (sanc == 15)
		sc = 250 + success;

	if (dest->stEffect[0].cEffect == EF_SANC || dest->stEffect[0].cEffect >= 116 && dest->stEffect[0].cEffect <= 125)
		dest->stEffect[0].cValue = sc;

	else if (dest->stEffect[1].cEffect == EF_SANC || dest->stEffect[1].cEffect >= 116 && dest->stEffect[1].cEffect <= 125)
		dest->stEffect[1].cValue = sc;
	
	else
	{
		if (dest->stEffect[2].cEffect != EF_SANC && (dest->stEffect[2].cEffect < 116 || dest->stEffect[2].cEffect > 125))
			return FALSE;

		dest->stEffect[2].cValue = sc;
	}

	return TRUE;
}

int BASE_GetItemAmount(STRUCT_ITEM *item)
{
	int amount = 0;

	if(item->stEffect[0].cEffect == EF_AMOUNT || item->stEffect[1].cEffect == EF_AMOUNT || item->stEffect[2].cEffect == EF_AMOUNT)
	{
		if(item->stEffect[0].cEffect == EF_AMOUNT)
			amount = item->stEffect[0].cValue;
		else if(item->stEffect[1].cEffect == EF_AMOUNT)
			amount = item->stEffect[1].cValue;
		else
			amount = item->stEffect[2].cValue;
	}

	return amount;
}

void BASE_SetItemAmount(STRUCT_ITEM *item, int amount)
{
	if(item->stEffect[0].cEffect == EF_AMOUNT || item->stEffect[1].cEffect == EF_AMOUNT || item->stEffect[2].cEffect == EF_AMOUNT)
	{
		if(item->stEffect[0].cEffect == EF_AMOUNT)
			item->stEffect[0].cValue = amount;
		
		else if(item->stEffect[1].cEffect == EF_AMOUNT)
			item->stEffect[1].cValue = amount;
		
		else
			item->stEffect[2].cValue = amount;
	}
	else if(item->stEffect[0].cEffect == 0 || item->stEffect[1].cEffect == 0 || item->stEffect[2].cEffect == 0)
	{
		
		if(item->stEffect[0].cEffect == 0)
		{
			item->stEffect[0].cEffect = EF_AMOUNT;
			item->stEffect[0].cValue = amount;
		}
		else if(item->stEffect[1].cEffect == 0 )
		{

			item->stEffect[1].cEffect = EF_AMOUNT;
			item->stEffect[1].cValue = amount;

		}
		else
		{
			item->stEffect[2].cEffect = EF_AMOUNT;
			item->stEffect[2].cValue = amount;	
		}
	}
}

STRUCT_ITEM *GetItemPointer(STRUCT_MOB *mob, STRUCT_ITEM *cargo, int type, int pos)
{
	STRUCT_ITEM * sour = NULL;

    if(type == ITEM_PLACE_EQUIP)
	{   
		if(pos < 0 || pos >= MAX_EQUIP)
			return NULL;

		sour = &mob->Equip[pos];
	}   
	else if(type == ITEM_PLACE_CARRY)
	{   
		if(pos < 0 || pos >= MAX_CARRY)
			return NULL;

		sour = &mob->Carry[pos];
	}   
	else if(type == ITEM_PLACE_CARGO)
	{   
		if(pos < 0 || pos >= MAX_CARGO) 
			return NULL;

		sour = &cargo[pos];
	}  
	else 
		return NULL; 

	if(sour->sIndex < 0 || sour->sIndex >= MAX_ITEMLIST)		
		return NULL;

   return sour;
}

int BASE_GetMobAbility(STRUCT_MOB *mob, unsigned char Type)
{
	int value = 0;
	int nUnique[MAX_EQUIP];

	if(Type == EF_RANGE)
	{
		value = BASE_GetMaxAbility(mob, Type);

		int cls = mob->Equip[0].sIndex / 10;

		if(value < 2 && cls == 3)
		{
			if((mob->LearnedSkill & 0x100000) != 0)
                value = 2;
		}

		return value;
	}

	for(int i = 0; i < MAX_EQUIP; i++)
	{
		nUnique[i] = 0;

		int item = mob->Equip[i].sIndex;

		if(item == 0 && i != 7)
			continue;

		if(i >= 1 && i <= 5)
			nUnique[i] = g_pItemList[item].nUnique;

		if(Type == EF_DAMAGE && i == 6)
            continue;

		if(Type == EF_MAGIC && i == 7)
            continue;

		if(i == 7 && Type == EF_DAMAGE)
		{
			int ldam = BASE_GetItemAbility(&mob->Equip[6], Type) + BASE_GetItemAbility(&mob->Equip[6], EF_DAMAGE2);
			int rdam = BASE_GetItemAbility(&mob->Equip[7], Type) + BASE_GetItemAbility(&mob->Equip[7], EF_DAMAGE2);

			int lidx = mob->Equip[6].sIndex;
			int ridx = mob->Equip[7].sIndex;

			int ltype = 0;

			if(lidx > 0 && lidx < MAX_ITEMLIST)
                ltype = g_pItemList[lidx].nUnique;

            int rtype = 0;

            if(ridx > 0 && ridx < MAX_ITEMLIST)
                rtype = g_pItemList[ridx].nUnique;

			if(ltype != 0 && rtype != 0)
			{
				int multi = 0;

				if(ltype == rtype)
                    multi = 50;
				else
					multi = 30;

				//Pericia do caçador
				if(mob->LearnedSkill & (1 << 10) && mob->Class == 3)
					multi = 100;

				//Mestre das Armas
				if(mob->LearnedSkill & (1 << 9) && mob->Class == 0)
					multi = 100;

				if(ldam > rdam)
                    value = ((value + ldam) + ((rdam * multi) / 100));
                else
                    value = ((value + rdam) + ((ldam * multi) / 100));

				continue;
			}

			if(ldam > rdam)
                value += ldam;
            else
                value += rdam;

			continue;
		}

		int tvalue = 0;

		tvalue = BASE_GetItemAbility(&mob->Equip[i], Type);

		if(Type == EF_ATTSPEED && tvalue == 1)
            tvalue = 10;

		value += tvalue;
	}

	 if(Type == EF_AC && nUnique[1] != 0)
	 {
        if(nUnique[1] == nUnique[2] && nUnique[2] == nUnique[3] && nUnique[3] == nUnique[4] && nUnique[4] == nUnique[5])
            value = ((value * 105) / 100);
	 }

	 return value;
}

int BASE_GetMaxAbility(STRUCT_MOB *mob, unsigned char Type)
{
	int value = 0;
	int tvalue = 0;

	for(int i = 0; i < MAX_EQUIP; i++)
	{
		int item = mob->Equip[i].sIndex;

		if(item == 0)
			continue;

		tvalue = BASE_GetItemAbility(&mob->Equip[i], Type);

		if(value < tvalue)
			value = tvalue;
	}

	return value;
}

int BASE_GetMobCheckSum(STRUCT_MOB *mob)
{
	int sum = 0;
	int i;

	memset((void*)&mob->BaseScore, 0, sizeof(mob->BaseScore));

	for(i = 0; i < MAX_EQUIP; i++)   
		sum = sum + mob->Equip[i].sIndex * i;

	for(i = 0; i < MAX_CARRY; i++)    
		sum = sum + mob->Carry[i].sIndex * i;

	return sum;
}

#define ISALPHA(x) (((x) >= 'a' && (x) <= 'z') || ((x) >= 'A' && (x) <= 'Z'))
#define ISDIGIT(x) ((x) >= '0' && (x) <= '9')

bool BASE_CheckValidString(char *name)
{
	int l = strlen(name);

	if (l < 4 || l >= NAME_LENGTH)  
		return FALSE;

	if(strcmp(name, "Reino") == 0 || strcmp(name, "subcreate") == 0 || strcmp(name, "create") == 0 || strcmp(name, "gritar") == 0 || strcmp(name, "king") == 0 || strcmp(name, "kingdom") == 0 ||
	   strcmp(name, "getout") == 0 || strcmp(name, "gfame") == 0 || strcmp(name, "expulsar") == 0 || strcmp(name, "summonguild") == 0 || strcmp(name, "summon") == 0 || strcmp(name, "time") == 0 ||
	   strcmp(name, "relo") == 0 || strcmp(name, "stopally") == 0 || strcmp(name, "stopwar") == 0)
	   return FALSE;

	for(int j = 0; j < l; j++)
	{	 
		char x = name[j];

		if(x < 0) 
		{
			j++;
			continue;
		}

		if(ISALPHA(x) || ISDIGIT(x) || x == '-')  
			continue;

		return false;
	}

	return true;
}

bool BASE_CheckHangul(unsigned short word)
{   
	 unsigned char x = (unsigned char)word;
	 unsigned char y = (unsigned char)(word >> 8);

	 if(x >= 0xb0 && x <= 0xc9 && y >= 0xa1 && y<= 0xff)	
		 return true;
	 else 
		 return false;
}

int BASE_InitializeBaseDef()
{
	memset(ChargedGuildList, 0, sizeof(ChargedGuildList));

	BASE_InitializeServerList();

	int ret	= 1;
	int tret = 0;

	BASE_InitializeSkill();
	BASE_InitializeEffectName();
	BASE_InitializeItemList();
	BASE_InitializeInitItem();
	BASE_InitializeGuildName();
	BASE_InitializeAttribute();

	return ret;
}

void BASE_ApplyAttribute(char *pHeight, int size)
{
	int endx = g_HeightPosX + size;
	int endy = g_HeightPosY + size;

	int xx;
	int yy;

	for(int y = g_HeightPosY; y < endy; y++)
	{
		for(int x = g_HeightPosX; x < endx; x++)
		{
			xx = (x >> 2) & 0x3FF;

			yy = (y >> 2) & 0x3FF;

			unsigned char att = g_pAttribute[yy][xx];

			if(att & 2)
				pHeight[x + g_HeightWidth * (y - g_HeightPosY) - g_HeightPosX] = 127;
		}

	}
}

void BASE_InitModuleDir()
{
	char String[256];
	
	GetModuleFileName(NULL, String, 256);

	int len = strlen(String);

	len--;

	for(int i = len; i > 0; i--)
	{
		if(String[i] == '\\')
		{
			String[i] = 0;

			break;
		}
	}

	SetCurrentDirectory(String);	
}

int BASE_GetSum(char * p ,int size)
{
	int sum = 0;

    for(int i = 0; i < size; i++)
	{    
		int mod = i%7;

	    if(mod == 0)
			sum = sum + p[i] / 2;
	    
		if(mod == 1)
			sum = sum + (p[i] ^ 0xFF);
		
		if(mod == 2)
			sum = sum + p[i] * 3;
	 	
		if(mod == 3)
			sum = sum + p[i] * 2;
		
		if(mod == 4)
			sum = sum - (p[i] / 7);
		
		if(mod == 5)
			sum =sum - p[i];
		else
			sum = sum + (p[i] / 3);
	}

	return sum;
}

int BASE_GetSum2(char *p, int size)
{
	int sum = 0;

    for(int i = 0; i < size; i++)
	{
		 int mod = i%9;
	    
		 if (mod == 0)
			 sum = sum + p[i] * 2;
	     
		 if (mod == 1)
			 sum = sum + (p[i] ^ 0xFF);
		 
		 if (mod == 2)
			 sum = sum + p[i] / 3;
	 	 
		 if (mod == 3)
			 sum = sum + p[i] * 2;
		 
		 if (mod == 4)
			 sum = sum - (p[i] ^ 0x5A);
		 
		 if (mod == 5)
			 sum = sum - p[i];
		 
		 else
			 sum = sum + (p[i] / 5);
	}

	return sum;
}

void BASE_WriteSkillBin()
{
	int size = sizeof(STRUCT_SPELL) * MAX_SKILLINDEX;
	char *temp = (char*)g_pSpell;

	for(int i = 0; i < size; i++)
	{
		temp[i] = temp[i] ^ 0x5A;
	}

	FILE *fp = fopen("SkillData.bin", "wb");

	if(fp != NULL)
	{	
		fwrite(g_pSpell, sizeof(STRUCT_SPELL), MAX_SKILLINDEX , fp);

		int sum = BASE_GetSum2((char*)g_pSpell, size);
		int nRand = rand();

		fwrite(&nRand, sizeof(nRand), 1 , fp);
		fclose(fp);

		FILE *fpCSum = fopen("..\\SkillCSum.h", "wt");

		if(fpCSum != NULL)
		{
			fprintf(fpCSum, "#define SKILL_CHECKSUM %d", sum);
			fclose(fp);
		}
	}

	for(int i = 0; i < size; i++)
	{ 
		temp[i] = temp[i] ^ 0x5A;
	}

}

void BASE_WriteItemList(int nItemList)
{
	int size = sizeof(STRUCT_ITEMLIST)*MAX_ITEMLIST;
	char *temp = (char*)&g_pItemList;

	for(int i = 0; i < size; i++)
	{
		temp[i] = temp[i] ^ 0x5A;
	}

	FILE *fp = NULL;

	if(nItemList == 0)
	{
		fp = fopen("../../Common/ItemList.bin", "wb");

		if(fp == NULL)
		{
			MessageBox(NULL, "ItemList.binÀ» »ý¼ºÇÒ¼ö ¾ø½À´Ï´Ù. ", "ERROR", NULL);
			return;
		}

		fwrite(g_pItemList, sizeof(STRUCT_ITEMLIST)*MAX_ITEMLIST, 1, fp);
	}

	int sum = BASE_GetSum2((char*)g_pItemList, size);

	int nRand = rand();

	if(nItemList == 0)
	{
		fwrite(&nRand, sizeof(nRand), 1 , fp);
		fclose(fp);
	}

	FILE* fpCSum = NULL;

	if(nItemList == 0)
		fpCSum = fopen("..\\ItemCSum.h", "wt");
	else
		fpCSum = fopen("..\\ItemCSum.h", "a+");

	if(fpCSum != NULL)
	{
		if(nItemList == 0)
			fprintf(fpCSum, "#define ITEM_CHECKSUM %d\n", sum);
		else
			fprintf(fpCSum, "#define ITEM_CHECKSUM2 %d\n", sum);

		fclose(fpCSum);
	}

    for(int i = 0; i < size; i++)
	{
		temp[i] = temp[i]^0x5A;
	}
}

int BASE_ReadSkillBin()
{
	int size = sizeof(STRUCT_SPELL) * MAX_SKILLINDEX;
	char *temp = (char*)g_pSpell;
	int tsum = 0;

	FILE *fp = fopen("SkillData.bin", "rb");

	if(fp != NULL)
	{
		fread(g_pSpell, size , 1 , fp);
		fread(&tsum, sizeof(tsum), 1 , fp);

		fclose(fp);
	}
	else
	{
		MessageBox(NULL, "Can't read SkillData.bin", "ERROR", NULL);
		return FALSE;
	}

	int sum = BASE_GetSum2((char*)g_pSpell, size);

#ifndef _DEBUG
	//if(SKILL_CHECKSUM != sum) 
	//	return FALSE;
#endif

	for(int i = 0; i < size;i++)
	{	
		temp[i] = temp[i] ^ 0x5A;
	}

	return TRUE;
}

int BASE_ReadItemList()
{
	int size = sizeof(STRUCT_ITEMLIST) * MAX_ITEMLIST;
	char *temp = (char*)g_pItemList;
	int tsum = 0;

	FILE *fp;
	fp = fopen("../../Common/ItemList.bin", "rb");

	if(fp == NULL) 
	{
		MessageBox(NULL, "Can't read ItemList.bin", "ERROR", NULL);

		return FALSE;
	}

	fread(g_pItemList, sizeof(STRUCT_ITEMLIST)*MAX_ITEMLIST, 1, fp);
	fread(&tsum, sizeof(tsum), 1, fp);

	fclose(fp);

	int sum = BASE_GetSum2((char*)g_pItemList, size);

#ifndef _DEBUG
	//if (ITEM_CHECKSUM != sum)
	//	return FALSE;
#endif

	for(int i = 0; i < size; i++)
	{ 
		temp[i] = temp[i] ^ 0x5A;
	}

	int Handle = _open("../../Common/ExtraItem.bin", _O_RDONLY|_O_BINARY, 0);

	if(Handle != -1)
	{
		while(1)
		{ 
			char buff[256];

			STRUCT_ITEMLIST *item;

			int sz = _read(Handle, buff, sizeof(STRUCT_ITEMLIST)+2);

			if(sz < sizeof(STRUCT_ITEMLIST)+2) 
				break;

			short idx = *((short*)buff);

			if(idx <= 0 || idx >= MAX_ITEMLIST) 
				continue;

			item = (STRUCT_ITEMLIST *)(buff+2);

			g_pItemList[idx] = *item;
		}

		_close(Handle);
#ifndef _DEBUG
		for(int i = 0; i < size; i++)
		{ 
			temp[i] = temp[i] ^ 0x5A;
		}

		int sum = BASE_GetSum2((char*)g_pItemList, size);

		//if(ITEM_CHECKSUM2 != sum) 
		//	return FALSE;

		for(int i = 0; i < size; i++)
		{
			temp[i] = temp[i] ^ 0x5A;
		}
#endif
	}

	return TRUE;
}

void BASE_SpaceToUnderBar(char *szStr)
{
	for(unsigned int i = 0; i < strlen(szStr); i++)
	{
		if(szStr[i] == ' ')
			szStr[i] = '_';
	}
}

void BASE_UnderBarToSpace(char *szStr)
{ 	
	int nStrLen = strlen(szStr);
	/*
	unsigned char *pStr = (unsigned char*)szStr;
	unsigned char *pNext = _mbschr(pStr, '_');

	while(1)
	{
		*pNext = ' ';

		pNext = _mbschr(pNext, '_');

		nStrLen--;

		if(nStrLen < 0)
			break;
	}*/

	for (int i = 0; i < nStrLen; i++)
	{
		if (szStr[i] == '_')
			szStr[i] = ' ';
	}
}

void BASE_ClearMob(STRUCT_MOB *mob)
{
	memset(mob, 0, sizeof(STRUCT_MOB));

	mob->SPX = 2112;
	mob->SPY = 2112;

	memset(&mob->BaseScore, 0, sizeof(STRUCT_SCORE));
	memset(&mob->CurrentScore, 0, sizeof(STRUCT_SCORE));

	for(int i = 0; i < MAX_EQUIP; i++)
		BASE_ClearItem(&mob->Equip[i]);

	for(int i = 0; i < MAX_CARRY; i++)
		BASE_ClearItem(&mob->Carry[i]);

	mob->SkillBar[0] = 0;
	mob->SkillBar[1] = 0;
	mob->SkillBar[2] = 0;
	mob->SkillBar[3] = 0;
}

void BASE_ClearMobExtra(STRUCT_MOBEXTRA *extra)
{
	memset(extra, 0, sizeof(STRUCT_MOBEXTRA));

	extra->ClassMaster = MORTAL;
	memset(&extra->QuestInfo, 0, sizeof(extra->QuestInfo));
}

void BASE_GetCurrentScore(STRUCT_MOB& MOB, STRUCT_AFFECT* Affect, STRUCT_MOBEXTRA* extra, int* ExpBonus, int* ForceMobDamage, int isSummon, int* Accuracy, int* AbsHp, int* ForceDamage)
{
	MOB.Rsv = 0;

	int hp = MOB.CurrentScore.Hp;
	int mp = MOB.CurrentScore.Mp;
	MOB.CurrentScore = MOB.BaseScore;

	MOB.CurrentScore.Hp = hp;
	MOB.CurrentScore.Mp = mp;

	int acadd = BASE_GetMobAbility(&MOB, EF_AC) + BASE_GetMobAbility(&MOB, EF_ACADD);

	MOB.CurrentScore.Ac += acadd;
	MOB.CurrentScore.Damage += BASE_GetMobAbility(&MOB, EF_DAMAGE);

	MOB.CurrentScore.MaxHp += BASE_GetMobAbility(&MOB, EF_HP);
	MOB.CurrentScore.MaxMp += BASE_GetMobAbility(&MOB, EF_MP);

	MOB.CurrentScore.Str += BASE_GetMobAbility(&MOB, EF_STR);
	MOB.CurrentScore.Int += BASE_GetMobAbility(&MOB, EF_INT);
	MOB.CurrentScore.Dex += BASE_GetMobAbility(&MOB, EF_DEX);
	MOB.CurrentScore.Con += BASE_GetMobAbility(&MOB, EF_CON);

	int isKibitaSoul = 0;

	if (isSummon == 0)
	{
#pragma region Soul Hp/Mp add
		int bInt = MOB.CurrentScore.Int;
		int bCon = MOB.CurrentScore.Con;

		for (int i = 0; i < MAX_AFFECT; i++)
		{
			int Type = Affect[i].Type;

			if (Type == 29 && extra->ClassMaster != MORTAL && extra->ClassMaster != ARCH) // Soul Celestial e Sub-Celestial
			{
				if (extra->Soul == SOUL_I)
					bInt = (int)(bInt * 2.2f);

				if (extra->Soul == SOUL_C)
					bCon = (int)(bCon * 2.2f);

				else if (extra->Soul == SOUL_FI)
					bInt = (int)(bInt * 1.40f);

				else if (extra->Soul == SOUL_FC)
					bCon = (int)(bCon * 1.40f);


				else if (extra->Soul == SOUL_IF)
					bInt = (int)(bInt * 1.80f);

				else if (extra->Soul == SOUL_ID)
					bInt = (int)(bInt * 1.80f);


				else if (extra->Soul == SOUL_IC)
				{
					bInt = (int)(bInt * 1.80f);
					bCon = (int)(bCon * 1.40f);
				}

				else if (extra->Soul == SOUL_DI)
					bInt = (int)(bInt * 1.40f);

				else if (extra->Soul == SOUL_DC)
					bCon = (int)(bCon * 1.40f);


				else if (extra->Soul == SOUL_CF)
					bCon = (int)(bCon * 1.80f);


				else if (extra->Soul == SOUL_CI)
				{
					bCon = (int)(bCon * 1.80f);
					bInt = (int)(bInt * 1.40f);
				}

				else if (extra->Soul == SOUL_CD)
					bCon = (int)(bCon * 1.80f);
			}

			else if (Type == 29 && extra->ClassMaster == MORTAL && MOB.CurrentScore.Level < 369) // Soul Kibita
				isKibitaSoul = 1;

			else if (Type == 29 && extra->ClassMaster == MORTAL) // Soul Mortal
			{
				if (extra->Soul == SOUL_I)
					bInt = (int)(bInt * 1.8f);

				if (extra->Soul == SOUL_C)
					bCon = (int)(bCon * 1.8f);

				else if (extra->Soul == SOUL_FI)
					bInt = (int)(bInt * 1.40f);

				else if (extra->Soul == SOUL_FC)
					bCon = (int)(bCon * 1.40f);


				else if (extra->Soul == SOUL_IF)
					bInt = (int)(bInt * 1.60f);

				else if (extra->Soul == SOUL_ID)
					bInt = (int)(bInt * 1.60f);


				else if (extra->Soul == SOUL_IC)
				{
					bInt = (int)(bInt * 1.60f);
					bCon = (int)(bCon * 1.40f);
				}

				else if (extra->Soul == SOUL_DI)
					bInt = (int)(bInt * 1.40f);

				else if (extra->Soul == SOUL_DC)
					bCon = (int)(bCon * 1.40f);


				else if (extra->Soul == SOUL_CF)
					bCon = (int)(bCon * 1.60f);


				else if (extra->Soul == SOUL_CI)
				{
					bCon = (int)(bCon * 1.60f);
					bInt = (int)(bInt * 1.40f);
				}

				else if (extra->Soul == SOUL_CD)
					bCon = (int)(bCon * 1.60f);
			}
		}

		int ItemInt = MOB.CurrentScore.Int - MOB.BaseScore.Int;
		int ItemCon = MOB.CurrentScore.Con - MOB.BaseScore.Con;

		int MPDelta = ItemInt * 2;
		int HPDelta = ItemCon * 2;

		unsigned int mhp = MOB.CurrentScore.MaxHp + HPDelta;
		unsigned int mmp = MOB.CurrentScore.MaxMp + MPDelta;


		MOB.CurrentScore.MaxHp += mhp;
		MOB.CurrentScore.MaxMp += mmp;
#pragma endregion
	}

	int special0 = MOB.CurrentScore.Special[0] + BASE_GetMobAbility(&MOB, EF_SPECIAL1);

	if (special0 > 200)
		special0 = 200;

	int special1 = MOB.CurrentScore.Special[1] + BASE_GetMobAbility(&MOB, EF_SPECIAL2) + BASE_GetMobAbility(&MOB, EF_SPECIALALL);

	if (special1 > 255)
		special1 = 255;

	int special2 = MOB.CurrentScore.Special[2] + BASE_GetMobAbility(&MOB, EF_SPECIAL3) + BASE_GetMobAbility(&MOB, EF_SPECIALALL);

	if (special2 > 255)
		special2 = 255;

	int special3 = MOB.CurrentScore.Special[3] + BASE_GetMobAbility(&MOB, EF_SPECIAL4) + BASE_GetMobAbility(&MOB, EF_SPECIALALL);

	if (special3 > 255)
		special3 = 255;

	MOB.CurrentScore.Special[0] = special0;
	MOB.CurrentScore.Special[1] = special1;
	MOB.CurrentScore.Special[2] = special2;
	MOB.CurrentScore.Special[3] = special3;

	MOB.SaveMana = BASE_GetMobAbility(&MOB, EF_SAVEMANA);

	int magic = BASE_GetMobAbility(&MOB, EF_MAGIC) + BASE_GetMobAbility(&MOB, EF_MAGICADD);
	magic = (magic + 1) / 4;

	int Run = MOB.CurrentScore.AttackRun & 15;
	Run += BASE_GetMobAbility(&MOB, EF_RUNSPEED);

	int Att = MOB.BaseScore.AttackRun / 16 * 10;
	Att += BASE_GetMobAbility(&MOB, EF_ATTSPEED);

	if (Run > 6)
		Run = 6;

	int RegenHP = BASE_GetMobAbility(&MOB, EF_REGENHP);
	int RegenMP = BASE_GetMobAbility(&MOB, EF_REGENMP);

	int Critical = (BASE_GetMobAbility(&MOB, EF_CRITICAL) / 4);

	int face = MOB.Equip[0].sIndex / 10;

	if (face < 4)
	{
		MOB.Equip[0].stEffect[0].cEffect = 43;
		MOB.Equip[0].stEffect[0].cValue = 0;

		if (MOB.Clan == 7 || MOB.Clan == 8 || MOB.Clan == 9)
			MOB.Clan = 0;
	}

	int mantle = MOB.Equip[15].sIndex;

	if (MOB.Clan != 4 && (mantle == 543 || mantle == 545))
		MOB.Clan = 7;

	if (MOB.Clan != 4 && (mantle == 544 || mantle == 546))
		MOB.Clan = 8;

	if (MOB.Clan != 4 && (mantle == 734 || mantle == 736))
		MOB.Clan = 7;

	if (MOB.Clan != 4 && (mantle == 735 || mantle == 737))
		MOB.Clan = 8;

	if (MOB.Clan != 4 && (mantle == 3191 || mantle == 3194))
		MOB.Clan = 7;

	if (MOB.Clan != 4 && (mantle == 3192 || mantle == 3195))
		MOB.Clan = 8;

	if (MOB.Clan != 4 && (mantle == 3197))
		MOB.Clan = 7;

	if (MOB.Clan != 4 && (mantle == 3198))
		MOB.Clan = 8;

	// Balanceamento
	if (MOB.Class == 0) // TK
	{

		if (MOB.LearnedSkill & (1 << 7)) // Confiança
		{
			int ArmaUnique = g_pItemList[MOB.Equip[6].sIndex].nUnique;
			char ArmaPos = g_pItemList[MOB.Equip[6].sIndex].nPos;

			if (ArmaUnique == 43) // Garra
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.38) + (MOB.CurrentScore.Str * 0.42));
			}

			if (ArmaUnique == 42) // Arco
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.55) + (MOB.CurrentScore.Str * 0.60));
			}

			if (ArmaUnique == 46) // Hermai
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.36) + (MOB.CurrentScore.Str * 0.40));
			}

			if (ArmaUnique == 48) // Espada 2 mãos
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.56) + (MOB.CurrentScore.Str * 0.60));
			}

			if (ArmaUnique == 49) // Martelo
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.43) + (MOB.CurrentScore.Str * 0.47));
			}

			if (ArmaUnique == 45) // Caliburn
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.22) + (MOB.CurrentScore.Str * 0.27));
			}

			if (ArmaUnique == 41) // Balmung
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.22) + (MOB.CurrentScore.Str * 0.27));
			}

			if (ArmaUnique == 44) // Lança
			{
				magic = magic + (((MOB.CurrentScore.Dex * 2.0) + (MOB.CurrentScore.Int * 2.5)) / 100);
			}

			if (ArmaUnique == 47) // Cajado e Cetro
			{
				magic = magic + (((MOB.CurrentScore.Dex * 1.2) + (MOB.CurrentScore.Int * 1.5)) / 100);
			}

			if (ArmaPos == 64) // Cajado
			{
				magic = magic + (((MOB.CurrentScore.Dex * 2.0) + (MOB.CurrentScore.Int * 2.5)) / 100);
			}

		}

		if (MOB.LearnedSkill & (1 << 15)) // Trans
		{
			int ArmaUnique = g_pItemList[MOB.Equip[6].sIndex].nUnique;
			char ArmaPos = g_pItemList[MOB.Equip[6].sIndex].nPos;

			if (ArmaUnique == 43) // Garra
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.38) + (MOB.CurrentScore.Str * 0.42));
			}

			if (ArmaUnique == 42) // Arco
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.55) + (MOB.CurrentScore.Str * 0.60));
			}

			if (ArmaUnique == 46) // Hermai
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.36) + (MOB.CurrentScore.Str * 0.40));
			}

			if (ArmaUnique == 48) // Espada 2 mãos
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.56) + (MOB.CurrentScore.Str * 0.60));
			}

			if (ArmaUnique == 49) // Martelo
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.43) + (MOB.CurrentScore.Str * 0.47));
			}

			if (ArmaUnique == 45) // Caliburn
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.22) + (MOB.CurrentScore.Str * 0.27));
			}

			if (ArmaUnique == 41) // Balmung
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.22) + (MOB.CurrentScore.Str * 0.27));
			}

			if (ArmaUnique == 44) // Lança
			{
				magic = magic + (((MOB.CurrentScore.Dex * 2.0) + (MOB.CurrentScore.Int * 2.5)) / 100);
			}

			if (ArmaUnique == 47) // Cajado e Cetro
			{
				magic = magic + (((MOB.CurrentScore.Dex * 1.2) + (MOB.CurrentScore.Int * 1.5)) / 100);
			}

			if (ArmaPos == 64) // Cajado
			{
				magic = magic + (((MOB.CurrentScore.Dex * 2.0) + (MOB.CurrentScore.Int * 2.5)) / 100);
			}

			
			// add crítico e def
			int criticaladd = ((special3 + 1) / 10) + (MOB.CurrentScore.Dex / 75);

			if (criticaladd < 4)
				criticaladd = 4;

			Critical += criticaladd; 
				
			int totalac = ((MOB.CurrentScore.Ac / 100) * 10) + MOB.CurrentScore.Ac;

			MOB.CurrentScore.Ac = totalac;			
		}

		if (MOB.LearnedSkill & (1 << 23)) // Espada Mágica
		{
			int ArmaUnique = g_pItemList[MOB.Equip[6].sIndex].nUnique;
			char ArmaPos = g_pItemList[MOB.Equip[6].sIndex].nPos;

			if (ArmaUnique == 43) // Garra
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.38) + (MOB.CurrentScore.Str * 0.42));
			}

			if (ArmaUnique == 42) // Arco
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.55) + (MOB.CurrentScore.Str * 0.60));
			}

			if (ArmaUnique == 46) // Hermai
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.36) + (MOB.CurrentScore.Str * 0.40));
			}

			if (ArmaUnique == 48) // Espada 2 mãos
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.56) + (MOB.CurrentScore.Str * 0.60));
			}

			if (ArmaUnique == 49) // Martelo
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.43) + (MOB.CurrentScore.Str * 0.47));
			}

			if (ArmaUnique == 45) // Caliburn
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.22) + (MOB.CurrentScore.Str * 0.27));
			}

			if (ArmaUnique == 41) // Balmung
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.22) + (MOB.CurrentScore.Str * 0.27));
			}

			if (ArmaUnique == 44) // Lança
			{
				magic = magic + (((MOB.CurrentScore.Dex * 2.0) + (MOB.CurrentScore.Int * 2.5)) / 100);
			}

			if (ArmaUnique == 47) // Cajado e Cetro
			{
				magic = magic + (((MOB.CurrentScore.Dex * 1.2) + (MOB.CurrentScore.Int * 1.5)) / 100);
			}

			if (ArmaPos == 64) // Cajado
			{
				magic = magic + (((MOB.CurrentScore.Dex * 2.0) + (MOB.CurrentScore.Int * 2.5)) / 100);
			}
		}

		/*if (MOB.LearnedSkill & (1 << 14)) 
		// Noção de combate
		{
			MOB.CurrentScore.Dex = ((MOB.CurrentScore.Dex * 105) / 100);
			MOB.CurrentScore.Str = ((MOB.CurrentScore.Str * 115) / 100);
		}

		// Noção de combate
		if (MOB.LearnedSkill & (1 << 14))
		{
			int addtk = ((MOB.CurrentScore.Damage / 100) * 10) + MOB.CurrentScore.Damage;
			MOB.CurrentScore.Damage = addtk;
		}*/

	}

	if (MOB.Class == 1) // FM
	{

		if (MOB.LearnedSkill & (1 << 7)) // Blood
		{
			int ArmaUnique = g_pItemList[MOB.Equip[6].sIndex].nUnique;
			char ArmaPos = g_pItemList[MOB.Equip[6].sIndex].nPos;

			if (ArmaUnique == 43) // Garra
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.50) + (MOB.CurrentScore.Str * 0.55));
			}

			if (ArmaUnique == 42) // Arco
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.75) + (MOB.CurrentScore.Str * 0.80));
			}

			if (ArmaUnique == 46) // Hermai
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.40) + (MOB.CurrentScore.Str *0.45));
			}

			if (ArmaUnique == 48) // Espada 2 mãos
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.50) + (MOB.CurrentScore.Str * 0.55));
			}

			if (ArmaUnique == 49) // Martelo
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.60) + (MOB.CurrentScore.Str * 0.65));
			}

			if (ArmaUnique == 45) // Caliburn
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.55) + (MOB.CurrentScore.Str * 0.60));
			}

			if (ArmaUnique == 41) // Balmung
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.50) + (MOB.CurrentScore.Str * 0.55));
			}

			if (ArmaUnique == 44) // Lança
			{
				magic = magic + (((MOB.CurrentScore.Dex * 6.3) + (MOB.CurrentScore.Int * 4.2)) / 100);
			}

			if (ArmaUnique == 47) // Cajado e Cetro
			{
				magic = magic + (((MOB.CurrentScore.Dex * 6.3) + (MOB.CurrentScore.Int * 4.2)) / 100);
			}

			if (ArmaPos == 64) // Cajado
			{
				magic = magic + (((MOB.CurrentScore.Dex * 6.3) + (MOB.CurrentScore.Int * 4.2)) / 100);
			}
		}

		if (MOB.LearnedSkill & (1 << 15)) // 8ª Black
		{
			int ArmaUnique = g_pItemList[MOB.Equip[6].sIndex].nUnique;
			char ArmaPos = g_pItemList[MOB.Equip[6].sIndex].nPos;

			if (ArmaUnique == 43) // Garra
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.50) + (MOB.CurrentScore.Str * 0.55));
			}

			if (ArmaUnique == 42) // Arco
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.75) + (MOB.CurrentScore.Str * 0.80));
			}

			if (ArmaUnique == 46) // Hermai
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.40) + (MOB.CurrentScore.Str * 0.45));
			}

			if (ArmaUnique == 48) // Espada 2 mãos
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.50) + (MOB.CurrentScore.Str * 0.55));
			}

			if (ArmaUnique == 49) // Martelo
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.60) + (MOB.CurrentScore.Str * 0.65));
			}

			if (ArmaUnique == 45) // Caliburn
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.55) + (MOB.CurrentScore.Str * 0.60));
			}

			if (ArmaUnique == 41) // Balmung
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.50) + (MOB.CurrentScore.Str * 0.55));
			}

			if (ArmaUnique == 44) // Lança
			{
				magic = magic + (((MOB.CurrentScore.Dex * 6.3) + (MOB.CurrentScore.Int * 4.2)) / 100);
			}

			if (ArmaUnique == 47) // Cajado e Cetro
			{
				magic = magic + (((MOB.CurrentScore.Dex * 6.3) + (MOB.CurrentScore.Int * 4.2)) / 100);
			}

			if (ArmaPos == 64) // Cajado
			{
				magic = magic + (((MOB.CurrentScore.Dex * 6.3) + (MOB.CurrentScore.Int * 4.2)) / 100);
			}
		}

		if (MOB.LearnedSkill & (1 << 23)) // 8ª Magia Especial
		{
			int ArmaUnique = g_pItemList[MOB.Equip[6].sIndex].nUnique;
			char ArmaPos = g_pItemList[MOB.Equip[6].sIndex].nPos;

			if (ArmaUnique == 43) // Garra
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.50) + (MOB.CurrentScore.Str * 0.55));
			}

			if (ArmaUnique == 42) // Arco
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.75) + (MOB.CurrentScore.Str * 0.80));
			}

			if (ArmaUnique == 46) // Hermai
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.40) + (MOB.CurrentScore.Str * 0.45));
			}

			if (ArmaUnique == 48) // Espada 2 mãos
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.50) + (MOB.CurrentScore.Str * 0.55));
			}

			if (ArmaUnique == 49) // Martelo
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.60) + (MOB.CurrentScore.Str * 0.65));
			}

			if (ArmaUnique == 45) // Caliburn
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.55) + (MOB.CurrentScore.Str * 0.60));
			}

			if (ArmaUnique == 41) // Balmung
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.50) + (MOB.CurrentScore.Str * 0.55));
			}

			if (ArmaUnique == 44) // Lança
			{
				magic = magic + (((MOB.CurrentScore.Dex * 6.3) + (MOB.CurrentScore.Int * 4.2)) / 100);
			}

			if (ArmaUnique == 47) // Cajado e Cetro
			{
				magic = magic + (((MOB.CurrentScore.Dex * 6.3) + (MOB.CurrentScore.Int * 4.2)) / 100);
			}

			if (ArmaPos == 64) // Cajado
			{
				magic = magic + (((MOB.CurrentScore.Dex * 6.3) + (MOB.CurrentScore.Int * 4.2)) / 100);
			}
		}

	}

	if (MOB.Class == 2) //BM
	{

		if (MOB.LearnedSkill & (1 << 7)) // BM Elemental
		{
			int ArmaUnique = g_pItemList[MOB.Equip[6].sIndex].nUnique;
			char ArmaPos = g_pItemList[MOB.Equip[6].sIndex].nPos;

			if (ArmaUnique == 43) // Garra
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.42) + (MOB.CurrentScore.Str * 0.46));
			}

			if (ArmaUnique == 42) // Arco
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.68) + (MOB.CurrentScore.Str * 0.72));
			}

			if (ArmaUnique == 46) // Hermai
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.44) + (MOB.CurrentScore.Str * 0.48));
			}

			if (ArmaUnique == 48) // Espada 2 mãos
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.41) + (MOB.CurrentScore.Str * 0.46));
			}

			if (ArmaUnique == 49) // Martelo
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.65) + (MOB.CurrentScore.Str * 0.70));
			}

			if (ArmaUnique == 45) // Caliburn
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.22) + (MOB.CurrentScore.Str * 0.32));
			}

			if (ArmaUnique == 41) // Balmung
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.22) + (MOB.CurrentScore.Str * 0.32));
			}

			if (ArmaUnique == 44) // Lança
			{
				magic = magic + (((MOB.CurrentScore.Dex * 6.5) + (MOB.CurrentScore.Int * 5.5)) / 100);
			}

			if (ArmaUnique == 47) // Cajado e Cetro
			{
				magic = magic + (((MOB.CurrentScore.Dex * 6.5) + (MOB.CurrentScore.Int * 5.5)) / 100);
			}

			if (ArmaPos == 64) // Cajado
			{
				magic = magic + (((MOB.CurrentScore.Dex * 5.5) + (MOB.CurrentScore.Int * 4.5)) / 100);
			}
		}

		if (MOB.LearnedSkill & (1 << 15)) // BM Evocações
		{
			int ArmaUnique = g_pItemList[MOB.Equip[6].sIndex].nUnique;
			char ArmaPos = g_pItemList[MOB.Equip[6].sIndex].nPos;

			if (ArmaUnique == 43) // Garra
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.42) + (MOB.CurrentScore.Str * 0.46));
			}

			if (ArmaUnique == 42) // Arco
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.68) + (MOB.CurrentScore.Str * 0.72));
			}

			if (ArmaUnique == 46) // Hermai
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.44) + (MOB.CurrentScore.Str * 0.48));
			}

			if (ArmaUnique == 48) // Espada 2 mãos
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.41) + (MOB.CurrentScore.Str * 0.46));
			}

			if (ArmaUnique == 49) // Martelo
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.65) + (MOB.CurrentScore.Str * 0.70));
			}

			if (ArmaUnique == 45) // Caliburn
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.22) + (MOB.CurrentScore.Str * 0.32));
			}

			if (ArmaUnique == 41) // Balmung
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.22) + (MOB.CurrentScore.Str * 0.32));
			}

			if (ArmaUnique == 44) // Lança
			{
				magic = magic + (((MOB.CurrentScore.Dex * 6.5) + (MOB.CurrentScore.Int * 5.5)) / 100);
			}

			if (ArmaUnique == 47) // Cajado e Cetro
			{
				magic = magic + (((MOB.CurrentScore.Dex * 6.5) + (MOB.CurrentScore.Int * 5.5)) / 100);
			}

			if (ArmaPos == 64) // Cajado
			{
				magic = magic + (((MOB.CurrentScore.Dex * 5.5) + (MOB.CurrentScore.Int * 4.5)) / 100);
			}
		}

		if (MOB.LearnedSkill & (1 << 23)) // BM Natureza
		{
			int ArmaUnique = g_pItemList[MOB.Equip[6].sIndex].nUnique;
			char ArmaPos = g_pItemList[MOB.Equip[6].sIndex].nPos;

			if (ArmaUnique == 43) // Garra
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.42) + (MOB.CurrentScore.Str * 0.46));
			}

			if (ArmaUnique == 42) // Arco
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.68) + (MOB.CurrentScore.Str * 0.72));
			}

			if (ArmaUnique == 46) // Hermai
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.44) + (MOB.CurrentScore.Str * 0.48));
			}

			if (ArmaUnique == 48) // Espada 2 mãos
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.41) + (MOB.CurrentScore.Str * 0.46));
			}

			if (ArmaUnique == 49) // Martelo
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.65) + (MOB.CurrentScore.Str * 0.70));
			}

			if (ArmaUnique == 45) // Caliburn
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.22) + (MOB.CurrentScore.Str * 0.32));
			}

			if (ArmaUnique == 41) // Balmung
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.22) + (MOB.CurrentScore.Str * 0.32));
			}

			if (ArmaUnique == 44) // Lança
			{
				magic = magic + (((MOB.CurrentScore.Dex * 6.5) + (MOB.CurrentScore.Int * 5.5)) / 100);
			}

			if (ArmaUnique == 47) // Cajado e Cetro
			{
				magic = magic + (((MOB.CurrentScore.Dex * 6.5) + (MOB.CurrentScore.Int * 5.5)) / 100);
			}

			if (ArmaPos == 64) // Cajado
			{
				magic = magic + (((MOB.CurrentScore.Dex * 5.5) + (MOB.CurrentScore.Int * 4.5)) / 100);
			}
		}
	}

	if (MOB.Class == 3) // HT
	{
		//Agressividade
		if (MOB.LearnedSkill & (1 << 2))
		{
			int ArmaUnique = g_pItemList[MOB.Equip[6].sIndex].nUnique;

			if (ArmaUnique == 43) // Garra
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.49) + (MOB.CurrentScore.Str * 0.53));
			}

			if (ArmaUnique == 42) // Arco
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.68) + (MOB.CurrentScore.Str * 0.72));
			}

			if (ArmaUnique == 46) // Hermai
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.53) + (MOB.CurrentScore.Str * 0.57));
			}

			if (ArmaUnique == 48) // Espada 2 mãos
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.51) + (MOB.CurrentScore.Str * 0.55));
			}

			if (ArmaUnique == 49) // Martelo
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.47) + (MOB.CurrentScore.Str * 0.51));
			}

			if (ArmaUnique == 45) // Caliburn
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.27) + (MOB.CurrentScore.Str * 0.37));
			}

			if (ArmaUnique == 41) // Balmung
			{
				MOB.CurrentScore.Damage = MOB.CurrentScore.Damage + ((MOB.CurrentScore.Dex * 0.27) + (MOB.CurrentScore.Str * 0.37));
			}

			if (ArmaUnique == 44) // Lança
			{
				magic = magic + (((MOB.CurrentScore.Dex * 40.0) + (MOB.CurrentScore.Int * 20.0)) / 100);
			}

			if (ArmaUnique == 47) // Cajado e Cetro
			{
				magic = magic + (((MOB.CurrentScore.Dex * 40.0) + (MOB.CurrentScore.Int * 20.0)) / 100);
			}

		}

		//Tempestade de Raios Bonus em Dano
		if (MOB.LearnedSkill & (1 << 7))
		{
			MOB.CurrentScore.Damage += 200;
		}

		//Visão do Caçador
		if (MOB.LearnedSkill & (1 << 18))
		{
			int criticaladd = ((special3 + 1) / 10) + (MOB.CurrentScore.Dex / 75);

			if (criticaladd < 4)
				criticaladd = 4;

			Critical += criticaladd;
		}

		//Proteção das sombras
		if (MOB.LearnedSkill & (1 << 23)) // 22
		{
			int sombraadd = (special3 / 3) + 10;
			MOB.CurrentScore.Ac += sombraadd;
		}
	}

	/*if (extra->SecLearnedSkill & 0x100) // Espelho Mágico
	{
		int calculo = (MOB.CurrentScore.Special[3]) / 10;

		int totalmp = ((MOB.CurrentScore.MaxMp / 100) * calculo) + MOB.CurrentScore.MaxMp;

		if (totalmp > MAX_MP)
			MOB.CurrentScore.MaxMp = MAX_MP;
		else
			MOB.CurrentScore.MaxMp = totalmp;
	}*/

	int HpAdd = BASE_GetMobAbility(&MOB, EF_HPADD) + BASE_GetMobAbility(&MOB, EF_HPADD2) + 100;

	long long curHP = MOB.CurrentScore.MaxHp;
	long long maxHP = curHP * HpAdd / 100;

	if (maxHP >= MAX_HP)
		MOB.CurrentScore.MaxHp = MAX_HP;

	else
		MOB.CurrentScore.MaxHp = (int)maxHP;

	int MpAdd = BASE_GetMobAbility(&MOB, EF_MPADD) + BASE_GetMobAbility(&MOB, EF_MPADD2) + 100;

	long long curMP = MOB.CurrentScore.MaxMp;
	long long maxMP = curMP * MpAdd / 100;

	if (maxMP >= MAX_MP)
		MOB.CurrentScore.MaxMp = MAX_MP;

	else
		MOB.CurrentScore.MaxMp = (int)maxMP;

	if ((MOB.Equip[0].sIndex >= 20 && MOB.Equip[0].sIndex < 30 || MOB.Equip[0].sIndex == 32))
		MOB.Equip[0].sIndex = extra->ClassMaster == MORTAL ? extra->MortalFace : extra->MortalFace + 5 + MOB.Class;

	/*if ((MOB.Equip[0].sIndex == 316 || MOB.Equip[0].sIndex == 317 || MOB.Equip[0].sIndex == 297 || MOB.Equip[0].sIndex == 202
		|| MOB.Equip[0].sIndex == 209 || MOB.Equip[0].sIndex == 212 || MOB.Equip[0].sIndex == 356 || MOB.Equip[0].sIndex == 228))
		MOB.Equip[0].sIndex = Extra->ClassMaster == MORTAL ? Extra->MortalFace : Extra->MortalFace + 5 + MOB.Class;*/

	int AttackSpeedBonus = 0;
	int RunSpeedBonus = 0;
	int DAMAGEMULTI = 100;

	int Sagrado = MOB.Resist[0];
	int Trovao = MOB.Resist[1];
	int Fogo = MOB.Resist[2];
	int Gelo = MOB.Resist[3];

#pragma region Buff Loop
	for (int i = 0; i < MAX_AFFECT; i++)
	{
		if (Affect == 0)
			continue;

		int Type = Affect[i].Type;

		if (Type == 0)
			continue;

		int Value = Affect[i].Value;
		int Level = Affect[i].Level;

		if (Type == 1) // Toque Sagrado (Lentidão)
		{
			Run -= Value;
			int tmp = 30;
			Att -= tmp;
			if (MOB.Equip[0].sIndex > 50)
			{
				int tempint = MOB.CurrentScore.Int;
				tempint -= tmp + 10;
				MOB.CurrentScore.Int = tempint;
			}
		}
		else if (Type == 2) // Velocidade
		{
			Run += Value;
			MOB.Rsv |= RSV_HASTE;
		}
		else if (Type == 3) // Perseguição (Diminuir imunidade)
		{
			int val1 = 0;
			int tval = Value;

			if (MOB.Equip[0].sIndex < 50)
				tval /= 2;
			else
				tval -= 10;

			Sagrado -= tval;
			Fogo -= tval;
			Trovao -= tval;
			Gelo -= tval;
		}
		else if (Type == 4) // Poder Superior
		{
			DAMAGEMULTI += 4;
			MOB.CurrentScore.Damage += 30;
			magic += 5;
		}
		else if (Type == 5)// Fanatismo	
		{
			float fValue = (100 - Value) / 100.0f;
			MOB.CurrentScore.Dex = (short)(MOB.CurrentScore.Dex * fValue);
		}
		else if (Type == 6)
		{
			float fValue = (Value + 100) / 100.0f;
			MOB.CurrentScore.Dex = (short)(MOB.CurrentScore.Dex * fValue);
		}
		else if (Type == 7) // Lâmina Congelada
		{
			int tmp = Level / 10 + 10;
			Att -= tmp;

			if (MOB.Equip[0].sIndex > 50)
			{
				int tempint = MOB.CurrentScore.Int;
				tempint -= (tmp + 10);
				MOB.CurrentScore.Int = tempint;
			}
		}

		else if (Type == 9) // Arma mágica
		{
			int add = Level * 5 / 20 + Value;
			add = add * 3 / 2;

			DAMAGEMULTI += 5;

			if (MOB.Class == 1)
			{
				if (MOB.LearnedSkill & 0x80000)
				{
					add *= 3;
					DAMAGEMULTI += 10;
				}
			}

			MOB.CurrentScore.Damage += add;
			magic += 5;
		}
		else if (Type == 10) // Enfraquecer
		{
			int sub = Level / 5 + Value;

			MOB.CurrentScore.Damage -= sub;
		}
		else if (Type == 11) // Escudo Mágico
		{
			int add = Level / 3 + Value;

			MOB.CurrentScore.Ac += add;
		}
		else if (Type == 12)	
		{
			float fvalue = (100 - Value) / 100.0f;

			MOB.CurrentScore.Ac = (int)(MOB.CurrentScore.Ac * fvalue);
		}
		else if (Type == 13) // Assalto
		{
			int tv = Level / 10 + Value;
			int totaldn = ((MOB.CurrentScore.Damage / 100) * 15) + MOB.CurrentScore.Damage; // 32 == 20k

			if (totaldn >= MAX_DAMAGE)
				MOB.CurrentScore.Damage = MAX_DAMAGE;
			else
				MOB.CurrentScore.Damage = totaldn;

			DAMAGEMULTI += tv;

			tv = MOB.CurrentScore.MaxHp;
			tv = tv * 9 / 10;

			MOB.CurrentScore.MaxHp = tv;
		}
		else if (Type == 14)// Possuído
		{
			int value = Level * 3 / 4 + Value;

			if (extra->ClassMaster != MORTAL && extra->ClassMaster != ARCH)
				value *= 3;

			MOB.CurrentScore.MaxHp += value * 22; // (106k)

			int tv = MOB.CurrentScore.Con + value;

			MOB.CurrentScore.Con += tv * 125 / 100; // (5004)
		}

		else if (Type == 15)//Skill
		{
			int value = Level / 10 + Value;

			int tv = MOB.CurrentScore.Special[0] + value;

			if (tv > 200)
				tv = 200;

			MOB.CurrentScore.Special[0] = tv;

			tv = MOB.CurrentScore.Special[1] + value;

			if (tv > 255)
				tv = 255;

			MOB.CurrentScore.Special[1] = tv;

			tv = MOB.CurrentScore.Special[2] + value;

			if (tv > 255)
				tv = 255;

			MOB.CurrentScore.Special[2] = tv;

			tv = MOB.CurrentScore.Special[3] + value;

			if (tv > 255)
				tv = 255;

			MOB.CurrentScore.Special[3] = tv;
		}
		else if (Type == 16) // Transformações
		{
			int value = Value - 1;

			if (value < 0 || value >= 5 || MOB.Class != 2)
				continue;

			MOB.Equip[0].sIndex = value == 4 ? 32 : value + 22;

			int DamAdd = 0;
			int HpAdd = 0;
			int AcAdd = 0;
			int AttAdd = 0;
			int IsWolf = MOB.LearnedSkill & 0x20000;
			int IsBear = MOB.LearnedSkill & 0x80000;
			int IsAsta = MOB.LearnedSkill & 0x200000;
			int RegAdd = 0;

			if (MOB.Equip[0].sIndex == 22 && IsWolf != 0) // Lobo
			{
				DamAdd = 10;// 10 Damage
			   //RegAdd += 15;
				Critical += 5;// 5 Crítico
				HpAdd = 5;// 0 HP Add
				AcAdd = 3;// 0 Defesa
			}
			else if (MOB.Equip[0].sIndex == 23 && IsBear != 0) // Urso
			{
				HpAdd = 15;// 10 HP Add
				//AcAdd = 7; // New 7
				RegAdd += 40;// 20 HP Regen
				AttAdd += 40;// 20 Velocidade de ataque
			}
			else if (MOB.Equip[0].sIndex == 24 && IsAsta != 0) // Astaroth
			{
				DamAdd = 10;// 10 Damage
				AcAdd = 5;// 0 Defesa (5)
				HpAdd = 5;// 5 HP Add
				RegAdd += 15;// 15 HP Regen
				AttAdd += 20;// 20 Velocidade de Ataque
				Critical += 5;// 0 Crítico
			}
			else if (MOB.Equip[0].sIndex == 25) // Titã
			{
				AttAdd += 30;// 30 Velocidade de Ataque
				HpAdd = 5;// 10 HP Add
				AcAdd = 10;// 0 Defesa
			}
			else if (MOB.Equip[0].sIndex == 32) // Éden
			{
				DamAdd = 10;// 10 Damage
				AcAdd = 5;// 5 Defesa
				HpAdd = 10;// 10 HP Add
				RegAdd += 10;// 10 HP Regen
				AttAdd += 20;// 20 Velocidade de ataque
				Critical += 6;// 0 Crítico
			}

			int sanc = (special3 + (extra->ClassMaster != ARCH && extra->ClassMaster != MORTAL ? MOB.CurrentScore.Level + MAX_LEVEL : MOB.CurrentScore.Level) * 2) / 3;
			sanc = (sanc - pTransBonus[value].Unk17) / 12;

			if (sanc <= 0)
				sanc = 0;

			if (sanc > 9)
				sanc = 9;

			MOB.Equip[0].stEffect[0].cEffect = EF_SANC;
			MOB.Equip[0].stEffect[0].cValue = sanc;

			int min = DamAdd + pTransBonus[value].Unk;
			int max = DamAdd + pTransBonus[value].Unk2;
			int delta = max - min;
			int multi = delta * Level / 200 + min;
			int tv = MOB.CurrentScore.Damage;

			if (MOB.Equip[0].sIndex == 22)
				tv += 10;

			MOB.CurrentScore.Damage = tv;

			DAMAGEMULTI = DAMAGEMULTI + multi - 100;

			min = AcAdd + pTransBonus[value].Unk3;
			max = AcAdd + pTransBonus[value].Unk4;
			delta = max - min;
			multi = delta * Level / 200 + min;
			tv = MOB.CurrentScore.Ac;
			tv = tv * multi / 100;

			if (MOB.Equip[0].sIndex == 22)
				tv += 5;

			MOB.CurrentScore.Ac = tv;

			min = HpAdd + pTransBonus[value].Unk5;
			max = HpAdd + pTransBonus[value].Unk6;
			delta = max - min;
			multi = delta * Level / 200 + min;
			tv = MOB.CurrentScore.MaxHp;
			tv = tv * multi / 100;
			MOB.CurrentScore.MaxHp = tv;

			Sagrado += RegAdd;
			Fogo += RegAdd;
			Trovao += RegAdd;
			Gelo += RegAdd;

			AttackSpeedBonus = pTransBonus[value].Unk9 + AttAdd;
			RunSpeedBonus = pTransBonus[value].Unk7;
		}

		else if(Type == 19) // Imunidade
			MOB.Rsv |= RSV_BLOCK;

		else if(Type == 21) // Meditação
		{
			int add = Level / 3 + 10;

			MOB.CurrentScore.Ac -= add;

			int tv = Level / 10 + Value;

			DAMAGEMULTI += tv;
		}

		else if(Type == 24) // Samaritano
		{
		int add = MOB.CurrentScore.Ac / 4 + Value;

		MOB.CurrentScore.Ac += add;
		}

		else if(Type == 25) // // Proteção Elemental
		{
			int add = (Value + Level / 4) / 10;

			if (Level >= 255)
				add += 20;

			Fogo += add;
			Trovao += add;
			Gelo += add;
		}

		else if (Type == 29 && extra->ClassMaster != MORTAL)//Soul
		{
			if (extra->Soul == SOUL_F)
				MOB.CurrentScore.Str = (int)(MOB.CurrentScore.Str * 2.2f);

			else if (extra->Soul == SOUL_I)
				MOB.CurrentScore.Int = (int)(MOB.CurrentScore.Int * 2.2f);

			else if (extra->Soul == SOUL_D)
				MOB.CurrentScore.Dex = (int)(MOB.CurrentScore.Dex * 2.2f);

			else if (extra->Soul == SOUL_C)
				MOB.CurrentScore.Con = (int)(MOB.CurrentScore.Con * 2.2f);
			
			else if (extra->Soul == SOUL_FI)
			{
				MOB.CurrentScore.Str = (int)(MOB.CurrentScore.Str * 1.80f);
				MOB.CurrentScore.Int = (int)(MOB.CurrentScore.Int * 1.40f);
			}

			else if (extra->Soul == SOUL_FD)
			{
				MOB.CurrentScore.Str = (int)(MOB.CurrentScore.Str * 1.80f);
				MOB.CurrentScore.Dex = (int)(MOB.CurrentScore.Dex * 1.40f);
			}

			else if (extra->Soul == SOUL_FC)
			{
				MOB.CurrentScore.Str = (int)(MOB.CurrentScore.Str * 1.80f);
				MOB.CurrentScore.Con = (int)(MOB.CurrentScore.Con * 1.40f);
			}

			else if (extra->Soul == SOUL_IF)
			{
				MOB.CurrentScore.Int = (int)(MOB.CurrentScore.Int * 1.80f);
				MOB.CurrentScore.Str = (int)(MOB.CurrentScore.Str * 1.40f);
			}

			else if (extra->Soul == SOUL_ID)
			{
				MOB.CurrentScore.Int = (int)(MOB.CurrentScore.Int * 1.80f);
				MOB.CurrentScore.Dex = (int)(MOB.CurrentScore.Dex * 1.40f);
			}

			else if (extra->Soul == SOUL_IC)
			{
				MOB.CurrentScore.Int = (int)(MOB.CurrentScore.Int * 1.80f);
				MOB.CurrentScore.Con = (int)(MOB.CurrentScore.Con * 1.40f);
			}

			else if (extra->Soul == SOUL_DF)
			{
				MOB.CurrentScore.Dex = (int)(MOB.CurrentScore.Dex * 1.80f);
				MOB.CurrentScore.Str = (int)(MOB.CurrentScore.Str * 1.40f);
			}

			else if (extra->Soul == SOUL_DI)
			{
				MOB.CurrentScore.Dex = (int)(MOB.CurrentScore.Dex * 1.80f);
				MOB.CurrentScore.Int = (int)(MOB.CurrentScore.Int * 1.40f);
			}

			else if (extra->Soul == SOUL_DC)
			{
				MOB.CurrentScore.Dex = (int)(MOB.CurrentScore.Dex * 1.80f);
				MOB.CurrentScore.Con = (int)(MOB.CurrentScore.Con * 1.40f);
			}

			else if (extra->Soul == SOUL_CF)
			{
				MOB.CurrentScore.Con = (int)(MOB.CurrentScore.Con * 1.80f);
				MOB.CurrentScore.Str = (int)(MOB.CurrentScore.Str * 1.40f);
			}

			else if (extra->Soul == SOUL_CI)
			{
				MOB.CurrentScore.Con = (int)(MOB.CurrentScore.Con * 1.80f);
				MOB.CurrentScore.Int = (int)(MOB.CurrentScore.Int * 1.40f);
			}

			else if (extra->Soul == SOUL_CD)
			{
				MOB.CurrentScore.Con = (int)(MOB.CurrentScore.Con * 1.80f);
				MOB.CurrentScore.Dex = (int)(MOB.CurrentScore.Dex * 1.40f);
			}
		}

		else if (Type == 29 && extra->ClassMaster == MORTAL)//Soul
		{
			if (extra->Soul == SOUL_F)
				MOB.CurrentScore.Str = (int)(MOB.CurrentScore.Str * 1.8f);

			else if (extra->Soul == SOUL_I)
				MOB.CurrentScore.Int = (int)(MOB.CurrentScore.Int * 1.8f);

			else if (extra->Soul == SOUL_D)
				MOB.CurrentScore.Dex = (int)(MOB.CurrentScore.Dex * 1.8f);

			else if (extra->Soul == SOUL_C)
				MOB.CurrentScore.Con = (int)(MOB.CurrentScore.Con * 1.8f);
			
			else if (extra->Soul == SOUL_FI)
			{
				MOB.CurrentScore.Str = (int)(MOB.CurrentScore.Str * 1.60f);
				MOB.CurrentScore.Int = (int)(MOB.CurrentScore.Int * 1.20f);
			}

			else if (extra->Soul == SOUL_FD)
			{
				MOB.CurrentScore.Str = (int)(MOB.CurrentScore.Str * 1.60f);
				MOB.CurrentScore.Dex = (int)(MOB.CurrentScore.Dex * 1.20f);
			}

			else if (extra->Soul == SOUL_FC)
			{
				MOB.CurrentScore.Str = (int)(MOB.CurrentScore.Str * 1.60f);
				MOB.CurrentScore.Con = (int)(MOB.CurrentScore.Con * 1.20f);
			}

			else if (extra->Soul == SOUL_IF)
			{
				MOB.CurrentScore.Int = (int)(MOB.CurrentScore.Int * 1.60f);
				MOB.CurrentScore.Str = (int)(MOB.CurrentScore.Str * 1.20f);
			}

			else if (extra->Soul == SOUL_ID)
			{
				MOB.CurrentScore.Int = (int)(MOB.CurrentScore.Int * 1.60f);
				MOB.CurrentScore.Dex = (int)(MOB.CurrentScore.Dex * 1.20f);
			}

			else if (extra->Soul == SOUL_IC)
			{
				MOB.CurrentScore.Int = (int)(MOB.CurrentScore.Int * 1.60f);
				MOB.CurrentScore.Con = (int)(MOB.CurrentScore.Con * 1.20f);
			}

			else if (extra->Soul == SOUL_DF)
			{
				MOB.CurrentScore.Dex = (int)(MOB.CurrentScore.Dex * 1.60f);
				MOB.CurrentScore.Str = (int)(MOB.CurrentScore.Str * 1.20f);
			}

			else if (extra->Soul == SOUL_DI)
			{
				MOB.CurrentScore.Dex = (int)(MOB.CurrentScore.Dex * 1.60f);
				MOB.CurrentScore.Int = (int)(MOB.CurrentScore.Int * 1.20f);
			}

			else if (extra->Soul == SOUL_DC)
			{
				MOB.CurrentScore.Dex = (int)(MOB.CurrentScore.Dex * 1.60f);
				MOB.CurrentScore.Con = (int)(MOB.CurrentScore.Con * 1.20f);
			}

			else if (extra->Soul == SOUL_CF)
			{
				MOB.CurrentScore.Con = (int)(MOB.CurrentScore.Con * 1.60f);
				MOB.CurrentScore.Str = (int)(MOB.CurrentScore.Str * 1.20f);
			}

			else if (extra->Soul == SOUL_CI)
			{
				MOB.CurrentScore.Con = (int)(MOB.CurrentScore.Con * 1.60f);
				MOB.CurrentScore.Int = (int)(MOB.CurrentScore.Int * 1.20f);
			}

			else if (extra->Soul == SOUL_CD)
			{
				MOB.CurrentScore.Con = (int)(MOB.CurrentScore.Con * 1.60f);
				MOB.CurrentScore.Dex = (int)(MOB.CurrentScore.Dex * 1.20f); // 1.4
			}
		}

		else if (Type == 26) // Evasão aprimorada
		MOB.Rsv |= RSV_PARRY;

		else if (Type == 27)  // Encantar gelo && BASE_GetItemAbility(&MOB.Equip[6], EF_WTYPE) == 101
		MOB.Rsv |= RSV_FROST;

		else if (Type == 28) // Invisibilidade
		MOB.Rsv |= RSV_HIDE;

		else if (Type == 30) // Aumento de dano
		*ForceMobDamage += Level;

		else if (Type == 31) // Escudo Dourado
		{
		int add = Level / 2 + Value;

		MOB.CurrentScore.Ac += add;
		}

		else if (Type == 36) // Drenagem de HP (Veneno) Olhos de águia (?)  && BASE_GetItemAbility(&MOB.Equip[6], EF_WTYPE) == 41
		MOB.Rsv |= RSV_DRAIN;

		else if (Type == 37) // Ligação espectral
			*ForceDamage += special2;
		
		else if(Type == 38) // Troca de espíritos 
		{
			int mana = MOB.CurrentScore.MaxMp / 2;

			MOB.CurrentScore.MaxHp += mana;
			MOB.CurrentScore.MaxMp -= mana;
		}

		else if (Type == 39) // Baú de XP
			*ExpBonus += 100;

		else if (Type == 42)
		{
		int mana = MOB.CurrentScore.MaxMp / 5 + (((extra->ClassMaster != ARCH && extra->ClassMaster != MORTAL ? MOB.CurrentScore.Level + MAX_LEVEL : MOB.CurrentScore.Level) + Level) / 2);

		MOB.CurrentScore.MaxHp += mana;
		MOB.CurrentScore.MaxMp -= mana;
		}
	}
#pragma endregion


#pragma region Loop affect2
	for(int i = 0; i < MAX_AFFECT; i++)
	{
		if(Affect == 0)
			continue;

		int Type = Affect[i].Type;

		if(Type == 0)
			continue;

		int Value = Affect[i].Value;
		int Level = Affect[i].Level;

		if(Type == 8)
		{
			//Resistencia
			if((Level & (1 << 1)) != 0)
			{
				Sagrado += 25;
				Fogo += 25;
				Trovao += 25;
				Gelo += 25;
			}
			//Sagacidade
			if ((Level & (1 << 2)) != 0)
			{
				MOB.Rsv |= RSV_CAST;
			}
			//Abs
			if((Level & (1 << 3)) != 0)
				*AbsHp += 20;
			//+HP DEF
			if((Level & (1 << 4)) != 0)
			{
				int totalhp = ((MOB.CurrentScore.MaxHp / 100) * 10) + MOB.CurrentScore.MaxHp;

				if (totalhp >= MAX_HP)
					MOB.CurrentScore.MaxHp = MAX_HP;
				else
					MOB.CurrentScore.MaxHp = totalhp;

				int totalac = ((MOB.CurrentScore.Ac / 100) * 10) + MOB.CurrentScore.Ac;

				MOB.CurrentScore.Ac = totalac;
			}

			//+HP ATK
			if((Level & (1 << 5)) != 0)
			{
				int totalhp = ((MOB.CurrentScore.MaxHp / 100) * 10) + MOB.CurrentScore.MaxHp;

				if (totalhp >= MAX_HP)
					MOB.CurrentScore.MaxHp = MAX_HP;
				else
					MOB.CurrentScore.MaxHp = totalhp;

				int totaldam = ((MOB.CurrentScore.Damage / 100) * 10) + MOB.CurrentScore.Damage;

				if (totaldam >= MAX_DAMAGE)
					MOB.CurrentScore.Damage = MAX_DAMAGE;
				else
					MOB.CurrentScore.Damage = totaldam;

				int totalmg = ((MOB.Magic / 100) * 20) + MOB.Magic;

				if (totalmg >= MAX_DAMAGE_MG)
					MOB.Magic = MAX_DAMAGE_MG;
				else
					MOB.Magic = totalmg;
			}

			//Precisão
			if((Level & (1 << 6)) != 0)
				*Accuracy += 50;

			//MP = HP
			if((Level & (1 << 7)) != 0)
			{
				int mana = ((MOB.CurrentScore.MaxMp + 1) / 2);

				MOB.CurrentScore.MaxHp += mana;
				MOB.CurrentScore.MaxMp -= mana;
			}

		}
		else if (Type == 34)//Divina
		{
			int totalhp = ((MOB.CurrentScore.MaxHp / 100) * 20) + MOB.CurrentScore.MaxHp;

			if (totalhp >= MAX_HP)
				MOB.CurrentScore.MaxHp = MAX_HP;
			else
				MOB.CurrentScore.MaxHp = totalhp;

			int totalmp = ((MOB.CurrentScore.MaxMp / 100) * 20) + MOB.CurrentScore.MaxMp;

			if (totalmp >= MAX_MP)
				MOB.CurrentScore.MaxMp = MAX_MP;
			else
				MOB.CurrentScore.MaxMp = totalmp;

			int totaldn = ((MOB.CurrentScore.Damage / 100) * 20) + MOB.CurrentScore.Damage;

			if (totaldn >= MAX_DAMAGE)
				MOB.CurrentScore.Damage = MAX_DAMAGE;
			else
				MOB.CurrentScore.Damage = totaldn;

			int totalmg = ((MOB.Magic / 100) * 20) + MOB.Magic;

			if (totalmg >= MAX_DAMAGE_MG)
				MOB.Magic = MAX_DAMAGE_MG;
			else
				MOB.Magic = totalmg;
		}

		else if (Type == 35)//Vigor
		{
			int totalhp = ((MOB.CurrentScore.MaxHp / 100) * 10) + MOB.CurrentScore.MaxHp;

			if (totalhp >= MAX_HP)
				MOB.CurrentScore.MaxHp = MAX_HP;
			else
				MOB.CurrentScore.MaxHp = totalhp;

			int totalmp = ((MOB.CurrentScore.MaxMp / 100) * 10) + MOB.CurrentScore.MaxMp;

			if (totalmp >= MAX_MP)
				MOB.CurrentScore.MaxMp = MAX_MP;
			else
				MOB.CurrentScore.MaxMp = totalmp;
		}
	}
#pragma endregion

	for(int p = 1; p <= 7; p++)
	{
		int idx = MOB.Equip[p].sIndex;

		if(idx <= 0 || idx > MAX_ITEMLIST)
			continue;

		int sa = BASE_GetItemSanc(&MOB.Equip[p]);

		if(sa < 9)
			continue;

		int nPos = g_pItemList[idx].nPos;

		 if(nPos == 4)
			MOB.CurrentScore.Ac += 25;
		else if(nPos == 8)
			MOB.CurrentScore.Ac += 25;
		else if(nPos == 16)
			MOB.Rsv |= RSV_CAST;
		else if(nPos == 128)
			MOB.CurrentScore.Ac += 25;
		
		else if(nPos == 64 || nPos == 192)
		{
			int nu = g_pItemList[idx].nUnique;

			if(nu == 47)
				magic += 8;
			else
				MOB.CurrentScore.Damage += 40;
		}
	}

	if (isKibitaSoul)
	{
		MOB.CurrentScore.Str = 2000;
		MOB.CurrentScore.Int = 2000;

		MOB.CurrentScore.MaxHp = 10000;

		int ItemInt = MOB.CurrentScore.Int - MOB.BaseScore.Int > 2000 ? 2000 : MOB.BaseScore.Int;

		int MPDelta = ItemInt * 2;
		unsigned int mmp = MOB.CurrentScore.MaxMp + MPDelta;

		MOB.CurrentScore.MaxMp += mmp;
	}


	if (face < 4)
		MOB.CurrentScore.Damage += (MOB.CurrentScore.Str / 2) + (MOB.CurrentScore.Dex / 3) + MOB.CurrentScore.Special[0] + (extra->ClassMaster == ARCH || extra->ClassMaster == MORTAL ? MOB.CurrentScore.Level : MOB.CurrentScore.Level + MAX_LEVEL);

	if(DAMAGEMULTI != 100)
		MOB.CurrentScore.Damage = MOB.CurrentScore.Damage * DAMAGEMULTI  / 100;

	if(RegenHP < 0)
		RegenHP = 0;

	if(RegenHP > 255)
		RegenHP = 255;

	MOB.RegenHP = RegenHP;

	if(RegenMP < 0)
		RegenMP = 0;

	if(RegenMP > 255)
		RegenMP = 255;

	MOB.RegenMP = RegenMP;

	MOB.CurrentScore.Merchant = (MOB.CurrentScore.Merchant & 240) | MOB.Merchant;

	Att += AttackSpeedBonus + MOB.CurrentScore.Dex / 5;
	Run += RunSpeedBonus;

	int mount = MOB.Equip[14].sIndex;

	if(face <= 4 && (mount >= 2360 && mount <= 2390 || mount >= 3980 && mount <= 3994))
	{
		if (mount >= 3980 && mount <= 3994)
		{
			int code = mount - 3980;

			if (Run < g_pMountTempBonus[code][4])
				Run = g_pMountTempBonus[code][4];
		}

		else if(MOB.Equip[14].stEffect[0].sValue > 0 )
		{
			int code = mount - 2360;

			if(Run < g_pMountBonus[code][4])
				Run = g_pMountBonus[code][4];
		}
	}

	if(Run <= 0)
		Run = 0;
	if(Run > 6)
		Run = 6;

	if(Att < 0)
		Att = 0;
	if(Att > 150)
		Att = 150;

	Att /= 10;

	MOB.CurrentScore.AttackRun = Att * 16 + Run;


	if (MOB.CurrentScore.Hp < 0)
		MOB.CurrentScore.Hp = 0;

	if (MOB.CurrentScore.Mp < 0)
		MOB.CurrentScore.Mp = 0;

	if(MOB.CurrentScore.Hp > MOB.CurrentScore.MaxHp)
		MOB.CurrentScore.Hp = MOB.CurrentScore.MaxHp;

	if(MOB.CurrentScore.Mp > MOB.CurrentScore.MaxMp)
		MOB.CurrentScore.Mp = MOB.CurrentScore.MaxMp;

	if (magic >= MAX_DAMAGE_MG)
		magic = MAX_DAMAGE_MG;

	MOB.Magic = magic;

	if (Critical >= 255)
		Critical = 255;

	MOB.Critical = Critical;


	if (Sagrado < 0)
		Sagrado = 0;

	if (Sagrado > 100)
		Sagrado = 100;

	MOB.Resist[0] = Sagrado;

	if (Trovao < 0)
		Trovao = 0;

	if (Trovao > 100)
		Trovao = 100;

	MOB.Resist[1] = Trovao;

	if (Fogo < 0)
		Fogo = 0;

	if (Fogo > 100)
		Fogo = 100;

	MOB.Resist[2] = Fogo;

	if (Gelo < 0)
		Gelo = 0;

	if (Gelo > 100)
		Gelo = 100;

	MOB.Resist[3] = Gelo;
}

void BASE_GetFirstKey(char * source, char * dest)
{
	 if((source[0] >= 'A' && source[0] <= 'Z' ) || (source[0] >= 'a' && source[0] <= 'z' ))
	 {
		 dest[0] = source[0];
	     dest[1] = 0;

	 	 return;
	 }

	 else if(source[0] < 0)
	 {
		 unsigned char *unsource = (unsigned char*)source;
		 int Point = (unsource[0] - 176) * 94  + unsource[1] - 161; 

         if (Point < 0 || Point >= 2350)
		 {
			strcpy(dest, "etc");

		    return;
		 }

		 int a;

		 for(a = 0; a < 18; a++)
		 {
			 if (Point < KorIndex[a])
				 break;
		 }

		 if  (a >= 0 && a <= 17)
		 {
			 dest[0] = KorFirst[a * 2];
		     dest[1] = KorFirst[a * 2 + 1];
		     dest[2] = 0;
		 }
		 else
		 {
			 strcpy(dest, "etc");

		     return;
		 }

	 } 
	 else
		 strcpy(dest, "etc");

     return;
}

int BASE_GetGuild(int x, int y)
{
	for(int i = 0; i < MAX_GUILDZONE; ++i)
	{
		if (x >= g_pGuildZone[i].CityLimitX1 && x <= g_pGuildZone[i].CityLimitX2 && y >= g_pGuildZone[i].CityLimitY1 && y <= g_pGuildZone[i].CityLimitY2)
			return i;
	}

	return 5;
}

int BASE_GetIndex(STRUCT_ITEM * item)
{
	int hword = 0;
	int lword = 0;

	if(item->sIndex >= 2330 && item->sIndex < 2390) 
		return 0;

	if(item->stEffect[0].cEffect == EF_HWORDINDEX || item->stEffect[1].cEffect == EF_HWORDINDEX || item->stEffect[2].cEffect == EF_HWORDINDEX)
	{ 
		if(item->stEffect[0].cEffect == EF_HWORDINDEX) 
			hword = (unsigned char)item->stEffect[0].cValue; 
		else if(item->stEffect[1].cEffect == EF_HWORDINDEX) 
			hword = (unsigned char)item->stEffect[1].cValue; 
		else
			hword = (unsigned char)item->stEffect[2].cValue;
	}
	if(item->stEffect[0].cEffect == EF_LWORDINDEX || item->stEffect[1].cEffect == EF_LWORDINDEX || item->stEffect[2].cEffect == EF_LWORDINDEX)
	{  
		if(item->stEffect[0].cEffect == EF_LWORDINDEX) 
			lword = (unsigned char)item->stEffect[0].cValue; 
		else if (item->stEffect[1].cEffect == EF_LWORDINDEX) 
			lword = (unsigned char)item->stEffect[1].cValue; 
		else
			lword = (unsigned char)item->stEffect[2].cValue;
	}

	int value = hword * 256 + lword;

	return value;
}

int BASE_GetGuild(STRUCT_ITEM *item)
{
	int hword = 0;
	int lword = 0;
	int value = 0;

	if(item->stEffect[0].cEffect == EF_HWORDGUILD || item->stEffect[1].cEffect == EF_HWORDGUILD || item->stEffect[2].cEffect == EF_HWORDGUILD)
	{
		if(item->stEffect[0].cEffect == EF_HWORDGUILD)
			hword = item->stEffect[0].cValue;
		else if(item->stEffect[1].cEffect == EF_HWORDGUILD)
			hword = item->stEffect[1].cValue;
		else if(item->stEffect[2].cEffect == EF_HWORDGUILD)
			hword = item->stEffect[2].cValue;
	}
	if(item->stEffect[0].cEffect == EF_LWORDGUILD || item->stEffect[1].cEffect == EF_LWORDGUILD || item->stEffect[2].cEffect == EF_LWORDGUILD)
	{
		if(item->stEffect[0].cEffect == EF_LWORDGUILD)
			lword = item->stEffect[0].cValue;
		else if(item->stEffect[1].cEffect == EF_LWORDGUILD)
			lword = item->stEffect[1].cValue;
		else if(item->stEffect[2].cEffect == EF_LWORDGUILD)
			lword = item->stEffect[2].cValue;
	}

	value = (hword * 256) + lword;

	return value;
}

int BASE_GetSubGuild(int item)
{
	int ret = 0;

	if(item >= 526 && item <= 537)
		ret = (item - 526) % 3 + 1;

	return ret;
}

int BASE_GetArena(int x, int y)
{
	for(int i = 0; i < MAX_GUILDZONE; i++)
	{
		if(x >= g_pGuildZone[i].WarAreaX1 && x <= g_pGuildZone[i].WarAreaX2 && y >= g_pGuildZone[i].WarAreaY1 && y <= g_pGuildZone[i].WarAreaY2)
			return i;
	}

	return 5;
}

int BASE_GetVillage(int x, int y)
{
	for(int i = 0; i < 5; ++i )
	{
		if (x >= g_pGuildZone[i].CityLimitX1 && x <= g_pGuildZone[i].CityLimitX2 && y >= g_pGuildZone[i].CityLimitY1 && y <= g_pGuildZone[i].CityLimitY2)
			return i;
	}

	return 5;
}

void BASE_GetKorFirst(int temp, int *a)
{
     int word1;
     int temp1;

     word1 = (temp >> 10) & 31;
     word1 = word1-1;
     temp1 = word1;

     *a = temp1;
}

void BASE_ClearItem(STRUCT_ITEM *item)
{
	item->sIndex = 0;

	item->stEffect[0].cEffect = 0;
	item->stEffect[0].sValue = 0;
	item->stEffect[1].cEffect = 0;
	item->stEffect[1].sValue = 0;
	item->stEffect[2].cEffect = 0;
	item->stEffect[2].sValue = 0;
}

int BASE_CanEquip(STRUCT_ITEM *item, STRUCT_SCORE *score, int Pos, int Class, STRUCT_ITEM *pBaseEquip, STRUCT_MOBEXTRA *extra)
{
	if(item->sIndex <= 0 || item->sIndex >= MAX_ITEMLIST)
		return FALSE;

	int nUnique = g_pItemList[item->sIndex].nUnique;

	if(Pos == 15)
		return FALSE;

	if(Pos != -1)
	{
		int tpos = BASE_GetItemAbility(item, EF_POS);
		int pos = (tpos >> Pos) & 1;

		if(pos == 0)
			return FALSE;

		if(Pos == 6 || Pos == 7)
		{
			int OtherPos = (Pos == 6) ? 7 : 6;
			int	OtherIdx = pBaseEquip[OtherPos].sIndex;

			if(OtherIdx > 0 && OtherIdx < MAX_ITEMLIST)
			{
				int nUnique2 = g_pItemList[OtherIdx].nUnique;
				int otherpos = BASE_GetItemAbility(&pBaseEquip[OtherPos], EF_POS);

				if(tpos == 64 || otherpos == 64)
				{
					if(nUnique == 46)
					{
						if(otherpos != 128)
							return FALSE;
					}
					else if(nUnique2 == 46)
					{
						if(tpos != 128)
							return FALSE;
					}
					else
						return FALSE;
				}
			}
		}
	}

	int cls = BASE_GetItemAbility(item, EF_CLASS);

	if (extra->ClassMaster != MORTAL)
		Class = (extra->MortalFace/10);

	cls = (cls >> Class) & 1;

	if (cls == 0 && extra->ClassMaster == MORTAL)
		return FALSE;

	if (cls == 0 && extra->ClassMaster != MORTAL && Pos != 6 && Pos != 7)
		return FALSE;

	if (BASE_GetItemAbility(item, EF_MOBTYPE) == ARCH && extra->ClassMaster == MORTAL)
		return FALSE;

	if (BASE_GetItemAbility(item, EF_MOBTYPE) == MORTAL && extra->ClassMaster != MORTAL)
		return FALSE;

	if (BASE_GetItemAbility(item, EF_MOBTYPE) == MORTAL && extra->ClassMaster != MORTAL)
		return FALSE;

	if (BASE_GetItemAbility(item, EF_MOBTYPE) == CELESTIAL && (extra->ClassMaster == MORTAL || extra->ClassMaster == ARCH))
		return FALSE;

	if(extra->ClassMaster != MORTAL && Pos == 1 && item->sIndex != 747 && (item->sIndex < 3500 || item->sIndex > 3507))
		return FALSE;

	int lvl = extra->ClassMaster == MORTAL ? BASE_GetItemAbility(item, EF_LEVEL) : 0;
	int str = extra->ClassMaster == MORTAL ? BASE_GetItemAbility(item, EF_REQ_STR) : 0;
	int spt = extra->ClassMaster == MORTAL ? BASE_GetItemAbility(item, EF_REQ_INT) : 0;
	int agi = extra->ClassMaster == MORTAL ? BASE_GetItemAbility(item, EF_REQ_DEX) : 0; 
	int con = extra->ClassMaster == MORTAL ? BASE_GetItemAbility(item, EF_REQ_CON) : 0;
    int weapontype = BASE_GetItemAbility(item, EF_WTYPE);

	weapontype %= 10;
	int modweapon = weapontype;
	int divweapon = weapontype / 10;

	if(Pos == 7 && weapontype != 0)
	{
		int rate = 100;

		if(divweapon == 0 && modweapon > 1)
			rate = 130;
		else if(divweapon == 6 && modweapon > 1)
			rate = 150;

		lvl = lvl * rate / 100;
		str = str * rate / 100;
		spt = spt * rate / 100;
		agi = agi * rate / 100;
		con = con * rate / 100;
	}

	if(lvl > score->Level)
		return FALSE;

	if(str > score->Str)
		return FALSE;

	if(spt > score->Int)
		return FALSE;

	if(agi > score->Dex)
		return FALSE;

	if(con > score->Con)
		return FALSE;

	return TRUE;
}

int BASE_CanCarry(STRUCT_ITEM *item, STRUCT_ITEM *carry, int DestX, int DestY, int *error)
{
	int grid = BASE_GetItemAbility(item, EF_GRID);

	char SourGrid[8];

	memcpy(SourGrid, g_pItemGrid[grid], sizeof(SourGrid));

	char CarryGrid[MAX_CARRY];

	memset(CarryGrid, 0, MAX_CARRY-1);

	for(int i = 0; i < MAX_CARRY; i++)
	{
		if(carry[i].sIndex == 0)
			continue;

		grid = BASE_GetItemAbility(&carry[i], EF_GRID);

		char TempGrid[8];

		memcpy(TempGrid, g_pItemGrid[grid], sizeof(TempGrid));

		int tx = i % 9;
		int ty = i / 9;

		for(int yy = 0; yy < 4; yy++)
		{
			for(int xx = 0; xx < 2; xx++)
			{
				if(TempGrid[yy * 2 + xx] == 0)
					continue;

				if(yy + ty < 0 || xx + tx < 0 || yy + ty >= CARRYGRIDY || xx + tx >= CARRYGRIDX)
					continue;

				CarryGrid[(yy + ty) * CARRYGRIDX + xx + tx] = i + 1;
			}
		}
	}

	for(int yy = 0; yy < 4; yy++)
	{
		for(int xx = 0; xx < 2; xx++)
		{
			if(SourGrid[(yy * 2) + xx] == 0)
				continue;

			if(yy + DestY < 0 || xx + DestX < 0 || yy + DestY >= CARRYGRIDY || xx + DestX >= CARRYGRIDX)
			{
				*error = -1;

				return FALSE;
			}

			if(CarryGrid[(yy + DestY) * CARRYGRIDY + xx + DestX] != 0)
			{
				*error = CarryGrid[(yy + DestY) * CARRYGRIDY + xx + DestX];

				return FALSE;
			}
		}
	}

	return TRUE;
}

int BASE_CanCargo(STRUCT_ITEM *item, STRUCT_ITEM *cargo, int DestX, int DestY)
{
	int grid = BASE_GetItemAbility(item, EF_GRID);

	char SourGrid[8];

	memcpy(SourGrid, g_pItemGrid[grid], sizeof(SourGrid));

	char CargoGrid[MAX_CARGO];

	memset(CargoGrid, 0, MAX_CARGO-2);

	for(int i = 0; i < MAX_CARGO; i++)
	{
		if(cargo[i].sIndex == 0)
			continue;

		int grid = BASE_GetItemAbility(&cargo[i], EF_GRID);

		char TempGrid[8];

		memcpy(TempGrid, g_pItemGrid[grid], sizeof(TempGrid));

		int tx = i % 9;
		int ty = i / 9;

		for(int yy = 0; yy < 4; yy++)
		{
			for(int xx = 0; xx < 2; xx++)
			{
				if(TempGrid[(yy * 2) + xx] == 0)
					continue;

				if(yy + ty < 0 || xx + tx < 0 || yy + ty >= CARGOGRIDY || xx + tx >= CARGOGRIDX)
					continue;

				CargoGrid[(yy + ty) * CARGOGRIDX + xx + tx] = 1;
			}
		}
	}

	for(int yy = 0; yy < 4; yy++)
	{
		for(int xx = 0; xx < 2; xx++)
		{
			if(SourGrid[(yy * 2) + xx] == 0)
				continue;

			if(yy + DestY < 0 || xx + DestX < 0 || yy + DestY >= CARGOGRIDY || xx + DestX >= CARGOGRIDX)
				return FALSE;

			if(CargoGrid[(yy + DestY) * CARGOGRIDY + xx + DestX] == 1)
				return FALSE;
		}
	}

	return TRUE;
}

void BASE_SortTradeItem(STRUCT_ITEM *Item, int Type)
{
	int Buffer[MAX_TRADE];

	memset(Buffer, -1, sizeof(Buffer));

	for(int i = 0; i < MAX_TRADE; i++)
	{
		if(!Item[i].sIndex)
		{
			Buffer[i] = -1;

			continue;
		}

		Buffer[i] = BASE_GetItemAbility(&Item[i], Type);
	}

	STRUCT_ITEM ItemTemp[MAX_TRADE];

	memset(ItemTemp, 0, sizeof(ItemTemp));

	for(int i = 0; i < MAX_TRADE; i++)
	{
		int MaxBufferIndex = 0;
		int MaxBuffer = -1;		

		for(int j = 0; j < MAX_TRADE; j++)
		{
			if(Buffer[j] <= MaxBuffer)
				continue;

			MaxBufferIndex = j;

			MaxBuffer = Buffer[j];
		}

		if(MaxBuffer == -1)
			break;

		Buffer[MaxBufferIndex] = -1;

		*(int*)&ItemTemp[i] = *(int*)&Item[MaxBufferIndex];
		*(int*)((int)&ItemTemp[i] + 4) = *(int*)((int)&Item[MaxBufferIndex] + 4);
	}

	memcpy(Item, ItemTemp, sizeof(ItemTemp));
}

int BASE_CanTrade(STRUCT_ITEM *Dest, STRUCT_ITEM *Carry, unsigned char *MyTrade, STRUCT_ITEM *OpponentTrade, int MaxCarry)
{
	STRUCT_ITEM OpponentTemp[MAX_TRADE];

	memcpy(OpponentTemp, OpponentTrade, sizeof(OpponentTemp));

	memcpy(Dest, Carry, sizeof(STRUCT_ITEM)*MAX_CARRY);

	for (int i = 0; i < MAX_TRADE; i++)
	{
		int pos = MyTrade[i];

		if (pos == -1 || pos == 255)
			continue;

		if (pos >= 0 && pos < MAX_CARRY)
			BASE_ClearItem(&Dest[pos]);
	}

	BASE_SortTradeItem(OpponentTemp, 33);

	for (int i = 0; i < MAX_TRADE; i++)
	{
		if (!OpponentTemp[i].sIndex)
			continue;

		int j = 0;

		for (j = 0; j < MaxCarry; j++)
		{
			int x = j % 9;
			int y = j / 9;
			int error = -2;

			if (Dest[j].sIndex != 0)
				continue;

			memcpy(&Dest[j], &OpponentTemp[i], sizeof(STRUCT_ITEM));

			break;
		}

		if (j == MaxCarry)
			return FALSE;
	}

	return TRUE;
}

void BASE_InitializeMessage(char *file)
{
	memset(g_pMessageStringTable, MAX_STRING, 128);

	FILE *fp = fopen(file, "rt");

	if(fp == NULL)
		return;

	 char str[1024];
	 char part1[256];
	 char *cont;

	while(1)
	{
		int Index = -1;
		char *ret = fgets((char*)str, 1024, fp);

		if(ret == NULL)
			break;

        sscanf(str, "%d %s", &Index, part1);

		str[510] = 0; 
		str[511] = 0; 

		if(Index <= 0 || Index > MAX_STRING) 
			continue;

		int TabPos = 0;

		for(int i = 0; i < 256; i++)
		{   
			if(str[i] == '\t') 
			{
				TabPos = i;

				break;
			} 
		}

		if(TabPos == 0)
		{  
			MessageBox(NULL, str, "Can't parse String", MB_OK);

		   continue;
		}

        cont = (char*)(str + TabPos + 1);

		for(int i = 0; i < 128; i++)
		{   
			if (cont[i] == '\t' || cont[i] == '\n' || cont[i] == 0)
			{ 
				cont[i] = 0;
				
				break;
			} 
		}

        int len = (int)strlen(cont);

		if(len <= 0 || len >= 128)
		{  
			MessageBox(NULL, str, "Empty or Long String", MB_OK);

			continue;
		}

		strcpy(g_pMessageStringTable[Index], cont);
	}

	fclose(fp);
}

void BASE_InitializeMobname(char *file, int offset)
{
	if(offset == 0)
		memset(g_pEnglish, 0, 19200);

	FILE *fp = fopen(file, "rt");

	if(fp == NULL)
		return;

	int Index = 0;

	char str[1024];
	char part1[256];
	char *cont;

	if(offset == 0)
	{
		while(1)
		{
			char *ret = fgets((char*)str, 1024, fp);

			if(ret == NULL)
				break;

			sscanf(str, "%s", part1);

			str[510] = 0;
			str[511] = 0;

			int TabPos = 0;

			for(int i = 0; i < 256; i++)
			{   
				if(str[i] == '\t') 
				{
					TabPos = i;

					break;
				} 
			}

			if(TabPos == 0)
			{  
				MessageBox(NULL, str, "Can't parse String", MB_OK);

				continue;
			}

			cont = (char*)(str + TabPos + 1);

			for(int i = 0; i < 128; i++)
			{   
				if(cont[i] == '\t' || cont[i] == '\n' || cont[i] == 0)
				{ 
					cont[i] = 0;

					break;
				} 
			}

			int len = 0;
			len = (int)strlen(part1);

			if(len <= 0 || len >= 16)
			{  
				MessageBox(NULL, str, "Empty or Long String-1st", MB_OK);

				continue;
			}

			len = (int)strlen(cont);

			if(len <= 0 || len >= 16)
			{  
				MessageBox(NULL, str, "Empty or Long String-2nd", MB_OK);

				continue;
			}


			if(Index < 0 || Index >= MAX_STRING) 
			{
				MessageBox(NULL, str, "Too much items in mobname.txt", MB_OK);

				break;
			}

			strcpy(g_pEnglish[Index][0], part1);
			strcpy(g_pEnglish[Index][1], cont);
			strcpy(g_pEnglish[Index][2], part1);

			Index++;
		}
		
	}
	else
	{
		while(1)
		{
			char *ret = fgets((char*)str, 1024, fp);

			if(ret == NULL)
				break;

			sscanf(str, "%s", part1);

			str[510] = 0;
			str[511] = 0;

			int TabPos = 0;

			for(int i = 0; i < 256; i++)
			{   
				if(str[i] == '\t') 
				{
					TabPos = i;

					break;
				} 
			}

			if(TabPos == 0)
			{  
				MessageBox(NULL, str, "Can't parse String", MB_OK);

				continue;
			}

			cont = (char*)(str + TabPos + 1);

			for(int i = 0; i < 128; i++)
			{   
				if(cont[i] == '\t' || cont[i] == '\n' || cont[i] == 0)
				{ 
					cont[i] = 0;

					break;
				} 
			}

			int len = 0;
			len = strlen(part1);

			if(len <= 0 || len >= NAME_LENGTH)
			{  
				MessageBox(NULL, str, "Empty or Long String-1st", MB_OK);

				continue;
			}

			len = (int)strlen(cont);

			if(len <= 0 || len >= NAME_LENGTH)
			{  
				MessageBox(NULL, str, "Empty or Long String-2nd", MB_OK);

				continue;
			}


			if(Index < 0 || Index >= MAX_STRING) 
			{
				MessageBox(NULL, str, "Too much items in mobname.txt", MB_OK);

				break;
			}


			for(Index = 0; Index < MAX_STRING; Index++)
			{
				if(!strcmp(part1, g_pEnglish[Index][0]))
					break;

				if(!strcmp(part1, g_pEnglish[Index][1]))
					break;
			}

			if(Index == MAX_STRING)
			{
                MessageBoxA(NULL, str, "Can't Find match mobname", MB_OK);

                break;
			}

			strcpy(g_pEnglish[Index][2], cont);

		}
	}
}

void BASE_WriteMessageBin()
{
	int size = sizeof(char) * MAX_STRING * 128;
	char *temp = (char *)g_pMessageStringTable;

	for(int i = 0; i < size; i++)
	{ 
		temp[i] = temp[i] ^ 0x5A;
	}

	FILE *fp = fopen("UI/strdef.bin", "wb");

	if(fp != NULL)
	{	
		fwrite(g_pMessageStringTable, sizeof(char), MAX_STRING * 128, fp);

		int sum = BASE_GetSum((char*)g_pMessageStringTable, size);
		fwrite(&sum, sizeof(sum), 1 , fp);

		fclose(fp);
	}

	for(int i = 0; i < size; i++)
	{
		temp[i] = temp[i] ^ 0x5A;
	}

}

int BASE_ReadMessageBin()
{
	memset(g_pMessageStringTable, MAX_STRING, 128);

	int size = sizeof(char) * MAX_STRING * 128;
	char *temp = (char *)g_pMessageStringTable;
	int tsum = 0;

	FILE *fp = fopen("UI/strdef.bin", "rb");

	if(fp != NULL)
	{
		fread(g_pMessageStringTable, size , 1, fp);
		fread(&tsum, sizeof(tsum), 1, fp);

		fclose(fp);
	}

	int sum = BASE_GetSum((char*)g_pMessageStringTable, size);
	/*
	if(sum != tsum) 
		return FALSE;
	*/
	for(int i = 0; i < size; i++)
	{ 
		temp[i] = temp[i] ^ 0x5A;
	}

	return TRUE;
}

void BASE_InitializeItemList()
{
	int size = sizeof(STRUCT_ITEMLIST)*MAX_ITEMLIST;

	memset(g_pItemList, 0, size);

	int ret = 1;

	ret = BASE_ReadItemListFile("../../Common/itemlist.csv", 1);

	if(ret == FALSE)
	{
		MessageBox(NULL, "There is no file.", "itemlist.csv", MB_OK);

		return;
	}

	char extraitem[128];

	strcpy(extraitem, "../../Common/extraitem.csv");

	ret = BASE_ReadItemListFile(extraitem, 0);
}

int BASE_ReadItemListFile(char *filename, int Build)
{
	FILE *wfp = NULL;

	if(Build != 0)
	{	
		BASE_WriteItemList(1);

		wfp =  fopen("../../Common/extraitem.bin", "wb");

		if(wfp == NULL)
		{
			MessageBox(NULL, "Can't write extraitem.bin", "ERROR", MB_OK);

			return FALSE;
		}
	}

	char temp[256];

	sprintf(temp, "%s", filename);

	FILE *fp = fopen(temp, "rb");

    if(fp == NULL) 
	{
		sprintf(temp, "../../TMSRV/Run/%s", filename);
		
		fp = fopen(temp, "rb");
	}

	if(fp == NULL)
		return FALSE;

	while(1)
	{
		int  Index = -1;

      	char Name[256];

		memset(Name, 0, 256);

	    char nE[MAX_STATICEFFECT][256];
		memset(nE, 0, sizeof(nE));

	    int  vE[MAX_STATICEFFECT];
		memset(vE, 0, sizeof(vE));

		int ve = sizeof(vE);
		int ne = sizeof(nE);

		char str[1024];
		char *ret = fgets((char*)str, 1024, fp);

		if(ret == NULL)
			break;

		for(int i = 0; i < 1024; i++)
		{
			if (str[i] == ',')
				str[i] = ' ';
		}

		char temp1[256];
		temp1[0] = 0;

		char temp2[256];
		temp2[0] = 0;

		int unique = 0;
		int nprice = 0;
		int npos = 0;
		int	Grade = 0;
		int	Extra = 0;

		sscanf(str, "%d %s %s %s %d %d %d %d %d %s %d %s %d %s %d %s %d %s %d %s %d %s %d %s %d %s %d %s %d %s %d %s %d", &Index, Name, temp1, temp2, &unique, &nprice, &npos, &Extra, &Grade, 
			nE[0], &vE[0], nE[1], &vE[1], nE[2], &vE[2], nE[3], &vE[3], nE[4], &vE[4], nE[5], &vE[5], nE[6], &vE[6], nE[7], &vE[7], nE[8], &vE[8], nE[9], &vE[9], nE[10], &vE[10], nE[11], &vE[11]);

		if(Index == -1)
			continue;

		if(Index >= MAX_ITEMLIST)
		{
			MessageBox(NULL, str, "check MAX_ITEMLIST", MB_OK);

			continue;
		}

		int len = strlen(Name);

        if (len >= ITEMNAME_LENGTH-1)
		{
			MessageBox(NULL, "too long Item Name", Name, MB_OK);

			Name[ITEMNAME_LENGTH-1] = 0;
			Name[ITEMNAME_LENGTH-2] = 0;
		}

		strcpy(g_pItemList[Index].Name, Name);
 
		
		int  Mesh = 0;
	    int  Texture = 0;
		int  ReqLvl = 0;
        int  ReqStr = 0;
		int  ReqInt = 0;
		int  ReqDex = 0;
		int  ReqCon = 0;

		if(temp1[0] == '.')
			temp1[0] = 32;
		
		if(temp2[0] == '.') 
			temp2[0] = 32;

		sscanf(temp1, "%d.%d", &Mesh, &Texture);
		sscanf(temp2, "%d.%d.%d.%d.%d", &ReqLvl, &ReqStr, &ReqInt, &ReqDex, &ReqCon);

		g_pItemList[Index].IndexMesh          = Mesh;
		g_pItemList[Index].IndexTexture       = Texture;
		g_pItemList[Index].ReqLvl             = ReqLvl;
	    g_pItemList[Index].ReqStr             = ReqStr;
	    g_pItemList[Index].ReqInt             = ReqInt;
	    g_pItemList[Index].ReqDex             = ReqDex;
	    g_pItemList[Index].ReqCon             = ReqCon;
		g_pItemList[Index].nUnique             = unique;
		g_pItemList[Index].Price              = nprice;
		g_pItemList[Index].nPos                = npos;
		g_pItemList[Index].Grade			   = Grade;
		g_pItemList[Index].Extra			   = Extra;

   	    g_pItemList[Index].IndexVisualEffect  = 0;


        for(int j = 0; j < MAX_STATICEFFECT; j++)
		{
			if(nE[j][0] == 0)
				continue;

			int v;

		    for(v = 0; v < MAX_EFFECTINDEX; v++)
			{
				if (!strcmp(EffectNameTable[v], nE[j]))
					break;
			}

		    if (j == MAX_EFFECTINDEX)
			{
				MessageBox(0, nE[j], "Unregisterd effect", 0);

                continue;
			}   

			g_pItemList[Index].stEffect[j].sEffect = v;
			g_pItemList[Index].stEffect[j].sValue  = vE[j];
		}

		if(Build != 0)
		{
			fwrite(&Index, 2, 1, wfp);
			fwrite(&g_pItemList[Index], sizeof(STRUCT_ITEMLIST), 1, wfp);
		}
	}

	fclose(fp);

	if(Build != 0)
	{
		fclose(wfp);
		BASE_WriteItemList(0);
	}

	return TRUE;
}

void BASE_InitializeEffectName()
{
	for(int i = 0; i < MAX_EFFECTINDEX; i++)
		EffectNameTable[i][0] = 0;

	FILE *fp = NULL;

	fp = fopen("../../Common/ItemEffect.h", "rb");

	if(fp == NULL)
		fp = fopen("../../TMSRV/Run/ItemEffect.h", "rb");

	if(fp == NULL)
	{
		MessageBoxA(NULL, "There is no file", "ItemEffect", MB_OK);

		return;
	}

	char temp[256];

	char str5[256];
	char str4[256];
	char str3[256];
	char str2[256];
	char str1[256];

	while(1)
	{
		char *ret = fgets(temp, 255, fp);

		if(ret == NULL)
			break;

		if(temp[0] == '\r')
			continue;

		str1[0] = 0;

		sscanf(temp, "%s", str1);

		if(strcmp(str1, "#include") == 0)
		{
			sscanf(temp, "%s %s", str1, str2);

			str2[1 + strlen(str2)] = 0;

			BASE_InitializeEffectName();

			continue;
		}

		else if(strcmp(str1, "#define") != 0)
			continue;

		sscanf(temp, "%s %s %s %s %s", str1, str2, str3, str4, str5);

		int val = atoi(str3);

		if(val < 0 || val > MAX_EFFECTINDEX)
		{
			MessageBox(NULL, temp,"Effect.hÀÇ Define Value°¡ ¼ýÀÚ°¡ ¾Æ´Ï°Å³ª 0ÀÌÇÏ ¶Ç´Â MAX_EFFECTINDEXÀÌ»óÀÌ´Ù", MB_OK);

			continue;
		}

		if(EffectNameTable[val][0] != 0)
		{
			MessageBox(NULL, temp, EffectNameTable[val], MB_OK);

			continue;
		}
		
		memcpy(EffectNameTable[val], str2, 24);

		continue;
	}

	fclose(fp);
}

void BASE_InitializeHitRate()
{
	memset(g_pHitRate, 0, sizeof(g_pHitRate));

	int Jump = 512;
	int Start = 0;
	int Quad = 0;

	do
	{
		for(int i = 0; i < 1024; i++)
		{
			if(g_pHitRate[i])
				continue;

			if(Quad == 0)
				g_pHitRate[i] = Start;
			else if(Quad == 1)
				g_pHitRate[i] = 512 - Start;
			else if(Quad == 2)
				g_pHitRate[i] = Start + 512;
			else
				g_pHitRate[i] = 1024 - Start;

			if(g_pHitRate[i] > 999)
				g_pHitRate[i] = 999;

			Quad++;

			if(Quad >= 4)
				Quad = 0;

			if(Quad == 0)
				Start++;
		}

		Jump /= 2;

		if(Jump)
			continue;
		else
			break;

	} while(1);

	g_pHitRate[0] = 512;
}

int BASE_UpdateItem(int maskidx, int CurrentState, int NextState, int xx, int yy, char *pHeight, int rotate, int *height)
{
	*height = 0;
	int delta = 0;

	if(maskidx >= MAX_GROUNDMASK || maskidx < 0)
		return FALSE;

	if(CurrentState == 1 && (NextState == 3 || NextState == 2))
		delta = 1;
	if(NextState == 1 && (CurrentState == 3 || CurrentState == 2))
		delta = -1;

	if(delta == 0)
		return FALSE;

	for(int y = 0; y <= MAX_GROUNDMASK-1; y++)
	{
		for(int x = 0; x <= MAX_GROUNDMASK-1; x++)
		{
			int val = g_pGroundMask[maskidx][rotate][y][x];
			int xp = xx + x - 2;
            int yp = yy + y - 2;

			if(val == 0)
				continue;

			val *= delta;

			if((xp - g_HeightPosX < 1) || (yp - g_HeightPosY < 1) || ((xp - g_HeightPosX) > (g_HeightWidth - 2)) || ((yp - g_HeightPosY) > (g_HeightHeight - 2)))
				break;

			int Last = val + pHeight[(((yp - g_HeightPosY) * g_HeightWidth) + xp) - g_HeightPosX];

			if(Last > 255)
				Last = 255;
			if(Last < 0)
				Last = 0;

			if(g_pGroundMask[maskidx][rotate][y][x] != 0)
				*height = Last;

			pHeight[(((yp - g_HeightPosY) * g_HeightWidth) + xp) - g_HeightPosX] = Last;
		}
	}

	return TRUE;
}

int BASE_UpdateItem2(int maskidx, int CurrentState, int NextState, int xx, int yy, char *pHeight, int rotate, int height)
{
	int delta = 0;

	if(maskidx >= MAX_GROUNDMASK || maskidx < 0)
		return FALSE;

	for(int y = 0; y <= MAX_GROUNDMASK-1; y++)
	{
		for(int x = 0; x <= MAX_GROUNDMASK-1; x++)
		{
			int val = g_pGroundMask[maskidx][rotate][y][x];
			int xp = xx + x - 2;
			int yp = yy + y - 2;

			if((xp - g_HeightPosX < 1) || (yp - g_HeightPosY < 1) || ((xp - g_HeightPosX) > (g_HeightWidth - 2)) || ((yp - g_HeightPosY) > (g_HeightHeight - 2)))
				break;

			if(val != 0)
				pHeight[(((yp - g_HeightPosY) * g_HeightWidth) + xp) - g_HeightPosX] = height;
		}
	}

	return TRUE;
}

void BASE_GetDestByAction(unsigned short *x, unsigned short *y, MSG_Action *mv)
{
	*x = mv->PosX;
	*y = mv->PosY;

	for(int i = 0; i < MAX_ROUTE; i++)
	{
		if(mv->Route[i] == 0)
			break;

		if(mv->Route[i] == '6')
			*x++;

		if(mv->Route[i] == '4')
			*x--;

		if(mv->Route[i] == '8')
			*y--;

		if(mv->Route[i] == '2')
			*y++;

		if(mv->Route[i] == '9')
		{
			*x++;
			*y--;
		}

		if(mv->Route[i] == '3')
		{
			*x++;
			*y++;
		}

		if(mv->Route[i] == '7')
		{
			*x--;
			*y--;
		}

		if(mv->Route[i] == '1')
		{
			*x--;
			*y++;
		}
	}
}

int BASE_GetManaSpent(int SkillNumber, int SaveMana, int Special)
{
	int manaspent = g_pSpell[SkillNumber].ManaSpent * (Special / 2 + 100) / 100;
	manaspent = (100 - SaveMana) * manaspent / 100;

	return manaspent;
}

int BASE_GetHitRate(STRUCT_SCORE *att, STRUCT_SCORE *def)
{
	if(def->Dex == 0 || def->Level == 0)
		return 95;

	if(att->Dex < 0 || def->Dex < 0 || att->Level < 0 || def->Level < 0)
		return 0;

	int parta = 60 * att->Dex / (def->Dex / 2 + att->Dex);
	int partb = 100 * (def->Level + att->Level) / def->Level;
	int rate = partb * parta / 100;

	if(rate >= 95)
		rate = 95;
	if(rate < 40)
		rate = 40;

	return rate;
}

int BASE_GetDamageRate(STRUCT_SCORE* att, int Skill)
{
	int rate = Skill + att->Special[0];

	if(rate >= 200)
		rate = 200;
	if(rate < 0)
		rate = 0;

	return rate;
}

int BASE_GetAccuracyRate(STRUCT_SCORE *att)
{
	int ret = att->Dex;
	ret = ret / 2 + 50;

	if(ret > 100)
		ret = 100;

	return ret;
}

int BASE_GetDoubleCritical(STRUCT_MOB *mob, unsigned short *sProgress, unsigned short *cProgress, unsigned char *bDoubleCritical)
{
	*bDoubleCritical = 0;

	if(cProgress == 0)
		return FALSE;

	if(*cProgress >= 1024)
	{	
		if((*cProgress & 0x3FF) < 0)
			*cProgress = ((*cProgress & 0x3FF) - 1 | 0xFC00) + 1;		
	}

	if(sProgress != 0 && *sProgress >= 1024)
	{
		int val = *sProgress & 0x3FF;

		if(val < 0)
			val = ((val - 1) | 0xFC00) + 1;	

		*sProgress = val;
	}

	int ret = 1;

	int hitvalue[2];

	hitvalue[0] = 100 * ((mob->CurrentScore.AttackRun >> 4) - 5);
	hitvalue[1] = mob->Critical;

	if((sProgress != NULL && cProgress != NULL) && *cProgress != *sProgress)
	{
		*sProgress = *cProgress;

		int tProgress = *cProgress;

		if(tProgress < *sProgress)
			tProgress += 256;

		if(tProgress < *sProgress && tProgress < *sProgress + 5)
			ret = 1;
	}

	int value = g_pHitRate[*cProgress];

	for(int i = 0; i < 2; i++)
	{
		int bit = 0;

		if(i == 0)
		{
			if(value < hitvalue[0])
				bit = 1;
		}

		if(i == 1)
		{
			if (rand() % 255 < hitvalue[1])
				bit = 1;
		}

		*bDoubleCritical |= bit << i;
	}

	if(sProgress)
		++*sProgress;

	++*cProgress;

	return ret;
}

int BASE_GetRoute(int x, int y, int *targetx, int *targety, char *Route, int distance, char *pHeight)
{
	int lastx = x;
	int lasty = y;
	int tx = *targetx;
	int ty = *targety;

	memset(Route, 0, MAX_ROUTE);

	for(int i = 0; i < distance && i < MAX_ROUTE-1; i++)
	{
		if(x - g_HeightPosX < 1 || y - g_HeightPosY < 1 || x - g_HeightPosX > g_HeightWidth - 2 || y - g_HeightPosY > g_HeightHeight - 2)
		{
			Route[i] = 0;

			break;
		}

		int cul = pHeight[(y - g_HeightPosY) * g_HeightWidth + x - g_HeightPosX];

		if(cul > MH)
			int kk = 0;

		 int n  = pHeight[(y - g_HeightPosY - 1) * g_HeightWidth + x - g_HeightPosX    ];
		 int ne = pHeight[(y - g_HeightPosY - 1) * g_HeightWidth + x - g_HeightPosX + 1];
		 int e  = pHeight[(y - g_HeightPosY    ) * g_HeightWidth + x - g_HeightPosX + 1];
		 int se = pHeight[(y - g_HeightPosY + 1) * g_HeightWidth + x - g_HeightPosX + 1];
		 int s  = pHeight[(y - g_HeightPosY + 1) * g_HeightWidth + x - g_HeightPosX    ];
		 int sw = pHeight[(y - g_HeightPosY + 1) * g_HeightWidth + x - g_HeightPosX - 1];
		 int w  = pHeight[(y - g_HeightPosY    ) * g_HeightWidth + x - g_HeightPosX - 1];
		 int nw = pHeight[(y - g_HeightPosY - 1) * g_HeightWidth + x - g_HeightPosX - 1];

		 if(tx == x && ty == y) 
		 {
			 Route[i] = 0;
			 break;
		 }

		 if(tx == x && ty < y && n < cul + MH && n > cul - MH) 
		 {
			 Route[i] = '2'; 
			 y--;

			 continue;
		 }

		 if(tx > x && ty < y && ne < cul + MH && ne > cul - MH) 
		 {
			 Route[i] = '3';

			 x++;
			 y--;

			 continue;
		 }

		 if(tx > x && ty == y && e < cul + MH && e > cul - MH) 
		 {
			 Route[i] = '6';

			 x++;

			 continue;
		 }

		 if(tx > x && ty > y && se < cul + MH && se > cul - MH) 
		 {
			 Route[i] = '9';
			 
			 x++;
			 y++;
			 
			 continue;
		 }   

		 if(tx == x && ty > y && s < cul + MH && s > cul - MH) 
		 {
			 Route[i] = '8';
			 
			 y++;
			 
			 continue;
		 }

		 if(tx < x && ty > y && sw < cul + MH && sw > cul - MH) 
		 {
			 Route[i] = '7';
			 
			 x--;
			 y++;
			 
			 continue;
		 }

		 if(tx < x && ty == y && w < cul + MH && w > cul - MH) 
		 {
			 Route[i] = '4';
			 
			 x--;
			 
			 continue;
		 }

		 if(tx < x && ty < y && nw < cul + MH && nw > cul - MH) 
		 {
			 Route[i] = '1';
			 
			 x--;
			 y--;
			 
			 continue;
		 }

		 if(tx > x && ty < y && e < cul + MH && e > cul - MH) 
		 {
			 Route[i] = '6';

			 x++;
			 
			 continue;
		 }

		 if(tx > x && ty < y && n < cul + MH && n > cul - MH) 
		 {
			 Route[i] = '2';
			 
			 y--;
			 
			 continue;
		 }

		 if(tx > x && ty > y && e < cul + MH && e > cul - MH) 
		 {
			 Route[i] = '6';
			 
			 x++;
			 
			 continue;
		 }         

		 if(tx > x && ty > y && s < cul + MH && s > cul - MH) 
		 {
			 Route[i] = '8';

			 y++;
			 
			 continue;
		 }   

		 if(tx < x && ty > y && w < cul + MH && w > cul - MH) 
		 {
			 Route[i] = '4';
			 
			 x--;
			 
			 continue;
		 }

		 if(tx < x && ty > y && s < cul + MH && s > cul - MH) 
		 {
			 Route[i] = '8';

			 y++;

			 continue;
		 }

		 if(tx < x && ty < y && w < cul + MH && w > cul - MH) 
		 {
			 Route[i] = '4';

			 x--;

			 continue;
		 }

		 if(tx < x && ty < y && n < cul + MH && n > cul - MH) 
		 {
			 Route[i] = '2';
			 
			 y--;
			 
			 continue;
		 }

		 if(tx == x + 1 || ty == y + 1 || tx == x - 1 || ty == y - 1) 
		 {
			 Route[i] = 0;

			 break;
		 }

		 if(tx == x && ty > y && se < cul + MH && se > cul - MH) 
		 {
			 Route[i] = '9';
			 
			 x++;			 
			 y++;
			 
			 continue;
		 }         

		 if(tx == x && ty > y && sw < cul + MH && sw > cul - MH) 
		 {
			 Route[i] = '7';
			 
			 x--;
			 y++;
			 
			 continue;
		 }

		 if(tx == x && ty < y && ne < cul + MH && ne > cul - MH) 
		 {
			 Route[i] = '3';

			 x++;
			 y--;

			 continue;
		 }

		 if(tx == x && ty < y && nw < cul + MH && nw > cul - MH) 
		 {
			 Route[i] = '1';
			 
			 x--;
			 y--;
			 
			 continue;
		 }

		 if(tx < x && ty == y && sw < cul + MH && sw > cul - MH) 
		 {
			 Route[i] = '7';
			 
			 x--;
			 y++;
			 
			 continue;
		 }

		 if(tx < x && ty == y && nw < cul + MH && nw > cul - MH) 
		 {
			 Route[i] = '1';
			 
			 x--;
			 y--;
			 
			 continue;
		 }

		 if(tx > x && ty == y && se < cul + MH && se > cul - MH) 
		 {
			 Route[i] = '9';
			 
			 x++;
			 y++;
			 
			 continue;
		 }         

		 if(tx > x && ty == y && ne < cul + MH && ne > cul - MH) 
		 {
			 Route[i] = '3';

			 x++;
			 y--;

			 continue;
		 }

		 Route[i] = 0;
		 break; 
	}

	if(lastx == x && lasty == y)
		return 0;

	int kk = 0;

	*targetx = x;
	*targety = y;

	if(lastx == x && lasty == y) 
		return 0;

	return 1;
}

int BASE_GetDistance(int x1, int y1, int x2, int y2)
{
	int	dx, dy;

	if(x1 > x2) 
		dx = x1 - x2;
	else	
		dx = x2 - x1;

	if(y1 > y2) 
		dy = y1 - y2;
	else       
		dy = y2 - y1;

	if(dx <= 6 && dy <= 6)
		return g_pDistanceTable[dy][dx];

	if(dx > dy) 
		return dx+1;
	else       
		return dy+1;
}

void BASE_WriteInitItem()
{
	int size = sizeof(STRUCT_INITITEM) * MAX_INITITEM;
	char *temp = (char *)g_pInitItem;

	for(int i = 0; i < size; i++)
	{
		temp[i] = temp[i] ^ 0xFF;
	} //STRUCT_INITITEM g_pInitItem[MAX_INITITEM];

	FILE *fp = fopen("../../Common/InitItem.bin", "wb");

	if(fp == NULL) 
	{
		MessageBox(NULL, "Can't write inititem.bin", "ERROR", MB_OK | MB_SYSTEMMODAL);

		return;
	}

	fwrite(g_pInitItem, size,1, fp);

	fclose(fp);

	for(int i = 0; i < size; i++)
	{
		temp[i] = temp[i] ^ 0xFF;
	}
}

int BASE_ReadInitItem()
{
	char szPathName[256];
	char szFileName[256];

	memset(szPathName, 0, 256);
	memset(szFileName, 0, 256);

	GetModuleFileName(NULL, szPathName, 256);

	int len = strlen(szPathName);

	for(int nIndex = len-1; nIndex > 0; nIndex--)
	{
		if(szPathName[nIndex]=='\\')
		{
			sprintf(szFileName, "%s", &szPathName[nIndex+1]);

			break;
		}
	}

#ifdef _ADMIN_TOOL
#ifdef _CHINESESIMPLE
	sprintf(szFileName, "WYDSC.EXE");
#elif _CHINESETRADITION
	sprintf(szFileName, "WYDTC.EXE");
#else
	sprintf(szFileName, "WYD.EXE");
#endif
#endif

	int size = sizeof(STRUCT_INITITEM) * MAX_INITITEM;
	char *temp = (char *)g_pInitItem;

	FILE *fp = fopen(szFileName, "rb");

	if(fp == NULL) 
	{
		MessageBox(NULL, "Can't read inititem", "ERROR", MB_OK | MB_SYSTEMMODAL);

		return FALSE;
	}

	fseek(fp, -size, SEEK_END);
	fread(g_pInitItem, size, 1, fp);

	fclose(fp);

	for(int i = 0; i < size; i++)
	{ 
		temp[i]= temp[i] ^ 0xFF;
	}

	for(int i = 0; i < MAX_INITITEM; i++)
	{
		if(g_pInitItem[i].PosX <= 0)
		{
			g_dwInitItem = i;

			break;
		}
	}

	return TRUE;
} 

void BASE_InitializeInitItem()
{
	FILE *fp = NULL;

	fp = fopen("../../Common/InitItem.csv", "rb");

	if(fp == NULL)
		fp = fopen("../../TMSRV/Run/InitItem.csv", "rb");

	if(fp == NULL)
	{
		MessageBox(NULL, "There is no file", "InitItem.csv", MB_OK);

		return;
	}
	
	char str[1024];

	g_dwInitItem = 0;

	while(1)
	{
		char *ret = fgets(str, 1024, fp);

		if(ret == 0)
			break;

		for(int i = 0; i < 1024; i++)
		{
			if(str[i] == ',')
				str[i] = ' ';
		}

		int Index = -1;
		int PosX = 0;
		int PosY = 0;
		int Rotate = 0;

		sscanf(str, "%d %d %d %d", &Index, &PosX, &PosY, &Rotate);

		if(Index == -1)
			continue;

		g_pInitItem[g_dwInitItem].PosX = PosX;
		g_pInitItem[g_dwInitItem].PosY = PosY;
		g_pInitItem[g_dwInitItem].ItemIndex = Index;
		g_pInitItem[g_dwInitItem].Rotate = Rotate;

		g_dwInitItem++;
	}

	fclose(fp);
}

void BASE_InitializeSkill()
{
     FILE *fp  =  fopen("../../Common/SkillData.csv", "rb");

     if(fp == NULL)
		 fp = fopen("../../TMSRV/Run/SkillData.csv", "rb");

	 if(fp == NULL)
	 {
		 MessageBox(NULL, "There is no file", "SkillData.csv", NULL);

		 return;
	 }	

	 char str[1024];

     memset(g_pSpell, 0, sizeof(g_pSpell));


	 while(1)
	 {
		char * ret = fgets((char*)str, 1024, fp);

		if(ret == NULL)
			break;     

		for(int i = 0; i < 1024; i++)
		{
			if (str[i] == ',')
				str[i] = ' ';

		    if(str[i] == -0)
				break;
		}

		STRUCT_SPELL temp;
		
		memset(&temp, 0, sizeof(temp));

        int Index = -1;
        char skilldata[256];

		sscanf(str, "%d %d %d %d %d %d %d %d %d %d %d %d %d %s %s %d %d %d %d %d %d %d", 
								&Index,				&temp.SkillPoint,	&temp.TargetType,	&temp.ManaSpent,	&temp.Delay,
								&temp.Range,		&temp.InstanceType,	&temp.InstanceValue,&temp.TickType,		&temp.TickValue,
								&temp.AffectType,   &temp.AffectValue, &temp.AffectTime,    skilldata,			skilldata, &temp.InstanceAttribute,
								&temp.TickAttribute,&temp.Aggressive,	&temp.MaxTarget,	&temp.bParty,		&temp.AffectResist,&temp.Passive);
		
		if(Index < 0 || Index >= MAX_SKILLINDEX)
			continue;

		temp.AffectTime = temp.AffectTime / 4;

		temp.Act[0] = 0;
		temp.Act[1] = 0;
		temp.Act[2] = 0;
		temp.Act[3] = 0;
		temp.Act[4] = 0;
		temp.Act[5] = 0;
		temp.Act[6] = 0;
		temp.Act[7] = 0;

		sscanf(skilldata, "%hhd.%hhd.%hhd.%hhd.%hhd.%hhd", &temp.Act[0], &temp.Act[1], &temp.Act[2], &temp.Act[3], &temp.Act[4], &temp.Act[5]);

		g_pSpell[Index] = temp;
	 }

	 fclose(fp);

	 return;
}

int IsClearString3(char *str, int nTarget)
{
	if(str == NULL)
		return TRUE;

	char *pNextRightChar = CharNext(&str[nTarget]);

	if	(str+nTarget+1 != pNextRightChar)
		return FALSE;

	return TRUE;
}

void BASE_InitializeClientGuildName(int group)
{
	FILE *fp = NULL;

	fp = fopen("../../Common/Guilds.txt", "rb");

	if(fp == NULL)
		fp = fopen("./Guilds.txt", "rb");

	if(fp == NULL)
		fp = fopen("../../TMSRV/Run/Guilds.txt", "rb");

	if(fp == NULL)
		return;

	memset(g_pClientGuildName, 0, sizeof(g_pClientGuildName));

	while(1)
	{
		int Group = -1;
		int Server = -1;
		int Guild = -1;

		char szGuild[4][4096];
		char str[1024];


		char *ret = fgets((char*)str, 1024, fp);

		if(ret == NULL)
			break;

		sscanf(str, "%d %d %d %s %s %s %s", &Group, &Server, &Guild, szGuild[0], szGuild[1], szGuild[2], szGuild[3]);

		if((Group < 0 || Group >= MAX_SERVERGROUP || Server < 0 || Server >= 16) || (Guild < 0 || Guild >= MAX_GUILD) || szGuild[0][0] == 0)
		{
			MessageBoxA(NULL, str, "Can`t parse string in Guilds.txt", MB_OK);
			continue;
		}

		if(Group != group)
			continue;

		for(int i = 0; i < 4; i++)
		{
			int len = strlen(szGuild[i]);

			if(len > 11)
			{
				szGuild[i][11] = 0;
				len = 11;
			}
			else if(len > 0)
			{
				if(!IsClearString3(szGuild[i], len - 1))
					szGuild[i][len - 1] = 0;
			}

			strcpy(g_pClientGuildName[Server][Guild][i], szGuild[i]);
		}
	}

	fclose(fp);

	CurrentClientGuild = group;
}

void BASE_InitializeGuildName()
{
	FILE *fp = fopen("../../Common/Guilds.txt", "rb");

	if(fp == NULL)
		fp = fopen("./Guilds.txt", "rb");

	if(fp == NULL)
		fp = fopen("../../TMSRV/Run/Guilds.txt", "rb");

	if(fp == NULL)
	{
		MessageBox(0, "can't find guilds.txt", "error", MB_OK | MB_SYSTEMMODAL);
		memset(g_pGuildName, 0, sizeof(g_pGuildName));

		return;
	}

	int nSize = sizeof(g_pGuildName);

	memset(g_pGuildName, 0, nSize);

	char str[1024];

	while(1)
	{
		int Group = -1;
		int Server = -1;
		int Guild = -1;
		char szGuild[256] = {0,};

		char *ret = fgets(str, 1024, fp);

		if(ret == NULL)
			break;

		sscanf(str, "%d %d %d %s", &Group, &Server, &Guild, szGuild);

		if((Group < 0 || Group >= MAX_SERVERGROUP) || (Server < 0 || Server >= 16) || (Guild < 0 || Guild >= MAX_GUILD) || szGuild[0] == 0)
		{
			MessageBox(0, str, "Can't parse string in Guilds.txt", 0);

			break;
		}

		int len = strlen(szGuild);

		if(len > GUILDNAME_LENGTH-1)
		{
			szGuild[GUILDNAME_LENGTH-1] = 0;
			len = GUILDNAME_LENGTH-1;
		}

		if(len > 0)
		{
			if(!IsClearString3(szGuild, len - 1))
				ret[3 + len] = 0;
		}

		strncpy(g_pGuildName[Group][Server][Guild], szGuild, GUILDNAME_LENGTH);
	};

	fclose(fp);
}

int BASE_InitializeServerList()
{
	FILE *fp = NULL;

	fp = fopen("./serverlist.txt", "rt");

	if(fp == NULL)
		fp = fopen("../../Common/serverlist.txt", "rt");

	if(fp == NULL)
	{
		MessageBoxA(NULL, "Can't open server list.txt", "adress", MB_OK);

		return FALSE;
	}
	
	memset(g_pServerList, 0, sizeof(g_pServerList));

	char str[256];
	char address[64];

	int ServerGroup;
	int ServerNumber;

	while(1)
	{
		char *ret = fgets((char*)str, 255, fp);

		if(ret == NULL)
			break;

		ServerGroup = -1;
		ServerNumber = -1;

		address[0] = 0;

		sscanf(str, "%d %d %s", &ServerGroup, &ServerNumber, address);

		if((ServerGroup < 0 ||ServerGroup >= MAX_SERVERGROUP) || (ServerNumber < 0 || ServerNumber > MAX_SERVERNUMBER))
			  break;

		strcpy(g_pServerList[ServerGroup][ServerNumber], address);
	};

	fclose(fp);

	FILE *fpBin = fopen("./serverlist.bin", "w");

	if(fp != NULL)
	{
		int szList[64] = {
			0xA4, 0xA1, 0xA4, 0xA4, 0xA4, 0xA7, 0xA4, 0xA9, 0xA4, 0xB1, 0xA4, 0xB2, 0xA4, 0xB5, 0xA4, 0xB7, 
			0xA4, 0xB8, 0xA4, 0xBA, 0xA4, 0xBB, 0xA4, 0xBC, 0xA4, 0xBD, 0xA4, 0xBE, 0xA4, 0xBF, 0xA4, 0xC1, 
			0xA4, 0xC3, 0xA4, 0xC5, 0xA4, 0xC7, 0xA4, 0xCB, 0xA4, 0xCC, 0xA4, 0xD0, 0xA4, 0xD1, 0xA4, 0xD3, 
			0xA4, 0xBF, 0xA4, 0xC4, 0xA4, 0xD3, 0xA4, 0xC7, 0xA4, 0xCC, 0xB0, 0xA1, 0xB3, 0xAA, 0xB4, 0xD9
		};

		for(int k = 0; k < MAX_SERVERGROUP; k++)
		{
			for(int j = 0; j < MAX_SERVERNUMBER; j++)
			{
				for(int i = 0; i < 64; i++)
				{
					g_pServerList[k][j][i] += szList[63-i];

				}
			}
		}

		fwrite(g_pServerList, MAX_SERVERGROUP*MAX_SERVERNUMBER, 64, fpBin);
		fclose(fpBin);

		for(int k = 0; k < MAX_SERVERGROUP; k++)
		{
			for(int j = 0; j < MAX_SERVERNUMBER; j++)
			{
				for(int i = 0; i < 64; i++)
				{
					g_pServerList[k][j][i] -= szList[63-i];
				}
			}
		}

	}

	return TRUE;
		
}

int BASE_InitializeAttribute()
{
	int tsum = 0;
	int sum = 0;

	char FileName[128];

	strcpy(FileName, "./AttributeMap.dat");

	FILE *fp = NULL;

	fp = fopen(FileName, "rb");

	if (fp == NULL)
		fp = fopen("../../TMSRV/Run/AttributeMap.dat", "rb");
	
	if(fp == NULL)
	{
		MessageBoxA(NULL, "There is no file", "AttributeMap.dat", MB_OK);

		return FALSE;
	}

	fread(g_pAttribute, 1024, 1024, fp);

	fclose(fp);

	return TRUE;
}

int BASE_GetHttpRequest(char *httpname, char *Request, int MaxBuffer)
{
	return 1;
}

int BASE_GetSkillDamage(int skillnum, STRUCT_MOB *mob, int weather, int weapondamage)
{
	int instanceindex = g_pSpell[skillnum].InstanceType;

	int kind = skillnum % MAX_SKILL / 8 + 1;

	int level = mob->CurrentScore.Level;

	if(level < 0)
		level = 0;

	if(level >= MAX_LEVEL)
		level = MAX_LEVEL;

	int special = mob->CurrentScore.Special[kind];

	int base = g_pSpell[skillnum].InstanceValue;
	int affectbase = g_pSpell[skillnum].AffectValue;

	int dam = 0;

	if(instanceindex == 0)
	{
		if(skillnum == 11)		
			dam = special / 10 + affectbase;

		else if(skillnum == 13)		
			dam = 3 * special / 4 + affectbase;

		else if(skillnum == 41)		
			dam = special / 25 + 2;

		else if(skillnum == 43)		
			dam = special / 3 + affectbase;

		else if(skillnum == 44)		
			dam = 2 * (3 * special / 20 + affectbase);

		else if(skillnum == 45)
			dam = special / 10 + affectbase;
	}

	else if(instanceindex >= 1 && instanceindex <= 5)
	{
		int skind = skillnum / 8;
		
		if(skillnum == 97)
			dam = 15 * level + base;
		
		else if(mob->Class == 0 && skind == 1) // TK
			dam =  3 * weapondamage + 3 * mob->CurrentScore.Str + level + special + base;
		
		else if(mob->Class == 0 && skind != 1) // TK
			dam = special + base + weapondamage + level + mob->CurrentScore.Int / 4 + mob->CurrentScore.Int / 40;
		
		else if(mob->Class == 1)//Foema
			dam = mob->CurrentScore.Int / 30 + mob->CurrentScore.Int / 3 + level + base + 2 * special;
		
		else if(mob->Class == 2)//BeastMaster
			dam = mob->CurrentScore.Int / 30 + mob->CurrentScore.Int / 3 + level + base + 2 * special;
		
		else if(mob->Class == 3)//Huntress
			dam = 3 * weapondamage + 3 * mob->CurrentScore.Str + level / 2 + special + base;		

		if(weather == 1)
		{
			if(instanceindex == 2)
				dam = 90 * dam / 100;
			
			if(instanceindex == 5)
				dam = 130 * dam / 100;
		}
		
		else if(weather == 2 && instanceindex == 3)
			dam = 120 * dam / 100;


		if ((mob->Class || skind != 1) && mob->Class != 3)
        {
			dam = (4 * mob->Magic + 100) * dam / 100;
			dam = 5 * dam / 4;
        }
        else
			dam = 5 * dam / 4;
	}

	else if(instanceindex == 6)
		dam = 3 * special / 2 + base;
	
	else if(instanceindex == 11)
		dam = base;
	
	else
		dam = mob->Magic;

	if (skillnum == 79) // Tempestade de raios
		dam = mob->CurrentScore.Damage*180/100;

	return dam;
}

int BASE_GetEnglish(char *name)
{
	char temp[256];
	strncpy(temp, name, sizeof(name));
    
	for(int i = 0; i < MAX_ENGLISH; i++)
	{
		if(!strcmp(name, g_pEnglish[i][0]))
		{ 
			strcpy(name, g_pEnglish[i][2]);

		   return TRUE;
		}
	}

    return FALSE;
}

int BASE_GetWeekNumber()
{
	time_t now;

	time(&now);

	unsigned int week = 86400;

	int ret = (int)(now / week - 3);

	return ret;
}

BOOL BASE_CheckPacket(MSG_STANDARD *m)
{/*
	if (m == NULL)
		return 0;

	int code = 0;
	
	if	(m->Type==  _MSG_MessagePanel				&& m->Size != sizeof( MSG_MessagePanel			))code = 1;
	if	(m->Type==  _MSG_MessageBoxOk 				&& m->Size != sizeof( MSG_MessageBoxOk 			))code = 1;
	if	(m->Type==  _MSG_DBNewAccount   				&& m->Size != sizeof( MSG_NewAccount   		))code = 1;
	if	(m->Type==  _MSG_AccountLogin   			&& m->Size != sizeof( MSG_AccountLogin   		))code = 1;
	if	(m->Type==  _MSG_CNFAccountLogin 			&& m->Size != sizeof( MSG_DBCNFAccountLogin 	))code = 1;
	if  (m->Type ==  _MSG_CreateCharacter				&& m->Size != sizeof(MSG_CreateCharacter	))code = 1;
	if	(m->Type==  _MSG_CNFNewCharacter			&& m->Size != sizeof( MSG_CNFNewCharacter		))code = 1;
	if	(m->Type==  _MSG_DeleteCharacter			&& m->Size != sizeof( MSG_DeleteCharacter		))code = 1;
	if	(m->Type==  _MSG_CNFDeleteCharacter 		&& m->Size != sizeof( MSG_CNFDeleteCharacter 	))code = 1;
	if	(m->Type==  _MSG_CharacterLogin  			&& m->Size != sizeof( MSG_CharacterLogin  		))code = 1;
	if	(m->Type==  _MSG_CNFCharacterLogin 			&& m->Size != sizeof( MSG_CNFCharacterLogin 	))code = 1;
	if	(m->Type==  _MSG_CNFCharacterLogout 		&& m->Size != sizeof( MSG_STANDARD 				))code = 1;
	if	(m->Type==  _MSG_NewAccountFail    			&& m->Size != sizeof( MSG_STANDARD    			))code = 1;
	if  (m->Type == _MSG_DBAccountLoginFail_Account	&& m->Size != sizeof(MSG_STANDARD				))code = 1;
	if	(m->Type==  _MSG_CharacterLoginFail			&& m->Size != sizeof( MSG_STANDARD  			))code = 1;
	if	(m->Type==  _MSG_NewCharacterFail   		&& m->Size != sizeof( MSG_STANDARD   			))code = 1;
	if	(m->Type==  _MSG_DeleteCharacterFail		&& m->Size != sizeof( MSG_STANDARD 				))code = 1;
	if	(m->Type==  _MSG_AlreadyPlaying   			&& m->Size != sizeof( MSG_STANDARD   			))code = 1;
	if	(m->Type==  _MSG_StillPlaying   			&& m->Size != sizeof( MSG_STANDARD   			))code = 1;
	if	(m->Type==  _MSG_MessageChat   				&& m->Size != sizeof( MSG_MessageChat   		))code = 1;
	if	(m->Type==  _MSG_MessageWhisper				&& m->Size != sizeof( MSG_MessageWhisper		))code = 1;
	if	(m->Type==  _MSG_UpdateScore  				&& m->Size != sizeof( MSG_UpdateScore  			))code = 1;
	if	(m->Type==  _MSG_UpdateEtc					&& m->Size != sizeof( MSG_UpdateEtc				))code = 1;
	if	(m->Type==  _MSG_CNFMobKill   				&& m->Size != sizeof( MSG_CNFMobKill   			))code = 1;
	if	(m->Type==  _MSG_CreateMob  				&& m->Size != sizeof( MSG_CreateMob  			))code = 1;
	if	(m->Type==  _MSG_RemoveMob					&& m->Size != sizeof( MSG_RemoveMob				))code = 1;
	if	(m->Type==  _MSG_Action						&& m->Size != sizeof( MSG_Action				))code = 1;
//	if	(m->Type==  _MSG_REQMobByID					&& m->Size != sizeof( MSG_REQMobByID			))code = 1;
	if	(m->Type==  _MSG_Motion    					&& m->Size != sizeof( MSG_Motion    			))code = 1;
	if	(m->Type==  _MSG_UpdateEquip				&& m->Size != sizeof( MSG_UpdateEquip			))code = 1;
	if	(m->Type==  _MSG_Attack						&& m->Size != sizeof( MSG_Attack				))code = 1;
//	if	(m->Type==  _MSG_Stop          				&& m->Size != sizeof( MSG_Action          		))code = 1;
	if	(m->Type==  _MSG_CreateItem   				&& m->Size != sizeof( MSG_CreateItem   			))code = 1;
//	if	(m->Type==  _MSG_RemoveItem					&& m->Size != sizeof( MSG_RemoveItem			))code = 1;
	if	(m->Type==  _MSG_GetItem					&& m->Size != sizeof( MSG_GetItem				))code = 1;
	if	(m->Type==  _MSG_CNFGetItem					&& m->Size != sizeof( MSG_CNFGetItem			))code = 1;
	if	(m->Type==  _MSG_DropItem 					&& m->Size != sizeof( MSG_DropItem 				))code = 1;
	if	(m->Type==  _MSG_UseItem					&& m->Size != sizeof( MSG_UseItem				))code = 1;
	if	(m->Type==  _MSG_UpdateItem 				&& m->Size != sizeof( MSG_UpdateItem 			))code = 1;
	if	(m->Type==  _MSG_CNFDropItem 				&& m->Size != sizeof( MSG_CNFDropItem 			))code = 1;
//	if	(m->Type==  _MSG_SwapItem					&& m->Size != sizeof( MSG_SwapItem				))code = 1;
	if	(m->Type==  _MSG_ApplyBonus					&& m->Size != sizeof( MSG_ApplyBonus			))code = 1;
	if	(m->Type==  _MSG_SetShortSkill				&& m->Size != sizeof( MSG_SetShortSkill			))code = 1;
	if	(m->Type==  _MSG_Buy						&& m->Size != sizeof( MSG_Buy					))code = 1;
	if	(m->Type==  _MSG_Sell						&& m->Size != sizeof( MSG_Sell					))code = 1;
	if	(m->Type==  _MSG_REQShopList				&& m->Size != sizeof( MSG_REQShopList			))code = 1;
	if	(m->Type==  _MSG_ShopList					&& m->Size != sizeof( MSG_ShopList				))code = 1;
//	if	(m->Type==  _MSG_AddParty					&& m->Size != sizeof( MSG_AddParty				))code = 1;
	if	(m->Type==  _MSG_RemoveParty				&& m->Size != sizeof( MSG_RemoveParty			))code = 1;
	if (m->Type == _MSG_SendReqParty				&& m->Size != sizeof(MSG_SendReqParty))code = 1;
	if	(m->Type==  _MSG_CNFAddParty				&& m->Size != sizeof( MSG_CNFAddParty				))code = 1;
	if	(m->Type==  _MSG_SetHpMp					&& m->Size != sizeof( MSG_SetHpMp				))code = 1;
	if	(m->Type==  _MSG_SendItem					&& m->Size != sizeof( MSG_SendItem				))code = 1;
	if	(m->Type==  _MSG_Trade						&& m->Size != sizeof( MSG_Trade					))code = 1;
	if	(m->Type==  _MSG_QuitTrade					&& m->Size != sizeof( MSG_STANDARD				))code = 1;
	if (m->Type == _MSG_TradingItem						&& m->Size != sizeof(MSG_TradingItem		))code = 1;
	if	(m->Type==  _MSG_CNFCheck					&& m->Size != sizeof( MSG_STANDARD				))code = 1;
	if	(m->Type==  _MSG_Withdraw        			&& m->Size != sizeof( MSG_STANDARDPARM        	))code = 1;
	if	(m->Type==  _MSG_Deposit					&& m->Size != sizeof( MSG_STANDARDPARM			))code = 1;
	if	(m->Type==  _MSG_Restart					&& m->Size != sizeof( MSG_STANDARD 				))code = 1;
	if	(m->Type==  _MSG_SetHpDam					&& m->Size != sizeof( MSG_SetHpDam				))code = 1;
	if (m->Type == _MSG_UpdateWeather          			&& m->Size != sizeof(MSG_UpdateWeather))code = 1;
	if	(m->Type==  _MSG_Quest             			&& m->Size != sizeof( MSG_STANDARDPARM2         ))code = 1;
	if	(m->Type==  _MSG_Deprivate        			&& m->Size != sizeof( MSG_STANDARDPARM       	))code = 1;
	if	(m->Type==  _MSG_ReqChallange       		&& m->Size != sizeof( MSG_STANDARD       		))code = 1;
	if	(m->Type==  _MSG_Challange        			&& m->Size != sizeof( MSG_STANDARDPARM        	))code = 1;
	if	(m->Type==  _MSG_ChallangeConfirm  			&& m->Size != sizeof( MSG_STANDARDPARM2  		))code = 1;
	if	(m->Type==  _MSG_ReqTeleport       			&& m->Size != sizeof( MSG_STANDARDPARM       	))code = 1;
	if (m->Type ==  _MSG_ChangeCity      			&& m->Size != sizeof(MSG_STANDARDPARM))code = 1;
	if	(m->Type==  _MSG_SetHpMode           		&& m->Size != sizeof( MSG_STANDARDPARM          ))code = 1;
	if	(m->Type==  _MSG_SetClan             		&& m->Size != sizeof( MSG_STANDARDPARM          ))code = 1;
//	if	(m->Type==  _MSG_BillingPage				&& m->Size != sizeof( MSG_STANDARDPARM          ))code = 1;
	if	(m->Type==  _MSG_DBMessagePanel				&& m->Size != sizeof( MSG_MessagePanel			))code = 1;
	if	(m->Type==  _MSG_DBMessageBoxOk				&& m->Size != sizeof( MSG_MessageBoxOk       	))code = 1;
//	if	(m->Type==  _MSG_DBCNFNewAccount			&& m->Size != sizeof( MSG_DBCNFNewAccount		))code = 1; // À¥¿¡¼­¸¸Áö¿ø
	if	(m->Type==  _MSG_DBCNFAccountLogin			&& m->Size != sizeof( MSG_DBCNFAccountLogin    	))code = 1;
	if	(m->Type==  _MSG_DBCNFCharacterLogin		&& m->Size != sizeof( MSG_CNFCharacterLogin  	))code = 1;
	if	(m->Type==  _MSG_DBCNFNewCharacter			&& m->Size != sizeof( MSG_CNFNewCharacter    	))code = 1;
	if	(m->Type==  _MSG_DBCNFDeleteCharacter		&& m->Size != sizeof( MSG_CNFDeleteCharacter 	))code = 1;
	if	(m->Type==  _MSG_DBNewAccountFail			&& m->Size != sizeof( MSG_STANDARD     	        ))code = 1;
//	if	(m->Type==  _MSG_DBAccountLoginFail			&& m->Size != sizeof( MSG_STANDARD   	        ))code = 1;
	if	(m->Type==  _MSG_DBCharacterLoginFail		&& m->Size != sizeof( MSG_STANDARD 	            ))code = 1;
	if	(m->Type==  _MSG_DBNewCharacterFail			&& m->Size != sizeof( MSG_STANDARD   	        ))code = 1;
	if	(m->Type==  _MSG_DBDeleteCharacterFail		&& m->Size != sizeof( MSG_STANDARD			    ))code = 1;
	if	(m->Type==  _MSG_DBAlreadyPlaying			&& m->Size != sizeof( MSG_STANDARD 	            ))code = 1;
	if	(m->Type==  _MSG_DBStillPlaying				&& m->Size != sizeof( MSG_STANDARD   	        ))code = 1;
	if	(m->Type==  _MSG_DBAccountLoginFail_Account && m->Size != sizeof( MSG_STANDARD				))code = 1;
	if	(m->Type==  _MSG_DBAccountLoginFail_Pass    && m->Size != sizeof( MSG_STANDARD				))code = 1;
	if	(m->Type==  _MSG_DBSetIndex					&& m->Size != sizeof( MSG_STANDARDPARM3			))code = 1;
	if	(m->Type==  _MSG_DBAccountLoginFail_Block	&& m->Size != sizeof( MSG_STANDARD				))code = 1;
	if	(m->Type==  _MSG_DBAccountLoginFail_Disable	&& m->Size != sizeof( MSG_STANDARD				))code = 1;
	if	(m->Type==  _MSG_DBNewAccount				&& m->Size != sizeof(MSG_NewAccount				))code = 1;
	if	(m->Type==  _MSG_DBCreateCharacter			&& m->Size != sizeof(MSG_CreateCharacter     		))code = 1;
	if	(m->Type==  _MSG_DBAccountLogin				&& m->Size != sizeof(MSG_AccountLogin     		))code = 1;
	if	(m->Type==  _MSG_DBCharacterLogin			&& m->Size != sizeof(MSG_CharacterLogin			))code = 1;
	if	(m->Type==  _MSG_DBNoNeedSave          		&& m->Size != sizeof(MSG_STANDARD       		))code = 1;
	if	(m->Type==  _MSG_DBSaveMob             		&& m->Size != sizeof(MSG_DBSaveMob				))code = 1;
	if	(m->Type==  _MSG_DBDeleteCharacter     		&& m->Size != sizeof(MSG_DeleteCharacter		))code = 1;
	if	(m->Type==  _MSG_NPReqIDPASS           		&& m->Size != sizeof(MSG_NPIDPASS        		))code = 1;
	if	(m->Type==  _MSG_NPIDPASS              		&& m->Size != sizeof(MSG_NPIDPASS           	))code = 1;
	if	(m->Type==  _MSG_NPReqAccount          		&& m->Size != sizeof(MSG_NPReqAccount       	))code = 1;
	if	(m->Type==  _MSG_NPNotFound            		&& m->Size != sizeof(MSG_STANDARD         		))code = 1;
	if	(m->Type==  _MSG_NPAccountInfo         		&& m->Size != sizeof(MSG_NPAccountInfo      	))code = 1;
	if	(m->Type==  _MSG_NPReqSaveAccount      		&& m->Size != sizeof(MSG_NPAccountInfo   		))code = 1;
	if	(m->Type==  _MSG_NPDisable             		&& m->Size != sizeof(MSG_NPEnable          		))code = 1;
	if	(m->Type==  _MSG_NPEnable              		&& m->Size != sizeof(MSG_NPEnable           	))code = 1;
	if	(m->Type==  _MSG_NPNotice              		&& m->Size != sizeof(MSG_NPNotice				))code = 1;
	if	(m->Type==  _MSG_NPState               		&& m->Size != sizeof(MSG_STANDARDPARM			))code = 1;
	if	(m->Type==  _MSG_War						&& m->Size != sizeof(MSG_STANDARDPARM2			))code = 1;
	if	(m->Type==	_MSG_ReqTransper				&& m->Size != sizeof(MSG_ReqTransper			))code = 1;
	
	return code;
	*/
	return FALSE;
}

int  BASE_VisualItemCode(STRUCT_ITEM *Item, int mnt)
{
	int value = 0;

	if (mnt == 14)
	{// montaria
		if (Item->sIndex >= 3980 && Item->sIndex < 3995)
			return Item->sIndex;

		if (Item->sIndex >= 2360 && Item->sIndex < 2360)
		{
			if (Item->stEffect[0].sValue > 0)
			{
				value = Item->stEffect[1].cEffect/10;
			}
			else
				return 0;
		}

		return Item->sIndex | (value * 0x1000);
	}
	else
	{
		if (Item->stEffect[0].cEffect == 43)
			value = Item->stEffect[0].cValue;
		else if (Item->stEffect[1].cEffect == 43)
			value = Item->stEffect[1].cValue;
		else if (Item->stEffect[2].cEffect == 43)
			value = Item->stEffect[2].cValue;

		else if(Item->stEffect[0].cEffect >= 116 && Item->stEffect[0].cEffect <= 125)
				return Item->sIndex | (12 * 0x1000);

		else if(Item->stEffect[1].cEffect >= 116 && Item->stEffect[1].cEffect <= 125)
				return Item->sIndex | (12 * 0x1000);

		else if(Item->stEffect[2].cEffect >= 116 && Item->stEffect[2].cEffect <= 125)
				return Item->sIndex | (12 * 0x1000);

		else
			return Item->sIndex;
	}

	if (value < 230)
		value %= 10;

	else if (value < 234)
		value = 10;
	else if (value < 238)
		value = 11;
	else if (value < 242)
		value = 12;
	else if (value < 246)
		value = 13;
	else if (value < 250)
		value = 14;
	else if (value < 254)
		value = 15;
	else// value < 256
		value = 16;

	return Item->sIndex | (value * 0x1000);
}

int BASE_VisualAnctCode(STRUCT_ITEM *Item)
{
	int value = 0;

	if (Item->sIndex >= 2360 && Item->sIndex <= 2390 && Item->stEffect[0].sValue > 0)
	{
		switch (Item->stEffect[2].cValue)
		{
		case 11:
			return 0x10B;
		case 12:
			return 0x10C;
		case 13:
			return 0x10D;
		case 14:
			return 0x10E;
		case 15:
			return 0x10F;
		case 16:
			return 0x110;
		case 17:
			return 0x111;
		case 18:
			return 0x112;
		case 19:
			return 0x113;
		case 20:
			return 0x114;

		case 35:
			if (Item->sIndex == 2363 || Item->sIndex == 2377)
				return 0x115;
			break;

		default:
			return 0x00;
		}

		return 0;
	}
	if (Item->stEffect[0].cEffect == 43)
		value = Item->stEffect[0].cValue;
	else if (Item->stEffect[1].cEffect == 43)
		value = Item->stEffect[1].cValue;
	else if (Item->stEffect[2].cEffect == 43)
		value = Item->stEffect[2].cValue;


	if (Item->stEffect[0].cEffect >= 116 && Item->stEffect[0].cEffect <= 125)
		return value = Item->stEffect[0].cEffect-3;
	if (Item->stEffect[1].cEffect >= 116 && Item->stEffect[1].cEffect <= 125)
		return value = Item->stEffect[1].cEffect-3;
	if (Item->stEffect[2].cEffect >= 116 && Item->stEffect[2].cEffect <= 125)
		return value = Item->stEffect[2].cEffect-3;

	if (value == 0)
		return 0;

	if (value < 230)
		return 43;

	switch (value % 4)
	{
	case 0:  return 0x30;
	case 1:  return 0x40;
	case 2:  return 0x10;
	default: return 0x20;
	}
}

void BASE_SetItemDate(STRUCT_ITEM *Item, int day)
{
	struct tm when;
	time_t now;
	time(&now);
	when = *localtime(&now);

	int daynext = when.tm_mday + day;
	int month = day >= 30 ? when.tm_mon + 1 : when.tm_mon;
	int year = month >= 12 ? when.tm_year+1 : when.tm_year;


	if(month >= 12)
		month = 0;

	if(daynext >= 30)
		daynext -= 29;

	if(month == 1 && daynext >= 27)
		daynext -= 26;

	Item->stEffect[0].cEffect = EF_WDAY;
	Item->stEffect[0].cValue = daynext;

	Item->stEffect[1].cEffect = EF_WMONTH;
	Item->stEffect[1].cValue = month + 1;

	Item->stEffect[2].cEffect = EF_YEAR;
	Item->stEffect[2].cValue = year - 100;
}

int  BASE_CheckItemDate(STRUCT_ITEM *Item)
{
	struct tm when;
	time_t now;
	time(&now);
	when = *localtime(&now);

	if(when.tm_mday >= Item->stEffect[0].cValue && when.tm_mon >= (Item->stEffect[1].cValue-1) && (when.tm_year-100) >= Item->stEffect[2].cValue)
		return 1;

	else if(when.tm_mon > Item->stEffect[1].cValue && (when.tm_year-100) >= Item->stEffect[2].cValue)
		return 1;

	else if((when.tm_year-100) > Item->stEffect[2].cValue)
		return 1;

	return 0;
}

void  BASE_CheckFairyDate(STRUCT_ITEM *Item)
{
	if(Item->sIndex < 3900 || Item->sIndex > 3913)
		return;

	int day = Item->stEffect[0].cValue;
	int hour = Item->stEffect[1].cValue;
	int min = Item->stEffect[2].cValue;

	if(day == 0 && hour == 0 && min <= 1)
	{
		BASE_ClearItem(Item);
		return;
	}
	else if(day >= 1 && hour == 0 && min == 1)
	{
		day--;
		hour = 23;
		min = 59;
	}

	else if(hour >= 1 && min == 1)
	{
		hour--;
		min = 59;
	}
	else if(min >= 2)
		min--;

	Item->stEffect[0].cValue = day;
	Item->stEffect[1].cValue = hour;
	Item->stEffect[2].cValue = min;
}