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

void Exec_MSG_AccountLogin(int conn, char *pMsg)
{
	MSG_AccountLogin *m = (MSG_AccountLogin*)pMsg;

	int Size = m->Size;
	int ClientVersion = APP_VERSION;

	pUser[conn].AccountName[NAME_LENGTH - 1] = 0;

	if (conn <= 0 || (conn >= (MAX_USER - ADMIN_RESERV)))
	{
		sprintf(temp, g_pMessageStringTable[_NN_Reconnect]);
		SendClientMessage(conn, temp);

		pUser[conn].cSock.SendMessageA();

		CloseUser(conn);
		return;
	}
		
#ifdef _PACKET_DEBUG
	if (Size < sizeof(MSG_AccountLogin))
#else
	if (Size < sizeof(MSG_AccountLogin) || m->ClientVersion != ClientVersion)
#endif
	{
		sprintf(temp, g_pMessageStringTable[_NN_Version_Not_Match_Rerun], APP_VERSION);
		SendClientMessage(conn, temp);

		pUser[conn].cSock.SendMessageA();

		CloseUser(conn);
		return;
	}
			
	if(pUser[conn].Mode != USER_ACCEPT)
	{
		SendClientMessage(conn, "Login now, wait a moment.");

		CrackLog(conn, " accountlogin");
		pUser[conn].cSock.SendMessageA(); 
		return;
	}

	int tMac = 0;

	if (m->Size < sizeof(MSG_AccountLogin))
		memset(pUser[conn].Mac, 0xFF, sizeof(pUser[conn].Mac));
	else
		memcpy(pUser[conn].Mac, m->AdapterName, sizeof(pUser[conn].Mac));


	m->Type = _MSG_DBAccountLogin;
	m->ID = conn;

	sscanf(m->AccountName, "%s", pUser[conn].AccountName);

	_strupr(pUser[conn].AccountName);

	strncpy(m->AccountName, pUser[conn].AccountName, NAME_LENGTH);

	int check = CheckFailAccount(m->AccountName);

	if(check >= 3)
	{
		SendClientMessage(conn, g_pMessageStringTable[_NN_3_Tims_Wrong_Pass]);

		pUser[conn].cSock.SendMessageA();
		return;
	}

	DBServerSocket.SendOneMessage((char*)m, sizeof(MSG_AccountLogin));

	pUser[conn].Mode = USER_LOGIN;
	pMob[conn].Mode = MOB_EMPTY;
}