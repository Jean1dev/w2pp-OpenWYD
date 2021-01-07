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
#ifndef __CREAD_FILES_H__
#define __CREAD_FILES_H__

#include <stdio.h>
#include "..\Basedef.h"

class CReadFiles
{
public:
	CReadFiles();

	static void ReadSancRate();
	static void ReadQuestsRate();
	static void ReadCompRate();
	static void ReadMobMerc();
	static void ReadAdmin();
	static void ReadMacblock();
	static void ReadChallanger();
	static void WriteChallanger();
	static void ReadGuild();
	static void WriteGuild();

public:
	static int QuestExp[5][2];
	static int QuestCoin[5];
	static int QuestLevel[5][4];

private:
	static const char* const SANC_RATE_PATH;
	static const char* const QUESTS_RATE_PATH;
	static const char* const COMP_RATE_PATH;
	static const char* const MOB_MERC_PATH;
	static const char* const ADMIN_PATH;
	static const char* const MAC_BLOCK_PATH;
	static const char* const CHALL_PATH;
	static const char* const GUILD_PATH;
	static const char* const GUILDCHAMP_PATH;
};

#endif // __CREAD_FILES_H__