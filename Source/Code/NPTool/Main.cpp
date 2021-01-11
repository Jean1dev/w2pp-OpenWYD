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
#include "CUser.h"
#include "ProcessMessage.h"

unsigned int CurrentTime = 0;
unsigned int LastSendTime = 0;

HWND hWndMain;
HWND hWndBanimento;

char temp[1024];
int LastLogDay = -1;

FILE *fLogFile;

CUser Admin;

BOOL WINAPI WinMain(HINSTANCE hInstance, HINSTANCE hPrevInstance, LPSTR lpCmdLine, int nCmdShow)
{
	BOOL ret;
	MSG msg;

	hWndMain = CreateDialogParam(hInstance, MAKEINTRESOURCE(IDD_MainDlg), 0, MainDlgProc, 0);

	ShowWindow(hWndMain, nCmdShow);
	
	StartLog();

	BASE_InitializeServerList();

	SendDlgItemMessage(hWndMain, IDC_LIST1, LB_ADDSTRING, 0, (LPARAM)"Char 0");
	SendDlgItemMessage(hWndMain, IDC_LIST1, LB_ADDSTRING, 0, (LPARAM)"Char 1");
	SendDlgItemMessage(hWndMain, IDC_LIST1, LB_ADDSTRING, 0, (LPARAM)"Char 2");
	SendDlgItemMessage(hWndMain, IDC_LIST1, LB_ADDSTRING, 0, (LPARAM)"Char 3");

	SendDlgItemMessage(hWndMain, IDC_LIST1, LB_ADDSTRING, 0, (LPARAM)"Storage 0");
	SendDlgItemMessage(hWndMain, IDC_LIST1, LB_ADDSTRING, 0, (LPARAM)"Storage 1");

	while((ret = GetMessage(&msg, 0, 0, 0)) != 0) 
	{
		if(ret == -1) /* error found */
			return -1;

		if(!IsDialogMessage(hWndMain, &msg)) 
		{
			TranslateMessage(&msg); /* translate virtual-key messages */
			DispatchMessage(&msg); /* send it to dialog procedure */
		}
	}
	return TRUE;
}

INT_PTR CALLBACK MainDlgProc(HWND hDlg, UINT uMsg, WPARAM wParam, LPARAM lParam)
{
	switch(uMsg)
	{
	case WM_COMMAND:
		switch(LOWORD(wParam))
		{
		case IDC_BDelete:
		{
			HWND hwndList = GetDlgItem(hDlg, IDC_LIST1); 

            // Get selected index.
            int lbItem = (int)SendMessage(hwndList, LB_GETCURSEL, 0, 0);

			if(lbItem < 4)
			{
				memset(&Admin.NPAccount.Char[lbItem], 0, sizeof(STRUCT_MOB));
				memset(&Admin.NPAccount.mobExtra[lbItem], 0, sizeof(STRUCT_MOBEXTRA));

				STRUCT_MOB *mob = &Admin.NPAccount.Char[lbItem];

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

				sprintf(temp, "%d %d %d %d %d %d %d", mob->Equip[1].sIndex, mob->Equip[1].stEffect[0].cEffect, mob->Equip[1].stEffect[0].cValue, mob->Equip[1].stEffect[1].cEffect, mob->Equip[1].stEffect[1].cValue, mob->Equip[1].stEffect[2].cEffect, mob->Equip[1].stEffect[2].cValue);
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
			}
			else
			{
				memset(&Admin.NPAccount.Cargo[(lbItem-4)*63], 0, sizeof(STRUCT_ITEM)*63);

				if(lbItem == 4)
				{
					sprintf(temp, "%d %d %d %d %d %d %d", Admin.NPAccount.Cargo[0].sIndex, Admin.NPAccount.Cargo[0].stEffect[0].cEffect, Admin.NPAccount.Cargo[0].stEffect[0].cValue, Admin.NPAccount.Cargo[0].stEffect[1].cEffect, Admin.NPAccount.Cargo[0].stEffect[1].cValue, Admin.NPAccount.Cargo[0].stEffect[2].cEffect, Admin.NPAccount.Cargo[0].stEffect[2].cValue);
					SetDlgItemText(hWndMain, IDC_EItem0, temp);

					for(int i = 1; i < 63; i++)
					{
						sprintf(temp, "%d %d %d %d %d %d %d", Admin.NPAccount.Cargo[i].sIndex, Admin.NPAccount.Cargo[i].stEffect[0].cEffect, Admin.NPAccount.Cargo[i].stEffect[0].cValue, Admin.NPAccount.Cargo[i].stEffect[1].cEffect, Admin.NPAccount.Cargo[i].stEffect[1].cValue, Admin.NPAccount.Cargo[i].stEffect[2].cEffect, Admin.NPAccount.Cargo[i].stEffect[2].cValue);
							
						SetDlgItemText(hWndMain, IDC_EItem1 + i-1, temp);
					}
				}
				else if(lbItem == 5)
				{
					sprintf(temp, "%d %d %d %d %d %d %d", Admin.NPAccount.Cargo[63].sIndex, Admin.NPAccount.Cargo[63].stEffect[0].cEffect, Admin.NPAccount.Cargo[63].stEffect[0].cValue, Admin.NPAccount.Cargo[63].stEffect[1].cEffect, Admin.NPAccount.Cargo[63].stEffect[1].cValue, Admin.NPAccount.Cargo[63].stEffect[2].cEffect, Admin.NPAccount.Cargo[63].stEffect[2].cValue);
					SetDlgItemText(hWndMain, IDC_EItem0, temp);

					for(int i = 1; i < 63; i++)
					{
						sprintf(temp, "%d %d %d %d %d %d %d", Admin.NPAccount.Cargo[i+63].sIndex, Admin.NPAccount.Cargo[i+63].stEffect[0].cEffect, Admin.NPAccount.Cargo[i+63].stEffect[0].cValue, Admin.NPAccount.Cargo[i+63].stEffect[1].cEffect, Admin.NPAccount.Cargo[i+63].stEffect[1].cValue, Admin.NPAccount.Cargo[i+63].stEffect[2].cEffect, Admin.NPAccount.Cargo[i+63].stEffect[2].cValue);
							
						SetDlgItemText(hWndMain, IDC_EItem1 + i-1, temp);
					}
				}
			}
		} break;

		case IDC_RBSrv1:
		case IDC_RBSrv2:
		case IDC_RBSrv3:
		case IDC_RBSrv4:
		case IDC_RBSrv5:
		case IDC_RBSrv6:
		case IDC_RBSrv7:
		case IDC_RBSrv8:
		case IDC_RBSrv9:
			Admin.DB = IDC_RBSrv1 - LOWORD(wParam);
			break;

		case IDC_RBExtra:
			Admin.DB = 0;
			break;

		case IDC_BConnect2://Botão connect
		{
			Admin.cSock.WSAInitialize();
			
			SOCKET s = Admin.cSock.ConnectServer(g_pServerList[Admin.DB][0], 8895, 0, WSA_READ);

			if (s == NULL)
				MessageBoxA(hDlg, "Não foi possivel se conectar ao servidor.", "Warning!!!", MB_ICONEXCLAMATION | MB_OK);

		} break;

		case IDC_BNotice:
		{
			MSG_NPNotice sm;
			memset(&sm, 0, sizeof(MSG_NPNotice));

			sm.Type = _MSG_NPNotice;
			sm.Size = sizeof(MSG_NPNotice);

			GetDlgItemText(hWndMain, IDC_ENotice, sm.String, MESSAGE_LENGTH);

			sm.Parm1 = 1;
			sm.Parm2 = 0;

			Admin.cSock.SendOneMessage((char*)&sm, sizeof(MSG_NPNotice));
		} break;

		case IDC_BFAccount:
		{
			MSG_NPReqAccount sm;
			memset(&sm, 0, sizeof(MSG_NPReqAccount));

			sm.Type = _MSG_NPReqAccount;
			sm.Size = sizeof(MSG_NPReqAccount);

			GetDlgItemText(hWndMain, IDC_EFAccount, sm.Account, 16);

			strcpy(sm.Char, "");

			SetDlgItemText(hWndMain, IDC_STFResult, "Result   : Not found");
			SetDlgItemText(hWndMain, IDC_STFSession6, "Session: Not playing");
			SetDlgItemText(hWndMain, IDC_STFState, "State     : Normal");

			Admin.cSock.SendOneMessage((char*)&sm, sizeof(MSG_NPReqAccount));
		} break;

		case IDC_BFChar:
		{
			MSG_NPReqAccount sm;
			memset(&sm, 0, sizeof(MSG_NPReqAccount));

			sm.Type = _MSG_NPReqAccount;
			sm.Size = sizeof(MSG_NPReqAccount);

			GetDlgItemText(hWndMain, IDC_EFChar, sm.Char, 16);

			strcpy(sm.Account, "");

			SetDlgItemText(hWndMain, IDC_STFResult,	  "Result   : Not found");
			SetDlgItemText(hWndMain, IDC_STFSession6, "Session: Not playing");
			SetDlgItemText(hWndMain, IDC_STFState,	  "State     : Normal");

			Admin.cSock.SendOneMessage((char*)&sm, sizeof(MSG_NPReqAccount));
		} break;

		case IDC_LIST1:
		{
			switch (HIWORD(wParam)) 
            { 
            case LBN_SELCHANGE:
                {
                    HWND hwndList = GetDlgItem(hDlg, IDC_LIST1); 

                    // Get selected index.
                    int lbItem = (int)SendMessage(hwndList, LB_GETCURSEL, 0, 0); 

					if(lbItem < 4)
					{
						STRUCT_MOB *mob = &Admin.NPAccount.Char[lbItem];

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

						sprintf(temp, "%d %d %d %d %d %d %d", mob->Equip[1].sIndex, mob->Equip[1].stEffect[0].cEffect, mob->Equip[1].stEffect[0].cValue, mob->Equip[1].stEffect[1].cEffect, mob->Equip[1].stEffect[1].cValue, mob->Equip[1].stEffect[2].cEffect, mob->Equip[1].stEffect[2].cValue);
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
					}
					else if(lbItem == 4)
					{
						sprintf(temp, "%d %d %d %d %d %d %d", Admin.NPAccount.Cargo[0].sIndex, Admin.NPAccount.Cargo[0].stEffect[0].cEffect, Admin.NPAccount.Cargo[0].stEffect[0].cValue, Admin.NPAccount.Cargo[0].stEffect[1].cEffect, Admin.NPAccount.Cargo[0].stEffect[1].cValue, Admin.NPAccount.Cargo[0].stEffect[2].cEffect, Admin.NPAccount.Cargo[0].stEffect[2].cValue);
						SetDlgItemText(hWndMain, IDC_EItem0, temp);

						for(int i = 1; i < 63; i++)
						{
							sprintf(temp, "%d %d %d %d %d %d %d", Admin.NPAccount.Cargo[i].sIndex, Admin.NPAccount.Cargo[i].stEffect[0].cEffect, Admin.NPAccount.Cargo[i].stEffect[0].cValue, Admin.NPAccount.Cargo[i].stEffect[1].cEffect, Admin.NPAccount.Cargo[i].stEffect[1].cValue, Admin.NPAccount.Cargo[i].stEffect[2].cEffect, Admin.NPAccount.Cargo[i].stEffect[2].cValue);
							
							SetDlgItemText(hWndMain, IDC_EItem1 + i-1, temp);
						}
					}
					else if(lbItem == 5)
					{
						sprintf(temp, "%d %d %d %d %d %d %d", Admin.NPAccount.Cargo[63].sIndex, Admin.NPAccount.Cargo[63].stEffect[0].cEffect, Admin.NPAccount.Cargo[63].stEffect[0].cValue, Admin.NPAccount.Cargo[63].stEffect[1].cEffect, Admin.NPAccount.Cargo[63].stEffect[1].cValue, Admin.NPAccount.Cargo[63].stEffect[2].cEffect, Admin.NPAccount.Cargo[63].stEffect[2].cValue);
						SetDlgItemText(hWndMain, IDC_EItem0, temp);

						for(int i = 1; i < 63; i++)
						{
							sprintf(temp, "%d %d %d %d %d %d %d", Admin.NPAccount.Cargo[i+63].sIndex, Admin.NPAccount.Cargo[i+63].stEffect[0].cEffect, Admin.NPAccount.Cargo[i+63].stEffect[0].cValue, Admin.NPAccount.Cargo[i+63].stEffect[1].cEffect, Admin.NPAccount.Cargo[i+63].stEffect[1].cValue, Admin.NPAccount.Cargo[i+63].stEffect[2].cEffect, Admin.NPAccount.Cargo[i+63].stEffect[2].cValue);
							
							SetDlgItemText(hWndMain, IDC_EItem1 + i-1, temp);
						}
					}
                    break;
                } 
            }
		} break;

		case IDC_BConfirm:
		{
			HWND hwndList = GetDlgItem(hDlg, IDC_LIST1); 

            // Get selected index.
            int lbItem = (int)SendMessage(hwndList, LB_GETCURSEL, 0, 0); 

			if(lbItem < 4)
			{
				STRUCT_MOB *mob = &Admin.NPAccount.Char[lbItem];

				GetDlgItemText(hWndMain, IDC_EName, mob->MobName, 12);
				mob->Clan = GetDlgItemInt(hWndMain, IDC_ERace, FALSE, TRUE);
				mob->Merchant = GetDlgItemInt(hWndMain, IDC_EMerc, FALSE, TRUE);
				mob->Class = GetDlgItemInt(hWndMain, IDC_EClass, FALSE, TRUE);

				mob->Coin = GetDlgItemInt(hWndMain, IDC_ECoin, FALSE, TRUE);

				GetDlgItemText(hWndMain, IDC_EEXP, temp, 50);
				sscanf(temp, "%llu", &mob->Exp);

				mob->SPX = GetDlgItemInt(hWndMain, IDC_EHX, FALSE, TRUE);
				mob->SPY = GetDlgItemInt(hWndMain, IDC_EHY, FALSE, TRUE);

	
				GetDlgItemText(hWndMain, IDC_ESkill, temp, 128);
				sscanf(temp, "%d %d %d %d", &mob->SkillBar[0], &mob->SkillBar[1], &mob->SkillBar[2], &mob->SkillBar[3]);

				mob->LearnedSkill = GetDlgItemInt(hWndMain, IDC_ELearn, FALSE, FALSE);

				
				GetDlgItemText(hWndMain, IDC_ERegist, temp, 128);
				sscanf(temp, "%d %d %d %d", &mob->Resist[0], &mob->Resist[1], &mob->Resist[2], &mob->Resist[3]);

				mob->ScoreBonus = GetDlgItemInt(hWndMain, IDC_EBonus, FALSE, TRUE);

				mob->RegenHP = GetDlgItemInt(hWndMain, IDC_ERegenHP, FALSE, FALSE);
				mob->RegenMP = GetDlgItemInt(hWndMain, IDC_ERegenMP, FALSE, FALSE);

				mob->Quest = GetDlgItemInt(hWndMain, IDC_EQuest, FALSE, FALSE);
				mob->Merchant = GetDlgItemInt(hWndMain, IDC_EDir, FALSE, FALSE);

				GetDlgItemText(hWndMain, IDC_EFace, temp, 128);
				sscanf(temp, "%d %d %d %d %d %d %d", &mob->Equip[0].sIndex, &mob->Equip[0].stEffect[0].cEffect, &mob->Equip[0].stEffect[0].cValue, &mob->Equip[0].stEffect[1].cEffect, &mob->Equip[0].stEffect[1].cValue, &mob->Equip[0].stEffect[2].cEffect, &mob->Equip[0].stEffect[2].cValue);

				GetDlgItemText(hWndMain, IDC_EHelm, temp, 128);
				sscanf(temp, "%d %d %d %d %d %d %d", &mob->Equip[1].sIndex, &mob->Equip[1].stEffect[0].cEffect, &mob->Equip[1].stEffect[0].cValue, &mob->Equip[1].stEffect[1].cEffect, &mob->Equip[1].stEffect[1].cValue, &mob->Equip[1].stEffect[2].cEffect, &mob->Equip[1].stEffect[2].cValue);

				for(int i = 2; i < 16; i++)
				{
					GetDlgItemText(hWndMain, IDC_EBody + i - 2, temp, 128);
					sscanf(temp, "%d %d %d %d %d %d %d", &mob->Equip[i].sIndex, &mob->Equip[i].stEffect[0].cEffect, &mob->Equip[i].stEffect[0].cValue, &mob->Equip[i].stEffect[1].cEffect, &mob->Equip[i].stEffect[1].cValue, &mob->Equip[i].stEffect[2].cEffect, &mob->Equip[i].stEffect[2].cValue);
				}

				mob->BaseScore.Special[0] = GetDlgItemInt(hWndMain, IDC_ESpecial1, FALSE, FALSE);
				mob->BaseScore.Special[1] = GetDlgItemInt(hWndMain, IDC_ESpecial2, FALSE, FALSE);
				mob->BaseScore.Special[2] = GetDlgItemInt(hWndMain, IDC_ESpecial3, FALSE, FALSE);
				mob->BaseScore.Special[3] = GetDlgItemInt(hWndMain, IDC_ESpecial4, FALSE, FALSE);

				mob->BaseScore.Level = GetDlgItemInt(hWndMain, IDC_ELevel, FALSE, FALSE);
				mob->BaseScore.Ac = GetDlgItemInt(hWndMain, IDC_EAC, FALSE, FALSE);
				mob->BaseScore.Damage = GetDlgItemInt(hWndMain, IDC_EDam, FALSE, FALSE);
				mob->BaseScore.ChaosRate = GetDlgItemInt(hWndMain, IDC_ERegen, FALSE, FALSE);

				mob->BaseScore.Str = GetDlgItemInt(hWndMain, IDC_EStr, FALSE, FALSE);
				mob->BaseScore.Int = GetDlgItemInt(hWndMain, IDC_EInt, FALSE, FALSE);
				mob->BaseScore.Dex = GetDlgItemInt(hWndMain, IDC_EDex, FALSE, FALSE);
				mob->BaseScore.Con = GetDlgItemInt(hWndMain, IDC_ECon, FALSE, FALSE);

				mob->BaseScore.Hp = GetDlgItemInt(hWndMain, IDC_EHP, FALSE, FALSE);
				mob->BaseScore.MaxHp = GetDlgItemInt(hWndMain, IDC_EHPMax, FALSE, FALSE);

				mob->BaseScore.Mp = GetDlgItemInt(hWndMain, IDC_EMP, FALSE, FALSE);
				mob->BaseScore.MaxMp = GetDlgItemInt(hWndMain, IDC_EMPMax, FALSE, FALSE);

				GetDlgItemText(hWndMain, IDC_EItem0, temp, 128);
				sscanf(temp, "%d %d %d %d %d %d %d", &mob->Carry[0].sIndex, &mob->Carry[0].stEffect[0].cEffect, &mob->Carry[0].stEffect[0].cValue, &mob->Carry[0].stEffect[1].cEffect, &mob->Carry[0].stEffect[1].cValue, &mob->Carry[0].stEffect[2].cEffect, &mob->Carry[0].stEffect[2].cValue);

				for(int i = 1; i < 63; i++)
				{
					GetDlgItemText(hWndMain, IDC_EItem1 + i-1, temp, 128);
					sscanf(temp, "%d %d %d %d %d %d %d", &mob->Carry[i].sIndex, &mob->Carry[i].stEffect[0].cEffect, &mob->Carry[i].stEffect[0].cValue, &mob->Carry[i].stEffect[1].cEffect, &mob->Carry[i].stEffect[1].cValue, &mob->Carry[i].stEffect[2].cEffect, &mob->Carry[i].stEffect[2].cValue);
				}
			}
			else if(lbItem == 4)
			{
				GetDlgItemText(hWndMain, IDC_EItem0, temp, 128);
				sscanf(temp, "%d %d %d %d %d %d %d", &Admin.NPAccount.Cargo[0].sIndex, &Admin.NPAccount.Cargo[0].stEffect[0].cEffect, &Admin.NPAccount.Cargo[0].stEffect[0].cValue, &Admin.NPAccount.Cargo[0].stEffect[1].cEffect, &Admin.NPAccount.Cargo[0].stEffect[1].cValue, &Admin.NPAccount.Cargo[0].stEffect[2].cEffect, &Admin.NPAccount.Cargo[0].stEffect[2].cValue);

				for(int i = 1; i < 63; i++)
				{	
					GetDlgItemText(hWndMain, IDC_EItem1 + i-1, temp, 128);
					sscanf(temp, "%d %d %d %d %d %d %d", &Admin.NPAccount.Cargo[i].sIndex, &Admin.NPAccount.Cargo[i].stEffect[0].cEffect, &Admin.NPAccount.Cargo[i].stEffect[0].cValue, &Admin.NPAccount.Cargo[i].stEffect[1].cEffect, &Admin.NPAccount.Cargo[i].stEffect[1].cValue, &Admin.NPAccount.Cargo[i].stEffect[2].cEffect, &Admin.NPAccount.Cargo[i].stEffect[2].cValue);				
				}
			}
			else if(lbItem == 5)
			{
				GetDlgItemText(hWndMain, IDC_EItem0, temp, 128);
				sscanf(temp, "%d %d %d %d %d %d %d", &Admin.NPAccount.Cargo[63].sIndex, &Admin.NPAccount.Cargo[63].stEffect[0].cEffect, &Admin.NPAccount.Cargo[63].stEffect[0].cValue, &Admin.NPAccount.Cargo[63].stEffect[1].cEffect, &Admin.NPAccount.Cargo[63].stEffect[1].cValue, &Admin.NPAccount.Cargo[63].stEffect[2].cEffect, &Admin.NPAccount.Cargo[63].stEffect[2].cValue);

				for(int i = 1; i < 63; i++)
				{	
					GetDlgItemText(hWndMain, IDC_EItem1 + i-1, temp, 128);
					sscanf(temp, "%d %d %d %d %d %d %d", &Admin.NPAccount.Cargo[i+63].sIndex, &Admin.NPAccount.Cargo[i+63].stEffect[0].cEffect, &Admin.NPAccount.Cargo[i+63].stEffect[0].cValue, &Admin.NPAccount.Cargo[i+63].stEffect[1].cEffect, &Admin.NPAccount.Cargo[i+63].stEffect[1].cValue, &Admin.NPAccount.Cargo[i+63].stEffect[2].cEffect, &Admin.NPAccount.Cargo[i+63].stEffect[2].cValue);				
				}
			}
		} break;

		case IDC_BSaveAccount:
		{
			MSG_NPAccountInfo sm;
			memset(&sm, 0, sizeof(MSG_NPAccountInfo));

			sm.Type = _MSG_NPReqSaveAccount;
			sm.Size = sizeof(MSG_NPAccountInfo);

			memcpy(&sm.account, &Admin.NPAccount, sizeof(STRUCT_ACCOUNTFILE));
			sm.Session = Admin.Session;
			sm.State = Admin.State;

			Admin.cSock.SendOneMessage((char*)&sm, sizeof(MSG_NPAccountInfo));
		} break;

		case IDC_BFEnable:
		{
			MSG_NPEnable sm;
			memset(&sm, 0, sizeof(MSG_NPEnable));

			sm.Type = _MSG_NPEnable;
			sm.Size = sizeof(MSG_NPEnable);

			strcpy(sm.AccountName, Admin.NPAccount.Info.AccountName);

			Admin.cSock.SendOneMessage((char*)&sm, sizeof(MSG_NPEnable));
		} break;

		case IDC_BFDisable:
		{
			MSG_NPEnable sm;
			memset(&sm, 0, sizeof(MSG_NPEnable));

			sm.Type = _MSG_NPDisable;
			sm.Size = sizeof(MSG_NPEnable);

			strcpy(sm.AccountName, Admin.NPAccount.Info.AccountName);

			struct tm when;
			time_t now;
			time(&now);
			when = *localtime(&now);

			int Year = when.tm_year + 20;
			int YearDay = when.tm_yday;

			sm.Year = Year;
			sm.YearDay = YearDay;

			Admin.cSock.SendOneMessage((char*)&sm, sizeof(MSG_NPEnable));
		} break;

		}break;


	case WSA_READ:
	{
		if (WSAGETSELECTEVENT(lParam) != FD_READ)
		{
			Admin.cSock.CloseSocket();
			break;
		}

		CurrentTime = timeGetTime();

		if (Admin.cSock.Receive() != 1)
			break;

		char temp[256];
		int Error;
		int ErrorCode;

		while (1)
		{
			char *Msg = Admin.cSock.ReadMessage(&Error, &ErrorCode);

			if (Msg == NULL)
				break;

			if (Error == 1 || Error == 2)
			{
				sprintf(temp, "err wsa_read (%d),%d", Error, ErrorCode);
				Log(temp, "-system", 0);

				break;
			}

			ProcessMessage(Msg);
		};
	} break;

	case WM_CLOSE:
		if(MessageBox(hDlg, "Você deseja fechar o aplicativo?", "Close", MB_ICONQUESTION | MB_YESNO) == IDYES)
			DestroyWindow(hDlg);
		return TRUE;

	case WM_DESTROY:
		Admin.cSock.CloseSocket();
		PostQuitMessage(0);
		return TRUE;
	}

	return FALSE;
}

void Log(char *str1, char *str2, unsigned int ip)
{
	struct tm when;
	time_t now;
	time(&now);
	when = *localtime(&now);

	char LogTemp[1024];

	unsigned char *cIP = (unsigned char*)&ip;


	if (ip != 0)
		sprintf(LogTemp, "[%02d/%02d/%04d][%02d:%02d:%02d] IP: %d.%d.%d.%d", when.tm_mday, when.tm_mon + 1, when.tm_year + 1900, when.tm_hour, when.tm_min, when.tm_sec, cIP[0], cIP[1], cIP[2], cIP[3]);
	else
		sprintf(LogTemp, "[%02d/%02d/%04d][%02d:%02d:%02d] ", when.tm_mday, when.tm_mon + 1, when.tm_year + 1900, when.tm_hour, when.tm_min, when.tm_sec);
	sprintf(LogTemp, "%s %s %s \n", LogTemp, str2, str1);

	if (fLogFile)
		fprintf(fLogFile, LogTemp);

	SetWindowText(hWndMain, LogTemp);
}

void StartLog()
{
	if (fLogFile != NULL)
		fclose(fLogFile);

	struct tm when;
	time_t now;
	time(&now);
	when = *localtime(&now);

	sprintf(temp, ".\\Log\\NP_%02d_%02d_%04d_%02d_%02d_%02d.txt", when.tm_mday, when.tm_mon + 1, when.tm_year + 1900, when.tm_hour, when.tm_min, when.tm_sec);

	fLogFile = fopen(temp, "wt");

	LastLogDay = when.tm_mday;
}
