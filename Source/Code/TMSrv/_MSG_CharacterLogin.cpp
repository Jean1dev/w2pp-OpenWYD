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

void Exec_MSG_CharacterLogin(int conn, char *pMsg)
{
	MSG_CharacterLogin *m = (MSG_CharacterLogin*)pMsg;

	if (m->Slot < 0 || m->Slot >= MOB_PER_ACCOUNT)
	{
		SendClientMessage(conn, g_pMessageStringTable[_NN_SelectCharacter]);
		return;
	}

	if (pUser[conn].SelChar.Score[m->Slot].Level < FREEEXP || pUser[conn].SelChar.Score[m->Slot].Level >= 999 || BILLING != 2)
		goto Label1;

	if (pUser[conn].Unk_1816 > 1)
	{
		if (pUser[conn].Unk_1816 == 3)
		{
			sprintf(temp, g_pMessageStringTable[_DN_Not_Allowed_Account], FREEEXP);
			SendClientMessage(conn, temp);

			SendClientSignalParm(conn, 0, 404, 0);
			pUser[conn].cSock.SendMessageA();
		}
		else
		{
			if (pUser[conn].Unk_1816 == 4)
				SendClientMessage(conn, g_pMessageStringTable[_NN_Using_Other_Server_Group]);

			else
			{
			Label1:
				
				if (pUser[conn].SelChar.Score[m->Slot].Level >= FREEEXP && pUser[conn].SelChar.Score[m->Slot].Level < 999
					&& BILLING == 3 && pUser[conn].SelChar.Score[m->Slot].Level >= 1000)
				{
					if (pUser[conn].Unk_1816 <= 1)
						SendClientMessage(conn, g_pMessageStringTable[_NN_Wait_Checking_Billing]);

					if (pUser[conn].Unk_1816 == 3)
					{
						sprintf(temp, g_pMessageStringTable[_DN_Not_Allowed_Account], FREEEXP);
						SendClientMessage(conn, temp);
					}

					if (pUser[conn].Unk_1816 == 4)
						SendClientMessage(conn, g_pMessageStringTable[_NN_Using_Other_Server_Group]);
				}

				if (BILLING != 2 || pUser[conn].Unk_2728 != 1 || pUser[conn].SelChar.Score[m->Slot].Level < FREEEXP || g_Hour > 7 && g_Hour < 23)
				{
					if (pUser[conn].Mode == USER_SELCHAR)
					{
						m->Type = _MSG_DBCharacterLogin;
						m->ID = conn;

						pUser[conn].Mode = USER_CHARWAIT;
						pMob[conn].Mode = MOB_USER;

						pMob[conn].MOB.Merchant = 0;

						DBServerSocket.SendOneMessage((char*)m, sizeof(MSG_CharacterLogin));
					}
					else
					{
						SendClientMessage(conn, "Wait a moment.");
						sprintf(temp, "err,charlogin not user_selchar %d %d", conn, pUser[conn].Mode);
						Log(temp, pUser[conn].AccountName, pUser[conn].IP);
					}
				}
				else
					SendClientMessage(conn, g_pMessageStringTable[_NN_Child_Pay]);
			}
		}
	}
	else
	{
		if (pUser[conn].Unk_2732 && pUser[conn].Unk_2732 < SecCounter - 10)
		{
			pUser[conn].Unk_2732 = 0;
			pUser[conn].Unk_1816 = 5;
		}
		else
		{
			SendClientMessage(conn, g_pMessageStringTable[_NN_Wait_Checking_Billing]);
			SendBilling(conn, pUser[conn].AccountName, 1, 1);
		}
	}

	return;
}