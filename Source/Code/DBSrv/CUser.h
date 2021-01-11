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

#ifndef _CUSER_ // Last updated July 2012
#define _CUSER_

#include "../Basedef.h"
#include "../CPSock.h"

#define USER_EMPTY    0   // is empty
#define USER_ACCEPT   1   // ACCEPT state after connected

class CUser
{
public:
	unsigned int    IP;
	int    Mode;
	CPSock cSock;
	int    Count;
	int    Level;
	int    Encode1;
	int    Encode2;
	char   Name[ACCOUNTNAME_LENGTH];
	int    DisableID;

	int Year;
	int YearDay;

public:
	CUser();
	~CUser();

	BOOL AcceptUser(int ListenSocket, int WSA);
};

#endif