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
#ifndef __SERVER_H__
#define	__SERVER_H__

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

#include "..\Basedef.h"
#include "..\CPSock.h"

#include "CItem.h"
#include "CMob.h"
#include "CUser.h"
#include "CNPCGene.h"

class ConsoleWindow {
public:
	ConsoleWindow();
	void writeToConsole(char* textToWrite);
};

// Externs
extern HWND hWndMain;
extern AccountBanned BannedUser[MAX_USER];

extern int ServerGroup;

extern unsigned int CurrentTime;
extern int BrState;

extern unsigned short g_pGuildWar[65536];
extern unsigned short g_pGuildAlly[65536];

extern unsigned short pMobGrid  [MAX_GRIDY][MAX_GRIDX];
extern unsigned short pItemGrid [MAX_GRIDY][MAX_GRIDX];
extern char pHeightGrid [MAX_GRIDY][MAX_GRIDX];

extern int CurrentWeather;

extern int GuildImpostoID[MAX_GUILDZONE];

extern CNPCGenerator mNPCGen;

extern CUser pUser[MAX_USER];
extern STRUCT_BLOCKMAC pMac[MAX_MAC];

extern int TowerCount;
extern int TowerStage;
extern int GuildTower;
extern int GuildScore[MAX_GUILD];

// Funcs

void CreateExportInspectTable(int conn);
void ExportConfig();
void  ClearCrown(int conn);
BOOL  AddCrackError(int conn, int val, int Type);
BOOL  PutItem(int conn, STRUCT_ITEM *item);
BOOL  PutItemCargo(int conn, STRUCT_ITEM* item);
void  GuildZoneReport();
HFONT__ *  GetAFont();
void  ReadConfig();
void  DrawConfig(int wb);
void  AddFailAccount(char *Account);
int   CheckFailAccount(char *Account);
int   IsFree(STRUCT_SELCHAR *sel);
int   SendBilling(int a1, char *a2, int a3, int a4);
int   SendBilling2(_AUTH_GAME *m, int hostlong);
void  SetItemBonus(STRUCT_ITEM *Dest, int Level, int a3, int DropBonus);
int   Challange(int conn, int mob, int coin);
int   GenerateSummon(int conn, int SummonID, STRUCT_ITEM *sItem, int Num);
int   CreateMob(char *MobName, int PosX, int PosY, char *folder, int Type); 
void  GenerateMob(int index, int PosX, int PosY);
void  RebuildGenerator();
BOOL  InitApplication(HANDLE hInstance);
BOOL  InitInstance(HANDLE hInstance, int nCmdShow);
FILE* WriteHeader(char *fileName);
void  WriteArmor(FILE *fp);
void  WriteWeapon(FILE *fp);
void  BuildList();
LONG  APIENTRY MainWndProc(HWND hWnd, UINT message, UINT wParam, LONG lParam);
int   GetUserInArea(int x1, int y1, int x2, int y2, char *MobName);
void  MountProcess(int conn, STRUCT_ITEM *Mount);
void  LinkMountHp(int mobConn);
void  ProcessAdultMount(int conn, int HpLost);
void  ProcessBILLMessage(char *pMsg);
void  ProcessDBMessage(char *Msg);
void  CheckIdle(int conn); 
void  FinishCastleWar();
void  RegenMob(int conn);
int   ProcessAffect(int conn);
void  SendDamage(int x1, int y1, int x2, int y2);
void  ProcessSecTimer();
void  ProcessMinTimer();
void  SummonGuild2(int Guild, int tx, int ty, int maxsummon, int village);
void  SummonGuild(int Guild, int tx, int ty, int maxsummon, int village);
void  ClearArea(int x1, int y1, int x2, int y2);
void  ClearAreaLevel(int x1, int y1, int x2, int y2, int minLv, int maxLv);
int BASE_GetOnScreen(int mob, int target);
void  ClearAreaGuild(int x1, int y1, int x2, int y2, int Guild);
void  ClearAreaTeleport(int x1, int y1, int x2, int y2, int tx, int ty);
void  ClearGuildPKZone();
void  SetColoseumDoor(int state); 
void  SetColoseumDoor2(int state);
void  GenerateColoseum(int InitID);
void  DeleteColoseum();
void  SetArenaDoor(int state);
void  SetCastleDoor(int state);
void  DecideWinner();
void  GuildProcess(); 
void  StartLog();
void  CrackLog(int conn, char *str);
void  Log(char *str1, char *str2, unsigned int ip);
void MyLog(LogType logType, char* str1, char* str2, char* str3, unsigned int ip);
int   GetUserFromSocket(int Sock);
int   GetEmptyUser();
int   GetEmptyItem();
int   GetEmptyNPCMob();
int   GetUserByName(char *name);
int   IsImple(int user);
void  CloseUser(int conn);
CMob *GetMobFromIndex(unsigned int index);
int   GetAngle(int x, int y);
int   GetLength(int x, int y);
int   ReadMob(STRUCT_MOB *mob, char *dir);
void  DeleteMob(int conn, int Type);
void  SaveUser(int conn,int Export);
void  CharLogOut(int conn);
void  Reboot();  
void  ClearChallanger(); 
void  SetBattle(int mob, int target);      
void  MobKilled(int conn, int target, int PosX, int PosY); 
int   CreateItem(int x, int y, STRUCT_ITEM *item, int rotate, int Create);
void  RemoveTrade(int conn);

void BASE_ReadGuardFile();
void BASE_ReadQuestDiaria();

void  TransferGroup(int conn, int trans);

void  RemoveParty(int conn);
int   UpdateItem(int Gate, int state, int *height);
void  DoRecall(int conn);  
void  DoWar(int myguild, int target);     
void  DoAlly(int myguild, int target);       
void  DoDeprivate(int conn, int target);     
void  ProcessRanking();    
void  DoRanking(int tDuel, int conn, int enemy);   
void  DoRemoveHide(int conn);      
void  DoTeleport(int mob, int x, int y);     
void  DoSummon(int target, int x, int y);       
int   SetAffect(int conn, int skillnum, int time, int Level);
int   SetTick(int mob, int skillnum, int Delay, int Level); 
void  ProcessDecayItem();
int   CombineTreasureMap(int conn);
void  Combine(int conn, int item, int PrizeItem);
int   ApplyHp(int conn); 
int   ApplyMp(int conn); 
void  SetReqHp(int conn); 
void  SetReqMp(int conn); 
void  ClearMapa(int x, int y);
void  DeleteMobMapa(int x, int y);
void  SetItemBonus2(STRUCT_ITEM *Dest);
void  DoRemoveSamaritano(int conn);
void  SaveAll();
void  ReadLevelItemConfig(void);
void  DoItemLevel(int conn);
void  SetCircletSubGod(int conn);
void  SummonServer(int tx, int ty);
void  DoGreatTeleport(int mob, int x, int y);
void  SendDamageKingdom(int x1, int y1, int x2, int y2, int Clan);
void  SendDamageLeader(int x1, int y1, int x2, int y2);
void  MobAttack(int attacker, MSG_Attack sm);
void  SendMsgExp(int conn, char* msg, TNColor Color, bool flag);
void  SendSpkMsg(int conn, char* msg, TNColor Color, bool Flag);
void  SendUpdateWoteBattle();
void  StartChatLog();
void  ChatLog(char *str1, char *str2, unsigned int ip);
void  StartItemLog();
void  ItemLog(char *str1, char *str2, unsigned int ip);
int   ParseCastleString(int Num, char *str);
int   ParseMobMercString(int Num, char *str);
int   ParseCampString(int Num, char *str);
void  DeleteGenerateMob(int generate);
int   CreateTreasureBox(int x, int y, STRUCT_ITEM *item, int rotate, int State);
void  ClearAreaQuest(int x1, int y1, int x2, int y2);
int RemoveItem(int mob, int object, int count);
/*int GetUserInAreaMac(int notUsage, int x1, int y1, int x2, int y2);
int GetUserInAreaMac2(int notUsage, int x1, int y1);*/
void SetBigCuboDoor(int state);
// External func

void ProcessImple(int conn, int level, char * str);



//Externs

extern  int FREEEXP;

extern int ServerDown;

extern int g_EmptyMob;

extern int ServerIndex;
extern int Sapphire;
extern int ServerGroup;
extern int GuildCounter;

extern int ValidGuild ;
extern int WaterScrollPosition[3][10][2];

extern short weapon[32][9];

extern CPSock BillServerSocket;
extern char   BillServerAddress[32];
extern int	   BillServerPort;

extern HWND  hWndMain;
extern HMENU hMainMenu;
extern HFONT hFont;

extern FILE *fLogFile;
extern FILE *fChatLogFile;
extern FILE *fItemLogFile;

extern CPSock DBServerSocket;

extern CNPCGenerator mNPCGen;
extern CNPCSummon mSummon;

extern CUser pUser[MAX_USER];
extern CMob pMob[MAX_MOB];

extern STRUCT_TREASURE g_pTreasure[8];
extern STRUCT_ITEM LevelItem[4][4][400];

extern CItem pItem[MAX_ITEM];

extern unsigned int CurrentTime;
extern unsigned int LastSendTime;

extern STRUCT_GUARD g_pGuard[MAX_NPC_GUARD_COUT];


extern int BillCounter;

extern int	g_Week;
extern int	g_Hour;

extern int LastLogDay;
extern int LastChatLogDay;
extern int LastItemLogDay;
extern int maxNightmare;

extern int ConfigReady;

///////////////////////////////////////
//Config for Event
//////////////////////////////////////
extern int evNotice;
extern int evStartIndex;
extern int evEndIndex;
extern int evCurrentIndex;
extern int evRate;
extern int evItem;
extern int evIndex;
extern int evOn;
extern int evDelete;
////////////////////////////////////

extern int NewbieEventServer;
extern int CastleServer;

extern int DOUBLEMODE;
extern int DUNGEONEVENT;
extern int DEADPOINT;
extern int StatSapphire;
extern int BRItem;

extern int BILLING;
extern int CHARSELBILL;
extern int POTIONCOUNT;
extern int PARTYBONUS;
extern int GUILDBOARD;

extern int UserCount;
extern int SaveCount;
extern int ItemCount;

extern int ForceWeather;

extern int WeekMode;
extern int ForceWeekMode;
extern int GuildDay;
extern int GuildHour;

extern int NumGuildBattle;
extern int PotionReady;
extern int PotionDelay;
extern int NewbieHour;

extern int RvRState;

extern int NumServerInGroup;

extern int LiveTower[3];

extern int pChallangerMoney[6];

extern unsigned short g_pGuildWar[65536];
extern unsigned short g_pGuildAlly[65536];

extern unsigned short pMobGrid[MAX_GRIDY][MAX_GRIDX];
extern unsigned short pItemGrid[MAX_GRIDY][MAX_GRIDX];
extern char pHeightGrid[MAX_GRIDY][MAX_GRIDX];

extern unsigned char LocalIP[4];

extern char FailAccount[16][16];

extern CPSock ListenSocket;
extern unsigned int pAdminIP[MAX_ADMIN];

extern char DBServerAddress[32];
extern int DBServerPort;

extern int CurrentWeather;
extern int TESTSERVER;
extern int LOCALSERVER;

extern int TransperCharacter;

extern unsigned int SecCounter;
extern unsigned int MinCounter;

extern int GuildImpostoID[MAX_GUILDZONE];

extern int BrState;
extern int BrMod;
extern int BrGrid;
extern int CastleState;
extern int Colo150Limit;
extern int KefraLive;
extern char KefraKiller[32];

extern int g_dLevel;
extern int g_dLevel1;
extern int g_dLevel2;

extern int Kingdom1Clear;
extern int Kingdom2Clear;
extern int PARTY_DIF;
extern int isDropItem;

///////////////////////////////////////
//Config for Duel: 1:1 5:5 10:10 All:All
//////////////////////////////////////
extern char nRanking1[32];
extern char nRanking2[32];
extern char nTitle1[32];
extern char nTitle2[32];

extern int RankingProgress;
extern int Ranking1;
extern int Ranking2;
extern int lTitle1;
extern int lTitle2;
extern int RankingTime;

extern int Ranking1X;
extern int Ranking1Y;
extern int Ranking2X;
extern int Ranking2Y;

extern int cRanking1X;
extern int cRanking1Y;
extern int cRanking2X;
extern int cRanking2Y;

extern int kRanking1X;
extern int kRanking1Y;
extern int kRanking2X;
extern int kRanking2Y;

extern char temp[4096];

extern int TaxChanged[MAX_GUILDZONE];


//////////////////////////////////////

extern int WaterClear1[3][10];
extern int PartyPesa[3];
extern int PesaNPosStandard[13][2];
extern int PesaMPosStandard[13][2];
extern int PesaAPosStandard[13][2];
extern int CartaPos[4][2];
extern int PistaPos[7][3][2];
extern int CartaTime;
extern int CartaSala;

extern STRUCT_PISTA Pista[7];
extern STRUCT_CALABOUCO g_calabouco;
extern STRUCT_QUESTS g_quests;

extern STRUCT_MERC pMobMerc[MAX_MOB_MERC];

extern int PistaRune[7][10];
extern int PistaBalrogPos[5][2];
extern int PistaBalrogPortalPos[5][5][2];
extern int RvRBonus;
extern int Citizen;
extern int QuestIceTime;
extern int GTorreState;
extern int GTorreHour;
extern int GTorreGuild;

extern int RvRBluePoint;
extern int RvRRedPoint;

/////////////////////////////////////
//Externs

extern STRUCT_GUILDZONE g_pGuildZone[MAX_GUILDZONE];
#endif