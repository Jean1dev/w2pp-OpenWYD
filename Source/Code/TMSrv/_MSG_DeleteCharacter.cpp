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

void Exec_MSG_DeleteCharacter(int conn, char *pMsg)
{
	MSG_DeleteCharacter *m = (MSG_DeleteCharacter*)pMsg;

	m->MobName[NAME_LENGTH - 1] = 0;

	if (pUser[conn].Mode == USER_SELCHAR)
	{
		m->Type = _MSG_DBDeleteCharacter;
		m->ID = conn;

		pUser[conn].Mode = USER_WAITDB;

		DBServerSocket.SendOneMessage((char*)m, sizeof(MSG_DeleteCharacter));

		sprintf(temp, "etc,delchar name:%s %d %d", pMob[conn].MOB.MobName, conn, pUser[conn].Mode);
		Log(temp, pUser[conn].AccountName, pUser[conn].IP);
	}
	else
	{
		SendClientMessage(conn, "Deleting Character. wait a moment.");

		sprintf(temp, "err,delchar not user_selchar %d %d", conn, pUser[conn].Mode);
		Log(temp, pUser[conn].AccountName, pUser[conn].IP);
	}
}