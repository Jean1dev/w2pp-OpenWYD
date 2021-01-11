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

void Exec_MSG_ChangeCity(int conn, char *pMsg)
{
	int tx = pMob[conn].TargetX;
	int ty = pMob[conn].TargetY;

	int city = BASE_GetVillage(tx, ty);

	if (city >= 0 && city <= 4)
	{
		pMob[conn].MOB.Merchant &= 0x3F;
		pMob[conn].MOB.Merchant |= city << 6;
	}

	sprintf(temp, "etc,changecity city:%d X:%d Y:%d", city, tx, ty);
	Log(temp, pUser[conn].AccountName, 0);
}