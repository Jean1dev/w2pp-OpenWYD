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
#include "ProcessMessage.h"

extern HWND hWndMain;
extern CUser Admin;

void ProcessMessage(char *pMsg)
{
	HEADER *std = (HEADER*)pMsg;

	switch (std->Type)
	{
		case _MSG_NPReqIDPASS:
		{
			MSG_NPIDPASS *m = (MSG_NPIDPASS*)pMsg;

			Admin.Encode1 = m->Encode1;
			Admin.Encode2 = m->Encode2;

			//Conectado e pegou as chave de enc

			MSG_NPIDPASS sm;
			memset(&sm, 0, sizeof(MSG_NPIDPASS));

			sm.Type = _MSG_NPIDPASS;
			sm.Size = sizeof(MSG_NPIDPASS);

			GetDlgItemText(hWndMain, IDC_ELogin, sm.Account, 12);
			GetDlgItemText(hWndMain, IDC_EPass, sm.Pass, 12);

			sm.Encode1 = Admin.Encode1;
			sm.Encode2 = Admin.Encode2;

			Admin.cSock.SendOneMessage((char*)&sm, sizeof(MSG_NPIDPASS));

		} break;

		case _MSG_NPNotice:
		{
			MSG_NPNotice *m = (MSG_NPNotice*)pMsg;

			Log(m->String, "-system", 0);

			SendDlgItemMessage(hWndMain, IDC_LIST, LB_ADDSTRING, 0, (LPARAM)m->String);
		} break;

		case _MSG_NPAccountInfo:
		{
			MSG_NPAccountInfo *m = (MSG_NPAccountInfo*)pMsg;

			memcpy(&Admin.NPAccount, &m->account, sizeof(STRUCT_ACCOUNTFILE));

			SetDlgItemText(hWndMain, IDC_STFResult, "Result   : Found");

			if(m->Session == 0)
				SetDlgItemText(hWndMain, IDC_STFSession6, "Session: Not playing");
			else
				SetDlgItemText(hWndMain, IDC_STFSession6, "Session: Playing");

			if(m->State == 0)
				SetDlgItemText(hWndMain, IDC_STFState, "State     : Normal");
			else if(m->State == 1)
				SetDlgItemText(hWndMain, IDC_STFState, "State     : Blocked");
			else if(m->State == 2)
				SetDlgItemText(hWndMain, IDC_STFState, "State     : Damage");
			else
				SetDlgItemText(hWndMain, IDC_STFState, "State     : Disabled");

			Admin.Session = m->Session;
			Admin.State = m->State;

			char temp[256];

			sprintf(temp, "RealName : %s", m->account.Info.RealName);
			SetDlgItemText(hWndMain, IDC_STFRealName, temp);

			sprintf(temp, "Email : %s", m->account.Info.Email);
			SetDlgItemText(hWndMain, IDC_STFEmail, temp);

			sprintf(temp, "Telephone : %s", m->account.Info.Telephone);
			SetDlgItemText(hWndMain, IDC_STFTelephone, temp);

			sprintf(temp, "NumericToken : %s", m->account.Info.NumericToken);
			SetDlgItemText(hWndMain, IDC_STFNumericToken, temp);

			sprintf(temp, "Senha : %s", m->account.Info.AccountPass);
			SetDlgItemText(hWndMain, IDC_STFSenha, temp);

			Log(m->account.Info.AccountName, "_MSG_NPAccountInfo", 0);

			STRUCT_MOB *mob = &Admin.NPAccount.Char[0];

			SetDlgItemText(hWndMain, IDC_EName, mob->MobName);
			SetDlgItemInt(hWndMain, IDC_ERace, mob->Clan, TRUE);
			SetDlgItemInt(hWndMain, IDC_EMerc, mob->Merchant, TRUE);
			SetDlgItemInt(hWndMain, IDC_EClass, mob->Class, TRUE);

			SetDlgItemInt(hWndMain, IDC_ECoin, mob->Coin, TRUE);

			sprintf(temp, "%llu", mob->Exp);
			SetDlgItemText(hWndMain, IDC_EEXP, temp);

			SetDlgItemInt(hWndMain, IDC_EHX, mob->SPX, TRUE);
			SetDlgItemInt(hWndMain, IDC_EHY, mob->SPY, TRUE);

			sprintf(temp, "%d %d %d %d", mob->SkillBar[0], mob->SkillBar[1], mob->SkillBar[2], mob->SkillBar[3]);
			SetDlgItemText(hWndMain, IDC_ESkill, temp);

			SetDlgItemInt(hWndMain, IDC_ELearn, mob->LearnedSkill, FALSE);

			sprintf(temp, "%d %d %d %d", mob->Resist[0], mob->Resist[1], mob->Resist[2], mob->Resist[3]);
			SetDlgItemText(hWndMain, IDC_ERegist, temp);

			SetDlgItemInt(hWndMain, IDC_EBonus, mob->ScoreBonus, TRUE);

			SetDlgItemInt(hWndMain, IDC_ERegenHP, mob->RegenHP, FALSE);
			SetDlgItemInt(hWndMain, IDC_ERegenMP, mob->RegenMP, FALSE);

			SetDlgItemInt(hWndMain, IDC_EQuest, mob->Quest, FALSE);
			SetDlgItemInt(hWndMain, IDC_EDir, mob->Merchant, FALSE);

			sprintf(temp, "%d %d %d %d %d %d %d", mob->Equip[0].sIndex, mob->Equip[0].stEffect[0].cEffect, mob->Equip[0].stEffect[0].cValue, mob->Equip[0].stEffect[1].cEffect, mob->Equip[0].stEffect[1].cValue, mob->Equip[0].stEffect[2].cEffect, mob->Equip[0].stEffect[2].cValue);
			SetDlgItemText(hWndMain, IDC_EFace, temp);

			sprintf(temp, "%d %d %d %d %d %d %d", mob->Equip[1].sIndex, mob->Equip[1].stEffect[0].cEffect, mob->Equip[1].stEffect[1].cValue, mob->Equip[1].stEffect[0].cEffect, mob->Equip[1].stEffect[1].cValue, mob->Equip[1].stEffect[2].cEffect, mob->Equip[1].stEffect[2].cValue);
			SetDlgItemText(hWndMain, IDC_EHelm, temp);

			for(int i = 2; i < 16; i++)
			{
				sprintf(temp, "%d %d %d %d %d %d %d", mob->Equip[i].sIndex, mob->Equip[i].stEffect[0].cEffect, mob->Equip[i].stEffect[0].cValue, mob->Equip[i].stEffect[1].cEffect, mob->Equip[i].stEffect[1].cValue, mob->Equip[i].stEffect[2].cEffect, mob->Equip[i].stEffect[2].cValue);
				SetDlgItemText(hWndMain, IDC_EBody + i - 2, temp);
			}

			SetDlgItemInt(hWndMain, IDC_ESpecial1, mob->BaseScore.Special[0], FALSE);
			SetDlgItemInt(hWndMain, IDC_ESpecial2, mob->BaseScore.Special[1], FALSE);
			SetDlgItemInt(hWndMain, IDC_ESpecial3, mob->BaseScore.Special[2], FALSE);
			SetDlgItemInt(hWndMain, IDC_ESpecial4, mob->BaseScore.Special[3], FALSE);

			SetDlgItemInt(hWndMain, IDC_ELevel, mob->BaseScore.Level, FALSE);
			SetDlgItemInt(hWndMain, IDC_EAC, mob->BaseScore.Ac, FALSE);
			SetDlgItemInt(hWndMain, IDC_EDam, mob->BaseScore.Damage, FALSE);
			SetDlgItemInt(hWndMain, IDC_ERegen, mob->BaseScore.ChaosRate, FALSE);

			SetDlgItemInt(hWndMain, IDC_EStr, mob->BaseScore.Str, FALSE);
			SetDlgItemInt(hWndMain, IDC_EInt, mob->BaseScore.Int, FALSE);
			SetDlgItemInt(hWndMain, IDC_EDex, mob->BaseScore.Dex, FALSE);
			SetDlgItemInt(hWndMain, IDC_ECon, mob->BaseScore.Con, FALSE);

			SetDlgItemInt(hWndMain, IDC_EHP, mob->BaseScore.Hp, FALSE);
			SetDlgItemInt(hWndMain, IDC_EHPMax, mob->BaseScore.MaxHp, FALSE);

			SetDlgItemInt(hWndMain, IDC_EMP, mob->BaseScore.Mp, FALSE);
			SetDlgItemInt(hWndMain, IDC_EMPMax, mob->BaseScore.MaxMp, FALSE);

			sprintf(temp, "%d %d %d %d %d %d %d", mob->Carry[0].sIndex, mob->Carry[0].stEffect[0].cEffect, mob->Carry[0].stEffect[0].cValue, mob->Carry[0].stEffect[1].cEffect, mob->Carry[0].stEffect[1].cValue, mob->Carry[0].stEffect[2].cEffect, mob->Carry[0].stEffect[2].cValue);
			SetDlgItemText(hWndMain, IDC_EItem0, temp);

			for(int i = 1; i < 63; i++)
			{
				sprintf(temp, "%d %d %d %d %d %d %d", mob->Carry[i].sIndex, mob->Carry[i].stEffect[0].cEffect, mob->Carry[i].stEffect[0].cValue, mob->Carry[i].stEffect[1].cEffect, mob->Carry[i].stEffect[1].cValue, mob->Carry[i].stEffect[2].cEffect, mob->Carry[i].stEffect[2].cValue);
							
				SetDlgItemText(hWndMain, IDC_EItem1 + i-1, temp);
			}
		} break;

		case _MSG_MagicTrumpet:
		{
			MSG_MagicTrumpet *m = (MSG_MagicTrumpet*)pMsg;

			Log(m->String, "_MSG_MagicTrumpet", 0);
		} break;
	}
}