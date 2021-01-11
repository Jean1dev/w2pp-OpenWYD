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

#include <Windows.h>
#include "Main.h"

void strip_xtrap()
{
	*(char*)0x5D9491 = 0xEB;
}

void strip_odd_connection_request()
{
	// This weird connection request to 211.115.86.66:2424 that always fails make the client laggy.
	// This patch kill the call.
	*(short*)0x5D8E8C = 0x9090;
}

int __stdcall DllMain(HINSTANCE hInstDLL, DWORD catchReason, LPVOID lpResrv)
{
	
	if (catchReason == DLL_PROCESS_ATTACH)
	{
		DWORD dwOldProtectFlag_text;

		VirtualProtect((void*)0x401000, 0x1F3000, PAGE_READWRITE, &dwOldProtectFlag_text);

		LoadHooks();
		/*char buffer[MAX_PATH] = { "", };
		GetModuleFileNameA(NULL, buffer, MAX_PATH);
		auto  lpFilename = (strrchr(buffer, '\\') + 1);
		auto WindowHandler = GetActiveWindow();
		if (strcmp(lpFilename, "WYD.EXE") || !strcmp(lpFilename, "WYDLauncher.exe"))
		{
			MessageBoxA(WindowHandler, "Execute o jogo pelo launcher.", "WYD Launcher", 0);
			SendMessage(WindowHandler, WM_SYSCOMMAND, SC_CLOSE, 0);
			TerminateProcess(WindowHandler, 0);
			return FALSE;
		}*/
		strip_xtrap();
		strip_odd_connection_request();
		VirtualProtect((void*)0x401000, 0x1F3000, dwOldProtectFlag_text, &dwOldProtectFlag_text);
	}
	else if (catchReason == DLL_PROCESS_DETACH)
	{
		FreeLibrary(hInstDLL);
	}

	return TRUE;
}