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

char temp[1024];
int LastLogDay = -1;

FILE *fLogFile;

CUser Admin;

BOOL WINAPI WinMain(HINSTANCE hInstance, HINSTANCE hPrevInstance, LPSTR lpCmdLine, int nCmdShow)
{
	BOOL ret;
	MSG msg;

	hWndMain = CreateDialogParam(hInstance, MAKEINTRESOURCE(IDD_DIALOGBAR), 0, MainDlgProc, 0);

	ShowWindow(hWndMain, nCmdShow);
	
	StartLog();

	BASE_InitializeServerList();


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

	sprintf(temp, ".\\Log\\BI_%02d_%02d_%04d_%02d_%02d_%02d.txt", when.tm_mday, when.tm_mon + 1, when.tm_year + 1900, when.tm_hour, when.tm_min, when.tm_sec);

	fLogFile = fopen(temp, "wt");

	LastLogDay = when.tm_mday;
}
