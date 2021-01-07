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
#pragma once

#include <stdio.h>
#include "..\Basedef.h"


class CCastleZakum
{
private:
	static const char* const CASTLE_QUEST_PATH;

public:
	static BOOL OpenCastleGate(int conn, int gateid, MSG_UpdateItem *m);
	static void MobKilled(int target, int conn, int PosX, int PosY);
	static BOOL KeyDrop(int target, int conn, int PosX, int PosY, STRUCT_ITEM *Key);

	static void ProcessSecTimer();
	static void ProcessMinTimer();

	static void CheckMove(int conn, int targetx, int targety);

	static void ReadCastleQuest();
	static int  ParseCastleString(int Num, char *str);
};
