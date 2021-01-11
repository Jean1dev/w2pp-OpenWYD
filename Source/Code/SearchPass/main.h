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

#include <Windows.h>

#include <fcntl.h>
#include <io.h>

#include <ctime>
#include <cstdio>
#include <cstdlib>
#include <cstdarg>
#include <stdio.h>

#include <string>
#include <vector>
#include <list>
#include <map>

#define IDC_EXIT 9000
#define IDC_MOBRELOAD 9001
#define IDC_REBOOT 9002
#define IDC_SAVEALL 9003
#define IDC_READGUILD 9004
#define IDC_READGUILDNAME 9005
#define IDC_READGAMECONFIG 9006

#define IDC_SHUTDOWNNP 9050


#define		APP_VERSION				3590

#define		GAME_PORT				8281	  // Game server listening port, client connects to it
#define		DB_PORT					7514      // DB server listening port, Game server connects to it
#define		ADMIN_PORT				8895	  // DB server listening port, NPTool connects to it

#define		MAX_SERVER				10      // Max number of game servers that can connect to DB server
#define		MAX_SERVERGROUP			10		// Max number of servers that can exist
#define		MAX_SERVERNUMBER		(MAX_SERVER+1) // DB + TMSrvs + BISrv

#define		MAX_IP_LENGTH			16

#define		SECSTANDINGBY			8
#define		SECBATTLE				8
#define		MAX_USER				1000      // Max users on CUser pUser and starting index of npcs and mobs
#define		ADMIN_RESERV			10		  // Max reserv slot in gameserver for admin's
#define		MAX_ADMIN				10        // Max number of NP that can connect to DB server

#define		MAX_DBACCOUNT			(MAX_USER * MAX_SERVER) // Max players connected to a single DBSrv at the same time
#define		MAX_RANK_INDEX			500

#pragma region Account related defines

#define		ACCOUNTNAME_LENGTH		16		// Max length of the account login
#define		ACCOUNTPASS_LENGTH		12      // Max length of the account password
#define		REALNAME_LENGTH			24      // Max length of the name of the account owner
#define		EMAIL_LENGTH			48      // Max length of the email address of the account owner
#define		ADDRESS_LENGTH			78      // Max length of the real address of the account owner
#define		TELEPHONE_LENGTH		16      // Max length of the telephone number of the account owner
#define		MOB_PER_ACCOUNT			4       // Max number of mobs an account can have

#pragma endregion

#define		MAX_EQUIP				16
#define		MAX_CARRY				64
#define		MAX_CARGO				128

#define		MAX_TRADE				15
#define		MAX_AUTOTRADE			12
#define		MAX_AUTOTRADETITLE		24
#define		MAX_SHOPLIST			27

#define		MAX_GROUNDMASK			6

#define		ITEM_PLACE_EQUIP		0
#define		ITEM_PLACE_CARRY		1
#define		ITEM_PLACE_CARGO		2

#define		CARGOGRIDX				9
#define		CARGOGRIDY				14
#define		CARRYGRIDX				9
#define		CARRYGRIDY				7

#define		VIEWGRIDX				33
#define		VIEWGRIDY				33
#define		HALFGRIDX				16
#define		HALFGRIDY				16

#define		MAX_GRIDX				4096
#define		MAX_GRIDY				4096

#define		MAX_ITEM				5000

#define		MAX_SUMMONLIST			50

#define		MAX_MOB					25000

#define		MAX_NPCGENERATOR		8192

#define		MAX_SKILL				24

#define		MAX_COMBINE				8

#define		MAX_CLASS				4

#define		MAX_LEVEL				399
#define		MAX_CLEVEL				199

#define		MAX_CLASSMASTER			4

#define		MAX_AFFECT				32

#define		MAX_ROUTE				24

#define		MAX_TRANS				20480

#define		MAX_NOTIFY_LENGTH		96
#define		MESSAGE_LENGTH			96 
#define		MESSAGEWHISPER_LENGTH	100 

#define		NAME_LENGTH				16

#define		MH						8

#define		TIMER_SEC				0
#define		TIMER_MIN				1

#define		MAX_ITEMLIST			6500
#define		MAX_SKILLINDEX			103
#define		MAX_INITITEM			256
#define		MAX_EFFECTINDEX			128
#define		ITEMNAME_LENGTH			64
#define		MAX_STATICEFFECT		12

#define		GUILDNAME_LENGTH		12
#define		MAX_GUILD				4096
#define		MAX_GUILDZONE			5

#define		MAX_STRING				2000
#define		MAX_ENGLISH				600
#define		MAX_MAC					200
#define		MAX_ACCOUNTMAC			2

#define     MAX_CASTLE_POS			9
#define		MAX_CASTLE_QUEST		64

#define		MAX_MOB_MERC			128

#define		MAX_CAMP_QUEST			64

#define		MAX_CAMP_PRIZE			12

#define		MAX_PIN_LENGTH			37

#define		KILL_MARK				63

#define		MAX_PARTY				12

#define		ESCENE_FIELD			30000 // Identifier for the server sent messages

#define		SKIPCHECKTICK			235543242 // base number for the tick checking so people don't get kicked out before doing something
#define		MAX_TARGET				13

#define		REINO_RED				8
#define		REINO_BLUE				7

#define		MORTAL					2
#define		ARCH					1
#define		CELESTIAL				3
#define		CELESTIALCS				4
#define		SCELESTIAL				5

#define		RSV_FROST    0x01
#define		RSV_DRAIN    0x02
#define		RSV_VISION   0x04
#define		RSV_PARRY    0x08
#define		RSV_HIDE	 0x10
#define		RSV_HASTE    0x20
#define		RSV_CAST     0x40
#define		RSV_BLOCK    0x80
#define		RSV_IMUNIDADE 0x100

#define		MSV_SNOOP	0x01

#define		REF_10					10
#define		REF_11					12
#define		REF_12					15
#define		REF_13					18
#define		REF_14					22
#define		REF_15					27

#define		MAX_HP					2000000000
#define		MAX_MP					2000000000
#define		MAX_DAMAGE				1000000
#define		MAX_DAMAGE_MG			254
#define		AFFECT_1H				450
#define		AFFECT_1D				10800

#define		SOUL_NULL				0
#define		SOUL_KIBITA				1

#define		SOUL_F					2
#define		SOUL_I					3
#define		SOUL_D					4
#define		SOUL_C					5

#define		SOUL_FI					6
#define		SOUL_FD					7
#define		SOUL_FC					8

#define		SOUL_IF					9
#define		SOUL_ID					10
#define		SOUL_IC					11

#define		SOUL_DF					12
#define		SOUL_DI					13
#define		SOUL_DC					14

#define		SOUL_CF					15
#define		SOUL_CI					16
#define		SOUL_CD					17

//////////////////// QST ////////////////////////////////////

#define	QST_AMUMISTICO			1

////////////////////MSG_Quest NPC's//////////////////////////

#define GOLD_DRAGON				1
#define AMU_MISTICO				2
#define MESTREHAB				3
#define EXPLOIT_LEADER			4
#define ARZAN_DRAGON			5
#define TREINADORNEWBIE1		6
#define TREINADORNEWBIE2		7
#define TREINADORNEWBIE3		8
#define TREINADORNEWBIE4		9
#define COMP_SEPHI				10
#define CAPAVERDE_TELEPORT		11
#define MOUNT_MASTER			12
#define ZAKUM					13
#define JEFFI					14
#define SHAMA					15
#define KING					16
#define KINGDOM					17
#define URNAMMU					18
#define CAPAVERDE_TRADE			19
#define QUEST_COVEIRO			20
#define QUEST_JARDINEIRO		21
#define QUEST_KAIZEN			22
#define QUEST_HIDRA				23
#define QUEST_ELFOS				24
#define UXMAL					25
#define MOLARGARGULA			26
#define BLACKORACLE				27
#define QUEST_CAPAREAL			28
#define CARBUNCLE_WIND			29
#define PERZEN					30
#define KIBITA					31
#define GODGOVERNMENT			32
#define SOBREVIVENTE			33
#define LIDER_APRENDIZ			34
#define GUARDA_REAL_EVT1		35

/////////////////////////////////////////////////////////////

///////////////////////NPCGENER IDS//////////////////////////

#define ZAKUM_LORD							4
#define ORC_GUERREIRO						3

#define REI_HARABARD						8
#define REI_GLANTUAR						9

#define TORRE_NOATUM1						23
#define TORRE_NOATUM2						24
#define TORRE_NOATUM3						25

#define WATER_M_INITIAL						10
#define WATER_N_INITIAL						171
#define WATER_A_INITIAL						183

#define SECRET_ROOM_N_INITIAL				2395

#define SECRET_ROOM_N_SALA1_MOB_1			2395
#define SECRET_ROOM_N_SALA1_MOB_2			2396
#define SECRET_ROOM_N_SALA2_MOB_1			2397
#define SECRET_ROOM_N_SALA2_MOB_2			2398
#define SECRET_ROOM_N_SALA3_MOB_1			2399
#define SECRET_ROOM_N_SALA3_MOB_2			2400
#define SECRET_ROOM_N_SALA4_MOB_1			2401
#define SECRET_ROOM_N_SALA4_MOB_2			2402
#define SECRET_ROOM_N_SALA4_MOB_3			2403
#define SECRET_ROOM_N_SALA4_MOB_4			2404

#define SECRET_ROOM_N_END					2402
#define SECRET_ROOM_N_BOSS_INITIAL			2403

#define SECRET_ROOM_M_INITIAL				2405

#define SECRET_ROOM_M_SALA1_MOB_1			2405
#define SECRET_ROOM_M_SALA1_MOB_2			2406
#define SECRET_ROOM_M_SALA2_MOB_1			2407
#define SECRET_ROOM_M_SALA2_MOB_2			2408
#define SECRET_ROOM_M_SALA3_MOB_1			2409
#define SECRET_ROOM_M_SALA3_MOB_2			2410
#define SECRET_ROOM_M_SALA4_MOB_1			2411
#define SECRET_ROOM_M_SALA4_MOB_2			2412
#define SECRET_ROOM_M_SALA4_MOB_3			2413
#define SECRET_ROOM_M_SALA4_MOB_4			2414

#define SECRET_ROOM_M_END					2412
#define SECRET_ROOM_M_BOSS_INITIAL			2413

#define SECRET_ROOM_A_INITIAL				2415

#define SECRET_ROOM_A_SALA1_MOB_1			2415
#define SECRET_ROOM_A_SALA1_MOB_2			2416
#define SECRET_ROOM_A_SALA2_MOB_1			2417
#define SECRET_ROOM_A_SALA2_MOB_2			2418
#define SECRET_ROOM_A_SALA3_MOB_1			2419
#define SECRET_ROOM_A_SALA3_MOB_2			2420
#define SECRET_ROOM_A_SALA4_MOB_1			2421
#define SECRET_ROOM_A_SALA4_MOB_2			2422
#define SECRET_ROOM_A_SALA4_MOB_3			2423
#define SECRET_ROOM_A_SALA4_MOB_4			2424

#define SECRET_ROOM_A_END					2422
#define SECRET_ROOM_A_BOSS_INITIAL			2423

#define NIGHTMARE_N_INITIAL					2368
#define NIGHTMARE_N_END						2375

#define NIGHTMARE_M_INITIAL					2377
#define NIGHTMARE_M_END						2384

#define NIGHTMARE_A_INITIAL					2385
#define NIGHTMARE_A_END						2394

#define RUNEQUEST_LV0_LICH1					5653
#define RUNEQUEST_LV0_LICH2					5654

#define RUNEQUEST_LV1_TORRE1				5706
#define RUNEQUEST_LV1_TORRE2				5707
#define RUNEQUEST_LV1_TORRE3				5708

#define RUNEQUEST_LV1_MOB_INITIAL			5709
#define RUNEQUEST_LV1_MOB_END				5764

#define RUNEQUEST_LV2_MOB_BOSS				5789
#define RUNEQUEST_LV2_MOB_INITIAL			5790
#define RUNEQUEST_LV2_MOB_END				5848

#define RUNEQUEST_LV3_MOB_SULRANG_INITIAL	5972
#define RUNEQUEST_LV3_MOB_SULRANG_END		5975

#define RUNEQUEST_LV3_MOB_BOSS_INITIAL		5948
#define RUNEQUEST_LV3_MOB_BOSS_END			5955

#define RUNEQUEST_LV4_MOB_INITIAL			5854
#define RUNEQUEST_LV4_MOB_END				5898
#define RUNEQUEST_LV4_MOB_BOSS				5849

#define RUNEQUEST_LV5_MOB_BOSS				5899

#define RUNEQUEST_LV6_MOB_INITIAL			5775
#define RUNEQUEST_LV6_MOB_END				5785
#define RUNEQUEST_LV6_MOB_INITIAL2			5768
#define RUNEQUEST_LV6_MOB_END2				5774
#define RUNEQUEST_LV6_MOB_BOSS				5767

#define RVRTORRE_1							4236
#define RVRTORRE_2							4237

#define GTORRE								1078

#define KEFRA_BOSS							396
#define KEFRA_MOB_INITIAL					397
#define KEFRA_MOB_END						400

////////////////////////////////////////////////////////////

#pragma endregion

#pragma region Structures

struct STRUCT_ITEM
{
	short sIndex;
	union
	{
		short sValue;

		struct
		{
			unsigned char cEffect;
			unsigned char cValue;
		};

	}stEffect[3];
};

struct STRUCT_SCORE
{
	int Level;	 // The mob's level
	int Ac;		 // The mob's defense
	int Damage;   // The mob's damage force

	unsigned char  Merchant; // UNK
	unsigned char  AttackRun; // The mob's speed
	unsigned char  Direction;
	unsigned char  ChaosRate;

	int MaxHp;     // The max HP the mob can have
	int MaxMp;	  // The max MP the mob can have
	int Hp;		  // The current HP of the mob
	int Mp;		  // The current MP of the mob

	short		   Str;		  // The mob's strength points, affects it's attack power
	short		   Int;		  // The mob's intelligence points, affects it's skill attack powers and MP
	short		   Dex;		  // The mob's dexterity points, affects it's attack speed
	short		   Con;       // The mob's constitution points, affects it's HP

	short  Special[4]; // The mob's special points, affects it's skill tiers
};

struct STRUCT_MOB
{
	char		   MobName[NAME_LENGTH];	  // The name of the mob
	char		   Clan;		  // The clan the mob belongs to
	unsigned char  Merchant;	  // The mob's merchant ID
	unsigned short Guild;		  // The ID of the guild the mob belongs to
	unsigned char  Class;		  // The mobs class
	unsigned short  Rsv;
	unsigned char Quest;

	int			   Coin;		  // The ammount of coins the mob has

	long long   Exp;			  // The ammount of experience the mob has to level up

	short		   SPX;			 // The Y position saved by the stellar gem, to teleport the mob there when using warp scroll
	short		   SPY;			 // The Y position saved by the stellar gem, to teleport the mob there when using warp scroll

	STRUCT_SCORE   BaseScore;    // The base score of the mob 
	STRUCT_SCORE   CurrentScore; // The score the mob actually has

	STRUCT_ITEM    Equip[MAX_EQUIP];	 // The items the mob is wearing
	STRUCT_ITEM	   Carry[MAX_CARRY];	 // The items the mob is carrying

	long LearnedSkill; // The skills the mob learned, divided into four categories (00 _ 00 _ 00 _ 00)

	unsigned int Magic;

	unsigned short ScoreBonus;   // The points the mob can use to increase score (Str, Int, Dex, Con)
	unsigned short SpecialBonus; // The points the mob can use to increase special, to increase effect of learned skills (score->Special[4])
	unsigned short SkillBonus;	 // The points the mob can use to buy skills

	unsigned char  Critical;	 // The chance the mob has to deliver critical hits
	unsigned char  SaveMana;	 // Uknown use, nomenclature of variable is correct to all current standards

	unsigned char  SkillBar[4];  // The skills saved on the first 4 slots of the skill bar

	unsigned char  GuildLevel;   // The mob's guuld level, used to define if it's a guild member or leader

	unsigned short  RegenHP;		 // UNK
	unsigned short  RegenMP;		 // UNK

	char  Resist[4];	 // The mob's resistencies, to fire / ice / thunder / magic

};

struct STRUCT_MOBEXTRA
{
	short ClassMaster;
	char  Citizen;

	int	  Fame;

	char  Soul;

	short MortalFace;

	struct
	{
		struct
		{
			char Newbie;//00_01_02_03_04  quest com quatro etapas
			char TerraMistica;//0 : não pegou a quest 1: pegou a quest e não concluiu 2: quest completa
			char MolarGargula;
			char PilulaOrc;

			char EMPTY[30];
		}Mortal;

		struct
		{
			char MortalSlot;
			char MortalLevel;

			char Level355;
			char Level370;

			char Cristal;//00_01_02_03_04 quest com quatro etapas

			char EMPTY[30];
		}Arch;

		struct
		{
			short ArchLevel;
			short CelestialLevel;
			short SubCelestialLevel;

			char Lv40;
			char Lv90;

			char Add120;
			char Add150;
			char Add180;
			char Add200;

			char Arcana;
			char Reset;

			char EMPTY[30];
		}Celestial;

		char Circle;
		char EMPTY[30];

	}QuestInfo;

	struct
	{
		int Class;

		long long   Exp;			  // The ammount of experience the mob has to level up

		short		   SPX;			 // The Y position saved by the stellar gem, to teleport the mob there when using warp scroll
		short		   SPY;			 // The Y position saved by the stellar gem, to teleport the mob there when using warp scroll

		STRUCT_SCORE   BaseScore;    // The base score of the mob 

		long LearnedSkill; // The skills the mob learned, divided into four categories (00 _ 00 _ 00 _ 00)


		unsigned short ScoreBonus;   // The points the mob can use to increase score (Str, Int, Dex, Con)
		unsigned short SpecialBonus; // The points the mob can use to increase special, to increase effect of learned skills (score->Special[4])
		unsigned short SkillBonus;	 // The points the mob can use to buy skills

		unsigned char  SkillBar1[4];  // The skills saved on the first 4 slots of the skill bar
		unsigned char  SkillBar2[16];  // The skills saved on the first 4 slots of the skill bar

		char Soul;
		char EMPTY[30];
	}SaveCelestial[2];

	time_t LastNT;
	int NT;

	int KefraTicket;

	time_t DivineEnd;

	unsigned int Hold;

	struct
	{
		long long Exp;
		int		  YearDay;
	}DayLog;

	struct
	{
		time_t LastTime;
		int Count;
	}DonateInfo;

	int EMPTY[9];
};

struct STRUCT_AFFECT
{
	unsigned char Type;
	unsigned char Value;
	unsigned short Level;
	unsigned int Time;
};

struct STRUCT_LEVELBONUS
{
	unsigned short Level;
	unsigned short ScoreBonus;   // The points the mob can use to increase score (Str, Int, Dex, Con)
	unsigned short SpecialBonus; // The points the mob can use to increase special, to increase effect of learned skills (score->Special[4])
	unsigned short SkillBonus;	 // The points the mob can use to buy skills
	unsigned short Ac;
};

struct STRUCT_CAPSULE
{
	STRUCT_MOB MOB;
	STRUCT_MOBEXTRA extra;
};

struct STRUCT_BEASTBONUS
{
	int Unk; // MinDamAdd ? // 0
	int Unk2; // MaxDamAdd ? // 4
	int Unk3; // MinAcAdd ? // 8
	int Unk4; // MaxAcAdd ? //12
	int Unk5; // MinHpAdd ? // 16
	int Unk6; // MaxHpAdd ? // 20
	int Unk7; // RunSpeedBonus ? // 24
	int Unk8; // 28
	int Unk9; // AttackSpeedBonus ? // 32
	int Unk10; // 36
	int Unk11; // 40
	int Unk12; // 44 
	int Unk13; // 48
	int Unk14; // 52
	int Unk15; // 56
	int Unk16; // 60
	int Unk17; // BaseSanc ? // 64
};

struct STRUCT_TREASURE
{
	short Source;

	STRUCT_ITEM Target[5];

	short Rate[5];
};

struct STRUCT_PISTA
{
	struct
	{
		short LeaderID;
		char  LeaderName[NAME_LENGTH];

		int Sala;
		int MobCount;
	}Party[3];
};

struct STRUCT_CASTLEQUEST
{
	int MOB_INITIAL;
	int MOB_END;
	int BOSS[2];

	STRUCT_ITEM Prize[MAX_CARRY];
	int			CoinPrize;
	int			ExpPrize[6];
	int			PartyPrize;

	int			QuestTime;
};

struct STRUCT_MERC
{
	int GenerateIndex;
	char MobName[NAME_LENGTH];

	STRUCT_ITEM Carry[MAX_CARRY];
	short		Stock[MAX_CARRY];
	short		MaxStock[MAX_CARRY];
	int			RenewTime;
	int			RebornTime;
};

struct STRUCT_CAMP
{
	int			BauIndex;
	int			BauQuest;
	char		BauName[256];
	int			KeyIndex;

	int			MOB_INITIAL;
	int			MOB_END;

	int			MobX;
	int			MobY;
	int			BauX;
	int			BauY;
	int			BauRotate;


	STRUCT_ITEM Prize[MAX_CAMP_PRIZE];
	int			CoinPrize;
	int			ExpPrize[6];
	int			PartyPrize;

	int			RenewTime;
};

struct STRUCT_ITEMLOG
{
	int	Num;
};

struct STRUCT_GUILDZONE
{
	int ChargeGuild;
	int ChallangeGuild;

	int GuildSpawnX;
	int GuildSpawnY;

	int CitySpawnX;
	int CitySpawnY;

	int CityLimitX1;
	int CityLimitY1;
	int CityLimitX2;
	int CityLimitY2;

	int WarAreaX1;
	int WarAreaY1;
	int WarAreaX2;
	int WarAreaY2;

	int ChargeWarSpawnX;
	int ChargeWarSpawnY;

	int ChallangeWarSpawnX;
	int ChallangeWarSpawnY;

	int CityTax;
	int Clan;
	int Victory;
};

struct STRUCT_SEALOFSOUL
{
	int Read;
	int MortalClass;
	int ClassCele;
	int SubClass;
	int LevelCele;
	int LevelSub;
	int For;
	int Int;
	int Dex;
	int Con;
	int ScoreBonus;
	int SkillPoint;
	int ArchQuest;
	int CelestialQuest;
	int ArcanaQuest;
};

struct STRUCT_SELCHAR
{
	short SPX[4];
	short SPY[4];
	char Name[4][16];

	STRUCT_SCORE Score[4];
	STRUCT_ITEM  Equip[4][16];

	unsigned short Guild[4];

	int Coin[4];
	long long Exp[4];
};

struct STRUCT_ACCOUNTINFO
{
	char AccountName[ACCOUNTNAME_LENGTH];
	char AccountPass[ACCOUNTPASS_LENGTH];

	char RealName[REALNAME_LENGTH];
	int  SSN1;
	int  SSN2;
	char Email[EMAIL_LENGTH];
	char Telephone[TELEPHONE_LENGTH];
	char Address[ADDRESS_LENGTH];
	char NumericToken[6];

	int Year;
	int YearDay;
};


struct STRUCT_ACCOUNTFILE
{
	STRUCT_ACCOUNTINFO Info; // 0 - 216

	STRUCT_MOB	Char[MOB_PER_ACCOUNT]; // 216 - 3480
	STRUCT_ITEM Cargo[MAX_CARGO]; // 3480 - 4504
	int			Coin; // 4504 - 4508

	unsigned char ShortSkill[MOB_PER_ACCOUNT][16]; // 4508 - 4572

	STRUCT_AFFECT affect[MOB_PER_ACCOUNT][MAX_AFFECT];
	STRUCT_MOBEXTRA mobExtra[MOB_PER_ACCOUNT];

	int Donate;

	char TempKey[52];
};
