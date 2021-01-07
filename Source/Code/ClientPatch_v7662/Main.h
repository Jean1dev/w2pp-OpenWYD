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

#include <stdio.h>
#include <time.h>
#include <iostream>
#include <stdarg.h>
#include <string.h>
#include <stdlib.h>
#include <windows.h>
#include "PE_Hook.h"
#include "..\Basedef.h"

//////////////////////////

static void(*SendPacket)(char*, int) = (void(__cdecl *)(char*, int)) 0x54DA23;

//////////////////////////

int		SendPack(const int client, char* const pMsg, const int len);
char*	ReadMessage(char* pMsg, int pSize);
int		SendChat(char *command);
bool	SetVolatileMessageBoxText(int itemID, char* text);
int		AddVolatileMessageItem(int itemID);

//////////////////////////

void LoadHooks();

//////////////////////////

