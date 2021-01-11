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
#ifndef __GETFUNC__
#define __GETFUNC__

#include "..\Basedef.h"
#include <Windows.h>

int GetRecycleBin(int conn, STRUCT_ITEM *item, int price);
int	 GetMatchCombine		(STRUCT_ITEM *item);
int  GetMatchCombineEhre	(STRUCT_ITEM *Item);
int  GetMatchCombineTiny	(STRUCT_ITEM *Item);
int  GetMatchCombineShany	(STRUCT_ITEM *Item);
int  GetMatchCombineAilyn	(STRUCT_ITEM *Item);
int  GetMatchCombineAgatha	(STRUCT_ITEM *Item);
int  GetMatchCombineOdin	(STRUCT_ITEM *Item);
int  GetMatchCombineDedekinto(STRUCT_ITEM* Item);
int  GetMatchCombineAlquimia(STRUCT_ITEM *Item);
int  GetParryRate			(STRUCT_MOB *mob, int add, int attackerdex, int attackaccuracy);
int	 GetEmptyAffect			(int mob, int type);  // return value -1 if it's full.
int	 GetHide				(int mob);
int	 GetInView				(int a, int b);
int  GetInHalf				(int a, int b);
int	 GetTeleportPosition	(int conn, int *x, int * y); // returns the cost of the teleport
int	 GetExpApply			(STRUCT_MOBEXTRA extra, int exp, int attacker, int target);
int	 GetCreateMob			(int mob, MSG_CreateMob *sm);
int	 GetCreateMobTrade		(int mob, MSG_CreateMobTrade* sm);
void GetAffect              (unsigned short *out, STRUCT_AFFECT *affect);
void GetCreateItem			(int idx, MSG_CreateItem* sm);
void GetAction			    (int mob, int targetx, int targety, MSG_Action* sm);
unsigned char GetAttribute	(int x, int y);
void GetAttack             (int mob, int target, MSG_AttackOne *sm);
void GetAttackArea			(int mob, MSG_Attack *sm);
void GetGuild				(int conn);
int  GetGuild				(STRUCT_ITEM *item);
int  GetEmptyItemGrid		(int *gridx, int *gridy);
int	 GetEmptyMobGrid        (int conn, int  *tx, int *ty);
int  GetCurKill				(int conn);
int	 GetTotKill				(int conn);
void SetCurKill				(int conn, int value);
void SetTotKill				(int conn, int value);
int  GetPKPoint				(int conn);
int	 GetGuilty				(int conn);
void SetGuilty				(int conn, int value);
void SetPKPoint				(int conn, int value);
BOOL GetEmptyMobGridGreat	(int mob, int *tx, int *ty);


int GetAttackMiss(int conn, int target);

#endif


