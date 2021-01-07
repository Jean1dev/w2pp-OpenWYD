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


#include <winsock2.h>
#include <ws2tcpip.h>
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

#include "../Basedef.h"
#include "../CPSock.h"
#include "../ItemEffect.h"
#include "CUser.h"
#include "CFileDB.h"
#include "Server.h"
#include "CRanking.h"
#include "CReadFiles.h"

HWND		hWndMain;
HMENU		hMainMenu;
HINSTANCE   hInst;

int ServerIndex = -1;
int CurrentTransInfo = 0;
int Sapphire = 8;
int MinCounter = 0;
int SecCounter = 0;
int HourCounter = 0;
int TransperCharacter = 0;
unsigned int CurrentTime = 0;
unsigned int LastSendTime = 0;

int UserConnection[MAX_SERVER] = {0,};

unsigned char LocalIP[4] = {0,};
unsigned int pAdminIP[MAX_ADMIN] = {0,};

int TransGuildConn[MAX_TRANS];
int TransGuildIndex[MAX_TRANS];

extern unsigned short g_pGuildWar[65536];
extern unsigned short g_pGuildAlly[65536];

extern char g_pServerList[MAX_SERVERGROUP][MAX_SERVERNUMBER][64];

char adminclientid[256];
char adminclientpass[256];

FILE *fLogFile;
FILE *fDayLogFile;

int		LastLogDay	  =	-1;
int		LastDayLogDay = -1;

CUser TempUser;

CUser pUser[MAX_SERVER];
CUser pAdmin[MAX_ADMIN];

CPSock AdminClient;
CPSock AdminSocket;
CPSock ListenSocket;

CFileDB cFileDB;

STRUCT_MOB g_pBaseSet[MAX_CLASS];
STRUCT_ITEMLOG ItemDayLog[MAX_ITEMLIST];

HFONT hFont = NULL;
HFONT h;

int x = 0;
int y = 0;
HDC hDC = NULL;

void TextOutWind(char *str, int color)
{
	char String[1024];

	SetTextColor(hDC, color);

	sprintf(String, str);

	int len = strlen(String);


	TextOutA(hDC, x, y, String, len);

	y += 16;
}

void DrawConfig()
{
	x = 0;
	y = 0;

	h = 0;

	int VERMELHO = 255;
	int PRETO = 0;

	char String[1024];
	int len = 0;

	hDC = GetDC(hWndMain);

	if (hDC == NULL)
		return;

	if (hFont == 0)
		return;

	if (SelectObject(hDC, hFont) != 0)
		h = (HFONT)SelectObject(hDC, hFont);

	
	TextOutWind("Server Zone Status:", VERMELHO);

	for (int i = 0; i < MAX_SERVER; i++)
	{
		unsigned char *cIP = (unsigned char *)&(pUser[i].IP);

		sprintf(String, " %d - IP: %3d. %3d. %3d. %3d  Socket: %3d  Guild: %4d %4d %4d %4d %4d  User: %4d    ", i, cIP[0], cIP[1], cIP[2], cIP[3], pUser[i].cSock.Sock, ChargedGuildList[i][0], ChargedGuildList[i][1], ChargedGuildList[i][2], ChargedGuildList[i][3], ChargedGuildList[i][4], pUser[i].Count);
		TextOutWind(String, PRETO);
	}
	
	TextOutWind("Admin Status:", VERMELHO);
	for (int i = 0; i < MAX_ADMIN; i++)
	{
		unsigned char *cIP = (unsigned char *)&(pAdmin[i].IP);

		sprintf(String, "%2d - IP: %3d. %3d. %3d. %3d  Socket: %3d", i, cIP[0], cIP[1], cIP[2], cIP[3], pAdmin[i].cSock.Sock);
		TextOutWind(String, PRETO);
	}
	
	if (hFont && h)
		h = (HFONT)SelectObject(hDC, h);

	ReleaseDC(hWndMain, hDC);
}

int ReadTrandGuildInfo()
{
	memset(TransGuildConn, 0, MAX_TRANS * sizeof(int));
	memset(TransGuildIndex, 0, MAX_TRANS * sizeof(int));

	FILE *fp = fopen("Trans.txt", "rt");

	if(fp == NULL)
		return FALSE;

	int ret = fscanf(fp, "%d", &CurrentTransInfo);

	if(!ret)
		return FALSE;

	int i = 0;

	for(; i < MAX_TRANS; i++)
	{
		int target = 0, conn = 0, index = 0;

		int ret = fscanf(fp, "%d %d %d", &target, &conn, &index);

		if(ret < 0)
			break;

		if(target != i + MAX_TRANS*2)
			break;

		TransGuildConn[i] = conn;
		TransGuildIndex[i] = index;
	}

	fclose(fp);

	CurrentTransInfo = i;

	return TRUE;
}

int WriteTransGuildInfo()
{
	FILE *fp = fopen("Trans.txt", "wt");

	int ret = fprintf(fp, "%d\n", CurrentTransInfo);

	if(!ret)
		return FALSE;

	for(int i = 0; i < CurrentTransInfo; i++)
	{
		int ret = fprintf(fp, "%d %d %d\n", i + MAX_TRANS*2, TransGuildConn[i], TransGuildIndex[i]);

		if(ret < 0)
			break;
	}

	fclose(fp);

	return TRUE;
}

void ConvertGuildNumber(int group, struct STRUCT_ITEM *sour)
{
	unsigned short guildnumber = BASE_GetGuild(sour);

	int i = 0;

	for(; i < CurrentTransInfo && (TransGuildConn[i] != group || TransGuildIndex[i] != guildnumber); i++);

	if(i == CurrentTransInfo)
	{
		TransGuildConn[i]  = group;
		TransGuildIndex[i] = guildnumber;

		CurrentTransInfo++;

		WriteTransGuildInfo();
	}

	unsigned short targetguild = (i + MAX_TRANS*2);

	sour->stEffect[0].cEffect = EF_HWORDGUILD;
	sour->stEffect[0].cValue = HIBYTE(targetguild);
	sour->stEffect[1].cEffect = EF_LWORDGUILD;
	sour->stEffect[1].cValue = targetguild % 256;
}

int SendAdminSignal(int svr, unsigned short id, unsigned short signal)
{
	MSG_STANDARD sm;

	sm.Type = signal;
	sm.ID = id;
	sm.Size = sizeof(MSG_STANDARD);

	pAdmin[svr].cSock.SendOneMessage((char*)&sm, sizeof(sm));

	return TRUE;
}

int SendAdminMessage(int svr, unsigned short id, char *message)
{
	MSG_NPNotice sm;

	sm.Type = _MSG_NPNotice;
	sm.ID = id;
	sm.Size = sizeof(MSG_NPNotice);

	strncpy(sm.String, message, MESSAGE_LENGTH);

	pAdmin[svr].cSock.SendOneMessage((char*)&sm, sizeof(sm));

	return TRUE;
}

int SendAdminParm(int svr, unsigned short id, unsigned short signal, int parm)
{
	MSG_STANDARDPARM sm;

	sm.Type = signal;
	sm.ID = id;
	sm.Size = sizeof(MSG_STANDARDPARM);
	sm.Parm = parm;

	pAdmin[svr].cSock.SendOneMessage((char*)&sm, sizeof(sm));

	return TRUE;
}

int SendAdminState(int svr, unsigned short id, unsigned short signal, char *pass)
{
	MSG_STANDARDPARM sm;

	sm.Type = signal;
	sm.ID	= id;
	sm.Size = sizeof(MSG_STANDARDPARM);

	if(pass[0] == '@') sm.Parm = 1;
	if(pass[0] == '_') sm.Parm = 2;
	if(pass[0] == '#') sm.Parm = 3;

	pAdmin[svr].cSock.SendOneMessage((char*)&sm, sizeof(sm));

	return TRUE;
}

int InitApplication(HANDLE hInstance) // Last updated 15/01/2013
{
	WNDCLASS  wc;  
	
	wc.style = CS_VREDRAW | CS_HREDRAW | CS_DBLCLKS;    
	wc.lpfnWndProc = (WNDPROC)MainWndProc;  
	wc.cbClsExtra = 0;  
	wc.cbWndExtra = 0;                    
	wc.hIcon = (HICON)LoadIcon ((HINSTANCE)hInstance, "MAINICON");
	wc.hInstance = (HINSTANCE) hInstance;  
	wc.hCursor = LoadCursor(NULL, IDC_ARROW);
	wc.hbrBackground = (HBRUSH) GetStockObject(WHITE_BRUSH);  
	wc.lpszMenuName =  NULL;           
	wc.lpszClassName = "MainClass"; 
	
	if(!RegisterClass(&wc)) 
		return RegisterClass(&wc);

	return TRUE;
}

int InitInstance(HANDLE hInstance, int nCmdShow) // Last updated 15/01/2013
{    
	hMainMenu = CreateMenu();

	hWndMain = CreateWindow("MainClass",  "DB Server", 
		WS_OVERLAPPED | WS_CLIPCHILDREN | WS_SYSMENU | WS_MINIMIZEBOX | WS_MAXIMIZEBOX,    // Window style. 
		CW_USEDEFAULT, CW_USEDEFAULT, 800, 600,            // W,H[MenuÇÕÄ¡¸é 480]
		NULL, hMainMenu, (HINSTANCE)hInstance, NULL);

	if (!hWndMain) 
		return FALSE;

	ShowWindow(hWndMain, nCmdShow);  
	UpdateWindow(hWndMain);      

	return TRUE;     
}

void ReadAdmin()
{
	char temp[256];

	sprintf(temp, "Admin.txt");

	FILE *fp = fopen(temp, "r");

	if(fp == NULL) 
		return;

	while(1)
	{
		int a, b, c, d;
		a = b = c = d = 0;

		char * ret = fgets((char*)temp, 127, fp);    

		if(ret == NULL) 
			break;

		int idx = -1;

		for(int i = 0; i < 255; i++) 
			if(temp[i] == '.')
				temp[i] = ' ';

		sscanf(temp, "%d %d %d %d %d", &idx, &a, &b, &c, &d);

		unsigned int ip = (d << 24) + (c << 16) + (b << 8) + a; 

		if (idx < 0 || idx >= MAX_ADMIN) 
			continue;

		pAdminIP[idx] = ip;
	};

	fclose(fp);
}

int ReadConfig()
{
	FILE *fp = fopen("Config.txt", "rt");
	
	if(fp == NULL) 
	{ 
		MessageBox(hWndMain, "cant find Config.txt", "BOOTING ERROR", NULL);

		return FALSE; 
	}

	fscanf(fp, "Sapphire %d\n", &Sapphire);
	fscanf(fp, "LastCapsule %hu\n", &LastCapsule);

	fclose(fp);

	return TRUE;
}

int WriteConfig()
{
	FILE * fp= fopen("Config.txt", "wt");

	if(fp == NULL)
		return FALSE;

	fprintf(fp, "Sapphire %d\n", Sapphire);
	fprintf(fp, "LastCapsule %d\n", LastCapsule);

	fclose(fp);

	return TRUE;
}

int WINAPI WinMain(HINSTANCE hInstance, HINSTANCE hPrevInstance, LPSTR lpCmdLine, int nCmdShow) // Last Updated 19/01/2013
{
	MSG msg;

	UNREFERENCED_PARAMETER(lpCmdLine);

	hInst = hInstance;

	if(!hPrevInstance)
		if(!InitApplication(hInstance)) 
			return FALSE;      

	if(!InitInstance(hInstance, nCmdShow)) 
		return FALSE;


	BASE_InitModuleDir();
    BASE_InitializeBaseDef();

#pragma region Load Base Characters

	int handle = -1;

	handle = _open("./BaseMob/TK", _O_RDONLY | _O_BINARY);

	if(handle == -1)
	{
		MessageBoxA(hWndMain, "no TransKnight file", "BOOTING ERROR", MB_OK);

		return FALSE;
	}

	_read(handle, &g_pBaseSet[0], sizeof(STRUCT_MOB));
	_close(handle);


	handle = -1;

	handle = _open("./BaseMob/FM", _O_RDONLY | _O_BINARY);

	if(handle == -1)
	{
		MessageBoxA(hWndMain, "no Foema file", "BOOTING ERROR", MB_OK);

		return FALSE;
	}

	_read(handle, &g_pBaseSet[1], sizeof(STRUCT_MOB));
	_close(handle);

	handle = -1;

	handle = _open("./BaseMob/BM", _O_RDONLY | _O_BINARY);

	if(handle == -1)
	{
		MessageBoxA(hWndMain, "no BeastMaster file", "BOOTING ERROR", MB_OK);

		return FALSE;
	}

	_read(handle, &g_pBaseSet[2], sizeof(STRUCT_MOB));
	_close(handle);

	handle = -1;

	handle = _open("./BaseMob/HT", _O_RDONLY | _O_BINARY);

	if(handle == -1)
	{
		MessageBoxA(hWndMain, "no Huntress file", "BOOTING ERROR", MB_OK);

		return FALSE;
	}

	_read(handle, &g_pBaseSet[3], sizeof(STRUCT_MOB));
	_close(handle);

	g_pBaseSet[0].BaseScore = g_pBaseSet[0].CurrentScore;
	g_pBaseSet[1].BaseScore = g_pBaseSet[1].CurrentScore;
	g_pBaseSet[2].BaseScore = g_pBaseSet[2].CurrentScore;
	g_pBaseSet[3].BaseScore = g_pBaseSet[3].CurrentScore;

#pragma endregion

	StartLog("A");
	Log("start log", "-system", 0);

	memset(g_pGuildWar, 0, sizeof(g_pGuildWar));
	memset(g_pGuildAlly, 0, sizeof(g_pGuildAlly));

	ReadConfig();

	int ret = ListenSocket.WSAInitialize();

	if(!ret)
	{
		Log("err,wsainitialize fail", "-system", 0);

		return FALSE;
	}

	BASE_InitializeServerList();

	char name[256];
	int i = 0;

	struct addrinfo hints, *res, *p;

	memset(&hints, 0, sizeof(addrinfo));
	
	hints.ai_family = AF_INET;
	hints.ai_socktype = SOCK_STREAM;
	hints.ai_flags = AI_PASSIVE;

	gethostname(name, sizeof(name));

	getaddrinfo(name, NULL, &hints, &res);

	for (p = res; p != NULL; p = p->ai_next)
	{
		if (p->ai_family != AF_INET)
			continue;

		struct sockaddr_in *ipv4 = (struct sockaddr_in *)p->ai_addr;

		inet_ntop(p->ai_family, &(ipv4->sin_addr), name, sizeof(name));

		for (i = 0; i < MAX_SERVERGROUP; i++)
		{
			if (!strcmp(g_pServerList[i][0], name))
			{
				ServerIndex = i;

				sscanf(name, "%hhu.%hhu.%hhu.%hhu", &LocalIP[0], &LocalIP[1], &LocalIP[2], &LocalIP[3]);

				break;
			}
		}

		if (ServerIndex != -1)
			break;
	}

	/*freeaddrinfo(res);
	freeaddrinfo(p);
	freeaddrinfo(&hints);
	*/
    if(ServerIndex == -1)
    {	
		MessageBox(hWndMain, "Can't get Server Group Index LocalIP:", "", MB_OK|MB_SYSTEMMODAL);
		MessageBox(hWndMain, "Can't get Server Group Index TestServerIP:", g_pServerList[i][0], MB_OK | MB_SYSTEMMODAL);

	   return TRUE;
    }

	CReadFiles::ImportItem();
	ReadTrandGuildInfo();

	ListenSocket.StartListen(hWndMain, 0, DB_PORT, WSA_ACCEPT);
	AdminSocket.StartListen(hWndMain, 0, ADMIN_PORT, WSA_ACCEPTADMIN);

	FILE *fp = fopen("redirect.txt", "rt");

	if(fp)	
	{	
		char sip[256];
		int  port;

		fscanf(fp, "%s %d %s %s", sip, &port, adminclientid, adminclientpass);
		fclose(fp);

		ret = AdminClient.ConnectServer(sip, port, INADDR_ANY, WSA_READADMINCLIENT);

		if(ret == NULL)
		{	
			MessageBox(hWndMain, "there's redirect.txt but can't connect", "BOOTING ERROR", NULL);
			return FALSE;		
		}	
		else
		{	
			Log("sys redirection server connected","-system",0);
			TransperCharacter = 1;
		}
	}

	SetTimer(hWndMain, TIMER_SEC, 1000, NULL); 

	CReadFiles::ReadGuildInfo();

	while(GetMessage(&msg, NULL, 0, 0))
	{    
		TranslateMessage(&msg);
		DispatchMessage(&msg); 
	}

	return msg.wParam;  
}

int ProcessAdminClientMessage(char *msg)
{
	MSG_STANDARD *std = (MSG_STANDARD *)msg;

	if(!(std->Type & FLAG_NP2DB) || (std->ID < 0) || (std->ID >= MAX_DBACCOUNT))
	{	
		char temp[128];
		sprintf(temp, "err,adminclient_packet:%d ID:%d Size:%d KeyWord:%d", std->Type, std->ID, std->Size, std->KeyWord);

		Log(temp,"-system",0);

		return FALSE;
	} 

    switch(std->Type)    
	{
		case _MSG_NPCreateCharacter_Reply:
		{	
			MSG_NPCreateCharacter_Reply *m = (MSG_NPCreateCharacter_Reply*)msg;

			char * account	= m->Account;
			int slot		= m->Slot;
			int result		= m->Result;
			int IdxName		= cFileDB.GetIndex(m->Account);
			int	svr			= IdxName / MAX_USER;
			int	id			= IdxName % MAX_USER;

			MSG_ReqTransper sm; 
			memset(&sm, 0, sizeof(sm));

			sm.Type = _MSG_ReqTransper; 
			sm.Size = sizeof(sm);
			sm.ID	= id;
			sm.Slot = slot;
			sm.Result = m->Result;

			EnableAccount(-1, account);

			if(m->Result == 0)//; //0: Success Delete Parm2
			{	
				STRUCT_ACCOUNTFILE file;
				memcpy(file.Info.AccountName, m->Account, ACCOUNTNAME_LENGTH);

				int ret = cFileDB.DBReadAccount(&file);

				if(ret == FALSE)
				{	
					char tt[256]; 
					sprintf(tt, "etc tranper_FAILED!!!!!(read) %s %s", m->Account, m->Name);

					Log(tt, file.Info.AccountName, 0);

					if(IdxName)	
					{
						sm.Result = 4;
						pUser[svr].cSock.SendOneMessage((char*)&sm, sm.Size);
					}

					return TRUE;
				}

				char tt[256]; 
				sprintf(tt, "etc tranper_success(deleted) %s %s", m->Account, m->Name);

				Log(tt, file.Info.AccountName, 0);

				memset(&file.Char[slot], 0, sizeof(STRUCT_MOB));

				_strupr(file.Info.AccountName);

				ret = cFileDB.DBWriteAccount(&file);

	            if(ret == FALSE)
				{	
					char tt[256]; 
					sprintf(tt, "etc tranper_FAILED!!!!!(write) %s %s", m->Account, m->Name);

					Log(tt, file.Info.AccountName, 0);

					if(IdxName)	
					{
						sm.Result = 4;	
						pUser[svr].cSock.SendOneMessage((char*)&sm, sm.Size);
					}

					return TRUE;
				}

				if(IdxName)
				{	
					pUser[svr].cSock.SendOneMessage((char*)&sm, sm.Size);

					memset(&(cFileDB.pAccountList[IdxName].File.Char[slot]), 0, sizeof(STRUCT_MOB));
				}
			}
			if(result == 1) //1: Threre's Same Name Already
			{	
				if(IdxName)	
					pUser[svr].cSock.SendOneMessage((char*)&sm, sm.Size);
			}	
			else if(result == 2) //2: No character space. delete first.
			{	
				if(IdxName)	
					pUser[svr].cSock.SendOneMessage((char*)&sm, sm.Size);
			}	
			else if(result == 3) //3: That account is playing
			{	
				if(IdxName)	
					pUser[svr].cSock.SendOneMessage((char*)&sm, sm.Size);
			}
		}
		break;	
	}
	
	return TRUE;
}

LONG APIENTRY MainWndProc(HWND hWnd, UINT message, UINT wParam, LONG lParam)
{
	switch(message)    
	{

	case WM_TIMER:
		{ 
			if(wParam == TIMER_SEC) 
				ProcessSecTimer(); 
			else if(wParam == TIMER_MIN) 
				ProcessMinTimer(); 
		} break;

	case WSA_READADMINCLIENT:
		{
			if(WSAGETSELECTEVENT(lParam) != FD_READ)
			{	
				Log("adminclient close fd read","-system",0);

				AdminClient.CloseSocket();

				break;
			}

			if(AdminClient.Receive() != 1)
			{	
				Log("adminclient %d close receive", "-system", 0);

				break;
			}

			char temp[256];	
			int Error;	
			int ErrorCode;

			while(1)
			{	
				char *Msg = AdminClient.ReadMessage(&Error, &ErrorCode); 

				if(Msg == NULL) 
					break;

				if(Error == 1 || Error == 2)
				{	
					sprintf(temp, "err wsa_ReadadminClient (%d),%d", Error, ErrorCode);
					Log(temp, "-system", 0);

					break;
				}

				int ret = ProcessAdminClientMessage(Msg);
			};
		}	break;

	case WSA_READADMIN:
		{
			int User = GetAdminFromSocket(wParam);  

			if(User == -1)
			{	
				closesocket(wParam);

				Log("err, wsa_read unregistered sever socket", "-system", 0);

				break;
			} 

			if(WSAGETSELECTEVENT(lParam) != FD_READ)
			{	
				char temp[256];
				sprintf(temp, "admin close fd %d", User);
				Log(temp, "-system", 0);

				pAdmin[User].cSock.CloseSocket();
				pAdmin[User].Mode = USER_EMPTY;
				pAdmin[User].Name[0] = 0;

				break;
			} 

		 	CurrentTime = timeGetTime();

			if(pAdmin[User].cSock.Receive() != 1)
			{	
				char temp[256];
				sprintf(temp, "admin %d close receive", User);
				Log(temp, "-system", 0);

				break;
			}

			char temp[256];
			int Error;
			int ErrorCode;

            while(1)  
			{	
				char *Msg = pAdmin[User].cSock.ReadMessage(&Error, &ErrorCode);

				if(Msg == NULL) 
					break;

	            #ifdef  _PACKET_DEBUG
				MSG_STANDARD * debug = (MSG_STANDARD*) Msg;

				if(BASE_CheckPacket(debug))
				{	
					sprintf(temp, "err packet Type:%d Size:%d", debug->Type, debug->Size);
					Log(temp, "-system", 0);
				}
				#endif

				if(Error == 1 || Error == 2)
				{	
					sprintf(temp, "err,wsa_Readadmin (%d),%d", Error, ErrorCode);
					Log(temp, "-system", 0);

					break;
				}

				int ret = ProcessAdminMessage(User, Msg);

				if(ret == FALSE) 
				{	
					pAdmin[User].cSock.CloseSocket();
					pAdmin[User].Mode = USER_EMPTY;
					pAdmin[User].Name[0] = 0;

					break;
				}
			}; 

		} break;

	case WSA_READ:
		{
			int User = GetUserFromSocket(wParam);  

			if(User == -1)
			{	
				closesocket(wParam);

				Log("err wsa_read unregistered game server socket", "-system", 0);

				break;
			}

			if(WSAGETSELECTEVENT(lParam) != FD_READ)
			{	
				char temp[256];
				sprintf(temp, "clo server fd %d", User);
				Log(temp, "-system", 0);

				pUser[User].cSock.CloseSocket();
				pUser[User].Mode = USER_EMPTY;

				break;
			}

			CurrentTime = timeGetTime();

			if(pUser[User].cSock.Receive() != 1)
			{	
				char temp[256];
				sprintf(temp, "clo, server receive %d", User);
				Log(temp, "-system", 0);
				Log(temp, "-system", 0);
				Log(temp, "-system", 0);

				pUser[User].cSock.nRecvPosition = 0;
				pUser[User].cSock.nProcPosition = 0;

				break;
			}

			char temp[256];
			int Error;
			int ErrorCode;

			do 
			{	
				char *Msg = pUser[User].cSock.ReadMessage(&Error, &ErrorCode); 

				if (Msg == NULL) 
					break;

				#ifdef  _PACKET_DEBUG
				MSG_STANDARD * debug = (MSG_STANDARD*) Msg;
				if(BASE_CheckPacket(debug))
				{	
					sprintf(temp, "**PACKET_DEBUG** Type:%d Size:%d", debug->Type, debug->Size);
					Log(temp, "-system", 0);
				}
				#endif

				if(Error == 1 || Error == 2)
				{	
					sprintf(temp, "err wsa_read (%d),%d", Error, ErrorCode);
					Log(temp, "-system", 0);

					break;
				} 

				ProcessClientMessage(User, Msg);
			} while(1);

		} break;

	case WSA_ACCEPTADMIN:
		{
			 ReadAdmin();

			 if(WSAGETSELECTERROR(lParam))
			 {
				 Log("err,accept_admin accepting fails", "-system", 0);

				 break;
			 }

			 TempUser.AcceptUser(AdminSocket.Sock, WSA_READADMIN);

			 unsigned char *cIP = (unsigned char *)&(TempUser.IP);
			 int User = -1;
			 int i = 0;

			 for(i = 0; i <= MAX_ADMIN; i++)
			 {   
				 if(pAdmin[i].IP == TempUser.IP) 
				 {
					 User = i;
					 break;
				 }
			 }

			 if(User == -1)
			 {
				 int i = 0;

				 for(i = 0; i < MAX_ADMIN; i++)
				 {	
					 int a, b, c, d; a = b = c = d = 0;

					 if(pAdminIP[i] == 0) 
						 continue;

					 if(pAdminIP[i] == TempUser.IP) 
					 {
						 User = i;
						 break;
					 }
				 }

				 if(i == MAX_ADMIN)
				 {	
					 TempUser.cSock.CloseSocket();

					 char temp[256];   
					 sprintf(temp, "err, wsa_acceptadmin request accept from  %d.%d.%d.%d", cIP[0], cIP[1], cIP[2], cIP[3]);
					 Log(temp, "-system", 0);
				 }
			 }

			 if(User >= 0 && User < MAX_ADMIN)
			 {
				 if(pAdmin[User].Mode != USER_EMPTY)
				 {	
					 char temp[256];
					 sprintf(temp, "err, wsa_acceptadmin not empty previous slot %d", i);
					 Log(temp, "-system", 0);

					 TempUser.cSock.CloseSocket();

					 pAdmin[User].cSock.CloseSocket();
					 pAdmin[User].Mode = USER_EMPTY;

					 break;
				 } 
			 }   
			 else
			 {   
				 Log("err,accept_admin unknown attempt", "-system", 0);

				 TempUser.cSock.CloseSocket();

				 break;
			 }
			 
			 if(User == -1)
			 {	
				 Log("err,accept_admin no empty", "-system", 0);
				 break;
			 } 

			 pAdmin[User].Level               = -1;
			 pAdmin[User].DisableID           = 0;
			 pAdmin[User].IP                  = TempUser.IP;
			 pAdmin[User].Mode                = TempUser.Mode;
			 pAdmin[User].Name[0]			 = 0;
			 pAdmin[User].cSock.Sock          = TempUser.cSock.Sock;
			 pAdmin[User].cSock.nRecvPosition = TempUser.cSock.nRecvPosition;
			 pAdmin[User].cSock.nProcPosition = TempUser.cSock.nProcPosition;
			 pAdmin[User].cSock.nSendPosition = TempUser.cSock.nSendPosition;

			 cIP = (unsigned char *)&(pAdmin[User].IP);

			 char tmp[100];
			 sprintf(tmp, "sys,wsa_acceptadmin %d.%d.%d.%d <%d>", cIP[0], cIP[1], cIP[2], cIP[3], User);
			 Log(tmp, "-system", 0);

			 pAdmin[User].Encode1 = rand() % 10000;
			 pAdmin[User].Encode2 = rand() % 10000;
			 
			 MSG_NPIDPASS sm;
			 memset(&sm, 0, sizeof(MSG_NPIDPASS));

			 sm.Type = _MSG_NPReqIDPASS;
			 sm.Size = sizeof(MSG_NPIDPASS);
			 sm.ID = 0;

			 sm.Encode1 = pAdmin[User].Encode1;
			 sm.Encode2 = pAdmin[User].Encode2;

			 pAdmin[User].cSock.SendOneMessage((char*)&sm, sizeof(MSG_NPIDPASS));
		} break;

	case WSA_ACCEPT:
		{
			if(WSAGETSELECTERROR(lParam)) 
			{	
				Log("err WSAGETSELECTERROR - accepting new client.", "-system", 0);
				break;
			}

			TempUser.AcceptUser(ListenSocket.Sock, WSA_READ);

			unsigned char *cIP = (unsigned char *)&(TempUser.IP);

			char sIP[256]; 
			
			sprintf(sIP, "%d.%d.%d.%d", cIP[0], cIP[1], cIP[2], cIP[3]);

			int User = -1;

			for(int i = 0; i < MAX_SERVER; i++)
			{
				if(pUser[i].IP == TempUser.IP) 
				{
					User = i;
					break;
				}
			}

			if(User == -1)
			{ 
				int i = 1;

				for(; i < MAX_SERVERNUMBER; i++)
				{	
					if(!strcmp(g_pServerList[ServerIndex][i], sIP)) 
						break;
				}

				if(i == MAX_SERVERNUMBER)
				{	
					TempUser.cSock.CloseSocket();

					char temp[256];   
					sprintf(temp, "err wsa_accept request from  %s", sIP );

					Log(temp, "-system", 0);

					break;
				}

				User = i - 1;
			}
			else if(User >= 0 && User < MAX_SERVERNUMBER)
			{   
				if(pUser[User].Mode != USER_EMPTY)
				{	
					char temp[256];
					sprintf(temp, "err wsa_accept no previous slot %d", User );

					Log(temp, "-system", 0);

					TempUser.cSock.CloseSocket();
					TempUser.Mode = 0;

					pUser[User].cSock.CloseSocket();
					pUser[User].Mode = 0;

					break;
				}
			}
			else
			{
				Log("err wsa_accept unknown attempt", "-system", 0);
				TempUser.cSock.CloseSocket();
			}

			if(User == -1)
			{	
				Log("err wsa_accept no empty", "-system", 0);

				break;
			}

			if(cIP[0] == LocalIP[0] && cIP[1] == LocalIP[1] && cIP[2] == LocalIP[2])
			{
				pUser[User].IP                  = TempUser.IP;
				pUser[User].Mode                = TempUser.Mode;
				pUser[User].cSock.Sock          = TempUser.cSock.Sock;
				pUser[User].cSock.nRecvPosition = TempUser.cSock.nRecvPosition;
				pUser[User].cSock.nProcPosition = TempUser.cSock.nProcPosition;
				pUser[User].cSock.nSendPosition = TempUser.cSock.nSendPosition;

				cFileDB.SendDBSignalParm3(User, 0, _MSG_DBSetIndex, ServerIndex, Sapphire, User);

				for(int i = 0; i < 65536; i++)
				{
					if(GuildInfo[i].Citizen)
						cFileDB.SendGuildInfo(User, i);

					if(g_pGuildWar[i])
						cFileDB.SendDBSignalParm2(User, 0, _MSG_War, i, g_pGuildWar[i]);

					if(g_pGuildAlly[i])
						cFileDB.SendDBSignalParm2(User, 0, _MSG_GuildAlly, i, g_pGuildAlly[i]);
				}

				if(TransperCharacter)
					cFileDB.SendDBSignalParm2(User, 0, _MSG_TransperCharacter, 0, 0);

				break;
			}
			else
			{
				TempUser.cSock.CloseSocket();

				char temp[256];
				sprintf(temp, "err,wsa_accept outer ethernet IP: %d.%d.%d.%d", cIP[0], cIP[1], cIP[2], cIP[3]);

				Log(temp, "-system", 0);

				break;
			}
		  		

		} break;

	case WM_CREATE:
	{
			HMENU hMenu, hSubMenu;
			// HICON hIcon, hIconSm;

			hMenu = CreateMenu();

			hSubMenu = CreatePopupMenu();
			AppendMenu(hSubMenu, MF_STRING, IDC_EXIT, "&Exit");
			AppendMenu(hMenu, MF_STRING | MF_POPUP, (UINT)hSubMenu, "&File");

			hSubMenu = CreatePopupMenu();
			AppendMenu(hSubMenu, MF_STRING, IDC_SHUTDOWNNP, "&Reload MobBase");
			AppendMenu(hMenu, MF_STRING | MF_POPUP, (UINT)hSubMenu, "&Sistema");

			SetMenu(hWnd, hMenu);
	} break;

	case WM_COMMAND:
	{
		switch(LOWORD(wParam))
		{
			case IDC_EXIT:
				PostMessage(hWnd, WM_CLOSE, 0, 0);
			break;

			case IDC_SHUTDOWNNP:
			{
#pragma region Load Base Characters

				int handle = -1;

				handle = _open("./BaseMob/TK", _O_RDONLY | _O_BINARY);

				if (handle == -1)
				{
					MessageBoxA(hWndMain, "no TransKnight file", "BOOTING ERROR", MB_OK);

					return FALSE;
				}

				_read(handle, &g_pBaseSet[0], sizeof(STRUCT_MOB));
				_close(handle);


				handle = -1;

				handle = _open("./BaseMob/FM", _O_RDONLY | _O_BINARY);

				if (handle == -1)
				{
					MessageBoxA(hWndMain, "no Foema file", "BOOTING ERROR", MB_OK);

					return FALSE;
				}

				_read(handle, &g_pBaseSet[1], sizeof(STRUCT_MOB));
				_close(handle);

				handle = -1;

				handle = _open("./BaseMob/BM", _O_RDONLY | _O_BINARY);

				if (handle == -1)
				{
					MessageBoxA(hWndMain, "no BeastMaster file", "BOOTING ERROR", MB_OK);

					return FALSE;
				}

				_read(handle, &g_pBaseSet[2], sizeof(STRUCT_MOB));
				_close(handle);

				handle = -1;

				handle = _open("./BaseMob/HT", _O_RDONLY | _O_BINARY);

				if (handle == -1)
				{
					MessageBoxA(hWndMain, "no Huntress file", "BOOTING ERROR", MB_OK);

					return FALSE;
				}

				_read(handle, &g_pBaseSet[3], sizeof(STRUCT_MOB));
				_close(handle);

				g_pBaseSet[0].BaseScore = g_pBaseSet[0].CurrentScore;
				g_pBaseSet[1].BaseScore = g_pBaseSet[1].CurrentScore;
				g_pBaseSet[2].BaseScore = g_pBaseSet[2].CurrentScore;
				g_pBaseSet[3].BaseScore = g_pBaseSet[3].CurrentScore;

#pragma endregion
			} break;
		}
	} break;

	case WM_PAINT:
		PAINTSTRUCT ps;
		memset(&ps, 0, sizeof(PAINTSTRUCT));

		BeginPaint(hWnd, &ps);

		hFont = CreateFont(12, 0, 0, 0, FW_LIGHT, FALSE, FALSE, FALSE, DEFAULT_CHARSET, OUT_DEVICE_PRECIS, CLIP_DEFAULT_PRECIS, DEFAULT_QUALITY, DEFAULT_PITCH, "Fixedsys"); // |FF_DECORATIVE

		hDC = ps.hdc;

		DrawConfig();

		EndPaint(hWnd, &ps);
		break; 

	case WM_CLOSE:
		{	
			int rret = MessageBox(hWndMain, "Desligando servidor, não se esqueça de salvar as contas !", "Aviso !! Desligando Servidor !!", MB_OKCANCEL | MB_ICONWARNING);

			if(rret == 1)
			{
				WriteConfig();

				if (fLogFile)
					fclose(fLogFile);

				DayLog_ExpLog();
				DayLog_ItemLog();

				DefWindowProc(hWnd,message,wParam,lParam);
			}	

			return true;			

		} break;

	case WM_DESTROY:
		WSACleanup();
		PostQuitMessage(0);
		break;

	default:
		return DefWindowProc(hWnd, message, wParam, lParam);

	}

	return 0;
}

void ProcessClientMessage(int conn, char *msg)
{
	MSG_STANDARD *std = (MSG_STANDARD *)msg;

	if (!(std->Type & FLAG_GAME2DB) || (std->ID < 0) || (std->ID > MAX_USER)) 
	{	
		char temp[256];

		MSG_STANDARD *m = (MSG_STANDARD *)msg;

		sprintf(temp, "err,packet Type:%d ID:%d Size:%d KeyWord:%d", m->Type, m->ID, m->Size, m->KeyWord);

		Log(temp,"-system",0);

		return;
	}

	cFileDB.ProcessMessage(msg, conn);
}

void DisableAccount(int conn, char *account, int Year, int YearDay)
{
	account[ACCOUNTNAME_LENGTH-1] = 0;
	account[ACCOUNTNAME_LENGTH-2] = 0;

	_strupr(account);

	STRUCT_ACCOUNTFILE file;
	memcpy(file.Info.AccountName, account, ACCOUNTNAME_LENGTH);

	int iret = cFileDB.DBReadAccount(&file); 

	if(iret == FALSE) 
	{	
		if(conn >= 0) 
			SendAdminSignal(conn, 0, _MSG_NPNotFound);

		if(conn >= 0) 
			SendAdminMessage(conn, 0, "There's no account with that account name");

		return;
	}
	
	if(file.Info.YearDay != 0 && file.Info.Year != 0) 
	{	
		if(conn >= 0) 
			SendAdminState(conn, 0, _MSG_NPState, "1");

		return;
	}

	file.Info.AccountPass[ACCOUNTPASS_LENGTH-1] = 0;
	file.Info.AccountPass[ACCOUNTPASS_LENGTH-2] = 0;

	file.Info.Year = Year;
	file.Info.YearDay = YearDay;

	cFileDB.DBWriteAccount(&file);

	if(conn >= 0)
	{	
		int idx = pAdmin[conn].DisableID;

		MSG_NPReqAccount sm; 
		memset(&sm, 0, sizeof(MSG_NPReqAccount));

		sm.Type = _MSG_NPReqAccount; 
		sm.Size = sizeof(MSG_NPReqAccount);
		sm.ID = idx;

		strncpy(sm.Account, account, ACCOUNTNAME_LENGTH);

		sm.Char[0] = 0;

		ProcessAdminMessage(conn, (char*)&sm);

		pAdmin[conn].DisableID = 0;
	}
}

void EnableAccount(int conn, char *account)
{
	account[ACCOUNTNAME_LENGTH-1] = 0;
	account[ACCOUNTNAME_LENGTH-2] = 0;

	_strupr(account);

	STRUCT_ACCOUNTFILE file;
  	memcpy(file.Info.AccountName, account, ACCOUNTNAME_LENGTH);

	int iret = cFileDB.DBReadAccount(&file);  

	if(iret == FALSE) 
	{	
		if(conn >= 0) 
			SendAdminSignal(conn, 0, _MSG_NPNotFound);

		if(conn >= 0) 
			SendAdminMessage(conn, 0, "There's no account with that account name");

		return;
	}

	if(file.Info.Year == 0 && file.Info.YearDay == 0) 
	{	
		if(conn >= 0) 
			SendAdminState(conn, 0, _MSG_NPState, file.Info.AccountPass);

		return;
	}

	file.Info.AccountPass[ACCOUNTPASS_LENGTH-1] = 0;
	file.Info.AccountPass[ACCOUNTPASS_LENGTH-2] = 0;
	file.Info.Year = 0;
	file.Info.YearDay = 0;

	cFileDB.DBWriteAccount(&file);

	if(conn >= 0)
		SendAdminState(conn, 0, _MSG_NPState, file.Info.AccountPass);

	return ;
}

int ProcessAdminMessage(int conn, char *msg)
{
	MSG_STANDARD *std = (MSG_STANDARD *)msg;

	if(!(std->Type & FLAG_NP2DB) || (std->ID < 0) || (std->ID >= MAX_DBACCOUNT))
	{	
		MSG_STANDARD *m = (MSG_STANDARD *)msg;

		char temp[128];
		sprintf(temp, "err,packet:%d ID:%d Size:%d KeyWord:%d", m->Type, m->ID, m->Size, m->KeyWord);

		Log(temp,"-system",0);

		return FALSE;
	} 

	switch(std->Type)
	{
	case _MSG_NPCreateCharacter:
		{
			MSG_NPCreateCharacter *m = (MSG_NPCreateCharacter*)msg;

			MSG_NPCreateCharacter_Reply	sm; 
			memset(&sm, 0, sizeof(MSG_NPCreateCharacter_Reply));
			sm.Type = _MSG_NPCreateCharacter_Reply;
			sm.Size		= sizeof(MSG_NPCreateCharacter_Reply);
			sm.Slot		= m->Slot;
			sm.ID		= m->ID;
			sm.Result	= 0;

			strncpy(sm.Account, m->Account, ACCOUNTNAME_LENGTH);
			strncpy(sm.Name, m->Mob.MobName, NAME_LENGTH);

			char *account	= m->Account;
			int slot		= m->Slot;
			int IdxName		= cFileDB.GetIndex(m->Account);

			if(IdxName != 0)	
			{
				sm.Result = 3;

				pAdmin[conn].cSock.SendOneMessage((char*)&sm, sm.Size);

				return TRUE;
			}

			STRUCT_ACCOUNTFILE file;

			memcpy(file.Info.AccountName, m->Account, ACCOUNTNAME_LENGTH);

			int ret = cFileDB.DBReadAccount(&file);

			if(ret == FALSE)
			{	
				char tt[256]; 
				sprintf(tt, "etc,tranper_FAILED!!!!!(read) %s %s", m->Account, m->Mob.MobName);
				Log(tt, file.Info.AccountName, 0);

				sm.Result = 4;

				pAdmin[conn].cSock.SendOneMessage((char*)&sm, sm.Size);

				return TRUE;
			}

			int empty;

			for(empty = 0; empty < MOB_PER_ACCOUNT; empty++) 
			{	
				if(file.Char[empty].MobName[0] == 0) 
					break;
			}

			if(empty == MOB_PER_ACCOUNT)	
			{
				sm.Result = 2;
				
				pAdmin[conn].cSock.SendOneMessage((char*)&sm, sm.Size);

				return TRUE;
			}

			ret = cFileDB.CreateCharacter(m->Account, m->Mob.MobName);

			if(ret == FALSE)
			{	
				sm.Result = 1;

				pAdmin[conn].cSock.SendOneMessage((char*)&sm, sm.Size);

				return TRUE;
			}

			for(int i = 0; i < MAX_EQUIP + MAX_CARRY -1 ; i++)
			{
				STRUCT_ITEM *sour;

				if(i < 16)
					sour = &m->Mob.Equip[i];
				else
					sour = &m->Mob.Carry[i];

				int sidx = sour->sIndex;

				if(sidx == 446 || (sidx >= 508 && sidx <= 509) || (sidx >= 526 && sidx <= 537) || sidx == 522)
					ConvertGuildNumber(conn, sour);
			}
	
			char tt[256]; 
			sprintf(tt, "etc,tranper_success(deleted) %s %s", m->Account, m->Mob.MobName);
			Log(tt, file.Info.AccountName, 0);

			memcpy(&file.Char[empty], &(m->Mob), sizeof(STRUCT_MOB));

			_strupr(file.Info.AccountName);

			ret = cFileDB.DBWriteAccount(&file);

			if(ret == FALSE)
			{	
				char tt[256]; 
				sprintf(tt, "etc,tranper_FAILED!!!!!(write) %s %s", m->Account, m->Mob.MobName);
				Log(tt, file.Info.AccountName, 0);

				sm.Result = 4;
				
				pAdmin[conn].cSock.SendOneMessage((char*)&sm, sm.Size);

				return TRUE;
			}

			sm.Result = 0;
			
			pAdmin[conn].cSock.SendOneMessage((char*)&sm, sm.Size);

		} break;

	case _MSG_NPNotice:
		{
			MSG_NPNotice *m = (MSG_NPNotice*)msg;

			m->AccountName[ACCOUNTNAME_LENGTH-1] = 0;
			m->AccountName[ACCOUNTNAME_LENGTH-2] = 0;
			m->String[MAX_NOTIFY_LENGTH-1] = 0;
			m->String[MAX_NOTIFY_LENGTH-2] = 0;

			if(m->AccountName[0] == 0 && m->Parm1 == 1)
			{
				if(pAdmin[conn].Level < 2)
					return TRUE;

				for (int i = 0; i < MAX_SERVER; i++)
				{	
					if(pUser[i].Mode == USER_EMPTY)
						continue;

					if(pUser[i].cSock.Sock == NULL)	
						continue;

					MSG_NPNotice sm;
					memset(&sm, 0, sizeof(MSG_NPNotice));

					sm.Type = _MSG_NPNotice;
					sm.ID = 0;
					sm.Size = sizeof(MSG_NPNotice);

					sm.Parm1 = 1;
					sm.Parm2 = 1;

					strncpy(sm.String, m->String, MAX_NOTIFY_LENGTH);

					pUser[i].cSock.SendOneMessage((char*)&sm, sizeof(sm));
				}

				return TRUE;
			}

			if(pAdmin[conn].Level <= 0)
				return TRUE;

			int IdxName = cFileDB.GetIndex(m->AccountName);

			if(IdxName == 0)
			{   
				SendAdminMessage(conn, 0, "Specified user not connected. can't send notice.");

				return TRUE;
			}

			int svr = IdxName / MAX_USER;
			int id = IdxName % MAX_USER;

			if(svr < 0|| svr >= MAX_SERVER || id <= 0 || id >= MAX_USER)
			{	
				SendAdminMessage(conn, 0, "Wrong SVR and ID");

				return TRUE;
			}

			MSG_NPNotice sm;
			memset(&sm, 0, sizeof(MSG_NPNotice));

			sm.Type = _MSG_NPNotice;
			sm.ID = id;
			sm.Size = sizeof(MSG_NPNotice);

			sm.Parm1 = 0;
			sm.Parm2 = 0;

			strncpy(sm.String, m->String, MAX_NOTIFY_LENGTH);

			pUser[svr].cSock.SendOneMessage((char*)&sm, sizeof(sm));

			return TRUE;

		} break;

	case _MSG_NPIDPASS:
		{
			MSG_NPIDPASS *m = (MSG_NPIDPASS *) msg;

			_strupr(m->Account);

			if(pAdmin[conn].Encode1 != m->Encode1 || pAdmin[conn].Encode2 != m->Encode2) 
				return FALSE;

			if(pAdmin[conn].Level != -1) 
				return TRUE;

			STRUCT_ACCOUNTFILE file;

			memcpy(file.Info.AccountName, m->Account, ACCOUNTNAME_LENGTH);

			int iret = cFileDB.DBReadAccount(&file);  

			if(iret == FALSE) 
				return TRUE;

			if(file.Info.AccountPass[0] == '_') 
				return FALSE;
			if(file.Info.AccountPass[0] == '@') 
				return FALSE; 

			char *p = (char*)file.Info.AccountPass;

			iret = strncmp(m->Pass, p, ACCOUNTPASS_LENGTH);

			if(iret) 
				return FALSE;

			int maxlevel = 0;

			for (int i = 0; i < MOB_PER_ACCOUNT; i++)
			{  
				if (file.Char[i].MobName[0] != 0 && file.Char[i].BaseScore.Level > maxlevel) 
					maxlevel = file.Char[i].BaseScore.Level;
			}

			if(maxlevel < 1000) 
				return FALSE;

			int admin = maxlevel -  1000;

			pAdmin[conn].Level = admin;
			pAdmin[conn].DisableID = 0;

			strncpy(pAdmin[conn].Name, m->Account, ACCOUNTNAME_LENGTH);

			char temp[256];
			sprintf(temp, "sys,Admin Login Success - Level: %d", admin);
			Log(temp, pAdmin[conn].Name, pAdmin[conn].IP);  
			SendAdminMessage(conn, 0, temp);

		} break;

	case _MSG_NPReqAccount: 
		{
			MSG_NPReqAccount *m = (MSG_NPReqAccount*)msg;

			if(pAdmin[conn].Level <= 0)
				return TRUE;

			_strupr(m->Account);
			_strupr(m->Char);

			if(m->Char[0] != 0)		
				cFileDB.GetAccountByChar(m->Account, m->Char); 


			if(m->Account[0] == 0)
			{
				SendAdminSignal(conn, 0, _MSG_NPNotFound);
				SendAdminMessage(conn, 0, "There's no account with that character name");

				return TRUE;
			}  

			STRUCT_ACCOUNTFILE file;

			memcpy(file.Info.AccountName, m->Account, ACCOUNTNAME_LENGTH);

			int iret = cFileDB.DBReadAccount(&file);

			if(iret == FALSE)
			{	
				SendAdminSignal(conn, 0, _MSG_NPNotFound);
				SendAdminMessage(conn, 0, "There's no account with that account name");

				return TRUE;
			}

			MSG_NPAccountInfo sm; 
			memset(&sm, 0, sizeof(MSG_NPAccountInfo));

			sm.Type = _MSG_NPAccountInfo; 
			sm.Size = sizeof(MSG_NPAccountInfo);
			sm.ID = 0;
			sm.account = file; 

			int IdxName = cFileDB.GetIndex(m->Account);

			sm.Session = IdxName;

			file.Char[0].MobName[NAME_LENGTH-1] = 0;
			file.Char[0].MobName[NAME_LENGTH-2] = 0;
			file.Char[1].MobName[NAME_LENGTH-1] = 0;
			file.Char[1].MobName[NAME_LENGTH-2] = 0;
			file.Char[2].MobName[NAME_LENGTH-1] = 0;
			file.Char[2].MobName[NAME_LENGTH-2] = 0;
			file.Char[3].MobName[NAME_LENGTH-1] = 0;
			file.Char[3].MobName[NAME_LENGTH-2] = 0;
			file.Info.AccountName[ACCOUNTNAME_LENGTH-1] = 0;
			file.Info.AccountName[ACCOUNTNAME_LENGTH-2] = 0;

			if(pAdmin[conn].Level <= 0)
				return TRUE;

			int maxlevel = 0;

			for(int i = 0; i < MOB_PER_ACCOUNT; i++)
			{
				if (file.Char[i].MobName[0] != 0 && file.Char[i].BaseScore.Level > maxlevel) 
					maxlevel = file.Char[i].BaseScore.Level;

			}

			int admin = maxlevel - 1000;

			sm.State = 0;

			char accountstate[256];
			memset(accountstate, 0, 255);

			if (sm.account.Info.AccountPass[0] == '@' || sm.account.Info.Year != 0) 
			{
				sprintf(accountstate, "Conta bloqueada huehuheuhe.");
				sm.State = 1;
			}
			if (sm.account.Info.AccountPass[0] == '_') 
			{
				sprintf(accountstate, "Conta Defeituosa.");
				sm.State = 2;
			}
			if (sm.account.Info.AccountPass[0] == '#') 
			{
				sprintf(accountstate, "Conta Desativada.");
				sm.State = 3;
			}

			char temp[256]; 			
			sprintf(temp, "%s < %s | %s | %s | %s> - %s", file.Info.AccountName, file.Char[0].MobName, file.Char[1].MobName, file.Char[2].MobName, file.Char[3].MobName, accountstate);

			temp[MESSAGE_LENGTH-1] = 0;
			temp[MESSAGE_LENGTH-2] = 0;

			SendAdminMessage(conn, 0, temp);
			/*
			sm.account.Info.AccountPass[0] = rand() % 24 + 'A';
			sm.account.Info.AccountPass[1] = rand() % 24 + 'A';
			sm.account.Info.AccountPass[2] = rand() % 24 + 'A';
			sm.account.Info.AccountPass[3] = rand() % 24 + 'A';
			sm.account.Info.AccountPass[4] = 0;
			*/
			pAdmin[conn].cSock.SendOneMessage((char*)&sm, sizeof(MSG_NPAccountInfo));

		} break;

	case _MSG_NPReqSaveAccount:
		{
			if(pAdmin[conn].Level <= 2)
			{
				SendAdminMessage(conn, 0, "Not allowed");

				return TRUE;
			}

			MSG_NPAccountInfo *m = (MSG_NPAccountInfo*)msg;

			int IdxName = cFileDB.GetIndex(m->account.Info.AccountName);

			if(IdxName != 0)
			{	
				SendAdminMessage(conn, 0, "For saving, account should be disabled.");

				return TRUE;
			}

			int maxlevel = 0;

			for(int i = 0; i < MOB_PER_ACCOUNT; i++)
			{

				if (m->account.Char[i].MobName[0] != 0 && m->account.Char[i].BaseScore.Level > maxlevel) 
					maxlevel = m->account.Char[i].BaseScore.Level;

			}

			int admin = maxlevel - 1000;

			if(maxlevel >= 2000)
			{
				if(admin > pAdmin[conn].Level)
				{
					SendAdminMessage(conn, 0, "Set admin level error.");

					return TRUE;
				}

				if(pAdmin[conn].Level == admin && strcmp(pAdmin[conn].Name, m->account.Info.AccountName))
				{
					SendAdminMessage(conn, 0, "Set admin level error.");

					return TRUE;
				}

			}

			STRUCT_ACCOUNTFILE tmpact;

			memcpy(tmpact.Info.AccountName, m->account.Info.AccountName, ACCOUNTNAME_LENGTH);

			int iret = cFileDB.DBReadAccount(&tmpact);

			if(!iret)
			{
				SendAdminMessage(conn, 0, "There's no account with that account name");

				return TRUE;
			}

			memcpy(m->account.Info.AccountPass, tmpact.Info.AccountPass, ACCOUNTPASS_LENGTH);

			cFileDB.DBWriteAccount(&m->account);

			char temp[128]; 
			sprintf(temp, "Saving account [%s] success", m->account.Info.AccountName);
			SendAdminMessage(conn, 0, temp);

			sprintf(temp, "sys,%s written", m->account.Info.AccountName);

			Log(temp, pAdmin[conn].Name, pAdmin[conn].IP);

		} break;

	case _MSG_NPDisable:
		{
			if(pAdmin[conn].Level <= 2) 
				return TRUE;

			MSG_NPEnable *m = (MSG_NPEnable*)msg;

			m->AccountName[ACCOUNTNAME_LENGTH-1] = 0;
			m->AccountName[ACCOUNTNAME_LENGTH-2] = 0;

			_strupr(m->AccountName);

			int IdxName = cFileDB.GetIndex(m->AccountName);

			if(IdxName == 0)
			{	
				DisableAccount(conn, m->AccountName, m->Year, m->YearDay);

				return TRUE;
			}	
			else
			{	
				cFileDB.SendDBSavingQuit(IdxName, 1);
				pAdmin[conn].DisableID = IdxName;
				pAdmin[conn].Year = m->Year;
				pAdmin[conn].YearDay = m->YearDay;
			}

		} break;

	case _MSG_NPEnable:
		{
			if(pAdmin[conn].Level <= 2) 
				return TRUE;

			MSG_NPEnable *m = (MSG_NPEnable*)msg;

			m->AccountName[ACCOUNTNAME_LENGTH-1] = 0;
			m->AccountName[ACCOUNTNAME_LENGTH-2] = 0;

			_strupr(m->AccountName);

			int IdxName = cFileDB.GetIndex(m->AccountName);

			if (IdxName != 0)
			{	
				SendAdminMessage(conn, 0, "Check session. already playing.");

				return TRUE;
			}

			EnableAccount(conn, m->AccountName);	

		} break;

	case _MSG_NPDonate:
		{
			MSG_NPDonate *m = (MSG_NPDonate*)msg;

			char filename[256];
			char name[256];


			struct tm when;
			time_t now;
			time(&now);
			when = *localtime(&now);

			sprintf(name, "%02d%02d%04d%02d%02d%02d%02d", when.tm_mday, when.tm_mon, when.tm_year+1900, when.tm_hour, when.tm_min, when.tm_sec, rand()%255);
			sprintf(filename, "../../Common/ImportDonate/%s", name);

			FILE *fp = fopen(filename, "wt");

			if(fp == NULL)
				break;

			fprintf(fp, "%s %d", m->AccountName, m->Donate);

			fclose(fp);
		} break;
	}

	return TRUE;

}

void ProcessSecTimer()
{
	CReadFiles::ImportUser();
	CReadFiles::UpdateUser();

	SecCounter++;

	if(SecCounter % 1 == 0) 
	{
		CReadFiles::ImportItem(); // Once every two minutes//
		CReadFiles::ImportDonate();
	}

	DrawConfig();

	if(SecCounter % 1 == 0)
	{
		CReadFiles::UpdateConnection();

		MinCounter++;

		if(MinCounter % 1 == 0)
		{
			CReadFiles::UpdateConnectionData();
			HourCounter++;
		}
	}

	if (SecCounter % 600 == 0)
	{
#pragma region Gerador de Ranking
		CReadFiles::WriteRanking();
#pragma endregion
	}
	struct tm when;
	time_t now;
	time(&now);
	when = *localtime(&now);

	if (when.tm_mday != LastLogDay)
		StartLog("A");

	if (when.tm_mday != LastDayLogDay)
	{
		DayLog_ItemLog();
		DayLog_ExpLog();
		LastDayLogDay = when.tm_mday;
	}
	if(when.tm_hour == 0 && when.tm_wday == 0 && when.tm_min == 0 && when.tm_sec == 0)
	{
		for(int i = 0; i < 65536;i++)
		{
			if(GuildInfo[i].Fame)
			{
				GuildInfo[i].Fame = 0;

				for(int j = 0; j < MAX_SERVER; j++)
				{
					if(pUser[j].cSock.Sock == 0)
						continue;

					if(pUser[j].Mode == 0)
						continue;

					cFileDB.SendGuildInfo(j, i);
				}
			}
		}

		CReadFiles::WriteGuildInfo();
	}
}

void ProcessMinTimer()
{

}

void StartLog(char *cccc)
{
	 char Temp[256];

     if(fLogFile != NULL)
	 {  
		 int ret = fclose(fLogFile);

		 if(ret)
			 Log("Logfile close fail!!", "-system", 0);
	 }

	 struct tm when;
	 time_t now;
	 time(&now); 
	 when = *localtime(&now);

	 sprintf(Temp, ".\\Log\\DB_%02d_%02d_%04d_%02d_%02d_%02d_%s.txt", when.tm_mday, when.tm_mon + 1, when.tm_year + 1900, when.tm_hour, when.tm_min, when.tm_sec, cccc);

   	 fLogFile = fopen(Temp, "a+");

	 LastLogDay = when.tm_mday;
}

void Log(char *str1, char *str2, unsigned int ip)
{
	struct tm when;
	time_t now;
	time(&now);
	when = *localtime(&now);

	char LogTemp[1024];

	unsigned char *cIP = (unsigned char*)&ip;

	//sprintf(LogTemp, "%2.2d%2.2d%2.2d,%2.2d%2.2d%2.2d,%8.8x", when.tm_year - 100, when.tm_mon + 1, when.tm_mday, when.tm_hour, when.tm_min, when.tm_sec, ip);
	//sprintf(LogTemp, "%s,%s,%s \n", LogTemp, str2, str1);

	if (ip != 0)
		sprintf(LogTemp, "[%02d/%02d/%04d][%02d:%02d:%02d] IP: %d.%d.%d.%d", when.tm_mday, when.tm_mon + 1, when.tm_year + 1900, when.tm_hour, when.tm_min, when.tm_sec, cIP[0], cIP[1], cIP[2], cIP[3]);
	else
		sprintf(LogTemp, "[%02d/%02d/%04d][%02d:%02d:%02d] ", when.tm_mday, when.tm_mon + 1, when.tm_year + 1900, when.tm_hour, when.tm_min, when.tm_sec);
	sprintf(LogTemp, "%s %s %s \n", LogTemp, str2, str1);

	if (fLogFile)
		fprintf(fLogFile, LogTemp);

	SetWindowText(hWndMain, LogTemp);
}

void DayLog_ExpLog()
{
	FILE *fp = NULL;

	char Temp[1024];
	// char Temp2[1024];

	struct tm when;
	time_t now;
	time(&now);
	when = *localtime(&now);

	sprintf(Temp, ".\\DayLog\\EXP_DB_%02d_%02d_%04d_%02d_%02d_%02d.txt", when.tm_mday, when.tm_mon + 1, when.tm_year + 1900, when.tm_hour, when.tm_min, when.tm_sec);

	fp = fopen(Temp, "a+");

	if (fp == NULL)
	{
		Log("err,daylog item open fail!!", "-system", 0);
		return;
	}

	DayLog_ReadAccountInDir("./account/*", fp);

	fclose(fp);
}


int GetUserFromSocket(int Sock)
{
	for(int i = 0; i < MAX_SERVER; i++)
	{   
		if(pUser[i].cSock.Sock == (unsigned)Sock) 
			return i;
	}

	return -1;
}

int GetAdminFromSocket(int Sock)
{
	for(int i = 0; i < MAX_ADMIN; i++)
	{   
		if(pAdmin[i].cSock.Sock == (unsigned)Sock) 
			return i;
	}

	return -1;
}

int GetEmptyUser()
{
	for(int i = 0; i < MAX_SERVER; i++)
	{   
		if(pUser[i].Mode == USER_EMPTY) 
			return i;
	}

	return -1;
}

void DayLog_ReadAccountInDir(char *dir, FILE *log)
{
	HANDLE handle;
	WIN32_FIND_DATA win32_find_data;

	char tmp[1024];
	// char tmp2[1024];

	handle = FindFirstFile(dir, &win32_find_data);

	if (handle == INVALID_HANDLE_VALUE)
	{
		// TODO: Error: can't read initial file
	}
	else
	{
		while (FindNextFile(handle, &win32_find_data))
		{
			char *fileName = win32_find_data.cFileName;

			if (!strncmp(fileName, ".", 1))
				continue;

			if (win32_find_data.dwFileAttributes == FILE_ATTRIBUTE_DIRECTORY)
			{
				sprintf(tmp, "./account/%s/*", fileName);
				DayLog_ReadAccountInDir(tmp, log);
			}
			else
			{
				SYSTEMTIME fWriteTime;
				SYSTEMTIME sysTime;
				FileTimeToSystemTime(&win32_find_data.ftLastWriteTime, &fWriteTime);
				GetLocalTime(&sysTime);

				// Only use fresh active accounts 
				if (fWriteTime.wYear != sysTime.wYear || fWriteTime.wMonth != sysTime.wMonth)
					continue;

				strcpy(tmp, dir);
				tmp[strlen(dir) - 1] = '\0';
				sprintf(tmp, "%s%s", tmp, fileName);

				errno = 0;
				FILE* fs;
				fopen_s(&fs, tmp, "rb");

				if (fs != NULL)
				{
					STRUCT_ACCOUNTFILE accBuffer;
					memset(&accBuffer, 0, sizeof(STRUCT_ACCOUNTFILE));

					fread(&accBuffer, sizeof(STRUCT_ACCOUNTFILE), 1, fs);

					struct tm when;
					time_t now;
					time(&now);
					when = *localtime(&now);

					for (int i = 0; i < MOB_PER_ACCOUNT; i++)
					{
						if (accBuffer.Char[i].MobName[0] == '\0')
							continue;

						if (accBuffer.mobExtra[i].DayLog.YearDay == when.tm_yday-1)
							fprintf(log, "[%02d/%02d/%04d][%02d:%02d:%02d] %s Char:%s recebeu %llu exp hoje.\n", when.tm_mday, when.tm_mon + 1, when.tm_year + 1900, when.tm_hour, when.tm_min, when.tm_sec, accBuffer.Info.AccountName, accBuffer.Char[i].MobName, accBuffer.mobExtra[i].DayLog.Exp);
						
					}

					fclose(fs);
				}
				else
				{
					// TODO: Cant open file to read
				}
			}
		}
	}

	FindClose(handle);
}

void DayLog_ItemLog()
{
	FILE *fp = NULL;

	char Temp[1024];

	struct tm when;
	time_t now;
	time(&now);
	when = *localtime(&now);

	sprintf(Temp, ".\\DayLog\\ITEM_DB_%02d_%02d_%04d_%02d_%02d_%02d.txt", when.tm_mday, when.tm_mon + 1, when.tm_year + 1900, when.tm_hour, when.tm_min, when.tm_sec);

	fp = fopen(Temp, "a+");

	if (fp == NULL)
	{
		Log("err,daylog item open fail!!", "-system", 0);
		return;
	}

	for (int i = 0; i < MAX_ITEMLIST; i++)
	{
		if (ItemDayLog[i].Num == 0)
			continue;

		sprintf(Temp, "[%02d/%02d/%04d][%02d:%02d:%02d] ", when.tm_mday, when.tm_mon + 1, when.tm_year + 1900, when.tm_hour, when.tm_min, when.tm_sec);

		sprintf(Temp, "%s ItemID:%d ItemName:%s ItemNum:%d \n", Temp, i, g_pItemList[i].Name, ItemDayLog[i].Num);

		if (fp)
			fprintf(fp, Temp);

		ItemDayLog[i].Num = 0;
	}

	fclose(fp);
}

BOOL file_exists(char *s, ...)
{
	char buf[1024];

	va_list pArgList;

	va_start(pArgList, s);
	vsprintf(buf, s, pArgList);
	va_end(pArgList);

	FILE *fp = fopen(buf, "r");

	if (fp)
	{
		fclose(fp);
		return TRUE;
	}

	return FALSE;
}