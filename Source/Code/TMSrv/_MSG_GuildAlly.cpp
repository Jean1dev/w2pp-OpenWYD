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

void Exec_MSG_GuildAlly(int conn, char *pMsg)
{
	MSG_GuildAlly *m = (MSG_GuildAlly*)pMsg;

	int Guild = m->Guild;
	int Ally = m->Ally;

	if(Guild <= 0 || Guild >= 65536)
		return;

	if(Ally <= 0 || Ally >= 65536)
		return;

	if(pMob[conn].MOB.Guild != Guild || pMob[conn].MOB.GuildLevel != 9)
		return;

	MSG_STANDARDPARM2 sm_ga;
	memset(&sm_ga, 0, sizeof(MSG_STANDARDPARM2));

	sm_ga.Size = sizeof(MSG_STANDARDPARM2);
	sm_ga.Type = _MSG_GuildAlly;
	sm_ga.ID = conn;

	sm_ga.Parm1 = Guild;
	sm_ga.Parm2 = Ally;

	DBServerSocket.SendOneMessage((char*)&sm_ga, sizeof(MSG_STANDARDPARM2));
}