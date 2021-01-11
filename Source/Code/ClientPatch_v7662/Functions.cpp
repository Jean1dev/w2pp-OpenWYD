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

#include "Main.h"

int Buff = 0;

int SendPack(const int conn, char* const pMsg, const int len)
{
	int retnValue = 0;

	__asm
	{
		PUSH len
			PUSH pMsg
			MOV ECX, conn
			IMUL ECX, ECX, 0xC58
			ADD ECX, 0x752BAF8

			MOV EAX, 0x40126A
			CALL EAX

			MOV retnValue, EAX
	}
	return retnValue;
}

char* ReadMessage(char* pMsg, int pSize)
{
	auto header = reinterpret_cast<MSG_STANDARD*>(pMsg);

	DWORD OldProtect;
	VirtualProtect((int*)0x0401000, 0x5F0FFF - 0x0401000, 0x40, &OldProtect);

	if (header->Type == 0xD1D)
	{
		*(int*)0x4A016A = 0xFF00CD00;
		return pMsg;
	}

	VirtualProtect((int*)0x0401000, 0x5F0FFF - 0x0401000, OldProtect, &OldProtect);
	return pMsg;
}

int SendChat(char *command)
{
	DWORD OldProtect;
	VirtualProtect((int*)0x0401000, 0x5F0FFF - 0x0401000, 0x40, &OldProtect);
	//	if (*command != '@')
	//		return true;

	VirtualProtect((int*)0x0401000, 0x5F0FFF - 0x0401000, OldProtect, &OldProtect);
	
	return true;
}


bool SetVolatileMessageBoxText(int itemID, char* text)
{
	if (itemID == 3314)
	{
		sprintf(text, "Deseja comer esse delicioso Frango Assado?");
		return true;
	}

	return false;
}

int AddVolatileMessageItem(int itemID)
{
	switch (itemID)
	{
	case 3314:
		return 1;
	}
	return 0;
}