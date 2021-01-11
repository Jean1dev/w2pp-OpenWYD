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

void Exec_MSG_War(int conn, char *pMsg)
{
	MSG_STANDARDPARM2 *m = (MSG_STANDARDPARM2*)pMsg;

	int Guild = m->Parm1;
	int Enemy = m->Parm2;

	if(Guild <= 0 || Guild >= 65536)
		return;

	if(Enemy <= 0 || Enemy >= 65536)
		return;

	if(pMob[conn].MOB.Guild != Guild || pMob[conn].MOB.GuildLevel != 9)
		return;

	MSG_STANDARDPARM2 sm_war;
	memset(&sm_war, 0, sizeof(MSG_STANDARDPARM2));

	sm_war.Size = sizeof(MSG_STANDARDPARM2);
	sm_war.Type = _MSG_War;
	sm_war.ID = conn;

	sm_war.Parm1 = Guild;
	sm_war.Parm2 = Enemy;

	DBServerSocket.SendOneMessage((char*)&sm_war, sizeof(MSG_STANDARDPARM2));
}