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
#ifndef __CITEM__
#define __CITEM__

#include "../Basedef.h"

class CItem
{  
public:
	STRUCT_ITEM ITEM;
	int  Mode;       
	int  PosX, PosY;
	int  GridCharge;
	int  State;
	int  Rotate;
	int  Delay;
	int  Decay;
	int  Height;
	int  Money;
	int  Open;
	int  ItemQuest;
	char Unk[20];

public:
	CItem();
	~CItem();
};
extern CItem pItem[MAX_ITEM];

#endif