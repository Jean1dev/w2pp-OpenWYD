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

unsigned int CurrentTime = 0;
unsigned int LastSendTime = 0;

HWND hWndMain;

char temp[1024];
int LastLogDay = -1;

FILE *fLogFile;

STRUCT_MOB LoadMob[30000];

BOOL WINAPI WinMain(HINSTANCE hInstance, HINSTANCE hPrevInstance, LPSTR lpCmdLine, int nCmdShow)
{
	BOOL ret;
	MSG msg;

	hWndMain = CreateDialogParam(hInstance, MAKEINTRESOURCE(IDD_MainDlg), 0, MainDlgProc, 0);

	ShowWindow(hWndMain, nCmdShow);

	BASE_InitializeItemList();
	ReadMob("npc");

	SetTimer(hWndMain, ITEMNAME_TIMER, 500, 0);

	while ((ret = GetMessage(&msg, 0, 0, 0)) != 0)
	{
		if (ret == -1) /* error found */
			return -1;

		if (!IsDialogMessage(hWndMain, &msg))
		{
			TranslateMessage(&msg); /* translate virtual-key messages */
			DispatchMessage(&msg); /* send it to dialog procedure */
		}
	}
	return TRUE;
}


INT_PTR CALLBACK MainDlgProc(HWND hDlg, UINT uMsg, WPARAM wParam, LPARAM lParam)
{
	switch (uMsg)
	{
	case WM_TIMER:
	{
		if (wParam == ITEMNAME_TIMER)
		{
			HWND hwndList = GetDlgItem(hDlg, IDC_LNPCList);

			// Get selected index.
			int lbItem = (int)SendMessage(hwndList, LB_GETCURSEL, 0, 0);

			if (lbItem == -1)
				break;

			for (int i = 0; i < MAX_SHOPLIST; i++)
			{
				int invpos = (i % 9) + ((i / 9) * 27);
				
				int itemIndex = 0;
				int Eff[3] = { 0, 0, 0 };
				int Val[3] = { 0, 0, 0 };

				GetDlgItemText(hWndMain, IDC_EItem0 + i, temp, 128);
				sscanf(temp, "%d %d %d %d %d %d %d", &itemIndex, &Eff[0], &Val[0], &Eff[1], &Val[1], &Eff[2], &Val[2]);

				if (itemIndex < 0 || itemIndex >= MAX_ITEMLIST)
					continue;

				SetDlgItemText(hWndMain, IDC_EItemName1 + i, g_pItemList[itemIndex].Name);
			}
		}
	} break;

	case WM_COMMAND:
		switch (LOWORD(wParam))
		{
		case IDC_LNPCList:
		{
			HWND hwndList = GetDlgItem(hDlg, IDC_LNPCList);

			// Get selected index.
			int lbItem = (int)SendMessage(hwndList, LB_GETCURSEL, 0, 0);


			STRUCT_MOB *mob = &LoadMob[lbItem];

			SetDlgItemText(hWndMain, IDC_EName, mob->MobName);
			SetDlgItemInt(hWndMain, IDC_ERace, mob->Clan, TRUE);
			SetDlgItemInt(hWndMain, IDC_EMerchant, mob->Merchant, TRUE);
			SetDlgItemInt(hWndMain, IDC_EClass, mob->Class, TRUE);

			SetDlgItemInt(hWndMain, IDC_EMoney, mob->Coin, TRUE);

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

			SetDlgItemInt(hWndMain, IDC_EDirection, mob->BaseScore.AttackRun, FALSE);


			for (int i = 0; i < MAX_EQUIP; i++)
			{
				sprintf(temp, "%d %d %d %d %d %d %d", mob->Equip[i].sIndex, mob->Equip[i].stEffect[0].cEffect, mob->Equip[i].stEffect[0].cValue, mob->Equip[i].stEffect[1].cEffect, mob->Equip[i].stEffect[1].cValue, mob->Equip[i].stEffect[2].cEffect, mob->Equip[i].stEffect[2].cValue);
				SetDlgItemText(hWndMain, IDC_EFace + i, temp);
			}


			SetDlgItemInt(hWndMain, IDC_ELevel, mob->BaseScore.Level, FALSE);
			SetDlgItemInt(hWndMain, IDC_EAC, mob->BaseScore.Ac, FALSE);
			SetDlgItemInt(hWndMain, IDC_EDam, mob->BaseScore.Damage, FALSE);
			SetDlgItemInt(hWndMain, IDC_EDam2, mob->BaseScore.ChaosRate, FALSE);

			SetDlgItemInt(hWndMain, IDC_EStr, mob->BaseScore.Str, FALSE);
			SetDlgItemInt(hWndMain, IDC_EInt, mob->BaseScore.Int, FALSE);
			SetDlgItemInt(hWndMain, IDC_EDex, mob->BaseScore.Dex, FALSE);
			SetDlgItemInt(hWndMain, IDC_ECon, mob->BaseScore.Con, FALSE);

			SetDlgItemInt(hWndMain, IDC_ENear, mob->BaseScore.Special[0], FALSE);
			SetDlgItemInt(hWndMain, IDC_ENearMotion, mob->BaseScore.Special[1], FALSE);
			SetDlgItemInt(hWndMain, IDC_EFar, mob->BaseScore.Special[2], FALSE);
			SetDlgItemInt(hWndMain, IDC_EFarMotion, mob->BaseScore.Special[3], FALSE);

			SetDlgItemInt(hWndMain, IDC_EHP, mob->BaseScore.Hp, FALSE);
			SetDlgItemInt(hWndMain, IDC_EHPMax, mob->BaseScore.MaxHp, FALSE);

			SetDlgItemInt(hWndMain, IDC_EMP, mob->BaseScore.Mp, FALSE);
			SetDlgItemInt(hWndMain, IDC_EMPMax, mob->BaseScore.MaxMp, FALSE);

			for (int i = 0; i < MAX_SHOPLIST; i++)
			{
				int invpos = (i % 9) + ((i / 9) * 27);

				sprintf(temp, "%d %d %d %d %d %d %d", mob->Carry[invpos].sIndex, mob->Carry[invpos].stEffect[0].cEffect, mob->Carry[invpos].stEffect[0].cValue, mob->Carry[invpos].stEffect[1].cEffect, mob->Carry[invpos].stEffect[1].cValue, mob->Carry[invpos].stEffect[2].cEffect, mob->Carry[invpos].stEffect[2].cValue);

				SetDlgItemText(hWndMain, IDC_EItem0 + i, temp);
				SetDlgItemText(hWndMain, IDC_EItemName1 + i, g_pItemList[mob->Carry[invpos].sIndex].Name);
			}
		} break;

		case IDC_BSave:
		{
			HWND hwndList = GetDlgItem(hDlg, IDC_LNPCList);

			// Get selected index.
			int lbItem = (int)SendMessage(hwndList, LB_GETCURSEL, 0, 0);


			STRUCT_MOB *mob = &LoadMob[lbItem];

			GetDlgItemText(hWndMain, IDC_EName, mob->MobName, 16);
			mob->Clan = GetDlgItemInt(hWndMain, IDC_ERace, FALSE, TRUE);
			mob->Merchant = GetDlgItemInt(hWndMain, IDC_EMerchant, FALSE, TRUE);
			mob->Class = GetDlgItemInt(hWndMain, IDC_EClass, FALSE, TRUE);

			mob->Coin = GetDlgItemInt(hWndMain, IDC_EMoney, FALSE, TRUE);

			GetDlgItemText(hWndMain, IDC_EEXP, temp, 50);
			sscanf(temp, "%llu", &mob->Exp);

			mob->SPX = GetDlgItemInt(hWndMain, IDC_EHX, FALSE, TRUE);
			mob->SPY = GetDlgItemInt(hWndMain, IDC_EHY, FALSE, TRUE);


			GetDlgItemText(hWndMain, IDC_ESkill, temp, 128);
			sscanf(temp, "%hhu %hhu %hhu %hhu", &mob->SkillBar[0], &mob->SkillBar[1], &mob->SkillBar[2], &mob->SkillBar[3]);

			mob->LearnedSkill = GetDlgItemInt(hWndMain, IDC_ELearn, FALSE, FALSE);

			GetDlgItemText(hWndMain, IDC_ERegist, temp, 128);
			sscanf(temp, "%hhd %hhd %hhd %hhd", &mob->Resist[0], &mob->Resist[1], &mob->Resist[2], &mob->Resist[3]);

			mob->ScoreBonus = GetDlgItemInt(hWndMain, IDC_EBonus, FALSE, TRUE);

			mob->RegenHP = GetDlgItemInt(hWndMain, IDC_ERegenHP, FALSE, FALSE);
			mob->RegenMP = GetDlgItemInt(hWndMain, IDC_ERegenMP, FALSE, FALSE);

			mob->BaseScore.AttackRun = GetDlgItemInt(hWndMain, IDC_EDirection, FALSE, FALSE);

			for (int i = 0; i < MAX_EQUIP; i++)
			{
				GetDlgItemText(hWndMain, IDC_EFace + i, temp, 128);
				sscanf(temp, "%d %d %d %d %d %d %d", &mob->Equip[i].sIndex, &mob->Equip[i].stEffect[0].cEffect, &mob->Equip[i].stEffect[0].cValue, &mob->Equip[i].stEffect[1].cEffect, &mob->Equip[i].stEffect[1].cValue, &mob->Equip[i].stEffect[2].cEffect, &mob->Equip[i].stEffect[2].cValue);
			}

			mob->BaseScore.Special[0] = GetDlgItemInt(hWndMain, IDC_ENear, FALSE, FALSE);
			mob->BaseScore.Special[1] = GetDlgItemInt(hWndMain, IDC_ENearMotion, FALSE, FALSE);
			mob->BaseScore.Special[2] = GetDlgItemInt(hWndMain, IDC_EFar, FALSE, FALSE);
			mob->BaseScore.Special[3] = GetDlgItemInt(hWndMain, IDC_EFarMotion, FALSE, FALSE);

			mob->BaseScore.Level = GetDlgItemInt(hWndMain, IDC_ELevel, FALSE, FALSE);
			mob->BaseScore.Ac = GetDlgItemInt(hWndMain, IDC_EAC, FALSE, FALSE);
			mob->BaseScore.Damage = GetDlgItemInt(hWndMain, IDC_EDam, FALSE, FALSE);
			mob->BaseScore.ChaosRate = GetDlgItemInt(hWndMain, IDC_EDam2, FALSE, FALSE);

			mob->BaseScore.Str = GetDlgItemInt(hWndMain, IDC_EStr, FALSE, FALSE);
			mob->BaseScore.Int = GetDlgItemInt(hWndMain, IDC_EInt, FALSE, FALSE);
			mob->BaseScore.Dex = GetDlgItemInt(hWndMain, IDC_EDex, FALSE, FALSE);
			mob->BaseScore.Con = GetDlgItemInt(hWndMain, IDC_ECon, FALSE, FALSE);

			mob->BaseScore.Hp = GetDlgItemInt(hWndMain, IDC_EHP, FALSE, FALSE);
			mob->BaseScore.MaxHp = GetDlgItemInt(hWndMain, IDC_EHPMax, FALSE, FALSE);

			mob->BaseScore.Mp = GetDlgItemInt(hWndMain, IDC_EMP, FALSE, FALSE);
			mob->BaseScore.MaxMp = GetDlgItemInt(hWndMain, IDC_EMPMax, FALSE, FALSE);

			mob->CurrentScore = mob->BaseScore;

			for (int i = 0; i < MAX_SHOPLIST; i++)
			{
				int invpos = (i % 9) + ((i / 9) * 27);

				GetDlgItemText(hWndMain, IDC_EItem0 + i, temp, 128);
				sscanf(temp, "%d %d %d %d %d %d %d", &mob->Carry[invpos].sIndex, &mob->Carry[invpos].stEffect[0].cEffect, &mob->Carry[invpos].stEffect[0].cValue, &mob->Carry[invpos].stEffect[1].cEffect, &mob->Carry[invpos].stEffect[1].cValue, &mob->Carry[invpos].stEffect[2].cEffect, &mob->Carry[invpos].stEffect[2].cValue);
			}

			int handle = -1;

			char filename[256];
			sprintf(filename, "./%s/%s", "NPC", mob->MobName);

			handle = _open(filename, O_RDWR | O_CREAT | O_BINARY, _S_IREAD | _S_IWRITE);

			_write(handle, &mob->MobName, sizeof(STRUCT_MOB));
			_close(handle);

			sprintf(temp, "[%s] Save!!!", mob->MobName);

			SetDlgItemText(hWndMain, IDC_EChinese, mob->MobName);

			SetWindowText(hWndMain, temp);
		} break;

		case IDC_BReload:
		{
			HWND hwndList = GetDlgItem(hDlg, IDC_LNPCList);

			// Get selected index.
			int lbItem = (int)SendMessage(hwndList, LB_GETCURSEL, 0, 0);


			STRUCT_MOB *mob = &LoadMob[lbItem];

			ReadMob(mob, "npc");

			SetDlgItemText(hWndMain, IDC_EName, mob->MobName);
			SetDlgItemInt(hWndMain, IDC_ERace, mob->Clan, TRUE);
			SetDlgItemInt(hWndMain, IDC_EMerchant, mob->Merchant, TRUE);
			SetDlgItemInt(hWndMain, IDC_EClass, mob->Class, TRUE);

			SetDlgItemInt(hWndMain, IDC_EMoney, mob->Coin, TRUE);

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

			SetDlgItemInt(hWndMain, IDC_EDirection, mob->BaseScore.Direction, FALSE);


			for (int i = 0; i < MAX_EQUIP; i++)
			{
				sprintf(temp, "%d %d %d %d %d %d %d", mob->Equip[i].sIndex, mob->Equip[i].stEffect[i].cEffect, mob->Equip[i].stEffect[i].cValue, mob->Equip[i].stEffect[1].cEffect, mob->Equip[i].stEffect[1].cValue, mob->Equip[i].stEffect[2].cEffect, mob->Equip[i].stEffect[2].cValue);
				SetDlgItemText(hWndMain, IDC_EFace + i, temp);
			}


			SetDlgItemInt(hWndMain, IDC_ELevel, mob->BaseScore.Level, FALSE);
			SetDlgItemInt(hWndMain, IDC_EAC, mob->BaseScore.Ac, FALSE);
			SetDlgItemInt(hWndMain, IDC_EDam, mob->BaseScore.Damage, FALSE);
			SetDlgItemInt(hWndMain, IDC_EDam2, mob->BaseScore.ChaosRate, FALSE);

			SetDlgItemInt(hWndMain, IDC_EStr, mob->BaseScore.Str, FALSE);
			SetDlgItemInt(hWndMain, IDC_EInt, mob->BaseScore.Int, FALSE);
			SetDlgItemInt(hWndMain, IDC_EDex, mob->BaseScore.Dex, FALSE);
			SetDlgItemInt(hWndMain, IDC_ECon, mob->BaseScore.Con, FALSE);

			SetDlgItemInt(hWndMain, IDC_ENear, mob->BaseScore.Special[0], FALSE);
			SetDlgItemInt(hWndMain, IDC_ENearMotion, mob->BaseScore.Special[1], FALSE);
			SetDlgItemInt(hWndMain, IDC_EFar, mob->BaseScore.Special[2], FALSE);
			SetDlgItemInt(hWndMain, IDC_EFarMotion, mob->BaseScore.Special[3], FALSE);

			SetDlgItemInt(hWndMain, IDC_EHP, mob->BaseScore.Hp, FALSE);
			SetDlgItemInt(hWndMain, IDC_EHPMax, mob->BaseScore.MaxHp, FALSE);

			SetDlgItemInt(hWndMain, IDC_EMP, mob->BaseScore.Mp, FALSE);
			SetDlgItemInt(hWndMain, IDC_EMPMax, mob->BaseScore.MaxMp, FALSE);

			for (int i = 0; i < MAX_SHOPLIST; i++)
			{
				int invpos = (i % 9) + ((i / 9) * 27);

				sprintf(temp, "%d %d %d %d %d %d %d", mob->Carry[invpos].sIndex, mob->Carry[invpos].stEffect[0].cEffect, mob->Carry[invpos].stEffect[0].cValue, mob->Carry[invpos].stEffect[1].cEffect, mob->Carry[invpos].stEffect[1].cValue, mob->Carry[invpos].stEffect[2].cEffect, mob->Carry[invpos].stEffect[2].cValue);

				SetDlgItemText(hWndMain, IDC_EItem0 + i, temp);
				SetDlgItemText(hWndMain, IDC_EItemName1 + i, g_pItemList[mob->Carry[invpos].sIndex].Name);
			}

			sprintf(temp, "[%s] Reload!!!", mob->MobName);

			SetWindowText(hWndMain, temp);
		} break;

		case IDC_BClear:
		{
			HWND hwndList = GetDlgItem(hDlg, IDC_LNPCList);

			// Get selected index.
			int lbItem = (int)SendMessage(hwndList, LB_GETCURSEL, 0, 0);

			STRUCT_MOB *mob = &LoadMob[lbItem];

			for (int i = 0; i < MAX_SHOPLIST; i++)
			{
				int invpos = (i % 9) + ((i / 9) * 27);

				memset(&mob->Carry[invpos], 0, sizeof(STRUCT_ITEM));
				sprintf(temp, "%d %d %d %d %d %d %d", mob->Carry[invpos].sIndex, mob->Carry[invpos].stEffect[0].cEffect, mob->Carry[invpos].stEffect[0].cValue, mob->Carry[invpos].stEffect[1].cEffect, mob->Carry[invpos].stEffect[1].cValue, mob->Carry[invpos].stEffect[2].cEffect, mob->Carry[invpos].stEffect[2].cValue);

				SetDlgItemText(hWndMain, IDC_EItem0 + i, temp);
			}

			sprintf(temp, "[%s] Clear!!!", mob->MobName);

			SetWindowText(hWndMain, temp);
		} break;

		case IDC_BDelete:
		{
			HWND hwndList = GetDlgItem(hDlg, IDC_LNPCList);

			// Get selected index.
			int lbItem = (int)SendMessage(hwndList, LB_GETCURSEL, 0, 0);


			STRUCT_MOB *mob = &LoadMob[lbItem];

			char filename[256];

			sprintf(filename, "./%s/%s", "npc", mob->MobName);
			DeleteFileA(filename);

			sprintf(temp, "[%s] Delete!!!", mob->MobName);

			SetWindowText(hWndMain, temp);

			memset(mob, 0, sizeof(STRUCT_MOB));

			SetDlgItemText(hWndMain, IDC_EName, mob->MobName);
			SetDlgItemInt(hWndMain, IDC_ERace, mob->Clan, TRUE);
			SetDlgItemInt(hWndMain, IDC_EMerchant, mob->Merchant, TRUE);
			SetDlgItemInt(hWndMain, IDC_EClass, mob->Class, TRUE);

			SetDlgItemInt(hWndMain, IDC_EMoney, mob->Coin, TRUE);

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

			SetDlgItemInt(hWndMain, IDC_EDirection, mob->BaseScore.Direction, FALSE);


			for (int i = 0; i < MAX_EQUIP; i++)
			{
				sprintf(temp, "%d %d %d %d %d %d %d", mob->Equip[i].sIndex, mob->Equip[i].stEffect[i].cEffect, mob->Equip[i].stEffect[i].cValue, mob->Equip[i].stEffect[1].cEffect, mob->Equip[i].stEffect[1].cValue, mob->Equip[i].stEffect[2].cEffect, mob->Equip[i].stEffect[2].cValue);
				SetDlgItemText(hWndMain, IDC_EFace + i, temp);
			}


			SetDlgItemInt(hWndMain, IDC_ELevel, mob->BaseScore.Level, FALSE);
			SetDlgItemInt(hWndMain, IDC_EAC, mob->BaseScore.Ac, FALSE);
			SetDlgItemInt(hWndMain, IDC_EDam, mob->BaseScore.Damage, FALSE);
			SetDlgItemInt(hWndMain, IDC_EDam2, mob->BaseScore.ChaosRate, FALSE);

			SetDlgItemInt(hWndMain, IDC_EStr, mob->BaseScore.Str, FALSE);
			SetDlgItemInt(hWndMain, IDC_EInt, mob->BaseScore.Int, FALSE);
			SetDlgItemInt(hWndMain, IDC_EDex, mob->BaseScore.Dex, FALSE);
			SetDlgItemInt(hWndMain, IDC_ECon, mob->BaseScore.Con, FALSE);

			SetDlgItemInt(hWndMain, IDC_ENear, mob->BaseScore.Special[0], FALSE);
			SetDlgItemInt(hWndMain, IDC_ENearMotion, mob->BaseScore.Special[1], FALSE);
			SetDlgItemInt(hWndMain, IDC_EFar, mob->BaseScore.Special[2], FALSE);
			SetDlgItemInt(hWndMain, IDC_EFarMotion, mob->BaseScore.Special[3], FALSE);

			SetDlgItemInt(hWndMain, IDC_EHP, mob->BaseScore.Hp, FALSE);
			SetDlgItemInt(hWndMain, IDC_EHPMax, mob->BaseScore.MaxHp, FALSE);

			SetDlgItemInt(hWndMain, IDC_EMP, mob->BaseScore.Mp, FALSE);
			SetDlgItemInt(hWndMain, IDC_EMPMax, mob->BaseScore.MaxMp, FALSE);

			for (int i = 0; i < MAX_CARRY; i++)
			{
				sprintf(temp, "%d %d %d %d %d %d %d", mob->Carry[i].sIndex, mob->Carry[i].stEffect[0].cEffect, mob->Carry[i].stEffect[0].cValue, mob->Carry[i].stEffect[1].cEffect, mob->Carry[i].stEffect[1].cValue, mob->Carry[i].stEffect[2].cEffect, mob->Carry[i].stEffect[2].cValue);

				if (i < 27)
					SetDlgItemText(hWndMain, IDC_EItem0 + i, temp);
			}

			SendDlgItemMessage(hWndMain, IDC_LNPCList, LB_DELETESTRING, lbItem, (LPARAM)0);
		} break;

		}break;

	case WM_CLOSE:
		if (MessageBox(hDlg, "Você deseja fechar o aplicativo?", "Close", MB_ICONQUESTION | MB_YESNO) == IDYES)
			DestroyWindow(hDlg);
		return TRUE;

	case WM_DESTROY:
		PostQuitMessage(0);
		return TRUE;
	}

	return FALSE;
}

