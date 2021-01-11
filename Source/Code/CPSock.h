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

#ifndef _CPSOCK_ // Last updated august 2012
#define _CPSOCK_ 

#include <Windows.h>

#define WSA_READ            (WM_USER + 100)
#define WSA_READDB          (WM_USER + 2) 
#define WSA_ACCEPT          (WM_USER + 3) 
#define WSA_READBILL		(WM_USER + 4)
#define WSA_ACCEPTADMIN     (WM_USER + 5) 
#define WSA_READADMIN		(WM_USER + 6) 
#define WSA_READADMINCLIENT	(WM_USER + 7) 

#define MAX_PENDING_CONNECTS  8

#define RECV_BUFFER_SIZE      (128*1024)         // Maximum buffer size to receive messages of 64k
#define SEND_BUFFER_SIZE      (128*1024)         // Maximum buffer size to send messages of 64K

#define MAX_MESSAGE_SIZE           8192         // Maximum size a single message can have, 4K

#define INITCODE               0x1F11F311

typedef struct _HEADER
{
	short		  Size;
	char		  KeyWord;
	char		  CheckSum;
	short		  Type;
	short		  ID;
	unsigned int  ClientTick; 
} HEADER, *PHEADER;


class  CPSock
{   
public:
	unsigned int  Sock;
	char   *pSendBuffer;
	char   *pRecvBuffer;
	int		nSendPosition;
	int		nRecvPosition;
	int		nProcPosition;
	int		nSentPosition;
	int     Init;	

public:
	CPSock();
	~CPSock();

	BOOL	CloseSocket			();
	BOOL	WSAInitialize		();
	SOCKET	StartListen			(HWND hWnd, int ip, int Port, int WSA);
	SOCKET	ConnectServer		(char *HostAddr, int Port, int ip, int WSA);
	SOCKET  ConnectBillServer	(char *HostAddr, int Port, int ip, int WSA);

	BOOL	Receive				();
	char   *ReadMessage			(int *ErrorCode, int *ErrorType);
	
	BOOL	AddMessage			(char *pMsg, int Size);
	BOOL	SendMessageA		();
	BOOL	SendOneMessage		(char* Msg, int Size);
	
	void	RefreshRecvBuffer	(void);
	void	RefreshSendBuffer	(void);
	BOOL	SendBillMessage		(char * Msg);
	char   *ReadBillMessage		(int *ErrorCode, int *ErrorType);
};

struct _AUTH_GAME // NEEDS TO BE FIXED ACCORDING TO WYD 1.2 6.13 SIZE IS 0xC4 (196)
{
	char Unk[196];	
};

/* FROM TANTRA
struct _AUTH_GAME{
	int 	Packet_Type;	//	4
	int		Result;			//	4 
	char	S_KEY[32];		//	4*8	 
	char	Session[32];	//  4*8
	char	User_CC[4];		//	4
	char	User_No[20];	//	4*5
	char	User_ID[52];	//  4*13
	char	User_IP[24];	//  4*6
	char	User_Gender;	
	char	User_Status[3];	//	4	
	char	User_PayType[4];//	2 ?
	int 	User_Age;		//  4
	int 	Game_No;		//  4
	char	Bill_PayType[2];
	char	Bill_Method[2];	//	4
	char	Bill_Expire[12];//	4*3 	
	int 	Bill_Remain;	//	4
};

struct _AUTH_GAME2{
	int 	Packet_Type;
	int		Packet_result;
	char	User_ID[52];
	char	User_roleName[20];
	int		Map_number;	
	int		User_co[2];	
	int		article_number;
	int		Dressed;
	int		Time_Exchanged;	
	int		ItemNo;		
	char	cardNumber[20];	
	char	cardPassword[20];
	char	messageflag[4];
	char	activeMessage[20];	
	char 	reserved[36];
};*/

#define g_cGame  (sizeof(_AUTH_GAME))

#endif
