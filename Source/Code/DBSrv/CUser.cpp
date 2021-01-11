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

#include "CUser.h"

extern HWND hWndMain; // Server.cpp

CUser::CUser()
{
	Mode = USER_EMPTY;
	IP   = 0;
	Count = 0;
	DisableID = 0;
	Level = -1;
}

CUser::~CUser()
{
}

BOOL CUser::AcceptUser(int ListenSocket, int wsa)
{
	SOCKADDR_IN acc_sin; 
	int Size = sizeof(acc_sin);

	int tSock = accept(ListenSocket, (struct sockaddr FAR *)&acc_sin, (int FAR *)&(Size));

	if(tSock < 0) 
		return FALSE;

	if(WSAAsyncSelect(tSock, hWndMain, wsa , FD_READ | FD_CLOSE) > 0)
	{ 
		closesocket(tSock);
		return FALSE;
	}

	cSock.Sock          = tSock;
	cSock.nRecvPosition = 0;
	cSock.nProcPosition = 0;
	cSock.nSendPosition = 0;

	char * tmp = inet_ntoa(acc_sin.sin_addr);
	IP		   = acc_sin.sin_addr.S_un.S_addr;
	Mode	   = USER_ACCEPT;

	return TRUE;
}